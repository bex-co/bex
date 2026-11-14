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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// w4/m363: a service's or group's secret files hold Render's 1 MB of contents,
// and a 1 MiB file of `"` characters JSON-escapes to 2 MiB — past the default
// 2 MiB body cap. The routes that write secret files take the environment cap,
// so such a Render-valid write reaches its handler; every other route keeps
// the global cap.
func TestSecretFileWritesTakeTheEnvironmentBodyCap(t *testing.T) {
	const global = 2 << 20 // BEX_MAX_BODY_BYTES' default
	quoted, err := json.Marshal(map[string]string{"content": strings.Repeat(`"`, 1<<20)})
	if err != nil {
		t.Fatal(err)
	}
	if len(quoted) <= global {
		t.Fatalf("the escaped body is %d bytes; the case needs one past the global cap", len(quoted))
	}
	// The worst whole batch: both maps full, every byte escaped to two.
	worst := 2*(1<<20+512<<10) + 64<<10
	if worst > secrets.MaxEnvironmentBodyBytes {
		t.Fatalf("MaxEnvironmentBodyBytes = %d, under a doubled full environment (%d)", secrets.MaxEnvironmentBodyBytes, worst)
	}

	var reached int
	handler := restBodyLimit(global)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reached = len(body)
		w.WriteHeader(http.StatusOK)
	}))
	serve := func(method, path string, body []byte) int {
		reached = 0
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(string(body))))
		return rec.Code
	}

	for _, route := range []struct{ method, path string }{
		{http.MethodPut, "/v1/services/srv-a/secret-files/cert.pem"},
		{http.MethodPut, "/v1/services/srv-a/secret-files"},
		{http.MethodPut, "/v1/build-sources/bds-a/secret-files/cert.pem"},
		{http.MethodPatch, "/v1/services/srv-a/environment"},
		{http.MethodPut, "/v1/env-groups/evg-a/secret-files/cert.pem"},
		{http.MethodPatch, "/v1/env-groups/evg-a/environment"},
		{http.MethodPatch, "/v1/env-groups/evg-a/contents"},
	} {
		if code := serve(route.method, route.path, quoted); code != http.StatusOK || reached != len(quoted) {
			t.Errorf("%s %s with a 1 MiB file of quotes = %d (read %d), want it to reach the handler", route.method, route.path, code, reached)
		}
		if code := serve(route.method, route.path, make([]byte, secrets.MaxEnvironmentBodyBytes+1)); code != http.StatusRequestEntityTooLarge {
			t.Errorf("%s %s past the environment cap = %d, want 413", route.method, route.path, code)
		}
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/services/srv-a/deploys"},
		{http.MethodPut, "/v1/services/srv-a/env-vars"},
		{http.MethodPatch, "/v1/services/srv-a"},
		{http.MethodPatch, "/v1/agent-environment"},
	} {
		if code := serve(route.method, route.path, quoted); code != http.StatusRequestEntityTooLarge {
			t.Errorf("%s %s past the global cap = %d, want 413", route.method, route.path, code)
		}
	}

	if got := environmentBodyLimit(0); got != 0 {
		t.Errorf("a disabled global cap = %d, want it to stay disabled", got)
	}
	if got := environmentBodyLimit(8 << 20); got != 8<<20 {
		t.Errorf("a global cap above the environment's = %d, want the global", got)
	}
}
