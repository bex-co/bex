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
)

// A post-Running crash and its recovery, observed by the reconciler across
// many resync ticks, must land as exactly one server_failed + one
// server_available fact pair (ADR052 gap-register item 2 / w3/m78): the
// service_event_checkpoints diff — not any producer-side care — is what keeps
// a level-triggered 30s poll from re-emitting the same edge.
func TestPGObservedCrashEdgeEmitsExactlyOnePair(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
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
