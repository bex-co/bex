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
	"strings"
	"testing"
)

// A new Key Value instance always runs Valkey 8, as a new Render Key Value
// does, and no create surface offers a version (w4/743). Render's
// keyValuePOSTInput has no version property, so through the composed stack (the
// pinned Render validator, then the handler's strict decoder) a REST body naming
// one is an unknown field on both Render paths, and GraphQL has no argument for
// it. A create that names none reads back Valkey 8.
func TestKeyValueCreateOffersNoVersionAndRunsValkey8(t *testing.T) {
	const own = "tea-00000000000000000000"
	h := renderBodyGateServer(t, own, "bsr-00000000000000000000")

	for _, path := range []string{"/v1/key-value", "/v1/redis"} {
		w := do(t, h, http.MethodPost, path, testToken, `{"name":"kv7","ownerId":"`+own+`","plan":"free","version":"7"}`)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `unknown field \"version\"`) {
			t.Fatalf("POST %s with version 7 = %d %s, want 400 naming the unknown field version", path, w.Code, w.Body.String())
		}
	}

	w := do(t, h, http.MethodPost, "/v1/key-value", testToken, `{"name":"kv8","ownerId":"`+own+`","plan":"free"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /v1/key-value = %d %s, want 201", w.Code, w.Body.String())
	}
	var created struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Version != "8" {
		t.Fatalf("a new store reads version %q, want 8", created.Version)
	}

	body, _ := json.Marshal(map[string]string{"query": `mutation { createKeyValue(name:"gql7", ownerId:"` + own + `", version:"7") { id version } }`})
	w = do(t, h, http.MethodPost, "/graphql", testToken, string(body))
	if !strings.Contains(w.Body.String(), `Unknown argument \"version\"`) {
		t.Fatalf("createKeyValue(version:\"7\") = %d %s, want GraphQL's unknown-argument refusal", w.Code, w.Body.String())
	}

	data := gql(t, h, `mutation { createKeyValue(name:"gql8", ownerId:"`+own+`") { version } }`)
	if got := data["createKeyValue"].(map[string]any)["version"]; got != "8" {
		t.Fatalf("a store created on GraphQL reads version %v, want 8", got)
	}
}
