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
	"math"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
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
	started  []time.Time
}

type cpuReading struct {
	seconds float64
	sampled bool
	err     error
}

func (f *fakeCPU) PodCPUSeconds(_ context.Context, namespace, pod string, started, from, to time.Time) (float64, bool, error) {
	f.started = append(f.started, started)
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

// promSeries is one series a fakeProm holds: scrape times in Unix seconds and
// the counter's value at each.
type promSeries [][2]float64

// fakeProm is a Prometheus that holds the given series of the sandbox pod
// counter and answers the instant queries a CPU read can make over them: the
// raw range selector, and the count_over_time() and increase() of it that the
// meter asked for before w22/024. increase() is Prometheus' own arithmetic,
// extrapolation included, so a test can show what that query read.
type fakeProm struct {
	series  []promSeries
	status  int
	queries []string
}

var promQueryRE = regexp.MustCompile(`^(?:sum\((count_over_time|increase)\()?sandbox_pod_cpu_usage_seconds_total\{namespace="tea-a-sandbox",pod="os-1-0"\}\[(\d+)s\]\)*$`)

func (f *fakeProm) start(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		f.queries = append(f.queries, q+" @"+r.URL.Query().Get("time"))
		if f.status != 0 {
			http.Error(w, "down", f.status)
			return
		}
		m := promQueryRE.FindStringSubmatch(q)
		end, err := strconv.ParseFloat(r.URL.Query().Get("time"), 64)
		if m == nil || err != nil {
			http.Error(w, "the fake does not evaluate "+q, http.StatusBadRequest)
			return
		}
		seconds, _ := strconv.ParseFloat(m[2], 64)
		var matrix []string
		total, any := 0.0, false
		for _, series := range f.series {
			var in promSeries
			var values []string
			for _, s := range series {
				if s[0] > end-seconds && s[0] <= end {
					in = append(in, s)
					values = append(values, fmt.Sprintf(`[%.3f,"%v"]`, s[0], s[1]))
				}
			}
			if len(in) == 0 {
				continue
			}
			matrix = append(matrix, `{"metric":{},"values":[`+strings.Join(values, ",")+`]}`)
			switch m[1] {
			case "count_over_time":
				total, any = total+float64(len(in)), true
			case "increase":
				if len(in) >= 2 {
					total, any = total+promIncrease(in, end-seconds, end), true
				}
			}
		}
		switch {
		case m[1] == "":
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[%s]}}`, strings.Join(matrix, ","))
		case any:
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[%v,"%v"]}]}}`, end, total)
		default:
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// promIncrease is Prometheus' increase() over the samples of one counter in
// (from, to]: the growth between the first and the last sample, extended to
// the window's edges at the average rate (promql's extrapolatedRate).
func promIncrease(in promSeries, from, to float64) float64 {
	first, last := in[0], in[len(in)-1]
	growth, prev := last[1]-first[1], first[1]
	for _, s := range in[1:] {
		if s[1] < prev {
			growth += prev
		}
		prev = s[1]
	}
	sampled := last[0] - first[0]
	average := sampled / float64(len(in)-1)
	toStart, toEnd := first[0]-from, to-last[0]
	if toStart >= average*1.1 {
		toStart = average / 2
	}
	if toZero := sampled * (first[1] / growth); growth > 0 && toZero < toStart {
		toStart = toZero
	}
	if toEnd >= average*1.1 {
		toEnd = average / 2
	}
	return growth * (sampled + toStart + toEnd) / sampled
}

// busyThenIdle is the busy sandbox's pod counter as production's Prometheus
// held it for sandbox-measure run 44867679299 (w22/024): five minutes of one
// busy core, about 20 s idle, then the pause. Its running interval was
// 04:47:29Z to 04:53:02Z on 2026-11-21, and the last sample came after it.
var busyThenIdle = promSeries{
	{1795236454.598, 0.947664}, {1795236469.182, 9.126307}, {1795236482.596, 22.539885}, {1795236512.801, 52.745231},
	{1795236527.943, 67.233761}, {1795236547.828, 87.120128}, {1795236562.045, 101.340394}, {1795236579.992, 119.285067},
	{1795236596.980, 135.62262}, {1795236612.841, 151.483105}, {1795236625.255, 163.897553}, {1795236638.742, 177.383315},
	{1795236657.272, 195.258726}, {1795236676.649, 214.637294}, {1795236690.565, 228.550983}, {1795236703.292, 240.590077},
	{1795236713.707, 251.003068}, {1795236726.233, 263.529866}, {1795236741.317, 278.616057}, {1795236758.294, 295.589056},
	{1795236771.987, 298.42295}, {1795236799.995, 298.531882},
}

const (
	busyStarted = 1795236449 // 04:47:29Z
	busyEnded   = 1795236782 // 04:53:02Z
)

// readCPU reads the window from..to of a running interval that began at started.
func readCPU(t *testing.T, prom *fakeProm, started, from, to int64) (float64, bool) {
	t.Helper()
	got, sampled, err := PrometheusCPU{Base: prom.start(t)}.PodCPUSeconds(context.Background(), "tea-a-sandbox", "os-1-0", time.Unix(started, 0), time.Unix(from, 0), time.Unix(to, 0))
	if err != nil {
		t.Fatalf("read %d..%d: %v", from, to, err)
	}
	return got, sampled
}

func near(got, want float64) bool { return math.Abs(got-want) < 1e-6 }

// w22/024: a sandbox that is busy and then idle before it stops is read at no
// more than its pod's counter. increase() over the same window read 307.81
// CPU-seconds from a counter that never passed 298.54, because it filled the
// ten seconds after the last scrape at the window's average rate.
func TestPrometheusCPUNeverReadsMoreThanThePodCounter(t *testing.T) {
	prom := &fakeProm{series: []promSeries{busyThenIdle}}
	got, sampled := readCPU(t, prom, busyStarted, busyStarted, busyEnded)
	lastInWindow, counterTotal := 298.42295, busyThenIdle[len(busyThenIdle)-1][1]
	if !sampled || !near(got, lastInWindow) {
		t.Errorf("reading = %v sampled=%v, want the counter's last value in the window, %v", got, sampled, lastInWindow)
	}
	if got > counterTotal {
		t.Errorf("reading %v is more than the pod's counter ever showed (%v)", got, counterTotal)
	}
	want := `sandbox_pod_cpu_usage_seconds_total{namespace="tea-a-sandbox",pod="os-1-0"}[633s] @` + fmt.Sprint(busyEnded)
	if strings.Join(prom.queries, "\n") != want {
		t.Errorf("queries =\n%s\nwant\n%s", strings.Join(prom.queries, "\n"), want)
	}
	// The fake's increase() is the figure production's Prometheus gave.
	if over := promIncrease(busyThenIdle[:21], busyStarted, busyEnded); math.Abs(over-307.80771140861214) > 1e-6 {
		t.Errorf("the fake's increase() = %v, production's read 307.80771140861214", over)
	}
}

// Consecutive windows add up to the counter's last scraped value, wherever
// the interval is cut: each starts from the last sample before it, and a first
// window too short to hold a sample leaves the new pod's counter to the next.
func TestPrometheusCPUWindowsAddUpToTheCounter(t *testing.T) {
	for _, cut := range []int64{busyStarted + 1, 1795236482, 1795236600, 1795236772, busyEnded - 1} {
		head, _ := readCPU(t, &fakeProm{series: []promSeries{busyThenIdle}}, busyStarted, busyStarted, cut)
		tail, _ := readCPU(t, &fakeProm{series: []promSeries{busyThenIdle}}, busyStarted, cut, busyEnded)
		if !near(head+tail, 298.42295) {
			t.Errorf("cut at %d: %v + %v = %v, want 298.42295", cut, head, tail, head+tail)
		}
	}
}

func TestPrometheusCPUSeriesWithNothingBeforeTheWindow(t *testing.T) {
	// The counter of a pod that has run for days, with no scrape in the five
	// minutes before the window.
	old := promSeries{{1000, 90000}, {1500, 90040}, {1515, 90055}, {1530, 90070}}
	for _, tc := range []struct {
		name     string
		series   []promSeries
		from, to int64
		age      int64 // seconds from the interval's start to the window's
		want     float64
		sampled  bool
	}{
		// A scrape gap (or a relabelled series, or a Prometheus that lost its
		// history) is never read as a counter that started at zero.
		{"later window after a scrape gap", []promSeries{old}, 1400, 1600, 3600, 30, true},
		{"later window, last sample before it inside the lookback", []promSeries{old}, 1200, 1600, 3600, 70, true},
		{"later window, one sample and nothing before", []promSeries{old}, 1400, 1510, 3600, 0, true},
		// A new pod's counter started at zero with the pod.
		{"first window counts a new pod from zero", []promSeries{{{1505, 3}, {1520, 9}}}, 1500, 1600, 0, 9, true},
		{"first window, one sample", []promSeries{{{1505, 3}}}, 1500, 1600, 0, 3, true},
		{"first window of a pod scraped before it", []promSeries{{{1490, 2}, {1505, 3}, {1520, 9}}}, 1500, 1600, 0, 7, true},
		{"a window five minutes into the interval still reads a new pod", []promSeries{{{1505, 3}, {1520, 9}}}, 1500, 1600, 300, 9, true},
		{"a window later than that does not", []promSeries{{{1505, 3}, {1520, 9}}}, 1500, 1600, 301, 6, true},
		// A sample after the window is another window's.
		{"samples after the window are not read", []promSeries{{{1505, 3}, {1520, 9}, {1601, 50}}}, 1500, 1600, 0, 9, true},
		{"a sample at the window's start belongs to the window before", []promSeries{{{1500, 4}, {1520, 9}}}, 1500, 1600, 3600, 5, true},
		{"a sample at the window's end is read", []promSeries{{{1490, 4}, {1600, 9}}}, 1500, 1600, 3600, 5, true},
		// A counter that restarts counts from zero from the restart.
		{"counter restart", []promSeries{{{1490, 100}, {1505, 110}, {1520, 4}, {1535, 6}}}, 1500, 1600, 3600, 16, true},
		// A pod recreated under the same name is a second series.
		{"two series", []promSeries{{{1490, 100}, {1505, 110}}, {{1520, 1}, {1535, 6}}}, 1500, 1600, 3600, 15, true},
		{"no sample in the window", []promSeries{{{1490, 100}}}, 1500, 1600, 3600, 0, false},
		{"no series", nil, 1500, 1600, 0, 0, false},
	} {
		got, sampled := readCPU(t, &fakeProm{series: tc.series}, tc.from-tc.age, tc.from, tc.to)
		if !near(got, tc.want) || sampled != tc.sampled {
			t.Errorf("%s: reading = %v sampled=%v, want %v sampled=%v", tc.name, got, sampled, tc.want, tc.sampled)
		}
	}
}

// An outage or an answer that is not samples is an error, never zero, and a
// name that could not be a pod's never reaches a query.
func TestPrometheusCPURefusesWhatItCannotRead(t *testing.T) {
	ctx := context.Background()
	from, to := time.Unix(1500, 400_000_000), time.Unix(1600, 900_000_000)
	down := &fakeProm{status: http.StatusServiceUnavailable}
	if got, sampled, err := (PrometheusCPU{Base: down.start(t)}).PodCPUSeconds(ctx, "tea-a-sandbox", "os-1-0", from, from, to); err == nil || got != 0 || sampled {
		t.Errorf("outage: reading = %v sampled=%v err=%v, want an error", got, sampled, err)
	}
	for _, body := range []string{
		`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1,"5"]}]}}`,
		`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1505,"NaN"]]}]}}`,
		`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1505,"-1"]]}]}}`,
		`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1520,"1"],[1505,"2"]]}]}}`,
		`{"status":"error","data":{"resultType":"matrix","result":[]}}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, body) }))
		if got, sampled, err := (PrometheusCPU{Base: srv.URL}).PodCPUSeconds(ctx, "tea-a-sandbox", "os-1-0", from, from, to); err == nil || got != 0 || sampled {
			t.Errorf("%s: reading = %v sampled=%v err=%v, want an error", body, got, sampled, err)
		}
		srv.Close()
	}
	prom := &fakeProm{series: []promSeries{busyThenIdle}}
	if _, _, err := (PrometheusCPU{Base: prom.start(t)}).PodCPUSeconds(ctx, "tea-a-sandbox", `x"} or vector(1e9) #`, from, from, to); err == nil || len(prom.queries) != 0 {
		t.Errorf("an invalid pod name was queried: err=%v queries=%v", err, prom.queries)
	}
}

// The reader is told when each window's interval began, which is what lets it
// read a new pod's counter from zero and no other.
func TestCPUMeterTellsTheReaderWhenTheIntervalBegan(t *testing.T) {
	started := time.Date(2026, 8, 3, 10, 59, 0, 0, time.UTC)
	later := cpuWindow("a", started.Add(time.Minute), time.Hour)
	later.StartedAt = started
	cpu := &fakeCPU{readings: map[string]cpuReading{"a-0": {seconds: 1, sampled: true}}}
	m := &Meter{Store: &fakeMeterStore{cpuWindows: []store.SandboxCPUWindow{later}}, CPU: cpu, Now: func() time.Time { return started.Add(3 * time.Hour) }}
	m.settleCPU(context.Background())
	if len(cpu.started) != 1 || !cpu.started[0].Equal(started) {
		t.Fatalf("started = %v, want the interval's start %v, not the window's", cpu.started, started)
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
