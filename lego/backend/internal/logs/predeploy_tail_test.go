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
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func runningPreDeployPod(app, name, namespace string, created time.Time) *corev1.Pod {
	p := preDeployPodIn(app, name, namespace)
	p.UID = types.UID("uid-" + name)
	p.CreationTimestamp = metav1.NewTime(created)
	p.Status = corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name:  core.PreDeployContainer,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(created)}},
		}},
	}
	return p
}

func finishedPreDeployPod(app, name, namespace string, phase corev1.PodPhase, exitCode int32, started, finished time.Time) *corev1.Pod {
	p := preDeployPodIn(app, name, namespace)
	p.UID = types.UID("uid-" + name)
	p.CreationTimestamp = metav1.NewTime(started)
	p.Status = corev1.PodStatus{
		Phase: phase,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: core.PreDeployContainer,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode:   exitCode,
				StartedAt:  metav1.NewTime(started),
				FinishedAt: metav1.NewTime(finished),
			}},
		}},
	}
	return p
}

func dialLogWS(t *testing.T, url string) (*websocket.Conn, <-chan renderLog) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("WS dial %s: %v", url, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ch := make(chan renderLog, 128)
	go func() {
		defer close(ch)
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err == nil {
				ch <- line
			}
		}
	}()
	return conn, ch
}

// w32/092: the CLI's live WebSocket build tail carries a pre-deploy step's own
// output from the App's own namespace (ADR043 D8), between `==> Starting
// pre-deploy: <command>` / `==> Running '<command>'` and the step's verdict,
// each line once, with the exact same ids and labels as the Alloy-shipped
// history read.
func TestWebSocketBuildTailCarriesPreDeployOutputInOrderWithHistoryIDs(t *testing.T) {
	resource := ids.New(ids.Service)
	app := logTenantNSApp("web", logTestWS, resource)
	app.Spec.Repo = rolloutRepo
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }

	buildPodName := appv1alpha1.BuildJobName(app.Name, appv1alpha1.BuildRevision(3)) + "-a1b2c"
	stepPodName := appv1alpha1.PreDeployJobName(app.Name, appv1alpha1.BuildRevision(3)) + "-x7k2p"
	oldPodName := appv1alpha1.PreDeployJobName(app.Name, appv1alpha1.BuildRevision(2)) + "-old11"
	decoyPodName := appv1alpha1.PreDeployJobName(app.Name, appv1alpha1.BuildRevision(3)) + "-decoy"
	const (
		out1 = "Prisma schema loaded from prisma/schema.prisma"
		out2 = "Error: P3009 migrate found failed migrations"
	)
	buildAt := soon(60 * time.Minute)
	launchAt := soon(61 * time.Minute)
	containerAt := soon(62 * time.Minute).Truncate(time.Second)
	out1At := containerAt.Add(150 * time.Millisecond)
	out2At := containerAt.Add(600 * time.Millisecond)
	settledAt := soon(63 * time.Minute)
	closeAt := settledAt.Add(time.Second)

	oldPod := finishedPreDeployPod(app.Name, oldPodName, app.Namespace, corev1.PodSucceeded, 0, opened.Add(-20*time.Minute), opened.Add(-19*time.Minute))
	decoyPod := runningPreDeployPod(app.Name, decoyPodName, "bex-build", opened)
	svc := newService(nil, app, oldPod, decoyPod)
	svc.BuildNamespace = "bex-build"
	svc.BuildPodWaitInterval = 10 * time.Millisecond

	stepPR, stepPW := io.Pipe()
	var readsMu sync.Mutex
	var followReads []string
	svc.PodLogsFollow = func(_ context.Context, ns, pod, container string, _ time.Time) (io.ReadCloser, error) {
		readsMu.Lock()
		followReads = append(followReads, ns+"/"+pod+"/"+container)
		readsMu.Unlock()
		switch pod {
		case buildPodName:
			if ns != "bex-build" {
				return nil, fmt.Errorf("build pod read from wrong namespace %q", ns)
			}
			return io.NopCloser(strings.NewReader(buildAt.Format(time.RFC3339Nano) + " ==> Build successful 🎉\n")), nil
		case stepPodName:
			if ns != app.Namespace || container != core.PreDeployContainer {
				return nil, fmt.Errorf("pre-deploy pod read from %s/%s, want %s/%s", ns, container, app.Namespace, core.PreDeployContainer)
			}
			return stepPR, nil
		default:
			return nil, fmt.Errorf("unexpected follow of %s/%s", ns, pod)
		}
	}

	var mu sync.Mutex
	rows := []DeployProgress{
		{
			ID: "dep-3", Generation: 3, Status: deployStatusBuildInProgress, Built: true, Commit: "abc1234def5678",
			CreatedAt: soon(30 * time.Minute), StartedAt: soon(31 * time.Minute),
		},
		{
			ID: "dep-2", Generation: 2, Status: "live", Built: true, Commit: "0000000aaaa",
			CreatedAt: opened.Add(-30 * time.Minute), StartedAt: opened.Add(-29 * time.Minute), FinishedAt: opened.Add(-15 * time.Minute),
			PreDeploy: PreDeployRun{
				Status: preDeploySucceeded, StartedAt: opened.Add(-20 * time.Minute),
				ContainerStartedAt: opened.Add(-20 * time.Minute), FinishedAt: opened.Add(-19 * time.Minute),
				Command: stepCommand, Native: true,
			},
		},
	}
	update := func(change func(*DeployProgress)) { mu.Lock(); change(&rows[0]); mu.Unlock() }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(rows), nil
	}

	// History read mirrors what Alloy's build_pods pipeline stores in Loki.
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
		return []LogEntry{
			{Timestamp: buildAt.Format(time.RFC3339Nano), Message: "==> Build successful 🎉",
				Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: buildPodName, "container": "buildkit"}},
			{Timestamp: out1At.Format(time.RFC3339Nano), Message: out1,
				Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: stepPodName, "container": core.PreDeployContainer}},
			{Timestamp: out2At.Format(time.RFC3339Nano), Message: out2,
				Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: stepPodName, "container": core.PreDeployContainer}},
		}, nil
	}

	srv := revalidationTestServer(svc)
	defer srv.Close()
	_, ch := dialLogWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource="+resource+"&type=build")

	var got []renderLog
	readUntil := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for !slices.ContainsFunc(got, func(l renderLog) bool { return l.Message == want }) {
			select {
			case line, ok := <-ch:
				if !ok {
					t.Fatalf("stream closed waiting for %q after %v", want, got)
				}
				got = append(got, line)
			case <-deadline:
				t.Fatalf("timed out waiting for %q after %v", want, got)
			}
		}
	}

	buildPod := buildPodFor(app.Name, buildPodName, "bex-build", "buildkit", opened)
	if err := svc.Client.Create(context.Background(), buildPod); err != nil {
		t.Fatal(err)
	}
	readUntil("==> Build successful 🎉")

	// Mark build pod done and enter pre-deploy.
	buildPod.Status.Phase = corev1.PodSucceeded
	if err := svc.Client.Status().Update(context.Background(), buildPod); err != nil {
		t.Fatal(err)
	}
	update(func(d *DeployProgress) {
		d.Status = deployStatusPreDeployInProgress
		d.PreDeploy = PreDeployRun{Status: "running", StartedAt: launchAt, Command: stepCommand, Native: true}
	})
	readUntil(rendersStepStart)

	// Step pod starts in the App's workspace namespace (not bex-build), first Pending then Running.
	stepPod := preDeployPodIn(app.Name, stepPodName, app.Namespace)
	stepPod.UID = "uid-step-3"
	stepPod.CreationTimestamp = metav1.NewTime(launchAt)
	stepPod.Status.Phase = corev1.PodPending
	if err := svc.Client.Create(context.Background(), stepPod); err != nil {
		t.Fatal(err)
	}
	update(func(d *DeployProgress) { d.PreDeploy.ContainerStartedAt = containerAt })
	readUntil(rendersStepRunning)

	stepPod.Status.Phase = corev1.PodRunning
	stepPod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  core.PreDeployContainer,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(containerAt)}},
	}}
	if err := svc.Client.Status().Update(context.Background(), stepPod); err != nil {
		t.Fatal(err)
	}

	if _, err := fmt.Fprintf(stepPW, "%s %s\n%s %s\n", out1At.Format(time.RFC3339Nano), out1, out2At.Format(time.RFC3339Nano), out2); err != nil {
		t.Fatal(err)
	}
	readUntil(out2)

	// Even if the deploy row settles while the pod's stream is still open,
	// the verdict must stay held until the pod reaches EOF.
	code := int32(1)
	update(func(d *DeployProgress) {
		d.PreDeploy.Status, d.PreDeploy.FinishedAt, d.PreDeploy.ExitCode = "failed", settledAt, &code
		d.Status, d.FinishedAt = deployStatusPreDeployFailed, closeAt
		d.FailureReason = "the pre-deploy command exited with code 1; check the pre-deploy logs"
	})
	select {
	case extra := <-ch:
		t.Fatalf("emitted %q before the pre-deploy pod reached EOF", extra.Message)
	case <-time.After(60 * time.Millisecond):
	}

	stepPod.Status.Phase = corev1.PodFailed
	stepPod.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Terminated: &corev1.ContainerStateTerminated{ExitCode: 1, StartedAt: metav1.NewTime(containerAt), FinishedAt: metav1.NewTime(settledAt)},
	}
	if err := svc.Client.Status().Update(context.Background(), stepPod); err != nil {
		t.Fatal(err)
	}
	if err := stepPW.Close(); err != nil {
		t.Fatal(err)
	}
	readUntil(rendersTroubleshoot)

	// Let several more poll passes run: nothing repeats.
	drain := time.NewTimer(100 * time.Millisecond)
	defer drain.Stop()
	for {
		select {
		case extra := <-ch:
			got = append(got, extra)
		case <-drain.C:
			goto done
		}
	}
done:
	wantOrder := []string{
		"==> Build successful 🎉",
		rendersStepStart,
		rendersStepRunning,
		out1,
		out2,
		rendersStepFailed,
		rendersExited1,
		rendersTroubleshoot,
	}
	var messages []string
	tailByMsg := map[string]renderLog{}
	for _, l := range got {
		messages = append(messages, l.Message)
		tailByMsg[l.Message] = l
	}
	startIdx := slices.Index(messages, "==> Build successful 🎉")
	if startIdx < 0 || !slices.Equal(messages[startIdx:], wantOrder) {
		t.Fatalf("tail messages = %q\nwant suffix %q", messages, wantOrder)
	}

	// Verify namespace isolation: never read the decoy in bex-build or the old pod.
	readsMu.Lock()
	reads := slices.Clone(followReads)
	readsMu.Unlock()
	for _, r := range reads {
		if strings.Contains(r, decoyPodName) || strings.Contains(r, oldPodName) {
			t.Errorf("unexpected pod follow %q (all reads: %v)", r, reads)
		}
	}
	if !slices.Contains(reads, app.Namespace+"/"+stepPodName+"/"+core.PreDeployContainer) {
		t.Errorf("reads = %v, want pre-deploy pod read from %s", reads, app.Namespace)
	}

	// Compare every line's id and labels against the history read.
	history, err := svc.QueryLogs(context.Background(), LogQuery{
		App: resource, Types: []string{LogTypeBuild}, Since: opened, End: soon(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	histByMsg := map[string]renderLog{}
	for _, e := range history {
		rl := toRenderLog(e)
		histByMsg[rl.Message] = rl
	}
	for _, msg := range wantOrder {
		h, ok := histByMsg[msg]
		if !ok {
			t.Fatalf("history missing %q", msg)
		}
		tl := tailByMsg[msg]
		if tl.ID != h.ID {
			t.Errorf("%q: tail ID %q != history ID %q", msg, tl.ID, h.ID)
		}
		if !slices.Equal(tl.Labels, h.Labels) {
			t.Errorf("%q: tail labels %+v != history labels %+v", msg, tl.Labels, h.Labels)
		}
	}
}

// w32/092: a passing pre-deploy step prints its output before `==> Pre-deploy
// complete!` and the rollout lines, and a second deploy on the same WebSocket
// tail streams its own pre-deploy pod without repeating the first.
func TestWebSocketBuildTailCarriesPassingPreDeployAndFollowsSecondDeploy(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }

	const (
		step1Pod = "predeploy-web-gen-3-step1"
		step2Pod = "predeploy-web-gen-4-step2"
	)
	lines := map[string][]string{
		"bld-web-gen-3-a": {soon(10*time.Minute).Format(time.RFC3339Nano) + " ==> Build successful 🎉"},
		step1Pod:          {soon(12*time.Minute).Format(time.RFC3339Nano) + " migration 1 applied"},
		"bld-web-gen-4-b": {soon(30*time.Minute).Format(time.RFC3339Nano) + " ==> Build successful 🎉"},
		step2Pod:          {soon(32*time.Minute).Format(time.RFC3339Nano) + " migration 2 failed"},
	}
	svc := newService(lines, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond

	var mu sync.Mutex
	var rows []DeployProgress
	setRows := func(r ...DeployProgress) { mu.Lock(); rows = r; mu.Unlock() }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(rows), nil
	}

	srv := revalidationTestServer(svc)
	defer srv.Close()
	_, ch := dialLogWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build")

	var got []string
	readUntil := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for !slices.Contains(got, want) {
			select {
			case line, ok := <-ch:
				if !ok {
					t.Fatalf("stream closed waiting for %q after %q", want, got)
				}
				got = append(got, line.Message)
			case <-deadline:
				t.Fatalf("timed out waiting for %q after %q", want, got)
			}
		}
	}

	// Deploy 1: build -> passing pre-deploy -> rollout -> live.
	zero := int32(0)
	dep1 := DeployProgress{
		ID: "dep-3", Generation: 3, Status: deployStatusBuildInProgress, Built: true, Commit: "aaaaaaa1111",
		CreatedAt: soon(5 * time.Minute), StartedAt: soon(6 * time.Minute),
	}
	setRows(dep1)
	b1 := buildPodFor("web", "bld-web-gen-3-a", "builds", "buildkit", opened)
	if err := svc.Client.Create(context.Background(), b1); err != nil {
		t.Fatal(err)
	}
	readUntil("==> Build successful 🎉")

	b1.Status.Phase = corev1.PodSucceeded
	_ = svc.Client.Status().Update(context.Background(), b1)
	if err := svc.Client.Create(context.Background(), finishedPreDeployPod("web", step1Pod, app.Namespace, corev1.PodSucceeded, 0, soon(11*time.Minute), soon(13*time.Minute))); err != nil {
		t.Fatal(err)
	}
	dep1.Status = "live"
	dep1.FinishedAt = soon(20 * time.Minute)
	dep1.PreDeploy = PreDeployRun{
		Status: preDeploySucceeded, StartedAt: soon(11 * time.Minute),
		ContainerStartedAt: soon(11*time.Minute + time.Second).Truncate(time.Second), FinishedAt: soon(13 * time.Minute),
		Command: stepCommand, Native: true, ExitCode: &zero,
	}
	dep1.RolloutStartedAt = soon(14 * time.Minute)
	dep1.WebConcurrencyDefault = 1
	setRows(dep1)
	readUntil(rendersLive)

	// Deploy 2 on the same tail: build -> failing pre-deploy.
	one := int32(1)
	dep2 := DeployProgress{
		ID: "dep-4", Generation: 4, Status: deployStatusBuildInProgress, Built: true, Commit: "bbbbbbb2222",
		CreatedAt: soon(25 * time.Minute), StartedAt: soon(26 * time.Minute),
	}
	setRows(dep2, dep1)
	b2 := buildPodFor("web", "bld-web-gen-4-b", "builds", "buildkit", soon(time.Second))
	if err := svc.Client.Create(context.Background(), b2); err != nil {
		t.Fatal(err)
	}
	// Wait for dep2's build start banner, then finish its build and pre-deploy.
	readUntil("==> Building from https://github.com/x/y@bbbbbbb")
	b2.Status.Phase = corev1.PodSucceeded
	_ = svc.Client.Status().Update(context.Background(), b2)
	if err := svc.Client.Create(context.Background(), finishedPreDeployPod("web", step2Pod, app.Namespace, corev1.PodFailed, 1, soon(31*time.Minute), soon(33*time.Minute))); err != nil {
		t.Fatal(err)
	}
	dep2.Status = deployStatusPreDeployFailed
	dep2.FinishedAt = soon(33*time.Minute + time.Second)
	dep2.PreDeploy = PreDeployRun{
		Status: "failed", StartedAt: soon(31 * time.Minute),
		ContainerStartedAt: soon(31*time.Minute + time.Second).Truncate(time.Second), FinishedAt: soon(33 * time.Minute),
		Command: stepCommand, Native: true, ExitCode: &one,
	}
	setRows(dep2, dep1)
	readUntil(rendersTroubleshoot)

	// Check ordering for both deploys and that migration 1 was never repeated.
	idx := func(msg string) int { return slices.Index(got, msg) }
	if idx("migration 1 applied") < 0 || idx(rendersStepComplete) < 0 || idx("migration 1 applied") > idx(rendersStepComplete) {
		t.Fatalf("deploy 1: migration output must come before Pre-deploy complete!: %q", got)
	}
	if idx(rendersStepComplete) > idx(rendersDeploying) || idx(rendersDeploying) > idx(rendersLive) {
		t.Fatalf("deploy 1: Pre-deploy complete! must precede Deploying... and live: %q", got)
	}
	if idx("migration 2 failed") < 0 || idx(rendersStepFailed) < 0 || idx("migration 2 failed") > idx(rendersStepFailed) {
		t.Fatalf("deploy 2: migration output must come before Pre-deploy has failed: %q", got)
	}
	count := 0
	for _, m := range got {
		if m == "migration 1 applied" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("migration 1 applied sent %d times, want 1: %q", count, got)
	}
}

// w32/092: an image-backed deploy (no build pod at all) whose non-native
// pre-deploy step finishes fast before the first poll still prints `==>
// Starting pre-deploy`, the command's output, and only then the verdict (and
// no `==> Running` line).
func TestWebSocketBuildTailHoldsPreDeployVerdictOnFastImageDeploy(t *testing.T) {
	app := sampleApp("web")
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }

	const stepPodName = "predeploy-web-gen-5-fast1"
	stepPod := finishedPreDeployPod("web", stepPodName, app.Namespace, corev1.PodFailed, 1, soon(10*time.Minute), soon(10*time.Minute+2*time.Second))
	svc := newService(map[string][]string{
		stepPodName: {
			soon(10*time.Minute+500*time.Millisecond).Format(time.RFC3339Nano) + " migrating...",
			soon(10*time.Minute+time.Second).Format(time.RFC3339Nano) + " fatal: bad SQL",
		},
	}, app, stepPod)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond

	one := int32(1)
	row := DeployProgress{
		ID: "dep-5", Generation: 5, Status: deployStatusPreDeployFailed, Image: "docker.io/library/node:22",
		CreatedAt: soon(9 * time.Minute), StartedAt: soon(9*time.Minute + time.Second), FinishedAt: soon(10*time.Minute + 3*time.Second),
		PreDeploy: PreDeployRun{
			Status: "failed", StartedAt: soon(10 * time.Minute), FinishedAt: soon(10*time.Minute + 2*time.Second),
			Command: "node migrate.js", Native: false, ExitCode: &one,
		},
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}

	srv := revalidationTestServer(svc)
	defer srv.Close()
	_, ch := dialLogWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build")

	var got []string
	deadline := time.After(5 * time.Second)
	for !slices.Contains(got, rendersTroubleshoot) {
		select {
		case line, ok := <-ch:
			if !ok {
				t.Fatalf("stream closed after %q", got)
			}
			got = append(got, line.Message)
		case <-deadline:
			t.Fatalf("timed out after %q", got)
		}
	}
	want := []string{
		"==> Deploy queued",
		"==> Deploying image docker.io/library/node:22",
		"==> Starting pre-deploy: node migrate.js",
		"migrating...",
		"fatal: bad SQL",
		rendersStepFailed,
		rendersExited1,
		rendersTroubleshoot,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tail = %q\nwant %q", got, want)
	}
}

// w32/092: the type-less WebSocket deploy tail (`bex logs -r <srv> --tail`,
// `bex deploys create --wait`) routes through followBuilds too, so it carries
// the pre-deploy step's output alongside app lines.
func TestWebSocketTypelessTailCarriesPreDeployOutput(t *testing.T) {
	app := sampleApp("web")
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }

	const stepPodName = "predeploy-web-gen-6-live1"
	stepPod := finishedPreDeployPod("web", stepPodName, app.Namespace, corev1.PodSucceeded, 0, soon(10*time.Minute), soon(10*time.Minute+time.Second))
	svc := newService(map[string][]string{
		stepPodName: {soon(10*time.Minute+500*time.Millisecond).Format(time.RFC3339Nano) + " schema up to date"},
	}, app, stepPod)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond

	zero := int32(0)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{{
			ID: "dep-6", Generation: 6, Status: "live", Image: "docker.io/library/node:22",
			CreatedAt: soon(9 * time.Minute), StartedAt: soon(9*time.Minute + time.Second), FinishedAt: soon(11 * time.Minute),
			PreDeploy: PreDeployRun{
				Status: preDeploySucceeded, StartedAt: soon(10 * time.Minute), FinishedAt: soon(10*time.Minute + time.Second),
				Command: "node migrate.js", Native: false, ExitCode: &zero,
			},
		}}, nil
	}

	srv := revalidationTestServer(svc)
	defer srv.Close()
	_, ch := dialLogWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web")

	var got []string
	deadline := time.After(5 * time.Second)
	for !slices.Contains(got, rendersLive) {
		select {
		case line, ok := <-ch:
			if !ok {
				t.Fatalf("stream closed after %q", got)
			}
			got = append(got, line.Message)
		case <-deadline:
			t.Fatalf("timed out after %q", got)
		}
	}
	idxOut := slices.Index(got, "schema up to date")
	idxStart := slices.Index(got, "==> Starting pre-deploy: node migrate.js")
	idxComplete := slices.Index(got, rendersStepComplete)
	if idxStart < 0 || idxOut < 0 || idxComplete < 0 || !(idxStart < idxOut && idxOut < idxComplete) {
		t.Fatalf("typeless tail order = %q, want start < output < complete", got)
	}
}

// w32/092: when a native pre-deploy pod starts running just before the
// operator/reconciler writes ContainerStartedAt, the tail waits briefly so
// `==> Running '<command>'` still lands before the command's output.
func TestWebSocketBuildTailOrdersNativeRunningBeforePreDeployOutput(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }

	const stepPodName = "predeploy-web-gen-7-race1"
	stepPod := runningPreDeployPod("web", stepPodName, app.Namespace, soon(10*time.Minute))
	svc := newService(map[string][]string{
		stepPodName: {soon(10*time.Minute+500*time.Millisecond).Format(time.RFC3339Nano) + " migrating tables"},
	}, app, stepPod)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond

	var mu sync.Mutex
	reads := 0
	one := int32(1)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		reads++
		d := DeployProgress{
			ID: "dep-7", Generation: 7, Status: deployStatusPreDeployInProgress, Built: true, Commit: "abc1234",
			CreatedAt: soon(9 * time.Minute), StartedAt: soon(9*time.Minute + time.Second),
			PreDeploy: PreDeployRun{Status: "running", StartedAt: soon(10 * time.Minute), Command: stepCommand, Native: true},
		}
		if reads >= 3 {
			d.PreDeploy.ContainerStartedAt = soon(10*time.Minute + time.Second).Truncate(time.Second)
		}
		if reads >= 5 {
			d.Status = deployStatusPreDeployFailed
			d.FinishedAt = soon(11 * time.Minute)
			d.PreDeploy.Status = "failed"
			d.PreDeploy.FinishedAt = soon(10*time.Minute + 5*time.Second)
			d.PreDeploy.ExitCode = &one
		}
		return []DeployProgress{d}, nil
	}

	srv := revalidationTestServer(svc)
	defer srv.Close()
	_, ch := dialLogWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build")

	var got []string
	deadline := time.After(5 * time.Second)
	for !slices.Contains(got, rendersTroubleshoot) {
		select {
		case line, ok := <-ch:
			if !ok {
				t.Fatalf("stream closed after %q", got)
			}
			got = append(got, line.Message)
		case <-deadline:
			t.Fatalf("timed out after %q", got)
		}
	}
	iStart := slices.Index(got, rendersStepStart)
	iRun := slices.Index(got, rendersStepRunning)
	iOut := slices.Index(got, "migrating tables")
	iFail := slices.Index(got, rendersStepFailed)
	if !(0 <= iStart && iStart < iRun && iRun < iOut && iOut < iFail) {
		t.Fatalf("order = %q, want start < Running < output < failed", got)
	}
}

// w32/092: when a pre-deploy pod's container never starts (StartError /
// ImagePullBackOff) and the deploy settles pre_deploy_failed, the tail emits
// the step's verdict promptly without hanging.
func TestWebSocketBuildTailDoesNotHangWhenPreDeployPodNeverStarts(t *testing.T) {
	app := sampleApp("web")
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }

	pod := preDeployPodIn("web", "predeploy-web-gen-8-nostart", app.Namespace)
	pod.UID = "uid-nostart"
	pod.CreationTimestamp = metav1.NewTime(soon(10 * time.Minute))
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodPending,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name:  core.PreDeployContainer,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CreateContainerError"}},
		}},
	}
	svc := newService(nil, app, pod)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{{
			ID: "dep-8", Generation: 8, Status: deployStatusPreDeployFailed, Image: "docker.io/library/node:22",
			CreatedAt: soon(9 * time.Minute), StartedAt: soon(9*time.Minute + time.Second), FinishedAt: soon(10*time.Minute + 3*time.Second),
			FailureReason: appv1alpha1.PreDeployCouldNotStartPrefix + `the program "release" does not exist in this image` + appv1alpha1.PreDeployNeverRanSuffix,
			PreDeploy: PreDeployRun{
				Status: "failed", StartedAt: soon(10 * time.Minute), FinishedAt: soon(10*time.Minute + 2*time.Second),
				Command: "release", Native: false,
			},
		}}, nil
	}

	srv := revalidationTestServer(svc)
	defer srv.Close()
	_, ch := dialLogWS(t, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build")

	var got []string
	deadline := time.After(5 * time.Second)
	for !slices.Contains(got, rendersTroubleshoot) {
		select {
		case line, ok := <-ch:
			if !ok {
				t.Fatalf("stream closed after %q", got)
			}
			got = append(got, line.Message)
		case <-deadline:
			t.Fatalf("timed out after %q", got)
		}
	}
	want := []string{
		"==> Deploy queued",
		"==> Deploying image docker.io/library/node:22",
		"==> Starting pre-deploy: release",
		rendersStepFailed,
		rendersExited128,
		rendersTroubleshoot,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("tail = %q\nwant %q", got, want)
	}
}
