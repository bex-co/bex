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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeSecretScopes is the agent_secret_scopes table.
type fakeSecretScopes struct {
	rows    map[string]map[string]store.AgentSecretScope // workspace -> name -> row
	listErr error
	upserts int
}

func newFakeSecretScopes() *fakeSecretScopes {
	return &fakeSecretScopes{rows: map[string]map[string]store.AgentSecretScope{}}
}

func (f *fakeSecretScopes) ListAgentSecretScopes(_ context.Context, ws string) ([]store.AgentSecretScope, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []store.AgentSecretScope
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range f.rows[ws] {
			if !yield(k) {
				return
			}
		}
	}) {
		out = append(out, f.rows[ws][name])
	}
	return out, nil
}

func (f *fakeSecretScopes) UpsertAgentSecretScope(_ context.Context, ws string, in store.AgentSecretScope) error {
	if f.rows[ws] == nil {
		f.rows[ws] = map[string]store.AgentSecretScope{}
	}
	f.rows[ws][in.Name] = in
	f.upserts++
	return nil
}

func (f *fakeSecretScopes) DeleteAgentSecretScope(_ context.Context, ws, name string) error {
	delete(f.rows[ws], name)
	return nil
}

func (f *fakeSecretScopes) put(ws, name, scope, repo string) {
	_ = f.UpsertAgentSecretScope(context.Background(), ws, store.AgentSecretScope{Name: name, Scope: scope, Repo: repo})
}

func scopePtr(s string) *string { return &s }

func TestAgentSecretScopesOnEverySurface(t *testing.T) {
	svc, keys := secretService()
	scopes := newFakeSecretScopes()
	svc.SecretScopes = scopes
	// A secret stored before scopes existed has no row: it stays explicit.
	keys.data = map[string]map[string]string{workspaceSecretPath("tea-a", "LEGACY"): {agentSecretValueField: "legacy-value"}}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(caller("admin")))
		return rec
	}

	// A new secret without a scope is workspace-wide (Devin's organization
	// secrets).
	rec := serve(http.MethodPut, "/v1/agent-secrets/ORG_TOKEN?ownerId=tea-a", `{"value":"org-value"}`)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"name":"ORG_TOKEN","scope":"workspace"}`+"\n" {
		t.Fatalf("REST new secret = %d %s", rec.Code, rec.Body.String())
	}
	// A repo scope names its repository; the name is normalized like a
	// session's repo so the two compare equal.
	rec = serve(http.MethodPut, "/v1/agent-secrets/REPO_TOKEN?ownerId=tea-a", `{"value":"repo-value","scope":"repo","repo":"Bex-Co/App.git"}`)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"name":"REPO_TOKEN","scope":"repo","repo":"bex-co/app"}`+"\n" {
		t.Fatalf("REST repo secret = %d %s", rec.Code, rec.Body.String())
	}
	// Rotating the legacy secret without a scope keeps it explicit.
	if out, err := svc.SetAgentSecret(caller("admin"), "tea-a", "LEGACY", AgentSecretInput{Value: "rotated"}); err != nil || out.Scope != store.AgentSecretScopeExplicit {
		t.Fatalf("rotate legacy = %+v %v", out, err)
	}
	// A scope-only update keeps the value.
	if out, err := svc.SetAgentSecret(caller("admin"), "tea-a", "ORG_TOKEN", AgentSecretInput{Scope: scopePtr("explicit")}); err != nil || out.Scope != "explicit" {
		t.Fatalf("rescope = %+v %v", out, err)
	}
	if keys.data[workspaceSecretPath("tea-a", "ORG_TOKEN")][agentSecretValueField] != "org-value" {
		t.Fatal("a scope-only update changed the value")
	}
	// Asking for the scope a secret already has writes nothing.
	before := scopes.upserts
	if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "ORG_TOKEN", AgentSecretInput{Scope: scopePtr("explicit")}); err != nil || scopes.upserts != before {
		t.Fatalf("a no-op rescope = %v, %d writes", err, scopes.upserts-before)
	}

	// GraphQL and MCP read the same scopes, and set them.
	schema := environmentSchema(t, svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString: `mutation { setAgentSecret(ownerId: "tea-a", name: "ORG_TOKEN", scope: "workspace") { name scope repo } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL rescope = %v", result.Errors)
	}
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("dev"),
		RequestString: `{ agentSecrets(ownerId: "tea-a") { secrets { name scope repo } } }`})
	got, _ := json.Marshal(result.Data)
	want := `{"agentSecrets":{"secrets":[{"name":"LEGACY","repo":null,"scope":"explicit"},{"name":"ORG_TOKEN","repo":null,"scope":"workspace"},{"name":"REPO_TOKEN","repo":"bex-co/app","scope":"repo"}]}}`
	if len(result.Errors) != 0 || string(got) != want {
		t.Fatalf("GraphQL list = %s %v", got, result.Errors)
	}
	client := newMCPClient(t, caller("admin"), svc)
	res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "set_agent_secret",
		Arguments: map[string]any{"ownerId": "tea-a", "name": "LEGACY", "scope": "repo", "repo": "bex-co/app"}})
	if err != nil || res.IsError {
		t.Fatalf("MCP rescope = %+v %v", res, err)
	}
	rec = serve(http.MethodGet, "/v1/agent-secrets?ownerId=tea-a", "")
	if !strings.Contains(rec.Body.String(), `{"name":"LEGACY","scope":"repo","repo":"bex-co/app"}`) {
		t.Fatalf("REST list after MCP rescope = %s", rec.Body.String())
	}
	assertNoSecretValue(t, "REST list", rec.Body.String())
	for _, v := range []string{"org-value", "repo-value", "rotated"} {
		if strings.Contains(rec.Body.String(), v) {
			t.Fatalf("REST list leaked a value: %s", rec.Body.String())
		}
	}

	// Delete removes the scope with the secret.
	if err := svc.DeleteAgentSecret(caller("admin"), "tea-a", "REPO_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if _, ok := scopes.rows["tea-a"]["REPO_TOKEN"]; ok {
		t.Fatal("the scope outlived its secret")
	}
	// A new secret of a deleted name starts from the default, not the old row.
	scopes.put("tea-a", "REPO_TOKEN", "repo", "bex-co/old")
	if out, err := svc.SetAgentSecret(caller("admin"), "tea-a", "REPO_TOKEN", AgentSecretInput{Value: "again"}); err != nil || out.Scope != "workspace" || out.Repo != "" {
		t.Fatalf("recreated secret = %+v %v", out, err)
	}
}

func TestAgentSecretScopeRefusals(t *testing.T) {
	svc, _ := secretService()
	svc.SecretScopes = newFakeSecretScopes()
	for _, tc := range []struct {
		name string
		in   AgentSecretInput
		code string
	}{
		{"unknown scope", AgentSecretInput{Value: "v", Scope: scopePtr("team")}, "AGENT_SECRET_SCOPE_INVALID"},
		{"repo scope without a repo", AgentSecretInput{Value: "v", Scope: scopePtr("repo")}, "AGENT_SECRET_SCOPE_INVALID"},
		{"repo scope with a URL", AgentSecretInput{Value: "v", Scope: scopePtr("repo"), Repo: "https://github.com/a/b"}, "AGENT_SECRET_SCOPE_INVALID"},
		{"workspace scope naming a repo", AgentSecretInput{Value: "v", Scope: scopePtr("workspace"), Repo: "a/b"}, "AGENT_SECRET_SCOPE_INVALID"},
		{"a repo without a scope", AgentSecretInput{Value: "v", Repo: "a/b"}, "AGENT_SECRET_SCOPE_INVALID"},
		{"a new secret without a value", AgentSecretInput{Scope: scopePtr("workspace")}, "AGENT_SECRET_VALUE_INVALID"},
	} {
		if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "NEW_ONE", tc.in); codeOf(err) != tc.code {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.code)
		}
	}
	if _, err := svc.SetAgentSecret(caller("dev"), "tea-a", "NEW_ONE", AgentSecretInput{Value: "v", Scope: scopePtr("workspace")}); err == nil {
		t.Fatal("a developer scoped a secret")
	}
}

// The secrets a session receives by scope stay within the session total: the
// workspace-scoped values plus the largest one repository's repo-scoped ones.
func TestScopedSecretsStayWithinTheSessionTotal(t *testing.T) {
	svc, _ := secretService()
	svc.SecretScopes = newFakeSecretScopes()
	big := strings.Repeat("x", maxSecretValueBytes) // 32 KiB; four fill 128 KiB
	for i, name := range []string{"W1", "W2", "W3"} {
		if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", name, AgentSecretInput{Value: big}); err != nil {
			t.Fatalf("workspace secret %d = %v", i, err)
		}
	}
	// Repo-scoped secrets of different repositories never reach one session
	// together, so each repository has its own room.
	for _, repo := range []string{"a/one", "a/two"} {
		if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "R_"+strings.ReplaceAll(repo, "/", "_"), AgentSecretInput{Value: big, Scope: scopePtr("repo"), Repo: repo}); err != nil {
			t.Fatalf("repo secret for %s = %v", repo, err)
		}
	}
	if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "R_MORE", AgentSecretInput{Value: "y", Scope: scopePtr("repo"), Repo: "a/one"}); codeOf(err) != "AGENT_SECRET_LIMIT_EXCEEDED" {
		t.Fatalf("a repository past the total = %v", err)
	}
	if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "W4", AgentSecretInput{Value: "y"}); codeOf(err) != "AGENT_SECRET_LIMIT_EXCEEDED" {
		t.Fatalf("a workspace secret past the total = %v", err)
	}
	// Explicit secrets never count: only sessions that name them receive them.
	if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "E1", AgentSecretInput{Value: big, Scope: scopePtr("explicit")}); err != nil {
		t.Fatalf("explicit secret = %v", err)
	}
}

func scopedSessionFixture(t *testing.T) (*Service, *fakeStore, *fakeLifecycle, *fakeModelKeys, *fakeSecretScopes) {
	t.Helper()
	svc, st, lc, keys := secretSessionFixture(t)
	scopes := newFakeSecretScopes()
	svc.SecretScopes = scopes
	for name, value := range map[string]string{"ORG_TOKEN": "org-value", "REPO_TOKEN": "repo-value", "OTHER_REPO": "other-value", "ONLY_NAMED": "named-value"} {
		keys.data[workspaceSecretPath("tea-a", name)] = map[string]string{agentSecretValueField: value}
	}
	scopes.put("tea-a", "ORG_TOKEN", store.AgentSecretScopeWorkspace, "")
	scopes.put("tea-a", "REPO_TOKEN", store.AgentSecretScopeRepo, "bex-co/example")
	scopes.put("tea-a", "OTHER_REPO", store.AgentSecretScopeRepo, "bex-co/other")
	scopes.put("tea-a", "ONLY_NAMED", store.AgentSecretScopeExplicit, "")
	// A row with no value behind it (deleted since) is skipped, not fatal.
	scopes.put("tea-a", "GONE", store.AgentSecretScopeWorkspace, "")
	// Another workspace's scopes never apply.
	keys.data[workspaceSecretPath("tea-b", "B_TOKEN")] = map[string]string{agentSecretValueField: "b-value"}
	scopes.put("tea-b", "B_TOKEN", store.AgentSecretScopeWorkspace, "")
	return svc, st, lc, keys, scopes
}

func TestDispatchAppliesScopedSecretsWithoutNames(t *testing.T) {
	svc, st, lc, _, _ := scopedSessionFixture(t)
	view, err := svc.Create(caller("alice"), createInput()) // bex-co/example, no secretNames
	if err != nil {
		t.Fatal(err)
	}
	if lc.driverEnv["ORG_TOKEN"] != "org-value" || lc.driverEnv["REPO_TOKEN"] != "repo-value" {
		t.Fatalf("scoped secrets missing from the dispatch env: %v", slices.Sorted(func(yield func(string) bool) {
			for k := range lc.driverEnv {
				if !yield(k) {
					return
				}
			}
		}))
	}
	for _, name := range []string{"OTHER_REPO", "ONLY_NAMED", "GONE", "B_TOKEN", "STRIPE_KEY"} {
		if _, ok := lc.driverEnv[name]; ok {
			t.Errorf("%s reached a session it is not scoped to", name)
		}
	}
	// Every applied value is on the driver's scrub list.
	if lc.driverEnv[secretNamesEnv] != "ORG_TOKEN,REPO_TOKEN" {
		t.Fatalf("%s = %q", secretNamesEnv, lc.driverEnv[secretNamesEnv])
	}
	want := []store.AppliedSecret{{Name: "ORG_TOKEN", Source: "workspace"}, {Name: "REPO_TOKEN", Source: "repo"}}
	if !slices.Equal(st.rows[view.ID].AppliedSecrets, want) {
		t.Fatalf("recorded applied secrets = %v", st.rows[view.ID].AppliedSecrets)
	}
	got, err := svc.Get(caller("alice"), view.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.AppliedSecrets, []AppliedSecret{{Name: "ORG_TOKEN", Source: "workspace"}, {Name: "REPO_TOKEN", Source: "repo"}}) {
		t.Fatalf("view appliedSecrets = %v", got.AppliedSecrets)
	}
	body, _ := json.Marshal(got)
	for _, v := range []string{"org-value", "repo-value"} {
		if strings.Contains(string(body), v) {
			t.Fatalf("the view leaked a value: %s", body)
		}
	}
	if len(got.SecretNames) != 0 {
		t.Fatalf("scoped secrets were frozen into secretNames: %v", got.SecretNames)
	}
}

func TestDispatchSourcesNamedInlineAndScopedSecrets(t *testing.T) {
	svc, st, lc, _, scopes := scopedSessionFixture(t)
	in := createInput()
	in.SecretNames = []string{"ONLY_NAMED", "ORG_TOKEN"}
	in.Secrets = []SecretInput{{Name: "REPO_TOKEN", Value: inlineValue}}
	view, err := svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	// A name the session attached keeps its own source; the inline value
	// wins over the repo-scoped secret of the same name.
	want := []store.AppliedSecret{{Name: "ONLY_NAMED", Source: "explicit"}, {Name: "ORG_TOKEN", Source: "explicit"}, {Name: "REPO_TOKEN", Source: "inline"}}
	if !slices.Equal(st.rows[view.ID].AppliedSecrets, want) {
		t.Fatalf("applied = %v", st.rows[view.ID].AppliedSecrets)
	}
	if lc.driverEnv["REPO_TOKEN"] != inlineValue || lc.driverEnv["ONLY_NAMED"] != "named-value" {
		t.Fatal("named and inline values did not win")
	}

	// A workspace secret added later reaches the session's next turn, and
	// a rescoped one leaves it.
	scopes.put("tea-a", "OTHER_REPO", store.AgentSecretScopeWorkspace, "")
	row := st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: view.ID, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if lc.driverEnv["OTHER_REPO"] != "other-value" {
		t.Fatal("a newly workspace-scoped secret did not reach the next turn")
	}
	if !strings.Contains(lc.driverEnv[secretNamesEnv], "OTHER_REPO") {
		t.Fatalf("the new secret is not scrubbed: %q", lc.driverEnv[secretNamesEnv])
	}
}

func TestScopeReadFailureFailsTheDispatch(t *testing.T) {
	svc, st, lc, _, scopes := scopedSessionFixture(t)
	scopes.listErr = errors.New("postgres down")
	view, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if lc.created != 0 {
		t.Fatal("a sandbox started without knowing which secrets apply")
	}
	if st.rows[view.ID].FailureReason == "" {
		t.Fatal("the dispatch failure was not recorded")
	}
}
