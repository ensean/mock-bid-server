# mock-bid-server

A Go toolkit for testing OpenRTB 2.5 bid pipelines. Contains three binaries:

- **`cmd/server`** — fake DSP/bidder that receives bid requests and responds with randomly priced bids
- **`cmd/adx`** — ad exchange that fans out bid requests to multiple DSPs, runs a second-price auction, and returns the clearing-price response
- **`cmd/generator`** — CLI load tester that fires bid requests at a running server at a configurable RPS

---

## Quick start

```bash
# Build all binaries
go build -o mock-server ./cmd/server/
go build -o mock-adx ./cmd/adx/
go build -o mock-generator ./cmd/generator/
```

### Run DSP + Generator directly

```bash
# Start the DSP server
./mock-server

# In another terminal, fire requests at it
./mock-generator --rps 20 --imps 2 --duration 30s
```

### Run the full ADX pipeline

```bash
# Terminal 1 — DSP-1 on :8081
echo 'port: 8081
seat: "dsp-1"' > /tmp/dsp1.yaml
CONFIG_PATH=/tmp/dsp1.yaml ./mock-server

# Terminal 2 — DSP-2 on :8082
echo 'port: 8082
seat: "dsp-2"' > /tmp/dsp2.yaml
CONFIG_PATH=/tmp/dsp2.yaml ./mock-server

# Terminal 3 — ADX on :8090 (reads dsps from config.yaml)
./mock-adx

# Terminal 4 — send a request through the exchange
curl -s -X POST http://localhost:8090/openrtb \
  -H "Content-Type: application/json" \
  -d '{"id":"req-1","imp":[{"id":"imp-1","banner":{"w":300,"h":250}}]}' | jq .
```

---

## Server (mock DSP)

Receives `POST /bid` with an OpenRTB 2.5 `BidRequest` and responds with a random bid or no-bid.

### Configuration

Loaded from `config.yaml` (override path with `CONFIG_PATH` env var):

```yaml
port: 8080
no_bid_rate: 0.20    # probability of returning no-bid (HTTP 204)
min_price_cpm: 0.10
max_price_cpm: 10.00
seat: "mock-seat"
```

### API: `POST /bid`

| Condition | Response |
|---|---|
| Valid request, bid wins | `200 OK` — JSON `BidResponse` |
| No-bid (rate roll or empty `imp`) | `204 No Content` |
| Malformed JSON | `400 Bad Request` |
| Wrong HTTP method | `405 Method Not Allowed` |

Example:

```bash
curl -s -X POST http://localhost:8080/bid \
  -H "Content-Type: application/json" \
  -d '{"id":"req-1","imp":[{"id":"imp-1","banner":{"w":300,"h":250}}]}' | jq .
```

### Logging

Structured key=value logs via `log/slog` to stdout:

```
time=... level=INFO msg="bid" bid_request_id=req-1 imp_count=1 outcome=bid prices=[4.23]
```

### Graceful shutdown

Handles `SIGINT`/`SIGTERM` — drains in-flight requests with a 5-second timeout.

---

## ADX (Ad Exchange)

Receives `POST /openrtb` with an OpenRTB 2.5 `BidRequest`, fans it out to all configured DSPs concurrently, runs a second-price auction, and returns a `BidResponse` with the clearing price.

### Architecture

```
publisher / generator
        │  POST /openrtb
        ▼
      ADX (:8090)
        │  concurrent POST /bid
   ┌────┴────┐
 DSP-1     DSP-2  ...   ← mock-server instances
   └────┬────┘
        │  collect bids
        ▼
  second-price auction
        │
        ├─→ BidResponse to caller (price = clear price)
        └─→ async POST /win to winning DSP
```

### Configuration

ADX fields in `config.yaml`:

```yaml
adx_port: 8090
adx_timeout_ms: 200    # per-DSP call timeout
adx_floor_cpm: 0.50    # minimum bid to enter auction
dsps:
  - id: dsp-1
    url: http://localhost:8081/bid
  - id: dsp-2
    url: http://localhost:8082/bid
```

### API: `POST /openrtb`

| Condition | Response |
|---|---|
| Auction winner found | `200 OK` — JSON `BidResponse` (price = clearing price) |
| No valid bids (all no-bid, all below floor) | `204 No Content` |
| Malformed JSON | `400 Bad Request` |
| Wrong HTTP method | `405 Method Not Allowed` |

### Auction rules

1. The BidRequest is sent to **all** DSPs concurrently, each with an independent timeout (`adx_timeout_ms`).
2. Bids below `adx_floor_cpm` are discarded.
3. **Second-price (Vickrey) auction**: the highest bidder wins, but pays the **second-highest** bid price.
4. If only one valid bid exists, the clearing price equals `adx_floor_cpm`.
5. A fire-and-forget `POST /win` with `{"price": <clear_price>}` is sent to the winning DSP asynchronously.

### Error handling

| Scenario | Behavior |
|---|---|
| DSP timeout / network error | Logged as warn, DSP skipped |
| All DSPs no-bid | `204 No Content` |
| All bids below floor | `204 No Content` |
| Only one valid bid | Clear price = floor |
| Win notice failure | Logged as warn, response unaffected |

### Known limitations

- The mock DSP server does not handle `POST /win` — win notices are silently dropped (DSP returns 404).
- Loss notices are not implemented — losing DSPs are not notified.

---

## Generator

Fires OpenRTB bid requests at a running server at a fixed RPS and prints periodic summaries.

### Flags

| Flag | Default | Description |
|---|---|---|
| `--target` | `http://localhost:8080/bid` | Server URL |
| `--rps` | `10` | Requests per second |
| `--imps` | `2` | Impressions per request |
| `--duration` | `30s` | Run time (`0` = until Ctrl+C) |
| `--interval` | `5s` | Summary print interval |

### Example output

```
[  5s] sent=50   bid=40 (80%)  no-bid=10 (20%)  p50=1.2ms  p99=5.4ms  errors=0
[ 10s] sent=100  bid=80 (80%)  no-bid=20 (20%)  p50=1.1ms  p99=4.8ms  errors=0
```

Handles `SIGINT`/`SIGTERM` — prints a final summary and exits cleanly.

---

## Running tests

```bash
go test ./...
```

## Project layout

```
mock-bid-server/
├── cmd/
│   ├── adx/main.go          # ADX entry point
│   ├── server/main.go        # DSP server entry point
│   └── generator/main.go     # generator entry point
├── internal/
│   ├── config.go             # Config struct + Load()
│   ├── bidder.go             # Bidder logic + newID()
│   ├── handler.go            # HTTP handler for POST /bid
│   ├── adx.go                # ADX fan-out + second-price auction
│   ├── adx_handler.go        # HTTP handler for POST /openrtb
│   └── generator.go          # Generator + runStats + summary printer
├── config.yaml
└── go.mod
```
