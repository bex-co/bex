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
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// served_release_pull_failure_test.go is w8/m156 t005. A wake, a restart or a
// reschedule of the live release opens no deploy row, so a pull failure there
// was recorded nowhere: no event, no alert, while the service could not start.

func servedPullFailureApp(servingGeneration, failedGeneration int64, reason string, at time.Time) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "web", "ws"
	app.Status.Image = "zot.bex-registry.svc:5000/ws/web:gen-8@sha256:" + strings.Repeat("a", 64)
	app.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: servingGeneration, Digest: "sha256:" + strings.Repeat("a", 64)}
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: reason,
		Message:            "image pull failed: the build this release runs on (ws/web:gen-8) is no longer stored",
		ObservedGeneration: failedGeneration, LastTransitionTime: metav1.NewTime(at),
	}}
	return app
}

func TestServedReleasePullFailureIsRecordedOncePerIncident(t *testing.T) {
	// The Rollout condition went False at an earlier release's failed rollout,
	// long before this one: rewriting it False keeps that time (w32/036).
	at := time.Date(2026, 11, 11, 15, 52, 0, 0, time.UTC)
	app := servedPullFailureApp(8, 8, "ImagePullBackOff", at)

	before := time.Now()
	fact, ok := servedReleasePullFailure("srv-1", app)
	if !ok {
		t.Fatal("the served release's pull failure produced no fact")
	}
	if fact.Type != EventFactImagePullFailed || fact.ReasonCode != EventReasonImagePullBackoff || fact.AppID != "srv-1" ||
		fact.Image != app.Status.Image || fact.DeployID != "" {
		t.Errorf("fact = %+v", fact)
	}
	if fact.At.Before(before) || fact.At.After(time.Now()) {
		t.Errorf("fact at %s, want the pass that saw it, not the condition's transition at %s", fact.At, at)
	}
	// Every later pass sees the same condition and writes the same key, which
	// the store's unique source key turns into a no-op.
	again, _ := servedReleasePullFailure("srv-1", app)
	if again.SourceKey != fact.SourceKey {
		t.Errorf("source key moved between passes: %q then %q", fact.SourceKey, again.SourceKey)
	}
	// The operator drops the verdict once the release recovers, so a later
	// failure is written with a transition time of its own: a new incident
	// (w32/042; the whole sequence through the reconciler is below).
	later := servedPullFailureApp(8, 8, "ImagePullBackOff", at.Add(3*time.Hour))
	if next, ok := servedReleasePullFailure("srv-1", later); !ok || next.SourceKey == fact.SourceKey {
		t.Errorf("a later incident reused the key %q (ok %v)", next.SourceKey, ok)
	}
}

// w32/042: the same served release fails to pull, recovers, and fails again,
// as the operator writes it. During the first outage the Rollout condition is
// rewritten False on every pass and keeps its transition time, the one an
// earlier failed rollout left. The recovery drops the release's own verdict
// (the operator's clearRecoveredRolloutVerdict), so the second outage's
// condition is written afresh. Two outages, two image_pull_failed facts. Until
// the operator dropped the verdict the second outage carried the first's key
// and recorded nothing.
func TestServedReleasePullFailureIsRecordedForEachOutage(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, _ := st.CreateTenant(ctx, "acme", "free")
	const image = "registry.example/acme/web:v1"
	row, _ := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: image, Port: 80, Replicas: 1, Tier: "free"})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("create projection: %v", err)
	}
	history, _ := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if len(history) != 1 {
		t.Fatalf("history = %+v, want the create deploy", history)
	}
	gen := history[0].Generation
	pass := func(mutate func(*appv1alpha1.App)) {
		t.Helper()
		app := getApp(t, cl)
		mutate(app)
		if err := cl.Status().Update(ctx, app); err != nil {
			t.Fatal(err)
		}
		if err := rec.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	served := func(a *appv1alpha1.App) {
		a.Status.Phase, a.Status.ReleaseGeneration, a.Status.ActiveRevision = appv1alpha1.PhaseRunning, gen, fmt.Sprintf("rev-%d", gen)
		a.Status.Image, a.Status.ImageDigest = image, &appv1alpha1.ImageDigestStatus{Generation: gen, Digest: "sha256:" + strings.Repeat("a", 64)}
		meta.RemoveStatusCondition(&a.Status.Conditions, appv1alpha1.ConditionRollout)
		meta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed",
			Message: "1/1 replicas ready", ObservedGeneration: a.Generation,
		})
	}
	cannotPull := func(message string) func(*appv1alpha1.App) {
		return func(a *appv1alpha1.App) {
			meta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
				Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "ImagePullBackOff",
				Message: message, ObservedGeneration: gen,
			})
			meta.SetStatusCondition(&a.Status.Conditions, metav1.Condition{
				Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: appv1alpha1.ReasonPriorReleaseServing,
				Message: "the latest rollout failed: " + message + "; the previously deployed release keeps serving", ObservedGeneration: a.Generation,
			})
		}
	}
	facts := func() []ServiceEventFact {
		t.Helper()
		st.mu.Lock()
		defer st.mu.Unlock()
		var out []ServiceEventFact
		for key, fact := range st.eventFacts {
			if strings.HasPrefix(key, fmt.Sprintf("release:%s:%d:image_pull_failed:", row.ID, gen)) {
				out = append(out, fact)
			}
		}
		return out
	}

	pass(served)
	if d, _ := st.GetDeploy(ctx, row.ID, history[0].ID); d.Status != DeployLive {
		t.Fatalf("setup: deploy %q, want live", d.Status)
	}

	// The first outage. The condition is False since an earlier release's
	// failed rollout three weeks ago, and keeps that time.
	earlier := metav1.NewTime(time.Now().Add(-21 * 24 * time.Hour).Truncate(time.Second))
	pass(func(a *appv1alpha1.App) {
		a.Status.Conditions = append(a.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
			Message: "rollout did not become healthy", ObservedGeneration: gen - 1, LastTransitionTime: earlier,
		})
		cannotPull("image pull is failing: the build this release runs on is no longer stored")(a)
		if c := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionRollout); !c.LastTransitionTime.Equal(&earlier) {
			t.Fatalf("setup: Rollout's transition moved to %s; the operator's write keeps %s", c.LastTransitionTime, earlier)
		}
	})
	pass(cannotPull("image pull is failing: the platform could not pull this release's build"))
	if got := facts(); len(got) != 1 || got[0].Type != EventFactImagePullFailed || got[0].DeployID != "" {
		t.Fatalf("first outage facts = %+v, want one image_pull_failed with no deploy", got)
	}

	// The pull recovers: the instances are ready, and the operator drops the
	// verdict. Nothing is recorded.
	pass(served)
	if got := facts(); len(got) != 1 {
		t.Fatalf("facts after the recovery = %+v, want the first outage's only", got)
	}

	// The second outage, written afresh.
	pass(cannotPull("image pull is failing: the build this release runs on is no longer stored"))
	if got := facts(); len(got) != 2 {
		t.Fatalf("facts after the second outage = %+v, want one for each outage", got)
	}
	if open, ok, _ := openDeployFor(ctx, st, row.ID); ok {
		t.Fatalf("an outage of the served release opened deploy %+v", open)
	}
}

func TestServedReleasePullFailureIgnoresWhatADeployRowRecords(t *testing.T) {
	at := time.Date(2026, 11, 11, 15, 52, 0, 0, time.UTC)
	for name, app := range map[string]*appv1alpha1.App{
		// A newer release failed to pull over the served one: its deploy row
		// recorded the failure when it closed.
		"a newer release failed over the served one": servedPullFailureApp(8, 9, "ImagePullBackOff", at),
		// The served release failed for another reason: not an image pull.
		"a crash, not a pull": servedPullFailureApp(8, 8, "CrashLoopBackOff", at),
		// Nothing has served yet.
		"nothing serving": func() *appv1alpha1.App {
			a := servedPullFailureApp(8, 8, "ImagePullBackOff", at)
			a.Status.ImageDigest = nil
			return a
		}(),
		// A verdict that passed.
		"a rollout that succeeded": func() *appv1alpha1.App {
			a := servedPullFailureApp(8, 8, "ImagePullBackOff", at)
			a.Status.Conditions[0].Status = metav1.ConditionTrue
			return a
		}(),
		"no rollout verdict": func() *appv1alpha1.App {
			a := servedPullFailureApp(8, 8, "ImagePullBackOff", at)
			a.Status.Conditions = nil
			return a
		}(),
	} {
		if fact, ok := servedReleasePullFailure("srv-1", app); ok {
			t.Errorf("%s: recorded %+v", name, fact)
		}
	}
}
