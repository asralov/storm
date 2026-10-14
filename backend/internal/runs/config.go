package runs

import (
	"errors"
	"github.com/yourusername/storm/internal/loadtest"
	"net"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	URL             string `json:"url"`
	Method          string `json:"method"`
	Workers         int    `json:"workers"`
	DurationSeconds int    `json:"duration_seconds"`
	Authorized      bool   `json:"authorized"`
	RPS             int    `json:"rps"`
}

func (c *Config) validate() error {
	target, err := url.Parse(c.URL)
	if err != nil || target.Host == "" || target.Scheme != "http" || target.User != nil || target.Fragment != "" {
		return errors.New("valid local HTTP URL required")
	}
	host := strings.ToLower(target.Hostname())
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("only localhost or loopback IP addresses are supported")
	}
	if strings.ToUpper(c.Method) != "GET" {
		return errors.New("only GET is supported")
	}
	c.Method = "GET"
	if c.Workers < 1 || c.Workers > 10 {
		return errors.New("workers must be between 1 and 10")
	}
	if c.DurationSeconds < 1 || c.DurationSeconds > 30 {
		return errors.New("duration must be between 1 and 30 seconds")
	}
	if !c.Authorized {
		return errors.New("authorization required")
	}
	if c.RPS == 0 {
		c.RPS = 20
	}
	if c.RPS < 1 || c.RPS > 100 {
		return errors.New("RPS must be between 1 and 100")
	}
	return nil
}

func (c Config) engineConfig() loadtest.Config {
	return loadtest.Config{URL: c.URL, Workers: c.Workers, Duration: time.Duration(c.DurationSeconds) * time.Second, RPS: c.RPS}
}
