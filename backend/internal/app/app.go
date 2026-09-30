package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strefa/internal/cache"
	"strings"
	"sync"
	"time"
)

type App struct {
	cache    cache.Store
	DB       *pgxpool.Pool
	mu       sync.Mutex
	limits   map[string]*limit
	shutdown func(context.Context) error
}
type limit struct {
	n     int
	until time.Time
}
type identity struct {
	ID, Email, CSRF string
	Admin, Verified bool
}
type key int

const identityKey key = 1

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func New(ctx context.Context) (*App, error) {
	config, err := pgxpool.ParseConfig(env("DATABASE_URL", "postgres://strefa:strefa@localhost:5432/strefa?sslmode=disable"))
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = dbTracer{}
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	a := &App{DB: db, limits: map[string]*limit{}, shutdown: func(context.Context) error { return nil }}
	var logOutput io.Writer = os.Stdout
	if path := os.Getenv("LOG_FILE"); path != "" {
		f, e := newRotatingLog(path)
		if e != nil {
			return nil, e
		}
		logOutput = io.MultiWriter(os.Stdout, f)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(logOutput, nil)).With("service.name", env("OTEL_SERVICE_NAME", "strefa-api")))
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exp, e := otlptracehttp.New(ctx)
		if e != nil {
			return nil, e
		}
		p := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(resource.NewWithAttributes("", attribute.String("service.name", env("OTEL_SERVICE_NAME", "strefa-api")))))
		otel.SetTracerProvider(p)
		otel.SetTextMapPropagator(propagation.TraceContext{})
		a.shutdown = p.Shutdown
	}
	return a, nil
}
func (a *App) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = a.shutdown(ctx)
	a.DB.Close()
}
func (a *App) Migrate(ctx context.Context) error {
	data, err := os.ReadFile(env("MIGRATION_FILE", "migrations/001_init.sql"))
	if err != nil {
		return err
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(492764)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, string(data)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", a.metrics)
	mux.HandleFunc("GET /public-media/{name}", a.publicMedia)
	mux.HandleFunc("GET /api/v1/admin/organizations/{id}", a.protected(a.adminPreview))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.DB.Ping(r.Context()); err != nil {
			fail(w, 503, "Baza niedostępna")
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/dictionaries", a.dictionaries)
	mux.HandleFunc("GET /api/v1/search", a.search)
	mux.HandleFunc("GET /api/v1/map", a.mapSearch)
	mux.HandleFunc("GET /api/v1/trainings/{id}", a.trainingDetail)
	mux.HandleFunc("GET /api/v1/locations/{id}", a.locationDetail)
	mux.HandleFunc("GET /api/v1/sitemap", a.sitemap)
	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/reset-request", a.resetRequest)
	mux.HandleFunc("POST /api/v1/auth/token", a.consumeToken)
	mux.HandleFunc("GET /api/v1/auth/me", a.me)
	mux.HandleFunc("POST /api/v1/auth/logout", a.protected(a.logout))
	mux.HandleFunc("GET /api/v1/panel", a.protected(a.panel))
	mux.HandleFunc("POST /api/v1/organizations", a.protected(a.createOrganization))
	mux.HandleFunc("PUT /api/v1/organizations/{id}", a.protected(a.updateOrganization))
	mux.HandleFunc("POST /api/v1/organizations/{id}/submit", a.protected(a.submitOrganization))
	mux.HandleFunc("POST /api/v1/organizations/{id}/invite", a.protected(a.invite))
	mux.HandleFunc("DELETE /api/v1/organizations/{id}/members/{user}", a.protected(a.removeMember))
	mux.HandleFunc("POST /api/v1/locations", a.protected(a.saveLocation))
	mux.HandleFunc("PUT /api/v1/locations/{id}", a.protected(a.saveLocation))
	mux.HandleFunc("POST /api/v1/trainings", a.protected(a.saveTraining))
	mux.HandleFunc("PUT /api/v1/trainings/{id}", a.protected(a.saveTraining))
	mux.HandleFunc("POST /api/v1/schedules", a.protected(a.saveSchedule))
	mux.HandleFunc("PUT /api/v1/schedules/{id}", a.protected(a.saveSchedule))
	mux.HandleFunc("PUT /api/v1/occurrences/{id}", a.protected(a.updateOccurrence))
	mux.HandleFunc("POST /api/v1/media", a.protected(a.upload))
	mux.HandleFunc("GET /api/v1/geocode", a.protected(a.geocode))
	mux.HandleFunc("GET /api/v1/admin", a.protected(a.adminData))
	mux.HandleFunc("PUT /api/v1/admin/organizations/{id}", a.protected(a.moderate))
	mux.HandleFunc("PUT /api/v1/admin/dictionaries/{kind}", a.protected(a.saveDictionary))
	mux.HandleFunc("POST /api/v1/admin/jobs/{id}/retry", a.protected(a.retryJob))
	return otelhttp.NewHandler(a.measure(a.middleware(a.cached(mux))), "http", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string { return r.Method + " request" }))
}
func (a *App) Serve(ctx context.Context) error {
	srv := &http.Server{Addr: ":8080", Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("API listening", "port", 8080)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth") || strings.HasPrefix(r.URL.Path, "/api/v1/panel") || strings.HasPrefix(r.URL.Path, "/api/v1/admin") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			origin := r.Header.Get("Origin")
			if origin != "" && origin != env("PUBLIC_URL", "http://localhost:3000") {
				fail(w, 403, "Niedozwolone pochodzenie żądania")
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") && r.Method == "POST" {
			// Caddy overwrites X-Real-IP; never expose this port in production.
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if os.Getenv("TRUST_PROXY") == "true" && r.Header.Get("X-Real-IP") != "" {
				ip = r.Header.Get("X-Real-IP")
			}
			if !a.allow(ip) {
				fail(w, 429, "Zbyt wiele prób. Spróbuj za minutę.")
				return
			}
		}
		if c, err := r.Cookie("strefa_session"); err == nil {
			var u identity
			err = a.DB.QueryRow(r.Context(), `SELECT u.id,u.email,u.admin,u.verified,s.csrf FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=$1 AND s.expires_at>now()`, hash(c.Value)).Scan(&u.ID, &u.Email, &u.Admin, &u.Verified, &u.CSRF)
			if err == nil {
				r = r.WithContext(context.WithValue(r.Context(), identityKey, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) allow(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if len(a.limits) > 10000 {
		for k, v := range a.limits {
			if now.After(v.until) {
				delete(a.limits, k)
			}
		}
	}
	v := a.limits[ip]
	if v == nil || now.After(v.until) {
		a.limits[ip] = &limit{1, now.Add(time.Minute)}
		return true
	}
	v.n++
	return v.n <= 20
}
func (a *App) protected(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(identityKey).(identity)
		if !ok {
			fail(w, 401, "Zaloguj się")
			return
		}
		if r.Method != "GET" && r.Header.Get("X-CSRF-Token") != u.CSRF {
			fail(w, 403, "Nieprawidłowy token CSRF")
			return
		}
		h(w, r)
	}
}
func user(r *http.Request) identity { u, _ := r.Context().Value(identityKey).(identity); return u }
func (a *App) member(ctx context.Context, uid, org string, owner bool) bool {
	var ok bool
	err := a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memberships WHERE user_id=$1 AND organization_id=$2 AND (NOT $3 OR role='owner'))`, uid, org, owner).Scan(&ok)
	return err == nil && ok
}
func (a *App) orgFor(ctx context.Context, kind, id string) string {
	q := map[string]string{"location": `SELECT organization_id FROM locations WHERE id=$1`, "training": `SELECT l.organization_id FROM trainings t JOIN locations l ON l.id=t.location_id WHERE t.id=$1`, "series": `SELECT l.organization_id FROM series s JOIN trainings t ON t.id=s.training_id JOIN locations l ON l.id=t.location_id WHERE s.id=$1`, "occurrence": `SELECT l.organization_id FROM occurrences o JOIN trainings t ON t.id=o.training_id JOIN locations l ON l.id=t.location_id WHERE o.id=$1`}[kind]
	var org string
	_ = a.DB.QueryRow(ctx, q, id).Scan(&org)
	return org
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "Nieprawidłowe dane formularza")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}
func dbfail(w http.ResponseWriter, err error) {
	slog.Error("database operation failed", "error", err)
	fail(w, 400, "Nie udało się zapisać danych. Sprawdź wartości i powiązania.")
}
func token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func rowsJSON(ctx context.Context, db *pgxpool.Pool, q string, args ...any) ([]map[string]any, error) {
	ctx, span := otel.Tracer("strefa.database").Start(ctx, "query")
	defer span.End()
	rows, err := db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var v map[string]any
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (a *App) jsonRows(w http.ResponseWriter, r *http.Request, q string, args ...any) {
	v, err := rowsJSON(r.Context(), a.DB, q, args...)
	if err != nil {
		dbfail(w, err)
		return
	}
	respond(w, 200, v)
}
func queue(ctx context.Context, tx pgx.Tx, kind string, payload any, dedupe string) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO jobs(kind,payload,dedupe) VALUES($1,$2,$3) ON CONFLICT(dedupe) DO NOTHING`, kind, b, dedupe)
	return err
}
func transact(ctx context.Context, db *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var errInvalid = errors.New("invalid request")

func audit(ctx context.Context, tx pgx.Tx, uid, org, action string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_log(actor,organization_id,action) VALUES($1,$2,$3)`, uid, org, action)
	return err
}
func idResult(w http.ResponseWriter, id string, err error) {
	if err != nil {
		dbfail(w, err)
		return
	}
	respond(w, 200, map[string]string{"id": id})
}
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !user(r).Admin {
		fail(w, 403, "Tylko administrator")
		return false
	}
	return true
}
func required(values ...string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) == "" || len(v) > 10000 {
			return false
		}
	}
	return true
}
func fmtError(s string) error { return fmt.Errorf("%s", s) }
