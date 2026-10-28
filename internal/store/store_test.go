package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/janenen/auswertung/internal/model"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	s := open(t, dir)
	if _, err := os.Stat(filepath.Join(dir, DBFile)); err != nil {
		t.Errorf("database not created: %v", err)
	}
	if users, err := s.Users(); err != nil || len(users) != 0 {
		t.Errorf("users = %v, %v", users, err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyDir)); !os.IsNotExist(err) {
		t.Error("legacy directory created without legacy files")
	}
}

func TestRecordMatchFlow(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	u, err := s.SaveUser(model.User{Name: "Anna"})
	if err != nil {
		t.Fatal(err)
	}
	club, _ := s.SaveClub(model.Club{Name: "SV"})
	c, err := s.CreateCompetition(model.Competition{Name: "Training", Date: "01.02.2024", Count: 3, TypeOfTarget: "LG", Modus: model.ModusBest}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.RecordNewMatch(NewMatch{Shooter: u.ID, Club: club.ID, TypeOfTarget: "LG", Competition: c.ID, Count: 3,
		Shots: model.Shots{{Ringe: 10.1, Teiler: 50, X: 0.25, Y: -0.5}, {Ringe: 9.5, Teiler: 300}}})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Active {
		t.Error("match with 2/3 shots should be active")
	}
	quick, err := s.RecordNewMatch(NewMatch{Shooter: u.ID, TypeOfTarget: "LP", Count: 1, Shots: model.Shots{{Ringe: 7}}})
	if err != nil {
		t.Fatal(err)
	}
	m, err = s.AppendShots(m.ID, model.Shots{{Ringe: 8.0, Teiler: 600}}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if m.Active || len(m.Shots) != 3 || m.Shots[2].Teiler != 600 {
		t.Errorf("after append: %+v", m)
	}

	// Reopen and check the stored data.
	s.Close()
	s = open(t, dir)
	entries, err := s.CompetitionMatches(c.ID)
	if err != nil || len(entries) != 1 || entries[0].ID != m.ID {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	got := entries[0]
	if got.Club != club.ID || got.Team != "" || got.Competition != c.ID || got.Shots[0] != (model.Shot{Ringe: 10.1, Teiler: 50, X: 0.25, Y: -0.5}) {
		t.Errorf("match = %+v", got)
	}
	if got.Shots.SummeGanz() != 27 {
		t.Errorf("SummeGanz = %d, want 27", got.Shots.SummeGanz())
	}
	if q, _ := s.Match(quick.ID); q.Competition != "" || q.Date != quick.Date {
		t.Errorf("quick analysis = %+v", q)
	}
	if counts, _ := s.MatchCounts(); counts[u.ID] != 2 {
		t.Errorf("match counts = %v", counts)
	}
	if counts, _ := s.EntryCounts(); counts[c.ID] != 1 || len(counts) != 1 {
		t.Errorf("entry counts = %v", counts)
	}
	if c2, _ := s.Competition(c.ID); c2.Date != "01.02.2024" || !c2.Active {
		t.Errorf("competition = %+v", c2)
	}
}

// A database of an older schema version is upgraded on open.
func TestSchemaUpgrade(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		migrations[0], "PRAGMA user_version = 1",
		"INSERT INTO users (id, name) VALUES ('u1', 'Anna')",
		"INSERT INTO matches (id, shooter_id, target_type, date, active) VALUES ('m1', 'u1', 'LG', '2025-01-02', 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	s := open(t, dir)
	m, err := s.Match("m1")
	if err != nil || m.Title != "" || m.Decimal || m.Date != "02.01.2025" {
		t.Fatalf("match = %+v, %v", m, err)
	}
	quick, err := s.RecordNewMatch(NewMatch{Shooter: "u1", Title: "Zu Hause", Decimal: true, Count: 1})
	if err != nil {
		t.Fatal(err)
	}
	if ms, _ := s.UserMatches("u1"); len(ms) != 2 || ms[1].ID != quick.ID || ms[1].Title != "Zu Hause" || !ms[1].Decimal {
		t.Errorf("user matches = %+v", ms)
	}
}

func TestCompetitions(t *testing.T) {
	s := open(t, t.TempDir())
	if _, err := s.CreateCompetition(model.Competition{Name: "X", Date: "2024-02-01"}, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid date: %v", err)
	}
	home, _ := s.SaveClub(model.Club{Name: "Heim"})
	team, _ := s.SaveTeam(model.Team{Name: "1"})
	older, _ := s.CreateCompetition(model.Competition{Name: "B", Date: "31.12.2023"}, nil)
	league, err := s.CreateCompetition(model.Competition{Name: "A", Date: "01.01.2024", Modus: model.ModusLeague},
		&model.RSBLeague{League: "Kreisliga", Group: "1", HomeClub: home.ID, HomeTeam: team.ID})
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Competitions(true); len(list) != 2 || list[0].ID != league.ID || list[1].ID != older.ID {
		t.Errorf("not newest first: %+v", list)
	}
	l, err := s.League(league.League)
	if err != nil || l.HomeClub != home.ID || l.GuestClub != "" || l.Group != "1" {
		t.Errorf("league = %+v, %v", l, err)
	}

	u, _ := s.SaveUser(model.User{Name: "Anna"})
	s.RecordNewMatch(NewMatch{Shooter: u.ID, Competition: older.ID, Count: 1})
	if err := s.DeleteCompetition(older.ID); !errors.Is(err, ErrHasEntries) {
		t.Errorf("delete with entries: %v", err)
	}
	if err := s.DeleteCompetition(league.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.League(league.League); !errors.Is(err, ErrNotFound) {
		t.Errorf("league not deleted with competition: %v", err)
	}
	if err := s.SetCompetitionActive("nope", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown competition: %v", err)
	}
	if _, err := s.RecordNewMatch(NewMatch{Shooter: "nope", Count: 1}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown shooter: %v", err)
	}
}

func TestLegacyImport(t *testing.T) {
	dir := t.TempDir()
	// matches v0 (embedded shooter, Teiler/coordinates as integers), users v0.
	writeFile(t, filepath.Join(dir, "matches.json"), `{"matches": {
		"m1": {"shooter": {"name": "Bert", "birthday": "1.1.1990"}, "type_of_target": "LG", "date": "01.01.2020",
			"shots": [{"ringe": 10.2, "teiler": 1324, "x": 125, "y": -42}], "id": "m1", "club": "c1", "team": "gone"},
		"m2": {"shooter": {"name": "Neu"}, "type_of_target": "LG", "date": "02.01.2020", "shots": [], "id": "m2"},
		"m3": {"shooter": {"name": "Bert"}, "type_of_target": "LP", "date": "03.01.2020", "shots": [], "id": "m3"}}}`)
	writeFile(t, filepath.Join(dir, "users.json"), `{"users": {"u1": {
		"shooter": {"name": "Bert", "birthday": "1.1.1990"}, "niceness": 4, "id": "u1",
		"matches": ["m1"], "competitions": []}}}`)
	writeFile(t, filepath.Join(dir, "clubs.json"), `{"clubs": {"c1": {"name": "SV", "id": "c1"}}}`)
	writeFile(t, filepath.Join(dir, "teams.json"), `{"teams": {}}`)
	writeFile(t, filepath.Join(dir, "leagues.json"), `{"leagues": {"l1": {"league": "Kreisliga", "group": "2",
		"home_club": "c1", "home_team": "", "guest_club": "missing", "guest_team": "", "discipline": "LG", "id": "l1"}}}`)
	writeFile(t, filepath.Join(dir, "competitions.json"), `{"competitions": {"k1": {"name": "Liga", "date": "05.01.2020",
		"count": 1, "shots_per_target": 1, "type_of_target": "LG", "decimal": false, "active": true,
		"modus": "Liga des RSB (Kreis/Bezirk/Landesliga)", "entries": ["m2", "unknown", "m1"], "id": "k1", "league": "l1"}}}`)

	s := open(t, dir)
	m, err := s.Match("m1")
	if err != nil {
		t.Fatal(err)
	}
	want := model.Shot{Ringe: 10.2, Teiler: 132.4, X: 1.25, Y: -0.42}
	if m.Shooter != "u1" || m.Shots[0] != want || m.Club != "c1" || m.Team != "" || m.Date != "01.01.2020" {
		t.Errorf("match = %+v", m)
	}
	entries, _ := s.CompetitionMatches("k1")
	if len(entries) != 2 || entries[0].ID != "m2" || entries[1].ID != "m1" {
		t.Errorf("entry order = %+v", entries)
	}
	if m3, _ := s.Match("m3"); m3.Competition != "" || m3.Shooter != "u1" {
		t.Errorf("quick analysis = %+v", m3)
	}
	users, _ := s.Users()
	if len(users) != 2 || users[0].Name != "Bert" || users[0].Birthday != "1.1.1990" || users[1].Name != "Neu" {
		t.Errorf("users = %+v", users)
	}
	c, _ := s.Competition("k1")
	l, err := s.League(c.League)
	if err != nil || l.HomeClub != "c1" || l.GuestClub != "" || l.Group != "2" {
		t.Errorf("league = %+v, %v", l, err)
	}

	// The legacy files are archived and not imported again.
	for _, f := range legacyFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Errorf("%s not moved", f)
		}
		if _, err := os.Stat(filepath.Join(dir, legacyDir, f)); err != nil {
			t.Errorf("%s not archived: %v", f, err)
		}
	}
	s.Close()
	writeFile(t, filepath.Join(dir, "users.json"), `{"users": {"u9": {"name": "Spät", "id": "u9"}}, "version": 1}`)
	s = open(t, dir)
	if users, _ := s.Users(); len(users) != 2 {
		t.Errorf("imported again: %+v", users)
	}
}

func TestLegacyIni(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "db")
	writeFile(t, filepath.Join(root, legacyIni), "[Neu]\nName = x\n\n[S1]\nName = Carla\nniceness = 3\n")
	s := open(t, dir)
	if users, _ := s.Users(); len(users) != 1 || users[0].Name != "Carla" {
		t.Errorf("users = %+v", users)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyDir, legacyIni)); err != nil {
		t.Errorf("%s not archived: %v", legacyIni, err)
	}
}

// A failed import leaves the legacy files in place and is retried.
func TestLegacyImportFailure(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "users.json"), `{"users": `)
	if s, err := Open(dir); err == nil {
		s.Close()
		t.Fatal("broken legacy file accepted")
	}
	writeFile(t, filepath.Join(dir, "users.json"), `{"users": {"u1": {"name": "Anna", "id": "u1"}}, "version": 1}`)
	s := open(t, dir)
	if users, _ := s.Users(); len(users) != 1 {
		t.Errorf("users = %+v", users)
	}
}

// TestRealDatabase imports a copy of the developer's legacy database if present.
func TestRealDatabase(t *testing.T) {
	src := filepath.Join("..", "..", "db")
	if _, err := os.Stat(filepath.Join(src, "matches.json")); err != nil {
		t.Skip("no legacy ./db present")
	}
	dir := t.TempDir()
	var legacy struct {
		Matches map[string]json.RawMessage
	}
	for _, f := range legacyFiles {
		data, err := os.ReadFile(filepath.Join(src, f))
		if err != nil {
			continue
		}
		if f == "matches.json" {
			json.Unmarshal(data, &legacy)
		}
		writeFile(t, filepath.Join(dir, f), string(data))
	}
	s := open(t, dir)
	users, _ := s.Users()
	counts, _ := s.MatchCounts()
	total := 0
	for _, n := range counts {
		total += n
	}
	active, _ := s.Competitions(true)
	inactive, _ := s.Competitions(false)
	t.Logf("users=%d competitions=%d matches=%d", len(users), len(active)+len(inactive), total)
	if total != len(legacy.Matches) {
		t.Errorf("imported %d of %d matches", total, len(legacy.Matches))
	}
}
