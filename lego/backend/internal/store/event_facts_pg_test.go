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

package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// A post-Running crash and its recovery, observed by the reconciler across
// many resync ticks, must land as exactly one server_failed + one
// server_available fact pair (ADR052 gap-register item 2 / w3/m78): the
// service_event_checkpoints diff — not any producer-side care — is what keeps
// a level-triggered 30s poll from re-emitting the same edge.
func TestPGObservedCrashEdgeEmitsExactlyOnePair(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "crash-edge-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "web-" + stamp, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	appCR := func(phase appv1alpha1.AppPhase, ready metav1.ConditionStatus, reason string) *appv1alpha1.App {
		cr := &appv1alpha1.App{}
		cr.Status.Phase = phase
		cr.Status.ActiveRevision = "rev-1"
		cr.Status.Conditions = []metav1.Condition{{Type: "Ready", Status: ready, Reason: reason}}
		return cr
	}
	healthy := appCR(appv1alpha1.PhaseRunning, metav1.ConditionTrue, "Deployed")
	// A crash after Running routes the CR back through PhaseDeploying with a
	// CrashLoopBackOff Ready reason (app_controller's stuckPodMessage overlay);
	// there is no dedicated operator "was Ready, now failed" phase.
	crashed := appCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "CrashLoopBackOff")

	var emitted []ServiceEventFact
	observe := func(cr *appv1alpha1.App, hasOpenDeploy bool, ticks int) {
		t.Helper()
		for i := 0; i < ticks; i++ {
			facts, err := st.RecordObservedServiceState(ctx, observedServiceStateFor(app.ID, cr, hasOpenDeploy))
			if err != nil {
				t.Fatalf("record observed state: %v", err)
			}
			emitted = append(emitted, facts...)
		}
	}

	observe(healthy, false, 3) // baseline + steady healthy replays
	observe(crashed, false, 3) // the outage, re-observed across resyncs
	observe(healthy, false, 3) // the recovery, re-observed across resyncs

	var failed, available []ServiceEventFact
	for _, fact := range emitted {
		switch fact.Type {
		case EventFactServerFailed:
			failed = append(failed, fact)
		case EventFactServerAvailable:
			available = append(available, fact)
		default:
			t.Fatalf("unexpected fact %+v", fact)
		}
	}
	if len(failed) != 1 || len(available) != 1 {
		t.Fatalf("emitted %d server_failed + %d server_available, want exactly 1 + 1 (all: %+v)", len(failed), len(available), emitted)
	}
	if failed[0].ReasonCode != EventReasonReadinessFailed {
		t.Fatalf("server_failed reason = %q, want %q", failed[0].ReasonCode, EventReasonReadinessFailed)
	}

	// Both edges must be visible on the composed feed the outbound webhook
	// worker and PushWorker tail (tenant-filtered, same rows).
	rows, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), []string{}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	feed := map[string]int{}
	for _, row := range rows {
		if row.Source == EventSourceFact {
			feed[row.FactType]++
		}
	}
	if feed[string(EventFactServerFailed)] != 1 || feed[string(EventFactServerAvailable)] != 1 {
		t.Fatalf("feed fact counts = %v, want one server_failed and one server_available", feed)
	}

	// A rollout in progress is not an outage: Ready=False with an ordinary
	// progress reason under an open deploy must not emit server_failed.
	progressing := appCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "RolloutProgressing")
	facts, err := st.RecordObservedServiceState(ctx, observedServiceStateFor(app.ID, progressing, true))
	if err != nil {
		t.Fatalf("record progressing state: %v", err)
	}
	if len(facts) != 0 {
		t.Fatalf("open-deploy rollout progress emitted %+v, want none", facts)
	}
}

// TestPGAvailabilityEdgesCarryTheReadyTransitionTime is w4/196: an outage
// users saw as ~45 s read as 13 s because both edges were stamped with the
// pass that noticed them. Each edge now carries the operator's Ready
// transition — unless that would predate the state it leaves.
func TestPGAvailabilityEdgesCarryTheReadyTransitionTime(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "edge-time-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web-" + stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	base := time.Date(2026, 10, 4, 2, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	record := func(obsAt time.Time, availability string, transition time.Time) []ServiceEventFact {
		t.Helper()
		obs := ObservedServiceState{AppID: app.ID, At: obsAt, ServicePhase: string(appv1alpha1.PhaseRunning),
			Availability: availability, AvailabilityObserved: true, ReadyTransitionAt: transition}
		if availability == "unhealthy" {
			obs.ReasonCode = EventReasonReadinessFailed
		}
		facts, err := st.RecordObservedServiceState(ctx, obs)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return facts
	}

	record(at(10), "healthy", at(5)) // baseline
	// Ready flipped False at :68; the debounced edge is emitted at :106.
	failed := record(at(106), "unhealthy", at(68))
	if len(failed) != 1 || !failed[0].At.Equal(at(68)) {
		t.Fatalf("server_failed = %+v, want one edge stamped at the :68 Ready transition", failed)
	}
	// Ready flipped True at :113; noticed at :119.
	available := record(at(119), "healthy", at(113))
	if len(available) != 1 || !available[0].At.Equal(at(113)) {
		t.Fatalf("server_available = %+v, want one edge stamped at the :113 Ready transition", available)
	}

	// A transition older than the state it leaves (Ready's time moves on
	// status flips only) never backdates an edge, nor does a future one.
	if f := record(at(200), "unhealthy", at(100)); len(f) != 1 || !f[0].At.Equal(at(200)) {
		t.Fatalf("stale transition: %+v, want the observation time", f)
	}
	if f := record(at(300), "healthy", at(400)); len(f) != 1 || !f[0].At.Equal(at(300)) {
		t.Fatalf("future transition: %+v, want the observation time", f)
	}
}

// w5/m113 (w4/200): a readiness loss flips Ready=False and the phase back to
// Deploying in one status write, and the reconciler's first unhealthy pass is
// debounced — availability unseen, phase change recorded. That write must not
// become the floor the failure edge is ordered after, or server_failed lands a
// resync late. Replayed through the reconciler's own guards.
func TestPGFailureEdgeOutlivesTheDebouncedPhaseWrite(t *testing.T) {
	st, _, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: fmt.Sprintf("web-%d", time.Now().UnixNano()), Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	base := time.Date(2026, 10, 5, 6, 56, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	r := &Reconciler{Store: st}
	pass := func(sec int, cr *appv1alpha1.App, hasOpenDeploy bool) []ServiceEventFact {
		t.Helper()
		obs := observedServiceStateFor(app.ID, cr, hasOpenDeploy)
		obs.At = at(sec)
		facts, err := st.RecordObservedServiceState(ctx, r.guardServiceObservation(ctx, obs))
		if err != nil {
			t.Fatalf("record :%d: %v", sec, err)
		}
		return facts
	}

	pass(5, healthyCR(at(0)), false)
	pass(35, healthyCR(at(0)), false)
	// Readiness lost at :62, as reportRolloutProgress writes it.
	lost := readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "RolloutProgressing", at(62))
	if f := pass(65, lost, false); len(f) != 0 {
		t.Fatalf("debounced pass emitted %+v", f)
	}
	if f := pass(95, lost, false); len(f) != 1 || f[0].Type != EventFactServerFailed || !f[0].At.Equal(at(62)) {
		t.Fatalf("server_failed = %+v, want one edge at the :62 Ready transition", f)
	}
	if f := pass(125, healthyCR(at(120)), false); len(f) != 1 || f[0].Type != EventFactServerAvailable || !f[0].At.Equal(at(120)) {
		t.Fatalf("server_available = %+v, want one edge at the :120 Ready transition", f)
	}

	// A write no guard suppressed still moves the floor. A deploy's rollout
	// flips Ready=False at :198 and stays undecided; when its pods crash at
	// :250, Ready's transition time is still the rollout start, which must not
	// become the outage's start.
	pass(200, readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "RolloutProgressing", at(198)), true)
	pass(260, crashedCR(at(198)), true)
	if f := pass(290, crashedCR(at(198)), true); len(f) != 1 || f[0].Type != EventFactServerFailed || !f[0].At.Equal(at(290)) {
		t.Fatalf("server_failed = %+v, want one edge at the :290 observation, not the :198 rollout start", f)
	}

	contradictory := ObservedServiceState{AppID: app.ID, At: at(300), AvailabilityObserved: true, AvailabilitySuppressed: true, Availability: "healthy"}
	if _, err := st.RecordObservedServiceState(ctx, contradictory); err == nil {
		t.Fatal("a pass both observed and suppressed was recorded")
	}
}

// w5/083: a crash under an open deploy is dated when the serving revision
// stopped, never before the rollout began. The first pass to see the rollout
// is already the debounced crash, so no earlier write floors the edge, and
// Ready's transition (the rollout start, :198) is the other clock on the CR.
func TestPGOpenDeployCrashIsDatedWhenServingStopped(t *testing.T) {
	st, _, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 6, 6, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	// crashed is the CR a rollout that began at :198 shows once its pods crash,
	// with Serving as the operator's observeServingRevision last wrote it.
	crashed := func(serving *metav1.Condition) *appv1alpha1.App {
		cr := crashedCR(at(198))
		cr.Generation = 2
		if serving != nil {
			cr.Status.Conditions = append(cr.Status.Conditions, *serving)
		}
		return cr
	}
	servingStopped := func(sec int, generation int64) *metav1.Condition {
		return &metav1.Condition{Type: appv1alpha1.ConditionServing, Status: metav1.ConditionFalse,
			Reason: appv1alpha1.ReasonServingRevisionUnavailable, LastTransitionTime: metav1.NewTime(at(sec)), ObservedGeneration: generation}
	}
	for _, tc := range []struct {
		name    string
		serving *metav1.Condition
		want    time.Time
	}{
		// A RollingUpdate keeps the old pods serving until they die mid-rollout.
		{"rolling update", servingStopped(250, 2), at(250)},
		// A Recreate rollout (a disk-backed service) stops the old pod first.
		{"recreate", servingStopped(198, 2), at(198)},
		// No recorded stop: Ready's time, so the stale guard keeps a clock.
		{"no serving stop", nil, at(198)},
		// A stop observed for an earlier generation says nothing about this one.
		{"serving from an older generation", servingStopped(250, 1), at(198)},
		// A stop carried over from before the rollout is older than the healthy
		// checkpoint at :170: alone it would date the crash before the rollout
		// and make the stale guard refuse it.
		{"stop carried over from before the rollout", servingStopped(150, 2), at(198)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: fmt.Sprintf("web-%d", time.Now().UnixNano()), Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
			if err != nil {
				t.Fatalf("create app: %v", err)
			}
			r := &Reconciler{Store: st}
			pass := func(sec int, cr *appv1alpha1.App, hasOpenDeploy bool) []ServiceEventFact {
				t.Helper()
				obs := observedServiceStateFor(app.ID, cr, hasOpenDeploy)
				obs.At = at(sec)
				facts, err := st.RecordObservedServiceState(ctx, r.guardServiceObservation(ctx, obs))
				if err != nil {
					t.Fatalf("record :%d: %v", sec, err)
				}
				return facts
			}
			pass(175, healthyCR(at(170)), false)
			pass(180, healthyCR(at(170)), false)
			if f := pass(260, crashed(tc.serving), true); len(f) != 0 {
				t.Fatalf("debounced pass emitted %+v", f)
			}
			if f := pass(290, crashed(tc.serving), true); len(f) != 1 || f[0].Type != EventFactServerFailed || !f[0].At.Equal(tc.want) {
				t.Fatalf("server_failed = %+v, want one edge at %s", f, tc.want.Format("15:04:05"))
			}
		})
	}
}

// The datastore twin of TestPGAvailabilityEdgesCarryTheReadyTransitionTime.
func TestPGDatastoreAvailabilityEdgesCarryTheReadyTransitionTime(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "ds-edge-time-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })

	base := time.Date(2026, 10, 4, 2, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	record := func(obsAt time.Time, availability string, transition time.Time) []DatastoreEventFact {
		t.Helper()
		obs := ObservedDatastoreState{DatastoreID: "dpg-edge" + stamp, WorkspaceID: tenant.ID, Kind: DatastoreKindPostgres,
			Phase: "running", Availability: availability, AvailabilityObserved: true, At: obsAt, ReadyTransitionAt: transition}
		if availability == "unhealthy" {
			obs.ReasonCode = EventReasonReadinessFailed
		}
		facts, err := st.RecordObservedDatastoreState(ctx, obs)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return facts
	}
	record(at(10), "healthy", at(5))
	if f := record(at(106), "unhealthy", at(68)); len(f) != 1 || !f[0].At.Equal(at(68)) {
		t.Fatalf("unavailable edge = %+v, want it at the :68 transition", f)
	}
	if f := record(at(119), "healthy", at(113)); len(f) != 1 || !f[0].At.Equal(at(113)) {
		t.Fatalf("available edge = %+v, want it at the :113 transition", f)
	}
	if f := record(at(200), "unhealthy", at(100)); len(f) != 1 || !f[0].At.Equal(at(200)) {
		t.Fatalf("stale transition: %+v, want the observation time", f)
	}
}

// The datastore twin of TestPGFailureEdgeOutlivesTheDebouncedPhaseWrite: a
// Database that loses its only instance reports Ready → Provisioning, and the
// debounced first pass still records that phase.
func TestPGDatastoreFailureEdgeOutlivesTheDebouncedPhaseWrite(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	name := fmt.Sprintf("dpg-debounce%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_event_facts WHERE datastore_id = $1`, name)
		_, _ = pool.Exec(context.Background(), `DELETE FROM datastore_observed_checkpoints WHERE datastore_id = $1`, name)
	})

	base := time.Date(2026, 10, 5, 6, 56, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	r := &Reconciler{Store: st}
	pass := func(sec int, db *appv1alpha1.Database) []DatastoreEventFact {
		t.Helper()
		db.Name, db.Labels = name, map[string]string{LabelTenant: tenant.ID}
		obs, ok := observedDatabaseStateFor(db)
		if !ok {
			t.Fatal("database not attributable")
		}
		obs.At = at(sec)
		facts, err := st.RecordObservedDatastoreState(ctx, r.guardDatastoreObservation(ctx, obs))
		if err != nil {
			t.Fatalf("record :%d: %v", sec, err)
		}
		return facts
	}

	pass(5, readyDatabase(at(0)))
	pass(35, readyDatabase(at(0)))
	if f := pass(65, downDatabase(at(62))); len(f) != 0 {
		t.Fatalf("debounced pass emitted %+v", f)
	}
	if f := pass(95, downDatabase(at(62))); len(f) != 1 || f[0].Type != DatastoreFactPostgresUnavailable || !f[0].At.Equal(at(62)) {
		t.Fatalf("unavailable edge = %+v, want one at the :62 Ready transition", f)
	}
	if f := pass(125, readyDatabase(at(120))); len(f) != 1 || f[0].Type != DatastoreFactPostgresAvailable || !f[0].At.Equal(at(120)) {
		t.Fatalf("available edge = %+v, want one at the :120 Ready transition", f)
	}
}
