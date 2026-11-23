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
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// w3/m475: the pinned Render CLI's log viewer pages with limit=1000, so
// list-logs admits 1 to 1000. Every other operation, the two other log routes
// included, keeps the pinned 1 to 100.
func TestRenderListLogsAdmitsTheCLIPageLimit(t *testing.T) {
	mux := http.NewServeMux()
	for _, pattern := range []string{"GET /v1/logs", "GET /v1/logs/values", "GET /v1/logs/subscribe", "GET /v1/services"} {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path    string
		message string // "" means admitted
	}{
		{"/v1/logs?resource=srv-x&limit=1", ""},
		{"/v1/logs?resource=srv-x&limit=101", ""},
		{"/v1/logs?resource=srv-x&limit=1000", ""},
		{"/v1/logs?resource=srv-x&limit=1001", "limit must be an integer between 1 and 1000"},
		{"/v1/logs?resource=srv-x&limit=0", "limit must be an integer between 1 and 1000"},
		{"/v1/logs?resource=srv-x&limit=-5", "limit must be an integer between 1 and 1000"},
		// A value that is no integer at all fails the parse, before any bound,
		// with the gate's generic refusal, as on every other operation.
		{"/v1/logs?resource=srv-x&limit=abc", `invalid query parameter \"limit\"`},
		{"/v1/logs?resource=srv-x&limit=2.5", `invalid query parameter \"limit\"`},
		{"/v1/logs/subscribe?resource=srv-x&limit=1000", "limit must be an integer between 1 and 100"},
		// Render's project/environment/region scope is admitted on all three
		// log endpoints (w4/m596), and w3/m505's filterDocument stays admitted on tail.
		{"/v1/logs/subscribe?project=prj-x&type=app&filterDocument=%7B%22filters%22%3A%5B%5D%2C%22version%22%3A1%7D", ""},
		{"/v1/logs?project=prj-x&resource=srv-x", ""},
		{"/v1/logs?project=prj-x&environment=evm-x&region=oregon", ""},
		{"/v1/logs/values?project=prj-x&environment=evm-x&region=oregon&label=level", ""},
		{"/v1/logs/values?resource=srv-x&label=level&limit=1000", "limit must be an integer between 1 and 100"},
		{"/v1/services?limit=1000", "limit must be an integer between 1 and 100"},
	} {
		w := requestOpenAPITest(t, h, http.MethodGet, tc.path, "", "")
		if tc.message == "" {
			if w.Code != http.StatusNoContent {
				t.Errorf("%s = %d %s, want admitted", tc.path, w.Code, w.Body)
			}
			continue
		}
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"message":"`+tc.message) {
			t.Errorf("%s = %d %s, want 400 %q", tc.path, w.Code, w.Body, tc.message)
		}
	}
}

// Like the path-pattern overrides (TestRenderPathParameterOverridesOnlyWiden),
// a raised maximum may only widen Render's contract, and only on the operation
// that names it: the shared limitParam component keeps Render's bound for
// everyone else.
func TestRenderQueryMaximumsOnlyWiden(t *testing.T) {
	pristine, err := openapi3.NewLoader().LoadFromData(renderOpenAPISource)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	for operationID, maxima := range renderQueryMaximumCompatibility {
		item, operation := findRenderOperation(t, pristine, operationID)
		widenedItem, widenedOperation := findRenderOperation(t, contract.document, operationID)
		for name, maximum := range maxima {
			original := queryParameterSchema(item, operation, name)
			if original == nil || original.Max == nil {
				t.Errorf("%s: query parameter %q declares no maximum upstream; the override is dead weight", operationID, name)
				continue
			}
			if maximum <= *original.Max {
				t.Errorf("%s: %q maximum %g does not raise Render's %g; this layer may only widen", operationID, name, maximum, *original.Max)
			}
			widened := queryParameterSchema(widenedItem, widenedOperation, name)
			if widened == nil || widened.Max == nil || *widened.Max != maximum {
				t.Errorf("%s: the loaded contract's %q maximum is not %g", operationID, name, maximum)
				continue
			}
			if (widened.Min == nil) != (original.Min == nil) || (widened.Min != nil && *widened.Min != *original.Min) {
				t.Errorf("%s: raising %q's maximum moved its minimum", operationID, name)
			}
		}
	}

	// Every query parameter no entry names keeps its pinned maximum.
	for path, widenedItem := range contract.document.Paths.Map() {
		item := pristine.Paths.Value(path)
		for method, widenedOperation := range widenedItem.Operations() {
			operation := item.GetOperation(method)
			for _, ref := range slices.Concat(widenedItem.Parameters, widenedOperation.Parameters) {
				if ref.Value.In != openapi3.ParameterInQuery || ref.Value.Schema == nil {
					continue
				}
				name := ref.Value.Name
				if _, raised := renderQueryMaximumCompatibility[compatOperationID(operation.OperationID)][name]; raised {
					continue
				}
				original := queryParameterSchema(item, operation, name)
				widened := ref.Value.Schema.Value
				if original != nil && original.Max != nil && (widened.Max == nil || *widened.Max != *original.Max) {
					t.Errorf("%s %s: %q maximum moved from Render's %g with no entry naming it", method, path, name, *original.Max)
				}
			}
		}
	}
}

func queryParameterSchema(item *openapi3.PathItem, operation *openapi3.Operation, name string) *openapi3.Schema {
	for _, params := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
		for _, ref := range params {
			if ref != nil && ref.Value != nil && ref.Value.In == openapi3.ParameterInQuery && ref.Value.Name == name &&
				ref.Value.Schema != nil {
				return ref.Value.Schema.Value
			}
		}
	}
	return nil
}
