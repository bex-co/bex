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

package controller

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/tsdb"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests evaluate the production activity query with the PromQL engine
// production runs — github.com/prometheus/prometheus v0.54.1 is Prometheus
// 2.54.1, the version sweep 71 read from the live server (w4/m164) — over
// series shaped like the ones that sweep recorded, on the real absolute
// timeline, so the subquery grid lands where it landed live. (The GitOps
// workflow's promtool is 2.55.1; this pin follows the server, not that tool.)
// Until w4/m164 the activity tests answered from a fake that returned a chosen
// timestamp, which pinned the query's text but not what Prometheus makes of it.

// legacyActivityQuery is the query before w4/m164, kept to show both captured
// failures reproduce against it.
func legacyActivityQuery(app *appv1alpha1.App, lookback time.Duration) string {
	rose := func(series string) string { return "(sum(increase(" + series + "[1m])) > 0)" }
	service := traefikServiceLabel(app.Namespace, app.Name, app.Spec.EffectivePort())
	appID := strconv.Quote(appIDOrName(app))
	return "max_over_time(timestamp(" +
		rose("traefik_service_requests_total{service="+strconv.Quote(service)+"}") + " or " +
		rose("bex_websocket_egress_bytes_total{app_id="+appID+"}") + " or " +
		rose("bex_websocket_ingress_bytes_total{app_id="+appID+"}") + ")[" +
		strconv.Itoa(int(math.Ceil(max(lookback, time.Minute).Seconds()))) + "s:15s])"
}

// counter is one scraped series: its samples, in time order. A NaN value is a
// staleness marker (a failed scrape or a vanished target).
type counter struct {
	labels  labels.Labels
	samples []sample
}

type sample struct {
	at time.Time
	v  float64
}

var stale = math.Float64frombits(value.StaleNaN)

// at is a sweep-71 wall-clock instant on 2026-10-04 UTC, "15:04:05.000".
func at(t *testing.T, clock string) time.Time {
	t.Helper()
	ts, err := time.Parse("2006-01-02T15:04:05.000Z", "2026-10-04T"+clock+"Z")
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// scrapes samples values one scrape interval apart from first.
func scrapes(first time.Time, values ...float64) []sample {
	out := make([]sample, len(values))
	for i, v := range values {
		out[i] = sample{first.Add(time.Duration(i) * activityStep), v}
	}
	return out
}

// between is a counter that read v at every scrape from first through last.
func between(first, last time.Time, v float64) []sample {
	var out []sample
	for ts := first; !ts.After(last); ts = ts.Add(activityStep) {
		out = append(out, sample{ts, v})
	}
	return out
}

// flat is a counter that has read v at every scrape for the 30 minutes up to
// and including last.
func flat(last time.Time, v float64) []sample { return between(last.Add(-30*time.Minute), last, v) }

func then(a, b []sample) []sample { return append(append([]sample{}, a...), b...) }

func activitySeries(app *appv1alpha1.App, metric string, extra ...string) labels.Labels {
	kv := []string{labels.MetricName, metric, "job", "traefik", "instance", "10.244.36.122:9100", "pod", "traefik-776c44f94f-l9bfd"}
	if metric == "traefik_service_requests_total" {
		kv = append(kv, "service", traefikServiceLabel(app.Namespace, app.Name, app.Spec.EffectivePort()),
			"method", "GET", "code", "200", "protocol", "http")
	} else {
		kv = append(kv, "app_id", appIDOrName(app))
	}
	b := labels.NewBuilder(labels.FromStrings(kv...))
	for i := 0; i+1 < len(extra); i += 2 {
		b.Set(extra[i], extra[i+1])
	}
	return b.Labels()
}

// promStore loads the series into a real TSDB.
func promStore(t *testing.T, series []counter) *tsdb.DB {
	t.Helper()
	db, err := tsdb.Open(t.TempDir(), nil, nil, tsdb.DefaultOptions(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := db.Appender(context.Background())
	for _, s := range series {
		for _, p := range s.samples {
			if _, err := app.Append(0, s.labels, p.at.UnixMilli(), p.v); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := app.Commit(); err != nil {
		t.Fatal(err)
	}
	return db
}

var promEngine = promql.NewEngine(promql.EngineOpts{
	MaxSamples: 1_000_000, Timeout: 10 * time.Second,
	EnableAtModifier: true, EnableNegativeOffset: true, // Prometheus server defaults
	NoStepSubqueryIntervalFn: func(int64) int64 { return activityStep.Milliseconds() },
})

// evalQuery evaluates query at ts and returns its samples' values.
func evalQuery(db *tsdb.DB, query string, ts time.Time) ([]float64, error) {
	q, err := promEngine.NewInstantQuery(context.Background(), db, nil, query, ts)
	if err != nil {
		return nil, err
	}
	defer q.Close()
	res := q.Exec(context.Background())
	if res.Err != nil {
		return nil, res.Err
	}
	vec, err := res.Vector()
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(vec))
	for i, s := range vec {
		out[i] = s.F
	}
	return out, nil
}

// latestActivity is the reader's answer: the newest returned instant after
// since, or zero.
func latestActivity(t *testing.T, db *tsdb.DB, query string, now, since time.Time) time.Time {
	t.Helper()
	values, err := evalQuery(db, query, now)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	var latest time.Time
	for _, v := range values {
		if ts := time.UnixMilli(int64(math.Round(v * 1000))).UTC(); ts.After(latest) {
			latest = ts
		}
	}
	if !latest.After(since) {
		return time.Time{}
	}
	return latest
}

func TestActivityQueryUnderPrometheus(t *testing.T) {
	app := activityApp(time.Time{})
	reqs := func(extra ...string) labels.Labels {
		return activitySeries(app, "traefik_service_requests_total", extra...)
	}
	podB := []string{"pod", "traefik-776c44f94f-pjhp2", "instance", "10.244.35.149:9100"}
	type tc struct {
		name       string
		series     []counter
		since, now string // decision clock times; window is ttl
		ttl        time.Duration
		want       string // "" = no activity
		legacyWant string // what the pre-w4/m164 query answered
	}
	cases := []tc{
		{
			// Fixture B: the sole GET at 01:05:13.843 created a series that read
			// 1 at 01:05:14.727 and stayed 1; the service slept at 01:05:35,
			// 21.2s after it.
			name:   "sweep 71 B: absent→1→1, a fresh service's only request",
			series: []counter{{reqs(), scrapes(at(t, "01:05:14.727"), 1, 1)}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:15.000",
		},
		{
			// Fixture A: GET 00:53:46.38 and HEAD 00:53:49.18 began series on
			// two Traefik pods at 1 and stayed flat; the service slept at
			// 00:58:19, 270s after HEAD.
			name: "sweep 71 A: first-only GET and HEAD on distinct pods",
			series: []counter{
				{reqs(), scrapes(at(t, "00:53:59.727"), 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)},
				{reqs(append([]string{"method", "HEAD"}, podB...)...), scrapes(at(t, "00:54:03.738"), 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1)},
			},
			since: "00:53:19.000", now: "00:58:19.000", ttl: 5 * time.Minute,
			want: "00:54:15.000",
		},
		{
			// The steady control: a recreated series read 1, 1, then 2 at
			// 01:09:08.738. At 01:09:13 the instant increase was 1.39, but the
			// last 15s step was 01:09:00, and the service got a wake 503.
			name:   "sweep 71 steady control: a rise scraped after the last aligned step",
			series: []counter{{reqs(podB...), scrapes(at(t, "01:08:38.738"), 1, 1, 2)}},
			since:  "01:08:13.000", now: "01:09:13.000", ttl: time.Minute,
			want: "01:09:13.000",
		},
		{
			name:   "established counter rises after the last aligned step",
			series: []counter{{reqs(), then(flat(at(t, "01:08:53.738"), 3), scrapes(at(t, "01:09:08.738"), 4))}},
			since:  "01:08:13.000", now: "01:09:13.000", ttl: time.Minute,
			want: "01:09:13.000",
		},
		{
			name:   "established counter rises on the grid: both queries see it",
			series: []counter{{reqs(), then(flat(at(t, "01:08:23.738"), 3), scrapes(at(t, "01:08:38.738"), 4, 4))}},
			since:  "01:08:13.000", now: "01:09:13.000", ttl: time.Minute,
			want: "01:09:13.000", legacyWant: "01:09:00.000",
		},
		{
			name:   "zero→1: a series exported at zero, then the request",
			series: []counter{{reqs(), scrapes(at(t, "01:05:14.727"), 0, 1)}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:35.000", legacyWant: "01:05:30.000",
		},
		{
			name:   "a new labelset beside a flat one: a first 404",
			series: []counter{{reqs(), flat(at(t, "01:05:29.727"), 9)}, {reqs("code", "404"), scrapes(at(t, "01:05:14.727"), 1, 1)}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:15.000",
		},
		{
			name:   "a pre-existing positive counter that stays flat is not traffic",
			series: []counter{{reqs(), flat(at(t, "01:20:29.727"), 7)}, {reqs(podB...), flat(at(t, "01:20:33.738"), 2)}},
			since:  "01:05:35.000", now: "01:20:35.000", ttl: 15 * time.Minute,
		},
		{
			name:   "a counter reset to zero is not traffic",
			series: []counter{{reqs(), then(flat(at(t, "01:04:44.727"), 7), scrapes(at(t, "01:04:59.727"), 0, 0, 0))}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
		},
		{
			name:   "a reset to a positive value is the request the new process counted",
			series: []counter{{reqs(), then(flat(at(t, "01:04:44.727"), 7), scrapes(at(t, "01:04:59.727"), 1, 1, 1))}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:35.000", legacyWant: "01:05:30.000",
		},
		{
			name: "missed and jittered scrapes of a flat counter are not traffic",
			series: []counter{{reqs(), then(flat(at(t, "01:04:14.727"), 7), []sample{
				{at(t, "01:04:59.981"), 7}, {at(t, "01:05:31.002"), 7}, // 45s, then 31s apart
			})}},
			since: "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
		},
		{
			name:   "a sparse first sample whose next scrape was missed still counts",
			series: []counter{{reqs(), []sample{{at(t, "01:05:01.311"), 1}}}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:15.000",
		},
		{
			// A failed scrape writes staleness markers; the series reappearing at
			// its old value looks new once, for one step: a bounded extension,
			// never a service kept awake by a flat counter.
			name: "a stale flat series reappearing counts once",
			series: []counter{{reqs(), then(flat(at(t, "01:04:44.727"), 7),
				[]sample{{at(t, "01:04:59.727"), stale}, {at(t, "01:05:14.727"), 7}, {at(t, "01:05:29.727"), 7}})}},
			since: "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:15.000",
		},
		{
			name: "…and not again in the next window",
			series: []counter{{reqs(), then(flat(at(t, "01:04:44.727"), 7),
				then([]sample{{at(t, "01:04:59.727"), stale}}, scrapes(at(t, "01:05:14.727"), 7, 7, 7, 7, 7, 7)))}},
			since: "01:05:15.000", now: "01:06:15.000", ttl: time.Minute,
		},
		{
			name:   "traffic older than the window is not reported",
			series: []counter{{reqs(), then(flat(at(t, "00:59:14.727"), 3), between(at(t, "00:59:29.727"), at(t, "01:05:29.727"), 4))}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
		},
		{
			name:   "a client-only WebSocket's first frames count",
			series: []counter{{activitySeries(app, "bex_websocket_ingress_bytes_total"), scrapes(at(t, "01:05:14.727"), 31, 31)}},
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
			want: "01:05:15.000",
		},
		{
			name: "a server-only WebSocket's frames count, after the last aligned step",
			series: []counter{{activitySeries(app, "bex_websocket_egress_bytes_total"),
				then(flat(at(t, "01:09:00.738"), 512), scrapes(at(t, "01:09:15.738"), 640))}},
			since: "01:08:20.000", now: "01:09:20.000", ttl: time.Minute,
			want: "01:09:20.000",
		},
		{
			name:   "no series at all (no traffic, plugin counters absent)",
			series: nil,
			since:  "01:04:35.000", now: "01:05:35.000", ttl: time.Minute,
		},
	}
	clock := func(s string) time.Time {
		if s == "" {
			return time.Time{}
		}
		return at(t, s)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := promStore(t, c.series)
			since, now := clock(c.since), clock(c.now)
			// The lookback exactly as NewPrometheusAppActivityReader bounds it.
			lookback := min(now.Sub(since), c.ttl+activityStep)
			if got := latestActivity(t, db, activityQuery(app, lookback), now, since); !got.Equal(clock(c.want)) {
				t.Errorf("activity = %v, want %v", got, clock(c.want))
			}
			if got := latestActivity(t, db, legacyActivityQuery(app, lookback), now, since); !got.Equal(clock(c.legacyWant)) {
				t.Errorf("legacy activity = %v, want %v (the case no longer describes the old query)", got, clock(c.legacyWant))
			}
		})
	}
}

// sweepProm is Prometheus's HTTP query API over a TSDB holding a sweep
// timeline, answering every query at that timeline's decision instant. The
// controller decides on the wall clock, so the App's stamps are written skew
// earlier than the timeline's and every instant the query returns is shifted
// back by the same skew: the production reader, recentlyActive and
// shouldAutoHibernate all run unmodified.
type sweepProm struct {
	t    *testing.T
	db   *tsdb.DB
	srv  *httptest.Server
	mu   sync.Mutex
	now  time.Time
	skew time.Duration // timeline minus wall clock, in whole seconds
}

func newSweepProm(t *testing.T, series []counter) *sweepProm {
	p := &sweepProm{t: t, db: promStore(t, series)}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		now, skew := p.now, p.skew
		p.mu.Unlock()
		values, err := evalQuery(p.db, r.URL.Query().Get("query"), now)
		if err != nil {
			t.Errorf("prometheus: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		type result struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		}
		results := make([]result, 0, len(values))
		for _, v := range values {
			results = append(results, result{map[string]string{}, []any{
				float64(now.Unix()), strconv.FormatFloat(v-skew.Seconds(), 'f', -1, 64)}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success", "data": map[string]any{"resultType": "vector", "result": results},
		})
	}))
	t.Cleanup(p.srv.Close)
	return p
}

// decide runs the controller's idle decision at the timeline instant now for a
// free web service with window ttl last stamped at stamp, and returns whether
// it sleeps and its stamp afterwards, on the timeline.
func (p *sweepProm) decide(ttl time.Duration, stamp, now time.Time) (bool, time.Time) {
	t := p.t
	t.Helper()
	// Rounding the skew up makes the wall-clock age of the stamp at least its
	// age on the timeline, so a window that has elapsed there has elapsed here.
	skew := time.Until(now)
	if r := skew % time.Second; r != 0 {
		skew += time.Second - r
	}
	p.mu.Lock()
	p.now, p.skew = now, skew
	p.mu.Unlock()

	app := activityApp(stamp.Add(-skew))
	app.Spec.IdleTTLSeconds = int32(ttl / time.Second)
	r, cl := activityReconciler(t, app, NewPrometheusAppActivityReader(p.srv.URL, p.srv.Client()))
	_, _, plan := r.desiredReplicas(context.Background(), app, effectiveReplicas(app), releaseObservation{})
	sleeping := plan.autoHibernating
	after, err := time.Parse(time.RFC3339, storedStamp(t, cl, app))
	if err != nil {
		t.Fatal(err)
	}
	return sleeping, after.Add(skew)
}

// sleepAt follows the controller from stamp, deciding each time the window
// elapses by the stamp (runningRequeue's schedule), and returns when it puts
// the service to sleep.
func (p *sweepProm) sleepAt(ttl time.Duration, stamp time.Time) time.Time {
	t := p.t
	t.Helper()
	for range 200 {
		now := stamp.Add(ttl)
		sleeping, after := p.decide(ttl, stamp, now)
		if sleeping {
			return now
		}
		if !after.After(stamp) {
			t.Fatalf("awake at %v without advancing the stamp %v", now, stamp)
		}
		stamp = after
	}
	t.Fatal("the service never slept")
	return time.Time{}
}

// The sweep 71 journeys through the production reader and idle decision: each
// service stays awake for its full window after its last request, then sleeps
// within bounded scrape and query slack.
func TestIdleDecisionUnderPrometheus(t *testing.T) {
	app := activityApp(time.Time{})
	reqs := func(extra ...string) labels.Labels {
		return activitySeries(app, "traefik_service_requests_total", extra...)
	}
	podB := []string{"pod", "traefik-776c44f94f-pjhp2", "instance", "10.244.35.149:9100"}
	// slack bounds how long past the window a quiet service may stay awake: a
	// minute of increase() evidence, a scrape and a step.
	const slack = time.Minute + 2*activityStep
	within := func(t *testing.T, slept, last time.Time, ttl time.Duration) {
		t.Helper()
		if slept.Before(last.Add(ttl)) || slept.After(last.Add(ttl+slack)) {
			t.Fatalf("slept at %v, want within [%v, %v] (last request %v)", slept, last.Add(ttl), last.Add(ttl+slack), last)
		}
	}

	t.Run("sweep 71 B: the sole GET of a fresh service at TTL 60", func(t *testing.T) {
		p := newSweepProm(t, []counter{{reqs(), between(at(t, "01:05:14.727"), at(t, "01:10:14.727"), 1)}})
		// Before w4/m164 this decision put the service to sleep 21.2s after the GET.
		sleeping, stamp := p.decide(time.Minute, at(t, "01:04:35.000"), at(t, "01:05:35.000"))
		if sleeping || !stamp.Equal(at(t, "01:05:15.000")) {
			t.Fatalf("at 01:05:35 sleeping=%v stamp=%v, want awake, stamped at the GET's first scrape step", sleeping, stamp)
		}
		within(t, p.sleepAt(time.Minute, stamp), at(t, "01:05:13.843"), time.Minute)
	})

	t.Run("sweep 71 A: first GET and HEAD on distinct pods at TTL 300", func(t *testing.T) {
		p := newSweepProm(t, []counter{
			{reqs(), between(at(t, "00:53:59.727"), at(t, "01:05:00.000"), 1)},
			{reqs(append([]string{"method", "HEAD"}, podB...)...), between(at(t, "00:54:03.738"), at(t, "01:05:00.000"), 1)},
		})
		within(t, p.sleepAt(5*time.Minute, at(t, "00:53:19.000")), at(t, "00:53:49.180"), 5*time.Minute)
	})

	// Steady traffic on a woken service, on a fresh series as after sweep 71's
	// wake: no sleep while it lasts, a sleep once it stops. m151's control at
	// the default window, and sweep 71's TTL-60 control.
	for _, c := range []struct {
		name       string
		ttl, every time.Duration
		requests   int
		first      time.Duration // after the wake
	}{
		{"every 15s for 20 minutes, default window", 15 * time.Minute, 15 * time.Second, 80, 8 * time.Second},
		{"every 20s across several TTL-60 windows", time.Minute, 20 * time.Second, 15, 21 * time.Second},
	} {
		t.Run(c.name, func(t *testing.T) {
			wake := at(t, "01:08:13.000")
			requests := make([]time.Time, c.requests)
			for i := range requests {
				requests[i] = wake.Add(c.first + time.Duration(i)*c.every)
			}
			last := requests[len(requests)-1]
			var scraped []sample
			for s := at(t, "01:08:08.738"); s.Before(last.Add(time.Hour)); s = s.Add(activityStep) {
				n := 0
				for _, r := range requests {
					if !r.After(s) {
						n++
					}
				}
				if n > 0 {
					scraped = append(scraped, sample{s, float64(n)})
				}
			}
			p := newSweepProm(t, []counter{{reqs(podB...), scraped}})
			within(t, p.sleepAt(c.ttl, wake), last, c.ttl)
		})
	}
}
