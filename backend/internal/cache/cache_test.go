package cache

import "testing"

func TestRevisionEvictsHiddenContent(t *testing.T) {
	var s Store
	s.Get(1, "map")
	s.Set(1, "map", []byte("published"))
	if _, ok := s.Get(1, "map"); !ok {
		t.Fatal("missing cache")
	}
	if _, ok := s.Get(2, "map"); ok {
		t.Fatal("stale publication")
	}
	s.Set(1, "map", []byte("old in-flight response"))
	if _, ok := s.Get(2, "map"); ok {
		t.Fatal("old revision overwrote new")
	}
}
