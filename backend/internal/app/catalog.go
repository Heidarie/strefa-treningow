package app

import (
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
)

func (a *App) panel(w http.ResponseWriter, r *http.Request) {
	uid := user(r).ID
	queries := map[string]string{
		"organizations": `SELECT to_jsonb(o)||jsonb_build_object('role',m.role) FROM organizations o JOIN memberships m ON m.organization_id=o.id WHERE m.user_id=$1 ORDER BY o.name`,
		"locations":     `SELECT to_jsonb(l)-'point'||jsonb_build_object('lat',ST_Y(point::geometry),'lng',ST_X(point::geometry)) FROM locations l JOIN memberships m ON m.organization_id=l.organization_id WHERE m.user_id=$1 ORDER BY l.name`,
		"trainings":     `SELECT to_jsonb(t) FROM trainings t JOIN locations l ON l.id=t.location_id JOIN memberships m ON m.organization_id=l.organization_id WHERE m.user_id=$1 ORDER BY t.name`,
		"series":        `SELECT to_jsonb(s) FROM series s JOIN trainings t ON t.id=s.training_id JOIN locations l ON l.id=t.location_id JOIN memberships m ON m.organization_id=l.organization_id WHERE m.user_id=$1`,
		"occurrences":   `SELECT to_jsonb(oc) FROM occurrences oc JOIN trainings t ON t.id=oc.training_id JOIN locations l ON l.id=t.location_id JOIN memberships m ON m.organization_id=l.organization_id WHERE m.user_id=$1 AND oc.starts_at>=now()-interval '1 day' ORDER BY starts_at`,
		"members":       `SELECT jsonb_build_object('organization_id',m.organization_id,'user_id',u.id,'email',u.email,'role',m.role) FROM memberships m JOIN users u ON u.id=m.user_id JOIN memberships mine ON mine.organization_id=m.organization_id WHERE mine.user_id=$1`,
		"media":         `SELECT to_jsonb(d) FROM media d JOIN memberships m ON m.organization_id=d.organization_id WHERE m.user_id=$1`,
		"warnings":      `SELECT to_jsonb(w) FROM schedule_warnings w JOIN series s ON s.id=w.series_id JOIN trainings t ON t.id=s.training_id JOIN locations l ON l.id=t.location_id JOIN memberships m ON m.organization_id=l.organization_id WHERE m.user_id=$1`,
	}
	out := map[string]any{}
	for k, q := range queries {
		v, e := rowsJSON(r.Context(), a.DB, q, uid)
		if e != nil {
			dbfail(w, e)
			return
		}
		out[k] = v
	}
	respond(w, 200, out)
}
func (a *App) createOrganization(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !required(b.Name) {
		fail(w, 400, "Podaj nazwę")
		return
	}
	var id string
	e := transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		if e := tx.QueryRow(r.Context(), `INSERT INTO organizations(name) VALUES($1) RETURNING id`, b.Name).Scan(&id); e != nil {
			return e
		}
		_, e := tx.Exec(r.Context(), `INSERT INTO memberships VALUES($1,$2,'owner')`, id, user(r).ID)
		return e
	})
	idResult(w, id, e)
}
func (a *App) updateOrganization(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.member(r.Context(), user(r).ID, id, true) {
		fail(w, 403, "Brak uprawnień właściciela")
		return
	}
	var b struct {
		Name string `json:"name"`
		Logo string `json:"logo"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !required(b.Name) || !a.ownedMedia(r, b.Logo, id) {
		fail(w, 400, "Nieprawidłowa nazwa lub logo")
		return
	}
	_, e := a.DB.Exec(r.Context(), `UPDATE organizations SET name=$1,logo=$2 WHERE id=$3`, b.Name, b.Logo, id)
	idResult(w, id, e)
}
func (a *App) submitOrganization(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.member(r.Context(), user(r).ID, id, true) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	tag, e := a.DB.Exec(r.Context(), `UPDATE organizations SET status='pending',reason='' WHERE id=$1 AND status IN ('draft','rejected')`, id)
	if e != nil {
		dbfail(w, e)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "Organizacja jest już zgłoszona, zatwierdzona lub zawieszona")
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
func (a *App) invite(w http.ResponseWriter, r *http.Request) {
	org := r.PathValue("id")
	if !a.member(r.Context(), user(r).ID, org, true) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	var b struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	m, e := mail.ParseAddress(b.Email)
	if e != nil || m.Address != b.Email {
		fail(w, 400, "Nieprawidłowy e-mail")
		return
	}
	e = transact(r.Context(), a.DB, func(tx pgx.Tx) error { return issueToken(r.Context(), tx, "", b.Email, "invite", org) })
	idResult(w, org, e)
}
func (a *App) removeMember(w http.ResponseWriter, r *http.Request) {
	org := r.PathValue("id")
	if !a.member(r.Context(), user(r).ID, org, true) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	_, e := a.DB.Exec(r.Context(), `DELETE FROM memberships WHERE organization_id=$1 AND user_id=$2 AND role='editor'`, org, r.PathValue("user"))
	idResult(w, org, e)
}
func (a *App) ownedMedia(r *http.Request, path, org string) bool {
	if path == "" {
		return true
	}
	var ok bool
	e := a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM media WHERE organization_id=$1 AND status='ready' AND (url=$2 OR thumbnail=$2))`, org, path).Scan(&ok)
	return e == nil && ok
}
func (a *App) saveLocation(w http.ResponseWriter, r *http.Request) {
	var b struct {
		OrganizationID string  `json:"organization_id"`
		Name           string  `json:"name"`
		City           string  `json:"city"`
		Address        string  `json:"address"`
		Logo           string  `json:"logo"`
		Lat            float64 `json:"lat"`
		Lng            float64 `json:"lng"`
		Hidden         bool    `json:"hidden"`
	}
	if !decode(w, r, &b) {
		return
	}
	id := r.PathValue("id")
	org := b.OrganizationID
	if id != "" {
		org = a.orgFor(r.Context(), "location", id)
	}
	if !a.member(r.Context(), user(r).ID, org, false) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	if !required(b.Name, b.City, b.Address) || math.Abs(b.Lat) > 90 || math.Abs(b.Lng) > 180 || !a.ownedMedia(r, b.Logo, org) {
		fail(w, 400, "Sprawdź dane lokalizacji i logo")
		return
	}
	var e error
	if id == "" {
		e = a.DB.QueryRow(r.Context(), `INSERT INTO locations(organization_id,name,city,address,logo,point,hidden) VALUES($1,$2,$3,$4,$5,ST_SetSRID(ST_MakePoint($6,$7),4326)::geography,$8) RETURNING id`, org, b.Name, b.City, b.Address, b.Logo, b.Lng, b.Lat, b.Hidden).Scan(&id)
	} else {
		_, e = a.DB.Exec(r.Context(), `UPDATE locations SET name=$1,city=$2,address=$3,logo=$4,point=ST_SetSRID(ST_MakePoint($5,$6),4326)::geography,hidden=$7 WHERE id=$8`, b.Name, b.City, b.Address, b.Logo, b.Lng, b.Lat, b.Hidden, id)
	}
	idResult(w, id, e)
}
func validURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, e := url.Parse(raw)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.User == nil
}
func (a *App) saveTraining(w http.ResponseWriter, r *http.Request) {
	var b struct {
		LocationID  string   `json:"location_id"`
		Name        string   `json:"name"`
		Category    string   `json:"category"`
		Description string   `json:"description"`
		Price       *int     `json:"price"`
		Cards       []string `json:"cards"`
		Photos      []string `json:"photos"`
		SignupURL   string   `json:"signup_url"`
		Hidden      bool     `json:"hidden"`
	}
	if !decode(w, r, &b) {
		return
	}
	id := r.PathValue("id")
	org := a.orgFor(r.Context(), "location", b.LocationID)
	if !a.member(r.Context(), user(r).ID, org, false) || (id != "" && a.orgFor(r.Context(), "training", id) != org) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	if !required(b.Name, b.Category) || !validURL(b.SignupURL) || (b.Price != nil && *b.Price < 0) {
		fail(w, 400, "Sprawdź nazwę, cenę i link zapisu")
		return
	}
	for _, p := range b.Photos {
		if !a.ownedMedia(r, p, org) {
			fail(w, 400, "Nieprawidłowe zdjęcie")
			return
		}
	}
	var count int
	if e := a.DB.QueryRow(r.Context(), `SELECT count(*) FROM cards WHERE slug=ANY($1)`, b.Cards).Scan(&count); e != nil || count != len(b.Cards) {
		fail(w, 400, "Nieprawidłowe karty")
		return
	}
	if b.Cards == nil {
		b.Cards = []string{}
	}
	if b.Photos == nil {
		b.Photos = []string{}
	}
	var e error
	if id == "" {
		e = a.DB.QueryRow(r.Context(), `INSERT INTO trainings(location_id,name,category,description,price,cards,photos,signup_url,hidden) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, b.LocationID, b.Name, b.Category, b.Description, b.Price, b.Cards, b.Photos, b.SignupURL, b.Hidden).Scan(&id)
	} else {
		_, e = a.DB.Exec(r.Context(), `UPDATE trainings SET location_id=$1,name=$2,category=$3,description=$4,price=$5,cards=$6,photos=$7,signup_url=$8,hidden=$9 WHERE id=$10`, b.LocationID, b.Name, b.Category, b.Description, b.Price, b.Cards, b.Photos, b.SignupURL, b.Hidden, id)
	}
	idResult(w, id, e)
}
func (a *App) adminData(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	orgs, e := rowsJSON(r.Context(), a.DB, `SELECT to_jsonb(o) FROM organizations o ORDER BY created_at DESC`)
	if e != nil {
		dbfail(w, e)
		return
	}
	jobs, e := rowsJSON(r.Context(), a.DB, `SELECT jsonb_build_object('id',id,'kind',kind,'status',status,'attempts',attempts,'error',error,'created_at',created_at) FROM jobs WHERE status!='done' ORDER BY id DESC LIMIT 100`)
	if e != nil {
		dbfail(w, e)
		return
	}
	respond(w, 200, map[string]any{"organizations": orgs, "jobs": jobs})
}
func (a *App) moderate(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var b struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.Status != "approved" && b.Status != "rejected" && b.Status != "suspended" {
		fail(w, 400, "Nieprawidłowy status")
		return
	}
	if b.Status != "approved" && !required(b.Reason) {
		fail(w, 400, "Podaj uzasadnienie")
		return
	}
	id := r.PathValue("id")
	e := transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		tag, e := tx.Exec(r.Context(), `UPDATE organizations SET status=$1,reason=$2 WHERE id=$3`, b.Status, b.Reason, id)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return errInvalid
		}
		return audit(r.Context(), tx, user(r).ID, id, "moderate:"+b.Status)
	})
	idResult(w, id, e)
}
func (a *App) saveDictionary(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var b struct {
		Slug    string   `json:"slug"`
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
		Region  string   `json:"region"`
		Lat     float64  `json:"lat"`
		Lng     float64  `json:"lng"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !required(b.Slug, b.Name) || strings.ContainsAny(b.Slug, "/?# ") {
		fail(w, 400, "Nieprawidłowa nazwa lub slug")
		return
	}
	if b.Aliases == nil {
		b.Aliases = []string{}
	}
	var e error
	switch r.PathValue("kind") {
	case "categories":
		_, e = a.DB.Exec(r.Context(), `INSERT INTO categories VALUES($1,$2,$3) ON CONFLICT(slug) DO UPDATE SET name=$2,aliases=$3`, b.Slug, b.Name, b.Aliases)
	case "cards":
		_, e = a.DB.Exec(r.Context(), `INSERT INTO cards VALUES($1,$2) ON CONFLICT(slug) DO UPDATE SET name=$2`, b.Slug, b.Name)
	case "cities":
		if math.Abs(b.Lat) > 90 || math.Abs(b.Lng) > 180 {
			fail(w, 400, "Błędne współrzędne")
			return
		}
		_, e = a.DB.Exec(r.Context(), `INSERT INTO cities VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(slug) DO UPDATE SET name=$2,region=$3,aliases=$4,lat=$5,lng=$6`, b.Slug, b.Name, b.Region, b.Aliases, b.Lat, b.Lng)
	default:
		fail(w, 404, "Nieznany słownik")
		return
	}
	idResult(w, b.Slug, e)
}
func (a *App) retryJob(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	e := transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		var kind string
		var payload []byte
		if e := tx.QueryRow(r.Context(), `UPDATE jobs SET status='pending',attempts=0,run_at=now(),error='' WHERE id=$1 AND status='failed' RETURNING kind,payload`, r.PathValue("id")).Scan(&kind, &payload); e != nil {
			return e
		}
		if kind == "image" {
			var p map[string]string
			if e := json.Unmarshal(payload, &p); e != nil {
				return e
			}
			_, e := tx.Exec(r.Context(), `UPDATE media SET status='pending',error='' WHERE id=$1`, p["id"])
			return e
		}
		return nil
	})
	idResult(w, r.PathValue("id"), e)
}
