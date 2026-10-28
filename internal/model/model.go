// Package model contains the domain types of Auswertung. Their JSON field
// names are the API of the web frontend (web/src/lib/api.ts).
package model

import (
	"math"
	"slices"
)

type Shot struct {
	Ringe  float64 `json:"ringe"`
	Teiler float64 `json:"teiler"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// RingeGanz is the integer ring value (no tenths).
func (s Shot) RingeGanz() int { return int(math.Floor(s.Ringe)) }

// Shots is a list of shots with derived statistics. It is used for whole
// matches as well as for single series.
type Shots []Shot

func (s Shots) Summe() float64 {
	var sum float64
	for _, shot := range s {
		sum += shot.Ringe
	}
	return round1(sum)
}

func (s Shots) SummeGanz() int {
	sum := 0
	for _, shot := range s {
		sum += shot.RingeGanz()
	}
	return sum
}

// Result returns the decimal or integer total.
func (s Shots) Result(decimal bool) float64 {
	if decimal {
		return s.Summe()
	}
	return float64(s.SummeGanz())
}

// Best returns the shot with the lowest Teiler.
func (s Shots) Best() Shot {
	return slices.MinFunc(s, func(a, b Shot) int { return cmpFloat(a.Teiler, b.Teiler) })
}

// Worst returns the shot with the highest Teiler.
func (s Shots) Worst() Shot {
	return slices.MaxFunc(s, func(a, b Shot) int { return cmpFloat(a.Teiler, b.Teiler) })
}

// AblageRL is the mean horizontal deviation.
func (s Shots) AblageRL() float64 {
	return mean(s, func(shot Shot) float64 { return shot.X })
}

// AblageHT is the mean vertical deviation.
func (s Shots) AblageHT() float64 {
	return mean(s, func(shot Shot) float64 { return shot.Y })
}

// CountRing counts the shots whose integer ring value equals ring.
func (s Shots) CountRing(ring int) int {
	n := 0
	for _, shot := range s {
		if shot.RingeGanz() == ring {
			n++
		}
	}
	return n
}

// Series splits the shots into series of 10.
func (s Shots) Series() []Shots {
	var series []Shots
	for start := 0; start < len(s); start += 10 {
		series = append(series, s[start:min(start+10, len(s))])
	}
	return series
}

// Match is the result of one shooter. Club, Team and Competition are empty
// when not set; a match without competition is a quick analysis.
type Match struct {
	ID           string `json:"id"`
	Shooter      string `json:"shooter"`
	Club         string `json:"club"`
	Team         string `json:"team"`
	Competition  string `json:"competition"`
	TypeOfTarget string `json:"type_of_target"`
	// Date is formatted as DD.MM.YYYY.
	Date string `json:"date"`
	// Active marks an incomplete match that can be continued.
	Active bool  `json:"active"`
	Shots  Shots `json:"shots"`
}

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Birthday string `json:"birthday"`
}

type Club struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Team struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type Competition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Date is formatted as DD.MM.YYYY.
	Date           string `json:"date"`
	Count          int    `json:"count"`
	ShotsPerTarget int    `json:"shots_per_target"`
	TypeOfTarget   string `json:"type_of_target"`
	Decimal        bool   `json:"decimal"`
	Active         bool   `json:"active"`
	Modus          string `json:"modus"`
	// League is the ID of the league pairing, empty for other competitions.
	League string `json:"league"`
}

// RSBLeague describes a league match between a home and a guest team.
type RSBLeague struct {
	Discipline string `json:"discipline"`
	League     string `json:"league"`
	Group      string `json:"group"`
	HomeClub   string `json:"home_club"`
	HomeTeam   string `json:"home_team"`
	GuestClub  string `json:"guest_club"`
	GuestTeam  string `json:"guest_team"`
	ID         string `json:"id"`
}

var Leagues = []string{"Kreisliga", "Bezirksliga", "Landesliga"}

func mean(s Shots, f func(Shot) float64) float64 {
	if len(s) == 0 {
		return 0
	}
	var sum float64
	for _, shot := range s {
		sum += f(shot)
	}
	return sum / float64(len(s))
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
