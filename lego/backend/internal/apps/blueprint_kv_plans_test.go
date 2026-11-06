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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func kvPlanManifest(plan string) string {
	return `
services:
  - name: cache
    type: keyvalue
    ipAllowList: []
    plan: ` + plan + `
`
}

// A Blueprint names Render's larger Key Value plans by size or legacy name; a
// paused one is refused with the API's code unless the store already runs it,
// so re-syncing an existing store's own plan keeps working.
func TestBlueprintKeyValueLargePlans(t *testing.T) {
	if !tiers.Valkey.Paused("pro_ultra") {
		t.Skip("pro_ultra is no longer paused")
	}
	svc, cl := newService(nil)
	ctx := context.Background()
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("10g")})
	if err != nil {
		t.Fatalf("deploy plan 10g: %v", err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil {
		t.Fatal(err)
	}
	if kv.Spec.Plan != "pro_plus" {
		t.Errorf("10g stored as %q, want pro_plus", kv.Spec.Plan)
	}

	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("40g")})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != keyvalue.InstanceTypeUnavailableCode {
		t.Fatalf("upgrade to paused 40g => %v, want 400 %s", err, keyvalue.InstanceTypeUnavailableCode)
	}
	if err := cl.Get(ctx, key(kv.Name), &kv); err != nil {
		t.Fatal(err)
	}
	if kv.Spec.Plan != "pro_plus" {
		t.Errorf("refused upgrade changed the plan to %q", kv.Spec.Plan)
	}

	// A store already on the paused tier may re-declare it, here in the
	// Blueprint schema's spaced spelling.
	kv.Spec.Plan = "pro_ultra"
	if err := cl.Update(ctx, &kv); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("pro ultra")}); err != nil {
		t.Errorf("re-sync of the store's own paused plan => %v, want nil", err)
	}
}
