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
	"crypto/sha256"
	"encoding/hex"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// DefaultSealHours is the rewrite horizon before a usage_hourly row is
	// exported (ADR040 §3), aligned with ADR023's
	// 48h compaction clamp: past this point an hour's quantity is final, so it
	// can be shipped after its quantity is final. Overridable via
	// BEX_STRIPE_SEAL_HOURS, but never below the usage rollup's catch-up window
	// (ClampSealHours) — see that function for the integrity invariant.
	DefaultSealHours = 48 * time.Hour

	// BackfillHorizon bounds how far back the emitter ever ships when
	// BEX_STRIPE_EPOCH is unset (§7): the effective floor is
	// max(epoch, now−BackfillHorizon). Stripe rejects meter-event timestamps
	// older than 35 calendar days; 34 days leaves a timezone/API-latency margin
	// and prevents a first enable from billing unbounded history.
	BackfillHorizon = 34 * 24 * time.Hour

	// DefaultInterval is how often the emitter drains the outbox. Newly sealed
	// rows appear continuously; hourly export latency is immaterial against
	// monthly invoicing.
	DefaultInterval = time.Hour

	// DefaultBatchLimit caps rows fetched per outbox pass; emitOnce loops until
	// the backlog drains, so this only bounds memory + one ingest fan-out.
	DefaultBatchLimit = 1000

	// ProviderDedupWindow is Stripe v1 meter events' documented rolling
	// identifier uniqueness window. Attempts older than this are quarantined for
	// reconciliation rather than replayed.
	ProviderDedupWindow = 24 * time.Hour
)

// ClampSealHours raises a configured seal horizon to at least catchupWindow — the
// window during which the usage rollup may still rewrite a usage_hourly row's
// quantity (usage.CatchupWindow). The emitter exports a row once it is older than
// SealHours; if that were shorter than catchupWindow, a row could be exported to
// Stripe and then rewritten by a later catch-up rollup — a silent, unre-emitted
// over/under-bill (w7/m57). Clamping makes the export and rewrite windows
// non-overlapping by construction, so an exported row is provably final. The
// composition root (cmd/api) passes usage.CatchupWindow so the two packages'
// constants are coupled by code, not by a 48h==48h coincidence.
func ClampSealHours(configured, catchupWindow time.Duration) time.Duration {
	if configured < catchupWindow {
		return catchupWindow
	}
	return configured
}

// EmitterStore is the slice of internal/store the emitter needs: the billing
// outbox read + stamp. *store.PGStore satisfies it.
type EmitterStore interface {
	SelectUnemittedUsage(ctx context.Context, floor, sealBefore time.Time, limit int, requirePaymentMethod bool) ([]store.HourlyRow, error)
	MarkUsageAttempted(ctx context.Context, attempts []store.UsageExportAttempt, at time.Time) error
	RecordUsageExportResult(ctx context.Context, accepted []store.UsageExportAttempt, rejected []store.UsageExportReject, at time.Time) error
	QuarantineOldUsageAttempts(ctx context.Context, before, at time.Time) (int64, error)
	BillingExportStats(ctx context.Context, floor, sealBefore, now time.Time, requirePaymentMethod bool) (store.BillingExportStats, error)
}

// Emitter is the seal-then-emit loop (ADR040 §3–5): it ships each sealed,
// above-floor, non-excluded usage_hourly row to Stripe with a deterministic
// meter-event identifier, then stamps emitted_at. Built only when the Stripe
// client is enabled, so with the key unset it never exists.
type Emitter struct {
	Store  EmitterStore
	Client Ingester

	// SealHours, Epoch, Interval, BatchLimit are the tunables; zero values fall
	// back to the Default… constants (Epoch zero ⇒ the BackfillHorizon floor).
	SealHours  time.Duration
	Epoch      time.Time
	Interval   time.Duration
	BatchLimit int
	Metrics    *Metrics
	// RequirePaymentMethod withholds card-less, non-comped workspaces at the
	// durable outbox read. False preserves the pre-ADR046 export path exactly.
	RequirePaymentMethod bool

	now     func() time.Time
	gapOnce sync.Once
}

// NewEmitter builds an Emitter with production defaults and a real clock. The
// tunable fields (SealHours/Interval/BatchLimit/Epoch) may be overridden before
// Run; main.go sets SealHours/Epoch from env.
func NewEmitter(st EmitterStore, client Ingester) *Emitter {
	return &Emitter{
		Store:      st,
		Client:     client,
		SealHours:  DefaultSealHours,
		Interval:   DefaultInterval,
		BatchLimit: DefaultBatchLimit,
		now:        time.Now,
	}
}

// Run drains the outbox at startup and every Interval until ctx is cancelled.
func (e *Emitter) Run(ctx context.Context) {
	e.emitOnce(ctx)
	t := time.NewTicker(e.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.emitOnce(ctx)
		}
	}
}

// floor is the lowest window_start the emitter will ever export:
// max(Epoch, now−BackfillHorizon). Rows below it are permanently skipped — the
// guard that stops a first enable from shipping months of pre-billing history.
func (e *Emitter) floor(now time.Time) time.Time {
	backfill := now.Add(-BackfillHorizon)
	if e.Epoch.After(backfill) {
		return e.Epoch
	}
	return backfill
}

// emitOnce exports every currently sealed, above-floor, non-excluded un-emitted
// row, draining in batches. The deterministic identifier deduplicates a row
// re-sent after a crash within Stripe's documented rolling uniqueness window.
func (e *Emitter) emitOnce(ctx context.Context) {
	now := e.now().UTC()
	defer e.refreshMetrics(ctx, now)
	sealBefore := now.Add(-e.SealHours)
	floor := e.floor(now)
	e.gapOnce.Do(func() {
		log.Printf("billing: Stripe export active — floor %s (windows before it are never emitted), seal horizon %s",
			floor.Format(time.RFC3339), e.SealHours)
	})
	// With the floor at or after the seal horizon, no row is both sealed and
	// above the floor — nothing to do until more rows age past the horizon.
	if !floor.Before(sealBefore) {
		return
	}
	quarantined, err := e.Store.QuarantineOldUsageAttempts(ctx, now.Add(-ProviderDedupWindow), now)
	if err != nil {
		log.Printf("billing: quarantine expired export attempts: %v", err)
		return
	}
	if quarantined > 0 {
		log.Printf("billing: quarantined %d export attempts beyond provider deduplication window", quarantined)
	}
	for {
		if ctx.Err() != nil {
			return
		}
		rows, err := e.Store.SelectUnemittedUsage(ctx, floor, sealBefore, e.BatchLimit, e.RequirePaymentMethod)
		if err != nil {
			log.Printf("billing: select unemitted usage: %v", err)
			return
		}
		if len(rows) == 0 {
			return
		}
		emitted := e.emitRows(ctx, rows, now)
		// emitted == 0 means this pass made no progress (transient ingest failure
		// or every customer's provisioning failed) — stop rather than hot-loop;
		// the next tick retries. A short batch means the backlog is drained.
		if emitted == 0 || len(rows) < e.BatchLimit {
			return
		}
	}
}

// emitRows ensures each distinct workspace's Stripe customer exists, maps the
// rows whose customer is ready into events, ingests them, and stamps emitted_at
// only on a fully successful ingest. Returns how many rows were durably emitted.
func (e *Emitter) emitRows(ctx context.Context, rows []store.HourlyRow, now time.Time) int {
	customerReady := map[string]bool{}
	attempts := make([]store.UsageExportAttempt, 0, len(rows))
	events := make([]Event, 0, len(rows))
	for _, r := range rows {
		ready, seen := customerReady[r.WorkspaceID]
		if !seen {
			err := e.ensureBillingSetup(ctx, r.WorkspaceID)
			ready = err == nil
			customerReady[r.WorkspaceID] = ready
			if !ready {
				log.Printf("billing: provision %s failed, skipping its rows this cycle: %v", r.WorkspaceID, err)
				e.Metrics.Operation("provisioning", "error")
			} else {
				e.Metrics.Operation("provisioning", "success")
			}
		}
		if !ready {
			continue
		}
		event := toEvent(r)
		eventName, _, skip := stripeMeterEvent(event)
		if skip {
			eventName = "local_zero_rate"
		}
		attempts = append(attempts, store.UsageExportAttempt{Row: r, TransactionID: event.TransactionID, EventName: eventName})
		events = append(events, event)
	}
	if len(events) == 0 {
		return 0
	}
	if err := e.Store.MarkUsageAttempted(ctx, attempts, now); err != nil {
		log.Printf("billing: persist %d export attempts before provider call: %v", len(attempts), err)
		return 0
	}
	result := e.Client.IngestBatch(ctx, events)
	byID := make(map[string]store.UsageExportAttempt, len(attempts))
	for _, attempt := range attempts {
		byID[attempt.TransactionID] = attempt
	}
	accepted := make([]store.UsageExportAttempt, 0, len(result.Accepted))
	for _, id := range result.Accepted {
		if attempt, ok := byID[id]; ok {
			accepted = append(accepted, attempt)
		}
	}
	rejected := make([]store.UsageExportReject, 0)
	transient := 0
	for _, failure := range result.Failed {
		attempt, ok := byID[failure.TransactionID]
		if !ok {
			continue
		}
		if failure.Permanent {
			e.Metrics.Operation("meter_event", "rejected")
			rejected = append(rejected, store.UsageExportReject{Attempt: attempt, Code: failure.Code, Message: failure.Message})
		} else {
			e.Metrics.Operation("meter_event", "retryable")
			transient++
		}
	}
	if len(accepted)+len(rejected) > 0 {
		if err := e.Store.RecordUsageExportResult(ctx, accepted, rejected, now); err != nil {
			e.Metrics.Operation("local_stamp", "error")
			// Provider calls may have succeeded while the local transaction failed.
			// The immutable first-attempt timestamp bounds automatic retry; once the
			// 24h identifier window closes the rows become explicit ambiguity.
			log.Printf("billing: persist export result accepted=%d rejected=%d: %v", len(accepted), len(rejected), err)
			return 0
		}
		e.Metrics.Operation("local_stamp", "success")
	}
	for range accepted {
		e.Metrics.Operation("meter_event", "accepted")
	}
	if transient > 0 {
		log.Printf("billing: %d meter events remain pending after retryable provider failures", transient)
		return 0 // stop this pass so retryable rows cannot hot-loop
	}
	return len(accepted) + len(rejected)
}

func (e *Emitter) refreshMetrics(ctx context.Context, now time.Time) {
	if e.Metrics == nil {
		return
	}
	stats, err := e.Store.BillingExportStats(ctx, e.floor(now), now.Add(-e.SealHours), now, e.RequirePaymentMethod)
	if err != nil {
		e.Metrics.Operation("outbox_snapshot", "error")
		return
	}
	e.Metrics.SetExportStats(stats)
	e.Metrics.Operation("outbox_snapshot", "success")
	if stats.WithheldWorkspaces > 0 {
		log.Printf("billing: withheld sealed usage for %d workspace(s) awaiting a payment method", stats.WithheldWorkspaces)
	}
}

// ensureBillingSetup provisions a workspace's Stripe billing before its usage
// ships: the metadata-keyed Customer and the multi-item Subscription that rates
// every configured meter. Mode A exclusions are filtered by the store query
// before this point.
func (e *Emitter) ensureBillingSetup(ctx context.Context, workspaceID string) error {
	if err := e.Client.EnsureCustomer(ctx, workspaceID); err != nil {
		return err
	}
	return e.Client.EnsureContract(ctx, workspaceID)
}

// toEvent maps one sealed usage row onto a Stripe meter event. The identifier is
// a deterministic hash of the row identity so retries inside Stripe's rolling
// uniqueness window deduplicate; quantity and dimensions remain strings.
func toEvent(r store.HourlyRow) Event {
	rk := store.NormalizeResourceKind(r.ResourceKind)
	return Event{
		TransactionID: transactionID(rk, r.ServiceID, r.Kind, r.Tier, r.WindowStart),
		CustomerID:    r.WorkspaceID,
		EventType:     r.Kind,
		Timestamp:     r.WindowStart,
		Properties: map[string]string{
			"tier":          r.Tier,
			"resource_kind": rk,
			"service_id":    r.ServiceID,
			"value":         strconv.FormatInt(r.Quantity, 10),
		},
	}
}

// transactionID is sha256("<resource_kind>|<service_id>|<kind>|<tier>|<window_start RFC3339>")
// — deterministic, so the same sealed row always maps to the same idempotency
// key (ADR040 §3). Its 64 hexadecimal characters fit Stripe's 100-character
// identifier limit.
func transactionID(resourceKind, serviceID, kind, tier string, windowStart time.Time) string {
	sum := sha256.Sum256([]byte(resourceKind + "|" + serviceID + "|" + kind + "|" + tier + "|" + windowStart.UTC().Format(time.RFC3339)))
	return hex.EncodeToString(sum[:])
}
