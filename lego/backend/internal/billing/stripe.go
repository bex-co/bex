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
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Re-basing constants: Stripe's unit_amount_decimal caps at 12 decimal places,
// too coarse for bex's per-byte / per-GB-second rates, so those meters are
// priced (and their events valued) in coarser units — GiB and GB-hours. The
// setup script (scripts/stripe-billing-setup.py) prices the matching meters
// identically; the two MUST agree.
const (
	bytesPerGiB    = 1073741824
	secondsPerHour = 3600
)

// workspaceMetadataKey tags each Stripe customer with its bex workspace id, so a
// customer is looked up by `metadata['bex_workspace']:'tea-…'` — the Stripe
// equivalent of Metronome's ingest alias (m50; supersedes ADR040 §2). bex never
// has to persist Stripe's own `cus_…` id: it is resolved (and process-cached)
// from the workspace tag.
const workspaceMetadataKey = "bex_workspace"

// StripeConfig configures NewStripe. SecretKey is the only required field; empty
// ⇒ the billing sink is disabled (NewStripe returns nil), byte-identical.
type StripeConfig struct {
	SecretKey string // BEX_STRIPE_SECRET_KEY (restricted key) — empty ⇒ disabled
	// HTTPClient / BaseURL override the SDK transport for tests (a stub backend);
	// production leaves both zero (Stripe's default api.stripe.com).
	HTTPClient *http.Client
	BaseURL    string
	// MaxNetworkRetries overrides the SDK retry count (nil ⇒ 2). Tests set 0 so
	// a 5xx surfaces immediately without real backoff.
	MaxNetworkRetries *int64
}

// compile-time check: the Stripe sink satisfies the emitter's Ingester seam,
// so m47's emitter/outbox is reused unchanged.
var _ Ingester = (*StripeClient)(nil)

// StripeClient ships bex usage to Stripe Billing: a workspace becomes a Stripe
// Customer, and each sealed usage row becomes a meter event (m50). It satisfies
// the emitter's Ingester seam, so the seal-then-emit outbox, epoch floor, and
// deterministic id (m47) are reused unchanged — only the sink is Stripe.
type StripeClient struct {
	sc *client.API

	mu        sync.Mutex
	customers map[string]string // workspace id (tea-…) → Stripe customer id (cus-…)
}

// NewStripe builds a StripeClient, or returns nil when SecretKey is unset — the
// byte-identical "billing off" path. Network retries are handled by the SDK; the
// deterministic meter-event identifier makes every retry a safe dedup.
func NewStripe(cfg StripeConfig) *StripeClient {
	if cfg.SecretKey == "" {
		return nil
	}
	retries := int64(2)
	if cfg.MaxNetworkRetries != nil {
		retries = *cfg.MaxNetworkRetries
	}
	apiBackend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		HTTPClient:        cfg.HTTPClient,
		URL:               stripeURL(cfg.BaseURL),
		MaxNetworkRetries: stripe.Int64(retries),
	})
	sc := &client.API{}
	sc.Init(cfg.SecretKey, &stripe.Backends{API: apiBackend})
	return &StripeClient{sc: sc, customers: map[string]string{}}
}

func stripeURL(base string) *string {
	if base == "" {
		return nil // SDK default (https://api.stripe.com)
	}
	return stripe.String(base)
}

// EnsureCustomer resolves the workspace's Stripe customer, creating it on first
// sight. Idempotent: a process cache short-circuits, and a metadata search
// (`bex_workspace`) recovers an existing customer after a restart before any
// create. The customer id is cached so IngestBatch can stamp meter events with
// it.
func (c *StripeClient) EnsureCustomer(ctx context.Context, tenantID string) error {
	if _, ok := c.lookup(tenantID); ok {
		return nil
	}
	// Recover an existing customer by its workspace tag (durable idempotency).
	sp := &stripe.CustomerSearchParams{}
	sp.Context = ctx
	sp.Query = fmt.Sprintf("metadata['%s']:'%s'", workspaceMetadataKey, tenantID)
	iter := c.sc.Customers.Search(sp)
	if iter.Next() {
		c.store(tenantID, iter.Customer().ID)
		return nil
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("stripe: search customer %s: %w", tenantID, err)
	}
	// None exists — create one tagged with the workspace id.
	cp := &stripe.CustomerParams{
		Name:     stripe.String(tenantID),
		Metadata: map[string]string{workspaceMetadataKey: tenantID},
	}
	cp.Context = ctx
	cp.SetIdempotencyKey("bex-customer-" + tenantID)
	cust, err := c.sc.Customers.New(cp)
	if err != nil {
		return fmt.Errorf("stripe: create customer %s: %w", tenantID, err)
	}
	c.store(tenantID, cust.ID)
	return nil
}

// EnsureContract is a no-op for the Stripe sink's shadow phase: usage lands as
// meter events regardless, and the Subscription that rates them (the Metronome
// "contract" equivalent) is provisioned separately (m50/t005). Kept to satisfy
// the Ingester seam.
func (c *StripeClient) EnsureContract(context.Context, string) error { return nil }

// IngestBatch ships each event to Stripe as a meter event (the /v1/ingest
// equivalent). Stripe dedups by the deterministic Identifier, so retries and
// re-scans are safe. A permanent client error (4xx) on one event is
// dead-lettered — logged and skipped — so one bad event never blocks the batch;
// a transient/other error is returned so the emitter leaves the rows un-stamped
// and retries next cycle.
func (c *StripeClient) IngestBatch(ctx context.Context, events []Event) error {
	for _, e := range events {
		eventName, value, skip := stripeMeterEvent(e)
		if skip {
			continue // free tier (no meter/price) — nothing owed, nothing to send
		}
		custID, ok := c.lookup(e.CustomerID)
		if !ok {
			// The emitter ensures the customer before ingesting; a miss means a
			// transient ensure gap — surface it so the batch retries.
			return fmt.Errorf("stripe: no customer for workspace %s", e.CustomerID)
		}
		params := &stripe.BillingMeterEventParams{
			EventName:  stripe.String(eventName),
			Identifier: stripe.String(e.TransactionID),
			Timestamp:  stripe.Int64(e.Timestamp.Unix()),
			Payload: map[string]string{
				"stripe_customer_id": custID,
				"value":              value,
			},
		}
		params.Context = ctx
		if _, err := c.sc.BillingMeterEvents.New(params); err != nil {
			if permanentStripeError(err) {
				log.Printf("stripe: DLQ meter event %s (%s) dropped: %v", e.TransactionID, eventName, err)
				continue
			}
			return fmt.Errorf("stripe: meter event %s: %w", e.TransactionID, err)
		}
	}
	return nil
}

// stripeMeterEvent maps a generic bex usage Event onto the Stripe meter it
// belongs to (m50): instance_seconds rates per (resource_kind, tier), so it
// carries a composed event name; bandwidth and storage are re-based to GiB /
// GB-hours to fit Stripe's price precision. Free-tier instance_seconds has no
// meter (nothing is owed), so it is skipped. Kept in lockstep with
// scripts/stripe-billing-setup.py.
func stripeMeterEvent(e Event) (eventName, value string, skip bool) {
	switch e.EventType {
	case store.UsageKindInstanceSeconds:
		tier := e.Properties["tier"]
		if tier == "" || tier == "free" {
			return "", "", true
		}
		rk := store.NormalizeResourceKind(e.Properties["resource_kind"])
		return fmt.Sprintf("instance_seconds.%s.%s", rk, tier), e.Properties["value"], false
	case store.UsageKindEgressBytes:
		return "egress_gib", scaleDown(e.Properties["value"], bytesPerGiB), false
	case store.UsageKindStorageGBSeconds:
		return "storage_gb_hours", scaleDown(e.Properties["value"], secondsPerHour), false
	case store.UsageKindBuildSeconds:
		return "build_seconds", e.Properties["value"], false
	default:
		return e.EventType, e.Properties["value"], false
	}
}

// scaleDown divides an integer-string quantity by divisor, returning the
// shortest exact decimal string — for re-basing bytes→GiB and GB-seconds→GB-hours.
func scaleDown(intStr string, divisor int64) string {
	n, err := strconv.ParseInt(intStr, 10, 64)
	if err != nil {
		return "0"
	}
	return strconv.FormatFloat(float64(n)/float64(divisor), 'f', -1, 64)
}

// permanentStripeError reports whether err is a non-retryable client error
// (a 4xx other than 429) — dead-letter it rather than retrying forever.
func permanentStripeError(err error) bool {
	var se *stripe.Error
	if !errors.As(err, &se) {
		return false // network/unknown → transient, retry
	}
	s := se.HTTPStatusCode
	return s >= 400 && s < 500 && s != http.StatusTooManyRequests
}

func (c *StripeClient) lookup(tenantID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.customers[tenantID]
	return id, ok
}

func (c *StripeClient) store(tenantID, customerID string) {
	c.mu.Lock()
	c.customers[tenantID] = customerID
	c.mu.Unlock()
}
