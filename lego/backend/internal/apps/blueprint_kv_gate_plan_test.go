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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A sync that omits plan keeps the store's plan (w33/m436): it never newly
// selects a tier, so the paused-tier gate must judge the plan the sync leaves
// the store on — the same value the plan judges as probe.Plan. Judging the
// wanted plan instead (the create default a plan-less file carries) refuses
// every plan-less sync, changed or not, once that default tier is paused,
// while the plan reports update or noop. A create has no store to keep, so
// the default is the new selection there.
func TestBlueprintKeyValueGatePlan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		declare    bool
		specPlan   string
		current    string
		wantJudged string
	}{
		{name: "declared plan is judged", declare: true, specPlan: "standard", current: "starter", wantJudged: "standard"},
		{name: "re-declared own plan is judged", declare: true, specPlan: "standard", current: "standard", wantJudged: "standard"},
		{name: "omitted plan on update keeps the store's", specPlan: "starter", current: "standard", wantJudged: "standard"},
		{name: "omitted plan on update already on the default", specPlan: "starter", current: "starter", wantJudged: "starter"},
		{name: "omitted plan on create selects the default", specPlan: "starter", current: "", wantJudged: "starter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kv := parsedKeyValue{spec: appv1alpha1.KeyValueSpec{Plan: tc.specPlan}}
			if tc.declare {
				kv.fields = map[string]BlueprintField{"plan": {}}
			}
			if got := blueprintKeyValueGatePlan(kv, tc.current); got != tc.wantJudged {
				t.Fatalf("gate judges %q, want %q", got, tc.wantJudged)
			}
		})
	}
}

// The gate's judged plan is the applied plan for every combination of live
// store and declared-or-omitted plan: the agreement the paused-tier refusal
// rests on. Plan-less and planned manifests go through parse, so spec.Plan
// carries the create default exactly as the apply sees it.
func TestBlueprintKeyValueGateJudgesAppliedPlan(t *testing.T) {
	t.Parallel()
	manifests := map[string]string{
		"plan-less": `services:
  - name: cache
    type: keyvalue
    ipAllowList: []
    maxmemoryPolicy: volatile-lru
`,
		"planned": `services:
  - name: cache
    type: keyvalue
    ipAllowList: []
    plan: standard
    maxmemoryPolicy: volatile-lru
`,
	}
	for _, livePlan := range []string{"free", "starter", "standard"} {
		for name, manifest := range manifests {
			t.Run(livePlan+"/"+name, func(t *testing.T) {
				t.Parallel()
				st := parseBlueprintStackForTest(t, manifest)
				if len(st.keyValues) != 1 {
					t.Fatalf("key values = %+v, want one", st.keyValues)
				}
				kv := st.keyValues[0]
				live := appv1alpha1.KeyValueSpec{
					Name: "cache", Plan: livePlan, Public: true,
					IPAllowList: []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.0/24"}},
				}
				probe := *live.DeepCopy()
				ApplyBlueprintKeyValueSpec(&probe, kv.spec, kv.fields)
				if got := blueprintKeyValueGatePlan(kv, live.Plan); got != probe.Plan {
					t.Fatalf("gate judges %q but the sync leaves %q", got, probe.Plan)
				}
			})
		}
	}
}
