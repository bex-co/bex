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
)

// The Render SDK v1.8 Python client validates every sandbox it reads against
// Render's schema, which requires region and createdAt on a sandbox and cursor
// on each list item (w32/m284). A response missing one made create, get and
// list raise even though the call succeeded.
func TestSandboxReadsCarryRenderRequiredFields(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	svc.Region = "frankfurt"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(`{"template":"node"}`)).WithContext(callerCtx()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created["region"] != "frankfurt" || created["createdAt"] == nil {
		t.Fatalf("create body lacks region/createdAt: %s", rec.Body)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sandboxes", nil).WithContext(callerCtx()))
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil || len(items) != 1 {
		t.Fatalf("list = %d %s", rec.Code, rec.Body)
	}
	sb, _ := items[0]["sandbox"].(map[string]any)
	if items[0]["cursor"] != created["id"] || sb["region"] != "frankfurt" {
		t.Fatalf("list item = %v, want cursor = id and the platform region", items[0])
	}
}

// A page is cut only when asked for; the cursor resumes after that sandbox.
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
	if got := ids("?limit=2"); strings.Join(got, ",") != "sbx-0,sbx-1" {
		t.Fatalf("first page = %v", got)
	}
	if got := ids("?limit=2&cursor=sbx-1"); strings.Join(got, ",") != "sbx-2" {
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
