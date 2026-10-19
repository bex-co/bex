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

package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A Render list-logs filter bex cannot honor is refused by name on every logs
// route, never ignored: ignoring task, taskRun and sandbox widened a filtered
// read to the service's whole log (w8/132).
func TestUnsupportedLogFiltersAreRefusedByName(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z first boot ok"}}, sampleApp("web"), podFor("web", "web-1"))
	if rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web"); rec.Code != http.StatusOK {
		t.Fatalf("control: unfiltered logs = %d %s", rec.Code, rec.Body.String())
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, name := range []string{"task", "taskRun", "sandbox"} {
		for _, route := range []string{"/v1/logs?", "/v1/logs/values?label=instance&", "/v1/logs/subscribe?"} {
			// A subscribe that is not refused streams until its deadline.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route+"resource=web&"+name+"=bogus-w8-132", nil).WithContext(ctx))
			cancel()
			var body struct{ Message string }
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(body.Message, strconv.Quote(name)) {
				t.Errorf("%s%s = %d %s, want a 400 naming the filter", route, name, rec.Code, rec.Body.String())
			}
		}
	}
}

// Every query name the pinned Render OpenAPI declares for the logs routes is
// either read into the query or refused: none can pass the strict-query gate
// and then vanish between the gate and the parser (w8/132).
func TestEveryDeclaredLogFilterIsParsedOrRefused(t *testing.T) {
	raw, err := os.ReadFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	// Read by the handlers themselves, not the shared parser.
	handlerLevel := map[string]bool{"ownerId": true, "label": true}
	baseReq := httptest.NewRequest(http.MethodGet, "/v1/logs?resource=web", nil)
	baseResources, baseQuery, err := parseLogParams(baseReq)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/logs", "/logs/values", "/logs/subscribe"} {
		names := declaredQueryNames(t, spec, path)
		if len(names) == 0 {
			t.Fatalf("%s declares no query parameters; the spec path moved", path)
		}
		for _, name := range names {
			if handlerLevel[name] {
				continue
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/logs?resource=web&"+name+"=bex-guard-value", nil)
			resources, query, err := parseLogParams(req)
			if err == nil && slices.Equal(resources, baseResources) && reflect.DeepEqual(query, baseQuery) {
				t.Errorf("%s declares %q, but the parser neither reads nor refuses it", path, name)
			}
		}
	}
}

// declaredQueryNames resolves a GET operation's query parameter names,
// following the spec's local $refs.
func declaredQueryNames(t *testing.T, spec any, path string) []string {
	t.Helper()
	resolve := func(node any) map[string]any {
		for {
			m, _ := node.(map[string]any)
			ref, ok := m["$ref"].(string)
			if !ok {
				return m
			}
			node = spec
			for _, key := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				key = strings.NewReplacer("~1", "/", "~0", "~").Replace(key)
				switch n := node.(type) {
				case map[string]any:
					node = n[key]
				case []any:
					i, err := strconv.Atoi(key)
					if err != nil || i >= len(n) {
						t.Fatalf("bad $ref %s", ref)
					}
					node = n[i]
				}
			}
		}
	}
	paths := resolve(spec)["paths"].(map[string]any)
	op := resolve(resolve(paths[path])["get"])
	var names []string
	params, _ := op["parameters"].([]any)
	for _, p := range params {
		if param := resolve(p); param["in"] == "query" {
			names = append(names, param["name"].(string))
		}
	}
	return names
}
