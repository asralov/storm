package api

import (
	"encoding/json"
	"errors"
	"github.com/asralov/storm/internal/runs"
	"io"
	"net/http"
)

func (s *Server) createTestHandler(w http.ResponseWriter, r *http.Request) {
	var cfg runs.Config
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		writeError(w, 400, "invalid JSON configuration")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, 400, "expected one JSON object")
		return
	}
	run, err := s.manager.Start(cfg)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, runs.ErrBusy) {
			status = http.StatusConflict
		}
		if errors.Is(err, runs.ErrClosed) {
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, err.Error())
		return
	}
	w.Header().Set("Location", "/api/tests/"+run.ID)
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) getTestHandler(w http.ResponseWriter, r *http.Request) {
	run, err := s.manager.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, run)
}

func (s *Server) activeTestHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"run": s.manager.Active()})
}

func (s *Server) stopTestHandler(w http.ResponseWriter, r *http.Request) {
	run, err := s.manager.Stop(r.PathValue("id"))
	if err != nil {
		status := 404
		if errors.Is(err, runs.ErrFinished) {
			status = 409
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, 202, map[string]any{"id": run.ID, "status": run.Status, "cancellation_requested": true})
}
