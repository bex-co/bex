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

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
)

// w10/m516 t003: a message sent while a turn runs queues with its model; the
// session keeps running its model until the message runs, then that turn
// runs the new one and the session records it. A model the catalog does not
// offer is refused at send and never queues.
func TestAQueuedMessageSwitchesTheModelWhenItRuns(t *testing.T) {
	enablePlatformCatalog(t)
	e := newQueueEnv(t)
	ctx := context.Background()
	opus := testCatalog[2]
	in := e.input("")
	in.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := e.svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	if e.id = created.ID; e.phase(t, e.id) != PhaseRunning || created.AgentConfig.Model != agentsession.DefaultDevModel {
		t.Fatalf("claude session = %s on %q, want running on the catalog default", e.phase(t, e.id), created.AgentConfig.Model)
	}

	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "too cheap", Model: "anthropic/claude-sonnet-4.5"}); !isCode(err, CodeSessionModelNotOffered) {
		t.Fatalf("queued send with a model outside the catalog = %v, want %s", err, CodeSessionModelNotOffered)
	}
	if got := e.pending(t); len(got) != 0 {
		t.Fatalf("a refused model queued: %v", got)
	}
	view, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "think harder", Model: opus})
	if err != nil {
		t.Fatal(err)
	}
	if view.Queued == nil || view.Queued.Model != opus || view.AgentConfig.Model != agentsession.DefaultDevModel {
		t.Fatalf("queued = %+v running on %q, want the message queued with %s and the session on its model", view.Queued, view.AgentConfig.Model, opus)
	}
	list, err := e.svc.QueuedMessages(caller("alice"), e.id)
	if err != nil || len(list.QueuedMessages) != 1 || list.QueuedMessages[0].Model != opus {
		t.Fatalf("queue read = %+v err=%v", list, err)
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
	if config.Model != opus || e.lc.driverEnv["ANTHROPIC_MODEL"] != opus {
		t.Fatalf("delivered turn: stored %q sandbox %q, want %s", config.Model, e.lc.driverEnv["ANTHROPIC_MODEL"], opus)
	}
}
