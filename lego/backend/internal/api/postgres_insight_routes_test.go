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
	"net/http"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestPostgresInsightRenderRoutes(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli"}}, Deps{APIKeys: newFakeKeyStore()})
	for _, insight := range []string{"processes", "top-queries", "sizes", "table-scans"} {
		var body string
		for _, prefix := range []string{"", "query/"} {
			w := do(t, h, http.MethodGet, "/v1/postgres/dpg-missing/"+prefix+insight, testToken, "")
			if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"message":"not found"`) {
				t.Fatalf("%s%s: %d %s", prefix, insight, w.Code, w.Body.String())
			}
			if body != "" && body != w.Body.String() {
				t.Fatalf("alias mismatch: %s vs %s", body, w.Body.String())
			}
			body = w.Body.String()
		}
	}
}
