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
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/client"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
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
// stable external-billing identity (m50; supersedes ADR040's old alias). bex never
// has to persist Stripe's own `cus_…` id: it is resolved (and process-cached)
// from the workspace tag.
const workspaceMetadataKey = "bex_workspace"

const (
	subscriptionMetadataKey = "bex_billing_contract"
	defaultCompCouponID     = "bex-comp-100"
)

var billableMeterNames = func() map[string]struct{} {
	out := make(map[string]struct{})
	for _, name := range pricing.Default.BillableMeterNames() {
		out[name] = struct{}{}
	}
	return out
}()

// StripeConfig configures NewStripe. SecretKey is the only required field; empty
// ⇒ the billing sink is disabled (NewStripe returns nil), byte-identical.
type StripeConfig struct {
	SecretKey string // BEX_STRIPE_SECRET_KEY (restricted key) — empty ⇒ disabled
	// PublishableKey is safe to return only to the authenticated dashboard. It
	// must be from the same Stripe mode as SecretKey; the composition root
	// refuses a mismatch before constructing the client.
	PublishableKey string
	// BillingEpoch is both the emitter floor and the earliest subscription
	// backdate. It keeps pre-billing usage outside Stripe rating.
	BillingEpoch time.Time
	// CompCouponID is the perpetual 100%-off coupon provisioned by the setup
	// script. Empty uses the stable default "bex-comp-100".
	CompCouponID string
	// DashboardURL is the only origin Checkout and Portal may return to. An
	// empty value leaves hosted sessions unavailable while meter export and
	// invoice reads continue unchanged.
	DashboardURL string
	// PortalConfigurationID optionally pins sessions to the operator-owned test
	// portal configuration. Empty uses Stripe's default configuration.
	PortalConfigurationID string
	// TaxCode and TaxBehavior are an operator-confirmed pair. Tax remains
	// explicitly unconfigured unless both are present and the account has an
	// active registration; the runtime never guesses either value.
	TaxCode     string
	TaxBehavior string
	// HTTPClient / BaseURL override the SDK transport for tests (a stub backend);
	// production leaves both zero (Stripe's default api.stripe.com).
	HTTPClient *http.Client
	BaseURL    string
	// MaxNetworkRetries overrides the SDK retry count (nil ⇒ 2). Tests set 0 so
	// a 5xx surfaces immediately without real backoff.
	MaxNetworkRetries *int64
	// State persists the provider mapping/lifecycle default used by webhook
	// intake and polling. Nil is retained for isolated unit tests.
	State BillingStateStore
	// Metrics is the process-local, low-cardinality operational sink. Nil keeps
	// isolated tests and non-instrumented callers unchanged.
	Metrics *Metrics
}

// BillingStateStore is the narrow persistence seam the Stripe client needs.
// *store.PGStore satisfies it without making provider ids public state.
type BillingStateStore interface {
	UpsertBillingProviderMapping(context.Context, store.BillingProviderMapping) error
	EnsureBillingLifecycle(context.Context, string) (store.BillingLifecycle, error)
	SetPaymentMethodBound(context.Context, string, time.Time) error
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
	customers map[string]string // workspace id (tea-…) → Stripe customer id (cus_…)
	priceIDs  []string          // complete active catalog, resolved once

	billingEpoch          time.Time
	compCouponID          string
	dashboardURL          string
	portalConfigurationID string
	taxCode               string
	taxBehavior           string
	testMode              bool
	publishableKey        string
	state                 BillingStateStore
	metrics               *Metrics
}

// NewStripe builds a StripeClient, or returns nil when SecretKey is unset — the
// byte-identical "billing off" path. Network retries are handled by the SDK;
// deterministic meter-event identifiers deduplicate within Stripe's documented
// rolling uniqueness window.
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
	compCouponID := cfg.CompCouponID
	if compCouponID == "" {
		compCouponID = defaultCompCouponID
	}
	return &StripeClient{
		sc:                    sc,
		customers:             map[string]string{},
		billingEpoch:          cfg.BillingEpoch.UTC(),
		compCouponID:          compCouponID,
		dashboardURL:          strings.TrimRight(cfg.DashboardURL, "/"),
		portalConfigurationID: cfg.PortalConfigurationID,
		taxCode:               cfg.TaxCode,
		taxBehavior:           cfg.TaxBehavior,
		testMode:              strings.Contains(cfg.SecretKey, "_test_"),
		publishableKey:        cfg.PublishableKey,
		state:                 cfg.State,
		metrics:               cfg.Metrics,
	}
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
	if _, found, err := c.findCustomer(ctx, tenantID); err != nil {
		return err
	} else if found {
		return nil
	}
	// None exists — create one tagged with the workspace id. The idempotency
	// key closes concurrent create races while Customer Search catches restarts.
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
	return c.rememberCustomer(ctx, tenantID, cust.ID)
}

// findCustomer resolves a workspace without creating it. Billing reads use
// this path so a billing_excluded workspace can never gain a Stripe Customer
// merely because somebody opened its usage page.
func (c *StripeClient) findCustomer(ctx context.Context, tenantID string) (string, bool, error) {
	if id, ok := c.lookupCustomer(tenantID); ok {
		return id, true, nil
	}
	sp := &stripe.CustomerSearchParams{}
	sp.Context = ctx
	sp.Limit = stripe.Int64(100)
	sp.Query = fmt.Sprintf("metadata['%s']:'%s'", workspaceMetadataKey, tenantID)
	iter := c.sc.Customers.Search(sp)
	var found []string
	for iter.Next() {
		found = append(found, iter.Customer().ID)
	}
	if err := iter.Err(); err != nil {
		return "", false, fmt.Errorf("stripe: search customer %s: %w", tenantID, err)
	}
	if len(found) > 1 {
		c.metrics.Operation("duplicate_customer", "error")
		return "", false, fmt.Errorf("stripe: workspace %s has %d Customers with metadata %s: %v", tenantID, len(found), workspaceMetadataKey, found)
	}
	if len(found) == 0 {
		return "", false, nil
	}
	if err := c.rememberCustomer(ctx, tenantID, found[0]); err != nil {
		return "", false, err
	}
	return found[0], true, nil
}

func (c *StripeClient) rememberCustomer(ctx context.Context, tenantID, customerID string) error {
	if c.state != nil {
		if err := c.state.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{
			WorkspaceID: tenantID, CustomerID: customerID, Livemode: !c.testMode,
		}); err != nil {
			return fmt.Errorf("stripe: persist customer mapping for %s: %w", tenantID, err)
		}
		if _, err := c.state.EnsureBillingLifecycle(ctx, tenantID); err != nil {
			return fmt.Errorf("stripe: initialize billing lifecycle for %s: %w", tenantID, err)
		}
	}
	c.storeCustomer(tenantID, customerID)
	return nil
}

func (c *StripeClient) rememberSubscription(ctx context.Context, tenantID, customerID, subscriptionID string) error {
	if c.state == nil {
		return nil
	}
	if err := c.state.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{
		WorkspaceID: tenantID, CustomerID: customerID, SubscriptionID: subscriptionID, Livemode: !c.testMode,
	}); err != nil {
		return fmt.Errorf("stripe: persist Subscription mapping for %s: %w", tenantID, err)
	}
	_, err := c.state.EnsureBillingLifecycle(ctx, tenantID)
	return err
}

func (c *StripeClient) ExpectedLivemode() bool { return !c.testMode }

// IngestBatch ships each event to Stripe as a meter event (the /v1/ingest
// equivalent). Stripe deduplicates the deterministic Identifier within its
// documented rolling uniqueness window. A permanent client error (4xx) is
// dead-lettered — logged and skipped — so one bad event never blocks the batch;
// a transient/other error is returned so the emitter leaves the rows un-stamped
// and retries next cycle.
func (c *StripeClient) IngestBatch(ctx context.Context, events []Event) IngestResult {
	result := IngestResult{}
	for _, e := range events {
		eventName, value, skip := stripeMeterEvent(e)
		if skip {
			// Free/zero-rate rows are intentionally accounted locally without a
			// provider event; otherwise they would remain in the outbox forever.
			result.Accepted = append(result.Accepted, e.TransactionID)
			continue
		}
		custID, ok := c.lookupCustomer(e.CustomerID)
		if !ok {
			result.Failed = append(result.Failed, IngestFailure{
				TransactionID: e.TransactionID, Code: "customer_not_ready",
				Message: "Stripe customer mapping unavailable", Permanent: false,
			})
			continue
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
			// Two bex-api replicas can race on the same deterministic event. Stripe
			// guarantees identifier uniqueness for at least 24h; its duplicate
			// response therefore proves that this exact outbox event was already
			// accepted and is safe to stamp locally.
			if duplicateMeterEvent(err) {
				result.Accepted = append(result.Accepted, e.TransactionID)
				continue
			}
			if permanentStripeError(err) {
				code, message := safeStripeError(err)
				log.Printf("billing: Stripe meter event rejected transaction=%s event=%s code=%s", e.TransactionID, eventName, code)
				result.Failed = append(result.Failed, IngestFailure{
					TransactionID: e.TransactionID, Code: code, Message: message, Permanent: true,
				})
				continue
			}
			code, _ := safeStripeError(err)
			log.Printf("billing: Stripe meter event retryable transaction=%s event=%s code=%s", e.TransactionID, eventName, code)
			result.Failed = append(result.Failed, IngestFailure{
				TransactionID: e.TransactionID, Code: code,
				Message: "transient Stripe meter-event failure", Permanent: false,
			})
			continue
		}
		result.Accepted = append(result.Accepted, e.TransactionID)
	}
	return result
}

func duplicateMeterEvent(err error) bool {
	var se *stripe.Error
	return errors.As(err, &se) && string(se.Code) == "duplicate_meter_event"
}

// safeStripeError reduces an SDK error to bounded, non-payment diagnostics.
// The provider request id, response body, parameters, and full payload are
// deliberately excluded from durable state and logs.
func safeStripeError(err error) (code, message string) {
	code = "provider_error"
	message = "Stripe rejected the meter event"
	var se *stripe.Error
	if !errors.As(err, &se) {
		return code, message
	}
	if se.Code != "" {
		code = string(se.Code)
	} else if se.Type != "" {
		code = string(se.Type)
	}
	if se.Msg != "" {
		message = se.Msg
	}
	if len(code) > 80 {
		code = code[:80]
	}
	if len(message) > 240 {
		message = message[:240]
	}
	return code, message
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
		name := fmt.Sprintf("instance_seconds.%s.%s", rk, tier)
		if _, ok := billableMeterNames[name]; !ok {
			return "", "", true // unknown/zero-rate tier: same policy as pricing estimate
		}
		return name, e.Properties["value"], false
	case store.UsageKindEgressBytes:
		return "egress_gib", scaleDown(e.Properties["value"], bytesPerGiB), false
	case store.UsageKindStorageGBSeconds:
		return "storage_gb_hours", scaleDown(e.Properties["value"], secondsPerHour), false
	case store.UsageKindDiskGBSeconds:
		// Same GB-hour rebase as storage, a separate meter: the basis
		// (provisioned vs used) and the rate both differ (ADR082 D8/D9).
		return "disk_gb_hours", scaleDown(e.Properties["value"], secondsPerHour), false
	case store.UsageKindBuildSeconds:
		return "build_seconds", e.Properties["value"], false
	case store.UsageKindSandboxComputeSeconds:
		if _, ok := billableMeterNames[store.UsageKindSandboxComputeSeconds]; !ok {
			return "", "", true
		}
		return store.UsageKindSandboxComputeSeconds, e.Properties["value"], false
	default:
		return e.EventType, e.Properties["value"], false
	}
}

// scaleDown divides an integer-string quantity by divisor and rounds to the 12
// decimal places Stripe permits in a meter-event payload. Rational arithmetic
// avoids float64 tails that Stripe permanently rejects.
func scaleDown(intStr string, divisor int64) string {
	n, ok := new(big.Int).SetString(intStr, 10)
	if !ok || divisor <= 0 {
		return "0"
	}
	value := new(big.Rat).SetFrac(n, big.NewInt(divisor)).FloatString(12)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" || value == "-0" {
		return "0"
	}
	return value
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

func (c *StripeClient) lookupCustomer(tenantID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.customers[tenantID]
	return id, ok
}

func (c *StripeClient) storeCustomer(tenantID, customerID string) {
	c.mu.Lock()
	c.customers[tenantID] = customerID
	c.mu.Unlock()
}
