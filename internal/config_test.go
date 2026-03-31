package internal

import (
	"os"
	"testing"
)

func TestLoad_FromFile(t *testing.T) {
	content := `
port: 9090
no_bid_rate: 0.30
min_price_cpm: 0.50
max_price_cpm: 5.00
seat: "test-seat"
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil { t.Fatal(err) }
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil { t.Fatal(err) }
	f.Close()
	t.Setenv("CONFIG_PATH", f.Name())
	cfg, err := Load()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if cfg.Port != 9090 { t.Errorf("Port: want 9090, got %d", cfg.Port) }
	if cfg.NoBidRate != 0.30 { t.Errorf("NoBidRate: want 0.30, got %f", cfg.NoBidRate) }
	if cfg.MinPriceCPM != 0.50 { t.Errorf("MinPriceCPM: want 0.50, got %f", cfg.MinPriceCPM) }
	if cfg.MaxPriceCPM != 5.00 { t.Errorf("MaxPriceCPM: want 5.00, got %f", cfg.MaxPriceCPM) }
	if cfg.Seat != "test-seat" { t.Errorf("Seat: want test-seat, got %s", cfg.Seat) }
}

func TestLoad_Defaults(t *testing.T) {
	content := "port: 8080\n"
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil { t.Fatal(err) }
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()
	t.Setenv("CONFIG_PATH", f.Name())
	cfg, err := Load()
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if cfg.NoBidRate != 0.20 { t.Errorf("NoBidRate default: want 0.20, got %f", cfg.NoBidRate) }
	if cfg.MinPriceCPM != 0.10 { t.Errorf("MinPriceCPM default: want 0.10, got %f", cfg.MinPriceCPM) }
	if cfg.MaxPriceCPM != 10.00 { t.Errorf("MaxPriceCPM default: want 10.00, got %f", cfg.MaxPriceCPM) }
	if cfg.Seat != "mock-seat" { t.Errorf("Seat default: want mock-seat, got %s", cfg.Seat) }
}

func TestLoad_MissingFile(t *testing.T) {
	t.Setenv("CONFIG_PATH", "/nonexistent/path/config.yaml")
	_, err := Load()
	if err == nil { t.Error("expected error for missing file, got nil") }
}
