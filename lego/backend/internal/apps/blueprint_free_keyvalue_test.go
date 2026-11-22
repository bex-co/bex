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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render: "only one Free Key Value can be active for any given workspace".
// A Blueprint sync holds a Key Value move onto free to that rule exactly as
// it holds a free create, judging the workspace the whole file leaves — the
// Key Value sibling of the Postgres whole-file rule (w4/m241). A store that
// stays free holds the slot; otherwise the first claim in file order takes it
// and every later claim is refused naming the holder. A sync that hands the
// slot from one store to another is admitted in either file order.

// keyValuesBlueprint declares one key-value service per name/plan pair, in
// order.
func keyValuesBlueprint(pairs ...string) string {
	var out strings.Builder
	out.WriteString("services:\n")
	for i := 0; i+1 < len(pairs); i += 2 {
		out.WriteString("  - type: keyvalue\n    name: " + pairs[i] + "\n    ipAllowList: []\n    plan: " + pairs[i+1] + "\n")
	}
	return out.String()
}

// keyValuePlans maps each store's display name to its stored plan.
func keyValuePlans(t *testing.T, svc *Service) map[string]string {
	t.Helper()
	var list appv1alpha1.KeyValueList
	if err := svc.Client.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	plans := map[string]string{}
	for _, kv := range list.Items {
		plans[kv.Spec.Name] = kv.Spec.Plan
	}
	return plans
}

func wantKeyValuePlans(t *testing.T, svc *Service, want map[string]string) {
	t.Helper()
	if got := keyValuePlans(t, svc); len(got) != len(want) {
		t.Fatalf("key values = %v, want %v", got, want)
	} else {
		for name, plan := range want {
			if got[name] != plan {
				t.Fatalf("key values = %v, want %v", got, want)
			}
		}
	}
}

func wantFreeKeyValueLimit(t *testing.T, what string, err error, holder string) {
	t.Helper()
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.FreeInstanceLimitCode || !strings.Contains(err.Error(), "already has a free Key Value store ("+holder+")") {
		t.Fatalf("%s = %v, want %s naming %s", what, err, core.FreeInstanceLimitCode, holder)
	}
}

// A sync that hands the free slot from one store to another is admitted
// whichever the file lists first: the apply moves the holder off free before
// it moves the other onto it, and the plan judges the workspace the file
// leaves, not the workspace it found. The claim side below runs a tier the
// catalog no longer names (a store paid-to-free moves cannot supply: the
// downgrade gate refuses those on both sides, as the plan-downgrade test
// pins).
func TestABlueprintSyncSwapsTheFreeKeyValueSlotInEitherFileOrder(t *testing.T) {
	for _, order := range []struct {
		name     string
		manifest string
		views    []string
	}{
		{"the holder listed first", keyValuesBlueprint("holder", "starter", "legacy-store", "free"), []string{"holder", "legacy-store"}},
		{"the claim listed first", keyValuesBlueprint("legacy-store", "free", "holder", "starter"), []string{"legacy-store", "holder"}},
	} {
		t.Run(order.name, func(t *testing.T) {
			svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
			locks := &tenantLockSpy{}
			svc.Workspace = &lockingFakeWorkspace{fakeWorkspace: fakeWorkspace{"identity-a": "tea-a"}, spy: locks}
			ctx := paidGateContext()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: keyValuesBlueprint("holder", "free")}); err != nil {
				t.Fatalf("seed holder: %v", err)
			}
			legacy := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-legacy", Namespace: "default", Labels: core.TenantLabels("tea-a")},
				Spec:       appv1alpha1.KeyValueSpec{Name: "legacy-store", Plan: "retired-tier"},
			}
			if err := cl.Create(ctx, legacy); err != nil {
				t.Fatalf("seed legacy store: %v", err)
			}
			if v, err := svc.ValidateBlueprint(ctx, "", order.manifest, ""); err != nil || !v.Valid {
				t.Fatalf("validate = %+v, %v; want valid", v, err)
			}
			res, err := svc.DeployStack(ctx, DeployRequest{Manifest: order.manifest})
			if err != nil {
				t.Fatalf("sync: %v", err)
			}
			wantKeyValuePlans(t, svc, map[string]string{"holder": "starter", "legacy-store": "free"})
			if len(res.KeyValues) != 2 || res.KeyValues[0].Name != order.views[0] || res.KeyValues[1].Name != order.views[1] {
				t.Fatalf("applied views = %+v, want %v as the file lists them", res.KeyValues, order.views)
			}
			// The seed's free create and the handoff's move onto free each
			// took the tenant lock once; the move off it took none.
			if got := locks.taken(); got != 2 {
				t.Fatalf("tenant lock taken %d times, want 2 (one free create, one move onto free)", got)
			}
		})
	}
}

// The file is judged as a whole: two stores claiming the free slot are one
// too many, whatever the order, and the sync is refused before it writes
// either.
func TestABlueprintSyncClaimingTheFreeKeyValueSlotTwiceIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, holder string
	}{
		{"two free creates", keyValuesBlueprint("kv-a", "free", "kv-b", "free"), "kv-a"},
		{"two free creates reversed", keyValuesBlueprint("kv-b", "free", "kv-a", "free"), "kv-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
			ctx := paidGateContext()
			v, err := svc.ValidateBlueprint(ctx, "", tc.manifest, "")
			if err != nil || v.Valid || len(v.Errors) != 1 {
				t.Fatalf("validate = %+v, %v; want one refusal", v, err)
			}
			e := v.Errors[0]
			if e.Code != core.FreeInstanceLimitCode || !strings.Contains(e.Error, "("+tc.holder+")") {
				t.Fatalf("validate refusal = %s: %s; want %s naming %s", e.Code, e.Error, core.FreeInstanceLimitCode, tc.holder)
			}
			_, err = svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest})
			wantFreeKeyValueLimit(t, "sync", err, tc.holder)
			if got := keyValuePlans(t, svc); len(got) != 0 {
				t.Fatalf("refused sync wrote %v, want nothing", got)
			}
		})
	}
}

// A store the file moves onto free meets the one-free rule a free create
// meets: beside a live holder the plan refuses exactly what the apply
// refuses. The store below runs a tier the catalog no longer names (a plan
// the downgrade gate cannot compare, so it lets the move through); the free
// rule is what refuses it.
func TestABlueprintSyncMoveOntoFreeBesideAFreeHolderIsRefused(t *testing.T) {
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := paidGateContext()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: keyValuesBlueprint("holder", "free")}); err != nil {
		t.Fatalf("seed holder: %v", err)
	}
	legacy := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-legacy", Namespace: "default", Labels: core.TenantLabels("tea-a")},
		Spec:       appv1alpha1.KeyValueSpec{Name: "legacy-store", Plan: "retired-tier"},
	}
	if err := cl.Create(ctx, legacy); err != nil {
		t.Fatalf("seed legacy store: %v", err)
	}
	moved := keyValuesBlueprint("holder", "free", "legacy-store", "free")
	v, err := svc.ValidateBlueprint(ctx, "", moved, "")
	if err != nil || v.Valid || len(v.Errors) != 1 {
		t.Fatalf("validate = %+v, %v; want one refusal", v, err)
	}
	e := v.Errors[0]
	if e.Code != core.FreeInstanceLimitCode || !strings.Contains(e.Error, "(holder)") {
		t.Fatalf("validate refusal = %s: %s; want %s naming holder", e.Code, e.Error, core.FreeInstanceLimitCode)
	}
	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: moved})
	wantFreeKeyValueLimit(t, "sync", err, "holder")
	wantKeyValuePlans(t, svc, map[string]string{"holder": "free", "legacy-store": "retired-tier"})
}
