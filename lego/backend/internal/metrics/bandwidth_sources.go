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
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/egressquery"
)

// Render's GET /metrics/bandwidth-sources (w8/190): hourly bandwidth broken down
// by traffic source. bex meters the sources as Prometheus counters (egressquery),
// so each Render source is one source-narrowed bandwidth read:
//
//	http       Traefik's response bytes
//	websocket  the WebSocket meter
//	nat        direct egress (Render's NAT gateway; the egress meter)
//	privatelink does not exist in bex and is omitted, as the spec allows
//	total      every source summed (the all-sources bandwidth read)
//
// Values are MiB per hour, the unit of the month-to-date figures (MonthToDateBandwidth).

// bandwidthSourcesSince is when Render's traffic-source data begins; an earlier
// window is a 400, as Render's spec says.
var bandwidthSourcesSince = time.Date(2025, time.March, 9, 0, 0, 0, 0, time.UTC)

// maxBandwidthSourceResources is Render's cap (error code too_many_resources).
const maxBandwidthSourceResources = 30

// bandwidthTrafficSources maps Render's trafficSource values onto the metric
// source each is read from; "" is the all-sources total.
var bandwidthTrafficSources = []struct{ render, metric string }{
	{"total", ""},
	{"http", "http"},
	{"websocket", "websocket"},
	{"nat", "nat"},
}

// bandwidthSpecs are the egress sources a bandwidth request reads: every
// applicable one, or only the requested source's.
func bandwidthSpecs(req RequestMetricsRequest) []egressquery.Spec {
	specs := egressquery.App(req.AppID, req.Routers, req.Direct)
	want := map[string]egressquery.Source{
		"http": egressquery.HTTP, "websocket": egressquery.WebSocket, "nat": egressquery.Direct,
	}[req.BandwidthSource]
	if req.BandwidthSource == "" {
		return specs
	}
	out := specs[:0:0]
	for _, spec := range specs {
		if spec.Source == want {
			out = append(out, spec)
		}
	}
	return out
}

type bandwidthSourceSeries struct {
	Labels struct {
		Resource      string `json:"resource"`
		TrafficSource string `json:"trafficSource"`
	} `json:"labels"`
	Values []bandwidthSourcePoint `json:"values"`
}

type bandwidthSourcePoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// bandwidthSources answers GET /v1/metrics/bandwidth-sources.
func (s *Service) bandwidthSources(w http.ResponseWriter, r *http.Request) {
	resources, q, err := parseMetricParams(r)
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(resources) > maxBandwidthSourceResources {
		core.WriteErrStatus(w, http.StatusBadRequest, fmt.Sprintf(
			"too_many_resources: the breakdown supports at most %d resources", maxBandwidthSourceResources))
		return
	}
	if !q.Start.IsZero() && q.Start.Before(bandwidthSourcesSince) {
		core.WriteErrStatus(w, http.StatusBadRequest, "bandwidth sources data is only available after 2025-03-09")
		return
	}
	// The cap and the order hold for the window as requested; the read then
	// widens it to the whole hours it touches.
	if err := s.checkWindow(q.Start, q.End); err != nil {
		core.WriteErr(w, err)
		return
	}
	now := s.Now()
	q = q.normalized(now)
	if err := core.ValidateQueryRange(q.Start, q.End); err != nil {
		core.WriteErr(w, err)
		return
	}
	q.Metric = MetricBandwidth
	read := q
	read.Start, read.End, read.Resolution = hourlyRead(q.Start, q.End, now)
	// The widened reads skip Metrics' cap, so they name its verb themselves.
	ctx := core.WithAuditVerb(r.Context(), AuditVerbMetrics)
	out := []bandwidthSourceSeries{}
	for _, res := range resources {
		if core.IsDatastoreResource(res) {
			// The same named refusal /v1/metrics/bandwidth gives a datastore.
			if _, err := s.datastoreAppMetric(r.Context(), q, res); err != nil {
				core.WriteErr(w, err)
				return
			}
			continue
		}
		read.App = res
		for _, source := range bandwidthTrafficSources {
			read.BandwidthSource = source.metric
			series, err := s.metricsInWindow(ctx, read)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			out = append(out, bandwidthSourcesFor(res, source.render, read.Start, read.Resolution, series))
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

// hourSteps are the bandwidth steps that divide an hour, finest first, so a
// read from a whole hour has a grid point on every hour boundary.
var hourSteps = []time.Duration{
	time.Minute, 2 * time.Minute, 3 * time.Minute, 4 * time.Minute, 5 * time.Minute, 6 * time.Minute,
	10 * time.Minute, 12 * time.Minute, 15 * time.Minute, 20 * time.Minute, 30 * time.Minute, time.Hour,
}

// hourlyRead is the stepped read under Render's hourly points (w8/414). It
// spans the UTC hours [start, end) touches, from the first one's start to the
// last one's end, and stops at now: the hour in progress is read so far. The
// step is the finest one in hourSteps within the per-series point budget (a
// minute up to about seven days, four minutes for the default 720-hour cap).
//
// An hourly step would not do: Prometheus evaluates at start+k*step, and each
// point holds the step that ends there, so the hour in progress showed only on
// a point after now, which w8/413's bound never reads, and the grid followed
// startTime instead of the clock.
//
// The read is at least one step long, so one that begins in the current
// hour's first minute, or after now, still authorizes the resource and then
// answers nothing: the transport stops at now (promQueryRange).
func hourlyRead(start, end, now time.Time) (from, to time.Time, step time.Duration) {
	from = start.UTC().Truncate(time.Hour)
	to = end.UTC().Add(time.Hour - 1).Truncate(time.Hour)
	if to.After(now) {
		to = now
	}
	step = hourSteps[len(hourSteps)-1]
	for _, candidate := range hourSteps {
		if to.Sub(from)/candidate <= maxPointsPerSeries {
			step = candidate
			break
		}
	}
	if minimum := from.Add(step); to.Before(minimum) {
		to = minimum
	}
	return from, to, step
}

// bandwidthSourcesFor folds one source's stepped read into Render's hourly MiB
// points. Each point holds the step that ends at its timestamp, so its bytes
// belong to the UTC hour that step began in, and the point for an hour is
// stamped with the hour's start, as Render's example stamps whole hours
// (1709856000, 1709859600). The read's first point holds the step before from
// and is dropped. An hour with no point at all (before the service existed)
// has no entry; an hour with no traffic reads 0.
func bandwidthSourcesFor(resource, trafficSource string, from time.Time, step time.Duration, series []MetricSeries) bandwidthSourceSeries {
	var out bandwidthSourceSeries
	out.Labels.Resource, out.Labels.TrafficSource = resource, trafficSource
	const bytesPerMiB = 1 << 20
	hours := map[int64]float64{}
	for _, ser := range series {
		for _, p := range ser.Points {
			at, err := time.Parse(time.RFC3339Nano, p.Timestamp)
			if err != nil {
				continue // every source stamps RFC 3339; an unparseable point has no place on a time axis
			}
			hour := at.Add(-step).Truncate(time.Hour)
			if hour.Before(from) {
				continue
			}
			hours[hour.Unix()] += p.Value
		}
	}
	out.Values = make([]bandwidthSourcePoint, 0, len(hours))
	for _, hour := range slices.Sorted(maps.Keys(hours)) {
		out.Values = append(out.Values, bandwidthSourcePoint{Timestamp: hour, Value: hours[hour] / bytesPerMiB})
	}
	return out
}
