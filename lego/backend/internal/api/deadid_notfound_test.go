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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// deadid_notfound_test.go is w6/m44: a dead resource id must resolve to GraphQL
// `null` and REST 404 on every single-resource read verb — never to an all-zero
// object that a client cannot tell apart from a real, empty resource.
//
// The defect this pins is graphql-go's, not any one feature's: its executor
// leaks a resolver's un-completed return value into the response when that
// resolver also returns an error, so every `func(ctx, id) (SomeView, error)`
// verb answered `{"id":"","name":"", ...}`. gqlutil.NilOnError closes it for the
// whole composed schema, which is why this test sweeps the schema rather than
// asserting one verb — a new read verb joins the sweep automatically.

// deadIDBase seeds one live App/Database/KeyValue and nothing else, so any other
// id the tests ask for is provably absent. Deliberately store-off (no Authz, no
// Workspace resolver): m44 is about the ABSENT-vs-present answer, and a
// tenant-scoped fixture would fold authorization denials into the same
// not-found channel and make a passing test ambiguous. The cross-workspace
// matrices already own the denial half.
func deadIDBase() *core.Base {
	return &core.Base{
		Client:    fakeClient(sampleApp("web"), ownedDB("pg", ""), ownedKV("kv", "")),
		Namespace: "default",
	}
}

func deadIDIdentity() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "qa", Method: "session"})
}

// deadIDVerbs are the single-resource read verbs w6/m44's DoD names, each with
// the live id seeded by deadIDBase. `server` and `service` are the same resolver
// under Render's two dashboard spellings; both are listed so a rename can't
// quietly drop one.
var deadIDVerbs = []struct{ field, live string }{
	{"server", "web"},
	{"service", "web"},
	{"database", "pg"},
	{"keyValue", "kv"},
}

// TestDeadIDResolvesToNull is the contract itself: a dead id is `null` plus an
// error envelope, and a live id still resolves to data. Both branches matter —
// niling an errored field must not nil a working one.
func TestDeadIDResolvesToNull(t *testing.T) {
	srv := wireAllFeatures(deadIDBase())
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := deadIDIdentity()

	for _, v := range deadIDVerbs {
		t.Run(v.field, func(t *testing.T) {
			dead := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ ` + v.field + `(id: "does-not-exist") { id name } }`})
			got, ok := dead.Data.(map[string]any)[v.field]
			if !ok {
				t.Fatalf("%s: no %q key in data %#v", v.field, v.field, dead.Data)
			}
			if got != nil {
				t.Errorf("%s(dead) = %#v, want nil — a zero-value object is indistinguishable "+
					"from a real empty resource and defeats the dashboard's !resource predicate", v.field, got)
			}
			if len(dead.Errors) == 0 {
				t.Errorf("%s(dead): no errors entry — a dead id must say WHY, not just go quiet", v.field)
			}

			live := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ ` + v.field + `(id: "` + v.live + `") { id name } }`})
			if len(live.Errors) > 0 {
				t.Fatalf("%s(live): %v", v.field, live.Errors)
			}
			row, _ := live.Data.(map[string]any)[v.field].(map[string]any)
			if row == nil || row["id"] != v.live {
				t.Errorf("%s(live) = %#v, want the seeded resource — the null-on-error guard "+
					"must not touch a resolver that succeeded", v.field, live.Data)
			}
		})
	}
}

// TestDeadIDSweepsEverySingleResourceReadVerb is the anti-regression half: the
// leak lived in a SHARED executor path, so it reappears on any read verb built
// later. Sweeping every `(id: String!)`-shaped root query proves the guard is
// schema-wide, not four hand-patched verbs.
func TestDeadIDSweepsEverySingleResourceReadVerb(t *testing.T) {
	srv := wireAllFeatures(deadIDBase())
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := deadIDIdentity()

	swept := 0
	for name, def := range schema.QueryType().Fields() {
		if _, ok := graphql.GetNamed(def.Type).(*graphql.Object); !ok {
			continue // scalar/list-of-scalar verbs have no zero-value object to leak
		}
		if !idAddressable(def) {
			continue
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
			RequestString: `{ ` + name + `(id: "does-not-exist") { __typename } }`})
		data, ok := res.Data.(map[string]any)
		if !ok {
			continue // the document never executed (a validation error) — nothing to leak
		}
		swept++
		if got := data[name]; got != nil && len(res.Errors) > 0 {
			t.Errorf("%s(dead) errored but still returned %#v — graphql-go's un-completed "+
				"resolver value leaked past gqlutil.NilOnError", name, got)
		}
	}
	if swept <= len(deadIDVerbs) {
		t.Fatalf("only %d id-shaped read verbs swept — schema assembly broke?", swept)
	}
	t.Logf("%d single-resource read verbs sweep clean", swept)
}

// idAddressable reports whether a root query can be asked for ONE resource by
// id alone: it takes an `id` argument and every other argument is optional, so
// filling just `id` produces a valid document. That is the whole family the
// zero-value-object leak applied to — `blueprint(id, ownerId)` included, which
// is why the sweep tests the shape rather than an argument count.
func idAddressable(def *graphql.FieldDefinition) bool {
	hasID := false
	for _, a := range def.Args {
		if a.Name() == "id" {
			hasID = true
			continue
		}
		if _, required := a.Type.(*graphql.NonNull); required {
			return false
		}
	}
	return hasID
}

// TestDeadIDBlueprintResolvesToNull covers the verb that proves the fix had to
// live in the schema, not in gqlutil's verb helpers: `blueprint` is a
// hand-written &graphql.Field literal that never goes through KeyVerb, and it
// leaked the same zero-value object — surfacing as a "Something went wrong"
// blueprint page titled with the raw bpt-… id.
func TestDeadIDBlueprintResolvesToNull(t *testing.T) {
	srv := wireAllFeatures(deadIDBase())
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: deadIDIdentity(),
		RequestString: `{ blueprint(id: "bpt-does-not-exist") { id name } }`})
	if got := res.Data.(map[string]any)["blueprint"]; got != nil {
		t.Errorf("blueprint(dead) = %#v, want nil", got)
	}
	if len(res.Errors) == 0 {
		t.Error("blueprint(dead): no errors entry")
	}
}

// TestDeadIDOverTheWire is the end-to-end proof, in the exact bytes a client
// reads: the QA hunt that opened m44 was reading a RESPONSE BODY, not a
// resolver return, and two layers sit between them — json encoding and
// sanitizeGraphQLErrors. The second one matters more than it looks: the
// dashboard tells a dead id from an outage by matching "not found" in the
// message, so if the sanitizer ever swallowed core.ErrNotFound into "internal
// error", every detail page would land back on its retry state instead of
// redirecting. Both halves are pinned here.
func TestDeadIDOverTheWire(t *testing.T) {
	srv := wireAllFeatures(deadIDBase())
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	srv.schema = schema

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/graphql",
		strings.NewReader(`{"query":"{ server(id: \"does-not-exist\") { id name slug phase } }"}`)).
		WithContext(deadIDIdentity())
	srv.graphqlHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /graphql => %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		Data   map[string]json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	if got := string(out.Data["server"]); got != "null" {
		t.Errorf("wire data.server = %s, want null — this is the exact byte sequence "+
			"the dashboard's !resource predicate reads", got)
	}
	if len(out.Errors) != 1 || !strings.Contains(strings.ToLower(out.Errors[0].Message), "not found") {
		t.Errorf("wire errors = %+v, want one message containing \"not found\" — the dashboard's "+
			"resourceNotFound() matches on it to tell a deleted resource from an outage", out.Errors)
	}
}

// TestDeadIDMCPReportsNotFound is the third surface: an MCP agent asking for a
// dead id must get a tool ERROR, never a resource whose every field is empty —
// an agent has no dashboard to notice the difference and would go on to act on
// a service that does not exist.
func TestDeadIDMCPReportsNotFound(t *testing.T) {
	srv := wireAllFeatures(deadIDBase())
	cs := mcpSessionAs(t, srv, "qa")
	ctx := context.Background()

	for _, tc := range []struct{ tool, arg, live string }{
		{"get_service", "serviceId", "web"},
		{"get_postgres", "postgresId", "pg"},
		{"get_key_value", "keyValueId", "kv"},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			dead, err := cs.CallTool(ctx, &mcp.CallToolParams{
				Name: tc.tool, Arguments: map[string]any{tc.arg: "does-not-exist"}})
			if err == nil && (dead == nil || !dead.IsError) {
				t.Errorf("%s(dead) returned a result, not an error: %+v", tc.tool, dead)
			}

			live, err := cs.CallTool(ctx, &mcp.CallToolParams{
				Name: tc.tool, Arguments: map[string]any{tc.arg: tc.live}})
			if err != nil || live == nil || live.IsError {
				t.Errorf("%s(live) => err=%v result=%+v, want the seeded resource", tc.tool, err, live)
			}
		})
	}
}

// TestDeadIDRESTReturns404 pins the REST half of the same contract: the surfaces
// must agree, so a client that gets `null` from GraphQL gets 404 from REST for
// the very same id.
func TestDeadIDRESTReturns404(t *testing.T) {
	base := deadIDBase()
	srv := wireAllFeatures(base)
	mux := http.NewServeMux()
	srv.Apps.RegisterREST(mux)
	srv.Postgres.RegisterREST(mux)
	srv.KeyValue.RegisterREST(mux)

	for _, tc := range []struct{ path, live string }{
		{"/v1/services/", "web"},
		{"/v1/postgres/", "pg"},
		{"/v1/key-value/", "kv"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path+"does-not-exist", nil).WithContext(deadIDIdentity())
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Errorf("GET %sdoes-not-exist => %d, want 404: %s", tc.path, rec.Code, rec.Body)
			}

			req = httptest.NewRequest("GET", tc.path+tc.live, nil).WithContext(deadIDIdentity())
			rec = httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s%s => %d, want 200: %s", tc.path, tc.live, rec.Code, rec.Body)
			}
		})
	}
}
