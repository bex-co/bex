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

package objectstorage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type stubEgress struct {
	total int64
	err   error
	since []time.Time
}

func (s *stubEgress) ObjectStorageEgressSince(_ context.Context, since time.Time) (int64, error) {
	s.since = append(s.since, since)
	return s.total, s.err
}

func TestTheEgressGaugeReadsTheCurrentUTCMonth(t *testing.T) {
	reg := prometheus.NewRegistry()
	usage := &stubEgress{total: 123_456_789}
	m := NewEgressMonitor(reg, usage)
	// 23:30 on the last day of November in Honolulu is already December in UTC.
	honolulu := time.FixedZone("HST", -10*3600)
	m.now = func() time.Time { return time.Date(2026, 11, 30, 23, 30, 0, 0, honolulu) }
	if err := m.publish(t.Context()); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC); !usage.since[0].Equal(want) {
		t.Errorf("read since %s, want %s: the month is UTC's, as usage_hourly's windows are", usage.since[0], want)
	}
	if got := testutil.ToFloat64(m.gauge); got != 123_456_789 {
		t.Errorf("gauge = %v", got)
	}
	// The series name is the one the alert and the panel read.
	if n, err := testutil.GatherAndCount(reg, "bex_usage_object_storage_egress_month_bytes"); err != nil || n != 1 {
		t.Errorf("the registry holds %d series named bex_usage_object_storage_egress_month_bytes, %v", n, err)
	}
	// A failed read keeps the last value: the alert prefers a stale total to none.
	usage.err = errors.New("database is away")
	usage.total = 1
	if err := m.publish(t.Context()); err == nil {
		t.Error("a failed read should be reported to the loop")
	}
	if got := testutil.ToFloat64(m.gauge); got != 123_456_789 {
		t.Errorf("after a failed read the gauge = %v, want the last value", got)
	}
}

func TestTheEgressGaugeHasAHelpTextThatSaysWhatItCounts(t *testing.T) {
	reg := prometheus.NewRegistry()
	NewEgressMonitor(reg, &stubEgress{})
	families, err := reg.Gather()
	if err != nil || len(families) != 1 {
		t.Fatalf("%v %v", families, err)
	}
	if help := families[0].GetHelp(); !strings.Contains(help, "month") || !strings.Contains(help, "Wasabi") {
		t.Errorf("help = %q", help)
	}
}
