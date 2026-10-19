package playback

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetriesTransientFailure(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{
		Method: http.MethodPost, Path: "/v1/x", Body: []byte(`{"n":1}`),
	}}, Policy{MaxAttempts: 3, BaseDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 || out[0].Attempts != 3 || out[0].Status != http.StatusOK || string(out[0].Body) != "ok" {
		t.Fatalf("hits %d outcome %+v", hits, out[0])
	}
}

func TestDoesNotRetryClientError(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{
		Method: http.MethodPost, Path: "/v1/x",
	}}, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 || out[0].Attempts != 1 || out[0].Status != http.StatusBadRequest {
		t.Fatalf("hits %d outcome %+v", hits, out[0])
	}
}

func TestAttemptTimeoutRetriesAndParentCancellationIsFinal(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			time.Sleep(40 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{Method: http.MethodGet, Path: "/"}}, Policy{MaxAttempts: 2, AttemptTimeout: 10 * time.Millisecond})
	if err != nil || out[0].Attempts != 2 || out[0].Status != http.StatusOK {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Play(ctx, srv.Client(), srv.URL, []Call{{Method: http.MethodGet, Path: "/"}}, DefaultPolicy())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want canceled", err)
	}
}

func TestResponseBodyReadUsesLiveAttemptContext(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Write([]byte("x"))
			w.(http.Flusher).Flush()
			time.Sleep(40 * time.Millisecond)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{Method: http.MethodGet, Path: "/"}}, Policy{MaxAttempts: 2, AttemptTimeout: 10 * time.Millisecond})
	if err != nil || out[0].Status != http.StatusOK || string(out[0].Body) != "ok" {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestStopsAfterMaxAttempts(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	out, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{
		Method: http.MethodGet, Path: "/v1/x",
	}}, Policy{MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 || out[0].Attempts != 2 || out[0].Status != http.StatusServiceUnavailable {
		t.Fatalf("hits %d outcome %+v", hits, out[0])
	}
}
