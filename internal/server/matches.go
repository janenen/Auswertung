package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/model"
	"github.com/janenen/auswertung/internal/reading"
	"github.com/janenen/auswertung/internal/report"
	"github.com/janenen/auswertung/internal/store"
)

type statsView struct {
	Count     int        `json:"count"`
	Summe     float64    `json:"summe"`
	SummeGanz int        `json:"summeGanz"`
	Best      model.Shot `json:"best"`
	AblageRL  float64    `json:"ablageRL"`
	AblageHT  float64    `json:"ablageHT"`
	// Rings counts shots per ring value 10 down to 0.
	Rings []int `json:"rings"`
}

func stats(s model.Shots) statsView {
	v := statsView{Count: len(s), Summe: s.Summe(), SummeGanz: s.SummeGanz(), AblageRL: s.AblageRL(), AblageHT: s.AblageHT()}
	if len(s) > 0 {
		v.Best = s.Best()
	}
	for ring := 10; ring >= 0; ring-- {
		v.Rings = append(v.Rings, s.CountRing(ring))
	}
	return v
}

type matchView struct {
	Match   model.Match `json:"match"`
	Shooter string      `json:"shooter"`
	Club    string      `json:"club"`
	Team    string      `json:"team"`
	Total   statsView   `json:"total"`
	Series  []statsView `json:"series"`
}

func (s *Server) getMatch(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.Match(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	v := matchView{Match: m, Total: stats(m.Shots), Series: []statsView{}}
	v.Shooter, v.Club, v.Team = s.names(m)
	for _, series := range m.Shots.Series() {
		v.Series = append(v.Series, stats(series))
	}
	writeJSON(w, v)
}

func (s *Server) matchCSV(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.Match(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	shooter, _, _ := s.names(m)
	send(w, report.CSV(m), "text/csv; charset=utf-8", report.SafeName(shooter)+"_"+report.FileDate(m.Date)+".csv", true)
}

func (s *Server) matchQR(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.Match(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	data, err := report.QRPNG(m)
	if err != nil {
		fail(w, errBadRequest("QR-Code kann nicht erzeugt werden: "+err.Error()))
		return
	}
	// Shown on the result page only, so no copy is saved.
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

// --- machines and readings ---

type machineView struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Input  string `json:"input"`
	Accept string `json:"accept"`
}

func (s *Server) listMachines(w http.ResponseWriter, r *http.Request) {
	out := []machineView{}
	for _, m := range machine.Discover() {
		out = append(out, machineView{m.ID(), m.Name(), m.Input(), m.Accept()})
	}
	writeJSON(w, out)
}

type quickSettings struct {
	Name           string `json:"name"`
	Count          int    `json:"count"`
	ShotsPerTarget int    `json:"shots_per_target"`
	TypeOfTarget   string `json:"type_of_target"`
	Decimal        bool   `json:"decimal"`
}

type startReadingRequest struct {
	MachineID     string `json:"machineId"`
	CompetitionID string `json:"competitionId"`
	// Quick is used instead of a competition for a quick analysis.
	Quick *quickSettings `json:"quick"`
	// MatchID continues an incomplete match of the competition.
	MatchID string `json:"matchId"`
	UserID  string `json:"userId"`
	ClubID  string `json:"clubId"`
	TeamID  string `json:"teamId"`
	// Data is the uploaded file or entered QR code (JSON encodes []byte as base64).
	Data []byte `json:"data"`
}

func (s *Server) startReading(w http.ResponseWriter, r *http.Request) {
	var req startReadingRequest
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	params, err := s.readingParams(req)
	if err != nil {
		fail(w, err)
		return
	}
	sess, err := s.readings.Start(params)
	if err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, map[string]string{"id": sess.ID})
}

func (s *Server) readingParams(req startReadingRequest) (reading.Params, error) {
	m, err := machine.ByID(req.MachineID)
	if err != nil {
		return reading.Params{}, errBadRequest(err.Error())
	}
	if m.Input() != machine.InputNone && len(req.Data) == 0 {
		return reading.Params{}, errBadRequest("Keine Datei bzw. kein QR-Code angegeben")
	}
	p := reading.Params{Machine: m}

	switch {
	case req.MatchID != "":
		c, err := s.store.Competition(req.CompetitionID)
		if err != nil {
			return p, err
		}
		match, err := s.store.Match(req.MatchID)
		if err != nil {
			return p, err
		}
		rest := c.Count - len(match.Shots)
		if rest <= 0 {
			return p, errBadRequest("Dieses Ergebnis ist bereits vollständig")
		}
		p.Request = machine.Request{Count: rest, ShotsPerTarget: c.ShotsPerTarget}
		p.ExpectedTarget = match.TypeOfTarget
		p.Tag = reading.Tag{CompetitionID: c.ID, MatchID: match.ID, ShooterID: match.Shooter}
		p.Finish = func(shots model.Shots, _ string) (string, error) {
			m, err := s.store.AppendShots(match.ID, shots, c.Count)
			return m.ID, err
		}

	case req.CompetitionID != "":
		c, err := s.store.Competition(req.CompetitionID)
		if err != nil {
			return p, err
		}
		if !c.Active {
			return p, errBadRequest("Wettkampf ist beendet")
		}
		if _, err := s.store.User(req.UserID); err != nil {
			return p, errBadRequest("Schütze auswählen")
		}
		p.Request = machine.Request{Count: c.Count, ShotsPerTarget: c.ShotsPerTarget}
		p.ExpectedTarget = c.TypeOfTarget
		p.Tag = reading.Tag{CompetitionID: c.ID, ShooterID: req.UserID}
		p.Finish = s.recordNew(req, store.NewMatch{Competition: c.ID, Count: c.Count})

	case req.Quick != nil:
		q := req.Quick
		if _, ok := model.Targets[q.TypeOfTarget]; !ok {
			return p, errBadRequest("Kein gültiger Scheibentyp ausgewählt")
		}
		if q.Count < 1 || q.ShotsPerTarget < 1 || q.ShotsPerTarget > 5 {
			return p, errBadRequest("Ungültige Schusszahl")
		}
		if _, err := s.store.User(req.UserID); err != nil {
			return p, errBadRequest("Schütze auswählen")
		}
		p.Request = machine.Request{Count: q.Count, ShotsPerTarget: q.ShotsPerTarget}
		p.ExpectedTarget = q.TypeOfTarget
		p.Finish = s.recordNew(req, store.NewMatch{Title: strings.TrimSpace(q.Name), Decimal: q.Decimal, Count: q.Count})

	default:
		return p, errBadRequest("Wettkampf oder Schnellauswertung angeben")
	}
	p.Request.TargetType = p.ExpectedTarget
	p.Request.Data = req.Data
	return p, nil
}

// recordNew saves a new match of the requesting shooter; n holds the
// competition or quick analysis settings.
func (s *Server) recordNew(req startReadingRequest, n store.NewMatch) reading.FinishFunc {
	return func(shots model.Shots, targetType string) (string, error) {
		n.Shooter, n.Club, n.Team, n.TypeOfTarget, n.Shots = req.UserID, req.ClubID, req.TeamID, targetType, shots
		m, err := s.store.RecordNewMatch(n)
		return m.ID, err
	}
}

func (s *Server) readingEvents(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.readings.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("Einlesevorgang nicht gefunden"))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	next := 0
	if last, err := strconv.Atoi(r.Header.Get("Last-Event-ID")); err == nil {
		next = last + 1
	}
	for {
		events, done, changed := sess.Events(next)
		for _, e := range events {
			data, _ := json.Marshal(e)
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", next, data)
			next++
		}
		flusher.Flush()
		if done {
			return
		}
		select {
		case <-changed:
		case <-r.Context().Done():
			return
		case <-time.After(15 * time.Second):
			fmt.Fprint(w, ": ping\n\n")
		}
	}
}

func (s *Server) stopReading(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.readings.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("Einlesevorgang nicht gefunden"))
		return
	}
	var req struct {
		Keep bool `json:"keep"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, err)
		return
	}
	sess.Stop(req.Keep)
	w.WriteHeader(http.StatusNoContent)
}
