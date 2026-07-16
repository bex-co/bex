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

package egressmeter

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type collector struct {
	meter          *Meter
	sourceInstance string
	startedAt      float64
	bytes          *prometheus.Desc
	healthy        *prometheus.Desc
	expected       *prometheus.Desc
	attributed     *prometheus.Desc
	errors         *prometheus.Desc
	lossEvents     *prometheus.Desc
	lastLoss       *prometheus.Desc
	unsupported    *prometheus.Desc
	mapPressure    *prometheus.Desc
	startTime      *prometheus.Desc
	diagnostics    *prometheus.Desc
}

func NewCollector(meter *Meter) prometheus.Collector {
	labels := []string{"node", "source_instance"}
	return &collector{
		meter: meter, sourceInstance: persistentSourceInstance(meter.config.SourceIDPath), startedAt: float64(time.Now().Unix()),
		bytes:       prometheus.NewDesc("bex_app_direct_egress_bytes_total", "Allowed App-initiated L3 bytes sent to public IPv4/IPv6 destinations at netfilter POST_ROUTING.", []string{"namespace", "app_id", "node", "source_instance"}, nil),
		healthy:     prometheus.NewDesc("bex_egress_meter_healthy", "1 when both host POST_ROUTING hooks, Pod-IP attribution, exclusions, and persistent counters are healthy.", labels, nil),
		expected:    prometheus.NewDesc("bex_egress_meter_expected_pods", "Number of non-terminal App pods expected on this node.", labels, nil),
		attributed:  prometheus.NewDesc("bex_egress_meter_attributed_pod_ips", "Number of current App Pod IPs present in the packet-attribution maps.", labels, nil),
		errors:      prometheus.NewDesc("bex_egress_meter_reconcile_errors_total", "Pod-IP, exclusion, map, checkpoint, or hook errors; any increment invalidates source health until a clean reconcile.", labels, nil),
		lossEvents:  prometheus.NewDesc("bex_egress_meter_counter_loss_events_total", "Durable count of detected counter-map restores or decreases.", labels, nil),
		lastLoss:    prometheus.NewDesc("bex_egress_meter_last_counter_loss_time_seconds", "Unix time of the most recent detected counter-map restore or decrease; zero before any loss.", labels, nil),
		unsupported: prometheus.NewDesc("bex_egress_meter_unsupported_kernel", "1 when the kernel netfilter-BPF contract is unavailable.", labels, nil),
		mapPressure: prometheus.NewDesc("bex_egress_meter_resource_map_pressure_ratio", "Fraction of the bounded resource counter map currently occupied.", labels, nil),
		startTime:   prometheus.NewDesc("bex_egress_meter_process_start_time_seconds", "Unix time when this exporter process started; pinned links and the persistent source identity preserve accounting across a restart.", labels, nil),
		diagnostics: prometheus.NewDesc("bex_egress_meter_packets_total", "POST_ROUTING packets by bounded accounting decision; used to diagnose attribution and exclusion without sampled flows.", []string{"node", "source_instance", "decision"}, nil),
	}
}

func persistentSourceInstance(path string) string {
	if raw, err := os.ReadFile(path); err == nil {
		if value := strings.TrimSpace(string(raw)); len(value) == 32 {
			return value
		}
	}
	var raw [16]byte
	value := ""
	if _, err := rand.Read(raw[:]); err == nil {
		value = hex.EncodeToString(raw[:])
	} else {
		value = fmt.Sprintf("fallback-%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err == nil {
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err == nil {
			return value
		}
	}
	return value
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{c.bytes, c.healthy, c.expected, c.attributed, c.errors, c.lossEvents, c.lastLoss, c.unsupported, c.mapPressure, c.startTime, c.diagnostics} {
		ch <- desc
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	base := []string{c.meter.config.NodeName, c.sourceInstance}
	rows, err := c.meter.counterRows()
	if err == nil {
		for _, row := range rows {
			ch <- prometheus.MustNewConstMetric(c.bytes, prometheus.CounterValue, float64(row.Bytes), row.Namespace, row.AppID, base[0], base[1])
		}
	}
	unsupported := 0.0
	c.meter.mu.RLock()
	if c.meter.loadErr != nil || c.meter.objects == nil {
		unsupported = 1
	}
	c.meter.mu.RUnlock()
	pressure := float64(len(rows)) / 16384
	ch <- prometheus.MustNewConstMetric(c.healthy, prometheus.GaugeValue, float64(c.meter.healthy.Load()), base...)
	ch <- prometheus.MustNewConstMetric(c.expected, prometheus.GaugeValue, float64(c.meter.expectedPods.Load()), base...)
	ch <- prometheus.MustNewConstMetric(c.attributed, prometheus.GaugeValue, float64(c.meter.attributedIPs.Load()), base...)
	ch <- prometheus.MustNewConstMetric(c.errors, prometheus.CounterValue, float64(c.meter.reconcileErrors.Load()), base...)
	ch <- prometheus.MustNewConstMetric(c.lossEvents, prometheus.CounterValue, float64(c.meter.lossEvents.Load()), base...)
	ch <- prometheus.MustNewConstMetric(c.lastLoss, prometheus.GaugeValue, float64(c.meter.lastLossUnix.Load()), base...)
	ch <- prometheus.MustNewConstMetric(c.unsupported, prometheus.GaugeValue, unsupported, base...)
	if errors.Is(err, os.ErrNotExist) {
		pressure = 1
	}
	ch <- prometheus.MustNewConstMetric(c.mapPressure, prometheus.GaugeValue, pressure, base...)
	ch <- prometheus.MustNewConstMetric(c.startTime, prometheus.GaugeValue, c.startedAt, base...)
	c.meter.mu.RLock()
	if c.meter.objects != nil {
		for index, decision := range []string{"seen", "packet_read_error", "unknown_version", "ipv4_unattributed", "ipv4_excluded", "ipv4_counted", "ipv6_unattributed", "ipv6_excluded", "ipv6_counted"} {
			var value uint64
			if lookupErr := c.meter.objects.Diagnostics.Lookup(uint32(index), &value); lookupErr == nil {
				ch <- prometheus.MustNewConstMetric(c.diagnostics, prometheus.CounterValue, float64(value), base[0], base[1], decision)
			}
		}
	}
	c.meter.mu.RUnlock()
}
