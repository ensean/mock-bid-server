package internal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Port        int     `yaml:"port"`
	NoBidRate   float64 `yaml:"no_bid_rate"`
	MinPriceCPM float64 `yaml:"min_price_cpm"`
	MaxPriceCPM float64 `yaml:"max_price_cpm"`
	Seat        string  `yaml:"seat"`
}

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
	if cfg.Port == 0 { cfg.Port = 8080 }
	if cfg.NoBidRate == 0 { cfg.NoBidRate = 0.20 }
	if cfg.MinPriceCPM == 0 { cfg.MinPriceCPM = 0.10 }
	if cfg.MaxPriceCPM == 0 { cfg.MaxPriceCPM = 10.00 }
	if cfg.Seat == "" { cfg.Seat = "mock-seat" }
	return cfg, nil
}
