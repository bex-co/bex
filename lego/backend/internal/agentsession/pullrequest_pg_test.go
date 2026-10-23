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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGPullRequestVerbRecordsThePRMidTurn drives the bex-api PR verb against
// the real store: create then two updates reuse one PR, its body carries the
// footer exactly once, and the session's REST/GraphQL/MCP source row shows the
// PR while the turn is still running.
func TestPGPullRequestVerbRecordsThePRMidTurn(t *testing.T) {
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
	tenant, err := pg.CreateTenant(ctx, "pr-verb-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()
	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "octo/repo", Branch: "bex-agent/task-1",
		AgentConfig: []byte(`{"agent":"claude","task":"add retries"}`), InitialPrompt: "add retries",
	})
	if err != nil {
		t.Fatal(err)
	}
	if session, err = pg.SetAgentSessionLifecycle(ctx, session.ID, "sbx-1", "running", "running", false); err != nil {
		t.Fatal(err)
	}

	forge := &fakeForge{}
	p := &PullRequester{
		GitHub: forge, Sessions: pg, Audit: pg,
		Connections:  fakeConnections{row: store.GitConnection{WorkspaceID: tenant.ID, InstallationID: 42, AccountLogin: "octo"}},
		DashboardURL: "https://dashboard.bex.co",
	}
	request := func(title, body string) PullRequestRequest {
		return PullRequestRequest{
			MintRequest: MintRequest{
				SessionID: session.ID, Workspace: tenant.ID, Repository: "octo/repo",
				Branch: "bex-agent/task-1", PodName: "sbx-1-0", PodUID: "uid-1",
			},
			PullRequestContent: PullRequestContent{Title: title, Body: body},
		}
	}

	created, err := p.Sync(ctx, request("Add retries", "## Summary\nfirst"))
	if err != nil {
		t.Fatal(err)
	}
	row, err := pg.GetAgentSession(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Phase != "running" || row.PRURL != created.URL || row.PRNumber != created.Number || row.HeadSHA != created.HeadSHA {
		t.Fatalf("mid-turn session row = %+v, want the PR %+v while running", row, created)
	}
	for _, body := range []string{"## Summary\nsecond", forge.bodies[created.Number] + "\nthird"} {
		updated, err := p.Sync(ctx, request("Add retries", body))
		if err != nil {
			t.Fatal(err)
		}
		if updated.Created || updated.Number != created.Number {
			t.Fatalf("update = %+v, want PR #%d reused", updated, created.Number)
		}
	}
	final := forge.bodies[created.Number]
	if strings.Count(final, PullRequestFooterMarker) != 1 || !strings.HasSuffix(final, "[view session](https://dashboard.bex.co/agents/"+session.ID+")") {
		t.Fatalf("final body = %q", final)
	}

	// Once the turn is terminal the verb refuses and the record is not touched.
	if _, _, err := pg.FinalizeAgentSession(ctx, session.ID, "completed", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Sync(ctx, request("Late", "late")); err == nil {
		t.Fatal("a terminal session's sandbox opened a PR")
	}
	if row, _ := pg.GetAgentSession(ctx, session.ID); row.PRNumber != created.Number || row.Phase != "completed" {
		t.Fatalf("finalized row = %+v", row)
	}
}
