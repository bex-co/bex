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
	},
})

// metricsFiltersQueryInputType mirrors Render's MetricsFiltersQueryInput. ownerId
// is accepted (so a Render-shaped client's query validates) but ignored.
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
				"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(metricsQueryInputType)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				resources, q, err := metricsQueryInputFromArgs(p.Args["query"])
				if err != nil {
					return nil, err
				}
				var all []MetricSeries
				for _, res := range resources {
					q.App = res
					series, err := s.Metrics(p.Context, q)
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
				for _, ser := range all {
					out = append(out, metricSeriesResult{MetricSeries: ser, Quantile: quantile, HasQuantile: hasQuantile})
				}
				return out, nil
			},
		},
		"datastoreMetrics": &graphql.Field{
			Type: graphql.NewList(metricSeriesGQLType),
			Args: graphql.FieldConfigArgument{
				"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(datastoreMetricsQueryInputType)},
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
				"resourceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.MonthToDateBandwidth(p.Context, p.Args["resourceId"].(string))
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
				"query": &graphql.ArgumentConfig{Type: graphql.NewNonNull(metricsPathFilterSuggestionsInputType)},
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
			// HOST/PATH are parsed only so Metrics can refuse them (see MetricQuery.Host).
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
	// aggregateBy carries Render's per-chart "Group by" breakdown: an entry
	// naming the label to break the series out by (STATUS_CODE / METHOD, the
	// captured filter-field vocabulary), mapped onto Core's GroupBy exactly
	// like REST's `groupBy` param so the two surfaces stay parity-equal.
	// Other values (Render also sends instance-flavored ones) keep today's
	// behavior — ignored, since bex's request PromQL already always sums
	// across instances.
	for _, v := range stringsFromAny(input["aggregateBy"]) {
		switch strings.ToUpper(v) {
		case "STATUS_CODE":
			q.GroupBy = "status"
		case "METHOD":
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

	return q, nil
}

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
