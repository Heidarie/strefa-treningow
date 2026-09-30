package app

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscredentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"time"
)

func storage() *s3.Client {
	return s3.New(s3.Options{Region: "us-east-1", Credentials: awscredentials.NewStaticCredentialsProvider(env("S3_ACCESS_KEY", "strefa"), env("S3_SECRET_KEY", "local-development-only"), ""), BaseEndpoint: aws.String(env("S3_ENDPOINT", "http://localhost:8333")), UsePathStyle: true})
}
func putObject(ctx context.Context, key string, b []byte, mime string) error {
	_, e := storage().PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(env("S3_BUCKET", "strefa")), Key: aws.String(key), Body: bytes.NewReader(b), ContentType: aws.String(mime)})
	return e
}
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if e := r.ParseMultipartForm(8 << 20); e != nil {
		fail(w, 400, "Maksymalny rozmiar zdjęcia to 8 MB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	org := r.FormValue("organization_id")
	if !a.member(r.Context(), user(r).ID, org, false) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	f, _, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, "Brak pliku")
		return
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if e != nil || len(b) > 8<<20 {
		fail(w, 400, "Plik jest zbyt duży")
		return
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "jpeg" && format != "png") || cfg.Width*cfg.Height > 25000000 {
		fail(w, 400, "Wybierz JPEG lub PNG do 25 megapikseli")
		return
	}
	id := uuid.NewString()
	if e = putObject(r.Context(), "raw/"+id, b, "application/octet-stream"); e != nil {
		fail(w, 503, "Magazyn zdjęć niedostępny")
		return
	}
	e = transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		if _, e := tx.Exec(r.Context(), `INSERT INTO media(id,organization_id) VALUES($1,$2)`, id, org); e != nil {
			return e
		}
		return queue(r.Context(), tx, "image", map[string]string{"id": id}, "image:"+id)
	})
	if e != nil {
		_, _ = storage().DeleteObject(r.Context(), &s3.DeleteObjectInput{Bucket: aws.String(env("S3_BUCKET", "strefa")), Key: aws.String("raw/" + id)})
	}
	idResult(w, id, e)
}
func (a *App) geocode(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 3 || len(q) > 200 {
		fail(w, 400, "Podaj adres (3–200 znaków)")
		return
	}
	key := env("MAPTILER_KEY", "")
	if key == "" {
		fail(w, 503, "Brak klucza MapTiler. Wpisz współrzędne ręcznie.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.maptiler.com/geocoding/"+url.PathEscape(q)+".json?country=pl&language=pl&limit=5&key="+url.QueryEscape(key), nil)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		fail(w, 503, "Geokodowanie niedostępne")
		return
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fail(w, 503, "Geokodowanie niedostępne")
		return
	}
	var v any
	if e = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&v); e != nil {
		fail(w, 503, "Nieprawidłowa odpowiedź map")
		return
	}
	respond(w, 200, v)
}
