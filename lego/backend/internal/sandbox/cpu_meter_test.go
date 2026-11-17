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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeCPU answers PodCPUSeconds per pod name.
type fakeCPU struct {
	readings map[string]cpuReading
	asked    []string
}

type cpuReading struct {
	seconds float64
	sampled bool
	err     error
}

func (f *fakeCPU) PodCPUSeconds(_ context.Context, namespace, pod string, from, to time.Time) (float64, bool, error) {
	f.asked = append(f.asked, fmt.Sprintf("%s/%s %s..%s", namespace, pod, from.Format(time.RFC3339), to.Format(time.RFC3339)))
	r := f.readings[pod]
	return r.seconds, r.sampled, r.err
}

func cpuWindow(id string, from time.Time, length time.Duration) store.SandboxCPUWindow {
	return store.SandboxCPUWindow{WorkspaceID: "tea-a", SandboxID: id, PodName: id + "-0", Tier: "1c-2g",
		StartedAt: from, From: from, To: from.Add(length)}
}

// w22/m314 t003: what the meter does with each kind of answer. A reading is
// added, zero included. No reading is never written as zero: an outage, a
// running pod with no sample and a reading no pod can produce are deferred; a
// window too short to have been scraped and one Prometheus still cannot answer
// for 36 hours later are closed with nothing, the second one counted.
func TestCPUMeterReadsDefersAndGivesUp(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	hourAgo := now.Add(-time.Hour)
	fake := &fakeMeterStore{cpuStats: store.SandboxCPUStats{Pending: 7, Deferred: 3}, cpuWindows: []store.SandboxCPUWindow{
		cpuWindow("busy", hourAgo, 30*time.Minute),
		cpuWindow("idle", hourAgo, 30*time.Minute),
		cpuWindow("outage", hourAgo, 30*time.Minute),
		cpuWindow("unscraped", hourAgo, 30*time.Minute),
		cpuWindow("blink", hourAgo, 20*time.Second),
		cpuWindow("lost", now.Add(-40*time.Hour), 30*time.Minute),
		cpuWindow("corrupt", hourAgo, time.Minute),
	}}
	cpu := &fakeCPU{readings: map[string]cpuReading{
		"busy-0":      {seconds: 632.06, sampled: true},
		"idle-0":      {seconds: 0, sampled: true},
		"outage-0":    {err: errors.New("prometheus: status 503")},
		"unscraped-0": {},
		"blink-0":     {},
		"lost-0":      {},
		"corrupt-0":   {seconds: 60 * 300, sampled: true}, // 300 cores for a minute
	}}
	reg := prometheus.NewRegistry()
	metrics := NewCPUMetrics(reg)
	m := &Meter{Store: fake, CPU: cpu, CPUMetrics: metrics, Now: func() time.Time { return now }}
	m.settleCPU(context.Background())

	if got := fake.cpuAccrued["busy"]; got != 632.06 {
		t.Errorf("busy window accrued %v CPU-seconds, want the measured 632.06 with no subtraction", got)
	}
	for _, id := range []string{"idle", "blink", "lost"} {
		if got, ok := fake.cpuAccrued[id]; !ok || got != 0 {
			t.Errorf("%s window accrued %v (present %v), want it closed with zero", id, got, ok)
		}
	}
	for _, id := range []string{"outage", "unscraped", "corrupt"} {
		if _, ok := fake.cpuAccrued[id]; ok {
			t.Errorf("%s window was written although it has no usable reading", id)
		}
	}
	if got := strings.Join(fake.cpuDeferred, ","); got != "outage,unscraped,corrupt" {
		t.Errorf("deferred = %q, want outage,unscraped,corrupt", got)
	}
	if len(cpu.asked) != 7 || cpu.asked[0] != "tea-a-sandbox/busy-0 2026-08-03T11:00:00Z..2026-08-03T11:30:00Z" {
		t.Errorf("reads = %v, want every window read by pod name in the workspace's sandbox namespace", cpu.asked)
	}
	if got := testutil.ToFloat64(metrics.abandoned); got != 1 {
		t.Errorf("abandoned windows = %v, want 1 (the one with no samples after 36 hours)", got)
	}
	if got := testutil.ToFloat64(metrics.readFailures); got != 2 {
		t.Errorf("read failures = %v, want 2 (the outage and the corrupt reading)", got)
	}
	if got := testutil.ToFloat64(metrics.intervals.WithLabelValues("deferred")); got != 3 {
		t.Errorf("deferred gauge = %v, want the store's 3", got)
	}
	if got := testutil.ToFloat64(metrics.intervals.WithLabelValues("pending")); got != 7 {
		t.Errorf("pending gauge = %v, want the store's 7", got)
	}
}

// A Prometheus that is down fails every read: the poll stops after three in a
// row instead of spending its budget on the rest, and nothing is written.
func TestCPUMeterStopsAfterConsecutiveFailures(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	fake := &fakeMeterStore{}
	cpu := &fakeCPU{readings: map[string]cpuReading{}}
	for i := range 6 {
		id := fmt.Sprintf("sbx%d", i)
		fake.cpuWindows = append(fake.cpuWindows, cpuWindow(id, now.Add(-time.Hour), 30*time.Minute))
		cpu.readings[id+"-0"] = cpuReading{err: errors.New("connection refused")}
	}
	m := &Meter{Store: fake, CPU: cpu, Now: func() time.Time { return now }}
	m.settleCPU(context.Background())
	if len(cpu.asked) != cpuReadFailures || len(fake.cpuAccrued) != 0 || len(fake.cpuDeferred) != cpuReadFailures {
		t.Fatalf("reads = %d, accrued = %v, deferred = %v; want %d reads, each deferred, none written",
			len(cpu.asked), fake.cpuAccrued, fake.cpuDeferred, cpuReadFailures)
	}
}

// Without a CPU source the meter reads nothing: the feature is off.
func TestCPUMeterOffWithoutSource(t *testing.T) {
	fake := &fakeMeterStore{cpuWindows: []store.SandboxCPUWindow{cpuWindow("a", time.Now().Add(-time.Hour), time.Minute)}}
	(&Meter{Store: fake}).settleCPU(context.Background())
	if len(fake.cpuAccrued) != 0 || len(fake.cpuDeferred) != 0 {
		t.Fatalf("a meter with no CPU source wrote %v / deferred %v", fake.cpuAccrued, fake.cpuDeferred)
	}
}

// promStub answers the two instant queries PrometheusCPU makes.
func promStub(t *testing.T, samples, increase string, status int) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		queries = append(queries, q+" @"+r.URL.Query().Get("time"))
		if status != http.StatusOK {
			http.Error(w, "down", status)
			return
		}
		value := increase
		if strings.Contains(q, "count_over_time") {
			value = samples
		}
		if value == "" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1,%q]}]}}`, value)
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

// The reading is the pod-level counter's increase over exactly the window,
// selected by namespace and pod name; one sample is a reading of nothing
// measurable, no sample is no reading, and an outage is an error, never zero.
func TestPrometheusCPUReadsThePodCounterOverTheWindow(t *testing.T) {
	from := time.Date(2026, 8, 3, 11, 0, 0, 400_000_000, time.UTC)
	to := from.Add(10 * time.Minute)
	ctx := context.Background()

	srv, queries := promStub(t, "41", "632.06", http.StatusOK)
	got, sampled, err := PrometheusCPU{Base: srv.URL}.PodCPUSeconds(ctx, "tea-a-sandbox", "os-1-0", from, to)
	if err != nil || !sampled || got != 632.06 {
		t.Fatalf("reading = %v sampled=%v err=%v, want 632.06", got, sampled, err)
	}
	at := fmt.Sprint(to.Unix())
	want := []string{
		`sum(count_over_time(sandbox_pod_cpu_usage_seconds_total{namespace="tea-a-sandbox",pod="os-1-0"}[600s])) @` + at,
		`sum(increase(sandbox_pod_cpu_usage_seconds_total{namespace="tea-a-sandbox",pod="os-1-0"}[600s])) @` + at,
	}
	if strings.Join(*queries, "\n") != strings.Join(want, "\n") {
		t.Errorf("queries =\n%s\nwant\n%s", strings.Join(*queries, "\n"), strings.Join(want, "\n"))
	}

	for _, tc := range []struct {
		name, samples string
		status        int
		sampled, fail bool
	}{
		{"one sample", "1", http.StatusOK, true, false},
		{"no sample", "", http.StatusOK, false, false},
		{"outage", "", http.StatusServiceUnavailable, false, true},
	} {
		srv, _ := promStub(t, tc.samples, "999", tc.status)
		got, sampled, err := PrometheusCPU{Base: srv.URL}.PodCPUSeconds(ctx, "tea-a-sandbox", "os-1-0", from, to)
		if got != 0 || sampled != tc.sampled || (err != nil) != tc.fail {
			t.Errorf("%s: reading = %v sampled=%v err=%v, want 0, sampled=%v, error=%v", tc.name, got, sampled, err, tc.sampled, tc.fail)
		}
	}

	// A name that could not be a pod's never reaches a query.
	srv, queries = promStub(t, "5", "5", http.StatusOK)
	if _, _, err := (PrometheusCPU{Base: srv.URL}).PodCPUSeconds(ctx, "tea-a-sandbox", `x"} or vector(1e9) #`, from, to); err == nil || len(*queries) != 0 {
		t.Errorf("an invalid pod name was queried: err=%v queries=%v", err, *queries)
	}
}

// The pod read is `<substrate id>-0`, and only a Render sandbox is billed on
// use: an agent session's keeps the weighted meter.
func TestMeterObservationCarriesPodAndBillingModel(t *testing.T) {
	raw := osSandbox{ID: "os-1", Metadata: map[string]string{
		metadataWorkspace: "tea-a", metadataPlan: "1c-2g", metadataRegime: metadataSandboxRegime,
		metadataComputeWeight: "1211", metadataPublicID: "sbx-public",
	}}
	raw.Status.State = "Running"
	obs, ok := meterObservation(raw, time.Now())
	if !ok || obs.PodName != "os-1-0" || !obs.UsageBilled || obs.SandboxID != "sbx-public" {
		t.Fatalf("Render sandbox observation = %+v ok=%v, want pod os-1-0 billed on use", obs, ok)
	}
	raw.Metadata[metadataAgentSession] = "ags-1"
	if obs, ok := meterObservation(raw, time.Now()); !ok || obs.UsageBilled || obs.PodName != "os-1-0" {
		t.Fatalf("agent session sandbox observation = %+v ok=%v, want the weighted meter", obs, ok)
	}
}
