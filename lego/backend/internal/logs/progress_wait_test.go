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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- m99: a queued deploy says why it waits ---
//
// The operator already writes the wait to the App's `Ready` condition; before
// m99 bex-api dropped it, so an 8¾-minute capacity wait read exactly like a
// stuck deploy (`==> Build queued`, then silence). These tests pin what a
// tenant is now told — and, just as load-bearing, what it is NOT told.

const workspaceCapMessage = "workspace has 2/2 concurrent builds active; waiting for a slot"

// queuedWaitDeploy is a repo-backed row still queued: created, never started.
func queuedWaitDeploy() DeployProgress {
	return DeployProgress{
		ID:        "dep-queued-1",
		Status:    "queued",
		Commit:    "abc1234def5678",
		CreatedAt: time.Date(2026, 9, 14, 10, 23, 1, 0, time.UTC),
	}
}

// waitingApp is a repo-backed App parked in PhaseBuilding with one wait reason
// on its current generation's Ready condition — exactly the shape the operator
// leaves behind while a build waits for a slot, a node, or a registry credential.
func waitingApp(name, reason, message string) *appv1alpha1.App {
	app := sampleApp(name)
	app.Spec.Repo = "https://github.com/x/y.git"
	app.Status.Phase = appv1alpha1.PhaseBuilding
	app.Status.Conditions = []metav1.Condition{{
		Type:               appv1alpha1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: app.Generation,
		LastTransitionTime: metav1.NewTime(time.Date(2026, 9, 14, 10, 23, 1, 0, time.UTC)),
	}}
	return app
}

// queuedWaitService wires a store-backed build read over one queued row.
func queuedWaitService(t *testing.T, app *appv1alpha1.App, extra ...client.Object) *Service {
	t.Helper()
	svc := newService(nil, append([]client.Object{app}, extra...)...)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{queuedWaitDeploy()}, nil
	}
	return svc
}

func buildNarration(t *testing.T, svc *Service, app string) []string {
	t.Helper()
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: app, Types: []string{LogTypeBuild}})
	if err != nil {
		t.Fatalf("QueryLogs(type=build): %v", err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message)
	}
	return out
}

// TestQueuedDeployNarratesTheWorkspaceBuildSlotWait is the milestone's first
// definition-of-done bullet: the tenant's own workspace cap is named, with its
// own counts, right under the queued line.
func TestQueuedDeployNarratesTheWorkspaceBuildSlotWait(t *testing.T) {
	svc := queuedWaitService(t, waitingApp("web", appv1alpha1.ReasonBuildQueued, workspaceCapMessage))
	got := buildNarration(t, svc, "web")
	want := []string{
		"==> Build queued",
		"==> Waiting for a build slot: this workspace has 2/2 builds running",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("queued narration = %v, want %v", got, want)
	}
}

// TestQueuedWaitCopyNeverLeaksAnotherTenantsActivity is the leak guard. The
// cluster-wide cap counts OTHER tenants' builds and the scheduler wait quotes a
// Kubernetes message naming nodes and taints; neither may reach a tenant. Only
// the one provably workspace-scoped message shape renders operator digits.
func TestQueuedWaitCopyNeverLeaksAnotherTenantsActivity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reason  string
		message string
		want    string
	}{
		{"workspace cap", appv1alpha1.ReasonBuildQueued, workspaceCapMessage,
			"==> Waiting for a build slot: this workspace has 2/2 builds running"},
		{"cluster cap", appv1alpha1.ReasonBuildQueued, "cluster has 4/4 concurrent builds active; waiting for a slot",
			waitLineNeutral},
		{"scheduler wait", appv1alpha1.ReasonBuildQueued, "waiting for build capacity: 0/7 nodes are available: 7 Insufficient cpu",
			waitLineNeutral},
		{"lost the last slot", appv1alpha1.ReasonBuildQueued, "a concurrent dispatch took the last build slot; waiting for a slot",
			waitLineNeutral},
		{"message a newer operator invents", appv1alpha1.ReasonBuildQueued, "tenant tea-other is hogging 9 builders on node-3",
			waitLineNeutral},
		{"empty message", appv1alpha1.ReasonBuildQueued, "", waitLineNeutral},
		{"registry credentials", appv1alpha1.ReasonRegistryCredsPending, "Waiting for the registry to accept this app's build credential",
			waitLineRegistryCreds},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := queuedWaitService(t, waitingApp("web", tc.reason, tc.message))
			got := buildNarration(t, svc, "web")
			if len(got) != 2 || got[1] != tc.want {
				t.Fatalf("narration = %v, want [\"==> Build queued\" %q]", got, tc.want)
			}
			if tc.want != waitLineNeutral {
				return
			}
			// The neutral line is a fixed constant, so nothing the operator
			// wrote can survive into it — least of all a count of builds this
			// tenant does not own.
			if strings.ContainsAny(got[1], "0123456789") {
				t.Errorf("neutral wait line %q carries a count — a tenant must not learn platform capacity", got[1])
			}
		})
	}
}

// TestQueuedWaitLineIsAbsentWithoutAReadableReason is the failure mode: an App
// with no Ready condition, or one stamped for a superseded generation, says
// nothing extra rather than guessing.
func TestQueuedWaitLineIsAbsentWithoutAReadableReason(t *testing.T) {
	bare := sampleApp("web")
	bare.Spec.Repo = "https://github.com/x/y.git"
	if got := buildNarration(t, queuedWaitService(t, bare), "web"); !slices.Equal(got, []string{"==> Build queued"}) {
		t.Errorf("no Ready condition => %v, want the bare queued line", got)
	}

	stale := waitingApp("web", appv1alpha1.ReasonBuildQueued, workspaceCapMessage)
	stale.Generation = 7 // the condition still reports generation 0
	if got := buildNarration(t, queuedWaitService(t, stale), "web"); !slices.Equal(got, []string{"==> Build queued"}) {
		t.Errorf("stale-generation condition => %v, want the bare queued line", got)
	}

	running := waitingApp("web", appv1alpha1.ReasonBuilding, "Building image from https://github.com/x/y.git")
	if got := buildNarration(t, queuedWaitService(t, running), "web"); !slices.Equal(got, []string{"==> Build queued"}) {
		t.Errorf("Building reason => %v, want no wait line", got)
	}
}

// TestNonQueuedRowsKeepTheirNarrationByteIdentical is the control: a wait
// reason lingering on the App must not add a line to a row that has already
// started or finished, and the pre-m99 lines are unchanged.
func TestNonQueuedRowsKeepTheirNarrationByteIdentical(t *testing.T) {
	app := waitingApp("web", appv1alpha1.ReasonBuildQueued, workspaceCapMessage)
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	row := inFlightDeploy() // build_in_progress: created + started
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}
	want := []string{"==> Build queued", "==> Building from https://github.com/x/y.git@abc1234"}
	if got := buildNarration(t, svc, "web"); !slices.Equal(got, want) {
		t.Fatalf("in-flight narration = %v, want %v", got, want)
	}
}

// TestQueuedWaitNarrationIsNotSourcedFromFailureReason keeps `failureReason`
// terminal-only on the narration side: a live wait must never be rendered from
// the field that means "this deploy failed" (the row carries none while it
// waits, and if one somehow existed it would still not surface here).
func TestQueuedWaitNarrationIsNotSourcedFromFailureReason(t *testing.T) {
	app := waitingApp("web", appv1alpha1.ReasonBuildQueued, workspaceCapMessage)
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	row := queuedWaitDeploy()
	row.FailureReason = "the build never started: " + workspaceCapMessage
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}
	for _, msg := range buildNarration(t, svc, "web") {
		if strings.Contains(msg, "the build never started") {
			t.Fatalf("queued narration %q rendered a terminal failureReason", msg)
		}
	}
}

// TestQueuedWaitLineIsIdenticalOnEveryLogSurface is t003: the line is produced
// once, in the core verb, so REST `GET /v1/logs`, GraphQL `logs` and the MCP
// `list_logs` tool carry it byte for byte. Driving all three in one test is the
// point — asserting each in isolation would not catch an adapter that rewrote
// or truncated it.
func TestQueuedWaitLineIsIdenticalOnEveryLogSurface(t *testing.T) {
	const wantLine = "==> Waiting for a build slot: this workspace has 2/2 builds running"
	svc := queuedWaitService(t, waitingApp("web", appv1alpha1.ReasonBuildQueued, workspaceCapMessage))

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/logs?resource=web&type=build", nil))
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode REST logs: %v (body %s)", err, rec.Body.String())
	}
	var restLine string
	for _, l := range env.Logs {
		if strings.HasPrefix(l.Message, "==> Waiting") {
			restLine = l.Message
		}
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", type:"build") { logs { message } } }`)
	var gqlLine string
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		msg, _ := row.(map[string]any)["message"].(string)
		if strings.HasPrefix(msg, "==> Waiting") {
			gqlLine = msg
		}
	}

	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("mcp server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("mcp client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	out, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "type": []string{LogTypeBuild},
	}})
	if err != nil || out.IsError {
		t.Fatalf("list_logs = %+v, err %v", out, err)
	}
	var mcpEnv struct {
		Logs []struct {
			Message string `json:"message"`
		} `json:"logs"`
	}
	mcpText := out.Content[0].(*mcp.TextContent).Text
	if err := json.Unmarshal([]byte(mcpText), &mcpEnv); err != nil {
		t.Fatalf("decode MCP list_logs payload: %v (%s)", err, mcpText)
	}
	var mcpLine string
	for _, l := range mcpEnv.Logs {
		if strings.HasPrefix(l.Message, "==> Waiting") {
			mcpLine = l.Message
		}
	}

	if restLine != wantLine {
		t.Errorf("REST line = %q, want %q", restLine, wantLine)
	}
	if gqlLine != restLine {
		t.Errorf("GraphQL line = %q, REST line = %q — the surfaces have drifted", gqlLine, restLine)
	}
	if mcpLine != restLine {
		t.Errorf("MCP line = %q, REST line = %q — the surfaces have drifted", mcpLine, restLine)
	}
}

// TestBuildTailReEmitsOnlyWhenTheWaitReasonChanges is the re-emission contract:
// the SSE tail (the fourth surface, which shares this producer) narrates the
// CURRENT reason, once per distinct reason — not once per poll.
func TestBuildTailReEmitsOnlyWhenTheWaitReasonChanges(t *testing.T) {
	pending := buildPodFor("web", "bld-web-gen-1-wait", "builds", "buildkit", time.Unix(1, 0))
	pending.Status = corev1.PodStatus{Phase: corev1.PodPending}
	app := waitingApp("web", appv1alpha1.ReasonBuildQueued, workspaceCapMessage)
	svc := newService(map[string][]string{
		pending.Name: {"2026-09-14T10:31:40Z #1 real build line"},
	}, app, pending)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{queuedWaitDeploy()}, nil
	}

	// Many ticks pass on the first reason (proving no per-poll spam), then the
	// reason changes twice, then the pod starts and ends the tail.
	go func() {
		time.Sleep(60 * time.Millisecond)
		setWaitReason(t, svc, "web", appv1alpha1.ReasonBuildQueued, "workspace has 1/2 concurrent builds active; waiting for a slot")
		time.Sleep(40 * time.Millisecond)
		setWaitReason(t, svc, "web", appv1alpha1.ReasonRegistryCredsPending, "Waiting for the registry to accept this app's build credential")
		time.Sleep(40 * time.Millisecond)
		started := buildPodFor("web", pending.Name, "builds", "buildkit", time.Unix(1, 0))
		started.ResourceVersion = ""
		var cur corev1.Pod
		if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "builds", Name: pending.Name}, &cur); err != nil {
			t.Errorf("get build pod: %v", err)
			return
		}
		started.ResourceVersion = cur.ResourceVersion
		if err := svc.Client.Status().Update(context.Background(), started); err != nil {
			t.Errorf("flip pod to Running: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var msgs []string
	if err := svc.FollowLogs(ctx, LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(e LogEntry) error {
		msgs = append(msgs, e.Message)
		return nil
	}); err != nil {
		t.Fatalf("FollowLogs: %v", err)
	}
	want := []string{
		"==> Build queued",
		"==> Waiting for a build slot: this workspace has 2/2 builds running",
		"==> Waiting for a build slot: this workspace has 1/2 builds running",
		waitLineRegistryCreds,
		"#1 real build line",
	}
	if !slices.Equal(msgs, want) {
		t.Fatalf("tail = %v,\nwant one line per distinct reason (never per poll): %v", msgs, want)
	}
}

// setWaitReason rewrites the App's Ready condition mid-tail, the way a
// reconcile does.
func setWaitReason(t *testing.T, svc *Service, name, reason, message string) {
	t.Helper()
	ctx := context.Background()
	var cur appv1alpha1.App
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, &cur); err != nil {
		t.Errorf("get app: %v", err)
		return
	}
	cur.Status.Conditions[0].Reason = reason
	cur.Status.Conditions[0].Message = message
	if err := svc.Client.Update(ctx, &cur); err != nil {
		t.Errorf("update app condition: %v", err)
	}
}

// --- w4/m110 t004: no build, no build narration ---
//
// Every repo-backed deploy used to be narrated as a build, because the two
// lines were synthesized from the row's own timestamps. Live on 2026-09-17,
// three config-change deploys that reused the active artifact (13-30s, no
// build steps, no Build events, same image digest) each logged
// `==> Build queued` / `==> Building from …@039c347` and read as builds. The
// rollback told the same story about its intentional reuse.

// buildLessRow is a terminal repo-backed deploy the Events tab has no
// build_started fact for — the shape of a reuse rollout.
func buildLessRow() DeployProgress {
	d := inFlightDeploy()
	d.Status = "live"
	d.FinishedAt = d.StartedAt.Add(13 * time.Second)
	d.Built = false
	return d
}

func TestBuildlessDeployNarratesTheRolloutNotABuild(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	row := buildLessRow()
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}

	got := buildNarration(t, svc, "web")
	for _, line := range got {
		if strings.Contains(line, "Build queued") || strings.Contains(line, "Building from") {
			t.Fatalf("buildless deploy claims a build: %v", got)
		}
	}
	want := []string{"==> Deploy queued", "==> Rolling out the current release", "==> Your service is live 🎉"}
	if !slices.Equal(got, want) {
		t.Fatalf("buildless narration = %v, want %v", got, want)
	}
}

// A rollback carries the exact image it restores, so it names it rather than
// claiming a build of the commit that image happens to hold.
func TestRollbackNarratesTheRestoredImage(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	row := buildLessRow()
	row.Image = "zot.example/ws/web:gen-1@sha256:f825b7"
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}

	want := []string{
		"==> Deploy queued",
		"==> Deploying image zot.example/ws/web:gen-1@sha256:f825b7",
		"==> Your service is live 🎉",
	}
	if got := buildNarration(t, svc, "web"); !slices.Equal(got, want) {
		t.Fatalf("rollback narration = %v, want %v", got, want)
	}
}

// The control: a deploy that DID build keeps its lines byte-identical.
func TestBuiltDeployKeepsItsBuildNarration(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	row := buildLessRow()
	row.Built = true
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}

	want := []string{
		"==> Build queued",
		"==> Building from https://github.com/x/y.git@abc1234",
		"==> Your service is live 🎉",
	}
	if got := buildNarration(t, svc, "web"); !slices.Equal(got, want) {
		t.Fatalf("built narration = %v, want %v", got, want)
	}
}

// An OPEN deploy keeps the build story: build_started is written as the
// reconciler observes the dispatch, so a row still in flight may simply not
// have its fact yet, and a still-queued row has none by design. Only a
// terminal row with no fact is settled evidence that no build ran.
func TestOpenDeployIsNotAccusedOfSkippingItsBuild(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	row := inFlightDeploy() // no FinishedAt, Built false
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}

	want := []string{"==> Build queued", "==> Building from https://github.com/x/y.git@abc1234"}
	if got := buildNarration(t, svc, "web"); !slices.Equal(got, want) {
		t.Fatalf("in-flight narration = %v, want %v", got, want)
	}
}
