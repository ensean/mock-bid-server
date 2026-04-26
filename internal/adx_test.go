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
