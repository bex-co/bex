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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// renderSandboxSchemas loads Render's pinned sandbox schema (the create, get
// and list item body) and its list item, which the SDK's generated models
// decode every sandbox read against. The id keeps bex's 20-character sbx- ids
// beside Render's 25, as the request gate widens it (renderSandboxIDPattern,
// ADR018 round 5); neither SDK checks the id's shape.
func renderSandboxSchemas(t *testing.T) (sandbox, listItem *openapi3.Schema) {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	idSchema := openapi3.NewStringSchema().WithPattern(`^sbx-[0-9a-v]{20}(?:[0-9a-v]{5})?$`).NewRef()
	widenID := func(sc *openapi3.Schema) {
		if sc == nil {
			return
		}
		if _, ok := sc.Properties["id"]; ok {
			sc.Properties["id"] = idSchema
		}
		for _, sub := range sc.AllOf {
			if sub != nil && sub.Value != nil {
				if _, ok := sub.Value.Properties["id"]; ok {
					sub.Value.Properties["id"] = idSchema
				}
			}
		}
	}
	sandbox = spec.Paths.Find("/sandboxes").Post.Responses.Status(http.StatusCreated).Value.Content.Get("application/json").Schema.Value
	widenID(sandbox)
	item := spec.Components.Schemas["sandboxWithCursor"].Value
	if s := item.Properties["sandbox"]; s != nil {
		widenID(s.Value)
	}
	return sandbox, item
}

// The Render SDK v1.8 Python client validates every sandbox it reads against
// Render's schema, whose required fields include region and createdAt, and
// cursor on each list item (w32/m284). A response missing one made create, get
// and list raise even though the call succeeded. The production walk of
// scripts/render-sdk-compat.sh (t002) decoded every read; this pins it against
// the pinned schema, so a field the next pin makes required fails here.
func TestSandboxReadsCarryRenderRequiredFields(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	svc.Region = "frankfurt"
	svc.Names = &memNames{}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	sandboxSchema, itemSchema := renderSandboxSchemas(t)
	conforms := func(step string, schema *openapi3.Schema, doc any) {
		t.Helper()
		if err := schema.VisitJSON(doc); err != nil {
			t.Errorf("%s is not Render's shape: %v", step, err)
		}
	}
	read := func(method, path, body string, status int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(callerCtx()))
		if rec.Code != status {
			t.Fatalf("%s %s = %d %s", method, path, rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}

	var created map[string]any
	_ = json.Unmarshal(read(http.MethodPost, "/v1/sandboxes", `{"template":"node","name":"sdk","networkPolicy":{"type":"deny-all"}}`, http.StatusCreated), &created)
	if created["region"] != "frankfurt" || created["createdAt"] == nil {
		t.Fatalf("create body lacks region/createdAt: %v", created)
	}
	conforms("create", sandboxSchema, created)
	id, _ := created["id"].(string)

	var got map[string]any
	_ = json.Unmarshal(read(http.MethodGet, "/v1/sandboxes/"+id, "", http.StatusOK), &got)
	conforms("get", sandboxSchema, got)

	var items []map[string]any
	if err := json.Unmarshal(read(http.MethodGet, "/v1/sandboxes", "", http.StatusOK), &items); err != nil || len(items) != 1 {
		t.Fatalf("list = %v (%v)", items, err)
	}
	conforms("list item", itemSchema, items[0])
	sb, _ := items[0]["sandbox"].(map[string]any)
	if items[0]["cursor"] != id || sb["region"] != "frankfurt" {
		t.Fatalf("list item = %v, want cursor = id and the platform region", items[0])
	}

	// The check is real: a read without a required field is refused.
	delete(got, "createdAt")
	if sandboxSchema.VisitJSON(got) == nil {
		t.Fatal("a sandbox without createdAt validated; the schema check is vacuous")
	}
}

// A page is cut only when asked for; the cursor resumes after that sandbox in
// the newest-first order (equal creation times fall back to the id).
func TestSandboxListPagesOnlyWhenAsked(t *testing.T) {
	var rows []string
	for i := 0; i < 3; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":"os-%d","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox","bex.co/sandbox-id":"sbx-%d"},"status":{"state":"Running"}}`, i, i))
	}
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	})
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ids := func(query string) []string {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sandboxes"+query, nil).WithContext(callerCtx()))
		var items []SandboxEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("%s: %d %s", query, rec.Code, rec.Body)
		}
		var out []string
		for _, it := range items {
			out = append(out, it.Cursor)
		}
		return out
	}
	if got := ids(""); len(got) != 3 {
		t.Fatalf("unpaged list = %v, want all three", got)
	}
	if got := ids("?limit=2"); strings.Join(got, ",") != "sbx-2,sbx-1" {
		t.Fatalf("first page = %v", got)
	}
	if got := ids("?limit=2&cursor=sbx-1"); strings.Join(got, ",") != "sbx-0" {
		t.Fatalf("second page = %v", got)
	}
}

// memNames is an in-memory NameStore.
type memNames struct {
	mu    sync.Mutex
	names map[string]string
}

func (m *memNames) SetSandboxName(_ context.Context, _, id, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.names == nil {
		m.names = map[string]string{}
	}
	m.names[id] = name
	return nil
}

func (m *memNames) DeleteSandboxName(_ context.Context, _, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.names, id)
	return nil
}

func (m *memNames) SandboxNames(context.Context, string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	for k, v := range m.names {
		out[k] = v
	}
	return out, nil
}

func (m *memNames) PruneSandboxNames(context.Context, string, []string) error { return nil }

// w5/m287: Render CLI v2.36 `sandboxes create --name` and `list --name` —
// a named create answers its name, a list shows names (null when unnamed)
// and filters on an exact one, and Render's rule refuses a bad name before
// anything is created.
func TestSandboxNames(t *testing.T) {
	var mu sync.Mutex
	var rows []string
	failCreate := false
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost {
			if failCreate {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var body createRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := body.Metadata[metadataPublicID]
			rows = append(rows, fmt.Sprintf(`{"id":"os-%d","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox","bex.co/sandbox-id":%q},"status":{"state":"Running"}}`, len(rows), id))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"os-x","status":{"state":"Running"}}`))
			return
		}
		_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	})
	svc.DefaultTemplate = "node"
	names := &memNames{}
	svc.Names = names
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	do := func(method, path, body string) (int, string) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(callerCtx()))
		return rec.Code, rec.Body.String()
	}

	for _, bad := range []string{strings.Repeat("a", 65), "sbx-mine"} {
		if code, body := do(http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"name":%q}`, bad)); code != http.StatusBadRequest || !strings.Contains(body, CodeInvalidSandboxName) {
			t.Errorf("name %q: %d %s, want invalid_sandbox_name", bad, code, body)
		}
	}
	if len(rows) != 0 {
		t.Fatal("a refused name reached OpenSandbox")
	}
	code, body := do(http.MethodPost, "/v1/sandboxes", `{"name":"my agent ✨"}`)
	var created Sandbox
	if err := json.Unmarshal([]byte(body), &created); code != http.StatusCreated || err != nil || created.Name == nil || *created.Name != "my agent ✨" {
		t.Fatalf("named create = %d %s", code, body)
	}
	if code, body = do(http.MethodPost, "/v1/sandboxes", `{}`); code != http.StatusCreated || !strings.Contains(body, `"name":null`) {
		t.Fatalf("unnamed create = %d %s, want name null", code, body)
	}
	if _, body = do(http.MethodPost, "/v1/sandboxes", `{"name":"my agent ✨"}`); !strings.Contains(body, `"name":"my agent ✨"`) {
		t.Fatalf("second same-named create = %s, want accepted (names are not unique)", body)
	}

	list := func(query string) []SandboxEnvelope {
		t.Helper()
		code, body := do(http.MethodGet, "/v1/sandboxes"+query, "")
		var items []SandboxEnvelope
		if err := json.Unmarshal([]byte(body), &items); code != http.StatusOK || err != nil {
			t.Fatalf("list %s = %d %s", query, code, body)
		}
		return items
	}
	if got := list(""); len(got) != 3 {
		t.Fatalf("list = %d sandboxes, want 3", len(got))
	}
	named := list("?name=" + url.QueryEscape("my agent ✨"))
	if len(named) != 2 || named[0].Sandbox.Name == nil || *named[0].Sandbox.Name != "my agent ✨" {
		t.Fatalf("list --name = %+v, want the two named sandboxes", named)
	}
	if got := list("?name=other"); len(got) != 0 {
		t.Fatalf("list --name other = %d, want none", len(got))
	}
	if code, body := do(http.MethodGet, "/v1/sandboxes/"+created.ID, ""); code != http.StatusOK || !strings.Contains(body, `"name":"my agent ✨"`) {
		t.Fatalf("get = %d %s, want its name", code, body)
	}

	// A create that fails leaves no name behind.
	mu.Lock()
	failCreate = true
	mu.Unlock()
	before := len(names.names)
	if code, _ := do(http.MethodPost, "/v1/sandboxes", `{"name":"doomed"}`); code < 500 {
		t.Fatalf("failing create = %d, want a server error", code)
	}
	if len(names.names) != before {
		t.Errorf("a failed create left its name: %v", names.names)
	}
}
