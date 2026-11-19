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

// Package savedsearch is the model of a project's observability view (w3/m505,
// docs/ADR010-observability.md § Project views): Render's saved-search types,
// its versioned filter document, the one validator every read and write path
// shares, and the rule that narrows a project's resources to a view's scope.
// It is a leaf: the projects feature serves the views and the logs feature
// reads a view's scope, and neither depends on the other.
package savedsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's SavedSearchType, less WORKFLOW_RUNS: Render Workflows is a bex
// non-goal (ADR018), so no view of that type exists.
const (
	TypeApplicationLogs = "APPLICATION_LOGS"
	TypeMetrics         = "METRICS"
	TypeRequestLogs     = "REQUEST_LOGS"
)

// Types lists the view types in Render's enum order.
var Types = []string{TypeApplicationLogs, TypeMetrics, TypeRequestLogs}

// Version is the only filter document version Render writes.
const Version = 1

// Render's filter operators.
const (
	OpIs    = "IS"
	OpIsNot = "IS_NOT"
)

// The filter fields, the labels of Render's Edit view modal in upper snake
// case: Render's own enum spellings were not captured (ADR010).
const (
	FieldEnvironment  = "ENVIRONMENT"
	FieldResource     = "RESOURCE"
	FieldResourceType = "RESOURCE_TYPE"
	FieldLevel        = "LEVEL"
	FieldInstance     = "INSTANCE"
	FieldText         = "TEXT"
	FieldTimeRange    = "TIME_RANGE"
	FieldMethod       = "METHOD"
	FieldStatusCode   = "STATUS_CODE"
	FieldHost         = "HOST"
	FieldPath         = "PATH"
)

// Render's Resource Type values: Services, Cron jobs, Postgres, Key Value.
const (
	ResourceTypeService  = "SERVICE"
	ResourceTypeCronJob  = "CRON_JOB"
	ResourceTypePostgres = "POSTGRES"
	ResourceTypeKeyValue = "KEY_VALUE"
)

var resourceTypes = []string{ResourceTypeService, ResourceTypeCronJob, ResourceTypePostgres, ResourceTypeKeyValue}

// timeRanges are the presets a TIME_RANGE filter names: the dashboard's log
// ranges, which hold Render's (Last hour … Last 30 days).
var timeRanges = []string{"30m", "1h", "4h", "12h", "24h", "2d", "7d", "14d", "30d"}

// fieldsByType is which fields each view type's editor offers (the capture's
// table). A metrics view narrows only its resources and its range (w3/m508):
// its charts have no line to filter.
var fieldsByType = map[string][]string{
	TypeApplicationLogs: {FieldEnvironment, FieldResource, FieldResourceType, FieldLevel, FieldInstance, FieldText, FieldTimeRange},
	TypeRequestLogs:     {FieldEnvironment, FieldResource, FieldResourceType, FieldLevel, FieldInstance, FieldMethod, FieldStatusCode, FieldHost, FieldPath, FieldTimeRange},
	TypeMetrics:         {FieldEnvironment, FieldResource, FieldResourceType, FieldTimeRange},
}

// scopeFields select resources; the rest select lines.
var scopeFields = []string{FieldEnvironment, FieldResource, FieldResourceType}

// Bounds on one document, so a stored view cannot grow without limit.
const (
	maxValues     = 50
	maxValueBytes = 512
)

// Filter is one condition row: a field, an operator and the values it takes.
type Filter struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// Document is Render's saved-search filter document.
type Document struct {
	Filters []Filter `json:"filters"`
	Version int      `json:"version"`
}

// Default is the document of Render's default views: no filters.
func Default() Document { return Document{Filters: []Filter{}, Version: Version} }

// ValidType reports whether t is a view type bex serves.
func ValidType(t string) bool { return slices.Contains(Types, t) }

// Parse decodes and validates a filter document for a view of type viewType.
// Every refusal is core.ErrBadRequest naming what it objects to.
func Parse(raw []byte, viewType string) (Document, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("%w: filterDocument is not a filter document: %v", core.ErrBadRequest, err)
	}
	if dec.More() {
		return Document{}, fmt.Errorf("%w: filterDocument is not a filter document: trailing data", core.ErrBadRequest)
	}
	if doc.Filters == nil {
		doc.Filters = []Filter{}
	}
	return doc, doc.Validate(viewType)
}

// ParseValue is Parse for an already-decoded JSON value (a GraphQL JSONObject
// argument).
func ParseValue(v any, viewType string) (Document, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return Document{}, fmt.Errorf("%w: filterDocument is not a filter document: %v", core.ErrBadRequest, err)
	}
	return Parse(raw, viewType)
}

// Validate checks the version, each filter's field against the view type, its
// operator and its values. A field appears at most once.
func (d Document) Validate(viewType string) error {
	allowed, ok := fieldsByType[viewType]
	if !ok {
		return fmt.Errorf("%w: unknown saved search type %q (want one of %v)", core.ErrBadRequest, viewType, Types)
	}
	if d.Version != Version {
		return fmt.Errorf("%w: filterDocument version %d is not supported (want %d)", core.ErrBadRequest, d.Version, Version)
	}
	seen := map[string]bool{}
	for _, f := range d.Filters {
		if !slices.Contains(allowed, f.Field) {
			return fmt.Errorf("%w: filter field %q is not a field of a %s view (want one of %v)", core.ErrBadRequest, f.Field, viewType, allowed)
		}
		if seen[f.Field] {
			return fmt.Errorf("%w: filter field %q appears more than once", core.ErrBadRequest, f.Field)
		}
		seen[f.Field] = true
		if err := f.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (f Filter) validate() error {
	switch f.Operator {
	case OpIs:
	case OpIsNot:
		// The log query has no negation, so a line field's IS_NOT would read
		// as IS. Refuse it by name rather than answer the opposite (ADR010).
		if !slices.Contains(scopeFields, f.Field) || f.Field == FieldEnvironment {
			return fmt.Errorf("%w: filter field %q does not support operator %s", core.ErrBadRequest, f.Field, OpIsNot)
		}
	default:
		return fmt.Errorf("%w: filter field %q has unknown operator %q (want %s or %s)", core.ErrBadRequest, f.Field, f.Operator, OpIs, OpIsNot)
	}
	if len(f.Values) == 0 {
		return fmt.Errorf("%w: filter field %q needs at least one value", core.ErrBadRequest, f.Field)
	}
	if len(f.Values) > maxValues {
		return fmt.Errorf("%w: filter field %q takes at most %d values", core.ErrBadRequest, f.Field, maxValues)
	}
	single := f.Field == FieldEnvironment || f.Field == FieldTimeRange
	if single && len(f.Values) != 1 {
		return fmt.Errorf("%w: filter field %q takes exactly one value", core.ErrBadRequest, f.Field)
	}
	for _, v := range f.Values {
		if v == "" || len(v) > maxValueBytes || !core.ValidText(v) {
			return fmt.Errorf("%w: filter field %q has an empty, invalid or over-long value", core.ErrBadRequest, f.Field)
		}
		if err := f.validateValue(v); err != nil {
			return err
		}
	}
	return nil
}

func (f Filter) validateValue(v string) error {
	ok := true
	switch f.Field {
	case FieldEnvironment:
		kind, known := ids.KindOf(ids.EnvironmentPublicID(v))
		ok = known && kind == ids.Environment
	case FieldResource:
		kind, known := ids.KindOf(v)
		ok = known && (kind == ids.Service || kind == ids.Postgres || kind == ids.KeyValue)
	case FieldResourceType:
		ok = slices.Contains(resourceTypes, v)
	case FieldTimeRange:
		ok = slices.Contains(timeRanges, v)
	}
	if !ok {
		return fmt.Errorf("%w: filter field %q has an invalid value %q", core.ErrBadRequest, f.Field, v)
	}
	return nil
}

// Values returns the values the document's filter on field takes, and whether
// the operator negates them. nil when the document does not filter on field.
func (d Document) Values(field string) (values []string, negated bool) {
	for _, f := range d.Filters {
		if f.Field == field {
			return f.Values, f.Operator == OpIsNot
		}
	}
	return nil, false
}

// Resource is one resource a project view can read: its id, shown name, type
// and environment. Type is the service type (web_service, private_service,
// background_worker, cron_job, static_site) or KindPostgres / KindKeyValue.
type Resource struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	EnvironmentID string `json:"environmentId,omitempty"`
}

// The Resource.Type of a datastore.
const (
	KindPostgres = "postgres"
	KindKeyValue = "key_value"
)

// ResourceType is r's Render Resource Type, "" for a resource no log view
// reads (a static site, which has no application log; a metrics view reads it
// as a Service, see Scope).
func (r Resource) ResourceType() string {
	switch r.Type {
	case KindPostgres:
		return ResourceTypePostgres
	case KindKeyValue:
		return ResourceTypeKeyValue
	case appv1alpha1.TypeCronJob:
		return ResourceTypeCronJob
	case "", appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker:
		return ResourceTypeService
	}
	return ""
}

// servesRequests reports whether r has request lines: a web service, the one
// type behind the public edge (an empty type is a pre-w6/m46 web service).
func (r Resource) servesRequests() bool {
	return r.Type == "" || r.Type == appv1alpha1.TypeWebService
}

// Scope narrows a project's resources to the ones a view of viewType reads:
// every service, cron job, Postgres and Key Value (Request Logs: every web
// service; Service Metrics: static sites too, as Services, for their requests
// and bandwidth), then the document's ENVIRONMENT, RESOURCE and RESOURCE_TYPE
// filters. The order of resources is kept.
func (d Document) Scope(viewType string, resources []Resource) []Resource {
	env, _ := d.Values(FieldEnvironment)
	named, notNamed := d.Values(FieldResource)
	types, notTypes := d.Values(FieldResourceType)
	out := []Resource{}
	for _, r := range resources {
		kind := r.ResourceType()
		if viewType == TypeMetrics && r.Type == appv1alpha1.TypeStaticSite {
			kind = ResourceTypeService
		}
		switch {
		case kind == "":
		case viewType == TypeRequestLogs && !r.servesRequests():
		case len(env) > 0 && ids.EnvironmentPublicID(env[0]) != r.EnvironmentID:
		case len(named) > 0 && slices.Contains(named, r.ID) == notNamed:
		case len(types) > 0 && slices.Contains(types, kind) == notTypes:
		default:
			out = append(out, r)
		}
	}
	return out
}
