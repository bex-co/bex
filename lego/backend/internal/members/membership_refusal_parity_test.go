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

package members

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// membership_refusal_parity_test.go is w5/m101 t004: the four membership
// refusals must reach REST, GraphQL and MCP with the SAME machine-readable
// code, the same 409 class, and a message that names the recovery path. A code
// added on one adapter and missed on the others fails here.
//
// The shared plumbing does the carrying (core.CodedError → WriteErr's `code`
// field, gqlerrors extensions, core.MCPError's prefix), which is precisely what
// this test pins: it would catch a refusal returned as a bare fmt.Errorf on any
// one verb.
func TestMembershipRefusalsCarryTheSameCodeOnEverySurface(t *testing.T) {
	cases := []struct {
		name         string
		verb         string // "remove" | "changeRole"
		target       string
		role         string
		wantCode     string
		wantRecovery string // substring the message must contain
	}{
		{
			name: "owner removal", verb: "remove", target: "owner-1",
			wantCode: ErrorOwnerCannotBeRemoved, wantRecovery: "ownership",
		},
		{
			name: "owner demotion", verb: "changeRole", target: "owner-1", role: "DEVELOPER",
			wantCode: ErrorOwnerRoleCannotChange, wantRecovery: "must remain an admin",
		},
		{
			name: "self removal", verb: "remove", target: "admin-2",
			wantCode: ErrorCannotRemoveSelf, wantRecovery: "leave the workspace",
		},
		{
			name: "self role change", verb: "changeRole", target: "admin-2", role: "VIEWER",
			wantCode: ErrorCannotChangeOwnRole, wantRecovery: "another admin",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The caller is admin-2; owner-1 owns the workspace. Both are admins,
			// so the last-admin rule can never be what refuses.
			newSvc := func() *Service {
				st := newFakeStore("pro")
				st.ownerSubject = "owner-1"
				st.seedMember("owner-1", "admin")
				st.seedMember("admin-2", "admin")
				return svc(st, newFakeGranter(), nil, nil)
			}
			ctx := ctxWith("admin-2")

			restCode, restStatus, restMsg := refusalOverREST(t, newSvc(), ctx, tc.verb, tc.target, tc.role)
			if restStatus != http.StatusConflict {
				t.Errorf("REST status = %d, want 409", restStatus)
			}
			gqlCode, gqlMsg := refusalOverGraphQL(t, newSvc(), ctx, tc.verb, tc.target, tc.role)
			mcpMsg := refusalOverMCP(t, newSvc(), ctx, tc.verb, tc.target, tc.role)

			for surface, got := range map[string]string{"REST": restCode, "GraphQL": gqlCode} {
				if got != tc.wantCode {
					t.Errorf("%s code = %q, want %q", surface, got, tc.wantCode)
				}
			}
			// MCP has no structured error envelope: core.MCPError prefixes the
			// code onto the message so an agent can tell "not allowed" from
			// "transient".
			if !strings.HasPrefix(mcpMsg, tc.wantCode+": ") {
				t.Errorf("MCP error = %q, want %q prefix", mcpMsg, tc.wantCode)
			}
			for surface, msg := range map[string]string{"REST": restMsg, "GraphQL": gqlMsg, "MCP": mcpMsg} {
				if !strings.Contains(msg, tc.wantRecovery) {
					t.Errorf("%s message %q does not name the recovery path (%q)", surface, msg, tc.wantRecovery)
				}
			}
		})
	}
}

func refusalOverREST(t *testing.T, s *Service, ctx context.Context, verb, target, role string) (code string, status int, msg string) {
	t.Helper()
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	var req *http.Request
	if verb == "remove" {
		req = httptest.NewRequest(http.MethodDelete, "/v1/workspaces/tea-1/members/"+target, nil)
	} else {
		req = httptest.NewRequest(http.MethodPatch, "/v1/workspaces/tea-1/members/"+target,
			strings.NewReader(`{"role":"`+role+`"}`))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req.WithContext(ctx))
	var body struct{ Code, Message string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("REST decode %s: %v", rec.Body, err)
	}
	return body.Code, rec.Code, body.Message
}

func refusalOverGraphQL(t *testing.T, s *Service, ctx context.Context, verb, target, role string) (code, msg string) {
	t.Helper()
	mutation := graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: s.GraphQLMutation()})
	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	req := `mutation($w: String!, $s: String!) { removeWorkspaceMember(workspaceId: $w, subject: $s) }`
	vars := map[string]any{"w": "tea-1", "s": target}
	if verb != "remove" {
		req = `mutation($w: String!, $s: String!, $r: String!) {
			changeWorkspaceMemberRole(workspaceId: $w, subject: $s, role: $r) { subject role } }`
		vars["r"] = role
	}
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: req, VariableValues: vars, Context: ctx})
	if len(result.Errors) != 1 {
		t.Fatalf("graphql errors = %v, want exactly one refusal", result.Errors)
	}
	gerr := result.Errors[0]
	c, _ := gerr.Extensions["code"].(string)
	return c, gerr.Message
}

func refusalOverMCP(t *testing.T, s *Service, ctx context.Context, verb, target, role string) string {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	s.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	mcpCtx := core.WithWorkspace(ctx, "tea-1")
	if _, err := srv.Connect(mcpCtx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(mcpCtx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	params := &mcp.CallToolParams{Name: "remove_workspace_member", Arguments: map[string]any{"subject": target}}
	if verb != "remove" {
		params = &mcp.CallToolParams{
			Name:      "change_workspace_member_role",
			Arguments: map[string]any{"subject": target, "role": role},
		}
	}
	res, err := cs.CallTool(context.Background(), params)
	if err != nil {
		return err.Error()
	}
	if !res.IsError {
		t.Fatalf("MCP tool succeeded on a refusal: %+v", res)
	}
	var out strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			out.WriteString(text.Text)
		}
	}
	return out.String()
}
