package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/janenen/auswertung/internal/model"
)

func writeJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpenEmptyCreatesFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	if _, err := Open(dir); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"users.json", "matches.json", "competitions.json", "clubs.json", "teams.json", "leagues.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s not created: %v", f, err)
		}
	}
}

func TestRecordMatchFlow(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.SaveUser(model.User{Name: "Anna"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateCompetition(model.Competition{Name: "Training", Date: "01.02.2024", Count: 3, TypeOfTarget: "LG", Modus: model.ModusBest}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.RecordNewMatch(NewMatch{Shooter: u.ID, TypeOfTarget: "LG", Competition: c.ID, Count: 3,
		Shots: model.Shots{{Ringe: 10.1, Teiler: 50}, {Ringe: 9.5, Teiler: 300}}})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Active {
		t.Error("match with 2/3 shots should be active")
	}
	m, err = s.AppendShots(m.ID, model.Shots{{Ringe: 8.0, Teiler: 600}}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if m.Active || len(m.Shots) != 3 {
		t.Errorf("after append: active=%v shots=%d", m.Active, len(m.Shots))
	}

	// Reopen from disk and verify links.
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	u, _ = s.User(u.ID)
	c, _ = s.Competition(c.ID)
	if u.Niceness != 1 || len(u.Matches) != 1 || len(u.Competitions) != 1 {
		t.Errorf("user not linked: %+v", u)
	}
	if len(c.Entries) != 1 || c.Entries[0] != m.ID {
		t.Errorf("competition not linked: %+v", c)
	}
	if got := s.Matches(c.Entries)[0].Shots.SummeGanz(); got != 27 {
		t.Errorf("SummeGanz = %d, want 27", got)
	}
}

func TestMigrateV0(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "matches.json"), `{"matches": {"m1": {
		"shooter": {"name": "Bert", "birthday": "1.1.1990"}, "type_of_target": "LG", "date": "01.01.2020",
		"shots": [{"ringe": 10.2, "teiler": 1324, "x": 125, "y": -42}], "id": "m1"}}}`)
	writeJSON(t, filepath.Join(dir, "users.json"), `{"users": {"u1": {
		"shooter": {"name": "Bert", "birthday": "1.1.1990"}, "niceness": 4, "id": "u1",
		"matches": ["m1"], "competitions": []}}}`)

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Match("m1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Shooter != "u1" {
		t.Errorf("shooter = %q, want u1", m.Shooter)
	}
	want := model.Shot{Ringe: 10.2, Teiler: 132.4, X: 1.25, Y: -0.42}
	if m.Shots[0] != want {
		t.Errorf("shot = %+v, want %+v", m.Shots[0], want)
	}
	u, err := s.User("u1")
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "Bert" || u.Niceness != 4 {
		t.Errorf("user = %+v", u)
	}

	var raw struct{ Version int }
	data, _ := os.ReadFile(filepath.Join(dir, "matches.json"))
	json.Unmarshal(data, &raw)
	if raw.Version != 2 {
		t.Errorf("matches version = %d, want 2", raw.Version)
	}
}

func TestMigrateIni(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "db")
	writeJSON(t, filepath.Join(root, "schuetzen.ini"), "[Neu]\nName = x\n\n[S1]\nName = Carla\nniceness = 3\n")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	users := s.Users()
	if len(users) != 1 || users[0].Name != "Carla" || users[0].Niceness != 3 {
		t.Errorf("users = %+v", users)
	}
	if _, err := os.Stat(filepath.Join(root, "schuetzen.ini")); !os.IsNotExist(err) {
		t.Error("schuetzen.ini not removed")
	}
}

// TestRealDatabase round-trips the developer's database if present.
func TestRealDatabase(t *testing.T) {
	src := filepath.Join("..", "..", "db")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Skip("no ./db present")
	}
	dir := t.TempDir()
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		writeJSON(t, filepath.Join(dir, e.Name()), string(data))
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	all := append(s.Competitions(true), s.Competitions(false)...)
	t.Logf("users=%d competitions=%d clubs=%d teams=%d", len(s.Users()), len(all), len(s.Clubs()), len(s.Teams()))
	for _, c := range all {
		if got := len(s.Matches(c.Entries)); got != len(c.Entries) {
			t.Errorf("competition %s: %d of %d entries resolvable", c.Name, got, len(c.Entries))
		}
	}
}
