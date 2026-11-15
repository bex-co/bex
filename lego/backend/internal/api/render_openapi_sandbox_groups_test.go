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
	"net/http"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestSandboxGroupListAnswersRendersErrorCodes pins w22/004: the REST group
// list refuses a missing or repeated ownerId, an out-of-range limit and a
// malformed cursor with the codes of Render's pinned errorCode vocabulary, and
// still passes every well-formed request through (including a well-formed
// cursor that names no group, which is an empty page, Render's end of list).
func TestSandboxGroupListAnswersRendersErrorCodes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/sandbox-groups", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(299) })
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	sbg := ids.New(ids.SandboxGroup)
	for _, c := range []struct {
		name, query string
		status      int
		code        string
	}{
		{"a bare workspace", "ownerId=tea-a", 299, ""},
		{"the largest page", "ownerId=tea-a&limit=100", 299, ""},
		{"a page of one", "ownerId=tea-a&limit=1", 299, ""},
		{"a well-formed cursor", "ownerId=tea-a&cursor=" + sbg, 299, ""},
		{"a workspace missing", "limit=5", http.StatusBadRequest, "invalid_owner_id"},
		{"an empty workspace", "ownerId=", http.StatusBadRequest, "invalid_owner_id"},
		{"a workspace repeated", "ownerId=tea-a&ownerId=tea-b", http.StatusBadRequest, "invalid_owner_id"},
		{"a limit of zero", "ownerId=tea-a&limit=0", http.StatusBadRequest, "invalid_limit"},
		{"a limit above 100", "ownerId=tea-a&limit=101", http.StatusBadRequest, "invalid_limit"},
		{"a limit that is not a number", "ownerId=tea-a&limit=many", http.StatusBadRequest, "invalid_limit"},
		{"a cursor that is not a group id", "ownerId=tea-a&cursor=nope", http.StatusBadRequest, "invalid_cursor"},
		{"a cursor of another kind", "ownerId=tea-a&cursor=" + ids.New(ids.Sandbox), http.StatusBadRequest, "invalid_cursor"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rr := requestOpenAPITest(t, h, http.MethodGet, "/v1/sandbox-groups?"+c.query, "", "")
			if rr.Code != c.status {
				t.Fatalf("GET /v1/sandbox-groups?%s = %d %s, want %d", c.query, rr.Code, rr.Body.String(), c.status)
			}
			if c.code == "" {
				return
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Code != c.code {
				t.Fatalf("GET /v1/sandbox-groups?%s answered %s, want code %s", c.query, rr.Body.String(), c.code)
			}
		})
	}
}

// TestSandboxBuildListAnswersRendersErrorCodes pins w22/016: the build list
// answers invalid_limit and invalid_cursor as the group list does, keeps its
// own invalid_status, and passes a well-formed cursor naming no build through
// (an empty page, Render's end of list).
func TestSandboxBuildListAnswersRendersErrorCodes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}/builds", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(299) })
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/sandbox-groups/" + ids.New(ids.SandboxGroup) + "/builds?"
	for _, c := range []struct {
		name, query string
		status      int
		code        string
	}{
		{"no query", "", 299, ""},
		{"the largest page", "limit=100", 299, ""},
		{"a well-formed cursor", "cursor=" + ids.New(ids.SandboxBuild), 299, ""},
		{"a limit of zero", "limit=0", http.StatusBadRequest, "invalid_limit"},
		{"a limit above 100", "limit=101", http.StatusBadRequest, "invalid_limit"},
		{"a limit that is not a number", "limit=many", http.StatusBadRequest, "invalid_limit"},
		{"a cursor that is not a build id", "cursor=nope", http.StatusBadRequest, "invalid_cursor"},
		{"a cursor of another kind", "cursor=" + ids.New(ids.SandboxGroup), http.StatusBadRequest, "invalid_cursor"},
		{"an unknown status", "status=nope", http.StatusBadRequest, "invalid_status"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rr := requestOpenAPITest(t, h, http.MethodGet, path+c.query, "", "")
			if rr.Code != c.status {
				t.Fatalf("GET %s%s = %d %s, want %d", path, c.query, rr.Code, rr.Body.String(), c.status)
			}
			if c.code == "" {
				return
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Code != c.code {
				t.Fatalf("GET %s%s answered %s, want code %s", path, c.query, rr.Body.String(), c.code)
			}
		})
	}
}
