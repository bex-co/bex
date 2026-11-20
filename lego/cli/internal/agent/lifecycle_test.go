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

package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// routeFake answers the routes a test registers and records every request.
type routeFake struct {
	t     *testing.T
	srv   *httptest.Server
	mu    sync.Mutex
	calls []capturedCall
}

func newRouteFake(t *testing.T, routes map[string]http.HandlerFunc) *routeFake {
	f := &routeFake{t: t}
	mux := http.NewServeMux()
	for pattern, handler := range routes {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			f.mu.Lock()
			f.calls = append(f.calls, capturedCall{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})
			f.mu.Unlock()
			r.Body = io.NopCloser(bytes.NewReader(raw))
			handler(w, r)
		})
	}
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *routeFake) deps() deps { return serverDeps(f.srv) }

func (f *routeFake) callsTo(method string) []capturedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []capturedCall
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c)
		}
	}
	return out
}

func (f *routeFake) only(t *testing.T) capturedCall {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 1 {
		t.Fatalf("%d requests, want 1: %+v", len(f.calls), f.calls)
	}
	return f.calls[0]
}

func answer(status int, v any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, status, v) }
}

func sessionJSON(id, state string) map[string]any {
	return map[string]any{"id": id, "state": state, "title": "Fix the crash", "agentConfig": map[string]any{"agent": "claude"}}
}

func TestSessionURLsAreAcceptedAsIDs(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{"GET /v1/agent-sessions/{id}": func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateSucceeded))
	}})
	if _, stderr, code := run(t, f.deps(), "get", "https://dashboard.example.com/agents/ags-abc123?tab=files"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if got := f.only(t).path; got != "/v1/agent-sessions/ags-abc123" {
		t.Fatalf("GET %s", got)
	}
	for _, bad := range []string{"https://dashboard.bex.co/services/srv-1", "srv-abc", "ags-", "ags-one ags-two", "ags-../x", "https://dashboard.bex.co/agents/ags-a%20b"} {
		if _, stderr, code := run(t, f.deps(), "get", bad); code != exitUsage {
			t.Errorf("%q: exit %d (%s), want 2", bad, code, stderr)
		}
	}
}

func TestListFiltersMapToTheAPIQuery(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-sessions":                 answer(http.StatusOK, []any{}),
		"GET /v1/agent-automations/nightly-deps": answer(http.StatusOK, map[string]any{"id": "aut-resolved1", "name": "nightly-deps"}),
	})
	_, stderr, code := run(t, f.deps(), "list", "--mine", "--created-by", "usr-a", "--created-by", "usr-b",
		"--automation", "nightly-deps", "--parent", "https://dashboard.bex.co/agents/ags-parent1",
		"--phase", "running", "--since", "2026-11-01T00:00:00Z", "--until", "2026-11-02T00:00:00+01:00")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	gets := f.callsTo(http.MethodGet)
	got, _ := url.ParseQuery(gets[len(gets)-1].query)
	want := url.Values{
		"ownerId": {"tea-ws"}, "limit": {"20"}, "mine": {"true"}, "createdBy": {"usr-a", "usr-b"},
		"automationId": {"aut-resolved1"}, "parentSessionId": {"ags-parent1"}, "phase": {"running"},
		"createdAfter": {"2026-11-01T00:00:00Z"}, "createdBefore": {"2026-11-01T23:00:00Z"},
	}
	if got.Encode() != want.Encode() {
		t.Fatalf("query\n got %s\nwant %s", got.Encode(), want.Encode())
	}
	if _, stderr, code := run(t, f.deps(), "list", "--since", "yesterday"); code != exitUsage || !strings.Contains(stderr, "is not a time") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestParseTimeOrAgeTakesUpstreamsSyntax(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"7d":                   now.AddDate(0, 0, -7),
		"90m":                  now.Add(-90 * time.Minute),
		"2026-11-01T08:00:00Z": time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC),
	} {
		got, err := parseTimeOrAge(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("%s: got %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"-3d", "soon", "2026-11-01"} {
		if _, err := parseTimeOrAge(bad, now); err == nil || exitCode(err) != exitUsage {
			t.Errorf("%q: %v, want a usage error", bad, err)
		}
	}
}

func TestRunParentSendsParentSessionID(t *testing.T) {
	f := newFakeAPI(t)
	d := f.deps(t).deps
	if _, stderr, code := run(t, d, "run", "--parent", "https://dashboard.bex.co/agents/ags-parent1", "Split the work"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var body map[string]any
	_ = json.Unmarshal(f.creates[0].body, &body)
	if body["parentSessionId"] != "ags-parent1" {
		t.Fatalf("create body %v", body)
	}
}

func TestArchiveReportsEveryOutcome(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{"POST /v1/agent-sessions/{id}/archive": func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "ags-missing" {
			writeAPIError(w, http.StatusNotFound, "AGENT_SESSION_NOT_FOUND", "agent session not found")
			return
		}
		writeJSON(w, http.StatusOK, sessionJSON(r.PathValue("id"), stateSucceeded))
	}})
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "archive", "ags-one", "ags-missing", "ags-two")
	if code != 1 || !strings.Contains(stdout, "Archived ags-one") || !strings.Contains(stdout, "Archived ags-two") ||
		!strings.Contains(stderr, "could not archive 1 of 3") || !strings.Contains(stderr, "ags-missing") {
		t.Fatalf("exit %d\nstdout %s\nstderr %s", code, stdout, stderr)
	}
	stdout, _, _ = run(t, f.deps(), "-o", "json", "archive", "ags-one")
	var out []map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil || len(out) != 1 || out[0]["id"] != "ags-one" {
		t.Fatalf("-o json is not the array of answers: %v\n%s", err, stdout)
	}
}

func TestDeleteTagsTreeCost(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"DELETE /v1/agent-sessions/{id}": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
		"PUT /v1/agent-sessions/{id}/tags": func(w http.ResponseWriter, r *http.Request) {
			var body map[string][]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			v := sessionJSON(r.PathValue("id"), stateSucceeded)
			v["tags"] = body["tags"]
			writeJSON(w, http.StatusOK, v)
		},
		"GET /v1/agent-sessions/{id}/tree": answer(http.StatusOK, map[string]any{"rootSessionId": "ags-root", "sessions": []any{
			map[string]any{"id": "ags-root", "state": "working", "title": "Plan", "depth": 0},
			map[string]any{"id": "ags-child", "state": "succeeded", "title": "Do part one", "depth": 1, "parentSessionId": "ags-root",
				"outcome": map[string]any{"cost": map[string]any{"totalUSD": "0.12"}}},
		}}),
		"GET /v1/agent-sessions/{id}/model-cost": answer(http.StatusOK, map[string]any{"sessionId": "ags-root", "modelSource": "platform",
			"costUsd": "1.56", "requests": 12, "turns": []any{map[string]any{"turn": 1, "costUsd": "1.56", "requests": 12,
				"inputTokens": 1000, "outputTokens": 200, "models": []string{"claude-sonnet-5-5"}}}}),
	})
	d := f.deps()
	if _, stderr, code := run(t, d, "delete", "ags-root"); code != exitUsage || !strings.Contains(stderr, "pass --confirm") {
		t.Fatalf("delete without --confirm: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "delete", "ags-root", "--confirm"); code != 0 {
		t.Fatalf("delete: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "tags", "ags-root", "ci", "team:payments"); code != 0 {
		t.Fatalf("tags: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "tags", "ags-root", "--clear"); code != 0 {
		t.Fatalf("tags --clear: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "tags", "ags-root"); code != exitUsage {
		t.Fatalf("tags with nothing: exit %d: %s", code, stderr)
	}
	var puts []string
	for _, c := range f.callsTo(http.MethodPut) {
		puts = append(puts, asJSON(t, c.body))
	}
	if strings.Join(puts, " ") != `{"tags":["ci","team:payments"]} {"tags":[]}` {
		t.Fatalf("PUT bodies %v", puts)
	}
	stdout, _, _ := run(t, d, "-o", "text", "tree", "ags-child")
	if !strings.Contains(stdout, "ags-root · working · Plan") || !strings.Contains(stdout, "  ags-child · succeeded · $0.12 · Do part one  ←") {
		t.Fatalf("tree:\n%s", stdout)
	}
	stdout, _, _ = run(t, d, "-o", "text", "cost", "ags-root")
	if !strings.Contains(stdout, "ags-root · $1.56 · 12 request(s) · platform model") || !strings.Contains(stdout, "claude-sonnet-5-5") {
		t.Fatalf("cost:\n%s", stdout)
	}
}

func TestSecretValuesNeverComeFromArguments(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"PUT /v1/agent-secrets/{name}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"name": r.PathValue("name")})
		},
	})
	d := f.deps()
	d.stdin = strings.NewReader("s3cret-value\n")
	stdout, stderr, code := run(t, d, "-o", "text", "secrets", "set", "SENTRY_TOKEN")
	if code != 0 || !strings.Contains(stdout, "Set agent secret SENTRY_TOKEN.") {
		t.Fatalf("exit %d: %s %s", code, stdout, stderr)
	}
	if got := f.only(t); got.body["value"] != "s3cret-value" || got.query != "ownerId=tea-ws" {
		t.Fatalf("PUT %+v: want the stdin value without its newline", got)
	}
	if strings.Contains(stdout+stderr, "s3cret-value") {
		t.Fatal("the value was echoed")
	}
	d.stdin = strings.NewReader("")
	d.lookupEnv = func(name string) (string, bool) { return map[string]string{"TOKEN": "from-env"}[name], name == "TOKEN" }
	if _, stderr, code := run(t, d, "secrets", "set", "X", "--from-env", "TOKEN"); code != 0 {
		t.Fatalf("--from-env: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "secrets", "set", "X"); code != exitUsage || !strings.Contains(stderr, "is empty") {
		t.Fatalf("empty value: exit %d: %s", code, stderr)
	}
	if _, stderr, code := run(t, d, "secrets", "set", "X", "--from-env", "MISSING"); code != exitUsage {
		t.Fatalf("missing env: exit %d: %s", code, stderr)
	}
}

// Scopes (ADR098 D9): set sends --scope/--repo with the value, rescope sends
// only the scope (the value is kept), and list prints each secret's scope.
func TestSecretScopes(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"PUT /v1/agent-secrets/{name}": answer(http.StatusOK, map[string]any{"name": "APP_TOKEN", "scope": "repo", "repo": "bex-co/app"}),
		"GET /v1/agent-secrets": answer(http.StatusOK, map[string]any{"ownerId": "tea-ws", "secrets": []any{
			map[string]any{"name": "APP_TOKEN", "scope": "repo", "repo": "bex-co/app"},
			map[string]any{"name": "OLD", "scope": "explicit"},
			map[string]any{"name": "ORG", "scope": "workspace"},
		}}),
	})
	d := f.deps()
	d.stdin = strings.NewReader("v\n")
	if _, stderr, code := run(t, d, "secrets", "set", "APP_TOKEN", "--scope", "repo", "--repo", "bex-co/app"); code != 0 {
		t.Fatalf("set --scope: exit %d: %s", code, stderr)
	}
	if got := f.only(t); got.body["value"] != "v" || got.body["scope"] != "repo" || got.body["repo"] != "bex-co/app" {
		t.Fatalf("set body = %+v", got.body)
	}
	stdout, stderr, code := run(t, d, "-o", "text", "secrets", "rescope", "APP_TOKEN", "--scope", "repo", "--repo", "bex-co/app")
	if code != 0 || !strings.Contains(stdout, "Agent secret APP_TOKEN is now repo:bex-co/app.") {
		t.Fatalf("rescope: exit %d: %s %s", code, stdout, stderr)
	}
	puts := f.callsTo(http.MethodPut)
	if got := puts[len(puts)-1]; got.body["scope"] != "repo" || got.body["repo"] != "bex-co/app" {
		t.Fatalf("rescope body = %+v", got.body)
	} else if _, ok := got.body["value"]; ok {
		t.Fatalf("rescope sent a value: %+v", got.body)
	}
	if _, stderr, code := run(t, d, "secrets", "rescope", "APP_TOKEN"); code != exitUsage || !strings.Contains(stderr, "--scope is required") {
		t.Fatalf("rescope without --scope: exit %d: %s", code, stderr)
	}
	stdout, _, code = run(t, d, "-o", "text", "secrets", "list")
	if code != 0 || stdout != "APP_TOKEN\trepo:bex-co/app\nOLD\texplicit\nORG\tworkspace\n" {
		t.Fatalf("list: exit %d: %q", code, stdout)
	}
}

// TestNoFlagCarriesASecretValue walks every `bex agent` command: a value on
// the command line lands in shell history and the process list (ADR098).
func TestNoFlagCarriesASecretValue(t *testing.T) {
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		c.Flags().VisitAll(func(f *pflag.Flag) {
			name := f.Name
			if name == "value" || strings.HasSuffix(name, "-value") || name == "model-key" || name == "api-key" || name == "token" {
				t.Errorf("%s --%s would take a secret on the command line", c.CommandPath(), name)
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(newCommand(stubDeps(nil)))
}

func TestSkillsCommands(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"POST /v1/agent-skills":                   answer(http.StatusCreated, map[string]any{"name": "triage", "version": 1}),
		"PATCH /v1/agent-skills/{ref}":            answer(http.StatusOK, map[string]any{"name": "triage", "version": 2}),
		"GET /v1/agent-skills/{ref}/versions/{n}": answer(http.StatusOK, map[string]any{"name": "triage", "content": "---\nname: triage\n---\nv1 body\n"}),
		"GET /v1/agent-skills/{ref}/versions": answer(http.StatusOK, map[string]any{"versions": []any{
			map[string]any{"version": 2, "shortId": "abc1234", "message": "Update skills/triage/SKILL.md", "createdBy": "usr-me", "createdAt": "2026-11-12T08:00:00Z", "current": true},
		}}),
	})
	d := f.deps()
	skill := filepath.Join(t.TempDir(), "SKILL.md")
	writeFile(t, skill, "---\nname: triage\ndescription: Triage errors\n---\nDo it.\n")
	stdout, stderr, code := run(t, d, "-o", "text", "skills", "create", "-f", skill)
	if code != 0 || !strings.Contains(stdout, "Saved skill triage, version 1.") {
		t.Fatalf("create: exit %d: %s %s", code, stdout, stderr)
	}
	if _, stderr, code := run(t, d, "skills", "update", "triage", "-f", skill); code != 0 {
		t.Fatalf("update: exit %d: %s", code, stderr)
	}
	for _, c := range f.calls[:2] {
		if c.body["content"] != "---\nname: triage\ndescription: Triage errors\n---\nDo it.\n" {
			t.Fatalf("%s body %v: want the file as content", c.method, c.body)
		}
	}
	stdout, _, _ = run(t, d, "-o", "text", "skills", "get", "triage", "--version", "1")
	if stdout != "---\nname: triage\n---\nv1 body\n" {
		t.Fatalf("get --version printed %q", stdout)
	}
	stdout, _, _ = run(t, d, "-o", "text", "skills", "versions", "triage")
	if !strings.Contains(stdout, "2 (current)") || !strings.Contains(stdout, "abc1234") {
		t.Fatalf("versions:\n%s", stdout)
	}
	if _, stderr, code := run(t, d, "skills", "create"); code != exitUsage || !strings.Contains(stderr, "-f FILE") {
		t.Fatalf("create without -f: exit %d: %s", code, stderr)
	}
}

func TestEnvSet(t *testing.T) {
	env := map[string]any{"networkTier": "standard", "modelSource": "platform", "agentModelMonthlySpendCapUsd": 50.0, "agentModelSpendMonthToDateUsd": 3.25,
		"enabledAgents": []any{"claude", "codex"}, "agentDataUse": []any{map[string]any{"agent": "muse", "acknowledgedAt": nil, "acknowledgedBy": nil}},
		"defaultTemplate": "agent-8c-16g", "templates": []any{
			map[string]any{"name": "agent", "cpu": 2, "memoryGb": 4},
			map[string]any{"name": "agent-8c-16g", "cpu": 8, "memoryGb": 16},
		}}
	f := newRouteFake(t, map[string]http.HandlerFunc{
		"GET /v1/agent-environment":   answer(http.StatusOK, env),
		"PATCH /v1/agent-environment": answer(http.StatusOK, env),
	})
	d := f.deps()
	stdout, _, code := run(t, d, "-o", "text", "env")
	if code != 0 || !strings.Contains(stdout, "$50.00 a month, $3.25 used this month") || !strings.Contains(stdout, "claude, codex") ||
		!strings.Contains(stdout, "needs an admin's --acknowledge-data-use to turn on") ||
		!regexp.MustCompile(`(?m)^Default sandbox\s+agent-8c-16g \(8 CPU, 16 GB\)$`).MatchString(stdout) ||
		// A bex-api that predates githubCliEnabled cannot turn gh off (w10/m524).
		!regexp.MustCompile(`(?m)^GitHub CLI\s+on$`).MatchString(stdout) {
		t.Fatalf("env:\n%s", stdout)
	}
	// Each body is compared whole, so githubCliEnabled is sent only when
	// --github-cli is given.
	cases := []struct {
		args []string
		body string
	}{
		{[]string{"--spend-cap", "default"}, `{"resetAgentModelMonthlySpendCap":true}`},
		{[]string{"--spend-cap", "120"}, `{"agentModelMonthlySpendCapUsd":120}`},
		{[]string{"--no-custom-hosts", "--default-model", ""}, `{"customHosts":[],"defaultModel":""}`},
		{[]string{"--clear-model-key"}, `{"clearModelApiKey":true}`},
		{[]string{"--agents", "claude,muse", "--acknowledge-data-use", "muse"}, `{"acknowledgeAgentDataUse":["muse"],"enabledAgents":["claude","muse"]}`},
		{[]string{"--agents", ""}, `{"enabledAgents":[]}`},
		{[]string{"--default-template", "agent-4c-8g"}, `{"defaultTemplate":"agent-4c-8g"}`},
		{[]string{"--default-template", ""}, `{"defaultTemplate":""}`},
		{[]string{"--github-cli", "off"}, `{"githubCliEnabled":false}`},
		{[]string{"--github-cli", "on", "--network-tier", "full"}, `{"githubCliEnabled":true,"networkTier":"full"}`},
		{[]string{"--models", "deepseek/deepseek-v4.1-flash-1017,anthropic/claude-haiku-5.5"}, `{"enabledModels":["deepseek/deepseek-v4.1-flash-1017","anthropic/claude-haiku-5.5"]}`},
	}
	for _, tc := range cases {
		f.calls = nil
		if _, stderr, code := run(t, d, append([]string{"env", "set"}, tc.args...)...); code != 0 {
			t.Fatalf("%v: exit %d: %s", tc.args, code, stderr)
		}
		if got := asJSON(t, f.only(t).body); got != tc.body {
			t.Errorf("%v: body %s, want %s", tc.args, got, tc.body)
		}
	}
	f.calls = nil
	d.stdin = strings.NewReader("sk-ant-key\n")
	stdout, stderr, code := run(t, d, "env", "set", "--model-key-stdin")
	if code != 0 || f.only(t).body["modelApiKey"] != "sk-ant-key" || strings.Contains(stdout+stderr, "sk-ant-key") {
		t.Fatalf("--model-key-stdin: exit %d, body %v, output %q", code, f.only(t).body, stdout+stderr)
	}
	f.calls = nil
	if _, stderr, code := run(t, d, "env", "set", "--models", "", "--network-tier", "full"); code != exitUsage || !strings.Contains(stderr, "the catalog's default always stays on") || len(f.calls) != 0 {
		t.Errorf("--models \"\": exit %d, %d requests, stderr %q; want a usage error and nothing sent", code, len(f.calls), stderr)
	}
	for _, bad := range [][]string{{}, {"--spend-cap", "lots"}, {"--model-key-stdin", "--clear-model-key"}} {
		if _, stderr, code := run(t, d, append([]string{"env", "set"}, bad...)...); code != exitUsage {
			t.Errorf("%v: exit %d (%s), want 2", bad, code, stderr)
		}
	}
	for _, bad := range []string{"yes", "ON", "false", ""} {
		f.calls = nil
		_, stderr, code := run(t, d, "env", "set", "--github-cli", bad, "--network-tier", "full")
		if code != exitUsage || !strings.Contains(stderr, "--github-cli takes on or off") || len(f.calls) != 0 {
			t.Errorf("--github-cli %q: exit %d, %d requests, stderr %q; want a usage error naming on and off and nothing sent", bad, code, len(f.calls), stderr)
		}
	}
}

// TestEnvShowsGitHubCLI: `bex agent env` words the setting, and -o json
// prints the API's githubCliEnabled as it came (w10/m524).
func TestEnvShowsGitHubCLI(t *testing.T) {
	for _, tc := range []struct {
		enabled bool
		line    string
	}{
		{true, `(?m)^GitHub CLI\s+on$`},
		{false, `(?m)^GitHub CLI\s+off \(sessions get no gh; Git pushes and the bex tools still work\)$`},
	} {
		f := newRouteFake(t, map[string]http.HandlerFunc{"GET /v1/agent-environment": answer(http.StatusOK,
			map[string]any{"networkTier": "registries", "modelSource": "platform", "githubCliEnabled": tc.enabled})})
		stdout, stderr, code := run(t, f.deps(), "-o", "text", "env")
		if code != 0 || !regexp.MustCompile(tc.line).MatchString(stdout) {
			t.Errorf("githubCliEnabled %v: exit %d (%s):\n%s", tc.enabled, code, stderr, stdout)
		}
		stdout, stderr, code = run(t, f.deps(), "-o", "json", "env")
		var got map[string]any
		if err := json.Unmarshal([]byte(stdout), &got); code != 0 || err != nil || got["githubCliEnabled"] != tc.enabled {
			t.Errorf("githubCliEnabled %v: -o json exit %d (%s), %v:\n%s", tc.enabled, code, stderr, err, stdout)
		}
	}
}

// TestEnvShowsTheEnabledModels: `bex agent env` lists the models members may
// choose with the catalog's default marked, the ones an admin turned off, and
// a saved default that is off with the model that runs instead (w10/m527). A
// bex-api that predates the list offers every catalog model.
func TestEnvShowsTheEnabledModels(t *testing.T) {
	catalog := []any{"deepseek/deepseek-v4.1-flash-1017", "anthropic/claude-haiku-5.5", "anthropic/claude-opus-5.5"}
	for _, tc := range []struct {
		name  string
		env   map[string]any
		lines []string
		not   string
	}{
		{"an older bex-api", map[string]any{"models": catalog}, []string{
			`(?m)^Default model\s+deepseek/deepseek-v4.1-flash-1017 \(the platform default\)$`,
			`(?m)^Models\s+deepseek/deepseek-v4.1-flash-1017 \(the platform default, always on\), anthropic/claude-haiku-5.5, anthropic/claude-opus-5.5$`,
		}, "Models off"},
		{"one model off", map[string]any{"models": catalog, "enabledModels": catalog[:2], "defaultModel": "anthropic/claude-haiku-5.5"}, []string{
			`(?m)^Default model\s+anthropic/claude-haiku-5.5$`,
			`(?m)^Models\s+deepseek/deepseek-v4.1-flash-1017 \(the platform default, always on\), anthropic/claude-haiku-5.5$`,
			`(?m)^Models off\s+anthropic/claude-opus-5.5 \(turned off by an admin\)$`,
		}, ""},
		{"the default off", map[string]any{"models": catalog, "enabledModels": catalog[:1], "defaultModel": "anthropic/claude-opus-5.5"}, []string{
			`(?m)^Default model\s+anthropic/claude-opus-5.5 \(turned off: sessions run deepseek/deepseek-v4.1-flash-1017, the platform default\)$`,
			`(?m)^Models off\s+anthropic/claude-haiku-5.5, anthropic/claude-opus-5.5 \(turned off by an admin\)$`,
		}, ""},
		{"a default the catalog dropped", map[string]any{"models": catalog, "enabledModels": catalog, "defaultModel": "anthropic/claude-sonnet-4.5"}, []string{
			`(?m)^Default model\s+anthropic/claude-sonnet-4.5 \(no longer offered: sessions run deepseek/deepseek-v4.1-flash-1017, the platform default\)$`,
		}, "Models off"},
		{"the workspace's own key", map[string]any{"models": []any{}, "enabledModels": []any{}, "defaultModel": "claude-opus-4-1", "modelKeySet": true}, []string{
			`(?m)^Default model\s+claude-opus-4-1$`,
		}, "Models"},
	} {
		tc.env["networkTier"], tc.env["modelSource"] = "registries", "platform"
		f := newRouteFake(t, map[string]http.HandlerFunc{"GET /v1/agent-environment": answer(http.StatusOK, tc.env)})
		stdout, stderr, code := run(t, f.deps(), "-o", "text", "env")
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", tc.name, code, stderr)
		}
		for _, line := range tc.lines {
			if !regexp.MustCompile(line).MatchString(stdout) {
				t.Errorf("%s: no line %s:\n%s", tc.name, line, stdout)
			}
		}
		if tc.not != "" && regexp.MustCompile(`(?m)^`+tc.not+`\s`).MatchString(stdout) {
			t.Errorf("%s: printed %q:\n%s", tc.name, tc.not, stdout)
		}
	}
}

// TestModelsListsOnlyTheOfferedModels: `bex agent models` prints the models
// the server offers, so a model an admin turned off is not listed (w10/m527).
func TestModelsListsOnlyTheOfferedModels(t *testing.T) {
	f := newRouteFake(t, map[string]http.HandlerFunc{"GET /v1/agent-sessions/capabilities": answer(http.StatusOK, map[string]any{
		"ready": true, "github": map[string]any{"connected": true, "accountLogin": "acme"},
		"agents": []any{map[string]any{"id": "claude", "label": "Claude Code", "modelKeyReady": true,
			"model": "deepseek/deepseek-v4.1-flash-1017", "models": []string{"deepseek/deepseek-v4.1-flash-1017", "anthropic/claude-haiku-5.5"}}},
	})})
	stdout, stderr, code := run(t, f.deps(), "-o", "text", "models")
	if code != 0 || !strings.Contains(stdout, "Models claude can run (--model ID):\n  deepseek/deepseek-v4.1-flash-1017 (default)\n  anthropic/claude-haiku-5.5\n") ||
		strings.Contains(stdout, "opus") {
		t.Fatalf("models: exit %d (%s):\n%s", code, stderr, stdout)
	}
}

func TestModels(t *testing.T) {
	catalog := []string{"deepseek/deepseek-v4.1-flash-1017", "anthropic/claude-sonnet-5.5", "anthropic/claude-opus-5.5"}
	f := newRouteFake(t, map[string]http.HandlerFunc{"GET /v1/agent-sessions/capabilities": answer(http.StatusOK, map[string]any{
		"ready": true, "github": map[string]any{"connected": true, "accountLogin": "acme"},
		"agents": []any{map[string]any{"id": "claude", "label": "Claude Code", "modelKeyReady": true, "model": catalog[1], "models": catalog},
			map[string]any{"id": "deepseek", "label": "DeepSeek", "modelKeyReady": true, "model": "~deepseek/deepseek-flash-latest", "models": []string{"~deepseek/deepseek-flash-latest"}},
			map[string]any{"id": "codex", "label": "Codex", "modelKeyReady": false, "model": nil, "models": []string{}}},
		// A made-up third template: the CLI prints what the server lists.
		"defaultTemplate": "agent-4c-8g", "templates": []any{
			map[string]any{"name": "agent", "cpu": 2, "memoryGb": 4},
			map[string]any{"name": "agent-4c-8g", "cpu": 4, "memoryGb": 8},
			map[string]any{"name": "agent-half", "cpu": 0.5, "memoryGb": 0.5},
		},
	})})
	stdout, _, code := run(t, f.deps(), "-o", "text", "models")
	if code != 0 || !strings.Contains(stdout, "connected (acme)") || !strings.Contains(stdout, "codex") || !strings.Contains(stdout, "needs the workspace's own model key") {
		t.Fatalf("models:\n%s", stdout)
	}
	// Each agent's model, and the catalog of the one that offers a choice
	// with its default marked (w10/m516).
	for _, want := range []string{
		"Claude Code  yes", catalog[1] + "\n", "~deepseek/deepseek-flash-latest\n", "the provider's default (any model its provider serves)",
		"Models claude can run (--model ID):\n  " + catalog[0] + "\n  " + catalog[1] + " (default)\n  " + catalog[2] + "\n",
		"Sandbox templates (--template NAME):\n  agent        2 CPU, 4 GB\n  agent-4c-8g  4 CPU, 8 GB (default)\n  agent-half   0.5 CPU, 0.5 GB\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("models output lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Models deepseek") {
		t.Errorf("a fixed-model agent listed a choice:\n%s", stdout)
	}
	if got := f.only(t).query; got != "ownerId=tea-ws" {
		t.Fatalf("query %q", got)
	}
}

// TestConfigContractFixtureDecodes reads the shapes ADR106 D4/D8 decode from
// the fixture the backend twin checks against the real types.
func TestConfigContractFixtureDecodes(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/agent-cli-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Session          session           `json:"session"`
		Tree             sessionTree       `json:"tree"`
		ModelCost        modelCost         `json:"modelCost"`
		Skill            agentSkill        `json:"skill"`
		SkillVersionList skillVersionList  `json:"skillVersionList"`
		Environment      agentEnvironment  `json:"environment"`
		SecretList       agentSecretList   `json:"secretList"`
		Capabilities     agentCapabilities `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Session.ParentSessionID == "" || golden.Session.Depth != 1 {
		t.Fatalf("session lineage lost: %+v", golden.Session)
	}
	if len(golden.Tree.Sessions) != 1 || golden.Tree.Sessions[0].Outcome == nil || golden.Tree.Sessions[0].Outcome.Cost.TotalUSD == "" {
		t.Fatalf("tree lost: %+v", golden.Tree)
	}
	if c := golden.ModelCost; c.CostUSD == "" || c.ModelSource == "" || len(c.Turns) != 1 || c.Turns[0].InputTokens == 0 || len(c.Turns[0].Models) != 1 {
		t.Fatalf("model cost lost: %+v", c)
	}
	if s := golden.Skill; s.Name == "" || s.Content == "" || s.Version != 2 || len(s.Triggers) != 2 {
		t.Fatalf("skill lost: %+v", s)
	}
	if v := golden.SkillVersionList; len(v.Versions) != 1 || v.Versions[0].ShortID == "" || !v.Versions[0].Current || v.NextCursor == "" {
		t.Fatalf("skill versions lost: %+v", v)
	}
	if e := golden.Environment; e.NetworkTier == "" || e.AgentModelMonthlySpendCapUSD == nil || e.TrustTier == nil || len(e.CustomHosts) != 1 || len(e.Models) != 3 ||
		e.DefaultTemplate != "agent-4c-8g" || len(e.Templates) != 2 || e.Templates[1].MemoryGB != 8 || e.GitHubCLIEnabled == nil || *e.GitHubCLIEnabled ||
		len(e.EnabledModels) != 2 {
		t.Fatalf("environment lost: %+v", e)
	}
	if l := golden.SecretList; len(l.Secrets) != 1 || l.Secrets[0].Name == "" {
		t.Fatalf("secret list lost: %+v", l)
	}
	if c := golden.Capabilities; !c.Ready || !c.GitHub.Connected || c.GitHub.AccountLogin == "" || len(c.Agents) != 2 || !c.Agents[0].ModelKeyReady ||
		c.Agents[0].Model == nil || len(c.Agents[0].Models) != 3 || len(c.Agents[1].Models) != 1 || len(c.Templates) != 2 || c.Templates[0].CPU != 2 || c.DefaultTemplate != "agent-4c-8g" {
		t.Fatalf("capabilities lost: %+v", c)
	}
}
