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
	"errors"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

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
	return policyOfApp(t, cl, "clickhouse")
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

// nativeApp is a migratable native runtime service named name with public id
// "srv-"+name in tea-qa.
func nativeApp(name, policy string, mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	return migratableApp(policy, append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
		a.Name = name
		a.Labels[core.LabelAppID] = "srv-" + name
		a.Spec.Image, a.Spec.Repo, a.Spec.Runtime, a.Spec.Builder = "", "https://github.com/example/app", "ruby", "native"
		a.Spec.StartCommand = "bundle exec passenger start"
	}}, mutate...)...)
}

func policyOfApp(t *testing.T, cl client.Client, name string) string {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "tea-qa", Name: name}, &a); err != nil {
		t.Fatal(err)
	}
	return a.Spec.ContainerPolicy
}

// w32/m462: a native runtime service moves onto render-v1 like a Docker one,
// one service at a time; a static site still has no policy to move to.
func TestMigrateContainerPolicyMovesANativeService(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newTenantStoreService(fakeWorkspace{}, rec, nativeApp("rb", ""))
	plan, err := svc.MigrateContainerPolicy(t.Context(), "srv-rb", false)
	if err != nil || plan.NoOp || plan.Applied || plan.To != appv1alpha1.ContainerPolicyRenderV1 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
	if plan, err = svc.MigrateContainerPolicy(t.Context(), "srv-rb", true); err != nil || !plan.Applied {
		t.Fatalf("apply = %+v, %v", plan, err)
	}
	if got := policyOfApp(t, cl, "rb"); got != appv1alpha1.ContainerPolicyRenderV1 {
		t.Fatalf("CR policy = %q", got)
	}
	if len(rec.deployCalls) != 1 || rec.deployCalls[0].Trigger != store.TriggerSettingsChange {
		t.Fatalf("deploys %+v, want one settings_change", rec.deployCalls)
	}
}

// The bulk mode selects native runtime services only — a linked service by
// its build source's runtime — and moves each one, reporting a refusal and
// going on to the next service.
func TestMigrateNativeContainerPoliciesSelectsAndContinuesPastRefusals(t *testing.T) {
	carrier := nativeApp("carrier", "", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeBuildSource })
	imageCarrier := migratableApp("", func(a *appv1alpha1.App) {
		a.Name, a.Labels[core.LabelAppID], a.Spec.Type = "img-carrier", "srv-img-carrier", appv1alpha1.TypeBuildSource
	})
	linked := func(to *appv1alpha1.App) func(*appv1alpha1.App) {
		return func(a *appv1alpha1.App) {
			a.Spec.Repo, a.Spec.Runtime, a.Spec.Builder = "", "", ""
			a.Spec.BuildSource = &appv1alpha1.BuildSourceLink{Name: to.Name}
		}
	}
	stray := nativeApp("stray", "")
	stray.Namespace = "default"
	apps := []*appv1alpha1.App{
		nativeApp("a-web", ""),
		nativeApp("b-strict", appv1alpha1.ContainerPolicyStrictV1, func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeBackgroundWorker }),
		nativeApp("c-suspended", "", func(a *appv1alpha1.App) { a.Spec.Suspended = true }),
		nativeApp("d-done", appv1alpha1.ContainerPolicyRenderV1),
		nativeApp("e-linked", "", linked(carrier)),
		nativeApp("f-unmanaged", "", func(a *appv1alpha1.App) { delete(a.Labels, store.LabelManagedBy) }),
		// Not native: a Docker service, a service linked to an image build
		// source, a static site, the build sources, a stray copy.
		migratableApp("", func(a *appv1alpha1.App) { a.Name, a.Labels[core.LabelAppID] = "docker", "srv-docker" }),
		nativeApp("img-linked", "", linked(imageCarrier)),
		nativeApp("site", "", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite }),
		carrier, imageCarrier, stray,
	}
	for _, apply := range []bool{false, true} {
		rec := &recordingStore{}
		svc, cl := newTenantStoreService(fakeWorkspace{}, rec, deepCopies(apps)...)
		var lines []ContainerPolicyMigrationLine
		summary, err := svc.MigrateNativeContainerPolicies(t.Context(), apply, func(l ContainerPolicyMigrationLine) { lines = append(lines, l) })
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, l := range lines {
			ids = append(ids, l.ServiceID)
		}
		want := []string{"srv-a-web", "srv-b-strict", "srv-c-suspended", "srv-d-done", "srv-e-linked", "srv-f-unmanaged"}
		if !slices.Equal(ids, want) {
			t.Fatalf("apply=%t: selected %v, want %v", apply, ids, want)
		}
		if lines[2].Refused == "" || !lines[3].NoOp {
			t.Fatalf("apply=%t: suspended %+v, already render-v1 %+v", apply, lines[2], lines[3])
		}
		if !apply {
			if summary != (ContainerPolicyMigrationSummary{Planned: 4, NoOp: 1, Refused: 1}) || len(rec.policyCalls) != 0 || len(rec.deployCalls) != 0 {
				t.Fatalf("dry run: summary %+v, row writes %v, deploys %d", summary, rec.policyCalls, len(rec.deployCalls))
			}
			continue
		}
		// The unmanaged service has no control-plane row to persist the
		// policy in: refused, after the suspended one, and the run went on.
		if lines[5].Refused == "" {
			t.Fatalf("a service with no control-plane row migrated: %+v", lines[5])
		}
		if summary != (ContainerPolicyMigrationSummary{Migrated: 3, NoOp: 1, Refused: 2}) {
			t.Fatalf("summary = %+v", summary)
		}
		for name, policy := range map[string]string{
			"a-web": appv1alpha1.ContainerPolicyRenderV1, "b-strict": appv1alpha1.ContainerPolicyRenderV1, "e-linked": appv1alpha1.ContainerPolicyRenderV1,
			"c-suspended": "", "f-unmanaged": "", "docker": "", "img-linked": "", "site": "", "carrier": "",
		} {
			if got := policyOfApp(t, cl, name); got != policy {
				t.Errorf("%s: policy %q, want %q", name, got, policy)
			}
		}
		if len(rec.deployCalls) != 3 {
			t.Fatalf("deploys = %d, want one rollout per migrated service", len(rec.deployCalls))
		}
	}
}

// A refusal found under the App's lock (the service was suspended between the
// plan and the patch) restores the persisted row, and the bulk run goes on.
func TestMigrateNativeContainerPoliciesRestoresARowRefusedUnderTheLock(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newTenantStoreService(fakeWorkspace{}, rec, nativeApp("a-raced", appv1alpha1.ContainerPolicyStrictV1), nativeApp("b-web", ""))
	raced := false
	svc.Client = interceptor.NewClient(cl.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
			if obj.GetName() == "a-raced" && !raced {
				raced = true
				var live appv1alpha1.App
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &live); err != nil {
					return err
				}
				live.Spec.Suspended = true
				if err := c.Update(ctx, &live); err != nil {
					return err
				}
			}
			return c.Patch(ctx, obj, p, opts...)
		},
	})
	var lines []ContainerPolicyMigrationLine
	summary, err := svc.MigrateNativeContainerPolicies(t.Context(), true, func(l ContainerPolicyMigrationLine) { lines = append(lines, l) })
	if err != nil {
		t.Fatal(err)
	}
	if !raced || len(lines) != 2 || lines[0].Refused == "" || !lines[1].Applied {
		t.Fatalf("raced=%t lines %+v", raced, lines)
	}
	if summary != (ContainerPolicyMigrationSummary{Migrated: 1, Refused: 1}) {
		t.Fatalf("summary = %+v", summary)
	}
	if want := []string{"srv-a-raced=render-v1", "srv-a-raced=strict-v1", "srv-b-web=render-v1"}; !slices.Equal(rec.policyCalls, want) {
		t.Fatalf("row writes %v, want %v", rec.policyCalls, want)
	}
	if got := policyOfApp(t, svc.Client, "a-raced"); got != appv1alpha1.ContainerPolicyStrictV1 {
		t.Fatalf("raced CR policy = %q", got)
	}
}

func deepCopies(apps []*appv1alpha1.App) []*appv1alpha1.App {
	out := make([]*appv1alpha1.App, len(apps))
	for i, a := range apps {
		out[i] = a.DeepCopy()
	}
	return out
}
