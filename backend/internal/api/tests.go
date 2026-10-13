package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yourusername/storm/internal/loadtest"
)

type TestConfig struct {
	URL             string `json:"url"`
	Method          string `json:"method"`
	Workers         int    `json:"workers"`
	DurationSeconds int    `json:"duration_seconds"`
	Authorized      bool   `json:"authorized"`
	RPS             int    `json:"rps"`
}

func (s *Server) createTestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var cfg TestConfig

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	target, err := url.Parse(cfg.URL)

	if err != nil || target.Host == "" ||
		target.Scheme != "http" ||
		target.User != nil {
		http.Error(w, `{"error":"Valid local HTTP URL required"}`, http.StatusBadRequest)
		return
	}

	// MVP: only permit localhost and loopback IP addresses.
	host := strings.ToLower(target.Hostname())
	ip := net.ParseIP(host)

	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		http.Error(w, `{"error":"Only localhost is supported"}`, http.StatusBadRequest)
		return
	}

	if strings.ToUpper(cfg.Method) != "GET" {
		http.Error(w, `{"error":"Only GET is supported"}`, http.StatusBadRequest)
		return
	}

	if cfg.Workers < 1 || cfg.Workers > 10 {
		http.Error(w, `{"error":"Workers must be between 1 and 10"}`, http.StatusBadRequest)
		return
	}

	if cfg.DurationSeconds < 1 || cfg.DurationSeconds > 30 {
		http.Error(w, `{"error":"Duration must be between 1 and 30 seconds"}`, http.StatusBadRequest)
		return
	}

	if !cfg.Authorized {
		http.Error(w, `{"error":"Authorization required"}`, http.StatusBadRequest)
		return
	}
	if cfg.RPS == 0 {
		cfg.RPS = 20
	}

	if cfg.RPS < 1 || cfg.RPS > 100 {
		http.Error(
			w,
			`{"error":"RPS must be between 1 and 100"}`,
			http.StatusBadRequest,
		)
		return
	}

	engineConfig := loadtest.Config{
		URL:      cfg.URL,
		Workers:  cfg.Workers,
		Duration: time.Duration(cfg.DurationSeconds) * time.Second,
		RPS:      cfg.RPS,
	}

	summary := loadtest.Run(r.Context(), engineConfig)

	json.NewEncoder(w).Encode(summary)
}
