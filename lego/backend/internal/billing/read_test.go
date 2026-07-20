/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package billing

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// routed builds a stub that answers costs vs invoices requests from two canned
// bodies (empty string ⇒ that route returns the given HTTP status with no body).
func routedClient(t *testing.T, costsStatus int, costsBody string, invStatus int, invBody string) *Client {
	t.Helper()
	stub := &stubTransport{respond: func(_ int, req *http.Request) (int, string) {
		switch {
		case strings.Contains(req.URL.Path, "/costs"):
			return costsStatus, costsBody
		case strings.Contains(req.URL.Path, "/invoices"):
			return invStatus, invBody
		default:
			return 200, "{}"
		}
	}}
	return newTestClient(t, stub)
}

func TestBillingForContractedCustomer(t *testing.T) {
	costs := `{"data":[
		{"start_timestamp":"2026-07-01T00:00:00Z","end_timestamp":"2026-07-02T00:00:00Z","credit_types":{"ct1":{"cost":300,"name":"USD (cents)"}}},
		{"start_timestamp":"2026-07-02T00:00:00Z","end_timestamp":"2026-07-03T00:00:00Z","credit_types":{"ct1":{"cost":200,"name":"USD (cents)"}}}
	]}`
	invoices := `{"data":[
		{"id":"inv_1","status":"FINALIZED","total":1200,"credit_type":{"id":"ct1","name":"USD (cents)"},"start_timestamp":"2026-06-01T00:00:00Z","end_timestamp":"2026-07-01T00:00:00Z"}
	]}`
	c := routedClient(t, 200, costs, 200, invoices)

	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	b, err := c.BillingFor(context.Background(), "tea-abc", start, start.AddDate(0, 0, 15))
	if err != nil {
		t.Fatalf("BillingFor: %v", err)
	}
	if b == nil || b.CurrentCost == nil {
		t.Fatalf("billing = %+v, want a current cost", b)
	}
	// 300 + 200 cents = $5.00.
	if b.CurrentCost.AmountUSD != "5.00" || b.CurrentCost.Currency != "USD" {
		t.Errorf("currentCost = %+v, want 5.00 USD", b.CurrentCost)
	}
	if len(b.Invoices) != 1 || b.Invoices[0].AmountUSD != "12.00" || b.Invoices[0].Status != "FINALIZED" {
		t.Errorf("invoices = %+v, want one FINALIZED $12.00", b.Invoices)
	}
}

func TestBillingForNoContractReturnsNil(t *testing.T) {
	c := routedClient(t, 200, `{"data":[]}`, 200, `{"data":[]}`)
	b, err := c.BillingFor(context.Background(), "tea-none", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("BillingFor: %v", err)
	}
	if b != nil {
		t.Fatalf("billing = %+v, want nil (no contract ⇒ estimate-only)", b)
	}
}

func TestBillingForDraftInvoicesOmitted(t *testing.T) {
	invoices := `{"data":[
		{"id":"inv_draft","status":"DRAFT","total":9900,"credit_type":{"id":"ct1","name":"USD (cents)"},"start_timestamp":"2026-07-01T00:00:00Z","end_timestamp":"2026-08-01T00:00:00Z"},
		{"id":"inv_final","status":"FINALIZED","total":4200,"credit_type":{"id":"ct1","name":"USD (cents)"},"start_timestamp":"2026-06-01T00:00:00Z","end_timestamp":"2026-07-01T00:00:00Z"}
	]}`
	c := routedClient(t, 200, `{"data":[]}`, 200, invoices)
	b, err := c.BillingFor(context.Background(), "tea-abc", time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("BillingFor: %v", err)
	}
	if b == nil || len(b.Invoices) != 1 || b.Invoices[0].ID != "inv_final" {
		t.Fatalf("invoices = %+v, want only the FINALIZED one", b)
	}
}

func TestBillingForDegradedReturnsError(t *testing.T) {
	// Metronome 5xx on costs ⇒ error surfaces (caller logs + falls back to
	// estimate-only, never a 500). The retry loop exhausts then errors.
	c := routedClient(t, http.StatusInternalServerError, `{"message":"down"}`, 200, `{"data":[]}`)
	_, err := c.BillingFor(context.Background(), "tea-abc", time.Now().Add(-time.Hour), time.Now())
	if err == nil {
		t.Fatal("BillingFor on Metronome 5xx = nil error, want a degraded error")
	}
}
