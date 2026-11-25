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

package apps

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type restartBillingGate struct{ err error }

func (g *restartBillingGate) CheckBillingMutationAllowed(_ context.Context, _ string) error {
	return g.err
}

// wiredService returns the fixture with the deploy verb wired, as the
// composition root wires it (deploys.Service.Restart): the projection then
// describes that verb's guards, not the store-off CR stamp. The caller acts
// in the fixture's workspace, as the postgres restart-guard tests do.
func wiredService(rec *recordingStore, a *appv1alpha1.App) *Service {
	svc, _ := newService(rec, a)
	svc.Workspace = fakeWorkspace{"developer-1": "tea-a"}
	svc.RestartDeploy = func(context.Context, string) error { return nil }
	return svc
}

// TestActionCapabilities_RestartAgreesWithTheVerb: the serverActions restart
// row is the restart verb's answer (GraphQL restartServer, REST POST
// .../restart and MCP restart_service all run it when wired). Before this
// test the row read allowed with no precondition in every state, while the
// wired verb refuses a suspended service with 409 (validateTrigger), a
// billing-blocked workspace at the dunning gate, an unmanaged service with
// 400, and a cron job with 400 — an enabled Restart button that always
// failed after the click. The projection and the verb agree in every cell
// below; the deploys package pins the verb's refusals
// (TestRestartRecordsServerRestartedOnlyWhenItRuns,
// TestRestartRefusalNamesTheCallersService).
func TestActionCapabilities_RestartAgreesWithTheVerb(t *testing.T) {
	ctx := ctxAs("developer-1")

	tenantApp := func() *appv1alpha1.App {
		a := managedApp("web", "srv-1")
		a.Labels[core.LabelTenant] = "tea-a"
		return a
	}
	restartOf := func(t *testing.T, svc *Service, name string) (core.ActionDecision, bool) {
		t.Helper()
		acts, err := svc.ActionCapabilities(ctx, name)
		if err != nil {
			t.Fatalf("ActionCapabilities: %v", err)
		}
		for _, a := range acts {
			if a.Action == core.ActionRestart {
				return a, true
			}
		}
		return core.ActionDecision{}, false
	}

	t.Run("running service is ready", func(t *testing.T) {
		svc := wiredService(&recordingStore{}, tenantApp())
		asNonAdmin(svc)
		got, ok := restartOf(t, svc, "web")
		if !ok {
			t.Fatal("running service projects no restart row")
		}
		if got.Outcome != core.DecisionAllowed || got.Precondition != "" {
			t.Fatalf("restart = %+v, want allowed with no precondition", got)
		}
	})

	t.Run("suspended service names the state block", func(t *testing.T) {
		a := tenantApp()
		a.Spec.Suspended = true
		svc := wiredService(&recordingStore{}, a)
		asNonAdmin(svc)
		got, ok := restartOf(t, svc, "web")
		if !ok {
			t.Fatal("suspended service projects no restart row")
		}
		if got.Outcome != core.DecisionAllowed || got.Precondition != core.PrecondSuspended {
			t.Fatalf("restart = %+v, want allowed/suspended like the verb's 409", got)
		}
	})

	t.Run("a saved paid plan lifting the suspension names only billing", func(t *testing.T) {
		a := tenantApp()
		a.Spec.Suspended = true
		a.Spec.Tier = "starter"
		a.Status.ServedTier = "free"
		a.Annotations = map[string]string{core.AnnotFreeHoursSuspended: FreeHoursMonth(time.Now())}
		svc := wiredService(&recordingStore{}, a)
		asNonAdmin(svc)
		got, ok := restartOf(t, svc, "web")
		if !ok {
			t.Fatal("lift-state service projects no restart row")
		}
		// validateTrigger exempts exactly this state, so the verb proceeds
		// past the suspended check to the billing gate, which passes here.
		if got.Outcome != core.DecisionAllowed || got.Precondition != "" {
			t.Fatalf("restart = %+v, want allowed with no precondition", got)
		}
	})

	t.Run("billing-blocked workspace names dunning", func(t *testing.T) {
		svc := wiredService(&recordingStore{}, tenantApp())
		asNonAdmin(svc)
		svc.Billing = &restartBillingGate{err: core.ErrBillingEnforced}
		got, ok := restartOf(t, svc, "web")
		if !ok {
			t.Fatal("billing-blocked service projects no restart row")
		}
		if got.Outcome != core.DecisionAllowed || got.Precondition != core.PrecondBillingBlocked {
			t.Fatalf("restart = %+v, want allowed/billing_blocked like the verb's dunning refusal", got)
		}
	})

	t.Run("unmanaged service reads unavailable", func(t *testing.T) {
		a := sampleApp("web")
		a.Labels = map[string]string{core.LabelTenant: "tea-a"}
		svc := wiredService(&recordingStore{}, a)
		asNonAdmin(svc)
		got, ok := restartOf(t, svc, "web")
		if !ok {
			t.Fatal("unmanaged service projects no restart row")
		}
		// The wired verb has no deploy machinery for it (validateTrigger's
		// not-store-managed refusal), as the deploy projection reports.
		if got.Outcome != core.DecisionAllowed || got.Precondition != core.PrecondUnavailable {
			t.Fatalf("restart = %+v, want allowed/unavailable", got)
		}
	})

	t.Run("cron job has no restart row", func(t *testing.T) {
		a := tenantApp()
		a.Name = "nightly"
		a.Spec.Type = appv1alpha1.TypeCronJob
		svc := wiredService(&recordingStore{}, a)
		asNonAdmin(svc)
		if got, ok := restartOf(t, svc, "nightly"); ok {
			t.Fatalf("cron job projects restart = %+v, want no row: the verb refuses it 400", got)
		}
	})

	t.Run("store-off restart stays unconditioned, without a cron row", func(t *testing.T) {
		a := tenantApp()
		a.Spec.Suspended = true
		svc, _ := newService(&recordingStore{}, a)
		svc.Workspace = fakeWorkspace{"developer-1": "tea-a"}
		asNonAdmin(svc)
		// The store-off verb runs no guards: it stamps and rolls the pods,
		// so the projection must not invent a block the verb never raises.
		got, ok := restartOf(t, svc, "web")
		if !ok {
			t.Fatal("store-off service projects no restart row")
		}
		if got.Outcome != core.DecisionAllowed || got.Precondition != "" {
			t.Fatalf("store-off restart = %+v, want allowed with no precondition", got)
		}
		if _, err := svc.Restart(ctx, "web"); err != nil {
			t.Fatalf("store-off Restart of a suspended service = %v, want success as projected", err)
		}

		cron := tenantApp()
		cron.Name = "nightly"
		cron.Spec.Type = appv1alpha1.TypeCronJob
		cronSvc, _ := newService(&recordingStore{}, cron)
		cronSvc.Workspace = fakeWorkspace{"developer-1": "tea-a"}
		asNonAdmin(cronSvc)
		if got, ok := restartOf(t, cronSvc, "nightly"); ok {
			t.Fatalf("store-off cron projects restart = %+v, want no row: the verb refuses it 400", got)
		}
	})
}
