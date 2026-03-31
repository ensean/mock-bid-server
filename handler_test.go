package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mxmCherry/openrtb/openrtb2"
)

// mockBidder is a test double for Bidder.
type mockBidder struct {
	resp *openrtb2.BidResponse
}

func (m *mockBidder) Bid(_ *openrtb2.BidRequest) *openrtb2.BidResponse {
	return m.resp
}

func TestHandler_BadJSON(t *testing.T) {
	h := NewHandler(&mockBidder{resp: nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bid", strings.NewReader("not-json{{"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}

func TestHandler_NoBid(t *testing.T) {
	h := NewHandler(&mockBidder{resp: nil})
	body := `{"id":"req-1","imp":[{"id":"imp-1"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bid", strings.NewReader(body))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status: want 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body: want empty, got %q", rec.Body.String())
	}
}

func TestHandler_Bid(t *testing.T) {
	bidResp := &openrtb2.BidResponse{
		ID: "req-1",
		SeatBid: []openrtb2.SeatBid{{
			Seat: "mock-seat",
			Bid:  []openrtb2.Bid{{ID: "bid-1", ImpID: "imp-1", Price: 3.5}},
		}},
	}
	h := NewHandler(&mockBidder{resp: bidResp})
	body := `{"id":"req-1","imp":[{"id":"imp-1"}]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bid", strings.NewReader(body))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %s", ct)
	}
	var got openrtb2.BidResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "req-1" {
		t.Errorf("BidResponse.ID: want req-1, got %s", got.ID)
	}
	if len(got.SeatBid) != 1 || len(got.SeatBid[0].Bid) != 1 {
		t.Errorf("unexpected seatbid/bid count: %+v", got.SeatBid)
	}
	if got.SeatBid[0].Bid[0].Price != 3.5 {
		t.Errorf("price: want 3.5, got %f", got.SeatBid[0].Bid[0].Price)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	h := NewHandler(&mockBidder{resp: nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bid", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: want 405, got %d", rec.Code)
	}
}
