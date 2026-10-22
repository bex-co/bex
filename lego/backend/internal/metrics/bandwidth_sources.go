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
	"net/http"
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
	q.Metric, q.Resolution = MetricBandwidth, time.Hour
	out := []bandwidthSourceSeries{}
	for _, res := range resources {
		if isDatastoreResource(res) {
			// The same named refusal /v1/metrics/bandwidth gives a datastore.
			if _, err := s.datastoreAppMetric(r.Context(), q, res); err != nil {
				core.WriteErr(w, err)
				return
			}
			continue
		}
		q.App = res
		for _, source := range bandwidthTrafficSources {
			q.BandwidthSource = source.metric
			series, err := s.Metrics(r.Context(), q)
			if err != nil {
				core.WriteErr(w, err)
				return
			}
			out = append(out, bandwidthSourcesFor(res, source.render, series))
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"data": out})
}

// bandwidthSourcesFor folds one source's series into Render's hourly MiB points.
func bandwidthSourcesFor(resource, trafficSource string, series []MetricSeries) bandwidthSourceSeries {
	var out bandwidthSourceSeries
	out.Labels.Resource, out.Labels.TrafficSource = resource, trafficSource
	out.Values = []bandwidthSourcePoint{}
	const bytesPerMiB = 1 << 20
	for _, ser := range series {
		for _, p := range ser.Points {
			at, err := time.Parse(time.RFC3339Nano, p.Timestamp)
			if err != nil {
				continue // every source stamps RFC 3339; an unparseable point has no place on a time axis
			}
			out.Values = append(out.Values, bandwidthSourcePoint{Timestamp: at.Unix(), Value: p.Value / bytesPerMiB})
		}
	}
	return out
}
