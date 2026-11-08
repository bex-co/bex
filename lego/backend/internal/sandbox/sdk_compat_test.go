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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSandboxNamesAreRefusedByName(t *testing.T) {
	svc := policyAdapterService(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(`{"name":"my-agent"}`)),
		httptest.NewRequest(http.MethodGet, "/v1/sandboxes?name=my-agent", nil),
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req.WithContext(callerCtx()))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "SANDBOX_NAMES_UNSUPPORTED") || !strings.Contains(rec.Body.String(), "--name") {
			t.Errorf("%s %s = %d %s, want the named refusal", req.Method, req.URL, rec.Code, rec.Body)
		}
	}
}
