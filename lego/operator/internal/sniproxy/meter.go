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
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ByteMeter struct {
	responseBytes *prometheus.CounterVec
	rejected      *prometheus.CounterVec
	healthy       prometheus.Gauge
}

func NewByteMeter(registry prometheus.Registerer, subsystem, resourceKind string) *ByteMeter {
	meter := &ByteMeter{
		responseBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: subsystem, Name: "egress_bytes_total",
			Help: "Bytes copied from a managed datastore backend to a public client.",
		}, []string{"resource_id", "resource_kind"}),
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: subsystem, Name: "connections_rejected_total",
			Help: "Connections shed by the admission limiter, by reason (global|source).",
		}, []string{"reason"}),
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
	registry.MustRegister(meter.responseBytes, meter.rejected, meter.healthy, processStart)
	return meter
}

// Reject records one connection shed by the admission limiter (finding 6).
func (m *ByteMeter) Reject(reason string) {
	m.rejected.WithLabelValues(reason).Inc()
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
//
// idle bounds how long either direction may stall with no bytes before the
// connection is torn down; maxLifetime caps the total duration of the routed
// copy. Both guard against a routed connection idling or living indefinitely
// after DB/Valkey auth (finding 6). A non-positive value disables that bound.
// When both are disabled the copy is a plain io.Copy, byte-identical to before.
// Either direction ending closes both conns so the peer copy unblocks — no
// leaked goroutine.
func CopyBidirectional(
	client, backend net.Conn,
	meter *ByteMeter,
	resourceID, resourceKind string,
	idle, maxLifetime time.Duration,
) {
	var lifetime time.Time
	if maxLifetime > 0 {
		lifetime = time.Now().Add(maxLifetime)
	}
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = client.Close()
			_ = backend.Close()
		})
	}

	done := make(chan struct{}, 2)
	go func() {
		_, _ = copyStream(backend, client, idle, lifetime)
		closeBoth()
		done <- struct{}{}
	}()
	go func() {
		bytes, _ := copyStream(client, backend, idle, lifetime)
		meter.Add(resourceID, resourceKind, bytes)
		closeBoth()
		done <- struct{}{}
	}()
	<-done
	<-done
}

// copyStream copies src→dst, refreshing src's read deadline to bound idle stalls
// and never reading past the absolute lifetime deadline. With no bounds it falls
// back to io.Copy. It returns the number of bytes written to dst (io.Copy
// semantics), so the metered direction still counts only delivered bytes.
func copyStream(dst io.Writer, src net.Conn, idle time.Duration, lifetime time.Time) (int64, error) {
	if idle <= 0 && lifetime.IsZero() {
		return io.Copy(dst, src)
	}
	buf := make([]byte, 32*1024)
	var total int64
	for {
		_ = src.SetReadDeadline(readDeadline(idle, lifetime))
		n, rerr := src.Read(buf)
		if n > 0 {
			w, werr := dst.Write(buf[:n])
			total += int64(w)
			if werr != nil {
				return total, werr
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return total, nil
			}
			return total, rerr
		}
	}
}

// readDeadline is the earlier of now+idle and the absolute lifetime deadline; a
// zero time (both bounds disabled for this read) clears the deadline.
func readDeadline(idle time.Duration, lifetime time.Time) time.Time {
	var d time.Time
	if idle > 0 {
		d = time.Now().Add(idle)
	}
	if !lifetime.IsZero() && (d.IsZero() || lifetime.Before(d)) {
		d = lifetime
	}
	return d
}
