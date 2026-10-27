// Package store persists the domain model in JSON files (db/*.json) using the
// same layout as the former Python implementation.
package store

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/janenen/auswertung/internal/model"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrHasEntries = errors.New("Wettkampf hat bereits Beiträge")
)

const (
	matchDBVersion = 2
	userDBVersion  = 1
)

// collection describes one JSON file: {"<key>": {id: item}, "version": n}.
type collection[T any] struct {
	file    string
	key     string
	version *int
	items   map[string]*T
}

func (c *collection[T]) load(dir string) error {
	c.items = map[string]*T{}
	data, err := os.ReadFile(filepath.Join(dir, c.file))
	if errors.Is(err, os.ErrNotExist) {
		return c.save(dir)
	}
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("%s: %w", c.file, err)
	}
	if items, ok := raw[c.key]; ok && string(items) != "null" {
		if err := json.Unmarshal(items, &c.items); err != nil {
			return fmt.Errorf("%s: %w", c.file, err)
		}
	}
	return nil
}

func (c *collection[T]) save(dir string) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{c.key: c.items, "version": c.version}); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, c.file), buf.Bytes())
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func intPtr(v int) *int { return &v }

// Store holds all data in memory and writes changes through to disk.
// All returned values are copies; mutate only through Store methods.
type Store struct {
	mu           sync.RWMutex
	dir          string
	users        collection[model.User]
	matches      collection[model.Match]
	competitions collection[model.Competition]
	clubs        collection[model.Club]
	teams        collection[model.Team]
	leagues      collection[model.RSBLeague]
}

// Open loads (and if necessary migrates) the database in dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := migrate(dir); err != nil {
		return nil, fmt.Errorf("migration: %w", err)
	}
	s := &Store{
		dir:          dir,
		users:        collection[model.User]{file: "users.json", key: "users", version: intPtr(userDBVersion)},
		matches:      collection[model.Match]{file: "matches.json", key: "matches", version: intPtr(matchDBVersion)},
		competitions: collection[model.Competition]{file: "competitions.json", key: "competitions"},
		clubs:        collection[model.Club]{file: "clubs.json", key: "clubs"},
		teams:        collection[model.Team]{file: "teams.json", key: "teams"},
		leagues:      collection[model.RSBLeague]{file: "leagues.json", key: "leagues"},
	}
	for _, load := range []func(string) error{
		s.users.load, s.matches.load, s.competitions.load,
		s.clubs.load, s.teams.load, s.leagues.load,
	} {
		if err := load(dir); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func newID() string { return uuid.NewString() }

func get[T any](c *collection[T], id string) (T, error) {
	item, ok := c.items[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("%s %q: %w", c.key, id, ErrNotFound)
	}
	return *item, nil
}

func values[T any](c *collection[T]) []T {
	out := make([]T, 0, len(c.items))
	for _, item := range c.items {
		out = append(out, *item)
	}
	return out
}

// Users returns all users, most frequent shooters first.
func (s *Store) Users() []model.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := values(&s.users)
	slices.SortFunc(users, func(a, b model.User) int {
		return cmp.Or(cmp.Compare(b.Niceness, a.Niceness), cmp.Compare(a.Name, b.Name))
	})
	return users
}

func (s *Store) User(id string) (model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return get(&s.users, id)
}

// SaveUser creates a user (empty ID) or updates name and birthday.
func (s *Store) SaveUser(u model.User) (model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.ID == "" {
		u = model.User{Name: u.Name, Birthday: u.Birthday, ID: newID(), Matches: []string{}, Competitions: []string{}}
		s.users.items[u.ID] = &u
	} else {
		existing, ok := s.users.items[u.ID]
		if !ok {
			return model.User{}, fmt.Errorf("user %q: %w", u.ID, ErrNotFound)
		}
		existing.Name, existing.Birthday = u.Name, u.Birthday
		u = *existing
	}
	return u, s.users.save(s.dir)
}

func sortedByName[T any](items []T, name func(T) string) []T {
	slices.SortFunc(items, func(a, b T) int { return cmp.Compare(name(a), name(b)) })
	return items
}

func (s *Store) Clubs() []model.Club {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedByName(values(&s.clubs), func(c model.Club) string { return c.Name })
}

func (s *Store) Club(id string) (model.Club, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return get(&s.clubs, id)
}

func (s *Store) SaveClub(c model.Club) (model.Club, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.ID == "" {
		c.ID = newID()
	} else if _, ok := s.clubs.items[c.ID]; !ok {
		return model.Club{}, fmt.Errorf("club %q: %w", c.ID, ErrNotFound)
	}
	s.clubs.items[c.ID] = &c
	return c, s.clubs.save(s.dir)
}

func (s *Store) Teams() []model.Team {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedByName(values(&s.teams), func(t model.Team) string { return t.Name })
}

func (s *Store) Team(id string) (model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return get(&s.teams, id)
}

func (s *Store) SaveTeam(t model.Team) (model.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.ID == "" {
		t.ID = newID()
	} else if _, ok := s.teams.items[t.ID]; !ok {
		return model.Team{}, fmt.Errorf("team %q: %w", t.ID, ErrNotFound)
	}
	s.teams.items[t.ID] = &t
	return t, s.teams.save(s.dir)
}

func (s *Store) League(id string) (model.RSBLeague, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return get(&s.leagues, id)
}

// Competitions returns the competitions with the given active state, newest first.
func (s *Store) Competitions(active bool) []model.Competition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.Competition
	for _, c := range s.competitions.items {
		if c.Active == active {
			out = append(out, cloneCompetition(*c))
		}
	}
	slices.SortFunc(out, func(a, b model.Competition) int {
		return cmp.Or(cmp.Compare(parseDate(b.Date), parseDate(a.Date)), cmp.Compare(a.Name, b.Name))
	})
	return out
}

func parseDate(d string) int64 {
	t, err := time.Parse("02.01.2006", d)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func cloneCompetition(c model.Competition) model.Competition {
	c.Entries = slices.Clone(c.Entries)
	return c
}

func (s *Store) Competition(id string) (model.Competition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := get(&s.competitions, id)
	return cloneCompetition(c), err
}

// CreateCompetition stores a new active competition and, for league
// competitions, its league pairing.
func (s *Store) CreateCompetition(c model.Competition, league *model.RSBLeague) (model.Competition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = newID()
	c.Active = true
	c.Entries = []string{}
	if league != nil {
		l := *league
		l.ID = newID()
		s.leagues.items[l.ID] = &l
		c.League = l.ID
		if err := s.leagues.save(s.dir); err != nil {
			return model.Competition{}, err
		}
	}
	s.competitions.items[c.ID] = &c
	return cloneCompetition(c), s.competitions.save(s.dir)
}

func (s *Store) SetCompetitionActive(id string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.competitions.items[id]
	if !ok {
		return fmt.Errorf("competition %q: %w", id, ErrNotFound)
	}
	c.Active = active
	return s.competitions.save(s.dir)
}

// DeleteCompetition removes a competition without entries and its league.
func (s *Store) DeleteCompetition(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.competitions.items[id]
	if !ok {
		return fmt.Errorf("competition %q: %w", id, ErrNotFound)
	}
	if len(c.Entries) > 0 {
		return ErrHasEntries
	}
	if c.League != "" {
		delete(s.leagues.items, c.League)
		if err := s.leagues.save(s.dir); err != nil {
			return err
		}
	}
	delete(s.competitions.items, id)
	return s.competitions.save(s.dir)
}

func cloneMatch(m model.Match) model.Match {
	m.Shots = slices.Clone(m.Shots)
	m.Competitions = slices.Clone(m.Competitions)
	return m
}

func (s *Store) Match(id string) (model.Match, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, err := get(&s.matches, id)
	return cloneMatch(m), err
}

// Matches returns the matches with the given IDs, skipping unknown IDs.
func (s *Store) Matches(ids []string) []model.Match {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Match, 0, len(ids))
	for _, id := range ids {
		if m, ok := s.matches.items[id]; ok {
			out = append(out, cloneMatch(*m))
		}
	}
	return out
}

// NewMatch describes the result of a reading that starts a new match.
type NewMatch struct {
	Shooter, Club, Team, TypeOfTarget string
	// Competition is empty for a quick analysis.
	Competition string
	// Count is the number of shots after which the match is complete.
	Count int
	Shots model.Shots
}

// RecordNewMatch stores a newly read match and links it to user and competition.
func (s *Store) RecordNewMatch(n NewMatch) (model.Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users.items[n.Shooter]
	if !ok {
		return model.Match{}, fmt.Errorf("user %q: %w", n.Shooter, ErrNotFound)
	}
	var comp *model.Competition
	if n.Competition != "" {
		if comp, ok = s.competitions.items[n.Competition]; !ok {
			return model.Match{}, fmt.Errorf("competition %q: %w", n.Competition, ErrNotFound)
		}
	}
	m := &model.Match{
		Shooter:      n.Shooter,
		TypeOfTarget: n.TypeOfTarget,
		Date:         time.Now().Format("02.01.2006"),
		Shots:        slices.Clone(n.Shots),
		ID:           newID(),
		Active:       len(n.Shots) < n.Count,
		Club:         n.Club,
		Team:         n.Team,
		Competitions: []string{},
	}
	s.matches.items[m.ID] = m
	user.AddMatch(m.ID)
	if comp != nil {
		user.Niceness++
		user.AddCompetition(comp.ID)
		m.AddCompetition(comp.ID)
		comp.AddMatch(m.ID)
	}
	return cloneMatch(*m), errors.Join(s.users.save(s.dir), s.matches.save(s.dir), s.competitions.save(s.dir))
}

// AppendShots continues an incomplete match.
func (s *Store) AppendShots(matchID string, shots model.Shots, count int) (model.Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.matches.items[matchID]
	if !ok {
		return model.Match{}, fmt.Errorf("match %q: %w", matchID, ErrNotFound)
	}
	m.Shots = append(m.Shots, shots...)
	m.Active = len(m.Shots) < count
	return cloneMatch(*m), s.matches.save(s.dir)
}
