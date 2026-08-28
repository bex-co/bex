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

package sshgateway

import (
	"slices"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsUseOnlyBoundedPrivacySafeLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	metrics.Handshake("established")
	metrics.Authentication("accepted")
	metrics.SessionStarted()
	metrics.SessionEnded("completed", 2*time.Second)
	metrics.LimitRejected("identity")
	metrics.ChannelOpened()
	metrics.ChannelClosed()
	metrics.Reauthorization("accepted")

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"bex_ssh_gateway_active_channels",
		"bex_ssh_gateway_active_sessions",
		"bex_ssh_gateway_authentications_total",
		"bex_ssh_gateway_channel_reauthorizations_total",
		"bex_ssh_gateway_channels_total",
		"bex_ssh_gateway_handshakes_total",
		"bex_ssh_gateway_limit_rejections_total",
		"bex_ssh_gateway_session_duration_seconds",
		"bex_ssh_gateway_sessions_total",
	}
	got := make([]string, 0, len(families))
	for _, family := range families {
		got = append(got, family.GetName())
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() != "result" && label.GetName() != "scope" {
					t.Fatalf("metric %s has privacy-unsafe/unbounded label %q", family.GetName(), label.GetName())
				}
			}
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("metric families = %v, want %v", got, want)
	}
}
