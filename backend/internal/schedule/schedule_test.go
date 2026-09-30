package schedule

import (
	"testing"
	"time"
)

func TestDST(t *testing.T) {
	if _, ok := Resolve("2026-03-29", "02:30"); ok {
		t.Fatal("gap accepted")
	}
	got, ok := Resolve("2026-10-25", "02:30")
	if !ok || got.Format(time.RFC3339) != "2026-10-25T00:30:00Z" {
		t.Fatalf("fold: %v", got)
	}
	got, ok = Resolve("2026-09-29", "18:00")
	if !ok || got.Format(time.RFC3339) != "2026-09-29T16:00:00Z" {
		t.Fatalf("ordinary: %v", got)
	}
	if _, ok = Resolve("invalid", "18:00"); ok {
		t.Fatal("invalid date")
	}
}
