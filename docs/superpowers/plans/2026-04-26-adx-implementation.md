# ADX Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an ADX (Ad Exchange) binary that fans out OpenRTB BidRequests to multiple DSPs concurrently, runs a second-price auction, and returns the clearing-price BidResponse.

**Architecture:** A new `cmd/adx/main.go` binary reads DSP URLs from `config.yaml` and starts an HTTP server on `POST /openrtb`. `internal/adx.go` holds the fan-out + auction logic; `internal/adx_handler.go` holds the HTTP adapter. This mirrors the existing `cmd/server` → `internal/bidder` + `internal/handler` pattern exactly.

**Tech Stack:** Go 1.22, `net/http`, `net/http/httptest` (tests), `log/slog`, `github.com/mxmCherry/openrtb/openrtb2`, `gopkg.in/yaml.v3`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/config.go` | Modify | Add `DSPConfig` struct + ADX fields to `Config` + defaults |
| `internal/adx.go` | Create | `ADX`, `AuctionResult`, `DSPResult`, `Auction()` |
| `internal/adx_handler.go` | Create | `ADXHandler.ServeHTTP` — HTTP adapter for `Auction()` |
| `internal/adx_test.go` | Create | Unit tests for auction logic + handler |
| `cmd/adx/main.go` | Create | Binary entry point |
| `config.yaml` | Modify | Add ADX + example DSP config |

---

## Task 1: Extend Config with ADX fields

**Files:**
- Modify: `internal/config.go`
- Modify: `internal/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config_test.go` (inside the existing `package internal`):

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/ubuntu/projects/mock-bid-server
go test ./internal/ -run "TestLoad_ADX" -v
```

Expected: compile error — `cfg.AdxPort undefined`

- [ ] **Step 3: Add DSPConfig struct and new fields to Config**

In `internal/config.go`, add `DSPConfig` before the `Config` struct, and add four new fields to `Config`:

```go
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
```

Then in `Load()`, after the existing defaults block, append:

```go
if cfg.AdxPort == 0 {
    cfg.AdxPort = 8090
}
if cfg.AdxTimeoutMS == 0 {
    cfg.AdxTimeoutMS = 200
}
if cfg.AdxFloorCPM == 0 {
    cfg.AdxFloorCPM = 0.50
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/ -run "TestLoad" -v
```

Expected: all `TestLoad_*` tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config.go internal/config_test.go
git commit -m "feat: add ADX config fields (AdxPort, AdxTimeoutMS, AdxFloorCPM, DSPs)"
```

---

## Task 2: Core auction logic (`internal/adx.go`)

**Files:**
- Create: `internal/adx.go`
- Create: `internal/adx_test.go` (auction unit tests only)

- [ ] **Step 1: Write the failing auction unit tests**

Create `internal/adx_test.go`:

```go
package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mxmCherry/openrtb/openrtb2"
)

// dspServer starts a test DSP that always returns a single bid at the given price.
// price <= 0 means no-bid (returns 204).
func dspServer(t *testing.T, price float64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if price <= 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var req openrtb2.BidRequest
		json.NewDecoder(r.Body).Decode(&req)
		impID := ""
		if len(req.Imp) > 0 {
			impID = req.Imp[0].ID
		}
		resp := openrtb2.BidResponse{
			ID: req.ID,
			SeatBid: []openrtb2.SeatBid{{
				Bid: []openrtb2.Bid{{ID: newID(), ImpID: impID, Price: price}},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func makeReq() *openrtb2.BidRequest {
	return &openrtb2.BidRequest{
		ID:  "req-1",
		Imp: []openrtb2.Imp{{ID: "imp-1"}},
	}
}

func TestAuction_SecondPrice(t *testing.T) {
	s1 := dspServer(t, 5.00)
	defer s1.Close()
	s2 := dspServer(t, 3.00)
	defer s2.Close()

	cfg := Config{
		AdxTimeoutMS: 500,
		AdxFloorCPM:  0.50,
		DSPs: []DSPConfig{
			{ID: "dsp-1", URL: s1.URL},
			{ID: "dsp-2", URL: s2.URL},
		},
	}
	adx := NewADX(cfg)
	result := adx.Auction(context.Background(), makeReq())
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.WinnerDSP != "dsp-1" {
		t.Errorf("winner: want dsp-1, got %s", result.WinnerDSP)
	}
	if result.ClearPrice != 3.00 {
		t.Errorf("clear price: want 3.00, got %f", result.ClearPrice)
	}
	if result.WinBid.Price != 5.00 {
		t.Errorf("win bid price: want 5.00, got %f", result.WinBid.Price)
	}
}

func TestAuction_SingleBidClearPriceIsFloor(t *testing.T) {
	s1 := dspServer(t, 4.00)
	defer s1.Close()
	s2 := dspServer(t, 0) // no-bid
	defer s2.Close()

	cfg := Config{
		AdxTimeoutMS: 500,
		AdxFloorCPM:  1.00,
		DSPs: []DSPConfig{
			{ID: "dsp-1", URL: s1.URL},
			{ID: "dsp-2", URL: s2.URL},
		},
	}
	adx := NewADX(cfg)
	result := adx.Auction(context.Background(), makeReq())
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ClearPrice != 1.00 {
		t.Errorf("clear price: want floor 1.00, got %f", result.ClearPrice)
	}
	if result.WinnerDSP != "dsp-1" {
		t.Errorf("winner: want dsp-1, got %s", result.WinnerDSP)
	}
}

func TestAuction_AllNoBid(t *testing.T) {
	s1 := dspServer(t, 0)
	defer s1.Close()
	s2 := dspServer(t, 0)
	defer s2.Close()

	cfg := Config{
		AdxTimeoutMS: 500,
		AdxFloorCPM:  0.50,
		DSPs: []DSPConfig{
			{ID: "dsp-1", URL: s1.URL},
			{ID: "dsp-2", URL: s2.URL},
		},
	}
	adx := NewADX(cfg)
	result := adx.Auction(context.Background(), makeReq())
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestAuction_AllBidsBelowFloor(t *testing.T) {
	s1 := dspServer(t, 0.10)
	defer s1.Close()
	s2 := dspServer(t, 0.20)
	defer s2.Close()

	cfg := Config{
		AdxTimeoutMS: 500,
		AdxFloorCPM:  1.00,
		DSPs: []DSPConfig{
			{ID: "dsp-1", URL: s1.URL},
			{ID: "dsp-2", URL: s2.URL},
		},
	}
	adx := NewADX(cfg)
	result := adx.Auction(context.Background(), makeReq())
	if result != nil {
		t.Errorf("expected nil for all-below-floor, got %+v", result)
	}
}

func TestAuction_DSPTimeout(t *testing.T) {
	// slow DSP: hangs until test is over
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer slow.Close()
	fast := dspServer(t, 3.00)
	defer fast.Close()

	cfg := Config{
		AdxTimeoutMS: 100, // short timeout — slow DSP will miss it
		AdxFloorCPM:  0.50,
		DSPs: []DSPConfig{
			{ID: "slow", URL: slow.URL},
			{ID: "fast", URL: fast.URL},
		},
	}
	adx := NewADX(cfg)
	result := adx.Auction(context.Background(), makeReq())
	if result == nil {
		t.Fatal("expected result from fast DSP, got nil")
	}
	if result.WinnerDSP != "fast" {
		t.Errorf("winner: want fast, got %s", result.WinnerDSP)
	}
}

func TestAuction_NoDSPs(t *testing.T) {
	cfg := Config{AdxTimeoutMS: 200, AdxFloorCPM: 0.50, DSPs: nil}
	adx := NewADX(cfg)
	result := adx.Auction(context.Background(), makeReq())
	if result != nil {
		t.Errorf("expected nil for no DSPs, got %+v", result)
	}
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./internal/ -run "TestAuction" -v
```

Expected: compile error — `NewADX undefined`

- [ ] **Step 3: Implement `internal/adx.go`**

Create `internal/adx.go`:

```go
package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mxmCherry/openrtb/openrtb2"
)

// ADX fans out BidRequests to multiple DSPs and runs a second-price auction.
type ADX struct {
	cfg    Config
	client *http.Client
}

// NewADX creates an ADX with a shared HTTP client.
func NewADX(cfg Config) *ADX {
	return &ADX{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// DSPResult holds the outcome of a single DSP call.
type DSPResult struct {
	DSPID    string
	TopPrice float64 // 0 if no-bid or error
	Bid      *openrtb2.Bid
}

// AuctionResult holds the outcome of a completed second-price auction.
type AuctionResult struct {
	WinnerDSP  string
	WinBid     openrtb2.Bid
	ClearPrice float64
	DSPResults []DSPResult
}

// Auction fans out req to all configured DSPs, collects responses, and
// returns the second-price auction winner. Returns nil if no valid bids.
func (a *ADX) Auction(ctx context.Context, req *openrtb2.BidRequest) *AuctionResult {
	if len(a.cfg.DSPs) == 0 {
		return nil
	}

	timeout := time.Duration(a.cfg.AdxTimeoutMS) * time.Millisecond
	results := make([]DSPResult, len(a.cfg.DSPs))
	var wg sync.WaitGroup

	for i, dsp := range a.cfg.DSPs {
		wg.Add(1)
		go func(idx int, d DSPConfig) {
			defer wg.Done()
			results[idx] = a.callDSP(ctx, d, req, timeout)
		}(i, dsp)
	}
	wg.Wait()

	// collect valid bids above floor
	type candidate struct {
		dspID string
		bid   openrtb2.Bid
	}
	var candidates []candidate
	for _, r := range results {
		if r.Bid != nil && r.Bid.Price >= a.cfg.AdxFloorCPM {
			candidates = append(candidates, candidate{dspID: r.DSPID, bid: *r.Bid})
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	// sort descending by price
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].bid.Price > candidates[j].bid.Price
	})

	winner := candidates[0]
	clearPrice := a.cfg.AdxFloorCPM
	if len(candidates) > 1 {
		clearPrice = candidates[1].bid.Price
	}

	go a.sendWinNotice(winner.dspID, clearPrice)

	return &AuctionResult{
		WinnerDSP:  winner.dspID,
		WinBid:     winner.bid,
		ClearPrice: clearPrice,
		DSPResults: results,
	}
}

// callDSP sends req to a single DSP and returns its top bid (or zero DSPResult on no-bid/error).
func (a *ADX) callDSP(ctx context.Context, dsp DSPConfig, req *openrtb2.BidRequest, timeout time.Duration) DSPResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(req)
	if err != nil {
		slog.Warn("adx: marshal error", "dsp", dsp.ID, "error", err)
		return DSPResult{DSPID: dsp.ID}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, dsp.URL, bytes.NewReader(data))
	if err != nil {
		slog.Warn("adx: build request error", "dsp", dsp.ID, "error", err)
		return DSPResult{DSPID: dsp.ID}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		slog.Warn("adx: dsp error", "dsp", dsp.ID, "error", err)
		return DSPResult{DSPID: dsp.ID}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return DSPResult{DSPID: dsp.ID}
	}
	if resp.StatusCode != http.StatusOK {
		slog.Warn("adx: unexpected dsp status", "dsp", dsp.ID, "status", resp.StatusCode)
		return DSPResult{DSPID: dsp.ID}
	}

	var bidResp openrtb2.BidResponse
	if err := json.NewDecoder(resp.Body).Decode(&bidResp); err != nil {
		slog.Warn("adx: decode error", "dsp", dsp.ID, "error", err)
		return DSPResult{DSPID: dsp.ID}
	}

	// find top bid across all seat bids
	var top *openrtb2.Bid
	for i := range bidResp.SeatBid {
		for j := range bidResp.SeatBid[i].Bid {
			b := &bidResp.SeatBid[i].Bid[j]
			if top == nil || b.Price > top.Price {
				top = b
			}
		}
	}
	if top == nil {
		return DSPResult{DSPID: dsp.ID}
	}
	return DSPResult{DSPID: dsp.ID, TopPrice: top.Price, Bid: top}
}

// sendWinNotice fires a win notice to the DSP that won the auction.
// The DSP URL has "/bid" suffix replaced with "/win"; errors are logged and ignored.
func (a *ADX) sendWinNotice(dspID string, clearPrice float64) {
	var winURL string
	for _, d := range a.cfg.DSPs {
		if d.ID == dspID {
			winURL = d.URL
			break
		}
	}
	if winURL == "" {
		return
	}
	// replace trailing path segment with /win
	// e.g. http://host/bid → http://host/win
	lastSlash := len(winURL) - 1
	for lastSlash >= 0 && winURL[lastSlash] != '/' {
		lastSlash--
	}
	if lastSlash < 0 {
		return
	}
	winURL = winURL[:lastSlash+1] + "win"

	payload, _ := json.Marshal(map[string]float64{"price": clearPrice})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, winURL, bytes.NewReader(payload))
	if err != nil {
		slog.Warn("adx: win notice build error", "dsp", dspID, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		slog.Warn("adx: win notice error", "dsp", dspID, "error", err)
		return
	}
	resp.Body.Close()
}
```

- [ ] **Step 4: Run auction tests to verify they pass**

```bash
go test ./internal/ -run "TestAuction" -v
```

Expected: all 6 `TestAuction_*` tests PASS

- [ ] **Step 5: Run full test suite to check for regressions**

```bash
go test ./internal/ -v
```

Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adx.go internal/adx_test.go
git commit -m "feat: add ADX core auction logic with second-price and fan-out"
```

---

## Task 3: HTTP handler (`internal/adx_handler.go`)

**Files:**
- Create: `internal/adx_handler.go`
- Modify: `internal/adx_test.go` (append handler tests)

- [ ] **Step 1: Append handler tests to `internal/adx_test.go`**

Add after the existing auction tests:

```go
type mockADX struct {
	result *AuctionResult
}

func (m *mockADX) Auction(_ context.Context, _ *openrtb2.BidRequest) *AuctionResult {
	return m.result
}

func TestADXHandler_Bid(t *testing.T) {
	winBid := openrtb2.Bid{ID: "bid-1", ImpID: "imp-1", Price: 5.00}
	mock := &mockADX{result: &AuctionResult{
		WinnerDSP:  "dsp-1",
		WinBid:     winBid,
		ClearPrice: 3.00,
	}}
	h := NewADXHandler(mock)
	body := `{"id":"req-1","imp":[{"id":"imp-1"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/openrtb", strings.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rec.Code)
	}
	var got openrtb2.BidResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.SeatBid) != 1 || len(got.SeatBid[0].Bid) != 1 {
		t.Fatalf("unexpected seatbid structure: %+v", got.SeatBid)
	}
	if got.SeatBid[0].Bid[0].Price != 3.00 {
		t.Errorf("price: want clear price 3.00, got %f", got.SeatBid[0].Bid[0].Price)
	}
}

func TestADXHandler_NoBid(t *testing.T) {
	mock := &mockADX{result: nil}
	h := NewADXHandler(mock)
	body := `{"id":"req-1","imp":[{"id":"imp-1"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/openrtb", strings.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status: want 204, got %d", rec.Code)
	}
}

func TestADXHandler_BadJSON(t *testing.T) {
	mock := &mockADX{result: nil}
	h := NewADXHandler(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/openrtb", strings.NewReader("{{bad"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}

func TestADXHandler_MethodNotAllowed(t *testing.T) {
	mock := &mockADX{result: nil}
	h := NewADXHandler(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/openrtb", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: want 405, got %d", rec.Code)
	}
}
```

Also add `"strings"` to the import block in `adx_test.go`.

- [ ] **Step 2: Run to verify handler tests fail**

```bash
go test ./internal/ -run "TestADXHandler" -v
```

Expected: compile error — `NewADXHandler undefined`

- [ ] **Step 3: Implement `internal/adx_handler.go`**

Create `internal/adx_handler.go`:

```go
package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mxmCherry/openrtb/openrtb2"
)

// adxIface allows handler tests to inject a mock ADX.
type adxIface interface {
	Auction(ctx context.Context, req *openrtb2.BidRequest) *AuctionResult
}

// ADXHandler is an http.Handler for POST /openrtb.
type ADXHandler struct {
	adx adxIface
}

// NewADXHandler creates an HTTP handler backed by the given ADX.
func NewADXHandler(a adxIface) *ADXHandler {
	return &ADXHandler{adx: a}
}

func (h *ADXHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req openrtb2.BidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	result := h.adx.Auction(r.Context(), &req)
	if result == nil {
		slog.Info("adx no-bid",
			"bid_request_id", req.ID,
			"imp_count", len(req.Imp))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// return a BidResponse with the winner's bid at clear price
	winBid := result.WinBid
	winBid.Price = result.ClearPrice
	resp := openrtb2.BidResponse{
		ID:      req.ID,
		SeatBid: []openrtb2.SeatBid{{Seat: result.WinnerDSP, Bid: []openrtb2.Bid{winBid}}},
	}
	slog.Info("adx bid",
		"bid_request_id", req.ID,
		"imp_count", len(req.Imp),
		"winner_dsp", result.WinnerDSP,
		"clear_price", result.ClearPrice)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("adx encode response", "error", err)
	}
}
```

- [ ] **Step 4: Run handler tests to verify they pass**

```bash
go test ./internal/ -run "TestADXHandler" -v
```

Expected: all 4 `TestADXHandler_*` tests PASS

- [ ] **Step 5: Run full test suite**

```bash
go test ./internal/ -v
```

Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/adx_handler.go internal/adx_test.go
git commit -m "feat: add ADX HTTP handler for POST /openrtb"
```

---

## Task 4: Binary entry point (`cmd/adx/main.go`)

**Files:**
- Create: `cmd/adx/main.go`

- [ ] **Step 1: Create `cmd/adx/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mock-bid-server/internal"
)

func main() {
	cfg, err := internal.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if len(cfg.DSPs) == 0 {
		slog.Error("no DSPs configured — add at least one entry under 'dsps:' in config.yaml")
		os.Exit(1)
	}

	slog.Info("adx starting",
		"port", cfg.AdxPort,
		"floor_cpm", cfg.AdxFloorCPM,
		"timeout_ms", cfg.AdxTimeoutMS,
		"dsp_count", len(cfg.DSPs))
	for _, d := range cfg.DSPs {
		slog.Info("dsp registered", "id", d.ID, "url", d.URL)
	}

	adx := internal.NewADX(cfg)
	handler := internal.NewADXHandler(adx)
	mux := http.NewServeMux()
	mux.Handle("/openrtb", handler)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.AdxPort), Handler: mux}

	go func() {
		slog.Info("adx server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("adx server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("adx shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("adx shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("adx stopped")
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
cd /home/ubuntu/projects/mock-bid-server
go build ./cmd/adx/
```

Expected: no errors, produces `./adx` binary

- [ ] **Step 3: Run full test suite to confirm no regressions**

```bash
go test ./...
```

Expected: all tests PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/adx/main.go
git commit -m "feat: add cmd/adx binary entry point"
```

---

## Task 5: Update config.yaml with ADX example config

**Files:**
- Modify: `config.yaml`

- [ ] **Step 1: Add ADX fields to config.yaml**

Replace the contents of `config.yaml` with:

```yaml
# DSP (mock bidder) settings
port: 8080
no_bid_rate: 0.20
min_price_cpm: 0.10
max_price_cpm: 10.00
seat: "mock-seat"

# ADX settings
adx_port: 8090
adx_timeout_ms: 200
adx_floor_cpm: 0.50
dsps:
  - id: dsp-1
    url: http://localhost:8081/bid
  - id: dsp-2
    url: http://localhost:8082/bid
```

- [ ] **Step 2: Verify config loads without error**

```bash
CONFIG_PATH=config.yaml go run ./cmd/adx/ 2>&1 | head -5
```

Expected: log lines showing `adx starting` with `dsp_count=2`, then exits with port-in-use or starts listening (either is fine — the config parsed correctly)

Note: press Ctrl+C if the server starts.

- [ ] **Step 3: Run full test suite one final time**

```bash
go test ./...
```

Expected: all tests PASS

- [ ] **Step 4: Final commit**

```bash
git add config.yaml
git commit -m "config: add ADX and DSP example configuration"
```

---

## Verification

After all tasks are complete, do a final end-to-end smoke test:

```bash
# Terminal 1: start DSP-1 on :8081 (create a temp config with port: 8081)
echo "port: 8081
no_bid_rate: 0.20
min_price_cpm: 0.10
max_price_cpm: 10.00
seat: dsp-1" > /tmp/dsp1.yaml
CONFIG_PATH=/tmp/dsp1.yaml go run ./cmd/server/

# Terminal 2: start DSP-2 on :8082
echo "port: 8082
no_bid_rate: 0.20
min_price_cpm: 0.10
max_price_cpm: 10.00
seat: dsp-2" > /tmp/dsp2.yaml
CONFIG_PATH=/tmp/dsp2.yaml go run ./cmd/server/

# Terminal 3: start ADX on :8090
CONFIG_PATH=config.yaml go run ./cmd/adx/

# Terminal 4: send a test bid request
curl -s -X POST http://localhost:8090/openrtb \
  -H "Content-Type: application/json" \
  -d '{"id":"test-1","imp":[{"id":"imp-1","banner":{"w":300,"h":250}}]}' | jq .
```

Expected: JSON BidResponse with one SeatBid, price = clear price (≤ winning DSP's bid).
