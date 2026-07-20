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
	"sync"
	"testing"
)

// pathCountStub records how many times each request path was hit and lets the
// test script a body per path substring.
type pathCountStub struct {
	mu     sync.Mutex
	counts map[string]int
	body   func(path string) (int, string)
}

func newPathStub(body func(path string) (int, string)) *pathCountStub {
	return &pathCountStub{counts: map[string]int{}, body: body}
}

func (s *pathCountStub) transport() *stubTransport {
	return &stubTransport{respond: func(_ int, req *http.Request) (int, string) {
		s.mu.Lock()
		s.counts[req.URL.Path]++
		s.mu.Unlock()
		return s.body(req.URL.Path)
	}}
}

func (s *pathCountStub) count(substr string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for p, c := range s.counts {
		if strings.Contains(p, substr) {
			n += c
		}
	}
	return n
}

func TestEnsureContractNoOpWithoutRateCard(t *testing.T) {
	stub := newPathStub(func(string) (int, string) { return 200, `{"data":[]}` })
	c := newTestClient(t, stub.transport()) // RateCardID unset
	if err := c.EnsureContract(context.Background(), "tea-abc"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	if stub.count("contracts") != 0 {
		t.Fatalf("made %d contract calls with no rate card, want 0 (shadow-only)", stub.count("contracts"))
	}
}

func TestEnsureContractCreatesWhenNone(t *testing.T) {
	stub := newPathStub(func(path string) (int, string) {
		if strings.Contains(path, "contracts/list") {
			return 200, `{"data":[]}` // no existing contract
		}
		return 200, `{"data":{"id":"con_1"}}`
	})
	c := newTestClient(t, stub.transport())
	c.RateCardID = "rate_card_1"

	if err := c.EnsureContract(context.Background(), "tea-abc"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	if got := stub.count("contracts/create"); got != 1 {
		t.Fatalf("contract creates = %d, want 1", got)
	}
	// Second call is cached — no further list/create.
	if err := c.EnsureContract(context.Background(), "tea-abc"); err != nil {
		t.Fatalf("EnsureContract (2nd): %v", err)
	}
	if got := stub.count("contracts/create"); got != 1 {
		t.Fatalf("contract creates after cache = %d, want 1 (idempotent)", got)
	}
}

func TestEnsureContractSkipsWhenAlreadyContracted(t *testing.T) {
	stub := newPathStub(func(path string) (int, string) {
		if strings.Contains(path, "contracts/list") {
			return 200, `{"data":[{"id":"con_existing","customer_id":"tea-abc"}]}`
		}
		return 200, `{"data":{"id":"con_new"}}`
	})
	c := newTestClient(t, stub.transport())
	c.RateCardID = "rate_card_1"

	if err := c.EnsureContract(context.Background(), "tea-abc"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	if got := stub.count("contracts/create"); got != 0 {
		t.Fatalf("contract creates = %d, want 0 (already contracted)", got)
	}
}

func TestCompCustomerCreatesContractAndCredit(t *testing.T) {
	stub := newPathStub(func(path string) (int, string) {
		if strings.Contains(path, "contracts/list") {
			return 200, `{"data":[]}`
		}
		return 200, `{"data":{"id":"x"}}`
	})
	c := newTestClient(t, stub.transport())
	c.RateCardID = "rate_card_1"
	c.USDCreditTypeID = "ct_usd"

	if err := c.CompCustomer(context.Background(), "tea-comp"); err != nil {
		t.Fatalf("CompCustomer: %v", err)
	}
	if got := stub.count("contracts/create"); got != 1 {
		t.Errorf("contract creates = %d, want 1", got)
	}
	if got := stub.count("credits/createGrant"); got != 1 {
		t.Errorf("credit-grant creates = %d, want 1 (Mode B comp)", got)
	}
}

func TestCompCustomerRequiresConfig(t *testing.T) {
	stub := newPathStub(func(string) (int, string) { return 200, `{"data":[]}` })
	c := newTestClient(t, stub.transport()) // no rate card, no credit type
	if err := c.CompCustomer(context.Background(), "tea-x"); err == nil {
		t.Fatal("CompCustomer with no rate card = nil, want an error")
	}
	c.RateCardID = "rate_card_1" // still missing credit type
	if err := c.CompCustomer(context.Background(), "tea-x"); err == nil {
		t.Fatal("CompCustomer with no USD credit type = nil, want an error")
	}
}
