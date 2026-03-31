# mock-bid-server

A Go HTTP server that acts as a fake DSP/bidder for [OpenRTB 2.5](https://www.iab.com/wp-content/uploads/2016/03/OpenRTB-API-Specification-Version-2-5-FINAL.pdf). It receives bid requests and responds with randomly priced bids or no-bids. Intended for testing SSP integrations and bid request pipelines.

## Quick start

```bash
go build -o mock-bid-server .
./mock-bid-server
```

The server starts on port 8080 by default.

Send a bid request:

```bash
curl -s -X POST http://localhost:8080/bid \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-req-001",
    "imp": [
      {"id": "imp-1", "banner": {"w": 300, "h": 250}},
      {"id": "imp-2", "banner": {"w": 728, "h": 90}}
    ],
    "site": {"page": "https://example.com"}
  }' | jq .
```

Example response (HTTP 200, ~80% of the time):

```json
{
  "id": "test-req-001",
  "seatbid": [
    {
      "bid": [
        {"id": "a3f9c2d1e8b74a56", "impid": "imp-1", "price": 4.23},
        {"id": "7b1e4f8a2c9d0e3f", "impid": "imp-2", "price": 7.81}
      ],
      "seat": "mock-seat"
    }
  ]
}
```

Or HTTP 204 with empty body (~20% of the time, the configurable no-bid rate).

## Configuration

Configuration is loaded from `config.yaml` at startup. Override the path with the `CONFIG_PATH` environment variable.

```yaml
port: 8080          # listening port
no_bid_rate: 0.20   # probability [0,1) of returning no-bid (HTTP 204)
min_price_cpm: 0.10 # minimum bid price in CPM
max_price_cpm: 10.00 # maximum bid price in CPM
seat: "mock-seat"   # seat identifier in the bid response
```

```bash
CONFIG_PATH=/etc/mock-bid-server/config.yaml ./mock-bid-server
```

## API

### `POST /bid`

Accepts an OpenRTB 2.5 `BidRequest` JSON body.

| Condition | Response |
|---|---|
| Valid request, bid wins | `200 OK` — JSON `BidResponse` |
| No-bid (rate roll or empty `imp`) | `204 No Content` |
| Malformed JSON | `400 Bad Request` — plain-text error |
| Wrong HTTP method | `405 Method Not Allowed` |

Each bid in the response contains:
- `id` — random 16-character hex string
- `impid` — echoes the corresponding `imp.id` from the request
- `price` — random float64 in `[min_price_cpm, max_price_cpm]`

## Logging

Structured key=value logs are written to stdout via `log/slog`.

```
time=2026-03-31T01:54:11Z level=INFO msg="config loaded" port=8080 no_bid_rate=0.2 ...
time=2026-03-31T01:54:11Z level=INFO msg="server starting" addr=:8080
time=2026-03-31T01:54:15Z level=INFO msg="bid" bid_request_id=test-req-001 imp_count=2 outcome=bid prices=[4.23,7.81]
```

## Graceful shutdown

The server handles `SIGINT` and `SIGTERM`. In-flight requests are drained with a 5-second timeout before exit.

## Running tests

```bash
go test ./... -v
```

## Project layout

```
mock-bid-server/
├── main.go          # entry point: load config, wire dependencies, start server
├── config.go        # Config struct + Load() from YAML
├── bidder.go        # pure bid logic: no-bid roll, price generation
├── handler.go       # HTTP handler for POST /bid
├── config.yaml      # default configuration
└── go.mod
```
