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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCredentialRoutesEmitTheCLIContractGolden proves bex-api's real REST
// handlers for all 7 sandbox-group credential operations emit the exact shapes
// and status codes pinned in lego/cli/testdata/sandbox-credentials-contract.json
// (w2/m536 t002, ADR122).
func TestCredentialRoutesEmitTheCLIContractGolden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "sandbox-credentials-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		GroupID      string `json:"groupId"`
		CredentialID string `json:"credentialId"`
		Create       struct {
			Status      int            `json:"status"`
			RequestBody map[string]any `json:"requestBody"`
			Response    map[string]any `json:"response"`
		} `json:"create"`
		List struct {
			Status   int              `json:"status"`
			Response []map[string]any `json:"response"`
		} `json:"list"`
		Get struct {
			Status   int            `json:"status"`
			Response map[string]any `json:"response"`
		} `json:"get"`
		Update struct {
			Status      int            `json:"status"`
			RequestBody map[string]any `json:"requestBody"`
			Response    map[string]any `json:"response"`
		} `json:"update"`
		Versions struct {
			Status   int              `json:"status"`
			Response []map[string]any `json:"response"`
		} `json:"versions"`
		Restore struct {
			Status      int            `json:"status"`
			RequestBody map[string]any `json:"requestBody"`
			Response    map[string]any `json:"response"`
		} `json:"restore"`
		Delete struct {
			Status int `json:"status"`
		} `json:"delete"`
		Errors map[string]struct {
			Status int    `json:"status"`
			Code   string `json:"code"`
			ID     string `json:"id"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}

	const ws = "tea-cph1rs3idesc73a2b2mg"
	const actor = "usr-cph1rs3idesc73a2b2mg"
	st := newMemCredStore(ws, golden.GroupID)
	kv := newMemCredentialKV()
	svc := &Service{
		Base: &core.Base{
			Authz:     roleAuthzChecker{roles: map[string]string{actor: "admin"}},
			Workspace: credWorkspaceResolver{members: map[string][]string{actor: {ws}}},
		},
		Groups:          st,
		Credentials:     st,
		CredentialVault: NewCredentialVault(kv),
	}
	mux := http.NewServeMux()
	svc.registerCredentialREST(mux)

	do := func(method, path string, body any) (int, string) {
		t.Helper()
		var bodyStr string
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			bodyStr = string(b)
		}
		req := httptest.NewRequest(method, path, strings.NewReader(bodyStr))
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: actor}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	assertCredShape := func(label string, got, want map[string]any) {
		t.Helper()
		sameShape(t, label, got, want)
		sameShape(t, label+".version", got["version"].(map[string]any), want["version"].(map[string]any))
		sameShape(t, label+".injection", got["injection"].(map[string]any), want["injection"].(map[string]any))
		if _, hasValue := got["value"]; hasValue {
			t.Errorf("%s leaked secret value key", label)
		}
	}

	basePath := "/v1/sandbox-groups/" + golden.GroupID + "/credentials"
	credPath := basePath + "/" + golden.CredentialID

	// 1. Create -> 201
	code, body := do(http.MethodPost, basePath, golden.Create.RequestBody)
	if code != golden.Create.Status {
		t.Fatalf("create = %d %s, want %d", code, body, golden.Create.Status)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("create decode: %v (%s)", err, body)
	}
	assertCredShape("create", created, golden.Create.Response)
	if strings.Contains(body, golden.Create.RequestBody["value"].(string)) {
		t.Fatalf("create response leaked secret value: %s", body)
	}

	// 2. List -> 200 bare JSON array
	code, body = do(http.MethodGet, basePath, nil)
	if code != golden.List.Status {
		t.Fatalf("list = %d %s, want %d", code, body, golden.List.Status)
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(body), &listed); err != nil || len(listed) != len(golden.List.Response) {
		t.Fatalf("list = %s (%v), want %d item", body, err, len(golden.List.Response))
	}
	assertCredShape("list[0]", listed[0], golden.List.Response[0])

	// 3. Get -> 200
	code, body = do(http.MethodGet, credPath, nil)
	if code != golden.Get.Status {
		t.Fatalf("get = %d %s, want %d", code, body, golden.Get.Status)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	assertCredShape("get", got, golden.Get.Response)

	// 4. Update -> 200
	code, body = do(http.MethodPatch, credPath, golden.Update.RequestBody)
	if code != golden.Update.Status {
		t.Fatalf("update = %d %s, want %d", code, body, golden.Update.Status)
	}
	var updated map[string]any
	if err := json.Unmarshal([]byte(body), &updated); err != nil {
		t.Fatal(err)
	}
	assertCredShape("update", updated, golden.Update.Response)

	// 5. Versions -> 200 newest first
	code, body = do(http.MethodGet, credPath+"/versions", nil)
	if code != golden.Versions.Status {
		t.Fatalf("versions = %d %s, want %d", code, body, golden.Versions.Status)
	}
	var versions []map[string]any
	if err := json.Unmarshal([]byte(body), &versions); err != nil || len(versions) != len(golden.Versions.Response) {
		t.Fatalf("versions = %s (%v), want %d items", body, err, len(golden.Versions.Response))
	}
	for i := range versions {
		sameShape(t, "versions", versions[i], golden.Versions.Response[i])
	}

	// 6. Restore -> 200 with restoredFromVersionId
	code, body = do(http.MethodPost, credPath+"/restore", golden.Restore.RequestBody)
	if code != golden.Restore.Status {
		t.Fatalf("restore = %d %s, want %d", code, body, golden.Restore.Status)
	}
	var restored map[string]any
	if err := json.Unmarshal([]byte(body), &restored); err != nil {
		t.Fatal(err)
	}
	assertCredShape("restore", restored, golden.Restore.Response)

	// 7. Error contracts (before delete)
	for name, call := range map[string]func() (int, string){
		"alreadyExists": func() (int, string) {
			return do(http.MethodPost, basePath, golden.Create.RequestBody)
		},
		"notFound": func() (int, string) {
			return do(http.MethodGet, basePath+"/missing-cred", nil)
		},
		"versionNotFound": func() (int, string) {
			return do(http.MethodPost, credPath+"/restore", map[string]any{"versionId": 999})
		},
		"invalid": func() (int, string) {
			return do(http.MethodPost, basePath, map[string]any{
				"id": "rnd-reserved", "domain": "api.example.com", "value": "v",
				"injection": map[string]any{"type": "header", "name": "Authorization"},
			})
		},
	} {
		want := golden.Errors[name]
		errCode, errBody := call()
		var gotErr map[string]any
		_ = json.Unmarshal([]byte(errBody), &gotErr)
		if errCode != want.Status || gotErr["code"] != want.Code || gotErr["id"] != want.ID {
			t.Errorf("error %s = %d %s, want %d %s (%s)", name, errCode, errBody, want.Status, want.Code, want.ID)
		}
	}

	// 8. Delete -> 202 empty body
	code, body = do(http.MethodDelete, credPath, nil)
	if code != golden.Delete.Status || body != "" {
		t.Fatalf("delete = %d %q, want %d empty body", code, body, golden.Delete.Status)
	}
}
