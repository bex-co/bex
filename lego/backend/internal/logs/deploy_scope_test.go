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
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w21/m324: a deploy's page is the service's build stream cut by the deploy's
// time window, so two deploys that overlap in time (a hook fired twice, two
// quick pushes) shared every line inside it. A Deploy-scoped read drops the
// other deploys' platform lines and their build pods.

var scopeBase = time.Date(2026, 11, 10, 22, 41, 48, 0, time.UTC)

// scopeRows is the live-QA shape: dep-first is superseded while dep-second
// (created a second later) builds and fails, and dep-old ran long before.
func scopeRows() []DeployProgress {
	return []DeployProgress{
		{ID: "dep-second", Generation: 5, Status: "build_failed", FailureReason: "fatal: couldn't find remote ref abc", Built: true,
			CreatedAt: scopeBase.Add(time.Second), StartedAt: scopeBase.Add(2 * time.Second), FinishedAt: scopeBase.Add(39 * time.Second)},
		{ID: "dep-first", Generation: 4, Status: "canceled",
			CreatedAt: scopeBase, FinishedAt: scopeBase.Add(26 * time.Second)},
		{ID: "dep-old", Generation: 3, Status: "live", Built: true,
			CreatedAt: scopeBase.Add(-time.Hour), StartedAt: scopeBase.Add(-time.Hour + time.Second), FinishedAt: scopeBase.Add(-50 * time.Minute)},
	}
}

func scopeService(rows []DeployProgress, history func(LogQuery)) *Service {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/bex-co/bex"
	svc := newService(nil, app)
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		if history != nil {
			history(q)
		}
		return nil, nil
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return rows, nil }
	return svc
}

func scopeRead(t *testing.T, svc *Service, deploy string) []string {
	t.Helper()
	entries, err := svc.QueryLogs(context.Background(), LogQuery{
		App: "web", Types: []string{LogTypeBuild}, Deploy: deploy,
		Since: scopeBase.Add(-time.Minute), End: scopeBase.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message)
	}
	return out
}

func TestDeployScopeKeepsOnlyThatDeploysPlatformLines(t *testing.T) {
	svc := scopeService(scopeRows(), nil)

	// Unscoped, the window alone: both overlapping deploys' lines interleave.
	all := scopeRead(t, svc, "")
	for _, want := range []string{"==> Deploy canceled", "==> Deploy queued", "==> Build queued"} {
		if !slices.Contains(all, want) {
			t.Fatalf("unscoped read lacks %q: %v", want, all)
		}
	}

	first := scopeRead(t, svc, "dep-first")
	if !slices.Equal(first, []string{"==> Deploy queued", "==> Deploy canceled"}) {
		t.Fatalf("the superseded deploy's page = %v, want only its own queued and canceled lines", first)
	}
	second := scopeRead(t, svc, "dep-second")
	if slices.Contains(second, "==> Deploy canceled") || slices.Contains(second, "==> Deploy queued") {
		t.Fatalf("the failed deploy's page = %v, want none of the superseded deploy's lines", second)
	}
	if !slices.Contains(second, "==> Build queued") ||
		!slices.ContainsFunc(second, func(m string) bool { return strings.HasPrefix(m, "==> Build failed") }) {
		t.Fatalf("the failed deploy's page = %v, want its own queued and failed lines", second)
	}
}

// The build pod's lines carry only its pod name, which the operator derives
// from the release generation, so every OTHER deploy's Job is excluded from the
// store selector.
func TestDeployScopeExcludesOtherDeploysBuildPods(t *testing.T) {
	var got LogQuery
	svc := scopeService(scopeRows(), func(q LogQuery) { got = q })
	scopeRead(t, svc, "dep-second")

	want := []string{
		appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(4)),
		appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(3)),
	}
	if !slices.Equal(got.excludeJobs, want) {
		t.Fatalf("excluded Jobs = %v, want %v (the other deploys', never its own gen-5)", got.excludeJobs, want)
	}
	selector := lokiSelectorFor("default", got)
	wantMatcher := `pod!~"^(bld-web-gen-4|bld-web-gen-3)-[^-]+$"`
	if !strings.Contains(selector, wantMatcher) {
		t.Fatalf("selector %s lacks %s", selector, wantMatcher)
	}
	// An unscoped read keeps the selector as it was.
	if unscoped := lokiSelectorFor("default", LogQuery{App: "web", Types: []string{LogTypeBuild}}); strings.Contains(unscoped, "pod!~") {
		t.Fatalf("an unscoped selector excludes pods: %s", unscoped)
	}
}

func TestExcludesPodNeverTakesAnotherGenerationsOrAnUnknownPod(t *testing.T) {
	q := LogQuery{excludeJobs: []string{"bld-web-gen-1"}}
	for pod, want := range map[string]bool{
		"bld-web-gen-1-abcde":  true,
		"bld-web-gen-12-abcde": false, // gen-1 is a prefix of gen-12, never a match
		"bld-web-gen-1":        false,
		"bld-web-gen-1-":       false,
		"bld-web-gen-1-ab-cd":  false,
		// A kpack build pod is named for its Image, not the build Job: it cannot
		// be attributed, so it is never hidden.
		"web-image-build-1-build-pod": false,
	} {
		if got := q.excludesPod(pod); got != want {
			t.Errorf("excludesPod(%q) = %v, want %v", pod, got, want)
		}
	}
}

// A missing scope costs a stray line; a wrong one would hide real output, so
// every way of not knowing the deploy reads as the window alone.
func TestDeployScopeFailsOpen(t *testing.T) {
	for name, svc := range map[string]*Service{
		"unknown deploy": scopeService(scopeRows(), nil),
		"no rows":        scopeService(nil, nil),
		"source error": func() *Service {
			svc := scopeService(nil, nil)
			svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
				return nil, errors.New("store down")
			}
			return svc
		}(),
		"no source": func() *Service { svc := scopeService(nil, nil); svc.DeployProgress = nil; return svc }(),
	} {
		var got LogQuery
		inner := svc.History
		svc.History = func(ctx context.Context, ns string, q LogQuery) ([]LogEntry, error) {
			got = q
			return inner(ctx, ns, q)
		}
		scopeRead(t, svc, "dep-nope")
		if len(got.excludeJobs) != 0 {
			t.Errorf("%s: excluded Jobs = %v, want none", name, got.excludeJobs)
		}
	}
}

func TestDeployFilterAppliesOnlyToBuildAndPreDeployLogs(t *testing.T) {
	svc := scopeService(scopeRows(), nil)
	for _, types := range [][]string{nil, {LogTypeApp}, {LogTypeRequest}, {LogTypeApp, LogTypeRequest}} {
		_, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: types, Deploy: "dep-first"})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "deploy filter") {
			t.Errorf("types %v with a deploy => %v, want a 400 naming the deploy filter", types, err)
		}
	}
	for _, types := range [][]string{{LogTypeBuild}, {LogTypePreDeploy}} {
		if err := (LogQuery{Types: types, Deploy: "dep-first"}).validate(); err != nil {
			t.Errorf("types %v with a deploy refused: %v", types, err)
		}
	}
	// A managed datastore has no deploys.
	_, err := svc.QueryLogs(context.Background(), LogQuery{App: postgresID, Deploy: "dep-first"})
	if err == nil {
		t.Error("a datastore read with a deploy filter was accepted")
	}
}

// A pre-deploy step is a Job in the App's namespace named like the build's, so
// the same generations tell one deploy's migration pod from another's.
func TestDeployScopeDropsOtherDeploysPreDeployPods(t *testing.T) {
	own := appv1alpha1.PreDeployJobName("web", appv1alpha1.BuildRevision(5)) + "-aaaaa"
	other := appv1alpha1.PreDeployJobName("web", appv1alpha1.BuildRevision(4)) + "-bbbbb"
	svc := newService(
		map[string][]string{
			own:   {"2026-11-10T22:41:50Z migrating (own)"},
			other: {"2026-11-10T22:41:51Z migrating (other)"},
		},
		sampleApp("web"),
		preDeployPod("web", own),
		preDeployPod("web", other),
	)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return scopeRows(), nil }

	read := func(deploy string) []string {
		entries, err := svc.QueryLogs(context.Background(), LogQuery{
			App: "web", Types: []string{LogTypePreDeploy}, Deploy: deploy, Direction: DirectionForward,
		})
		if err != nil {
			t.Fatalf("QueryLogs: %v", err)
		}
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Message)
		}
		return out
	}
	if got := read("dep-second"); !slices.Equal(got, []string{"migrating (own)"}) {
		t.Fatalf("scoped pre-deploy read = %v, want only the deploy's own step", got)
	}
	if got := read(""); len(got) != 2 {
		t.Fatalf("unscoped pre-deploy read = %v, want both steps", got)
	}
}

func TestGraphQLLogsTakesADeployID(t *testing.T) {
	var got LogQuery
	svc := scopeService(scopeRows(), func(q LogQuery) { got = q })
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := runQuery(t, schema, `{ logs(resource: "web", type: "build", deployId: "dep-second", startTime: "2026-11-10T22:40:48Z", endTime: "2026-11-10T22:42:48Z") { logs { message } } }`)
	logs := res["logs"].(map[string]any)["logs"].([]any)
	messages := make([]string, 0, len(logs))
	for _, l := range logs {
		messages = append(messages, l.(map[string]any)["message"].(string))
	}
	if got.Deploy != "dep-second" || slices.Contains(messages, "==> Deploy canceled") {
		t.Fatalf("deploy = %q, lines = %v; want the filter applied and the other deploy's lines gone", got.Deploy, messages)
	}
}
