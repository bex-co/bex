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

// Package usage is the metering feature (w8/m1–m2, retention m4): an hourly
// pipeline that rolls Prometheus cAdvisor/Traefik data and build-Job durations
// into durable usage_hourly rows (internal/store), keyed per workspace. The
// Service exposes month-to-date aggregates as REST/GraphQL/MCP adapters (m2)
// and bounds usage_hourly's growth by compacting months older than the hot
// window into usage_monthly aggregates daily (m4, docs/ADR023-usage-metering.md).
// The loop needs BEX_CP_DB_URI (store); metering additionally needs
// BEX_PROM_URL (Prometheus) — with the store absent the package is a no-op
// and the rest of bex-api is byte-for-byte unchanged.
package usage

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/egressquery"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// UsageStore is the slice of internal/store the usage feature needs: the
// write/read methods over the usage_hourly + usage_monthly tables, plus a
// read for the store's apps list (workspace attribution) and tenant lookup.
type UsageStore interface {
	ListApps(ctx context.Context) ([]store.App, error)
	UpsertUsageHourly(ctx context.Context, row store.HourlyRow) error
	RecordUsageSourceHealth(ctx context.Context, records []store.UsageSourceRecord) error
	ReconcileUsageSourceStreams(ctx context.Context, active []store.UsageResourceRef, through time.Time) error
	CurrentUsageCoverage(ctx context.Context, workspaceID string, now time.Time) (store.UsageCoverage, error)
	LatestUsageWindow(ctx context.Context, resourceKind, serviceID, kind string) (time.Time, error)
	UsageMonthToDate(ctx context.Context, workspaceID string, now time.Time) ([]store.UsageSummaryRow, error)
	CompactUsage(ctx context.Context, before time.Time) (store.UsageCompaction, error)
}

// Service is the usage feature. Base carries the Kubernetes client, namespace,
// and the authz gate every verb calls first. Store is the control-plane
// backing; nil means the store isn't wired and usage verbs report unavailable.
type Service struct {
	*core.Base
	Store    UsageStore
	PromBase string // BEX_PROM_URL, empty means no Prometheus
	// RetentionMonths is the hot window: how many calendar months (current
	// month included) stay at hourly granularity before compaction folds them
	// into usage_monthly (BEX_USAGE_RETENTION_MONTHS). Values < 1 mean
	// DefaultRetentionMonths.
	RetentionMonths int
	// Billing reads real Stripe cost/invoices to surface beside the advisory
	// estimate (m48, ADR040 Phase 2). nil ⇒ billing surface off: every summary is
	// estimate-only, byte-identical to ADR030. A read failure degrades to
	// estimate-only too (logged, never a 500).
	Billing BillingReader
	// billingCache memoizes the Stripe reads BillingFor makes, keyed by
	// (tenant, period), for a short TTL: /v1/usage is polled (dashboard every 60s)
	// across three surfaces, but the billing figure only moves ~hourly, so this
	// keeps that hot path off Stripe. Positive results only — errors are never
	// cached, preserving the poll-as-retry graceful-degradation contract.
	billingCache *core.TTLCache[*billing.Billing]
	promHTTP     *http.Client
}

// BillingReader reads a workspace's real Stripe cost + finalized invoices
// for [periodStart, periodEnd). Returns (nil, nil) when there is nothing real
// to show (no contract / comped Mode A / excluded) so the caller falls back to
// estimate-only. Implemented by *billing.Client.
type BillingReader interface {
	BillingFor(ctx context.Context, customerID string, periodStart, periodEnd time.Time) (*billing.Billing, error)
}

// NewService constructs a Service, normalising PromBase (strips trailing slash
// so every Prometheus URL is canonical) and resolving a nil HTTP client to the
// bounded core.UpstreamClient (codex round-8 #10). Callers in tests may pass
// their own *http.Client to target an httptest.Server.
func NewService(base *core.Base, st UsageStore, promBase string, hc *http.Client) *Service {
	if hc == nil {
		hc = core.UpstreamClient
	}
	return &Service{
		Base:         base,
		Store:        st,
		PromBase:     strings.TrimRight(promBase, "/"),
		billingCache: core.NewTTLCache[*billing.Billing](),
		promHTTP:     hc,
	}
}

// --- Month-to-date verb (m2) ---

// Summary is the month-to-date usage for one workspace — the core result the
// three adapters present. Total values are in the natural unit of each kind
// (seconds/bytes/seconds/GB-seconds).
type Summary struct {
	WorkspaceID   string
	Period        string // "YYYY-MM" — the calendar month this summary covers
	Services      []ServiceUsage
	EstimatedCost pricing.EstimatedCost
	// Billing is the real Stripe-computed cost + finalized invoices (m48/m50).
	// nil ⇒ estimate-only: no contract, comped/excluded, billing off, or a
	// degraded Stripe read — clients show EstimatedCost alone.
	Billing  *billing.Billing
	Coverage Coverage
}

const (
	CoverageComplete = "complete"
	CoveragePartial  = "partial"
	CoverageUnknown  = "unknown"
)

// Coverage describes the evidence behind current-month totals. Through is the
// common end of every active source stream's contiguous hourly prefix. It is
// zero for unknown evidence and never inferred for historical periods.
type Coverage struct {
	State           string
	Through         time.Time
	DegradedSources []string
}

// ServiceUsage is one service's contribution to the workspace's month-to-date
// totals, broken out by kind (and tier for instance_seconds).
type ServiceUsage struct {
	ServiceID string
	// ServiceName is the user-facing display name, resolved best-effort from
	// the store (Apps) or the CR spec (datastores). Empty when the resource no
	// longer exists — presenters fall back to ServiceID.
	ServiceName  string
	ResourceKind string // store.ResourceKind* — "service", "postgres", "key_value"
	Rows         []store.UsageSummaryRow
}

// monthToDateAt is the implementation used by adapters. now controls both the
// calendar month (monthStart is derived from it) and the inclusive upper bound
// of the query. Adapters pass s.Now().UTC() for the live query or the start of
// the next month for a full historical-month query. ownerID names the
// workspace to query (Render's `ownerId`, w6/m18): empty means the caller's
// default workspace, honored via core.WithWorkspace/Base.Tenant the same way
// every other read-side verb resolves an explicit target — a workspace the
// caller is not a member of is core.ErrForbidden.
func (s *Service) monthToDateAt(ctx context.Context, ownerID string, now time.Time) (Summary, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Summary{}, err
	}
	if s.Store == nil {
		return Summary{}, core.ErrUsageUnavailable
	}
	tenantID, ok := s.Base.Tenant(ctx)
	if !ok {
		// Store is off or caller has no tenant: return an empty summary rather
		// than an error — the same pattern as apps.List returning all apps in the
		// store-off mode.
		return Summary{}, nil
	}
	rows, err := s.Store.UsageMonthToDate(ctx, tenantID, now)
	if err != nil {
		return Summary{}, fmt.Errorf("usage: %w", err)
	}
	sum := summarise(tenantID, rows)
	s.resolveServiceNames(ctx, sum.Services)
	sum.Period = now.Format(periodLayout)
	sum.EstimatedCost = pricing.Default.Estimate(rows)
	sum.Billing = s.readBilling(ctx, tenantID, now)
	sum.Coverage = Coverage{State: CoverageUnknown, DegradedSources: []string{}}
	current := s.Now().UTC()
	if now.Year() == current.Year() && now.Month() == current.Month() {
		evidence, err := s.Store.CurrentUsageCoverage(ctx, tenantID, current)
		if err != nil {
			return Summary{}, fmt.Errorf("usage coverage: %w", err)
		}
		if evidence.Known {
			sum.Coverage.State = CoveragePartial
			if evidence.Complete {
				sum.Coverage.State = CoverageComplete
			}
			sum.Coverage.Through = evidence.Through
			sum.Coverage.DegradedSources = append([]string(nil), evidence.DegradedSources...)
		}
	}
	return sum, nil
}

// readBilling fetches the workspace's real Stripe cost for the month containing
// now. Any failure (Stripe off/unreachable, no subscription) yields
// nil — the summary stays estimate-only and never 500s (ADR040 graceful
// degradation). The estimate always remains for the in-flight window.
func (s *Service) readBilling(ctx context.Context, tenantID string, now time.Time) *billing.Billing {
	if s.Billing == nil {
		return nil
	}
	key := tenantID + "|" + now.Format(periodLayout)
	if s.billingCache != nil {
		if b, ok := s.billingCache.Get(key); ok {
			return b
		}
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	b, err := s.Billing.BillingFor(ctx, tenantID, monthStart, now)
	if err != nil {
		log.Printf("usage: billing read for %s: %v (estimate-only)", tenantID, err)
		return nil // never cache an error — the next poll retries
	}
	if s.billingCache != nil {
		// Real wall-clock expiry (the cache's own clock), independent of s.Now().
		s.billingCache.Put(key, b, time.Now().Add(core.PositiveTTL))
	}
	return b
}

// resolveServiceNames fills each ServiceUsage's display name from the store
// (Apps) and the Database/KeyValue CRs (datastores). Best-effort: a lookup
// failure or a since-deleted resource leaves ServiceName empty and the summary
// otherwise intact. Keys pair ResourceKind with the id because different kinds
// may legally share a service_id.
func (s *Service) resolveServiceNames(ctx context.Context, svcs []ServiceUsage) {
	if len(svcs) == 0 {
		return
	}
	names := map[string]string{}
	if apps, err := s.Store.ListApps(ctx); err != nil {
		log.Printf("usage: resolve names: list apps: %v", err)
	} else {
		for _, app := range apps {
			names[store.ResourceKindService+"/"+app.ID] = app.Name
		}
	}
	if datastores, err := s.listDatastores(ctx); err != nil {
		log.Printf("usage: resolve names: list datastores: %v", err)
	} else {
		for _, ds := range datastores {
			names[ds.Kind+"/"+ds.ID] = ds.Display
		}
	}
	for i := range svcs {
		svcs[i].ServiceName = names[svcs[i].ResourceKind+"/"+svcs[i].ServiceID]
	}
}

// MonthToDate returns ownerID's month-to-date usage summary for the current
// calendar month ("" => the caller's default workspace).
func (s *Service) MonthToDate(ctx context.Context, ownerID string) (Summary, error) {
	return s.monthToDateAt(ctx, ownerID, s.Now().UTC())
}

// summarise groups flat summary rows by resource identity. ResourceKind is
// required because different Kubernetes kinds may legally share a service_id.
func summarise(workspaceID string, rows []store.UsageSummaryRow) Summary {
	rows = append([]store.UsageSummaryRow(nil), rows...)
	for i := range rows {
		rows[i].ResourceKind = store.NormalizeResourceKind(rows[i].ResourceKind)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ServiceID != b.ServiceID {
			return a.ServiceID < b.ServiceID
		}
		if a.ResourceKind != b.ResourceKind {
			return a.ResourceKind < b.ResourceKind
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Tier < b.Tier
	})
	type resourceKey struct {
		serviceID, resourceKind string
	}
	var order []resourceKey
	byResource := map[resourceKey][]store.UsageSummaryRow{}
	for _, r := range rows {
		key := resourceKey{r.ServiceID, r.ResourceKind}
		if _, seen := byResource[key]; !seen {
			order = append(order, key)
		}
		byResource[key] = append(byResource[key], r)
	}
	svcs := make([]ServiceUsage, 0, len(order))
	for _, key := range order {
		svcs = append(svcs, ServiceUsage{
			ServiceID:    key.serviceID,
			ResourceKind: key.resourceKind,
			Rows:         byResource[key],
		})
	}
	return Summary{WorkspaceID: workspaceID, Services: svcs}
}

// --- Metering + retention loop ---

// Interval is the default rollup cadence.
const Interval = time.Hour

// CompactInterval is the default retention-compaction cadence (w8/m4). Daily
// is plenty: eligibility only changes at month boundaries.
const CompactInterval = 24 * time.Hour

// DefaultRetentionMonths is the default hot window (t001): the current month
// plus the prior two stay hourly — the common historical-comparison range.
const DefaultRetentionMonths = 3

// catchupLimit caps how far back a restart can catch up (48 h covers
// Prometheus's typical retention + a weekend outage).
const catchupLimit = 48 * time.Hour

// CatchupWindow exports catchupLimit so the billing seal horizon can be clamped
// to never fall inside the rollup's rewrite window (w7/m57): a usage_hourly row
// is rewritable for up to CatchupWindow, so the Stripe seal horizon must be at
// least this long, else an already-exported row could be silently rewritten and
// never re-emitted (under/over-billing). The two constants lived in separate
// packages coupled only by the coincidence 48h == 48h; this makes the coupling
// explicit (billing.ClampSealHours reads it, main.go enforces it).
const CatchupWindow = catchupLimit

// Run is the metering + retention loop: catches up missed windows on startup,
// rolls up every Interval, and compacts spent months every CompactInterval,
// until ctx is cancelled. Only call when Store is set; with PromBase empty the
// metering side is skipped and only compaction runs.
func (s *Service) Run(ctx context.Context) {
	s.RunWithIntervals(ctx, Interval, CompactInterval)
}

// RunWithIntervals is like Run but with explicit cadences — for tests.
func (s *Service) RunWithIntervals(ctx context.Context, rollup, compact time.Duration) {
	// Without Prometheus the metering side is structurally off: rollupC stays
	// nil (a receive on a nil channel never fires) and only compaction ticks.
	var rollupC <-chan time.Time
	if s.PromBase != "" {
		rollupTicker := time.NewTicker(rollup)
		defer rollupTicker.Stop()
		rollupC = rollupTicker.C
		s.catchUp(ctx)
	}
	compactTicker := time.NewTicker(compact)
	defer compactTicker.Stop()
	s.compact(ctx) // once at startup so restarts don't defer compaction a day
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-rollupC:
			s.rollup(ctx, t)
		case <-compactTicker.C:
			s.compact(ctx)
		}
	}
}

// compact folds hourly rows older than the hot window into usage_monthly and
// purges them (w8/m4). The boundary is the start of the oldest hot month,
// clamped to catchupLimit ago: the rollup's restart catch-up can rewrite any
// window inside that limit, and re-metering an already-compacted window would
// double-count against the additive monthly upsert. Rows the clamp defers are
// compacted by a later pass once they age out of the catch-up range.
func (s *Service) compact(ctx context.Context) {
	months := s.RetentionMonths
	if months < 1 {
		months = DefaultRetentionMonths
	}
	now := s.Now().UTC()
	before := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1-months, 0)
	if clamp := now.Add(-catchupLimit); before.After(clamp) {
		before = clamp
	}
	res, err := s.Store.CompactUsage(ctx, before)
	if err != nil {
		log.Printf("usage: compact before %s: %v", before.Format(time.RFC3339), err)
		return
	}
	if res.HourlyRows > 0 {
		log.Printf("usage: compacted %d months (%d hourly rows) into usage_monthly (boundary %s)",
			res.Months, res.HourlyRows, before.Format(time.RFC3339))
	}
}

// datastoreEntry holds the metering attributes for one Database or KeyValue CR.
type datastoreEntry struct {
	ID        string // immutable CR name (== service_id in usage rows)
	Name      string // immutable data-plane name used in Prometheus selectors
	Namespace string // the CR's OWN namespace (ADR043 D8) — Prometheus series are namespace-labeled
	Display   string // required mutable user-facing spec.name
	TenantID  string // from core.LabelTenant
	Plan      string // Spec.Plan (== tier in usage rows)
	Kind      string // store.ResourceKindPostgres or store.ResourceKindKeyValue
	Public    bool   // whether the public proxy source applies
	CreatedAt time.Time
}

// listDatastores returns all tenant-owned Database and KeyValue CRs via the k8s
// client. Skips unlabeled CRs (not tenant-owned). Returns nil when the k8s
// client is not wired.
//
// The lists are CLUSTER-WIDE: metering runs fleet-wide with no caller identity,
// and under ADR043 D8 each workspace's datastores sit in its own namespace, so
// a namespace-scoped list would meter only the un-migrated ones — under-billing
// silently, which is the worst failure mode this loop has.
func (s *Service) listDatastores(ctx context.Context) ([]datastoreEntry, error) {
	if s.Client == nil {
		return nil, nil
	}
	var out []datastoreEntry
	var dbs appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &dbs); err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	for _, d := range dbs.Items {
		tenantID := d.Labels[core.LabelTenant]
		if tenantID == "" {
			continue
		}
		out = append(out, datastoreEntry{
			ID:        d.Name,
			Name:      d.Name,
			Namespace: d.Namespace,
			Display:   d.Spec.Name,
			TenantID:  tenantID,
			Plan:      d.Spec.Plan,
			Kind:      store.ResourceKindPostgres,
			Public:    d.Spec.Public,
			CreatedAt: d.CreationTimestamp.Time,
		})
	}
	var kvs appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &kvs); err != nil {
		return nil, fmt.Errorf("list keyvalues: %w", err)
	}
	for _, kv := range kvs.Items {
		tenantID := kv.Labels[core.LabelTenant]
		if tenantID == "" {
			continue
		}
		out = append(out, datastoreEntry{
			ID:        kv.Name,
			Name:      kv.Name,
			Namespace: kv.Namespace,
			Display:   kv.Spec.Name,
			TenantID:  tenantID,
			Plan:      kv.Spec.Plan,
			Kind:      store.ResourceKindKeyValue,
			Public:    kv.Spec.Public,
			CreatedAt: kv.CreationTimestamp.Time,
		})
	}
	return out, nil
}

var datastoreMeterKinds = [...]string{
	store.UsageKindInstanceSeconds,
	store.UsageKindStorageGBSeconds,
	store.UsageKindEgressBytes,
}

// processDatastoreWindow measures every applicable meter for one datastore.
// The live loop gives each meter an independent cursor; this seam runs them
// together for tests and explicit one-window processing.
func (s *Service) processDatastoreWindow(ctx context.Context, ds datastoreEntry, window time.Time) bool {
	var wg sync.WaitGroup
	results := make(chan bool, len(datastoreMeterKinds))
	for _, kind := range datastoreMeterKinds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.processDatastoreMeterWindow(ctx, ds, kind, window)
		}()
	}
	wg.Wait()
	close(results)
	ok := true
	for result := range results {
		ok = ok && result
	}
	return ok
}

func (s *Service) processDatastoreMeterWindow(ctx context.Context, ds datastoreEntry, kind string, window time.Time) bool {
	end := window.Add(time.Hour)
	var quantity int64
	var ok bool
	var health []store.UsageSourceObservation
	expectedFrom := usageExpectedFrom(ds.CreatedAt)
	switch kind {
	case store.UsageKindInstanceSeconds:
		quantity, ok = s.queryInstanceSecondsStateful(ctx, ds.Name, ds.Namespace, window, end)
		health = oneSourceHealth(store.UsageSourceInstance, ok, expectedFrom)
	case store.UsageKindStorageGBSeconds:
		quantity, ok = s.queryStorageGBSeconds(ctx, ds, window, end)
		health = oneSourceHealth(store.UsageSourceStorage, ok, expectedFrom)
	case store.UsageKindEgressBytes:
		quantity, health, ok = s.queryDatastoreEgressBytesEvidence(ctx, ds, window, end)
		for i := range health {
			health[i].ExpectedFrom = expectedFrom
		}
	default:
		return false
	}
	if !ok {
		s.recordSourceHealth(ctx, ds.TenantID, ds.Kind, ds.ID, kind, window, health)
		return false
	}
	tier := ""
	if kind == store.UsageKindInstanceSeconds {
		tier = ds.Plan
	}
	if err := s.Store.UpsertUsageHourly(ctx, store.HourlyRow{
		WorkspaceID: ds.TenantID, ServiceID: ds.ID, ResourceKind: ds.Kind,
		Kind: kind, Tier: tier, WindowStart: window, Quantity: quantity,
		SourceHealth: health,
	}); err != nil {
		log.Printf("usage: upsert %s datastore %s: %v", kind, ds.ID, err)
		return false
	}
	return true
}

// catchUpDatastoreThrough advances every datastore meter independently. A
// proxy outage can hold egress back without blocking compute or storage, and
// each meter retries its oldest gap before writing a newer window.
func (s *Service) catchUpDatastoreThrough(ctx context.Context, ds datastoreEntry, last time.Time) {
	var wg sync.WaitGroup
	for _, kind := range datastoreMeterKinds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.catchUpDatastoreMeterThrough(ctx, ds, kind, last)
		}()
	}
	wg.Wait()
}

func (s *Service) catchUpDatastoreMeterThrough(ctx context.Context, ds datastoreEntry, kind string, last time.Time) {
	latest, err := s.Store.LatestUsageWindow(ctx, ds.Kind, ds.ID, kind)
	if err != nil {
		log.Printf("usage: catch-up: latest %s window for datastore %s: %v", kind, ds.ID, err)
		return
	}
	floor := last.Add(time.Hour - catchupLimit)
	start := latest.UTC().Truncate(time.Hour)
	if start.IsZero() || start.Before(floor) {
		start = floor
	} else {
		start = start.Add(time.Hour)
	}
	for w := start; !w.After(last); w = w.Add(time.Hour) {
		if !s.processDatastoreMeterWindow(ctx, ds, kind, w) {
			return
		}
	}
}

// catchUp finds the latest recorded window per service and fills any gap since
// then (bounded to catchupLimit so a long-dead service doesn't sweep months).
func (s *Service) catchUp(ctx context.Context) {
	apps, err := s.Store.ListApps(ctx)
	if err != nil {
		log.Printf("usage: catch-up: list apps: %v", err)
		return
	}
	now := time.Now().UTC()
	last := now.Truncate(time.Hour).Add(-time.Hour)
	for _, app := range apps {
		s.catchUpAppThrough(ctx, app, last)
	}
	datastores, err := s.listDatastores(ctx)
	if err != nil {
		log.Printf("usage: catch-up: list datastores: %v", err)
		return
	}
	for _, ds := range datastores {
		s.catchUpDatastoreThrough(ctx, ds, last)
	}
	s.reconcileSourceInventory(ctx, apps, datastores, last.Add(time.Hour))
}

// rollup processes the window that just closed (the hour ending at t).
func (s *Service) rollup(ctx context.Context, t time.Time) {
	window := t.UTC().Truncate(time.Hour).Add(-time.Hour)
	apps, err := s.Store.ListApps(ctx)
	if err != nil {
		log.Printf("usage: rollup: list apps: %v", err)
		return
	}
	for _, app := range apps {
		s.catchUpAppThrough(ctx, app, window)
	}
	datastores, err := s.listDatastores(ctx)
	if err != nil {
		log.Printf("usage: rollup: list datastores: %v", err)
		return
	}
	for _, ds := range datastores {
		s.catchUpDatastoreThrough(ctx, ds, window)
	}
	s.reconcileSourceInventory(ctx, apps, datastores, window.Add(time.Hour))
	log.Printf("usage: rolled up window %s for %d services + %d datastores",
		window.Format(time.RFC3339), len(apps), len(datastores))
}

func (s *Service) reconcileSourceInventory(ctx context.Context, apps []store.App, datastores []datastoreEntry, through time.Time) {
	active := make([]store.UsageResourceRef, 0, len(apps)+len(datastores))
	for _, app := range apps {
		active = append(active, store.UsageResourceRef{ResourceKind: store.ResourceKindService, ServiceID: app.ID})
	}
	for _, ds := range datastores {
		active = append(active, store.UsageResourceRef{ResourceKind: ds.Kind, ServiceID: ds.ID})
	}
	if err := s.Store.ReconcileUsageSourceStreams(ctx, active, through); err != nil {
		log.Printf("usage: reconcile source inventory through %s: %v", through.Format(time.RFC3339), err)
	}
}

var appMeterKinds = [...]string{
	store.UsageKindInstanceSeconds,
	store.UsageKindEgressBytes,
	store.UsageKindBuildSeconds,
}

// processWindow measures one (app, window) triplet and upserts all three kinds.
// It is kept as the direct one-window seam used by acceptance tests. The live
// loop calls catchUpAppThrough so each meter owns an independent contiguous
// cursor. A failure in one source never holds the other two meters back.
func (s *Service) processWindow(ctx context.Context, app store.App, window time.Time) {
	var wg sync.WaitGroup
	for _, kind := range appMeterKinds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.processAppMeterWindow(ctx, app, kind, window)
		}()
	}
	wg.Wait()
}

// catchUpAppThrough advances each App meter independently through last. Each
// meter processes oldest-first and stops at its first unavailable source or
// failed store write, preserving a contiguous cursor without blocking healthy
// meters. The next hourly pass retries the failed window (restart not needed).
func (s *Service) catchUpAppThrough(ctx context.Context, app store.App, last time.Time) {
	var wg sync.WaitGroup
	for _, kind := range appMeterKinds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.catchUpAppMeterThrough(ctx, app, kind, last)
		}()
	}
	wg.Wait()
}

func (s *Service) catchUpAppMeterThrough(ctx context.Context, app store.App, kind string, last time.Time) {
	latest, err := s.Store.LatestUsageWindow(ctx, store.ResourceKindService, app.ID, kind)
	if err != nil {
		log.Printf("usage: catch-up: latest %s window for %s: %v", kind, app.ID, err)
		return
	}
	floor := last.Add(time.Hour - catchupLimit)
	if created := app.CreatedAt.UTC().Truncate(time.Hour); !created.IsZero() && created.After(floor) {
		floor = created
	}
	start := latest.UTC().Truncate(time.Hour)
	initializing := start.IsZero() || start.Before(floor)
	if initializing {
		start = floor
	} else {
		start = start.Add(time.Hour)
	}
	for window := start; !window.After(last); window = window.Add(time.Hour) {
		result := s.processAppMeterWindowResult(ctx, app, kind, window)
		if result == appMeterWindowSuccess {
			initializing = false
			continue
		}
		// A newly introduced egress composition has no meaningful cursor before
		// all of its sources exist. Probe forward through source-unavailable
		// history until the first measurable hour establishes the evidence
		// origin. Once any row exists, every later failure still stops the cursor
		// and is retried before a newer window. Store failures are never skipped.
		if kind == store.UsageKindEgressBytes && initializing && result == appMeterWindowSourceUnavailable {
			continue
		}
		return
	}
}

type appMeterWindowResult uint8

const (
	appMeterWindowSuccess appMeterWindowResult = iota
	appMeterWindowSourceUnavailable
	appMeterWindowStoreFailure
)

// processAppMeterWindow persists a successful measurement even when it is
// zero. Zero is real coverage evidence; source unavailability is represented
// by no row so the per-meter cursor retries the window later.
func (s *Service) processAppMeterWindow(ctx context.Context, app store.App, kind string, window time.Time) bool {
	return s.processAppMeterWindowResult(ctx, app, kind, window) == appMeterWindowSuccess
}

func (s *Service) processAppMeterWindowResult(ctx context.Context, app store.App, kind string, window time.Time) appMeterWindowResult {
	end := window.Add(time.Hour)
	var quantity int64
	var ok bool
	var health []store.UsageSourceObservation
	expectedFrom := usageExpectedFrom(app.CreatedAt)
	switch kind {
	case store.UsageKindInstanceSeconds:
		// The App's pods live in its per-tenant namespace under ADR043, so the
		// cAdvisor `namespace=` matcher must target AppNamespace(app.TenantID), not
		// the shared namespace (which after the migration holds none of its pods).
		quantity, ok = s.queryInstanceSeconds(ctx, app.Name, s.AppNamespace(app.TenantID), window, end)
		health = oneSourceHealth(store.UsageSourceInstance, ok, expectedFrom)
	case store.UsageKindEgressBytes:
		quantity, health, ok = s.queryEgressBytesEvidence(ctx, app, window, end)
		for i := range health {
			health[i].ExpectedFrom = expectedFrom
		}
	case store.UsageKindBuildSeconds:
		quantity, ok = s.queryBuildSeconds(ctx, app.Name, window, end)
		health = oneSourceHealth(store.UsageSourceBuild, ok, expectedFrom)
	default:
		log.Printf("usage: unknown App meter %q for %s", kind, app.ID)
		return appMeterWindowSourceUnavailable
	}
	if !ok {
		s.recordSourceHealth(ctx, app.TenantID, store.ResourceKindService, app.ID, kind, window, health)
		return appMeterWindowSourceUnavailable
	}
	tier := ""
	if kind == store.UsageKindInstanceSeconds {
		tier = app.Tier
	}
	if err := s.Store.UpsertUsageHourly(ctx, store.HourlyRow{
		WorkspaceID:  app.TenantID,
		ServiceID:    app.ID,
		ResourceKind: store.ResourceKindService,
		Kind:         kind,
		Tier:         tier,
		WindowStart:  window,
		Quantity:     quantity,
		SourceHealth: health,
	}); err != nil {
		log.Printf("usage: upsert %s %s: %v", kind, app.ID, err)
		return appMeterWindowStoreFailure
	}
	return appMeterWindowSuccess
}

func usageExpectedFrom(created time.Time) time.Time {
	if created.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return created.UTC().Truncate(time.Hour)
}

func oneSourceHealth(source string, healthy bool, expectedFrom time.Time) []store.UsageSourceObservation {
	state := store.UsageSourceHealthy
	if !healthy {
		state = store.UsageSourceUnavailable
	}
	return []store.UsageSourceObservation{{Source: source, State: state, ExpectedFrom: expectedFrom}}
}

func (s *Service) recordSourceHealth(ctx context.Context, workspaceID, resourceKind, serviceID, kind string, window time.Time, observations []store.UsageSourceObservation) {
	records := make([]store.UsageSourceRecord, 0, len(observations))
	for _, observation := range observations {
		records = append(records, store.UsageSourceRecord{
			WorkspaceID: workspaceID, ResourceKind: resourceKind, ServiceID: serviceID,
			Kind: kind, WindowStart: window, UsageSourceObservation: observation,
		})
	}
	if err := s.Store.RecordUsageSourceHealth(ctx, records); err != nil {
		log.Printf("usage: record %s source health for %s: %v", kind, serviceID, err)
	}
}

// --- t002: Prometheus rollup queries ---

// queryInstanceSeconds returns how many seconds at least one container for the
// app was running in [start, end), using cAdvisor's
// container_memory_working_set_bytes as a presence signal (same matcher as the
// metrics feature's instance-count query). The result is count-of-present-pods
// × window-seconds, truncated to an integer. Used for App (ReplicaSet) pods
// whose names follow the two-segment pattern <appName>-<hash>-<hash5>.
func (s *Service) queryInstanceSeconds(ctx context.Context, appName, namespace string, start, end time.Time) (int64, bool) {
	matchers := fmt.Sprintf(
		`namespace=%q,pod=~"%s-[a-z0-9]+-[a-z0-9]{5}",container!=""`,
		namespace, egressquery.RegexEscape(appName))
	return s.queryInstanceSecondsByMatcher(ctx, matchers, start, end, appName)
}

// queryInstanceSecondsStateful returns instance-seconds for a StatefulSet-style
// workload (CNPG Cluster pods <dbName>-1, <dbName>-2 or Valkey StatefulSet pods
// <kvName>-0). The pod pattern is <name>-[0-9]+ (ordinal suffix), distinct from
// App ReplicaSet pods which use two random-char segments.
func (s *Service) queryInstanceSecondsStateful(ctx context.Context, name, namespace string, start, end time.Time) (int64, bool) {
	matchers := fmt.Sprintf(
		`namespace=%q,pod=~"%s-[0-9]+",container!=""`,
		namespace, egressquery.RegexEscape(name))
	return s.queryInstanceSecondsByMatcher(ctx, matchers, start, end, name)
}

const bytesPerDecimalGB = 1_000_000_000

// queryStorageGBSeconds returns average used PVC bytes over [start,end),
// converted to decimal GB and multiplied by window seconds. kubelet volume
// stats are already scraped cluster-wide for datastore metrics. CNPG PVCs are
// named <name>-<ordinal>; Valkey's volumeClaimTemplate produces
// data-<name>-<ordinal>. The sum includes every CNPG instance/replica PVC.
//
// The bool distinguishes a successful zero (persist it) from an unavailable
// Prometheus query (leave the row absent so catch-up can retry).
func (s *Service) queryStorageGBSeconds(ctx context.Context, ds datastoreEntry, start, end time.Time) (int64, bool) {
	if s.PromBase == "" {
		return 0, false
	}
	pattern := fmt.Sprintf(`^%s-[0-9]+$`, egressquery.RegexEscape(ds.Name))
	if ds.Kind == store.ResourceKindKeyValue {
		pattern = fmt.Sprintf(`^data-%s-[0-9]+$`, egressquery.RegexEscape(ds.Name))
	}
	windowSecs := int64(end.Sub(start) / time.Second)
	q := fmt.Sprintf(
		`sum(avg_over_time(kubelet_volume_stats_used_bytes{namespace=%q,persistentvolumeclaim=~%q}[%ds]))`,
		ds.Namespace, pattern, windowSecs)
	usedBytes, err := egressquery.Instant(ctx, s.promHTTP, s.PromBase, q, end)
	if err != nil {
		log.Printf("usage: storage_gb_seconds for %s: %v", ds.Name, err)
		return 0, false
	}
	if usedBytes < 0 {
		usedBytes = 0
	}
	return int64(math.Round(usedBytes * float64(windowSecs) / bytesPerDecimalGB)), true
}

// queryInstanceSecondsByMatcher is the shared cAdvisor instant-query body.
// It counts pods matching matchers, multiplies by windowSecs, and returns
// pod-count × window-seconds. ok is false when Prometheus is unavailable.
func (s *Service) queryInstanceSecondsByMatcher(ctx context.Context, matchers string, start, end time.Time, logName string) (int64, bool) {
	if s.PromBase == "" {
		return 0, false
	}
	// Count the average number of running pods over the window, then multiply
	// by 3600 to get instance-seconds. Using avg_over_time with a range
	// covering the whole window gives a pod-count average.
	windowSecs := int64(end.Sub(start) / time.Second)
	q := fmt.Sprintf(
		`count(avg_over_time(container_memory_working_set_bytes{%s}[%ds]))`,
		matchers, windowSecs)
	v, err := egressquery.Instant(ctx, s.promHTTP, s.PromBase, q, end)
	if err != nil {
		log.Printf("usage: instance_seconds for %s: %v", logName, err)
		return 0, false
	}
	return int64(math.Round(v * float64(windowSecs))), true
}

// queryEgressBytes composes the applicable App sources atomically: exact-router
// HTTP responses, downstream WebSocket frames, and App-pod direct public L3
// bytes. Static sites have no tenant pod, so direct is explicit N/A. A failed
// required source or health signal rejects the whole sum.
func (s *Service) queryEgressBytes(ctx context.Context, app store.App, start, end time.Time) (int64, bool) {
	quantity, _, ok := s.queryEgressBytesEvidence(ctx, app, start, end)
	return quantity, ok
}

func (s *Service) queryEgressBytesEvidence(ctx context.Context, app store.App, start, end time.Time) (int64, []store.UsageSourceObservation, bool) {
	if s.Client == nil {
		return 0, unavailableAppEgressSources(false), false
	}
	// Resolve the App CR by its unique app-id across every workspace: per-tenant
	// namespaces (ADR043) scatter the CRs across `<ws>` namespaces, so this must
	// list cluster-wide, never scoped to a single namespace.
	var projected appv1alpha1.AppList
	if err := s.Client.List(ctx, &projected, client.MatchingLabels{store.LabelAppID: app.ID}); err != nil {
		log.Printf("usage: egress_bytes resolve App CR for %s: %v", app.ID, err)
		return 0, unavailableAppEgressSources(false), false
	}
	if len(projected.Items) != 1 {
		log.Printf("usage: egress_bytes resolve App CR for %s: found %d projected Apps", app.ID, len(projected.Items))
		return 0, unavailableAppEgressSources(false), false
	}
	direct := projected.Items[0].Spec.Type != appv1alpha1.TypeStaticSite
	routers, err := s.TraefikRouterNames(ctx, &projected.Items[0])
	if err != nil {
		log.Printf("usage: egress_bytes resolve routers for %s: %v", app.ID, err)
		return 0, unavailableAppEgressSources(false), false
	}
	specs := egressquery.App(app.ID, routers, direct)
	if len(specs) == 0 {
		// A private/no-router static site has explicit healthy zero egress; it is
		// not missing evidence merely because no public counter applies.
		return 0, []store.UsageSourceObservation{{Source: store.UsageSourceHTTP, State: store.UsageSourceHealthy}}, true
	}
	if s.PromBase == "" {
		observations := make([]store.UsageSourceObservation, 0, len(specs))
		for _, spec := range specs {
			observations = append(observations, store.UsageSourceObservation{Source: string(spec.Source), State: store.UsageSourceUnavailable})
		}
		return 0, observations, false
	}
	return s.queryEgressSourcesEvidence(ctx, app.ID, specs, start, end)
}

func unavailableAppEgressSources(direct bool) []store.UsageSourceObservation {
	out := []store.UsageSourceObservation{{Source: store.UsageSourceHTTP, State: store.UsageSourceUnavailable}}
	if direct {
		out = append(out,
			store.UsageSourceObservation{Source: store.UsageSourceWebSocket, State: store.UsageSourceUnavailable},
			store.UsageSourceObservation{Source: store.UsageSourceDirect, State: store.UsageSourceUnavailable})
	}
	return out
}

func (s *Service) queryDatastoreEgressBytes(ctx context.Context, ds datastoreEntry, start, end time.Time) (int64, bool) {
	quantity, _, ok := s.queryDatastoreEgressBytesEvidence(ctx, ds, start, end)
	return quantity, ok
}

func (s *Service) queryDatastoreEgressBytesEvidence(ctx context.Context, ds datastoreEntry, start, end time.Time) (int64, []store.UsageSourceObservation, bool) {
	source := store.UsageSourcePostgres
	if ds.Kind == store.ResourceKindKeyValue {
		source = store.UsageSourceKeyValue
	}
	if !ds.Public {
		return 0, []store.UsageSourceObservation{{Source: source, State: store.UsageSourceHealthy}}, true
	}
	return s.queryEgressSourcesEvidence(ctx, ds.ID, egressquery.Datastore(ds.ID, ds.Kind, true), start, end)
}

// queryEgressSources sums the hour's egress across sources with PER-SOURCE
// health gating (w1/m51, ADR023 § Observability reads vs billing reads): a
// source failing its health product is SKIPPED for this hour (undercount,
// never invent — its increase() could be reset-inflated), while the healthy
// sources still record. Only a transport failure defers the hour: a past
// hour's samples never improve, so deferring on a health term was equivalent
// to never recording (prod evidence: zero egress_bytes rows for all of July
// 2026 under the old any-source-defers rule, w1/034). All sources skipped
// still records a successful zero — the gap-free cursor advances.
func (s *Service) queryEgressSources(ctx context.Context, resourceID string, specs []egressquery.Spec, start, end time.Time) (int64, bool) {
	quantity, _, ok := s.queryEgressSourcesEvidence(ctx, resourceID, specs, start, end)
	return quantity, ok
}

func (s *Service) queryEgressSourcesEvidence(ctx context.Context, resourceID string, specs []egressquery.Spec, start, end time.Time) (int64, []store.UsageSourceObservation, bool) {
	if len(specs) == 0 {
		return 0, nil, true
	}
	seconds := int64(end.Sub(start) / time.Second)
	type result struct {
		source  egressquery.Source
		value   float64
		skipped bool
		err     error
	}
	results := make(chan result, len(specs))
	for _, spec := range specs {
		go func() {
			health, err := egressquery.Instant(ctx, s.promHTTP, s.PromBase, egressquery.Health(spec, seconds), end)
			if err != nil {
				results <- result{source: spec.Source, err: err}
				return
			}
			if health < 1 {
				results <- result{source: spec.Source, skipped: true}
				return
			}
			value, err := egressquery.Instant(ctx, s.promHTTP, s.PromBase, egressquery.Increase(spec, seconds), end)
			results <- result{source: spec.Source, value: value, err: err}
		}()
	}
	var total float64
	deferred := false
	observations := make([]store.UsageSourceObservation, 0, len(specs))
	for range specs {
		result := <-results
		state := store.UsageSourceHealthy
		switch {
		case result.err != nil:
			log.Printf("usage: egress_bytes source %s for %s: %v (hour deferred)", result.source, resourceID, result.err)
			deferred = true
			state = store.UsageSourceUnavailable
		case result.skipped:
			log.Printf("usage: egress_bytes source %s for %s unhealthy in [%s, %s): skipped this hour", result.source, resourceID, start.Format(time.RFC3339), end.Format(time.RFC3339))
			state = store.UsageSourceDegraded
		default:
			total += result.value
		}
		observations = append(observations, store.UsageSourceObservation{Source: string(result.source), State: state})
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].Source < observations[j].Source })
	if deferred {
		return 0, observations, false
	}
	return int64(math.Round(total)), observations, true
}

// --- t004: build-Job duration metering ---

// labelBuild is the label key on BuildKit Jobs (operator/internal/build/build.go).
const labelBuild = "app.bex.co/build"

// queryBuildSeconds sums the durations of completed build Jobs whose
// completionTime falls in [window, end). It lists Jobs in the service
// namespace by the app-name label. The Kubernetes client comes from core.Base.
func (s *Service) queryBuildSeconds(ctx context.Context, appName string, window, end time.Time) (int64, bool) {
	if s.Client == nil {
		return 0, false
	}
	var jobs batchv1.JobList
	if err := s.Client.List(ctx, &jobs,
		client.InNamespace(s.Namespace),
		client.MatchingLabels{labelBuild: appName},
	); err != nil {
		log.Printf("usage: build_seconds list jobs for %s: %v", appName, err)
		return 0, false
	}
	var total int64
	for _, job := range jobs.Items {
		ct := job.Status.CompletionTime
		if ct == nil {
			continue // still running
		}
		t := ct.UTC()
		if t.Before(window) || !t.Before(end) {
			continue // not in this window
		}
		if job.Status.StartTime == nil {
			continue
		}
		dur := ct.Sub(job.Status.StartTime.Time)
		if dur > 0 {
			total += int64(dur / time.Second)
		}
	}
	return total, true
}
