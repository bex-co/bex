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
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ByteMeter struct {
	responseBytes *prometheus.CounterVec
	rejected      *prometheus.CounterVec
	healthy       prometheus.Gauge
	// marks numbers AwaitFinalScrape's calls, and scraped is the latest mark a
	// Prometheus scrape began after (w6/151).
	marks, scraped atomic.Uint64
}

func NewByteMeter(registry prometheus.Registerer, subsystem, resourceKind string) *ByteMeter {
	meter := &ByteMeter{
		responseBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: subsystem, Name: "egress_bytes_total",
			Help: "Bytes copied from a managed datastore backend to a public client.",
		}, []string{"resource_id", "resource_kind"}),
		rejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: subsystem, Name: "connections_rejected_total",
			Help: "Connections shed by the admission limiter, by reason (global|source|handshake|resource).",
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

// Add counts backend→client bytes as they reach the client. A resource's
// series first appears with its first bytes and then stays in every scrape for
// the life of the process, which is what lets billing count a series that
// first appears inside an hour from zero (ADR023 § New series count from zero,
// w6/133). Never delete or reset a series while the process runs: one that
// came back carrying its old value would be billed twice.
func (m *ByteMeter) Add(resourceID, resourceKind string, bytes int64) {
	if bytes > 0 {
		m.responseBytes.WithLabelValues(resourceID, resourceKind).Add(float64(bytes))
	}
}

// scrapeTimeoutHeader is on every Prometheus scrape request and on no kubelet
// probe, so it tells a scrape from the readiness probe on the same path.
const scrapeTimeoutHeader = "X-Prometheus-Scrape-Timeout-Seconds"

// FinalScrapeWait bounds how long a drained proxy waits for a Prometheus
// scrape: two of the bex-kv-proxy and bex-pg-proxy jobs' 15 s scrapes
// (deploy/gitops/base/prometheus.yaml), so one failed scrape leaves another,
// and 5 s for the second to finish. The DaemonSets' grace period covers it
// after the drain (TestTheDaemonSetGivesTheDrainItsBound).
const FinalScrapeWait = 35 * time.Second

// Handler serves gatherer's metrics as promhttp does, and records each
// Prometheus scrape for AwaitFinalScrape. gatherer is the registry the meter
// was registered with.
func (m *ByteMeter) Handler(gatherer prometheus.Gatherer) http.Handler {
	metrics := promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mark := m.marks.Load()
		metrics.ServeHTTP(w, r)
		if r.Header.Get(scrapeTimeoutHeader) == "" {
			return
		}
		for {
			read := m.scraped.Load()
			if mark <= read || m.scraped.CompareAndSwap(read, mark) {
				return
			}
		}
	})
}

// AwaitFinalScrape returns true once a Prometheus scrape that began after the
// call has been served, or false after wait. A drained proxy calls it after
// Serve returns, so it exits just after a scrape that read every byte it
// counted, those of its last streams and of the streams its bound closed
// included (w6/151). Prometheus then has a whole interval to drop the target
// before its next scrape, which would read up 0 from the exited pod and fail
// the hour's egress health for every store.
func (m *ByteMeter) AwaitFinalScrape(wait time.Duration) bool {
	mark := m.marks.Add(1)
	deadline := time.Now().Add(wait)
	for m.scraped.Load() < mark {
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
	return true
}

// CopyBidirectional splices a public client and private backend. Only the
// backend→client bytes increment the meter, and each write counts as it lands
// (w6/151): a scrape taken while a stream runs reads what it has delivered, so
// a long stream is billed in the hours it ran, and a process that exits or
// crashes loses at most what it copied since its last scrape. Request bytes
// can never be reversed into egress by a shared call-site mistake.
//
// idle bounds how long the stream may go with no bytes in either direction
// before the connection is torn down: bytes either way keep both directions
// open, so a subscriber that only receives stays connected while its messages
// arrive (w6/148). maxLifetime caps the total duration of the routed copy,
// however busy. A non-positive value disables that bound, and both proxies
// disable both by default (StreamBounds, w6/149). Either direction ending
// closes both conns so the peer copy unblocks — no leaked goroutine.
func CopyBidirectional(
	client, backend net.Conn,
	meter *ByteMeter,
	resourceID, resourceKind string,
	idle, maxLifetime time.Duration,
) {
	bounds := newStreamBounds(idle, maxLifetime)
	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = client.Close()
			_ = backend.Close()
		})
	}

	done := make(chan struct{}, 2)
	go func() {
		copyStream(backend, client, bounds, nil)
		closeBoth()
		done <- struct{}{}
	}()
	go func() {
		copyStream(client, backend, bounds, func(bytes int64) {
			meter.Add(resourceID, resourceKind, bytes)
		})
		closeBoth()
		done <- struct{}{}
	}()
	<-done
	<-done
}

// streamBounds is one relayed stream's idle and lifetime bounds. Both
// directions share it, so the stream is idle only when neither has read a byte
// for idle (w6/148). The last read is kept as an offset from start so the
// deadlines stay on the monotonic clock.
type streamBounds struct {
	idle     time.Duration // non-positive: no idle bound
	lifetime time.Time     // zero: no lifetime bound
	start    time.Time
	since    atomic.Int64 // nanoseconds from start to the latest read of a byte
}

func newStreamBounds(idle, maxLifetime time.Duration) *streamBounds {
	b := &streamBounds{idle: idle, start: time.Now()}
	if maxLifetime > 0 {
		b.lifetime = b.start.Add(maxLifetime)
	}
	return b
}

func (b *streamBounds) touch() { b.since.Store(int64(time.Since(b.start))) }

// deadline is the earlier of the last read plus idle and the lifetime; a zero
// time (both bounds disabled) clears the deadline.
func (b *streamBounds) deadline() time.Time {
	var d time.Time
	if b.idle > 0 {
		d = b.start.Add(time.Duration(b.since.Load()) + b.idle)
	}
	if !b.lifetime.IsZero() && (d.IsZero() || b.lifetime.Before(d)) {
		d = b.lifetime
	}
	return d
}

// copyStream copies src→dst until src ends, a write fails, the stream has been
// idle both ways for the idle bound, or the lifetime passes. delivered, if
// non-nil, is handed each write's bytes once they are written to dst, so the
// metered direction counts only delivered bytes, as the stream runs. It never
// hands the copy to io.Copy, whose splice on Linux reports nothing until the
// stream ends (w6/151).
func copyStream(dst io.Writer, src net.Conn, bounds *streamBounds, delivered func(int64)) {
	bounded := bounds.idle > 0 || !bounds.lifetime.IsZero()
	buf := make([]byte, 32*1024)
	for {
		var deadline time.Time
		if bounded {
			deadline = bounds.deadline()
			_ = src.SetReadDeadline(deadline)
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			bounds.touch()
			w, werr := dst.Write(buf[:n])
			if w > 0 && delivered != nil {
				delivered(int64(w))
			}
			if werr != nil || w < n {
				return
			}
		}
		if rerr != nil {
			// The other direction read bytes after this deadline was set, so
			// the stream is not idle: read on to the later deadline.
			if errors.Is(rerr, os.ErrDeadlineExceeded) && bounds.deadline().After(deadline) {
				continue
			}
			return
		}
	}
}
