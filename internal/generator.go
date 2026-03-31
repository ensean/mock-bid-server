package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mxmCherry/openrtb/openrtb2"
)

var bannerSizes = [][2]int64{
	{300, 250},
	{728, 90},
	{320, 50},
	{160, 600},
}

// Generator sends OpenRTB bid requests to Target at a fixed RPS and prints
// periodic outcome/latency summaries.
type Generator struct {
	Target   string
	RPS      int
	Imps     int
	Duration time.Duration // 0 = run until context is cancelled
	Interval time.Duration
	client   *http.Client
}

// runStats accumulates outcomes and latencies across goroutines.
type runStats struct {
	mu        sync.Mutex
	bids      int
	noBids    int
	errors    int
	latencies []time.Duration
}

func (s *runStats) record(outcome string, latency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latencies = append(s.latencies, latency)
	switch outcome {
	case "bid":
		s.bids++
	case "nobid":
		s.noBids++
	default:
		s.errors++
	}
}

// snapshot returns a consistent view of the stats and p50/p99 latency percentiles.
func (s *runStats) snapshot() (bids, noBids, errors int, p50, p99 time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bids, noBids, errors = s.bids, s.noBids, s.errors
	if len(s.latencies) == 0 {
		return
	}
	sorted := make([]time.Duration, len(s.latencies))
	copy(sorted, s.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	p50 = sorted[(n-1)*50/100]
	p99 = sorted[(n-1)*99/100]
	return
}

// Run starts the generator and blocks until the duration elapses or ctx is cancelled.
func (g *Generator) Run(ctx context.Context) error {
	if g.RPS <= 0 {
		return fmt.Errorf("rps must be > 0, got %d", g.RPS)
	}
	if g.Imps <= 0 {
		return fmt.Errorf("imps must be > 0, got %d", g.Imps)
	}
	if g.client == nil {
		g.client = &http.Client{Timeout: 5 * time.Second}
	}

	if g.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.Duration)
		defer cancel()
	}

	st := &runStats{}
	start := time.Now()

	rateTicker := time.NewTicker(time.Second / time.Duration(g.RPS))
	defer rateTicker.Stop()

	summaryTicker := time.NewTicker(g.Interval)
	defer summaryTicker.Stop()

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			printSummary(st, time.Since(start))
			return nil
		case <-rateTicker.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				g.sendOne(st)
			}()
		case <-summaryTicker.C:
			printSummary(st, time.Since(start))
		}
	}
}

func (g *Generator) sendOne(st *runStats) {
	req := g.buildRequest()
	data, err := json.Marshal(req)
	if err != nil {
		st.record("error", 0)
		return
	}

	t0 := time.Now()
	resp, err := g.client.Post(g.Target, "application/json", bytes.NewReader(data))
	latency := time.Since(t0)

	if err != nil {
		st.record("error", latency)
		return
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		st.record("bid", latency)
	case http.StatusNoContent:
		st.record("nobid", latency)
	default:
		st.record("error", latency)
	}
}

func (g *Generator) buildRequest() openrtb2.BidRequest {
	imps := make([]openrtb2.Imp, g.Imps)
	for i := range imps {
		size := bannerSizes[rand.Intn(len(bannerSizes))]
		w, h := size[0], size[1]
		imps[i] = openrtb2.Imp{
			ID:     newID(),
			Banner: &openrtb2.Banner{W: &w, H: &h},
		}
	}
	return openrtb2.BidRequest{
		ID:  newID(),
		Imp: imps,
	}
}

func printSummary(st *runStats, elapsed time.Duration) {
	bids, noBids, errors, p50, p99 := st.snapshot()
	sent := bids + noBids + errors
	bidPct, noBidPct := 0.0, 0.0
	if sent > 0 {
		bidPct = float64(bids) / float64(sent) * 100
		noBidPct = float64(noBids) / float64(sent) * 100
	}
	fmt.Printf("[%4ds] sent=%-4d bid=%d (%.0f%%)  no-bid=%d (%.0f%%)  p50=%s  p99=%s  errors=%d\n",
		int(elapsed.Seconds()), sent, bids, bidPct, noBids, noBidPct,
		p50.Round(100*time.Microsecond), p99.Round(100*time.Microsecond), errors)
}
