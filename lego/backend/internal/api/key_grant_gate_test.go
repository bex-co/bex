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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// key_grant_gate_test.go is w9/m496 t005's dispatch-gate proof without
// external dependencies: a scoped API key is refused, on REST, GraphQL and
// MCP alike, every operation its grant does not cover, whatever its creator
// may do (here: authorization off, so the grant is the only thing that can
// refuse). The real-Postgres and real-OpenFGA proof, where the grant also
// composes with the creator ceiling, is TestScopedAPIKeyE2E.

// grantStore is a core.KeyGrantResolver over a map the test edits between
// requests, as the edit verb does the stored row.
type grantStore struct {
	mu     sync.Mutex
	grants map[string]core.KeyGrant
	err    error
	reads  int
}

func (g *grantStore) APIKeyGrant(_ context.Context, clientID string) (core.KeyGrant, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.reads++
	if g.err != nil {
		return core.KeyGrant{}, g.err
	}
	if grant, ok := g.grants[clientID]; ok {
		return grant, nil
	}
	return core.AllKeyGrant(), nil
}

func (g *grantStore) set(clientID string, grant core.KeyGrant) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.grants[clientID] = grant
}

func grantOf(entries ...string) core.KeyGrant {
	if entries == nil {
		entries = []string{}
	}
	return core.KeyGrantFromEntries(entries)
}

func scopedKeyServer(t *testing.T) (*Server, *grantStore) {
	t.Helper()
	grants := &grantStore{grants: map[string]core.KeyGrant{}}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", KeyGrants: grants}
	srv := NewServer(base, Deps{})
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatal(err)
	}
	srv.schema = schema
	return srv, grants
}

func restAs(t *testing.T, srv *Server, id core.Identity, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(core.WithIdentity(context.Background(), id))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux := srv.restHandler()
	srv.withScopeClassREST(mux, mux).ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func grantRefused(status int, body map[string]any) bool {
	return status == http.StatusForbidden && body["code"] == core.KeyPermissionDeniedCode
}

func TestScopedKeyIsRefusedOutsideItsGrantOnEverySurface(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	key := keyOf("key-services-read")
	grants.set(key.Subject, grantOf("services:read"))

	t.Run("REST", func(t *testing.T) {
		for _, call := range []struct{ method, path, body string }{
			{http.MethodGet, "/v1/services", ""},
			{http.MethodGet, "/v1/services/web", ""},
		} {
			if status, body := restAs(t, srv, key, call.method, call.path, call.body); grantRefused(status, body) {
				t.Errorf("Services Read only key refused %s %s: %v", call.method, call.path, body)
			}
		}
		for _, call := range []struct{ method, path, body string }{
			{http.MethodPost, "/v1/services", `{"type":"web_service","name":"x"}`},
			{http.MethodDelete, "/v1/services/web", ""},
			{http.MethodPut, "/v1/services/web/env-vars/K", `{"value":"v"}`},
			{http.MethodGet, "/v1/services/web/env-vars", ""},
			{http.MethodPost, "/v1/services/web/deploys", `{}`},
			{http.MethodGet, "/v1/services/web/deploys", ""},
			{http.MethodGet, "/v1/postgres", ""},
		} {
			status, body := restAs(t, srv, key, call.method, call.path, call.body)
			if !grantRefused(status, body) {
				t.Errorf("Services Read only key %s %s = %d %v, want 403 %s", call.method, call.path, status, body, core.KeyPermissionDeniedCode)
			}
		}
		// The refusal names what is missing, in Render's error shape.
		_, body := restAs(t, srv, key, http.MethodPost, "/v1/services/web/deploys", `{}`)
		if msg, _ := body["message"].(string); !strings.Contains(msg, "Trigger and cancel deploys") || body["id"] != "forbidden" {
			t.Errorf("refusal = %v, want Render's shape naming the permission", body)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		if out := graphqlDo(t, srv, key, `{ services { id } }`); graphqlCode(out) == core.KeyPermissionDeniedCode {
			t.Errorf("Services Read only key refused a services read: %v", out)
		}
		for _, doc := range []string{
			`mutation { deleteService(id: "web") }`,
			`mutation { setEnvVar(id: "web", key: "K", value: "v") { id } }`,
			// A batch cannot slip a refused field past a permitted one.
			`query A { services { id } } mutation B { deleteService(id: "web") }`,
			`{ services { id } deploys(serviceId: "web") { id } }`,
			// A secret through a nested field of a permitted root.
			`{ service(id: "web") { envVar(key: "K") { value } } }`,
		} {
			if out := graphqlDo(t, srv, key, doc); graphqlCode(out) != core.KeyPermissionDeniedCode {
				t.Errorf("Services Read only key ran %s: %v", doc, out)
			}
		}
	})

	t.Run("MCP", func(t *testing.T) {
		if res := mcpCall(t, srv, key, "list_services", map[string]any{}); strings.Contains(mcpText(res), core.KeyPermissionDeniedCode) ||
			strings.Contains(mcpText(res), "not granted") {
			t.Errorf("Services Read only key refused list_services: %s", mcpText(res))
		}
		for tool, args := range map[string]map[string]any{
			"delete_service":          {"serviceId": "web"},
			"trigger_deploy":          {"serviceId": "web"},
			"set_env_var":             {"serviceId": "web", "key": "K", "value": "v"},
			"list_postgres_instances": {},
		} {
			res := mcpCall(t, srv, key, tool, args)
			if !res.IsError || !strings.Contains(mcpText(res), "not granted") {
				t.Errorf("Services Read only key ran %s: %s", tool, mcpText(res))
			}
		}
	})
}

// A grant is several permissions, each at its own level: Deploys Run with
// Services Read triggers deploys and still cannot create a service; Render's
// Write-without-Read sets a value it cannot read back.
func TestScopedKeyGrantComposesPermissionsAndLevels(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	deployer := keyOf("key-deployer")
	grants.set(deployer.Subject, grantOf("services:read", "deploys:run", "deploys_run:run"))
	for _, call := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/services/web/deploys", `{}`},
		{http.MethodPost, "/v1/services/web/deploys/dep-x/cancel", `{}`},
		{http.MethodPost, "/v1/services/web/rollback", `{"deployId":"dep-x"}`},
		{http.MethodGet, "/v1/services/web/deploys", ""},
	} {
		if status, body := restAs(t, srv, deployer, call.method, call.path, call.body); grantRefused(status, body) {
			t.Errorf("Deploys Run key refused %s %s: %v", call.method, call.path, body)
		}
	}
	if status, body := restAs(t, srv, deployer, http.MethodPost, "/v1/services", `{"type":"web_service","name":"x"}`); !grantRefused(status, body) {
		t.Errorf("Deploys Run key created a service: %d %v", status, body)
	}

	writer := keyOf("key-env-writer")
	grants.set(writer.Subject, grantOf("services_env:write"))
	if status, body := restAs(t, srv, writer, http.MethodPut, "/v1/services/web/env-vars/K", `{"value":"v"}`); grantRefused(status, body) {
		t.Errorf("Write-only key refused setting a value: %v", body)
	}
	if status, body := restAs(t, srv, writer, http.MethodGet, "/v1/services/web/env-vars/K", ""); !grantRefused(status, body) {
		t.Errorf("Write-only key read a value back: %d %v", status, body)
	}
}

// I7: an unreadable grant refuses the request (503), never falls back to the
// creator's role; an operation with no entry is refused to a scoped key.
func TestScopedKeyGrantFailsClosed(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	key := keyOf("key-any")
	grants.set(key.Subject, grantOf("services:read"))

	grants.err = errors.New("postgres: connection refused")
	if status, body := restAs(t, srv, key, http.MethodGet, "/v1/services", ""); status != http.StatusServiceUnavailable {
		t.Errorf("unreadable grant = %d %v, want 503", status, body)
	}
	if out := graphqlDo(t, srv, key, `{ services { id } }`); out["data"] != nil || !strings.Contains(out["errors"].([]any)[0].(map[string]any)["message"].(string), "unavailable") {
		t.Errorf("unreadable grant answered GraphQL: %v", out)
	}
	grants.err = nil

	ctx := core.WithIdentity(context.Background(), key)
	err := srv.requireKeyGrant(ctx, "REST GET /v1/an-operation-nobody-mapped")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.KeyPermissionDeniedCode {
		t.Errorf("unmapped operation = %v, want the grant refusal", err)
	}
	// Even an empty grant reaches what every key may: its own identity.
	grants.set(key.Subject, grantOf())
	if err := srv.requireKeyGrant(ctx, "REST GET /v1/users", "REST GET /v1/owners"); err != nil {
		t.Errorf("Clear all key refused its own identity: %v", err)
	}
	// And All reaches the unmapped too (its own gates decide), at no grant cost.
	grants.set(key.Subject, core.AllKeyGrant())
	if err := srv.requireKeyGrant(ctx, "REST GET /v1/an-operation-nobody-mapped"); err != nil {
		t.Errorf("All key refused by the grant gate: %v", err)
	}
}

// I8: a narrowed grant refuses on the key's very next request; the grant is
// read on every request, never cached.
func TestNarrowedGrantRefusesTheNextRequest(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	key := keyOf("key-narrowed")
	grants.set(key.Subject, grantOf("services:read_write"))
	if status, body := restAs(t, srv, key, http.MethodDelete, "/v1/services/web", ""); grantRefused(status, body) {
		t.Fatalf("Services Read & write key refused a delete: %v", body)
	}
	grants.set(key.Subject, grantOf("services:read"))
	if status, body := restAs(t, srv, key, http.MethodDelete, "/v1/services/web", ""); !grantRefused(status, body) {
		t.Fatalf("narrowed key deleted on its next request: %d %v", status, body)
	}
	before := grants.reads
	restAs(t, srv, key, http.MethodGet, "/v1/services", "")
	restAs(t, srv, key, http.MethodGet, "/v1/services", "")
	if grants.reads-before != 2 {
		t.Errorf("two requests read the grant %d times, want once each", grants.reads-before)
	}
}

// Sessions and other callers never consult a grant.
func TestOnlyAPIKeysAreHeldToAGrant(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	grants.err = errors.New("must not be read")
	session := core.Identity{Subject: "person", Method: "session"}
	if status, body := restAs(t, srv, session, http.MethodGet, "/v1/services", ""); status == http.StatusServiceUnavailable || grantRefused(status, body) {
		t.Errorf("a session was held to a grant: %d %v", status, body)
	}
}
