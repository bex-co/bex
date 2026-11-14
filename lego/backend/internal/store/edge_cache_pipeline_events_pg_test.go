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
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cacheToggle names an edge_cache_enabled_to value: kept (no toggle), on or off.
func cacheToggle(to *bool) string {
	switch {
	case to == nil:
		return "kept"
	case *to:
		return "on"
	default:
		return "off"
	}
}

// w9/m345 against real Postgres: an edge-cache profile save is an event only
// when it turned caching on or off, and the type filter tells the two apart in
// SQL, before the LIMIT.
func TestPGEdgeCacheProfileSavesListOnlyWhenCachingTurned(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "cached", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	record := func(minute int, verb string, enabledTo *bool) {
		t.Helper()
		if err := st.Record(ctx, core.AuditEvent{
			Caller: "fixture-owner", Verb: verb, Resource: core.WorkspaceObject(tenant.ID),
			Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed,
			At: at.Add(time.Duration(minute) * time.Minute), EdgeCacheEnabledTo: enabledTo,
		}); err != nil {
			t.Fatal(err)
		}
	}
	record(0, core.AuditVerbSetCacheProfile, new(true))  // turned on
	record(1, core.AuditVerbSetCacheProfile, nil)        // static -> all: no event
	record(2, core.AuditVerbPurgeCache, nil)             // purged
	record(3, core.AuditVerbSetCacheProfile, new(false)) // turned off

	verbs := []string{core.AuditVerbSetCacheProfile, core.AuditVerbPurgeCache}
	list := func(filter EdgeCacheFilter) []string {
		t.Helper()
		rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: verbs, EdgeCache: filter, Since: at.Add(-time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range rows {
			if r.Verb == core.AuditVerbPurgeCache {
				out = append(out, "purged")
				continue
			}
			out = append(out, cacheToggle(r.EdgeCacheEnabledTo))
		}
		return out
	}
	for _, tc := range []struct {
		name   string
		filter EdgeCacheFilter
		want   []string
	}{
		{"unfiltered", EdgeCacheFilterNone, []string{"off", "purged", "on"}},
		{"enabled", EdgeCacheFilterEnabled, []string{"purged", "on"}},
		{"disabled", EdgeCacheFilterDisabled, []string{"off", "purged"}},
	} {
		if got := list(tc.filter); !slices.Equal(got, tc.want) {
			t.Errorf("%s feed = %v, want %v (newest first; a save that kept caching on never lists)", tc.name, got, tc.want)
		}
	}

	// The by-id read carries the toggle too.
	rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: verbs, EdgeCache: EdgeCacheFilterDisabled, Since: at.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, rows[0].Key))
	if err != nil {
		t.Fatal(err)
	}
	if got := cacheToggle(lookup.Event.EdgeCacheEnabledTo); got != "off" {
		t.Fatalf("by-id toggle = %s, want off", got)
	}
	// The webhook feed never returns the save that kept caching on, so it
	// spends no page slot on a row that is no event.
	feed, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Hour), verbs, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var toggles []string
	for _, r := range feed {
		if r.Verb == core.AuditVerbSetCacheProfile {
			toggles = append(toggles, cacheToggle(r.EdgeCacheEnabledTo))
		}
	}
	if want := []string{"on", "off"}; !slices.Equal(toggles, want) {
		t.Fatalf("webhook feed toggles = %v, want %v", toggles, want)
	}
}

// w9/088 against real Postgres: a plan change's edge-cache toggle
// (apps.SetPlanEdgeCache, recorded beside its apps.SetPlan row) lists, filters
// and reads by id like a profile save's, carrying its edge_cache_enabled_to,
// and reaches the webhook feed; the plan_changed rows beside it are untouched
// by the edge-cache filter. RecordAll writes a pair all or none: the failed
// pair at minute 5 lists nothing.
func TestPGPlanChangeEdgeCacheTogglesListLikeProfileSaves(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
	})
	app, err := st.CreateApp(ctx, App{
		TenantID: tenant.ID, Name: "replanned", Image: "nginx:1", Type: "web_service",
		Branch: "main", Port: 80, Replicas: 1, Tier: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	row := func(minute int, verb string, enabledTo *bool) core.AuditEvent {
		ev := core.AuditEvent{
			Caller: "fixture-owner", Verb: verb, Resource: core.WorkspaceObject(tenant.ID),
			Target: core.ServiceTarget(app.ID), Outcome: core.AuditAllowed,
			At: at.Add(time.Duration(minute) * time.Minute), EdgeCacheEnabledTo: enabledTo,
		}
		if verb == core.AuditVerbSetPlan {
			ev.PlanFrom, ev.PlanTo = new("standard"), new("free")
		}
		return ev
	}
	if err := st.Record(ctx, row(0, core.AuditVerbSetCacheProfile, new(true))); err != nil { // the member turned caching on
		t.Fatal(err)
	}
	// Each plan change and the toggle it caused land in one transaction.
	for _, pair := range [][]core.AuditEvent{
		{row(1, core.AuditVerbSetPlan, nil), row(2, core.AuditVerbSetPlanEdgeCache, new(false))}, // to Free: cache stopped
		{row(3, core.AuditVerbSetPlan, nil), row(4, core.AuditVerbSetPlanEdgeCache, new(true))},  // back to paid: restarted
	} {
		if err := st.RecordAll(ctx, pair); err != nil {
			t.Fatal(err)
		}
	}
	// A pair whose second row fails leaves neither.
	bad := row(5, core.AuditVerbSetPlanEdgeCache, new(false))
	bad.Outcome = "bogus"
	if err := st.RecordAll(ctx, []core.AuditEvent{row(5, core.AuditVerbSetPlan, nil), bad}); err == nil {
		t.Fatal("RecordAll accepted a row the outcome check refuses")
	}

	verbs := []string{core.AuditVerbSetCacheProfile, core.AuditVerbSetPlanEdgeCache, core.AuditVerbSetPlan}
	label := func(verb string, to *bool) string {
		switch verb {
		case core.AuditVerbSetPlan:
			return "plan"
		case core.AuditVerbSetPlanEdgeCache:
			return "system-" + cacheToggle(to)
		}
		return cacheToggle(to)
	}
	list := func(filter EdgeCacheFilter) ([]string, []ServiceEventRow) {
		t.Helper()
		rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{Verbs: verbs, EdgeCache: filter, Since: at.Add(-time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range rows {
			out = append(out, label(r.Verb, r.EdgeCacheEnabledTo))
		}
		return out, rows
	}
	for _, tc := range []struct {
		name   string
		filter EdgeCacheFilter
		want   []string
	}{
		{"unfiltered", EdgeCacheFilterNone, []string{"system-on", "plan", "system-off", "plan", "on"}},
		{"enabled", EdgeCacheFilterEnabled, []string{"system-on", "plan", "plan", "on"}},
		{"disabled", EdgeCacheFilterDisabled, []string{"plan", "system-off", "plan"}},
	} {
		if got, _ := list(tc.filter); !slices.Equal(got, tc.want) {
			t.Errorf("%s feed = %v, want %v (newest first)", tc.name, got, tc.want)
		}
	}

	_, rows := list(EdgeCacheFilterDisabled)
	lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, rows[1].Key))
	if err != nil {
		t.Fatal(err)
	}
	if got := label(lookup.Event.Verb, lookup.Event.EdgeCacheEnabledTo); got != "system-off" {
		t.Fatalf("by-id read = %s, want system-off", got)
	}
	feed, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Hour), verbs, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var delivered []string
	for _, r := range feed {
		if r.Source == EventSourceAudit {
			delivered = append(delivered, label(r.Verb, r.EdgeCacheEnabledTo))
		}
	}
	if want := []string{"on", "plan", "system-off", "plan", "system-on"}; !slices.Equal(delivered, want) {
		t.Fatalf("webhook feed = %v, want %v (oldest first)", delivered, want)
	}
}

// buildRefusedCR is the App CR the operator leaves after refusing generation
// gen's build with reason: phase Failed, and the Build verdict for the release.
// The operator keeps the Build condition False from one failure to the next,
// so its transition time is the App's first build failure, conditionAt.
func buildRefusedCR(gen int64, reason, message string, conditionAt time.Time) *appv1alpha1.App {
	cr := &appv1alpha1.App{}
	cr.Generation = gen
	cr.Status.Phase = appv1alpha1.PhaseFailed
	cr.Status.ReleaseGeneration = gen
	cr.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
		Message: message, ObservedGeneration: gen, LastTransitionTime: metav1.NewTime(conditionAt),
	}, {
		Type: appv1alpha1.ConditionBuild, Status: metav1.ConditionFalse, Reason: reason,
		Message: message, ObservedGeneration: gen, LastTransitionTime: metav1.NewTime(conditionAt),
	}}
	return cr
}

// w9/m345 against real Postgres: a build the operator refused for exhausted
// pipeline minutes closes its deploy build_failed and records exactly one
// pipeline_minutes_exhausted, dated when the refusal is observed (never at the
// Build condition's older transition) and carrying the deploy's trigger on the
// list, the by-id read and the webhook feed. A build that fails for any other
// reason records none.
func TestPGPipelineMinutesRefusalRecordsOneEventBesideTheFailedDeploy(t *testing.T) {
	st, pool, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	r := &Reconciler{Store: st}

	deployFor := func(t *testing.T, name string) (App, Deploy) {
		t.Helper()
		// An id's leading characters are its timestamp; the tail differs.
		unique := ids.New(ids.Deploy)
		app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name + "-" + unique[len(unique)-8:],
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		d, err := st.CreateDeploy(ctx, app.ID, TriggerManual, "", 2, CommitInfo{}, "user-x")
		if err != nil {
			t.Fatal(err)
		}
		return app, d
	}
	factsOf := func(t *testing.T, appID string) []ServiceEventRow {
		t.Helper()
		rows, err := st.ListServiceEvents(ctx, appID, tenant.ID, ServiceEventFilter{FactTypes: []string{string(EventFactPipelineMinutesExhausted)}, Since: time.Now().Add(-72 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}

	for _, tc := range []struct {
		name        string
		conditionAt time.Time
	}{
		{"the first build failure", time.Now().UTC().Add(-5 * time.Second)},
		// The Build condition stayed False since a failure two days ago, so
		// its transition time says nothing about this refusal.
		{"a refusal after an earlier build failure", time.Now().UTC().Add(-48 * time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, open := deployFor(t, "refused")
			cr := buildRefusedCR(2, appv1alpha1.ReasonBuildMinutesExhausted, appv1alpha1.BuildMinutesExhaustedMessage, tc.conditionAt)
			r.recordDeploy(ctx, DesiredApp{App: app}, open, cr)
			r.recordDeploy(ctx, DesiredApp{App: app}, open, cr) // a resync of the same refusal
			closed, err := st.GetDeploy(ctx, app.ID, open.ID)
			if err != nil {
				t.Fatal(err)
			}
			if closed.Status != DeployBuildFailed || closed.FailureReason != appv1alpha1.BuildMinutesExhaustedMessage || closed.FinishedAt == nil {
				t.Fatalf("deploy = %s %q, want build_failed with the minutes message", closed.Status, closed.FailureReason)
			}
			facts := factsOf(t, app.ID)
			if len(facts) != 1 {
				t.Fatalf("pipeline_minutes_exhausted rows = %d, want exactly one for one refused build", len(facts))
			}
			f := facts[0]
			if f.DeployID != open.ID || f.Trigger != TriggerManual || f.Caller != "user-x" || f.ClearCache {
				t.Fatalf("fact = deploy %s trigger %q caller %q clearCache %v, want %s, manual by user-x",
					f.DeployID, f.Trigger, f.Caller, f.ClearCache, open.ID)
			}
			// Dated now, after its deploy_started and no later than its
			// deploy_ended. The second of slack is between the database clock
			// (deploy rows) and this process's (the fact).
			const skew = time.Second
			if f.At.Before(open.CreatedAt.Add(-skew)) || f.At.After(closed.FinishedAt.Add(skew)) || time.Since(f.At) > time.Minute {
				t.Fatalf("fact dated %s, want between deploy_started %s and deploy_ended %s (the Build condition says %s)",
					f.At, open.CreatedAt, *closed.FinishedAt, tc.conditionAt)
			}
			lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, f.Key))
			if err != nil {
				t.Fatal(err)
			}
			if e := lookup.Event; e.FactType != string(EventFactPipelineMinutesExhausted) || e.DeployID != open.ID || e.Trigger != TriggerManual || e.Caller != "user-x" {
				t.Fatalf("by-id = %+v, want the fact with its deploy's trigger", e)
			}
			feed, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Hour), nil, []string{tenant.ID}, 100)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.ContainsFunc(feed, func(w WebhookEventRow) bool {
				return w.FactType == string(EventFactPipelineMinutesExhausted) && w.Key == f.Key && w.AppID == app.ID
			}) {
				t.Fatalf("webhook feed has no pipeline_minutes_exhausted for %s: %+v", open.ID, feed)
			}
			var stored int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM service_event_facts WHERE app_id = $1 AND fact_type = 'pipeline_minutes_exhausted'`, app.ID).Scan(&stored); err != nil || stored != 1 {
				t.Fatalf("stored facts = %d (%v), want 1", stored, err)
			}
		})
	}

	// An ordinary build failure is no pipeline event.
	other, failing := deployFor(t, "failing")
	r.recordDeploy(ctx, DesiredApp{App: other}, failing, buildRefusedCR(2, appv1alpha1.ReasonBuildFailedUserError, "build failed: exit 1", time.Now().UTC()))
	if got, err := st.GetDeploy(ctx, other.ID, failing.ID); err != nil || got.Status != DeployBuildFailed {
		t.Fatalf("failing deploy = %+v (%v), want build_failed", got, err)
	}
	if facts := factsOf(t, other.ID); len(facts) != 0 {
		t.Fatalf("an ordinary build failure recorded %d pipeline_minutes_exhausted rows", len(facts))
	}
}

// w4/639: a build_started fact carries its deploy's trigger and opener on the
// feed and by id, as Render's Build Started requires; build_ended still
// carries none.
func TestPGBuildStartedCarriesItsDeploysTrigger(t *testing.T) {
	st, _, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	unique := ids.New(ids.Deploy)
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "built-" + unique[len(unique)-8:],
		Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.CreateDeploy(ctx, app.ID, TriggerManual, "", 2, CommitInfo{}, "user-x")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := st.InsertServiceEventFacts(ctx, []ServiceEventFact{
		{SourceKey: "build:" + d.ID + ":started", AppID: app.ID, Type: EventFactBuildStarted, At: now, DeployID: d.ID},
		{SourceKey: "build:" + d.ID + ":ended", AppID: app.ID, Type: EventFactBuildEnded, At: now.Add(time.Second), DeployID: d.ID, Status: EventStatusFailed},
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{
		FactTypes: []string{string(EventFactBuildStarted), string(EventFactBuildEnded)}, Since: now.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want build_started and build_ended", len(rows))
	}
	for _, r := range rows {
		switch r.FactType {
		case string(EventFactBuildStarted):
			if r.Trigger != TriggerManual || r.Caller != "user-x" {
				t.Errorf("build_started trigger %q caller %q, want manual by user-x", r.Trigger, r.Caller)
			}
			lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, r.Key))
			if err != nil {
				t.Fatal(err)
			}
			if lookup.Event.Trigger != TriggerManual || lookup.Event.Caller != "user-x" {
				t.Errorf("by-id build_started = %+v, want the deploy's trigger", lookup.Event)
			}
		case string(EventFactBuildEnded):
			if r.Trigger != "" || r.Caller != "" {
				t.Errorf("build_ended trigger %q caller %q, want none", r.Trigger, r.Caller)
			}
		}
	}
}
