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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// readyCR builds the App CR status the reconciler's observation path reads,
// with an explicit Ready-condition LastTransitionTime — the operator-side
// timestamp the w6/m41 stale-conclusion guard orders on.
func readyCR(phase appv1alpha1.AppPhase, ready metav1.ConditionStatus, reason string, transitionedAt time.Time) *appv1alpha1.App {
	cr := &appv1alpha1.App{}
	cr.Status.Phase = phase
	cr.Status.ActiveRevision = "rev-1"
	cr.Status.Conditions = []metav1.Condition{{
		Type:               "Ready",
		Status:             ready,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(transitionedAt),
	}}
	return cr
}

func healthyCR(transitionedAt time.Time) *appv1alpha1.App {
	return readyCR(appv1alpha1.PhaseRunning, metav1.ConditionTrue, "Deployed", transitionedAt)
}

func crashedCR(transitionedAt time.Time) *appv1alpha1.App {
	// A post-Running crash routes the CR back through PhaseDeploying with a
	// CrashLoopBackOff Ready reason (same shape as the m78 harness).
	return readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "CrashLoopBackOff", transitionedAt)
}

func driveObservations(t *testing.T, rec *Reconciler, appID string, cr *appv1alpha1.App, ticks int) {
	t.Helper()
	for i := 0; i < ticks; i++ {
		rec.recordObservations(context.Background(), DesiredApp{App: App{ID: appID}}, cr, nil)
	}
}

func edgeCounts(st *memStore) (failed, available int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, fact := range st.eventFacts {
		switch fact.Type {
		case EventFactServerFailed:
			failed++
		case EventFactServerAvailable:
			available++
		}
	}
	return failed, available
}

func staleRejections(t *testing.T, m *ReconcilerMetrics) float64 {
	t.Helper()
	return testutil.ToFloat64(m.observationRejections.WithLabelValues(rejectReasonStaleTransition))
}

func newGuardedReconciler() (*Reconciler, *memStore, *ReconcilerMetrics) {
	st := newMemStore()
	metrics := NewReconcilerMetrics(prometheus.NewRegistry())
	return &Reconciler{Store: st, Metrics: metrics}, st, metrics
}

// A time-traveled conclusion — its Ready=False transition predates the
// recorded healthy checkpoint — records no edge no matter how many consecutive
// ticks deliver it, and never arms the debounce. The rejection counter moves
// once per refused conclusion, so the suppression is observable.
func TestStaleConclusionRecordsNoPhantomPair(t *testing.T) {
	rec, st, metrics := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()
	recoveredAt := base.Add(10 * time.Minute)

	driveObservations(t, rec, "srv-web", healthyCR(recoveredAt), 1) // baseline + healthy checkpoint
	driveObservations(t, rec, "srv-web", crashedCR(base), 2)        // crash-era conclusion, delivered twice
	driveObservations(t, rec, "srv-web", healthyCR(recoveredAt), 1)

	failed, available := edgeCounts(st)
	if failed != 0 || available != 0 {
		t.Fatalf("time-traveled conclusion recorded %d server_failed + %d server_available, want none", failed, available)
	}
	if got := staleRejections(t, metrics); got != 2 {
		t.Fatalf("stale rejections = %v, want 2 (one per refused tick)", got)
	}
}

// The guard must never swallow a real outage: each genuinely NEW crash
// transition is newer than the last healthy checkpoint, so a crash → recovery
// → crash → recovery sequence still records exactly two pairs, and none of it
// touches the rejection counter.
func TestStaleConclusionGuardKeepsRealOutages(t *testing.T) {
	rec, st, metrics := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	driveObservations(t, rec, "srv-web", healthyCR(base), 1)
	driveObservations(t, rec, "srv-web", crashedCR(base.Add(10*time.Minute)), 2)
	driveObservations(t, rec, "srv-web", healthyCR(base.Add(20*time.Minute)), 1)
	driveObservations(t, rec, "srv-web", crashedCR(base.Add(30*time.Minute)), 2) // a genuine second outage
	driveObservations(t, rec, "srv-web", healthyCR(base.Add(40*time.Minute)), 1)

	failed, available := edgeCounts(st)
	if failed != 2 || available != 2 {
		t.Fatalf("two real outages recorded %d server_failed + %d server_available, want exactly 2 + 2", failed, available)
	}
	if got := staleRejections(t, metrics); got != 0 {
		t.Fatalf("real outages moved the rejection counter to %v, want 0", got)
	}
}

// Fail open toward reporting: a healthy checkpoint whose transition time is
// unknown (a pre-migration row, or a condition recorded without a timestamp)
// must not suppress anything.
func TestStaleConclusionGuardFailsOpenWithoutCheckpointTime(t *testing.T) {
	rec, st, metrics := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	driveObservations(t, rec, "srv-web", healthyCR(time.Time{}), 1) // healthy, transition time unknown
	driveObservations(t, rec, "srv-web", crashedCR(base), 2)

	failed, _ := edgeCounts(st)
	if failed != 1 {
		t.Fatalf("unhealthy edge with unknown healthy checkpoint recorded %d server_failed, want 1 (fail open)", failed)
	}
	if got := staleRejections(t, metrics); got != 0 {
		t.Fatalf("fail-open record moved the rejection counter to %v, want 0", got)
	}
}

// A stale conclusion arriving between two real edges is rejected without
// disturbing either edge.
func TestStaleConclusionBetweenRealEdgesIsRejectedCleanly(t *testing.T) {
	rec, st, metrics := newGuardedReconciler()
	base := time.Now().Add(-time.Hour).UTC()

	driveObservations(t, rec, "srv-web", healthyCR(base.Add(10*time.Minute)), 1)
	driveObservations(t, rec, "srv-web", crashedCR(base.Add(20*time.Minute)), 2) // the real outage
	driveObservations(t, rec, "srv-web", healthyCR(base.Add(30*time.Minute)), 1) // the real recovery
	driveObservations(t, rec, "srv-web", crashedCR(base.Add(20*time.Minute)), 2) // stale re-read of the SAME crash

	failed, available := edgeCounts(st)
	if failed != 1 || available != 1 {
		t.Fatalf("recorded %d server_failed + %d server_available, want exactly 1 + 1", failed, available)
	}
	if got := staleRejections(t, metrics); got != 2 {
		t.Fatalf("stale rejections = %v, want 2", got)
	}
}

// observedServiceStateFor must surface the Ready condition's
// LastTransitionTime on every condition-derived availability conclusion — it
// is what the guard and the healthy checkpoint both order on.
func TestObservedServiceStateCarriesReadyTransitionTime(t *testing.T) {
	transitioned := time.Now().Add(-time.Minute).UTC()
	if obs := observedServiceStateFor("srv-web", healthyCR(transitioned), false); obs.ReadyTransitionAt.UTC() != transitioned {
		t.Fatalf("healthy observation transition = %v, want %v", obs.ReadyTransitionAt, transitioned)
	}
	if obs := observedServiceStateFor("srv-web", crashedCR(transitioned), false); obs.ReadyTransitionAt.UTC() != transitioned {
		t.Fatalf("unhealthy observation transition = %v, want %v", obs.ReadyTransitionAt, transitioned)
	}
}
