package api

import (
	"encoding/json"
	"github.com/asralov/storm/internal/buildinfo"
	"github.com/asralov/storm/internal/runs"
	"github.com/asralov/storm/internal/storage"
	"net/http"
)

type Server struct {
	mux     *http.ServeMux
	manager *runs.Manager
	history *storage.Store
}

func NewServer(stores ...*storage.Store) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		manager: runs.NewManager(),
	}
	if len(stores) > 0 && stores[0] != nil {
		s.history = stores[0]
		s.manager = runs.NewManager(s.history)
	}
	s.mux.HandleFunc("GET /api/health", s.healthHandler)
	s.mux.HandleFunc("POST /api/tests", s.createTestHandler)
	s.mux.HandleFunc("GET /api/tests/active", s.activeTestHandler)
	s.mux.HandleFunc("GET /api/tests/{id}", s.getTestHandler)
	s.mux.HandleFunc("POST /api/tests/{id}/stop", s.stopTestHandler)
	s.mux.HandleFunc("GET /api/history", s.listHistoryHandler)
	s.mux.HandleFunc("GET /api/history/{id}", s.getHistoryHandler)
	s.mux.HandleFunc("DELETE /api/history/{id}", s.deleteHistoryHandler)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "endpoint not found") })
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !safeRequest(w, r) {
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Close() { s.manager.Close() }

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "storm",
		"version": buildinfo.Version(),
	})
}
