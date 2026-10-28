// Package reading runs machine readings in the background and buffers their
// progress as events that clients can follow.
package reading

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/model"
)

type Event struct {
	Type       string      `json:"type"` // status, shot, error, done
	Message    string      `json:"message,omitempty"`
	Shot       *model.Shot `json:"shot,omitempty"`
	TargetType string      `json:"targetType,omitempty"`
	// MatchID is set on the done event if a match was saved.
	MatchID string `json:"matchId,omitempty"`
}

// FinishFunc persists the shots of a reading and returns the match ID.
type FinishFunc func(shots model.Shots, targetType string) (string, error)

// Tag identifies what a reading belongs to, for showing it live.
type Tag struct {
	CompetitionID string
	// MatchID is set when an existing match is continued.
	MatchID   string
	ShooterID string
}

type Params struct {
	Machine machine.Machine
	Request machine.Request
	// ExpectedTarget aborts the reading if the machine reports another target type.
	ExpectedTarget string
	Finish         FinishFunc
	Tag            Tag
}

type Session struct {
	ID        string
	MachineID string
	Tag       Tag

	cancel context.CancelFunc

	mu         sync.Mutex
	events     []Event
	shots      model.Shots
	targetType string
	changed    chan struct{}
	done       bool
	finished   time.Time
	keep       bool
}

// Live is a snapshot of a running reading.
type Live struct {
	Tag        Tag
	TargetType string
	Shots      model.Shots
}

// Events returns the events from index from on, whether the session is
// done, and a channel that is closed on the next change.
func (s *Session) Events(from int) ([]Event, bool, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var events []Event
	if from < len(s.events) {
		events = append(events, s.events[from:]...)
	}
	return events, s.done, s.changed
}

// Stop ends the reading. With keep the shots read so far are saved.
func (s *Session) Stop(keep bool) {
	s.mu.Lock()
	s.keep = keep
	s.mu.Unlock()
	s.cancel()
}

func (s *Session) emit(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	if e.Type == "shot" {
		s.shots = append(s.shots, *e.Shot)
		s.targetType = e.TargetType
	}
	if e.Type == "done" {
		s.done = true
		s.finished = time.Now()
	}
	close(s.changed)
	s.changed = make(chan struct{})
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewManager() *Manager { return &Manager{sessions: map[string]*Session{}} }

// Live returns the readings that are still running.
func (m *Manager) Live() []Live {
	m.mu.Lock()
	defer m.mu.Unlock()
	var live []Live
	for _, s := range m.sessions {
		s.mu.Lock()
		if !s.done {
			live = append(live, Live{Tag: s.Tag, TargetType: s.targetType, Shots: slices.Clone(s.shots)})
		}
		s.mu.Unlock()
	}
	return live
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Start begins a reading. Only one reading per machine may run at a time.
func (m *Manager) Start(p Params) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.mu.Lock()
		done, finished := s.done, s.finished
		s.mu.Unlock()
		if !done && s.MachineID == p.Machine.ID() {
			return nil, fmt.Errorf("%s liest bereits ein", p.Machine.Name())
		}
		if done && time.Since(finished) > 10*time.Minute {
			delete(m.sessions, id)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ID: uuid.NewString(), MachineID: p.Machine.ID(), Tag: p.Tag, cancel: cancel, changed: make(chan struct{})}
	m.sessions[s.ID] = s
	go s.run(ctx, p)
	return s, nil
}

var errWrongTarget = errors.New("wrong target")

func (s *Session) run(ctx context.Context, p Params) {
	defer s.cancel()
	s.emit(Event{Type: "status", Message: "Einlesen gestartet – Scheiben eingeben!"})

	out := make(chan machine.Reading)
	readErr := make(chan error, 1)
	go func() {
		readErr <- p.Machine.Read(ctx, p.Request, out)
		close(out)
	}()

	var shots model.Shots
	targetType := p.ExpectedTarget
	var abort error
	for r := range out {
		if abort != nil {
			continue
		}
		if p.ExpectedTarget != "" && r.TargetType != p.ExpectedTarget {
			abort = errWrongTarget
			s.emit(Event{Type: "error", Message: fmt.Sprintf("Falscher Scheibentyp: %s statt %s", r.TargetType, p.ExpectedTarget)})
			s.cancel()
			continue
		}
		targetType = r.TargetType
		shot := r.Shot
		shots = append(shots, shot)
		s.emit(Event{Type: "shot", Shot: &shot, TargetType: r.TargetType})
	}
	err := <-readErr

	s.mu.Lock()
	keep := s.keep
	s.mu.Unlock()
	switch {
	case abort != nil:
		s.emit(Event{Type: "done"})
		return
	case errors.Is(err, context.Canceled) && !keep:
		s.emit(Event{Type: "status", Message: "Einlesen abgebrochen"})
		s.emit(Event{Type: "done"})
		return
	case err != nil && !errors.Is(err, context.Canceled):
		s.emit(Event{Type: "error", Message: err.Error()})
	}
	if len(shots) == 0 {
		s.emit(Event{Type: "status", Message: "Keine Schüsse eingelesen"})
		s.emit(Event{Type: "done"})
		return
	}
	matchID, err := p.Finish(shots, targetType)
	if err != nil {
		s.emit(Event{Type: "error", Message: "Speichern fehlgeschlagen: " + err.Error()})
		s.emit(Event{Type: "done"})
		return
	}
	s.emit(Event{Type: "status", Message: "Einlesen abgeschlossen"})
	s.emit(Event{Type: "done", MatchID: matchID})
}
