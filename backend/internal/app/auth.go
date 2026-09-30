package app

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func validCredentials(c credentials) bool {
	m, e := mail.ParseAddress(c.Email)
	return e == nil && m.Address == c.Email && len(c.Password) >= 10 && len(c.Password) <= 72
}
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	if !validCredentials(c) {
		fail(w, 400, "Podaj e-mail i hasło od 10 do 72 znaków")
		return
	}
	p, _ := bcrypt.GenerateFromPassword([]byte(c.Password), 12)
	err := transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		var id string
		e := tx.QueryRow(r.Context(), `INSERT INTO users(email,password) VALUES($1,$2) RETURNING id`, c.Email, string(p)).Scan(&id)
		if e != nil {
			return e
		}
		return issueToken(r.Context(), tx, id, c.Email, "verify", "")
	})
	if err != nil {
		fail(w, 400, "Nie udało się zarejestrować. Jeśli masz konto, zaloguj się lub zresetuj hasło.")
		return
	}
	respond(w, 201, map[string]string{"message": "Sprawdź e-mail i potwierdź konto."})
}
func issueToken(ctx context.Context, tx pgx.Tx, uid, email, kind, org string) error {
	raw := token()
	var u, o any
	if uid != "" {
		u = uid
	}
	if org != "" {
		o = org
	}
	_, err := tx.Exec(ctx, `INSERT INTO tokens(token,user_id,email,kind,organization_id,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '24 hours')`, hash(raw), u, email, kind, o)
	if err != nil {
		return err
	}
	link := env("PUBLIC_URL", "http://localhost:3000") + "/konto?token=" + raw + "&kind=" + kind
	return queue(ctx, tx, "email", map[string]string{"to": email, "subject": "Strefa Treningów — " + map[string]string{"verify": "potwierdź konto", "reset": "zmień hasło", "invite": "zaproszenie do zespołu"}[kind], "body": "Otwórz link (ważny 24 godziny):\n" + link}, hash(raw))
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decode(w, r, &c) {
		return
	}
	var id, p string
	var verified bool
	err := a.DB.QueryRow(r.Context(), `SELECT id,password,verified FROM users WHERE email=$1`, strings.ToLower(strings.TrimSpace(c.Email))).Scan(&id, &p, &verified)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(p), []byte(c.Password)) != nil {
		fail(w, 401, "Nieprawidłowy e-mail lub hasło")
		return
	}
	if !verified {
		fail(w, 403, "Najpierw potwierdź adres e-mail")
		return
	}
	raw, csrf := token(), token()
	_, err = a.DB.Exec(r.Context(), `INSERT INTO sessions(token,user_id,csrf,expires_at) VALUES($1,$2,$3,now()+interval '7 days')`, hash(raw), id, csrf)
	if err != nil {
		dbfail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "strefa_session", Value: raw, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(env("PUBLIC_URL", ""), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 604800})
	respond(w, 200, map[string]string{"csrf": csrf})
}
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	u, ok := r.Context().Value(identityKey).(identity)
	if !ok {
		fail(w, 401, "Zaloguj się")
		return
	}
	respond(w, 200, map[string]any{"id": u.ID, "email": u.Email, "admin": u.Admin, "csrf": u.CSRF})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("strefa_session"); e == nil {
		_, _ = a.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token=$1`, hash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "strefa_session", Value: "", Path: "/", HttpOnly: true, Secure: strings.HasPrefix(env("PUBLIC_URL", ""), "https://"), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	respond(w, 200, map[string]bool{"ok": true})
}
func (a *App) resetRequest(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &b) {
		return
	}
	_ = transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		var id, email string
		if err := tx.QueryRow(r.Context(), `SELECT id,email FROM users WHERE email=$1`, strings.ToLower(strings.TrimSpace(b.Email))).Scan(&id, &email); err != nil {
			return err
		}
		return issueToken(r.Context(), tx, id, email, "reset", "")
	})
	respond(w, 200, map[string]string{"message": "Jeśli konto istnieje, wysłaliśmy wiadomość."})
}
func (a *App) consumeToken(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	err := transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		var uid, org *string
		var kind, email string
		var expires time.Time
		if e := tx.QueryRow(r.Context(), `SELECT user_id,organization_id,kind,email,expires_at FROM tokens WHERE token=$1 FOR UPDATE`, hash(b.Token)).Scan(&uid, &org, &kind, &email, &expires); e != nil {
			return e
		}
		if time.Now().After(expires) {
			return errInvalid
		}
		switch kind {
		case "verify":
			if _, e := tx.Exec(r.Context(), `UPDATE users SET verified=true WHERE id=$1`, uid); e != nil {
				return e
			}
		case "reset":
			if len(b.Password) < 10 || len(b.Password) > 72 {
				return errInvalid
			}
			p, _ := bcrypt.GenerateFromPassword([]byte(b.Password), 12)
			if _, e := tx.Exec(r.Context(), `UPDATE users SET password=$1,verified=true WHERE id=$2`, string(p), uid); e != nil {
				return e
			}
			if _, e := tx.Exec(r.Context(), `DELETE FROM sessions WHERE user_id=$1`, uid); e != nil {
				return e
			}
		case "invite":
			u := user(r)
			if u.ID == "" || u.Email != email || r.Header.Get("X-CSRF-Token") != u.CSRF {
				return fmtError("zaloguj się na zaproszone konto")
			}
			if _, e := tx.Exec(r.Context(), `INSERT INTO memberships VALUES($1,$2,'editor') ON CONFLICT DO NOTHING`, org, u.ID); e != nil {
				return e
			}
		}
		_, e := tx.Exec(r.Context(), `DELETE FROM tokens WHERE token=$1`, hash(b.Token))
		return e
	})
	if err != nil {
		fail(w, 400, "Link wygasł, jest nieprawidłowy lub wymaga zalogowania na zaproszone konto.")
		return
	}
	respond(w, 200, json.RawMessage(`{"ok":true}`))
}
