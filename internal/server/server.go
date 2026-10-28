// Package server provides the JSON API and serves the web frontend.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/janenen/auswertung/internal/reading"
	"github.com/janenen/auswertung/internal/store"
)

type Options struct {
	// LeagueTemplate is the path of the RSB league result form (.xlsx).
	LeagueTemplate string
	// Shutdown is called by the quit button and may be nil.
	Shutdown func()
}

type Server struct {
	store    *store.Store
	readings *reading.Manager
	opts     Options
}

// New returns the HTTP handler.
func New(s *store.Store, frontend fs.FS, opts Options) http.Handler {
	srv := &Server{store: s, readings: reading.NewManager(), opts: opts}
	return srv.routes(frontend)
}

func (s *Server) routes(frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/meta", s.meta)

	mux.HandleFunc("GET /api/users", s.listUsers)
	mux.HandleFunc("POST /api/users", s.saveUser)
	mux.HandleFunc("PUT /api/users/{id}", s.saveUser)
	mux.HandleFunc("GET /api/users/{id}/matches", s.userMatches)
	mux.HandleFunc("GET /api/clubs", s.listClubs)
	mux.HandleFunc("POST /api/clubs", s.saveClub)
	mux.HandleFunc("PUT /api/clubs/{id}", s.saveClub)
	mux.HandleFunc("GET /api/teams", s.listTeams)
	mux.HandleFunc("POST /api/teams", s.saveTeam)
	mux.HandleFunc("PUT /api/teams/{id}", s.saveTeam)

	mux.HandleFunc("GET /api/competitions", s.listCompetitions)
	mux.HandleFunc("POST /api/competitions", s.createCompetition)
	mux.HandleFunc("GET /api/competitions/{id}", s.getCompetition)
	mux.HandleFunc("PATCH /api/competitions/{id}", s.updateCompetition)
	mux.HandleFunc("DELETE /api/competitions/{id}", s.deleteCompetition)
	mux.HandleFunc("GET /api/competitions/{id}/results", s.competitionResults)
	mux.HandleFunc("GET /api/competitions/{id}/report.pdf", s.competitionPDF)
	mux.HandleFunc("GET /api/competitions/{id}/league.xlsx", s.leagueXLSX)
	mux.HandleFunc("POST /api/competitions/{id}/simulate", s.simulateLeague)
	mux.HandleFunc("GET /api/board", s.board)

	mux.HandleFunc("GET /api/matches/{id}", s.getMatch)
	mux.HandleFunc("GET /api/matches/{id}/report.pdf", s.matchPDF)
	mux.HandleFunc("GET /api/matches/{id}/export.csv", s.matchCSV)
	mux.HandleFunc("GET /api/matches/{id}/qr.png", s.matchQR)

	mux.HandleFunc("GET /api/machines", s.listMachines)
	mux.HandleFunc("POST /api/readings", s.startReading)
	mux.HandleFunc("GET /api/readings/{id}/events", s.readingEvents)
	mux.HandleFunc("POST /api/readings/{id}/stop", s.stopReading)

	mux.HandleFunc("POST /api/shutdown", s.shutdown)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, errors.New("unbekannter API-Pfad"))
	})
	mux.Handle("/", spa(frontend))
	return mux
}

// spa serves the built frontend and falls back to index.html.
func spa(frontend fs.FS) http.Handler {
	files := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(frontend, path); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(frontend, "index.html")
		if err != nil {
			http.Error(w, "Frontend wurde nicht gebaut (npm run build in web/)", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}

type badRequest struct{ msg string }

func (e badRequest) Error() string { return e.msg }

func errBadRequest(msg string) error { return badRequest{msg} }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writing response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// fail maps err to a status code and writes it.
func fail(w http.ResponseWriter, err error) {
	var br badRequest
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, store.ErrHasEntries):
		writeError(w, http.StatusConflict, err)
	case errors.As(err, &br), errors.Is(err, store.ErrInvalid):
		writeError(w, http.StatusBadRequest, err)
	default:
		log.Printf("error: %v", err)
		writeError(w, http.StatusInternalServerError, err)
	}
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errBadRequest("ungültige Anfrage: " + err.Error())
	}
	return nil
}

func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	if s.opts.Shutdown == nil {
		writeError(w, http.StatusNotImplemented, errors.New("nicht verfügbar"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
	go s.opts.Shutdown()
}
