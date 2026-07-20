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
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
)

const (
	renderOpenAPISHA256 = "2d27d5834d8bbc586e0aee62160cf996bb07f4be747112f15ec02c14fc11b315"
	renderAPIPrefix     = "/v1"
)

// renderOpenAPISource is Render's complete, unversioned public contract,
// fetched from https://api-docs.render.com/openapi/render-public-api-1.json on
// 2026-07-20. It is deliberately embedded: API startup and CI never depend on
// Render's availability or silently accept an upstream contract change.
//
//go:embed openapi/render-public-api-1.json
var renderOpenAPISource []byte

type renderOpenAPIContract struct {
	document *openapi3.T
	router   routers.Router
}

var renderContractOnce = sync.OnceValues(loadRenderOpenAPIContract)

// bex relaxes only requiredness that predates the runtime contract gate. The
// fields remain type-checked whenever supplied. Keeping this operation-keyed
// makes each compatibility concession visible and testable instead of
// weakening shared component schemas indiscriminately.
var renderRequiredCompatibility = map[string][]string{
	"create-env-group": {"ownerId", "envVars"},
	"create-key-value": {"ownerId", "plan"},
	"create-postgres":  {"ownerId", "plan", "version"},
	"create-project":   {"environments"},
	"create-service":   {"ownerId", "type"},
	"create-webhook":   {"ownerId", "enabled", "eventFilter"},
}

var renderOptionalParameterCompatibility = map[string][]string{
	// bex resolves an omitted ownerId from the authenticated caller's selected
	// or default workspace on all three log reads.
	"list-logs":        {"ownerId"},
	"list-logs-values": {"ownerId"},
	"subscribe-logs":   {"ownerId"},
}

// These are deliberate bex query extensions on otherwise Render-shaped
// operations. All other query names must come from the matched OpenAPI
// operation (including its path-level parameters).
var renderQueryExtensions = map[string]map[string]struct{}{
	"create-service":  {"dryRun": {}},
	"update-service":  {"dryRun": {}},
	"delete-service":  {"confirm": {}},
	"suspend-service": {"confirm": {}},
	"resume-service":  {"confirm": {}},
	"restart-service": {"confirm": {}},

	"create-key-value":  {"dryRun": {}},
	"update-key-value":  {"dryRun": {}},
	"delete-key-value":  {"confirm": {}},
	"suspend-key-value": {"confirm": {}},
	"resume-key-value":  {"confirm": {}},

	"create-postgres":  {"dryRun": {}},
	"update-postgres":  {"dryRun": {}},
	"delete-postgres":  {"confirm": {}},
	"suspend-postgres": {"confirm": {}},
	"resume-postgres":  {"confirm": {}},
	"restart-postgres": {"confirm": {}},
}

func loadRenderOpenAPIContract() (*renderOpenAPIContract, error) {
	return loadRenderOpenAPIContractData(renderOpenAPISource, renderOpenAPISHA256)
}

func loadRenderOpenAPIContractData(data []byte, expectedSHA256 string) (*renderOpenAPIContract, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != expectedSHA256 {
		return nil, fmt.Errorf("Render OpenAPI integrity mismatch: got %s, want %s", digest, expectedSHA256)
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("load embedded Render OpenAPI: %w", err)
	}
	// The upstream server includes both the Render hostname and /v1. bex accepts
	// its own hostnames, so the request wrapper removes /v1 on a cloned URL and
	// routes against the document paths without a server constraint.
	doc.Servers = nil
	applyRenderCompatibility(doc)

	router, err := legacy.NewRouter(doc, openapi3.AllowExtraSiblingFields("description", "default"))
	if err != nil {
		return nil, fmt.Errorf("validate embedded Render OpenAPI: %w", err)
	}
	return &renderOpenAPIContract{document: doc, router: router}, nil
}

func applyRenderCompatibility(doc *openapi3.T) {
	for _, item := range doc.Paths.Map() {
		for _, operation := range item.Operations() {
			optional := renderOptionalParameterCompatibility[operation.OperationID]
			for _, parameters := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range parameters {
					if ref != nil && ref.Value != nil && containsString(optional, ref.Value.Name) {
						ref.Value.Required = false
					}
				}
			}

			remove := renderRequiredCompatibility[operation.OperationID]
			if operation.RequestBody == nil || operation.RequestBody.Value == nil {
				continue
			}
			media := operation.RequestBody.Value.Content.Get("application/json")
			if media == nil || media.Schema == nil || media.Schema.Value == nil {
				continue
			}
			if len(remove) > 0 {
				required := media.Schema.Value.Required[:0]
				for _, name := range media.Schema.Value.Required {
					if !containsString(remove, name) {
						required = append(required, name)
					}
				}
				media.Schema.Value.Required = required
			}
			applyRenderSchemaCompatibility(operation.OperationID, media.Schema.Value)
		}
	}
}

func applyRenderSchemaCompatibility(operationID string, schema *openapi3.Schema) {
	if operationID == "create-postgres" || operationID == "update-postgres" {
		extendSchemaEnum(schema.Properties["plan"], tiers.Postgres.IDs())
	}
	if operationID != "create-service" && operationID != "update-service" {
		return
	}
	details := schema.Properties["serviceDetails"]
	if details == nil || details.Value == nil || len(details.Value.OneOf) == 0 {
		return
	}
	// Render's serviceDetails request branches overlap by design (for example a
	// plan-only PATCH matches web, private, worker, and cron details). kin's
	// literal oneOf evaluation rejects that valid common subset, so request
	// validation uses anyOf while all branch constraints remain active.
	details.Value.AnyOf = append(details.Value.AnyOf, details.Value.OneOf...)
	details.Value.OneOf = nil
}

func extendSchemaEnum(schema *openapi3.SchemaRef, values []string) {
	if schema == nil || schema.Value == nil {
		return
	}
	for _, value := range values {
		present := false
		for _, existing := range schema.Value.Enum {
			if existing == value {
				present = true
				break
			}
		}
		if !present {
			schema.Value.Enum = append(schema.Value.Enum, value)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type renderRequestValidator struct {
	next     *http.ServeMux
	contract *renderOpenAPIContract
}

func newRenderRequestValidator(next *http.ServeMux) (http.Handler, error) {
	contract, err := renderContractOnce()
	if err != nil {
		return nil, err
	}
	return &renderRequestValidator{next: next, contract: contract}, nil
}

func (v *renderRequestValidator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A route that exists only in Render's much larger spec must retain bex's
	// existing 404/405. Conversely, bex-native routes and aliases are outside
	// this contract and pass through byte-for-byte.
	if _, pattern := v.next.Handler(r); pattern == "" {
		v.next.ServeHTTP(w, r)
		return
	}

	validationRequest := r.Clone(r.Context())
	urlCopy := *validationRequest.URL
	urlCopy.Path = strings.TrimPrefix(urlCopy.Path, renderAPIPrefix)
	if urlCopy.Path == "" {
		urlCopy.Path = "/"
	}
	validationRequest.URL = &urlCopy

	route, pathParams, err := v.contract.router.FindRoute(validationRequest)
	if err != nil {
		v.next.ServeHTTP(w, r)
		return
	}
	if operationHasNoBody(route) && requestHasBody(r) {
		core.WriteErrStatus(w, http.StatusBadRequest, "request body is not allowed for this operation")
		return
	}
	if hasUnknownRenderQuery(route, r) {
		core.WriteErrStatus(w, http.StatusBadRequest, "request contains an unsupported query parameter")
		return
	}

	// Clone copied the original Body interface. kin consumes and restores the
	// clone, so transfer its restored reader back before the real handler runs.
	validationRequest.Body = r.Body
	validationRequest.GetBody = r.GetBody
	validationRequest.ContentLength = r.ContentLength
	if requestHasBody(r) && validationRequest.Header.Get("Content-Type") == "" {
		validationRequest.Header = validationRequest.Header.Clone()
		validationRequest.Header.Set("Content-Type", "application/json")
	}
	err = validateRenderRequest(r.Context(), validationRequest, route, pathParams)
	r.Body = validationRequest.Body
	r.GetBody = validationRequest.GetBody
	r.ContentLength = validationRequest.ContentLength
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, safeRenderValidationMessage(err))
		return
	}

	v.next.ServeHTTP(w, r.WithContext(core.WithStrictJSONDecoding(r.Context())))
}

func validateRenderRequest(ctx context.Context, request *http.Request, route *routers.Route, pathParams map[string]string) (err error) {
	// Validation is an untrusted-input boundary. A library panic must not take
	// down the API process or expose the submitted payload in a panic page.
	defer func() {
		if recover() != nil {
			err = errors.New("Render OpenAPI request validation failed")
		}
	}()
	return openapi3filter.ValidateRequest(ctx, &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc:  openapi3filter.NoopAuthenticationFunc,
			MultiError:          false,
			SkipSettingDefaults: true,
			SchemaValidationOptions: []openapi3.SchemaValidationOption{
				openapi3.EnableFormatValidation(),
			},
		},
	})
}

func operationHasNoBody(route *routers.Route) bool {
	return route == nil || route.Operation == nil || route.Operation.RequestBody == nil
}

func requestHasBody(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0
}

func hasUnknownRenderQuery(route *routers.Route, r *http.Request) bool {
	allowed := make(map[string]struct{})
	addQueryParameters := func(parameters openapi3.Parameters) {
		for _, ref := range parameters {
			if ref != nil && ref.Value != nil && ref.Value.In == openapi3.ParameterInQuery {
				allowed[ref.Value.Name] = struct{}{}
			}
		}
	}
	addQueryParameters(route.PathItem.Parameters)
	addQueryParameters(route.Operation.Parameters)
	for name := range renderQueryExtensions[route.Operation.OperationID] {
		allowed[name] = struct{}{}
	}
	for name := range r.URL.Query() {
		if _, ok := allowed[name]; !ok {
			return true
		}
	}
	return false
}

func safeRenderValidationMessage(err error) string {
	var requestError *openapi3filter.RequestError
	if errors.As(err, &requestError) {
		if requestError.Parameter != nil {
			return fmt.Sprintf("invalid %s parameter %q", requestError.Parameter.In, requestError.Parameter.Name)
		}
		if requestError.RequestBody != nil {
			var schemaError *openapi3.SchemaError
			if errors.As(err, &schemaError) {
				if pointer := schemaError.JSONPointer(); len(pointer) > 0 {
					return "invalid request body at /" + strings.Join(pointer, "/")
				}
			}
			return "request body does not match the Render API schema"
		}
	}
	return "request does not match the Render API schema"
}
