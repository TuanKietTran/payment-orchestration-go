package api

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOpenAPIContractParsesAndSeparatesImplementedRoutes(t *testing.T) {
	b, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			Summary   string                     `json:"summary"`
			Responses map[string]json.RawMessage `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("OpenAPI YAML/JSON parse: %v", err)
	}
	if doc.OpenAPI != "3.0.3" {
		t.Fatalf("version = %q", doc.OpenAPI)
	}
	for _, route := range []string{"/v1/payments", "/v1/payments/{id}"} {
		if len(doc.Paths[route]) == 0 {
			t.Errorf("implemented route missing: %s", route)
		}
	}
	for _, route := range []string{"/v1/payments/{id}/process", "/v1/payments/{id}/capture", "/v1/payments/{id}/refunds", "/v1/reconciliation"} {
		for _, op := range doc.Paths[route] {
			if len(op.Summary) < 8 || op.Summary[:8] != "PLANNED:" {
				t.Errorf("planned route %s not marked planned: %q", route, op.Summary)
			}
		}
	}
	if _, ok := doc.Paths["/v1/payments"]["post"].Responses["400"]; !ok {
		t.Error("create missing documented 400")
	}
	for _, code := range []string{"200", "400", "404"} {
		if _, ok := doc.Paths["/v1/payments/{id}"]["get"].Responses[code]; !ok {
			t.Errorf("get missing status %s", code)
		}
	}
}
