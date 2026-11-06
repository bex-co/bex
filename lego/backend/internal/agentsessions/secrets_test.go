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
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ADR098: workspace agent secrets and inline session secrets. Values are
// write-only, reach the sandbox as env vars at every dispatch, and never
// appear in a read, a view or an error.

const (
	stripeValue = "sk_live_WORKSPACE-SECRET-VALUE-1"
	inlineValue = "inline-SESSION-SECRET-VALUE-2"
	steerValue  = "steer-SESSION-SECRET-VALUE-3"
)

func assertNoSecretValue(t *testing.T, where, body string) {
	t.Helper()
	for _, v := range []string{stripeValue, inlineValue, steerValue} {
		if strings.Contains(body, v) {
			t.Fatalf("%s leaked a secret value: %s", where, body)
		}
	}
}

func secretService() (*Service, *fakeModelKeys) {
	keys := &fakeModelKeys{}
	svc := environmentService("tea-a", newFakeEnvironments())
	svc.Audit = &auditCapture{}
	svc.ModelKeys = keys
	return svc, keys
}

func TestAgentSecretsAreWriteOnlyAndAdminManaged(t *testing.T) {
	svc, keys := secretService()
	if _, err := svc.SetAgentSecret(caller("dev"), "tea-a", "STRIPE_KEY", stripeValue); err == nil {
		t.Fatal("a developer set a workspace agent secret")
	}
	if _, err := svc.AgentSecrets(caller("viewer"), "tea-a"); err == nil {
		t.Fatal("a viewer listed workspace agent secrets")
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, path, body, who string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(caller(who)))
		return rec
	}
	rec := serve(http.MethodPut, "/v1/agent-secrets/STRIPE_KEY?ownerId=tea-a", `{"value":"`+stripeValue+`"}`, "admin")
	if rec.Code != http.StatusOK || rec.Body.String() != `{"name":"STRIPE_KEY"}`+"\n" {
		t.Fatalf("REST set = %d %q", rec.Code, rec.Body.String())
	}
	if keys.data[workspaceSecretPath("tea-a", "STRIPE_KEY")][agentSecretValueField] != stripeValue {
		t.Fatal("REST set did not store the value")
	}

	schema := environmentSchema(t, svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString: `mutation { setAgentSecret(ownerId: "tea-a", name: "OTHER", value: "` + inlineValue + `") { name } }`})
	out, _ := json.Marshal(result)
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL set = %s", out)
	}
	assertNoSecretValue(t, "GraphQL set", string(out))

	client := newMCPClient(t, caller("admin"), svc)
	res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "set_agent_secret",
		Arguments: map[string]any{"ownerId": "tea-a", "name": "THIRD", "value": steerValue}})
	if err != nil || res.IsError {
		t.Fatalf("MCP set = %+v err=%v", res, err)
	}
	mcpOut, _ := json.Marshal(res)
	assertNoSecretValue(t, "MCP set", string(mcpOut))

	// Every read surface lists names, never values; developers may read.
	rec = serve(http.MethodGet, "/v1/agent-secrets?ownerId=tea-a", "", "dev")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"STRIPE_KEY"`) {
		t.Fatalf("REST list = %d %s", rec.Code, rec.Body.String())
	}
	assertNoSecretValue(t, "REST list", rec.Body.String())
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("dev"),
		RequestString: `{ agentSecrets(ownerId: "tea-a") { ownerId secrets { name } } }`})
	out, _ = json.Marshal(result)
	if len(result.Errors) != 0 || !strings.Contains(string(out), `"OTHER"`) {
		t.Fatalf("GraphQL list = %s", out)
	}
	assertNoSecretValue(t, "GraphQL list", string(out))
	res, err = newMCPClient(t, caller("dev"), svc).CallTool(caller("dev"), &mcp.CallToolParams{Name: "list_agent_secrets", Arguments: map[string]any{"ownerId": "tea-a"}})
	if err != nil || res.IsError {
		t.Fatalf("MCP list = %+v err=%v", res, err)
	}
	mcpOut, _ = json.Marshal(res)
	if !strings.Contains(string(mcpOut), "THIRD") {
		t.Fatalf("MCP list = %s", mcpOut)
	}
	assertNoSecretValue(t, "MCP list", string(mcpOut))
	assertNoSecretValue(t, "the audit trail", svc.Audit.(*auditCapture).dump())

	if rec = serve(http.MethodDelete, "/v1/agent-secrets/STRIPE_KEY?ownerId=tea-a", "", "admin"); rec.Code != http.StatusNoContent {
		t.Fatalf("REST delete = %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := keys.data[workspaceSecretPath("tea-a", "STRIPE_KEY")]; ok {
		t.Fatal("delete left the value in the secret store")
	}
	if err := svc.DeleteAgentSecret(caller("admin"), "tea-a", "STRIPE_KEY"); codeOf(err) != "AGENT_SECRET_NOT_FOUND" {
		t.Fatalf("delete of a missing secret = %v", err)
	}
}

func TestAgentSecretNamesAndLimits(t *testing.T) {
	svc, _ := secretService()
	for name, code := range map[string]string{
		"1BAD": "AGENT_SECRET_NAME_INVALID", "has-dash": "AGENT_SECRET_NAME_INVALID", "": "AGENT_SECRET_NAME_INVALID",
		"BEX_AGENT_PROMPT": "AGENT_SECRET_NAME_RESERVED", "anthropic_api_key": "AGENT_SECRET_NAME_RESERVED",
		"PATH": "AGENT_SECRET_NAME_RESERVED", "HTTPS_PROXY": "AGENT_SECRET_NAME_RESERVED", "GIT_ASKPASS": "AGENT_SECRET_NAME_RESERVED",
	} {
		if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", name, "v"); codeOf(err) != code {
			t.Errorf("set %q = %v, want %s", name, err, code)
		}
	}
	for _, value := range []string{"", "a\x00b", strings.Repeat("x", maxSecretValueBytes+1)} {
		if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "OK_NAME", value); codeOf(err) != "AGENT_SECRET_VALUE_INVALID" {
			t.Errorf("set value len %d = %v", len(value), err)
		}
	}
	for i := range maxWorkspaceAgentSecrets {
		if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "S"+strings.Repeat("A", i), "value"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "ONE_TOO_MANY", "value"); codeOf(err) != "AGENT_SECRET_LIMIT_EXCEEDED" {
		t.Fatalf("51st secret = %v", err)
	}
	// Rotating an existing one is not a new secret.
	if _, err := svc.SetAgentSecret(caller("admin"), "tea-a", "SA", "rotated"); err != nil {
		t.Fatalf("rotate at the limit = %v", err)
	}
}

func secretSessionFixture(t *testing.T) (*Service, *fakeStore, *fakeLifecycle, *fakeModelKeys) {
	t.Helper()
	svc, st, _, lc := fixture()
	keys := &fakeModelKeys{data: map[string]map[string]string{
		workspaceSecretPath("tea-a", "STRIPE_KEY"): {agentSecretValueField: stripeValue},
	}}
	svc.ModelKeys = keys
	return svc, st, lc, keys
}

func TestCreateInjectsSecretsAtDispatchAndNeverReturnsThem(t *testing.T) {
	svc, st, lc, keys := secretSessionFixture(t)
	in := createInput()
	in.SecretNames = []string{"STRIPE_KEY"}
	in.Secrets = []SecretInput{{Name: "TEST_TOKEN", Value: inlineValue}}
	view, err := svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(view.SecretNames, []string{"STRIPE_KEY", "TEST_TOKEN"}) {
		t.Fatalf("view secretNames = %v", view.SecretNames)
	}
	body, _ := json.Marshal(view)
	assertNoSecretValue(t, "the create view", string(body))
	if lc.driverEnv["STRIPE_KEY"] != stripeValue || lc.driverEnv["TEST_TOKEN"] != inlineValue {
		t.Fatalf("dispatch env lacks the secrets: %v", slices.Sorted(maps.Keys(lc.driverEnv)))
	}
	if lc.driverEnv[secretNamesEnv] != "STRIPE_KEY,TEST_TOKEN" {
		t.Fatalf("%s = %q", secretNamesEnv, lc.driverEnv[secretNamesEnv])
	}
	row := st.rows[view.ID]
	rowDump, _ := json.Marshal(row)
	assertNoSecretValue(t, "the session row", string(rowDump))
	if keys.data[sessionSecretsPath("tea-a", view.ID)]["TEST_TOKEN"] != inlineValue {
		t.Fatal("the inline secret was not stored in the session document")
	}

	// A steer adds an inline secret; the next dispatch carries all three.
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	steer := SteerRequest{SessionID: view.ID, Prompt: "again"}
	steer.Secrets = []SecretInput{{Name: "LATER", Value: steerValue}}
	steered, err := svc.Steer(caller("alice"), steer)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(steered)
	assertNoSecretValue(t, "the steer view", string(body))
	if !slices.Equal(steered.SecretNames, []string{"LATER", "STRIPE_KEY", "TEST_TOKEN"}) {
		t.Fatalf("steer secretNames = %v", steered.SecretNames)
	}
	if lc.driverEnv["LATER"] != steerValue || lc.driverEnv["TEST_TOKEN"] != inlineValue || lc.driverEnv["STRIPE_KEY"] != stripeValue {
		t.Fatal("the steer dispatch lost a secret")
	}

	// Delete removes the inline document with the session.
	row = st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	if err := svc.Delete(caller("alice"), view.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := keys.data[sessionSecretsPath("tea-a", view.ID)]; ok {
		t.Fatal("the session's inline secrets outlived it")
	}
}

func TestAttachRefusesUnknownOrOversizedSecrets(t *testing.T) {
	svc, st, lc, _ := secretSessionFixture(t)
	in := createInput()
	in.SecretNames = []string{"NOT_STORED"}
	if _, err := svc.Create(caller("alice"), in); codeOf(err) != "AGENT_SECRET_NOT_FOUND" {
		t.Fatalf("unknown ref = %v", err)
	}
	in = createInput()
	in.Secrets = []SecretInput{{Name: "A", Value: "x"}, {Name: "A", Value: "y"}}
	if _, err := svc.Create(caller("alice"), in); codeOf(err) != "AGENT_SECRET_DUPLICATE" {
		t.Fatalf("duplicate inline = %v", err)
	}
	in = createInput()
	for i := range 5 {
		in.Secrets = append(in.Secrets, SecretInput{Name: "BIG" + strings.Repeat("_", i), Value: strings.Repeat("x", maxSecretValueBytes)})
	}
	if _, err := svc.Create(caller("alice"), in); codeOf(err) != "AGENT_SECRET_LIMIT_EXCEEDED" {
		t.Fatalf("oversized total = %v", err)
	}
	if len(st.rows) != 0 || lc.created != 0 {
		t.Fatalf("a refused attach still created a session (rows=%d sandboxes=%d)", len(st.rows), lc.created)
	}
	svc.ModelKeys = nil
	in = createInput()
	in.Secrets = []SecretInput{{Name: "A", Value: "x"}}
	if _, err := svc.Create(caller("alice"), in); codeOf(err) != "AGENT_SECRETS_UNAVAILABLE" {
		t.Fatalf("no secret store = %v", err)
	}
}

// A references-only steer still counts the session's earlier inline values:
// four full inline values fill the 128 KiB total, so adding a stored secret
// by reference must be refused rather than slipping past the cap.
func TestReferenceOnlySteerCountsPriorInlineValues(t *testing.T) {
	svc, st, _, _ := secretSessionFixture(t)
	in := createInput()
	for i := range 4 {
		in.Secrets = append(in.Secrets, SecretInput{Name: "BIG" + strings.Repeat("_", i), Value: strings.Repeat("x", maxSecretValueBytes)})
	}
	view, err := svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	steer := SteerRequest{SessionID: view.ID, Prompt: "again"}
	steer.SecretNames = []string{"STRIPE_KEY"}
	if _, err := svc.Steer(caller("alice"), steer); codeOf(err) != "AGENT_SECRET_LIMIT_EXCEEDED" {
		t.Fatalf("references-only steer over the total = %v", err)
	}
}

func TestDeletedWorkspaceSecretFailsTheNextDispatch(t *testing.T) {
	svc, st, lc, keys := secretSessionFixture(t)
	in := createInput()
	in.SecretNames = []string{"STRIPE_KEY"}
	view, err := svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	delete(keys.data, workspaceSecretPath("tea-a", "STRIPE_KEY"))
	row := st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	before := lc.created
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: view.ID, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if lc.created != before {
		t.Fatal("a sandbox started without its attached secret")
	}
	if got := st.rows[view.ID].FailureReasonCode; got != secretUnavailableFailure.Code {
		t.Fatalf("failure code = %q, want %s", got, secretUnavailableFailure.Code)
	}
}

// versionedKeys adds OpenBao's check-and-set to the fake: every write bumps
// the path's version, and PutCAS refuses a stale expectation.
type versionedKeys struct {
	*fakeModelKeys
	versions map[string]uint64
	// beforeCAS runs once just before the next PutCAS, to stage a race.
	beforeCAS func()
}

func (v *versionedKeys) GetVersioned(ctx context.Context, path string) (core.SecretKVSnapshot, error) {
	data, err := v.Get(ctx, path)
	return core.SecretKVSnapshot{Data: maps.Clone(data), Version: v.versions[path]}, err
}

func (v *versionedKeys) PutCAS(ctx context.Context, path string, data map[string]string, expected uint64) (uint64, error) {
	if hook := v.beforeCAS; hook != nil {
		v.beforeCAS = nil
		hook()
	}
	if v.versions[path] != expected {
		return 0, core.ErrConflict
	}
	if err := v.Put(ctx, path, data); err != nil {
		return 0, err
	}
	v.versions[path]++
	return v.versions[path], nil
}

// Two steers that both read the inline document cannot drop each other's
// values: the second write's check-and-set fails and its turn never begins,
// so no names are attached for it.
func TestConcurrentSteerInlineWriteIsCompareAndSet(t *testing.T) {
	svc, st, _, keys := secretSessionFixture(t)
	vk := &versionedKeys{fakeModelKeys: keys, versions: map[string]uint64{}}
	svc.ModelKeys = vk
	in := createInput()
	in.Secrets = []SecretInput{{Name: "TEST_TOKEN", Value: inlineValue}}
	view, err := svc.Create(caller("alice"), in)
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row
	path := sessionSecretsPath("tea-a", view.ID)
	vk.beforeCAS = func() { // another steer lands its write first
		_, _ = vk.PutCAS(context.Background(), path, map[string]string{"TEST_TOKEN": inlineValue, "OTHER": "x"}, vk.versions[path])
	}
	steer := SteerRequest{SessionID: view.ID, Prompt: "again"}
	steer.Secrets = []SecretInput{{Name: "LATER", Value: steerValue}}
	if _, err := svc.Steer(caller("alice"), steer); codeOf(err) != "AGENT_SESSION_CONFLICT" {
		t.Fatalf("stale inline write = %v", err)
	}
	if got := st.rows[view.ID]; got.Turns != row.Turns || slices.Contains(got.SecretNames, "LATER") {
		t.Fatalf("a refused steer began a turn or attached names: turns=%d names=%v", got.Turns, got.SecretNames)
	}
	if _, ok := keys.data[path]["OTHER"]; !ok {
		t.Fatal("the losing steer overwrote the winner's inline document")
	}
}
