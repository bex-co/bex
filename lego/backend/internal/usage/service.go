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
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	// Selections is the shared MCP per-session workspace selection
	// (w6/m2/t005, core.WorkspaceSelections): get_usage's ownerId precedence
	// (explicit arg > the session's select_workspace > the caller's default).
	// nil (e.g. no MCP transport wired) degrades to explicit-arg-or-default.
	Selections core.WorkspaceSelectionReader
	promHTTP   *http.Client
}

// NewService constructs a Service, normalising PromBase (strips trailing slash
// so every Prometheus URL is canonical) and resolving a nil HTTP client to
// http.DefaultClient. Callers in tests may pass their own *http.Client to
// target an httptest.Server.
func NewService(base *core.Base, st UsageStore, promBase string, hc *http.Client) *Service {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Service{
		Base:     base,
		Store:    st,
		PromBase: strings.TrimRight(promBase, "/"),
		promHTTP: hc,
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
}

// ServiceUsage is one service's contribution to the workspace's month-to-date
// totals, broken out by kind (and tier for instance_seconds).
type ServiceUsage struct {
	ServiceID    string
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
	sum.Period = now.Format("2006-01")
	sum.EstimatedCost = pricing.Default.Estimate(rows)
	return sum, nil
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
	ID       string // immutable CR name (== service_id in usage rows)
	Name     string // immutable data-plane name used in Prometheus selectors
	TenantID string // from core.LabelTenant
	Plan     string // Spec.Plan (== tier in usage rows)
	Kind     string // store.ResourceKindPostgres or store.ResourceKindKeyValue
}

// listDatastores returns all tenant-owned Database and KeyValue CRs in the
// service namespace via the k8s client. Skips unlabeled CRs (not tenant-owned).
// Returns nil when the k8s client is not wired.
func (s *Service) listDatastores(ctx context.Context) ([]datastoreEntry, error) {
	if s.Client == nil {
		return nil, nil
	}
	var out []datastoreEntry
	var dbs appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &dbs, client.InNamespace(s.Namespace)); err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	for _, d := range dbs.Items {
		tenantID := d.Labels[core.LabelTenant]
		if tenantID == "" {
			continue
		}
		out = append(out, datastoreEntry{
			ID:       d.Name,
			Name:     d.Name,
			TenantID: tenantID,
			Plan:     d.Spec.Plan,
			Kind:     store.ResourceKindPostgres,
		})
	}
	var kvs appv1alpha1.KeyValueList
	if err := s.Client.List(ctx, &kvs, client.InNamespace(s.Namespace)); err != nil {
		return nil, fmt.Errorf("list keyvalues: %w", err)
	}
	for _, kv := range kvs.Items {
		tenantID := kv.Labels[core.LabelTenant]
		if tenantID == "" {
			continue
		}
		out = append(out, datastoreEntry{
			ID:       kv.Name,
			Name:     kv.Name,
			TenantID: tenantID,
			Plan:     kv.Spec.Plan,
			Kind:     store.ResourceKindKeyValue,
		})
	}
	return out, nil
}

// processDatastoreWindow measures compute and used-storage time for one
// (datastore, window) pair and upserts both rows. The independent meter keys
// make reprocessing idempotent. egress_bytes is N/A (Traefik TCP/SNI doesn't
// emit the HTTP frontend metric) and build_seconds is N/A (no build Jobs).
func (s *Service) processDatastoreWindow(ctx context.Context, ds datastoreEntry, window time.Time) bool {
	end := window.Add(time.Hour)
	var (
		instanceSecs int64
		storageSecs  int64
		storageOK    bool
		wg           sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		instanceSecs = s.queryInstanceSecondsStateful(ctx, ds.Name, s.Namespace, window, end)
	}()
	go func() {
		defer wg.Done()
		storageSecs, storageOK = s.queryStorageGBSeconds(ctx, ds, window, end)
	}()
	wg.Wait()

	if instanceSecs > 0 || ds.Plan != "" {
		if err := s.Store.UpsertUsageHourly(ctx, store.HourlyRow{
			WorkspaceID:  ds.TenantID,
			ServiceID:    ds.ID,
			ResourceKind: ds.Kind,
			Kind:         store.UsageKindInstanceSeconds,
			Tier:         ds.Plan,
			WindowStart:  window,
			Quantity:     instanceSecs,
		}); err != nil {
			log.Printf("usage: upsert instance_seconds datastore %s: %v", ds.ID, err)
		}
	}
	// A successful empty query is a real zero and is persisted. A failed query
	// is not written, so the per-meter catch-up cursor can retry it later.
	if storageOK {
		if err := s.Store.UpsertUsageHourly(ctx, store.HourlyRow{
			WorkspaceID:  ds.TenantID,
			ServiceID:    ds.ID,
			ResourceKind: ds.Kind,
			Kind:         store.UsageKindStorageGBSeconds,
			Tier:         "",
			WindowStart:  window,
			Quantity:     storageSecs,
		}); err != nil {
			log.Printf("usage: upsert storage_gb_seconds datastore %s: %v", ds.ID, err)
			return false
		}
	}
	return storageOK
}

// catchUpDatastoreThrough fills every missing storage window through last.
// It stops at the first failed storage query, preserving a contiguous cursor;
// the next collector run retries that window before advancing, so a transient
// Prometheus failure cannot leave a permanent hole hidden by newer rows.
func (s *Service) catchUpDatastoreThrough(ctx context.Context, ds datastoreEntry, last time.Time) {
	latest, err := s.Store.LatestUsageWindow(ctx, ds.Kind, ds.ID, store.UsageKindStorageGBSeconds)
	if err != nil {
		log.Printf("usage: catch-up: latest storage window for datastore %s: %v", ds.ID, err)
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
		if !s.processDatastoreWindow(ctx, ds, w) {
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
	}
	for _, ds := range datastores {
		s.catchUpDatastoreThrough(ctx, ds, now.Truncate(time.Hour).Add(-time.Hour))
	}
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
	}
	for _, ds := range datastores {
		s.catchUpDatastoreThrough(ctx, ds, window)
	}
	log.Printf("usage: rolled up window %s for %d services + %d datastores",
		window.Format(time.RFC3339), len(apps), len(datastores))
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
	if start.IsZero() || start.Before(floor) {
		start = floor
	} else {
		start = start.Add(time.Hour)
	}
	for window := start; !window.After(last); window = window.Add(time.Hour) {
		if !s.processAppMeterWindow(ctx, app, kind, window) {
			return
		}
	}
}

// processAppMeterWindow persists a successful measurement even when it is
// zero. Zero is real coverage evidence; source unavailability is represented
// by no row so the per-meter cursor retries the window later.
func (s *Service) processAppMeterWindow(ctx context.Context, app store.App, kind string, window time.Time) bool {
	end := window.Add(time.Hour)
	var quantity int64
	var ok bool
	switch kind {
	case store.UsageKindInstanceSeconds:
		quantity, ok = s.queryInstanceSeconds(ctx, app.Name, s.Namespace, window, end)
	case store.UsageKindEgressBytes:
		quantity, ok = s.queryEgressBytes(ctx, app, window, end)
	case store.UsageKindBuildSeconds:
		quantity, ok = s.queryBuildSeconds(ctx, app.Name, window, end)
	default:
		log.Printf("usage: unknown App meter %q for %s", kind, app.ID)
		return false
	}
	if !ok {
		return false
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
	}); err != nil {
		log.Printf("usage: upsert %s %s: %v", kind, app.ID, err)
		return false
	}
	return true
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
		namespace, promEscape(appName))
	return s.queryInstanceSecondsByMatcher(ctx, matchers, start, end, appName)
}

// queryInstanceSecondsStateful returns instance-seconds for a StatefulSet-style
// workload (CNPG Cluster pods <dbName>-1, <dbName>-2 or Valkey StatefulSet pods
// <kvName>-0). The pod pattern is <name>-[0-9]+ (ordinal suffix), distinct from
// App ReplicaSet pods which use two random-char segments.
func (s *Service) queryInstanceSecondsStateful(ctx context.Context, name, namespace string, start, end time.Time) int64 {
	matchers := fmt.Sprintf(
		`namespace=%q,pod=~"%s-[0-9]+",container!=""`,
		namespace, promEscape(name))
	value, _ := s.queryInstanceSecondsByMatcher(ctx, matchers, start, end, name)
	return value
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
	pattern := fmt.Sprintf(`^%s-[0-9]+$`, promEscape(ds.Name))
	if ds.Kind == store.ResourceKindKeyValue {
		pattern = fmt.Sprintf(`^data-%s-[0-9]+$`, promEscape(ds.Name))
	}
	windowSecs := int64(end.Sub(start) / time.Second)
	q := fmt.Sprintf(
		`sum(avg_over_time(kubelet_volume_stats_used_bytes{namespace=%q,persistentvolumeclaim=~%q}[%ds]))`,
		s.Namespace, pattern, windowSecs)
	usedBytes, err := promInstantScalar(ctx, s.promHTTP, s.PromBase, q, end)
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
	v, err := promInstantScalar(ctx, s.promHTTP, s.PromBase, q, end)
	if err != nil {
		log.Printf("usage: instance_seconds for %s: %v", logName, err)
		return 0, false
	}
	return int64(math.Round(v * float64(windowSecs))), true
}

// queryEgressBytes returns HTTP response bytes for the exact App Ingress
// routers in [start, end) via Traefik's increase() instant query at the window
// end, mirroring the monthToDateBandwidth source in internal/metrics but
// window-bounded and returning bytes (not MB). Other outbound classes are
// composed by w8/m14 after their independently attributable sources land.
func (s *Service) queryEgressBytes(ctx context.Context, app store.App, start, end time.Time) (int64, bool) {
	if s.PromBase == "" || s.Client == nil {
		return 0, false
	}
	var projected appv1alpha1.AppList
	if err := s.Client.List(ctx, &projected,
		client.InNamespace(s.Namespace),
		client.MatchingLabels{store.LabelAppID: app.ID},
	); err != nil {
		log.Printf("usage: egress_bytes resolve App CR for %s: %v", app.ID, err)
		return 0, false
	}
	if len(projected.Items) != 1 {
		log.Printf("usage: egress_bytes resolve App CR for %s: found %d projected Apps", app.ID, len(projected.Items))
		return 0, false
	}
	routers, err := s.TraefikRouterNames(ctx, &projected.Items[0])
	if err != nil {
		log.Printf("usage: egress_bytes resolve routers for %s: %v", app.ID, err)
		return 0, false
	}
	matcher := core.TraefikRouterMatcher(routers)
	if matcher == "" {
		return 0, true
	}
	elapsed := end.Sub(start)
	q := fmt.Sprintf(
		`sum(increase(traefik_router_responses_bytes_total{router=~%q}[%ds]))`,
		matcher, int64(elapsed/time.Second))
	v, err := promInstantScalar(ctx, s.promHTTP, s.PromBase, q, end)
	if err != nil {
		log.Printf("usage: egress_bytes for %s: %v", app.ID, err)
		return 0, false
	}
	return int64(math.Round(v)), true
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

// --- Prometheus instant query helper (shared by t002 queries) ---

// promInstantScalar evaluates a PromQL expression as an instant query at the
// given time and returns the scalar sum of all returned vector elements.
// Returns (0, nil) for empty results (no running containers = 0 instance-seconds).
func promInstantScalar(ctx context.Context, hc *http.Client, base, query string, at time.Time) (float64, error) {
	if hc == nil {
		hc = http.DefaultClient
	}
	u := fmt.Sprintf("%s/api/v1/query?%s", base, url.Values{
		"query": {query},
		"time":  {strconv.FormatInt(at.Unix(), 10)},
	}.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus: status %d", resp.StatusCode)
	}
	var pr promInstantResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return 0, fmt.Errorf("decode prometheus: %w", err)
	}
	if pr.Status != "" && pr.Status != "success" {
		return 0, fmt.Errorf("prometheus status %q", pr.Status)
	}
	var total float64
	for _, res := range pr.Data.Result {
		if len(res.Value) != 2 {
			continue
		}
		s, ok := res.Value[1].(string)
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) {
			continue
		}
		total += v
	}
	return total, nil
}

// promInstantResponse is the Prometheus /api/v1/query result subset we parse.
type promInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []any `json:"value"` // [unixSeconds(float), "value"(string)]
		} `json:"result"`
	} `json:"data"`
}

// promEscape escapes PromQL regex metacharacters in an app name — identical to
// the helper in internal/metrics/source.go; duplicated rather than exported
// from that package to keep feature packages independent.
func promEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`.+*?()|[]{}^$\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
