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

package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// organization_login_surfaces_test.go proves an org's login requirement
// (w7/m466, docs/ADR012-auth.md §9c) reaches every surface through the one
// enforcement point: in an org workspace whose own setting is off, a member
// who does not meet the org's requirement gets the workspace setting's coded
// 403 naming the org on REST, GraphQL and MCP; the org's own objects (its
// audit log, its members) refuse them the same way with no workspace named;
// the exempt reads (the workspace list, the org list) still answer. The gate
// is the real twofactor.Gate.

const orgLoginOrg = "org-d5t5d4v8g3c73f5m9pe0"

func orgLoginSurfaces(t *testing.T, org core.LoginOrganization) (*Server, string, string) {
	t.Helper()
	st := newFakeWSStore()
	inOrg := mustCreate(t, st, "org-workspace", "scale", "unenrolled")
	standalone := mustCreate(t, st, "standalone", "pro", "unenrolled")
	for _, workspace := range []string{inOrg.ID, standalone.ID} {
		for _, subject := range []string{"enrolled", "key-by-unenrolled"} {
			st.addMember(workspace, subject, "developer")
		}
	}
	gate := twofactor.New(twofactor.Config{
		Settings: surfaceSettings{
			inOrg.ID:                             core.LoginRequirements{}.WithOrganization(org.ID, org.Name, org.TwoFactor, org.GoogleLogin),
			core.OrganizationObject(orgLoginOrg): core.LoginRequirements{}.WithOrganization(org.ID, org.Name, org.TwoFactor, org.GoogleLogin),
		},
		Enrollment: surfaceEnrollment{"unenrolled": false, "enrolled": true},
		Google:     surfaceGoogle{"unenrolled": true, "enrolled": true},
		Keys:       surfaceKeys{creators: map[string]string{"key-by-unenrolled": "unenrolled"}, signIns: map[string]core.SignIn{"key-by-unenrolled": core.SignInPassword}},
	})
	cl := fakeClient(appWithOwnerLabel("org-web", inOrg.ID), appWithOwnerLabel("alone-web", standalone.ID))
	base := &core.Base{Client: cl, Namespace: "default", Workspace: &apiFakeResolver{store: st}, TwoFactor: gate}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	srv.schema = schema
	return srv, inOrg.ID, standalone.ID
}

// wantOrgRefusal checks a coded refusal's details: the org always, the
// workspace when one is the object.
func wantOrgRefusal(t *testing.T, surface, code string, gotCode string, details map[string]any, workspaceID string) {
	t.Helper()
	if gotCode != code || details["organizationId"] != orgLoginOrg || details["organizationName"] != "Acme" {
		t.Fatalf("%s = %s %v, want %s naming org %s", surface, gotCode, details, code, orgLoginOrg)
	}
	if got, _ := details["workspaceId"].(string); got != workspaceID {
		t.Fatalf("%s workspaceId = %q, want %q", surface, got, workspaceID)
	}
}

func TestAnOrganizationsTwoFactorRequirementRefusesOnEverySurfaceNamingTheOrg(t *testing.T) {
	srv, inOrg, standalone := orgLoginSurfaces(t, core.LoginOrganization{ID: orgLoginOrg, Name: "Acme", TwoFactor: true})
	member := sessionOf("unenrolled")

	t.Run("REST", func(t *testing.T) {
		status, body := restDo(t, srv, member, http.MethodGet, "/v1/services?ownerId="+inOrg)
		params, _ := body["params"].(map[string]any)
		code, _ := body["code"].(string)
		if status != http.StatusForbidden {
			t.Fatalf("REST = %d %v", status, body)
		}
		wantOrgRefusal(t, "REST", twoFactorCode, code, params, inOrg)
	})
	t.Run("GraphQL", func(t *testing.T) {
		out := graphqlDo(t, srv, member, `{ services(ownerId: "`+inOrg+`") { id } }`)
		ext := map[string]any{}
		if errs, _ := out["errors"].([]any); len(errs) > 0 {
			ext, _ = errs[0].(map[string]any)["extensions"].(map[string]any)
		}
		wantOrgRefusal(t, "GraphQL", twoFactorCode, graphqlCode(out), ext, inOrg)
	})
	t.Run("MCP", func(t *testing.T) {
		res := mcpCall(t, srv, member, "list_services", map[string]any{"workspaceId": inOrg})
		if !res.IsError || !strings.HasPrefix(mcpText(res), twoFactorCode+":") || !strings.Contains(mcpText(res), "Acme") {
			t.Fatalf("MCP = error=%v %q, want the 2FA refusal naming the org", res.IsError, mcpText(res))
		}
	})
	t.Run("an API key borrows its creator's factor", func(t *testing.T) {
		status, body := restDo(t, srv, keyOf("key-by-unenrolled"), http.MethodGet, "/v1/services?ownerId="+inOrg)
		params, _ := body["params"].(map[string]any)
		code, _ := body["code"].(string)
		if status != http.StatusForbidden {
			t.Fatalf("REST as the key = %d %v", status, body)
		}
		wantOrgRefusal(t, "REST as the key", twoFactorCode, code, params, inOrg)
	})
	t.Run("the org's own objects", func(t *testing.T) {
		status, body := restDo(t, srv, member, http.MethodGet, "/v1/organizations/"+orgLoginOrg+"/audit-logs")
		params, _ := body["params"].(map[string]any)
		code, _ := body["code"].(string)
		if status != http.StatusForbidden {
			t.Fatalf("the org audit log = %d %v", status, body)
		}
		wantOrgRefusal(t, "the org audit log", twoFactorCode, code, params, "")
		out := graphqlDo(t, srv, member, `{ organizationMembers(organizationId: "`+orgLoginOrg+`") { subject } }`)
		if got := graphqlCode(out); got != twoFactorCode {
			t.Fatalf("GraphQL organizationMembers = %v, want %s", out, twoFactorCode)
		}
		if res := mcpCall(t, srv, member, "list_organization_members", map[string]any{"organizationId": orgLoginOrg}); !res.IsError || !strings.HasPrefix(mcpText(res), twoFactorCode+":") {
			t.Fatalf("MCP list_organization_members = error=%v %q", res.IsError, mcpText(res))
		}
	})
	t.Run("an enrolled member and a standalone workspace are untouched", func(t *testing.T) {
		if status, _ := restDo(t, srv, sessionOf("enrolled"), http.MethodGet, "/v1/services?ownerId="+inOrg); status != http.StatusOK {
			t.Fatalf("an enrolled member = %d, want 200", status)
		}
		if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/services?ownerId="+standalone); status != http.StatusOK {
			t.Fatalf("the standalone workspace = %d, want 200", status)
		}
	})
	t.Run("the exempt reads still answer", func(t *testing.T) {
		if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/owners"); status != http.StatusOK {
			t.Fatalf("GET /v1/owners = %d, want 200", status)
		}
		if out := graphqlDo(t, srv, member, `{ workspaces { id } }`); graphqlCode(out) != "" {
			t.Fatalf("GraphQL workspaces = %v, want no error", out)
		}
	})
}

func TestAnOrganizationsGoogleLoginRequirementRefusesOnEverySurfaceNamingTheOrg(t *testing.T) {
	srv, inOrg, standalone := orgLoginSurfaces(t, core.LoginOrganization{ID: orgLoginOrg, Name: "Acme", GoogleLogin: true})
	password := core.Identity{Subject: "enrolled", Method: core.MethodSession, Human: true, SignIn: core.SignInPassword}
	const code = "WORKSPACE_GOOGLE_LOGIN_REQUIRED"

	status, body := restDo(t, srv, password, http.MethodGet, "/v1/services?ownerId="+inOrg)
	params, _ := body["params"].(map[string]any)
	got, _ := body["code"].(string)
	if status != http.StatusForbidden {
		t.Fatalf("REST = %d %v", status, body)
	}
	wantOrgRefusal(t, "REST", code, got, params, inOrg)
	if out := graphqlDo(t, srv, password, `{ services(ownerId: "`+inOrg+`") { id } }`); graphqlCode(out) != code {
		t.Fatalf("GraphQL = %v, want %s", out, code)
	}
	if res := mcpCall(t, srv, password, "list_services", map[string]any{"workspaceId": inOrg}); !res.IsError || !strings.HasPrefix(mcpText(res), code+":") {
		t.Fatalf("MCP = error=%v %q", res.IsError, mcpText(res))
	}
	// A key created in a password sign-in is refused too.
	status, body = restDo(t, srv, keyOf("key-by-unenrolled"), http.MethodGet, "/v1/services?ownerId="+inOrg)
	params, _ = body["params"].(map[string]any)
	got, _ = body["code"].(string)
	if status != http.StatusForbidden {
		t.Fatalf("REST as a password-made key = %d %v", status, body)
	}
	wantOrgRefusal(t, "REST as a password-made key", code, got, params, inOrg)
	// The org's own audit log refuses the password session, naming no workspace.
	status, body = restDo(t, srv, password, http.MethodGet, "/v1/organizations/"+orgLoginOrg+"/audit-logs")
	params, _ = body["params"].(map[string]any)
	got, _ = body["code"].(string)
	if status != http.StatusForbidden {
		t.Fatalf("the org audit log = %d %v", status, body)
	}
	wantOrgRefusal(t, "the org audit log", code, got, params, "")

	google := password
	google.SignIn = core.SignInGoogle
	if status, _ := restDo(t, srv, google, http.MethodGet, "/v1/services?ownerId="+inOrg); status != http.StatusOK {
		t.Fatalf("a Google session = %d, want 200", status)
	}
	if status, _ := restDo(t, srv, password, http.MethodGet, "/v1/services?ownerId="+standalone); status != http.StatusOK {
		t.Fatalf("the standalone workspace = %d, want 200", status)
	}
}
