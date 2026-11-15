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

	"github.com/getkin/kin-openapi/openapi3"

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

// TestSandboxGroupCreateTypesTheBexEnvironmentIdExtension pins w22/m412: the
// pinned create-sandbox-group body has no environmentId (Render's REST create
// takes none; its response carries a nullable one), and bex accepts an optional
// one as an extension so a group can be placed in a project environment. The
// gate types it (a string, or null as a group read back carries) and refuses
// anything else before the handler runs, and an official client's body, which
// never names it, passes as before.
func TestSandboxGroupCreateTypesTheBexEnvironmentIdExtension(t *testing.T) {
	for _, c := range []struct {
		name, body string
		status     int
	}{
		{"an official client's body", `{"ownerId":"tea-a","name":"CI agents","region":"oregon"}`, 299},
		{"an environment id", `{"ownerId":"tea-a","name":"g","environmentId":"evm-cph1rs3idesc73a2b2mg"}`, 299},
		{"a legacy environment id", `{"ownerId":"tea-a","name":"g","environmentId":"env-cph1rs3idesc73a2b2mg"}`, 299},
		{"a null environment id", `{"ownerId":"tea-a","name":"g","environmentId":null}`, 299},
		{"a number", `{"ownerId":"tea-a","name":"g","environmentId":5}`, http.StatusBadRequest},
		{"an object", `{"ownerId":"tea-a","name":"g","environmentId":{"id":"evm-x"}}`, http.StatusBadRequest},
		{"an environment id without the required fields", `{"environmentId":"evm-cph1rs3idesc73a2b2mg"}`, http.StatusBadRequest},
	} {
		t.Run(c.name, func(t *testing.T) {
			reached := false
			mux := http.NewServeMux()
			mux.HandleFunc("POST /v1/sandbox-groups", func(w http.ResponseWriter, _ *http.Request) {
				reached = true
				w.WriteHeader(299)
			})
			h, err := newRenderRequestValidator(mux)
			if err != nil {
				t.Fatal(err)
			}
			rr := requestOpenAPITest(t, h, http.MethodPost, "/v1/sandbox-groups", "application/json", c.body)
			if rr.Code != c.status || reached != (c.status == 299) {
				t.Fatalf("POST /v1/sandbox-groups %s = %d %s (handler reached: %v), want %d", c.body, rr.Code, rr.Body.String(), reached, c.status)
			}
		})
	}
}

// The extension is on the create body alone: the response shape every group
// read shares keeps Render's nullable string, and another operation's body does
// not gain the field.
func TestSandboxGroupEnvironmentIdExtensionIsOnTheCreateBodyOnly(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	body := func(operationID string) *openapi3.Schema {
		_, op := findRenderOperation(t, contract.document, operationID)
		return op.RequestBody.Value.Content.Get("application/json").Schema.Value
	}
	create := body("create-sandbox-group").Properties["environmentId"]
	if create == nil || create.Value.Type == nil || !create.Value.Type.Is("string") || !create.Value.Nullable {
		t.Fatalf("create-sandbox-group environmentId = %+v, want a nullable string", create)
	}
	if _, ok := body("update-sandbox-group").Properties["environmentId"]; ok {
		t.Error("update-sandbox-group gained an environmentId: a placement is chosen at create only")
	}
	_, get := findRenderOperation(t, contract.document, "retrieve-sandbox-group")
	read := get.Responses.Status(http.StatusOK).Value.Content.Get("application/json").Schema.Value.Properties["environmentId"].Value
	if read.Type == nil || !read.Type.Is("string") || !read.Nullable {
		t.Errorf("the group response's environmentId = %+v, want Render's nullable string", read)
	}
}
