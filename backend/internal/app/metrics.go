package app

import (
	"fmt"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

var requests, serverErrors atomic.Uint64
var durationNS atomic.Uint64
var buckets = [8]atomic.Uint64{}
var limits = []float64{.01, .025, .05, .1, .25, .5, 1, 5}

type responseStatus struct {
	http.ResponseWriter
	status int
}

func (w *responseStatus) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (a *App) measure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rw := &responseStatus{w, 200}
		next.ServeHTTP(rw, r)
		d := time.Since(start)
		requests.Add(1)
		durationNS.Add(uint64(d))
		if rw.status >= 500 {
			serverErrors.Add(1)
		}
		for i, b := range limits {
			if d.Seconds() <= b {
				buckets[i].Add(1)
			}
		}
		slog.Info("http request", "method", r.Method, "status", rw.status, "duration_ms", d.Milliseconds(), "trace_id", trace.SpanFromContext(r.Context()).SpanContext().TraceID().String())
	})
}
func (a *App) metrics(w http.ResponseWriter, r *http.Request) {
	var failed int
	var oldest float64
	e := a.DB.QueryRow(r.Context(), `SELECT count(*) FILTER(WHERE status='failed'),coalesce(extract(epoch FROM now()-min(created_at) FILTER(WHERE status IN ('pending','running'))),0) FROM jobs`).Scan(&failed, &oldest)
	if e != nil {
		http.Error(w, "database unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE strefa_http_requests_total counter\nstrefa_http_requests_total %d\n# TYPE strefa_http_errors_total counter\nstrefa_http_errors_total %d\n# TYPE strefa_http_duration_seconds histogram\n", requests.Load(), serverErrors.Load())
	for i, b := range limits {
		fmt.Fprintf(w, "strefa_http_duration_seconds_bucket{le=\"%g\"} %d\n", b, buckets[i].Load())
	}
	fmt.Fprintf(w, "strefa_http_duration_seconds_bucket{le=\"+Inf\"} %d\nstrefa_http_duration_seconds_count %d\nstrefa_http_duration_seconds_sum %g\nstrefa_jobs_failed %d\nstrefa_jobs_oldest_seconds %g\n", requests.Load(), requests.Load(), float64(durationNS.Load())/1e9, failed, oldest)
}
