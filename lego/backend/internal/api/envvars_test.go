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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// memSecretStore is an in-memory secrets.SecretStore for the server-level
// env-vars wiring tests (the deep behavior is covered in internal/secrets).
type memSecretStore struct{ m map[string]map[string]string }

func newMemSecretStore() *memSecretStore { return &memSecretStore{m: map[string]map[string]string{}} }

func (s *memSecretStore) GetEnv(_ context.Context, svc string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range s.m[svc] {
		out[k] = v
	}
	return out, nil
}

func (s *memSecretStore) PutEnv(_ context.Context, svc string, env map[string]string) error {
	cp := map[string]string{}
	for k, v := range env {
		cp[k] = v
	}
	s.m[svc] = cp
	return nil
}

func (s *memSecretStore) DeleteEnv(_ context.Context, svc string) error {
	delete(s.m, svc)
	return nil
}

// TestEnvVars_RESTAndGraphQLDashboardShape drives the wired server: REST is the
// Render public-API shape (flat {key,value}), GraphQL is the dashboard shape
// (env vars nested under the service, keys-only + per-key value fetch).
func TestEnvVars_RESTAndGraphQLDashboardShape(t *testing.T) {
	store := newMemSecretStore()
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default",
		Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	h, _ := serverWith(t, base, Deps{Secrets: store})

	// REST replace-all writes to the store (public-API surface).
	if code := do(t, h, "PUT", "/v1/services/web/env-vars", testToken,
		`[{"key":"FOO","value":"bar"},{"key":"BAZ","value":"qux"}]`).Code; code != 200 {
		t.Fatalf("PUT env-vars => 200, got %d", code)
	}
	if store.m["web"]["FOO"] != "bar" {
		t.Fatal("REST PUT did not write the store")
	}

	// GraphQL, Render dashboard shape: the env page's real query is
	// `service(id){ envVarKeys{ id key } }` (operation serviceEnvVarKeys). Resolves
	// byte-for-byte via the service(id) alias; keys-only (no value in the list).
	svc := gql(t, h, `{ service(id:"web") { envVarKeys { id key value } } }`)["service"].(map[string]any)
	keys := svc["envVarKeys"].([]any)
	if len(keys) != 2 {
		t.Fatalf("envVarKeys: %+v", keys)
	}
	first := keys[0].(map[string]any)
	if first["key"] != "BAZ" || first["id"] != "BAZ" {
		t.Fatalf("envVarKeys should be sorted with id==key: %+v", first)
	}
	if first["value"] != "" {
		t.Fatalf("envVarKeys is keys-only — value must be empty, got %q", first["value"])
	}

	// Per-key value fetch nested under the service ("Show secret"); server(id) alias too.
	one := gql(t, h, `{ server(id:"web") { envVar(key:"FOO") { id key value } } }`)["server"].(map[string]any)["envVar"].(map[string]any)
	if one["value"] != "bar" || one["id"] != "FOO" {
		t.Fatalf("nested envVar(key): %+v", one)
	}

	// setEnvVar mutation (upsert-one) travels the same Service path.
	if gql(t, h, `mutation { setEnvVar(serviceId:"web", key:"NEW", value:"z") }`)["setEnvVar"] != true {
		t.Fatal("setEnvVar mutation should be true")
	}
	if store.m["web"]["NEW"] != "z" || store.m["web"]["FOO"] != "bar" {
		t.Fatalf("setEnvVar should merge: %+v", store.m["web"])
	}
}

// TestEnvVars_UnconfiguredIs503 confirms that with no secret store wired the
// env-vars REST routes 503 while the rest of the API is unaffected.
func TestEnvVars_UnconfiguredIs503(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	if code := do(t, h, "GET", "/v1/services/web/env-vars", testToken, "").Code; code != 503 {
		t.Errorf("GET env-vars without a store => 503, got %d", code)
	}
	if code := do(t, h, "PUT", "/v1/services/web/env-vars", testToken, `[]`).Code; code != 503 {
		t.Errorf("PUT env-vars without a store => 503, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/services/web", testToken, "").Code; code != 200 {
		t.Errorf("service read unaffected by nil store, got %d", code)
	}
}
