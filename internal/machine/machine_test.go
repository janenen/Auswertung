package machine

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/janenen/auswertung/internal/model"
)

// fakeRika simulates the RIKA SAG-2 serial protocol.
type fakeRika struct {
	mu      sync.Mutex
	pending []byte
	shots   []model.Shot
	target  string
	written [][]byte
	closed  bool
}

func (f *fakeRika) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, append([]byte(nil), b...))
	switch b[0] {
	case einzel:
		f.pending = append(f.pending, "200"...)
	case esc, abm:
		f.pending = append(f.pending, ack)
	case syn:
		if len(f.shots) == 0 {
			f.pending = append(f.pending, 0)
			break
		}
		s := f.shots[0]
		f.shots = f.shots[1:]
		msg := fmt.Sprintf("12345678\r001\r00000000\r%s\r01\r001\r%03d\r%05d\r%+06d\r%+06d\rXX",
			f.target, int(s.Ringe*10+0.5), int(s.Teiler*10+0.5), int(s.X*100), int(s.Y*100))
		f.pending = append(f.pending, soh)
		f.pending = append(f.pending, msg...)
	}
	return len(b), nil
}

func (f *fakeRika) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.pending) == 0 {
		time.Sleep(time.Millisecond)
		return 0, nil
	}
	n := copy(b, f.pending)
	f.pending = f.pending[n:]
	return n, nil
}

func (f *fakeRika) Close() error { f.closed = true; return nil }

func collect(t *testing.T, m Machine, req Request) ([]Reading, error) {
	t.Helper()
	out := make(chan Reading, 100)
	err := m.Read(context.Background(), req, out)
	close(out)
	var readings []Reading
	for r := range out {
		readings = append(readings, r)
	}
	return readings, err
}

func TestRika(t *testing.T) {
	want := []model.Shot{{Ringe: 10.4, Teiler: 132.4, X: 1.25, Y: -0.42}, {Ringe: 9.8, Teiler: 300.1, X: -0.92, Y: 2.85}}
	fake := &fakeRika{shots: want, target: "LG"}
	openPort = func(string) (io.ReadWriteCloser, error) { return fake, nil }

	readings, err := collect(t, Rika{Port: "COM9"}, Request{Count: 2, ShotsPerTarget: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(readings) != 2 {
		t.Fatalf("got %d readings", len(readings))
	}
	for i, r := range readings {
		if r.Shot != want[i] || r.TargetType != "LG" {
			t.Errorf("reading %d = %+v", i, r)
		}
	}
	if !bytes.Equal(fake.written[1], []byte{esc, 'S', '0', '0', '2', cr}) {
		t.Errorf("count command = %q", fake.written[1])
	}
	if last := fake.written[len(fake.written)-1]; last[0] != abm || !fake.closed {
		t.Error("reading not terminated with ABM and closed")
	}
}

func TestRikaCancel(t *testing.T) {
	fake := &fakeRika{target: "LG"}
	openPort = func(string) (io.ReadWriteCloser, error) { return fake, nil }
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := (Rika{Port: "COM9"}).Read(ctx, Request{Count: 5, ShotsPerTarget: 1}, make(chan Reading)); err != context.DeadlineExceeded {
		t.Errorf("err = %v", err)
	}
}

func TestCSV(t *testing.T) {
	data := "04.09.2023;3;LG\r10,4;132,4;1,25;-0,42\r9,8;300,1;0,92;-2,85\r"
	target, shots, err := ParseCSV([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if target != "LG" || len(shots) != 2 || shots[1] != (model.Shot{Ringe: 9.8, Teiler: 300.1, X: 0.92, Y: -2.85}) {
		t.Errorf("target=%s shots=%+v", target, shots)
	}
	if _, _, err := ParseCSV([]byte("04.09.2023;1;b'LG'\n1;2;3;4\n1;2;3;4\n")); err == nil {
		t.Error("expected error for too many shots")
	}
}

func TestQR(t *testing.T) {
	// Decimal format as written by the export, and the old integer format.
	target, shots, err := ParseQR("04092023LGA4132.4+1.25-0.42B8300.1-0.92+2.85")
	if err != nil {
		t.Fatal(err)
	}
	if target != "LG" || len(shots) != 2 || shots[0] != (model.Shot{Ringe: 10.4, Teiler: 132.4, X: 1.25, Y: -0.42}) || shots[1].Ringe != 9.8 {
		t.Errorf("target=%s shots=%+v", target, shots)
	}
	_, shots, err = ParseQR("04092023LGA41324+125-42")
	if err != nil || len(shots) != 1 || shots[0].Teiler != 1324 {
		t.Errorf("int format: %+v %v", shots, err)
	}
}

func TestQSD(t *testing.T) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, [5]float64{10.43, 1.25, -0.42, 132.4, 0})
	binary.Write(&buf, binary.LittleEndian, [5]float64{})
	binary.Write(&buf, binary.LittleEndian, [5]float64{9, 9, 9, 9, 9})
	shots, err := ParseQSD(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(shots) != 1 || shots[0] != (model.Shot{Ringe: 10.4, Teiler: 132.4, X: 1.25, Y: -0.42}) {
		t.Errorf("shots = %+v", shots)
	}
}
