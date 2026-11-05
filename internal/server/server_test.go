package server

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/model"
	"github.com/janenen/auswertung/internal/reading"

	"github.com/janenen/auswertung/internal/store"
)

// openStore opens a store in a temporary directory, closed after the test.
func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	st := openStore(t)
	frontend := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	ts := httptest.NewServer(New(st, frontend, Options{LeagueTemplate: filepath.Join("..", "..", "extern", "formular_ligaergebnisse_kr_bez_ll.xlsx")}))
	t.Cleanup(ts.Close)
	return ts
}

func call(t *testing.T, ts *httptest.Server, method, path string, body, out any) int {
	t.Helper()
	var r *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		r = bytes.NewReader(data)
	} else {
		r = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, ts.URL+path, r)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

// readEvents follows the SSE stream until it ends.
func readEvents(t *testing.T, ts *httptest.Server, id string) []map[string]any {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/readings/" + id + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var events []map[string]any
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			var e map[string]any
			json.Unmarshal([]byte(data), &e)
			events = append(events, e)
		}
	}
	return events
}

const testCSV = "04.09.2023;3;LG\r10,4;132,4;1,25;-0,42\r9,8;300,1;0,92;-2,85\r"

func TestReadingFlow(t *testing.T) {
	ts := newTestServer(t)

	var user struct{ ID string }
	call(t, ts, "POST", "/api/users", map[string]string{"name": "Anna"}, &user)

	var comp struct{ ID string }
	if code := call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "Training", "date": "04.09.2023", "count": 3, "shots_per_target": 1,
		"type_of_target": "LG", "modus": "Bestes Ergebnis",
	}, &comp); code != 200 {
		t.Fatalf("create competition: %d", code)
	}

	// First reading: 2 of 3 shots from CSV.
	var sess struct{ ID string }
	if code := call(t, ts, "POST", "/api/readings", map[string]any{
		"machineId": "csv", "competitionId": comp.ID, "userId": user.ID, "data": []byte(testCSV),
	}, &sess); code != 200 {
		t.Fatalf("start reading: %d", code)
	}
	events := readEvents(t, ts, sess.ID)
	last := events[len(events)-1]
	matchID, _ := last["matchId"].(string)
	if last["type"] != "done" || matchID == "" {
		t.Fatalf("events = %v", events)
	}

	var res resultsView
	call(t, ts, "GET", "/api/competitions/"+comp.ID+"/results", nil, &res)
	if len(res.Ranked) != 1 || res.Ranked[0].Shots != 2 || !res.Ranked[0].Active || res.Ranked[0].Value != "19" {
		t.Fatalf("results = %+v", res.Ranked)
	}

	// Continue the incomplete match with one QR shot.
	call(t, ts, "POST", "/api/readings", map[string]any{
		"machineId": "qr", "competitionId": comp.ID, "matchId": matchID, "data": []byte("04092023LGA0100.0+0.1+0.2"),
	}, &sess)
	readEvents(t, ts, sess.ID)
	var mv matchView
	call(t, ts, "GET", "/api/matches/"+matchID, nil, &mv)
	if len(mv.Match.Shots) != 3 || mv.Match.Active || mv.Total.SummeGanz != 29 || mv.Shooter != "Anna" {
		t.Fatalf("match = %+v", mv)
	}

	// Wrong target type is rejected and nothing is saved.
	call(t, ts, "POST", "/api/readings", map[string]any{
		"machineId": "qr", "competitionId": comp.ID, "userId": user.ID, "data": []byte("04092023LPA0100.0+0.1+0.2"),
	}, &sess)
	events = readEvents(t, ts, sess.ID)
	if events[len(events)-1]["matchId"] != nil {
		t.Errorf("wrong target saved: %v", events)
	}

	// The QR code is not saved, so it has no file name.
	if resp, err := http.Get(ts.URL + "/api/matches/" + matchID + "/qr.png"); err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Disposition") != "" {
		t.Errorf("QR code: %v %v", resp, err)
	} else {
		resp.Body.Close()
	}

	// Reports are printed in the browser; only the exports remain on the server.
	if resp, err := http.Get(ts.URL + "/api/matches/" + matchID + "/export.csv"); err != nil || resp.StatusCode != 200 ||
		!strings.HasPrefix(resp.Header.Get("Content-Disposition"), `attachment; filename="Anna_`) {
		t.Errorf("CSV export: %v %v", resp, err)
	} else {
		resp.Body.Close()
	}
	if code := call(t, ts, "GET", "/api/matches/"+matchID+"/report.pdf", nil, nil); code != http.StatusNotFound {
		t.Errorf("PDF endpoint still answers: %d", code)
	}

	if code := call(t, ts, "DELETE", "/api/competitions/"+comp.ID, nil, nil); code != http.StatusConflict {
		t.Errorf("delete with entries: %d", code)
	}
	call(t, ts, "PATCH", "/api/competitions/"+comp.ID, map[string]bool{"active": false}, nil)
	var inactive []competitionSummary
	call(t, ts, "GET", "/api/competitions?active=false", nil, &inactive)
	if len(inactive) != 1 || inactive[0].EntryCount != 1 {
		t.Errorf("inactive = %+v", inactive)
	}

	// Entries can only be removed while the competition is running.
	if code := call(t, ts, "DELETE", "/api/matches/"+matchID, nil, nil); code != http.StatusBadRequest {
		t.Errorf("delete entry of finished competition: %d", code)
	}
	call(t, ts, "PATCH", "/api/competitions/"+comp.ID, map[string]bool{"active": true}, nil)
	if code := call(t, ts, "DELETE", "/api/matches/"+matchID, nil, nil); code != http.StatusNoContent {
		t.Errorf("delete entry: %d", code)
	}
	if code := call(t, ts, "GET", "/api/matches/"+matchID, nil, nil); code != http.StatusNotFound {
		t.Errorf("deleted entry still found: %d", code)
	}
	if code := call(t, ts, "DELETE", "/api/competitions/"+comp.ID, nil, nil); code != http.StatusNoContent {
		t.Errorf("delete emptied competition: %d", code)
	}
}

func TestQuickContinue(t *testing.T) {
	ts := newTestServer(t)
	var user struct{ ID string }
	call(t, ts, "POST", "/api/users", map[string]string{"name": "Anna"}, &user)
	quick := map[string]any{"name": "Training", "count": 3, "shots_per_target": 1, "type_of_target": "LG", "decimal": true}

	// 2 of 3 shots, then the rest from the stored settings.
	var sess struct{ ID string }
	call(t, ts, "POST", "/api/readings", map[string]any{
		"machineId": "csv", "quick": quick, "userId": user.ID, "data": []byte(testCSV),
	}, &sess)
	events := readEvents(t, ts, sess.ID)
	matchID, _ := events[len(events)-1]["matchId"].(string)
	var mv matchView
	call(t, ts, "GET", "/api/matches/"+matchID, nil, &mv)
	if !mv.Match.Active || mv.Match.Count != 3 || mv.Match.ShotsPerTarget != 1 || mv.Match.Title != "Training" {
		t.Fatalf("match = %+v", mv.Match)
	}
	if code := call(t, ts, "POST", "/api/readings", map[string]any{
		"machineId": "qr", "matchId": matchID, "data": []byte("04092023LGA0100.0+0.1+0.2"),
	}, &sess); code != 200 {
		t.Fatalf("continue: %d", code)
	}
	readEvents(t, ts, sess.ID)
	call(t, ts, "GET", "/api/matches/"+matchID, nil, &mv)
	if len(mv.Match.Shots) != 3 || mv.Match.Active || !mv.Match.Decimal {
		t.Fatalf("continued match = %+v", mv.Match)
	}
	if code := call(t, ts, "POST", "/api/readings", map[string]any{
		"machineId": "qr", "matchId": matchID, "data": []byte("04092023LGA0100.0+0.1+0.2"),
	}, &sess); code != http.StatusBadRequest {
		t.Errorf("continue complete match: %d", code)
	}

	if code := call(t, ts, "DELETE", "/api/matches/"+matchID, nil, nil); code != http.StatusNoContent {
		t.Errorf("delete: %d", code)
	}
	if code := call(t, ts, "DELETE", "/api/matches/"+matchID, nil, nil); code != http.StatusNotFound {
		t.Errorf("delete again: %d", code)
	}
}

func TestValidation(t *testing.T) {
	ts := newTestServer(t)
	if code := call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "x", "count": 40, "shots_per_target": 1, "type_of_target": "XX", "modus": "Bestes Ergebnis",
	}, nil); code != 400 {
		t.Errorf("invalid target: %d", code)
	}
	if code := call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "Liga", "count": 40, "shots_per_target": 1, "type_of_target": "LG",
		"modus": "Liga des RSB (Kreis/Bezirk/Landesliga)",
	}, nil); code != 400 {
		t.Errorf("league without teams: %d", code)
	}
	if code := call(t, ts, "POST", "/api/users", map[string]string{"name": " "}, nil); code != 400 {
		t.Errorf("empty name: %d", code)
	}
	if code := call(t, ts, "GET", "/api/matches/nope", nil, nil); code != 404 {
		t.Errorf("unknown match: %d", code)
	}
	if code := call(t, ts, "GET", "/some/route", nil, nil); code != 200 {
		t.Errorf("spa fallback: %d", code)
	}
}

// endlessMachine sends one shot and then waits until cancelled.
type endlessMachine struct{}

func (endlessMachine) ID() string     { return "endless" }
func (endlessMachine) Name() string   { return "endless" }
func (endlessMachine) Input() string  { return "" }
func (endlessMachine) Accept() string { return "" }
func (endlessMachine) Read(ctx context.Context, _ machine.Request, out chan<- machine.Reading) error {
	out <- machine.Reading{Shot: model.Shot{Ringe: 10.3, Teiler: 50}, TargetType: "LG"}
	<-ctx.Done()
	return ctx.Err()
}

func TestBoard(t *testing.T) {
	st := openStore(t)
	srv := &Server{store: st, readings: reading.NewManager()}
	u, _ := st.SaveUser(model.User{Name: "Anna"})
	c, err := st.CreateCompetition(model.Competition{Name: "Pokal", Date: "01.03.2026", Count: 40, TypeOfTarget: "LG", Modus: model.ModusBestDecimal, Decimal: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateCompetition(model.Competition{Name: "Leer", Date: "01.03.2026", Count: 40, TypeOfTarget: "LG", Modus: model.ModusBest}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordNewMatch(store.NewMatch{Shooter: u.ID, Competition: c.ID, TypeOfTarget: "LG", Count: 40,
		Shots: model.Shots{{Ringe: 9.5}, {Ringe: 10.1}}}); err != nil {
		t.Fatal(err)
	}

	sess, err := srv.readings.Start(reading.Params{Machine: endlessMachine{}, Tag: reading.Tag{CompetitionID: c.ID, ShooterID: u.ID}})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Stop(false)
	for deadline := time.Now().Add(2 * time.Second); len(srv.readings.Live()) == 0 || len(srv.readings.Live()[0].Shots) == 0; {
		if time.Now().After(deadline) {
			t.Fatal("no live shot")
		}
		time.Sleep(10 * time.Millisecond)
	}

	rec := httptest.NewRecorder()
	srv.board(rec, httptest.NewRequest("GET", "/api/board", nil))
	var board []boardCompetition
	json.Unmarshal(rec.Body.Bytes(), &board)
	if len(board) != 1 || len(board[0].Tiles) != 2 {
		t.Fatalf("board = %+v", board)
	}
	saved, running := board[0].Tiles[0], board[0].Tiles[1]
	if saved.Live || saved.Result != "19.6" || saved.Shooter != "Anna" {
		t.Errorf("saved tile = %+v", saved)
	}
	if !running.Live || len(running.Shots) != 1 || running.Result != "10.3" {
		t.Errorf("live tile = %+v", running)
	}
}

func TestUserMatches(t *testing.T) {
	ts := newTestServer(t)
	var user struct{ ID string }
	var comp struct{ ID string }
	call(t, ts, "POST", "/api/users", map[string]string{"name": "Anna"}, &user)
	call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "Pokal", "date": "01.02.2026", "count": 1, "shots_per_target": 1, "type_of_target": "LG", "modus": "Bestes Ergebnis",
	}, &comp)
	var sess struct{ ID string }
	for _, req := range []map[string]any{
		{"competitionId": comp.ID},
		{"quick": map[string]any{"name": "Zu Hause", "count": 1, "shots_per_target": 1, "type_of_target": "LG", "decimal": true}},
	} {
		req["machineId"], req["userId"], req["data"] = "qr", user.ID, []byte("04092023LGA0100.0+0.1+0.2")
		if code := call(t, ts, "POST", "/api/readings", req, &sess); code != 200 {
			t.Fatalf("start reading %v: %d", req, code)
		}
		readEvents(t, ts, sess.ID)
	}

	var res struct {
		User    struct{ Name string }
		Matches []shooterMatchView
	}
	if code := call(t, ts, "GET", "/api/users/"+user.ID+"/matches", nil, &res); code != 200 {
		t.Fatalf("user matches: %d", code)
	}
	if res.User.Name != "Anna" || len(res.Matches) != 2 {
		t.Fatalf("result = %+v", res)
	}
	// Newest first: the quick analysis keeps its title and scoring.
	quick, entry := res.Matches[0], res.Matches[1]
	if quick.CompetitionID != "" || quick.Title != "Zu Hause" || !quick.Decimal || quick.Value != "10.0" || quick.Shots != 1 {
		t.Errorf("quick analysis = %+v", quick)
	}
	if entry.CompetitionID != comp.ID || entry.Title != "Pokal" || entry.Decimal || entry.Count != 1 {
		t.Errorf("competition entry = %+v", entry)
	}
	var mv matchView
	call(t, ts, "GET", "/api/matches/"+quick.MatchID, nil, &mv)
	if mv.Match.Title != "Zu Hause" || !mv.Match.Decimal {
		t.Errorf("stored quick analysis = %+v", mv.Match)
	}
	if code := call(t, ts, "GET", "/api/users/nope/matches", nil, nil); code != 404 {
		t.Errorf("unknown user: %d", code)
	}
}

func TestSimulateLeague(t *testing.T) {
	ts := newTestServer(t)
	var home, guest, team, league struct{ ID string }
	call(t, ts, "POST", "/api/clubs", map[string]string{"name": "SV Heim"}, &home)
	call(t, ts, "POST", "/api/clubs", map[string]string{"name": "SV Gast"}, &guest)
	call(t, ts, "POST", "/api/teams", map[string]string{"name": "1"}, &team)
	call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "Liga", "count": 20, "shots_per_target": 1, "type_of_target": "LP",
		"modus":      "Liga des RSB (Kreis/Bezirk/Landesliga)",
		"leagueData": map[string]string{"league": "Kreisliga", "home_club": home.ID, "home_team": team.ID, "guest_club": guest.ID, "guest_team": team.ID},
	}, &league)
	path := "/api/competitions/" + league.ID + "/simulate"

	if code := call(t, ts, "POST", path, nil, nil); code != 400 {
		t.Errorf("simulate without simulator: %d", code)
	}
	machine.SetSimulator(true)
	defer machine.SetSimulator(false)
	for range 2 {
		if code := call(t, ts, "POST", path, nil, nil); code != 200 {
			t.Fatalf("simulate: %d", code)
		}
	}

	var res resultsView
	call(t, ts, "GET", "/api/competitions/"+league.ID+"/results", nil, &res)
	if len(res.Home) != 6 || len(res.Guest) != 6 {
		t.Fatalf("home %d, guest %d entries", len(res.Home), len(res.Guest))
	}
	for _, e := range append(res.Home, res.Guest...) {
		if e.Shots != 20 || e.Active {
			t.Errorf("entry = %+v", e)
		}
	}
	if s := res.Score; s == nil || !s.Complete || len(s.HomePlaces) != 6 || len(s.Pairings) != 3 ||
		s.HomePoints+s.GuestPoints != 6 || s.HomeTeamPoints+s.GuestTeamPoints != 2 {
		t.Errorf("score = %+v", res.Score)
	}
	// Repeated simulations reuse the six simulated shooters.
	var users []model.User
	call(t, ts, "GET", "/api/users", nil, &users)
	if len(users) != 6 {
		t.Errorf("%d users", len(users))
	}
}

func TestLeagueExport(t *testing.T) {
	ts := newTestServer(t)
	var home, guest, team struct{ ID string }
	call(t, ts, "POST", "/api/clubs", map[string]string{"name": "SV Heim"}, &home)
	call(t, ts, "POST", "/api/clubs", map[string]string{"name": "SV Gast"}, &guest)
	call(t, ts, "POST", "/api/teams", map[string]string{"name": "1"}, &team)
	var plain, league struct{ ID string }
	call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "Training", "count": 40, "shots_per_target": 1, "type_of_target": "LG", "modus": "Bestes Ergebnis",
	}, &plain)
	call(t, ts, "POST", "/api/competitions", map[string]any{
		"name": "Liga", "date": "31.07.2025", "count": 40, "shots_per_target": 1, "type_of_target": "LG",
		"modus":      "Liga des RSB (Kreis/Bezirk/Landesliga)",
		"leagueData": map[string]string{"league": "Kreisliga", "home_club": home.ID, "home_team": team.ID, "guest_club": guest.ID, "guest_team": team.ID},
	}, &league)

	if code := call(t, ts, "GET", "/api/competitions/"+plain.ID+"/league.xlsx", nil, nil); code != 400 {
		t.Errorf("non-league export: %d", code)
	}
	want := 200
	if _, err := os.Stat(filepath.Join("..", "..", "extern", "formular_ligaergebnisse_kr_bez_ll.xlsx")); err != nil {
		want = 400 // template missing
	}
	if code := call(t, ts, "GET", "/api/competitions/"+league.ID+"/league.xlsx", nil, nil); code != want {
		t.Errorf("league export: %d, want %d", code, want)
	}
}
