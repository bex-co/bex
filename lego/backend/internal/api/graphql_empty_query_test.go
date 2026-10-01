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
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/graphql-go/graphql"
)

func TestGraphQLEmptyQueriesAreClientErrors(t *testing.T) {
	srv := matrixServer(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)
	for _, body := range []string{`{}`, `{"query":""}`, `{"query":" \t\r\n "}`, `{"query":null}`, `null`} {
		t.Run(body, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/graphql", testToken, body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				t.Errorf("Content-Type=%q", rec.Header().Get("Content-Type"))
			}
			var result graphql.Result
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			assertEmptyGraphQLQueryError(t, result)
		})
	}
	if strings.Contains(logs.String(), "bex-api graphql: internal error") {
		t.Errorf("invalid client queries logged as server failures: %s", logs.String())
	}
}

func assertEmptyGraphQLQueryError(t *testing.T, result graphql.Result) {
	t.Helper()
	if len(result.Errors) != 1 || result.Errors[0].Message != "query is required" {
		t.Errorf("errors=%v, want one actionable missing-query error", result.Errors)
	}
	if result.Data != nil {
		t.Errorf("invalid operation returned data: %#v", result.Data)
	}
}

func TestGraphQLEmptyQueriesPreserveBatchResults(t *testing.T) {
	srv := matrixServer(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	body := `[{"query":"{ services { id } }"},{},{"query":" \t\n "},{"query":null},null,{"query":"{ services { id } }"}]`
	rec := do(t, h, http.MethodPost, "/graphql", testToken, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var results []graphql.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 6 {
		t.Fatalf("batch result count=%d, want6", len(results))
	}
	for _, i := range []int{1, 2, 3, 4} {
		assertEmptyGraphQLQueryError(t, results[i])
	}
	for _, i := range []int{0, 5} {
		if len(results[i].Errors) != 0 {
			t.Errorf("valid operation%d errors=%v", i, results[i].Errors)
		}
		data, err := json.Marshal(results[i].Data)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != `{"services":[{"id":"web"}]}` {
			t.Errorf("valid operation%d data=%s", i, data)
		}
	}
}

func TestGraphQLEmptyQueriesHaveInvalidTelemetry(t *testing.T) {
	srv := matrixServer(t)
	if _, err := srv.Handler(); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{}`, `{"query":""}`, `{"query":" \t\n "}`, `{"query":null}`, `null`, `[{}, {"query":" "},null]`, `[{"query":"{ services { id } }"},{"query":" "}]`} {
		t.Run(body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
			req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "client-1", Method: "session"}))
			rec := httptest.NewRecorder()
			operation, opType, outcome := srv.serveGraphQL(rec, req)
			if outcome != gqlOutcomeInvalid {
				t.Errorf("outcome=%q, want%q", outcome, gqlOutcomeInvalid)
			}
			if strings.Contains(body, "services") {
				var results []graphql.Result
				if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
					t.Fatal(err)
				}
				if len(results) != 2 || len(results[0].Errors) != 0 || results[0].Data == nil {
					t.Fatalf("valid leading batch operation failed: %s", rec.Body.String())
				}
				assertEmptyGraphQLQueryError(t, results[1])
				if operation != "services" || opType != gqlTypeQuery {
					t.Errorf("mixed batch dimensions: %q %q", operation, opType)
				}
			} else if operation != gqlOperationOther || opType != gqlTypeQuery {
				t.Errorf("unbounded telemetry dimensions: operation=%q type=%q", operation, opType)
			}
		})
	}
}
