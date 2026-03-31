package main

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"

	"github.com/mxmCherry/openrtb/openrtb2"
)

// randSource is the interface satisfied by *rand.Rand — used for injection in tests.
type randSource interface {
	Float64() float64
}

// Bidder holds configuration and a random source for bid generation.
type Bidder struct {
	cfg Config
	rng randSource
}

// NewBidder creates a Bidder with the given config and random source.
// Pass rand.New(rand.NewSource(seed)) for deterministic tests.
func NewBidder(cfg Config, rng randSource) *Bidder {
	return &Bidder{cfg: cfg, rng: rng}
}

// Bid processes a BidRequest and returns a BidResponse, or nil for no-bid.
func (b *Bidder) Bid(req *openrtb2.BidRequest) *openrtb2.BidResponse {
	if len(req.Imp) == 0 {
		return nil
	}
	if b.rng.Float64() < b.cfg.NoBidRate {
		return nil
	}

	bids := make([]openrtb2.Bid, 0, len(req.Imp))
	for _, imp := range req.Imp {
		price := b.cfg.MinPriceCPM + b.rng.Float64()*(b.cfg.MaxPriceCPM-b.cfg.MinPriceCPM)
		bids = append(bids, openrtb2.Bid{
			ID:    newID(),
			ImpID: imp.ID,
			Price: price,
		})
	}

	return &openrtb2.BidResponse{
		ID: req.ID,
		SeatBid: []openrtb2.SeatBid{
			{
				Seat: b.cfg.Seat,
				Bid:  bids,
			},
		},
	}
}

// newID returns a random 16-byte hex string suitable for bid IDs.
func newID() string {
	b := make([]byte, 8)
	if _, err := cryptorand.Read(b); err != nil {
		return fmt.Sprintf("%d", rand.Int63())
	}
	return hex.EncodeToString(b)
}
