package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/janenen/auswertung/internal/camera"
	"github.com/janenen/auswertung/internal/model"
)

// cameraView is a camera with the state of its connection.
type cameraView struct {
	model.Camera
	Status camera.Status `json:"status"`
}

func (s *Server) listCameras(w http.ResponseWriter, r *http.Request) {
	cams, err := s.store.Cameras()
	if err != nil {
		fail(w, err)
		return
	}
	out := make([]cameraView, len(cams))
	for i, c := range cams {
		out[i] = cameraView{c, s.cameras.Status(c.URL)}
	}
	writeJSON(w, out)
}

func (s *Server) saveCamera(w http.ResponseWriter, r *http.Request) {
	var c model.Camera
	if err := decode(r, &c); err != nil {
		fail(w, err)
		return
	}
	c.ID = r.PathValue("id")
	var err error
	if c.Name, err = requireName(c.Name); err != nil {
		fail(w, err)
		return
	}
	if c.URL, err = camera.StreamURL(c.URL); err != nil {
		fail(w, errBadRequest(err.Error()))
		return
	}
	if c, err = s.store.SaveCamera(c); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, c)
}

func (s *Server) deleteCamera(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCamera(r.PathValue("id")); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cameraStream relays the camera's MJPEG stream. The boundary follows each
// frame, so browsers show a frame as soon as it is complete.
func (s *Server) cameraStream(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.Camera(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-store")
	rc := http.NewResponseController(w)
	if _, err := io.WriteString(w, "--frame\r\n"); err != nil {
		return
	}
	rc.Flush()
	s.cameras.Watch(r.Context(), c.URL, func(frame []byte) error {
		// Drop viewers that stop reading instead of blocking forever.
		rc.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprintf(w, "Content-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame)); err != nil {
			return err
		}
		if _, err := w.Write(frame); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\r\n--frame\r\n"); err != nil {
			return err
		}
		return rc.Flush()
	})
}

// cameraSnapshot returns the camera's next frame as JPEG.
func (s *Server) cameraSnapshot(w http.ResponseWriter, r *http.Request) {
	c, err := s.store.Camera(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	frame, err := s.cameras.Snapshot(ctx, c.URL)
	if err != nil {
		msg := "keine Verbindung zur Kamera"
		if st := s.cameras.Status(c.URL); st.Error != "" {
			msg = "Kamera: " + st.Error
		}
		writeError(w, http.StatusBadGateway, errors.New(msg))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	send(w, frame, "image/jpeg", c.Name+".jpg", false)
}
