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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

// renderSpec indexes the response operations in the same complete, pinned
// contract that protects live requests. Keeping one parsed document prevents
// the conformance suite from drifting onto a hand-maintained schema subset.
type renderSpec struct {
	routes map[string]*routers.Route
}

func loadRenderSpec(t *testing.T) *renderSpec {
	t.Helper()
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatalf("conformance: load embedded Render OpenAPI: %v", err)
	}

	routes := make(map[string]*routers.Route)
	for path, item := range contract.document.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation.OperationID == "" {
				continue
			}
			routes[operation.OperationID] = &routers.Route{
				Spec:      contract.document,
				Path:      path,
				PathItem:  item,
				Method:    strings.ToUpper(method),
				Operation: operation,
			}
		}
	}
	return &renderSpec{routes: routes}
}

// validate checks body against operationID's documented 200 response. kin's
// response validator enforces the complete schema, including enums, formats,
// ranges, unions, and additionalProperties behavior.
func (rs *renderSpec) validate(operationID string, body []byte) []string {
	return rs.validateStatus(operationID, http.StatusOK, body)
}

// validateStatus is validate for a success response documented under a status
// other than 200 — a create's 201 (create-project) reads its schema from the
// 201 response object, not the 200 one, so a plain validate() would report the
// operation as having no resolvable response.
func (rs *renderSpec) validateStatus(operationID string, status int, body []byte) []string {
	route, ok := rs.routes[operationID]
	if !ok {
		return []string{fmt.Sprintf("unknown operationId %q in pinned spec", operationID)}
	}
	req := httptest.NewRequest(route.Method, "https://api.bex.co/v1", nil)
	input := (&openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: req,
			Route:   route,
		},
		Status: status,
		Header: http.Header{"Content-Type": {"application/json"}},
		Options: &openapi3filter.Options{
			MultiError: true,
			SchemaValidationOptions: []openapi3.SchemaValidationOption{
				openapi3.EnableFormatValidation(),
			},
		},
	}).SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		return conformanceErrors(err)
	}
	return nil
}

// conformanceErrors keeps allowlist entries narrow. kin's default Error text
// includes complete schemas and values, and a MultiError arrives as one joined
// string. A union failure is one actionable divergence at its JSON pointer;
// ordinary independent failures remain separate entries.
func conformanceErrors(err error) []string {
	if err == nil {
		return nil
	}
	var responseErr *openapi3filter.ResponseError
	if errors.As(err, &responseErr) && responseErr == err && responseErr.Err != nil {
		return conformanceErrors(responseErr.Err)
	}
	if many, ok := err.(openapi3.MultiError); ok {
		var out []string
		for _, item := range many {
			out = append(out, conformanceErrors(item)...)
		}
		return out
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) && schemaErr == err {
		pointer := "/" + strings.Join(schemaErr.JSONPointer(), "/")
		if pointer == "/" {
			pointer = "$"
		}
		if schemaErr.Origin != nil {
			return []string{pointer + ": does not match the documented union"}
		}
		return []string{pointer + ": " + schemaErr.Reason}
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return conformanceErrors(unwrapped)
	}
	return []string{err.Error()}
}
