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

// Package rendertest validates bex's REST responses against the pinned Render
// OpenAPI document, for tests. Wrap a test's handler with Wrap and every
// response to a request that matches a Render operation is checked for a
// documented status and a body matching that status's schema; a violation the
// shrink-only Allowlist does not excuse fails the calling test.
//
// Test-only: depguard (lego/backend/.golangci.yml) forbids importing it from
// non-test code. It imports no feature package, so internal/api tests and the
// feature packages' own httptest suites (internal/apps) share one validator.
package rendertest

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/bex-co/bex/lego/backend/internal/api/openapi"
)

const apiPrefix = "/v1"

// Contract is the pinned Render document and a router over its paths.
type Contract struct {
	Document *openapi3.T
	router   routers.Router
}

var loadContract = sync.OnceValues(func() (*Contract, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(openapi.RenderPublicAPI)
	if err != nil {
		return nil, fmt.Errorf("load pinned Render OpenAPI: %w", err)
	}
	// bex serves its own hostnames; routes match on the /v1-relative path.
	doc.Servers = nil
	router, err := legacy.NewRouter(doc, openapi3.AllowExtraSiblingFields("description", "default"))
	if err != nil {
		return nil, fmt.Errorf("route pinned Render OpenAPI: %w", err)
	}
	return &Contract{Document: doc, router: router}, nil
})

// Load returns the pinned contract, parsed once per test binary.
func Load(t testing.TB) *Contract {
	t.Helper()
	c, err := loadContract()
	if err != nil {
		t.Fatalf("rendertest: %v", err)
	}
	return c
}

// Matcher names the Render operation a request is served as, or nil for a
// request outside the contract (a bex-native route, a non-/v1 path).
type Matcher func(*http.Request) *routers.Route

// MuxMatcher matches a request served by a bare feature mux (no request gate):
// the mux must route it, and the Render template must not hold a parameter
// where the mux pattern has a literal segment, nor differ in length — those
// are bex-native routes that only look like a Render path, which the
// production gate (internal/api renderOperation) also passes through
// unvalidated.
func (c *Contract) MuxMatcher(mux *http.ServeMux) Matcher {
	return func(r *http.Request) *routers.Route {
		_, pattern := mux.Handler(r)
		route := c.Route(r)
		if pattern == "" || route == nil {
			return nil
		}
		if space := strings.LastIndex(pattern, " "); space >= 0 {
			pattern = pattern[space+1:]
		}
		bex := strings.Split(strings.Trim(strings.TrimPrefix(pattern, apiPrefix), "/"), "/")
		render := strings.Split(strings.Trim(route.Path, "/"), "/")
		if len(bex) != len(render) {
			return nil
		}
		for i, segment := range render {
			if isParam(segment) && !isParam(bex[i]) {
				return nil
			}
		}
		return route
	}
}

func isParam(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

// Route finds the Render operation for r's method and /v1-relative path.
func (c *Contract) Route(r *http.Request) *routers.Route {
	if !strings.HasPrefix(r.URL.Path, apiPrefix+"/") {
		return nil
	}
	clone := r.Clone(r.Context())
	u := *clone.URL
	u.Path = strings.TrimPrefix(u.Path, apiPrefix)
	clone.URL = &u
	route, _, err := c.router.FindRoute(clone)
	if err != nil {
		return nil
	}
	return route
}

// Operation returns the route for a Render operationId, or nil.
func (c *Contract) Operation(operationID string) *routers.Route {
	for path, item := range c.Document.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == operationID {
				return &routers.Route{Spec: c.Document, Path: path, PathItem: item, Method: method, Operation: op}
			}
		}
	}
	return nil
}

// Validate checks one response against route: the status must be documented
// (or covered by a default response) and the body must match that status's
// schema — enums, formats, unions and required keys included. Each violation
// is one string led by its JSON pointer, narrow enough for an Allowlist entry.
//
// An error status (4xx/5xx) the operation does not list is checked against
// Render's shared error schema instead: Render's per-operation error lists are
// incomplete (no operation lists 405, few list 403 or 409), so an honest
// status with Render's error body is not drift. A success or redirect status
// the operation does not list is a violation — a client generated from the
// spec parses no body for it.
func Validate(route *routers.Route, status int, header http.Header, body []byte) []string {
	if header == nil {
		header = http.Header{}
	}
	if strings.EqualFold(header.Get("Content-Encoding"), "gzip") {
		decoded, err := gunzip(body)
		if err != nil {
			return []string{"$: gzip body does not decode: " + err.Error()}
		}
		body = decoded
	}
	responses := route.Operation.Responses
	if responses.Status(status) == nil && responses.Default() == nil {
		if status < http.StatusBadRequest {
			return []string{fmt.Sprintf("status %d: not a documented response", status)}
		}
		return validateError(route.Spec, body)
	}
	req := httptest.NewRequest(route.Method, "https://api.bex.co/v1", nil)
	input := (&openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, Route: route},
		Status:                 status,
		Header:                 header,
		Options: &openapi3filter.Options{
			MultiError:            true,
			IncludeResponseStatus: true,
			SchemaValidationOptions: []openapi3.SchemaValidationOption{
				openapi3.EnableFormatValidation(),
			},
		},
	}).SetBodyBytes(body)
	var found []string
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		found = violations(err)
	}
	if !documentsBody(responses, status) {
		// Render documents no body here (a 202 lifecycle verb), so whatever
		// bex sends beyond it is an extension, not a service to check.
		return found
	}
	var root any
	if json.Unmarshal(body, &root) != nil {
		return found
	}
	return checkEventDetails(route.Spec, root, checkServiceDetails(route.Spec, root, found))
}

// documentsBody reports whether status's documented response has a schema.
func documentsBody(responses *openapi3.Responses, status int) bool {
	ref := responses.Status(status)
	if ref == nil {
		ref = responses.Default()
	}
	if ref == nil || ref.Value == nil {
		return false
	}
	for _, media := range ref.Value.Content {
		if media.Schema != nil {
			return true
		}
	}
	return false
}

const unionMismatch = ": does not match the documented union"

// serviceDetailsBranch is the serviceDetails union member for each service
// type, as Render's schema pairs them.
var serviceDetailsBranch = map[string]string{
	"static_site":       "staticSiteDetails",
	"web_service":       "webServiceDetails",
	"private_service":   "privateServiceDetails",
	"background_worker": "backgroundWorkerDetails",
	"cron_job":          "cronJobDetails",
}

// checkServiceDetails validates every service in body's serviceDetails
// against the union member its sibling type selects. Render's union alone
// accepts details of the wrong kind (cron details on a web service match the
// cron member) and reports a member's missing field only as one opaque union
// failure; this reports each violation at its own pointer, replacing that
// service's union failure.
func checkServiceDetails(doc *openapi3.T, root any, found []string) []string {
	var refined []string
	replaced := map[string]bool{}
	walkServices(root, "", func(pointer string, service map[string]any) {
		branch := doc.Components.Schemas[serviceDetailsBranch[service["type"].(string)]]
		if branch == nil || branch.Value == nil {
			return
		}
		replaced[pointer+"/serviceDetails"+unionMismatch] = true
		err := branch.Value.VisitJSON(service["serviceDetails"], openapi3.MultiErrors(), openapi3.EnableFormatValidation())
		for _, v := range violations(err) {
			refined = append(refined, pointer+"/serviceDetails"+strings.TrimPrefix(v, "$"))
		}
	})
	out := slices.DeleteFunc(found, func(v string) bool { return replaced[v] })
	return append(out, refined...)
}

// checkEventDetails validates every event's details in body against the
// member of Render's details union its type names (w3/m302). The union alone
// matches empty details to every member without required keys, so it fails
// them as ambiguous, and it reports a member's renamed or mistyped key only as
// one opaque union failure; this reports each violation at its own pointer,
// replacing that event's union failure; each names the event type last, so an
// Allowlist entry can excuse one type's field. A bex-named type has no member, and
// its type is already its own enum violation, so its union failure goes too.
// A documented type without a member keeps it.
func checkEventDetails(doc *openapi3.T, root any, found []string) []string {
	events := eventSchemaOf(doc)
	if events.members == nil {
		return found
	}
	var refined []string
	replaced := map[string]bool{}
	walkEvents(root, "", func(pointer string, event map[string]any) {
		kind := event["type"].(string)
		member := events.members[eventMemberKey(kind)]
		if member == nil && events.documented[kind] {
			return
		}
		replaced[pointer+"/details"+unionMismatch] = true
		if member == nil {
			return
		}
		err := member.VisitJSON(event["details"], openapi3.MultiErrors(), openapi3.EnableFormatValidation())
		for _, v := range violations(err) {
			refined = append(refined, fmt.Sprintf("%s/details%s (%s)", pointer, strings.TrimPrefix(v, "$"), kind))
		}
	})
	out := slices.DeleteFunc(found, func(v string) bool { return replaced[v] })
	return append(out, refined...)
}

// eventSchema is the pinned spec's event details union members, keyed by
// eventMemberKey of their titles, and the event types it documents.
type eventSchema struct {
	members    map[string]*openapi3.Schema
	documented map[string]bool
}

// eventSchemas caches eventSchemaOf per document: Validate runs on every
// response a suite produces, and the pinned document does not change.
var eventSchemas sync.Map // *openapi3.T → eventSchema

// eventSchemaOf reads doc's event details union; its members are nil when the
// spec has no single-event operation.
func eventSchemaOf(doc *openapi3.T) eventSchema {
	if cached, ok := eventSchemas.Load(doc); ok {
		return cached.(eventSchema)
	}
	var out eventSchema
	if item := doc.Paths.Value("/events/{eventId}"); item != nil && item.Get != nil {
		if ok := item.Get.Responses.Status(http.StatusOK); ok != nil && ok.Value != nil && ok.Value.Content.Get("application/json") != nil {
			event := ok.Value.Content.Get("application/json").Schema.Value
			if details, kind := event.Properties["details"], event.Properties["type"]; details != nil && kind != nil {
				out.members, out.documented = map[string]*openapi3.Schema{}, map[string]bool{}
				for _, group := range details.Value.OneOf {
					for _, member := range group.Value.OneOf {
						out.members[eventMemberKey(member.Value.Title)] = member.Value
					}
				}
				for _, v := range kind.Value.Enum {
					out.documented[fmt.Sprint(v)] = true
				}
			}
		}
	}
	eventSchemas.Store(doc, out)
	return out
}

// MissingEventDetail is the violation for an eventType event whose details
// lack field, a JSON pointer tail under details ("suspendedByUser/email").
func MissingEventDetail(eventType, field string) string {
	return fmt.Sprintf(`/details/%s: property %q is missing (%s)`, field, field[strings.LastIndex(field, "/")+1:], eventType)
}

// eventMemberKey matches an event type to its details member's title: Render
// titles each member after its type ("Build Ended" for build_ended, "Postgres
// HA Status Changed" for postgres_ha_status_changed), except plan_changed,
// whose member is "Instance Type Changed".
func eventMemberKey(name string) string {
	if name == "plan_changed" {
		name = "Instance Type Changed"
	}
	return strings.ToLower(memberKeySeparators.Replace(name))
}

var memberKeySeparators = strings.NewReplacer(" ", "", "_", "")

// walkEvents calls visit for every event object in node (one with a type, a
// timestamp and details), with its JSON pointer, in document order.
func walkEvents(node any, pointer string, visit func(string, map[string]any)) {
	switch n := node.(type) {
	case map[string]any:
		_, timed := n["timestamp"].(string)
		_, detailed := n["details"]
		if _, typed := n["type"].(string); typed && timed && detailed {
			visit(pointer, n)
			return
		}
		for _, key := range slices.Sorted(maps.Keys(n)) {
			walkEvents(n[key], pointer+"/"+key, visit)
		}
	case []any:
		for i, item := range n {
			walkEvents(item, pointer+"/"+strconv.Itoa(i), visit)
		}
	}
}

// walkServices calls visit for every object in node that has a service type
// and a serviceDetails member, with its JSON pointer, in document order.
func walkServices(node any, pointer string, visit func(string, map[string]any)) {
	switch n := node.(type) {
	case map[string]any:
		if kind, ok := n["type"].(string); ok && serviceDetailsBranch[kind] != "" {
			if _, ok := n["serviceDetails"]; ok {
				visit(pointer, n)
			}
		}
		for _, key := range slices.Sorted(maps.Keys(n)) {
			walkServices(n[key], pointer+"/"+key, visit)
		}
	case []any:
		for i, item := range n {
			walkServices(item, pointer+"/"+strconv.Itoa(i), visit)
		}
	}
}

// validateError checks body against Render's shared error schema
// (#/components/schemas/error, the body of every documented error response).
func validateError(doc *openapi3.T, body []byte) []string {
	schema := doc.Components.Schemas["error"]
	if schema == nil || schema.Value == nil {
		return []string{`$: the pinned spec has no "error" schema`}
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return []string{"$: error body is not JSON: " + err.Error()}
	}
	if err := schema.Value.VisitJSON(value, openapi3.MultiErrors()); err != nil {
		return violations(err)
	}
	return nil
}

func gunzip(body []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return io.ReadAll(zr)
}

// violations keeps allowlist entries narrow. kin's default Error text includes
// complete schemas and values, and a MultiError arrives as one joined string. A
// union failure is one actionable divergence at its JSON pointer; ordinary
// independent failures remain separate entries.
func violations(err error) []string {
	if err == nil {
		return nil
	}
	var responseErr *openapi3filter.ResponseError
	if errors.As(err, &responseErr) && responseErr == err {
		if responseErr.Err != nil {
			return violations(responseErr.Err)
		}
		return []string{"$: " + responseErr.Reason}
	}
	if many, ok := err.(openapi3.MultiError); ok {
		var out []string
		for _, item := range many {
			out = append(out, violations(item)...)
		}
		return out
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) && schemaErr == err {
		pointer := "/" + strings.Join(schemaErr.JSONPointer(), "/")
		if pointer == "/" {
			pointer = "$"
		}
		switch {
		case schemaErr.Origin != nil:
			return []string{pointer + unionMismatch}
		case schemaErr.SchemaField == "enum":
			// kin's reason lists every allowed value; the offending one is
			// what an entry and a reader need.
			return []string{fmt.Sprintf("%s: %q is not in the documented enum", pointer, fmt.Sprint(schemaErr.Value))}
		}
		return []string{pointer + ": " + schemaErr.Reason}
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return violations(unwrapped)
	}
	return []string{err.Error()}
}

// Wrap serves next and validates every response to a request that match
// names as a Render operation, failing t (with the operation, status and JSON
// pointer) for each violation the Allowlist does not excuse. Other requests
// pass through untouched. The response streams to the caller as it is
// written; only a copy is kept for validation.
func Wrap(t testing.TB, next http.Handler, match Matcher, unconfigured ...Source) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := match(r)
		if route == nil {
			next.ServeHTTP(w, r)
			return
		}
		tee := &teeWriter{ResponseWriter: w}
		next.ServeHTTP(tee, r)
		if tee.unwrapped || isStream(tee.Header()) {
			return
		}
		status := tee.status
		if status == 0 {
			status = http.StatusOK
		}
		opID := route.Operation.OperationID
		if errs := Unallowed(opID, Validate(route, status, tee.Header(), tee.body.Bytes()), unconfigured...); len(errs) > 0 {
			t.Errorf("Render response conformance: %s %s => %d (operationId=%s)\n"+
				"Fix the response to match the pinned Render schema (rendertest.Allowlist only shrinks).\n  %s",
				r.Method, r.URL.Path, status, opID, strings.Join(errs, "\n  "))
		}
	})
}

// isStream reports a server-sent event stream: an unbounded body the
// documented JSON schema does not describe frame by frame.
func isStream(h http.Header) bool {
	return strings.HasPrefix(h.Get("Content-Type"), "text/event-stream")
}

// teeWriter passes a response through while keeping its status and a body
// copy. Flush and Unwrap keep streaming and http.ResponseController working.
type teeWriter struct {
	http.ResponseWriter
	status    int
	body      bytes.Buffer
	unwrapped bool
}

func (w *teeWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *teeWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func (w *teeWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *teeWriter) Unwrap() http.ResponseWriter {
	w.unwrapped = true // the caller may hijack or write around the copy
	return w.ResponseWriter
}
