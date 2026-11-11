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

package keyvalue

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// plan_downgrade_test.go is w8/249. Render: "It is not currently possible to
// downgrade a Key Value instance" (render.com/docs/key-value § Changing compute
// plans). bex accepted any plan change, so `bex kv update --plan free` restarted
// a paid store onto the smaller size with no check on what it held.

func TestKeyValuePlanCannotMoveToASmallerPlan(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService()
	seedKeyValue(t, cl, "red-paid")
	storedPlan := func() string {
		t.Helper()
		var kv appv1alpha1.KeyValue
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "red-paid"}, &kv); err != nil {
			t.Fatal(err)
		}
		return kv.Spec.Plan
	}
	// Free to paid is an upgrade, and so is each larger plan after it.
	for _, plan := range []string{"starter", "standard"} {
		if _, err := svc.SetPlan(ctx, "red-paid", plan); err != nil {
			t.Fatalf("upgrade to %s: %v", plan, err)
		}
	}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"plan change to a smaller paid plan", func() error { _, err := svc.SetPlan(ctx, "red-paid", "starter"); return err }},
		{"plan change back to Free", func() error { _, err := svc.SetPlan(ctx, "red-paid", "free"); return err }},
		{"plan change by a size alias of a smaller plan", func() error { _, err := svc.SetPlan(ctx, "red-paid", "256mb"); return err }},
		{"dry run of a smaller plan", func() error { _, err := svc.SetPlanDryRun(ctx, "red-paid", "free"); return err }},
		{"patch to a smaller plan", func() error {
			_, err := svc.UpdateKeyValue(ctx, "red-paid", KeyValuePatch{Plan: new("starter")})
			return err
		}},
		{"patch dry run to Free", func() error {
			_, err := svc.UpdateKeyValueDryRun(ctx, "red-paid", KeyValuePatch{Plan: new("free")})
			return err
		}},
	} {
		err := tc.run()
		if errorCode(err) != PlanDowngradeUnsupportedCode {
			t.Errorf("%s => %v, want %s", tc.name, err, PlanDowngradeUnsupportedCode)
			continue
		}
		rec := httptest.NewRecorder()
		core.WriteErr(rec, err)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), PlanDowngradeUnsupportedCode) {
			t.Errorf("%s over REST = %d %s, want 400 + %s", tc.name, rec.Code, rec.Body.String(), PlanDowngradeUnsupportedCode)
		}
		if got := storedPlan(); got != "standard" {
			t.Fatalf("%s changed the stored plan to %q", tc.name, got)
		}
	}
	// Re-sending the current plan is no change, under its id or a size alias.
	for _, plan := range []string{"standard", "1g"} {
		if _, err := svc.SetPlan(ctx, "red-paid", plan); err != nil {
			t.Errorf("re-sending the current plan as %q: %v", plan, err)
		}
	}
	if _, err := svc.UpdateKeyValue(ctx, "red-paid", KeyValuePatch{Plan: new("standard")}); err != nil {
		t.Errorf("patching the current plan: %v", err)
	}
	// An upgrade still works on both paths.
	if _, err := svc.UpdateKeyValue(ctx, "red-paid", KeyValuePatch{Plan: new("3g")}); err != nil {
		t.Fatalf("patch upgrade to 3g: %v", err)
	}
	if _, err := svc.SetPlan(ctx, "red-paid", "pro"); err != nil || storedPlan() != "pro" {
		t.Fatalf("upgrade to pro: stored %q, err %v", storedPlan(), err)
	}
}

func TestErrPlanDowngrade(t *testing.T) {
	for _, tc := range []struct {
		plan, current string
		refused       bool
	}{
		{"free", "starter", true},
		{"starter", "pro", true},
		{"", "starter", true}, // an empty plan is the default, Free
		{"starter", "free", false},
		{"starter", "", false},
		{"pro_plus", "pro", false},
		{"pro", "pro", false},
		{"5g", "pro", false},    // the same plan by its size alias
		{"nope", "pro", false},  // an unknown plan is the caller's validation
		{"free", "nope", false}, // and so is an unknown stored plan
	} {
		err := ErrPlanDowngrade(tc.plan, tc.current)
		if got := errorCode(err) == PlanDowngradeUnsupportedCode; got != tc.refused {
			t.Errorf("ErrPlanDowngrade(%q, %q) = %v, want refused=%v", tc.plan, tc.current, err, tc.refused)
		}
	}
	err := ErrPlanDowngrade("starter", "pro_plus")
	if err == nil || !strings.Contains(err.Error(), "Pro Plus to Starter") {
		t.Errorf("message = %v, want both plans by their display names", err)
	}
}

// The dashboard's plan picker disables the plans listed before the current
// one, and the server compares memory. Both are the same rule only while the
// ladder lists plans from the smallest to the largest.
func TestValkeyLadderIsOrderedByMemory(t *testing.T) {
	var previous resource.Quantity
	for _, id := range tiers.Valkey.IDs() {
		tier, _ := tiers.Valkey.ByID(id)
		memory := resource.MustParse(tier.Memory)
		if memory.Cmp(previous) <= 0 {
			t.Fatalf("plan %q (%s) is not larger than the plan before it (%s)", id, tier.Memory, previous.String())
		}
		previous = memory
	}
}
