package camera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStreamURL(t *testing.T) {
	for in, want := range map[string]string{
		"192.168.1.50":                     "http://192.168.1.50:81/stream",
		" kamera-1.local/ ":                "http://kamera-1.local:81/stream",
		"http://10.0.0.7":                  "http://10.0.0.7:81/stream",
		"http://10.0.0.7:8080/video.mjpeg": "http://10.0.0.7:8080/video.mjpeg",
		"10.0.0.7:81/stream":               "http://10.0.0.7:81/stream",
		"https://cam.example/live?x=1":     "https://cam.example/live?x=1",
		"[fe80::1]":                        "http://[fe80::1]:81/stream",
	} {
		if got, err := StreamURL(in); err != nil || got != want {
			t.Errorf("StreamURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "  ", "ftp://host/x", "http://"} {
		if got, err := StreamURL(in); err == nil {
			t.Errorf("StreamURL(%q) = %q, want error", in, got)
		}
	}
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mjpegServer streams frames like the firmware (boundary after each frame)
// and counts the connections.
type mjpegServer struct {
	*httptest.Server
	connections atomic.Int32
	frames      int // per connection, then the stream ends; 0 = endless
}

func newMJPEGServer(t *testing.T, frame []byte, frames int) *mjpegServer {
	s := &mjpegServer{frames: frames}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.connections.Add(1)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace;boundary=b0undary")
		fmt.Fprint(w, "--b0undary\r\n")
		for i := 0; s.frames == 0 || i < s.frames; i++ {
			fmt.Fprintf(w, "Content-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
			w.Write(frame)
			fmt.Fprint(w, "\r\n--b0undary\r\n")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func testHub(t *testing.T) *Hub {
	h := NewHub()
	h.Linger = 50 * time.Millisecond
	h.RetryDelay = 10 * time.Millisecond
	h.StaleTimeout = time.Second
	t.Cleanup(h.Close)
	return h
}

func TestWatchSharesConnection(t *testing.T) {
	frame := testJPEG(t, 32, 24)
	srv := newMJPEGServer(t, frame, 0)
	h := testHub(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
			err := h.Watch(ctx, srv.URL, func(got []byte) error {
				if !bytes.Equal(got, frame) {
					return errors.New("wrong frame")
				}
				if n++; n == 5 {
					return errDone
				}
				return nil
			})
			if !errors.Is(err, errDone) {
				t.Errorf("Watch: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := srv.connections.Load(); n != 1 {
		t.Errorf("%d connections to the camera, want 1", n)
	}
	st := h.Status(srv.URL)
	if !st.Active || !st.Connected || st.Width != 32 || st.Height != 24 || st.FPS <= 0 {
		t.Errorf("status = %+v", st)
	}

	// The connection is closed once the linger time has passed.
	deadline := time.Now().Add(2 * time.Second)
	for h.Status(srv.URL).Active {
		if time.Now().After(deadline) {
			t.Fatal("connection not closed after the last viewer left")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReconnect(t *testing.T) {
	frame := testJPEG(t, 8, 8)
	srv := newMJPEGServer(t, frame, 2) // the stream ends after two frames
	h := testHub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n := 0
	err := h.Watch(ctx, srv.URL, func([]byte) error {
		if n++; n == 6 {
			return errDone
		}
		return nil
	})
	if !errors.Is(err, errDone) {
		t.Fatalf("Watch: %v", err)
	}
	if c := srv.connections.Load(); c < 3 {
		t.Errorf("%d connections, want at least 3", c)
	}
}

func TestSnapshotAndErrors(t *testing.T) {
	frame := testJPEG(t, 8, 8)
	srv := newMJPEGServer(t, frame, 0)
	h := testHub(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := h.Snapshot(ctx, srv.URL)
	if err != nil || !bytes.Equal(got, frame) {
		t.Fatalf("Snapshot = %d bytes, %v", len(got), err)
	}

	notMJPEG := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer notMJPEG.Close()
	short, cancelShort := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelShort()
	if _, err := h.Snapshot(short, notMJPEG.URL); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Snapshot of a non-stream: %v", err)
	}
	// Status is kept during the linger time.
	if st := h.Status(notMJPEG.URL); st.Connected || st.Error == "" {
		t.Errorf("status = %+v, want an error", st)
	}
}
