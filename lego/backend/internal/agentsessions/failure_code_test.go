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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// TestEverySurfaceServesTheFailureCode (w5/m132): REST, GraphQL and MCP serve
// a capacity failure's code beside its sentence. Another failure and a live
// session carry none: REST and MCP omit the key, and GraphQL answers null.
func TestEverySurfaceServesTheFailureCode(t *testing.T) {
	svc, _, _, lc := fixture()
	create := func(err error) string {
		t.Helper()
		lc.createErr = err
		view, createErr := svc.Create(caller("alice"), createInput())
		if createErr != nil {
			t.Fatal(createErr)
		}
		return view.ID
	}
	want := map[string]any{
		create(fmt.Errorf("create: %w", core.NewConflictError(sandbox.CodeSandboxCapacityLimit, "the workspace is at its sandbox limit", nil))): sandbox.CodeSandboxCapacityLimit,
		create(errors.New("pod schedule timeout")): nil,
		create(nil): nil,
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	ctx := caller("alice")
	client := newMCPClient(t, ctx, svc)
	// assertCode checks a JSON view's failureReasonCode against code; an absent
	// code must leave the key out.
	assertCode := func(surface, id string, raw []byte, code any) {
		t.Helper()
		var view map[string]any
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatalf("%s %s: %v", surface, id, err)
		}
		got, present := view["failureReasonCode"]
		if got != code || present != (code != nil) {
			t.Errorf("%s %s failureReasonCode = %v (present %v), want %v", surface, id, got, present, code)
		}
	}

	for id, code := range want {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id, nil).WithContext(ctx))
		if rec.Code != http.StatusOK {
			t.Fatalf("REST %s = %d %s", id, rec.Code, rec.Body.String())
		}
		assertCode("REST", id, rec.Body.Bytes(), code)

		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ agentSession(id: "` + id + `") { failureReasonCode } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL %s errors = %v", id, result.Errors)
		}
		if got := result.Data.(map[string]any)["agentSession"].(map[string]any)["failureReasonCode"]; got != code {
			t.Errorf("GraphQL %s failureReasonCode = %v, want %v", id, got, code)
		}

		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "get_agent_session", Arguments: map[string]any{"id": id}})
		if err != nil || res.IsError {
			t.Fatalf("MCP %s = %+v err=%v", id, res, err)
		}
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		assertCode("MCP", id, raw, code)
	}
}
