package server

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/janenen/auswertung/internal/model"
	"github.com/janenen/auswertung/internal/report"
)

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"targets":     model.Targets,
		"targetTypes": model.TargetTypes,
		"modi":        model.Modi,
		"leagueModus": model.ModusLeague,
		"leagues":     model.Leagues,
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
}

// rankedLists holds the result lists for the PDF report.
type rankedLists struct{ ranked, home, guest []report.RankedEntry }

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
		*entries = append(*entries, report.RankedEntry{Shooter: e.Shooter, Match: m})
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
		return res, lists, nil
	}
	ptrs := make([]*model.Match, len(matches))
	for i := range matches {
		ptrs[i] = &matches[i]
	}
	model.SortMatches(c.Modus, ptrs)
	for _, m := range ptrs {
		add(&res.Ranked, &lists.ranked, *m)
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

func (s *Server) competitionPDF(w http.ResponseWriter, r *http.Request) {
	res, lists, err := s.results(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	c := res.Competition
	data, err := report.CompetitionPDF(c, lists.ranked, lists.home, lists.guest)
	if err != nil {
		fail(w, err)
		return
	}
	s.deliver(w, data, "application/pdf", []string{"competitions", report.SafeName(c.Name)},
		report.FileDate(c.Date)+"_"+c.TypeOfTarget, ".pdf", false)
}

// deliver saves a generated file below the output directory and sends it.
func (s *Server) deliver(w http.ResponseWriter, data []byte, contentType string, dir []string, base, ext string, attachment bool) {
	path, err := report.Save(s.opts.OutDir, dir, base, ext, data)
	if err != nil {
		log.Printf("saving %s: %v", base+ext, err)
	} else {
		log.Printf("saved %s", path)
	}
	disposition := "inline"
	if attachment {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, base+ext))
	w.Write(data)
}
