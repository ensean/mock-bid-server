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
