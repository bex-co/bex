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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w3/m523: a project view's IS_NOT on Level, Instance, Method, Status Code,
// Host and Path reads every line its IS form does not, a line without the
// label included, on the store, the pod-log and datastore paths and the live
// tail.

// The store renders the complement matcher of each field: the same value
// pattern as the selecting form, with `!=` or `!~`.
func TestNegatedFiltersRenderTheComplementMatcher(t *testing.T) {
	request := []string{LogTypeRequest}
	for _, tc := range []struct {
		name string
		q    LogQuery
		want string
	}{
		{"level", LogQuery{Types: []string{LogTypeApp}, Level: []string{"info"}, negated: []string{LabelLevel}},
			`{namespace="default", app="web", container="app", level!="info"}`},
		{"a Render level name maps to its stored buckets first", LogQuery{Types: []string{LogTypeApp}, Level: []string{"warning"}, negated: []string{LabelLevel}},
			`{namespace="default", app="web", container="app", level!~"^(warning|warn)$"}`},
		{"instance", LogQuery{Types: []string{LogTypeApp}, Instance: []string{"web-1"}, negated: []string{LabelInstance}},
			`{namespace="default", app="web", container="app", pod!="web-1"}`},
		{"method", LogQuery{Types: request, Method: []string{"GET", "HEAD"}, negated: []string{LabelMethod}},
			`{namespace="default", app="web", type="request", method!~"^(GET|HEAD)$"}`},
		{"a status class expands before it negates", LogQuery{Types: request, StatusCode: []string{"4xx"}, negated: []string{LabelStatusCode}},
			`{namespace="default", app="web", type="request", status!~"^(4.*)$"}`},
		{"path", LogQuery{Types: request, Path: []string{"/healthz"}, negated: []string{LabelPath}},
			`{namespace="default", app="web", type="request"} | json request_path="RequestPath", request_host="RequestHost" | request_path!="/healthz"`},
		{"a host wildcard", LogQuery{Types: request, Host: []string{"*.example.com"}, negated: []string{LabelHost}},
			`{namespace="default", app="web", type="request"} | json request_path="RequestPath", request_host="RequestHost" | request_host!~"^(.*\\.example\\.com)$"`},
		{"a path regex stays unanchored", LogQuery{Types: request, Path: []string{"/api/"}, negated: []string{LabelPath}},
			`{namespace="default", app="web", type="request"} | json request_path="RequestPath", request_host="RequestHost" | request_path!~"^(.*(?:api).*)$"`},
		{"only the negated field inverts", LogQuery{Types: request, Method: []string{"GET"}, StatusCode: []string{"5xx"}, Path: []string{"/x"}, Host: []string{"h"}, negated: []string{LabelStatusCode, LabelHost}},
			`{namespace="default", app="web", type="request", method="GET", status!~"^(5.*)$"} | json request_path="RequestPath", request_host="RequestHost" | request_path="/x" | request_host!="h"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.q.App = "web"
			if got := lokiQueryFor("default", tc.q.normalized()); got != tc.want {
				t.Errorf("lokiQueryFor:\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// negationLine is one line of the negation project: where it was written,
// what its container wrote, what a read shows, and the level the shipper
// stores for it ("" for none).
type negationLine struct {
	resource, pod string
	at            int // seconds after fixtureDay
	raw, shown    string
	level         string
}

func pgRecord(at int, severity, message string) string {
	return fmt.Sprintf(`{"logger":"postgres","msg":"record","record":{"log_time":"2026-07-05 00:00:%02d UTC","process_id":7,"error_severity":%q,"message":%q}}`, at, severity, message)
}

// negationCorpus holds every level shape a project view reads: an App line at
// info and at warning, an unstructured one (the shipper's `unknown`) and one
// whose level is empty (no level at all), on two instances; a cron job's; a
// Postgres LOG (info) and ERROR record; a Valkey notice (info) and warning.
var negationCorpus = []negationLine{
	{projectWeb, "web-1", 1, `{"level":"info","msg":"web info"}`, "", "info"},
	{projectWeb, "web-1", 2, `{"level":"warn","msg":"web warning"}`, "", "warning"},
	{projectWeb, "web-2", 3, "web plain", "", "unknown"},
	{projectWeb, "web-2", 4, `{"level":"","msg":"web no level"}`, "", ""},
	{projectCron, "nightly-1", 5, `{"level":"info","msg":"cron info"}`, "", "info"},
	{projectCron, "nightly-1", 6, "cron plain", "", "unknown"},
	{postgresID, postgresID + "-1", 7, pgRecord(7, "LOG", "checkpoint complete"), "2026-07-05 00:00:07 UTC [7] LOG:  checkpoint complete", "info"},
	{postgresID, postgresID + "-1", 8, pgRecord(8, "ERROR", "deadlock detected"), "2026-07-05 00:00:08 UTC [7] ERROR:  deadlock detected", "error"},
	{keyValueID, keyValueID + "-0", 9, "1:M 05 Jul 2026 00:00:09.000 * Ready to accept connections", "", "info"},
	{keyValueID, keyValueID + "-0", 10, "1:M 05 Jul 2026 00:00:10.000 # WARNING overcommit_memory is set to 0", "", "warning"},
}

func (l negationLine) message() string {
	if l.shown != "" {
		return l.shown
	}
	return l.raw
}

func (l negationLine) key() string { return l.resource + ":" + l.message() }

// stream is the line as the store holds it, labelled as the shipper labels
// each kind of resource.
func (l negationLine) stream(day time.Time) fakeStream {
	labels := map[string]string{"namespace": "default", "pod": l.pod}
	switch l.resource {
	case postgresID:
		labels["database"] = postgresID
	case keyValueID:
		labels["keyvalue"] = keyValueID
	default:
		labels["app"], labels["container"], labels["type"] = l.resource, "app", LogTypeApp
	}
	if l.level != "" {
		labels["level"] = l.level
	}
	msg := l.message()
	return fakeStream{labels: labels, stamps: []int64{day.Add(time.Duration(l.at) * time.Second).UnixNano()}, text: func(int64) string { return msg }}
}

// negationProject is a project with a web service on two instances, a cron
// job, a Postgres and a Key Value, whose pods wrote negationCorpus and whose
// store holds it.
func negationProject(t *testing.T) *Service {
	t.Helper()
	day, _ := time.Parse(time.RFC3339, fixtureDay)
	lines := map[string][]string{}
	var streams []fakeStream
	for _, l := range negationCorpus {
		lines[l.pod] = append(lines[l.pod], day.Add(time.Duration(l.at)*time.Second).Format(time.RFC3339)+" "+l.raw)
		streams = append(streams, l.stream(day))
	}
	cron := sampleApp(projectCron)
	cron.Spec.Type = appv1alpha1.TypeCronJob
	svc := newService(lines,
		sampleApp(projectWeb), podFor(projectWeb, "web-1"), podFor(projectWeb, "web-2"),
		cron, podFor(projectCron, "nightly-1"),
		sampleDatabase(postgresID), databasePod(postgresID, postgresID+"-1"),
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}}, keyValuePod(keyValueID, keyValueID+"-0"))
	f := newFrontendLoki(t, streams...)
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.Projects = &fakeProject{workspace: "tea-home", resources: []savedsearch.Resource{
		{ID: projectWeb, Name: "storefront", Type: appv1alpha1.TypeWebService},
		{ID: projectCron, Name: "nightly-report", Type: appv1alpha1.TypeCronJob},
		{ID: postgresID, Name: "orders-db", Type: savedsearch.KindPostgres},
		{ID: keyValueID, Name: "sessions", Type: savedsearch.KindKeyValue},
	}}
	return svc
}

// viewDocument is a one-filter view document in GraphQL's literal syntax and
// in JSON, for the tail's query string.
func viewDocument(field, operator string, values ...string) (gql, raw string) {
	quoted, _ := json.Marshal(values)
	return fmt.Sprintf(`{filters: [{field: %q, operator: %q, values: %s}], version: 1}`, field, operator, quoted),
		fmt.Sprintf(`{"filters":[{"field":%q,"operator":%q,"values":%s}],"version":1}`, field, operator, quoted)
}

// projectHistory reads the view's lines from GraphQL `logs(project, …)`, as
// "resource:message" in time order.
func projectHistory(t *testing.T, svc *Service, viewType, doc, start, end string) []string {
	t.Helper()
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, fmt.Sprintf(`{ logs(project:"prj-observed", type:%q, direction:"forward", limit:100, startTime:%q, endTime:%q, filterDocument:%s) { logs { message resource } } }`, viewType, start, end, doc))
	page, _ := data["logs"].(map[string]any)
	var out []string
	for _, row := range page["logs"].([]any) {
		r := row.(map[string]any)
		out = append(out, fmt.Sprintf("%s:%s", r["resource"], r["message"]))
	}
	return out
}

// projectTail follows the view on /v1/logs/subscribe?project= for a moment and
// returns its lines as "resource:message", sorted. The client leaves by
// cancelling, not by a deadline: a store poll keeps a margin back from a
// deadline and would read nothing under a short one.
func projectTail(t *testing.T, svc *Service, viewType, doc string, wait time.Duration) []string {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ctx, cancel := context.WithCancel(core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"}))
	defer cancel()
	q := url.Values{"project": {"prj-observed"}, "type": {viewType}, "filterDocument": {doc}}
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+q.Encode(), nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(rec, req)
	}()
	time.Sleep(wait)
	cancel()
	<-done
	if rec.Code != http.StatusOK {
		t.Fatalf("project tail %s => %d %s", doc, rec.Code, rec.Body.String())
	}
	var out []string
	for resource, messages := range tailResources(t, rec.Body.String()) {
		for _, m := range messages {
			out = append(out, resource+":"+m)
		}
	}
	slices.Sort(out)
	return out
}

func corpusKeys(keep func(negationLine) bool) []string {
	var out []string
	for _, l := range negationCorpus {
		if keep(l) {
			out = append(out, l.key())
		}
	}
	return out
}

func sortedCopy(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// assertComplement checks that the IS_NOT read is want and holds exactly the
// lines of the whole read the IS read does not.
func assertComplement(t *testing.T, what string, is, isNot, all, want []string) {
	t.Helper()
	if !slices.Equal(sortedCopy(isNot), sortedCopy(want)) {
		t.Errorf("%s IS_NOT read\n %q\nwant %q", what, isNot, want)
	}
	union := sortedCopy(append(slices.Clone(is), isNot...))
	if !slices.Equal(union, sortedCopy(all)) {
		t.Errorf("%s: IS %q and IS_NOT %q together are not the whole view %q", what, is, isNot, all)
	}
	for _, line := range is {
		if slices.Contains(isNot, line) {
			t.Errorf("%s: %q is read by both IS and IS_NOT", what, line)
		}
	}
}

// DoD 1: an Application Logs view of `LEVEL IS_NOT [info]` reads every line of
// the project's services, cron jobs, Postgres and Key Value whose level is not
// info, the unstructured line and the line with no level included, on GraphQL
// and on the project tail.
func TestProjectViewReadsEveryLineButInfo(t *testing.T) {
	svc := negationProject(t)
	isGQL, _ := viewDocument("LEVEL", "IS", "info")
	notGQL, notRaw := viewDocument("LEVEL", "IS_NOT", "info")
	_, isRaw := viewDocument("LEVEL", "IS", "info")
	const start, end = fixtureDay, "2026-07-05T00:01:00Z"
	all := corpusKeys(func(negationLine) bool { return true })
	want := corpusKeys(func(l negationLine) bool { return l.level != "info" })

	history := projectHistory(t, svc, LogTypeApp, notGQL, start, end)
	if !slices.Equal(history, want) {
		t.Fatalf("GraphQL LEVEL IS_NOT info, in time order:\n %q\nwant %q", history, want)
	}
	assertComplement(t, "GraphQL level", projectHistory(t, svc, LogTypeApp, isGQL, start, end), history,
		projectHistory(t, svc, LogTypeApp, `{filters: [], version: 1}`, start, end), want)

	tail := projectTail(t, svc, LogTypeApp, notRaw, 400*time.Millisecond)
	assertComplement(t, "tail level", projectTail(t, svc, LogTypeApp, isRaw, 400*time.Millisecond), tail, all, want)
}

// DoD 2: a negated instance excludes its own pod's lines and leaves every
// other resource's, where it resolves to nothing; one that matches no pod
// anywhere excludes nothing.
func TestProjectViewNegatesAnInstanceInItsOwnResourceOnly(t *testing.T) {
	svc := negationProject(t)
	const start, end = fixtureDay, "2026-07-05T00:01:00Z"
	all := corpusKeys(func(negationLine) bool { return true })
	for _, tc := range []struct {
		name     string
		instance string
		want     []string
	}{
		{"one web instance", ids.ServiceInstanceID(projectWeb, "web-1"), corpusKeys(func(l negationLine) bool { return l.pod != "web-1" })},
		{"no such instance", ids.ServiceInstanceID(projectWeb, "web-9"), all},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isGQL, isRaw := viewDocument("INSTANCE", "IS", tc.instance)
			notGQL, notRaw := viewDocument("INSTANCE", "IS_NOT", tc.instance)
			assertComplement(t, "GraphQL instance", projectHistory(t, svc, LogTypeApp, isGQL, start, end),
				projectHistory(t, svc, LogTypeApp, notGQL, start, end), all, tc.want)
			assertComplement(t, "tail instance", projectTail(t, svc, LogTypeApp, isRaw, 400*time.Millisecond),
				projectTail(t, svc, LogTypeApp, notRaw, 400*time.Millisecond), all, tc.want)
		})
	}
}

// requestCorpus is a web service's request lines, stamped just now so the
// tail's store poll, which reads its last minute, finds them.
type requestRecord struct {
	id, method, path, host, status string
}

var requestCorpus = []requestRecord{
	{"r1", "GET", "/healthz", "web.onbex.co", "200"},
	{"r2", "GET", "/api/users", "web.onbex.co", "404"},
	{"r3", "POST", "/api/orders", "api.example.com", "500"},
	{"r4", "HEAD", "/", "web.onbex.co", "301"},
	{"r5", "GET", "/static/app.js", "cdn.example.com", "200"},
}

// requestProject is a project whose web service's store holds requestCorpus,
// written over the last 30 seconds, and the read window around it.
func requestProject(t *testing.T) (svc *Service, start, end string) {
	t.Helper()
	now := time.Now().UTC()
	var streams []fakeStream
	for i, r := range requestCorpus {
		record, _ := json.Marshal(map[string]any{"RequestHost": r.host, "RequestPath": r.path, "request_Rndr-Id": r.id})
		msg := string(record)
		streams = append(streams, fakeStream{
			labels: map[string]string{"namespace": "default", "app": projectWeb, "type": LogTypeRequest, "method": r.method, "status": r.status, "level": statusLevel(r.status)},
			stamps: []int64{now.Add(time.Duration(i-30) * time.Second).UnixNano()},
			text:   func(int64) string { return msg },
		})
	}
	svc = newService(nil, sampleApp(projectWeb), podFor(projectWeb, "web-1"))
	f := newFrontendLoki(t, streams...)
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.RequestPollInterval = 5 * time.Millisecond
	svc.Projects = &fakeProject{workspace: "tea-home", resources: []savedsearch.Resource{{ID: projectWeb, Name: "storefront", Type: appv1alpha1.TypeWebService}}}
	return svc, now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Second).Format(time.RFC3339)
}

func requestKeys(ids ...string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("%s:requestID=%q", projectWeb, id)
	}
	return out
}

// DoD 2: on a Request Logs view, IS_NOT on Method, Status Code (a class too),
// Host and Path, and on Level, reads the complement of IS, wildcards and
// regexes negating the same way, on GraphQL and on the project tail. A value
// that matches nothing excludes nothing.
func TestProjectRequestViewNegatesEveryRequestField(t *testing.T) {
	svc, start, end := requestProject(t)
	all := requestKeys("r1", "r2", "r3", "r4", "r5")
	for _, tc := range []struct {
		field  string
		values []string
		want   []string
	}{
		{"METHOD", []string{"GET"}, requestKeys("r3", "r4")},
		{"METHOD", []string{"GET", "HEAD"}, requestKeys("r3")},
		{"STATUS_CODE", []string{"2xx"}, requestKeys("r2", "r3", "r4")},
		{"STATUS_CODE", []string{"4xx", "500"}, requestKeys("r1", "r4", "r5")},
		{"HOST", []string{"*.example.com"}, requestKeys("r1", "r2", "r4")},
		{"PATH", []string{"/healthz"}, requestKeys("r2", "r3", "r4", "r5")},
		{"PATH", []string{"/api/"}, requestKeys("r1", "r4", "r5")}, // a regex: every path holding "api"
		{"PATH", []string{"/nowhere"}, all},
		{"LEVEL", []string{"info"}, requestKeys("r2", "r3")},
	} {
		t.Run(tc.field+" "+strings.Join(tc.values, ","), func(t *testing.T) {
			isGQL, isRaw := viewDocument(tc.field, "IS", tc.values...)
			notGQL, notRaw := viewDocument(tc.field, "IS_NOT", tc.values...)
			assertComplement(t, "GraphQL", projectHistory(t, svc, LogTypeRequest, isGQL, start, end),
				projectHistory(t, svc, LogTypeRequest, notGQL, start, end), all, tc.want)
			assertComplement(t, "tail", projectTail(t, svc, LogTypeRequest, isRaw, 300*time.Millisecond),
				projectTail(t, svc, LogTypeRequest, notRaw, 300*time.Millisecond), all, tc.want)
		})
	}
}

// A datastore read without the store, from its pods' process log, and an
// App's pod-log read invert their level and instance matching for a negated
// set: a line with no level passes a negated level, and a negated instance
// that resolves to no pod excludes nothing.
func TestPodLogPathsInvertNegatedFilters(t *testing.T) {
	svc := negationProject(t)
	svc.History = nil
	day, _ := time.Parse(time.RFC3339, fixtureDay)
	read := func(q LogQuery) []string {
		t.Helper()
		q.Since = day
		entries, err := svc.QueryLogs(context.Background(), q)
		if err != nil {
			t.Fatalf("QueryLogs(%+v): %v", q, err)
		}
		var out []string
		for _, e := range entries {
			out = append(out, e.Labels["service"]+":"+e.Message)
		}
		return out
	}
	resource := func(id string, keep func(negationLine) bool) []string {
		return corpusKeys(func(l negationLine) bool { return l.resource == id && keep(l) })
	}
	for id, pod := range map[string]string{postgresID: postgresID + "-1", keyValueID: keyValueID + "-0"} {
		if got, want := read(LogQuery{App: id, Level: []string{"info"}, negated: []string{LabelLevel}}),
			resource(id, func(l negationLine) bool { return l.level != "info" }); !slices.Equal(got, want) {
			t.Errorf("%s level IS_NOT info = %q, want %q", id, got, want)
		}
		if got := read(LogQuery{App: id, Instance: []string{ids.ServiceInstanceID(id, pod)}, negated: []string{LabelInstance}}); len(got) != 0 {
			t.Errorf("%s instance IS_NOT its only pod = %q, want nothing", id, got)
		}
		if got, want := read(LogQuery{App: id, Instance: []string{"srv-elsewhere-abcde"}, negated: []string{LabelInstance}}),
			resource(id, func(negationLine) bool { return true }); !slices.Equal(got, want) {
			t.Errorf("%s instance IS_NOT an unknown instance = %q, want every line %q", id, got, want)
		}
	}
	if got, want := read(LogQuery{App: projectWeb, Instance: []string{ids.ServiceInstanceID(projectWeb, "web-1")}, negated: []string{LabelInstance}}),
		resource(projectWeb, func(l negationLine) bool { return l.pod != "web-1" }); !slices.Equal(got, want) {
		t.Errorf("web instance IS_NOT web-1 = %q, want %q", got, want)
	}
	if got, want := read(LogQuery{App: projectWeb, Instance: []string{"web-9"}, negated: []string{LabelInstance}}),
		resource(projectWeb, func(negationLine) bool { return true }); !slices.Equal(got, want) {
		t.Errorf("web instance IS_NOT an unknown instance = %q, want every line %q", got, want)
	}
}

// withFilter is q with values on the structured filter named label.
func withFilter(q LogQuery, label string, values ...string) LogQuery {
	*map[string]*[]string{
		LabelLevel: &q.Level, LabelInstance: &q.Instance, LabelMethod: &q.Method,
		LabelStatusCode: &q.StatusCode, LabelHost: &q.Host, LabelPath: &q.Path,
	}[label] = values
	return q
}

// A negated request filter routes and refuses as its selecting form does:
// it follows request lines alone, and a datastore refuses it.
func TestNegatedRequestFiltersRouteAsTheirSelectingForm(t *testing.T) {
	for _, label := range []string{LabelMethod, LabelStatusCode, LabelHost, LabelPath} {
		q := withFilter(LogQuery{App: "web", negated: []string{label}}, label, "x")
		routed, err := q.liveSubscribable()
		if err != nil || !slices.Equal(routed.Types, []string{LogTypeRequest}) {
			t.Errorf("negated %s tail = types %v, %v; want request lines", label, routed.Types, err)
		}
		q.App = postgresID
		if err := q.datastoreFilters(postgresID); err == nil {
			t.Errorf("a Postgres read took a negated %s filter", label)
		}
	}
}

// On the pinned Loki (BEX_TEST_LOKI_URL), each negated field reads exactly the
// lines of the whole read its selecting form does not: a stream without the
// label (an App line with no level) and a request line without the field
// (an access record with no path or host) pass a negated matcher, a status
// class and a level name expand before they negate, and a wildcard or a
// regex negates what it matches.
func TestNegatedFiltersReadTheComplementOnThePinnedLoki(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	ns := fmt.Sprintf("m523-%d", time.Now().UnixNano())
	now := time.Now()
	since := now.Add(-time.Minute)
	n := 0
	push := func(labels map[string]string, line string) {
		t.Helper()
		n++
		labels["namespace"], labels["app"] = ns, "web"
		values := [][2]string{{strconv.FormatInt(since.Add(time.Duration(n)*time.Second).UnixNano(), 10), line}}
		body, _ := json.Marshal(map[string]any{"streams": []any{map[string]any{"stream": labels, "values": values}}})
		resp, err := http.Post(base+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("push: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			t.Fatalf("push: HTTP %d", resp.StatusCode)
		}
	}
	app := func(pod, level, line string) {
		labels := map[string]string{"container": "app", "type": LogTypeApp, "pod": pod}
		if level != "" {
			labels["level"] = level
		}
		push(labels, line)
	}
	app("web-1", "info", "app info")
	app("web-1", "warn", "app warn")
	app("web-2", "unknown", "app unknown")
	app("web-2", "", "app no level")
	request := func(id, method, status string, record map[string]any) {
		record["request_Rndr-Id"] = id
		line, _ := json.Marshal(record)
		push(map[string]string{"type": LogTypeRequest, "method": method, "status": status, "level": statusLevel(status)}, string(line))
	}
	request("r1", "GET", "200", map[string]any{"RequestPath": "/healthz", "RequestHost": "web.onbex.co"})
	request("r2", "GET", "404", map[string]any{"RequestPath": "/api/users", "RequestHost": "web.onbex.co"})
	request("r3", "POST", "500", map[string]any{"RequestPath": "/api/orders", "RequestHost": "api.example.com"})
	request("r4", "HEAD", "301", map[string]any{"RequestPath": "/", "RequestHost": "cdn.example.com"})
	request("r5", "GET", "200", map[string]any{}) // an access record with neither field

	read := func(q LogQuery) []string {
		t.Helper()
		q.App, q.Since, q.End, q.Limit, q.Direction = "web", since, now, 100, DirectionForward
		entries, err := NewLokiSource(base, nil)(context.Background(), ns, q.normalized())
		if err != nil {
			t.Fatalf("%+v: %v", q, err)
		}
		return entryStamps(entries)
	}
	appLines := []string{LogTypeApp}
	requestLines := []string{LogTypeRequest}
	for _, tc := range []struct {
		types  []string
		label  string
		values []string
		want   []string
	}{
		{appLines, LabelLevel, []string{"info"}, []string{"app warn", "app unknown", "app no level"}},
		{appLines, LabelLevel, []string{"warning", "error"}, []string{"app info", "app unknown", "app no level"}},
		{appLines, LabelInstance, []string{"web-1"}, []string{"app unknown", "app no level"}},
		{requestLines, LabelLevel, []string{"info"}, []string{`requestID="r2"`, `requestID="r3"`}},
		{requestLines, LabelMethod, []string{"GET"}, []string{`requestID="r3"`, `requestID="r4"`}},
		{requestLines, LabelStatusCode, []string{"4xx"}, []string{`requestID="r1"`, `requestID="r3"`, `requestID="r4"`, `requestID="r5"`}},
		{requestLines, LabelStatusCode, []string{"2*", "500"}, []string{`requestID="r2"`, `requestID="r4"`}},
		{requestLines, LabelHost, []string{"*.example.com"}, []string{`requestID="r1"`, `requestID="r2"`, `requestID="r5"`}},
		{requestLines, LabelPath, []string{"/healthz", "/"}, []string{`requestID="r2"`, `requestID="r3"`, `requestID="r5"`}},
		{requestLines, LabelPath, []string{"/api/"}, []string{`requestID="r1"`, `requestID="r4"`, `requestID="r5"`}},
		{requestLines, LabelPath, []string{"/nowhere"}, []string{`requestID="r1"`, `requestID="r2"`, `requestID="r3"`, `requestID="r4"`, `requestID="r5"`}},
	} {
		is := withFilter(LogQuery{Types: tc.types}, tc.label, tc.values...)
		isNot := is
		isNot.negated = []string{tc.label}
		assertComplement(t, fmt.Sprintf("%v %s %v", tc.types, tc.label, tc.values), read(is), read(isNot), read(LogQuery{Types: tc.types}), tc.want)
	}
}

// A pre-deploy read resolves its instances among the step's own pods: a
// negated one excludes the pod it names, and one that names no pod there
// excludes nothing rather than emptying the read as the selecting form does.
func TestPreDeployReadInvertsANegatedInstance(t *testing.T) {
	svc := newService(map[string][]string{
		"predeploy-web-gen-3": {"2026-07-14T00:00:01Z gen 3"},
		"predeploy-web-gen-4": {"2026-07-14T00:00:02Z gen 4"},
	}, sampleApp("web"), preDeployPod("web", "predeploy-web-gen-3"), preDeployPod("web", "predeploy-web-gen-4"))
	read := func(instance string, negated bool) []string {
		t.Helper()
		q := LogQuery{App: "web", Types: []string{LogTypePreDeploy}, Instance: []string{instance}}
		if negated {
			q.negated = []string{LabelInstance}
		}
		entries, err := svc.QueryLogs(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		return entryStamps(entries)
	}
	if got := read("predeploy-web-gen-3", true); !slices.Equal(got, []string{"gen 4"}) {
		t.Errorf("IS_NOT gen 3 = %q, want gen 4's line", got)
	}
	if got := read("predeploy-web-gen-9", true); !slices.Equal(got, []string{"gen 3", "gen 4"}) {
		t.Errorf("IS_NOT an unknown pod = %q, want every line", got)
	}
	if got := read("predeploy-web-gen-9", false); len(got) != 0 {
		t.Errorf("IS an unknown pod = %q, want none", got)
	}
}
