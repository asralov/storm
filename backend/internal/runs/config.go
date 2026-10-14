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
	Mode                 string `json:"mode,omitempty"`
	StartRPS             int    `json:"start_rps,omitempty"`
	RPSIncrement         int    `json:"rps_increment,omitempty"`
	MaxRPS               int    `json:"max_rps,omitempty"`
	StageDurationSeconds int    `json:"stage_duration_seconds,omitempty"`
	URL                  string `json:"url"`
	Method               string `json:"method"`
	Workers              int    `json:"workers"`
	DurationSeconds      int    `json:"duration_seconds"`
	Authorized           bool   `json:"authorized"`
	RPS                  int    `json:"rps"`
}

func (c *Config) validate() error {
	if c.Mode == "" {
		c.Mode = "standard"
	}
	if c.Mode != "standard" && c.Mode != "stepped" {
		return errors.New("mode must be standard or stepped")
	}
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
	if !c.Authorized {
		return errors.New("authorization required")
	}
	if c.Mode == "stepped" {
		if c.StartRPS < 1 || c.StartRPS > 100 || c.MaxRPS < c.StartRPS || c.MaxRPS > 100 || c.RPSIncrement < 1 || c.RPSIncrement > 100 {
			return errors.New("stepped rates must be 1–100, with maximum at least start and a positive increment")
		}
		if c.StageDurationSeconds < 1 || c.StageDurationSeconds > 30 {
			return errors.New("stage duration must be between 1 and 30 seconds")
		}
		count := (c.MaxRPS-c.StartRPS)/c.RPSIncrement + 1
		if count > 10 || count*c.StageDurationSeconds > 120 {
			return errors.New("stepped tests allow at most 10 stages and 120 seconds of configured load")
		}
		if c.RPS != 0 || c.DurationSeconds != 0 {
			return errors.New("use stepped rate and duration fields in stepped mode")
		}
		return nil
	}
	if c.StartRPS != 0 || c.MaxRPS != 0 || c.RPSIncrement != 0 || c.StageDurationSeconds != 0 {
		return errors.New("stepped fields require stepped mode")
	}
	if c.DurationSeconds < 1 || c.DurationSeconds > 30 {
		return errors.New("duration must be between 1 and 30 seconds")
	}
	if c.RPS == 0 {
		c.RPS = 20
	}
	if c.RPS < 1 || c.RPS > 100 {
		return errors.New("RPS must be between 1 and 100")
	}
	return nil
}

// stageConfigs is called only after validation. Maximum RPS is a ceiling;
// it is not appended as an extra stage when the increment does not reach it.
func (c Config) stageConfigs() []loadtest.Config {
	if c.Mode != "stepped" {
		return []loadtest.Config{c.engineConfig()}
	}
	var stages []loadtest.Config
	for rate := c.StartRPS; rate <= c.MaxRPS; rate += c.RPSIncrement {
		stages = append(stages, loadtest.Config{URL: c.URL, Workers: c.Workers, RPS: rate, Duration: time.Duration(c.StageDurationSeconds) * time.Second})
	}
	return stages
}

func (c Config) engineConfig() loadtest.Config {
	return loadtest.Config{URL: c.URL, Workers: c.Workers, Duration: time.Duration(c.DurationSeconds) * time.Second, RPS: c.RPS}
}
