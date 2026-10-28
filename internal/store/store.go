// Package store persists the domain model in an SQLite database
// (<dir>/auswertung.db). Databases of the former Python version (JSON files
// in <dir>) are imported once on first start, see legacy.go.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/janenen/auswertung/internal/model"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrHasEntries = errors.New("Wettkampf hat bereits Beiträge")
	ErrInvalid    = errors.New("ungültige Eingabe")
)

// DBFile is the database file name inside the data directory.
const DBFile = "auswertung.db"

// Store is safe for concurrent use. All methods return copies.
type Store struct {
	db *sql.DB
}

// Open opens (and if necessary creates, upgrades or imports) the database
// in dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// Immediate transactions avoid lock upgrades failing under concurrency.
	dsn := "file:" + filepath.ToSlash(filepath.Join(dir, DBFile)) + "?" + url.Values{
		"_pragma": {"foreign_keys(1)", "journal_mode(WAL)", "busy_timeout(5000)", "synchronous(NORMAL)"},
		"_txlock": {"immediate"},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.init(dir); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// init applies pending migrations. A new database also imports legacy
// data, in the same transaction, so a failed import leaves no half-filled
// database behind; the imported files are archived after the commit.
func (s *Store) init(dir string) error {
	var imported []string
	err := s.tx(func(tx *sql.Tx) error {
		var version int
		if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
			return err
		}
		if version > len(migrations) {
			return fmt.Errorf("database version %d is newer than this program (%d)", version, len(migrations))
		}
		for i := version; i < len(migrations); i++ {
			if _, err := tx.Exec(migrations[i]); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
		}
		if version == 0 {
			var err error
			if imported, err = importLegacy(tx, dir); err != nil {
				return fmt.Errorf("importing legacy database: %w", err)
			}
		}
		_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations)))
		return err
	})
	if err == nil && len(imported) > 0 {
		archiveLegacy(dir, imported)
	}
	return err
}

func (s *Store) tx(fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func newID() string { return uuid.NewString() }

// Dates are ISO 8601 in the database and DD.MM.YYYY in the model.
const (
	isoLayout     = "2006-01-02"
	displayLayout = "02.01.2006"
)

func isoDate(display string) (string, error) {
	t, err := time.Parse(displayLayout, strings.TrimSpace(display))
	if err != nil {
		return "", fmt.Errorf("%w: Datum %q (TT.MM.JJJJ)", ErrInvalid, display)
	}
	return t.Format(isoLayout), nil
}

func displayDate(iso string) string {
	t, err := time.Parse(isoLayout, iso)
	if err != nil {
		return iso
	}
	return t.Format(displayLayout)
}

// null maps an empty optional reference to NULL.
func null(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func notFound(kind, id string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s %q: %w", kind, id, ErrNotFound)
	}
	return err
}

// requireRow turns an UPDATE or DELETE that touched nothing into ErrNotFound.
func requireRow(res sql.Result, kind, id string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s %q: %w", kind, id, ErrNotFound)
	}
	return nil
}

// --- users, clubs, teams ---

// Users returns all users sorted by name.
func (s *Store) Users() ([]model.User, error) {
	rows, err := s.db.Query("SELECT id, name, birthday FROM users ORDER BY name, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []model.User{}
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Birthday); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) User(id string) (model.User, error) {
	u := model.User{ID: id}
	err := s.db.QueryRow("SELECT name, birthday FROM users WHERE id = ?", id).Scan(&u.Name, &u.Birthday)
	return u, notFound("user", id, err)
}

// SaveUser creates a user (empty ID) or updates name and birthday.
func (s *Store) SaveUser(u model.User) (model.User, error) {
	if u.ID == "" {
		u.ID = newID()
		_, err := s.db.Exec("INSERT INTO users (id, name, birthday) VALUES (?, ?, ?)", u.ID, u.Name, u.Birthday)
		return u, err
	}
	res, err := s.db.Exec("UPDATE users SET name = ?, birthday = ? WHERE id = ?", u.Name, u.Birthday, u.ID)
	if err != nil {
		return model.User{}, err
	}
	return u, requireRow(res, "user", u.ID)
}

// MatchCounts returns the number of matches per user ID.
func (s *Store) MatchCounts() (map[string]int, error) {
	return s.counts("SELECT shooter_id, COUNT(*) FROM matches GROUP BY shooter_id")
}

func (s *Store) counts(query string) (map[string]int, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// named are the tables with only ID and name.
type named string

const (
	clubs named = "clubs"
	teams named = "teams"
)

func (s *Store) list(table named) ([]model.Club, error) {
	rows, err := s.db.Query("SELECT id, name FROM " + string(table) + " ORDER BY name, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Club{}
	for rows.Next() {
		var c model.Club
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) get(table named, id string) (model.Club, error) {
	c := model.Club{ID: id}
	err := s.db.QueryRow("SELECT name FROM "+string(table)+" WHERE id = ?", id).Scan(&c.Name)
	return c, notFound(strings.TrimSuffix(string(table), "s"), id, err)
}

func (s *Store) save(table named, c model.Club) (model.Club, error) {
	if c.ID == "" {
		c.ID = newID()
		_, err := s.db.Exec("INSERT INTO "+string(table)+" (id, name) VALUES (?, ?)", c.ID, c.Name)
		return c, err
	}
	res, err := s.db.Exec("UPDATE "+string(table)+" SET name = ? WHERE id = ?", c.Name, c.ID)
	if err != nil {
		return model.Club{}, err
	}
	return c, requireRow(res, strings.TrimSuffix(string(table), "s"), c.ID)
}

func (s *Store) Clubs() ([]model.Club, error)              { return s.list(clubs) }
func (s *Store) Club(id string) (model.Club, error)        { return s.get(clubs, id) }
func (s *Store) SaveClub(c model.Club) (model.Club, error) { return s.save(clubs, c) }

// Teams have the same shape as clubs.

func (s *Store) Teams() ([]model.Team, error) {
	cs, err := s.list(teams)
	ts := make([]model.Team, len(cs))
	for i, c := range cs {
		ts[i] = model.Team(c)
	}
	return ts, err
}

func (s *Store) Team(id string) (model.Team, error) {
	c, err := s.get(teams, id)
	return model.Team(c), err
}

func (s *Store) SaveTeam(t model.Team) (model.Team, error) {
	c, err := s.save(teams, model.Club(t))
	return model.Team(c), err
}

// --- competitions and leagues ---

const competitionColumns = `c.id, c.name, c.date, c.shot_count, c.shots_per_target, c.target_type,
	c.decimal, c.active, c.modus, COALESCE(l.id, '')
	FROM competitions c LEFT JOIN leagues l ON l.competition_id = c.id`

type scanner interface{ Scan(...any) error }

func scanCompetition(row scanner) (model.Competition, error) {
	var c model.Competition
	err := row.Scan(&c.ID, &c.Name, &c.Date, &c.Count, &c.ShotsPerTarget, &c.TypeOfTarget,
		&c.Decimal, &c.Active, &c.Modus, &c.League)
	c.Date = displayDate(c.Date)
	return c, err
}

// Competitions returns the competitions with the given active state, newest first.
func (s *Store) Competitions(active bool) ([]model.Competition, error) {
	rows, err := s.db.Query("SELECT "+competitionColumns+" WHERE c.active = ? ORDER BY c.date DESC, c.name", active)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Competition{}
	for rows.Next() {
		c, err := scanCompetition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Competition(id string) (model.Competition, error) {
	c, err := scanCompetition(s.db.QueryRow("SELECT "+competitionColumns+" WHERE c.id = ?", id))
	return c, notFound("competition", id, err)
}

// EntryCounts returns the number of matches per competition ID.
func (s *Store) EntryCounts() (map[string]int, error) {
	return s.counts("SELECT competition_id, COUNT(*) FROM matches WHERE competition_id IS NOT NULL GROUP BY competition_id")
}

// CreateCompetition stores a new active competition and, for league
// competitions, its league pairing.
func (s *Store) CreateCompetition(c model.Competition, league *model.RSBLeague) (model.Competition, error) {
	date, err := isoDate(c.Date)
	if err != nil {
		return model.Competition{}, err
	}
	c.ID, c.Active, c.League = newID(), true, ""
	err = s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO competitions
			(id, name, date, shot_count, shots_per_target, target_type, decimal, active, modus)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`,
			c.ID, c.Name, date, c.Count, c.ShotsPerTarget, c.TypeOfTarget, c.Decimal, c.Modus); err != nil {
			return err
		}
		if league == nil {
			return nil
		}
		c.League = newID()
		return insertLeague(tx, c.League, c.ID, *league)
	})
	if err != nil {
		return model.Competition{}, err
	}
	return c, nil
}

func insertLeague(tx *sql.Tx, id, competition string, l model.RSBLeague) error {
	_, err := tx.Exec(`INSERT INTO leagues
		(id, competition_id, discipline, league, group_name, home_club, home_team, guest_club, guest_team)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, competition, l.Discipline, l.League, l.Group, null(l.HomeClub), null(l.HomeTeam), null(l.GuestClub), null(l.GuestTeam))
	return err
}

func (s *Store) League(id string) (model.RSBLeague, error) {
	l := model.RSBLeague{ID: id}
	err := s.db.QueryRow(`SELECT discipline, league, group_name, COALESCE(home_club, ''), COALESCE(home_team, ''),
		COALESCE(guest_club, ''), COALESCE(guest_team, '') FROM leagues WHERE id = ?`, id).
		Scan(&l.Discipline, &l.League, &l.Group, &l.HomeClub, &l.HomeTeam, &l.GuestClub, &l.GuestTeam)
	return l, notFound("league", id, err)
}

func (s *Store) SetCompetitionActive(id string, active bool) error {
	res, err := s.db.Exec("UPDATE competitions SET active = ? WHERE id = ?", active, id)
	if err != nil {
		return err
	}
	return requireRow(res, "competition", id)
}

// DeleteCompetition removes a competition without entries and its league.
func (s *Store) DeleteCompetition(id string) error {
	return s.tx(func(tx *sql.Tx) error {
		var entries int
		if err := tx.QueryRow("SELECT COUNT(*) FROM matches WHERE competition_id = ?", id).Scan(&entries); err != nil {
			return err
		}
		if entries > 0 {
			return ErrHasEntries
		}
		res, err := tx.Exec("DELETE FROM competitions WHERE id = ?", id)
		if err != nil {
			return err
		}
		return requireRow(res, "competition", id)
	})
}

// --- matches ---

const matchColumns = `id, shooter_id, COALESCE(club_id, ''), COALESCE(team_id, ''), COALESCE(competition_id, ''),
	target_type, date, active, title, decimal FROM matches`

// matches loads the matches selected by where (with args) in recording
// order, including their shots.
func (s *Store) matches(where string, args ...any) ([]model.Match, error) {
	rows, err := s.db.Query("SELECT "+matchColumns+" WHERE "+where+" ORDER BY seq", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Match{}
	index := map[string]int{}
	for rows.Next() {
		m := model.Match{Shots: model.Shots{}}
		if err := rows.Scan(&m.ID, &m.Shooter, &m.Club, &m.Team, &m.Competition, &m.TypeOfTarget, &m.Date, &m.Active,
			&m.Title, &m.Decimal); err != nil {
			return nil, err
		}
		m.Date = displayDate(m.Date)
		index[m.ID] = len(out)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil || len(out) == 0 {
		return out, err
	}
	shots, err := s.db.Query(`SELECT match_id, ringe, teiler, x, y FROM shots
		WHERE match_id IN (SELECT id FROM matches WHERE `+where+`) ORDER BY match_id, idx`, args...)
	if err != nil {
		return nil, err
	}
	defer shots.Close()
	for shots.Next() {
		var id string
		var shot model.Shot
		if err := shots.Scan(&id, &shot.Ringe, &shot.Teiler, &shot.X, &shot.Y); err != nil {
			return nil, err
		}
		m := &out[index[id]]
		m.Shots = append(m.Shots, shot)
	}
	return out, shots.Err()
}

func (s *Store) Match(id string) (model.Match, error) {
	ms, err := s.matches("id = ?", id)
	if err != nil {
		return model.Match{}, err
	}
	if len(ms) == 0 {
		return model.Match{}, fmt.Errorf("match %q: %w", id, ErrNotFound)
	}
	return ms[0], nil
}

// CompetitionMatches returns the entries of a competition in recording order.
func (s *Store) CompetitionMatches(competition string) ([]model.Match, error) {
	return s.matches("competition_id = ?", competition)
}

// UserMatches returns all matches of a shooter in recording order.
func (s *Store) UserMatches(user string) ([]model.Match, error) {
	return s.matches("shooter_id = ?", user)
}

// NewMatch describes the result of a reading that starts a new match.
type NewMatch struct {
	Shooter, Club, Team, TypeOfTarget string
	// Competition is empty for a quick analysis, which has a Title and
	// Decimal scoring instead.
	Competition string
	Title       string
	Decimal     bool
	// Count is the number of shots after which the match is complete.
	Count int
	Shots model.Shots
}

// RecordNewMatch stores a newly read match dated today.
func (s *Store) RecordNewMatch(n NewMatch) (model.Match, error) {
	now := time.Now()
	m := model.Match{
		ID: newID(), Shooter: n.Shooter, Club: n.Club, Team: n.Team, Competition: n.Competition,
		TypeOfTarget: n.TypeOfTarget, Date: now.Format(displayLayout), Active: len(n.Shots) < n.Count,
		Title: n.Title, Decimal: n.Decimal, Shots: append(model.Shots{}, n.Shots...),
	}
	err := s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO matches
			(id, shooter_id, competition_id, club_id, team_id, target_type, date, active, title, decimal)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.Shooter, null(m.Competition), null(m.Club), null(m.Team), m.TypeOfTarget, now.Format(isoLayout), m.Active,
			m.Title, m.Decimal); err != nil {
			return referenceError(err, n)
		}
		return insertShots(tx, m.ID, 0, m.Shots)
	})
	if err != nil {
		return model.Match{}, err
	}
	return m, nil
}

// referenceError reports a violated foreign key as the missing user or
// competition, like a lookup would.
func referenceError(err error, n NewMatch) error {
	if !strings.Contains(err.Error(), "FOREIGN KEY") {
		return err
	}
	return fmt.Errorf("user %q, competition %q, club %q or team %q: %w", n.Shooter, n.Competition, n.Club, n.Team, ErrNotFound)
}

func insertShots(tx *sql.Tx, match string, first int, shots model.Shots) error {
	stmt, err := tx.Prepare("INSERT INTO shots (match_id, idx, ringe, teiler, x, y) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, shot := range shots {
		if _, err := stmt.Exec(match, first+i, shot.Ringe, shot.Teiler, shot.X, shot.Y); err != nil {
			return err
		}
	}
	return nil
}

// AppendShots continues an incomplete match.
func (s *Store) AppendShots(matchID string, shots model.Shots, count int) (model.Match, error) {
	err := s.tx(func(tx *sql.Tx) error {
		var have int
		if err := tx.QueryRow("SELECT COUNT(*) FROM shots WHERE match_id = ?", matchID).Scan(&have); err != nil {
			return err
		}
		res, err := tx.Exec("UPDATE matches SET active = ? WHERE id = ?", have+len(shots) < count, matchID)
		if err != nil {
			return err
		}
		if err := requireRow(res, "match", matchID); err != nil {
			return err
		}
		return insertShots(tx, matchID, have, shots)
	})
	if err != nil {
		return model.Match{}, err
	}
	return s.Match(matchID)
}
