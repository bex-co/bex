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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w10/m420: a cron job has no instance and no health check, so it has no
// availability. Every Trigger Run and Cancel flipped the cron's Ready to False
// for a pass; sampled twice, that read as an outage, and production mailed
// "Service unavailable: qa-20261116-m300-cron (7 times)". Replayed through the
// reconciler's own guards, on real Postgres.
func TestPGCronJobRecordsNoAvailability(t *testing.T) {
	st, _, tenant := openDatastoreTestStore(t)
	ctx := context.Background()
	newApp := func(name string) string {
		t.Helper()
		app, err := st.CreateApp(ctx, App{
			TenantID: tenant.ID, Name: fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			Image: "busybox:1.36", Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
		})
		if err != nil {
			t.Fatalf("create app: %v", err)
		}
		return app.ID
	}
	base := time.Date(2026, 11, 16, 5, 48, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	cron := func(cr *appv1alpha1.App) *appv1alpha1.App {
		cr.Spec.Type = appv1alpha1.TypeCronJob
		return cr
	}
	r := &Reconciler{Store: st}
	pass := func(appID string, sec int, cr *appv1alpha1.App) []ServiceEventFact {
		t.Helper()
		obs := observedServiceStateFor(appID, cr, false)
		obs.At = at(sec)
		facts, err := st.RecordObservedServiceState(ctx, r.guardServiceObservation(ctx, obs))
		if err != nil {
			t.Fatalf("record :%d: %v", sec, err)
		}
		return facts
	}
	availabilityFacts := func(facts []ServiceEventFact) []ServiceEventFact {
		var out []ServiceEventFact
		for _, f := range facts {
			switch f.Type {
			case EventFactServerFailed, EventFactServerAvailable, EventFactServerHardwareFailure:
				out = append(out, f)
			}
		}
		return out
	}

	for _, reason := range []string{"Deploying", "CronRunFailed", "CronRunCancelFailed", "NodeLost", "CrashLoopBackOff"} {
		t.Run(reason, func(t *testing.T) {
			appID := newApp("cron")
			var facts []ServiceEventFact
			facts = append(facts, pass(appID, 0, cron(healthyCR(at(0))))...)
			down := cron(readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, reason, at(10)))
			for sec := 15; sec <= 75; sec += 15 {
				facts = append(facts, pass(appID, sec, down)...)
			}
			facts = append(facts, pass(appID, 90, cron(healthyCR(at(88))))...)
			if got := availabilityFacts(facts); len(got) != 0 {
				t.Fatalf("a cron job in Ready=False %s recorded availability facts %+v, want none", reason, got)
			}
		})
	}

	if got := instanceExitFacts("srv-cron", cron(&appv1alpha1.App{Status: appv1alpha1.AppStatus{
		InstanceExits: []appv1alpha1.InstanceExit{{Instance: "pod-1", RestartCount: 1, ExitCode: 1}},
	}})); len(got) != 0 {
		t.Fatalf("a cron job's instance exits produced %+v, want none", got)
	}

	// A checkpoint latched unhealthy before the fix shipped must not pair a
	// stray server_available with the outage it already reported.
	t.Run("pre-fix unhealthy checkpoint", func(t *testing.T) {
		appID := newApp("cron-latched")
		// Observed as the projection saw a cron before the fix: untyped.
		pass(appID, 0, healthyCR(at(0)))
		down := readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "Deploying", at(10))
		pass(appID, 15, down)
		if f := availabilityFacts(pass(appID, 30, down)); len(f) != 1 || f[0].Type != EventFactServerFailed {
			t.Fatalf("setup: the pre-fix projection recorded %+v, want one server_failed", f)
		}
		if f := availabilityFacts(pass(appID, 45, cron(healthyCR(at(40))))); len(f) != 0 {
			t.Fatalf("a cron job's recovery recorded %+v, want no server_available", f)
		}
	})

	// The guard is type-scoped: a web service in the same state is down.
	t.Run("web service still fails", func(t *testing.T) {
		appID := newApp("web")
		pass(appID, 0, healthyCR(at(0)))
		down := readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, "Deploying", at(10))
		pass(appID, 15, down)
		if f := availabilityFacts(pass(appID, 30, down)); len(f) != 1 || f[0].Type != EventFactServerFailed {
			t.Fatalf("a web service in Ready=False Deploying recorded %+v, want one server_failed", f)
		}
	})

	// Phase transitions still record: suspend and resume are a cron job's
	// events on Render too.
	t.Run("suspend and resume", func(t *testing.T) {
		appID := newApp("cron-suspend")
		pass(appID, 0, cron(healthyCR(at(0))))
		suspended := cron(readyCR(appv1alpha1.PhaseHibernated, metav1.ConditionFalse, "Suspended", at(10)))
		suspended.Spec.Suspended = true
		f := pass(appID, 15, suspended)
		if len(f) != 1 || f[0].Type != EventFactServiceSuspended {
			t.Fatalf("suspend recorded %+v, want one service_suspended", f)
		}
		f = pass(appID, 30, cron(healthyCR(at(25))))
		if len(f) != 1 || f[0].Type != EventFactServiceResumed {
			t.Fatalf("resume recorded %+v, want one service_resumed", f)
		}
	})
}
