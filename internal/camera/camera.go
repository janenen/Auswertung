// Package camera relays the MJPEG streams of network cameras such as the
// ESP32-CAM firmware in firmware/esp32-cam. Each camera is read over a
// single connection that is shared by all viewers, because the ESP32-CAM
// serves only one stream client at a time. Relaying also makes the frames
// same-origin for the web app, which a later camera calibration (warping
// the image in a canvas) needs.
package camera

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MaxFrameSize limits a single JPEG frame.
const MaxFrameSize = 8 << 20

// StreamURL normalizes a camera address as entered by the user. A bare host
// or IP address means the Auswertung firmware, whose stream is served at
// port 81, path /stream; anything with a port or path is taken as the URL
// of an MJPEG stream.
func StreamURL(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("Adresse fehlt")
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", fmt.Errorf("ungültige Kameraadresse %q", addr)
	}
	if u.Port() == "" && (u.Path == "" || u.Path == "/") && u.RawQuery == "" {
		u.Host = net.JoinHostPort(u.Hostname(), "81")
		u.Path = "/stream"
	}
	return u.String(), nil
}

// Status describes the connection to a camera.
type Status struct {
	// Active is true while the stream is being read (someone is watching).
	Active    bool    `json:"active"`
	Connected bool    `json:"connected"`
	FPS       float64 `json:"fps"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Error     string  `json:"error,omitempty"`
}

// Hub manages the connections to the cameras. Its zero value is not
// usable, see NewHub.
type Hub struct {
	client *http.Client
	// Linger keeps a connection open after the last viewer left, so
	// reloading a page does not reconnect.
	Linger time.Duration
	// RetryDelay is the pause before reconnecting after an error.
	RetryDelay time.Duration
	// StaleTimeout drops a connection that delivers no frames.
	StaleTimeout time.Duration

	mu    sync.Mutex
	feeds map[string]*feed
}

func NewHub() *Hub {
	return &Hub{
		client: &http.Client{Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 10 * time.Second,
		}},
		Linger:       10 * time.Second,
		RetryDelay:   2 * time.Second,
		StaleTimeout: 10 * time.Second,
		feeds:        map[string]*feed{},
	}
}

// feed is the connection to one stream URL.
type feed struct {
	url     string
	cancel  context.CancelFunc
	viewers int // guarded by Hub.mu
	linger  *time.Timer

	mu      sync.Mutex
	frame   []byte
	seq     uint64
	last    time.Time
	status  Status
	waiters map[chan struct{}]struct{}
}

func (h *Hub) acquire(streamURL string) *feed {
	h.mu.Lock()
	defer h.mu.Unlock()
	f := h.feeds[streamURL]
	if f == nil {
		ctx, cancel := context.WithCancel(context.Background())
		f = &feed{url: streamURL, cancel: cancel, waiters: map[chan struct{}]struct{}{}}
		f.status.Active = true
		h.feeds[streamURL] = f
		go h.run(ctx, f)
	}
	if f.linger != nil {
		f.linger.Stop()
		f.linger = nil
	}
	f.viewers++
	return f
}

func (h *Hub) release(f *feed) {
	h.mu.Lock()
	defer h.mu.Unlock()
	f.viewers--
	if f.viewers > 0 {
		return
	}
	f.linger = time.AfterFunc(h.Linger, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if f.viewers == 0 && h.feeds[f.url] == f {
			f.cancel()
			delete(h.feeds, f.url)
		}
	})
}

// Close disconnects all cameras.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for u, f := range h.feeds {
		f.cancel()
		delete(h.feeds, u)
	}
}

// Status returns the state of the connection to streamURL.
func (h *Hub) Status(streamURL string) Status {
	h.mu.Lock()
	f := h.feeds[streamURL]
	h.mu.Unlock()
	if f == nil {
		return Status{}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

// Watch calls fn with every new frame (JPEG) of the stream until ctx ends
// or fn returns an error. Frames are skipped when fn is slower than the
// camera. fn must not modify the frame.
func (h *Hub) Watch(ctx context.Context, streamURL string, fn func(frame []byte) error) error {
	f := h.acquire(streamURL)
	defer h.release(f)
	wake := make(chan struct{}, 1)
	f.mu.Lock()
	f.waiters[wake] = struct{}{}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.waiters, wake)
		f.mu.Unlock()
	}()

	var seen uint64
	for {
		f.mu.Lock()
		frame, seq := f.frame, f.seq
		f.mu.Unlock()
		if frame != nil && seq != seen {
			seen = seq
			if err := fn(frame); err != nil {
				return err
			}
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wake:
		}
	}
}

var errDone = errors.New("done")

// Snapshot returns the next frame of the stream.
func (h *Hub) Snapshot(ctx context.Context, streamURL string) ([]byte, error) {
	var out []byte
	err := h.Watch(ctx, streamURL, func(frame []byte) error {
		out = frame
		return errDone
	})
	if errors.Is(err, errDone) {
		return out, nil
	}
	return nil, err
}

// run reads the stream and reconnects after errors until ctx ends.
func (h *Hub) run(ctx context.Context, f *feed) {
	for {
		err := h.read(ctx, f)
		if ctx.Err() != nil {
			return
		}
		f.mu.Lock()
		f.frame = nil // do not show an outdated picture
		f.last = time.Time{}
		f.status = Status{Active: true, Error: err.Error()}
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(h.RetryDelay):
		}
	}
}

func (h *Hub) read(ctx context.Context, f *feed) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stale atomic.Bool
	watchdog := time.AfterFunc(h.StaleTimeout, func() { stale.Store(true); cancel() })
	defer watchdog.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		if stale.Load() {
			return errors.New("Zeitüberschreitung")
		}
		return fmt.Errorf("keine Verbindung: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Kamera antwortet mit %s", resp.Status)
	}
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") || params["boundary"] == "" {
		return fmt.Errorf("kein MJPEG-Stream (%s)", resp.Header.Get("Content-Type"))
	}

	mr := multipart.NewReader(resp.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == nil {
			var frame []byte
			frame, err = io.ReadAll(io.LimitReader(part, MaxFrameSize+1))
			if err == nil && len(frame) > MaxFrameSize {
				err = errors.New("Bild zu groß")
			}
			if err == nil {
				watchdog.Reset(h.StaleTimeout)
				f.publish(frame)
				continue
			}
		}
		switch {
		case stale.Load():
			return errors.New("Zeitüberschreitung, keine Bilder")
		case errors.Is(err, io.EOF):
			return errors.New("Stream beendet")
		default:
			return err
		}
	}
}

func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func (f *feed) publish(frame []byte) {
	now := time.Now()
	cfg, cfgErr := jpeg.DecodeConfig(bytes.NewReader(frame))

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.last.IsZero() {
		if dt := now.Sub(f.last).Seconds(); dt > 0 {
			fps := 1 / dt
			if f.status.FPS > 0 {
				fps = 0.9*f.status.FPS + 0.1*fps
			}
			f.status.FPS = fps
		}
	}
	f.last = now
	f.frame = frame
	f.seq++
	f.status.Active, f.status.Connected, f.status.Error = true, true, ""
	if cfgErr == nil {
		f.status.Width, f.status.Height = cfg.Width, cfg.Height
	}
	for w := range f.waiters {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}
