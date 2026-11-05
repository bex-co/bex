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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR091 D7: the workspace's own model provider key. Written by admins
// through the agent environment, never readable back, and it wins over the
// platform model route.

// The three keys the surfaces write, so a leak names the surface.
const (
	restModelKey = "sk-ant-api03-REST-WRITE-ONLY-SECRET"
	gqlModelKey  = "sk-ant-api03-GRAPHQL-WRITE-ONLY-SECRET"
	mcpModelKey  = "sk-ant-api03-MCP-WRITE-ONLY-SECRET"
)

var allModelKeys = []string{restModelKey, gqlModelKey, mcpModelKey}

type auditCapture struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (a *auditCapture) Record(_ context.Context, ev core.AuditEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, ev)
	return nil
}

func (a *auditCapture) dump() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fmt.Sprintf("%+v", a.events)
}

func modelKeyService(ws string, keys *fakeModelKeys) (*Service, *auditCapture) {
	svc := environmentService(ws, newFakeEnvironments())
	audit := &auditCapture{}
	svc.Audit = audit
	svc.ModelKeys = keys
	return svc, audit
}

func assertNoModelKey(t *testing.T, surface, body string) {
	t.Helper()
	for _, key := range allModelKeys {
		if strings.Contains(body, key) {
			t.Fatalf("%s returned the model key", surface)
		}
	}
}

func TestModelKeyIsWriteOnlyOnEverySurface(t *testing.T) {
	keys := &fakeModelKeys{}
	svc, audit := modelKeyService("tea-a", keys)
	exerciseModelKeyWriteOnly(t, svc, "tea-a", keys, audit.dump, "NONE")
}

// exerciseModelKeyWriteOnly sets the key on REST, GraphQL and MCP in turn
// (each a rotation), checks every write and read response carries only
// modelKeySet/modelSource, that the key reached the secret store, and that
// the audit trail names the model-key target without the value; then a clear
// removes it, leaving clearedSource (the GraphQL enum). auditDump renders
// every audit event recorded for ws.
func exerciseModelKeyWriteOnly(t *testing.T, svc *Service, ws string, keys *fakeModelKeys, auditDump func() string, clearedSource string) {
	t.Helper()
	path := agentsession.ModelKeySecretPath(ws)
	stored := func() string { return keys.data[path][agentsession.ModelKeyField] }

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, body string, ctx context.Context) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, "/v1/agent-environment?ownerId="+ws, strings.NewReader(body)).WithContext(ctx))
		return rec
	}

	// REST write.
	rec := serve(http.MethodPatch, `{"modelApiKey":"`+restModelKey+`"}`, caller("admin"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"modelKeySet":true`) || !strings.Contains(rec.Body.String(), `"modelSource":"byo"`) {
		t.Fatalf("REST set = %d %s", rec.Code, rec.Body.String())
	}
	assertNoModelKey(t, "REST PATCH", rec.Body.String())
	if stored() != restModelKey {
		t.Fatal("REST write did not store the key")
	}

	// GraphQL rotation.
	schema := environmentSchema(t, svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString:  `mutation($ws: String!, $input: AgentEnvironmentInput!) { updateAgentEnvironment(ownerId: $ws, input: $input) { modelKeySet modelSource modelPinned } }`,
		VariableValues: map[string]any{"ws": ws, "input": map[string]any{"modelApiKey": gqlModelKey}}})
	out, _ := json.Marshal(result)
	if len(result.Errors) != 0 || !strings.Contains(string(out), `"modelKeySet":true`) || !strings.Contains(string(out), `"modelSource":"BYO"`) {
		t.Fatalf("GraphQL set = %s", out)
	}
	assertNoModelKey(t, "GraphQL mutation", string(out))
	if stored() != gqlModelKey {
		t.Fatal("GraphQL write did not rotate the key")
	}

	// MCP rotation.
	client := newMCPClient(t, caller("admin"), svc)
	res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment",
		Arguments: map[string]any{"ownerId": ws, "modelApiKey": mcpModelKey}})
	if err != nil || res.IsError {
		t.Fatalf("MCP set = %+v err=%v", res, err)
	}
	mcpOut, _ := json.Marshal(res)
	if !strings.Contains(string(mcpOut), `modelKeySet`) {
		t.Fatalf("MCP set = %s", mcpOut)
	}
	assertNoModelKey(t, "MCP update", string(mcpOut))
	if stored() != mcpModelKey {
		t.Fatal("MCP write did not rotate the key")
	}

	// Every read surface, for every role that may read.
	for _, subject := range []string{"admin", "dev", "viewer"} {
		rec = serve(http.MethodGet, "", caller(subject))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"modelKeySet":true`) {
			t.Fatalf("REST read as %s = %d %s", subject, rec.Code, rec.Body.String())
		}
		assertNoModelKey(t, "REST GET", rec.Body.String())
		result = graphql.Do(graphql.Params{Schema: schema, Context: caller(subject),
			RequestString:  `query($ws: String!) { agentEnvironment(ownerId: $ws) { ownerId networkTier customHosts defaultModel modelPinned pinnedModel modelKeySet modelSource registryHosts updatedAt } }`,
			VariableValues: map[string]any{"ws": ws}})
		out, _ = json.Marshal(result)
		if len(result.Errors) != 0 || !strings.Contains(string(out), `"modelSource":"BYO"`) {
			t.Fatalf("GraphQL read as %s = %s", subject, out)
		}
		assertNoModelKey(t, "GraphQL query", string(out))
		reader := newMCPClient(t, caller(subject), svc)
		res, err = reader.CallTool(caller(subject), &mcp.CallToolParams{Name: "get_agent_environment", Arguments: map[string]any{"ownerId": ws}})
		if err != nil || res.IsError {
			t.Fatalf("MCP read as %s = %+v err=%v", subject, res, err)
		}
		mcpOut, _ = json.Marshal(res)
		assertNoModelKey(t, "MCP get", string(mcpOut))
	}

	trail := auditDump()
	if strings.Count(trail, ModelKeyAuditTarget) < 3 {
		t.Fatalf("audit trail lacks the model-key target for each write: %s", trail)
	}
	assertNoModelKey(t, "the audit trail", trail)

	// Clear (GraphQL can't send null: clearModelApiKey).
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString:  `mutation($ws: String!) { updateAgentEnvironment(ownerId: $ws, input: {clearModelApiKey: true}) { modelKeySet modelSource } }`,
		VariableValues: map[string]any{"ws": ws}})
	out, _ = json.Marshal(result)
	if len(result.Errors) != 0 || !strings.Contains(string(out), `"modelKeySet":false`) || !strings.Contains(string(out), `"modelSource":"`+clearedSource+`"`) {
		t.Fatalf("GraphQL clear = %s", out)
	}
	if _, ok := keys.data[path]; ok {
		t.Fatal("clear left the key in the secret store")
	}
}

// Only a workspace admin writes the key: developers, viewers and strangers
// are refused on the service and on REST, nothing reaches the secret store,
// and the refusal names the model-key target without the value.
func TestModelKeyWriteRefusedForNonAdmins(t *testing.T) {
	keys := &fakeModelKeys{}
	svc, audit := modelKeyService("tea-a", keys)
	for _, subject := range []string{"dev", "viewer", "stranger"} {
		for _, input := range []EnvironmentInput{{ModelAPIKey: ptr(restModelKey)}, {ClearModelAPIKey: true}} {
			if _, err := svc.UpdateEnvironment(caller(subject), "tea-a", input); !errors.Is(err, core.ErrForbidden) {
				t.Errorf("%s key write = %v, want forbidden", subject, err)
			}
		}
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a",
		strings.NewReader(`{"modelApiKey":"`+restModelKey+`"}`)).WithContext(caller("dev")))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("REST developer write = %d %s", rec.Code, rec.Body.String())
	}
	assertNoModelKey(t, "the REST refusal", rec.Body.String())
	if len(keys.data) != 0 {
		t.Fatal("a refused write reached the secret store")
	}
	trail := audit.dump()
	if !strings.Contains(trail, ModelKeyAuditTarget) || !strings.Contains(trail, string(core.AuditDenied)) {
		t.Fatalf("refusals not audited against the model-key target: %s", trail)
	}
	assertNoModelKey(t, "the audit trail", trail)
	if got, _ := svc.Environment(caller("admin"), "tea-a"); got.ModelKeySet {
		t.Fatal("a refused write set the flag")
	}
}

// A malformed key is a typed 400 that never echoes the value; surrounding
// whitespace (a pasted newline) is trimmed.
func TestModelKeyValidation(t *testing.T) {
	const secret = "sk-SECRET-VALUE"
	for name, input := range map[string]EnvironmentInput{
		"empty":       {ModelAPIKey: ptr("")},
		"blank":       {ModelAPIKey: ptr(" \n\t")},
		"inner space": {ModelAPIKey: ptr(secret + " x")},
		"control":     {ModelAPIKey: ptr(secret + "\x00x")},
		"too long":    {ModelAPIKey: ptr(secret + strings.Repeat("x", maxModelKeyBytes))},
		"set+clear":   {ModelAPIKey: ptr(secret), ClearModelAPIKey: true},
	} {
		keys := &fakeModelKeys{}
		svc, _ := modelKeyService("tea-a", keys)
		_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", input)
		if !errors.Is(err, core.ErrBadRequest) || !isCode(err, ModelKeyInvalidCode) {
			t.Errorf("%s = %v, want %s", name, err, ModelKeyInvalidCode)
			continue
		}
		if strings.Contains(fmt.Sprintf("%+v", err), secret) {
			t.Errorf("%s: the error echoes the key", name)
		}
		if len(keys.data) != 0 {
			t.Errorf("%s: a refused key was stored", name)
		}
	}
	keys := &fakeModelKeys{}
	svc, _ := modelKeyService("tea-a", keys)
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr("  " + secret + "\n")}); err != nil {
		t.Fatal(err)
	}
	if got := keys.data[agentsession.ModelKeySecretPath("tea-a")][agentsession.ModelKeyField]; got != secret {
		t.Fatalf("stored %q, want the trimmed key", got)
	}
}

// A secret store failure is a dependency error, and the flag is not set.
func TestModelKeyStoreFailureLeavesNoFlag(t *testing.T) {
	keys := &fakeModelKeys{err: errors.New("openbao down: " + restModelKey)}
	svc, _ := modelKeyService("tea-a", keys)
	_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr(restModelKey)})
	if err == nil || strings.Contains(err.Error(), restModelKey) {
		t.Fatalf("store failure = %v", err)
	}
	if got, _ := svc.Environment(caller("admin"), "tea-a"); got.ModelKeySet || got.UpdatedAt != nil {
		t.Fatalf("a failed key write changed the environment: %+v", got)
	}
}

// Precedence: a workspace on the platform route that set its own
// key runs claude sessions on the provider directly — the release endpoint,
// no pinned model, no trust-tier count — and is ready without OpenBao.
// Clearing the key puts it back on the platform route.
func TestBYOKeyWinsOverThePlatformRoute(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	setAt := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	svc, _, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, ModelKeySetAt: &setAt})
	envs := svc.Environments.(*fakeEnvironments)
	svc.Spend = &fakeSpend{fakeEnvironments: envs, plan: store.PlanPro} // T0: two platform sessions
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	for i := range 3 {
		created, err := svc.Create(caller("alice"), input)
		if err != nil {
			t.Fatalf("BYO session %d: %v (a BYO workspace is not tier-limited)", i+1, err)
		}
		if created.AgentConfig.ModelEndpoint != "https://api.anthropic.com/v1" || lifecycle.modelEndpoint != "https://api.anthropic.com/v1" {
			t.Fatalf("BYO endpoint = view %q sandbox %q, want the provider", created.AgentConfig.ModelEndpoint, lifecycle.modelEndpoint)
		}
		for _, name := range routeModelVars {
			if _, ok := lifecycle.driverEnv[name]; ok {
				t.Fatalf("%s pinned for a BYO workspace", name)
			}
		}
	}
	// An explicit model is the workspace's choice, not refused as pinned.
	input.AgentConfig.Model = "claude-opus-4-1"
	if _, err := svc.Create(caller("alice"), input); err != nil {
		t.Fatalf("explicit model in a BYO workspace: %v", err)
	}
	view := svc.environmentView(envs.rows["tea-a"])
	if view.ModelSource != ModelSourceBYO || !view.ModelKeySet || view.ModelPinned || view.PinnedModel != nil {
		t.Fatalf("BYO view = %+v", view)
	}
	svc.GitHub = &fakeGitHub{conn: map[string]github.Connection{"tea-a": {Connected: true}}}
	svc.ModelKeys = &fakeModelKeys{err: errors.New("openbao unreachable")}
	if caps, err := svc.Capabilities(caller("alice"), ""); err != nil || !caps.ModelKeyReady {
		t.Fatalf("BYO capabilities = %+v err=%v (the flag alone is readiness)", caps, err)
	}

	// Cleared: back on the platform route, pinned, and counted (four live
	// claude sessions are over T0's two).
	row := envs.rows["tea-a"]
	row.ModelKeySetAt = nil
	envs.rows["tea-a"] = row
	input.AgentConfig.Model = ""
	if _, err := svc.Create(caller("alice"), input); !isCode(err, TrustLimitCode) {
		t.Fatalf("platform session over the T0 limit = %v, want %s", err, TrustLimitCode)
	}
	if view := svc.environmentView(envs.rows["tea-a"]); view.ModelSource != ModelSourcePlatform || !view.ModelPinned {
		t.Fatalf("cleared view = %+v", view)
	}
}

// Setting a key lifts the platform pin in the same update, so a pinned
// workspace may choose its default model together with its own key.
func TestModelKeyLiftsThePlatformPin(t *testing.T) {
	enablePlatformRoute(t, "tea-pinned")
	keys := &fakeModelKeys{}
	svc, _ := modelKeyService("tea-pinned", keys)
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{DefaultModel: ptr("claude-opus-4-1")}); !isCode(err, "AGENT_ENVIRONMENT_MODEL_PINNED") {
		t.Fatalf("pinned default model = %v", err)
	}
	got, err := svc.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{ModelAPIKey: ptr(restModelKey), DefaultModel: ptr("claude-opus-4-1")})
	if err != nil || got.ModelPinned || got.ModelSource != ModelSourceBYO || got.DefaultModel == nil || *got.DefaultModel != "claude-opus-4-1" {
		t.Fatalf("key + default model = %+v err=%v", got, err)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{ClearModelAPIKey: true})
	if err != nil || !got.ModelPinned || got.ModelSource != ModelSourcePlatform || got.ModelKeySet {
		t.Fatalf("cleared = %+v err=%v", got, err)
	}
}

// A session created on the platform route stored the pinned OpenRouter model
// and endpoint. Once its workspace sets its own key, the next steer runs the
// workspace default model (or the profile's, with none) against the
// provider; once the key is cleared again, the pinned model on OpenRouter.
func TestSteerFollowsTheModelSource(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	svc, st, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries})
	envs := svc.Environments.(*fakeEnvironments)
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.Model != agentsession.DefaultDevModel || created.AgentConfig.ModelEndpoint != "https://openrouter.ai/api" {
		t.Fatalf("platform session = %+v", created.AgentConfig)
	}
	steer := func(byo bool, defaultModel string) {
		t.Helper()
		row := envs.rows["tea-a"]
		row.DefaultModel, row.ModelKeySetAt = defaultModel, nil
		if byo {
			at := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
			row.ModelKeySetAt = &at
		}
		envs.rows["tea-a"] = row
		session := st.rows[created.ID]
		session.Phase = PhaseCompleted
		st.rows[created.ID] = session
		lifecycle.driverEnv = nil
		if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "and the docs"}); err != nil {
			t.Fatalf("steer (byo=%t): %v", byo, err)
		}
	}

	steer(true, "claude-opus-4-1")
	if lifecycle.modelEndpoint != "https://api.anthropic.com/v1" || lifecycle.driverEnv["ANTHROPIC_MODEL"] != "claude-opus-4-1" {
		t.Fatalf("steer after set = endpoint %q model %q, want the provider and the workspace default", lifecycle.modelEndpoint, lifecycle.driverEnv["ANTHROPIC_MODEL"])
	}
	for _, name := range routeModelVars[1:] {
		if _, ok := lifecycle.driverEnv[name]; ok {
			t.Fatalf("%s still pinned after set", name)
		}
	}
	steer(true, "")
	if model, ok := lifecycle.driverEnv["ANTHROPIC_MODEL"]; ok {
		t.Fatalf("steer after set with no default model = %q, want the profile default", model)
	}

	steer(false, "")
	if lifecycle.modelEndpoint != "https://openrouter.ai/api" {
		t.Fatalf("steer after clear = endpoint %q, want the platform route", lifecycle.modelEndpoint)
	}
	for _, name := range routeModelVars {
		if lifecycle.driverEnv[name] != agentsession.DefaultDevModel {
			t.Fatalf("steer after clear %s = %q, want pinned", name, lifecycle.driverEnv[name])
		}
	}
}

// The flag write commits only after OpenBao succeeded; a flag write that
// fails after a first-time Put removes the key it just wrote, while a failed
// rotation keeps the new key (the old one cannot be restored) and reports
// the error. A successful write tells ModelKeyChanged.
func TestModelKeyFlagWriteFailure(t *testing.T) {
	path := agentsession.ModelKeySecretPath("tea-a")
	keys := &fakeModelKeys{}
	svc, _ := modelKeyService("tea-a", keys)
	var changed []string
	svc.ModelKeyChanged = func(ws string) { changed = append(changed, ws) }
	envs := svc.Environments.(*fakeEnvironments)

	envs.writeErr = errors.New("db down")
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr(restModelKey)}); !isCode(err, core.AgentSessionDependencyUnavailableCode) {
		t.Fatalf("flag write failure = %v", err)
	}
	if _, ok := keys.data[path]; ok || len(changed) != 0 {
		t.Fatal("a failed first-time set left the key behind or reported a change")
	}

	envs.writeErr = nil
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr(restModelKey)}); err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "tea-a" {
		t.Fatalf("ModelKeyChanged calls = %v", changed)
	}
	envs.writeErr = errors.New("db down")
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ModelAPIKey: ptr(gqlModelKey)}); err == nil {
		t.Fatal("a failed rotation reported success")
	}
	if keys.data[path][agentsession.ModelKeyField] != gqlModelKey || !envs.rows["tea-a"].ModelKeySet() {
		t.Fatal("a failed rotation must leave the new key active under the set flag")
	}
}
