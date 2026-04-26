package internal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DSPConfig holds the identity and endpoint of a single downstream DSP.
type DSPConfig struct {
	ID  string `yaml:"id"`
	URL string `yaml:"url"`
}

// Config holds all runtime configuration for the mock bid server.
type Config struct {
	Port        int     `yaml:"port"`
	NoBidRate   float64 `yaml:"no_bid_rate"`
	MinPriceCPM float64 `yaml:"min_price_cpm"`
	MaxPriceCPM float64 `yaml:"max_price_cpm"`
	Seat        string  `yaml:"seat"`

	AdxPort      int         `yaml:"adx_port"`
	AdxTimeoutMS int         `yaml:"adx_timeout_ms"`
	AdxFloorCPM  float64     `yaml:"adx_floor_cpm"`
	DSPs         []DSPConfig `yaml:"dsps"`
}

// Load reads configuration from the file at CONFIG_PATH env var (default: ./config.yaml).
// Any zero-value field receives its default.
func Load() (Config, error) {
	path := "config.yaml"
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		path = p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.NoBidRate == 0 {
		cfg.NoBidRate = 0.20
	}
	if cfg.MinPriceCPM == 0 {
		cfg.MinPriceCPM = 0.10
	}
	if cfg.MaxPriceCPM == 0 {
		cfg.MaxPriceCPM = 10.00
	}
	if cfg.Seat == "" {
		cfg.Seat = "mock-seat"
	}
	if cfg.AdxPort == 0 {
		cfg.AdxPort = 8090
	}
	if cfg.AdxTimeoutMS == 0 {
		cfg.AdxTimeoutMS = 200
	}
	if cfg.AdxFloorCPM == 0 {
		cfg.AdxFloorCPM = 0.50
	}
	return cfg, nil
}
