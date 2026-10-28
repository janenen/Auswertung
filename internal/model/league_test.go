package model

import (
	"slices"
	"testing"
)

// The example of the RSB instructions for the league form: a draw.
func TestScoreLeagueExample(t *testing.T) {
	home := []int{285, 295, 278, 289}  // Guske, Kolodziej, Wolf, Biletzki
	guest := []int{291, 289, 288, 286} // Gargan, Glück, Tuschy, Seringhaus
	r := ScoreLeague(home, guest, true)

	if !slices.Equal(r.HomePlaces, []int{3, 1, 4, 2}) || !slices.Equal(r.GuestPlaces, []int{1, 2, 3, 4}) {
		t.Errorf("places = %v %v", r.HomePlaces, r.GuestPlaces)
	}
	want := []LeaguePairing{
		{Place: 1, Home: 1, Guest: 0, HomePoints: 2, GuestPoints: 0}, // 295 : 291
		{Place: 2, Home: 3, Guest: 1, HomePoints: 1, GuestPoints: 1}, // 289 : 289
		{Place: 3, Home: 0, Guest: 2, HomePoints: 0, GuestPoints: 2}, // 285 : 288
	}
	if !slices.Equal(r.Pairings, want) {
		t.Errorf("pairings = %+v", r.Pairings)
	}
	if r.HomePoints != 3 || r.GuestPoints != 3 || !r.Complete || r.HomeTeamPoints != 1 || r.GuestTeamPoints != 1 {
		t.Errorf("result = %+v", r)
	}
}

func TestScoreLeagueIncomplete(t *testing.T) {
	r := ScoreLeague([]int{380, 375}, []int{370}, true)
	if r.Complete || r.HomeTeamPoints != 0 || r.GuestTeamPoints != 0 {
		t.Errorf("team points awarded: %+v", r)
	}
	// Only pairings with both shooters count.
	if r.HomePoints != 2 || r.GuestPoints != 0 || r.Pairings[1].Guest != -1 || r.Pairings[2].Home != -1 {
		t.Errorf("result = %+v", r)
	}
	if r := ScoreLeague([]int{1, 2, 3}, []int{3, 2, 1}, false); r.Complete {
		t.Error("complete with provisional results")
	}
	if r := ScoreLeague([]int{300, 290, 280}, []int{290, 280, 270}, true); r.HomeTeamPoints != 2 || r.GuestTeamPoints != 0 {
		t.Errorf("home win = %+v", r)
	}
}
