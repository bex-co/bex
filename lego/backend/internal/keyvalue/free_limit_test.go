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
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func seedFreeKeyValue(t *testing.T, svc *Service, name string) {
	t.Helper()
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: name, Plan: "free"},
	}
	if err := svc.Client.Create(context.Background(), kv); err != nil {
		t.Fatal(err)
	}
}

func errorCode(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func TestFreeKeyValueLimit(t *testing.T) {
	svc, _ := newService()
	seedFreeKeyValue(t, svc, "red-existing")

	if _, err := svc.CreateKeyValue(context.Background(), CreateKeyValueRequest{Name: "second-free", Plan: "free"}); err == nil || errorCode(err) != core.FreeInstanceLimitCode {
		t.Fatalf("second free create => %v, want FREE_INSTANCE_LIMIT", err)
	}
	if _, err := svc.CreateKeyValue(context.Background(), CreateKeyValueRequest{Name: "second-free-dry", Plan: "free", DryRun: true}); err == nil || errorCode(err) != core.FreeInstanceLimitCode {
		t.Fatalf("second free dry-run => %v, want FREE_INSTANCE_LIMIT", err)
	}
	if _, err := svc.CreateKeyValue(context.Background(), CreateKeyValueRequest{Name: "paid-one", Plan: "starter"}); err != nil {
		t.Fatalf("paid create with free present => %v, want admitted", err)
	}
}

func TestFreeKeyValueSlotIndependentFromPostgres(t *testing.T) {
	svc, _ := newService()
	// A free Key Value slot is its own: creating the first free KV passes.
	if _, err := svc.CreateKeyValue(context.Background(), CreateKeyValueRequest{Name: "only-free", Plan: "free"}); err != nil {
		t.Fatalf("first free KV => %v, want admitted", err)
	}
}

// A plan change onto free meets the one-free rule a free create meets: beside
// a live holder it is refused, on both the plan-only verbs, and a store that
// only reaches the check because its tier left the catalog (so the downgrade
// gate, which compares catalog memory, lets the move through) is refused
// there rather than silently taking a second slot.
func TestSetPlanOntoFreeBesideAHolderIsRefused(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService()
	seedFreeKeyValue(t, svc, "red-holder")
	retired := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-retired", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "retired", Plan: "retired-tier"},
	}
	if err := cl.Create(ctx, retired); err != nil {
		t.Fatal(err)
	}
	free := core.FreePlanID
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"SetPlan onto free", func() error { _, err := svc.SetPlan(ctx, "red-retired", "free"); return err }},
		{"SetPlan dry run onto free", func() error { _, err := svc.SetPlanDryRun(ctx, "red-retired", "free"); return err }},
		{"patch onto free", func() error { _, err := svc.UpdateKeyValue(ctx, "red-retired", KeyValuePatch{Plan: &free}); return err }},
		{"patch dry run onto free", func() error {
			_, err := svc.UpdateKeyValueDryRun(ctx, "red-retired", KeyValuePatch{Plan: &free})
			return err
		}},
	} {
		if err := tc.run(); err == nil || errorCode(err) != core.FreeInstanceLimitCode {
			t.Fatalf("%s => %v, want %s naming the holder", tc.name, err, core.FreeInstanceLimitCode)
		}
	}
	var stored appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "red-retired"}, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Spec.Plan != "retired-tier" {
		t.Fatalf("refused moves left plan %q, want retired-tier", stored.Spec.Plan)
	}
	// Re-sending the holder's own free plan is no change.
	if _, err := svc.SetPlan(ctx, "red-holder", "free"); err != nil {
		t.Fatalf("re-sending the holder's free plan => %v, want admitted", err)
	}
}
