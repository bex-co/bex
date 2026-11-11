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

// instance_exit_facts_test.go is w8/m158 t002. A running service's process
// that exits or is killed for memory was recorded as "The instance failed its
// health check", or, when it restarted before the second sampling pass, not
// at all. The operator records each exit; bex-api writes one server_failed
// per exit with its cause.

func TestInstanceExitFactsCarryTheCauseAndTheInstance(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 5, 14, 0, time.UTC)
	app := &appv1alpha1.App{}
	app.Status.InstanceExits = []appv1alpha1.InstanceExit{
		{Instance: "web-c", RestartCount: 1, ExitCode: 0, FinishedAt: metav1.NewTime(at.Add(3 * time.Minute))},
		{Instance: "web-b", RestartCount: 2, ExitCode: 137, OOMKilled: true, FinishedAt: metav1.NewTime(at.Add(90 * time.Second))},
		{Instance: "web-a", RestartCount: 1, ExitCode: 3, FinishedAt: metav1.NewTime(at)},
	}
	facts := instanceExitFacts("srv-1", app)
	if len(facts) != 3 {
		t.Fatalf("facts = %+v, want one per exit", facts)
	}
	for i, want := range []struct {
		key, instance, reason string
		exit                  *int32
	}{
		{"exit:srv-1:web-c:1", "web-c", EventReasonEarlyExit, nil},
		{"exit:srv-1:web-b:2", "web-b", EventReasonOOMKilled, nil},
		{"exit:srv-1:web-a:1", "web-a", EventReasonNonZeroExit, new(int32(3))},
	} {
		f := facts[i]
		if f.SourceKey != want.key || f.InstanceID != want.instance || f.ReasonCode != want.reason || f.Type != EventFactServerFailed || f.AppID != "srv-1" {
			t.Errorf("fact %d = %+v, want key %s instance %s reason %s", i, f, want.key, want.instance, want.reason)
		}
		if (f.ExitCode == nil) != (want.exit == nil) || (f.ExitCode != nil && *f.ExitCode != *want.exit) {
			t.Errorf("fact %d exit code = %v, want %v", i, f.ExitCode, want.exit)
		}
		if err := validateServiceEventFact(f); err != nil {
			t.Errorf("fact %d is not a valid fact: %v", i, err)
		}
	}
	if !facts[2].At.Equal(at) {
		t.Errorf("the fact is dated %s, want the exit's own time %s", facts[2].At, at)
	}
	// An exit that followed the outage is not an event.
	followed := &appv1alpha1.App{}
	followed.Status.InstanceExits = []appv1alpha1.InstanceExit{{Instance: "web-a", RestartCount: 2, ExitCode: 3, AfterUnready: true, FinishedAt: metav1.NewTime(at)}}
	if got := instanceExitFacts("srv-1", followed); len(got) != 0 {
		t.Errorf("an exit that followed the outage produced %+v", got)
	}
	// A service with nothing recorded writes nothing.
	if got := instanceExitFacts("srv-1", &appv1alpha1.App{}); len(got) != 0 {
		t.Errorf("no exits produced %+v", got)
	}
}

// One event per outage: when the process exiting also shows up as a sampled
// outage, the outage adds no server_failed of its own, and its
// server_available still follows. A process that is running and not Ready has
// no exit and keeps the outage's server_failed, and an exit the operator
// marked as following the outage adds nothing, then or after the recovery.
func TestPGAnInstanceExitIsTheOutagesOnlyServerFailed(t *testing.T) {
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

	appCR := func(phase appv1alpha1.AppPhase, ready metav1.ConditionStatus, reason string) *appv1alpha1.App {
		cr := &appv1alpha1.App{}
		cr.Status.Phase = phase
		cr.Status.ActiveRevision = "rev-1"
		cr.Status.Conditions = []metav1.Condition{{Type: "Ready", Status: ready, Reason: reason}}
		return cr
	}
	healthy := appCR(appv1alpha1.PhaseRunning, metav1.ConditionTrue, "Deployed")
	down := appCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "CrashLoopBackOff")

	// followed marks an exit of an instance that had already stopped being
	// ready, as the operator records it.
	exit := func(restart int32, code int32, after time.Duration, followed bool) appv1alpha1.InstanceExit {
		return appv1alpha1.InstanceExit{Instance: "web-a", RestartCount: restart, ExitCode: code, AfterUnready: followed, FinishedAt: metav1.NewTime(time.Now().Add(after))}
	}
	// Each step is what the operator's status shows on a run of passes: the
	// service down or up, and the exits recorded so far.
	type step struct {
		down  bool
		exits []appv1alpha1.InstanceExit
	}
	first := exit(1, 3, 0, false)
	for _, tc := range []struct {
		name       string
		steps      []step
		wantFailed []string // reason codes of the server_failed facts
	}{
		{"the process exited with status 3",
			[]step{{true, []appv1alpha1.InstanceExit{first}}, {false, []appv1alpha1.InstanceExit{first}}},
			[]string{EventReasonNonZeroExit}},
		{"a running process stopped passing its check",
			[]step{{true, nil}, {false, nil}},
			[]string{EventReasonReadinessFailed}},
		// The kubelet restarts an instance that keeps failing its check, and a
		// process that handles the signal exits cleanly. The outage already
		// has its event; the exit is not the cause.
		{"a health-check restart of a process that exits cleanly",
			[]step{{true, nil}, {true, []appv1alpha1.InstanceExit{exit(1, 0, time.Minute, true)}}, {false, []appv1alpha1.InstanceExit{exit(1, 0, time.Minute, true)}}},
			[]string{EventReasonReadinessFailed}},
		// A crash loop is one outage: its first exit is the event, and the
		// later ones fall inside it.
		{"a crash loop",
			[]step{
				{true, []appv1alpha1.InstanceExit{first}},
				{true, []appv1alpha1.InstanceExit{exit(2, 3, time.Minute, true), first}},
				{true, []appv1alpha1.InstanceExit{exit(3, 3, 2*time.Minute, true), exit(2, 3, time.Minute, true), first}},
				{false, []appv1alpha1.InstanceExit{exit(3, 3, 2*time.Minute, true), exit(2, 3, time.Minute, true), first}},
			},
			[]string{EventReasonNonZeroExit}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stamp := fmt.Sprintf("%d", time.Now().UnixNano())
			tenant, err := st.CreateWorkspace(ctx, "exit-"+stamp, PlanHobby, "exit-owner-"+stamp)
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
			observe := func(cr *appv1alpha1.App, ticks int) {
				t.Helper()
				for range ticks {
					// The reconciler's order: the exits first, then the sampled state.
					if err := st.InsertServiceEventFacts(ctx, instanceExitFacts(app.ID, cr)); err != nil {
						t.Fatalf("record exits: %v", err)
					}
					if _, err := st.RecordObservedServiceState(ctx, observedServiceStateFor(app.ID, cr, false)); err != nil {
						t.Fatalf("record observed state: %v", err)
					}
				}
			}
			observe(healthy, 3)
			for _, s := range tc.steps {
				cr := healthy.DeepCopy()
				if s.down {
					cr = down.DeepCopy()
				}
				cr.Status.InstanceExits = s.exits
				observe(cr, 3)
			}

			rows, err := pool.Query(ctx,
				`SELECT fact_type, reason_code FROM service_event_facts WHERE app_id = $1 AND fact_type IN ('server_failed', 'server_available') ORDER BY at, source_key`, app.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var failed []string
			available := 0
			for rows.Next() {
				var typ, reason string
				if err := rows.Scan(&typ, &reason); err != nil {
					t.Fatal(err)
				}
				if typ == string(EventFactServerFailed) {
					failed = append(failed, reason)
				} else {
					available++
				}
			}
			if fmt.Sprint(failed) != fmt.Sprint(tc.wantFailed) {
				t.Errorf("server_failed reasons = %v, want %v", failed, tc.wantFailed)
			}
			if available != 1 {
				t.Errorf("server_available facts = %d, want the recovery once", available)
			}
		})
	}
}
