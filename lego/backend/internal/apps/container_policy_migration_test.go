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
	"errors"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func migratableApp(policy string, mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	a := sampleApp("clickhouse")
	a.Namespace = "tea-qa"
	a.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue, core.LabelAppID: "srv-ch", core.LabelTenant: "tea-qa",
	}
	a.Spec.ContainerPolicy = policy
	for _, m := range mutate {
		m(a)
	}
	return a
}

func policyOf(t *testing.T, cl client.Client) string {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "tea-qa", Name: "clickhouse"}, &a); err != nil {
		t.Fatal(err)
	}
	return a.Spec.ContainerPolicy
}

// w6/m149 t008: an existing service moves onto render-v1 only through this
// operator-run step — a dry run writes nothing, the real run persists the row
// and rolls once through deploy history, and a rerun is a no-op.
func TestMigrateContainerPolicyPlansThenRollsOnce(t *testing.T) {
	for _, from := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1} {
		t.Run(from, func(t *testing.T) {
			rec := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{}, rec, migratableApp(from))
			plan, err := svc.MigrateContainerPolicy(t.Context(), "srv-ch", false)
			if err != nil || plan.Applied || plan.NoOp || plan.From != from || plan.To != appv1alpha1.ContainerPolicyRenderV1 || plan.Workspace != "tea-qa" {
				t.Fatalf("dry run = %+v, %v", plan, err)
			}
			if len(rec.policyCalls) != 0 || len(rec.deployCalls) != 0 || policyOf(t, cl) != from {
				t.Fatal("dry run wrote")
			}
			if plan, err = svc.MigrateContainerPolicy(t.Context(), "srv-ch", true); err != nil || !plan.Applied {
				t.Fatalf("apply = %+v, %v", plan, err)
			}
			if got := policyOf(t, cl); got != appv1alpha1.ContainerPolicyRenderV1 {
				t.Fatalf("CR policy = %q", got)
			}
			if len(rec.policyCalls) != 1 || rec.policyCalls[0] != "srv-ch=render-v1" || len(rec.deployCalls) != 1 || rec.deployCalls[0].Trigger != store.TriggerSettingsChange {
				t.Fatalf("row writes %v, deploys %+v", rec.policyCalls, rec.deployCalls)
			}
			if plan, err = svc.MigrateContainerPolicy(t.Context(), "srv-ch", true); err != nil || !plan.NoOp || plan.Applied || len(rec.deployCalls) != 1 {
				t.Fatalf("rerun = %+v, %v, deploys %d", plan, err, len(rec.deployCalls))
			}
		})
	}
}

func TestMigrateContainerPolicyRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		want error
	}{
		{"native runtime", migratableApp("", func(a *appv1alpha1.App) { a.Spec.Image, a.Spec.Runtime = "", "go" }), core.ErrBadRequest},
		{"static site", migratableApp("", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite }), core.ErrBadRequest},
		{"suspended", migratableApp("", func(a *appv1alpha1.App) { a.Spec.Suspended = true }), core.ErrConflict},
		{"deploy in flight", migratableApp("", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseDeploying }), core.ErrConflict},
		{"never deployed", migratableApp("", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhasePending }), core.ErrConflict},
		{"change not yet applied", migratableApp("", func(a *appv1alpha1.App) { a.Generation = 3; a.Status.ObservedGeneration = 2 }), core.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{}, rec, tc.app)
			if _, err := svc.MigrateContainerPolicy(t.Context(), "srv-ch", true); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if len(rec.policyCalls) != 0 || len(rec.deployCalls) != 0 || policyOf(t, cl) != "" {
				t.Fatal("a refused migration wrote")
			}
		})
	}
	svc, _ := newTenantStoreService(fakeWorkspace{}, &recordingStore{})
	if _, err := svc.MigrateContainerPolicy(t.Context(), "srv-missing", false); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown service: %v", err)
	}
}

// A stray same-id copy outside the service's own workspace namespace is
// ignored, not an ambiguity that blocks the real service.
func TestMigrateContainerPolicyIgnoresAStrayCopy(t *testing.T) {
	own := migratableApp("")
	stray := migratableApp("")
	stray.Namespace = "default"
	rec := &recordingStore{}
	svc, _ := newTenantStoreService(fakeWorkspace{}, rec, own, stray)
	if plan, err := svc.MigrateContainerPolicy(t.Context(), "srv-ch", false); err != nil || plan.Workspace != "tea-qa" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
}

// A failed row write leaves the persisted policy and the CR as they were.
func TestMigrateContainerPolicyFailureKeepsThePolicy(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newTenantStoreService(fakeWorkspace{}, rec, migratableApp(appv1alpha1.ContainerPolicyImageV1))
	if _, err := svc.MigrateContainerPolicy(t.Context(), "srv-ch", true); err == nil {
		t.Fatal("a failed row write reported success")
	}
	if got := policyOf(t, cl); got != appv1alpha1.ContainerPolicyImageV1 {
		t.Fatalf("CR policy moved to %q after a failed write", got)
	}
}
