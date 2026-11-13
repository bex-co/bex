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
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func agentModelIndex(t *testing.T) int {
	t.Helper()
	i := slices.Index(pricing.Default.BillableMeterNames(), store.UsageKindAgentModelCostMicroUSD)
	if i < 0 {
		t.Fatal("agent_model_cost_microusd is not a billable meter")
	}
	return i
}

// itemsWithAgentThreshold is stripeSubscriptionItemsJSON with the agent-model
// item carrying usageGTE (0 = no threshold).
func itemsWithAgentThreshold(t *testing.T, usageGTE int64) string {
	t.Helper()
	names := pricing.Default.BillableMeterNames()
	var list map[string]any
	if err := json.Unmarshal([]byte(stripeSubscriptionItemsJSON(t, len(names), nil)), &list); err != nil {
		t.Fatal(err)
	}
	if usageGTE > 0 {
		item := list["data"].([]any)[agentModelIndex(t)].(map[string]any)
		item["billing_thresholds"] = map[string]any{"usage_gte": usageGTE}
	}
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// thresholdStub is one workspace (tea-a, cus_1) with live Subscription
// sub_old whose items are items; item updates answer 200.
func thresholdStub(t *testing.T, catalogNames []string, items string) (*StripeClient, *stripeStub) {
	t.Helper()
	catalog := stripePriceCatalogJSON(t, catalogNames)
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodGet && path == "/v1/subscription_items":
			return 200, items
		case method == http.MethodPost && strings.HasPrefix(path, "/v1/subscription_items/"):
			return 200, `{"id":"` + strings.TrimPrefix(path, "/v1/subscription_items/") + `","object":"subscription_item"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	return c, stub
}

// itemUpdates is the decoded bodies of POST /v1/subscription_items/{id}.
func itemUpdates(t *testing.T, stub *stripeStub, itemID string) []url.Values {
	t.Helper()
	var out []url.Values
	for _, r := range stub.requests("/v1/subscription_items/" + itemID) {
		v, err := url.ParseQuery(r.body)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

func micro(v int64) *int64 { return &v }

// ADR091 D10.2: SyncAgentModelThreshold sets, raises and removes the
// agent-model item's usage_gte, writes nothing when Stripe already holds the
// tier's value, and touches no other item.
func TestSyncAgentModelThresholdSetsUpdatesAndRemoves(t *testing.T) {
	agentItem := fmt.Sprintf("si_%02d", agentModelIndex(t))
	names := pricing.Default.BillableMeterNames()
	for _, tc := range []struct {
		name    string
		current int64
		want    *int64
		wantSet string // "" ⇒ no write; "unset" ⇒ removed
	}{
		{"set T0 $10", 0, micro(10_000_000), "10000000"},
		{"raise $10 → $25", 10_000_000, micro(25_000_000), "25000000"},
		{"remove at T4", 50_000_000, nil, "unset"},
		{"equal is a no-op", 25_000_000, micro(25_000_000), ""},
		{"none and none is a no-op", 0, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, stub := thresholdStub(t, names, itemsWithAgentThreshold(t, tc.current))
			if err := c.SyncAgentModelThreshold(context.Background(), "tea-a", tc.want); err != nil {
				t.Fatalf("sync: %v", err)
			}
			updates := itemUpdates(t, stub, agentItem)
			if tc.wantSet == "" {
				if len(updates) != 0 {
					t.Fatalf("updates = %v, want none", updates)
				}
				return
			}
			if len(updates) != 1 {
				t.Fatalf("updates = %v, want one", updates)
			}
			u := updates[0]
			if tc.wantSet == "unset" {
				if v, ok := u["billing_thresholds"]; !ok || v[0] != "" {
					t.Fatalf("remove body = %v, want billing_thresholds=\"\"", u)
				}
			} else if got := u.Get("billing_thresholds[usage_gte]"); got != tc.wantSet {
				t.Fatalf("usage_gte = %q, want %s (body %v)", got, tc.wantSet, u)
			}
			if u.Get("proration_behavior") != "none" {
				t.Fatalf("body %v, want proration_behavior=none", u)
			}
			// Only the agent-model item is written.
			stub.mu.Lock()
			defer stub.mu.Unlock()
			for i, body := range stub.bodies {
				if body != "" && stub.hits[i] != "/v1/subscription_items/"+agentItem {
					t.Errorf("unexpected write %s: %s", stub.hits[i], body)
				}
			}
		})
	}
}

// A held agent-model meter has no Price and no item, so no threshold is
// ever written, by the sync or at Subscription creation.
func TestAgentModelThresholdNeverSetWhileTheMeterIsHeld(t *testing.T) {
	names := slices.DeleteFunc(pricing.Default.BillableMeterNames(), func(n string) bool {
		return n == store.UsageKindAgentModelCostMicroUSD
	})
	c, stub := thresholdStub(t, names, itemsWithAgentThreshold(t, 0))
	c.heldMeters = []string{store.UsageKindAgentModelCostMicroUSD}
	if err := c.SyncAgentModelThreshold(context.Background(), "tea-a", micro(10_000_000)); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if got := stub.count("/v1/subscription_items"); got != 0 {
		t.Fatalf("subscription_items calls = %d, want none while held", got)
	}

	// Creation: no billing_thresholds on any item.
	catalog := stripePriceCatalogJSON(t, names)
	c, stub = newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodPost && path == "/v1/subscriptions":
			return 200, `{"id":"sub_new","object":"subscription","status":"active"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.heldMeters = []string{store.UsageKindAgentModelCostMicroUSD}
	c.agentModelThreshold = func(context.Context, string) (*int64, error) { return micro(10_000_000), nil }
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	for _, r := range stub.requests("/v1/subscriptions") {
		if strings.Contains(r.body, "billing_thresholds") {
			t.Fatalf("held meter got a threshold: %s", r.body)
		}
	}
}

// A new Subscription's agent-model item is created with the tier's
// threshold, and no other item carries one.
func TestEnsureContractCreatesTheAgentModelItemWithItsThreshold(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	catalog := stripePriceCatalogJSON(t, names)
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodPost && path == "/v1/subscriptions":
			return 200, `{"id":"sub_new","object":"subscription","status":"active"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.agentModelThreshold = func(_ context.Context, ws string) (*int64, error) {
		if ws != "tea-a" {
			t.Errorf("threshold read for %s", ws)
		}
		return micro(10_000_000), nil
	}
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	posts := stub.requests("/v1/subscriptions")
	body, err := url.ParseQuery(posts[len(posts)-1].body)
	if err != nil {
		t.Fatal(err)
	}
	i := agentModelIndex(t)
	if got := body.Get(fmt.Sprintf("items[%d][billing_thresholds][usage_gte]", i)); got != "10000000" {
		t.Fatalf("agent-model item usage_gte = %q, want 10000000 (body %v)", got, body)
	}
	if got := body.Get(fmt.Sprintf("items[%d][price]", i)); got != fmt.Sprintf("price_%02d", i) {
		t.Fatalf("items[%d] price = %q", i, got)
	}
	thresholds := 0
	for k := range body {
		if strings.Contains(k, "billing_thresholds") {
			thresholds++
		}
	}
	if thresholds != 1 {
		t.Fatalf("billing_thresholds fields = %d, want only the agent-model item's", thresholds)
	}
}

// An existing Subscription synced by EnsureContract gains the agent-model item
// with its threshold, or has its existing item's threshold corrected.
func TestEnsureContractSyncAppliesTheThreshold(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	i := agentModelIndex(t)
	t.Run("added item", func(t *testing.T) {
		c, stub := existingContractStub(t, stripeSubscriptionItemsJSON(t, len(names), map[int]string{i: ""}))
		c.agentModelThreshold = func(context.Context, string) (*int64, error) { return micro(25_000_000), nil }
		if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
			t.Fatalf("EnsureContract: %v", err)
		}
		adds := itemAdds(stub)
		if len(adds) != 1 {
			t.Fatalf("item adds = %v, want the agent-model item", adds)
		}
		add, err := url.ParseQuery(adds[0])
		if err != nil {
			t.Fatal(err)
		}
		if add.Get("price") != fmt.Sprintf("price_%02d", i) || add.Get("billing_thresholds[usage_gte]") != "25000000" {
			t.Fatalf("item add = %v, want the agent-model item with usage_gte 25000000", add)
		}
	})
	t.Run("existing item", func(t *testing.T) {
		c, stub := thresholdStub(t, names, itemsWithAgentThreshold(t, 10_000_000))
		c.agentModelThreshold = func(context.Context, string) (*int64, error) { return micro(50_000_000), nil }
		if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
			t.Fatalf("EnsureContract: %v", err)
		}
		updates := itemUpdates(t, stub, fmt.Sprintf("si_%02d", i))
		if len(updates) != 1 || updates[0].Get("billing_thresholds[usage_gte]") != "50000000" {
			t.Fatalf("updates = %v, want usage_gte 50000000", updates)
		}
	})
}

type thresholdStoreFake struct {
	trust  map[string]store.BillingTrust
	synced map[string]*int64 // recorded value per workspace (absent = never)
	due    []string
	locked bool // another replica holds the sweep
	debt   store.AgentModelBadDebt
	// revenue is every month RefreshAgentModelRevenue was asked for.
	revenue []time.Time
}

func (f *thresholdStoreFake) AgentModelThresholdsDue(context.Context, time.Time, int) ([]string, error) {
	return f.due, nil
}

func (f *thresholdStoreFake) WithAgentModelThresholdSync(_ context.Context, ws string, _ time.Time, apply func(store.AgentModelThresholdState) (*int64, error)) error {
	state := store.AgentModelThresholdState{Trust: f.trust[ws]}
	state.SyncedMicroUSD, state.Synced = f.synced[ws]
	value, err := apply(state)
	if err != nil {
		return err
	}
	if f.synced == nil {
		f.synced = map[string]*int64{}
	}
	f.synced[ws] = value
	return nil
}

func (f *thresholdStoreFake) RunExclusive(ctx context.Context, _ string, fn func(context.Context) error) (bool, error) {
	if f.locked {
		return false, nil
	}
	return true, fn(ctx)
}

func (f *thresholdStoreFake) RefreshAgentModelRevenue(_ context.Context, month time.Time) error {
	f.revenue = append(f.revenue, month)
	return nil
}

func (f *thresholdStoreFake) AgentModelBadDebtForMonth(context.Context, time.Time) (store.AgentModelBadDebt, error) {
	return f.debt, nil
}

type thresholdSetterFake struct {
	calls map[string][]*int64
	fail  map[string]bool
}

func (f *thresholdSetterFake) SyncAgentModelThreshold(_ context.Context, ws string, want *int64) error {
	if f.calls == nil {
		f.calls = map[string][]*int64{}
	}
	f.calls[ws] = append(f.calls[ws], want)
	if f.fail[ws] {
		return fmt.Errorf("stripe down")
	}
	return nil
}

func thresholdValues(calls []*int64) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		if c == nil {
			out = append(out, "none")
			continue
		}
		out = append(out, fmt.Sprint(*c))
	}
	return out
}

// ADR091 D10.2a retired the Stripe item threshold: a webhook tier change
// (Trust.OnTierChanged) removes a workspace's item threshold once, at any
// tier, and a workspace whose recorded value is already none is not sent to
// Stripe again.
func TestTierChangeRemovesTheThreshold(t *testing.T) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	fake := &thresholdStoreFake{synced: map[string]*int64{"tea-a": micro(10_000_000)}}
	setter := &thresholdSetterFake{}
	th := &AgentModelThresholds{Store: fake, Stripe: setter, Clock: func() time.Time { return now }}
	th.OnTierChanged(context.Background(), "tea-a", pricing.TrustTier(1000, 0, time.Time{}, now))
	th.OnTierChanged(context.Background(), "tea-b", pricing.TrustTier(300_000, 6, time.Time{}, now))
	th.OnTierChanged(context.Background(), "tea-a", pricing.TrustTier(10_000, 0, time.Time{}, now))
	if got := thresholdValues(setter.calls["tea-a"]); !slices.Equal(got, []string{"none"}) {
		t.Fatalf("tea-a thresholds = %v, want [none] once", got)
	}
	if got := thresholdValues(setter.calls["tea-b"]); !slices.Equal(got, []string{"none"}) {
		t.Fatalf("tea-b thresholds = %v, want [none]", got)
	}
	if want, err := th.Want(context.Background(), "tea-a"); err != nil || want != nil {
		t.Fatalf("Want = %v, %v: a new agent-model item gets no threshold", want, err)
	}

	// Through the real Trust reducer: an invoice.paid that lifts T0 → T1
	// calls the hook, which removes the threshold.
	trustStore := &trustStoreFake{after: store.BillingTrust{LifetimePaidCents: 1000}}
	h := &Trust{Store: trustStore, OnTierChanged: th.OnTierChanged, Clock: func() time.Time { return now }}
	if err := h.HandleStripeEvent(context.Background(), trustEvent("evt_paid", stripe.EventTypeInvoicePaid, now,
		trustInvoice("tea-c", "in_1", 1000, "subscription_threshold", now))); err != nil {
		t.Fatal(err)
	}
	if got := thresholdValues(setter.calls["tea-c"]); !slices.Equal(got, []string{"none"}) {
		t.Fatalf("tea-c thresholds = %v, want [none]", got)
	}
}

// The sweep removes the item threshold of every due workspace even when one
// fails, skips Stripe when the recorded value is already none, runs only on
// the replica holding its lock, and snapshots the current and closed month's
// revenue (the closed one only until an hour into the new month).
func TestThresholdSweep(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 30, 0, 0, time.UTC)
	fake := &thresholdStoreFake{
		due: []string{"tea-a", "tea-b", "tea-c"},
		// tea-b still holds the T1 threshold it had before D10.2a; tea-c's
		// was already removed.
		synced: map[string]*int64{"tea-b": micro(25_000_000), "tea-c": nil},
	}
	setter := &thresholdSetterFake{fail: map[string]bool{"tea-a": true}}
	th := &AgentModelThresholds{Store: fake, Stripe: setter, Clock: func() time.Time { return now }}
	if err := th.RunOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "tea-a") {
		t.Fatalf("err = %v, want tea-a's failure surfaced", err)
	}
	if got := thresholdValues(setter.calls["tea-b"]); !slices.Equal(got, []string{"none"}) {
		t.Fatalf("tea-b = %v: a failure must not stop the others", got)
	}
	if _, called := setter.calls["tea-c"]; called {
		t.Fatal("tea-c's recorded value is already none: no Stripe call")
	}
	if _, recorded := fake.synced["tea-a"]; recorded {
		t.Fatal("a failed sync was recorded as applied")
	}

	fake.locked = true
	if err := th.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(setter.calls["tea-b"]); got != 1 {
		t.Fatalf("the replica without the lock swept (%d calls)", got)
	}
	fake.locked = false
	now = now.Add(time.Hour)
	if err := th.RunOnce(context.Background()); err == nil {
		t.Fatal("tea-a still fails")
	}
	now = now.Add(5 * time.Minute)
	_ = th.RunOnce(context.Background())
	nov, oct := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if want := []time.Time{nov, oct, nov, oct, nov}; !slices.Equal(fake.revenue, want) {
		t.Fatalf("revenue snapshots = %v, want %v", fake.revenue, want)
	}
}

// The agent-model item is picked by the catalog Price when two items rate
// its meter (a retired Price beside it), and that duplicate is reported.
func TestAgentModelItemPrefersTheCatalogPrice(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	i := agentModelIndex(t)
	c, _ := thresholdStub(t, names, itemsWithAgentThreshold(t, 0))
	if _, err := c.resolvePriceIDs(context.Background()); err != nil {
		t.Fatal(err)
	}
	meter := fmt.Sprintf("mtr_%02d", i)
	retired := &stripe.SubscriptionItem{ID: "si_retired", Price: &stripe.Price{ID: "price_old", Recurring: &stripe.PriceRecurring{Meter: meter}}}
	current := &stripe.SubscriptionItem{ID: "si_current", Price: &stripe.Price{ID: fmt.Sprintf("price_%02d", i), Recurring: &stripe.PriceRecurring{Meter: meter}}}
	for _, items := range [][]*stripe.SubscriptionItem{{retired, current}, {current, retired}} {
		if got := c.agentModelItem("tea-a", "sub_old", items); got != current {
			t.Fatalf("picked %v, want the catalog Price's item", got)
		}
	}
	if got := c.agentModelItem("tea-a", "sub_old", []*stripe.SubscriptionItem{retired}); got != retired {
		t.Fatalf("picked %v, want the retired item when it is the only one", got)
	}
}

// The sweep path reads items from the Subscription itself when its embedded
// list is complete, without a separate items list call.
func TestSyncAgentModelThresholdUsesEmbeddedItems(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	i := agentModelIndex(t)
	catalog := stripePriceCatalogJSON(t, names)
	items := itemsWithAgentThreshold(t, 10_000_000)
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","items":` + items +
				`,"metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodPost && strings.HasPrefix(path, "/v1/subscription_items/"):
			return 200, `{"id":"` + strings.TrimPrefix(path, "/v1/subscription_items/") + `","object":"subscription_item"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.SyncAgentModelThreshold(context.Background(), "tea-a", micro(50_000_000)); err != nil {
		t.Fatal(err)
	}
	if got := len(stub.requests("/v1/subscription_items")); got != 0 {
		t.Fatalf("items list calls = %d, want none", got)
	}
	if updates := itemUpdates(t, stub, fmt.Sprintf("si_%02d", i)); len(updates) != 1 || updates[0].Get("billing_thresholds[usage_gte]") != "50000000" {
		t.Fatalf("updates = %v", updates)
	}
}

// A threshold update Stripe refuses never fails the contract sync (usage
// export goes on); the next sync retries it.
func TestEnsureContractThresholdFailureDoesNotBlockExport(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	i := agentModelIndex(t)
	catalog := stripePriceCatalogJSON(t, names)
	items := itemsWithAgentThreshold(t, 10_000_000)
	refuse := true
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodGet && path == "/v1/subscription_items":
			return 200, items
		case method == http.MethodPost && strings.HasPrefix(path, "/v1/subscription_items/"):
			if refuse {
				return 400, `{"error":{"type":"invalid_request_error","code":"parameter_invalid","message":"no"}}`
			}
			return 200, `{"id":"` + strings.TrimPrefix(path, "/v1/subscription_items/") + `","object":"subscription_item"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	c.agentModelThreshold = func(context.Context, string) (*int64, error) { return micro(25_000_000), nil }
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract = %v, want nil despite the refused threshold", err)
	}
	refuse = false
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatal(err)
	}
	if got := len(itemUpdates(t, stub, fmt.Sprintf("si_%02d", i))); got != 2 {
		t.Fatalf("threshold updates = %d, want the refused one and its retry only", got)
	}
}

// invoice.created finalizes a draft threshold invoice with auto_advance, and
// leaves every other invoice alone; an invoice Stripe already finalized is a
// no-op.
func TestThresholdInvoiceIsFinalizedOnCreation(t *testing.T) {
	at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	created := func(id, reason, status string) *stripe.Event {
		return trustEvent("evt_"+id, stripe.EventTypeInvoiceCreated, at, fmt.Sprintf(
			`{"id":%q,"object":"invoice","billing_reason":%q,"status":%q,"parent":{"type":"subscription_details","subscription_details":`+
				`{"subscription":"sub_x","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}}}`, id, reason, status))
	}
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodPost && path == "/v1/invoices/in_draft/finalize":
			return 200, `{"id":"in_draft","object":"invoice","status":"open"}`
		case method == http.MethodPost && path == "/v1/invoices/in_done/finalize":
			return 400, `{"error":{"type":"invalid_request_error","code":"invoice_unexpected_state","message":"already finalized"}}`
		case method == http.MethodGet && path == "/v1/invoices/in_done":
			return 200, `{"id":"in_done","object":"invoice","status":"paid"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	h := &Trust{Store: &trustStoreFake{}, Invoices: c}
	for _, e := range []*stripe.Event{
		created("in_draft", "subscription_threshold", "draft"),
		created("in_done", "subscription_threshold", "draft"),
		created("in_cycle", "subscription_cycle", "draft"),
		created("in_open", "subscription_threshold", "open"),
	} {
		if err := h.HandleStripeEvent(context.Background(), e); err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
	}
	finalize := stub.requests("/v1/invoices/in_draft/finalize")
	if len(finalize) != 1 {
		t.Fatalf("finalize calls = %d", len(finalize))
	}
	if body, _ := url.ParseQuery(finalize[0].body); body.Get("auto_advance") != "true" {
		t.Fatalf("finalize body = %q, want auto_advance=true", finalize[0].body)
	}
	if finalize[0].header.Get("Idempotency-Key") != "bex-finalize-threshold-in_draft" {
		t.Fatalf("idempotency key = %q", finalize[0].header.Get("Idempotency-Key"))
	}
	for _, path := range []string{"/v1/invoices/in_cycle/finalize", "/v1/invoices/in_open/finalize"} {
		if stub.count(path) != 0 {
			t.Fatalf("%s finalized", path)
		}
	}
	if !isTrustEvent(stripe.EventTypeInvoiceCreated) || !isTrustEvent(stripe.EventTypeInvoiceVoided) {
		t.Fatal("invoice.created / invoice.voided are not dispatched to Trust")
	}
}

type trustStoreFake struct {
	before, after store.BillingTrust
	settled       []string
}

func (f *trustStoreFake) RecordBillingTrustPayment(context.Context, store.BillingTrustPayment) (store.BillingTrust, store.BillingTrust, bool, error) {
	return f.before, f.after, true, nil
}

func (f *trustStoreFake) RecordBillingTrustFailure(context.Context, string, string, time.Time) (store.BillingTrust, store.BillingTrust, bool, error) {
	return f.before, f.after, true, nil
}

func (f *trustStoreFake) SettleBillingTrustInvoice(_ context.Context, _, invoiceID string, _ time.Time) error {
	f.settled = append(f.settled, invoiceID)
	return nil
}

func (f *trustStoreFake) RecordBillingTrustDispute(context.Context, string, string) (store.BillingTrust, store.BillingTrust, error) {
	return f.before, f.after, nil
}

func (f *trustStoreFake) RecordBillingTrustDisputeWon(context.Context, string, string) (store.BillingTrust, store.BillingTrust, error) {
	return f.before, f.after, nil
}

func (f *trustStoreFake) BillingWorkspaceForCustomer(context.Context, string, bool) (string, bool, error) {
	return "", false, nil
}
