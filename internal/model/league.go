package model

import (
	"cmp"
	"slices"
)

// LeagueScored is the number of shooters per team that are scored
// (RSB-LO 5.2).
const LeagueScored = 3

// LeagueResult is the scoring of an RSB league match (RSB-LO 7.2, 7.3).
type LeagueResult struct {
	// HomePlaces and GuestPlaces are the places within the team, in entry order.
	HomePlaces  []int           `json:"homePlaces"`
	GuestPlaces []int           `json:"guestPlaces"`
	Pairings    []LeaguePairing `json:"pairings"`
	// HomePoints and GuestPoints are the Einzelpunkte.
	HomePoints  int `json:"homePoints"`
	GuestPoints int `json:"guestPoints"`
	// Complete is set when both teams have enough results; only then are
	// the Mannschaftspunkte awarded.
	Complete        bool `json:"complete"`
	HomeTeamPoints  int  `json:"homeTeamPoints"`
	GuestTeamPoints int  `json:"guestTeamPoints"`
}

// LeaguePairing compares the shooters on the same place. Home and Guest
// are entry indexes, -1 if the team has no shooter on that place.
type LeaguePairing struct {
	Place       int `json:"place"`
	Home        int `json:"home"`
	Guest       int `json:"guest"`
	HomePoints  int `json:"homePoints"`
	GuestPoints int `json:"guestPoints"`
}

// ScoreLeague scores a league match from the results (full rings) of the
// home and guest shooters in entry order. complete tells whether all
// results are final.
func ScoreLeague(home, guest []int, complete bool) LeagueResult {
	homeOrder, guestOrder := leagueOrder(home), leagueOrder(guest)
	r := LeagueResult{
		HomePlaces:  leaguePlaces(homeOrder),
		GuestPlaces: leaguePlaces(guestOrder),
		Pairings:    []LeaguePairing{},
		Complete:    complete && len(home) >= LeagueScored && len(guest) >= LeagueScored,
	}
	at := func(order []int, place int) int {
		if place < len(order) {
			return order[place]
		}
		return -1
	}
	for place := range LeagueScored {
		p := LeaguePairing{Place: place + 1, Home: at(homeOrder, place), Guest: at(guestOrder, place)}
		if p.Home >= 0 && p.Guest >= 0 {
			p.HomePoints, p.GuestPoints = points(home[p.Home], guest[p.Guest])
			r.HomePoints += p.HomePoints
			r.GuestPoints += p.GuestPoints
		}
		r.Pairings = append(r.Pairings, p)
	}
	if r.Complete {
		r.HomeTeamPoints, r.GuestTeamPoints = points(r.HomePoints, r.GuestPoints)
	}
	return r
}

// points gives 2:0 to the higher value and 1:1 on a tie.
func points(a, b int) (int, int) {
	switch {
	case a > b:
		return 2, 0
	case a < b:
		return 0, 2
	}
	return 1, 1
}

// leagueOrder returns the entry indexes from best to worst result; equal
// results keep the entry order.
func leagueOrder(results []int) []int {
	order := make([]int, len(results))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(results[b], results[a]) })
	return order
}

func leaguePlaces(order []int) []int {
	places := make([]int, len(order))
	for place, i := range order {
		places[i] = place + 1
	}
	return places
}
