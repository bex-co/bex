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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func oversizedBody(w http.ResponseWriter) {
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = 'a'
	}
	const total = core.MaxUpstreamResponseBytes + 2
	for written := int64(0); written < total; {
		n := int64(len(chunk))
		if remaining := total - written; remaining < n {
			n = remaining
		}
		if _, err := w.Write(chunk[:n]); err != nil {
			return
		}
		written += n
	}
}

// codex round-8 #10: the Prometheus range-query decode rejects an over-budget
// body explicitly instead of allocating whatever the backend sent.
func TestPrometheusSourceRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { oversizedBody(w) }))
	defer srv.Close()

	src := NewPrometheusRequestSource(srv.URL, srv.Client())
	_, err := src(context.Background(), RequestMetricsRequest{
		App: "web", Metric: MetricHTTPRequests,
		Start: time.Now().Add(-time.Minute), End: time.Now(), Resolution: time.Minute,
	})
	if !errors.Is(err, core.ErrUpstreamResponseTooLarge) {
		t.Fatalf("oversized Prometheus response => %v, want ErrUpstreamResponseTooLarge", err)
	}
}

// The filter-values source (its own inline transport, not queryRangeMatrix) is
// bounded the same way.
func TestPrometheusFilterValuesRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { oversizedBody(w) }))
	defer srv.Close()

	src := NewPrometheusFilterValuesSource(srv.URL, srv.Client())
	_, err := src(context.Background(), "default", "web", 80, "pod")
	if !errors.Is(err, core.ErrUpstreamResponseTooLarge) {
		t.Fatalf("oversized label-values response => %v, want ErrUpstreamResponseTooLarge", err)
	}
}
