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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
)

// graphql.go is the GraphQL adapter, matching the operation names Render's
// dashboard uses (captured live): query server(id) / services; mutations
// suspendService(id) / resumeService(id) / restartServer(id); type Service with
// the string `suspended` enum. Every resolver delegates to Core — the schema is
// presentation, the behavior is shared with REST.

// gqlField adapts a typed projection into a GraphQL resolver: it type-asserts
// the source and applies f, resolving nil for foreign sources. One helper for
// every object type (AppView, PostgresView, connection info, logs, API keys).
func gqlField[T any](f func(T) any) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		v, ok := p.Source.(T)
		if !ok {
			return nil, nil
		}
		return f(v), nil
	}
}

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// Render-shaped fields (id is the App name; type is always web_service).
		"id":           &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Name })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Name })},
		"type":         &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return renderWebService })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return suspendedEnum(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.URL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.CreatedAt })},
		// bex-native extras.
		"phase":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Phase })},
		"replicas": &graphql.Field{Type: graphql.Int, Resolve: gqlField(func(a AppView) any { return a.Replicas })},
		"revision": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(a AppView) any { return a.Revision })},
	},
})

func suspendedEnum(b bool) string {
	if b {
		return renderSuspended
	}
	return renderNotSuspended
}

// Render's dashboard GraphQL calls a managed Postgres a "database" (query
// database(id), databaseStatusQuery, databaseCredentialList, ...) — captured
// live — even though its REST noun is "postgres". bex mirrors that split: REST
// /v1/postgres, GraphQL database* (which also matches bex's own Database CRD).
var postgresGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Database",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.ID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Name })},
		"plan":                    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Plan })},
		"version":                 &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Version })},
		"status":                  &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Status })},
		"databaseName":            &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.DatabaseName })},
		"databaseUser":            &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.DatabaseUser })},
		"diskSizeGB":              &graphql.Field{Type: graphql.Int, Resolve: gqlField(func(v PostgresView) any { return v.DiskSizeGB })},
		"highAvailabilityEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlField(func(v PostgresView) any { return v.HighAvailabilityEnabled })},
		"suspended":               &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.Suspended })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.CreatedAt })},
		"externalHost":            &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresView) any { return v.ExternalHost })},
		"public":                  &graphql.Field{Type: graphql.Boolean, Resolve: gqlField(func(v PostgresView) any { return v.Public })},
	},
})

var connectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresConnectionInfo",
	Fields: graphql.Fields{
		"password":                 &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.Password })},
		"internalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.InternalConnectionString })},
		"externalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.ExternalConnectionString })},
		"psqlCommand":              &graphql.Field{Type: graphql.String, Resolve: gqlField(func(v PostgresConnectionInfo) any { return v.PSQLCommand })},
	},
})

// apiKeyGQLType mirrors the REST APIKey object (bex extension; Render's
// dashboard manages keys outside its public schemas). secret is non-empty only
// in the createApiKey payload — list resolves it empty.
var apiKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKey",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.Name })},
		"secret":    &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.Secret })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(k APIKey) any { return k.CreatedAt })},
	},
})

// logGQLType is the GraphQL projection of a LogEntry — a flat row (the REST
// adapter renders the same data as Render's labels array instead). type is
// Render's `app` value; instance comes from the entry's labels.
var logGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "LogEntry",
	Fields: graphql.Fields{
		"timestamp": &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return e.Timestamp })},
		"message":   &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return e.Message })},
		"type":      &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return renderLogTypeApp })},
		"instance":  &graphql.Field{Type: graphql.String, Resolve: gqlField(func(e LogEntry) any { return e.Labels["instance"] })},
	},
})

// logQueryFromArgs maps the GraphQL logs() args onto a Core LogQuery, accepting
// the same `app` alias for the application type as the REST adapter.
func logQueryFromArgs(args map[string]any) LogQuery {
	q := LogQuery{App: args["resource"].(string)}
	if t, ok := args["type"].(string); ok {
		switch t {
		case renderLogTypeApp, LogTypeApplication:
			q.Type = LogTypeApplication
		case LogTypeRequest:
			q.Type = LogTypeRequest
		case LogTypeBuild:
			q.Type = LogTypeBuild
		}
	}
	if s, ok := args["text"].(string); ok {
		q.Search = s
	}
	if n, ok := args["limit"].(int); ok {
		q.Limit = int64(n)
	}
	return q
}

// --- Metrics GraphQL types ---
//
// This shape is NOT Render's public REST metrics-API shape (which the REST
// adapter mirrors, in render.go/rest_metrics.go) — it's Render's *dashboard*
// GraphQL contract, captured live from a real Render dashboard session
// (docs/observability.md): `metrics(query: MetricsQueryInput!)`, filters as an
// array (not a flat resource string), an uppercase `name` enum, and values as
// `{time, value}` (not `{timestamp, value}` — that field name is REST-only).

// renderMetricNames maps Render's uppercase dashboard `name` enum onto bex's
// metric ids. ENRICHED_BANDWIDTH is Render's name (their bandwidth figure
// folds in extra dimensions bex's raw Traefik-scraped bandwidth doesn't); bex
// answers it with the same series as bandwidth — a documented subset, not a
// misrepresentation the field name would otherwise imply.
var renderMetricNames = map[string]string{
	"MEMORY":             MetricMemory,
	"MEMORY_LIMIT":       MetricMemoryLimit,
	"CPU":                MetricCPU,
	"CPU_LIMIT":          MetricCPULimit,
	"INSTANCES":          MetricInstanceCount,
	"HTTP_REQUESTS":      MetricHTTPRequests,
	"HTTP_LATENCY":       MetricHTTPLatency,
	"ENRICHED_BANDWIDTH": MetricBandwidth,
	"BANDWIDTH":          MetricBandwidth,
}

// metricsFilterInputType is Render's {field, values} filter shape — an array of
// these (not bex's older flat `resource` string) lets one query filter by
// RESOURCE, STATUS_CODE, HOST, etc. simultaneously.
var metricsFilterInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetricsFilterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"field":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"values": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
	},
})

var metricsParameterInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetricsParameterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"quantile": &graphql.InputObjectFieldConfig{Type: graphql.Float},
	},
})

// metricsQueryInputType mirrors Render's real MetricsQueryInput, field-for-field
// as captured. bex accepts every field (so a Render-shaped client's queries
// parse) but only honors what it can: aggregateBy/aggregationMethod describe
// behavior bex's request-metric PromQL already does unconditionally (always
// summed/quantiled across instances), so they're accepted, not re-interpreted.
var metricsQueryInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetricsQueryInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"filters":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(metricsFilterInputType)))},
		"start":              &graphql.InputObjectFieldConfig{Type: graphql.String},
		"end":                &graphql.InputObjectFieldConfig{Type: graphql.String},
		"name":               &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"resolution":         &graphql.InputObjectFieldConfig{Type: graphql.Int},
		"parameters":         &graphql.InputObjectFieldConfig{Type: graphql.NewList(metricsParameterInputType)},
		"aggregateBy":        &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		"aggregationMethod":  &graphql.InputObjectFieldConfig{Type: graphql.String},
		"aggregateAllMethod": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// metricValueGQLType is Render's `{time, value}` sample — note the field is
// `time`, not `timestamp` (REST's name for the same data); the two adapters
// mirror their own Render counterparts, not each other.
var metricValueGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricValue",
	Fields: graphql.Fields{
		"time": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricPoint).Timestamp, nil
		}},
		"value": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricPoint).Value, nil
		}},
	},
})

var metricLabelGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricLabel",
	Fields: graphql.Fields{
		"field": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(renderMetricLabel).Field, nil
		}},
		"value": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(renderMetricLabel).Value, nil
		}},
	},
})

var metricSeriesParameterGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricSeriesParameter",
	Fields: graphql.Fields{
		"quantile": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(float64), nil
		}},
	},
})

// metricSeriesResult pairs a Core series with the query's resolved quantile so
// the "parameters" field (Render's per-series quantile echo, present on
// HTTP_LATENCY responses — captured live) can be resolved without threading
// query state through MetricSeries itself, which every other adapter also uses.
type metricSeriesResult struct {
	MetricSeries
	Quantile    float64
	HasQuantile bool
}

// metricSeriesGQLType projects Core's MetricSeries; labels are exposed as
// Render's sorted {field,value} array (reusing the REST mapper) so both
// surfaces agree on label shape even though the top-level query args differ.
var metricSeriesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricSeries",
	Fields: graphql.Fields{
		"unit": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(metricSeriesResult).Unit, nil
		}},
		"labels": &graphql.Field{Type: graphql.NewList(metricLabelGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			return toRenderMetricSeries(p.Source.(metricSeriesResult).MetricSeries).Labels, nil
		}},
		"values": &graphql.Field{Type: graphql.NewList(metricValueGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(metricSeriesResult).Points, nil
		}},
		"parameters": &graphql.Field{Type: graphql.NewList(metricSeriesParameterGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			r := p.Source.(metricSeriesResult)
			if !r.HasQuantile {
				return []float64{}, nil
			}
			return []float64{r.Quantile}, nil
		}},
	},
})

// metricsQueryInputFromArgs maps Render's dashboard `metrics(query:
// MetricsQueryInput!)` shape onto resources + a Core MetricQuery. Unlike REST's
// query string, filters carry more than the resource — STATUS_CODE/HOST/PATH
// arrive the same way RESOURCE does, so a Render-shaped client's filter UI
// (Status Code / Host / Path) maps onto Core's existing MetricQuery fields.
func metricsQueryInputFromArgs(raw any) ([]string, MetricQuery, error) {
	input, ok := raw.(map[string]any)
	if !ok {
		return nil, MetricQuery{}, fmt.Errorf("query is required")
	}

	name, _ := input["name"].(string)
	metric, ok := renderMetricNames[name]
	if !ok {
		return nil, MetricQuery{}, fmt.Errorf("unknown metrics name %q", name)
	}
	q := MetricQuery{Metric: metric}

	var resources []string
	if filters, ok := input["filters"].([]any); ok {
		for _, f := range filters {
			filter, ok := f.(map[string]any)
			if !ok {
				continue
			}
			field, _ := filter["field"].(string)
			values := stringsFromAny(filter["values"])
			switch field {
			case filterFieldResource:
				resources = values
			case "STATUS_CODE":
				if len(values) > 0 {
					q.StatusCode = values[0]
				}
			case "HOST":
				if len(values) > 0 {
					q.Host = values[0]
				}
			case "PATH":
				if len(values) > 0 {
					q.Path = values[0]
				}
			}
		}
	}
	if len(resources) == 0 {
		return nil, MetricQuery{}, fmt.Errorf("filters must include a RESOURCE entry")
	}

	if s, ok := input["start"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			q.Start = t
		}
	}
	if e, ok := input["end"].(string); ok && e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			q.End = t
		}
	}
	if n, ok := input["resolution"].(int); ok {
		q.Resolution = time.Duration(n) * time.Second
	}
	if params, ok := input["parameters"].([]any); ok {
		for _, pr := range params {
			if m, ok := pr.(map[string]any); ok {
				if ql, ok := m["quantile"].(float64); ok {
					q.Quantile = ql
				}
			}
		}
	}
	if method, _ := input["aggregateAllMethod"].(string); method == "MAX" {
		q.AggregateMax = true
	}

	return resources, q, nil
}

func stringsFromAny(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// --- monthToDateBandwidth (Render's "Usage this month" bandwidth footer) ---

var monthToDateBandwidthGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MonthToDateBandwidth",
	Fields: graphql.Fields{
		"egressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).EgressBandwidthMB, nil
		}},
		"httpEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).HTTPEgressBandwidthMB, nil
		}},
		// bex has no metering for these egress paths (no NAT gateway, no private
		// link, no separate websocket accounting) — always 0, a documented
		// subset of Render's real figure, never a fabricated total.
		"natEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).NATEgressBandwidthMB, nil
		}},
		"privateLinkEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).PrivateLinkEgressBandwidthMB, nil
		}},
		"websocketEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).WebsocketEgressBandwidthMB, nil
		}},
	},
})

// --- metricsFilters / metricsPathFilterSuggestions (Status Code/Host/Path
// filter-dropdown population) ---

// metricsFiltersQueryInputType mirrors Render's MetricsFiltersQueryInput,
// captured live from the dashboard's filter-population query. `ownerId` is
// accepted (so a Render-shaped client's query still validates) but ignored —
// bex-api is single-tenant and has no owner-scoping concept.
var metricsFiltersQueryInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetricsFiltersQueryInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"filters":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(metricsFilterInputType)))},
		"start":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"end":           &graphql.InputObjectFieldConfig{Type: graphql.String},
		"type":          &graphql.InputObjectFieldConfig{Type: graphql.String}, // APPLICATION | HTTP
		"outputFilters": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
		"ownerId":       &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var metricsFilterValuesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricsFilterValues",
	Fields: graphql.Fields{
		"field": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricsFilterValues).Field, nil
		}},
		"values": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MetricsFilterValues).Values, nil
		}},
	},
})

// metricsFiltersResult wraps Core's []MetricsFilterValues under a "values"
// field — Render's response is {values: [{field, values}]}, not a bare list.
type metricsFiltersResult struct {
	Values []MetricsFilterValues
}

var metricsFiltersResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricsFiltersResult",
	Fields: graphql.Fields{
		"values": &graphql.Field{Type: graphql.NewList(metricsFilterValuesGQLType), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(metricsFiltersResult).Values, nil
		}},
	},
})

func metricsFiltersQueryFromArgs(raw any) (MetricsFiltersQuery, error) {
	input, ok := raw.(map[string]any)
	if !ok {
		return MetricsFiltersQuery{}, fmt.Errorf("query is required")
	}
	var app string
	if filters, ok := input["filters"].([]any); ok {
		for _, f := range filters {
			filter, ok := f.(map[string]any)
			if !ok {
				continue
			}
			if field, _ := filter["field"].(string); field == filterFieldResource {
				if values := stringsFromAny(filter["values"]); len(values) > 0 {
					app = values[0]
				}
			}
		}
	}
	if app == "" {
		return MetricsFiltersQuery{}, fmt.Errorf("filters must include a RESOURCE entry")
	}
	return MetricsFiltersQuery{App: app, OutputFilters: stringsFromAny(input["outputFilters"])}, nil
}

// metricsPathFilterSuggestionsInputType mirrors Render's
// MetricsPathFilterSuggestionsInput (captured live). bex always answers with
// no suggestions — Traefik's service-level metrics (the only request-metric
// source bex has) carry no path label, so there's no real path data to offer;
// an honest empty list, not a guessed one.
var metricsPathFilterSuggestionsInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MetricsPathFilterSuggestionsInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"serviceIDs": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
		"paths":      &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
	},
})

type metricsPathFilterSuggestionsResult struct {
	Paths []string
}

var metricsPathFilterSuggestionsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MetricsPathFilterSuggestions",
	Fields: graphql.Fields{
		"paths": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(metricsPathFilterSuggestionsResult).Paths, nil
		}},
	},
})

// newSchema builds the schema once; the Core is injected per-request via context.
func newSchema() (graphql.Schema, error) {
	idArg := graphql.FieldConfigArgument{
		"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
	}
	verb := func(fn func(*Core, context.Context, string) (AppView, error)) graphql.FieldResolveFn {
		return func(p graphql.ResolveParams) (any, error) {
			return fn(coreFrom(p.Context), p.Context, p.Args["id"].(string))
		}
	}

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"services": &graphql.Field{
					Type:    graphql.NewList(serviceGQLType),
					Resolve: func(p graphql.ResolveParams) (any, error) { return coreFrom(p.Context).List(p.Context) },
				},
				"server": &graphql.Field{ // Render's dashboard query name
					Type: serviceGQLType,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).Get(p.Context, p.Args["id"].(string))
					},
				},
				"databases": &graphql.Field{ // list; Render lists via env, bex offers a top-level list
					Type:    graphql.NewList(postgresGQLType),
					Resolve: func(p graphql.ResolveParams) (any, error) { return coreFrom(p.Context).ListPostgres(p.Context) },
				},
				"database": &graphql.Field{ // Render's dashboard query name
					Type: postgresGQLType,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).GetPostgres(p.Context, p.Args["id"].(string))
					},
				},
				"databaseConnectionInfo": &graphql.Field{
					Type: connectionInfoGQLType,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).PostgresConnectionInfo(p.Context, p.Args["id"].(string))
					},
				},
				"apiKeys": &graphql.Field{
					Type:    graphql.NewList(apiKeyGQLType),
					Resolve: func(p graphql.ResolveParams) (any, error) { return coreFrom(p.Context).ListAPIKeys(p.Context) },
				},
				"logs": &graphql.Field{
					Type: graphql.NewList(logGQLType),
					Args: graphql.FieldConfigArgument{
						"resource": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
						"type":     &graphql.ArgumentConfig{Type: graphql.String},
						"text":     &graphql.ArgumentConfig{Type: graphql.String},
						"limit":    &graphql.ArgumentConfig{Type: graphql.Int},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).QueryLogs(p.Context, logQueryFromArgs(p.Args))
					},
				},
				"metrics": &graphql.Field{
					Type: graphql.NewList(metricSeriesGQLType),
					Args: graphql.FieldConfigArgument{
						"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(metricsQueryInputType)},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						resources, q, err := metricsQueryInputFromArgs(p.Args["query"])
						if err != nil {
							return nil, err
						}
						core := coreFrom(p.Context)
						var all []MetricSeries
						for _, res := range resources {
							q.App = res
							series, err := core.Metrics(p.Context, q)
							if err != nil {
								return nil, err
							}
							all = append(all, series...)
						}
						quantile := q.Quantile
						if quantile <= 0 {
							quantile = defaultQuantile
						}
						hasQuantile := q.Metric == MetricHTTPLatency
						out := make([]metricSeriesResult, 0, len(all))
						for _, s := range all {
							out = append(out, metricSeriesResult{MetricSeries: s, Quantile: quantile, HasQuantile: hasQuantile})
						}
						return out, nil
					},
				},
				"monthToDateBandwidth": &graphql.Field{
					Type: monthToDateBandwidthGQLType,
					Args: graphql.FieldConfigArgument{
						"resourceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).MonthToDateBandwidth(p.Context, p.Args["resourceId"].(string))
					},
				},
				"metricsFilters": &graphql.Field{
					Type: metricsFiltersResultGQLType,
					Args: graphql.FieldConfigArgument{
						"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(metricsFiltersQueryInputType)},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						q, err := metricsFiltersQueryFromArgs(p.Args["query"])
						if err != nil {
							return nil, err
						}
						values, err := coreFrom(p.Context).MetricsFilters(p.Context, q)
						if err != nil {
							return nil, err
						}
						return metricsFiltersResult{Values: values}, nil
					},
				},
				"metricsPathFilterSuggestions": &graphql.Field{
					Type: metricsPathFilterSuggestionsGQLType,
					Args: graphql.FieldConfigArgument{
						"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(metricsPathFilterSuggestionsInputType)},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return metricsPathFilterSuggestionsResult{Paths: []string{}}, nil
					},
				},
			},
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "Mutation",
			Fields: graphql.Fields{
				"suspendService": &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Suspend)},
				"resumeService":  &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Resume)},
				"restartServer":  &graphql.Field{Type: serviceGQLType, Args: idArg, Resolve: verb((*Core).Restart)},
				"createDatabase": &graphql.Field{
					Type: postgresGQLType,
					Args: graphql.FieldConfigArgument{
						"name":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
						"plan":       &graphql.ArgumentConfig{Type: graphql.String},
						"version":    &graphql.ArgumentConfig{Type: graphql.String},
						"diskSizeGB": &graphql.ArgumentConfig{Type: graphql.Int},
						"public":     &graphql.ArgumentConfig{Type: graphql.Boolean},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						req := CreatePostgresRequest{Name: p.Args["name"].(string)}
						if v, ok := p.Args["plan"].(string); ok {
							req.Plan = v
						}
						if v, ok := p.Args["version"].(string); ok {
							req.Version = v
						}
						if v, ok := p.Args["diskSizeGB"].(int); ok {
							req.DiskSizeGB = int32(v)
						}
						if v, ok := p.Args["public"].(bool); ok {
							req.Public = v
						}
						return coreFrom(p.Context).CreatePostgres(p.Context, req)
					},
				},
				"deleteDatabase": &graphql.Field{
					Type: graphql.Boolean,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						err := coreFrom(p.Context).DeletePostgres(p.Context, p.Args["id"].(string))
						return err == nil, err
					},
				},
				"createApiKey": &graphql.Field{
					Type: apiKeyGQLType,
					Args: graphql.FieldConfigArgument{
						"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return coreFrom(p.Context).CreateAPIKey(p.Context, p.Args["name"].(string))
					},
				},
				"revokeApiKey": &graphql.Field{
					Type: graphql.Boolean,
					Args: idArg,
					Resolve: func(p graphql.ResolveParams) (any, error) {
						err := coreFrom(p.Context).RevokeAPIKey(p.Context, p.Args["id"].(string))
						return err == nil, err
					},
				},
			},
		}),
	})
}

type coreCtxKey struct{}

func coreFrom(ctx context.Context) *Core { return ctx.Value(coreCtxKey{}).(*Core) }

// graphqlHandler serves POST /graphql, injecting the Core per request so the
// compiled schema is reused.
func (s *Server) graphqlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query         string         `json:"query"`
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		result := graphql.Do(graphql.Params{
			Schema:         s.schema,
			RequestString:  body.Query,
			OperationName:  body.OperationName,
			VariableValues: body.Variables,
			Context:        context.WithValue(r.Context(), coreCtxKey{}, s.Core),
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
}
