package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestURLValidation(t *testing.T) {
	for _, s := range []string{"https://example.com/signup", "http://example.com", ""} {
		if !validURL(s) {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"javascript:alert(1)", "//evil.com", "https://u:p@example.com", "data:text/html,x"} {
		if validURL(s) {
			t.Fatal(s)
		}
	}
}
func TestIntegrationIsolationAndSchedule(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostGIS integration tests")
	}
	t.Setenv("DATABASE_URL", url)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("MIGRATION_FILE", "../../migrations/001_init.sql")
	ctx := context.Background()
	a, e := New(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if e = a.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	// All fixtures live under dedicated IDs and are removed after this test.
	var uid, other, org, lid, tid string
	pass, _ := bcrypt.GenerateFromPassword([]byte("integration-password"), bcrypt.MinCost)
	for i, dest := range []*string{&uid, &other} {
		if e = a.DB.QueryRow(ctx, `INSERT INTO users(email,password,verified) VALUES($1,$2,true) RETURNING id`, token()+"@integration.test", string(pass)).Scan(dest); e != nil {
			t.Fatal(i, e)
		}
	}
	defer a.DB.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::uuid[])`, []string{uid, other})
	if e = a.DB.QueryRow(ctx, `INSERT INTO organizations(name,status) VALUES('Integration fixture','approved') RETURNING id`).Scan(&org); e != nil {
		t.Fatal(e)
	}
	defer a.DB.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, org)
	a.DB.Exec(ctx, `INSERT INTO memberships VALUES($1,$2,'owner')`, org, uid)
	raw, rawOther, csrf := token(), token(), token()
	defer a.DB.Exec(ctx, `DELETE FROM sessions WHERE user_id=ANY($1::uuid[])`, []string{uid, other})
	for id, tok := range map[string]string{uid: raw, other: rawOther} {
		if _, e = a.DB.Exec(ctx, `INSERT INTO sessions VALUES($1,$2,$3,now()+interval '1 hour')`, hash(tok), id, csrf); e != nil {
			t.Fatal(e)
		}
	}
	handler := a.Handler()
	request := func(method, path, tok string, body any, withCSRF bool) (int, []byte) {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if tok != "" {
			r.AddCookie(&http.Cookie{Name: "strefa_session", Value: tok})
		}
		if withCSRF {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code, w.Body.Bytes()
	}
	location := map[string]any{"organization_id": org, "name": "Integration place", "city": "konin", "address": "Testowa 1", "logo": "", "lat": 52.22, "lng": 18.25, "hidden": false}
	if code, _ := request("POST", "/api/v1/locations", raw, location, false); code != 403 {
		t.Fatalf("CSRF: %d", code)
	}
	if code, _ := request("POST", "/api/v1/locations", rawOther, location, true); code != 403 {
		t.Fatalf("isolation: %d", code)
	}
	code, b := request("POST", "/api/v1/locations", raw, location, true)
	if code != 200 {
		t.Fatalf("location %d %s", code, b)
	}
	var id map[string]string
	json.Unmarshal(b, &id)
	lid = id["id"]
	training := map[string]any{"location_id": lid, "name": "Integration boxing", "category": "boks", "description": "test", "price": nil, "cards": []string{"multisport"}, "photos": []string{}, "signup_url": "https://example.com/signup", "hidden": false}
	code, b = request("POST", "/api/v1/trainings", raw, training, true)
	if code != 200 {
		t.Fatalf("training %d %s", code, b)
	}
	json.Unmarshal(b, &id)
	tid = id["id"]
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	sched := map[string]any{"training_id": tid, "start_date": tomorrow, "end_date": "", "weekdays": []int{0, 1, 2, 3, 4, 5, 6}, "local_time": "18:00", "duration": 60, "hidden": false, "one_off": false}
	code, b = request("POST", "/api/v1/schedules", raw, sched, true)
	if code != 200 {
		t.Fatalf("schedule %d %s", code, b)
	}
	json.Unmarshal(b, &id)
	sid := id["id"]
	var before, after int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM occurrences WHERE series_id=$1`, sid).Scan(&before)
	if e = transact(ctx, a.DB, func(tx pgx.Tx) error { return materialize(ctx, tx, sid) }); e != nil {
		t.Fatal(e)
	}
	a.DB.QueryRow(ctx, `SELECT count(*) FROM occurrences WHERE series_id=$1`, sid).Scan(&after)
	if before < 80 || before != after {
		t.Fatalf("materialization duplicates: %d %d", before, after)
	}
	for _, path := range []string{"/api/v1/search?city=konin&category=boks&card=multisport", "/api/v1/map?city=konin&category=boks&card=multisport&bbox=18,52,19,53&zoom=12", "/api/v1/trainings/" + tid, "/api/v1/panel"} {
		code, b = request("GET", path, raw, nil, false)
		if code != 200 {
			t.Fatalf("%s: %d %s", path, code, b)
		}
	}
	var oid string
	a.DB.QueryRow(ctx, `SELECT id FROM occurrences WHERE series_id=$1 ORDER BY starts_at LIMIT 1`, sid).Scan(&oid)
	code, b = request("PUT", "/api/v1/occurrences/"+oid, raw, map[string]any{"date": tomorrow, "local_time": "19:30", "duration": 45, "hidden": true}, true)
	if code != 200 {
		t.Fatalf("exception %s", b)
	}
	sched["local_time"] = "17:00"
	code, b = request("PUT", "/api/v1/schedules/"+sid, raw, sched, true)
	if code != 200 {
		t.Fatalf("edit %s", b)
	}
	var overridden, hidden bool
	if e = a.DB.QueryRow(ctx, `SELECT overridden,hidden FROM occurrences WHERE id=$1`, oid).Scan(&overridden, &hidden); e != nil || !overridden || !hidden {
		t.Fatalf("exception lost: %v", e)
	}
	request("GET", "/api/v1/search?city=konin&category=boks", "", nil, false)
	request("GET", "/api/v1/map?city=konin", "", nil, false)
	training["hidden"] = true
	code, b = request("PUT", "/api/v1/trainings/"+tid, raw, training, true)
	if code != 200 {
		t.Fatal(string(b))
	}
	if code, _ = request("GET", "/api/v1/trainings/"+tid, "", nil, false); code != 404 {
		t.Fatal("hidden training exposed")
	}
	for _, path := range []string{"/api/v1/search?city=konin&category=boks", "/api/v1/map?city=konin"} {
		code, body := request("GET", path, "", nil, false)
		if code != 200 || bytes.Contains(body, []byte(lid)) {
			t.Fatalf("cache leaked hidden location: %s %s", path, body)
		}
	}
	training["hidden"] = false
	request("PUT", "/api/v1/trainings/"+tid, raw, training, true)
	a.DB.Exec(ctx, `UPDATE organizations SET status='suspended' WHERE id=$1`, org)
	if code, _ = request("GET", "/api/v1/trainings/"+tid, "", nil, false); code != 404 {
		t.Fatal("suspended organization exposed")
	}
	if code, _ = request("GET", "/api/v1/admin", raw, nil, false); code != 403 {
		t.Fatal("non-admin admitted")
	}
}
