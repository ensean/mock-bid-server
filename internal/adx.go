package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
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
	if cfg.AdxTimeoutMS <= 0 {
		cfg.AdxTimeoutMS = 200
	}
	return &ADX{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// DSPResult holds the outcome of a single DSP call.
type DSPResult struct {
	DSPID string
	Bid   *openrtb2.Bid
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
	return DSPResult{DSPID: dsp.ID, Bid: top}
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
	u, err := url.Parse(winURL)
	if err != nil || u.Host == "" {
		slog.Warn("adx: win notice bad url", "dsp", dspID, "url", winURL)
		return
	}
	u.Path = "/win"
	winURL = u.String()

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
