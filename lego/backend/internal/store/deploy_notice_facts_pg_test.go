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

package store

import (
	"context"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w1/m659: a Blueprint sync's deploy row keeps the Blueprint and the sync run
// that opened it, durable keys whichever spelling the caller held, and a
// settings deploy keeps a move to the free plan; every other row keeps
// neither, and every read of the row returns what was written.
func TestPGDeployRowKeepsItsNoticeFacts(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	row, first := deferredRestartApp(t, st)
	if first.BlueprintID != "" || first.BlueprintSyncID != "" || first.FreePlanTransition {
		t.Fatalf("CreateApp's deploy = %+v, want no notice facts", first)
	}
	blueprint, sync := ids.New(ids.Blueprint), ids.New(ids.BlueprintSync)
	synced := WithBlueprintSync(ctx, ids.BlueprintPublicID(blueprint), ids.BlueprintSyncPublicID(sync))

	open := func(ctx context.Context, trigger string, generation int64) Deploy {
		t.Helper()
		d, err := st.CreateDeploy(ctx, row.ID, trigger, "", generation, CommitInfo{}, "")
		if err != nil {
			t.Fatal(err)
		}
		got, err := st.GetDeploy(ctx, row.ID, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.BlueprintID != d.BlueprintID || got.BlueprintSyncID != d.BlueprintSyncID || got.FreePlanTransition != d.FreePlanTransition {
			t.Fatalf("read back %+v, created %+v", got, d)
		}
		return got
	}
	if d := open(synced, TriggerBlueprintSync, first.Generation+1); d.BlueprintID != blueprint || d.BlueprintSyncID != sync || d.FreePlanTransition {
		t.Fatalf("sync deploy = blueprint %q sync %q free %v, want %s %s", d.BlueprintID, d.BlueprintSyncID, d.FreePlanTransition, blueprint, sync)
	}
	if d := open(synced, TriggerManual, first.Generation+2); d.BlueprintID != "" || d.BlueprintSyncID != "" {
		t.Fatalf("a manual deploy under a sync's context recorded blueprint %q sync %q", d.BlueprintID, d.BlueprintSyncID)
	}
	if d := open(ctx, TriggerBlueprintSync, first.Generation+3); d.BlueprintID != "" || d.BlueprintSyncID != "" {
		t.Fatalf("a sync deploy with no sync named recorded blueprint %q sync %q", d.BlueprintID, d.BlueprintSyncID)
	}
	if d := open(WithFreePlanTransition(ctx), TriggerSettingsChange, first.Generation+4); !d.FreePlanTransition || d.BlueprintID != "" {
		t.Fatalf("plan move deploy = %+v, want only the free plan move", d)
	}
	history, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var syncs, moves int
	for _, d := range history {
		if d.BlueprintSyncID == sync {
			syncs++
		}
		if d.FreePlanTransition {
			moves++
		}
	}
	if syncs != 1 || moves != 1 {
		t.Fatalf("history carries %d sync deploys and %d plan moves, want one each", syncs, moves)
	}
}
