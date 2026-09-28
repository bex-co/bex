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
	"reflect"
	"sort"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestServiceListFiltersThroughComposedServer(t *testing.T) {
	node, docker, suspended, static := sampleApp("node"), sampleApp("docker"), sampleApp("suspended"), sampleApp("static")
	node.Spec.Runtime = "node"
	docker.Spec.Runtime = "docker"
	suspended.Spec.Runtime = "docker"
	suspended.Spec.Suspended = true
	static.Spec.Type = appv1alpha1.TypeStaticSite
	h, _ := serverWith(t, &core.Base{Client: fakeClient(node, docker, suspended, static), Namespace: "default"}, Deps{APIKeys: newFakeKeyStore(), Region: "fsn1"})
	for _, tc := range []struct {
		query  string
		want   []string
		status int
	}{
		{"suspended=suspended", []string{"suspended"}, 200},
		{"suspended=not_suspended", []string{"docker", "node", "static"}, 200},
		{"suspended=suspended&suspended=not_suspended", []string{"docker", "node", "static", "suspended"}, 200},
		{"suspended=true", nil, 400},
		{"suspended=false", nil, 400},
		{"env=node", []string{"node"}, 200},
		{"env=node&env=docker", []string{"docker", "node", "suspended"}, 200},
		{"env=docker&suspended=not_suspended", []string{"docker"}, 200},
		{"region=oregon", []string{}, 200},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := do(t, h, http.MethodGet, "/v1/services?"+tc.query, testToken, "")
			if w.Code != tc.status {
				t.Fatalf("status=%d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status != 200 {
				return
			}
			var page []struct {
				Service struct {
					Name string `json:"name"`
				} `json:"service"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(page))
			for _, row := range page {
				got = append(got, row.Service.Name)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("names=%v want %v", got, tc.want)
			}
		})
	}
}
