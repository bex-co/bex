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

import "sort"

// render.go maps MetricSeries onto Render's metrics-API array shape: each series
// carries a `labels` array ({field, value}), a `unit`, and `values`
// ({timestamp, value}). The REST adapter renders this; the GraphQL adapter
// mirrors its own Render dashboard shape ({time, value}) from the same data.

// renderMetricLabel is Render's metric label ({field, value}); note logs use
// {name, value} but metrics use {field, value} in Render's spec.
type renderMetricLabel struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type renderMetricValue struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// renderMetricSeries is one element of a Render metrics response array.
type renderMetricSeries struct {
	Labels []renderMetricLabel `json:"labels"`
	Unit   string              `json:"unit"`
	Values []renderMetricValue `json:"values"`
}

func toRenderMetricSeries(s MetricSeries) renderMetricSeries {
	// Deterministic label order (sorted by field) so output is stable.
	fields := make([]string, 0, len(s.Labels))
	for k := range s.Labels {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	labels := make([]renderMetricLabel, 0, len(fields))
	for _, f := range fields {
		labels = append(labels, renderMetricLabel{Field: f, Value: s.Labels[f]})
	}
	values := make([]renderMetricValue, 0, len(s.Points))
	for _, p := range s.Points {
		values = append(values, renderMetricValue(p))
	}
	return renderMetricSeries{Labels: labels, Unit: s.Unit, Values: values}
}

// A per-status breakdown's label. The sources label it code, and REST and MCP
// rename it statusCode, Render's aggregateBy value and filters/http field
// (w3/080). GraphQL names it STATUS_CODE with its other labels
// (graphqlLabelFields).
const (
	sourceStatusCodeLabel = "code"
	restStatusCodeLabel   = "statusCode"
)

// restStatusCode renames a per-status breakdown's source label to REST's
// statusCode.
func restStatusCode(s MetricSeries, groupBy string) MetricSeries {
	if groupBy != groupByStatus {
		return s
	}
	if code, ok := s.Labels[sourceStatusCodeLabel]; ok {
		delete(s.Labels, sourceStatusCodeLabel)
		s.SetLabel(restStatusCodeLabel, code)
	}
	return s
}

// graphqlLabelFields is how GraphQL names each label Render's MetricQueryFilter
// enum has a value for. Render's GraphQL MetricResultLabel.field is that enum,
// and its dashboard finds a series' resource, instance, host and status code by
// it, as in labels.find(l => l.field === "INSTANCE") (w3/080, w3/087;
// docs/render-artifacts/metrics-page.md § GraphQL label names). REST and MCP
// keep the lowercase names, as Render's REST does. The labels not listed
// (quantile, aggregate, direction, degraded_sources) are Bex extensions with no
// value in the enum and keep their names on every surface (ADR010 § Series
// label names).
var graphqlLabelFields = map[string]string{
	LabelResource:         filterFieldResource,
	"instance":            filterFieldInstance,
	groupByHost:           filterFieldHost,
	groupByMethod:         filterFieldMethod,
	sourceStatusCodeLabel: filterFieldStatusCode,
}

// toGraphQLLabels is a series' labels as GraphQL answers them: renamed by
// graphqlLabelFields and sorted by field, so output is stable.
func toGraphQLLabels(s MetricSeries) []renderMetricLabel {
	labels := make([]renderMetricLabel, 0, len(s.Labels))
	for field, value := range s.Labels {
		if name, ok := graphqlLabelFields[field]; ok {
			field = name
		}
		labels = append(labels, renderMetricLabel{Field: field, Value: value})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Field < labels[j].Field })
	return labels
}

// toRenderMetrics maps series onto Render's metrics response array. Always a
// non-nil slice so an empty result serializes as [] (Render's shape), not null.
func toRenderMetrics(series []MetricSeries) []renderMetricSeries {
	out := make([]renderMetricSeries, 0, len(series))
	for _, s := range series {
		out = append(out, toRenderMetricSeries(s))
	}
	return out
}
