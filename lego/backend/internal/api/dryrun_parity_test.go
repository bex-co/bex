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

package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// crdInvalid marks a value the emulated CRD rules refuse.
const crdInvalid = "crd-invalid"

// admission stands in for the API server's admission chain: a create meets the
// workspace's count cap while atCap is set, and an object whose start command
// or name carries crdInvalid is refused as an invalid field — for a real write
// and its dry-run alike. persisted counts the writes that were not dry-runs.
// limits instead caps a quota key at that many objects, counted from used and
// from each persisted create of that kind.
type admission struct {
	atCap        bool
	persisted    int
	limits, used map[string]int
}

func (a *admission) client(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if err := a.admit(obj, true); err != nil {
					return err
				}
				var o client.CreateOptions
				o.ApplyOptions(opts)
				if !slices.Contains(o.DryRun, metav1.DryRunAll) {
					a.persisted++
					if key := admissionFacts(obj).key; key != "" && a.used != nil {
						a.used[key]++
					}
				}
				return c.Create(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
				if err := a.admit(obj, false); err != nil {
					return err
				}
				var o client.PatchOptions
				o.ApplyOptions(opts)
				if !slices.Contains(o.DryRun, metav1.DryRunAll) {
					a.persisted++
				}
				return c.Patch(ctx, obj, p, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				a.persisted++
				return c.Update(ctx, obj, opts...)
			},
		}).Build()
}

// admitted is what the emulated admission judges of a tenant object: its kind
// and resource, the quota key that counts it, and the one field the emulated
// CRD rules read. key is empty for any other object.
type admitted struct{ kind, resource, key, path, value string }

func admissionFacts(obj client.Object) admitted {
	switch o := obj.(type) {
	case *appv1alpha1.App:
		return admitted{"App", "apps", store.AppsQuotaCountKey, "startCommand", o.Spec.StartCommand}
	case *appv1alpha1.Database:
		return admitted{"Database", "databases", store.DatabasesQuotaCountKey, "name", o.Spec.Name}
	case *appv1alpha1.KeyValue:
		return admitted{"KeyValue", "keyvalues", store.KeyValuesQuotaCountKey, "name", o.Spec.Name}
	}
	return admitted{}
}

func (a *admission) admit(obj client.Object, create bool) error {
	f := admissionFacts(obj)
	if f.key == "" {
		return nil
	}
	if limit := a.limits[f.key]; create && (a.atCap || limit > 0 && a.used[f.key] >= limit) {
		limit = max(limit, 1)
		return apierrors.NewForbidden(schema.GroupResource{Group: "app.bex.co", Resource: f.resource}, obj.GetName(),
			fmt.Errorf("exceeded quota: %s, requested: %s=1, used: %s=%d, limited: %s=%d", core.TenantQuotaName, f.key, f.key, limit, f.key, limit))
	}
	if strings.Contains(f.value, crdInvalid) {
		return apierrors.NewInvalid(schema.GroupKind{Group: "app.bex.co", Kind: f.kind}, obj.GetName(),
			field.ErrorList{field.Invalid(field.NewPath("spec", f.path), f.value, "refused by the CRD's rules")})
	}
	return nil
}

// refusingGate is a workspace with no payment method on file.
type refusingGate struct{ calls int }

func (g *refusingGate) RequirePaymentMethod(context.Context, string) error {
	g.calls++
	return core.NewPaymentRequiredError()
}

// protectedEnvironments answers the datastores' protection lookup.
type protectedEnvironments map[string]string

func (p protectedEnvironments) GetEnvironmentProtectedStatus(_ context.Context, id string) (string, error) {
	return p[id], nil
}

// protectedServices answers the one store read a service patch's preflight
// makes, its environment's protection. Every other IntentStore method is the
// nil interface's: a refused patch must never reach one.
type protectedServices struct {
	apps.IntentStore
	status string
}

func (p protectedServices) GetAppProtectedStatus(context.Context, string) (string, error) {
	return p.status, nil
}

type parityFixture struct {
	adm  *admission
	gate *refusingGate
	apps *apps.Service
	pg   *postgres.Service
	kv   *keyvalue.Service
}

// newParityFixture wires the three kinds over one admission client, a
// workspace (tea-a) with no payment method, and one existing service, Postgres
// and Key Value, each a member of environment env-1.
func newParityFixture(authz core.Checker, protected bool) *parityFixture {
	status := core.ProtectedStatusUnprotected
	if protected {
		status = core.ProtectedStatusProtected
	}
	web := ownedApp("web", "tea-a")
	web.Spec.Replicas = 1 // free-plan eligible, so a plan change to free can pass
	web.Labels[core.LabelAppID] = "srv-parity"
	web.Labels[store.LabelManagedBy] = store.ManagedByValue
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-parity", Namespace: "tea-a",
			Labels: map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a", core.LabelEnvironment: "env-1"}},
		Spec: appv1alpha1.DatabaseSpec{Name: "orders", Plan: "free"},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-parity", Namespace: "tea-a",
			Labels: map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a", core.LabelEnvironment: "env-1"}},
		Spec: appv1alpha1.KeyValueSpec{Name: "cache", Plan: "free"},
	}
	f := &parityFixture{adm: &admission{}, gate: &refusingGate{}}
	base := &core.Base{
		Client: f.adm.client(web, db, kv), Namespace: "default",
		Workspace: coretest.Workspaces{"tea-a"}, Authz: authz, Payment: f.gate,
	}
	f.apps = &apps.Service{Base: base}
	if protected {
		f.apps.Store = protectedServices{status: status}
	}
	environments := protectedEnvironments{"env-1": status}
	f.pg = &postgres.Service{Base: base, Protection: environments}
	f.kv = &keyvalue.Service{Base: base, Protection: environments}
	return f
}

// TestDryRunAnswersExactlyTheRealCall is w5/m116's matrix: for each kind and
// verb, under each condition the real call refuses, its dry-run refuses with
// the identical error and persists nothing. The dry-run runs first, so it
// cannot lean on anything the real call wrote.
func TestDryRunAnswersExactlyTheRealCall(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	type call func(*parityFixture, bool) error
	cases := []struct {
		name      string
		atCap     bool
		protected bool
		call      call
	}{
		{"service create at the count cap", true, false, func(f *parityFixture, dry bool) error {
			_, err := f.apps.Create(ctx, apps.CreateRequest{Name: "capped", Image: "nginx:1", DryRun: dry})
			return err
		}},
		{"service create on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			_, err := f.apps.Create(ctx, apps.CreateRequest{Name: "paid", Image: "nginx:1", Plan: "standard", DryRun: dry})
			return err
		}},
		{"service create with a CRD-invalid field", false, false, func(f *parityFixture, dry bool) error {
			_, err := f.apps.Create(ctx, apps.CreateRequest{Name: "invalid", Image: "nginx:1", StartCommand: crdInvalid, DryRun: dry})
			return err
		}},
		{"service plan change on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			if dry {
				_, err := f.apps.SetPlanDryRun(ctx, "web", "standard")
				return err
			}
			_, err := f.apps.SetPlan(ctx, "web", "standard")
			return err
		}},
		{"service update on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			return servicePatch(ctx, f, dry, apps.ServicePatch{Plan: strp("standard")})
		}},
		{"service update in a protected environment", false, true, func(f *parityFixture, dry bool) error {
			return servicePatch(ctx, f, dry, apps.ServicePatch{StartCommand: strp("bin/other")})
		}},
		{"service update with a CRD-invalid field", false, false, func(f *parityFixture, dry bool) error {
			return servicePatch(ctx, f, dry, apps.ServicePatch{StartCommand: strp(crdInvalid)})
		}},
		{"Postgres create at the count cap", true, false, func(f *parityFixture, dry bool) error {
			_, err := f.pg.CreatePostgres(ctx, postgres.CreatePostgresRequest{Name: "capped", Plan: "free", DryRun: dry})
			return err
		}},
		{"Postgres create on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			_, err := f.pg.CreatePostgres(ctx, postgres.CreatePostgresRequest{Name: "paid", Plan: "basic-256mb", DryRun: dry})
			return err
		}},
		{"Postgres create with a CRD-invalid field", false, false, func(f *parityFixture, dry bool) error {
			_, err := f.pg.CreatePostgres(ctx, postgres.CreatePostgresRequest{Name: crdInvalid, Plan: "free", DryRun: dry})
			return err
		}},
		{"Postgres plan change on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			if dry {
				_, err := f.pg.SetPlanDryRun(ctx, "dpg-parity", "basic-256mb")
				return err
			}
			_, err := f.pg.SetPlan(ctx, "dpg-parity", "basic-256mb")
			return err
		}},
		{"Postgres update on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			return postgresPatch(ctx, f, dry, postgres.PostgresPatch{Plan: strp("basic-256mb")})
		}},
		{"Postgres update in a protected environment", false, true, func(f *parityFixture, dry bool) error {
			return postgresPatch(ctx, f, dry, postgres.PostgresPatch{Name: strp("renamed")})
		}},
		{"Postgres update with a CRD-invalid field", false, false, func(f *parityFixture, dry bool) error {
			return postgresPatch(ctx, f, dry, postgres.PostgresPatch{Name: strp(crdInvalid)})
		}},
		{"Key Value create at the count cap", true, false, func(f *parityFixture, dry bool) error {
			_, err := f.kv.CreateKeyValue(ctx, keyvalue.CreateKeyValueRequest{Name: "capped", Plan: "free", DryRun: dry})
			return err
		}},
		{"Key Value create on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			_, err := f.kv.CreateKeyValue(ctx, keyvalue.CreateKeyValueRequest{Name: "paid", Plan: "starter", DryRun: dry})
			return err
		}},
		{"Key Value create with a CRD-invalid field", false, false, func(f *parityFixture, dry bool) error {
			_, err := f.kv.CreateKeyValue(ctx, keyvalue.CreateKeyValueRequest{Name: crdInvalid, Plan: "free", DryRun: dry})
			return err
		}},
		{"Key Value plan change on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			if dry {
				_, err := f.kv.SetPlanDryRun(ctx, "red-parity", "starter")
				return err
			}
			_, err := f.kv.SetPlan(ctx, "red-parity", "starter")
			return err
		}},
		{"Key Value update on an unpaid paid plan", false, false, func(f *parityFixture, dry bool) error {
			return keyValuePatch(ctx, f, dry, keyvalue.KeyValuePatch{Plan: strp("starter")})
		}},
		{"Key Value update in a protected environment", false, true, func(f *parityFixture, dry bool) error {
			return keyValuePatch(ctx, f, dry, keyvalue.KeyValuePatch{Name: strp("renamed")})
		}},
		{"Key Value update with a CRD-invalid field", false, false, func(f *parityFixture, dry bool) error {
			return keyValuePatch(ctx, f, dry, keyvalue.KeyValuePatch{Name: strp(crdInvalid)})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newParityFixture(&fakeChecker{allow: true}, tc.protected)
			f.adm.atCap = tc.atCap
			dryErr := tc.call(f, true)
			if f.adm.persisted != 0 {
				t.Fatalf("dry-run persisted %d writes", f.adm.persisted)
			}
			realErr := tc.call(f, false)
			if realErr == nil {
				t.Fatal("the real call succeeded; the case does not exercise a refusal")
			}
			if dryErr == nil || dryErr.Error() != realErr.Error() {
				t.Fatalf("dry-run error = %v\nreal error    = %v", dryErr, realErr)
			}
		})
	}
}

// TestDryRunThatPassesWritesNothing: a dry-run every check accepts answers
// with its preview and persists nothing — the half of "stopped before its first
// write" the refusals above cannot show, since they never reach a write.
func TestDryRunThatPassesWritesNothing(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	f := newParityFixture(&fakeChecker{allow: true}, false)
	for name, dryRun := range map[string]func() error{
		"service create": func() error {
			_, err := f.apps.Create(ctx, apps.CreateRequest{Name: "fresh", Image: "nginx:1", DryRun: true})
			return err
		},
		"service plan change": func() error { _, err := f.apps.SetPlanDryRun(ctx, "web", "free"); return err },
		"service update": func() error {
			_, err := f.apps.ApplyServicePatchDryRun(ctx, "web", apps.ServicePatch{StartCommand: strp("bin/other")})
			return err
		},
		"Postgres create": func() error {
			_, err := f.pg.CreatePostgres(ctx, postgres.CreatePostgresRequest{Name: "fresh", Plan: "free", DryRun: true})
			return err
		},
		"Postgres plan change": func() error { _, err := f.pg.SetPlanDryRun(ctx, "dpg-parity", "free"); return err },
		"Postgres update": func() error {
			_, err := f.pg.UpdatePostgresDryRun(ctx, "dpg-parity", postgres.PostgresPatch{Name: strp("renamed")})
			return err
		},
		"Key Value create": func() error {
			_, err := f.kv.CreateKeyValue(ctx, keyvalue.CreateKeyValueRequest{Name: "fresh", Plan: "free", DryRun: true})
			return err
		},
		"Key Value plan change": func() error { _, err := f.kv.SetPlanDryRun(ctx, "red-parity", "free"); return err },
		"Key Value update": func() error {
			_, err := f.kv.UpdateKeyValueDryRun(ctx, "red-parity", keyvalue.KeyValuePatch{Name: strp("renamed")})
			return err
		},
	} {
		if err := dryRun(); err != nil {
			t.Fatalf("%s dry-run: %v", name, err)
		}
		if f.adm.persisted != 0 {
			t.Fatalf("%s dry-run persisted %d writes", name, f.adm.persisted)
		}
	}
	var db appv1alpha1.Database
	if err := f.pg.Client.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: "dpg-parity"}, &db); err != nil || db.Spec.Name != "orders" {
		t.Fatalf("Postgres after dry-runs = %q (%v), want it untouched", db.Spec.Name, err)
	}
	var kv appv1alpha1.KeyValue
	if err := f.kv.Client.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: "red-parity"}, &kv); err != nil || kv.Spec.Name != "cache" {
		t.Fatalf("Key Value after dry-runs = %q (%v), want it untouched", kv.Spec.Name, err)
	}
}

// TestDryRunShowsBillingOnlyToACallerWhoMayPerformTheCall: a viewer still gets
// the preview, without the payment state a 402 would disclose; the caller who
// may make the call gets exactly its 402 (w5/m116).
func TestDryRunShowsBillingOnlyToACallerWhoMayPerformTheCall(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "vera", Method: "session"})
	previews := map[string]func(*parityFixture) error{
		"service": func(f *parityFixture) error {
			_, err := f.apps.SetPlanDryRun(ctx, "web", "standard")
			return err
		},
		"Postgres": func(f *parityFixture) error {
			_, err := f.pg.SetPlanDryRun(ctx, "dpg-parity", "basic-256mb")
			return err
		},
		"Key Value": func(f *parityFixture) error {
			_, err := f.kv.SetPlanDryRun(ctx, "red-parity", "starter")
			return err
		},
	}
	for kind, preview := range previews {
		t.Run(kind, func(t *testing.T) {
			viewer := newParityFixture(&recordingChecker{decide: func(r string) bool { return roleGrants["viewer"][r] }}, false)
			if err := preview(viewer); err != nil || viewer.gate.calls != 0 {
				t.Fatalf("viewer preview = %v after %d payment checks, want the preview and none", err, viewer.gate.calls)
			}
			operator := newParityFixture(&fakeChecker{allow: true}, false)
			if err := preview(operator); !errors.Is(err, core.ErrPaymentRequired) {
				t.Fatalf("operator preview = %v, want the real call's 402", err)
			}
			if viewer.adm.persisted+operator.adm.persisted != 0 {
				t.Fatal("a preview persisted a write")
			}
		})
	}
}

func servicePatch(ctx context.Context, f *parityFixture, dry bool, p apps.ServicePatch) error {
	if dry {
		_, err := f.apps.ApplyServicePatchDryRun(ctx, "web", p)
		return err
	}
	_, err := f.apps.ApplyServicePatch(ctx, "web", p)
	return err
}

func postgresPatch(ctx context.Context, f *parityFixture, dry bool, p postgres.PostgresPatch) error {
	if dry {
		_, err := f.pg.UpdatePostgresDryRun(ctx, "dpg-parity", p)
		return err
	}
	_, err := f.pg.UpdatePostgres(ctx, "dpg-parity", p)
	return err
}

func keyValuePatch(ctx context.Context, f *parityFixture, dry bool, p keyvalue.KeyValuePatch) error {
	if dry {
		_, err := f.kv.UpdateKeyValueDryRun(ctx, "red-parity", p)
		return err
	}
	_, err := f.kv.UpdateKeyValue(ctx, "red-parity", p)
	return err
}
