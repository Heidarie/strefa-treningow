package schedule

import "time"

// Resolve chooses the earliest instant in a fold and rejects a DST gap.
func Resolve(date, clock string) (time.Time, bool) {
	loc, _ := time.LoadLocation("Europe/Warsaw")
	naive, err := time.Parse("2006-01-02 15:04", date+" "+clock)
	if err != nil {
		return time.Time{}, false
	}
	var first time.Time
	for offset := -14 * 60; offset <= 14*60; offset += 15 {
		candidate := naive.Add(-time.Duration(offset) * time.Minute)
		if candidate.In(loc).Format("2006-01-02 15:04") == date+" "+clock && (first.IsZero() || candidate.Before(first)) {
			first = candidate
		}
	}
	return first, !first.IsZero()
}
