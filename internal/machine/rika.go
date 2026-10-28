package machine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"sync"
	"time"

	"go.bug.st/serial"

	"github.com/janenen/auswertung/internal/model"
)

// RIKA SAG-2 control bytes.
const (
	soh    = 0x01
	ack    = 0x06
	ff     = 0x0c
	cr     = 0x0d
	syn    = 0x16
	esc    = 0x1b
	abm    = 0xd0
	einzel = 0xd1
)

const (
	rikaPrefix       = "rika:"
	rikaTimeout      = 500 * time.Millisecond
	rikaPoll         = 200 * time.Millisecond
	rikaProbeTimeout = 3 * time.Second
	rikaMessageLen   = 32 + 24 + 2
)

var rikaMessage = regexp.MustCompile(`^(\d{8})\r(\d{3})\r(\d{8})\r(\w{2})\r(\d{2})\r(\d{3})\r(\d{3})\r(\d{5})\r([+-]\d{5})\r([+-]\d{5})\r`)

// Rika is a RIKA SAG-2 scoring machine on a serial port.
type Rika struct{ Port string }

func (r Rika) ID() string { return rikaPrefix + r.Port }
func (r Rika) Name() string {
	if r.simulated() {
		return "Rika-Simulator"
	}
	return "Rika SAG-2 an " + r.Port
}
func (r Rika) Input() string  { return InputNone }
func (r Rika) Accept() string { return "" }

// openPort is replaced in tests.
var openPort = func(name string) (io.ReadWriteCloser, error) {
	p, err := serial.Open(name, &serial.Mode{BaudRate: 9600})
	if err != nil {
		return nil, err
	}
	if err := p.SetReadTimeout(50 * time.Millisecond); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func (r Rika) simulated() bool { return simulatorEnabled && r.Port == SimulatorPort }

// open opens the serial port, or the simulator producing shots on the
// expected target type.
func (r Rika) open(targetType string) (io.ReadWriteCloser, error) {
	if r.simulated() {
		return newSimulator(targetType), nil
	}
	return openPort(r.Port)
}

type rikaConn struct{ rw io.ReadWriter }

func (c rikaConn) write(b ...byte) error {
	_, err := c.rw.Write(b)
	return err
}

// read reads n bytes or as many as arrive before the timeout.
func (c rikaConn) read(n int) ([]byte, error) {
	buf := make([]byte, 0, n)
	tmp := make([]byte, n)
	deadline := time.Now().Add(rikaTimeout)
	for len(buf) < n && time.Now().Before(deadline) {
		k, err := c.rw.Read(tmp[:n-len(buf)])
		buf = append(buf, tmp[:k]...)
		if err != nil {
			return buf, err
		}
	}
	return buf, nil
}

// command writes a command and expects the given answer.
func (c rikaConn) command(cmd []byte, want string) error {
	if err := c.write(cmd...); err != nil {
		return err
	}
	ans, err := c.read(len(want))
	if err != nil {
		return err
	}
	if string(ans) != want {
		return fmt.Errorf("unerwartete Antwort %q", ans)
	}
	return nil
}

func (c rikaConn) abort() {
	c.write(abm)
	c.read(1)
}

func (c rikaConn) configure(req Request) error {
	if err := c.command([]byte{einzel}, "200"); err != nil {
		return fmt.Errorf("Rika antwortet nicht: %w", err)
	}
	steps := []struct {
		cmd string
		msg string
	}{
		{fmt.Sprintf("S%03d", req.Count), "Anzahl konnte nicht übernommen werden"},
		{fmt.Sprintf("U%d", req.ShotsPerTarget), "Schuss pro Scheibe konnte nicht übernommen werden"},
		{"Z3", "Teilerwertung konnte nicht übernommen werden"},
	}
	for _, s := range steps {
		cmd := append(append([]byte{esc}, s.cmd...), cr)
		if err := c.command(cmd, string([]byte{ack})); err != nil {
			c.write(abm)
			return fmt.Errorf("%s: %w", s.msg, err)
		}
	}
	return nil
}

func parseRikaMessage(msg []byte) (Reading, error) {
	g := rikaMessage.FindSubmatch(msg)
	if g == nil {
		return Reading{}, fmt.Errorf("ungültige Nachricht %q", msg)
	}
	num := func(i int) float64 {
		v, _ := strconv.Atoi(string(g[i]))
		return float64(v)
	}
	return Reading{
		TargetType: string(g[4]),
		Shot:       model.Shot{Ringe: num(7) / 10, Teiler: num(8) / 10, X: num(9) / 100, Y: num(10) / 100},
	}, nil
}

func (r Rika) Read(ctx context.Context, req Request, out chan<- Reading) error {
	p, err := r.open(req.TargetType)
	if err != nil {
		return fmt.Errorf("Port %s konnte nicht geöffnet werden: %w", r.Port, err)
	}
	defer p.Close()
	c := rikaConn{p}
	c.read(1) // flush buffer
	if err := c.configure(req); err != nil {
		return err
	}
	defer c.abort()

	for n := 0; n < req.Count; {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.write(syn); err != nil {
			return err
		}
		ans, err := c.read(1)
		if err != nil {
			return err
		}
		if len(ans) != 1 || ans[0] != soh {
			select {
			case <-time.After(rikaPoll):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		msg, err := c.read(rikaMessageLen)
		if err != nil {
			return err
		}
		reading, err := parseRikaMessage(msg)
		if err != nil {
			return err
		}
		if err := send(ctx, out, reading); err != nil {
			return err
		}
		n++
		if err := c.write(ff); err != nil {
			return err
		}
	}
	return nil
}

// probe checks whether a RIKA answers on the port.
func probe(name string) error {
	p, err := openPort(name)
	if err != nil {
		return err
	}
	defer p.Close()
	c := rikaConn{p}
	c.read(1)
	err = c.command([]byte{einzel}, "200")
	c.abort()
	return err
}

func discoverRika() []Rika {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Printf("listing serial ports: %v", err)
	}
	var found []Rika
	if simulatorEnabled {
		found = append(found, Rika{Port: SimulatorPort})
	}
	// Probe in parallel: some ports (e.g. Bluetooth) block on open.
	errs := make([]error, len(ports))
	var wg sync.WaitGroup
	for i, name := range ports {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = probeTimeout(name)
		}()
	}
	wg.Wait()
	for i, name := range ports {
		if err := errs[i]; err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("no RIKA on %s: %v", name, err)
			}
			continue
		}
		found = append(found, Rika{Port: name})
	}
	return found
}

// probeTimeout is probe with a time limit; a blocked probe is abandoned.
func probeTimeout(name string) error {
	done := make(chan error, 1)
	go func() { done <- probe(name) }()
	select {
	case err := <-done:
		return err
	case <-time.After(rikaProbeTimeout):
		return errors.New("Zeitüberschreitung")
	}
}
