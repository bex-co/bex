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

// w33/m436: Render's blueprint-spec § plan (read 2026-11-16) says "Render
// uses 256mb for a new Key Value instance" when a file omits plan, and that
// Render "retains the current compute plan for an existing service". 256mb is
// bex's starter. The fake client applies no CRD defaults, so a store the parse
// leaves plan-less is stored with an empty plan here (the apiserver would make
// it free), which is what these tests catch.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func keyValueBlueprint(kvType, name, plan string) string {
	out := "services:\n  - type: " + kvType + "\n    name: " + name + "\n    ipAllowList: []\n"
	if plan != "" {
		out += "    plan: " + plan + "\n"
	}
	return out
}

func storedKeyValue(t *testing.T, svc *Service, id string) appv1alpha1.KeyValue {
	t.Helper()
	var kv appv1alpha1.KeyValue
	if err := svc.Client.Get(context.Background(), key(id), &kv); err != nil {
		t.Fatalf("get key value %s: %v", id, err)
	}
	return kv
}

func TestAPlanLessBlueprintKeyValueCreatesOnStarter(t *testing.T) {
	for _, tc := range []struct {
		name, kvType, plan, suffix, want string
	}{
		{name: "keyvalue without plan", kvType: "keyvalue", want: "starter"},
		{name: "legacy redis without plan", kvType: "redis", want: "starter"},
		{name: "create_new copy without plan", kvType: "keyvalue", suffix: "ab12", want: "starter"},
		{name: "Render's 256mb", kvType: "keyvalue", plan: "256mb", want: "starter"},
		{name: "explicit free", kvType: "keyvalue", plan: "free", want: "free"},
		{name: "explicit standard", kvType: "redis", plan: "standard", want: "standard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil)
			res, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: keyValueBlueprint(tc.kvType, "cache", tc.plan), resourceSuffix: tc.suffix})
			if err != nil || len(res.KeyValues) != 1 {
				t.Fatalf("DeployStack = %+v, %v", res, err)
			}
			if got := storedKeyValue(t, svc, res.KeyValues[0].ID).Spec.Plan; got != tc.want {
				t.Fatalf("created plan = %q, want %q", got, tc.want)
			}
		})
	}
}

// The four render-examples files whose Key Value omits plan (counted
// 2026-11-16), each entry as its upstream render.yaml writes it. Each validates
// and is priced as a Starter store, not a free one.
func TestRenderExamplesPlanLessKeyValuesArePricedAsStarter(t *testing.T) {
	for repo, entry := range map[string]string{
		"git-scm.com": "  - name: git-scm-redis\n    type: redis\n    ipAllowList: []\n",
		"ioredis":     "  - type: redis\n    name: ioredis-redis\n    ipAllowList: []\n",
		"saleor":      "  - name: saleor-redis\n    type: redis\n    ipAllowList: []\n",
		"sidekiq":     "  - type: redis\n    name: sidekiq-redis\n    region: ohio\n    maxmemoryPolicy: noeviction\n    ipAllowList: []\n",
	} {
		t.Run(repo, func(t *testing.T) {
			manifest := "services:\n" + entry
			if st := parseBlueprintStackForTest(t, manifest); len(st.keyValues) != 1 || st.keyValues[0].spec.Plan != "starter" {
				t.Fatalf("parsed key values = %+v, want one on starter", st.keyValues)
			}
			svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
			v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
			if err != nil || !v.Valid || v.EstimatedPricing == nil {
				t.Fatalf("ValidateBlueprint = %+v, %v", v, err)
			}
			if lines := v.EstimatedPricing.Lines; len(lines) != 1 || lines[0].Tier != "starter" || lines[0].TierLabel != "Starter" || lines[0].MonthlyUSD == "0.00" {
				t.Fatalf("estimate lines = %+v, want one paid Starter line", lines)
			}
		})
	}
}

// Omitting plan on an existing store is not a change: the plan says noop and
// the sync writes nothing, so a free store stays free.
func TestASyncThatOmitsPlanLeavesAFreeKeyValueAlone(t *testing.T) {
	svc, _ := newService(nil)
	ctx := context.Background()
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: keyValueBlueprint("keyvalue", "cache", "free")})
	if err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	before := storedKeyValue(t, svc, res.KeyValues[0].ID)

	planLess := keyValueBlueprint("keyvalue", "cache", "")
	v, err := svc.ValidateBlueprint(ctx, "", planLess, "")
	if err != nil || !v.Valid || v.Plan == nil || len(v.Plan.Actions) != 1 {
		t.Fatalf("ValidateBlueprint = %+v, %v", v, err)
	}
	if action := v.Plan.Actions[0]; action.Operation != BlueprintPlanNoop || len(action.ChangedFields) != 0 {
		t.Fatalf("plan action = %+v, want noop with no changed fields", action)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planLess}); err != nil {
		t.Fatalf("plan-less sync: %v", err)
	}
	after := storedKeyValue(t, svc, res.KeyValues[0].ID)
	if after.Spec.Plan != "free" || after.ResourceVersion != before.ResourceVersion {
		t.Fatalf("plan-less sync left plan %q at resourceVersion %s (was %s), want free and no write", after.Spec.Plan, after.ResourceVersion, before.ResourceVersion)
	}
}

// The workspace's one free Key Value slot binds plan: free, and only that: a
// plan-less store is a paid starter create that never takes the slot. Before
// w33/m436 a plan-less store skipped the check and was then made free by the
// CRD default, a second free store in the workspace.
func TestTheFreeKeyValueLimitBindsExplicitFreeOnly(t *testing.T) {
	svc, _ := newService(nil)
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: keyValueBlueprint("keyvalue", "kv1", "free")}); err != nil {
		t.Fatalf("first free store: %v", err)
	}

	planLess := keyValueBlueprint("keyvalue", "kv2", "")
	if v, err := svc.ValidateBlueprint(ctx, "", planLess, ""); err != nil || !v.Valid {
		t.Fatalf("plan-less store beside a free one: validation %+v, %v", v, err)
	}
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: planLess})
	if err != nil {
		t.Fatalf("plan-less store beside a free one: %v", err)
	}
	if got := storedKeyValue(t, svc, res.KeyValues[0].ID).Spec.Plan; got != "starter" {
		t.Fatalf("plan-less store created on %q, want starter", got)
	}

	explicitFree := keyValueBlueprint("keyvalue", "kv3", "free")
	v, err := svc.ValidateBlueprint(ctx, "", explicitFree, "")
	if err != nil || v.Valid || len(v.Errors) != 1 || !strings.Contains(v.Errors[0].Error, "already has a free Key Value store (kv1)") {
		t.Fatalf("second free store: validation %+v, %v; want the free-instance refusal", v, err)
	}
	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: explicitFree})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.FreeInstanceLimitCode {
		t.Fatalf("second free store deploy = %v, want %s", err, core.FreeInstanceLimitCode)
	}
}

// A plan-less store is paid intent for the payment-method gate, refused before
// anything is written; an explicit free one is not.
func TestAPlanLessBlueprintKeyValueIsPaidIntent(t *testing.T) {
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	_, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: keyValueBlueprint("keyvalue", "cache", "")})
	if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
		t.Fatalf("plan-less Key Value deploy err=%v calls=%v, want the payment gate", err, gate.calls)
	}
	var kvs appv1alpha1.KeyValueList
	if err := cl.List(context.Background(), &kvs); err != nil || len(kvs.Items) != 0 {
		t.Fatalf("refused deploy wrote key values %+v, %v", kvs.Items, err)
	}

	gate.calls = nil
	if _, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: keyValueBlueprint("keyvalue", "cache", "free")}); err != nil {
		t.Fatalf("free Key Value deploy: %v", err)
	}
	if len(gate.calls) != 0 {
		t.Fatalf("free Key Value deploy consulted the payment gate: %v", gate.calls)
	}
}

// Generate used to leave plan out for a free store, because free was the
// default. With the default now starter, that export would re-create it on a
// paid plan, so a free store exports plan: free and round-trips free.
func TestGenerateBlueprintSaysFreeForAFreeKeyValue(t *testing.T) {
	for _, stored := range []string{"free", ""} {
		t.Run("stored "+stored, func(t *testing.T) {
			svc, _ := newService(nil)
			res, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: keyValueBlueprint("keyvalue", "cache", "free")})
			if err != nil {
				t.Fatal(err)
			}
			id := res.KeyValues[0].ID
			if stored == "" {
				// An empty spec plan is the CRD's own default, free.
				kv := storedKeyValue(t, svc, id)
				kv.Spec.Plan = ""
				if err := svc.Client.Update(context.Background(), &kv); err != nil {
					t.Fatal(err)
				}
			}
			out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{KeyValueIDs: []string{id}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.Manifest, "plan: free") {
				t.Fatalf("the export of a free store omits its plan:\n%s", out.Manifest)
			}
			if st := parseBlueprintStackForTest(t, out.Manifest); len(st.keyValues) != 1 || st.keyValues[0].spec.Plan != "free" {
				t.Fatalf("the export re-creates %+v, want one free store", st.keyValues)
			}
		})
	}
}
