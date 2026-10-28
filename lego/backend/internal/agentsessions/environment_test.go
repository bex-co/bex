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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeEnvironments is the in-memory EnvironmentStore: it applies a patch the
// way the PG upsert does (nil keeps, first write fills the column defaults).
type fakeEnvironments struct {
	rows    map[string]store.AgentEnvironment
	readErr error
}

func newFakeEnvironments() *fakeEnvironments {
	return &fakeEnvironments{rows: map[string]store.AgentEnvironment{}}
}

func (f *fakeEnvironments) GetAgentEnvironment(_ context.Context, ws string) (store.AgentEnvironment, error) {
	if f.readErr != nil {
		return store.AgentEnvironment{}, f.readErr
	}
	row, ok := f.rows[ws]
	if !ok {
		return store.AgentEnvironment{}, store.ErrNotFound
	}
	return row, nil
}

func (f *fakeEnvironments) UpsertAgentEnvironment(_ context.Context, ws string, patch store.AgentEnvironmentPatch) (store.AgentEnvironment, error) {
	row, ok := f.rows[ws]
	if !ok {
		row = store.AgentEnvironment{WorkspaceID: ws, NetworkTier: NetworkTierRegistries, CustomHosts: []string{}}
	}
	if patch.NetworkTier != nil {
		row.NetworkTier = *patch.NetworkTier
	}
	if patch.CustomHosts != nil {
		row.CustomHosts = slices.Clone(*patch.CustomHosts)
	}
	if patch.DefaultModel != nil {
		row.DefaultModel = *patch.DefaultModel
	}
	row.UpdatedAt = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	f.rows[ws] = row
	return row, nil
}

// roleChecker models one workspace's roles: admin holds every relation,
// developer everything but can_manage, viewer only can_view; anyone else
// (including every subject on any other workspace) is denied.
type roleChecker struct {
	workspace string
	roles     map[string]string
}

func (c roleChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	if object != "workspace:"+c.workspace {
		return false, nil
	}
	switch c.roles[strings.TrimPrefix(subject, "user:")] {
	case "admin":
		return true, nil
	case "developer":
		return relation != core.RelCanManage, nil
	case "viewer":
		return relation == core.RelCanView, nil
	}
	return false, nil
}

func environmentService(ws string, environments EnvironmentStore) *Service {
	return &Service{
		Base: &core.Base{Authz: roleChecker{workspace: ws, roles: map[string]string{
			"admin": "admin", "dev": "developer", "viewer": "viewer",
		}}},
		Environments: environments,
	}
}

func ptr[T any](v T) *T { return &v }

// TestEnvironmentDefaultsAndRoundTrip: a workspace with no row reads the
// defaults; a partial update keeps every field it does not name.
func TestEnvironmentDefaultsAndRoundTrip(t *testing.T) {
	exerciseEnvironmentRoundTrip(t, environmentService("tea-a", newFakeEnvironments()), "tea-a")
}

func exerciseEnvironmentRoundTrip(t *testing.T, svc *Service, ws string) {
	t.Helper()
	got, err := svc.Environment(caller("viewer"), ws)
	if err != nil {
		t.Fatalf("Environment: %v", err)
	}
	want := Environment{OwnerID: ws, NetworkTier: NetworkTierRegistries, CustomHosts: []string{}, RegistryHosts: sessionegress.DefaultRegistryDomains()}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults = %+v, want %+v", got, want)
	}

	got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{NetworkTier: ptr(NetworkTierCustom), CustomHosts: &[]string{"Docs.Example.com", "api.example.net"}})
	if err != nil {
		t.Fatalf("UpdateEnvironment: %v", err)
	}
	if got.NetworkTier != NetworkTierCustom || !slices.Equal(got.CustomHosts, []string{"api.example.net", "docs.example.com"}) || got.DefaultModel != nil || got.UpdatedAt == nil {
		t.Fatalf("after tier+hosts = %+v", got)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{DefaultModel: ptr("claude-sonnet-4-5")})
	if err != nil {
		t.Fatalf("UpdateEnvironment model: %v", err)
	}
	if got.NetworkTier != NetworkTierCustom || len(got.CustomHosts) != 2 || got.DefaultModel == nil || *got.DefaultModel != "claude-sonnet-4-5" {
		t.Fatalf("a model-only update lost another field: %+v", got)
	}
	if read, err := svc.Environment(caller("dev"), ws); err != nil || !reflect.DeepEqual(read, got) {
		t.Fatalf("read back = %+v err=%v, want %+v", read, err, got)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{CustomHosts: &[]string{}, DefaultModel: ptr("")})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(got.CustomHosts) != 0 || got.CustomHosts == nil || got.DefaultModel != nil || got.NetworkTier != NetworkTierCustom {
		t.Fatalf("clear = %+v, want no hosts, provider-default model, tier kept", got)
	}
}

// TestEnvironmentAuthorization: any member reads; only a workspace admin
// writes; a non-member is denied both, on the workspace they named.
func TestEnvironmentAuthorization(t *testing.T) {
	exerciseEnvironmentAuthorization(t, environmentService("tea-a", newFakeEnvironments()), "tea-a", "tea-b")
}

func exerciseEnvironmentAuthorization(t *testing.T, svc *Service, ws, other string) {
	t.Helper()
	for _, subject := range []string{"admin", "dev", "viewer"} {
		if _, err := svc.Environment(caller(subject), ws); err != nil {
			t.Errorf("%s read: %v", subject, err)
		}
	}
	for _, subject := range []string{"dev", "viewer", "stranger"} {
		if _, err := svc.UpdateEnvironment(caller(subject), ws, EnvironmentInput{NetworkTier: ptr(NetworkTierCustom)}); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s write = %v, want forbidden", subject, err)
		}
	}
	if got, err := svc.Environment(caller("admin"), ws); err != nil || got.NetworkTier != NetworkTierRegistries {
		t.Fatalf("a refused write changed the environment: %+v err=%v", got, err)
	}
	if _, err := svc.Environment(caller("admin"), other); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("cross-workspace read = %v, want forbidden", err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), other, EnvironmentInput{NetworkTier: ptr(NetworkTierFull)}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("cross-workspace write = %v, want forbidden", err)
	}
}

// TestEnvironmentValidationErrorsOnEverySurface: each refusal is a typed 400
// with the same code on REST, GraphQL and MCP, and writes nothing.
func TestEnvironmentValidationErrorsOnEverySurface(t *testing.T) {
	enablePlatformRoute(t, "tea-pinned")
	exerciseEnvironmentValidation(t, func() (*Service, string) { return environmentService("tea-a", newFakeEnvironments()), "tea-a" })

	pinned := environmentService("tea-pinned", newFakeEnvironments())
	if _, err := pinned.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{DefaultModel: ptr("claude-sonnet-4-5")}); !isCode(err, "AGENT_ENVIRONMENT_MODEL_PINNED") {
		t.Fatalf("pinned default model = %v, want AGENT_ENVIRONMENT_MODEL_PINNED", err)
	}
	got, err := pinned.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{DefaultModel: ptr(agentsession.DefaultDevModel)})
	if err != nil || !got.ModelPinned || got.PinnedModel == nil || *got.PinnedModel != agentsession.DefaultDevModel {
		t.Fatalf("pinned workspace = %+v err=%v, want modelPinned with the pinned model", got, err)
	}
}

// newService returns a fresh service and the workspace it administers, so
// each case starts from an unwritten environment.
func exerciseEnvironmentValidation(t *testing.T, newService func() (*Service, string)) {
	t.Helper()
	tooMany := make([]string, 33)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("h%d.example.com", i)
	}
	for _, tc := range []struct {
		name  string
		input EnvironmentInput
		code  string
	}{
		{"unknown tier", EnvironmentInput{NetworkTier: ptr("internet")}, "AGENT_ENVIRONMENT_TIER_INVALID"},
		{"url host", EnvironmentInput{CustomHosts: &[]string{"https://example.com"}}, "AGENT_ENVIRONMENT_HOSTS_INVALID"},
		{"private host", EnvironmentInput{CustomHosts: &[]string{"db.internal"}}, "AGENT_ENVIRONMENT_HOSTS_INVALID"},
		{"too many hosts", EnvironmentInput{CustomHosts: &tooMany}, "AGENT_ENVIRONMENT_HOSTS_INVALID"},
		{"github baseline", EnvironmentInput{CustomHosts: &[]string{"github.com"}}, "AGENT_ENVIRONMENT_HOSTS_INVALID"},
		{"model provider", EnvironmentInput{CustomHosts: &[]string{"openrouter.ai"}}, "AGENT_ENVIRONMENT_HOSTS_INVALID"},
		{"model id", EnvironmentInput{DefaultModel: ptr("claude sonnet")}, "AGENT_ENVIRONMENT_MODEL_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, ws := newService()
			_, err := svc.UpdateEnvironment(caller("admin"), ws, tc.input)
			if !errors.Is(err, core.ErrBadRequest) || !isCode(err, tc.code) {
				t.Fatalf("service error = %#v, want %s", err, tc.code)
			}
			if got, err := svc.Environment(caller("admin"), ws); err != nil || got.UpdatedAt != nil {
				t.Fatalf("a refused update was written: %+v err=%v", got, err)
			}

			body, _ := json.Marshal(tc.input)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId="+ws, strings.NewReader(string(body))).WithContext(caller("admin")))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
			}

			client := newMCPClient(t, caller("admin"), svc)
			args := map[string]any{"ownerId": ws}
			_ = json.Unmarshal(body, &args)
			res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: args})
			if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, tc.code) {
				t.Fatalf("MCP = %+v err=%v", res, err)
			}

			if tc.input.NetworkTier != nil {
				return // an unknown enum value never reaches the resolver on GraphQL
			}
			result := graphql.Do(graphql.Params{Schema: environmentSchema(t, svc), Context: caller("admin"),
				RequestString:  `mutation($ws: String!, $input: AgentEnvironmentInput!) { updateAgentEnvironment(ownerId: $ws, input: $input) { networkTier } }`,
				VariableValues: map[string]any{"ws": ws, "input": gqlInput(tc.input)}})
			if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != tc.code {
				t.Fatalf("GraphQL = %#v", result.Errors)
			}
		})
	}
}

func gqlInput(in EnvironmentInput) map[string]any {
	out := map[string]any{}
	if in.CustomHosts != nil {
		hosts := make([]any, 0, len(*in.CustomHosts))
		for _, h := range *in.CustomHosts {
			hosts = append(hosts, h)
		}
		out["customHosts"] = hosts
	}
	if in.DefaultModel != nil {
		out["defaultModel"] = *in.DefaultModel
	}
	return out
}

func environmentSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// TestEnvironmentSurfacesShareOneShape: GraphQL exposes exactly the JSON
// fields REST and MCP serialize, the tier as the AgentNetworkTier enum, and a
// GraphQL update round-trips through the same service.
func TestEnvironmentSurfacesShareOneShape(t *testing.T) {
	wantFields := map[string]bool{}
	typ := reflect.TypeFor[Environment]()
	for i := range typ.NumField() {
		wantFields[strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]] = true
	}
	gqlFields := agentEnvironmentGQLType.Fields()
	if len(gqlFields) != len(wantFields) {
		t.Fatalf("GraphQL fields=%d JSON fields=%d", len(gqlFields), len(wantFields))
	}
	for name := range wantFields {
		if gqlFields[name] == nil {
			t.Errorf("JSON field %q missing from GraphQL AgentEnvironment", name)
		}
	}

	svc := environmentService("tea-a", newFakeEnvironments())
	schema := environmentSchema(t, svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {networkTier: CUSTOM, customHosts: ["docs.example.com"]}) { ownerId networkTier customHosts defaultModel modelPinned pinnedModel registryHosts updatedAt } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL update: %v", result.Errors)
	}
	env := result.Data.(map[string]any)["updateAgentEnvironment"].(map[string]any)
	if env["networkTier"] != "CUSTOM" || env["ownerId"] != "tea-a" || env["defaultModel"] != nil || env["modelPinned"] != false || env["updatedAt"] == nil {
		t.Fatalf("GraphQL environment = %#v", env)
	}
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("viewer"), RequestString: `{ agentEnvironment(ownerId: "tea-a") { networkTier customHosts } }`})
	if len(result.Errors) != 0 || result.Data.(map[string]any)["agentEnvironment"].(map[string]any)["networkTier"] != "CUSTOM" {
		t.Fatalf("GraphQL read = %#v %v", result.Data, result.Errors)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil).WithContext(caller("viewer")))
	var rest map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &rest) != nil || rest["networkTier"] != "custom" || len(rest) != len(wantFields) {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"networkTier":"full","unknown":1}`)).WithContext(caller("admin")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("REST unknown field = %d, want 400 (strict decoding)", rec.Code)
	}

	client := newMCPClient(t, caller("viewer"), svc)
	res, err := client.CallTool(caller("viewer"), &mcp.CallToolParams{Name: "get_agent_environment", Arguments: map[string]any{"ownerId": "tea-a"}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"networkTier":"custom"`) {
		t.Fatalf("MCP read = %+v err=%v", res, err)
	}
}

// TestEnvironmentOwnerIDIsOptionalOnEverySurface: omitting ownerId resolves
// the caller's default workspace on GraphQL exactly as on REST and MCP (and
// as agentSessionCapabilities does), for the read and the update.
func TestEnvironmentOwnerIDIsOptionalOnEverySurface(t *testing.T) {
	svc := environmentService(core.DefaultTenant, newFakeEnvironments())
	schema := environmentSchema(t, svc)
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"),
		RequestString: `mutation { updateAgentEnvironment(input: {networkTier: FULL}) { ownerId networkTier } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL update without ownerId: %v", result.Errors)
	}
	if env := result.Data.(map[string]any)["updateAgentEnvironment"].(map[string]any); env["ownerId"] != core.DefaultTenant || env["networkTier"] != "FULL" {
		t.Fatalf("GraphQL update = %#v", env)
	}
	result = graphql.Do(graphql.Params{Schema: schema, Context: caller("viewer"), RequestString: `{ agentEnvironment { ownerId networkTier } }`})
	if len(result.Errors) != 0 || result.Data.(map[string]any)["agentEnvironment"].(map[string]any)["networkTier"] != "FULL" {
		t.Fatalf("GraphQL read without ownerId = %#v %v", result.Data, result.Errors)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-environment", nil).WithContext(caller("viewer")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"networkTier":"full"`) {
		t.Fatalf("REST read without ownerId = %d %s", rec.Code, rec.Body.String())
	}
	client := newMCPClient(t, caller("viewer"), svc)
	res, err := client.CallTool(caller("viewer"), &mcp.CallToolParams{Name: "get_agent_environment", Arguments: map[string]any{}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"networkTier":"full"`) {
		t.Fatalf("MCP read without ownerId = %+v err=%v", res, err)
	}
}

// --- sessions follow the workspace environment ----------------------------

func sessionWithEnvironment(t *testing.T, row *store.AgentEnvironment) (*Service, *fakeStore, *fakeLifecycle) {
	t.Helper()
	svc, st, _, lifecycle := fixture()
	envs := newFakeEnvironments()
	if row != nil {
		row.WorkspaceID = "tea-a"
		envs.rows["tea-a"] = *row
	}
	svc.Environments = envs
	return svc, st, lifecycle
}

// The tier decides which workspace hosts widen the session: only `custom`
// opens its hosts; `full` is enforced as `registries`. The API allowlist is
// merged in every tier.
func TestCreateEgressFollowsTheWorkspaceTier(t *testing.T) {
	hosts := []string{"internal-mirror.example.com"}
	for _, tc := range []struct {
		tier string
		want []string
	}{
		{NetworkTierRegistries, []string{"docs.example.com"}},
		{NetworkTierCustom, []string{"docs.example.com", "internal-mirror.example.com"}},
		{NetworkTierFull, []string{"docs.example.com"}},
	} {
		t.Run(tc.tier, func(t *testing.T) {
			svc, _, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: tc.tier, CustomHosts: hosts})
			if _, err := svc.Create(caller("alice"), createInput()); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(lifecycle.egressAllowlist, tc.want) {
				t.Fatalf("egress = %v, want %v", lifecycle.egressAllowlist, tc.want)
			}
		})
	}
}

// A workspace that never saved an environment gets the registries default and
// an unchanged API: the explicit allowlist alone.
func TestCreateWithoutAnEnvironmentRowKeepsTheRequestAllowlist(t *testing.T) {
	svc, _, lifecycle := sessionWithEnvironment(t, nil)
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lifecycle.egressAllowlist, []string{"docs.example.com"}) {
		t.Fatalf("egress = %v", lifecycle.egressAllowlist)
	}
}

// A failed environment read fails the create closed instead of guessing a
// network posture.
func TestCreateFailsClosedWhenTheEnvironmentIsUnreadable(t *testing.T) {
	svc, _, lifecycle := sessionWithEnvironment(t, nil)
	svc.Environments.(*fakeEnvironments).readErr = errors.New("db down")
	if _, err := svc.Create(caller("alice"), createInput()); !isCode(err, core.AgentSessionDependencyUnavailableCode) {
		t.Fatalf("create = %v, want dependency unavailable", err)
	}
	if lifecycle.created != 0 {
		t.Fatal("a sandbox was provisioned without the workspace environment")
	}
}

// The workspace default model applies when the request names none and
// reaches the claude driver as ANTHROPIC_MODEL; an explicit model wins.
func TestCreateUsesTheWorkspaceDefaultModel(t *testing.T) {
	svc, _, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultModel: "claude-opus-4-1"})
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.Model != "claude-opus-4-1" || lifecycle.driverEnv["ANTHROPIC_MODEL"] != "claude-opus-4-1" {
		t.Fatalf("model = view %q env %q, want the workspace default", created.AgentConfig.Model, lifecycle.driverEnv["ANTHROPIC_MODEL"])
	}
	input.AgentConfig.Model = "claude-sonnet-4-5"
	created, err = svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.Model != "claude-sonnet-4-5" || lifecycle.driverEnv["ANTHROPIC_MODEL"] != "claude-sonnet-4-5" {
		t.Fatalf("explicit model = view %q env %q", created.AgentConfig.Model, lifecycle.driverEnv["ANTHROPIC_MODEL"])
	}
	input.AgentConfig.Model = "claude sonnet; rm -rf /"
	if _, err := svc.Create(caller("alice"), input); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("malformed model = %v, want AGENT_SESSION_INPUT_INVALID", err)
	}
}

// t002: on the platform route a different explicit model is a typed 400 at
// create — never a gateway 403 mid-turn — and the sandbox always carries the
// pinned model, including on steer for a session whose stored model differs.
func TestPlatformRouteModelIsPinnedAtCreateAndSteer(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	svc, st, lifecycle := sessionWithEnvironment(t, nil)
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: "anthropic/claude-sonnet-4.5", Task: "fix the tests"}
	_, err := svc.Create(caller("alice"), input)
	if !errors.Is(err, core.ErrBadRequest) || !isCode(err, "AGENT_SESSION_MODEL_PINNED") {
		t.Fatalf("create with a different model = %v, want AGENT_SESSION_MODEL_PINNED", err)
	}
	var coded *core.CodedError
	if errors.As(err, &coded); coded.Params["pinnedModel"] != agentsession.DefaultDevModel {
		t.Fatalf("pinned refusal params = %v", coded.Params)
	}
	if lifecycle.created != 0 {
		t.Fatal("a refused create provisioned a sandbox")
	}

	for _, model := range []string{"", agentsession.DefaultDevModel} {
		input.AgentConfig.Model = model
		created, err := svc.Create(caller("alice"), input)
		if err != nil {
			t.Fatalf("create with model %q: %v", model, err)
		}
		if created.AgentConfig.Model != agentsession.DefaultDevModel || lifecycle.driverEnv["ANTHROPIC_MODEL"] != agentsession.DefaultDevModel {
			t.Fatalf("model %q = view %q env %q, want pinned", model, created.AgentConfig.Model, lifecycle.driverEnv["ANTHROPIC_MODEL"])
		}
	}

	// A session created before the workspace joined the route stored another
	// model; steering it carries the pinned one without an error.
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	row.AgentConfig = []byte(`{"agent":"claude","model":"anthropic/claude-sonnet-4.5","task":"fix the tests"}`)
	st.rows[created.ID] = row
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "and the docs"}); err != nil {
		t.Fatalf("steer: %v", err)
	}
	for _, name := range routeModelVars {
		if lifecycle.driverEnv[name] != agentsession.DefaultDevModel {
			t.Fatalf("steer %s = %q, want pinned", name, lifecycle.driverEnv[name])
		}
	}

	// codex is not served by the route: its explicit model is its own.
	input.AgentConfig = AgentConfig{Agent: "codex", Model: "gpt-5", Task: "fix the tests"}
	if created, err := svc.Create(caller("alice"), input); err != nil || created.AgentConfig.Model != "gpt-5" {
		t.Fatalf("codex in a pinned workspace = %+v err=%v", created.AgentConfig, err)
	}
}

// Outside the allowlist the explicit model passes through unchanged.
func TestModelOutsideThePlatformRouteIsUnpinned(t *testing.T) {
	enablePlatformRoute(t, "tea-b")
	svc, _, lifecycle := sessionWithEnvironment(t, nil)
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: "claude-sonnet-4-5", Task: "fix the tests"}
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if created.AgentConfig.Model != "claude-sonnet-4-5" || lifecycle.driverEnv["ANTHROPIC_MODEL"] != "claude-sonnet-4-5" || lifecycle.driverEnv["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "" {
		t.Fatalf("unpinned model = view %q env %v", created.AgentConfig.Model, lifecycle.driverEnv)
	}
}

// A tier or host change applies from the session's next turn.
func TestSteerReResolvesTheWorkspaceEnvironment(t *testing.T) {
	svc, st, lifecycle := sessionWithEnvironment(t, nil)
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row
	svc.Environments.(*fakeEnvironments).rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierCustom, CustomHosts: []string{"mirror.example.com"}}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again", EgressAllowlist: []string{"docs.example.com"}}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs.example.com", "mirror.example.com"}; !slices.Equal(lifecycle.egressAllowlist, want) {
		t.Fatalf("steer egress = %v, want %v", lifecycle.egressAllowlist, want)
	}
}
