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

package clitelemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func testMux() (*Service, *fakeStore, *http.ServeMux) {
	s, st := newTestService()
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	return s, st, mux
}

// upstreamSampleBody is the exact JSON the pinned CLI's sender emits for a
// trivial invocation (field names from upstream
// pkg/client/clitelemetry/clitelemetry_gen.go).
const upstreamSampleBody = `{
	"agent_signals": ["CLAUDECODE"],
	"arch": "arm64",
	"ci_signals": [],
	"cli_version": "2.27.0",
	"command": "workspaces",
	"completion_kind": "success",
	"current_workspace_id": "tea-abc",
	"duration_ms": 42,
	"exit_code": 0,
	"installation_id": "install-1",
	"launched_full_screen_tui": false,
	"os": "darwin",
	"output_format": "json",
	"started_at": "2026-09-10T04:00:00Z",
	"is_stdin_tty": false,
	"is_stdout_tty": false,
	"is_stderr_tty": true,
	"is_term_dumb": false
}`

func TestRESTIngestUpstreamShapeReturns202(t *testing.T) {
	_, st, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/cli-telemetry-events", strings.NewReader(upstreamSampleBody)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("ingest => 202, got %d: %s", rec.Code, rec.Body)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(body["id"], "cte-") {
		t.Errorf("response id = %q, want cte- id", body["id"])
	}
	if len(st.rows) != 1 || st.rows[0].Command != "workspaces" {
		t.Fatalf("stored rows = %+v", st.rows)
	}
}

// TestRESTIgnoresUnknownFields pins the forward-compat the route relies on:
// Render's embedded public spec does not list this path, so the OpenAPI gate
// passes it through without strict decoding. If a future spec addition ever
// turns strict decoding on for this route, this test fails loudly instead of
// 400ing newer CLIs in production.
func TestRESTIgnoresUnknownFields(t *testing.T) {
	_, st, mux := testMux()
	body := `{"command": "services list", "future_field_from_newer_cli": {"nested": [1,2,3]}, "another_one": 42}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/cli-telemetry-events", strings.NewReader(body)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("unknown fields => 202, got %d: %s", rec.Code, rec.Body)
	}
	if len(st.rows) != 1 || st.rows[0].Command != "services list" {
		t.Fatalf("stored rows = %+v", st.rows)
	}
}

func TestRESTMalformedBodyIs400(t *testing.T) {
	_, _, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/cli-telemetry-events", strings.NewReader(`{"command":`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed => 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestRESTEmptyCommandIs400(t *testing.T) {
	_, st, mux := testMux()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/cli-telemetry-events", strings.NewReader(`{"command": "", "cli_version": "2.27.0"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty command => 400, got %d: %s", rec.Code, rec.Body)
	}
	if len(st.rows) != 0 {
		t.Errorf("rows = %d, want 0", len(st.rows))
	}
}

func TestRESTStoreOffIs503(t *testing.T) {
	s := &Service{Base: &core.Base{}}
	mux := http.NewServeMux()
	s.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/cli-telemetry-events", strings.NewReader(upstreamSampleBody)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("store off => 503, got %d: %s", rec.Code, rec.Body)
	}
}
