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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/twofactor"
)

// twofactor_surfaces_test.go proves the three API surfaces reach the one
// enforcement point of w22/m402 (docs/ADR012-auth.md §9a) and answer the same
// coded 403 — the REST list of services, a GraphQL query and an MCP tool, each
// called as a member who has no second factor — and that everything the member
// must still reach, their other workspaces and the exempt operations, works.
//
// The gate here is the real twofactor.Gate over in-memory settings, enrollment
// and key creators, so the policy (API keys borrow their creator's factor,
// platform principals are exempt) is exercised end to end through each surface.

const twoFactorCode = "WORKSPACE_2FA_REQUIRED"

type surfaceSettings map[string]bool

func (s surfaceSettings) RequireTwoFactor(_ context.Context, workspaceID string) (bool, error) {
	return s[workspaceID], nil
}

type surfaceEnrollment map[string]bool // subject -> enrolled; absent = not a Kratos identity

func (e surfaceEnrollment) MFAEnrolled(_ context.Context, subject string) (bool, bool, error) {
	enrolled, human := e[subject]
	return enrolled, human, nil
}

type surfaceKeys struct {
	creators  map[string]string
	unstamped map[string]bool
}

func (k surfaceKeys) KeyCreator(_ context.Context, clientID string) (string, bool, error) {
	if c, ok := k.creators[clientID]; ok {
		return c, true, nil
	}
	return "", k.unstamped[clientID], nil
}

// twoFactorSurfaces is a server whose workspace "tea-1" requires 2FA and whose
// workspace "tea-2" does not. Every principal below is a member of both, tea-1
// being the default of each.
func twoFactorSurfaces(t *testing.T) (*Server, string, string) {
	t.Helper()
	st := newFakeWSStore()
	on := mustCreate(t, st, "required", "pro", "unenrolled")
	off := mustCreate(t, st, "optional", "pro", "unenrolled")
	for _, workspace := range []string{on.ID, off.ID} {
		for _, subject := range []string{"enrolled", "bex-platform", "key-by-unenrolled", "key-by-enrolled", "old-key"} {
			st.addMember(workspace, subject, "developer")
		}
	}
	st.tenants[0].RequireTwoFactor = true

	gate := twofactor.New(twofactor.Config{
		Settings:   surfaceSettings{on.ID: true},
		Enrollment: surfaceEnrollment{"unenrolled": false, "enrolled": true},
		Keys: surfaceKeys{
			creators:  map[string]string{"key-by-unenrolled": "unenrolled", "key-by-enrolled": "enrolled"},
			unstamped: map[string]bool{"old-key": true},
		},
	})
	cl := fakeClient(appWithOwnerLabel("on-web", on.ID), appWithOwnerLabel("off-web", off.ID))
	base := &core.Base{Client: cl, Namespace: "default", Workspace: &apiFakeResolver{store: st}, TwoFactor: gate}
	srv := NewServer(base, Deps{WorkspaceStore: st})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	srv.schema = schema // Handler() compiles it; the tests drive the GraphQL handler directly
	return srv, on.ID, off.ID
}

func sessionOf(subject string) core.Identity {
	return core.Identity{Subject: subject, Method: "session", Human: true}
}

func keyOf(clientID string) core.Identity {
	return core.Identity{Subject: clientID, ClientID: clientID, Method: "oauth2"}
}

// restRaw serves one request as id and returns the status and raw body.
func restRaw(t *testing.T, srv *Server, id core.Identity, method, path string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil).WithContext(core.WithIdentity(context.Background(), id))
	rec := httptest.NewRecorder()
	srv.restHandler().ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// restDo is restRaw with an object body decoded (a list answers an array, which
// leaves the map nil).
func restDo(t *testing.T, srv *Server, id core.Identity, method, path string) (int, map[string]any) {
	t.Helper()
	status, raw := restRaw(t, srv, id, method, path)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	return status, body
}

func requireTwoFactorREST(t *testing.T, srv *Server, id core.Identity, path, workspaceID string) {
	t.Helper()
	status, body := restDo(t, srv, id, http.MethodGet, path)
	params, _ := body["params"].(map[string]any)
	if status != http.StatusForbidden || body["code"] != twoFactorCode || params["workspaceId"] != workspaceID {
		t.Fatalf("GET %s as %s = %d %v, want 403 %s with workspaceId %s", path, id.Subject, status, body, twoFactorCode, workspaceID)
	}
}

func graphqlDo(t *testing.T, srv *Server, id core.Identity, query string) map[string]any {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"query": query})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(payload)).WithContext(core.WithIdentity(context.Background(), id))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.graphqlHandler().ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("graphql answer %q: %v", rec.Body.String(), err)
	}
	return out
}

func graphqlCode(out map[string]any) string {
	errs, _ := out["errors"].([]any)
	if len(errs) == 0 {
		return ""
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	code, _ := ext["code"].(string)
	return code
}

func mcpCall(t *testing.T, srv *Server, id core.Identity, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := mcpSessionIdentity(t, srv, id).CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s as %s: %v", tool, id.Subject, err)
	}
	return res
}

func mcpText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestAnUnenrolledMemberGetsTheSameCodedRefusalOnEverySurface(t *testing.T) {
	srv, on, _ := twoFactorSurfaces(t)
	member := sessionOf("unenrolled")

	t.Run("REST", func(t *testing.T) {
		requireTwoFactorREST(t, srv, member, "/v1/services?ownerId="+on, on)
		// Naming no workspace lists every workspace the member belongs to and
		// skips the ones that refuse them, as it skips any workspace they cannot
		// read: the blocked workspace's services are not in the answer.
		status, raw := restRaw(t, srv, member, http.MethodGet, "/v1/services")
		if status != http.StatusOK || strings.Contains(string(raw), "on-web") || !strings.Contains(string(raw), "off-web") {
			t.Fatalf("unfiltered list = %d %s, want 200 with only the workspace that does not require 2FA", status, raw)
		}
	})
	t.Run("GraphQL", func(t *testing.T) {
		out := graphqlDo(t, srv, member, `{ services { id name } }`)
		if got := graphqlCode(out); got != twoFactorCode {
			t.Fatalf("GraphQL = %v, want an error coded %s", out, twoFactorCode)
		}
		ext := out["errors"].([]any)[0].(map[string]any)["extensions"].(map[string]any)
		if ext["workspaceId"] != on {
			t.Fatalf("extensions = %v, want workspaceId %s", ext, on)
		}
	})
	t.Run("MCP", func(t *testing.T) {
		res := mcpCall(t, srv, member, "list_services", nil)
		if !res.IsError || !strings.HasPrefix(mcpText(res), twoFactorCode+":") {
			t.Fatalf("MCP = error=%v %q, want the tool error prefixed %s:", res.IsError, mcpText(res), twoFactorCode)
		}
	})
}

func TestAnEnrolledMemberIsUnaffectedOnEverySurface(t *testing.T) {
	srv, _, _ := twoFactorSurfaces(t)
	member := sessionOf("enrolled")

	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/services"); status != http.StatusOK {
		t.Fatalf("REST = %d, want 200", status)
	}
	if out := graphqlDo(t, srv, member, `{ services { id } }`); graphqlCode(out) != "" {
		t.Fatalf("GraphQL = %v, want no error", out)
	}
	if res := mcpCall(t, srv, member, "list_services", nil); res.IsError {
		t.Fatalf("MCP = %q, want success", mcpText(res))
	}
}

func TestTheBlockedMembersOtherWorkspaceIsUnaffectedOnEverySurface(t *testing.T) {
	srv, _, off := twoFactorSurfaces(t)
	member := sessionOf("unenrolled")

	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/services?ownerId="+off); status != http.StatusOK {
		t.Fatalf("REST in the workspace that does not require 2FA = %d, want 200", status)
	}
	if out := graphqlDo(t, srv, member, `{ services(ownerId: "`+off+`") { id } }`); graphqlCode(out) != "" {
		t.Fatalf("GraphQL = %v, want no error", out)
	}
	if res := mcpCall(t, srv, member, "list_services", map[string]any{"workspaceId": off}); res.IsError {
		t.Fatalf("MCP = %q, want success", mcpText(res))
	}
}

func TestAPlatformPrincipalIsExemptOnEverySurface(t *testing.T) {
	srv, _, _ := twoFactorSurfaces(t)
	// bex-platform is a member but not a Kratos identity: nothing to enroll.
	member := core.Identity{Subject: "bex-platform", Method: "session"}
	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/services"); status != http.StatusOK {
		t.Fatalf("REST = %d, want 200", status)
	}
	if res := mcpCall(t, srv, member, "list_services", nil); res.IsError {
		t.Fatalf("MCP = %q, want success", mcpText(res))
	}
}

func TestAnAPIKeyBorrowsItsCreatorsSecondFactorOnEverySurface(t *testing.T) {
	srv, on, off := twoFactorSurfaces(t)

	// A key created by the unenrolled member is refused with the same code.
	blocked := keyOf("key-by-unenrolled")
	requireTwoFactorREST(t, srv, blocked, "/v1/services?ownerId="+on, on)
	if got := graphqlCode(graphqlDo(t, srv, blocked, `{ services { id } }`)); got != twoFactorCode {
		t.Fatalf("GraphQL = %q, want %s", got, twoFactorCode)
	}
	if res := mcpCall(t, srv, blocked, "list_services", nil); !res.IsError || !strings.HasPrefix(mcpText(res), twoFactorCode+":") {
		t.Fatalf("MCP = error=%v %q", res.IsError, mcpText(res))
	}

	// One created by an enrolled member keeps working.
	fine := keyOf("key-by-enrolled")
	if status, _ := restDo(t, srv, fine, http.MethodGet, "/v1/services?ownerId="+on); status != http.StatusOK {
		t.Fatalf("a key created by an enrolled member = %d, want 200", status)
	}

	// A key with no recorded creator is refused where 2FA is required, and works
	// in the member's other workspace.
	orphan := keyOf("old-key")
	requireTwoFactorREST(t, srv, orphan, "/v1/services?ownerId="+on, on)
	if status, _ := restDo(t, srv, orphan, http.MethodGet, "/v1/services?ownerId="+off); status != http.StatusOK {
		t.Fatalf("an unattributed key in a workspace that does not require 2FA = %d, want 200", status)
	}
}

func TestTheOperationsABlockedMemberNeedsStillWork(t *testing.T) {
	srv, on, _ := twoFactorSurfaces(t)
	member := sessionOf("unenrolled")

	// The workspace list, which carries requireTwoFactor, and who they are.
	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/owners"); status != http.StatusOK {
		t.Fatalf("GET /v1/owners = %d, want 200", status)
	}
	if status, _ := restDo(t, srv, member, http.MethodGet, "/v1/users"); status != http.StatusOK {
		t.Fatalf("GET /v1/users = %d, want 200", status)
	}
	out := graphqlDo(t, srv, member, `{ workspaces { id requireTwoFactor } }`)
	if graphqlCode(out) != "" {
		t.Fatalf("GraphQL workspaces = %v, want no error", out)
	}
	rows, _ := out["data"].(map[string]any)["workspaces"].([]any)
	var seenOn bool
	for _, row := range rows {
		w := row.(map[string]any)
		if w["id"] == on {
			seenOn = true
			if w["requireTwoFactor"] != true {
				t.Fatalf("workspace %s = %v, want requireTwoFactor so the member can see why", on, w)
			}
		}
	}
	if !seenOn {
		t.Fatalf("workspaces = %v, want the required workspace listed", rows)
	}
	if res := mcpCall(t, srv, member, "list_workspaces", nil); res.IsError {
		t.Fatalf("MCP list_workspaces = %q, want success", mcpText(res))
	}

	// Leaving is reached (the members store is not wired here, so it stops at
	// the store with a 503 — anything but the 2FA refusal).
	status, body := restDo(t, srv, member, http.MethodDelete, "/v1/workspaces/"+on+"/members/me")
	if status == http.StatusForbidden && body["code"] == twoFactorCode {
		t.Fatalf("leaving a workspace that requires 2FA = %d %v, want it exempt", status, body)
	}
}
