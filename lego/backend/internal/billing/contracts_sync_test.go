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
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// stripeSubscriptionItemsJSON lists one metered item per catalog index in
// [0, n), on stripePriceCatalogJSON's price_NN / mtr_NN ids. priceOverride
// replaces an index's Price id (same meter: a retired Price); an index mapped
// to "" is left off the Subscription.
func stripeSubscriptionItemsJSON(t *testing.T, n int, priceOverride map[int]string) string {
	t.Helper()
	data := make([]map[string]any, 0, n)
	for i := range n {
		priceID := fmt.Sprintf("price_%02d", i)
		if override, ok := priceOverride[i]; ok {
			if override == "" {
				continue
			}
			priceID = override
		}
		data = append(data, map[string]any{
			"id":     fmt.Sprintf("si_%02d", i),
			"object": "subscription_item",
			"price": map[string]any{
				"id":     priceID,
				"object": "price",
				"recurring": map[string]any{
					"interval": "month", "interval_count": 1, "usage_type": "metered",
					"meter": fmt.Sprintf("mtr_%02d", i),
				},
			},
		})
	}
	b, err := json.Marshal(map[string]any{"object": "list", "data": data, "has_more": false, "url": "/v1/subscription_items"})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func existingContractStub(t *testing.T, items string) (*StripeClient, *stripeStub) {
	t.Helper()
	catalog := stripePriceCatalogJSON(t, pricing.Default.BillableMeterNames())
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodGet && path == "/v1/subscription_items":
			return 200, items
		case method == http.MethodPost && path == "/v1/subscription_items":
			return 200, `{"id":"si_new","object":"subscription_item"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	return c, stub
}

// TestStripeEnsureContractAddsCatalogDimensionsToExistingSubscription is the
// w2/m171 revenue gap: a Subscription created before the 1c-2g / pro-4gb /
// basic-4gb Postgres Prices existed gains exactly those items, without
// proration, idempotently, and only once per process.
func TestStripeEnsureContractAddsCatalogDimensionsToExistingSubscription(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	missing := map[int]string{}
	var wantPrices []string
	for i, name := range names {
		if strings.HasPrefix(name, "instance_seconds.postgres.") && slices.Contains([]string{"1c-2g", "pro-4gb", "basic-4gb"}, strings.TrimPrefix(name, "instance_seconds.postgres.")) {
			missing[i] = ""
			wantPrices = append(wantPrices, fmt.Sprintf("price_%02d", i))
		}
	}
	if len(wantPrices) != 3 {
		t.Fatalf("catalog has %d of the three new Postgres plans: %v", len(wantPrices), names)
	}
	c, stub := existingContractStub(t, stripeSubscriptionItemsJSON(t, len(names), missing))

	for range 2 {
		if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
			t.Fatalf("EnsureContract: %v", err)
		}
	}
	adds := stub.requests("/v1/subscription_items")
	var posts []string
	for _, r := range adds {
		if r.body == "" {
			continue // the GET list
		}
		posts = append(posts, r.body)
		if r.header.Get("Idempotency-Key") == "" || !strings.HasPrefix(r.header.Get("Idempotency-Key"), "bex-subscription-item-sub_old-") {
			t.Errorf("item add idempotency key = %q", r.header.Get("Idempotency-Key"))
		}
	}
	if len(posts) != len(wantPrices) {
		t.Fatalf("item adds = %d, want %d (once, not per call): %v", len(posts), len(wantPrices), posts)
	}
	for i, body := range posts {
		for _, want := range []string{"subscription=sub_old", "price=" + wantPrices[i], "proration_behavior=none"} {
			if !strings.Contains(body, want) {
				t.Errorf("item add %d body %q missing %q", i, body, want)
			}
		}
	}
	if got := stub.count("/v1/subscription_items"); got != 1+len(wantPrices) {
		t.Errorf("subscription_items calls = %d, want one list then %d adds (the second EnsureContract is cached)", got, len(wantPrices))
	}
}

// TestStripeEnsureContractNeverDoubleRatesARepricedMeter: an item on a retired
// Price (the pre-t012 basic-256mb rate) already rates that meter, so no second
// item is added. Usage would bill twice. The swap is left to the
// next-period runbook step, and nothing is deleted.
func TestStripeEnsureContractNeverDoubleRatesARepricedMeter(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	retired := slices.Index(names, "instance_seconds.postgres.basic-256mb")
	if retired < 0 {
		t.Fatal("basic-256mb missing from the catalog")
	}
	c, stub := existingContractStub(t, stripeSubscriptionItemsJSON(t, len(names), map[int]string{retired: "price_retired"}))
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	for i, body := range stub.bodies {
		if body != "" {
			t.Errorf("a repriced meter must not be written: %s %s", stub.hits[i], body)
		}
	}
}

// TestStripeEnsureContractSyncFailureIsRetried: a failed item add surfaces as
// an EnsureContract error (the emitter then holds that workspace's rows) and
// is retried on the next call rather than cached as synced.
func TestStripeEnsureContractSyncFailureIsRetried(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	catalog := stripePriceCatalogJSON(t, names)
	items := stripeSubscriptionItemsJSON(t, len(names), map[int]string{0: ""})
	fail := true
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodGet && path == "/v1/subscription_items":
			return 200, items
		case method == http.MethodPost && path == "/v1/subscription_items" && fail:
			return 400, `{"error":{"type":"invalid_request_error","message":"nope"}}`
		case method == http.MethodPost && path == "/v1/subscription_items":
			return 200, `{"id":"si_new","object":"subscription_item"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err == nil {
		t.Fatal("EnsureContract must fail when the missing item cannot be added")
	}
	fail = false
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	posts := 0
	for _, r := range stub.requests("/v1/subscription_items") {
		if r.body != "" {
			posts++
		}
	}
	if posts != 2 {
		t.Fatalf("item adds = %d, want the failed one and its retry", posts)
	}
}

// TestCatalogFitsOneSubscription: every billable dimension is one item on the
// workspace's single Subscription, and Stripe caps a Subscription at 20 items.
// The w2/m171 Postgres plans brought the catalog to 18; a dimension that would
// pass 20 needs a contract redesign (a second Subscription or fewer meters)
// before it ships.
func TestCatalogFitsOneSubscription(t *testing.T) {
	if n := len(pricing.Default.BillableMeterNames()); n > 20 {
		t.Fatalf("%d billable dimensions exceed Stripe's 20 items per Subscription", n)
	}
}

// TestStripeEnsureContractToleratesACatalogStripeLacks: when pricing.yaml ships
// before the live Stripe catalog carries its new Prices, an existing
// Subscription keeps rating what it already rates. EnsureContract must not
// fail, because that would hold every workspace's usage, and nothing is written.
func TestStripeEnsureContractToleratesACatalogStripeLacks(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	var present []string
	for _, name := range names {
		if name != "instance_seconds.postgres.1c-2g" {
			present = append(present, name)
		}
	}
	if len(present) == len(names) {
		t.Fatal("catalog has no 1c-2g dimension to withhold")
	}
	catalog := stripePriceCatalogJSON(t, present)
	items := stripeSubscriptionItemsJSON(t, len(present), nil)
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodGet && path == "/v1/subscription_items":
			return 200, items
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract must not block usage on an incomplete live catalog: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	for i, body := range stub.bodies {
		if body != "" {
			t.Errorf("nothing may be written while the catalog is unresolved: %s %s", stub.hits[i], body)
		}
	}
}
