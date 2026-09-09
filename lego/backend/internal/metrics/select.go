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
	"math"
	"sort"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// maxSelectedInstances bounds INSTANCE filter cardinality so a caller cannot
// pin the control plane with an unbounded selector list (w5/m89).
const maxSelectedInstances = 32

// Replica aggregate method values (Render aggregateAllMethod vocabulary).
const (
	replicaAggregateMin = "MIN"
	replicaAggregateMax = "MAX"
	replicaAggregateAvg = "AVG"
)

// validateInstanceSelection rejects empty/blank/oversized INSTANCE filters.
// Omitted selection (nil/empty slice) means all instances and is valid.
func validateInstanceSelection(instances []string) error {
	if len(instances) == 0 {
		return nil
	}
	if len(instances) > maxSelectedInstances {
		return fmt.Errorf("%w: at most %d INSTANCE values per request", core.ErrBadRequest, maxSelectedInstances)
	}
	for _, id := range instances {
		if id == "" {
			return fmt.Errorf("%w: INSTANCE filter values must be non-empty", core.ErrBadRequest)
		}
	}
	return nil
}

// parseReplicaAggregate maps aggregateAllMethod onto ReplicaAggregate and the
// legacy AggregateMax flag. Unknown methods error instead of being ignored.
// Empty method leaves both unset (raw per-instance series).
func parseReplicaAggregate(method string) (replica string, aggregateMax bool, err error) {
	switch method {
	case "":
		return "", false, nil
	case replicaAggregateMin, replicaAggregateAvg:
		return method, false, nil
	case replicaAggregateMax:
		// Preserve AggregateMax for existing limit callers; ReplicaAggregate
		// drives per-timestamp MAX when series carry instance labels.
		return replicaAggregateMax, true, nil
	default:
		return "", false, fmt.Errorf("%w: aggregateAllMethod must be MIN, MAX, or AVG", core.ErrBadRequest)
	}
}

// filterSeriesByInstances keeps series whose public instance label matches a
// selector (canonical id, legacy UID-derived id against live candidates, or
// raw pod name). Unresolved selectors are dropped; when nothing resolves the
// result is empty — never silently broadened to all instances (w5/m89).
func filterSeriesByInstances(resourceID string, selectors []string, series []MetricSeries, live []ids.InstanceCandidate) []MetricSeries {
	if len(selectors) == 0 {
		return series
	}
	want := map[string]struct{}{}
	for _, sel := range selectors {
		for _, c := range live {
			if !ids.MatchServiceInstance(sel, resourceID, c.Name, c.UID) {
				continue
			}
			want[ids.ServiceInstanceID(resourceID, c.Name)] = struct{}{}
		}
		for _, ser := range series {
			inst := ser.Labels["instance"]
			if inst == "" {
				continue
			}
			if sel == inst || ids.ServiceInstanceID(resourceID, sel) == inst {
				want[inst] = struct{}{}
			}
		}
	}
	if len(want) == 0 {
		return []MetricSeries{}
	}
	out := make([]MetricSeries, 0, len(series))
	for _, ser := range series {
		if _, ok := want[ser.Labels["instance"]]; ok {
			out = append(out, ser)
		}
	}
	return out
}

func seriesHaveInstanceLabels(series []MetricSeries) bool {
	for _, ser := range series {
		if ser.Labels["instance"] != "" {
			return true
		}
	}
	return false
}

// aggregateReplicasAtTimestamps combines selected per-instance series into one
// series holding MIN/MAX/AVG at each shared timestamp. Missing samples stay
// absent — no zero fill and no borrow from another timestamp (w5/m89).
func aggregateReplicasAtTimestamps(app, method string, series []MetricSeries) []MetricSeries {
	if len(series) == 0 {
		return series
	}
	unit := series[0].Unit
	type acc struct {
		min, max, sum float64
		n             int
	}
	byTS := map[string]*acc{}
	for _, ser := range series {
		for _, p := range ser.Points {
			a := byTS[p.Timestamp]
			if a == nil {
				a = &acc{min: p.Value, max: p.Value, sum: p.Value, n: 1}
				byTS[p.Timestamp] = a
				continue
			}
			if p.Value < a.min {
				a.min = p.Value
			}
			if p.Value > a.max {
				a.max = p.Value
			}
			a.sum += p.Value
			a.n++
		}
	}
	timestamps := make([]string, 0, len(byTS))
	for ts := range byTS {
		timestamps = append(timestamps, ts)
	}
	sort.Strings(timestamps)
	points := make([]MetricPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		a := byTS[ts]
		var v float64
		switch method {
		case replicaAggregateMin:
			v = a.min
		case replicaAggregateMax:
			v = a.max
		case replicaAggregateAvg:
			v = a.sum / float64(a.n)
		default:
			v = a.max
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		points = append(points, MetricPoint{Timestamp: ts, Value: v})
	}
	return []MetricSeries{{
		Labels: map[string]string{"resource": app, "aggregate": method},
		Unit:   unit,
		Points: points,
	}}
}

// applyInstanceSelection runs after metric dispatch: optional INSTANCE filter,
// then replica MIN/MAX/AVG or the legacy latest-point AggregateMax collapse.
//
// Pipeline order (w5/m90): selection precedes aggregation, and normalization
// precedes aggregation — a percentage read arrives here already normalized per
// instance (resourceMetric divides each replica by its own trustworthy limit),
// so this step only filters and aggregates normalized values. It never divides
// an aggregate by one limit, fills missing samples with zeroes, or borrows
// across timestamps (aggregateReplicasAtTimestamps averages only the replicas
// present at each timestamp).
func applyInstanceSelection(q MetricQuery, series []MetricSeries, live []ids.InstanceCandidate) ([]MetricSeries, error) {
	if err := validateInstanceSelection(q.Instances); err != nil {
		return nil, err
	}
	if len(q.Instances) > 0 {
		if !seriesHaveInstanceLabels(series) {
			return nil, fmt.Errorf("%w: INSTANCE filter applies only to per-instance cpu/memory metrics", core.ErrBadRequest)
		}
		series = filterSeriesByInstances(q.App, q.Instances, series, live)
	}
	if q.ReplicaAggregate != "" && seriesHaveInstanceLabels(series) {
		return aggregateReplicasAtTimestamps(q.App, q.ReplicaAggregate, series), nil
	}
	if q.AggregateMax {
		return aggregateMaxSeries(q.App, series), nil
	}
	return series, nil
}
