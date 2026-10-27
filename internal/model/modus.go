package model

import (
	"cmp"
	"slices"
)

const (
	ModusBest        = "Bestes Ergebnis"
	ModusBestDecimal = "Bestes Ergebnis Zehntel"
	ModusBestTeiler  = "Bester Teiler"
	ModusLeague      = "Liga des RSB (Kreis/Bezirk/Landesliga)"
	ModusMostTens    = "Höchste Anzahl 10er"
)

// Modi lists the scoring modes in display order.
var Modi = []string{ModusBest, ModusBestDecimal, ModusBestTeiler, ModusLeague, ModusMostTens}

// IsDecimal reports whether a modus scores tenths.
func IsDecimal(modus string) bool { return modus == ModusBestDecimal }

// RankValue is the value a match is ranked by in the given modus.
func RankValue(modus string, shots Shots) float64 {
	if len(shots) == 0 {
		return 0
	}
	switch modus {
	case ModusBestDecimal:
		return shots.Summe()
	case ModusBestTeiler:
		return shots.Best().Teiler
	case ModusMostTens:
		return float64(shots.CountRing(10))
	default:
		return float64(shots.SummeGanz())
	}
}

// SortMatches sorts matches best first according to modus.
func SortMatches(modus string, matches []*Match) {
	ascending := modus == ModusBestTeiler
	slices.SortStableFunc(matches, func(a, b *Match) int {
		// Empty matches always rank last.
		if len(a.Shots) == 0 || len(b.Shots) == 0 {
			return cmp.Compare(len(b.Shots), len(a.Shots))
		}
		c := cmp.Compare(RankValue(modus, a.Shots), RankValue(modus, b.Shots))
		if ascending {
			return c
		}
		return -c
	})
}
