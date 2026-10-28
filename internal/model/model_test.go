package model

import "testing"

func TestShotsStats(t *testing.T) {
	shots := Shots{
		{Ringe: 10.4, Teiler: 132.4, X: 1.25, Y: -0.42},
		{Ringe: 9.8, Teiler: 300.1, X: 0.92, Y: -2.85},
		{Ringe: 10.7, Teiler: 80, X: -0.5, Y: 0.3},
	}
	if got := shots.Summe(); got != 30.9 {
		t.Errorf("Summe = %v", got)
	}
	if got := shots.SummeGanz(); got != 29 {
		t.Errorf("SummeGanz = %v", got)
	}
	if got := shots.Best().Teiler; got != 80 {
		t.Errorf("Best = %v", got)
	}
	if got := shots.CountRing(10); got != 2 {
		t.Errorf("CountRing(10) = %v", got)
	}
}

func TestSeries(t *testing.T) {
	shots := make(Shots, 25)
	series := shots.Series()
	if len(series) != 3 || len(series[2]) != 5 {
		t.Errorf("series lengths: %d, last %d", len(series), len(series[len(series)-1]))
	}
	if len(Shots{}.Series()) != 0 {
		t.Error("empty shots should have no series")
	}
}

func TestSortMatches(t *testing.T) {
	a := &Match{ID: "a", Shots: Shots{{Ringe: 9, Teiler: 100}}}
	b := &Match{ID: "b", Shots: Shots{{Ringe: 10, Teiler: 200}}}
	empty := &Match{ID: "e"}

	ms := []*Match{a, empty, b}
	SortMatches(ModusBest, ms)
	if ms[0].ID != "b" || ms[2].ID != "e" {
		t.Errorf("best: %s %s %s", ms[0].ID, ms[1].ID, ms[2].ID)
	}
	SortMatches(ModusBestTeiler, ms)
	if ms[0].ID != "a" || ms[2].ID != "e" {
		t.Errorf("teiler: %s %s %s", ms[0].ID, ms[1].ID, ms[2].ID)
	}
}
