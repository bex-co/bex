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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
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
