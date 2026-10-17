package downstream

import (
	"context"
	"encoding/json"
	"github.com/shopspring/decimal"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFraudHTTPContract(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/score" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		if in["event_id"] != "pay-1" || in["account"] != "acct" || in["amount"] != "100.00" || in["currency"] != "USD" {
			t.Errorf("request: %#v", in)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"event_id":"pay-1","score":2,"decision":"ALLOW","reasons":[]}`))
	}))
	defer s.Close()
	c, err := NewHTTP(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	d, err := c.Score(context.Background(), "pay-1", "acct", decimal.NewFromInt(100), "USD")
	if err != nil || d.Decision != "ALLOW" {
		t.Fatalf("%+v %v", d, err)
	}
}
func TestHTTPClientHasBoundedTimeout(t *testing.T) {
	c, err := NewHTTP("http://localhost:1")
	if err != nil {
		t.Fatal(err)
	}
	if c.client.Timeout <= 0 || c.client.Timeout > 5*time.Second {
		t.Fatalf("timeout %s", c.client.Timeout)
	}
}
