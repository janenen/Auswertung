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

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	frontend := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	ts := httptest.NewServer(New(st, frontend, Options{OutDir: t.TempDir(), LeagueTemplate: filepath.Join("..", "..", "extern", "formular_ligaergebnisse_kr_bez_ll.xlsx")}))
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

	for _, path := range []string{
		"/api/matches/" + matchID + "/report.pdf",
		"/api/matches/" + matchID + "/report.pdf?title=Test&decimal=true",
		"/api/matches/" + matchID + "/export.csv",
		"/api/matches/" + matchID + "/qr.png",
		"/api/competitions/" + comp.ID + "/report.pdf",
	} {
		if code := call(t, ts, "GET", path, nil, nil); code != 200 {
			t.Errorf("GET %s: %d", path, code)
		}
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
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{store: st, readings: reading.NewManager()}
	u, _ := st.SaveUser(model.User{Name: "Anna"})
	c, _ := st.CreateCompetition(model.Competition{Name: "Pokal", Count: 40, TypeOfTarget: "LG", Modus: model.ModusBestDecimal, Decimal: true}, nil)
	st.CreateCompetition(model.Competition{Name: "Leer", Count: 40, TypeOfTarget: "LG", Modus: model.ModusBest}, nil)
	st.RecordNewMatch(store.NewMatch{Shooter: u.ID, Competition: c.ID, TypeOfTarget: "LG", Count: 40,
		Shots: model.Shots{{Ringe: 9.5}, {Ringe: 10.1}}})

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
