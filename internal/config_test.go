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

func TestLoad_ADXDefaults(t *testing.T) {
	content := "port: 8080\n"
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()
	t.Setenv("CONFIG_PATH", f.Name())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AdxPort != 8090 {
		t.Errorf("AdxPort default: want 8090, got %d", cfg.AdxPort)
	}
	if cfg.AdxTimeoutMS != 200 {
		t.Errorf("AdxTimeoutMS default: want 200, got %d", cfg.AdxTimeoutMS)
	}
	if cfg.AdxFloorCPM != 0.50 {
		t.Errorf("AdxFloorCPM default: want 0.50, got %f", cfg.AdxFloorCPM)
	}
}

func TestLoad_ADXDSPs(t *testing.T) {
	content := `
port: 8080
adx_port: 8090
adx_timeout_ms: 150
adx_floor_cpm: 1.00
dsps:
  - id: dsp-1
    url: http://localhost:8081/bid
  - id: dsp-2
    url: http://localhost:8082/bid
`
	f, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()
	t.Setenv("CONFIG_PATH", f.Name())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AdxPort != 8090 {
		t.Errorf("AdxPort: want 8090, got %d", cfg.AdxPort)
	}
	if cfg.AdxTimeoutMS != 150 {
		t.Errorf("AdxTimeoutMS: want 150, got %d", cfg.AdxTimeoutMS)
	}
	if cfg.AdxFloorCPM != 1.00 {
		t.Errorf("AdxFloorCPM: want 1.00, got %f", cfg.AdxFloorCPM)
	}
	if len(cfg.DSPs) != 2 {
		t.Fatalf("DSPs: want 2, got %d", len(cfg.DSPs))
	}
	if cfg.DSPs[0].ID != "dsp-1" || cfg.DSPs[0].URL != "http://localhost:8081/bid" {
		t.Errorf("DSPs[0]: got %+v", cfg.DSPs[0])
	}
	if cfg.DSPs[1].ID != "dsp-2" || cfg.DSPs[1].URL != "http://localhost:8082/bid" {
		t.Errorf("DSPs[1]: got %+v", cfg.DSPs[1])
	}
}
