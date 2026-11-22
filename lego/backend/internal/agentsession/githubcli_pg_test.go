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

package agentsession

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w10/m561 against the real store: the `gh` mints resolve the binding chain
// live, so an automation edited to turn gh off refuses its running session's
// next mint naming the automation, a mandatory flag saved while a session
// that turned gh on runs refuses its next mint naming the workspace, and a
// child of a session with gh off is refused naming the session.
func TestPGMintFollowsLiveBindingEdits(t *testing.T) {
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
	tenant, err := pg.CreateTenant(ctx, "gh-bindings-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()

	running := func(t *testing.T, in store.AgentSession, sandbox string) store.AgentSession {
		t.Helper()
		in.WorkspaceID, in.Repo, in.InitialPrompt = tenant.ID, "octo/repo", "t"
		created, err := pg.CreateAgentSession(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if created, err = pg.SetAgentSessionLifecycle(ctx, created.ID, sandbox, "running", "running", false); err != nil {
			t.Fatal(err)
		}
		return created
	}
	minter := &Minter{
		GitHub: &fakeSessionGitHub{}, Sessions: pg, Environments: pg, Automations: pg,
		Connections: fakeConnections{row: store.GitConnection{WorkspaceID: tenant.ID, InstallationID: 42, AccountLogin: "octo"}},
	}
	mint := func(session store.AgentSession, sandbox string) error {
		_, err := minter.MintGitHubRead(ctx, MintRequest{SessionID: session.ID, Workspace: tenant.ID, Repository: "octo/repo",
			Branch: session.Branch, PodName: sandbox + "-0", PodUID: "uid-" + sandbox})
		return err
	}
	expectOff := func(t *testing.T, when string, err error, want GitHubCLILevel) {
		t.Helper()
		if level, ok := GitHubCLIOffLevel(err); !ok || level != want {
			t.Fatalf("%s: err=%v level=%q, want gh off at %q", when, err, level, want)
		}
	}

	// An automation's session: on, then refused once the automation is edited.
	automation, err := pg.CreateAgentAutomation(ctx, store.AgentAutomation{
		ID: ids.New(ids.AgentAutomation), WorkspaceID: tenant.ID, Name: "nightly", Enabled: true, CreatedBy: "alice",
		Triggers: json.RawMessage(`[{"type":"schedule","cron":"*/5 * * * *"}]`), Template: automationTemplate(nil),
	})
	if err != nil {
		t.Fatal(err)
	}
	fired := running(t, store.AgentSession{AgentConfig: sessionConfig(nil), AutomationID: automation.ID}, "sbx-auto")
	if err := mint(fired, "sbx-auto"); err != nil {
		t.Fatalf("an automation's session with nothing bound: %v", err)
	}
	automation.Template = automationTemplate(off)
	if automation, err = pg.UpdateAgentAutomation(ctx, automation, automation.Version); err != nil {
		t.Fatal(err)
	}
	expectOff(t, "after the automation turned gh off", mint(fired, "sbx-auto"), GitHubCLIAutomation)
	// Its child follows the automation too.
	child := running(t, store.AgentSession{AgentConfig: sessionConfig(on), ParentSessionID: fired.ID}, "sbx-auto-child")
	expectOff(t, "the child of the automation's session", mint(child, "sbx-auto-child"), GitHubCLIAutomation)
	// Deleting the automation binds nothing: the automations default applies.
	if err := pg.DeleteAgentAutomation(ctx, automation.ID); err != nil {
		t.Fatal(err)
	}
	if err := mint(fired, "sbx-auto"); err != nil {
		t.Fatalf("after the automation was deleted: %v", err)
	}
	if _, err := pg.UpsertAgentEnvironment(ctx, tenant.ID, store.AgentEnvironmentPatch{SetAutomationsGitHubCLI: true, AutomationsGitHubCLI: off}); err != nil {
		t.Fatal(err)
	}
	expectOff(t, "the automations default's off", mint(fired, "sbx-auto"), GitHubCLIAutomationsDefault)

	// A session that turned gh on under a workspace off keeps it until the
	// off is made mandatory.
	pinned := running(t, store.AgentSession{AgentConfig: sessionConfig(on)}, "sbx-pinned")
	yes := true
	if _, err := pg.UpsertAgentEnvironment(ctx, tenant.ID, store.AgentEnvironmentPatch{GitHubCLIDisabled: &yes}); err != nil {
		t.Fatal(err)
	}
	if err := mint(pinned, "sbx-pinned"); err != nil {
		t.Fatalf("a session's on under a workspace off: %v", err)
	}
	if _, err := pg.UpsertAgentEnvironment(ctx, tenant.ID, store.AgentEnvironmentPatch{GitHubCLIMandatory: &yes}); err != nil {
		t.Fatal(err)
	}
	expectOff(t, "after the off was made mandatory", mint(pinned, "sbx-pinned"), GitHubCLIWorkspace)

	// A child of a session with gh off is refused naming the session.
	no := false
	if _, err := pg.UpsertAgentEnvironment(ctx, tenant.ID, store.AgentEnvironmentPatch{GitHubCLIDisabled: &no, GitHubCLIMandatory: &no}); err != nil {
		t.Fatal(err)
	}
	parent := running(t, store.AgentSession{AgentConfig: sessionConfig(off)}, "sbx-parent")
	kid := running(t, store.AgentSession{AgentConfig: sessionConfig(nil), ParentSessionID: parent.ID}, "sbx-kid")
	expectOff(t, "a child of a session with gh off", mint(kid, "sbx-kid"), GitHubCLISession)
	if err := mint(pinned, "sbx-pinned"); err != nil {
		t.Fatalf("with everything back on: %v", err)
	}
}
