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
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR102 D9 against the real store: the in-sandbox spawn creates the child
// through Service.Create as the person who started the parent, so every rule
// of lineage.go applies, and the sandbox holds no credential.

// pod is the gateway-verified identity of a session's current sandbox.
func (e lineageEnv) pod(t *testing.T, sessionID string) agentsession.SpawnEnvelope {
	t.Helper()
	row, err := e.pg.GetAgentSession(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if row.SandboxID == "" {
		e.sandbox(t, sessionID)
		row.SandboxID = "sbx-" + sessionID
	}
	return agentsession.SpawnEnvelope{Kind: agentsession.SpawnKind, Workspace: e.ws, SessionID: sessionID, PodName: row.SandboxID + "-0", PodUID: "uid-" + sessionID}
}

func (e lineageEnv) spawn(t *testing.T, parentID string, content agentsession.SpawnContent) (agentsession.SpawnResponse, error) {
	t.Helper()
	envelope := e.pod(t, parentID)
	envelope.Op, envelope.Spawn = agentsession.SpawnOpCreate, &content
	return e.svc.SpawnSession(context.Background(), envelope)
}

func spawnCode(err error) string {
	var refusal *agentsession.SpawnError
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	return ""
}

func TestSpawnSessionCreatesAChildOfTheCallingSessionAsItsCreator(t *testing.T) {
	e := newLineageEnv(t)
	parent := e.create(t, "")
	row, err := e.pg.GetAgentSession(context.Background(), parent.ID)
	if err != nil || row.CreatedBy != "alice" {
		t.Fatalf("the parent records its creator as %q / %v", row.CreatedBy, err)
	}

	got, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "write the docs", Title: "docs", Tags: []string{"docs"}})
	if err != nil || !got.Created || got.SessionID == "" || got.State != StateWorking {
		t.Fatalf("spawn = %+v / %v", got, err)
	}
	child, err := e.pg.GetAgentSession(context.Background(), got.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentSessionID != parent.ID || child.RootSessionID != parent.ID || child.Depth != 1 {
		t.Fatalf("child lineage = %+v", child)
	}
	if child.CreatedBy != "alice" || child.Repo != row.Repo || child.Title != "docs" {
		t.Fatalf("child = %+v", child)
	}
	if !strings.HasPrefix(child.Branch, store.DefaultAgentBranchPrefix+child.ID) {
		t.Fatalf("the default branch = %q, want a new %s<id>", child.Branch, store.DefaultAgentBranchPrefix)
	}
	var config AgentConfig
	if err := json.Unmarshal(child.AgentConfig, &config); err != nil || config.Agent != "codex" || config.Model != "gpt-5" || config.Task != "write the docs" {
		t.Fatalf("child agent config = %s / %v", child.AgentConfig, err)
	}
	// The child itself can spawn (depth 1 of 3) and is created as the same person.
	if grand, err := e.spawn(t, child.ID, agentsession.SpawnContent{Task: "nested"}); err != nil || !grand.Created {
		t.Fatalf("a child spawning = %+v / %v", grand, err)
	}
}

func TestSpawnSessionParentComesOnlyFromTheVerifiedPod(t *testing.T) {
	e := newLineageEnv(t)
	parent, other := e.create(t, ""), e.create(t, "")
	e.sandbox(t, parent.ID)
	e.sandbox(t, other.ID)
	content := agentsession.SpawnContent{Task: "t"}
	call := func(mutate func(*agentsession.SpawnEnvelope)) error {
		envelope := e.pod(t, parent.ID)
		envelope.Op, envelope.Spawn = agentsession.SpawnOpCreate, &content
		mutate(&envelope)
		_, err := e.svc.SpawnSession(context.Background(), envelope)
		return err
	}
	for name, mutate := range map[string]func(*agentsession.SpawnEnvelope){
		"another session's pod name": func(en *agentsession.SpawnEnvelope) { en.PodName = "sbx-" + other.ID + "-0" },
		"a stale pod":                func(en *agentsession.SpawnEnvelope) { en.PodName = "sbx-old-0" },
		"another workspace":          func(en *agentsession.SpawnEnvelope) { en.Workspace = "tea-elsewhere" },
		"no pod uid":                 func(en *agentsession.SpawnEnvelope) { en.PodUID = "" },
	} {
		if err := call(mutate); spawnCode(err) != agentsession.SpawnForbidden {
			t.Errorf("%s = %v, want forbidden", name, err)
		}
	}
	// A session that is not working has no sandbox to spawn from.
	for _, phase := range []string{PhaseCompleted, PhaseFailed, PhaseCanceled, PhaseCanceling} {
		e.setPhase(t, parent.ID, phase)
		if err := call(func(*agentsession.SpawnEnvelope) {}); spawnCode(err) != agentsession.SpawnForbidden {
			t.Errorf("a %s session spawning = %v, want forbidden", phase, err)
		}
	}
	if kids := e.children(t, parent.ID); len(kids) != 0 {
		t.Fatalf("refused calls created %d children", len(kids))
	}
	if kids := e.children(t, other.ID); len(kids) != 0 {
		t.Fatalf("the other session got %d children", len(kids))
	}
}

func TestSpawnSessionLimitsSurfaceTheirNamedCodes(t *testing.T) {
	e := newLineageEnv(t)
	if err := agentsession.EnablePlatformModelRoute(agentsession.DevModelRouteOpenRouter, "", []string{e.ws}, "sk-platform-test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = agentsession.EnablePlatformModelRoute("", "", nil, "") })
	e.keys.data[workspaceSecretPath(e.ws, "STRIPE_KEY")] = map[string]string{agentSecretValueField: stripeValue}
	in := e.input("")
	in.AgentConfig = AgentConfig{Agent: "claude", Task: "split the work"}
	in.MaxSpendUSD = ptr(5.0)
	in.SecretNames = []string{"STRIPE_KEY"}
	parent, err := e.svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}

	// Spend: more than the parent has left is refused with the parent's remainder.
	_, err = e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", MaxSpendUSD: ptr(6.0)})
	var refusal *agentsession.SpawnError
	if spawnCode(err) != ChildSpendExceedsParentCode || !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "$5") {
		t.Fatalf("a child above the parent's ceiling = %v", err)
	}
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", MaxSpendUSD: ptr(2.0)}); err != nil {
		t.Fatalf("a child within it: %v", err)
	}

	// Secrets: a subset, none, all; never more.
	subset := []string{"STRIPE_KEY"}
	if got, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", SecretNames: &subset}); err != nil {
		t.Fatalf("a subset: %v", err)
	} else if row, _ := e.pg.GetAgentSession(context.Background(), got.SessionID); !slices.Equal(row.SecretNames, subset) {
		t.Fatalf("subset child secrets = %v", row.SecretNames)
	}
	none := []string{}
	if got, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", SecretNames: &none}); err != nil {
		t.Fatalf("an empty list: %v", err)
	} else if row, _ := e.pg.GetAgentSession(context.Background(), got.SessionID); len(row.SecretNames) != 0 {
		t.Fatalf("an empty secretNames gave the child %v", row.SecretNames)
	}
	more := []string{"STRIPE_KEY", "OTHER_KEY"}
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", SecretNames: &more}); spawnCode(err) != ChildSecretNotInheritedCode {
		t.Fatalf("a secret the parent lacks = %v", err)
	}

	// Fan-out: parent has 3 live children now (2.0, subset, none); two more, no sixth.
	for range store.MaxLiveAgentSessionChildren - 3 {
		if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t"}); spawnCode(err) != ChildLimitCode {
		t.Fatalf("a sixth live child = %v", err)
	}

	// Depth: a depth-3 session cannot spawn, and gets no spawn URL.
	solo := e.create(t, "")
	deep := solo
	for range store.MaxAgentSessionDepth {
		deep = e.create(t, deep.ID)
	}
	if _, err := e.spawn(t, deep.ID, agentsession.SpawnContent{Task: "t"}); spawnCode(err) != DepthLimitCode {
		t.Fatalf("a fourth level = %v", err)
	}
	row, _ := e.pg.GetAgentSession(context.Background(), deep.ID)
	if _, present := e.svc.withSpawnEnv(map[string]string{}, row)["BEX_AGENT_SPAWN_URL"]; present {
		t.Fatal("a session at the depth limit was offered a spawn URL")
	}
	row, _ = e.pg.GetAgentSession(context.Background(), parent.ID)
	if _, present := e.svc.withSpawnEnv(map[string]string{}, row)["BEX_AGENT_SPAWN_URL"]; !present {
		t.Fatal("a session that can spawn was not offered a spawn URL")
	}
}

func TestSpawnSessionRepoStaysTheParents(t *testing.T) {
	e := newLineageEnv(t)
	parent := e.create(t, "")
	row, _ := e.pg.GetAgentSession(context.Background(), parent.ID)
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", Repo: strings.ToUpper(row.Repo) + ".git"}); err != nil {
		t.Fatalf("the parent's own repo: %v", err)
	}
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t", Repo: "someone/else"}); spawnCode(err) != ChildRepoMismatchCode {
		t.Fatalf("another repository = %v, want %s", err, ChildRepoMismatchCode)
	}
	// A repo-less parent cannot hand its child a repository.
	in := e.input("")
	in.Repo, in.Branch = "", ""
	chat, err := e.svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.spawn(t, chat.ID, agentsession.SpawnContent{Task: "t", Repo: row.Repo}); spawnCode(err) != ChildRepoMismatchCode {
		t.Fatalf("a repo for a repo-less parent = %v", err)
	}
	if got, err := e.spawn(t, chat.ID, agentsession.SpawnContent{Task: "t"}); err != nil {
		t.Fatalf("a repo-less child: %v", err)
	} else if child, _ := e.pg.GetAgentSession(context.Background(), got.SessionID); child.Repo != "" {
		t.Fatalf("a repo-less parent's child has repo %q", child.Repo)
	}
}

// A retried tool call returns the first child, and a different request under
// the same key is refused; keys of different parents never collide.
func TestSpawnSessionIsIdempotentPerParentAndKey(t *testing.T) {
	e := newLineageEnv(t)
	parent, other := e.create(t, ""), e.create(t, "")
	content := agentsession.SpawnContent{Task: "t", IdempotencyKey: "k1"}
	first, err := e.spawn(t, parent.ID, content)
	if err != nil || !first.Created {
		t.Fatalf("first = %+v / %v", first, err)
	}
	again, err := e.spawn(t, parent.ID, content)
	if err != nil || again.SessionID != first.SessionID || again.Created {
		t.Fatalf("retry = %+v / %v, want the first child with created=false", again, err)
	}
	if n := len(e.children(t, parent.ID, store.ActiveAgentSessionPhases...)); n != 1 {
		t.Fatalf("a retry made %d children", n)
	}
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "different", IdempotencyKey: "k1"}); spawnCode(err) != "AGENT_SESSION_IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("the same key with another task = %v", err)
	}
	// The same caller key under another session is another child.
	other2, err := e.spawn(t, other.ID, content)
	if err != nil || other2.SessionID == first.SessionID || !other2.Created {
		t.Fatalf("the same key under another parent = %+v / %v", other2, err)
	}
	// No key, no dedupe.
	a, _ := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "n"})
	b, _ := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "n"})
	if a.SessionID == b.SessionID {
		t.Fatal("two keyless spawns returned one child")
	}
}

// Authority is the creator's, re-checked on every spawn: a person who lost
// access to the workspace (or cannot operate the parent) cannot keep spawning.
func TestSpawnSessionStopsWhenTheCreatorLostAccess(t *testing.T) {
	e := newLineageEnv(t)
	parent := e.create(t, "")
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t"}); err != nil {
		t.Fatal(err)
	}

	// Lost can_create on the workspace (a viewer, say): alice's other sessions
	// keep running, but this one can no longer start children.
	allow := e.svc.Authz
	e.svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		if strings.TrimPrefix(subject, "user:") == "alice" && relation == core.RelCanCreate {
			return false, nil
		}
		return allow.Check(ctx, subject, relation, object)
	})
	_, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t2"})
	if spawnCode(err) != SpawnCreatorForbiddenCode {
		t.Fatalf("a creator without can_create = %v, want %s", err, SpawnCreatorForbiddenCode)
	}
	// Lost can_operate on the parent.
	e.svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		if strings.TrimPrefix(subject, "user:") == "alice" && relation == core.RelCanOperate && strings.HasPrefix(object, "agent_session:") {
			return false, nil
		}
		return allow.Check(ctx, subject, relation, object)
	})
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t3"}); spawnCode(err) != SpawnCreatorForbiddenCode {
		t.Fatalf("a creator who cannot operate the parent = %v", err)
	}
	if kids := e.children(t, parent.ID); len(kids) != 1 {
		t.Fatalf("refused spawns created children: %d", len(kids))
	}
	e.svc.Authz = allow

	// A session with no recorded creator (older than the column) cannot spawn.
	if _, err := e.pg.Pool.Exec(context.Background(), `UPDATE agent_sessions SET created_by = '' WHERE id = $1`, parent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t4"}); spawnCode(err) != SpawnNoCreatorCode {
		t.Fatalf("a session without a creator = %v, want %s", err, SpawnNoCreatorCode)
	}
}

func TestReadChildSessionIsLimitedToDescendants(t *testing.T) {
	e := newLineageEnv(t)
	parent, outsider := e.create(t, ""), e.create(t, "")
	childResp, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t"})
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := e.spawn(t, parent.ID, agentsession.SpawnContent{Task: "t"})
	if err != nil {
		t.Fatal(err)
	}
	grand, err := e.spawn(t, childResp.SessionID, agentsession.SpawnContent{Task: "t"})
	if err != nil {
		t.Fatal(err)
	}
	read := func(callerID, target string) (agentsession.ChildView, error) {
		envelope := e.pod(t, callerID)
		envelope.Op, envelope.ChildID = agentsession.SpawnOpRead, target
		return e.svc.ReadChildSession(context.Background(), envelope)
	}

	// A finished child's outcome and structured result are readable.
	e.setPhase(t, childResp.SessionID, PhaseCompleted)
	view, err := read(parent.ID, childResp.SessionID)
	if err != nil || view.SessionID != childResp.SessionID || view.State != StateSucceeded || view.ParentSessionID != parent.ID || view.Depth != 1 || len(view.Outcome) == 0 {
		t.Fatalf("read child = %+v / %v", view, err)
	}
	if view, err := read(parent.ID, grand.SessionID); err != nil || view.Depth != 2 {
		t.Fatalf("read grandchild = %+v / %v", view, err)
	}

	// Never a sibling's, a parent's, itself, an outsider, or an unknown id.
	e.sandbox(t, childResp.SessionID)
	for name, tc := range map[string]struct{ caller, target string }{
		"a sibling":                    {childResp.SessionID, sibling.SessionID},
		"the parent":                   {childResp.SessionID, parent.ID},
		"itself":                       {parent.ID, parent.ID},
		"an outsider":                  {parent.ID, outsider.ID},
		"an unknown id":                {parent.ID, "ags-d1abcdefghijklmnopqr"},
		"a grandparent's other branch": {grand.SessionID, sibling.SessionID},
	} {
		if _, err := read(tc.caller, tc.target); spawnCode(err) != agentsession.SpawnNotFound {
			t.Errorf("%s = %v, want not_found", name, err)
		}
	}
	// And an outsider's pod cannot read this tree.
	if _, err := read(outsider.ID, childResp.SessionID); spawnCode(err) != agentsession.SpawnNotFound {
		t.Errorf("an outsider reading a child = %v", err)
	}
	// A pod that is not the session's current sandbox is refused outright.
	envelope := e.pod(t, parent.ID)
	envelope.Op, envelope.ChildID, envelope.PodName = agentsession.SpawnOpRead, childResp.SessionID, "sbx-old-0"
	if _, err := e.svc.ReadChildSession(context.Background(), envelope); spawnCode(err) != agentsession.SpawnForbidden {
		t.Fatalf("a stale pod = %v", err)
	}
}
