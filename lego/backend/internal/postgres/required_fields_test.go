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

// required_fields_test.go is w6/m109's regression test: Render's pinned OpenAPI
// schema (render-public-api-1.json) declares ipAllowList and readReplicas
// REQUIRED on postgres — so every REST and MCP response must carry the key as
// [] when empty, not omit it (a strict generated client like the pinned
// render-oss/cli treats a missing required property as a decode failure, and
// "no allowlist, no replicas" is the overwhelmingly common state). GraphQL
// already serialized [] and is asserted here as the parity anchor.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// requireEmptyArray asserts the JSON key is present AND an empty array — the
// exact wire shape Render's required annotation demands for an empty list.
func requireEmptyArray(t *testing.T, step, key string, body map[string]any) {
	t.Helper()
	v, ok := body[key]
	if !ok {
		t.Errorf("%s: %q key absent — Render marks it required; want [] (w6/m109)", step, key)
		return
	}
	arr, ok := v.([]any)
	if !ok {
		t.Errorf("%s: %q = %T, want a JSON array", step, key, v)
		return
	}
	if len(arr) != 0 {
		t.Errorf("%s: %q = %v, want empty []", step, key, arr)
	}
}

// TestPostgresRequiredArraysPresentWhenEmpty drives create/get/list/patch plus
// MCP get_postgres and the GraphQL anchor for a database with no allowlist and
// no read replicas: both keys must serialize as [], never be omitted.
func TestPostgresRequiredArraysPresentWhenEmpty(t *testing.T) {
	svc, _ := newService()

	w := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"req-empty-pg","plan":"free"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	created := decodeMap(t, w.Body.Bytes())
	requireEmptyArray(t, "create", "ipAllowList", created)
	requireEmptyArray(t, "create", "readReplicas", created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create response has no id")
	}

	g := serveREST(svc, http.MethodGet, "/v1/postgres/"+id, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get => %d: %s", g.Code, g.Body.String())
	}
	got := decodeMap(t, g.Body.Bytes())
	requireEmptyArray(t, "get", "ipAllowList", got)
	requireEmptyArray(t, "get", "readReplicas", got)

	var list []map[string]any
	l := serveREST(svc, http.MethodGet, "/v1/postgres", "")
	if l.Code != http.StatusOK {
		t.Fatalf("list => %d: %s", l.Code, l.Body.String())
	}
	if err := json.Unmarshal(l.Body.Bytes(), &list); err != nil {
		t.Fatalf("list body: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("list returned no items")
	}
	item, _ := list[0]["postgres"].(map[string]any)
	if item == nil {
		t.Fatalf("list item has no postgres envelope: %#v", list[0])
	}
	requireEmptyArray(t, "list", "ipAllowList", item)
	requireEmptyArray(t, "list", "readReplicas", item)

	p := serveREST(svc, http.MethodPatch, "/v1/postgres/"+id, `{"name":"req-empty-pg-2"}`)
	if p.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", p.Code, p.Body.String())
	}
	patched := decodeMap(t, p.Body.Bytes())
	requireEmptyArray(t, "patch", "ipAllowList", patched)
	requireEmptyArray(t, "patch", "readReplicas", patched)

	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	mcpGot := call("get_postgres", map[string]any{"postgresId": id})
	requireEmptyArray(t, "mcp get_postgres", "ipAllowList", mcpGot)
	requireEmptyArray(t, "mcp get_postgres", "readReplicas", mcpGot)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: fmt.Sprintf(`{ database(id: %q) { ipAllowListEntries { cidrBlock } readReplicas { name } } }`, id),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", res.Errors)
	}
	gql, ok := res.Data.(map[string]any)["database"].(map[string]any)
	if !ok {
		t.Fatalf("GraphQL database nil: %#v", res.Data)
	}
	// Parity anchor: GraphQL serialized [] before the fix and must still [].
	if gql["ipAllowListEntries"] == nil || len(gql["ipAllowListEntries"].([]any)) != 0 {
		t.Errorf("GraphQL ipAllowListEntries = %#v, want []", gql["ipAllowListEntries"])
	}
	if gql["readReplicas"] == nil || len(gql["readReplicas"].([]any)) != 0 {
		t.Errorf("GraphQL readReplicas = %#v, want []", gql["readReplicas"])
	}
}

// TestPostgresAllowListRoundTripNonEmpty is the regression control: a saved
// CIDR entry still round-trips with its description through PATCH and GET.
func TestPostgresAllowListRoundTripNonEmpty(t *testing.T) {
	svc, _ := newService()

	w := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"req-full-pg","plan":"free"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	id, _ := decodeMap(t, w.Body.Bytes())["id"].(string)

	p := serveREST(svc, http.MethodPatch, "/v1/postgres/"+id,
		`{"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}`)
	if p.Code != http.StatusOK {
		t.Fatalf("patch allowlist => %d: %s", p.Code, p.Body.String())
	}
	body := decodeMap(t, p.Body.Bytes())
	entries, ok := body["ipAllowList"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("patch ipAllowList = %#v, want one entry", body["ipAllowList"])
	}
	entry, _ := entries[0].(map[string]any)
	if entry["cidrBlock"] != "203.0.113.0/24" || entry["description"] != "office" {
		t.Errorf("entry = %#v, want {203.0.113.0/24, office}", entry)
	}

	g := serveREST(svc, http.MethodGet, "/v1/postgres/"+id, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get => %d: %s", g.Code, g.Body.String())
	}
	if entries, _ := decodeMap(t, g.Body.Bytes())["ipAllowList"].([]any); len(entries) != 1 {
		t.Errorf("get ipAllowList = %#v, want one entry", entries)
	}
}

// TestPostgresReadReplicasNonEmptySerializes verifies a database whose operator
// status reports read replicas still serializes the full array on REST.
func TestPostgresReadReplicasNonEmptySerializes(t *testing.T) {
	svc, _ := newService(
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "rr-req-pg", Namespace: "default"},
			Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb"},
			Status: appv1alpha1.DatabaseStatus{
				Phase: appv1alpha1.DBPhaseReady,
				ReadReplicaStatuses: []appv1alpha1.DatabaseReadReplicaStatus{
					{Name: "reader-1", InternalHost: "rr-req-pg-ro.default.svc"},
				},
			},
		},
	)
	w := serveREST(svc, http.MethodGet, "/v1/postgres/rr-req-pg", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get => %d: %s", w.Code, w.Body.String())
	}
	body := decodeMap(t, w.Body.Bytes())
	replicas, ok := body["readReplicas"].([]any)
	if !ok || len(replicas) != 1 {
		t.Fatalf("readReplicas = %#v, want one replica", body["readReplicas"])
	}
	replica, _ := replicas[0].(map[string]any)
	if replica["name"] != "reader-1" {
		t.Errorf("replica = %#v, want name reader-1", replica)
	}
}

// TestPostgresConnectionInfoExternalStringAlwaysPresent is w6/m109/t008: Render
// marks externalConnectionString REQUIRED on postgresConnectionInfo, so a
// non-public database must serialize it as "" rather than drop the key. The
// pool strings are deliberately NOT swept with it — Render lists
// internalConnectionPoolString/externalConnectionPoolString as properties but
// not as required, so omitempty stays correct there and is asserted as such.
func TestPostgresConnectionInfoExternalStringAlwaysPresent(t *testing.T) {
	for _, tc := range []struct {
		name, external string
	}{
		{"non-public serializes empty string", ""},
		{"public keeps its real value", "postgresql://u:p@pg.example.com:5432/d?sslmode=verify-full"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(PostgresConnectionInfo{
				Password:                 "p",
				InternalConnectionString: "postgresql://u:p@pg-rw.default:5432/d",
				ExternalConnectionString: tc.external,
				PSQLCommand:              "PGPASSWORD=p psql -h pg-rw.default.svc -U u d",
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(b, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			v, ok := got["externalConnectionString"]
			if !ok {
				t.Fatalf("externalConnectionString key absent — Render marks it required (w6/m109/t008); got %s", b)
			}
			if v != tc.external {
				t.Errorf("externalConnectionString = %q, want %q", v, tc.external)
			}
			// The pool strings are not Render-required: absent when empty is correct.
			if _, ok := got["internalConnectionPoolString"]; ok {
				t.Errorf("internalConnectionPoolString present though empty — Render does not require it; got %s", b)
			}
		})
	}
}
