package app

import (
	"bytes"
	"net/http"
)

type capturedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (c *capturedResponse) Header() http.Header         { return c.header }
func (c *capturedResponse) WriteHeader(code int)        { c.status = code }
func (c *capturedResponse) Write(b []byte) (int, error) { return c.body.Write(b) }
func (a *App) cached(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if r.Method != "GET" || (path != "/api/v1/search" && path != "/api/v1/map" && path != "/api/v1/dictionaries" && path != "/api/v1/sitemap") || r.URL.Query().Has("lat") || r.URL.Query().Has("lng") {
			next.ServeHTTP(w, r)
			return
		}
		var version int64
		if e := a.DB.QueryRow(r.Context(), `SELECT version FROM public_revision WHERE singleton`).Scan(&version); e != nil {
			next.ServeHTTP(w, r)
			return
		}
		key := path + "?" + r.URL.Query().Encode()
		if b, ok := a.cache.Get(version, key); ok {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("X-Cache", "HIT")
			w.Write(b)
			return
		}
		c := &capturedResponse{header: make(http.Header), status: 200}
		next.ServeHTTP(c, r)
		if c.status == 200 {
			a.cache.Set(version, key, c.body.Bytes())
		}
		for k, v := range c.header {
			w.Header()[k] = v
		}
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(c.status)
		w.Write(c.body.Bytes())
	})
}
