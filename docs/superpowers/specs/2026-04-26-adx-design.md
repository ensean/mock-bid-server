# ADX Module Design

Date: 2026-04-26

## Overview

Add an ADX (Ad Exchange) module that acts as a real-time auction broker between publishers and DSPs. The ADX receives a BidRequest from a publisher or load generator, fans it out to all configured DSPs concurrently, runs a second-price auction, and returns a BidResponse with the clearing price.

## Architecture

```
publisher/generator
        │ POST /openrtb
        ▼
  cmd/adx/main.go          ← HTTP server, signal handling, graceful shutdown
        │
  internal/adx.go          ← fan-out, second-price auction, win notice
  internal/adx_handler.go  ← HTTP handler for POST /openrtb
        │
  ┌─────┴──────┐
  DSP-1        DSP-2  ...  ← existing mock-bid-server instances
```

Mirrors the existing `cmd/server` → `internal/bidder` + `internal/handler` structure exactly.

## Configuration

New fields added to `Config` (same `config.yaml`, same `Load()` function):

```yaml
adx_port: 8090
adx_timeout_ms: 200
adx_floor_cpm: 0.50
dsps:
  - id: dsp-1
    url: http://localhost:8081/bid
  - id: dsp-2
    url: http://localhost:8082/bid
```

Go structs added to `internal/config.go`:

```go
type DSPConfig struct {
    ID  string `yaml:"id"`
    URL string `yaml:"url"`
}

// Fields appended to Config:
AdxPort      int         `yaml:"adx_port"`
AdxTimeoutMS int         `yaml:"adx_timeout_ms"`
AdxFloorCPM  float64     `yaml:"adx_floor_cpm"`
DSPs         []DSPConfig `yaml:"dsps"`
```

Defaults: `AdxPort=8090`, `AdxTimeoutMS=200`, `AdxFloorCPM=0.50`.

## Core Logic (`internal/adx.go`)

### Struct

```go
type ADX struct {
    cfg    Config
    client *http.Client
}
```

### `Auction(ctx, req) *AuctionResult`

1. **Fan-out**: send identical BidRequest to all DSPs concurrently; each call uses a derived context with `AdxTimeoutMS` deadline.
2. **Collect**: gather responses via channel; wait for all goroutines.
3. **Filter**: discard bids below `AdxFloorCPM`.
4. **Second-price auction**:
   - Sort valid bids descending by price.
   - Winner = highest bid.
   - Clear price = second-highest price, or `AdxFloorCPM` if only one valid bid.
5. **Win notice**: fire-and-forget `POST /win` to winning DSP URL with `{"price": clearPrice}`; errors are logged as warn and ignored.
6. **Return** `AuctionResult`:
   - `WinnerDSP string`
   - `WinBid openrtb2.Bid`
   - `ClearPrice float64`
   - `DSPResults []DSPResult` (per-DSP: id, status, top bid price)

### `AuctionResult` nil cases

Returns `nil` when: all DSPs no-bid, all bids below floor, or zero DSPs configured.

## HTTP Handler (`internal/adx_handler.go`)

- `POST /openrtb`: decode BidRequest → call `adx.Auction()` → encode BidResponse (winner's bid with `Price` replaced by `ClearPrice`).
- No winner → 204 No Content.
- Decode error → 400 Bad Request.
- Wrong method → 405.
- Logs structured outcome via `log/slog`: winner DSP, clear price, total DSPs contacted, bid count.

## `cmd/adx/main.go`

- Load `Config`, validate `len(cfg.DSPs) > 0` (exit 1 if empty).
- Construct `ADX` + `ADXHandler`, register `POST /openrtb` on `AdxPort`.
- Log DSP list at startup.
- Graceful shutdown: SIGINT/SIGTERM → `http.Server.Shutdown` with 5s timeout.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| DSP timeout / network error | log warn, skip DSP |
| All DSPs no-bid | return 204 |
| All bids below floor | return 204 |
| Only one valid bid | clear price = AdxFloorCPM |
| BidRequest decode failure | return 400 |
| Win notice failure | log warn, continue |

## Testing (`internal/adx_test.go`)

Use `httptest.NewServer` to mock DSPs. Cover:

- Second-price calculation (multiple DSPs, verify clear price = 2nd highest)
- Floor price filtering (all bids below floor → nil result)
- Single valid bid (clear price = floor)
- All no-bid DSPs (nil result)
- DSP timeout (one DSP hangs → still auction with remaining responses)
- Full HTTP handler: 200 with correct BidResponse, 204 on no-bid, 400 on bad body

## File Inventory

| File | Action |
|------|--------|
| `internal/config.go` | Add `DSPConfig`, `AdxPort`, `AdxTimeoutMS`, `AdxFloorCPM`, `DSPs` fields + defaults |
| `internal/adx.go` | New: `ADX`, `AuctionResult`, `DSPResult`, `Auction()` |
| `internal/adx_handler.go` | New: `ADXHandler`, `ServeHTTP()` |
| `internal/adx_test.go` | New: unit + handler tests |
| `cmd/adx/main.go` | New: binary entry point |
| `config.yaml` | Add ADX + DSP example config |
