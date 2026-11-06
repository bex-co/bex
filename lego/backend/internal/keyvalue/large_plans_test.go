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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// w5/m266: Render's larger Key Value plans create under their size ids or
// legacy names; the xxl tiers share Pro Ultra's pause (user decision D3) and
// are refused by name on create and plan change, while the catalog lists
// them as unavailable.
func TestLargeKeyValuePlans(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService()
	for name, plan := range map[string]string{"by-size": "5g", "by-name": "pro_plus", "three": "3g"} {
		v, err := svc.CreateKeyValue(ctx, CreateKeyValueRequest{Name: name, Plan: plan})
		if err != nil {
			t.Fatalf("create %s: %v", plan, err)
		}
		var kv appv1alpha1.KeyValue
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: v.ID}, &kv); err != nil {
			t.Fatal(err)
		}
		if want := map[string]string{"5g": "pro", "pro_plus": "pro_plus", "3g": "3g"}[plan]; kv.Spec.Plan != want {
			t.Errorf("%s stored as %q, want %q", plan, kv.Spec.Plan, want)
		}
	}
	for _, plan := range []string{"pro_ultra", "40g", "50g"} {
		if _, err := svc.CreateKeyValue(ctx, CreateKeyValueRequest{Name: "paused-" + plan, Plan: plan}); errorCode(err) != InstanceTypeUnavailableCode {
			t.Errorf("create %s => %v, want %s", plan, err, InstanceTypeUnavailableCode)
		}
	}
	seedKeyValue(t, cl, "red-std")
	if _, err := svc.SetPlan(ctx, "red-std", "50g"); errorCode(err) != InstanceTypeUnavailableCode {
		t.Errorf("plan change to 50g => %v, want %s", err, InstanceTypeUnavailableCode)
	}
	if _, err := svc.UpdateKeyValue(ctx, "red-std", KeyValuePatch{Plan: new("40g")}); errorCode(err) != InstanceTypeUnavailableCode {
		t.Errorf("patch to 40g => %v, want %s", err, InstanceTypeUnavailableCode)
	}

	types, err := svc.InstanceTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]KeyValueInstanceType{}
	for _, it := range types {
		byID[it.ID] = it
	}
	if len(types) != 9 || !byID["pro_max"].Available || byID["pro_ultra"].Available || byID["50g"].UnavailableReason == "" {
		t.Fatalf("instance types = %+v", types)
	}
	if byID["pro_plus"].Name != "Pro Plus" || byID["3g"].Name != "3 GB" {
		t.Errorf("names = %q / %q", byID["pro_plus"].Name, byID["3g"].Name)
	}
}
