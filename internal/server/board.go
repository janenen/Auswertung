package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/janenen/auswertung/internal/model"
	"github.com/janenen/auswertung/internal/report"
)

func (s *Server) leagueXLSX(w http.ResponseWriter, r *http.Request) {
	res, lists, err := s.results(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	if res.League == nil {
		fail(w, errBadRequest("Wettkampf ist kein Ligawettkampf"))
		return
	}
	l := res.League
	data, err := report.LeagueXLSX(s.opts.LeagueTemplate, report.LeagueSheet{
		Competition:  res.Competition,
		League:       l.RSBLeague,
		Home:         l.HomeClubName + " - " + l.HomeTeamName,
		Guest:        l.GuestClubName + " - " + l.GuestTeamName,
		HomeEntries:  lists.home,
		GuestEntries: lists.guest,
	})
	if errors.Is(err, report.ErrNoLeagueTemplate) {
		fail(w, errBadRequest(err.Error()))
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	s.deliver(w, data, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil,
		report.FileDate(res.Competition.Date)+"_Liga", ".xlsx", true)
}

// boardTile is one shooter on the visualisation board.
type boardTile struct {
	Key        string      `json:"key"`
	Shooter    string      `json:"shooter"`
	TargetType string      `json:"targetType"`
	Shots      model.Shots `json:"shots"`
	Result     string      `json:"result"`
	Series     []string    `json:"series"`
	Live       bool        `json:"live"`
}

type boardCompetition struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Count int         `json:"count"`
	Tiles []boardTile `json:"tiles"`
}

func tile(key, shooter, targetType string, shots model.Shots, decimal, live bool) boardTile {
	t := boardTile{Key: key, Shooter: shooter, TargetType: targetType, Shots: shots, Live: live, Series: []string{}}
	format := func(s model.Shots) string {
		if decimal {
			return strconv.FormatFloat(s.Summe(), 'f', 1, 64)
		}
		return strconv.Itoa(s.SummeGanz())
	}
	t.Result = format(shots)
	for _, series := range shots.Series() {
		t.Series = append(t.Series, format(series))
	}
	return t
}

// board returns the active competitions with all entries, including the
// shots of readings that are still running.
func (s *Server) board(w http.ResponseWriter, r *http.Request) {
	live := s.readings.Live()
	comps, err := s.store.Competitions(true)
	if err != nil {
		fail(w, err)
		return
	}
	out := []boardCompetition{}
	for _, c := range comps {
		bc := boardCompetition{ID: c.ID, Name: c.Name, Count: c.Count, Tiles: []boardTile{}}
		matches, err := s.store.CompetitionMatches(c.ID)
		if err != nil {
			fail(w, err)
			return
		}
		for _, m := range matches {
			shots, running := m.Shots, false
			for _, l := range live {
				if l.Tag.MatchID == m.ID {
					shots, running = append(shots, l.Shots...), true
				}
			}
			shooter, _, _ := s.names(m)
			bc.Tiles = append(bc.Tiles, tile(m.ID, shooter, m.TypeOfTarget, shots, c.Decimal, running))
		}
		for i, l := range live {
			if l.Tag.CompetitionID != c.ID || l.Tag.MatchID != "" {
				continue
			}
			shooter := ""
			if u, err := s.store.User(l.Tag.ShooterID); err == nil {
				shooter = u.Name
			}
			bc.Tiles = append(bc.Tiles, tile("live-"+strconv.Itoa(i)+"-"+l.Tag.ShooterID, shooter, c.TypeOfTarget, l.Shots, c.Decimal, true))
		}
		if len(bc.Tiles) > 0 {
			out = append(out, bc)
		}
	}
	writeJSON(w, out)
}
