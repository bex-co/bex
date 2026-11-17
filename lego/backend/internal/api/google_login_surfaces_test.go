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
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// google_login_surfaces_test.go proves the Google-login requirement of
// w7/m465 (docs/ADR012-auth.md §9b) reaches REST, GraphQL and MCP through the
// same enforcement point as 2FA and answers one coded 403 there, with the same
// exempt reads.

const googleLoginCode = "WORKSPACE_GOOGLE_LOGIN_REQUIRED"

type surfaceGoogle map[string]bool // subject -> the matching Google account; absent = not a Kratos identity

func (g surfaceGoogle) GoogleAccountMatches(_ context.Context, subject string) (bool, bool, error) {
	matches, human := g[subject]
	return matches, human, nil
}

// googleLoginSurfaces is a server whose workspace "required" requires Google
// login and whose "optional" requires nothing; "member" has the matching
// Google account and is a member of both.
func googleLoginSurfaces(t *testing.T) (*Server, string, string) {
	t.Helper()
	st := newFakeWSStore()
	on := mustCreate(t, st, "required", "pro", "member")
	off := mustCreate(t, st, "optional", "pro", "member")
	for _, workspace := range []string{on.ID, off.ID} {
		for _, subject := range []string{"key-google", "key-password", "key-predates"} {
			st.addMember(workspace, subject, "developer")
		}
	}
	st.tenants[0].RequireGoogleLogin = true
	gate := twofactor.New(twofactor.Config{
		Settings: surfaceSettings{on.ID: {GoogleLogin: true}},
		Google:   surfaceGoogle{"member": true},
		Keys: surfaceKeys{
			creators: map[string]string{"key-google": "member", "key-password": "member", "key-predates": "member"},
			signIns:  map[string]core.SignIn{"key-google": core.SignInGoogle, "key-password": core.SignInPassword},
		},
	})
	cl := fakeClient(appWithOwnerLabel("on-web", on.ID), appWithOwnerLabel("off-web", off.ID))
	base := &core.Base{Client: cl, Namespace: "default", Workspace: &apiFakeResolver{store: st}, TwoFactor: gate}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	srv.schema = schema
	return srv, on.ID, off.ID
}

func signedInAs(subject string, how core.SignIn) core.Identity {
	return core.Identity{Subject: subject, Method: core.MethodSession, Human: true, SignIn: how}
}

func requireCodeOnEverySurface(t *testing.T, srv *Server, id core.Identity, workspaceID, code string) {
	t.Helper()
	status, body := restDo(t, srv, id, http.MethodGet, "/v1/services?ownerId="+workspaceID)
	params, _ := body["params"].(map[string]any)
	if status != http.StatusForbidden || body["code"] != code || params["workspaceId"] != workspaceID {
		t.Fatalf("REST as %+v = %d %v, want 403 %s", id, status, body, code)
	}
	out := graphqlDo(t, srv, id, `{ services(ownerId: "`+workspaceID+`") { id } }`)
	if got := graphqlCode(out); got != code {
		t.Fatalf("GraphQL as %+v = %v, want %s", id, out, code)
	}
	if ext := out["errors"].([]any)[0].(map[string]any)["extensions"].(map[string]any); ext["workspaceId"] != workspaceID {
		t.Fatalf("GraphQL extensions = %v, want workspaceId %s", ext, workspaceID)
	}
	if res := mcpCall(t, srv, id, "list_services", map[string]any{"workspaceId": workspaceID}); !res.IsError || !strings.HasPrefix(mcpText(res), code+":") {
		t.Fatalf("MCP as %+v = error=%v %q, want %s", id, res.IsError, mcpText(res), code)
	}
}

func requireAllowedOnEverySurface(t *testing.T, srv *Server, id core.Identity, workspaceID string) {
	t.Helper()
	if status, raw := restRaw(t, srv, id, http.MethodGet, "/v1/services?ownerId="+workspaceID); status != http.StatusOK {
		t.Fatalf("REST as %+v = %d %s, want 200", id, status, raw)
	}
	if out := graphqlDo(t, srv, id, `{ services(ownerId: "`+workspaceID+`") { id } }`); graphqlCode(out) != "" {
		t.Fatalf("GraphQL as %+v = %v, want no error", id, out)
	}
	if res := mcpCall(t, srv, id, "list_services", map[string]any{"workspaceId": workspaceID}); res.IsError {
		t.Fatalf("MCP as %+v = %q, want success", id, mcpText(res))
	}
}

func TestAMemberNotSignedInWithGoogleGetsTheSameCodedRefusalOnEverySurface(t *testing.T) {
	srv, on, off := googleLoginSurfaces(t)
	password := signedInAs("member", core.SignInPassword)
	requireCodeOnEverySurface(t, srv, password, on, googleLoginCode)
	requireCodeOnEverySurface(t, srv, core.Identity{Subject: "member", Method: core.MethodOAuth2, ClientID: "bex-cli", Human: true, SignIn: core.SignInPassword, CanonicalScopes: "bex.read bex.write"}, on, googleLoginCode)
	requireCodeOnEverySurface(t, srv, keyOf("key-password"), on, googleLoginCode)
	// The same member's other workspace requires nothing.
	requireAllowedOnEverySurface(t, srv, password, off)
}

func TestAMemberSignedInWithGoogleAndTheirGoogleKeysPass(t *testing.T) {
	srv, on, _ := googleLoginSurfaces(t)
	requireAllowedOnEverySurface(t, srv, signedInAs("member", core.SignInGoogle), on)
	requireAllowedOnEverySurface(t, srv, keyOf("key-google"), on)
	// A key created before the stamp existed is exempt, as Render exempts keys
	// older than the setting.
	requireAllowedOnEverySurface(t, srv, keyOf("key-predates"), on)
}

func TestTheReadsABlockedMemberNeedsStillWorkUnderGoogleLogin(t *testing.T) {
	srv, on, _ := googleLoginSurfaces(t)
	member := signedInAs("member", core.SignInPassword)
	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/owners"); status != http.StatusOK {
		t.Fatalf("GET /v1/owners = %d, want 200", status)
	}
	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/users"); status != http.StatusOK {
		t.Fatalf("GET /v1/users = %d, want 200", status)
	}
	out := graphqlDo(t, srv, member, `{ workspaces { id requireGoogleLogin } }`)
	if graphqlCode(out) != "" {
		t.Fatalf("GraphQL workspaces = %v, want no error", out)
	}
	var seen bool
	for _, row := range out["data"].(map[string]any)["workspaces"].([]any) {
		if w := row.(map[string]any); w["id"] == on {
			seen = w["requireGoogleLogin"] == true
		}
	}
	if !seen {
		t.Fatalf("workspaces = %v, want the required workspace listed with requireGoogleLogin", out)
	}
	if res := mcpCall(t, srv, member, "list_workspaces", nil); res.IsError {
		t.Fatalf("MCP list_workspaces = %q, want success", mcpText(res))
	}
	status, body := restDo(t, srv, member, http.MethodDelete, "/v1/workspaces/"+on+"/members/me")
	if status == http.StatusForbidden && body["code"] == googleLoginCode {
		t.Fatalf("leaving = %d %v, want it exempt", status, body)
	}
}
