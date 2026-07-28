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
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestThreeSurfaceParity_UserIDAndEmail asserts REST, GraphQL, and MCP all
// return the same userId/email for the same member — the w6/m10 DoD ("one
// service verb, three thin fragments" must not drift). Each surface is
// exercised through its real registration fragment (RegisterREST /
// GraphQLQuery / RegisterMCP), not by re-implementing enrichment.
func TestThreeSurfaceParity_UserIDAndEmail(t *testing.T) {
	st := newFakeStore("pro")
	st.seedMember("admin-1", "admin")
	s := svc(st, newFakeGranter(), nil, roleChecker{relation: "viewer"})
	s.Identities = fakeIdentities{"admin-1": {Email: "admin@example.com"}}
	ctx := ctxWith("viewer-1")

	// REST
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/tea-1/members", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST status = %d, body %s", rec.Code, rec.Body)
	}
	var restMembers []MemberView
	if err := json.Unmarshal(rec.Body.Bytes(), &restMembers); err != nil {
		t.Fatalf("REST decode: %v", err)
	}
	if len(restMembers) != 1 {
		t.Fatalf("REST members = %+v", restMembers)
	}

	// GraphQL
	query := graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	result := graphql.Do(graphql.Params{
		Schema:         schema,
		RequestString:  `query($w: String!) { workspaceMembers(workspaceId: $w) { subject userId email role } }`,
		VariableValues: map[string]any{"w": "tea-1"},
		Context:        ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}
	b, _ := json.Marshal(result.Data)
	var gqlOut struct {
		WorkspaceMembers []struct {
			Subject, UserID, Email, Role string
		} `json:"workspaceMembers"`
	}
	if err := json.Unmarshal(b, &gqlOut); err != nil {
		t.Fatalf("graphql decode: %v (%s)", err, b)
	}
	if len(gqlOut.WorkspaceMembers) != 1 {
		t.Fatalf("graphql members = %+v", gqlOut.WorkspaceMembers)
	}

	// MCP
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
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_workspace_members"})
	if err != nil || res.IsError {
		t.Fatalf("call tool: err=%v res=%+v", err, res)
	}
	mb, _ := json.Marshal(res.StructuredContent)
	var mcpOut struct {
		Members []MemberView `json:"members"`
	}
	if err := json.Unmarshal(mb, &mcpOut); err != nil {
		t.Fatalf("mcp decode: %v (%s)", err, mb)
	}
	if len(mcpOut.Members) != 1 {
		t.Fatalf("mcp members = %+v", mcpOut.Members)
	}

	// Compare all three.
	rv, gv, mv := restMembers[0], gqlOut.WorkspaceMembers[0], mcpOut.Members[0]
	if rv.UserID == "" || rv.Email == "" {
		t.Fatalf("REST member not enriched: %+v", rv)
	}
	if rv.UserID != gv.UserID || rv.UserID != mv.UserID {
		t.Errorf("userId drift: rest=%q graphql=%q mcp=%q", rv.UserID, gv.UserID, mv.UserID)
	}
	if rv.Email != gv.Email || rv.Email != mv.Email {
		t.Errorf("email drift: rest=%q graphql=%q mcp=%q", rv.Email, gv.Email, mv.Email)
	}
}
