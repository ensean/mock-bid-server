package internal

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mxmCherry/openrtb/openrtb2"
)

type bidderIface interface {
	Bid(req *openrtb2.BidRequest) *openrtb2.BidResponse
}

type Handler struct {
	bidder bidderIface
}

func NewHandler(b bidderIface) *Handler {
	return &Handler{bidder: b}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req openrtb2.BidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp := h.bidder.Bid(&req)
	if resp == nil {
		slog.Info("no-bid", "bid_request_id", req.ID, "imp_count", len(req.Imp), "outcome", "no_bid")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	prices := make([]float64, 0)
	for _, sb := range resp.SeatBid {
		for _, b := range sb.Bid { prices = append(prices, b.Price) }
	}
	slog.Info("bid", "bid_request_id", req.ID, "imp_count", len(req.Imp), "outcome", "bid", "prices", prices)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
