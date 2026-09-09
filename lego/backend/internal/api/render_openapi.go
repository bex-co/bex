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
	"maps"
	"net/http"
	"slices"
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
	// allowedQuery is the per-operation set of accepted query names, resolved
	// once at load time. Keyed by the operation pointer rather than its id so
	// operations the upstream document leaves unnamed cannot collide on "".
	allowedQuery map[*openapi3.Operation]map[string]struct{}
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

// renderPathParameterPatternCompatibility widens a path parameter's id-shape
// regex to also accept the prefix bex actually mints (w6/m96). Keyed
// operationId → parameter name → replacement pattern, the same
// one-concession-per-line shape as the three maps above.
//
// Render pins each id path parameter to its own historical prefix, and bex
// deliberately mints the SAME prefix for every kind these routes address —
// dsk-, whk-, evt-, job- — so ids are drop-in for Render-shaped clients
// (docs/ADR020-identifiers.md). Blueprint is the one exception: it predates
// this gate and was minted blp- against Render's exs-, which meant the
// validator rejected every REST call to a blueprint-id route with
// `invalid path parameter "blueprintId"` BEFORE authz or lookup ever ran. No
// bex id has ever matched exs- and none can: id.Kind is a closed compile-time
// registry.
//
// Every override here must be a strict SUPERSET of the pattern it replaces —
// enforced by TestRenderPathParameterOverridesOnlyWiden. That is the invariant
// the whole file already keeps (each of the other three maps only relaxes),
// and it is what makes this a compatibility concession rather than a quiet
// narrowing of Render's contract: a client holding a real Render-shaped id
// still gets it through the gate and gets an honest 404 from the store, not a
// syntax error about an id shape Render itself documents.
var renderPathParameterPatternCompatibility = map[string]map[string]string{
	"retrieve-blueprint":   {"blueprintId": renderBlueprintIDPattern},
	"update-blueprint":     {"blueprintId": renderBlueprintIDPattern},
	"disconnect-blueprint": {"blueprintId": renderBlueprintIDPattern},
	"list-blueprint-syncs": {"blueprintId": renderBlueprintIDPattern},
}

// renderBlueprintIDPattern accepts bex's blp- alongside Render's exs-. The
// character class stays Render's [0-9a-z] rather than narrowing to xid's own
// base32-hex [0-9a-v]: a real bex id satisfies both, and widening is the only
// direction this layer is allowed to move.
const renderBlueprintIDPattern = `^(?:blp|exs)-[0-9a-z]{20}$`

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

	// Render's endpoint-id operations do not carry a workspace selector. Bex
	// accepts one consistently so a caller can address a non-default workspace
	// without weakening the endpoint-scoped store lookup.
	"retrieve-webhook": {"ownerId": {}},
	"update-webhook":   {"ownerId": {}},
	"delete-webhook":   {"ownerId": {}},
	// Render's public history envelope is unchanged; bex additionally lets the
	// dashboard and live verifier select immutable attempt outcomes directly.
	"list-webhook-events": {"ownerId": {}, "status": {}},
	// Bex's dashboard carries its selected workspace explicitly. Render's event
	// route has no owner selector, so this remains a labeled query extension.
	"retrieve-event": {"ownerId": {}},

	// The same concession the webhook and event routes above already make, for
	// the blueprint-id routes it was never extended to (w6/m96). All four bex
	// handlers read ownerId (internal/apps/rest.go), and none of Render's four
	// operations declares it, so the strict-query gate 400'd every call that
	// named a workspace — the second half of the same bug the blueprintId
	// pattern override fixes, and invisible until a test drove these routes
	// through the real composed server rather than a bare mux.
	"retrieve-blueprint":   {"ownerId": {}},
	"update-blueprint":     {"ownerId": {}},
	"disconnect-blueprint": {"ownerId": {}},
	"list-blueprint-syncs": {"ownerId": {}},

	// Render's disk-capacity series covers only its own service disks, so its
	// schema has no `kind`. Bex reads capacity for three resource kinds through
	// one verb — a managed Postgres, a managed Key Value, and (since ADR082) a
	// service's attached disk — and selects between them with `kind`. Without
	// this entry the strict-query gate 400s the parameter, which left REST able
	// to read a disk's USED bytes (that route is bex-native, so ungated) but not
	// its capacity, while GraphQL and MCP could read both. Found by the w1/m86
	// parity audit.
	"get-disk-capacity": {"kind": {}},

	// Bex's replica-percentage surface (w5/m89's aggregateAllMethod +
	// w5/m90's percentage) on the eight App-metrics operations sharing
	// metrics.parseMetricParams. Render's pinned schema only names its own
	// interval aggregationMethod, so without these entries the strict-query
	// gate 400s the exact DoD call
	// (?percentage=true&aggregateAllMethod=AVG) while GraphQL and MCP serve
	// it — found by the w5/m90 t008 live walkthrough driving the real
	// composed server (bare-mux tests never see the gate, the same shape as
	// the w6/m96 blueprint ownerId miss above).
	"get-cpu":            {"percentage": {}, "aggregateAllMethod": {}},
	"get-memory":         {"percentage": {}, "aggregateAllMethod": {}},
	"get-cpu-target":     {"percentage": {}, "aggregateAllMethod": {}},
	"get-memory-target":  {"percentage": {}, "aggregateAllMethod": {}},
	"get-instance-count": {"percentage": {}, "aggregateAllMethod": {}},
	"get-http-requests":  {"percentage": {}, "aggregateAllMethod": {}},
	"get-http-latency":   {"percentage": {}, "aggregateAllMethod": {}},
	"get-bandwidth":      {"percentage": {}, "aggregateAllMethod": {}},
}

// These are deliberate bex body extensions on Render-shaped operations. The
// runtime contract is widened before validation so fields the handler already
// supports are not rejected as unknown by the pinned upstream schema.
var renderBodyPropertyExtensions = map[string]map[string]*openapi3.SchemaRef{
	"create-key-value": {"public": {Value: openapi3.NewBoolSchema()}},
	"create-postgres":  {"public": {Value: openapi3.NewBoolSchema()}},
}

func loadRenderOpenAPIContract() (*renderOpenAPIContract, error) {
	return loadRenderOpenAPIContractData(renderOpenAPISource, renderOpenAPISHA256)
}

// verifyPinnedArtifact checks an embedded upstream pin against its recorded
// digest. Shared by the REST spec and the MCP tool surface so both fail the same
// way on a hand-edit; `fix` names the refresh path for whoever tripped it.
func verifyPinnedArtifact(what string, data []byte, expectedSHA256, fix string) error {
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest == expectedSHA256 {
		return nil
	}
	if fix != "" {
		return fmt.Errorf("%s integrity mismatch: got %s, want %s (%s)", what, digest, expectedSHA256, fix)
	}
	return fmt.Errorf("%s integrity mismatch: got %s, want %s", what, digest, expectedSHA256)
}

func loadRenderOpenAPIContractData(data []byte, expectedSHA256 string) (*renderOpenAPIContract, error) {
	if err := verifyPinnedArtifact("Render OpenAPI", data, expectedSHA256, ""); err != nil {
		return nil, err
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
	return &renderOpenAPIContract{document: doc, router: router, allowedQuery: buildAllowedQueryNames(doc)}, nil
}

// buildAllowedQueryNames resolves every operation's accepted query names — its
// own parameters, its path item's, and any deliberate bex extension — once at
// load time. The inputs are immutable after the one-shot contract load, and
// hasUnknownRenderQuery runs on every request to the Render-compatible surface.
func buildAllowedQueryNames(doc *openapi3.T) map[*openapi3.Operation]map[string]struct{} {
	index := map[*openapi3.Operation]map[string]struct{}{}
	for _, item := range doc.Paths.Map() {
		for _, operation := range item.Operations() {
			allowed := map[string]struct{}{}
			for _, parameters := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range parameters {
					if ref != nil && ref.Value != nil && ref.Value.In == openapi3.ParameterInQuery {
						allowed[ref.Value.Name] = struct{}{}
					}
				}
			}
			maps.Copy(allowed, renderQueryExtensions[operation.OperationID])
			index[operation] = allowed
		}
	}
	return index
}

func applyRenderCompatibility(doc *openapi3.T) {
	for _, item := range doc.Paths.Map() {
		for _, operation := range item.Operations() {
			optional := renderOptionalParameterCompatibility[operation.OperationID]
			patterns := renderPathParameterPatternCompatibility[operation.OperationID]
			for _, parameters := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range parameters {
					if ref == nil || ref.Value == nil {
						continue
					}
					if slices.Contains(optional, ref.Value.Name) {
						ref.Value.Required = false
					}
					if pattern, ok := patterns[ref.Value.Name]; ok && ref.Value.Schema != nil && ref.Value.Schema.Value != nil {
						ref.Value.Schema.Value.Pattern = pattern
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
			maps.Copy(media.Schema.Value.Properties, renderBodyPropertyExtensions[operation.OperationID])
			if len(remove) > 0 {
				media.Schema.Value.Required = slices.DeleteFunc(media.Schema.Value.Required, func(name string) bool {
					return slices.Contains(remove, name)
				})
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
		// ContainsFunc, not Contains: Enum is []any, and == on an interface
		// holding an uncomparable dynamic type panics.
		if !slices.ContainsFunc(schema.Value.Enum, func(existing any) bool { return existing == value }) {
			schema.Value.Enum = append(schema.Value.Enum, value)
		}
	}
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
	if retiredPublicRESTAlias(r) {
		http.NotFound(w, r)
		return
	}
	// A route that exists only in Render's much larger spec must retain bex's
	// existing 404/405. Conversely, bex-native routes are outside
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
	if v.contract.hasUnknownRenderQuery(route, r) {
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

func retiredPublicRESTAlias(r *http.Request) bool {
	path := r.URL.Path
	switch {
	case path == "/v1/apps" || strings.HasPrefix(path, "/v1/apps/"):
		return true
	case path == "/v1/databases" || strings.HasPrefix(path, "/v1/databases/"):
		return true
	case path == "/v1/registry-credentials" || strings.HasPrefix(path, "/v1/registry-credentials/"):
		return true
	case path == "/v1/webhooks/endpoints" || strings.HasPrefix(path, "/v1/webhooks/endpoints/"):
		return true
	case strings.HasPrefix(path, "/v1/postgres/") && strings.HasSuffix(path, "/exports"):
		return true
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/v1/postgres/") && strings.HasSuffix(path, "/recovery-info"):
		return true
	default:
		return false
	}
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

func (c *renderOpenAPIContract) hasUnknownRenderQuery(route *routers.Route, r *http.Request) bool {
	allowed := c.allowedQuery[route.Operation]
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
