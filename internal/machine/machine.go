// Package machine reads shots from scoring machines and result files.
package machine

import (
	"context"
	"fmt"
	"strings"

	"github.com/janenen/auswertung/internal/model"
)

// Input kinds a machine needs from the user before reading.
const (
	InputNone = ""     // reads from hardware
	InputFile = "file" // needs an uploaded file
	InputText = "text" // needs a text (QR code content)
)

// Request configures a single reading.
type Request struct {
	// Count is the number of shots still to read.
	Count          int
	ShotsPerTarget int
	// TargetType is the expected target type. Sources without own target
	// information (QSD) report it unchanged.
	TargetType string
	// Data is the uploaded file content or entered text.
	Data []byte
}

// Reading is one shot together with the target type the source reported.
type Reading struct {
	Shot       model.Shot
	TargetType string
}

// Machine is a source of shots.
type Machine interface {
	ID() string
	Name() string
	Input() string
	// Accept is the file extension for InputFile machines.
	Accept() string
	// Read sends shots to out until the reading is complete, ctx is
	// cancelled or an error occurs. It does not close out.
	Read(ctx context.Context, req Request, out chan<- Reading) error
}

var fileMachines = []Machine{CSV{}, QR{}, QSD{}}

// Discover returns all currently available machines. It probes serial
// ports for RIKA devices, which takes a moment per port.
func Discover() []Machine {
	machines := []Machine{}
	for _, r := range discoverRika() {
		machines = append(machines, r)
	}
	return append(machines, fileMachines...)
}

// ByID returns the machine with the given ID without probing.
func ByID(id string) (Machine, error) {
	if port, ok := strings.CutPrefix(id, rikaPrefix); ok {
		return Rika{Port: port}, nil
	}
	for _, m := range fileMachines {
		if m.ID() == id {
			return m, nil
		}
	}
	return nil, fmt.Errorf("unbekannte Maschine %q", id)
}

func send(ctx context.Context, out chan<- Reading, r Reading) error {
	select {
	case out <- r:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sendAll(ctx context.Context, out chan<- Reading, shots []model.Shot, targetType string) error {
	for _, s := range shots {
		if err := send(ctx, out, Reading{Shot: s, TargetType: targetType}); err != nil {
			return err
		}
	}
	return nil
}
