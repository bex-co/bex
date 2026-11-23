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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w6/168 against the real store: once resume's start for a person's editor
// settles, the session is completed again and its editor sandbox still gets a
// Git token for the session branch, as a Devin machine's terminal pushes. A
// finished turn's own sandbox, the turn sandbox the editor replaced, the gh
// route and an archived session are refused.
func TestPGGitMintAdmitsACompletedSessionsEditorSandbox(t *testing.T) {
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
	tenant, err := pg.CreateTenant(ctx, "editor-git-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()

	gh := &fakeSessionGitHub{}
	minter := &Minter{
		GitHub: gh, Sessions: pg, Environments: pg, Automations: pg,
		Connections: fakeConnections{row: store.GitConnection{WorkspaceID: tenant.ID, InstallationID: 42, AccountLogin: "octo"}},
	}
	sandbox := func(name string) string { return "sbx-" + name + "-" + tenant.ID }
	mint := func(verb func(*Minter, context.Context, MintRequest) (MintResponse, error), session store.AgentSession, sandbox string) error {
		_, err := verb(minter, ctx, MintRequest{SessionID: session.ID, Workspace: tenant.ID, Repository: "octo/repo",
			Branch: session.Branch, PodName: sandbox + "-0", PodUID: "uid-" + sandbox})
		return err
	}
	refused := func(t *testing.T, when string, err error) {
		t.Helper()
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("%s: err=%v, want forbidden", when, err)
		}
	}
	// finished is a session whose one turn ran in sandbox and completed,
	// the sandbox still up in its idle window.
	finished := func(t *testing.T, sandbox string) store.AgentSession {
		t.Helper()
		created, err := pg.CreateAgentSession(ctx, store.AgentSession{WorkspaceID: tenant.ID, Repo: "octo/repo", InitialPrompt: "t", AgentConfig: sessionConfig(nil)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pg.SetAgentSessionLifecycle(ctx, created.ID, sandbox, "running", "running", false); err != nil {
			t.Fatal(err)
		}
		done, err := pg.SetAgentSessionLifecycle(ctx, created.ID, "", "completed", "completed", false)
		if err != nil {
			t.Fatal(err)
		}
		return done
	}

	turnSandbox, editorSandbox := sandbox("turn"), sandbox("editor")
	session := finished(t, turnSandbox)
	refused(t, "a finished turn's own sandbox", mint((*Minter).Mint, session, turnSandbox))

	// The idle close ends the turn's sandbox; resume starts the editor's.
	if err := pg.ClearAgentSessionSandbox(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.BeginAgentSessionEditorStart(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.BindAgentSessionEditorSandbox(ctx, session.ID, editorSandbox); err != nil {
		t.Fatal(err)
	}
	if err := mint((*Minter).Mint, session, editorSandbox); err != nil {
		t.Fatalf("the checkout's fetch while the start runs: %v", err)
	}
	settled, err := pg.SettleAgentSessionEditorStart(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settled.Phase != "completed" || !settled.EditorSandbox() {
		t.Fatalf("settled = phase %q sandbox %q editor %q, want a completed session holding its editor sandbox", settled.Phase, settled.SandboxID, settled.EditorSandboxID)
	}
	before := gh.calls
	if err := mint((*Minter).Mint, settled, editorSandbox); err != nil {
		t.Fatalf("the completed session's editor sandbox: %v", err)
	}
	if gh.calls != before+1 || gh.installation != 42 || gh.repository != "repo" {
		t.Fatalf("the editor's Git mint reached GitHub as %+v", gh)
	}
	refused(t, "the turn sandbox the editor's replaced", mint((*Minter).Mint, settled, turnSandbox))
	refused(t, "the gh read route in the editor's sandbox", mint((*Minter).MintGitHubRead, settled, editorSandbox))
	refused(t, "the gh write route in the editor's sandbox", mint((*Minter).MintGitHubWrite, settled, editorSandbox))

	// Resume on a finished turn whose sandbox is still up keeps that sandbox
	// for the editor, and then it pushes too.
	keptSandbox := sandbox("kept")
	kept := finished(t, keptSandbox)
	refused(t, "a finished turn's own sandbox", mint((*Minter).Mint, kept, keptSandbox))
	if kept, err = pg.KeepAgentSessionSandboxForEditor(ctx, kept.ID, keptSandbox); err != nil {
		t.Fatal(err)
	}
	if err := mint((*Minter).Mint, kept, keptSandbox); err != nil {
		t.Fatalf("a finished turn's sandbox kept for an editor: %v", err)
	}

	if _, err := pg.SetAgentSessionArchived(ctx, settled.ID, true); err != nil {
		t.Fatal(err)
	}
	refused(t, "the editor's sandbox of an archived session", mint((*Minter).Mint, settled, editorSandbox))
}
