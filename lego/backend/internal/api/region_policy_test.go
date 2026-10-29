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
	"fmt"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestCreateRegionPolicyThroughComposedServer drives w8/152's one region
// policy through the composed server, pinned Render OpenAPI validator
// included: on services, Postgres and Key Value a Render region creates the
// resource and the read back reports BEX_REGION, while a value outside
// Render's enum is a 400 before any write.
func TestCreateRegionPolicyThroughComposedServer(t *testing.T) {
	routes := []struct {
		name, path, body, idPath string
	}{
		{"services", "/v1/services", `{"name":"web-%s","ownerId":"tea-cli","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":""},"serviceDetails":{"plan":"starter","region":%q,"runtime":"image"}}`, "service"},
		{"postgres", "/v1/postgres", `{"name":"db-%s","ownerId":"tea-cli","plan":"free","region":%q}`, ""},
		{"key-value", "/v1/key-value", `{"name":"kv-%s","ownerId":"tea-cli","plan":"free","region":%q}`, ""},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			for _, tc := range []struct {
				region string
				want   int
			}{
				{"oregon", http.StatusCreated},
				{"virginia", http.StatusCreated},
				{"Vietnam", http.StatusBadRequest},
			} {
				// A fresh workspace per case: one free datastore each.
				base := &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli"}}
				h, _ := serverWith(t, base, Deps{APIKeys: newFakeKeyStore(), Region: "fsn1"})
				w := do(t, h, http.MethodPost, route.path, testToken, fmt.Sprintf(route.body, tc.region, tc.region))
				if w.Code != tc.want {
					t.Errorf("region %q: POST %s = %d, want %d: %s", tc.region, route.path, w.Code, tc.want, w.Body.String())
					continue
				}
				if tc.want != http.StatusCreated {
					continue
				}
				var created map[string]json.RawMessage
				if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
					t.Fatalf("decode create: %v", err)
				}
				if route.idPath != "" {
					if err := json.Unmarshal(created[route.idPath], &created); err != nil {
						t.Fatalf("decode %s envelope: %v", route.idPath, err)
					}
				}
				var id string
				_ = json.Unmarshal(created["id"], &id)
				r := do(t, h, http.MethodGet, route.path+"/"+id, testToken, "")
				var got struct {
					Region         string `json:"region"`
					ServiceDetails struct {
						Region string `json:"region"`
					} `json:"serviceDetails"`
				}
				if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil || r.Code != http.StatusOK {
					t.Fatalf("GET %s/%s = %d: %s", route.path, id, r.Code, r.Body.String())
				}
				if region := got.Region + got.ServiceDetails.Region; region != "fsn1" {
					t.Errorf("region %q: readback region = %q, want BEX_REGION fsn1", tc.region, region)
				}
			}
		})
	}
}
