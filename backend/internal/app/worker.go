package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"io"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strefa/internal/media"
	"time"
)

func (a *App) Worker(ctx context.Context) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			day := time.Now().UTC().Format("2006-01-02")
			if last != day {
				e := transact(ctx, a.DB, func(tx pgx.Tx) error { return queue(ctx, tx, "schedule", map[string]string{}, "schedule:"+day) })
				if e == nil {
					last = day
				}
			}
			if e := a.workOne(ctx); e != nil {
				slog.Error("worker tick failed", "error", e)
			}
		}
	}
}
func (a *App) workOne(ctx context.Context) error {
	var id int64
	var kind string
	var data []byte
	var attempts int
	err := transact(ctx, a.DB, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE jobs SET status=CASE WHEN attempts>=5 THEN 'failed' ELSE 'pending' END,run_at=now(),error='Worker lease expired' WHERE status='running' AND locked_at<now()-interval '10 minutes'`)
		if e != nil {
			return e
		}
		return tx.QueryRow(ctx, `UPDATE jobs SET status='running',locked_at=now(),attempts=attempts+1 WHERE id=(SELECT id FROM jobs WHERE status='pending' AND run_at<=now() ORDER BY run_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,kind,payload,attempts`).Scan(&id, &kind, &data, &attempts)
	})
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	ctx, span := otel.Tracer("strefa.worker").Start(ctx, "job."+kind)
	defer span.End()
	var p map[string]string
	if err = json.Unmarshal(data, &p); err == nil {
		switch kind {
		case "email":
			err = sendEmail(p)
		case "image":
			err = a.processImage(ctx, p["id"])
		case "schedule":
			err = a.materializeAll(ctx)
		default:
			err = fmt.Errorf("unknown job")
		}
	}
	if err != nil {
		span.RecordError(err)
		status := "pending"
		if attempts >= 5 {
			status = "failed"
			if kind == "image" {
				_, _ = a.DB.Exec(context.Background(), `UPDATE media SET status='failed',error='Przetwarzanie nie powiodło się. Administrator może ponowić zadanie.' WHERE id=$1`, p["id"])
			}
		}
		_, e := a.DB.Exec(context.Background(), `UPDATE jobs SET status=$1,error=$2,run_at=now()+($3 * interval '1 second') WHERE id=$4`, status, "Nie udało się wykonać zadania; sprawdź dostępność usługi", attempts*attempts*30, id)
		slog.Error("job failed", "id", id, "kind", kind)
		return e
	}
	_, err = a.DB.Exec(ctx, `UPDATE jobs SET status='done',error='' WHERE id=$1`, id)
	return err
}
func (a *App) materializeAll(ctx context.Context) error {
	rows, e := a.DB.Query(ctx, `SELECT id FROM series WHERE NOT hidden AND (end_date IS NULL OR end_date>=CURRENT_DATE)`)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = transact(ctx, a.DB, func(tx pgx.Tx) error { return materialize(ctx, tx, id) }); e != nil {
			return e
		}
	}
	_, e = a.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at<now(); DELETE FROM tokens WHERE expires_at<now()`)
	return e
}
func sendEmail(p map[string]string) error {
	host := env("SMTP_HOST", "mailpit")
	port := env("SMTP_PORT", "1025")
	from := env("SMTP_FROM", "noreply@strefa.local")
	var auth smtp.Auth
	if u := os.Getenv("SMTP_USER"); u != "" {
		auth = smtp.PlainAuth("", u, os.Getenv("SMTP_PASSWORD"), host)
	}
	conn, e := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if e != nil {
		return e
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	client, e := smtp.NewClient(conn, host)
	if e != nil {
		return e
	}
	defer client.Close()
	return sendSMTP(client, host, auth, from, p)
}
func (a *App) processImage(ctx context.Context, id string) error {
	var status string
	if e := a.DB.QueryRow(ctx, `SELECT status FROM media WHERE id=$1`, id).Scan(&status); e != nil {
		return e
	}
	if status == "ready" {
		return nil
	}
	res, e := storage().GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(env("S3_BUCKET", "strefa")), Key: aws.String("raw/" + id)})
	if e != nil {
		return e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 9<<20))
	if e != nil {
		return e
	}
	full, thumb, e := media.Process(b)
	if e != nil {
		_, _ = a.DB.Exec(ctx, `UPDATE media SET status='failed',error='Nieprawidłowe zdjęcie' WHERE id=$1`, id)
		return e
	}
	if e = putObject(ctx, "public/"+id+".jpg", full, "image/jpeg"); e != nil {
		return e
	}
	if e = putObject(ctx, "public/"+id+"-thumb.jpg", thumb, "image/jpeg"); e != nil {
		return e
	}
	_, e = a.DB.Exec(ctx, `UPDATE media SET status='ready',url=$1,thumbnail=$2,error='' WHERE id=$3`, "/media/"+id+".jpg", "/media/"+id+"-thumb.jpg", id)
	if e == nil {
		_, _ = storage().DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(env("S3_BUCKET", "strefa")), Key: aws.String("raw/" + id)})
	}
	return e
}
