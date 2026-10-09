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
	"testing"
)

// The pinned CLI serializes an unused buildFilter list as null (w8/m54). The
// one-sided filters mirror its captured requests; the rest are controls that
// must keep their existing outcome.
func TestRenderValidatorAcceptsOneSidedCLIBuildFilter(t *testing.T) {
	createBody := func(filter string) string {
		return `{"name":"web","ownerId":"tea-x","type":"web_service","repo":"https://github.com/bex-co/bex","buildFilter":` + filter + `}`
	}
	patchBody := func(filter string) string { return `{"buildFilter":` + filter + `}` }
	tests := []struct {
		name   string
		filter string
		want   int
	}{
		{"include only", `{"paths":["examples/hello-go/**"],"ignoredPaths":null}`, http.StatusNoContent},
		{"ignore only", `{"paths":null,"ignoredPaths":["*.md"]}`, http.StatusNoContent},
		{"both lists", `{"paths":["a/**"],"ignoredPaths":["*.md"]}`, http.StatusNoContent},
		{"empty arrays", `{"paths":[],"ignoredPaths":[]}`, http.StatusNoContent},
		{"missing list stays required", `{"paths":["a/**"]}`, http.StatusBadRequest},
		{"non-array list", `{"paths":"a/**","ignoredPaths":null}`, http.StatusBadRequest},
		{"non-string item", `{"paths":[7],"ignoredPaths":null}`, http.StatusBadRequest},
		{"null filter object", `null`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		for _, op := range []struct{ method, target, body string }{
			{http.MethodPost, "/v1/services", createBody(tt.filter)},
			{http.MethodPatch, "/v1/services/srv-db49ak3jrdls73co04p0", patchBody(tt.filter)},
		} {
			t.Run(op.method+" "+tt.name, func(t *testing.T) {
				mutations := 0
				mux := http.NewServeMux()
				count := func(w http.ResponseWriter, _ *http.Request) {
					mutations++
					w.WriteHeader(http.StatusNoContent)
				}
				mux.HandleFunc("POST /v1/services", count)
				mux.HandleFunc("PATCH /v1/services/{serviceId}", count)
				h, err := newRenderRequestValidator(mux)
				if err != nil {
					t.Fatal(err)
				}
				w := requestOpenAPITest(t, h, op.method, op.target, "application/json", op.body)
				if w.Code != tt.want {
					t.Fatalf("status = %d, want %d: %s", w.Code, tt.want, w.Body.String())
				}
				wantCalls := 0
				if tt.want == http.StatusNoContent {
					wantCalls = 1
				}
				if mutations != wantCalls {
					t.Errorf("handler calls = %d, want %d", mutations, wantCalls)
				}
			})
		}
	}
}

// The overlay is request-only: the shared component (and so every response
// schema that references it) still declares non-null arrays.
func TestBuildFilterNullabilityLeavesSharedComponentStrict(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	component := contract.document.Components.Schemas["buildFilter"]
	for _, name := range []string{"paths", "ignoredPaths"} {
		if component.Value.Properties[name].Value.Nullable {
			t.Errorf("shared buildFilter.%s became nullable", name)
		}
	}
	for _, operationID := range []string{"create-service", "update-service"} {
		_, operation := findRenderOperation(t, contract.document, operationID)
		filter := operation.RequestBody.Value.Content.Get("application/json").Schema.Value.Properties["buildFilter"].Value
		for _, name := range []string{"paths", "ignoredPaths"} {
			if !filter.Properties[name].Value.Nullable {
				t.Errorf("%s buildFilter.%s is not nullable", operationID, name)
			}
		}
	}
}
