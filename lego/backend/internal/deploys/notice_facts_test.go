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

package deploys

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestDeployNoticeFactsOnGraphQL (w1/m659): GraphQL Deploy answers the facts
// Render's deploy page Notices read, each absent where it does not apply: a
// cleared build cache, a move to the free plan, the Blueprint and sync that
// opened a sync's deploy (public ids, null for any other deploy and for a
// sync's row from before the record), and the reserved port the deploy's own
// stall or failure reason names. REST keeps Render's deploy object.
func TestDeployNoticeFactsOnGraphQL(t *testing.T) {
	at := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	blueprint, sync := ids.New(ids.Blueprint), ids.New(ids.BlueprintSync)
	reserved := func(port string) string {
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintReservedPort,
			appv1alpha1.DeployReasonParam{Name: "port", Value: "10000"}, appv1alpha1.DeployReasonParam{Name: "reserved", Value: port})
	}
	stall := appv1alpha1.OpenPortProbeStallReason("startup", 0)
	ds := newFakeStore()
	row := func(id, trigger, status string, mutate func(*store.Deploy)) store.Deploy {
		d := store.Deploy{ID: id, AppID: "srv-1", Trigger: trigger, Status: status, CreatedAt: at, UpdatedAt: at}
		if mutate != nil {
			mutate(&d)
		}
		at = at.Add(time.Minute)
		return d
	}
	ds.byApp["srv-1"] = []store.Deploy{
		row("dep-plain", store.TriggerManual, store.DeployDeactivated, nil),
		row("dep-cache", store.TriggerManual, store.DeployDeactivated, func(d *store.Deploy) { d.ClearCache = true }),
		row("dep-sync", store.TriggerBlueprintSync, store.DeployDeactivated, func(d *store.Deploy) {
			d.BlueprintID, d.BlueprintSyncID = blueprint, sync
		}),
		row("dep-old-sync", store.TriggerBlueprintSync, store.DeployDeactivated, nil),
		row("dep-free", store.TriggerSettingsChange, store.DeployDeactivated, func(d *store.Deploy) { d.FreePlanTransition = true }),
		row("dep-failed", store.TriggerManual, store.DeployUpdateFailed, func(d *store.Deploy) {
			d.FailureReason = strings.Replace(stall, appv1alpha1.ProbeStallWaiting, appv1alpha1.ProbeStallFailed, 1) + reserved("18013")
			d.End.TimedOutSeconds = 900
		}),
		row("dep-stalled", store.TriggerManual, store.DeployUpdateInProgress, func(d *store.Deploy) { d.StallReason = stall + reserved("18012") }),
		row("dep-other-stall", store.TriggerManual, store.DeployUpdateInProgress, func(d *store.Deploy) {
			d.StallReason = stall + appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintNoOpenPort, appv1alpha1.DeployReasonParam{Name: "port", Value: "10000"})
		}),
	}
	svc, _ := newService(ds, sampleApp("web", "srv-1"))

	res := graphql.Do(graphql.Params{Schema: testSchema(t, svc), Context: context.Background(), RequestString: `{
		deploys(serviceId: "web") { id clearCache isTransitionToFreeTier blueprintId blueprintSyncId reservedPort timedOutSeconds }
		one: deploy(serviceId: "web", deployId: "dep-sync") { blueprintId blueprintSyncId }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	type facts struct {
		cache, free     bool
		blueprint, sync any
		port, timedOut  any
	}
	want := map[string]facts{
		"dep-plain":       {},
		"dep-cache":       {cache: true},
		"dep-sync":        {blueprint: ids.BlueprintPublicID(blueprint), sync: ids.BlueprintSyncPublicID(sync)},
		"dep-old-sync":    {},
		"dep-free":        {free: true},
		"dep-failed":      {port: 18013, timedOut: 900},
		"dep-stalled":     {port: 18012},
		"dep-other-stall": {},
	}
	data := res.Data.(map[string]any)
	list := data["deploys"].([]any)
	if len(list) != len(want) {
		t.Fatalf("deploys = %d rows, want %d", len(list), len(want))
	}
	for _, d := range list {
		got := d.(map[string]any)
		id := got["id"].(string)
		w := want[id]
		if got["clearCache"] != w.cache || got["isTransitionToFreeTier"] != w.free || got["blueprintId"] != w.blueprint ||
			got["blueprintSyncId"] != w.sync || got["reservedPort"] != w.port || got["timedOutSeconds"] != w.timedOut {
			t.Errorf("%s = %v, want %+v", id, got, w)
		}
	}
	one := data["one"].(map[string]any)
	if one["blueprintId"] != ids.BlueprintPublicID(blueprint) || one["blueprintSyncId"] != ids.BlueprintSyncPublicID(sync) {
		t.Errorf("deploy(dep-sync) = %v, want the public exs- and exe- ids", one)
	}

	do := newRESTHarness(t, svc)
	var raw map[string]any
	if err := json.Unmarshal(do("GET", "/v1/services/web/deploys/dep-sync").Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"clearCache", "isTransitionToFreeTier", "blueprintId", "blueprintSyncId", "reservedPort", "timedOutSeconds"} {
		if _, ok := raw[field]; ok {
			t.Errorf("REST deploy carries %s, which Render's deploy object has not: %v", field, raw)
		}
	}
}
