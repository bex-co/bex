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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The dashboard is a GraphQL client, so updateDatabaseConnectionPool and
// updateDatabaseDiskSize (w2/m307) are its only way to do what Render's
// dashboard does from a database's Info and Plan pages. Each case runs the
// same input through REST's PATCH and the GraphQL setter on twin databases
// and requires the same outcome: the stored spec, and the refusal's code and
// message.

type setterOutcome struct {
	code, message string
	pooler        bool
	storageGB     int32
}

func restPatchOutcome(t *testing.T, svc *Service, id, body string) (code, message string) {
	t.Helper()
	w := serveREST(svc, http.MethodPatch, "/v1/postgres/"+id, body)
	if w.Code == http.StatusOK {
		return "", ""
	}
	var out struct{ Code, Message string }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("PATCH %s => %d: %s", id, w.Code, w.Body)
	}
	return out.Code, out.Message
}

func gqlSetterOutcome(t *testing.T, svc *Service, request string, vars map[string]any) (code, message string) {
	t.Helper()
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: request, VariableValues: vars})
	if len(res.Errors) == 0 {
		return "", ""
	}
	if len(res.Errors) != 1 {
		t.Fatalf("GraphQL errors = %+v", res.Errors)
	}
	code, _ = res.Errors[0].Extensions["code"].(string)
	return code, res.Errors[0].Message
}

// mcpPatchOutcome runs update_postgres, the MCP tool that already carried both
// fields. MCP renders a refusal as "CODE: message", so it is compared on the
// code and the stored spec; a code-less refusal reports "refused".
func mcpPatchOutcome(t *testing.T, svc *Service, args map[string]any) string {
	t.Helper()
	session, cleanup := pgMCPSession(t, svc)
	defer cleanup()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_postgres", Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		return ""
	}
	content, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("MCP error content = %+v", res.Content)
	}
	if code, _, found := strings.Cut(content.Text, ": "); found && code == strings.ToUpper(code) && !strings.Contains(code, " ") {
		return code
	}
	return "refused"
}

func storedSetterOutcome(t *testing.T, cl client.Client, id, code, message string) setterOutcome {
	t.Helper()
	db := getDatabase(t, cl, id)
	return setterOutcome{code: code, message: message, pooler: db.Spec.Pooler, storageGB: db.Spec.StorageGB}
}

func TestConnectionPoolSetterMatchesREST(t *testing.T) {
	const setPool = `mutation($id:String!, $v:String!) { updateDatabaseConnectionPool(id:$id, connectionPool:$v) { id connectionPool } }`
	for _, tc := range []struct {
		name, plan, value string
		pooled            bool // before the write
		want              setterOutcome
	}{
		{name: "enable", plan: "basic-256mb", value: "pgbouncer", want: setterOutcome{pooler: true}},
		{name: "disable", plan: "basic-256mb", value: "none", pooled: true, want: setterOutcome{pooler: false}},
		{name: "free plan refused", plan: "free", value: "pgbouncer", want: setterOutcome{code: "POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED"}},
		{name: "unknown value refused", plan: "basic-256mb", value: "pgpool", want: setterOutcome{code: "POSTGRES_CONNECTION_POOL_UNKNOWN"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := pgGQLNewService()
			for _, id := range []string{"rest-pool", "gql-pool", "mcp-pool"} {
				seedDatabaseSpec(t, cl, id, appv1alpha1.DatabaseSpec{Plan: tc.plan, Pooler: tc.pooled}, false)
			}
			restCode, restMessage := restPatchOutcome(t, svc, "rest-pool", fmt.Sprintf(`{"connectionPool":%q}`, tc.value))
			gqlCode, gqlMessage := gqlSetterOutcome(t, svc, setPool, map[string]any{"id": "gql-pool", "v": tc.value})
			mcpCode := mcpPatchOutcome(t, svc, map[string]any{"postgresId": "mcp-pool", "connectionPool": tc.value})
			rest := storedSetterOutcome(t, cl, "rest-pool", restCode, restMessage)
			gql := storedSetterOutcome(t, cl, "gql-pool", gqlCode, gqlMessage)
			mcpOut := storedSetterOutcome(t, cl, "mcp-pool", mcpCode, rest.message)
			if rest != gql || rest != mcpOut {
				t.Fatalf("REST %+v, GraphQL %+v, MCP %+v: the surfaces disagree", rest, gql, mcpOut)
			}
			if gql.code != tc.want.code || (tc.want.code == "" && gql.pooler != tc.want.pooler) {
				t.Fatalf("outcome %+v, want code %q pooler %v", gql, tc.want.code, tc.want.pooler)
			}
			if tc.want.code != "" && gql.pooler != tc.pooled {
				t.Fatalf("a refused write changed the pooler to %v", gql.pooler)
			}
		})
	}
}

// An empty value is "leave unchanged" in REST's PATCH body, where the field is
// optional. The GraphQL setter's argument is the whole request, so an empty
// one is refused by name rather than read as a successful no-op.
func TestConnectionPoolSetterRefusesEmptyValue(t *testing.T) {
	svc, cl := pgGQLNewService()
	seedDatabaseSpec(t, cl, "gql-pool", appv1alpha1.DatabaseSpec{Plan: "basic-256mb"}, false)
	code, _ := gqlSetterOutcome(t, svc,
		`mutation { updateDatabaseConnectionPool(id:"gql-pool", connectionPool:"") { id } }`, nil)
	if code != "POSTGRES_CONNECTION_POOL_UNKNOWN" {
		t.Fatalf("empty connectionPool code = %q, want POSTGRES_CONNECTION_POOL_UNKNOWN", code)
	}
}

func TestDiskSizeSetterMatchesREST(t *testing.T) {
	const setDisk = `mutation($id:String!, $gb:Int!) { updateDatabaseDiskSize(id:$id, diskSizeGB:$gb) { id diskSizeGB } }`
	for _, tc := range []struct {
		name, plan string
		before, gb int32
		wantCode   string
	}{
		{name: "grow", plan: "basic-256mb", before: 15, gb: 20},
		{name: "shrink refused", plan: "basic-256mb", before: 15, gb: 10},
		{name: "free plan refused", plan: "free", gb: 5, wantCode: "POSTGRES_STORAGE_PLAN_UNSUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := pgGQLNewService()
			for _, id := range []string{"rest-disk", "gql-disk", "mcp-disk"} {
				seedDatabaseSpec(t, cl, id, appv1alpha1.DatabaseSpec{Plan: tc.plan, StorageGB: tc.before}, false)
			}
			restCode, restMessage := restPatchOutcome(t, svc, "rest-disk", fmt.Sprintf(`{"diskSizeGB":%d}`, tc.gb))
			gqlCode, gqlMessage := gqlSetterOutcome(t, svc, setDisk, map[string]any{"id": "gql-disk", "gb": int(tc.gb)})
			mcpCode := mcpPatchOutcome(t, svc, map[string]any{"postgresId": "mcp-disk", "diskSizeGB": tc.gb})
			rest := storedSetterOutcome(t, cl, "rest-disk", restCode, restMessage)
			gql := storedSetterOutcome(t, cl, "gql-disk", gqlCode, gqlMessage)
			if rest != gql {
				t.Fatalf("REST %+v, GraphQL %+v: the surfaces disagree", rest, gql)
			}
			// A code-less refusal (the grow-only rule) reads "refused" on MCP.
			wantMCP := rest.code
			if wantMCP == "" && rest.message != "" {
				wantMCP = "refused"
			}
			if mcpOut := storedSetterOutcome(t, cl, "mcp-disk", mcpCode, ""); mcpOut.code != wantMCP || mcpOut.storageGB != rest.storageGB {
				t.Fatalf("MCP %+v, want code %q storageGB %d", mcpOut, wantMCP, rest.storageGB)
			}
			refused := gql.message != ""
			if gql.code != tc.wantCode {
				t.Fatalf("code = %q, want %q (%+v)", gql.code, tc.wantCode, gql)
			}
			if wantRefused := tc.gb < tc.before || tc.wantCode != ""; refused != wantRefused {
				t.Fatalf("refused = %v, want %v (%+v)", refused, wantRefused, gql)
			}
			if want := map[bool]int32{true: tc.before, false: tc.gb}[refused]; gql.storageGB != want {
				t.Fatalf("stored storageGB = %d, want %d", gql.storageGB, want)
			}
		})
	}
}

// Both setters go through gqlutil.PatchMutation, so dryRun previews the
// change without writing it.
func TestGraphQLDryRunPoolAndDiskSetters(t *testing.T) {
	svc, cl := pgGQLNewService()
	seedDatabaseSpec(t, cl, "gql-dry", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 15}, false)
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation {
		pool: updateDatabaseConnectionPool(id:"gql-dry", connectionPool:"pgbouncer", dryRun:true) { connectionPool }
		disk: updateDatabaseDiskSize(id:"gql-dry", diskSizeGB:30, dryRun:true) { diskSizeGB }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("dry-run errors: %+v", res.Errors)
	}
	data := res.Data.(map[string]any)
	if got := data["pool"].(map[string]any)["connectionPool"]; got != ConnectionPoolPGBouncer {
		t.Fatalf("pool preview connectionPool = %v, want %s", got, ConnectionPoolPGBouncer)
	}
	if got := data["disk"].(map[string]any)["diskSizeGB"]; got != 30 {
		t.Fatalf("disk preview diskSizeGB = %v (%T), want 30", got, got)
	}
	if db := getDatabase(t, cl, "gql-dry"); db.Spec.Pooler || db.Spec.StorageGB != 15 {
		t.Fatalf("dry-run wrote the spec: pooler=%v storageGB=%d", db.Spec.Pooler, db.Spec.StorageGB)
	}
}
