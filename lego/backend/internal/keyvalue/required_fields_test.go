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

package keyvalue

// required_fields_test.go is w6/m109's regression test: Render's pinned OpenAPI
// schema (render-public-api-1.json) declares ipAllowList REQUIRED on keyValue
// (and redis, which this feature serves) — so every REST and MCP response must
// carry the key as [] when empty, not omit it (a strict generated client like
// the pinned render-oss/cli treats a missing required property as a decode
// failure, and "no allowlist" is the overwhelmingly common state). GraphQL
// already serialized [] and is asserted here as the parity anchor.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"
)

// requireEmptyAllowList asserts the ipAllowList JSON key is present AND an
// empty array — the exact wire shape Render's required annotation demands for
// an empty list.
func requireEmptyAllowList(t *testing.T, step string, body map[string]any) {
	t.Helper()
	v, ok := body["ipAllowList"]
	if !ok {
		t.Errorf("%s: ipAllowList key absent — Render marks it required; want [] (w6/m109)", step)
		return
	}
	arr, ok := v.([]any)
	if !ok {
		t.Errorf("%s: ipAllowList = %T, want a JSON array", step, v)
		return
	}
	if len(arr) != 0 {
		t.Errorf("%s: ipAllowList = %v, want empty []", step, arr)
	}
}

// TestKeyValueRequiredAllowListPresentWhenEmpty drives create/get/list/patch
// plus MCP get_key_value and the GraphQL anchor for an instance with no
// allowlist: the key must serialize as [], never be omitted.
func TestKeyValueRequiredAllowListPresentWhenEmpty(t *testing.T) {
	svc, _ := newService()

	w := serveREST(svc, http.MethodPost, "/v1/key-value", `{"name":"req-empty-kv","plan":"free"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	created := decodeMap(t, w.Body.Bytes())
	requireEmptyAllowList(t, "create", created)
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create response has no id")
	}

	g := serveREST(svc, http.MethodGet, "/v1/key-value/"+id, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get => %d: %s", g.Code, g.Body.String())
	}
	requireEmptyAllowList(t, "get", decodeMap(t, g.Body.Bytes()))

	var list []map[string]any
	l := serveREST(svc, http.MethodGet, "/v1/key-value", "")
	if l.Code != http.StatusOK {
		t.Fatalf("list => %d: %s", l.Code, l.Body.String())
	}
	if err := json.Unmarshal(l.Body.Bytes(), &list); err != nil {
		t.Fatalf("list body: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("list returned no items")
	}
	item, _ := list[0]["keyValue"].(map[string]any)
	if item == nil {
		t.Fatalf("list item has no keyValue envelope: %#v", list[0])
	}
	requireEmptyAllowList(t, "list", item)

	p := serveREST(svc, http.MethodPatch, "/v1/key-value/"+id, `{"name":"req-empty-kv-2"}`)
	if p.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", p.Code, p.Body.String())
	}
	requireEmptyAllowList(t, "patch", decodeMap(t, p.Body.Bytes()))

	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()
	mcpGot := call("get_key_value", map[string]any{"keyValueId": id})
	requireEmptyAllowList(t, "mcp get_key_value", mcpGot)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: fmt.Sprintf(`{ keyValue(id: %q) { ipAllowListEntries { cidrBlock } } }`, id),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", res.Errors)
	}
	gql, ok := res.Data.(map[string]any)["keyValue"].(map[string]any)
	if !ok {
		t.Fatalf("GraphQL keyValue nil: %#v", res.Data)
	}
	// Parity anchor: GraphQL serialized [] before the fix and must still [].
	if gql["ipAllowListEntries"] == nil || len(gql["ipAllowListEntries"].([]any)) != 0 {
		t.Errorf("GraphQL ipAllowListEntries = %#v, want []", gql["ipAllowListEntries"])
	}
}

// TestKeyValueAllowListRoundTripNonEmpty is the regression control: a saved
// CIDR entry still round-trips with its description through PATCH and GET.
func TestKeyValueAllowListRoundTripNonEmpty(t *testing.T) {
	svc, _ := newService()

	w := serveREST(svc, http.MethodPost, "/v1/key-value", `{"name":"req-full-kv","plan":"free"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", w.Code, w.Body.String())
	}
	id, _ := decodeMap(t, w.Body.Bytes())["id"].(string)

	p := serveREST(svc, http.MethodPatch, "/v1/key-value/"+id,
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

	g := serveREST(svc, http.MethodGet, "/v1/key-value/"+id, "")
	if g.Code != http.StatusOK {
		t.Fatalf("get => %d: %s", g.Code, g.Body.String())
	}
	if entries, _ := decodeMap(t, g.Body.Bytes())["ipAllowList"].([]any); len(entries) != 1 {
		t.Errorf("get ipAllowList = %#v, want one entry", entries)
	}
}
