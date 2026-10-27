// Package model contains the domain types of Auswertung. The JSON field names
// match the files written by the former Python implementation so existing
// databases in ./db keep working.
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

type Match struct {
	Shooter      string   `json:"shooter"`
	TypeOfTarget string   `json:"type_of_target"`
	Date         string   `json:"date"`
	Shots        Shots    `json:"shots"`
	ID           string   `json:"id"`
	Active       bool     `json:"active"`
	Club         string   `json:"club"`
	Team         string   `json:"team"`
	Competitions []string `json:"competitions"`
}

func (m *Match) AddCompetition(id string) {
	if !slices.Contains(m.Competitions, id) {
		m.Competitions = append(m.Competitions, id)
	}
}

type User struct {
	Name         string   `json:"name"`
	Birthday     string   `json:"birthday"`
	Niceness     int      `json:"niceness"`
	ID           string   `json:"id"`
	Matches      []string `json:"matches"`
	Competitions []string `json:"competitions"`
}

func (u *User) AddMatch(id string) {
	if !slices.Contains(u.Matches, id) {
		u.Matches = append(u.Matches, id)
	}
}

func (u *User) AddCompetition(id string) {
	if !slices.Contains(u.Competitions, id) {
		u.Competitions = append(u.Competitions, id)
	}
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
	Name           string   `json:"name"`
	Date           string   `json:"date"`
	Count          int      `json:"count"`
	ShotsPerTarget int      `json:"shots_per_target"`
	TypeOfTarget   string   `json:"type_of_target"`
	Decimal        bool     `json:"decimal"`
	Active         bool     `json:"active"`
	Modus          string   `json:"modus"`
	Entries        []string `json:"entries"`
	ID             string   `json:"id"`
	League         string   `json:"league"`
}

func (c *Competition) AddMatch(id string) {
	if !slices.Contains(c.Entries, id) {
		c.Entries = append(c.Entries, id)
	}
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
