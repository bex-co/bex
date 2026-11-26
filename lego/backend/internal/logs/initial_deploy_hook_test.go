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
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w33/m435: Render runs a Blueprint's initialDeployHook after the first
// successful deploy. The deploy is live before the hook starts and stays live
// whatever it does, so what the hook did is read in the deploy's log: its own
// output, after the line that marks the deploy live, and how it ended.

var hookLive = time.Date(2026, 11, 16, 9, 0, 0, 250_000_000, time.UTC)

// hookRows is a first deploy that went live, and the deploy that followed it.
func hookRows() []DeployProgress {
	return []DeployProgress{
		{ID: "dep-second", Generation: 2, Status: "live",
			CreatedAt: hookLive.Add(20 * time.Second), StartedAt: hookLive.Add(21 * time.Second), FinishedAt: hookLive.Add(50 * time.Second)},
		{ID: "dep-first", Generation: 1, Status: "deactivated",
			CreatedAt: hookLive.Add(-time.Minute), StartedAt: hookLive.Add(-50 * time.Second), FinishedAt: hookLive},
	}
}

// hookedApp is a service whose Blueprint declares an initial deploy hook.
func hookedApp() *appv1alpha1.App {
	app := sampleApp("web")
	app.Spec.InitialDeployHook = "npm run seed"
	return app
}

// hookService serves rows for a service whose App records run as its hook's
// latest run; the store returns history when one is given.
func hookService(rows []DeployProgress, run *appv1alpha1.PreDeployStatus, history func(LogQuery) []LogEntry) *Service {
	app := hookedApp()
	app.Status.InitialDeployHook = run
	svc := newService(nil, app)
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		if history == nil {
			return nil, nil
		}
		return history(q), nil
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return rows, nil }
	return svc
}

func hookRun1(status, message string, finished bool) *appv1alpha1.PreDeployStatus {
	run := &appv1alpha1.PreDeployStatus{
		Job: appv1alpha1.InitialDeployHookJobName("web", "gen-1"), Generation: 1, Status: status, Message: message,
		// The operator stamps whole seconds: this start reads earlier than the
		// row's own live instant, which bex-api stamps a moment later.
		StartedAt: hookLive.Truncate(time.Second).Format(time.RFC3339),
	}
	if finished {
		run.FinishedAt = hookLive.Add(8 * time.Second).Truncate(time.Second).Format(time.RFC3339)
	}
	return run
}

func buildRead(t *testing.T, svc *Service, q LogQuery) []LogEntry {
	t.Helper()
	q.App, q.Types = "web", []string{LogTypeBuild}
	entries, err := svc.QueryLogs(context.Background(), q)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	return entries
}

// inOrder is the entries' messages as the read returned them.
func inOrder(entries []LogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message)
	}
	return out
}

func TestInitialDeployHookLinesFollowTheDeploysLiveLine(t *testing.T) {
	// The hook pod's own output, as the shipper files it: build type, under
	// the hook Job's pod, stamped in nanoseconds inside the run.
	output := LogEntry{
		Timestamp: hookLive.Add(7*time.Second + 600*time.Millisecond).Format(time.RFC3339Nano),
		Message:   "seeded 42 rows",
		Labels:    map[string]string{LabelType: LogTypeBuild, LabelInstance: "predeploy-web-gen-1-hook-x7k2p", "container": "predeploy"},
	}
	svc := hookService(hookRows(), hookRun1(appv1alpha1.PreDeploySucceeded, "", true), func(LogQuery) []LogEntry { return []LogEntry{output} })

	got := inOrder(buildRead(t, svc, LogQuery{Deploy: "dep-first", Since: hookLive.Add(-2 * time.Minute), End: hookLive.Add(time.Minute)}))
	want := []string{"==> Deploy queued", "==> Rolling out the current release", rendersLive,
		hookLineRunning, "seeded 42 rows", hookLineSucceeded}
	if !slices.Equal(got, want) {
		t.Fatalf("first deploy's log = %q\nwant %q", got, want)
	}

	// The deploy that followed carries none of it.
	second := inOrder(buildRead(t, svc, LogQuery{Deploy: "dep-second", Since: hookLive.Add(-2 * time.Minute), End: hookLive.Add(2 * time.Minute)}))
	for _, line := range second {
		if strings.Contains(line, "initial deploy hook") || strings.Contains(line, "Initial deploy hook") {
			t.Fatalf("the second deploy's log narrates the first deploy's hook: %q", second)
		}
	}
}

func TestInitialDeployHookFailureIsALineThatNamesTheHookAndItsExitCode(t *testing.T) {
	svc := hookService(hookRows(), hookRun1(appv1alpha1.PreDeployFailed, "the initial deploy hook exited with code 3", true), nil)
	entries := buildRead(t, svc, LogQuery{Deploy: "dep-first", Since: hookLive.Add(-2 * time.Minute), End: hookLive.Add(time.Minute)})
	got := inOrder(entries)
	want := []string{"==> Deploy queued", "==> Rolling out the current release", rendersLive, hookLineRunning,
		"==> Initial deploy hook failed: the initial deploy hook exited with code 3", hookLineRetry}
	if !slices.Equal(got, want) {
		t.Fatalf("log = %q\nwant %q", got, want)
	}
	// A failed hook is narration only: nothing reads as the deploy's failure.
	for _, line := range got {
		if strings.HasPrefix(line, "==> Deploy failed") || strings.HasPrefix(line, "==> Pre-deploy failed") {
			t.Fatalf("a failed hook produced a deploy failure line: %q", got)
		}
	}
	// The hook's lines are the platform's, on the deploy, like the live line.
	live := entries[2].Labels
	for _, e := range entries[3:] {
		if e.Labels[LabelInstance] != live[LabelInstance] || e.Labels["container"] != progressContainer || e.Labels[LabelType] != LogTypeBuild {
			t.Fatalf("hook line %q labels = %v, want the live line's (%v)", e.Message, e.Labels, live)
		}
	}
}

func TestInitialDeployHookStillRunningNarratesOnlyItsStart(t *testing.T) {
	svc := hookService(hookRows(), hookRun1(appv1alpha1.PreDeployRunning, "", false), nil)
	got := inOrder(buildRead(t, svc, LogQuery{Deploy: "dep-first", Since: hookLive.Add(-2 * time.Minute)}))
	if got[len(got)-1] != hookLineRunning {
		t.Fatalf("log = %q, want it to end at the running line", got)
	}
}

// A window that opens after the deploy went live still shows the hook: its
// row's story ends when the hook does, not when the deploy went live.
func TestInitialDeployHookLinesSurviveAWindowOpenedAfterTheDeployWentLive(t *testing.T) {
	svc := hookService(hookRows(), hookRun1(appv1alpha1.PreDeployFailed, "the initial deploy hook exited with code 3", true), nil)
	got := inOrder(buildRead(t, svc, LogQuery{Since: hookLive.Add(5 * time.Second), End: hookLive.Add(10 * time.Second)}))
	if !slices.Contains(got, "==> Initial deploy hook failed: the initial deploy hook exited with code 3") {
		t.Fatalf("log = %q, want the hook's end inside a window that opened after the deploy went live", got)
	}
	// Long after the hook, the row is over.
	late := inOrder(buildRead(t, svc, LogQuery{Since: hookLive.Add(time.Hour), End: hookLive.Add(2 * time.Hour)}))
	if len(late) != 0 {
		t.Fatalf("log = %q, want nothing long after the hook ended", late)
	}
}

// The App keeps one run. A run recorded for another release, or a row that
// never went live, narrates nothing.
func TestInitialDeployHookLinesBelongToTheDeployThatWentLiveUnderTheRun(t *testing.T) {
	run := hookRun{generation: 1, status: appv1alpha1.PreDeploySucceeded, startedAt: hookLive, finishedAt: hookLive.Add(time.Second)}
	for name, d := range map[string]DeployProgress{
		"another generation": {ID: "dep", Generation: 2, Status: "live", FinishedAt: hookLive},
		"not live":           {ID: "dep", Generation: 1, Status: "update_failed", FinishedAt: hookLive},
		"still open":         {ID: "dep", Generation: 1, Status: "update_in_progress"},
		"no generation":      {ID: "dep", Status: "live", FinishedAt: hookLive},
	} {
		if lines := hookLines(d, run); len(lines) != 0 {
			t.Errorf("%s: hook lines = %v, want none", name, inOrder(lines))
		}
	}
	if lines := hookLines(DeployProgress{ID: "dep", Generation: 1, Status: "live", FinishedAt: hookLive}, hookRun{}); len(lines) != 0 {
		t.Errorf("no run: hook lines = %v, want none", inOrder(lines))
	}
}

// A deploy-scoped read keeps its own hook's pod and drops the other deploys',
// in the durable build stream and in the live pre-deploy read alike. A service
// with no hook has no such Jobs, and its selector names none
// (TestDeployScopeExcludesOtherDeploysJobPods).
func TestDeployScopeKeepsItsOwnHookPodAndDropsOtherDeploys(t *testing.T) {
	var build LogQuery
	svc := hookService(hookRows(), nil, func(q LogQuery) []LogEntry { build = q; return nil })
	buildRead(t, svc, LogQuery{Deploy: "dep-second"})
	for pod, dropped := range map[string]bool{
		"predeploy-web-gen-1-hook-x7k2p": true,  // the first deploy's hook, running inside this deploy's window
		"predeploy-web-gen-2-hook-ab12c": false, // its own
		"bld-web-gen-1-ab12c":            true,
	} {
		if got := build.excludesPod(pod); got != dropped {
			t.Errorf("build read of dep-second: excludesPod(%q) = %v, want %v", pod, got, dropped)
		}
	}

	pod := func(name string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelPreDeploy: "web"}}}
	}
	line := func(msg string) []string { return []string{"2026-11-16T09:00:05.000000000Z " + msg} }
	live := newService(map[string][]string{
		"predeploy-web-gen-1-x7k2p":      line("migrated"),
		"predeploy-web-gen-1-hook-ab12c": line("seeded"),
		"predeploy-web-gen-2-cd34e":      line("migrated again"),
	}, hookedApp(), pod("predeploy-web-gen-1-x7k2p"), pod("predeploy-web-gen-1-hook-ab12c"), pod("predeploy-web-gen-2-cd34e"))
	live.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return hookRows(), nil }
	read := func(deploy string) []string {
		entries, err := live.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypePreDeploy}, Deploy: deploy})
		if err != nil {
			t.Fatalf("QueryLogs: %v", err)
		}
		got := inOrder(entries)
		slices.Sort(got)
		return got
	}
	if got := read("dep-first"); !slices.Equal(got, []string{"migrated", "seeded"}) {
		t.Fatalf("first deploy's pre-deploy read = %q, want its step's and its hook's output", got)
	}
	if got := read("dep-second"); !slices.Equal(got, []string{"migrated again"}) {
		t.Fatalf("second deploy's pre-deploy read = %q, want its own step's output only", got)
	}
}

// The operator stamps a hook pod with the shared pre-deploy-step spelling and
// bex-api attributes its logs to the hook. The stamp lives in lego/types so
// both sides spell it once; this test pins the recognition to that spelling.
func TestIsInitialDeployHookPodFollowsSharedStamp(t *testing.T) {
	stamped := func(labels map[string]string, name string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}
	for _, tc := range []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{"hook step label", stamped(map[string]string{appv1alpha1.LabelPreDeployStep: appv1alpha1.PreDeployStepInitialDeployHook}, "predeploy-web-gen-1-x7k2p"), true},
		{"hook name fallback", stamped(map[string]string{core.PodLabelPreDeploy: "web"}, "predeploy-web-gen-1-hook-ab12c"), true},
		{"plain step pod", stamped(map[string]string{core.PodLabelPreDeploy: "web"}, "predeploy-web-gen-1-x7k2p"), false},
		{"unlabeled pod", stamped(nil, "web-6f9c4b7d4c-x7k2p"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isInitialDeployHookPod(tc.pod); got != tc.want {
				t.Fatalf("isInitialDeployHookPod(%q) = %v, want %v", tc.pod.Name, got, tc.want)
			}
		})
	}
}
