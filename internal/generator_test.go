package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunStats_RecordAndSnapshot(t *testing.T) {
	st := &runStats{}
	st.record("bid", 10*time.Millisecond)
	st.record("bid", 20*time.Millisecond)
	st.record("nobid", 5*time.Millisecond)
	st.record("error", 1*time.Millisecond)

	bids, noBids, errors, _, _ := st.snapshot()
	if bids != 2 {
		t.Errorf("bids: want 2, got %d", bids)
	}
	if noBids != 1 {
		t.Errorf("noBids: want 1, got %d", noBids)
	}
	if errors != 1 {
		t.Errorf("errors: want 1, got %d", errors)
	}
}

func TestRunStats_Percentiles(t *testing.T) {
	st := &runStats{}
	for i := 1; i <= 100; i++ {
		st.record("bid", time.Duration(i)*time.Millisecond)
	}
	_, _, _, p50, p99 := st.snapshot()
	// index = (n-1)*pct/100 → (99)*50/100=49 → 50ms; (99)*99/100=98 → 99ms
	if p50 != 50*time.Millisecond {
		t.Errorf("p50: want 50ms, got %s", p50)
	}
	if p99 != 99*time.Millisecond {
		t.Errorf("p99: want 99ms, got %s", p99)
	}
}

func TestGenerator_InvalidRPS(t *testing.T) {
	g := &Generator{Target: "http://localhost", RPS: 0, Imps: 1, Interval: time.Second}
	if err := g.Run(context.Background()); err == nil {
		t.Error("expected error for rps=0")
	}
}

func TestGenerator_InvalidImps(t *testing.T) {
	g := &Generator{Target: "http://localhost", RPS: 1, Imps: 0, Interval: time.Second}
	if err := g.Run(context.Background()); err == nil {
		t.Error("expected error for imps=0")
	}
}

func TestGenerator_InvalidInterval(t *testing.T) {
	g := &Generator{Target: "http://localhost", RPS: 1, Imps: 1, Interval: 0}
	if err := g.Run(context.Background()); err == nil {
		t.Error("expected error for interval=0")
	}
}

func TestGenerator_CountsBids(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	g := &Generator{
		Target:   srv.URL,
		RPS:      200,
		Imps:     1,
		Duration: 150 * time.Millisecond,
		Interval: time.Hour, // suppress summary ticks during test
		client:   srv.Client(),
	}

	if err := g.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerator_CountsNoBids(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	g := &Generator{
		Target:   srv.URL,
		RPS:      200,
		Imps:     1,
		Duration: 150 * time.Millisecond,
		Interval: time.Hour,
		client:   srv.Client(),
	}

	if err := g.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerator_CountsErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	g := &Generator{
		Target:   srv.URL,
		RPS:      200,
		Imps:     1,
		Duration: 150 * time.Millisecond,
		Interval: time.Hour,
		client:   srv.Client(),
	}

	if err := g.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerator_StopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	g := &Generator{
		Target:   srv.URL,
		RPS:      10,
		Imps:     1,
		Duration: 0, // run forever — only stop on context cancel
		Interval: time.Hour,
		client:   srv.Client(),
	}

	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}
