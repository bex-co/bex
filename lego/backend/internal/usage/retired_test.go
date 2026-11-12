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

package usage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

var retiredHour = time.Date(2026, 10, 5, 7, 0, 0, 0, time.UTC)

// retiredProm is Prometheus with each short-lived resource's pods only in
// retiredHour, as 15s samples (22 → 330s, 33 → 495s), so a row can carry them
// only if the meter asked by that resource's own pods, for that hour. Public
// datastore egress answers 7 only if asked for, so its row proves the meter
// knew the store was public.
func retiredProm(t *testing.T, appPods string) *httptest.Server {
	t.Helper()
	ranAt := strconv.FormatInt(retiredHour.Add(time.Hour).Unix(), 10)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		ran := r.URL.Query().Get("time") == ranAt
		v := "0"
		switch {
		case strings.Contains(q, "healthy") || strings.Contains(q, `up{job=`):
			v = "1"
		case ran && strings.Contains(q, "container_memory_working_set_bytes") && strings.Contains(q, appPods):
			v = "22"
		case ran && strings.Contains(q, "container_memory_working_set_bytes") && strings.Contains(q, `namespace="tea-short-ns"`):
			v = "33"
		case ran && strings.Contains(q, "red-short") && !strings.Contains(q, "container_memory_working_set_bytes"):
			v = "7"
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"` + v + `"]}]}}`))
	}))
}

// w4/m173 t007, the live replay's shape: a web service and a public Free Key
// Value each lived a few minutes inside 07:00–08:00 and were deleted before
// the 08:00 rollup, which used to write no row for either. Each is metered
// through the hour it was deleted in, by its own identity, and no later; the
// store keeps its name and its coverage ends with it.
func TestRollupMetersResourcesDeletedBeforeTheirHourClosed(t *testing.T) {
	app := store.RetiredUsageResource{ResourceKind: store.ResourceKindService, ResourceID: "srv-short", WorkspaceID: "tea-short",
		Name: "whoami", Tier: "starter", CreatedAt: retiredHour.Add(21*time.Minute + 41*time.Second), RetiredAt: retiredHour.Add(27*time.Minute + 11*time.Second)}
	kv := store.RetiredUsageResource{ResourceKind: store.ResourceKindKeyValue, ResourceID: "red-short", WorkspaceID: "tea-short",
		Name: "red-short", Namespace: "tea-short-ns", Display: "cache", Public: true, Tier: "free",
		CreatedAt: retiredHour.Add(21*time.Minute + 27*time.Second), RetiredAt: retiredHour.Add(29*time.Minute + 54*time.Second)}
	st := newMemUsageStore()
	st.retired = []store.RetiredUsageResource{app, kv}
	prom := retiredProm(t, `pod=~"`+core.CRName(app.WorkspaceID, app.Name)+`-`)
	defer prom.Close()
	svc := &Service{Base: &core.Base{Client: appMeterClient(t, store.App{ID: "srv-live", TenantID: "tea-live", Name: "live"}), Namespace: "default"},
		Store: st, PromBase: prom.URL}

	svc.rollup(context.Background(), retiredHour.Add(time.Hour+5*time.Second))
	svc.rollup(context.Background(), retiredHour.Add(2*time.Hour+5*time.Second))

	st.mu.Lock()
	defer st.mu.Unlock()
	got := map[string]int64{}
	for _, row := range st.rows {
		id := row.ResourceKind + "/" + row.ServiceID + "/" + row.Kind
		if row.ServiceID != app.ResourceID && row.ServiceID != kv.ResourceID {
			continue
		}
		if !row.WindowStart.Equal(retiredHour) {
			t.Errorf("%s metered for %v; a resource created and deleted in %v owes only that hour", id, row.WindowStart, retiredHour)
			continue
		}
		got[id] = row.Quantity
		if row.Kind == store.UsageKindInstanceSeconds && row.Tier != map[string]string{app.ResourceID: "starter", kv.ResourceID: "free"}[row.ServiceID] {
			t.Errorf("%s tier = %q, want its retired tier", id, row.Tier)
		}
	}
	for id, want := range map[string]int64{
		"service/srv-short/instance_seconds":   330,
		"key_value/red-short/instance_seconds": 495,
		"key_value/red-short/egress_bytes":     7,
	} {
		if got[id] != want {
			t.Errorf("%s = %d (present %v), want %d: a short-lived resource was a silent zero", id, got[id], got[id] != 0, want)
		}
	}
	if _, metered := got["service/srv-short/egress_bytes"]; metered {
		t.Error("a deleted App's egress was metered without its live router names")
	}
	if st.retained["tea-short"][store.ResourceDisplayNameKey(store.ResourceKindKeyValue, "red-short")] != "cache" {
		t.Errorf("retained names = %v; the deleted store's charge line has no name", st.retained)
	}
	for _, ref := range []store.UsageResourceRef{{ResourceKind: app.ResourceKind, ServiceID: app.ResourceID}, {ResourceKind: kv.ResourceKind, ServiceID: kv.ResourceID}} {
		if through := st.closed[ref]; !through.Equal(retiredHour.Add(time.Hour)) {
			t.Errorf("%v coverage closed at %v, want its deletion hour's end %v", ref, through, retiredHour.Add(time.Hour))
		}
	}
}

// An App deleted and recreated under the same name in one hour shares its
// pod identity with the new one, whose meter already bills those pods.
func TestRetiredAppSharingALiveAppsPodsIsNotMeteredTwice(t *testing.T) {
	gone := store.RetiredUsageResource{ResourceKind: store.ResourceKindService, ResourceID: "srv-old", WorkspaceID: "tea-short",
		Name: "whoami", CreatedAt: retiredHour.Add(-2 * time.Hour), RetiredAt: retiredHour.Add(10 * time.Minute)}
	reborn := store.App{ID: "srv-new", TenantID: "tea-short", Name: "whoami", CreatedAt: retiredHour.Add(12 * time.Minute)}
	st := newMemUsageStore(reborn)
	st.retired = []store.RetiredUsageResource{gone}
	prom := retiredProm(t, `pod=~"`+core.CRName(gone.WorkspaceID, gone.Name)+`-`)
	defer prom.Close()
	svc := &Service{Base: &core.Base{Client: appMeterClient(t, reborn), Namespace: "default"}, Store: st, PromBase: prom.URL}

	svc.rollup(context.Background(), retiredHour.Add(time.Hour+5*time.Second))

	st.mu.Lock()
	defer st.mu.Unlock()
	for _, row := range st.rows {
		if row.ServiceID == "srv-old" && row.WindowStart.Equal(retiredHour) {
			t.Fatalf("the deleted App was metered for the hour its name's new App already bills: %+v", row)
		}
	}
	// Its coverage ends with its own last reading, not at its deletion hour,
	// whose hours no meter will write for it (w6/m334).
	ref := store.UsageResourceRef{ResourceKind: store.ResourceKindService, ServiceID: "srv-old"}
	if through, closed := st.closed[ref]; closed {
		t.Errorf("taken-over App's coverage closed at %v, which leaves its skipped hours expected", through)
	}
	if !slices.Contains(st.closedAtLast, ref) {
		t.Errorf("taken-over App's coverage closes = %v, want it closed at its last reading", st.closedAtLast)
	}
	if podIdentityReused(gone, retiredHour, nil, []store.RetiredUsageResource{gone}) {
		t.Fatal("a retired App reuses only its own identity")
	}
}

// Records retired before the catch-up window are not metered, and compaction
// — which runs with or without Prometheus — prunes them.
func TestRetiredRecordsPastTheCatchUpWindow(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 5, 0, time.UTC)
	old := store.RetiredUsageResource{ResourceKind: store.ResourceKindService, ResourceID: "srv-old", WorkspaceID: "tea-old",
		Name: "old", CreatedAt: now.Add(-80 * time.Hour), RetiredAt: now.Add(-72 * time.Hour)}
	st := newMemUsageStore()
	st.retired = []store.RetiredUsageResource{old}
	prom := fakeProm(60)
	defer prom.Close()
	svc := &Service{Base: &core.Base{Namespace: "default", Clock: clockAt(now)}, Store: st, PromBase: prom.URL}

	svc.rollup(context.Background(), now)
	st.mu.Lock()
	for _, row := range st.rows {
		if row.ServiceID == "srv-old" {
			t.Errorf("metered a resource retired before the catch-up window: %+v", row)
		}
	}
	st.mu.Unlock()

	svc.PromBase = ""
	svc.compact(context.Background())
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.retired) != 0 {
		t.Fatalf("retired = %+v after compaction, want the 72h-old record pruned", st.retired)
	}
}

// w2/m254 t015: a deleted build source still owes its last builds, and only
// those — a carrier never runs instances, so it records no instance usage.
func TestRetiredBuildSourceMetersBuildsOnly(t *testing.T) {
	carrier := store.RetiredUsageResource{ResourceKind: store.ResourceKindService, ResourceID: "bds-c185th5c2rvvnhbfiltg", WorkspaceID: "tea-short",
		Name: "bds-c185th5c2rvvnhbfiltg", Tier: "starter", CreatedAt: retiredHour.Add(5 * time.Minute), RetiredAt: retiredHour.Add(40 * time.Minute)}
	st := newMemUsageStore()
	st.retired = []store.RetiredUsageResource{carrier}
	prom := retiredProm(t, `pod=~"`+core.CRName(carrier.WorkspaceID, carrier.Name)+`-`)
	defer prom.Close()
	svc := &Service{Base: &core.Base{Client: appMeterClient(t, store.App{ID: "srv-live", TenantID: "tea-live", Name: "live"}), Namespace: "default"},
		Store: st, PromBase: prom.URL}

	svc.rollup(context.Background(), retiredHour.Add(time.Hour+5*time.Second))

	st.mu.Lock()
	defer st.mu.Unlock()
	sawBuild := false
	for _, row := range st.rows {
		if row.ServiceID != carrier.ResourceID {
			continue
		}
		if row.Kind != store.UsageKindBuildSeconds {
			t.Errorf("a deleted build source recorded %s usage; it runs builds only", row.Kind)
		}
		sawBuild = true
	}
	if !sawBuild {
		t.Error("a deleted build source's last hour of builds was not metered")
	}
}

// w6/m333: a web service deleted inside an hour owes that hour's bandwidth,
// as on Render. Its CR and Ingresses are gone, so the meter reads it by the
// router names and direct flag its live egress meter recorded, with the same
// per-source health gating as a live service, and still ends its coverage
// with the deletion hour.
func TestRetiredAppMetersItsDeletionHourEgressFromItsRecordedIdentity(t *testing.T) {
	const router = "default-tea-short-whoami-whoami-onbex-co@kubernetes"
	app := store.RetiredUsageResource{ResourceKind: store.ResourceKindService, ResourceID: "srv-short", WorkspaceID: "tea-short",
		// It lived across an earlier rollup, which is when its identity was recorded.
		Name: "whoami", Tier: "starter", CreatedAt: retiredHour.Add(-2*time.Hour + 21*time.Minute), RetiredAt: retiredHour.Add(27 * time.Minute),
		Egress: &store.EgressIdentity{Routers: []string{router}, Direct: true}}
	st := newMemUsageStore()
	st.retired = []store.RetiredUsageResource{app}
	ranAt := strconv.FormatInt(retiredHour.Add(time.Hour).Unix(), 10)
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		ran := r.URL.Query().Get("time") == ranAt
		v := "0"
		switch {
		case strings.Contains(q, "healthy") || strings.Contains(q, `up{job=`):
			v = "1"
		case ran && strings.Contains(q, "traefik_router_responses_bytes_total") && strings.Contains(q, router):
			v = "40"
		case ran && strings.Contains(q, "bex_app_direct_egress_bytes_total") && strings.Contains(q, `app_id="srv-short"`):
			v = "2"
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"` + v + `"]}]}}`))
	}))
	defer prom.Close()
	svc := &Service{Base: &core.Base{Client: appMeterClient(t, store.App{ID: "srv-live", TenantID: "tea-live", Name: "live"}), Namespace: "default"},
		Store: st, PromBase: prom.URL}

	svc.rollup(context.Background(), retiredHour.Add(time.Hour+5*time.Second))
	svc.rollup(context.Background(), retiredHour.Add(2*time.Hour+5*time.Second))

	st.mu.Lock()
	defer st.mu.Unlock()
	var deletionHour *store.HourlyRow
	for _, row := range st.rows {
		if row.ServiceID != app.ResourceID || row.Kind != store.UsageKindEgressBytes {
			continue
		}
		if row.WindowStart.After(retiredHour) {
			t.Errorf("egress metered for %v, after the deletion hour %v", row.WindowStart, retiredHour)
		}
		if row.WindowStart.Equal(retiredHour) {
			deletionHour = &row
		}
	}
	if deletionHour == nil || deletionHour.Quantity != 42 {
		t.Fatalf("deletion-hour egress row = %+v, want 42 bytes (40 HTTP + 2 direct)", deletionHour)
	}
	for _, health := range deletionHour.SourceHealth {
		if health.State != store.UsageSourceHealthy {
			t.Errorf("egress source %s = %s, want healthy", health.Source, health.State)
		}
	}
	ref := store.UsageResourceRef{ResourceKind: app.ResourceKind, ServiceID: app.ResourceID}
	if through := st.closed[ref]; !through.Equal(retiredHour.Add(time.Hour)) {
		t.Errorf("coverage closed at %v, want the deletion hour's end %v", through, retiredHour.Add(time.Hour))
	}
}

// The identity a retired App's egress is read by comes from its live meter:
// each catch-up pass resolves it once, uses it for every window, and records
// it for the App, so the retirement has it when the App is deleted (w6/m333).
func TestLiveEgressMeterRecordsTheAppsEgressIdentityOncePerPass(t *testing.T) {
	prom := fakeProm(1)
	defer prom.Close()
	app := store.App{ID: "srv-web", TenantID: "tea-one", Name: "web", CreatedAt: retiredHour.Add(-2 * time.Hour)}
	st := newMemUsageStore()
	svc := &Service{Base: &core.Base{Client: publicAppMeterClient(t, app, "tea-one-web", "tea-one-web", "web.onbex.co"), Namespace: "default"},
		Store: st, PromBase: prom.URL}

	svc.catchUpAppMeter(context.Background(), app, store.UsageKindEgressBytes, retiredHour.Add(-2*time.Hour), retiredHour)

	st.mu.Lock()
	defer st.mu.Unlock()
	if windows := len(st.rows); windows != 3 {
		t.Fatalf("metered %d egress windows, want the pass's 3", windows)
	}
	if st.egressIdentityRecords != 1 {
		t.Errorf("identity recorded %d times in one 3-window pass, want once", st.egressIdentityRecords)
	}
	got := st.egressIdentities[app.ID]
	// The bare router and its web-entrypoint name (traefikWebEntrypoint).
	want := []string{"default-tea-one-web-web-onbex-co@kubernetes", "web-default-tea-one-web-web-onbex-co@kubernetes"}
	if !got.Direct || !slices.Equal(got.Routers, want) {
		t.Fatalf("recorded identity = %+v, want routers %v with direct egress", got, want)
	}
}

// Recording the identity is for a later deletion only: a failed write must not
// cost the live App this hour's egress (w6/m333).
func TestLiveEgressIsMeteredWhenRecordingItsIdentityFails(t *testing.T) {
	prom := fakeProm(40)
	defer prom.Close()
	app := store.App{ID: "srv-web", TenantID: "tea-one", Name: "web", CreatedAt: retiredHour}
	st := newMemUsageStore()
	st.egressIdentityErr = errors.New("store unavailable")
	svc := &Service{Base: &core.Base{Client: publicAppMeterClient(t, app, "tea-one-web", "tea-one-web", "web.onbex.co"), Namespace: "default"},
		Store: st, PromBase: prom.URL}

	svc.catchUpAppMeter(context.Background(), app, store.UsageKindEgressBytes, retiredHour, retiredHour)

	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.rows) != 1 {
		t.Fatalf("egress rows with a failing identity record = %d, want the hour metered", len(st.rows))
	}
	for _, row := range st.rows {
		if row.Quantity < 40 {
			t.Fatalf("egress row = %+v, want the hour's bytes", row)
		}
	}
}
