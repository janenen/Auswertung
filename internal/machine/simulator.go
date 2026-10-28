package machine

import (
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/janenen/auswertung/internal/model"
)

// SimulatorPort is the port name of the built-in RIKA simulator
// (port of https://github.com/janenen/simulator).
const SimulatorPort = "SIM"

const (
	nul         = 0x00
	eot         = 0x04
	simSerial   = "20013591"
	simReadWait = 50 * time.Millisecond
)

var (
	simulatorEnabled bool
	// simulatorDelay is the number of polls answered with NUL before a shot
	// is available; replaced in tests.
	simulatorDelay = 3
)

// SetSimulator switches simulation mode: the simulated RIKA is available
// as machine "rika:SIM" and league results can be simulated.
func SetSimulator(enabled bool) { simulatorEnabled = enabled }

// SimulatorEnabled reports whether the app runs in simulation mode.
func SimulatorEnabled() bool { return simulatorEnabled }

// SimulatedShots returns n random shots on the target type, with the
// precision a RIKA reports.
func SimulatedShots(target string, n int) model.Shots {
	g, ok := model.Targets[target]
	if !ok {
		g = model.Targets["LG"]
	}
	shots := make(model.Shots, n)
	for i := range shots {
		shots[i] = randomShot(g)
	}
	return shots
}

// randomShot follows the distance distribution of the original simulator:
// denser near the centre.
func randomShot(g model.TargetGeometry) model.Shot {
	u := rand.Float64()
	t := u * u * 0.88 * g.Width() / 2
	phi := rand.Float64() * 2 * math.Pi
	return model.Shot{
		Ringe:  math.Round(simRing(g, t)*10) / 10,
		Teiler: math.Floor(t*10) / 10,
		X:      math.Trunc(t*math.Cos(phi)) / 100,
		Y:      math.Trunc(t*math.Sin(phi)) / 100,
	}
}

// simulator emulates a RIKA SAG-2 behind a serial port. It produces random
// shots on the given target type.
type simulator struct {
	mu      sync.Mutex
	pending []byte
	notify  chan struct{}

	target string
	state  int // 0 idle, 1 ready, 2 after ESC, 3 reading parameter
	param  byte
	digits []byte
	delay  int

	serie, teiler, faktor, schuss, zehntel int
}

func newSimulator(target string) *simulator {
	if _, ok := model.Targets[target]; !ok {
		target = "LG"
	}
	return &simulator{
		notify: make(chan struct{}, 1),
		target: target,
		delay:  simulatorDelay,
		serie:  10,
		faktor: 10,
		schuss: 1,
	}
}

func (s *simulator) Write(b []byte) (int, error) {
	s.mu.Lock()
	for _, c := range b {
		s.handle(c)
	}
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
	return len(b), nil
}

// Read returns pending output or waits like a serial port with read timeout.
func (s *simulator) Read(b []byte) (int, error) {
	s.mu.Lock()
	if len(s.pending) == 0 {
		s.mu.Unlock()
		select {
		case <-s.notify:
		case <-time.After(simReadWait):
		}
		s.mu.Lock()
	}
	n := copy(b, s.pending)
	s.pending = s.pending[n:]
	s.mu.Unlock()
	return n, nil
}

func (s *simulator) Close() error { return nil }

func (s *simulator) reply(b ...byte) { s.pending = append(s.pending, b...) }

// paramSpec is the digit count and valid range of each ESC parameter.
var paramSpec = map[byte]struct{ n, lo, hi int }{
	'S': {3, 1, 200},  // shots
	'T': {4, 1, 6500}, // Teiler
	'F': {2, 1, 99},   // factor
	'U': {1, 1, 5},    // shots per target
	'Z': {1, 0, 3},    // decimal scoring
	'E': {0, 0, 0},    // status query
}

func (s *simulator) handle(c byte) {
	switch s.state {
	case 0:
		if c >= einzel && c <= 0xd5 {
			s.state = 1
			s.reply([]byte("200")...)
		}
	case 1:
		switch c {
		case abm:
			s.state = 0
			s.reply(nul)
		case syn:
			if s.delay > 0 {
				s.delay--
				s.reply(nul)
				return
			}
			s.delay = simulatorDelay
			s.reply(s.shot()...)
		case esc:
			s.state = 2
		}
	case 2:
		if _, ok := paramSpec[c]; ok {
			s.param, s.digits, s.state = c, nil, 3
		} else {
			s.state = 1
		}
	case 3:
		spec := paramSpec[s.param]
		if len(s.digits) < spec.n {
			if c >= '0' && c <= '9' {
				s.digits = append(s.digits, c)
			}
			return
		}
		if c != cr {
			return
		}
		s.state = 1
		if s.param == 'E' {
			s.pending = fmt.Appendf(s.pending, "%s.\r%03d\r%03d\r%04d\r%02d\r%d\r%d\r",
				simSerial, 0, s.serie, s.teiler, s.faktor, s.schuss, s.zehntel)
			return
		}
		v, _ := strconv.Atoi(string(s.digits))
		if v < spec.lo || v > spec.hi {
			s.reply(nul)
			return
		}
		switch s.param {
		case 'S':
			s.serie = v
		case 'T':
			s.teiler = v
		case 'F':
			s.faktor = v
		case 'U':
			s.schuss = v
		case 'Z':
			s.zehntel = v
		}
		s.reply(ack)
	}
}

// shot returns a complete shot message: SOH, header, values, EOT, checksum.
func (s *simulator) shot() []byte {
	shot := randomShot(model.Targets[s.target])
	tenths := func(v float64) int { return int(math.Round(v * 10)) }
	hundredths := func(v float64) int { return int(math.Round(v * 100)) }

	msg := []byte{soh}
	msg = fmt.Appendf(msg, "%s\r%03d\r00000000\r%s\r%02d\r%03d\r", simSerial, 0, s.target, s.faktor, s.serie)
	msg = fmt.Appendf(msg, "%03d\r%05d\r%+06d\r%+06d\r", tenths(shot.Ringe), tenths(shot.Teiler), hundredths(shot.X), hundredths(shot.Y))
	msg = append(msg, eot)
	var sum byte
	for _, c := range msg {
		sum ^= c
	}
	return append(msg, sum)
}

// simRing is the decimal ring value of a shot at distance t (1/100 mm).
func simRing(g model.TargetGeometry, t float64) float64 {
	ten := g.RadiusTen + g.RadiusCalibre
	if t < ten {
		return 10.9 - math.Floor(t/(ten/10))/10
	}
	ring := 9.9 - math.Floor((t-ten)/(g.RingIncrement/10))/10
	if ring < 1 {
		return 0
	}
	return ring
}

var _ io.ReadWriteCloser = (*simulator)(nil)
