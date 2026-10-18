package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Pinger reports whether a dependency is reachable.
type Pinger interface {
	Ping(context.Context) error
}

// MountOps registers unauthenticated liveness and readiness probes. Liveness
// never touches dependencies; readiness requires the payment store to answer.
func MountOps(r chi.Router, store Pinger) {
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if store != nil {
			if err := store.Ping(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"unavailable"}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
}
