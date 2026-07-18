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
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCopyBidirectionalCountsOnlyBackendToClient(t *testing.T) {
	registry := prometheus.NewRegistry()
	meter := NewByteMeter(registry, "test_proxy", "postgres")
	client, proxyClient := net.Pipe()
	proxyBackend, backend := net.Pipe()
	done := make(chan struct{})
	go func() {
		CopyBidirectional(proxyClient, proxyBackend, meter, "dpg-one", "postgres")
		close(done)
	}()

	request := []byte("client request is deliberately larger than response")
	go func() { _, _ = client.Write(request) }()
	gotRequest := make([]byte, len(request))
	if _, err := io.ReadFull(backend, gotRequest); err != nil {
		t.Fatal(err)
	}
	response := []byte("response")
	go func() { _, _ = backend.Write(response) }()
	gotResponse := make([]byte, len(response))
	if _, err := io.ReadFull(client, gotResponse); err != nil {
		t.Fatal(err)
	}
	_ = backend.Close()
	_ = client.Close()
	<-done

	deadline := time.Now().Add(time.Second)
	for testutil.ToFloat64(meter.responseBytes.WithLabelValues("dpg-one", "postgres")) != float64(len(response)) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := testutil.ToFloat64(meter.responseBytes.WithLabelValues("dpg-one", "postgres")); got != float64(len(response)) {
		t.Fatalf("egress counter = %v, want backend response %d (request must be zero)", got, len(response))
	}
}

func TestByteMeterConcurrentAddsAreRaceSafe(t *testing.T) {
	registry := prometheus.NewRegistry()
	meter := NewByteMeter(registry, "race_proxy", "key_value")
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			meter.Add("kv-one", "key_value", 7)
		})
	}
	wg.Wait()
	if got := testutil.ToFloat64(meter.responseBytes.WithLabelValues("kv-one", "key_value")); got != 700 {
		t.Fatalf("concurrent counter = %v, want 700", got)
	}
}

func TestByteMeterExportsProcessStartForBoundaryResetDetection(t *testing.T) {
	registry := prometheus.NewRegistry()
	NewByteMeter(registry, "test_proxy", "postgres")
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "bex_test_proxy_process_start_time_seconds" {
			if len(family.Metric) != 1 || family.Metric[0].GetGauge().GetValue() <= 0 {
				t.Fatalf("invalid process-start metric: %v", family.Metric)
			}
			return
		}
	}
	t.Fatal("process-start metric was not registered")
}
