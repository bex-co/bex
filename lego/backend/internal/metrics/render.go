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

// A per-status breakdown's label. The sources label it code; each surface
// spells it as Render's same surface does (w3/080). REST and MCP say
// statusCode, Render's aggregateBy value and filters/http field. GraphQL says
// STATUS_CODE, the MetricQueryFilter value Render's dashboard finds the label
// by (docs/render-artifacts/metrics-page.md § Status-code label).
const (
	sourceStatusCodeLabel  = "code"
	restStatusCodeLabel    = "statusCode"
	graphqlStatusCodeLabel = "STATUS_CODE"
)

// nameStatusCodeLabel renames a per-status breakdown's source label to name.
func nameStatusCodeLabel(s MetricSeries, groupBy, name string) MetricSeries {
	if groupBy != groupByStatus {
		return s
	}
	if code, ok := s.Labels[sourceStatusCodeLabel]; ok {
		delete(s.Labels, sourceStatusCodeLabel)
		s.SetLabel(name, code)
	}
	return s
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
