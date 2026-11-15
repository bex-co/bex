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

package objectgw

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// requestResult is what a request ended in: the closed label of
// bex_objectgw_requests_total.
type requestResult string

const (
	resultOK                  requestResult = "ok"
	resultNotModified         requestResult = "not_modified"
	resultPreconditionFailed  requestResult = "precondition_failed"
	resultPreflight           requestResult = "preflight"
	resultNotFound            requestResult = "not_found"
	resultMethodNotAllowed    requestResult = "method_not_allowed"
	resultRateLimited         requestResult = "rate_limited"
	resultRangeNotSatisfiable requestResult = "range_not_satisfiable"
	resultUnavailable         requestResult = "unavailable"
	resultError               requestResult = "error"
)

var allResults = []requestResult{
	resultOK, resultNotModified, resultPreconditionFailed, resultPreflight, resultNotFound, resultMethodNotAllowed,
	resultRateLimited, resultRangeNotSatisfiable, resultUnavailable, resultError,
}

// responseOrigin is where a response's bytes came from: the closed label of
// bex_objectgw_response_bytes_total. Only wasabi counts against Wasabi's
// fair-use rule; cache hits cost the nodes' bandwidth and no Wasabi egress.
type responseOrigin string

const (
	originWasabi responseOrigin = "wasabi"
	originCache  responseOrigin = "cache"
)

// Metrics is the gateway's own telemetry, on its internal listener. Labels are
// bounded by construction: no bucket, workspace, host or key is ever a label.
type Metrics struct {
	registry      *prometheus.Registry
	requests      *prometheus.CounterVec
	responseBytes *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "objectgw", Name: "requests_total",
			Help: "Requests the public bucket gateway answered, by result.",
		}, []string{"result"}),
		responseBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "objectgw", Name: "response_bytes_total",
			Help: "Body bytes the public bucket gateway sent to visitors, by where they came from: wasabi (counts against Wasabi's fair-use egress rule) or its own cache.",
		}, []string{"origin"}),
	}
	m.registry.MustRegister(m.requests, m.responseBytes, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	// Every label value exists from the start, so a rate over a quiet series is 0, not absent.
	for _, r := range allResults {
		m.requests.WithLabelValues(string(r))
	}
	for _, o := range []responseOrigin{originWasabi, originCache} {
		m.responseBytes.WithLabelValues(string(o))
	}
	return m
}

// Handler serves the Prometheus exposition.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
