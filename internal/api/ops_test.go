package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakePing struct{ err error }

func (f fakePing) Ping(context.Context) error { return f.err }

func TestOpsProbes(t *testing.T) {
	cases := []struct {
		name  string
		store Pinger
		path  string
		want  int
	}{
		{"live", fakePing{errors.New("down")}, "/healthz", 200},
		{"ready", fakePing{}, "/readyz", 200},
		{"ready no store", nil, "/readyz", 200},
		{"not ready", fakePing{errors.New("down")}, "/readyz", 503},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := chi.NewRouter()
			MountOps(r, tc.store)
			if rec := do(r, http.MethodGet, tc.path, ""); rec.Code != tc.want {
				t.Fatalf("%s = %d %s", tc.path, rec.Code, rec.Body)
			}
		})
	}
}

func TestCreateRejectsOversizedBody(t *testing.T) {
	h := listReconSetup(nil)
	body := `{"idempotency_key":"` + strings.Repeat("x", 2<<20) + `"}`
	if rec := do(h, http.MethodPost, "/payments", body); rec.Code != 400 {
		t.Fatalf("oversized create = %d", rec.Code)
	}
}
