package reading

import (
	"context"
	"testing"
	"time"

	"github.com/janenen/auswertung/internal/machine"
	"github.com/janenen/auswertung/internal/model"
)

// slowMachine emits shots of the given target type until cancelled.
type slowMachine struct {
	target string
	n      int
}

func (slowMachine) ID() string     { return "slow" }
func (slowMachine) Name() string   { return "slow" }
func (slowMachine) Input() string  { return "" }
func (slowMachine) Accept() string { return "" }
func (m slowMachine) Read(ctx context.Context, req machine.Request, out chan<- machine.Reading) error {
	for i := 0; i < m.n; i++ {
		select {
		case out <- machine.Reading{Shot: model.Shot{Ringe: 10}, TargetType: m.target}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	<-ctx.Done()
	return ctx.Err()
}

func waitDone(t *testing.T, s *Session) []Event {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		events, done, changed := s.Events(0)
		if done {
			return events
		}
		select {
		case <-changed:
		case <-timeout:
			t.Fatal("session did not finish")
		}
	}
}

func waitShots(t *testing.T, s *Session, n int) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		events, _, changed := s.Events(0)
		shots := 0
		for _, e := range events {
			if e.Type == "shot" {
				shots++
			}
		}
		if shots >= n {
			return
		}
		select {
		case <-changed:
		case <-timeout:
			t.Fatal("shots not received")
		}
	}
}

func TestStopKeep(t *testing.T) {
	var saved model.Shots
	m := NewManager()
	s, err := m.Start(Params{Machine: slowMachine{"LG", 2}, ExpectedTarget: "LG", Tag: Tag{CompetitionID: "c1"},
		Finish: func(shots model.Shots, _ string) (string, error) { saved = shots; return "m1", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(Params{Machine: slowMachine{"LG", 0}}); err == nil {
		t.Error("second reading on the same machine should fail")
	}
	waitShots(t, s, 2)
	if live := m.Live(); len(live) != 1 || live[0].Tag.CompetitionID != "c1" || len(live[0].Shots) != 2 || live[0].TargetType != "LG" {
		t.Errorf("live = %+v", live)
	}
	s.Stop(true)
	events := waitDone(t, s)
	if live := m.Live(); len(live) != 0 {
		t.Errorf("finished reading still live: %+v", live)
	}
	if last := events[len(events)-1]; last.MatchID != "m1" || len(saved) != 2 {
		t.Errorf("last=%+v saved=%d", last, len(saved))
	}
}

func TestStopDiscard(t *testing.T) {
	called := false
	s, _ := NewManager().Start(Params{Machine: slowMachine{"LG", 1},
		Finish: func(model.Shots, string) (string, error) { called = true; return "", nil }})
	waitShots(t, s, 1)
	s.Stop(false)
	waitDone(t, s)
	if called {
		t.Error("discarded reading was saved")
	}
}

func TestWrongTarget(t *testing.T) {
	called := false
	s, _ := NewManager().Start(Params{Machine: slowMachine{"LP", 1}, ExpectedTarget: "LG",
		Finish: func(model.Shots, string) (string, error) { called = true; return "", nil }})
	events := waitDone(t, s)
	if called || events[len(events)-2].Type != "error" {
		t.Errorf("called=%v events=%+v", called, events)
	}
}
