package app

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/http"
	"regexp"
)

var imageName = regexp.MustCompile(`^[0-9a-f-]{36}(-thumb)?\.jpg$`)

func (a *App) publicMedia(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !imageName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	var org string
	var published bool
	e := a.DB.QueryRow(r.Context(), `SELECT m.organization_id,EXISTS(SELECT 1 FROM organizations o JOIN locations l ON l.organization_id=o.id LEFT JOIN trainings t ON t.location_id=l.id AND NOT t.hidden WHERE o.id=m.organization_id AND o.status='approved' AND NOT l.hidden AND (m.url=l.logo OR (l.logo='' AND m.url=o.logo) OR m.url=ANY(t.photos))) FROM media m WHERE m.id=$1 AND m.status='ready'`, name[:36]).Scan(&org, &published)
	u := user(r)
	if e != nil || (!published && !u.Admin && !a.member(r.Context(), u.ID, org, false)) {
		http.NotFound(w, r)
		return
	}
	obj, e := storage().GetObject(r.Context(), &s3.GetObjectInput{Bucket: aws.String(env("S3_BUCKET", "strefa")), Key: aws.String("public/" + name)})
	if e != nil {
		http.NotFound(w, r)
		return
	}
	defer obj.Body.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private,no-store")
	_, _ = io.Copy(w, obj.Body)
}
func (a *App) adminPreview(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	out := map[string]any{}
	queries := map[string]string{"organizations": `SELECT to_jsonb(o) FROM organizations o WHERE id=$1`, "locations": `SELECT to_jsonb(l)-'point' FROM locations l WHERE organization_id=$1`, "trainings": `SELECT to_jsonb(t) FROM trainings t JOIN locations l ON l.id=t.location_id WHERE l.organization_id=$1`, "series": `SELECT to_jsonb(s) FROM series s JOIN trainings t ON t.id=s.training_id JOIN locations l ON l.id=t.location_id WHERE l.organization_id=$1`}
	for k, q := range queries {
		v, e := rowsJSON(r.Context(), a.DB, q, id)
		if e != nil {
			dbfail(w, e)
			return
		}
		out[k] = v
	}
	respond(w, 200, out)
}
