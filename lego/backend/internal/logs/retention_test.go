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

package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w9/m389: every log read reaches back exactly as far as the workspace plan
// keeps logs — Hobby 7 days, Pro 14, Scale and Enterprise 30
// (render.com/docs/logging) — on every surface, and an upgrade reveals nothing
// that had already aged out.

const day = 24 * time.Hour

// retentionAges are the ages, in days, of the fixture's lines: one each side
// of every plan's edge, and one past Scale's.
var retentionAges = []int{1, 6, 7, 8, 13, 14, 15, 20, 29, 30, 31, 35}

// agedStamp is the instant of the line retentionAges names by age: half an
// hour inside the day, so a line of age N sits inside an N-day window even
// after the test's own seconds have passed.
func agedStamp(now time.Time, age int) time.Time {
	return now.Add(-time.Duration(age)*day + 30*time.Minute)
}

// ageOf reads a fixture line's age back from its timestamp.
func ageOf(t *testing.T, now time.Time, stamp string) int {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatalf("timestamp %q: %v", stamp, err)
	}
	return int((now.Sub(ts) + time.Hour) / day)
}

// retentionService is App "web" in workspace tea-a on plan, with one stored
// line per retention age behind the real Loki source and the same lines on
// its pod for the tail. kept is the floor the workspace's last plan change
// kept (zero: none). plan "" wires no plan reader at all.
func retentionService(t *testing.T, plan string, kept time.Time) (*Service, time.Time) {
	t.Helper()
	now := time.Now()
	stamps := make([]time.Time, len(retentionAges))
	for i, age := range retentionAges {
		stamps[i] = agedStamp(now, age)
	}
	store := exclusiveEndLoki(t, stamps)
	t.Cleanup(store.Close)
	labels := retentionLabelLoki(t, now)

	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	database := sampleDatabase(postgresID)
	database.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc := newService(nil, app, podFor("web", webInst), database)
	svc.History = NewLokiSource(store.URL, store.Client())
	svc.LabelValues = NewLokiLabelValuesSource(labels.URL, labels.Client())
	if plan != "" {
		svc.Plans = &tenantPlans{plan: plan, visibleFrom: kept}
	}
	return svc, now
}

// tenantPlans is the tenants store's shape (core.WorkspaceLogPlanReader): the
// plan and the floor its last change kept in one read, counting the reads.
type tenantPlans struct {
	plan        string
	visibleFrom time.Time
	mu          sync.Mutex
	reads       int
}

func (p *tenantPlans) WorkspacePlan(ctx context.Context, id string) (string, error) {
	plan, _, err := p.WorkspaceLogPlan(ctx, id)
	return plan, err
}

func (p *tenantPlans) WorkspaceLogPlan(context.Context, string) (string, time.Time, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reads++
	return p.plan, p.visibleFrom, nil
}

func (p *tenantPlans) readCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reads
}

// retentionLabelLoki answers Loki's label-values API with one `level` value
// per fixture line inside the requested [start, end), named by its age, so a
// discovery shows exactly which days the window reached.
func retentionLabelLoki(t *testing.T, now time.Time) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
		end, _ := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
		values := []string{}
		for _, age := range retentionAges {
			if ns := agedStamp(now, age).UnixNano(); ns >= start && ns < end {
				values = append(values, fmt.Sprintf("age%02d", age))
			}
		}
		body, _ := json.Marshal(map[string]any{"status": "success", "data": values})
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// visibleAges are the fixture ages a plan's window keeps, oldest edge
// included.
func visibleAges(maxAge int) []int {
	var out []int
	for _, age := range retentionAges {
		if age <= maxAge {
			out = append(out, age)
		}
	}
	return out
}

func agesOf(t *testing.T, now time.Time, stamps []string) []int {
	t.Helper()
	out := make([]int, 0, len(stamps))
	for _, s := range stamps {
		out = append(out, ageOf(t, now, s))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// forty days back: older than every plan's window, inside no query cap here.
func fortyDaysAgo() string {
	return time.Now().Add(-40 * day).UTC().Format(time.RFC3339)
}

// readEverySurface reads web's logs from forty days ago forward on REST,
// GraphQL and MCP, and its label values on REST, and returns the ages each
// surface saw.
func readEverySurface(t *testing.T, svc *Service, now time.Time) map[string][]int {
	t.Helper()
	start := fortyDaysAgo()
	seen := map[string][]int{}

	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&direction=forward&limit=100&startTime="+start)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST /v1/logs from 40 days ago = %d %s, want a clamped 200", rec.Code, rec.Body.String())
	}
	env := decodeLogList(t, rec)
	var stamps []string
	for _, l := range env.Logs {
		stamps = append(stamps, l.Timestamp)
	}
	seen["REST /v1/logs"] = agesOf(t, now, stamps)

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", direction:"forward", limit:100, startTime:"`+start+`") { logs { timestamp } } }`)
	stamps = nil
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		stamps = append(stamps, row.(map[string]any)["timestamp"].(string))
	}
	seen["GraphQL logs"] = agesOf(t, now, stamps)

	res, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "direction": "forward", "limit": 100, "startTime": start,
	}})
	if err != nil || res.IsError {
		t.Fatalf("MCP list_logs from 40 days ago = %+v, %v; want a clamped result", res, err)
	}
	// list_logs answers the REST GET /v1/logs body (w4/m378).
	var out struct {
		Logs []struct {
			Timestamp string `json:"timestamp"`
		} `json:"logs"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &out); err != nil {
		t.Fatal(err)
	}
	stamps = nil
	for _, e := range out.Logs {
		stamps = append(stamps, e.Timestamp)
	}
	seen["MCP list_logs"] = agesOf(t, now, stamps)

	rec = serveREST(svc, http.MethodGet, "/v1/logs/values?resource=web&label=level&limit=100&startTime="+start)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST /v1/logs/values from 40 days ago = %d %s", rec.Code, rec.Body.String())
	}
	var values []string
	if err := json.Unmarshal(rec.Body.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	var ages []int
	for _, v := range values {
		age, _ := strconv.Atoi(strings.TrimPrefix(v, "age"))
		ages = append(ages, age)
	}
	slices.Sort(ages)
	seen["REST /v1/logs/values"] = ages
	return seen
}

// Each plan's window holds on REST, GraphQL, MCP and label discovery: Hobby
// never reads day 8, Pro never day 15, Scale reaches day 30 and stops before
// 31. A start older than the window is clamped to it, never refused. An
// unknown plan gets the strictest window; with no plan reader (the store-off
// degrade) a read reaches as far as the store keeps.
func TestEveryReadSurfaceStopsAtThePlansWindow(t *testing.T) {
	for _, tc := range []struct {
		plan   string
		maxAge int
	}{
		{"hobby", 7},
		{"pro", 14},
		{"scale", 30},
		{"enterprise", 30},
		{"platinum", 7},
		{"", 35},
	} {
		t.Run("plan="+tc.plan, func(t *testing.T) {
			svc, now := retentionService(t, tc.plan, time.Time{})
			want := visibleAges(tc.maxAge)
			for surface, got := range readEverySurface(t, svc, now) {
				if !slices.Equal(got, want) {
					t.Errorf("%s read ages %v, want %v", surface, got, want)
				}
			}
		})
	}
}

// A window wholly before the floor is an empty page, not an error, and the
// default lookback (no startTime) stays inside every plan's window.
func TestAWindowBeforeTheFloorIsAnEmptyPage(t *testing.T) {
	svc, now := retentionService(t, "hobby", time.Time{})
	from := now.Add(-20 * day).UTC().Format(time.RFC3339)
	to := now.Add(-10 * day).UTC().Format(time.RFC3339)
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&startTime="+from+"&endTime="+to)
	if rec.Code != http.StatusOK {
		t.Fatalf("a window wholly before the floor = %d %s, want 200", rec.Code, rec.Body.String())
	}
	if env := decodeLogList(t, rec); len(env.Logs) != 0 || env.HasMore {
		t.Fatalf("a window wholly before the floor = %d lines, hasMore=%v; want an empty last page", len(env.Logs), env.HasMore)
	}

	env := decodeLogList(t, serveREST(svc, http.MethodGet, "/v1/logs?resource=web&limit=100"))
	var stamps []string
	for _, l := range env.Logs {
		stamps = append(stamps, l.Timestamp)
	}
	if got := agesOf(t, now, stamps); !slices.Equal(got, visibleAges(7)) {
		t.Fatalf("default lookback ages %v, want %v", got, visibleAges(7))
	}
}

// Backward paging walks to the floor and stops there: the last page says
// hasMore=false instead of sending the client into lines it may not read.
func TestBackwardPagingEndsAtTheFloor(t *testing.T) {
	svc, now := retentionService(t, "pro", time.Time{})
	path := "/v1/logs?resource=web&limit=2&startTime=" + fortyDaysAgo()
	var stamps []string
	for range 10 {
		env := decodeLogList(t, serveREST(svc, http.MethodGet, path))
		for _, l := range env.Logs {
			stamps = append(stamps, l.Timestamp)
		}
		if !env.HasMore {
			if got := agesOf(t, now, stamps); !slices.Equal(got, visibleAges(14)) {
				t.Fatalf("paged ages %v, want %v", got, visibleAges(14))
			}
			return
		}
		path = "/v1/logs?resource=web&limit=2&startTime=" + env.NextStartTime + "&endTime=" + env.NextEndTime
	}
	t.Fatal("paging never reached its last page")
}

// t004: after a Hobby -> Pro upgrade at T, nothing older than T-7d returns
// until T+7d; once the kept floor lapses, Pro's full 14 days apply. A
// downgrade narrows at once.
func TestAnUpgradeRevealsNothingThatHadAgedOut(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name           string
		oldPlan, plan  string
		changedDaysAgo int
		maxAge         int
	}{
		// Upgraded just now: day 8 stays hidden as on Hobby.
		{"hobby->pro just now", "hobby", "pro", 0, 7},
		// Upgraded two days ago: the floor is (now-2d)-7d, so the fixture's
		// day-8 line is the oldest, not its day-14 one.
		{"hobby->pro two days ago", "hobby", "pro", 2, 9},
		// Upgraded eight days ago: (now-8d)-7d is older than Pro's own floor,
		// so Pro's 14 days apply.
		{"hobby->pro eight days ago", "hobby", "pro", 8, 14},
		// Pro's floor at the upgrade, (now-3d)-14d, holds under Scale.
		{"pro->scale three days ago", "pro", "scale", 3, 17},
		{"scale->hobby just now", "scale", "hobby", 0, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := now.Add(-time.Duration(tc.changedDaysAgo) * day)
			kept := core.LogsVisibleFromAfterPlanChange(tc.oldPlan, tc.plan, time.Time{}, at)
			svc, readAt := retentionService(t, tc.plan, kept)
			want := visibleAges(tc.maxAge)
			for surface, got := range readEverySurface(t, svc, readAt) {
				if !slices.Equal(got, want) {
					t.Errorf("%s read ages %v, want %v", surface, got, want)
				}
			}
		})
	}
}

// Datastore logs and build logs take the same floor: a managed Postgres from
// its own workspace's plan (datastoreTarget), a build read from the App's.
func TestDatastoreAndBuildLogsStopAtThePlansWindow(t *testing.T) {
	svc, now := retentionService(t, "hobby", time.Time{})
	since := now.Add(-40 * day)
	for name, q := range map[string]LogQuery{
		"postgres": {App: postgresID, Since: since, Limit: 100, Direction: DirectionForward},
		"build":    {App: "web", Types: []string{LogTypeBuild}, Since: since, Limit: 100, Direction: DirectionForward},
	} {
		entries, err := svc.QueryLogs(context.Background(), q)
		if err != nil {
			t.Fatalf("%s from 40 days ago: %v", name, err)
		}
		var stamps []string
		for _, e := range entries {
			stamps = append(stamps, e.Timestamp)
		}
		if got := agesOf(t, now, stamps); !slices.Equal(got, visibleAges(7)) {
			t.Errorf("%s read ages %v, want %v", name, got, visibleAges(7))
		}
	}
	values, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: postgresID, Since: since})
	if err != nil || slices.Contains(values, "age08") || !slices.Contains(values, "age07") {
		t.Fatalf("postgres level values = %v, %v; want day 7 and not day 8", values, err)
	}
}

// agedLogStream serves a pod's log as the fixture's lines, oldest first, and
// records the since bound each follow asked the kubelet for.
type agedLogStream struct {
	now    time.Time
	mu     sync.Mutex
	sinces []time.Time
}

func (s *agedLogStream) follow(_ context.Context, _, _, _ string, since time.Time) (io.ReadCloser, error) {
	s.mu.Lock()
	s.sinces = append(s.sinces, since)
	s.mu.Unlock()
	var lines []string
	for i := len(retentionAges) - 1; i >= 0; i-- {
		lines = append(lines, agedStamp(s.now, retentionAges[i]).UTC().Format(time.RFC3339Nano)+" age")
	}
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n"))), nil
}

// The live tail (the verb behind the WebSocket, SSE and NDJSON subscribe
// transports) takes the floor too, for an App and a datastore alike: a
// startTime older than the plan's window replays nothing older than the
// window, and the kubelet is asked to start there rather than send the rest.
func TestTheLiveTailStartsNoEarlierThanThePlansWindow(t *testing.T) {
	now := time.Now()
	for _, resource := range []string{"web", postgresID} {
		t.Run(resource, func(t *testing.T) {
			stream := &agedLogStream{now: now}
			app := sampleApp("web")
			app.Labels = map[string]string{core.LabelTenant: "tea-a"}
			database := sampleDatabase(postgresID)
			database.Labels = map[string]string{core.LabelTenant: "tea-a"}
			svc := tailService(stream.follow, app, appPodIn("web", webInst, corev1.PodRunning, containerRunning, 0),
				database, databasePod(postgresID, postgresID+"-1"))
			svc.Plans = &tenantPlans{plan: "pro"}

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			var mu sync.Mutex
			var stamps []string
			err := svc.FollowLogs(ctx, LogQuery{App: resource, Since: now.Add(-40 * day)}, func(e LogEntry) error {
				mu.Lock()
				defer mu.Unlock()
				stamps = append(stamps, e.Timestamp)
				return nil
			})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("tail ended with %v, want the client leaving", err)
			}
			mu.Lock()
			got := agesOf(t, now, stamps)
			mu.Unlock()
			// A datastore line without a recognized record is dropped by its
			// parser; the App tail keeps every stamped line.
			if resource == "web" && !slices.Equal(got, visibleAges(14)) {
				t.Errorf("tail ages %v, want %v", got, visibleAges(14))
			}
			if slices.ContainsFunc(got, func(age int) bool { return age > 14 }) {
				t.Errorf("tail replayed ages %v, past Pro's 14 days", got)
			}
			stream.mu.Lock()
			defer stream.mu.Unlock()
			if len(stream.sinces) == 0 {
				t.Fatal("the tail never followed the pod")
			}
			for _, since := range stream.sinces {
				if floor := now.Add(-14 * day); since.Before(floor.Add(-time.Minute)) {
					t.Errorf("kubelet asked from %s, before the floor %s", since.UTC(), floor.UTC())
				}
			}
		})
	}
}

// The pod-log fallback (no store wired) floors each line in Go, for the
// filtered query and the unfiltered tail read alike.
func TestThePodLogFallbackDropsLinesPastTheWindow(t *testing.T) {
	now := time.Now()
	var lines []string
	for i := len(retentionAges) - 1; i >= 0; i-- {
		lines = append(lines, agedStamp(now, retentionAges[i]).UTC().Format(time.RFC3339Nano)+" age")
	}
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc := newService(map[string][]string{webInst: lines}, app, podFor("web", webInst))
	svc.Plans = &tenantPlans{plan: "hobby"}

	queried, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	tailed, err := svc.Logs(context.Background(), "web", 100)
	if err != nil {
		t.Fatal(err)
	}
	for name, entries := range map[string][]LogEntry{"QueryLogs": queried, "Logs": tailed} {
		var stamps []string
		for _, e := range entries {
			stamps = append(stamps, e.Timestamp)
		}
		if got := agesOf(t, now, stamps); !slices.Equal(got, visibleAges(7)) {
			t.Errorf("%s pod-log ages %v, want %v", name, got, visibleAges(7))
		}
	}
}

// The dashboard reads the window it bounds its range picker by from GraphQL's
// logRetention: the plan, its days, the cheapest plan that keeps logs longer,
// and an upgrade's kept floor only while it is later than the plan's own
// window, so the answer holds still between reads. A datastore answers from
// its own workspace; with no plan it is null.
func TestLogRetentionDescribesTheWindowEveryReadApplies(t *testing.T) {
	for _, tc := range []struct {
		plan, resource string
		kept           time.Duration // how long ago the kept floor lies; 0: none
		days           int
		visibleFrom    bool
		upgradePlan    any
		upgradeDays    any
	}{
		{plan: "hobby", resource: "web", days: 7, upgradePlan: "pro", upgradeDays: 14},
		{plan: "pro", resource: postgresID, days: 14, upgradePlan: "scale", upgradeDays: 30},
		{plan: "pro", resource: "web", kept: 9 * day, days: 14, visibleFrom: true, upgradePlan: "scale", upgradeDays: 30},
		// A kept floor Pro's own window has overtaken is no longer reported.
		{plan: "pro", resource: "web", kept: 20 * day, days: 14, upgradePlan: "scale", upgradeDays: 30},
		{plan: "scale", resource: "web", days: 30},
		{plan: "enterprise", resource: "web", days: 30},
	} {
		t.Run(fmt.Sprintf("%s/%s/kept=%s", tc.plan, tc.resource, tc.kept), func(t *testing.T) {
			var kept time.Time
			if tc.kept > 0 {
				kept = time.Now().Add(-tc.kept).Truncate(time.Second)
			}
			svc, _ := retentionService(t, tc.plan, kept)
			schema, err := gqlSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			data := runQuery(t, schema, `{ logRetention(resource:"`+tc.resource+`") { plan days visibleFrom upgradePlan upgradeDays } }`)
			got := data["logRetention"].(map[string]any)
			if got["plan"] != tc.plan || got["days"] != tc.days || got["upgradePlan"] != tc.upgradePlan || got["upgradeDays"] != tc.upgradeDays {
				t.Fatalf("logRetention = %+v", got)
			}
			if !tc.visibleFrom {
				if got["visibleFrom"] != nil {
					t.Fatalf("visibleFrom = %v, want null", got["visibleFrom"])
				}
				return
			}
			if from, err := time.Parse(time.RFC3339, fmt.Sprint(got["visibleFrom"])); err != nil || !from.Equal(kept) {
				t.Fatalf("visibleFrom = %v (%v), want the kept %s", got["visibleFrom"], err, kept.UTC())
			}
		})
	}

	svc, _ := retentionService(t, "", time.Time{})
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	if data := runQuery(t, schema, `{ logRetention(resource:"web") { days } }`); data["logRetention"] != nil {
		t.Fatalf("no plan: logRetention = %+v, want null", data["logRetention"])
	}
}

// A read that cannot reach past the newest any floor can be (now minus the
// shortest plan window) needs no floor: a datastore read reads no plan at
// all, an App read reads it once for its request-log rule and sets no floor,
// so the pod-log fast path keeps skipping the per-line filter.
func TestARecentReadNeedsNoRetentionFloor(t *testing.T) {
	svc, now := retentionService(t, "hobby", time.Time{})
	plans := svc.Plans.(*tenantPlans)
	var floors []time.Time
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		floors = append(floors, q.retentionFloor)
		return nil, nil
	}
	ctx := context.Background()
	hourAgo := now.Add(-time.Hour)

	for _, q := range []LogQuery{{App: postgresID, Since: hourAgo}, {App: postgresID}} {
		if _, err := svc.QueryLogs(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if got := plans.readCount(); got != 0 {
		t.Fatalf("recent datastore reads read the plan %d times, want none", got)
	}
	for _, q := range []LogQuery{{App: "web", Since: hourAgo}, {App: "web"}} {
		if _, err := svc.QueryLogs(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if got := plans.readCount(); got != 2 {
		t.Fatalf("recent App reads read the plan %d times, want once each for the request-log rule", got)
	}
	for i, floor := range floors {
		if !floor.IsZero() {
			t.Fatalf("recent read %d took floor %s, want none", i, floor)
		}
	}

	// A start-less read whose end lies in the past reaches back from that end,
	// past the floor, and takes it.
	floors = nil
	if _, err := svc.QueryLogs(ctx, LogQuery{App: postgresID, End: now.Add(-5 * day)}); err != nil {
		t.Fatal(err)
	}
	if got := plans.readCount(); got != 3 || len(floors) != 1 || floors[0].IsZero() {
		t.Fatalf("an old start-less datastore read: %d plan reads, floors %v; want one read and a floor", got, floors)
	}
}

// A multi-resource read takes each workspace's plan once for the whole page,
// on REST, MCP and label discovery alike.
func TestAMultiResourceReadReadsEachWorkspacePlanOnce(t *testing.T) {
	svc, _ := retentionService(t, "pro", time.Time{})
	plans := svc.Plans.(*tenantPlans)
	other := sampleApp("api")
	other.Labels = map[string]string{core.LabelTenant: "tea-a"}
	if err := svc.Client.Create(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	start := fortyDaysAgo()
	for _, path := range []string{
		"/v1/logs?resource=web&resource=api&limit=100&startTime=" + start,
		"/v1/logs/values?resource=web&resource=api&label=level&startTime=" + start,
	} {
		before := plans.readCount()
		if rec := serveREST(svc, http.MethodGet, path); rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, rec.Code, rec.Body.String())
		}
		if got := plans.readCount() - before; got != 1 {
			t.Errorf("%s read the plan %d times, want once", path, got)
		}
	}
	before := plans.readCount()
	res, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web", "api"}, "limit": 100, "startTime": start,
	}})
	if err != nil || res.IsError {
		t.Fatalf("MCP list_logs = %+v, %v", res, err)
	}
	if got := plans.readCount() - before; got != 1 {
		t.Errorf("MCP list_logs read the plan %d times, want once", got)
	}
}

// The window is the caller's to read only where its logs are: another
// workspace's resource is refused before any plan is read.
func TestLogRetentionIsAuthorizedLikeTheLogs(t *testing.T) {
	database := sampleDatabase(postgresID)
	database.Labels = map[string]string{core.LabelTenant: "tea-other", core.LabelWorkspace: "tea-other"}
	svc := newService(nil, database)
	svc.Base.Workspace = homeWorkspaceOnly{}
	plans := &tenantPlans{plan: "scale"}
	svc.Plans = plans
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	if window, err := svc.LogRetention(ctx, postgresID); !errors.Is(err, core.ErrForbidden) || window != nil {
		t.Fatalf("another workspace's database = %+v, %v; want forbidden", window, err)
	}
	if plans.readCount() != 0 {
		t.Fatal("the plan was read before the caller was authorized")
	}
}

// recentLines moves a fixture's 2026-07-05 lines to yesterday, inside every
// plan's retention window, for tests about something other than retention
// that wire a plan reader.
func recentLines(lines map[string][]string) map[string][]string {
	yesterday := time.Now().UTC().Add(-day).Format("2006-01-02") + "T"
	out := make(map[string][]string, len(lines))
	for pod, ls := range lines {
		for _, l := range ls {
			out[pod] = append(out[pod], strings.Replace(l, "2026-07-05T", yesterday, 1))
		}
	}
	return out
}
