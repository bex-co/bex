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

package agentsessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w10/m527 t003: a message queued with a model that an admin turns off while
// it waits runs the workspace default when it is delivered instead of
// failing, and the session records the model that ran. The list is read from
// the real agent_environments row.
func TestAQueuedMessageOnATurnedOffModelRunsTheDefault(t *testing.T) {
	enablePlatformCatalog(t)
	e := newQueueEnv(t)
	e.svc.Environments = e.pg
	ctx := context.Background()
	if _, err := e.pg.UpsertAgentEnvironment(ctx, e.ws, store.AgentEnvironmentPatch{DefaultModel: &catalogSonnet}); err != nil {
		t.Fatal(err)
	}
	in := e.input("")
	in.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := e.svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	if e.id = created.ID; e.phase(t, e.id) != PhaseRunning || created.AgentConfig.Model != catalogSonnet {
		t.Fatalf("claude session = %s on %q, want running on the workspace default", e.phase(t, e.id), created.AgentConfig.Model)
	}
	view, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "think harder", Model: catalogOpus})
	if err != nil || view.Queued == nil || view.Queued.Model != catalogOpus {
		t.Fatalf("queued = %+v err=%v, want the message queued with %s", view.Queued, err, catalogOpus)
	}

	off := []string{catalogOpus}
	if _, err := e.pg.UpsertAgentEnvironment(ctx, e.ws, store.AgentEnvironmentPatch{DisabledModels: &off}); err != nil {
		t.Fatal(err)
	}
	e.settle(t, e.id)
	if err := e.dispatcher().RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	row, err := e.pg.GetAgentSession(ctx, e.id)
	if err != nil {
		t.Fatal(err)
	}
	var config AgentConfig
	if err := json.Unmarshal(row.AgentConfig, &config); err != nil {
		t.Fatal(err)
	}
	if row.Phase == PhaseFailed || config.Model != catalogSonnet || e.lc.driverEnv["ANTHROPIC_MODEL"] != catalogSonnet {
		t.Fatalf("delivered turn: phase %s stored %q sandbox %q, want it run on %s", row.Phase, config.Model, e.lc.driverEnv["ANTHROPIC_MODEL"], catalogSonnet)
	}
	if got := e.pending(t); len(got) != 0 {
		t.Fatalf("the message is still queued: %v", got)
	}
}

// w10/m568 DoD 4, 6: a list narrowed under migration 0315 converts with its
// offer unchanged, and a model added to the catalog afterwards is off there
// and read as new. Off the platform route the conversion does not run, and
// the row keeps reading as the opt-out. The row is the real one.
func TestALegacyNarrowedListConvertsWithItsOfferUnchanged(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pg := store.NewPGStore(pool)
	ws, err := pg.CreateTenant(ctx, fmt.Sprintf("agent-legacy-model-list-%d", time.Now().UnixNano()), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(ctx, ws.ID) })
	svc := environmentService(ws.ID, pg)
	// What a bex-api before m568 writes: the opt-out alone.
	if _, err := pool.Exec(ctx, `INSERT INTO agent_environments (workspace_id, disabled_models) VALUES ($1, $2)`, ws.ID, []string{catalogOpus}); err != nil {
		t.Fatal(err)
	}
	offer := func(label string, enabled, added []string) {
		t.Helper()
		env, err := svc.Environment(caller("admin"), ws.ID)
		if err != nil || !slices.Equal(env.EnabledModels, enabled) || !slices.Equal(env.NewModels, added) {
			t.Fatalf("%s: enabledModels %v newModels %v err=%v, want %v and %v", label, env.EnabledModels, env.NewModels, err, enabled, added)
		}
	}

	if n, ran, err := ConvertLegacyModelLists(ctx, pg); ran || n != 0 || err != nil {
		t.Fatalf("conversion off the route = %d ran=%v err=%v, want skipped", n, ran, err)
	}
	enablePlatformCatalog(t)
	offer("before the conversion", withoutOpus, []string{})
	if n, ran, err := ConvertLegacyModelLists(ctx, pg); !ran || n < 1 || err != nil {
		t.Fatalf("conversion = %d ran=%v err=%v", n, ran, err)
	}
	offer("after the conversion", withoutOpus, []string{})
	growCatalog(t, catalogAdded)
	offer("after the catalog grew", withoutOpus, []string{catalogAdded})
	if _, _, err := ConvertLegacyModelLists(ctx, pg); err != nil {
		t.Fatal(err)
	}
	offer("after a second conversion", withoutOpus, []string{catalogAdded})
}
