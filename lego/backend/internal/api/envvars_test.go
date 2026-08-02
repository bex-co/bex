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
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// memSecretStore is an in-memory core.SecretKV (path-keyed) for the server-level
// env-vars wiring tests (the deep behavior is covered in internal/secrets). A
// service's env map lives at "services/<svc>/env".
type memSecretStore struct {
	m           map[string]map[string]string
	versions    map[string]uint64
	casCalls    int
	failCASCall int
}

func newMemSecretStore() *memSecretStore {
	return &memSecretStore{m: map[string]map[string]string{}, versions: map[string]uint64{}}
}

func (s *memSecretStore) Get(_ context.Context, path string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range s.m[path] {
		out[k] = v
	}
	return out, nil
}

func (s *memSecretStore) Put(_ context.Context, path string, data map[string]string) error {
	cp := map[string]string{}
	for k, v := range data {
		cp[k] = v
	}
	s.m[path] = cp
	s.versions[path]++
	return nil
}

func (s *memSecretStore) Delete(_ context.Context, path string) error {
	delete(s.m, path)
	s.versions[path]++
	return nil
}

func (s *memSecretStore) GetVersioned(ctx context.Context, path string) (core.SecretKVSnapshot, error) {
	data, err := s.Get(ctx, path)
	return core.SecretKVSnapshot{Data: data, Version: s.versions[path]}, err
}

func (s *memSecretStore) PutCAS(ctx context.Context, path string, data map[string]string, expectedVersion uint64) (uint64, error) {
	s.casCalls++
	if s.failCASCall == s.casCalls {
		return 0, errors.New("injected /openbao/private/path failed-writer-secret")
	}
	if s.versions[path] != expectedVersion {
		return 0, core.ErrConflict
	}
	if err := s.Put(ctx, path, data); err != nil {
		return 0, err
	}
	return s.versions[path], nil
}

func (s *memSecretStore) List(_ context.Context, path string) ([]string, error) {
	prefix := path + "/"
	seen := map[string]bool{}
	var out []string
	for k := range s.m {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out, nil
}

type compensationConflictClient struct {
	client.Client
	store      *memSecretStore
	done       bool
	concurrent bool
}

func (c *compensationConflictClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if !c.done {
		c.done = true
		if c.concurrent {
			path := envKey("web")
			_, _ = c.store.PutCAS(ctx, path, map[string]string{"TOKEN": "concurrent-winner"}, c.store.versions[path])
		}
		return errors.New("injected App /private/path failure")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

// envKey is the store path a service's env map lives at.
func envKey(svc string) string { return "services/" + svc + "/env" }

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
	if store.m[envKey("web")]["FOO"] != "bar" {
		t.Fatal("REST PUT did not write the store")
	}

	// GraphQL, Render dashboard shape: the env page's real query is
	// `service(id){ envVarKeys{ id key } }` (operation serviceEnvVarKeys). Resolves
	// byte-for-byte via the service(id) alias; keys-only (no value in the list).
	svc := gql(t, h, `{ service(id:"web") { envVarKeys { id key value revision } } }`)["service"].(map[string]any)
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
	revision, ok := first["revision"].(string)
	if !ok || revision == "" || keys[1].(map[string]any)["revision"] != revision {
		t.Fatalf("envVarKeys should share one nonempty revision: %+v", keys)
	}

	// Per-key value fetch nested under the service ("Show secret"); server(id) alias too.
	one := gql(t, h, `{ server(id:"web") { envVar(key:"FOO") { id key value revision } } }`)["server"].(map[string]any)["envVar"].(map[string]any)
	if one["value"] != "bar" || one["id"] != "FOO" {
		t.Fatalf("nested envVar(key): %+v", one)
	}
	if one["revision"] != revision {
		t.Fatalf("masked list/reveal revisions differ: keys=%q reveal=%q", revision, one["revision"])
	}

	// The mobile single-key mutation echoes the reveal's whole-map revision.
	cas := gql(t, h, `mutation {
		patchServiceEnvironment(
			serviceId:"web",
			envVars:[{key:"FOO", value:"cas-updated"}],
			saveMode:"save_only",
			expectedEnvRevision:"`+revision+`"
		) { envVarKeys rolledOut }
	}`)["patchServiceEnvironment"].(map[string]any)
	if cas["rolledOut"] != false || store.m[envKey("web")]["FOO"] != "cas-updated" {
		t.Fatalf("revision-aware GraphQL mutation = %+v store=%+v", cas, store.m[envKey("web")])
	}

	// setEnvVar mutation (upsert-one) travels the same Service path.
	if gql(t, h, `mutation { setEnvVar(serviceId:"web", key:"NEW", value:"z") }`)["setEnvVar"] != true {
		t.Fatal("setEnvVar mutation should be true")
	}
	if store.m[envKey("web")]["NEW"] != "z" || store.m[envKey("web")]["FOO"] != "cas-updated" {
		t.Fatalf("setEnvVar should merge: %+v", store.m[envKey("web")])
	}

	// The new GraphQL list is the paged twin of REST's per-item envelope. The
	// old nested envVarKeys field above remains available for compatibility.
	page := gql(t, h, `{ envVars(serviceId:"web", limit:2) { envVar { id key } cursor } }`)["envVars"].([]any)
	if len(page) != 2 {
		t.Fatalf("envVars first page: %+v", page)
	}
	cursor := page[1].(map[string]any)["cursor"].(string)
	next := gql(t, h, `{ envVars(serviceId:"web", limit:2, cursor:"`+cursor+`") { envVar { key } cursor } }`)["envVars"].([]any)
	if len(next) != 1 || next[0].(map[string]any)["envVar"].(map[string]any)["key"] == page[1].(map[string]any)["envVar"].(map[string]any)["key"] {
		t.Fatalf("envVars second page: first=%+v next=%+v", page, next)
	}

	// Both GraphQL write shapes carry generateValue through the same core verb.
	if gql(t, h, `mutation { setEnvVar(serviceId:"web", key:"GENERATED_ONE", generateValue:true) }`)["setEnvVar"] != true {
		t.Fatal("generated setEnvVar mutation should be true")
	}
	if len(store.m[envKey("web")]["GENERATED_ONE"]) != 44 {
		t.Fatalf("single generated value = %q", store.m[envKey("web")]["GENERATED_ONE"])
	}
	if gql(t, h, `mutation { setEnvVars(serviceId:"web", envVars:[{key:"GENERATED_ALL", generateValue:true}]) }`)["setEnvVars"] != true {
		t.Fatal("generated setEnvVars mutation should be true")
	}
	if len(store.m[envKey("web")]["GENERATED_ALL"]) != 44 {
		t.Fatalf("bulk generated value = %q", store.m[envKey("web")]["GENERATED_ALL"])
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
	if code := do(t, h, "PATCH", "/v1/services/web/environment", testToken, `{"saveMode":"save_only"}`).Code; code != 503 {
		t.Errorf("PATCH environment without a store => 503, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/services/web", testToken, "").Code; code != 200 {
		t.Errorf("service read unaffected by nil store, got %d", code)
	}
}

func TestEnvironmentBatch_RESTAndGraphQLWiring(t *testing.T) {
	store := newMemSecretStore()
	store.m[envKey("web")] = map[string]string{"KEEP": "opaque-secret", "RENAME": "rename-secret"}
	store.m["services/web/files"] = map[string]string{"keep.pem": "opaque-file"}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default",
		Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	h, _ := serverWith(t, base, Deps{Secrets: store})

	rec := do(t, h, "PATCH", "/v1/services/web/environment", testToken,
		`{"saveMode":"save_only","envVars":[{"key":"RENAMED","fromKey":"RENAME"},{"key":"ADDED","value":"new-secret"}],"secretFiles":[{"name":"new.pem","content":"new-file"}]}`)
	if rec.Code != 200 {
		t.Fatalf("REST batch => %d: %s", rec.Code, rec.Body.String())
	}
	for _, secret := range []string{"opaque-secret", "rename-secret", "new-secret", "opaque-file", "new-file"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("REST batch response leaked %q: %s", secret, rec.Body.String())
		}
	}
	if store.m[envKey("web")]["KEEP"] != "opaque-secret" || store.m[envKey("web")]["RENAMED"] != "rename-secret" {
		t.Fatalf("REST batch lost omitted/renamed values: %+v", store.m[envKey("web")])
	}

	result := gql(t, h, `mutation {
		patchServiceEnvironment(
			serviceId:"web",
			envVars:[{key:"ADDED", delete:true}],
			secretFiles:[{name:"keep.pem", delete:true}],
			saveMode:"deploy"
		) { envVarKeys secretFileNames rolledOut }
	}`)["patchServiceEnvironment"].(map[string]any)
	if result["rolledOut"] != true {
		t.Fatalf("GraphQL deploy result: %+v", result)
	}
	if _, ok := store.m[envKey("web")]["ADDED"]; ok {
		t.Fatalf("GraphQL batch did not delete env var: %+v", store.m[envKey("web")])
	}
	if _, ok := store.m["services/web/files"]["keep.pem"]; ok {
		t.Fatalf("GraphQL batch did not delete file: %+v", store.m["services/web/files"])
	}
}

func TestEnvironmentCASCompensationPreservesGraphQLErrorCodes(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		concurrent  bool
		failCASCall int
	}{
		{name: "restored", code: "ENVIRONMENT_UPDATE_RESTORED"},
		{name: "newer owner", code: "ENVIRONMENT_REVISION_CONFLICT", concurrent: true},
		{name: "restoration failed", code: "ENVIRONMENT_RESTORATION_FAILED", failCASCall: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemSecretStore()
			store.m[envKey("web")] = map[string]string{"TOKEN": "before-secret"}
			store.failCASCall = tc.failCASCall
			cl := &compensationConflictClient{Client: fakeClient(sampleApp("web")), store: store, concurrent: tc.concurrent}
			base := &core.Base{Client: cl, Namespace: "default", Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
			h, _ := serverWith(t, base, Deps{Secrets: store})

			revealed := gql(t, h, `{ service(id:"web") { envVar(key:"TOKEN") { revision } } }`)["service"].(map[string]any)["envVar"].(map[string]any)
			revision := revealed["revision"].(string)
			body, _ := json.Marshal(map[string]any{
				"query": `mutation Save($revision: String!) {
					patchServiceEnvironment(
						serviceId:"web",
						envVars:[{key:"TOKEN", value:"failed-writer-secret"}],
						saveMode:"deploy",
						expectedEnvRevision:$revision
					) { envVarKeys rolledOut }
				}`,
				"variables": map[string]any{"revision": revision},
			})
			w := do(t, h, "POST", "/graphql", testToken, string(body))
			if w.Code != 200 {
				t.Fatalf("GraphQL HTTP status = %d: %s", w.Code, w.Body.String())
			}
			var response struct {
				Errors []struct {
					Message    string         `json:"message"`
					Extensions map[string]any `json:"extensions"`
				} `json:"errors"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode GraphQL response: %v", err)
			}
			if len(response.Errors) != 1 || response.Errors[0].Extensions["code"] != tc.code {
				t.Fatalf("GraphQL errors = %#v, body=%s", response.Errors, w.Body.String())
			}
			for _, material := range []string{"TOKEN", "before-secret", "failed-writer-secret", "concurrent-winner", "/openbao/private/path", "/private/path", revision} {
				if strings.Contains(w.Body.String(), material) {
					t.Fatalf("GraphQL outcome leaked %q: %s", material, w.Body.String())
				}
			}
		})
	}
}
