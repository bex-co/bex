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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestMalformedMetricsQueryInputsArePublicErrors(t *testing.T) {
	for _, input := range []any{nil, "invalid", []any{}} {
		_, _, err := metricsQueryInputFromArgs(input)
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("metrics(%#v) = %v", input, err)
		}
		_, err = datastoreMetricsQueryInputFromArgs(input)
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("datastore(%#v) = %v", input, err)
		}
		_, err = metricsFiltersQueryFromArgs(input)
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("filters(%#v) = %v", input, err)
		}
	}
	_, err := metricsFiltersQueryFromArgs(map[string]any{"filters": []any{}})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "RESOURCE") {
		t.Errorf("empty resource filters = %v", err)
	}
}

func TestMetricsServiceRejectsUnknownNamesAndDefaultedInvalidRanges(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"), sampleDatabase("pg", false))
	for _, start := range []time.Time{fixedClock(), fixedClock().Add(time.Hour)} {
		_, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemory, Start: start})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "must be before") {
			t.Errorf("defaulted metrics range = %v", err)
		}
		_, err = svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDisk, Start: start})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "must be before") {
			t.Errorf("defaulted datastore range = %v", err)
		}
	}
	_, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: "unknown"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown metrics name = %v", err)
	}
	_, err = svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: "unknown"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown datastore metric name = %v", err)
	}
}
