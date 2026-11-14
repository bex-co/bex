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
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeRepoEnvs struct {
	rows map[string]store.AgentRepoEnvironment // workspace/repo
}

func newFakeRepoEnvs() *fakeRepoEnvs {
	return &fakeRepoEnvs{rows: map[string]store.AgentRepoEnvironment{}}
}

func (f *fakeRepoEnvs) ListAgentRepoEnvironments(_ context.Context, ws string) ([]store.AgentRepoEnvironment, error) {
	var out []store.AgentRepoEnvironment
	for _, row := range f.rows {
		if row.WorkspaceID == ws {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })
	return out, nil
}

func (f *fakeRepoEnvs) GetAgentRepoEnvironment(_ context.Context, ws, repo string) (store.AgentRepoEnvironment, error) {
	row, ok := f.rows[ws+"/"+repo]
	if !ok {
		return store.AgentRepoEnvironment{}, store.ErrNotFound
	}
	return row, nil
}

func (f *fakeRepoEnvs) UpsertAgentRepoEnvironment(_ context.Context, ws, repo string, setup, checks json.RawMessage, maxRepos int) (store.AgentRepoEnvironment, bool, error) {
	previous, ok := f.rows[ws+"/"+repo]
	if !ok {
		rows, _ := f.ListAgentRepoEnvironments(context.Background(), ws)
		if len(rows) >= maxRepos {
			return store.AgentRepoEnvironment{}, false, store.ErrAgentRepoEnvironmentLimit
		}
	}
	row := store.AgentRepoEnvironment{WorkspaceID: ws, Repo: repo, Setup: setup, Checks: checks, Version: string(setup), UpdatedAt: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)}
	f.rows[ws+"/"+repo] = row
	return row, !ok || previous.Version != row.Version, nil
}

func (f *fakeRepoEnvs) DeleteAgentRepoEnvironment(_ context.Context, ws, repo string) error {
	if _, ok := f.rows[ws+"/"+repo]; !ok {
		return store.ErrNotFound
	}
	delete(f.rows, ws+"/"+repo)
	return nil
}

func TestRepoEnvironmentsOnEverySurface(t *testing.T) {
	svc, _ := secretService()
	envs := newFakeRepoEnvs()
	svc.RepoEnvs = envs
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, path, body, who string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(caller(who)))
		return rec
	}

	body := `{"setup":[{"name":"deps","run":"npm ci"}],"checks":[{"name":"test","run":"npm test"}]}`
	rec := serve(http.MethodPut, "/v1/agent-repo-environments/Acme/API?ownerId=tea-a", body, "admin")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"repo":"acme/api"`) {
		t.Fatalf("REST set = %d %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodPut, "/v1/agent-repo-environments/acme/api?ownerId=tea-a", body, "dev"); rec.Code != http.StatusForbidden {
		t.Fatalf("a developer set an environment: %d", rec.Code)
	}

	// GraphQL and MCP write and read the same rows.
	schema := environmentSchema(t, svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString: `mutation { setAgentRepoEnvironment(ownerId: "tea-a", repo: "acme/web", setup: [], checks: [{name: "lint", run: "npm run lint"}]) { repo checks { name run } } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL set = %v", result.Errors)
	}
	client := newMCPClient(t, caller("admin"), svc)
	res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "set_agent_repo_environment",
		Arguments: map[string]any{"ownerId": "tea-a", "repo": "acme/cli", "setup": []any{map[string]any{"name": "build", "run": "go build ./..."}}, "checks": []any{}}})
	if err != nil || res.IsError {
		t.Fatalf("MCP set = %+v %v", res, err)
	}

	// Developers read (the environment shapes the sessions they start).
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("dev"),
		RequestString: `{ agentRepoEnvironments(ownerId: "tea-a") { environments { repo setup { name run } checks { name } } } }`})
	got, _ := json.Marshal(result.Data)
	want := `{"agentRepoEnvironments":{"environments":[{"checks":[{"name":"test"}],"repo":"acme/api","setup":[{"name":"deps","run":"npm ci"}]},{"checks":[],"repo":"acme/cli","setup":[{"name":"build","run":"go build ./..."}]},{"checks":[{"name":"lint"}],"repo":"acme/web","setup":[]}]}}`
	if len(result.Errors) != 0 || string(got) != want {
		t.Fatalf("GraphQL list = %s %v", got, result.Errors)
	}
	rec = serve(http.MethodGet, "/v1/agent-repo-environments?ownerId=tea-a", "", "dev")
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"repo":`) != 3 {
		t.Fatalf("REST list = %d %s", rec.Code, rec.Body.String())
	}

	// Delete, then a second delete is a named 404 on every surface.
	if rec := serve(http.MethodDelete, "/v1/agent-repo-environments/acme/cli?ownerId=tea-a", "", "admin"); rec.Code != http.StatusNoContent {
		t.Fatalf("REST delete = %d %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodDelete, "/v1/agent-repo-environments/acme/cli?ownerId=tea-a", "", "admin"); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "AGENT_REPO_ENVIRONMENT_NOT_FOUND") {
		t.Fatalf("REST second delete = %d %s", rec.Code, rec.Body.String())
	}
	res, err = client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "delete_agent_repo_environment", Arguments: map[string]any{"ownerId": "tea-a", "repo": "acme/cli"}})
	if err != nil || !res.IsError || !strings.Contains(mcpText(res), "AGENT_REPO_ENVIRONMENT_NOT_FOUND") {
		t.Fatalf("MCP second delete = %+v %v", res, err)
	}
}

func mcpText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func TestRepoEnvironmentRefusals(t *testing.T) {
	svc, _ := secretService()
	svc.RepoEnvs = newFakeRepoEnvs()
	step := func(name, run string) RepoStep { return RepoStep{Name: name, Run: run} }
	many := func(n int) []RepoStep {
		out := make([]RepoStep, n)
		for i := range out {
			out[i] = step("s"+strings.Repeat("x", i), "true")
		}
		return out
	}
	for _, tc := range []struct {
		name string
		repo string
		in   RepoEnvironmentInput
		code string
	}{
		{"not owner/name", "acme", RepoEnvironmentInput{Setup: []RepoStep{step("a", "true")}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"empty", "acme/api", RepoEnvironmentInput{}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"too many steps", "acme/api", RepoEnvironmentInput{Setup: many(maxRepoSetupSteps + 1)}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"too many checks", "acme/api", RepoEnvironmentInput{Checks: many(maxRepoChecks + 1)}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"bad name", "acme/api", RepoEnvironmentInput{Setup: []RepoStep{step("-x", "true")}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"duplicate name", "acme/api", RepoEnvironmentInput{Setup: []RepoStep{step("a", "true"), step("a", "false")}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"empty command", "acme/api", RepoEnvironmentInput{Setup: []RepoStep{step("a", "")}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"long setup", "acme/api", RepoEnvironmentInput{Setup: []RepoStep{step("a", strings.Repeat("x", maxRepoSetupRunBytes+1))}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"long check", "acme/api", RepoEnvironmentInput{Checks: []RepoStep{step("a", strings.Repeat("x", maxRepoCheckRunBytes+1))}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"NUL byte", "acme/api", RepoEnvironmentInput{Setup: []RepoStep{step("a", "a\x00b")}}, "AGENT_REPO_ENVIRONMENT_INVALID"},
		{"past the 64 KiB total", "acme/api", RepoEnvironmentInput{Setup: func() []RepoStep {
			out := make([]RepoStep, 9)
			for i := range out {
				out[i] = step("s"+strings.Repeat("x", i), strings.Repeat("y", maxRepoSetupRunBytes-16))
			}
			return out
		}()}, "AGENT_REPO_ENVIRONMENT_INVALID"},
	} {
		if _, err := svc.SetRepoEnvironment(caller("admin"), "tea-a", tc.repo, tc.in); codeOf(err) != tc.code {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.code)
		}
	}
	ok := RepoEnvironmentInput{Checks: []RepoStep{step("test", "go test ./...")}}
	for i := range maxRepoEnvironments {
		if _, err := svc.SetRepoEnvironment(caller("admin"), "tea-a", "acme/r"+strings.Repeat("x", i), ok); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SetRepoEnvironment(caller("admin"), "tea-a", "acme/one-too-many", ok); codeOf(err) != "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED" {
		t.Fatalf("past the limit = %v", err)
	}
	if _, err := svc.SetRepoEnvironment(caller("admin"), "tea-a", "acme/r", ok); err != nil {
		t.Fatalf("replacing an existing environment at the limit = %v", err)
	}
	svc.RepoEnvs = nil
	if _, err := svc.RepoEnvironments(caller("admin"), "tea-a"); codeOf(err) != "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE" {
		t.Fatalf("no store = %v", err)
	}
}

func TestDispatchCarriesTheSessionRepositorysEnvironment(t *testing.T) {
	svc, _, lc, _ := secretSessionFixture(t)
	envs := newFakeRepoEnvs()
	svc.RepoEnvs = envs
	if _, _, err := envs.UpsertAgentRepoEnvironment(context.Background(), "tea-a", "bex-co/example",
		json.RawMessage(`[{"name":"deps","run":"npm ci"}]`), json.RawMessage(`[{"name":"test","run":"npm test"}]`), 50); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	var got RepoEnvironmentInput
	if err := json.Unmarshal([]byte(lc.driverEnv[repoEnvironmentEnv]), &got); err != nil {
		t.Fatalf("%s = %q: %v", repoEnvironmentEnv, lc.driverEnv[repoEnvironmentEnv], err)
	}
	if len(got.Setup) != 1 || got.Setup[0].Run != "npm ci" || len(got.Checks) != 1 || got.Checks[0].Name != "test" {
		t.Fatalf("delivered environment = %+v", got)
	}
	// A session on another repository, or none, gets no environment.
	in := createInput()
	in.Repo = "bex-co/other"
	if _, err := svc.Create(caller("alice"), in); err != nil {
		t.Fatal(err)
	}
	if v, ok := lc.driverEnv[repoEnvironmentEnv]; ok {
		t.Fatalf("another repository received %q", v)
	}
}

func TestOnlyTheSetupFailureCodeIsBelievedFromTheDriver(t *testing.T) {
	c, st, _, id := completerFixture(`{"state":"failed","error":"setup step \"deps\" exited with code 7","code":"AGENT_SESSION_SETUP_FAILED"}`, nil)
	c.Reconcile(context.Background())
	if row := st.rows[id]; row.Phase != PhaseFailed || row.FailureReasonCode != FailureCodeSetupFailed || !strings.Contains(row.FailureReason, `"deps" exited with code 7`) {
		t.Fatalf("row = %+v", row)
	}
	c, st, _, id = completerFixture(`{"state":"failed","error":"boom","code":"STRUCTURED_OUTPUT_MISSING"}`, nil)
	c.Reconcile(context.Background())
	if row := st.rows[id]; row.Phase != PhaseFailed || row.FailureReasonCode != "" {
		t.Fatalf("a code the driver may not set was believed: %+v", row)
	}
}
