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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// healthCheckOutage is a live release whose one instance fails its health
// check: Ready names the failing check (reason), and the operator recorded
// what it last got back (result, "" for none).
func healthCheckOutage(reason, result string) *appv1alpha1.App {
	cr := &appv1alpha1.App{}
	cr.Status.Phase = appv1alpha1.PhaseRunning
	cr.Status.ActiveRevision = "rev-1"
	cr.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason}}
	if result != "" {
		cr.Status.HealthCheckFailure = &appv1alpha1.HealthCheckFailureStatus{Instance: "web-1", Message: result}
	}
	return cr
}

// w6/m634: a readiness failure carries the recorded result only while Ready
// names the failing health check, and only within the status field's bound.
func TestObservedOutageCarriesTheHealthCheckResult(t *testing.T) {
	const result = "HTTP health check failed with status code 503"
	for _, tc := range []struct {
		name string
		cr   *appv1alpha1.App
		want string
	}{
		{"failing check with a result", healthCheckOutage(appv1alpha1.ReasonHealthCheckFailing, result), result},
		{"failing check, no result", healthCheckOutage(appv1alpha1.ReasonHealthCheckFailing, ""), ""},
		{"a record left beside another reason", healthCheckOutage("RolloutProgressing", result), ""},
		{"too long a record", healthCheckOutage(appv1alpha1.ReasonHealthCheckFailing, "HTTP health check failed: "+strings.Repeat("é", 300)), ""},
	} {
		obs := observedServiceStateFor("srv-1", tc.cr, false)
		if obs.ReasonCode != EventReasonReadinessFailed {
			t.Fatalf("%s: observation %+v is no readiness failure", tc.name, obs)
		}
		if tc.name == "too long a record" {
			if len(obs.HealthCheckResult) > appv1alpha1.HealthCheckFailureMaxLength || !strings.HasPrefix(obs.HealthCheckResult, "HTTP health check failed: é") ||
				strings.ContainsRune(obs.HealthCheckResult, '�') {
				t.Errorf("%s: %d bytes %q", tc.name, len(obs.HealthCheckResult), obs.HealthCheckResult)
			}
			continue
		}
		if obs.HealthCheckResult != tc.want {
			t.Errorf("%s: result %q, want %q", tc.name, obs.HealthCheckResult, tc.want)
		}
		facts := observedStateFacts(obs, string(appv1alpha1.PhaseRunning), "healthy", false, time.Time{})
		if len(facts) != 1 || facts[0].Type != EventFactServerFailed || facts[0].Message != tc.want {
			t.Errorf("%s: facts %+v, want one server_failed with message %q", tc.name, facts, tc.want)
		}
	}
	// A node loss is server_hardware_failure, which carries no result.
	lost := healthCheckOutage(appv1alpha1.ReasonNodeLost, result)
	if obs := observedServiceStateFor("srv-1", lost, false); obs.HealthCheckResult != "" {
		t.Errorf("node loss carries %q", obs.HealthCheckResult)
	}
}

// TestPGServerFailedKeepsItsHealthCheckResult is w6/m634 t004: the result an
// outage's observation carries is stored with its server_failed fact and read
// back on the feed, by id and for the alert email; an outage with none reads
// none.
func TestPGServerFailedKeepsItsHealthCheckResult(t *testing.T) {
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
	tenant, err := st.CreateWorkspace(ctx, "hc-result-"+stamp, PlanHobby, "alice-"+stamp)
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
	healthy := &appv1alpha1.App{}
	healthy.Status.Phase = appv1alpha1.PhaseRunning
	healthy.Status.ActiveRevision = "rev-1"
	healthy.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed"}}
	const result = "HTTP health check failed (timed out after 5 seconds)"
	for _, cr := range []*appv1alpha1.App{healthy, healthCheckOutage(appv1alpha1.ReasonHealthCheckFailing, result),
		healthy, healthCheckOutage(appv1alpha1.ReasonHealthCheckFailing, "")} {
		if _, err := st.RecordObservedServiceState(ctx, observedServiceStateFor(app.ID, cr, false)); err != nil {
			t.Fatalf("record observed state: %v", err)
		}
	}

	rows, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, ServiceEventFilter{FactTypes: []string{string(EventFactServerFailed)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].HealthCheckResult != "" || rows[1].HealthCheckResult != result {
		t.Fatalf("server_failed rows, newest first = %+v, want none then %q", rows, result)
	}
	byID, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, rows[1].Key))
	if err != nil {
		t.Fatal(err)
	}
	if byID.Event.HealthCheckResult != result {
		t.Errorf("by id = %+v, want %q", byID.Event, result)
	}
	results, err := st.HealthCheckResults(ctx, []string{rows[0].Key, rows[1].Key, "fact:unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[rows[1].Key] != result {
		t.Errorf("alert results = %v, want only %s: %q", results, rows[1].Key, result)
	}
}
