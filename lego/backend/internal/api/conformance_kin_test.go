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
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/routers"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
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
	return rendertest.Validate(route, status, http.Header{"Content-Type": {"application/json"}}, body)
}

// validatingRenderResponses wraps a composed bex-api handler so every response
// to a request the production request gate validates as a Render operation is
// itself validated against that operation (rendertest.Wrap). It matches with
// the gate's own rule (renderOperation over the REST mux), so a bex-native
// route that only looks like a Render path stays unvalidated here exactly as
// it does there; GraphQL, MCP and every non-/v1 route pass through untouched.
func validatingRenderResponses(t testing.TB, rest *http.ServeMux, next http.Handler) http.Handler {
	t.Helper()
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatalf("conformance: load embedded Render OpenAPI: %v", err)
	}
	return rendertest.Wrap(t, next, func(r *http.Request) *routers.Route {
		if retiredPublicRESTAlias(r) {
			return nil
		}
		_, pattern := rest.Handler(r)
		if pattern == "" {
			return nil
		}
		route, _, _, ok := contract.renderOperation(pattern, r)
		if !ok {
			return nil
		}
		return route
	})
}

// recordingTB captures what the response validator reports, so a test can
// assert on a failure without failing itself.
type recordingTB struct {
	testing.TB
	errors []string
}

func (r *recordingTB) Helper() {}
func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// TestBuildHandlerFailsAWrongFieldInAServedResponse is the milestone's
// demonstration on the real composed server: the handler buildHandler returns
// fails the calling test when a served /v1 response carries a wrong-typed
// field, and leaves GraphQL (not a Render operation) alone.
func TestBuildHandlerFailsAWrongFieldInAServedResponse(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	muxes, err := srv.composedMuxes()
	if err != nil {
		t.Fatal(err)
	}
	// wrongSuspended serves the real response with Render's string enum
	// `suspended` turned into a boolean, the w2/008 class of drift.
	wrongSuspended := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			maps.Copy(w.Header(), rec.Header())
			w.WriteHeader(rec.Code)
			_, _ = w.Write(bytes.ReplaceAll(rec.Body.Bytes(), []byte(`"suspended":"not_suspended"`), []byte(`"suspended":false`)))
		})
	}

	honest := &recordingTB{TB: t}
	do(t, validatingRenderResponses(honest, muxes.rest, srv.wrapMuxes(muxes)), "GET", "/v1/services/web", testToken, "")
	if len(honest.errors) != 0 {
		t.Fatalf("the untampered response failed validation: %q", honest.errors)
	}

	drifted := &recordingTB{TB: t}
	h := validatingRenderResponses(drifted, muxes.rest, wrongSuspended(srv.wrapMuxes(muxes)))
	if w := do(t, h, "GET", "/v1/services/web", testToken, ""); w.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", w.Code, w.Body)
	}
	if len(drifted.errors) != 1 || !strings.Contains(drifted.errors[0], "operationId=retrieve-service") ||
		!strings.Contains(drifted.errors[0], "/suspended: ") {
		t.Fatalf("validator errors = %q, want one naming retrieve-service and /suspended", drifted.errors)
	}

	drifted.errors = nil
	body, _ := json.Marshal(map[string]string{"query": `{ services { name } }`})
	do(t, h, "POST", "/graphql", testToken, string(body))
	if len(drifted.errors) != 0 {
		t.Fatalf("GraphQL was validated as a Render operation: %q", drifted.errors)
	}
}
