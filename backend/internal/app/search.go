package app

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"net/http"
	"strconv"
	"strings"
)

const published = `o.status='approved' AND NOT l.hidden AND NOT t.hidden`
const trainingJSON = `jsonb_build_object('id',t.id,'name',t.name,'category',t.category,'description',t.description,'price',t.price,'cards',t.cards,'photos',t.photos,'signup_url',t.signup_url,'next_at',(SELECT min(starts_at) FROM occurrences occ WHERE occ.training_id=t.id AND NOT occ.hidden AND NOT EXISTS(SELECT 1 FROM series ss WHERE ss.id=occ.series_id AND ss.hidden) AND starts_at>=now()))`

func (a *App) dictionaries(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	for _, table := range []string{"categories", "cards", "cities"} {
		q := "SELECT to_jsonb(d) FROM " + table + " d ORDER BY name"
		v, err := rowsJSON(r.Context(), a.DB, q)
		if err != nil {
			dbfail(w, err)
			return
		}
		out[table] = v
	}
	respond(w, 200, out)
}

type filter struct {
	where string
	args  []any
}

func parseFilter(r *http.Request) (filter, error) {
	f := filter{where: published, args: []any{}}
	add := func(s string, v any) { f.args = append(f.args, v); f.where += " AND " + fmt.Sprintf(s, len(f.args)) }
	q := r.URL.Query()
	if city := q.Get("city"); city != "" {
		add("l.city=$%d", city)
	}
	if cat := q.Get("category"); cat != "" {
		add("t.category=$%d", cat)
	}
	if card := q.Get("card"); card != "" {
		add("$%d=ANY(t.cards)", card)
	}
	if day := q.Get("day"); day != "" {
		n, e := strconv.Atoi(day)
		if e != nil || n < 0 || n > 6 {
			return f, errInvalid
		}
		add(`EXISTS(SELECT 1 FROM occurrences oc WHERE oc.training_id=t.id AND NOT oc.hidden AND NOT EXISTS(SELECT 1 FROM series ss WHERE ss.id=oc.series_id AND ss.hidden) AND oc.starts_at>=now() AND extract(dow FROM oc.starts_at AT TIME ZONE 'Europe/Warsaw')=$%d)`, n)
	}
	if bounds := q.Get("bbox"); bounds != "" {
		parts := strings.Split(bounds, ",")
		if len(parts) != 4 {
			return f, errInvalid
		}
		nums := make([]float64, 4)
		for i, p := range parts {
			v, e := strconv.ParseFloat(p, 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				return f, errInvalid
			}
			nums[i] = v
		}
		if nums[0] < -180 || nums[2] > 180 || nums[1] < -90 || nums[3] > 90 || nums[0] >= nums[2] || nums[1] >= nums[3] {
			return f, errInvalid
		}
		start := len(f.args) + 1
		for _, n := range nums {
			f.args = append(f.args, n)
		}
		f.where += fmt.Sprintf(" AND l.point && ST_MakeEnvelope($%d,$%d,$%d,$%d,4326)::geography", start, start+1, start+2, start+3)
	}
	return f, nil
}
func (a *App) search(w http.ResponseWriter, r *http.Request) {
	f, e := parseFilter(r)
	if e != nil {
		fail(w, 400, "Nieprawidłowe filtry")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	distance := "NULL::double precision"
	order := "name,id"
	lat, lng := r.URL.Query().Get("lat"), r.URL.Query().Get("lng")
	if lat != "" || lng != "" {
		la, e1 := strconv.ParseFloat(lat, 64)
		lo, e2 := strconv.ParseFloat(lng, 64)
		if e1 != nil || e2 != nil || math.IsNaN(la) || math.IsNaN(lo) || math.Abs(la) > 90 || math.Abs(lo) > 180 {
			fail(w, 400, "Nieprawidłowe współrzędne")
			return
		}
		f.args = append(f.args, lo, la)
		distance = fmt.Sprintf("ST_Distance(l.point,ST_SetSRID(ST_MakePoint($%d,$%d),4326)::geography)", len(f.args)-1, len(f.args))
		order = "distance_m,name,id"
	}
	q := `WITH eligible AS (SELECT l.id,l.name,l.address,l.city,ST_Y(l.point::geometry) lat,ST_X(l.point::geometry) lng,coalesce(nullif(l.logo,''),o.logo) logo,o.name organization,` + distance + ` distance_m FROM locations l JOIN organizations o ON o.id=l.organization_id WHERE EXISTS (SELECT 1 FROM trainings t WHERE t.location_id=l.id AND ` + f.where + `)), page AS (SELECT *,count(*) OVER() total FROM eligible ORDER BY ` + order + fmt.Sprintf(" LIMIT 20 OFFSET %d", (page-1)*20) + `) SELECT to_jsonb(p)||jsonb_build_object('trainings',(SELECT jsonb_agg(` + trainingJSON + ` ORDER BY t.name) FROM trainings t JOIN locations l ON l.id=t.location_id JOIN organizations o ON o.id=l.organization_id WHERE t.location_id=p.id AND ` + f.where + `)) FROM page p`

	a.jsonRows(w, r, q, f.args...)
}
func (a *App) mapSearch(w http.ResponseWriter, r *http.Request) {
	f, e := parseFilter(r)
	if e != nil {
		fail(w, 400, "Nieprawidłowy obszar mapy")
		return
	}
	zoom, _ := strconv.Atoi(r.URL.Query().Get("zoom"))
	if zoom < 0 {
		zoom = 0
	}
	if zoom > 20 {
		zoom = 20
	}
	grid := 360 / math.Pow(2, float64(zoom+3))
	// Bound the grid to at most 41 x 41 cells; never silently drop locations.
	width, height := 360.0, 170.0
	if b := strings.Split(r.URL.Query().Get("bbox"), ","); len(b) == 4 {
		west, _ := strconv.ParseFloat(b[0], 64)
		south, _ := strconv.ParseFloat(b[1], 64)
		east, _ := strconv.ParseFloat(b[2], 64)
		north, _ := strconv.ParseFloat(b[3], 64)
		width = east - west
		height = north - south
	}
	grid = math.Max(grid, math.Max(width, height)/40)
	f.args = append(f.args, grid)
	n := len(f.args)
	q := `WITH pts AS (SELECT DISTINCT l.id,l.name,ST_X(l.point::geometry) lng,ST_Y(l.point::geometry) lat FROM locations l JOIN organizations o ON o.id=l.organization_id JOIN trainings t ON t.location_id=l.id WHERE ` + f.where + fmt.Sprintf(`), grouped AS(SELECT floor(lng/$%d) gx,floor(lat/$%d) gy,count(*) count,avg(lng) lng,avg(lat) lat,min(id::text) id,min(name) name FROM pts GROUP BY 1,2) SELECT jsonb_build_object('id',CASE WHEN count=1 THEN id ELSE NULL END,'name',CASE WHEN count=1 THEN name ELSE 'Lokalizacje' END,'count',count,'lng',lng,'lat',lat) FROM grouped ORDER BY count DESC LIMIT 2000`, n, n)
	a.jsonRows(w, r, q, f.args...)
}
func (a *App) trainingDetail(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		fail(w, 404, "Nie znaleziono strony")
		return
	}
	q := `SELECT ` + trainingJSON + ` || jsonb_build_object('location',jsonb_build_object('id',l.id,'name',l.name,'address',l.address,'city',l.city,'lat',ST_Y(l.point::geometry),'lng',ST_X(l.point::geometry),'logo',coalesce(nullif(l.logo,''),o.logo)),'organization',o.name,'occurrences',coalesce((SELECT jsonb_agg(to_jsonb(x)) FROM (SELECT id,starts_at,ends_at FROM occurrences WHERE training_id=t.id AND NOT hidden AND NOT EXISTS(SELECT 1 FROM series ss WHERE ss.id=occurrences.series_id AND ss.hidden) AND starts_at>=now() ORDER BY starts_at LIMIT 90)x),'[]'::jsonb)) FROM trainings t JOIN locations l ON l.id=t.location_id JOIN organizations o ON o.id=l.organization_id WHERE t.id=$1 AND ` + published
	v, e := rowsJSON(r.Context(), a.DB, q, r.PathValue("id"))
	if e != nil {
		fail(w, 503, "Dane są chwilowo niedostępne")
		return
	}
	if len(v) == 0 {
		fail(w, 404, "Nie znaleziono treningu")
		return
	}
	respond(w, 200, v[0])
}
func (a *App) locationDetail(w http.ResponseWriter, r *http.Request) {
	if _, err := uuid.Parse(r.PathValue("id")); err != nil {
		fail(w, 404, "Nie znaleziono strony")
		return
	}
	q := `SELECT jsonb_build_object('id',l.id,'name',l.name,'address',l.address,'city',l.city,'lat',ST_Y(l.point::geometry),'lng',ST_X(l.point::geometry),'logo',coalesce(nullif(l.logo,''),o.logo),'organization',o.name,'trainings',jsonb_agg(` + trainingJSON + ` ORDER BY t.name)) FROM locations l JOIN organizations o ON o.id=l.organization_id JOIN trainings t ON t.location_id=l.id WHERE l.id=$1 AND ` + published + ` GROUP BY l.id,o.name,o.logo`
	v, e := rowsJSON(r.Context(), a.DB, q, r.PathValue("id"))
	if e != nil {
		fail(w, 503, "Dane są chwilowo niedostępne")
		return
	}
	if len(v) == 0 {
		fail(w, 404, "Nie znaleziono lokalizacji")
		return
	}
	respond(w, 200, v[0])
}
func (a *App) sitemap(w http.ResponseWriter, r *http.Request) {
	a.jsonRows(w, r, `WITH live AS(SELECT t.id,l.id lid,l.city,t.category FROM trainings t JOIN locations l ON l.id=t.location_id JOIN organizations o ON o.id=l.organization_id WHERE `+published+`) SELECT jsonb_build_object('path',path) FROM (SELECT '/' path UNION SELECT '/'||city FROM live UNION SELECT '/'||city||'/'||category FROM live UNION SELECT '/trening/'||id FROM live UNION SELECT '/lokalizacja/'||lid FROM live)x`)
}
