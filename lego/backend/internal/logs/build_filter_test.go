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
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m442: Render's 2026-11-16 contract filters logs by build and labels a
// build line with its build. A service build is its deploy (LabelBuild), so a
// build's lines are its deploy's platform lines and its build Job's pod.

// buildPod is a pod of the build Job generation gen of the App web ran.
func buildPod(gen int64, suffix string) string {
	return appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(gen)) + "-" + suffix
}

// kpackPod is a buildpack build's pod, named for its Image, not a build Job.
const kpackPod = "web-image-build-1-build-pod"

// buildStore is a store holding one line from each of scopeRows' build Jobs
// and a kpack pod, inside scopeRead's window. It selects pods as the store's
// selector would (keepPod), and records the query it was given.
func buildStore(got *LogQuery) LogHistorySource {
	lines := map[string]string{
		buildPod(5, "aaaaa"): "#5 building the second",
		buildPod(4, "bbbbb"): "#4 building the first",
		buildPod(3, "ccccc"): "#3 building the old one",
		kpackPod:             "buildpack detect",
	}
	return func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		*got = q
		var out []LogEntry
		at := scopeBase
		for _, pod := range slices.Sorted(maps.Keys(lines)) {
			at = at.Add(time.Millisecond)
			if !q.keepPod(pod) {
				continue
			}
			out = append(out, LogEntry{Timestamp: at.Format(time.RFC3339Nano), Message: lines[pod],
				Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: pod, "service": "web"}})
		}
		return out, nil
	}
}

func buildFilterService(got *LogQuery) *Service {
	svc := scopeService(scopeRows(), nil)
	svc.History = buildStore(got)
	return svc
}

func filteredRead(t *testing.T, svc *Service, q LogQuery) []LogEntry {
	t.Helper()
	q.App = "web"
	q.Since, q.End = scopeBase.Add(-time.Minute), scopeBase.Add(time.Minute)
	entries, err := svc.QueryLogs(context.Background(), q)
	if err != nil {
		t.Fatalf("QueryLogs(%+v): %v", q, err)
	}
	return entries
}

// builds is each line's build, by message.
func builds(entries []LogEntry) map[string]string {
	out := map[string]string{}
	for _, e := range entries {
		out[e.Message] = e.Labels[LabelBuild]
	}
	return out
}

func TestABuildFilterKeepsOnlyThatBuildsLines(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)

	entries := filteredRead(t, svc, LogQuery{Build: []string{"dep-second"}})
	if len(entries) == 0 {
		t.Fatal("the build's read is empty")
	}
	for message, build := range builds(entries) {
		if build != "dep-second" {
			t.Errorf("line %q carries build %q, want only dep-second's lines", message, build)
		}
	}
	if !slices.ContainsFunc(entries, func(e LogEntry) bool { return e.Message == "#5 building the second" }) {
		t.Errorf("the build's own pod line is missing: %v", builds(entries))
	}
	// Absent a type, the filter reads build lines alone, and the store is
	// asked for the build's own Job.
	if !slices.Equal(got.Types, []string{LogTypeBuild}) {
		t.Errorf("store read types = %v, want [build]", got.Types)
	}
	if selector := lokiSelectorFor("default", got); !strings.Contains(selector, `pod=~"^(bld-web-gen-5)-[^-]+$"`) {
		t.Errorf("selector %s does not select the build's Job", selector)
	}
}

func TestTwoBuildsReadTheirUnion(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)

	entries := filteredRead(t, svc, LogQuery{Types: []string{LogTypeBuild}, Build: []string{"dep-second", "dep-old"}})
	seen := map[string]bool{}
	for _, build := range builds(entries) {
		seen[build] = true
	}
	if !seen["dep-second"] || !seen["dep-old"] || len(seen) != 2 {
		t.Fatalf("builds read = %v, want dep-second and dep-old only", builds(entries))
	}
	if !slices.Equal(got.includeJobs, []string{appv1alpha1.BuildJobName("web", "gen-3"), appv1alpha1.BuildJobName("web", "gen-5")}) {
		t.Errorf("included Jobs = %v", got.includeJobs)
	}
}

// An unknown build is an empty page, as any other filter matching nothing is,
// and the store is never asked for every pod.
func TestAnUnknownBuildIsAnEmptyPage(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)

	if entries := filteredRead(t, svc, LogQuery{Build: []string{"dep-nope"}}); len(entries) != 0 {
		t.Fatalf("an unknown build read %v, want nothing", builds(entries))
	}
	if selector := lokiSelectorFor("default", got); !strings.Contains(selector, `pod="`+noSuchInstance+`"`) {
		t.Errorf("an unknown build's selector %s can select a real pod", selector)
	}
}

// Only build lines carry a build: a filter beside types that read none of
// them answers nothing, without reading the store.
func TestABuildFilterOverOtherTypesReadsNothing(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)

	for _, types := range [][]string{{LogTypeApp}, {LogTypeRequest}, {LogTypeApp, LogTypeRequest}, {LogTypePreDeploy}} {
		got = LogQuery{}
		if entries := filteredRead(t, svc, LogQuery{Types: types, Build: []string{"dep-second"}}); len(entries) != 0 {
			t.Errorf("types %v with a build read %v, want nothing", types, builds(entries))
		}
		if got.App != "" {
			t.Errorf("types %v with a build read the store: %+v", types, got)
		}
	}
	// Beside build lines, the filter keeps the build's.
	entries := filteredRead(t, svc, LogQuery{Types: []string{LogTypeApp, LogTypeBuild}, Build: []string{"dep-second"}})
	if len(entries) == 0 || !slices.Equal(got.Types, []string{LogTypeBuild}) {
		t.Fatalf("app+build with a build read %v (store types %v), want the build's lines alone", builds(entries), got.Types)
	}
}

// Every build line names its build: a build Job's pod line by the deploy that
// ran the Job's generation, a platform line by its deploy. A pod no build Job
// owns (a kpack build) names none rather than a wrong one.
func TestEveryAttributableBuildLineCarriesItsBuild(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)

	labels := builds(filteredRead(t, svc, LogQuery{Types: []string{LogTypeBuild}}))
	for message, want := range map[string]string{
		"#5 building the second":  "dep-second",
		"#4 building the first":   "dep-first",
		"#3 building the old one": "dep-old",
		"buildpack detect":        "",
		"==> Deploy canceled":     "dep-first",
	} {
		if build, ok := labels[message]; !ok || build != want {
			t.Errorf("line %q build = %q (present %v), want %q", message, build, ok, want)
		}
	}
}

// Two rows of one generation: the Job's lines are the build of the row that
// built, never a row that ran no build.
func TestASharedGenerationIsTheBuildingRowsBuild(t *testing.T) {
	index := newBuildIndex("web", []DeployProgress{
		{ID: "dep-rollout", Generation: 7},
		{ID: "dep-built", Generation: 7, Built: true},
	})
	if got := index.buildOf(buildPod(7, "zzzzz")); got != "dep-built" {
		t.Fatalf("gen-7's pod = %q, want dep-built", got)
	}
	for _, pod := range []string{buildPod(70, "zzzzz"), appv1alpha1.BuildJobName("web", "gen-7"), buildPod(7, "a-b")} {
		if got := index.buildOf(pod); got != "" {
			t.Errorf("buildOf(%q) = %q, want none", pod, got)
		}
	}
}

// A filter that cannot resolve answers the error: an empty page would read as
// "no such build", and the unscoped read would show other builds' lines. An
// unfiltered read still serves its lines, unlabeled.
func TestABuildFilterFailsClosedWhenTheDeploysCannotBeRead(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return nil, errors.New("store down")
	}
	_, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Build: []string{"dep-second"}})
	if err == nil || !strings.Contains(err.Error(), "store down") {
		t.Fatalf("an unresolvable build filter = %v, want the store's error", err)
	}
	entries := filteredRead(t, svc, LogQuery{Types: []string{LogTypeBuild}})
	if len(entries) != 4 {
		t.Fatalf("an unfiltered build read without deploys = %v, want every pod's line", builds(entries))
	}
}

func TestRESTListLogsTakesTheBuildFilterAndLabelsTheBuild(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)

	rec := serveREST(svc, "GET", "/v1/logs?resource=web&build=dep-second&startTime=2026-11-10T22:40:48Z&endTime=2026-11-10T22:42:48Z&limit=100")
	env := decodeLogList(t, rec)
	if len(env.Logs) == 0 {
		t.Fatalf("GET /v1/logs?build= read nothing: %s", rec.Body)
	}
	for _, line := range env.Logs {
		names := make([]string, 0, len(line.Labels))
		value := ""
		for _, l := range line.Labels {
			names = append(names, l.Name)
			if l.Name == LabelBuild {
				value = l.Value
			}
		}
		// Render's label enum puts build just before type.
		if value != "dep-second" || len(names) < 2 || names[len(names)-2] != LabelBuild || names[len(names)-1] != LabelType {
			t.Errorf("line %q labels = %+v, want build dep-second before type", line.Message, line.Labels)
		}
	}
	if unknown := decodeLogList(t, serveREST(svc, "GET", "/v1/logs?resource=web&build=dep-nope&limit=5")); len(unknown.Logs) != 0 {
		t.Errorf("an unknown build answered %+v, want an empty page", unknown.Logs)
	}
	var both renderLogList
	if err := json.Unmarshal(serveREST(svc, "GET", "/v1/logs?resource=web&build=dep-second&build=dep-old&startTime=2026-11-10T21:40:48Z&endTime=2026-11-10T22:42:48Z&limit=100").Body.Bytes(), &both); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, line := range both.Logs {
		for _, l := range line.Labels {
			if l.Name == LabelBuild {
				seen[l.Value] = true
			}
		}
	}
	if !seen["dep-second"] || !seen["dep-old"] || len(seen) != 2 {
		t.Errorf("two build values read builds %v, want their union", seen)
	}
}

// A datastore has no builds: its read refuses the filter, as it refuses a type.
func TestADatastoreReadRefusesTheBuildFilter(t *testing.T) {
	if err := (LogQuery{Build: []string{"dep-second"}}).datastoreFilters(postgresID); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a datastore build filter = %v, want a 400", err)
	}
}

func TestTheLiveTailRefusesABuildFilterOverOtherTypes(t *testing.T) {
	for _, types := range [][]string{{LogTypeApp}, {LogTypeRequest}} {
		_, err := LogQuery{App: "web", Types: types, Build: []string{"dep-second"}}.liveSubscribable()
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "build filters build logs") {
			t.Errorf("types %v with a build = %v, want a 400 naming the filter", types, err)
		}
	}
	routed, err := LogQuery{App: "web", Build: []string{"dep-second"}}.liveSubscribable()
	if err != nil || !slices.Equal(routed.Types, []string{LogTypeBuild}) {
		t.Fatalf("a typeless build tail = %v %v, want build lines alone", routed.Types, err)
	}
}

// The live build tail follows only the named build's pod, and labels each of
// its lines with that build, even when a newer build is running.
func TestTheLiveBuildTailFollowsTheNamedBuild(t *testing.T) {
	older := buildPodFor("web", buildPod(4, "older"), "builds", "buildkit", time.Unix(1, 0))
	newer := buildPodFor("web", buildPod(5, "newer"), "builds", "buildkit", time.Unix(2, 0))
	svc := newService(map[string][]string{
		older.Name: {"2026-07-05T00:00:01Z the first build's line"},
		newer.Name: {"2026-07-05T00:00:02Z the second build's line"},
	}, sampleApp("web"), older, newer)
	svc.BuildNamespace = "builds"
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return scopeRows(), nil }

	follow := func(q LogQuery) []LogEntry {
		t.Helper()
		var entries []LogEntry
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		q.App = "web"
		_ = svc.FollowLogs(ctx, q, func(e LogEntry) error {
			if !strings.HasPrefix(e.Message, "==>") {
				entries = append(entries, e)
			}
			return nil
		})
		return entries
	}
	named := follow(LogQuery{Types: []string{LogTypeBuild}, Build: []string{"dep-first"}})
	if len(named) != 1 || named[0].Message != "the first build's line" || named[0].Labels[LabelBuild] != "dep-first" {
		t.Fatalf("a dep-first tail = %+v, want the first build's line labeled dep-first", named)
	}
	newest := follow(LogQuery{Types: []string{LogTypeBuild}})
	if len(newest) != 1 || newest[0].Labels[LabelBuild] != "dep-second" {
		t.Fatalf("an unfiltered tail = %+v, want the newest build's line labeled dep-second", newest)
	}
}

func TestLabelValuesTakeTheBuildFilter(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)
	svc.LabelValues = func(_ context.Context, _, _ string, q LogQuery) ([]string, error) {
		got = q
		return []string{"x"}, nil
	}
	values, err := svc.LogLabelValues(context.Background(), LabelInstance, LogQuery{App: "web", Build: []string{"dep-second"}})
	if err != nil || len(values) == 0 {
		t.Fatalf("values = %v %v", values, err)
	}
	if selector := lokiSelectorFor("default", got); !strings.Contains(selector, `pod=~"^(bld-web-gen-5)-[^-]+$"`) || !strings.Contains(selector, `type="build"`) {
		t.Errorf("values selector %s is not the build's", selector)
	}
	if values, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: "web", Types: []string{LogTypeApp}, Build: []string{"dep-second"}}); err != nil || len(values) != 0 {
		t.Errorf("app values under a build filter = %v %v, want none", values, err)
	}
}

func TestGraphQLAndMCPPassTheBuildFilter(t *testing.T) {
	var got LogQuery
	svc := buildFilterService(&got)
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := runQuery(t, schema, `{ logs(resource: "web", build: ["dep-second"], startTime: "2026-11-10T22:40:48Z", endTime: "2026-11-10T22:42:48Z") { logs { message build } } }`)
	logs := res["logs"].(map[string]any)["logs"].([]any)
	if len(logs) == 0 || !slices.Equal(got.Build, []string{"dep-second"}) {
		t.Fatalf("GraphQL logs(build:) = %v (query %+v)", logs, got)
	}
	for _, l := range logs {
		if l.(map[string]any)["build"] != "dep-second" {
			t.Errorf("GraphQL line %v, want build dep-second", l)
		}
	}
	q, err := logFilters{Resource: []string{"web"}, Build: []string{"dep-second"}}.query()
	if err != nil || !slices.Equal(q.Build, []string{"dep-second"}) {
		t.Fatalf("MCP filters = %+v %v, want the build", q, err)
	}
}
