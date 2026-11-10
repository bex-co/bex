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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestSandboxCallsPassTheRenderGate pins what the round-5 pin refresh (w2/m251)
// nearly broke: Render's sandbox operations entered the pinned contract after
// bex already served them, and their sbx- id pattern (25 characters) and
// required list ownerId would have 400'd every call bex's own 20-character ids
// and default-workspace resolution make. Each request is the pinned CLI's
// shape; the stub handler answers 299 only if the gate let it through.
func TestSandboxCallsPassTheRenderGate(t *testing.T) {
	mux := http.NewServeMux()
	for _, pattern := range []string{
		"POST /v1/sandboxes",
		"GET /v1/sandboxes",
		"GET /v1/sandboxes/{id}",
		"POST /v1/sandboxes/{id}/terminate",
		"POST /v1/sandboxes/{id}/runs/{operation}/token",
		"POST /v1/sandboxes/{id}/files/{operation}/token",
		"GET /v1/sandboxes/{id}/execs",
		"GET /v1/sandboxes/{id}/execs/{execId}",
	} {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(299) })
	}
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	sbx := ids.New(ids.Sandbox)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/v1/sandboxes", `{"ownerId":"tea-a","plan":"starter"}`},
		{http.MethodGet, "/v1/sandboxes", ""},
		{http.MethodGet, "/v1/sandboxes?ownerId=tea-a", ""},
		{http.MethodGet, "/v1/sandboxes/" + sbx + "?ownerId=tea-a", ""},
		{http.MethodPost, "/v1/sandboxes/" + sbx + "/terminate", ""},
		{http.MethodPost, "/v1/sandboxes/" + sbx + "/runs/stream/token", `{"command":"true"}`},
		{http.MethodPost, "/v1/sandboxes/" + sbx + "/files/upload/token?path=/workspace/a", ""},
		// Render's execution history (w32/m298), with and without bex's
		// optional ownerId.
		{http.MethodGet, "/v1/sandboxes/" + sbx + "/execs?limit=5", ""},
		{http.MethodGet, "/v1/sandboxes/" + sbx + "/execs?ownerId=tea-a&cursor=" + ids.New(ids.SandboxExecution), ""},
		{http.MethodGet, "/v1/sandboxes/" + sbx + "/execs/" + ids.New(ids.SandboxExecution), ""},
		{http.MethodGet, "/v1/sandboxes/" + sbx + "/execs/" + ids.New(ids.SandboxExecution) + "?ownerId=tea-a", ""},
	} {
		contentType := ""
		if c.body != "" {
			contentType = "application/json"
		}
		rr := requestOpenAPITest(t, h, c.method, c.path, contentType, c.body)
		if rr.Code != 299 {
			t.Errorf("%s %s: gate answered %d %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}

	// The widening keeps Render's own 25-character ids and still refuses a
	// malformed one.
	for id, want := range map[string]int{
		"sbx-" + strings.Repeat("a", 25): 299,
		"sbx-" + strings.Repeat("a", 22): http.StatusBadRequest,
		"exs-" + strings.Repeat("a", 20): http.StatusBadRequest,
	} {
		rr := requestOpenAPITest(t, h, http.MethodGet, "/v1/sandboxes/"+id, "", "")
		if rr.Code != want {
			t.Errorf("GET /v1/sandboxes/%s = %d, want %d", id, rr.Code, want)
		}
	}
}
