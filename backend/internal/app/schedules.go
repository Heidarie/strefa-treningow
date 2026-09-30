package app

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strefa/internal/schedule"
	"time"
)

type scheduleInput struct {
	TrainingID string `json:"training_id"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Weekdays   []int  `json:"weekdays"`
	LocalTime  string `json:"local_time"`
	Duration   int    `json:"duration"`
	Hidden     bool   `json:"hidden"`
	OneOff     bool   `json:"one_off"`
}

func (a *App) saveSchedule(w http.ResponseWriter, r *http.Request) {
	var b scheduleInput
	if !decode(w, r, &b) {
		return
	}
	id := r.PathValue("id")
	org := a.orgFor(r.Context(), "training", b.TrainingID)
	if !a.member(r.Context(), user(r).ID, org, false) || (id != "" && a.orgFor(r.Context(), "series", id) != org) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	start, e := time.Parse("2006-01-02", b.StartDate)
	_, ce := time.Parse("15:04", b.LocalTime)
	if e != nil || ce != nil || b.Duration < 5 || b.Duration > 1440 {
		fail(w, 400, "Sprawdź datę, godzinę i czas trwania")
		return
	}
	var end any
	if b.EndDate != "" {
		ed, e := time.Parse("2006-01-02", b.EndDate)
		if e != nil || ed.Before(start) {
			fail(w, 400, "Nieprawidłowy koniec serii")
			return
		}
		end = b.EndDate
	}
	if !b.OneOff && len(b.Weekdays) == 0 {
		fail(w, 400, "Wybierz dni tygodnia")
		return
	}
	seen := map[int]bool{}
	for _, d := range b.Weekdays {
		if d < 0 || d > 6 || seen[d] {
			fail(w, 400, "Nieprawidłowe dni tygodnia")
			return
		}
		seen[d] = true
	}
	if id != "" && b.OneOff {
		fail(w, 400, "Nie można zmienić serii w pojedynczy termin")
		return
	}
	e = transact(r.Context(), a.DB, func(tx pgx.Tx) error {
		if b.OneOff {
			at, ok := schedule.Resolve(b.StartDate, b.LocalTime)
			if !ok {
				return errInvalid
			}
			return tx.QueryRow(r.Context(), `INSERT INTO occurrences(training_id,local_date,starts_at,ends_at,hidden) VALUES($1,$2,$3,$4,$5) RETURNING id`, b.TrainingID, b.StartDate, at, at.Add(time.Duration(b.Duration)*time.Minute), b.Hidden).Scan(&id)
		}
		if id == "" {
			if e := tx.QueryRow(r.Context(), `INSERT INTO series(training_id,start_date,end_date,weekdays,local_time,duration,hidden) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, b.TrainingID, b.StartDate, end, b.Weekdays, b.LocalTime, b.Duration, b.Hidden).Scan(&id); e != nil {
				return e
			}
		} else {
			// A row lock serializes edits with the daily materializer.
			if _, e := tx.Exec(r.Context(), `SELECT id FROM series WHERE id=$1 FOR UPDATE`, id); e != nil {
				return e
			}
			if _, e := tx.Exec(r.Context(), `UPDATE series SET start_date=$1,end_date=$2,weekdays=$3,local_time=$4,duration=$5,hidden=$6 WHERE id=$7`, b.StartDate, end, b.Weekdays, b.LocalTime, b.Duration, b.Hidden, id); e != nil {
				return e
			}
			// Keep history and explicit exceptions. Series hiding takes precedence over them.
			if _, e := tx.Exec(r.Context(), `DELETE FROM occurrences WHERE series_id=$1 AND starts_at>=now() AND NOT overridden`, id); e != nil {
				return e
			}

		}
		return materialize(r.Context(), tx, id)
	})
	idResult(w, id, e)
}
func materialize(ctx context.Context, tx pgx.Tx, id string) error {
	var training, clock string
	var start time.Time
	var end *time.Time
	var days []int
	var duration int
	var hidden bool
	if e := tx.QueryRow(ctx, `SELECT training_id,start_date,end_date,weekdays,local_time,duration,hidden FROM series WHERE id=$1 FOR UPDATE`, id).Scan(&training, &start, &end, &days, &clock, &duration, &hidden); e != nil {
		return e
	}
	loc, _ := time.LoadLocation("Europe/Warsaw")
	today := time.Now().In(loc)
	from, _ := time.Parse("2006-01-02", today.Format("2006-01-02"))
	until := from.AddDate(0, 0, 90)
	if start.After(from) {
		from = start
	}
	if end != nil && end.Before(until) {
		until = *end
	}
	if _, e := tx.Exec(ctx, `DELETE FROM schedule_warnings WHERE series_id=$1 AND local_date>=CURRENT_DATE`, id); e != nil {
		return e
	}
	for d := from; !d.After(until); d = d.AddDate(0, 0, 1) {
		match := false
		for _, day := range days {
			if int(d.Weekday()) == day {
				match = true
			}
		}
		if !match {
			continue
		}
		date := d.Format("2006-01-02")
		at, ok := schedule.Resolve(date, clock)
		if !ok {
			if _, e := tx.Exec(ctx, `INSERT INTO schedule_warnings VALUES($1,$2,'Termin pominięty: godzina nie istnieje podczas zmiany czasu') ON CONFLICT DO NOTHING`, id, date); e != nil {
				return e
			}
			continue
		}
		if at.Before(time.Now()) {
			continue
		}
		if _, e := tx.Exec(ctx, `INSERT INTO occurrences(training_id,series_id,local_date,starts_at,ends_at,hidden) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(series_id,local_date) DO NOTHING`, training, id, date, at, at.Add(time.Duration(duration)*time.Minute), hidden); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) updateOccurrence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !a.member(r.Context(), user(r).ID, a.orgFor(r.Context(), "occurrence", id), false) {
		fail(w, 403, "Brak uprawnień")
		return
	}
	var b struct {
		Date      string `json:"date"`
		LocalTime string `json:"local_time"`
		Duration  int    `json:"duration"`
		Hidden    bool   `json:"hidden"`
	}
	if !decode(w, r, &b) {
		return
	}
	at, ok := schedule.Resolve(b.Date, b.LocalTime)
	if !ok || b.Duration < 5 || b.Duration > 1440 {
		fail(w, 400, "Nieprawidłowy termin")
		return
	}
	tag, e := a.DB.Exec(r.Context(), `UPDATE occurrences SET starts_at=$1,ends_at=$2,hidden=$3,overridden=true WHERE id=$4 AND starts_at>=now()`, at, at.Add(time.Duration(b.Duration)*time.Minute), b.Hidden, id)
	if e == nil && tag.RowsAffected() == 0 {
		fail(w, 409, "Nie można zmieniać historii")
		return
	}
	idResult(w, id, e)
}
