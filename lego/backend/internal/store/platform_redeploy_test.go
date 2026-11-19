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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRecordPlatformRedeployFacts(t *testing.T) {
	ctx := context.Background()
	rec, st, cl := newTestReconciler(t)
	ten, err := st.CreateTenant(ctx, "acme", "free")
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("initial reconcile: %v", err)
	}

	startedAt := time.Date(2026, 11, 19, 1, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(25 * time.Second)

	// Missing start time records nothing.
	app := getApp(t, cl)
	app.Status.PlatformRedeploy = &appv1alpha1.PlatformRedeployStatus{
		TransitionID: "node-a-2",
		Node:         "node-a",
		Generation:   2,
		Trigger:      appv1alpha1.PlatformRedeployTriggerNodeReplacement,
		State:        appv1alpha1.PlatformRedeployStateStarted,
	}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile without start time: %v", err)
	}
	st.mu.Lock()
	if _, ok := st.eventFacts["zdr:"+row.ID+":node-a-2:started"]; ok {
		st.mu.Unlock()
		t.Fatal("transition without startedAt must record no fact")
	}
	st.mu.Unlock()

	// Started writes exactly one started fact with its trigger and timestamp.
	app = getApp(t, cl)
	app.Status.PlatformRedeploy.StartedAt = startedAt.Format(time.RFC3339Nano)
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile started: %v", err)
	}
	// Re-read writes no duplicate.
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile started repeat: %v", err)
	}

	startedKey := "zdr:" + row.ID + ":node-a-2:started"
	endedKey := "zdr:" + row.ID + ":node-a-2:ended"
	st.mu.Lock()
	gotStarted, ok := st.eventFacts[startedKey]
	_, hasEnded := st.eventFacts[endedKey]
	st.mu.Unlock()
	if !ok {
		t.Fatalf("missing %s fact", startedKey)
	}
	if gotStarted.Type != EventFactZeroDowntimeRedeployStarted ||
		gotStarted.ReasonCode != EventReasonNodeReplacement ||
		!gotStarted.At.Equal(startedAt) {
		t.Fatalf("started fact = %+v, want type=%s reason=%s at=%s",
			gotStarted, EventFactZeroDowntimeRedeployStarted, EventReasonNodeReplacement, startedAt)
	}
	if hasEnded {
		t.Fatalf("unexpected ended fact while transition is Started")
	}

	// Ended writes exactly one ended fact dated by FinishedAt.
	app = getApp(t, cl)
	app.Status.PlatformRedeploy.State = appv1alpha1.PlatformRedeployStateEnded
	app.Status.PlatformRedeploy.FinishedAt = finishedAt.Format(time.RFC3339Nano)
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile ended: %v", err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile ended repeat: %v", err)
	}

	st.mu.Lock()
	gotEnded, ok := st.eventFacts[endedKey]
	st.mu.Unlock()
	if !ok {
		t.Fatalf("missing %s fact", endedKey)
	}
	if gotEnded.Type != EventFactZeroDowntimeRedeployEnded ||
		gotEnded.ReasonCode != "" ||
		!gotEnded.At.Equal(finishedAt) {
		t.Fatalf("ended fact = %+v, want type=%s reason=\"\" at=%s",
			gotEnded, EventFactZeroDowntimeRedeployEnded, finishedAt)
	}

	// Consolidation trigger records node_consolidation on its own transition key.
	caStart := finishedAt.Add(time.Minute)
	app = getApp(t, cl)
	app.Status.PlatformRedeploy = &appv1alpha1.PlatformRedeployStatus{
		TransitionID: "node-b-3",
		Node:         "node-b",
		Generation:   3,
		Trigger:      appv1alpha1.PlatformRedeployTriggerNodeConsolidation,
		State:        appv1alpha1.PlatformRedeployStateStarted,
		StartedAt:    caStart.Format(time.RFC3339Nano),
	}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile consolidation: %v", err)
	}
	st.mu.Lock()
	gotCA, ok := st.eventFacts["zdr:"+row.ID+":node-b-3:started"]
	st.mu.Unlock()
	if !ok || gotCA.ReasonCode != EventReasonNodeConsolidation {
		t.Fatalf("consolidation fact = %+v (ok=%v), want reason %q", gotCA, ok, EventReasonNodeConsolidation)
	}
}
