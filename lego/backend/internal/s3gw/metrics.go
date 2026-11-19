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

package s3gw

import (
	"io"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// The S3 surface's telemetry (ADR112 D3, "What watches it"): what it answers, who fails to
// authenticate and why, the streams in flight against their cap, the bytes in and out and what
// listings read. Labels are bounded by construction: the operation is one of the routes, the code
// one of S3's, the reason one of a closed set, and no workspace, bucket, key or address is ever a
// label.

// authFailureReasons maps an authentication refusal's code to the closed set of the
// auth-failure counter's reason label. "secret_store" is OpenBao failing on the key path, which
// is the platform's fault and not the client's.
var authFailureReasons = map[string]string{
	"InvalidAccessKeyId":                "unknown_key",
	"SignatureDoesNotMatch":             "bad_signature",
	"RequestTimeTooSkewed":              "clock_skew",
	"ExpiredToken":                      "expired",
	"AuthorizationHeaderMalformed":      "malformed",
	"AuthorizationQueryParametersError": "malformed",
	"AccessDenied":                      "denied",
	"ServiceUnavailable":                "secret_store",
	"SlowDown":                          "throttled",
	"InternalError":                     "internal",
}

// Metrics is the gateway's Prometheus telemetry. A nil *Metrics records nothing.
type Metrics struct {
	requests       *prometheus.CounterVec
	authFailures   *prometheus.CounterVec
	bytes          *prometheus.CounterVec
	egressRefusals prometheus.Counter
	listingBytes   prometheus.Counter
}

// NewMetrics registers the metrics on reg. streams reports the streams in flight on this
// replica and the replica's cap, which the saturation alert divides.
func NewMetrics(reg prometheus.Registerer, streams func() (inFlight, limit int)) *Metrics {
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "requests_total",
			Help: "Requests bex-s3 answered, by S3 operation, S3 error code (OK for a success) and HTTP status.",
		}, []string{"operation", "code", "status"}),
		authFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "auth_failures_total",
			Help: "Requests refused before an operation ran, by reason: unknown_key, bad_signature, clock_skew, expired, malformed, denied, throttled, internal, or secret_store (OpenBao failing on the key path).",
		}, []string{"reason"}),
		bytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "bytes_total",
			Help: "Body bytes bex-s3 moved, by direction: in (read from clients) or out (written to them).",
		}, []string{"direction"}),
		egressRefusals: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "egress_budget_refusals_total",
			Help: "GetObject requests refused because the workspace passed its monthly S3 egress budget.",
		}),
		listingBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "listing_response_bytes_total",
			Help: "Bytes of ListObjects and ListObjectsV2 answers: what Wasabi counts as egress for the listings bex serves.",
		}),
	}
	reg.MustRegister(m.requests, m.authFailures, m.bytes, m.egressRefusals, m.listingBytes,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "streams_in_flight",
			Help: "Uploads and downloads in flight on this replica.",
		}, func() float64 { n, _ := streams(); return float64(n) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "s3gw", Name: "stream_limit",
			Help: "The most streams this replica serves at once.",
		}, func() float64 { _, limit := streams(); return float64(limit) }),
	)
	// Every direction exists from the start, so a rate over a quiet series is 0, not absent.
	m.bytes.WithLabelValues("in")
	m.bytes.WithLabelValues("out")
	for _, reason := range authFailureReasons {
		m.authFailures.WithLabelValues(reason)
	}
	return m
}

// instrument wraps a request's writer and body so what moves is counted, and returns the
// function that records the request once it is answered. op names the operation for the
// request counter, "unauthenticated" until the request is bound to a key.
func (m *Metrics) instrument(w http.ResponseWriter, r *http.Request) (*countingWriter, func(op string, authStage bool, e *Error)) {
	cw := &countingWriter{ResponseWriter: w, status: http.StatusOK}
	if m == nil {
		return cw, func(string, bool, *Error) {}
	}
	var in atomic.Int64
	if r.Body != nil && r.Body != http.NoBody {
		r.Body = &countingBody{ReadCloser: r.Body, n: &in}
	}
	return cw, func(op string, authStage bool, e *Error) {
		code, status := "OK", cw.status
		if e != nil {
			code, status = e.Code, e.Status
		}
		m.requests.WithLabelValues(op, code, strconv.Itoa(status)).Inc()
		m.bytes.WithLabelValues("in").Add(float64(in.Load()))
		m.bytes.WithLabelValues("out").Add(float64(cw.bytes.Load()))
		switch {
		case e != nil && authStage:
			reason, ok := authFailureReasons[code]
			if !ok {
				reason = "denied"
			}
			m.authFailures.WithLabelValues(reason).Inc()
		case op == "GetObject" && code == "QuotaExceeded":
			m.egressRefusals.Inc()
		case (op == "ListObjects" || op == "ListObjectsV2") && e == nil:
			m.listingBytes.Add(float64(cw.bytes.Load()))
		}
	}
}

// countingWriter counts the bytes written and remembers the status.
type countingWriter struct {
	http.ResponseWriter
	status int
	bytes  atomic.Int64
}

func (c *countingWriter) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.ResponseWriter.Write(p)
	c.bytes.Add(int64(n))
	return n, err
}

// Unwrap lets http.ResponseController reach the connection to set deadlines.
func (c *countingWriter) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// countingBody counts the bytes read from a request body.
type countingBody struct {
	io.ReadCloser
	n *atomic.Int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.n.Add(int64(n))
	return n, err
}
