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
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// EgressGaugeInterval is how often the month-to-date egress gauge is refreshed.
// The meter flushes to usage_hourly every minute (TransferFlushInterval), so a
// faster read would only repeat itself.
const EgressGaugeInterval = time.Minute

// EgressUsage reads the object storage egress meter. *store.PGStore implements it.
type EgressUsage interface {
	ObjectStorageEgressSince(ctx context.Context, since time.Time) (int64, error)
}

// EgressMonitor publishes the month-to-date bytes read out of object storage as
// bex_usage_object_storage_egress_month_bytes, the series the egress alert reads
// (ADR112 D4). Prometheus keeps only three days, so a month cannot be summed
// there: the shadow meter in usage_hourly is the source, over the current UTC
// calendar month. Every bex-api replica reads the same rows, so they agree.
type EgressMonitor struct {
	gauge prometheus.Gauge
	usage EgressUsage
	now   func() time.Time
}

// NewEgressMonitor registers the gauge on registerer.
func NewEgressMonitor(registerer prometheus.Registerer, usage EgressUsage) *EgressMonitor {
	m := &EgressMonitor{
		gauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "usage", Name: "object_storage_egress_month_bytes",
			Help: "Bytes read out of object storage by every workspace since the start of the current UTC month, from the object_storage_egress_bytes shadow meter: Render's presigned reads, wrangler's streamed reads and public bucket reads together. An upper bound of Wasabi's own egress, which is a fair-use ratio against stored volume on a shared account.",
		}),
		usage: usage,
		now:   time.Now,
	}
	registerer.MustRegister(m.gauge)
	return m
}

// publish refreshes the gauge. A failed read keeps the last value and returns
// the error for the loop to log: the alert would rather read a stale total than
// none.
func (m *EgressMonitor) publish(ctx context.Context) error {
	total, err := m.usage.ObjectStorageEgressSince(ctx, store.MonthStartUTC(m.now()))
	if err != nil {
		return err
	}
	m.gauge.Set(float64(total))
	return nil
}

// Run refreshes the gauge every interval until ctx ends.
func (m *EgressMonitor) Run(ctx context.Context, interval time.Duration) {
	core.Poll(ctx, "object storage egress gauge", interval, m.publish)
}
