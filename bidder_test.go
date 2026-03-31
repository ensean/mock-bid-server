package main

import (
	"math/rand"
	"testing"

	"github.com/mxmCherry/openrtb/openrtb2"
)

func TestBid_NoBidByRate(t *testing.T) {
	cfg := Config{NoBidRate: 1.0, MinPriceCPM: 0.1, MaxPriceCPM: 10.0, Seat: "test"}
	b := NewBidder(cfg, rand.New(rand.NewSource(42)))

	req := &openrtb2.BidRequest{
		ID:  "req-1",
		Imp: []openrtb2.Imp{{ID: "imp-1"}},
	}
	if resp := b.Bid(req); resp != nil {
		t.Errorf("expected nil for 100%% no-bid rate, got %+v", resp)
	}
}

func TestBid_NoBidEmptyImp(t *testing.T) {
	cfg := Config{NoBidRate: 0.0, MinPriceCPM: 0.1, MaxPriceCPM: 10.0, Seat: "test"}
	b := NewBidder(cfg, rand.New(rand.NewSource(42)))

	req := &openrtb2.BidRequest{ID: "req-1", Imp: []openrtb2.Imp{}}
	if resp := b.Bid(req); resp != nil {
		t.Errorf("expected nil for empty Imp slice, got %+v", resp)
	}
}

func TestBid_ResponseShape(t *testing.T) {
	cfg := Config{NoBidRate: 0.0, MinPriceCPM: 1.0, MaxPriceCPM: 5.0, Seat: "test-seat"}
	b := NewBidder(cfg, rand.New(rand.NewSource(42)))

	req := &openrtb2.BidRequest{
		ID:  "req-abc",
		Imp: []openrtb2.Imp{{ID: "imp-1"}, {ID: "imp-2"}},
	}
	resp := b.Bid(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.ID != "req-abc" {
		t.Errorf("BidResponse.ID: want req-abc, got %s", resp.ID)
	}
	if len(resp.SeatBid) != 1 {
		t.Fatalf("SeatBid count: want 1, got %d", len(resp.SeatBid))
	}
	if resp.SeatBid[0].Seat != "test-seat" {
		t.Errorf("Seat: want test-seat, got %s", resp.SeatBid[0].Seat)
	}
	if len(resp.SeatBid[0].Bid) != 2 {
		t.Fatalf("Bid count: want 2, got %d", len(resp.SeatBid[0].Bid))
	}
}

func TestBid_PriceInRange(t *testing.T) {
	cfg := Config{NoBidRate: 0.0, MinPriceCPM: 2.0, MaxPriceCPM: 4.0, Seat: "test"}
	b := NewBidder(cfg, rand.New(rand.NewSource(99)))

	req := &openrtb2.BidRequest{
		ID:  "req-1",
		Imp: []openrtb2.Imp{{ID: "imp-1"}, {ID: "imp-2"}, {ID: "imp-3"}},
	}
	resp := b.Bid(req)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	for _, bid := range resp.SeatBid[0].Bid {
		if bid.Price < 2.0 || bid.Price > 4.0 {
			t.Errorf("price %f out of range [2.0, 4.0]", bid.Price)
		}
		if bid.ImpID == "" {
			t.Error("bid ImpID must not be empty")
		}
		if bid.ID == "" {
			t.Error("bid ID must not be empty")
		}
	}
}

func TestBid_ImpIDMapping(t *testing.T) {
	cfg := Config{NoBidRate: 0.0, MinPriceCPM: 1.0, MaxPriceCPM: 2.0, Seat: "test"}
	b := NewBidder(cfg, rand.New(rand.NewSource(1)))

	req := &openrtb2.BidRequest{
		ID:  "req-1",
		Imp: []openrtb2.Imp{{ID: "imp-A"}, {ID: "imp-B"}},
	}
	resp := b.Bid(req)
	impIDs := map[string]bool{}
	for _, bid := range resp.SeatBid[0].Bid {
		impIDs[bid.ImpID] = true
	}
	if !impIDs["imp-A"] || !impIDs["imp-B"] {
		t.Errorf("expected bids for imp-A and imp-B, got %v", impIDs)
	}
}
