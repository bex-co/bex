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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w2/m533 t003: REST, GraphQL, and MCP expose all 7 sandbox group credential
// operations identically over Service's shared verbs. Every scenario runs once
// per surface on an isolated fixture and compares normalized fields, sorted
// order, version history + restoredFromVersionId, split Admin/Developer/Viewer
// RBAC, write-only secret redaction, and coded 400/403/404/409 errors.

type credSurfaceOutcome struct {
	Code    string
	RawBody string
	Fields  any
}

type credSurfaceDriver interface {
	list(actor, groupID string) credSurfaceOutcome
	get(actor, groupID, credID string) credSurfaceOutcome
	versions(actor, groupID, credID string) credSurfaceOutcome
	create(actor, groupID string, req CreateGroupCredentialRequest) credSurfaceOutcome
	update(actor, groupID, credID string, req UpdateGroupCredentialRequest) credSurfaceOutcome
	delete(actor, groupID, credID string) credSurfaceOutcome
	restore(actor, groupID, credID string, versionID int64) credSurfaceOutcome
}

type credSurfaceFixture struct {
	ws             string
	otherWS        string
	groupID        string
	foreignGroupID string
	svc            *Service
}

func newCredSurfaceFixture() *credSurfaceFixture {
	ws := ids.New(ids.Workspace)
	otherWS := ids.New(ids.Workspace)
	groupID := ids.New(ids.SandboxGroup)
	foreignGroupID := ids.New(ids.SandboxGroup)

	st := newMemCredStore(ws, groupID)
	st.groups[foreignGroupID] = store.SandboxGroup{
		ID: foreignGroupID, WorkspaceID: otherWS, Name: "Other", Region: "oregon", ConcurrencyLimit: 10,
	}
	base := &core.Base{
		Authz: roleAuthzChecker{roles: map[string]string{
			"viewer-1": "viewer",
			"dev-1":    "developer",
			"admin-1":  "admin",
		}},
		Workspace: credWorkspaceResolver{members: map[string][]string{
			"viewer-1": {ws},
			"dev-1":    {ws},
			"admin-1":  {ws},
		}},
		Audit: &credAuditSpy{},
	}
	return &credSurfaceFixture{
		ws:             ws,
		otherWS:        otherWS,
		groupID:        groupID,
		foreignGroupID: foreignGroupID,
		svc: &Service{
			Base:            base,
			Groups:          st,
			Credentials:     st,
			CredentialVault: NewCredentialVault(newMemCredentialKV()),
		},
	}
}

func credActorCtx(actor string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{
		Subject: actor, Method: core.MethodSession, Human: true,
	})
}

func normalizeCredPayload(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if k == "updatedAt" {
				// Timestamp differs across runs; assert presence/non-empty separately.
				continue
			}
			// Omit null optional fields so GraphQL's explicit null matches REST/MCP omitempty.
			if val == nil && (k == "name" || k == "template" || k == "restoredFromVersionId") {
				continue
			}
			out[k] = normalizeCredPayload(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizeCredPayload(val)
		}
		return out
	default:
		return v
	}
}

func parseAndNormalizeJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal %q: %v", raw, err)
	}
	return normalizeCredPayload(v)
}

// --- REST driver ---

type credRESTDriver struct {
	t   *testing.T
	mux *http.ServeMux
}

func newCredRESTDriver(t *testing.T, svc *Service) credRESTDriver {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return credRESTDriver{t: t, mux: mux}
}

func (d credRESTDriver) do(actor, method, path string, body any) credSurfaceOutcome {
	d.t.Helper()
	var rawReq string
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			d.t.Fatal(err)
		}
		rawReq = string(b)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(rawReq))
	if rawReq != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	d.mux.ServeHTTP(rec, req.WithContext(credActorCtx(actor)))
	respBody := rec.Body.String()
	if rec.Code >= 400 {
		code := errCode(respBody)
		if code == "" && rec.Code == http.StatusForbidden {
			code = "FORBIDDEN"
		}
		return credSurfaceOutcome{Code: code, RawBody: respBody}
	}
	if rec.Code == http.StatusAccepted || rec.Code == http.StatusNoContent {
		return credSurfaceOutcome{RawBody: respBody, Fields: map[string]any{"deleted": true}}
	}
	return credSurfaceOutcome{RawBody: respBody, Fields: parseAndNormalizeJSON(d.t, respBody)}
}

func (d credRESTDriver) list(actor, groupID string) credSurfaceOutcome {
	return d.do(actor, http.MethodGet, "/v1/sandbox-groups/"+groupID+"/credentials", nil)
}

func (d credRESTDriver) get(actor, groupID, credID string) credSurfaceOutcome {
	return d.do(actor, http.MethodGet, "/v1/sandbox-groups/"+groupID+"/credentials/"+credID, nil)
}

func (d credRESTDriver) versions(actor, groupID, credID string) credSurfaceOutcome {
	return d.do(actor, http.MethodGet, "/v1/sandbox-groups/"+groupID+"/credentials/"+credID+"/versions", nil)
}

func (d credRESTDriver) create(actor, groupID string, req CreateGroupCredentialRequest) credSurfaceOutcome {
	return d.do(actor, http.MethodPost, "/v1/sandbox-groups/"+groupID+"/credentials", req)
}

func (d credRESTDriver) update(actor, groupID, credID string, req UpdateGroupCredentialRequest) credSurfaceOutcome {
	return d.do(actor, http.MethodPatch, "/v1/sandbox-groups/"+groupID+"/credentials/"+credID, req)
}

func (d credRESTDriver) delete(actor, groupID, credID string) credSurfaceOutcome {
	return d.do(actor, http.MethodDelete, "/v1/sandbox-groups/"+groupID+"/credentials/"+credID, nil)
}

func (d credRESTDriver) restore(actor, groupID, credID string, versionID int64) credSurfaceOutcome {
	return d.do(actor, http.MethodPost, "/v1/sandbox-groups/"+groupID+"/credentials/"+credID+"/restore",
		RestoreGroupCredentialRequest{VersionID: versionID})
}

// --- GraphQL driver ---

const gqlCredentialFields = `
	id
	domain
	enabled
	methods
	paths
	injection { type name template }
	version { id updatedAt updatedBy fingerprint restoredFromVersionId }
`

type credGQLDriver struct {
	t      *testing.T
	schema graphql.Schema
}

func newCredGQLDriver(t *testing.T, svc *Service) credGQLDriver {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return credGQLDriver{t: t, schema: schema}
}

func (d credGQLDriver) run(actor, root, query string, vars map[string]any) credSurfaceOutcome {
	d.t.Helper()
	res := graphql.Do(graphql.Params{
		Schema:         d.schema,
		RequestString:  query,
		VariableValues: vars,
		Context:        credActorCtx(actor),
	})
	rawBytes, _ := json.Marshal(res)
	raw := string(rawBytes)
	if len(res.Errors) > 0 {
		code, _ := res.Errors[0].Extensions["code"].(string)
		if code == "" {
			// Fall back to canonical FORBIDDEN for bare core.ErrForbidden refusals.
			if strings.Contains(strings.ToLower(res.Errors[0].Message), "forbidden") {
				code = "FORBIDDEN"
			}
		}
		return credSurfaceOutcome{Code: code, RawBody: raw}
	}
	data, _ := res.Data.(map[string]any)
	fieldBytes, _ := json.Marshal(data[root])
	return credSurfaceOutcome{RawBody: raw, Fields: parseAndNormalizeJSON(d.t, string(fieldBytes))}
}

func (d credGQLDriver) list(actor, groupID string) credSurfaceOutcome {
	return d.run(actor, "sandboxGroupCredentials",
		`query($g: String!) { sandboxGroupCredentials(sandboxGroupId: $g) { `+gqlCredentialFields+` } }`,
		map[string]any{"g": groupID})
}

func (d credGQLDriver) get(actor, groupID, credID string) credSurfaceOutcome {
	return d.run(actor, "sandboxGroupCredential",
		`query($g: String!, $id: String!) { sandboxGroupCredential(sandboxGroupId: $g, id: $id) { `+gqlCredentialFields+` } }`,
		map[string]any{"g": groupID, "id": credID})
}

func (d credGQLDriver) versions(actor, groupID, credID string) credSurfaceOutcome {
	return d.run(actor, "sandboxGroupCredentialVersions",
		`query($g: String!, $id: String!) { sandboxGroupCredentialVersions(sandboxGroupId: $g, id: $id) { id updatedAt updatedBy fingerprint restoredFromVersionId } }`,
		map[string]any{"g": groupID, "id": credID})
}

func (d credGQLDriver) create(actor, groupID string, req CreateGroupCredentialRequest) credSurfaceOutcome {
	in := map[string]any{
		"id":     req.ID,
		"domain": req.Domain,
		"value":  req.Value,
	}
	if req.Enabled != nil {
		in["enabled"] = *req.Enabled
	}
	if req.Methods != nil {
		in["methods"] = req.Methods
	}
	if req.Paths != nil {
		in["paths"] = req.Paths
	}
	if req.Injection != nil {
		inj := map[string]any{"type": req.Injection.Type}
		if req.Injection.Name != "" {
			inj["name"] = req.Injection.Name
		}
		if req.Injection.Template != "" {
			inj["template"] = req.Injection.Template
		}
		in["injection"] = inj
	}
	return d.run(actor, "createSandboxGroupCredential",
		`mutation($g: String!, $in: CreateSandboxGroupCredentialInput!) { createSandboxGroupCredential(sandboxGroupId: $g, input: $in) { `+gqlCredentialFields+` } }`,
		map[string]any{"g": groupID, "in": in})
}

func (d credGQLDriver) update(actor, groupID, credID string, req UpdateGroupCredentialRequest) credSurfaceOutcome {
	in := map[string]any{}
	if req.Value != nil {
		in["value"] = *req.Value
	}
	if req.Domain != nil {
		in["domain"] = *req.Domain
	}
	if req.Enabled != nil {
		in["enabled"] = *req.Enabled
	}
	if req.Methods != nil {
		in["methods"] = *req.Methods
	}
	if req.Paths != nil {
		in["paths"] = *req.Paths
	}
	if req.Injection != nil {
		inj := map[string]any{"type": req.Injection.Type}
		if req.Injection.Name != "" {
			inj["name"] = req.Injection.Name
		}
		if req.Injection.Template != "" {
			inj["template"] = req.Injection.Template
		}
		in["injection"] = inj
	}
	return d.run(actor, "updateSandboxGroupCredential",
		`mutation($g: String!, $id: String!, $in: UpdateSandboxGroupCredentialInput!) { updateSandboxGroupCredential(sandboxGroupId: $g, id: $id, input: $in) { `+gqlCredentialFields+` } }`,
		map[string]any{"g": groupID, "id": credID, "in": in})
}

func (d credGQLDriver) delete(actor, groupID, credID string) credSurfaceOutcome {
	out := d.run(actor, "deleteSandboxGroupCredential",
		`mutation($g: String!, $id: String!) { deleteSandboxGroupCredential(sandboxGroupId: $g, id: $id) }`,
		map[string]any{"g": groupID, "id": credID})
	if out.Code == "" {
		out.Fields = map[string]any{"deleted": true}
	}
	return out
}

func (d credGQLDriver) restore(actor, groupID, credID string, versionID int64) credSurfaceOutcome {
	return d.run(actor, "restoreSandboxGroupCredential",
		`mutation($g: String!, $id: String!, $v: Int!) { restoreSandboxGroupCredential(sandboxGroupId: $g, id: $id, versionId: $v) { `+gqlCredentialFields+` } }`,
		map[string]any{"g": groupID, "id": credID, "v": int(versionID)})
}

// --- MCP driver ---

type credMCPDriver struct {
	t      *testing.T
	svc    *Service
	client map[string]*mcp.ClientSession
}

func newCredMCPDriver(t *testing.T, svc *Service) *credMCPDriver {
	t.Helper()
	d := &credMCPDriver{t: t, svc: svc, client: map[string]*mcp.ClientSession{}}
	return d
}

func (d *credMCPDriver) sessionFor(actor string) *mcp.ClientSession {
	d.t.Helper()
	if cs, ok := d.client[actor]; ok {
		return cs
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "bex-test", Version: "0"}, nil)
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			return next(credActorCtx(actor), method, req)
		}
	})
	d.svc.RegisterMCP(srv)
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(context.Background(), st, nil); err != nil {
		d.t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(func() { _ = cs.Close() })
	d.client[actor] = cs
	return cs
}

func (d *credMCPDriver) call(actor, name string, args map[string]any, unwrapKey string) credSurfaceOutcome {
	d.t.Helper()
	res, err := d.sessionFor(actor).CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		d.t.Fatalf("MCP transport %s: %v", name, err)
	}
	var text string
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			text = tc.Text
		}
	}
	if res.IsError {
		code, _, found := strings.Cut(text, ": ")
		if !found {
			code = ""
		}
		if code == "" && strings.Contains(strings.ToLower(text), "forbidden") {
			code = "FORBIDDEN"
		}
		return credSurfaceOutcome{Code: code, RawBody: text}
	}
	parsed := parseAndNormalizeJSON(d.t, text)
	if unwrapKey != "" {
		if m, ok := parsed.(map[string]any); ok {
			parsed = m[unwrapKey]
		}
	}
	return credSurfaceOutcome{RawBody: text, Fields: parsed}
}

func (d *credMCPDriver) list(actor, groupID string) credSurfaceOutcome {
	return d.call(actor, "list_sandbox_group_credentials", map[string]any{"sandboxGroupId": groupID}, "credentials")
}

func (d *credMCPDriver) get(actor, groupID, credID string) credSurfaceOutcome {
	return d.call(actor, "get_sandbox_group_credential", map[string]any{"sandboxGroupId": groupID, "credentialId": credID}, "")
}

func (d *credMCPDriver) versions(actor, groupID, credID string) credSurfaceOutcome {
	return d.call(actor, "list_sandbox_group_credential_versions", map[string]any{"sandboxGroupId": groupID, "credentialId": credID}, "versions")
}

func (d *credMCPDriver) create(actor, groupID string, req CreateGroupCredentialRequest) credSurfaceOutcome {
	b, _ := json.Marshal(createGroupCredentialArgs{SandboxGroupID: groupID, CreateGroupCredentialRequest: req})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return d.call(actor, "create_sandbox_group_credential", m, "")
}

func (d *credMCPDriver) update(actor, groupID, credID string, req UpdateGroupCredentialRequest) credSurfaceOutcome {
	b, _ := json.Marshal(updateGroupCredentialArgs{
		SandboxGroupID: groupID, CredentialID: credID, UpdateGroupCredentialRequest: req,
	})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return d.call(actor, "update_sandbox_group_credential", m, "")
}

func (d *credMCPDriver) delete(actor, groupID, credID string) credSurfaceOutcome {
	out := d.call(actor, "delete_sandbox_group_credential", map[string]any{"sandboxGroupId": groupID, "credentialId": credID}, "")
	if out.Code == "" {
		out.Fields = map[string]any{"deleted": true}
	}
	return out
}

func (d *credMCPDriver) restore(actor, groupID, credID string, versionID int64) credSurfaceOutcome {
	return d.call(actor, "restore_sandbox_group_credential", map[string]any{
		"sandboxGroupId": groupID, "credentialId": credID, "versionId": versionID,
	}, "")
}

func TestSandboxGroupCredentialsCrossSurfaceParity(t *testing.T) {
	type surfaceRun struct {
		name string
		f    *credSurfaceFixture
		drv  credSurfaceDriver
	}
	makeRuns := func() []surfaceRun {
		fRest := newCredSurfaceFixture()
		fGql := newCredSurfaceFixture()
		fMcp := newCredSurfaceFixture()
		return []surfaceRun{
			{name: "REST", f: fRest, drv: newCredRESTDriver(t, fRest.svc)},
			{name: "GraphQL", f: fGql, drv: newCredGQLDriver(t, fGql.svc)},
			{name: "MCP", f: fMcp, drv: newCredMCPDriver(t, fMcp.svc)},
		}
	}

	secretsToRedact := []string{
		"sk-live-secret-v1-alpha",
		"sk-live-secret-v2-beta",
		"alice:topsecretpass",
	}
	assertNoSecretLeak := func(t *testing.T, label string, out credSurfaceOutcome) {
		t.Helper()
		for _, s := range secretsToRedact {
			if strings.Contains(out.RawBody, s) {
				t.Fatalf("%s leaked raw secret %q in response: %s", label, s, out.RawBody)
			}
		}
	}

	assertAllAgree := func(t *testing.T, step string, runs []surfaceRun, fn func(r surfaceRun) credSurfaceOutcome) []credSurfaceOutcome {
		t.Helper()
		outs := make([]credSurfaceOutcome, len(runs))
		for i, r := range runs {
			outs[i] = fn(r)
			assertNoSecretLeak(t, fmt.Sprintf("%s/%s", step, r.name), outs[i])
		}
		for i := 1; i < len(runs); i++ {
			if outs[i].Code != outs[0].Code {
				t.Errorf("%s: %s error code = %q, %s = %q (raw %s: %s)",
					step, runs[i].name, outs[i].Code, runs[0].name, outs[0].Code, runs[i].name, outs[i].RawBody)
			}
			if outs[0].Code == "" && !reflect.DeepEqual(outs[i].Fields, outs[0].Fields) {
				b0, _ := json.Marshal(outs[0].Fields)
				bi, _ := json.Marshal(outs[i].Fields)
				t.Errorf("%s: %s fields differ from %s:\n  %s: %s\n  %s: %s",
					step, runs[i].name, runs[0].name, runs[0].name, b0, runs[i].name, bi)
			}
		}
		return outs
	}

	runs := makeRuns()

	// 1. Empty list returns [] identically.
	outs := assertAllAgree(t, "empty-list", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.list("viewer-1", r.f.groupID)
	})
	if items, ok := outs[0].Fields.([]any); !ok || len(items) != 0 {
		t.Fatalf("empty list = %#v, want empty array", outs[0].Fields)
	}

	// 2. Developer cannot create (403 FORBIDDEN); Viewer cannot create (403 FORBIDDEN).
	createV1 := CreateGroupCredentialRequest{
		ID:      "z-openai",
		Domain:  "API.OpenAI.com",
		Value:   "sk-live-secret-v1-alpha",
		Methods: []string{"post", "GET"},
		Paths:   []string{"/v1/**"},
		Injection: &GroupCredentialInjection{
			Type:     "header",
			Name:     "Authorization",
			Template: "Bearer ${SECRET}",
		},
	}
	for _, actor := range []string{"dev-1", "viewer-1"} {
		outs = assertAllAgree(t, "create-denied-"+actor, runs, func(r surfaceRun) credSurfaceOutcome {
			return r.drv.create(actor, r.f.groupID, createV1)
		})
		if outs[0].Code != "FORBIDDEN" {
			t.Fatalf("create by %s code = %q, want FORBIDDEN", actor, outs[0].Code)
		}
	}

	// 3. Foreign workspace group answers 404 SANDBOX_GROUP_NOT_FOUND across all surfaces.
	outs = assertAllAgree(t, "foreign-group", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.list("admin-1", r.f.foreignGroupID)
	})
	if outs[0].Code != CodeSandboxGroupNotFound {
		t.Fatalf("foreign group code = %q, want %q", outs[0].Code, CodeSandboxGroupNotFound)
	}

	// 4. Invalid inputs return 400 SANDBOX_CREDENTIAL_INVALID across all surfaces.
	badBasic := CreateGroupCredentialRequest{
		ID:        "bad-basic",
		Domain:    "api.example.com",
		Value:     "alice:topsecretpass",
		Injection: &GroupCredentialInjection{Type: "basic", Name: "Authorization"},
	}
	outs = assertAllAgree(t, "invalid-basic-name", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.create("admin-1", r.f.groupID, badBasic)
	})
	if outs[0].Code != CodeSandboxCredentialInvalid {
		t.Fatalf("invalid basic code = %q, want %q", outs[0].Code, CodeSandboxCredentialInvalid)
	}

	// 5. Admin creates two credentials in reverse lexicographical order to verify sorted list.
	outs = assertAllAgree(t, "create-z-openai", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.create("admin-1", r.f.groupID, createV1)
	})
	if outs[0].Code != "" {
		t.Fatalf("create z-openai failed: %s %s", outs[0].Code, outs[0].RawBody)
	}
	createA := CreateGroupCredentialRequest{
		ID:        "a-basic",
		Domain:    "api.github.com",
		Value:     "alice:topsecretpass",
		Injection: &GroupCredentialInjection{Type: "basic"},
	}
	outs = assertAllAgree(t, "create-a-basic", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.create("admin-1", r.f.groupID, createA)
	})
	if outs[0].Code != "" {
		t.Fatalf("create a-basic failed: %s %s", outs[0].Code, outs[0].RawBody)
	}

	// 6. Duplicate ID answers 409 SANDBOX_CREDENTIAL_ALREADY_EXISTS across all surfaces.
	outs = assertAllAgree(t, "duplicate-create", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.create("admin-1", r.f.groupID, createV1)
	})
	if outs[0].Code != CodeSandboxCredentialExists {
		t.Fatalf("duplicate code = %q, want %q", outs[0].Code, CodeSandboxCredentialExists)
	}

	// 7. Viewer lists credentials — sorted lexicographically [a-basic, z-openai].
	outs = assertAllAgree(t, "list-sorted", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.list("viewer-1", r.f.groupID)
	})
	items := outs[0].Fields.([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != "a-basic" || items[1].(map[string]any)["id"] != "z-openai" {
		t.Fatalf("list order = %#v, want [a-basic, z-openai]", items)
	}

	// 8. Viewer gets single credential; missing credential returns 404 SANDBOX_CREDENTIAL_NOT_FOUND.
	assertAllAgree(t, "get-z-openai", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.get("viewer-1", r.f.groupID, "z-openai")
	})
	outs = assertAllAgree(t, "get-missing", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.get("viewer-1", r.f.groupID, "missing-id")
	})
	if outs[0].Code != CodeSandboxCredentialNotFound {
		t.Fatalf("missing cred code = %q, want %q", outs[0].Code, CodeSandboxCredentialNotFound)
	}

	// 9. Split RBAC update:
	//    - Developer updates enabled + clears methods/paths with explicit empty slices -> v2.
	//    - Developer denied on domain or value (403).
	//    - Viewer denied on enabled (403).
	//    - Admin rotates value -> v3.
	disabled := false
	emptySlice := []string{}
	outs = assertAllAgree(t, "dev-update-allowed", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.update("dev-1", r.f.groupID, "z-openai", UpdateGroupCredentialRequest{
			Enabled: &disabled,
			Methods: &emptySlice,
			Paths:   &emptySlice,
		})
	})
	if outs[0].Code != "" {
		t.Fatalf("dev allowed update failed: %s %s", outs[0].Code, outs[0].RawBody)
	}
	updatedMap := outs[0].Fields.(map[string]any)
	if updatedMap["enabled"] != false || len(updatedMap["methods"].([]any)) != 0 || len(updatedMap["paths"].([]any)) != 0 {
		t.Fatalf("dev update did not clear methods/paths or disable: %#v", updatedMap)
	}

	newDom := "api.anthropic.com"
	outs = assertAllAgree(t, "dev-update-domain-denied", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.update("dev-1", r.f.groupID, "z-openai", UpdateGroupCredentialRequest{Domain: &newDom})
	})
	if outs[0].Code != "FORBIDDEN" {
		t.Fatalf("dev domain update code = %q, want FORBIDDEN", outs[0].Code)
	}

	newSec := "sk-live-secret-v2-beta"
	outs = assertAllAgree(t, "dev-update-value-denied", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.update("dev-1", r.f.groupID, "z-openai", UpdateGroupCredentialRequest{Value: &newSec})
	})
	if outs[0].Code != "FORBIDDEN" {
		t.Fatalf("dev value update code = %q, want FORBIDDEN", outs[0].Code)
	}

	outs = assertAllAgree(t, "viewer-update-denied", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.update("viewer-1", r.f.groupID, "z-openai", UpdateGroupCredentialRequest{Enabled: &disabled})
	})
	if outs[0].Code != "FORBIDDEN" {
		t.Fatalf("viewer update code = %q, want FORBIDDEN", outs[0].Code)
	}

	outs = assertAllAgree(t, "admin-update-value", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.update("admin-1", r.f.groupID, "z-openai", UpdateGroupCredentialRequest{Value: &newSec})
	})
	if outs[0].Code != "" {
		t.Fatalf("admin value update failed: %s %s", outs[0].Code, outs[0].RawBody)
	}

	// 10. Restore: Developer denied (403); missing version 99 -> 404 SANDBOX_CREDENTIAL_VERSION_NOT_FOUND;
	//     Admin restores version 1 -> version 4 with restoredFromVersionId=1.
	outs = assertAllAgree(t, "dev-restore-denied", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.restore("dev-1", r.f.groupID, "z-openai", 1)
	})
	if outs[0].Code != "FORBIDDEN" {
		t.Fatalf("dev restore code = %q, want FORBIDDEN", outs[0].Code)
	}

	outs = assertAllAgree(t, "restore-missing-version", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.restore("admin-1", r.f.groupID, "z-openai", 99)
	})
	if outs[0].Code != CodeSandboxCredentialVersionNotFound {
		t.Fatalf("missing version code = %q, want %q", outs[0].Code, CodeSandboxCredentialVersionNotFound)
	}

	outs = assertAllAgree(t, "admin-restore-v1", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.restore("admin-1", r.f.groupID, "z-openai", 1)
	})
	if outs[0].Code != "" {
		t.Fatalf("admin restore failed: %s %s", outs[0].Code, outs[0].RawBody)
	}
	v4 := outs[0].Fields.(map[string]any)["version"].(map[string]any)
	if v4["id"] != float64(4) || v4["restoredFromVersionId"] != float64(1) {
		t.Fatalf("restored version = %#v, want id=4 restoredFromVersionId=1", v4)
	}

	// 11. Versions list: newest first (4, 3, 2, 1), identical across surfaces.
	outs = assertAllAgree(t, "list-versions", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.versions("viewer-1", r.f.groupID, "z-openai")
	})
	vers := outs[0].Fields.([]any)
	if len(vers) != 4 || vers[0].(map[string]any)["id"] != float64(4) || vers[3].(map[string]any)["id"] != float64(1) {
		t.Fatalf("versions order = %#v, want [4,3,2,1]", vers)
	}

	// 12. Delete: Developer denied (403); Admin deletes (idempotent on repeat).
	outs = assertAllAgree(t, "dev-delete-denied", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.delete("dev-1", r.f.groupID, "z-openai")
	})
	if outs[0].Code != "FORBIDDEN" {
		t.Fatalf("dev delete code = %q, want FORBIDDEN", outs[0].Code)
	}
	assertAllAgree(t, "admin-delete", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.delete("admin-1", r.f.groupID, "z-openai")
	})
	assertAllAgree(t, "admin-delete-idempotent", runs, func(r surfaceRun) credSurfaceOutcome {
		return r.drv.delete("admin-1", r.f.groupID, "z-openai")
	})
}

func TestSandboxGroupCredentialsMCPAnnotations(t *testing.T) {
	f := newCredSurfaceFixture()
	d := newCredMCPDriver(t, f.svc)
	listed, err := d.sessionFor("admin-1").ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		byName[tool.Name] = tool
	}
	want := []struct {
		name        string
		readOnly    bool
		destructive bool
		idempotent  bool
	}{
		{"list_sandbox_group_credentials", true, false, true},
		{"get_sandbox_group_credential", true, false, true},
		{"list_sandbox_group_credential_versions", true, false, true},
		{"create_sandbox_group_credential", false, false, false},
		{"update_sandbox_group_credential", false, true, false},
		{"delete_sandbox_group_credential", false, true, true},
		{"restore_sandbox_group_credential", false, true, false},
	}
	for _, w := range want {
		tool, ok := byName[w.name]
		if !ok {
			t.Fatalf("missing MCP tool %q", w.name)
		}
		if tool.Annotations == nil {
			t.Fatalf("tool %q has nil Annotations", w.name)
		}
		if tool.Annotations.ReadOnlyHint != w.readOnly {
			t.Errorf("%s ReadOnlyHint = %v, want %v", w.name, tool.Annotations.ReadOnlyHint, w.readOnly)
		}
		if tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint != w.destructive {
			t.Errorf("%s DestructiveHint = %v, want %v", w.name, tool.Annotations.DestructiveHint, w.destructive)
		}
		if tool.Annotations.IdempotentHint != w.idempotent {
			t.Errorf("%s IdempotentHint = %v, want %v", w.name, tool.Annotations.IdempotentHint, w.idempotent)
		}
	}
}
