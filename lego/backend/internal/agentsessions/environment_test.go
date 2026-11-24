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
	"net/url"
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
	rows     map[string]store.AgentEnvironment
	readErr  error
	writeErr error // fails the locked upsert after its write succeeded
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
	if patch.DefaultTemplate != nil {
		row.DefaultTemplate = *patch.DefaultTemplate
	}
	if patch.SpendCapOverrideMicroUSD != nil {
		row.SpendCapOverrideMicroUSD = patch.SpendCapOverrideMicroUSD
	}
	if patch.ClearSpendCapOverride {
		row.SpendCapOverrideMicroUSD = nil
	}
	if patch.ModelKeySet != nil {
		row.ModelKeySetAt = nil
		if *patch.ModelKeySet {
			at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
			row.ModelKeySetAt = &at
		}
	}
	if patch.DisabledAgents != nil {
		row.DisabledAgents = slices.Clone(*patch.DisabledAgents)
	}
	if patch.GitHubCLIDisabled != nil {
		row.GitHubCLIDisabled = *patch.GitHubCLIDisabled
	}
	if patch.SetAutomationsGitHubCLI {
		row.AutomationsGitHubCLI = patch.AutomationsGitHubCLI
	}
	if patch.GitHubCLIMandatory != nil {
		row.GitHubCLIMandatory = *patch.GitHubCLIMandatory
	}
	if patch.DisabledModels != nil {
		row.DisabledModels = slices.Clone(*patch.DisabledModels)
		row.KnownModels = slices.Clone(patch.KnownModels)
		row.EnabledProviders = slices.Clone(patch.EnabledProviders)
	}
	if patch.IdleTimeoutMinutes != nil {
		minutes := *patch.IdleTimeoutMinutes
		row.IdleTimeoutMinutes = &minutes
	}
	for _, agent := range patch.AcknowledgeDataUse {
		if _, kept := row.DataUseAcks[agent]; kept {
			continue
		}
		if row.DataUseAcks == nil {
			row.DataUseAcks = map[string]store.AgentDataUseAck{}
		}
		row.DataUseAcks[agent] = store.AgentDataUseAck{At: time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC), By: patch.AcknowledgedBy}
	}
	row.UpdatedAt = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	f.rows[ws] = row
	return row, nil
}

// UpsertAgentEnvironmentLocked runs write against the current row (the
// defaults when there is none) and applies patch only when it succeeded, like
// the PG transaction.
func (f *fakeEnvironments) UpsertAgentEnvironmentLocked(ctx context.Context, ws string, patch store.AgentEnvironmentPatch, write func(context.Context, store.AgentEnvironment) error) (store.AgentEnvironment, error) {
	current, ok := f.rows[ws]
	if !ok {
		current = store.AgentEnvironment{WorkspaceID: ws, NetworkTier: NetworkTierRegistries, CustomHosts: []string{}}
	}
	if err := write(ctx, current); err != nil {
		return store.AgentEnvironment{}, err
	}
	if f.writeErr != nil {
		return store.AgentEnvironment{}, f.writeErr
	}
	return f.UpsertAgentEnvironment(ctx, ws, patch)
}

// roleChecker models one workspace's roles: admin holds every relation,
// developer everything but can_manage, contributor and billing their
// model.fga relations, viewer only can_view; anyone else (including every
// subject on any other workspace) is denied.
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
	case "contributor":
		return slices.Contains([]string{core.RelCanView, core.RelCanViewActivity, core.RelCanViewLogs, core.RelCanOperate}, relation), nil
	case "billing":
		return slices.Contains([]string{core.RelCanView, core.RelCanViewBilling, core.RelCanManageBilling}, relation), nil
	case "viewer":
		return relation == core.RelCanView, nil
	}
	return false, nil
}

func environmentService(ws string, environments EnvironmentStore) *Service {
	return &Service{
		Base: &core.Base{Authz: roleChecker{workspace: ws, roles: map[string]string{
			"admin": "admin", "dev": "developer", "contrib": "contributor", "billing": "billing", "viewer": "viewer",
		}}},
		Environments: environments,
		// Production's two agent templates (fakeLifecycle.AgentTemplates).
		Sandbox: &fakeLifecycle{},
	}
}

func ptr[T any](v T) *T { return &v }

// productionTemplates are fakeLifecycle's templates as the reads list them.
var productionTemplates = []SandboxTemplate{{Name: "agent", CPU: 2, MemoryGB: 4}, {Name: "agent-4c-8g", CPU: 4, MemoryGB: 8}}

// offerOf is the TemplateOffer of a workspace whose default is def.
func offerOf(def string) TemplateOffer {
	return TemplateOffer{DefaultTemplate: def, Templates: productionTemplates}
}

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
	want := Environment{OwnerID: ws, NetworkTier: NetworkTierRegistries, CustomHosts: []string{}, Models: []string{}, AllowAllModels: true, EnabledModels: []string{}, NewModels: []string{}, Providers: []string{}, ModelSource: ModelSourceNone, RegistryHosts: sessionegress.DefaultRegistryDomains(),
		TemplateOffer: offerOf("agent"),
		EnabledAgents: []string{"claude", "codex", "deepseek", "gemini"}, AgentDataUse: []AgentDataUse{{Agent: "muse"}},
		GitHubCLIEnabled: true, IdleTimeoutMinutes: 5, IdleTimeoutConfigurable: true}
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

	// The default sandbox template (w10/m519): a registered one is stored and
	// read back, a template-only update keeps every other field, and
	// "" returns to the platform's.
	got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{DefaultTemplate: ptr(" agent-4c-8g ")})
	if err != nil {
		t.Fatalf("UpdateEnvironment template: %v", err)
	}
	if !reflect.DeepEqual(got.TemplateOffer, offerOf("agent-4c-8g")) || got.NetworkTier != NetworkTierCustom {
		t.Fatalf("after defaultTemplate = %+v", got)
	}
	if read, err := svc.Environment(caller("viewer"), ws); err != nil || !reflect.DeepEqual(read.TemplateOffer, got.TemplateOffer) {
		t.Fatalf("read back template = %+v err=%v", read, err)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{DefaultTemplate: ptr("")})
	if err != nil || !reflect.DeepEqual(got.TemplateOffer, offerOf("agent")) {
		t.Fatalf("clear template = %+v err=%v, want agent", got, err)
	}

	// The GitHub CLI setting (w10/m524): off reads back to every member, a
	// later update of another field keeps it, and on restores it.
	got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{GitHubCLIEnabled: ptr(false)})
	if err != nil || got.GitHubCLIEnabled || got.NetworkTier != NetworkTierCustom {
		t.Fatalf("turning gh off = %+v err=%v", got, err)
	}
	if got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{DefaultModel: ptr("claude-sonnet-4-5")}); err != nil || got.GitHubCLIEnabled {
		t.Fatalf("a model update turned gh back on: %+v err=%v", got, err)
	}
	if read, err := svc.Environment(caller("viewer"), ws); err != nil || read.GitHubCLIEnabled {
		t.Fatalf("read back gh = %+v err=%v, want off", read, err)
	}
	if got, err = svc.UpdateEnvironment(caller("admin"), ws, EnvironmentInput{GitHubCLIEnabled: ptr(true)}); err != nil || !got.GitHubCLIEnabled {
		t.Fatalf("turning gh back on = %+v err=%v", got, err)
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
		if _, err := svc.UpdateEnvironment(caller(subject), ws, EnvironmentInput{GitHubCLIEnabled: ptr(false)}); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s turning gh off = %v, want forbidden", subject, err)
		}
	}
	if got, err := svc.Environment(caller("admin"), ws); err != nil || got.NetworkTier != NetworkTierRegistries || !got.GitHubCLIEnabled {
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
	exerciseEnvironmentValidation(t, func() (*Service, string) { return environmentService("tea-a", newFakeEnvironments()), "tea-a" })

	enablePlatformCatalog(t)

	// On the platform route the default is any catalog model (w10/m516); a
	// model outside it is the typed 400 listing the offered ones.
	routed := environmentService("tea-pinned", newFakeEnvironments())
	_, err := routed.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{DefaultModel: ptr("claude-sonnet-4-5")})
	var coded *core.CodedError
	if !isCode(err, CodeEnvironmentModelNotOffered) || !errors.As(err, &coded) ||
		coded.Params["field"] != "defaultModel" || coded.Params["model"] != "claude-sonnet-4-5" || !slices.Equal(coded.Params["models"].([]string), testCatalog) {
		t.Fatalf("default model outside the catalog = %v, want %s with the offered models", err, CodeEnvironmentModelNotOffered)
	}
	for _, model := range testCatalog {
		got, err := routed.UpdateEnvironment(caller("admin"), "tea-pinned", EnvironmentInput{DefaultModel: ptr(model)})
		if err != nil || got.DefaultModel == nil || *got.DefaultModel != model || !slices.Equal(got.Models, testCatalog) ||
			!got.ModelPinned || got.PinnedModel == nil || *got.PinnedModel != agentsession.DefaultDevModel {
			t.Fatalf("catalog default %q = %+v err=%v", model, got, err)
		}
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
		{"unregistered template", EnvironmentInput{DefaultTemplate: ptr("agent-8c-16g")}, CodeEnvironmentTemplateInvalid},
		{"internal template", EnvironmentInput{DefaultTemplate: ptr("base")}, CodeEnvironmentTemplateInvalid},
		{"idle timeout under 5 minutes", EnvironmentInput{IdleTimeoutMinutes: ptr(4)}, IdleTimeoutInvalidCode},
		{"idle timeout over 120 minutes", EnvironmentInput{IdleTimeoutMinutes: ptr(121)}, IdleTimeoutInvalidCode},
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
	if in.DefaultTemplate != nil {
		out["defaultTemplate"] = *in.DefaultTemplate
	}
	if in.IdleTimeoutMinutes != nil {
		out[idleTimeoutField] = *in.IdleTimeoutMinutes
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
	// Embedded structs (TemplateOffer) serialize their fields inline.
	for _, field := range reflect.VisibleFields(reflect.TypeFor[Environment]()) {
		if !field.Anonymous {
			wantFields[strings.Split(field.Tag.Get("json"), ",")[0]] = true
		}
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

// w10/m524: the GitHub CLI setting is the same field on REST, GraphQL and
// MCP. Each surface reads it as true for a workspace that never saved an
// environment, an admin's write of false on one reads back false on the
// others, and a member's write is refused on each.
func TestEnvironmentGitHubCLISettingOnEverySurface(t *testing.T) {
	svc := environmentService("tea-a", newFakeEnvironments())
	schema := environmentSchema(t, svc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	restRead := func() any {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-environment?ownerId=tea-a", nil).WithContext(caller("viewer")))
		var body map[string]any
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
			t.Fatalf("REST read = %d %s", rec.Code, rec.Body.String())
		}
		return body["githubCliEnabled"]
	}
	gqlRead := func() any {
		t.Helper()
		result := graphql.Do(graphql.Params{Schema: schema, Context: caller("viewer"), RequestString: `{ agentEnvironment(ownerId: "tea-a") { githubCliEnabled } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL read: %v", result.Errors)
		}
		return result.Data.(map[string]any)["agentEnvironment"].(map[string]any)["githubCliEnabled"]
	}
	mcpRead := func() string {
		t.Helper()
		res, err := newMCPClient(t, caller("viewer"), svc).CallTool(caller("viewer"), &mcp.CallToolParams{Name: "get_agent_environment", Arguments: map[string]any{"ownerId": "tea-a"}})
		if err != nil || res.IsError {
			t.Fatalf("MCP read = %+v err=%v", res, err)
		}
		return res.Content[0].(*mcp.TextContent).Text
	}
	if restRead() != true || gqlRead() != true || !strings.Contains(mcpRead(), `"githubCliEnabled":true`) {
		t.Fatalf("defaults: REST %v GraphQL %v MCP %s, want gh on", restRead(), gqlRead(), mcpRead())
	}

	// A member cannot change it on any surface.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"githubCliEnabled":false}`)).WithContext(caller("dev")))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("REST member write = %d, want 403", rec.Code)
	}
	if result := graphql.Do(graphql.Params{Schema: schema, Context: caller("dev"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {githubCliEnabled: false}) { githubCliEnabled } }`}); len(result.Errors) == 0 {
		t.Fatal("GraphQL member write succeeded")
	}
	if res, err := newMCPClient(t, caller("dev"), svc).CallTool(caller("dev"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "githubCliEnabled": false}}); err != nil || !res.IsError {
		t.Fatalf("MCP member write = %+v err=%v, want a refusal", res, err)
	}
	if restRead() != true {
		t.Fatal("a refused write turned gh off")
	}

	// An admin's write on each surface reads back on the others.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(`{"githubCliEnabled":false}`)).WithContext(caller("admin")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"githubCliEnabled":false`) {
		t.Fatalf("REST admin write = %d %s", rec.Code, rec.Body.String())
	}
	if gqlRead() != false || !strings.Contains(mcpRead(), `"githubCliEnabled":false`) {
		t.Fatalf("after the REST write: GraphQL %v MCP %s, want off", gqlRead(), mcpRead())
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: `mutation { updateAgentEnvironment(ownerId: "tea-a", input: {githubCliEnabled: true}) { githubCliEnabled networkTier } }`})
	if len(result.Errors) != 0 || result.Data.(map[string]any)["updateAgentEnvironment"].(map[string]any)["githubCliEnabled"] != true {
		t.Fatalf("GraphQL admin write = %#v %v", result.Data, result.Errors)
	}
	if restRead() != true {
		t.Fatal("the GraphQL write did not reach REST")
	}
	res, err := newMCPClient(t, caller("admin"), svc).CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: map[string]any{"ownerId": "tea-a", "githubCliEnabled": false}})
	if err != nil || res.IsError {
		t.Fatalf("MCP admin write = %+v err=%v", res, err)
	}
	if restRead() != false || gqlRead() != false {
		t.Fatal("the MCP write did not reach REST and GraphQL")
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

// The tier decides the session's egress: only `custom` opens its hosts, and
// `full` selects the proxy posture (w10/m19) with the proxy env on the
// driver. The API allowlist is merged in every tier; under `full` the policy
// renderer drops it, since the proxy reaches any public host.
func TestCreateEgressFollowsTheWorkspaceTier(t *testing.T) {
	hosts := []string{"internal-mirror.example.com"}
	for _, tc := range []struct {
		tier string
		want []string
		full bool
	}{
		{NetworkTierRegistries, []string{"docs.example.com"}, false},
		{NetworkTierCustom, []string{"docs.example.com", "internal-mirror.example.com"}, false},
		{NetworkTierFull, []string{"docs.example.com"}, true},
	} {
		t.Run(tc.tier, func(t *testing.T) {
			svc, _, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: tc.tier, CustomHosts: hosts})
			view, err := svc.Create(caller("alice"), createInput())
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(lifecycle.egressAllowlist, tc.want) || lifecycle.fullInternet != tc.full {
				t.Fatalf("egress = %v full=%v, want %v full=%v", lifecycle.egressAllowlist, lifecycle.fullInternet, tc.want, tc.full)
			}
			env := lifecycle.driverEnv
			if env["BEX_AGENT_NETWORK_TIER"] != tc.tier {
				t.Fatalf("BEX_AGENT_NETWORK_TIER = %q, want %q", env["BEX_AGENT_NETWORK_TIER"], tc.tier)
			}
			if !tc.full {
				for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy", "NO_PROXY", "BEX_AGENT_JAVA_PROXY_OPTIONS"} {
					if _, ok := env[name]; ok {
						t.Fatalf("%s tier sandbox carries %s", tc.tier, name)
					}
				}
				return
			}
			want, _ := agentsession.EgressProxyURL(defaultEgressProxyURL, store.SandboxNamespace("tea-a"), view.ID)
			for _, name := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
				if env[name] != want {
					t.Fatalf("%s = %q, want %q", name, env[name], want)
				}
			}
			noProxy := strings.Split(env["NO_PROXY"], ",")
			for _, host := range []string{"localhost", "127.0.0.1", "bex-ssh-gateway.bex-system.svc.cluster.local"} {
				if !slices.Contains(noProxy, host) {
					t.Fatalf("NO_PROXY %q misses %s, so Git/model traffic would leave its dedicated proxy", env["NO_PROXY"], host)
				}
			}
			if env["no_proxy"] != env["NO_PROXY"] {
				t.Fatalf("no_proxy = %q, want NO_PROXY", env["no_proxy"])
			}
			// w10/032: the JVM reads none of the above, so Java tools get the
			// same proxy, claim and direct hosts as system properties.
			u, _ := url.Parse(want)
			user := u.User.Username()
			password, _ := u.User.Password()
			gateway := "bex-ssh-gateway.bex-system.svc.cluster.local"
			wantJava := "-Dhttp.proxyHost=" + gateway + " -Dhttp.proxyPort=8085 -Dhttp.proxyUser=" + user + " -Dhttp.proxyPassword=" + password +
				" -Dhttps.proxyHost=" + gateway + " -Dhttps.proxyPort=8085 -Dhttps.proxyUser=" + user + " -Dhttps.proxyPassword=" + password +
				" -Dhttp.nonProxyHosts=localhost|127.*|[::1]|0.0.0.0|[::0]|" + gateway + " -Djdk.http.auth.tunneling.disabledSchemes="
			if got := env["BEX_AGENT_JAVA_PROXY_OPTIONS"]; got != wantJava {
				t.Fatalf("BEX_AGENT_JAVA_PROXY_OPTIONS =\n%q\nwant\n%q", got, wantJava)
			}
		})
	}
}

// The JVM properties follow a configured proxy origin and every NO_PROXY
// host, and a portless origin gets HTTP's port, as curl reads it.
func TestJavaProxyOptionsFollowTheProxyURLAndNoProxy(t *testing.T) {
	proxy, err := agentsession.EgressProxyURL("http://egress.dev-10.svc:18085", "tea-a-sandbox", "ags-1")
	if err != nil {
		t.Fatal(err)
	}
	got := javaProxyOptions(proxy, []string{"localhost", "127.0.0.1", "::1", "git.dev-10.svc", "egress.dev-10.svc"})
	for _, want := range []string{
		"-Dhttps.proxyHost=egress.dev-10.svc", "-Dhttps.proxyPort=18085",
		"-Dhttps.proxyUser=dGVhLWEtc2FuZGJveA", "-Dhttps.proxyPassword=YWdzLTE",
		"-Dhttp.nonProxyHosts=localhost|127.*|[::1]|0.0.0.0|[::0]|git.dev-10.svc|egress.dev-10.svc",
	} {
		if !slices.Contains(strings.Fields(got), want) {
			t.Fatalf("options %q miss %q", got, want)
		}
	}
	portless, _ := agentsession.EgressProxyURL("http://egress.example", "ns", "s")
	if got := javaProxyOptions(portless, nil); !strings.Contains(got, "-Dhttps.proxyPort=80 ") {
		t.Fatalf("portless proxy options %q, want port 80", got)
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

// w10/m516 t002: on the platform route a session chooses its model from the
// catalog. A model outside it is a typed 400 at create listing the offered
// ones — never a gateway 403 mid-turn; no model runs the catalog's default;
// a catalog model reaches all four sandbox names. A session whose stored
// model left the catalog steers onto the default instead of failing.
func TestPlatformRouteCatalogAtCreateAndSteer(t *testing.T) {
	enablePlatformCatalog(t)
	svc, st, lifecycle := sessionWithEnvironment(t, nil)
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Model: "anthropic/claude-sonnet-4.5", Task: "fix the tests"}
	_, err := svc.Create(caller("alice"), input)
	if !errors.Is(err, core.ErrBadRequest) || !isCode(err, CodeSessionModelNotOffered) {
		t.Fatalf("create with a model outside the catalog = %v, want %s", err, CodeSessionModelNotOffered)
	}
	var coded *core.CodedError
	if errors.As(err, &coded); coded.Params["field"] != "agentConfig.model" || coded.Params["model"] != "anthropic/claude-sonnet-4.5" ||
		!slices.Equal(coded.Params["models"].([]string), testCatalog) {
		t.Fatalf("refusal params = %v", coded.Params)
	}
	if lifecycle.created != 0 {
		t.Fatal("a refused create provisioned a sandbox")
	}

	for model, want := range map[string]string{"": agentsession.DefaultDevModel, agentsession.DefaultDevModel: agentsession.DefaultDevModel, testCatalog[2]: testCatalog[2]} {
		input.AgentConfig.Model = model
		created, err := svc.Create(caller("alice"), input)
		if err != nil {
			t.Fatalf("create with model %q: %v", model, err)
		}
		if created.AgentConfig.Model != want {
			t.Fatalf("model %q = view %q, want %q", model, created.AgentConfig.Model, want)
		}
		for _, name := range routeModelVars {
			if lifecycle.driverEnv[name] != want {
				t.Fatalf("model %q: %s = %q, want %q", model, name, lifecycle.driverEnv[name], want)
			}
		}
	}

	// A session created before the workspace joined the route stored another
	// model; steering it runs the default without an error.
	input.AgentConfig.Model = ""
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	row.AgentConfig = []byte(`{"agent":"claude","model":"anthropic/claude-sonnet-4.5","task":"fix the tests"}`)
	st.rows[created.ID] = row
	steered, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "and the docs"})
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if steered.AgentConfig.Model != agentsession.DefaultDevModel {
		t.Fatalf("steer of a model that left the catalog = %q, want the default", steered.AgentConfig.Model)
	}
	for _, name := range routeModelVars {
		if lifecycle.driverEnv[name] != agentsession.DefaultDevModel {
			t.Fatalf("steer %s = %q, want the default", name, lifecycle.driverEnv[name])
		}
	}

	// codex is not served by the route: its explicit model is its own.
	input.AgentConfig = AgentConfig{Agent: "codex", Model: "gpt-5", Task: "fix the tests"}
	if created, err := svc.Create(caller("alice"), input); err != nil || created.AgentConfig.Model != "gpt-5" {
		t.Fatalf("codex in a routed workspace = %+v err=%v", created.AgentConfig, err)
	}
}

// A workspace default in the catalog is what a session that names no model
// runs; one that left the catalog falls back to the catalog's first.
func TestPlatformRouteRunsTheWorkspaceDefault(t *testing.T) {
	enablePlatformCatalog(t)
	for stored, want := range map[string]string{testCatalog[1]: testCatalog[1], "anthropic/claude-sonnet-4.5": agentsession.DefaultDevModel} {
		svc, _, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, DefaultModel: stored})
		input := createInput()
		input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
		created, err := svc.Create(caller("alice"), input)
		if err != nil {
			t.Fatal(err)
		}
		if created.AgentConfig.Model != want || lifecycle.driverEnv["ANTHROPIC_MODEL"] != want {
			t.Fatalf("default %q = view %q env %q, want %q", stored, created.AgentConfig.Model, lifecycle.driverEnv["ANTHROPIC_MODEL"], want)
		}
	}
}

// Without the platform route the explicit model passes through unchanged.
func TestModelWithoutThePlatformRouteIsUnpinned(t *testing.T) {
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

// Steering into and back out of `full` re-renders both the posture and the
// sandbox proxy env from the workspace's current tier.
func TestSteerFollowsAFullTierSwitch(t *testing.T) {
	svc, st, lifecycle := sessionWithEnvironment(t, nil)
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	envs := svc.Environments.(*fakeEnvironments)
	for _, tier := range []string{NetworkTierFull, NetworkTierRegistries} {
		row := st.rows[created.ID]
		row.Phase = PhaseCompleted
		st.rows[created.ID] = row
		envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: tier}
		if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again"}); err != nil {
			t.Fatal(err)
		}
		full := tier == NetworkTierFull
		_, hasProxy := lifecycle.driverEnv["HTTPS_PROXY"]
		if lifecycle.fullInternet != full || hasProxy != full || lifecycle.driverEnv["BEX_AGENT_NETWORK_TIER"] != tier {
			t.Fatalf("steer under %s: full=%v HTTPS_PROXY=%v tier env=%q", tier, lifecycle.fullInternet, hasProxy, lifecycle.driverEnv["BEX_AGENT_NETWORK_TIER"])
		}
	}
}

// w10/m524: a repo session dispatched while its workspace has gh off tells
// the driver so (BEX_AGENT_GITHUB_CLI=0); the next turn after an admin turns
// it back on does not, and a repo-less session, which has no gh, never does.
func TestDispatchTellsTheDriverWhenGitHubCLIIsOff(t *testing.T) {
	svc, st, lifecycle := sessionWithEnvironment(t, &store.AgentEnvironment{NetworkTier: NetworkTierRegistries, GitHubCLIDisabled: true})
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if lifecycle.driverEnv[envGitHubCLI] != "0" {
		t.Fatalf("create with gh off: %s = %q, want 0", envGitHubCLI, lifecycle.driverEnv[envGitHubCLI])
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row
	svc.Environments.(*fakeEnvironments).rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if value, ok := lifecycle.driverEnv[envGitHubCLI]; ok {
		t.Fatalf("steer with gh back on: %s = %q, want it absent", envGitHubCLI, value)
	}

	chat := (&Service{}).driverEnv(AgentConfig{Agent: "claude"}, store.AgentSession{ID: "ags-y", WorkspaceID: "tea-a", Turns: 1}, store.AgentEnvironment{GitHubCLIDisabled: true}, agentsession.GitHubCLIWorkspace)
	if value, ok := chat[envGitHubCLI]; ok {
		t.Fatalf("a repo-less session with gh off: %s = %q, want it absent", envGitHubCLI, value)
	}
}

// w7/m392: admins choose the available agents. Muse Code (whose provider may
// train on session data, ADR108) starts off and turns on only with an
// acknowledgement, which the first admin's record keeps; unknown ids and
// stray acknowledgements are refused; members cannot change the list.
func TestEnvironmentAgentAvailability(t *testing.T) {
	envs := newFakeEnvironments()
	svc := environmentService("tea-a", envs)

	_, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledAgents: &[]string{"claude", "muse"}})
	if !isCode(err, DataUseUnacknowledgedCode) {
		t.Fatalf("enable muse without acknowledgement = %v, want %s", err, DataUseUnacknowledgedCode)
	}
	for _, input := range []EnvironmentInput{
		{EnabledAgents: &[]string{"claude", "copilot"}},
		{AcknowledgeAgentDataUse: []string{"codex"}},
	} {
		if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", input); !isCode(err, AgentListInvalidCode) {
			t.Fatalf("update %+v = %v, want %s", input, err, AgentListInvalidCode)
		}
	}
	if _, err := svc.UpdateEnvironment(caller("dev"), "tea-a", EnvironmentInput{EnabledAgents: &[]string{"claude"}}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("member update = %v, want forbidden", err)
	}
	if len(envs.rows) != 0 {
		t.Fatalf("a refused update wrote %+v", envs.rows)
	}

	// An acknowledgement alone records it but turns nothing on.
	got, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{AcknowledgeAgentDataUse: []string{"muse"}})
	if err != nil || !slices.Equal(got.EnabledAgents, []string{"claude", "codex", "deepseek", "gemini"}) || got.AgentDataUse[0].AcknowledgedAt == nil {
		t.Fatalf("acknowledgement alone = %+v / %+v, %v", got.EnabledAgents, got.AgentDataUse, err)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{
		EnabledAgents: &[]string{"claude", "muse"}, AcknowledgeAgentDataUse: []string{"muse"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.EnabledAgents, []string{"claude", "muse"}) || len(got.AgentDataUse) != 1 ||
		got.AgentDataUse[0].AcknowledgedAt == nil || *got.AgentDataUse[0].AcknowledgedBy != "admin" {
		t.Fatalf("after enabling muse = %+v / %+v", got.EnabledAgents, got.AgentDataUse)
	}
	if row := envs.rows["tea-a"]; !slices.Equal(row.DisabledAgents, []string{"codex", "deepseek", "gemini"}) {
		t.Fatalf("stored disabled agents = %v, want codex, deepseek and gemini", row.DisabledAgents)
	}
	// Turning muse off and on again needs no new acknowledgement, and the
	// first one is kept.
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledAgents: &[]string{"claude"}}); err != nil {
		t.Fatal(err)
	}
	got, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{EnabledAgents: &[]string{"claude", "muse", "codex"}})
	if err != nil || !slices.Equal(got.EnabledAgents, []string{"claude", "codex", "muse"}) {
		t.Fatalf("re-enable = %+v, %v", got.EnabledAgents, err)
	}
}

// A turned-off agent disappears from capabilities and cannot start a
// session on any path (they all run resolveCreateModel); a session that is
// already running can still be steered.
func TestDisabledAgentIsRefusedAtCreateButKeepsSteering(t *testing.T) {
	svc, _, _, _ := fixture()
	running, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	envs := newFakeEnvironments()
	envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, DisabledAgents: []string{"codex"}}
	svc.Environments = envs

	caps, err := svc.Capabilities(caller("alice"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range caps.Agents {
		if profile.ID == "codex" || profile.ID == "muse" {
			t.Fatalf("capabilities offered %s: %+v", profile.ID, caps.Agents)
		}
	}
	second := createInput()
	second.Branch = "bex-agent/session-second"
	if _, err := svc.Create(caller("alice"), second); !isCode(err, AgentDisabledCode) {
		t.Fatalf("create a turned-off agent = %v, want %s", err, AgentDisabledCode)
	}
	if !agentEnabled(envs.rows["tea-a"], "claude") || agentEnabled(envs.rows["tea-a"], "muse") {
		t.Fatal("claude must stay on and muse off without its acknowledgement")
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: running.ID, Prompt: "keep going"}); isCode(err, AgentDisabledCode) {
		t.Fatalf("steering a running session of a turned-off agent was refused: %v", err)
	}
}
