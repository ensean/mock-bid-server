# mock-bid-server

A Go toolkit for testing OpenRTB 2.5 bid pipelines. Contains two binaries:

- **`cmd/server`** — fake DSP/bidder that receives bid requests and responds with randomly priced bids
- **`cmd/generator`** — CLI load tester that fires bid requests at a running server at a configurable RPS

---

## Quick start

```bash
# Build both binaries
go build -o mock-server ./cmd/server/
go build -o mock-generator ./cmd/generator/

# Start the server
./mock-server

# In another terminal, run the generator
./mock-generator --rps 20 --imps 2 --duration 30s
```

---

## Server

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
│   ├── server/main.go       # server entry point
│   └── generator/main.go    # generator entry point
├── internal/
│   ├── config.go            # Config struct + Load()
│   ├── bidder.go            # Bidder logic + newID()
│   ├── handler.go           # HTTP handler for POST /bid
│   └── generator.go         # Generator + runStats + summary printer
├── config.yaml
└── go.mod
```
