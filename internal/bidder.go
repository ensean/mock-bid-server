package internal

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"

	"github.com/mxmCherry/openrtb/openrtb2"
)

type randSource interface {
	Float64() float64
}

type Bidder struct {
	cfg Config
	rng randSource
}

func NewBidder(cfg Config, rng randSource) *Bidder {
	return &Bidder{cfg: cfg, rng: rng}
}

func (b *Bidder) Bid(req *openrtb2.BidRequest) *openrtb2.BidResponse {
	if len(req.Imp) == 0 { return nil }
	if b.rng.Float64() < b.cfg.NoBidRate { return nil }
	bids := make([]openrtb2.Bid, 0, len(req.Imp))
	for _, imp := range req.Imp {
		price := b.cfg.MinPriceCPM + b.rng.Float64()*(b.cfg.MaxPriceCPM-b.cfg.MinPriceCPM)
		bids = append(bids, openrtb2.Bid{ID: newID(), ImpID: imp.ID, Price: price})
	}
	return &openrtb2.BidResponse{
		ID: req.ID,
		SeatBid: []openrtb2.SeatBid{{Seat: b.cfg.Seat, Bid: bids}},
	}
}

func newID() string {
	b := make([]byte, 8)
	if _, err := cryptorand.Read(b); err != nil {
		return fmt.Sprintf("%d", rand.Int63())
	}
	return hex.EncodeToString(b)
}
