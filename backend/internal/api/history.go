package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/asralov/storm/internal/storage"
)

func (s *Server) historyReady(w http.ResponseWriter) bool {
	if s.history == nil {
		writeError(w, 503, "history storage is unavailable")
		return false
	}
	return true
}
func historyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeError(w, 404, err.Error())
	case errors.Is(err, storage.ErrInvalid):
		writeError(w, 400, err.Error())
	default:
		log.Printf("history operation: %v", err)
		writeError(w, 503, "history storage could not complete the operation; try again")
	}
}
func (s *Server) listHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if !s.historyReady(w) {
		return
	}
	values := r.URL.Query()
	limit, offset := 20, 0
	for key, list := range values {
		if (key != "limit" && key != "offset") || len(list) != 1 {
			writeError(w, 400, "use one limit and one offset parameter")
			return
		}
		n, err := strconv.Atoi(list[0])
		if err != nil {
			writeError(w, 400, "pagination must use whole numbers")
			return
		}
		if key == "limit" {
			limit = n
		} else {
			offset = n
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := s.history.List(ctx, limit, offset)
	if err != nil {
		historyError(w, err)
		return
	}
	writeJSON(w, 200, page)
}
func (s *Server) getHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if !s.historyReady(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	run, err := s.history.Get(ctx, r.PathValue("id"))
	if err != nil {
		historyError(w, err)
		return
	}
	writeJSON(w, 200, run)
}
func (s *Server) deleteHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if !s.historyReady(w) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	id := r.PathValue("id")
	if err := s.history.Delete(ctx, id); err != nil {
		historyError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"deleted": id})
}
