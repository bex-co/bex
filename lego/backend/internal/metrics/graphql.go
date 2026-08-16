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

package metrics

import (
	"fmt"
	"strings"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the metrics GraphQL fragment. This shape is Render's *dashboard*
// GraphQL contract, captured live (docs/ADR010-observability.md): `metrics(query:
// MetricsQueryInput!)`, filters as an array, an uppercase `name` enum, and values
// as `{time, value}` (not `{timestamp, value}` — that field name is REST-only).

// renderMetricNames maps Render's uppercase dashboard `name` enum onto bex's
// metric ids. ENRICHED_BANDWIDTH is Render's name; bex answers it with the same
// series as bandwidth — a documented subset.
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
	// bex extensions (w3/m10): App-scoped autoscale-target values.
	"CPU_TARGET":    MetricCPUTarget,
	"MEMORY_TARGET": MetricMemoryTarget,
}

// datastoreMetricNames is the datastoreMetrics query's `name` enum (w3/m10) —
// the Database/KeyValue-scoped sibling of renderMetricNames. All four are bex
// extensions; Render's metrics API has no datastore-scoped series.
var datastoreMetricNames = map[string]string{
	"DISK":            MetricDisk,
	"DISK_CAPACITY":   MetricDiskCapacity,
	"DB_CONNECTIONS":  MetricDBConnections,
	"REPLICATION_LAG": MetricReplicationLag,
	"MEMORY":          MetricKVMemory,      // key-value only
	"CONNECTIONS":     MetricKVConnections, // key-value only
}

// datastoreMetricsQueryInputType is datastoreMetrics' query input — the
// Database/KeyValue-scoped sibling of metricsQueryInputType. It names one
// resource directly (kind + resource), not a RESOURCE filter array: a
// datastore metric always targets exactly one instance, mirroring
// GET /v1/postgres/{id} and GET /v1/key-value/{id} rather than the app
// metrics' multi-resource filter shape.
var datastoreMetricsQueryInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "DatastoreMetricsQueryInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"kind":       &graphql.InputObjectFieldConfig{Type: graphql.String}, // DATABASE | KEYVALUE; default DATABASE
		"resource":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"name":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"start":      &graphql.InputObjectFieldConfig{Type: graphql.String},
		"end":        &graphql.InputObjectFieldConfig{Type: graphql.String},
		"resolution": &graphql.InputObjectFieldConfig{Type: graphql.Int},
	},
})

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

// metricsQueryInputType mirrors Render's real MetricsQueryInput, field-for-field.
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
// `time`, not `timestamp` (REST's name for the same data).
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

// metricSeriesResult pairs a series with the query's resolved quantile so the
// "parameters" field (Render's per-series quantile echo on HTTP_LATENCY) can be
// resolved without threading query state through MetricSeries itself.
type metricSeriesResult struct {
	MetricSeries
	Quantile    float64
	HasQuantile bool
}

// metricSeriesGQLType projects MetricSeries; labels are exposed as Render's
// sorted {field,value} array (reusing the REST mapper).
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

var monthToDateBandwidthGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MonthToDateBandwidth",
	Fields: graphql.Fields{
		"egressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).EgressBandwidthMB, nil
		}},
		"httpEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).HTTPEgressBandwidthMB, nil
		}},
		"natEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).NATEgressBandwidthMB, nil
		}},
		"privateLinkEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).PrivateLinkEgressBandwidthMB, nil
		}},
		"websocketEgressBandwidthMB": &graphql.Field{Type: graphql.Float, Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).WebsocketEgressBandwidthMB, nil
		}},
		// bex extension (w1/m50, ADR023 § Observability reads vs billing
		// reads): egress sources whose health product failed inside the month
		// window. The MB figures still include what those sources recorded.
		"degradedSources": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: func(p graphql.ResolveParams) (any, error) {
			return p.Source.(MonthToDateBandwidth).DegradedSources, nil
		}},
	},
})

// metricsFiltersQueryInputType mirrors Render's MetricsFiltersQueryInput. ownerId
// is accepted (so a Render-shaped client's query validates) but silently ignored:
// the metric query is already scoped to the App's own workspace via AuthorizeApp,
// so ownerId carries no additional filtering power and validating it would
// duplicate that seam. Recorded decision: ignore (w3/m18).
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

// metricsFiltersResult wraps []MetricsFilterValues under a "values" field —
// Render's response is {values: [{field, values}]}, not a bare list.
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

// metricsPathFilterSuggestionsInputType mirrors Render's input; bex always
// answers with no suggestions (Traefik service-level metrics carry no path label).
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

// GraphQLQuery returns the metrics dashboard queries for the composition root.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"metrics": &graphql.Field{
			Type: graphql.NewList(metricSeriesGQLType),
			Args: graphql.FieldConfigArgument{
				"query": gqlutil.ReqArg(metricsQueryInputType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				resources, q, err := metricsQueryInputFromArgs(p.Args["query"])
				if err != nil {
					return nil, err
				}
				if err := checkFanOut(len(resources), 1, latencyFan(q.Metric, q.Quantiles)); err != nil {
					return nil, err
				}
				var out []metricSeriesResult
				for _, res := range resources {
					q.App = res
					// MetricsWithQuantiles echoes the (single or per-quantile)
					// percentile per series, driving the "parameters" field for
					// HTTP_LATENCY's percentile "All" overlay (w5/m56).
					qs, err := s.MetricsWithQuantiles(p.Context, q)
					if err != nil {
						return nil, err
					}
					for _, r := range qs {
						out = append(out, metricSeriesResult{MetricSeries: r.MetricSeries, Quantile: r.Quantile, HasQuantile: r.HasQuantile})
					}
				}
				return out, nil
			},
		},
		"datastoreMetrics": &graphql.Field{
			Type: graphql.NewList(metricSeriesGQLType),
			Args: graphql.FieldConfigArgument{
				"query": gqlutil.ReqArg(datastoreMetricsQueryInputType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				q, err := datastoreMetricsQueryInputFromArgs(p.Args["query"])
				if err != nil {
					return nil, err
				}
				series, err := s.DatastoreMetrics(p.Context, q)
				if err != nil {
					return nil, err
				}
				out := make([]metricSeriesResult, 0, len(series))
				for _, ser := range series {
					out = append(out, metricSeriesResult{MetricSeries: ser})
				}
				return out, nil
			},
		},
		"monthToDateBandwidth": &graphql.Field{
			Type: monthToDateBandwidthGQLType,
			Args: graphql.FieldConfigArgument{
				"resourceId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.MonthToDateBandwidth(p.Context, p.Args["resourceId"].(string))
			},
		},
		"metricsFilters": &graphql.Field{
			Type: metricsFiltersResultGQLType,
			Args: graphql.FieldConfigArgument{
				"query": gqlutil.ReqArg(metricsFiltersQueryInputType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				q, err := metricsFiltersQueryFromArgs(p.Args["query"])
				if err != nil {
					return nil, err
				}
				values, err := s.MetricsFilters(p.Context, q)
				if err != nil {
					return nil, err
				}
				return metricsFiltersResult{Values: values}, nil
			},
		},
		"metricsPathFilterSuggestions": &graphql.Field{
			Type: metricsPathFilterSuggestionsGQLType,
			Args: graphql.FieldConfigArgument{
				"query": gqlutil.ReqArg(metricsPathFilterSuggestionsInputType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return metricsPathFilterSuggestionsResult{Paths: []string{}}, nil
			},
		},
	}
}

// metricsQueryInputFromArgs maps Render's dashboard `metrics(query:
// MetricsQueryInput!)` shape onto resources + a MetricQuery.
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

	filters := metricsFilterValues(input)
	q.StatusCode = firstValue(filters[filterFieldStatusCode])
	// HOST/PATH are parsed only so Metrics can refuse them (see MetricQuery.Host).
	q.Host = firstValue(filters[filterFieldHost])
	q.Path = firstValue(filters[filterFieldPath])

	resources := filters[filterFieldResource]
	if len(resources) == 0 {
		return nil, MetricQuery{}, fmt.Errorf("filters must include a RESOURCE entry")
	}

	var err error
	if q.Start, q.End, q.Resolution, err = windowFromInput(input); err != nil {
		return nil, MetricQuery{}, err
	}
	// Render's `parameters` is a list; each entry names a quantile. A single entry
	// is the ordinary percentile pick, several entries are the percentile "All"
	// overlay (w5/m56) — collect them all and let MetricsWithQuantiles fan out.
	if params, ok := input["parameters"].([]any); ok {
		for _, pr := range params {
			if m, ok := pr.(map[string]any); ok {
				if ql, ok := m["quantile"].(float64); ok {
					q.Quantiles = append(q.Quantiles, ql)
				}
			}
		}
	}
	if method, _ := input["aggregateAllMethod"].(string); method == "MAX" {
		q.AggregateMax = true
	}
	// aggregateBy carries Render's per-chart "Group by" breakdown: an entry
	// naming the label to break the series out by (STATUS_CODE / METHOD, the
	// captured filter-field vocabulary), mapped onto Core's GroupBy exactly
	// like REST's `groupBy` param so the two surfaces stay parity-equal.
	// Instance-flavored values (Render also sends SERVICE_INSTANCE_ID) are
	// silently ignored: bex's request PromQL already sums across instances at
	// the Traefik service level, and per-instance request breakdowns would
	// require per-pod Traefik metrics that aren't scraped. Recorded decision:
	// instance aggregateBy is a no-op (w3/m18).
	for _, v := range gqlutil.StringList(input["aggregateBy"]) {
		switch strings.ToUpper(v) {
		case filterFieldStatusCode:
			q.GroupBy = "status"
		case filterFieldMethod:
			q.GroupBy = "method"
		}
	}

	return resources, q, nil
}

// datastoreMetricsQueryInputFromArgs maps datastoreMetrics' input onto a
// DatastoreMetricQuery — the sibling of metricsQueryInputFromArgs.
func datastoreMetricsQueryInputFromArgs(raw any) (DatastoreMetricQuery, error) {
	input, ok := raw.(map[string]any)
	if !ok {
		return DatastoreMetricQuery{}, fmt.Errorf("query is required")
	}

	resource, _ := input["resource"].(string)
	if resource == "" {
		return DatastoreMetricQuery{}, fmt.Errorf("resource is required")
	}
	name, _ := input["name"].(string)
	metric, ok := datastoreMetricNames[strings.ToUpper(name)]
	if !ok {
		return DatastoreMetricQuery{}, fmt.Errorf("unknown datastore metrics name %q", name)
	}
	rawKind, _ := input["kind"].(string)
	kind := strings.ToLower(strings.TrimSpace(rawKind))
	if kind == "" {
		kind = DatastoreDatabase
	}
	q := DatastoreMetricQuery{Kind: kind, Resource: resource, Metric: metric}

	var err error
	if q.Start, q.End, q.Resolution, err = windowFromInput(input); err != nil {
		return DatastoreMetricQuery{}, err
	}

	return q, nil
}

// metricsFilterValues indexes Render's `filters: [{field, values}]` array by
// field name, shared by both GraphQL query inputs. Entries that aren't objects,
// and entries naming a field bex doesn't honor, are ignored — Render's clients
// send the full vocabulary regardless of which metric is being read.
func metricsFilterValues(input map[string]any) map[string][]string {
	filters, _ := input["filters"].([]any)
	values := make(map[string][]string, len(filters))
	for _, f := range filters {
		filter, ok := f.(map[string]any)
		if !ok {
			continue
		}
		field, _ := filter["field"].(string)
		values[field] = gqlutil.StringList(filter["values"])
	}
	return values
}

// firstValue takes the single value out of a filter that Render models as a
// list but bex honors as a scalar.
func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// windowFromInput parses the start/end/resolution window shared by both
// GraphQL query inputs. Malformed timestamps error (core.ParseTime) rather
// than silently falling back to the default window, which would bypass the
// MaxQueryHours cap on the range the caller actually asked for.
func windowFromInput(input map[string]any) (start, end time.Time, resolution time.Duration, err error) {
	rawStart, _ := input["start"].(string)
	if start, err = core.ParseTime("start", rawStart); err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	rawEnd, _ := input["end"].(string)
	if end, err = core.ParseTime("end", rawEnd); err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	if n, ok := input["resolution"].(int); ok {
		resolution = time.Duration(n) * time.Second
	}
	return start, end, resolution, nil
}

func metricsFiltersQueryFromArgs(raw any) (MetricsFiltersQuery, error) {
	input, ok := raw.(map[string]any)
	if !ok {
		return MetricsFiltersQuery{}, fmt.Errorf("query is required")
	}
	app := firstValue(metricsFilterValues(input)[filterFieldResource])
	if app == "" {
		return MetricsFiltersQuery{}, fmt.Errorf("filters must include a RESOURCE entry")
	}
	return MetricsFiltersQuery{App: app, OutputFilters: gqlutil.StringList(input["outputFilters"])}, nil
}
