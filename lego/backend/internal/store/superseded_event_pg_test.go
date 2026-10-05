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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5/pgxpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestPGSupersededBuildFactPersists(t *testing.T) {
	ctx := context.Background()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	rec, _, cl := newTestReconciler(t)
	rec.Store = st
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("superseded-%d", time.Now().UnixNano()), PlanHobby, "test-superseded")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()
	row, err := st.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Repo: "https://example.com/acme/web.git",
		Branch: "main", Port: 80, Replicas: 1, Tier: "free",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("create projection: %v", err)
	}

	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseBuilding
	app.Status.ReleaseGeneration = 1
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: "Building", ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("observe generation 1 building: %v", err)
	}
	deploys, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].Status != DeployBuildInProgress {
		t.Fatalf("deploys after building = %+v (err %v), want one build_in_progress", deploys, err)
	}
	deployID := deploys[0].ID

	// A newer release generation lands without ever adopting this row — the
	// missed-observation case supersededDeployStatus exists for.
	app = getApp(t, cl)
	app.Status.ReleaseGeneration = 3
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("observe supersede: %v", err)
	}

	deploys, err = st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].Status != DeployCanceled {
		t.Fatalf("deploys after supersede = %+v (err %v), want one canceled", deploys, err)
	}
	if deploys[0].FailureReason != "" {
		t.Errorf("failure_reason = %q, want empty on supersede cancel", deploys[0].FailureReason)
	}
	if deploys[0].CancelReason != "Superseded by a newer release" {
		// No deploy row yet at generation 3 — fallback line, not a blank.
		t.Errorf("cancel_reason = %q, want the supersede fallback", deploys[0].CancelReason)
	}

	// Replaying observations cannot duplicate the durable lifecycle edge.
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListServiceEvents(ctx, row.ID, ten.ID, ServiceEventFilter{Limit: 100, FactTypes: []string{string(EventFactBuildEnded)}})
	if err != nil {
		t.Fatal(err)
	}
	ended := 0
	for _, event := range rows {
		if event.FactType == string(EventFactBuildEnded) && event.DeployID == deployID {
			ended++
			if event.ReasonCode != string(EventReasonSuperseded) || event.FactStatus != EventStatusCanceled {
				t.Fatalf("ended fact=%+v", event)
			}
		}
	}
	if ended != 1 {
		t.Fatalf("superseded build-ended count=%d, rows=%+v", ended, rows)
	}
	hooks, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), nil, []string{ten.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	hookEnded := 0
	for _, event := range hooks {
		if event.FactType == string(EventFactBuildEnded) && event.DeployID == deployID {
			hookEnded++
		}
	}
	if hookEnded != 1 {
		t.Fatalf("webhook superseded facts=%d", hookEnded)
	}
	// Explicit user cancellation retains its deliberately empty reason.
	canceled, err := st.CreateDeploy(ctx, row.ID, "api", "", 4, CommitInfo{}, "")
	if err != nil {
		t.Fatal(err)
	}
	mustTransition(t, st, canceled.ID, DeployBuildInProgress, nil)
	canceled, err = st.GetDeploy(ctx, row.ID, canceled.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range CanceledBuildLifecycleFacts(canceled) {
		if _, err := st.InsertServiceEventFact(ctx, fact); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = st.ListServiceEvents(ctx, row.ID, ten.ID, ServiceEventFilter{Limit: 100, FactTypes: []string{string(EventFactBuildEnded)}})
	if err != nil {
		t.Fatal(err)
	}
	userEnded := 0
	for _, event := range rows {
		if event.FactType == string(EventFactBuildEnded) && event.DeployID == canceled.ID {
			userEnded++
			if event.ReasonCode != "" || event.FactStatus != EventStatusCanceled {
				t.Fatalf("user cancel fact=%+v", event)
			}
		}
	}
	if userEnded != 1 {
		t.Fatalf("user cancel ended=%d", userEnded)
	}
}
