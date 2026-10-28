package store

import (
	"bufio"
	"bytes"
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/janenen/auswertung/internal/model"
)

// The former Python version stored each entity type in <dir>/<name>.json
// as {"<name>": {id: item}, "version": n}, and shooters before that in
// ../schuetzen.ini. A new database imports these files once; afterwards
// they are moved to <dir>/legacy.

var legacyFiles = []string{"users.json", "matches.json", "competitions.json", "clubs.json", "teams.json", "leagues.json"}

const (
	legacyDir = "legacy"
	legacyIni = "schuetzen.ini"
)

type legacyMatch struct {
	ID           string      `json:"id"`
	Shooter      string      `json:"shooter"`
	TypeOfTarget string      `json:"type_of_target"`
	Date         string      `json:"date"`
	Shots        model.Shots `json:"shots"`
	Active       bool        `json:"active"`
	Club         string      `json:"club"`
	Team         string      `json:"team"`
	Competitions []string    `json:"competitions"`
}

type legacyCompetition struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Date           string   `json:"date"`
	Count          int      `json:"count"`
	ShotsPerTarget int      `json:"shots_per_target"`
	TypeOfTarget   string   `json:"type_of_target"`
	Decimal        bool     `json:"decimal"`
	Active         bool     `json:"active"`
	Modus          string   `json:"modus"`
	Entries        []string `json:"entries"`
	League         string   `json:"league"`
}

type legacyDB struct {
	users        map[string]model.User
	matches      map[string]legacyMatch
	competitions map[string]legacyCompetition
	clubs        map[string]model.Club
	teams        map[string]model.Club
	leagues      map[string]model.RSBLeague
}

// importLegacy fills a new database from the legacy files of dir and
// returns the files to archive once the transaction is committed. Broken
// references are dropped with a log message instead of failing the import.
func importLegacy(tx *sql.Tx, dir string) ([]string, error) {
	db, files, err := readLegacy(dir)
	if err != nil || len(files) == 0 {
		return nil, err
	}
	known := func(kind string, items map[string]model.Club) func(string) string {
		return func(id string) string {
			if _, ok := items[id]; id != "" && !ok {
				log.Printf("legacy import: dropping unknown %s %s", kind, id)
				return ""
			}
			return id
		}
	}
	club, team := known("club", db.clubs), known("team", db.teams)

	for _, id := range slices.Sorted(maps.Keys(db.users)) {
		u := db.users[id]
		if _, err := tx.Exec("INSERT INTO users (id, name, birthday) VALUES (?, ?, ?)", id, u.Name, u.Birthday); err != nil {
			return nil, fmt.Errorf("user %s: %w", id, err)
		}
	}
	for _, t := range []struct {
		table string
		items map[string]model.Club
	}{{"clubs", db.clubs}, {"teams", db.teams}} {
		for _, id := range slices.Sorted(maps.Keys(t.items)) {
			if _, err := tx.Exec("INSERT INTO "+t.table+" (id, name) VALUES (?, ?)", id, t.items[id].Name); err != nil {
				return nil, fmt.Errorf("%s %s: %w", t.table, id, err)
			}
		}
	}

	// Competitions oldest first, each with its league.
	comps := slices.SortedFunc(maps.Values(db.competitions), func(a, b legacyCompetition) int {
		return cmp.Or(cmp.Compare(legacyDate(a.Date), legacyDate(b.Date)), cmp.Compare(a.ID, b.ID))
	})
	usedLeagues := map[string]bool{}
	for _, c := range comps {
		if _, err := tx.Exec(`INSERT INTO competitions
			(id, name, date, shot_count, shots_per_target, target_type, decimal, active, modus)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, c.Name, legacyDate(c.Date), c.Count, c.ShotsPerTarget, c.TypeOfTarget, c.Decimal, c.Active, c.Modus); err != nil {
			return nil, fmt.Errorf("competition %s: %w", c.ID, err)
		}
		if c.League == "" {
			continue
		}
		l, ok := db.leagues[c.League]
		if !ok || usedLeagues[c.League] {
			log.Printf("legacy import: competition %s: dropping missing or shared league %s", c.ID, c.League)
			continue
		}
		usedLeagues[c.League] = true
		l.HomeClub, l.GuestClub, l.HomeTeam, l.GuestTeam = club(l.HomeClub), club(l.GuestClub), team(l.HomeTeam), team(l.GuestTeam)
		if err := insertLeague(tx, c.League, c.ID, l); err != nil {
			return nil, fmt.Errorf("league %s: %w", c.League, err)
		}
	}

	// Matches in the entry order of their competition, then the others by
	// date. A match belongs to the first competition listing it.
	competitionOf := map[string]string{}
	var order []string
	for _, c := range comps {
		for _, id := range c.Entries {
			if _, ok := db.matches[id]; ok && competitionOf[id] == "" {
				competitionOf[id] = c.ID
				order = append(order, id)
			}
		}
	}
	others := slices.SortedFunc(maps.Values(db.matches), func(a, b legacyMatch) int {
		return cmp.Or(cmp.Compare(legacyDate(a.Date), legacyDate(b.Date)), cmp.Compare(a.ID, b.ID))
	})
	for _, m := range others {
		if competitionOf[m.ID] != "" {
			continue
		}
		for _, c := range m.Competitions {
			if _, ok := db.competitions[c]; ok {
				competitionOf[m.ID] = c
				break
			}
		}
		order = append(order, m.ID)
	}
	for _, id := range order {
		m := db.matches[id]
		if _, ok := db.users[m.Shooter]; !ok {
			if m.Shooter == "" {
				m.Shooter = uuid.NewString()
			}
			log.Printf("legacy import: match %s: creating missing shooter %s", m.ID, m.Shooter)
			if _, err := tx.Exec("INSERT INTO users (id, name) VALUES (?, ?)", m.Shooter, "Unbekannter Schütze"); err != nil {
				return nil, err
			}
			db.users[m.Shooter] = model.User{ID: m.Shooter}
		}
		if _, err := tx.Exec(`INSERT INTO matches
			(id, shooter_id, competition_id, club_id, team_id, target_type, date, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.Shooter, null(competitionOf[m.ID]), null(club(m.Club)), null(team(m.Team)),
			m.TypeOfTarget, legacyDate(m.Date), m.Active); err != nil {
			return nil, fmt.Errorf("match %s: %w", m.ID, err)
		}
		if err := insertShots(tx, m.ID, 0, m.Shots); err != nil {
			return nil, fmt.Errorf("match %s: %w", m.ID, err)
		}
	}
	log.Printf("imported legacy database: %d users, %d competitions, %d matches, %d clubs, %d teams",
		len(db.users), len(db.competitions), len(db.matches), len(db.clubs), len(db.teams))
	return files, nil
}

// legacyDate converts DD.MM.YYYY to ISO; other values are kept as they are.
func legacyDate(d string) string {
	if iso, err := isoDate(d); err == nil {
		return iso
	}
	return d
}

// archiveLegacy moves the imported files to <dir>/legacy.
func archiveLegacy(dir string, files []string) {
	target := filepath.Join(dir, legacyDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		log.Printf("archiving legacy files: %v", err)
		return
	}
	for _, f := range files {
		if err := os.Rename(f, filepath.Join(target, filepath.Base(f))); err != nil {
			log.Printf("archiving legacy files: %v", err)
		}
	}
	log.Printf("legacy files moved to %s", target)
}

// readLegacy reads the legacy files of dir, upgrading old layouts in memory.
// files lists the files found.
func readLegacy(dir string) (legacyDB, []string, error) {
	db := legacyDB{
		users: map[string]model.User{}, matches: map[string]legacyMatch{}, competitions: map[string]legacyCompetition{},
		clubs: map[string]model.Club{}, teams: map[string]model.Club{}, leagues: map[string]model.RSBLeague{},
	}
	raw := map[string]rawDB{}
	var files []string
	for _, name := range legacyFiles {
		path := filepath.Join(dir, name)
		r, err := readRaw(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return db, nil, err
		}
		raw[name] = r
		files = append(files, path)
	}
	iniPath := filepath.Join(filepath.Dir(filepath.Clean(dir)), legacyIni)
	if raw["users.json"] == nil {
		users, err := readIni(iniPath)
		if err != nil {
			return db, nil, err
		}
		if users != nil {
			raw["users.json"] = users
			files = append(files, iniPath)
		}
	}
	if len(files) == 0 {
		return db, nil, nil
	}
	if raw["users.json"] == nil {
		raw["users.json"] = rawDB{"users": map[string]any{}}
	}
	upgradeUsers(raw["users.json"])
	if m := raw["matches.json"]; m != nil {
		upgradeMatches(m, raw["users.json"])
	}
	for _, d := range []struct {
		file, key string
		into      any
	}{
		{"users.json", "users", &db.users},
		{"matches.json", "matches", &db.matches},
		{"competitions.json", "competitions", &db.competitions},
		{"clubs.json", "clubs", &db.clubs},
		{"teams.json", "teams", &db.teams},
		{"leagues.json", "leagues", &db.leagues},
	} {
		items, ok := raw[d.file][d.key]
		if !ok || items == nil {
			continue
		}
		// Re-encode the (upgraded) raw items into the typed maps.
		data, err := json.Marshal(items)
		if err != nil {
			return db, nil, err
		}
		if err := json.Unmarshal(data, d.into); err != nil {
			return db, nil, fmt.Errorf("%s: %w", d.file, err)
		}
	}
	// The map keys are the IDs; older files did not always repeat them.
	setIDs(db.users, func(u *model.User, id string) { u.ID = id })
	setIDs(db.matches, func(m *legacyMatch, id string) { m.ID = id })
	setIDs(db.competitions, func(c *legacyCompetition, id string) { c.ID = id })
	setIDs(db.clubs, func(c *model.Club, id string) { c.ID = id })
	setIDs(db.teams, func(c *model.Club, id string) { c.ID = id })
	setIDs(db.leagues, func(l *model.RSBLeague, id string) { l.ID = id })
	return db, files, nil
}

func setIDs[T any](items map[string]T, set func(*T, string)) {
	for id, item := range items {
		set(&item, id)
		items[id] = item
	}
}

type rawDB = map[string]any

func readRaw(path string) (rawDB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var db rawDB
	if err := dec.Decode(&db); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return db, nil
}

// version is the layout version of a legacy file; files without one are v0.
func version(db rawDB) int {
	v, _ := db["version"].(json.Number)
	n, _ := v.Int64()
	return int(n)
}

func object(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// upgradeUsers converts users v0, which wrapped name and birthday in a
// "shooter" object and keyed users by name.
func upgradeUsers(users rawDB) {
	converted := map[string]any{}
	for key, u := range object(users["users"]) {
		user := object(u)
		if shooter, ok := user["shooter"]; ok {
			user["name"] = str(object(shooter)["name"])
			user["birthday"] = str(object(shooter)["birthday"])
			delete(user, "shooter")
		}
		if str(user["id"]) == "" {
			user["id"] = key
		}
		converted[str(user["id"])] = user
	}
	users["users"] = converted
}

// upgradeMatches converts matches v0 (Teiler in 1/10, coordinates in 1/100
// as integers) and v1 (shooter embedded instead of referenced by ID).
func upgradeMatches(matches, users rawDB) {
	if version(matches) == 0 {
		for _, m := range object(matches["matches"]) {
			shots, _ := object(m)["shots"].([]any)
			for _, s := range shots {
				shot := object(s)
				teiler, ok := shot["teiler"].(json.Number)
				if !ok || strings.ContainsAny(string(teiler), ".eE") {
					continue
				}
				for key, div := range map[string]float64{"teiler": 10, "x": 100, "y": 100} {
					if n, ok := shot[key].(json.Number); ok {
						f, _ := n.Float64()
						shot[key] = f / div
					}
				}
			}
		}
		matches["version"] = json.Number("1")
	}
	if version(matches) == 1 {
		userMap := object(users["users"])
		users["users"] = userMap
		for _, m := range object(matches["matches"]) {
			match := object(m)
			shooter := object(match["shooter"])
			name := str(shooter["name"])
			id := ""
			for uid, u := range userMap {
				if str(object(u)["name"]) == name {
					id = uid
					break
				}
			}
			if id == "" {
				id = uuid.NewString()
				userMap[id] = map[string]any{"name": name, "birthday": str(shooter["birthday"]), "id": id}
			}
			match["shooter"] = id
		}
		matches["version"] = json.Number("2")
	}
}

// readIni reads the shooters of the legacy schuetzen.ini; nil if missing.
func readIni(path string) (rawDB, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	users := map[string]any{}
	section, name := "", ""
	flush := func() {
		if section != "" && section != "Neu" && section != "NeuerSchütze" && name != "" {
			id := uuid.NewString()
			users[id] = map[string]any{"name": name, "birthday": "", "id": id}
		}
		name = ""
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			flush()
			section = line[1 : len(line)-1]
		default:
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				key, value, _ = strings.Cut(line, ":")
			}
			if strings.EqualFold(strings.TrimSpace(key), "name") {
				name = strings.TrimSpace(value)
			}
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rawDB{"users": users}, nil
}
