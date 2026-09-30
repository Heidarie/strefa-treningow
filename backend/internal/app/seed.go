package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"os"
	"time"
)

func (a *App) Seed(ctx context.Context) error {
	email, password := os.Getenv("ADMIN_EMAIL"), os.Getenv("ADMIN_PASSWORD")
	if email == "" || len(password) < 10 {
		return fmt.Errorf("set ADMIN_EMAIL and ADMIN_PASSWORD (at least 10 characters)")
	}
	p, e := bcrypt.GenerateFromPassword([]byte(password), 12)
	if e != nil {
		return e
	}
	return transact(ctx, a.DB, func(tx pgx.Tx) error {
		var uid string
		if e := tx.QueryRow(ctx, `INSERT INTO users(email,password,verified,admin) VALUES($1,$2,true,true) ON CONFLICT(email) DO UPDATE SET admin=true RETURNING id`, email, string(p)).Scan(&uid); e != nil {
			return e
		}
		if os.Getenv("SEED_DEMO") != "true" {
			return nil
		}
		var count int
		if e := tx.QueryRow(ctx, `SELECT count(*) FROM organizations WHERE name='Studio Forma · DEMO'`).Scan(&count); e != nil {
			return e
		}
		if count > 0 {
			return nil
		}
		var org string
		if e := tx.QueryRow(ctx, `INSERT INTO organizations(name,status) VALUES('Studio Forma · DEMO','approved') RETURNING id`).Scan(&org); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO memberships VALUES($1,$2,'owner')`, org, uid); e != nil {
			return e
		}
		type demo struct {
			city, name, address, category, title string
			lat, lng                             float64
			price                                int
		}
		for _, d := range []demo{{"poznan", "Studio Forma · Jeżyce", "ul. Dąbrowskiego 35", "pilates", "Pilates — siła i równowaga", 52.412, 16.904, 4500}, {"poznan", "Studio Forma · Centrum", "ul. Święty Marcin 24", "joga", "Joga na dobry początek", 52.405, 16.925, 3500}, {"konin", "Studio Forma · Konin", "ul. Dworcowa 6", "boks", "Boks od podstaw", 52.225, 18.254, 4000}} {
			var lid, tid, sid string
			if e := tx.QueryRow(ctx, `INSERT INTO locations(organization_id,name,city,address,point) VALUES($1,$2,$3,$4,ST_SetSRID(ST_MakePoint($5,$6),4326)::geography) RETURNING id`, org, d.name, d.city, d.address, d.lng, d.lat).Scan(&lid); e != nil {
				return e
			}
			if e := tx.QueryRow(ctx, `INSERT INTO trainings(location_id,name,category,description,price,cards) VALUES($1,$2,$3,'Przykładowa oferta demonstracyjna. Zajęcia w małej grupie, prowadzone w przyjaznej atmosferze. Dane nie reprezentują rzeczywistego klubu.',$4,ARRAY['multisport']) RETURNING id`, lid, d.title, d.category, d.price).Scan(&tid); e != nil {
				return e
			}
			if e := tx.QueryRow(ctx, `INSERT INTO series(training_id,start_date,weekdays,local_time,duration) VALUES($1,$2,ARRAY[1,3,5],'18:00',60) RETURNING id`, tid, time.Now().Format("2006-01-02")).Scan(&sid); e != nil {
				return e
			}
			if e := materialize(ctx, tx, sid); e != nil {
				return e
			}
		}
		return nil
	})
}
