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

// Package billing is the Metronome export seam (docs/ADR040-billing-metronome.md):
// a backend-only sidecar that ships each sealed usage_hourly row to Metronome's
// /v1/ingest exactly once, so Metronome — not bex — does the rating and
// invoicing. bex keeps usage_hourly/usage_monthly as the operational record and
// pricing.yaml as the dashboard estimate; this package only exports.
//
// The whole feature is gated by BEX_METRONOME_TOKEN: New returns nil when the
// token is unset, the emitter is then never constructed, and bex-api is
// byte-for-byte unchanged from its ADR030 (estimate-only) behavior. The
// operator never imports this package — money never reaches the mechanism layer.
package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	metronome "github.com/Metronome-Industries/metronome-go/v3"
	"github.com/Metronome-Industries/metronome-go/v3/option"
)

// DefaultBaseURL is Metronome's public API origin, used when BEX_METRONOME_URL
// is unset.
const DefaultBaseURL = "https://api.metronome.com"

// maxBatch is Metronome's documented per-ingest cap: /v1/ingest accepts at most
// 100 usage events per call (ADR040 §2). IngestBatch chunks to this.
const maxBatch = 100

// defaultBackoff is the retry schedule for a retryable ingest (429/5xx/network),
// per Metronome's guidance (ADR040 §4). Six attempts total (initial + 5 retries)
// over ~31s; a deterministic transaction_id makes every retry a safe no-op on
// the server. Overridable in tests.
var defaultBackoff = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

// Event is one usage record bex exports — the meter quantity and its dimensions
// mapped onto a Metronome ingest event (ADR040 §5). Timestamp is the row's
// window_start; TransactionID is the deterministic idempotency key the emitter
// derives so retries and re-scans dedup server-side within Metronome's 34-day
// window.
type Event struct {
	TransactionID string
	CustomerID    string // the workspace id (tea-…), resolved via its ingest alias
	EventType     string // the meter kind: instance_seconds / egress_bytes / …
	Timestamp     time.Time
	// Properties carries the numeric quantity (as the string "value") plus the
	// tier / resource_kind / service_id dimensions — all strings, per ADR040 §5.
	Properties map[string]string
}

// Ingester is the slice of the client the emitter depends on: ensure a
// workspace's customer and (when a rate card is configured) its contract exist,
// then ship its sealed rows. A fake implements it in tests without any HTTP.
type Ingester interface {
	EnsureCustomer(ctx context.Context, tenantID string) error
	EnsureContract(ctx context.Context, tenantID string) error
	IngestBatch(ctx context.Context, events []Event) error
}

// Config configures New. Token is the only required field; an empty Token means
// the feature is disabled (New returns nil).
type Config struct {
	Token   string // BEX_METRONOME_TOKEN — empty ⇒ disabled
	BaseURL string // BEX_METRONOME_URL — empty ⇒ DefaultBaseURL
	// RateCardID (BEX_METRONOME_RATE_CARD_ID) is the rate card contracts bind to
	// (m48). Empty ⇒ shadow-export only: usage lands in Metronome but is rated by
	// nothing and no contracts are created (m47 behavior, byte-identical).
	RateCardID string
	// USDCreditTypeID (BEX_METRONOME_USD_CREDIT_TYPE_ID) is the credit type a
	// Mode B comp grants against. Only needed to comp a customer.
	USDCreditTypeID string
	// HTTPClient overrides the SDK's transport. Production leaves it nil; tests
	// inject a stub RoundTripper to exercise batching + retry classification
	// without a network.
	HTTPClient *http.Client
}

// Client wraps the official Metronome Go SDK with bex's batching (≤100/call),
// retry/backoff, and dead-letter policy, so the emitter stays about mapping
// rows to events rather than HTTP mechanics.
type Client struct {
	mc      metronome.Client
	backoff []time.Duration
	// sleep waits d honoring ctx; overridable in tests to skip real waits.
	sleep func(ctx context.Context, d time.Duration) error

	// RateCardID / USDCreditTypeID mirror Config; ContractStart is the contract
	// starting_at (the billing epoch), set by main.go alongside the emitter.
	RateCardID      string
	USDCreditTypeID string
	ContractStart   time.Time

	ensuredMu  sync.Mutex
	ensured    map[string]struct{} // tenants whose customer this process has provisioned
	contracted map[string]struct{} // tenants whose contract this process has provisioned
}

// New builds a Client, or returns nil when Token is unset — the byte-identical
// "billing off" path. It disables the SDK's own retries (option.WithMaxRetries)
// so this package owns the retry loop and its classification is deterministic.
func New(cfg Config) *Client {
	if cfg.Token == "" {
		return nil
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	opts := []option.RequestOption{
		option.WithBearerToken(cfg.Token),
		option.WithBaseURL(base),
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}
	return &Client{
		mc:              metronome.NewClient(opts...),
		backoff:         defaultBackoff,
		sleep:           sleepCtx,
		RateCardID:      cfg.RateCardID,
		USDCreditTypeID: cfg.USDCreditTypeID,
		ensured:         map[string]struct{}{},
		contracted:      map[string]struct{}{},
	}
}

// EnsureCustomer registers the workspace as a Metronome customer keyed by its
// own id as an ingest alias (ADR040 §2), so usage events carrying
// customer_id=tenantID auto-associate. It is idempotent: a process-local cache
// skips the second call, and a 409 from a customer that already exists (e.g.
// after a restart) is treated as success.
func (c *Client) EnsureCustomer(ctx context.Context, tenantID string) error {
	if c.cached(c.ensured, tenantID) {
		return nil
	}
	_, err := c.mc.V1.Customers.New(ctx, metronome.V1CustomerNewParams{
		Name:          tenantID,
		IngestAliases: []string{tenantID},
	}, option.WithMaxRetries(3))
	if err != nil {
		var apiErr *metronome.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			c.mark(c.ensured, tenantID) // already exists — the alias is what we need
			return nil
		}
		return fmt.Errorf("metronome: ensure customer %s: %w", tenantID, err)
	}
	c.mark(c.ensured, tenantID)
	return nil
}

// cached / mark are the process-local provisioning memo shared by EnsureCustomer
// (c.ensured) and EnsureContract (c.contracted): two independent sets guarded by
// one mutex, so each per-workspace setup is attempted at most once per process.
func (c *Client) cached(set map[string]struct{}, id string) bool {
	c.ensuredMu.Lock()
	defer c.ensuredMu.Unlock()
	_, ok := set[id]
	return ok
}

func (c *Client) mark(set map[string]struct{}, id string) {
	c.ensuredMu.Lock()
	set[id] = struct{}{}
	c.ensuredMu.Unlock()
}

// IngestBatch ships events to /v1/ingest in ≤100-event chunks. A chunk is
// retried on a retryable failure (429/5xx/network) with backoff; a permanent
// client error (non-429 4xx) is dead-lettered — logged and dropped — so one bad
// chunk never blocks the rest. It returns an error only when a chunk exhausts
// its retries (transient outage): the caller then leaves those rows un-stamped
// and re-attempts next cycle, which is safe because the transaction_id dedups.
func (c *Client) IngestBatch(ctx context.Context, events []Event) error {
	for start := 0; start < len(events); start += maxBatch {
		end := min(start+maxBatch, len(events))
		if err := c.ingestChunk(ctx, events[start:end]); err != nil {
			return err
		}
	}
	return nil
}

// ingestChunk ingests one ≤100-event chunk with retries. It returns nil on
// success OR on a dead-lettered permanent failure (already logged), and a
// non-nil error only when retries are exhausted or ctx is cancelled.
func (c *Client) ingestChunk(ctx context.Context, chunk []Event) error {
	params := toIngestParams(chunk)
	for attempt := 0; ; attempt++ {
		// SDK retries disabled (WithMaxRetries 0) — this loop owns them.
		err := c.mc.V1.Usage.Ingest(ctx, params, option.WithMaxRetries(0))
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var apiErr *metronome.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode != http.StatusTooManyRequests && apiErr.StatusCode < 500 {
			// Non-429 4xx: retrying won't help (malformed/rejected). Dead-letter
			// it — record loudly for manual reconciliation, drop it, keep going.
			log.Printf("billing: DLQ %d events dropped after HTTP %d from Metronome ingest: %v", len(chunk), apiErr.StatusCode, err)
			return nil
		}
		if attempt >= len(c.backoff) {
			return fmt.Errorf("metronome ingest: %d events failed after %d attempts: %w", len(chunk), attempt+1, err)
		}
		if serr := c.sleep(ctx, c.backoff[attempt]); serr != nil {
			return serr
		}
	}
}

// toIngestParams maps bex Events onto the SDK's ingest params. Properties are
// strings on the wire (ADR040 §5); the map[string]any is the SDK's shape.
func toIngestParams(events []Event) metronome.V1UsageIngestParams {
	usage := make([]metronome.V1UsageIngestParamsUsage, 0, len(events))
	for _, e := range events {
		props := make(map[string]any, len(e.Properties))
		for k, v := range e.Properties {
			props[k] = v
		}
		usage = append(usage, metronome.V1UsageIngestParamsUsage{
			TransactionID: e.TransactionID,
			CustomerID:    e.CustomerID,
			EventType:     e.EventType,
			Timestamp:     e.Timestamp.UTC().Format(time.RFC3339),
			Properties:    props,
		})
	}
	return metronome.V1UsageIngestParams{Usage: usage}
}

// sleepCtx waits d, returning early with ctx.Err() if ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
