package server

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/model"
	"github.com/janenen/auswertung/internal/report"
	"github.com/janenen/auswertung/internal/store"
)

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"targets":     model.Targets,
		"targetTypes": model.TargetTypes,
		"modi":        model.Modi,
		"leagueModus": model.ModusLeague,
		"leagues":     model.Leagues,
		"simulator":   machine.SimulatorEnabled(),
	})
}

// --- users, clubs, teams ---

type userView struct {
	model.User
	MatchCount int `json:"matchCount"`
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.Users()
	if err != nil {
		fail(w, err)
		return
	}
	counts, err := s.store.MatchCounts()
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]userView, len(users))
	for i, u := range users {
		out[i] = userView{u, counts[u.ID]}
	}
	writeJSON(w, out)
}

// shooterMatchView is one result in the list of a shooter.
type shooterMatchView struct {
	entryView
	// CompetitionID is empty for a quick analysis; Title is the competition
	// name or the quick analysis title.
	CompetitionID string `json:"competitionId"`
	Title         string `json:"title"`
	TypeOfTarget  string `json:"typeOfTarget"`
	Decimal       bool   `json:"decimal"`
	// Count is the competition's number of shots, 0 for a quick analysis.
	Count int `json:"count"`
}

// userMatches returns a shooter with all results, newest first.
func (s *Server) userMatches(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.User(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	matches, err := s.store.UserMatches(u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	comps := map[string]model.Competition{}
	out := []shooterMatchView{}
	for _, m := range slices.Backward(matches) {
		c, title := model.Competition{Decimal: m.Decimal, Modus: model.ModusBest}, cmp.Or(m.Title, "Schnellauswertung")
		if m.Decimal {
			c.Modus = model.ModusBestDecimal
		}
		if m.Competition != "" {
			if _, ok := comps[m.Competition]; !ok {
				if comps[m.Competition], err = s.store.Competition(m.Competition); err != nil {
					fail(w, err)
					return
				}
			}
			c = comps[m.Competition]
			title = c.Name
		}
		out = append(out, shooterMatchView{
			entryView: s.entry(c, m), CompetitionID: m.Competition, Title: title,
			TypeOfTarget: m.TypeOfTarget, Decimal: c.Decimal, Count: c.Count,
		})
	}
	// Recording order is chronological except for imported data.
	slices.SortStableFunc(out, func(a, b shooterMatchView) int { return cmp.Compare(sortDate(b.Date), sortDate(a.Date)) })
	writeJSON(w, map[string]any{"user": u, "matches": out})
}

// sortDate makes DD.MM.YYYY sortable.
func sortDate(d string) string {
	if t, err := time.Parse("02.01.2006", d); err == nil {
		return t.Format("2006-01-02")
	}
	return d
}

func (s *Server) listClubs(w http.ResponseWriter, r *http.Request) {
	clubs, err := s.store.Clubs()
	respond(w, clubs, err)
}

func (s *Server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.store.Teams()
	respond(w, teams, err)
}

// respond writes v, or the error if there is one.
func respond(w http.ResponseWriter, v any, err error) {
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, v)
}

func requireName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errBadRequest("Name fehlt")
	}
	return name, nil
}

func (s *Server) saveUser(w http.ResponseWriter, r *http.Request) {
	var u model.User
	if err := decode(r, &u); err != nil {
		fail(w, err)
		return
	}
	u.ID = r.PathValue("id")
	var err error
	if u.Name, err = requireName(u.Name); err != nil {
		fail(w, err)
		return
	}
	u, err = s.store.SaveUser(u)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, u)
}

func (s *Server) saveClub(w http.ResponseWriter, r *http.Request) {
	var c model.Club
	if err := decode(r, &c); err != nil {
		fail(w, err)
		return
	}
	c.ID = r.PathValue("id")
	var err error
	if c.Name, err = requireName(c.Name); err != nil {
		fail(w, err)
		return
	}
	if c, err = s.store.SaveClub(c); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, c)
}

func (s *Server) saveTeam(w http.ResponseWriter, r *http.Request) {
	var t model.Team
	if err := decode(r, &t); err != nil {
		fail(w, err)
		return
	}
	t.ID = r.PathValue("id")
	var err error
	if t.Name, err = requireName(t.Name); err != nil {
		fail(w, err)
		return
	}
	if t, err = s.store.SaveTeam(t); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, t)
}

// --- competitions ---

type competitionSummary struct {
	model.Competition
	EntryCount int `json:"entryCount"`
}

func (s *Server) listCompetitions(w http.ResponseWriter, r *http.Request) {
	comps, err := s.store.Competitions(r.URL.Query().Get("active") != "false")
	if err != nil {
		fail(w, err)
		return
	}
	counts, err := s.store.EntryCounts()
	if err != nil {
		fail(w, err)
		return
	}
	out := []competitionSummary{}
	for _, c := range comps {
		out = append(out, competitionSummary{c, counts[c.ID]})
	}
	writeJSON(w, out)
}

type createCompetitionRequest struct {
	model.Competition
	League *model.RSBLeague `json:"leagueData"`
}

func validateCompetition(c *model.Competition, league *model.RSBLeague) error {
	var err error
	if c.Name, err = requireName(c.Name); err != nil {
		return err
	}
	if strings.TrimSpace(c.Date) == "" {
		c.Date = time.Now().Format("02.01.2006")
	}
	if _, ok := model.Targets[c.TypeOfTarget]; !ok {
		return errBadRequest("Kein gültiger Scheibentyp ausgewählt")
	}
	if c.Count < 1 {
		return errBadRequest("Schusszahl muss mindestens 1 sein")
	}
	if c.ShotsPerTarget < 1 || c.ShotsPerTarget > 5 {
		return errBadRequest("Schuss pro Scheibe muss zwischen 1 und 5 liegen")
	}
	if !slices.Contains(model.Modi, c.Modus) {
		return errBadRequest("Kein gültiger Modus ausgewählt")
	}
	c.Decimal = model.IsDecimal(c.Modus)
	if c.Modus == model.ModusLeague {
		if league == nil || league.HomeClub == "" || league.HomeTeam == "" || league.GuestClub == "" || league.GuestTeam == "" {
			return errBadRequest("Heim- und Gastverein sowie -mannschaft auswählen")
		}
	}
	return nil
}

func (s *Server) createCompetition(w http.ResponseWriter, r *http.Request) {
	var req createCompetitionRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	c := req.Competition
	if err := validateCompetition(&c, req.League); err != nil {
		fail(w, err)
		return
	}
	league := req.League
	if c.Modus != model.ModusLeague {
		league = nil
	}
	c, err := s.store.CreateCompetition(c, league)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, c)
}

// entryView is a match within a competition.
type entryView struct {
	MatchID   string   `json:"matchId"`
	ShooterID string   `json:"shooterId"`
	Shooter   string   `json:"shooter"`
	ClubID    string   `json:"clubId"`
	Club      string   `json:"club"`
	TeamID    string   `json:"teamId"`
	Team      string   `json:"team"`
	Date      string   `json:"date"`
	Shots     int      `json:"shots"`
	Active    bool     `json:"active"`
	Value     string   `json:"value"`
	SummeGanz int      `json:"summeGanz"`
	Summe     float64  `json:"summe"`
	Series    []string `json:"series"`
}

type leagueView struct {
	model.RSBLeague
	HomeClubName  string `json:"homeClubName"`
	HomeTeamName  string `json:"homeTeamName"`
	GuestClubName string `json:"guestClubName"`
	GuestTeamName string `json:"guestTeamName"`
}

type competitionView struct {
	Competition model.Competition `json:"competition"`
	League      *leagueView       `json:"league"`
	Entries     []entryView       `json:"entries"`
}

func (s *Server) names(m model.Match) (shooter, club, team string) {
	if u, err := s.store.User(m.Shooter); err == nil {
		shooter = u.Name
	}
	if c, err := s.store.Club(m.Club); err == nil {
		club = c.Name
	}
	if t, err := s.store.Team(m.Team); err == nil {
		team = t.Name
	}
	return
}

func (s *Server) entry(c model.Competition, m model.Match) entryView {
	shooter, club, team := s.names(m)
	e := entryView{
		MatchID: m.ID, ShooterID: m.Shooter, Shooter: shooter, ClubID: m.Club, Club: club, TeamID: m.Team, Team: team,
		Date: m.Date, Shots: len(m.Shots), Active: m.Active, Value: report.FormatRankValue(c.Modus, m.Shots),
		SummeGanz: m.Shots.SummeGanz(), Summe: m.Shots.Summe(), Series: []string{},
	}
	for _, series := range m.Shots.Series() {
		if c.Decimal {
			e.Series = append(e.Series, strconv.FormatFloat(series.Summe(), 'f', 1, 64))
		} else {
			e.Series = append(e.Series, strconv.Itoa(series.SummeGanz()))
		}
	}
	return e
}

func (s *Server) competitionView(id string) (competitionView, error) {
	c, err := s.store.Competition(id)
	if err != nil {
		return competitionView{}, err
	}
	v := competitionView{Competition: c, Entries: []entryView{}}
	if c.League != "" {
		if l, err := s.store.League(c.League); err == nil {
			lv := leagueView{RSBLeague: l}
			if club, err := s.store.Club(l.HomeClub); err == nil {
				lv.HomeClubName = club.Name
			}
			if club, err := s.store.Club(l.GuestClub); err == nil {
				lv.GuestClubName = club.Name
			}
			if team, err := s.store.Team(l.HomeTeam); err == nil {
				lv.HomeTeamName = team.Name
			}
			if team, err := s.store.Team(l.GuestTeam); err == nil {
				lv.GuestTeamName = team.Name
			}
			v.League = &lv
		}
	}
	matches, err := s.store.CompetitionMatches(c.ID)
	if err != nil {
		return competitionView{}, err
	}
	for _, m := range matches {
		v.Entries = append(v.Entries, s.entry(c, m))
	}
	return v, nil
}

func (s *Server) getCompetition(w http.ResponseWriter, r *http.Request) {
	v, err := s.competitionView(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) updateCompetition(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Active *bool `json:"active"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	if req.Active == nil {
		fail(w, errBadRequest("nichts zu ändern"))
		return
	}
	if err := s.store.SetCompetitionActive(r.PathValue("id"), *req.Active); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteCompetition(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCompetition(r.PathValue("id")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type resultsView struct {
	competitionView
	Ranked []entryView `json:"ranked"`
	Home   []entryView `json:"home"`
	Guest  []entryView `json:"guest"`
	// Score is the league scoring of Home against Guest.
	Score *model.LeagueResult `json:"score,omitempty"`
}

// rankedLists holds the home and guest lists for the league form.
type rankedLists struct{ home, guest []report.RankedEntry }

// results ranks the entries. For league competitions entries are split
// into home and guest by club, keeping the reading order.
func (s *Server) results(id string) (resultsView, rankedLists, error) {
	var lists rankedLists
	v, err := s.competitionView(id)
	if err != nil {
		return resultsView{}, lists, err
	}
	c := v.Competition
	matches, err := s.store.CompetitionMatches(c.ID)
	if err != nil {
		return resultsView{}, lists, err
	}
	res := resultsView{competitionView: v, Ranked: []entryView{}, Home: []entryView{}, Guest: []entryView{}}
	add := func(views *[]entryView, entries *[]report.RankedEntry, m model.Match) {
		e := s.entry(c, m)
		*views = append(*views, e)
		if entries != nil {
			*entries = append(*entries, report.RankedEntry{Shooter: e.Shooter, Match: m})
		}
	}
	if v.League != nil {
		for _, m := range matches {
			switch m.Club {
			case v.League.HomeClub:
				add(&res.Home, &lists.home, m)
			case v.League.GuestClub:
				add(&res.Guest, &lists.guest, m)
			}
		}
		final := true
		results := func(entries []entryView) []int {
			out := make([]int, len(entries))
			for i, e := range entries {
				out[i] = e.SummeGanz // full ring scoring (RSB-LO 6.3)
				final = final && !e.Active
			}
			return out
		}
		score := model.ScoreLeague(results(res.Home), results(res.Guest), final)
		res.Score = &score
		return res, lists, nil
	}
	ptrs := make([]*model.Match, len(matches))
	for i := range matches {
		ptrs[i] = &matches[i]
	}
	model.SortMatches(c.Modus, ptrs)
	for _, m := range ptrs {
		add(&res.Ranked, nil, *m)
	}
	return res, lists, nil
}

func (s *Server) competitionResults(w http.ResponseWriter, r *http.Request) {
	res, _, err := s.results(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, res)
}

// simulatedPerSide is the number of results simulateLeague records per team.
const simulatedPerSide = 3

// simulateLeague records random results for the home and guest team of a
// league competition (simulation mode only). Repeated calls reuse the
// simulated shooters.
func (s *Server) simulateLeague(w http.ResponseWriter, r *http.Request) {
	if !machine.SimulatorEnabled() {
		fail(w, errBadRequest("Simulator ist nicht aktiviert"))
		return
	}
	v, err := s.competitionView(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	c := v.Competition
	switch {
	case v.League == nil:
		fail(w, errBadRequest("Wettkampf ist kein Ligawettkampf"))
		return
	case !c.Active:
		fail(w, errBadRequest("Wettkampf ist beendet"))
		return
	}
	sides := []struct{ name, club, team string }{
		{"Simulierter Heimschütze", v.League.HomeClub, v.League.HomeTeam},
		{"Simulierter Gastschütze", v.League.GuestClub, v.League.GuestTeam},
	}
	ids := []string{}
	for _, side := range sides {
		for i := 1; i <= simulatedPerSide; i++ {
			user, err := s.userByName(fmt.Sprintf("%s %d", side.name, i))
			if err != nil {
				fail(w, err)
				return
			}
			m, err := s.store.RecordNewMatch(store.NewMatch{
				Shooter: user.ID, Club: side.club, Team: side.team, TypeOfTarget: c.TypeOfTarget,
				Competition: c.ID, Count: c.Count, Shots: machine.SimulatedShots(c.TypeOfTarget, c.Count),
			})
			if err != nil {
				fail(w, err)
				return
			}
			ids = append(ids, m.ID)
		}
	}
	writeJSON(w, map[string][]string{"matchIds": ids})
}

// userByName returns the user with the given name, creating it if needed.
func (s *Server) userByName(name string) (model.User, error) {
	users, err := s.store.Users()
	if err != nil {
		return model.User{}, err
	}
	for _, u := range users {
		if u.Name == name {
			return u, nil
		}
	}
	return s.store.SaveUser(model.User{Name: name})
}

// send streams a generated file. Nothing is saved: reports and exports can
// always be created again.
func send(w http.ResponseWriter, data []byte, contentType, filename string, attachment bool) {
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, filename))
	w.Write(data)
}
