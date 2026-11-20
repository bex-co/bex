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
	"unicode/utf8"

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

// Render's filter operators: CONTAINS is Text Search's, and only its.
const (
	OpContains = "CONTAINS"
	OpIs       = "IS"
	OpIsNot    = "IS_NOT"
)

var operators = []string{OpContains, OpIs, OpIsNot}

// The filter fields, in Render's saved-search field enum's spellings (its
// dashboard bundle, 2026-11-19, ADR010). The time range is no filter row: it
// is the document's top-level timeRange.relative.
const (
	FieldEnvironment  = "ENVIRONMENT"
	FieldResource     = "RESOURCE"
	FieldResourceType = "RESOURCE_TYPE"
	FieldLevel        = "LEVEL"
	FieldInstance     = "INSTANCE"
	FieldTextSearch   = "TEXT_SEARCH"
	FieldMethod       = "METHOD"
	FieldStatusCode   = "STATUS_CODE"
	FieldHost         = "HOST"
	FieldPath         = "PATH"
)

// bex's own spellings before w3/083, in every view stored until migration
// 0316 rewrote them: Text Search as TEXT with IS, and the time range as a
// TIME_RANGE row taking IS and one preset. Parse still reads them, into
// Render's shape, so a client or a link that kept one goes on working.
const (
	legacyFieldText      = "TEXT"
	legacyFieldTimeRange = "TIME_RANGE"
)

// Render's Resource Type values: Services, Cron jobs, Postgres, Key Value.
const (
	ResourceTypeService  = "SERVICE"
	ResourceTypeCronJob  = "CRON_JOB"
	ResourceTypePostgres = "POSTGRES"
	ResourceTypeKeyValue = "KEY_VALUE"
)

var resourceTypes = []string{ResourceTypeService, ResourceTypeCronJob, ResourceTypePostgres, ResourceTypeKeyValue}

// timeRanges are the presets a view's timeRange.relative names: the
// dashboard's log ranges, which hold Render's (Last hour … Last 30 days).
var timeRanges = []string{"30m", "1h", "4h", "12h", "24h", "2d", "7d", "14d", "30d"}

// fieldsByType is which filter fields each view type's editor offers (the
// capture's table, less Time Range, which every type takes as timeRange). A
// metrics view narrows only its resources (w3/m508): its charts have no line
// to filter.
var fieldsByType = map[string][]string{
	TypeApplicationLogs: {FieldEnvironment, FieldResource, FieldResourceType, FieldLevel, FieldInstance, FieldTextSearch},
	TypeRequestLogs:     {FieldEnvironment, FieldResource, FieldResourceType, FieldLevel, FieldInstance, FieldMethod, FieldStatusCode, FieldHost, FieldPath},
	TypeMetrics:         {FieldEnvironment, FieldResource, FieldResourceType},
}

// fieldOperators are the operators a field takes, as Render's Edit view
// offers them (w3/m523): Environment `is` one value, Text Search `contains`,
// and every other field `is one of` and `is not one of`.
func fieldOperators(field string) []string {
	switch field {
	case FieldEnvironment:
		return []string{OpIs}
	case FieldTextSearch:
		return []string{OpContains}
	}
	return []string{OpIs, OpIsNot}
}

// Bounds on one document, so a stored view cannot grow without limit.
const (
	maxValues       = 50
	maxValueBytes   = 512
	maxDisplayBytes = 4096
)

// Filter is one condition row: a field, an operator and the values it takes.
type Filter struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

// Document is Render's saved-search filter document, `{version, filters,
// timeRange: {relative}, display}` (its SavedViewFiltersModal, w3/083).
// Display is a metrics view's display options (Render's aggregationMethod,
// requestGroupBy, groupBy, rootTasksOnly), kept as written: bex reads none of
// it, and a Render document carrying it round-trips.
type Document struct {
	Filters   []Filter        `json:"filters"`
	Version   int             `json:"version"`
	TimeRange *TimeRange      `json:"timeRange,omitempty"`
	Display   json.RawMessage `json:"display,omitempty"`
}

// TimeRange is a view's default range: one preset, the range its page reads
// when the URL names none.
type TimeRange struct {
	Relative string `json:"relative"`
}

// Default is the document of Render's default views: no filters.
func Default() Document { return Document{Filters: []Filter{}, Version: Version} }

// ValidType reports whether t is a view type bex serves.
func ValidType(t string) bool { return slices.Contains(Types, t) }

// Parse decodes and validates a filter document for a view of type viewType,
// in Render's shape or bex's pre-w3/083 one, and returns it in Render's.
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
	// An empty timeRange sets no range, and a null display is none.
	if doc.TimeRange != nil && doc.TimeRange.Relative == "" {
		doc.TimeRange = nil
	}
	if bytes.Equal(doc.Display, []byte("null")) {
		doc.Display = nil
	}
	if err := doc.upgrade(); err != nil {
		return Document{}, err
	}
	return doc, doc.Validate(viewType)
}

// upgrade rewrites bex's pre-w3/083 spellings into Render's: a TEXT row is
// TEXT_SEARCH (its IS, CONTAINS), and a TIME_RANGE row, `is` one preset, is
// the top-level timeRange.
func (d *Document) upgrade() error {
	filters := d.Filters[:0:0]
	for _, f := range d.Filters {
		switch f.Field {
		case legacyFieldText:
			f.Field = FieldTextSearch
			if f.Operator == OpIs {
				f.Operator = OpContains
			}
		case legacyFieldTimeRange:
			switch {
			case d.TimeRange != nil:
				return fmt.Errorf("%w: filter field %q appears more than once, or beside timeRange", core.ErrBadRequest, f.Field)
			case f.Operator != OpIs:
				return fmt.Errorf("%w: filter field %q does not support operator %s", core.ErrBadRequest, f.Field, f.Operator)
			case len(f.Values) != 1:
				return fmt.Errorf("%w: filter field %q takes exactly one value", core.ErrBadRequest, f.Field)
			}
			d.TimeRange = &TimeRange{Relative: f.Values[0]}
			continue
		}
		filters = append(filters, f)
	}
	d.Filters = filters
	return nil
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
// operator and its values, the time range's preset and the display options'
// bounds. A field appears at most once.
func (d Document) Validate(viewType string) error {
	allowed, ok := fieldsByType[viewType]
	if !ok {
		return fmt.Errorf("%w: unknown saved search type %q (want one of %v)", core.ErrBadRequest, viewType, Types)
	}
	if d.Version != Version {
		return fmt.Errorf("%w: filterDocument version %d is not supported (want %d)", core.ErrBadRequest, d.Version, Version)
	}
	if d.TimeRange != nil && !slices.Contains(timeRanges, d.TimeRange.Relative) {
		return fmt.Errorf("%w: timeRange.relative has an invalid value %q (want one of %v)", core.ErrBadRequest, d.TimeRange.Relative, timeRanges)
	}
	if d.Display != nil && !validDisplay(d.Display) {
		return fmt.Errorf("%w: display must be a JSON object of valid text, at most %d bytes", core.ErrBadRequest, maxDisplayBytes)
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
	if !slices.Contains(operators, f.Operator) {
		return fmt.Errorf("%w: filter field %q has unknown operator %q (want one of %v)", core.ErrBadRequest, f.Field, f.Operator, operators)
	}
	if !slices.Contains(fieldOperators(f.Field), f.Operator) {
		return fmt.Errorf("%w: filter field %q does not support operator %s", core.ErrBadRequest, f.Field, f.Operator)
	}
	if len(f.Values) == 0 {
		return fmt.Errorf("%w: filter field %q needs at least one value", core.ErrBadRequest, f.Field)
	}
	if len(f.Values) > maxValues {
		return fmt.Errorf("%w: filter field %q takes at most %d values", core.ErrBadRequest, f.Field, maxValues)
	}
	if f.Field == FieldEnvironment && len(f.Values) != 1 {
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
	}
	if !ok {
		return fmt.Errorf("%w: filter field %q has an invalid value %q", core.ErrBadRequest, f.Field, v)
	}
	return nil
}

// validDisplay reports whether raw is a JSON object of valid text within
// maxDisplayBytes: anything Render keeps there, and nothing Postgres refuses.
func validDisplay(raw json.RawMessage) bool {
	if len(raw) > maxDisplayBytes || !utf8.Valid(raw) {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return false
	}
	obj, ok := v.(map[string]any)
	return ok && validJSONText(obj)
}

// validJSONText reports whether every key and string in v is core.ValidText.
func validJSONText(v any) bool {
	switch v := v.(type) {
	case string:
		return core.ValidText(v)
	case []any:
		for _, e := range v {
			if !validJSONText(e) {
				return false
			}
		}
	case map[string]any:
		for k, e := range v {
			if !core.ValidText(k) || !validJSONText(e) {
				return false
			}
		}
	}
	return true
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
