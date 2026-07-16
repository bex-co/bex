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

package sniproxy

import (
	"io"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ByteMeter struct {
	responseBytes *prometheus.CounterVec
	healthy       prometheus.Gauge
}

func NewByteMeter(registry prometheus.Registerer, subsystem, resourceKind string) *ByteMeter {
	meter := &ByteMeter{
		responseBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: subsystem, Name: "egress_bytes_total",
			Help: "Bytes copied from a managed datastore backend to a public client.",
		}, []string{"resource_id", "resource_kind"}),
		healthy: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: subsystem, Name: "healthy",
			Help:        "1 when the public SNI proxy routing table and metrics listener are ready.",
			ConstLabels: prometheus.Labels{"resource_kind": resourceKind},
		}),
	}
	processStart := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "bex", Subsystem: subsystem, Name: "process_start_time_seconds",
		Help:        "Unix time when this process-local proxy counter started.",
		ConstLabels: prometheus.Labels{"resource_kind": resourceKind},
	})
	processStart.Set(float64(time.Now().Unix()))
	registry.MustRegister(meter.responseBytes, meter.healthy, processStart)
	return meter
}

func (m *ByteMeter) SetHealthy(healthy bool) {
	if healthy {
		m.healthy.Set(1)
		return
	}
	m.healthy.Set(0)
}

func (m *ByteMeter) Add(resourceID, resourceKind string, bytes int64) {
	if bytes > 0 {
		m.responseBytes.WithLabelValues(resourceID, resourceKind).Add(float64(bytes))
	}
}

// CopyBidirectional splices a public client and private backend. Only the
// backend→client result increments the meter; request bytes can never be
// reversed into egress by a shared call-site mistake.
func CopyBidirectional(client, backend net.Conn, meter *ByteMeter, resourceID, resourceKind string) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(backend, client)
		done <- struct{}{}
	}()
	go func() {
		bytes, _ := io.Copy(client, backend)
		meter.Add(resourceID, resourceKind, bytes)
		done <- struct{}{}
	}()
	<-done
}
