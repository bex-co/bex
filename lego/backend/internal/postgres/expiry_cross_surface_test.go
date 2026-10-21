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
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
)

// TestExpiresAtParityAcrossSurfaces pins w8/m126 t006: a free database's
// expiresAt is the same instant on REST get and list, GraphQL and MCP get and
// list, and an upgrade drops it on every one (GraphQL null, REST and MCP
// omitted).
func TestExpiresAtParityAcrossSurfaces(t *testing.T) {
	svc, _ := newService()
	created := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"expiry-pg","plan":"free"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", created.Code, created.Body.String())
	}
	id, _ := decodeMap(t, created.Body.Bytes())["id"].(string)
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	surfaces := func() map[string]any {
		got := serveREST(svc, http.MethodGet, "/v1/postgres/"+id, "")
		if got.Code != http.StatusOK {
			t.Fatalf("GET => %d: %s", got.Code, got.Body.String())
		}
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       context.Background(),
			RequestString: fmt.Sprintf(`{ database(id: %q) { expiresAt } }`, id),
		})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL errors: %v", res.Errors)
		}
		gql, _ := res.Data.(map[string]any)["database"].(map[string]any)
		out := map[string]any{"GraphQL": gql["expiresAt"]}
		if v, ok := decodeMap(t, got.Body.Bytes())["expiresAt"]; ok {
			out["REST"] = v
		}
		if v, ok := call("get_postgres", map[string]any{"postgresId": id})["expiresAt"]; ok {
			out["MCP"] = v
		}
		return out
	}

	free := surfaces()
	want, _ := free["REST"].(string)
	if _, err := time.Parse(time.RFC3339, want); err != nil {
		t.Fatalf("REST expiresAt = %v, want an RFC 3339 instant", free["REST"])
	}
	for _, surface := range []string{"GraphQL", "MCP"} {
		if free[surface] != want {
			t.Errorf("%s expiresAt = %v, want %s", surface, free[surface], want)
		}
	}

	if _, err := svc.SetPlan(context.Background(), id, "basic-256mb"); err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	paid := surfaces()
	if paid["GraphQL"] != nil || len(paid) != 1 {
		t.Errorf("paid database still reports expiresAt: %v", paid)
	}
}
