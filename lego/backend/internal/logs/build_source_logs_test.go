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
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func sampleBuildSourceCarrier(ws, bdsID, displayName, repo string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws + "-" + bdsID,
			Namespace: ws,
			Labels: map[string]string{
				core.LabelTenant:      ws,
				core.LabelAppID:       bdsID,
				core.LabelServiceName: bdsID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: displayName,
			Repo:        repo,
			Branch:      "main",
		},
	}
}

func sampleWorkspaceService(ws, srvID, name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws + "-" + name,
			Namespace: ws,
			Labels: map[string]string{
				core.LabelTenant:      ws,
				core.LabelAppID:       srvID,
				core.LabelServiceName: name,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:   appv1alpha1.TypeWebService,
			Repo:   "https://github.com/bex-co/bex",
			Branch: "main",
		},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
}

type roleAuthzChecker struct {
	mu      sync.Mutex
	allowed map[string]bool // "subject|relation|object" -> allow
}

func (c *roleAuthzChecker) Check(_ context.Context, subject, relation, object string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.allowed[subject+"|"+relation+"|"+object], nil
}

func (c *roleAuthzChecker) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	return c.Check(ctx, subject, relation, object)
}

func (c *roleAuthzChecker) Grant(subject, relation, object string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.allowed == nil {
		c.allowed = map[string]bool{}
	}
	c.allowed["user:"+subject+"|"+relation+"|"+object] = true
}

func (c *roleAuthzChecker) Revoke(subject, relation, object string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.allowed, "user:"+subject+"|"+relation+"|"+object)
}

type fixedWorkspaces map[string][]string

func (w fixedWorkspaces) Tenant(_ context.Context, id core.Identity) (string, bool) {
	if list := w[id.Subject]; len(list) > 0 {
		return list[0], true
	}
	return "", false
}

func (w fixedWorkspaces) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	return slices.Contains(w[id.Subject], tenantID), nil
}

func TestBuildSourceNewBuildIndexAndPublicBuildID(t *testing.T) {
	dep1 := ids.New(ids.Deploy)
	dep2 := ids.New(ids.Deploy)
	bld1, ok1 := ids.Rekind(ids.Build, dep1)
	bld2, ok2 := ids.Rekind(ids.Build, dep2)
	if !ok1 || !ok2 {
		t.Fatalf("Rekind failed: %v %v", ok1, ok2)
	}
	rows := []DeployProgress{
		{ID: dep2, Generation: 2, Built: true},
		{ID: dep1, Generation: 1, Built: true},
	}

	carrierCR := "tea-a-bds-carrier"
	bsIndex := newBuildIndex(carrierCR, rows, true)
	job1 := appv1alpha1.BuildJobName(carrierCR, appv1alpha1.BuildRevision(1))
	job2 := appv1alpha1.BuildJobName(carrierCR, appv1alpha1.BuildRevision(2))

	if got := bsIndex.buildOf(job1 + "-aaaaa"); got != bld1 {
		t.Errorf("carrier buildOf(gen-1) = %q, want %q", got, bld1)
	}
	if got := bsIndex.buildOf(job2 + "-bbbbb"); got != bld2 {
		t.Errorf("carrier buildOf(gen-2) = %q, want %q", got, bld2)
	}
	if jobs := bsIndex.jobsOf([]string{bld2}); !slices.Equal(jobs, []string{job2}) {
		t.Errorf("carrier jobsOf([%s]) = %v, want [%s]", bld2, jobs, job2)
	}
	if jobs := bsIndex.jobsOf([]string{dep2}); len(jobs) != 0 {
		t.Errorf("carrier jobsOf([internal %s]) = %v, want empty", dep2, jobs)
	}

	labeled := bsIndex.label([]LogEntry{
		{Message: "step 1", Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: job2 + "-bbbbb"}},
	})
	if got := labeled[0].Labels[LabelBuild]; got != bld2 {
		t.Errorf("carrier labeled build = %q, want %q", got, bld2)
	}

	// Ordinary services keep dep-… unchanged.
	svcIndex := newBuildIndex("web", rows, false)
	svcJob2 := appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(2))
	if got := svcIndex.buildOf(svcJob2 + "-ccccc"); got != dep2 {
		t.Errorf("service buildOf(gen-2) = %q, want %q", got, dep2)
	}
	if jobs := svcIndex.jobsOf([]string{dep2}); !slices.Equal(jobs, []string{svcJob2}) {
		t.Errorf("service jobsOf([%s]) = %v, want [%s]", dep2, jobs, svcJob2)
	}
}

func TestBuildSourceProgressNarrationIsBuildOnly(t *testing.T) {
	base := time.Date(2026, 11, 23, 23, 0, 0, 0, time.UTC)
	depOK := ids.New(ids.Deploy)
	bldOK, _ := ids.Rekind(ids.Build, depOK)
	depFail := ids.New(ids.Deploy)
	bldFail, _ := ids.Rekind(ids.Build, depFail)
	depInfra := ids.New(ids.Deploy)
	bldInfra, _ := ids.Rekind(ids.Build, depInfra)
	depCancel := ids.New(ids.Deploy)
	bldCancel, _ := ids.Rekind(ids.Build, depCancel)
	depQueued := ids.New(ids.Deploy)
	bldQueued, _ := ids.Rekind(ids.Build, depQueued)

	exit1 := int32(1)
	pc := progressContext{
		repo:        "https://github.com/bex-co/monorepo",
		branch:      "main",
		serviceType: appv1alpha1.TypeBuildSource,
		wait:        buildWait{Reason: appv1alpha1.ReasonBuildQueued, Message: "workspace has 2/2 concurrent builds active; waiting for a slot"},
		hook:        hookRun{generation: 2, status: appv1alpha1.PreDeploySucceeded, startedAt: base.Add(30 * time.Second), finishedAt: base.Add(35 * time.Second)},
		oidc:        oidcRefusal{generation: 2, lines: []string{oidcRefusalLine([]string{"AWS_ROLE_ARN"})}},
	}

	// Populate every runtime-only field to prove carrier narration suppresses all of them.
	succeeded := DeployProgress{
		ID:                    depOK,
		Generation:            2,
		Status:                "live",
		Built:                 true,
		Commit:                "abcdef1234567890",
		DetectedHTTPPort:      8080,
		CreatedAt:             base,
		StartedAt:             base.Add(2 * time.Second),
		FinishedAt:            base.Add(20 * time.Second),
		PortScanSince:         base.Add(10 * time.Second),
		RolloutStartedAt:      base.Add(12 * time.Second),
		AppStartedAt:          []time.Time{base.Add(14 * time.Second)},
		StartCommand:          "node server.js",
		WebConcurrencyDefault: 2,
		LiveURLs:              []string{"https://should-never-appear.onbex.co"},
		PreDeploy:             PreDeployRun{Status: preDeploySucceeded, StartedAt: base.Add(8 * time.Second), FinishedAt: base.Add(9 * time.Second), Command: "npm run migrate", Native: true},
		ReadAt:                base.Add(30 * time.Second),
	}
	entries := progressLines(succeeded, pc)
	wantMsgs := []string{
		"==> Build queued",
		"==> Building from https://github.com/bex-co/monorepo@abcdef1",
		"==> Build succeeded 🎉",
	}
	gotMsgs := make([]string, 0, len(entries))
	for _, e := range entries {
		gotMsgs = append(gotMsgs, displayText(e.Message))
		if e.Labels[LabelType] != LogTypeBuild {
			t.Errorf("line %q type = %q, want build", e.Message, e.Labels[LabelType])
		}
		if e.Labels[LabelBuild] != bldOK {
			t.Errorf("line %q build label = %q, want %q (never internal %q)", e.Message, e.Labels[LabelBuild], bldOK, depOK)
		}
		if e.Labels[LabelInstance] != bldOK {
			t.Errorf("line %q instance label = %q, want %q", e.Message, e.Labels[LabelInstance], bldOK)
		}
	}
	if !slices.Equal(gotMsgs, wantMsgs) {
		t.Fatalf("succeeded carrier narration = %v, want %v", gotMsgs, wantMsgs)
	}

	// Build command failure emits Render's build failed verdict with bld-… label.
	failed := DeployProgress{
		ID:            depFail,
		Generation:    3,
		Status:        "build_failed",
		Built:         true,
		FailureReason: appv1alpha1.FormatBuildFailed("build command", "exit status 1"),
		CreatedAt:     base,
		StartedAt:     base.Add(time.Second),
		FinishedAt:    base.Add(10 * time.Second),
		ExitCode:      &exit1,
		FirstExitAt:   base.Add(5 * time.Second),
		FirstExitCode: &exit1,
	}
	failEntries := progressLines(failed, pc)
	failShown := shown(failEntries)
	if !slices.Contains(failShown, buildFailedLine) || !slices.Contains(failShown, troubleshootLine) {
		t.Errorf("failed carrier narration = %v, want build failed verdict", failShown)
	}
	for _, e := range failEntries {
		if e.Labels[LabelBuild] != bldFail {
			t.Errorf("failed line %q build = %q, want %q", e.Message, e.Labels[LabelBuild], bldFail)
		}
		if strings.Contains(displayText(e.Message), "Exited with status") || strings.Contains(displayText(e.Message), "service is live") {
			t.Errorf("failed carrier emitted runtime phrasing: %q", e.Message)
		}
	}

	// Non-tenant build failure keeps cause with Build failed prefix.
	infraFailed := DeployProgress{
		ID:            depInfra,
		Generation:    4,
		Status:        "build_failed",
		FailureReason: "build timed out after 30m",
		CreatedAt:     base,
		StartedAt:     base.Add(time.Second),
		FinishedAt:    base.Add(10 * time.Second),
	}
	infraEntries := progressLines(infraFailed, pc)
	if !slices.Contains(shown(infraEntries), "==> Build failed: build timed out after 30m") {
		t.Errorf("infra failed carrier narration = %v", shown(infraEntries))
	}
	for _, e := range infraEntries {
		if e.Labels[LabelBuild] != bldInfra {
			t.Errorf("infra failed line build = %q, want %q", e.Labels[LabelBuild], bldInfra)
		}
	}

	// Canceled while queued still reads Build queued + Build canceled (never Deploy queued / Deploy canceled).
	canceled := DeployProgress{
		ID:         depCancel,
		Generation: 5,
		Status:     "canceled",
		CreatedAt:  base,
		FinishedAt: base.Add(4 * time.Second),
	}
	cancelEntries := progressLines(canceled, pc)
	if got := shown(cancelEntries); !slices.Equal(got, []string{"==> Build queued", "==> Build canceled"}) {
		t.Errorf("canceled carrier narration = %v, want [==> Build queued, ==> Build canceled]", got)
	}
	for _, e := range cancelEntries {
		if e.Labels[LabelBuild] != bldCancel {
			t.Errorf("canceled line build = %q, want %q", e.Labels[LabelBuild], bldCancel)
		}
	}

	// Queued carrier narrates build slot wait with bld-… label.
	queued := DeployProgress{ID: depQueued, Status: deployStatusQueued, CreatedAt: base}
	queuedEntries := progressLines(queued, pc)
	if got := shown(queuedEntries); !slices.Equal(got, []string{"==> Build queued", "==> Waiting for a build slot: this workspace has 2/2 builds running"}) {
		t.Errorf("queued carrier narration = %v", got)
	}
	for _, e := range queuedEntries {
		if e.Labels[LabelBuild] != bldQueued {
			t.Errorf("queued line build = %q, want %q", e.Labels[LabelBuild], bldQueued)
		}
	}

	// Image-backed Build Source emits no build or deploy narration.
	imgPC := progressContext{serviceType: appv1alpha1.TypeBuildSource}
	imgRow := DeployProgress{ID: depOK, Status: "live", Image: "docker.io/library/nginx:1.27", CreatedAt: base, StartedAt: base.Add(time.Second), FinishedAt: base.Add(5 * time.Second)}
	if got := progressLines(imgRow, imgPC); len(got) != 0 {
		t.Errorf("image-backed Build Source narrated %+v, want none", got)
	}
}

func TestBuildSourceLogsHistoryLabelValuesAndCrossSurface(t *testing.T) {
	ws := "tea-d0000000000000000001"
	bdsID := ids.New(ids.BuildSource)
	srvID := ids.New(ids.Service)
	dep1 := ids.New(ids.Deploy)
	dep2 := ids.New(ids.Deploy)
	bld1, _ := ids.Rekind(ids.Build, dep1)
	bld2, _ := ids.Rekind(ids.Build, dep2)
	svcDep := ids.New(ids.Deploy)

	carrier := sampleBuildSourceCarrier(ws, bdsID, "mono-builds", "https://github.com/bex-co/monorepo")
	svcApp := sampleWorkspaceService(ws, srvID, "api")

	base := time.Date(2026, 11, 23, 22, 0, 0, 0, time.UTC)
	pod1 := appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(1)) + "-aaaaa"
	pod2 := appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(2)) + "-bbbbb"
	svcBuildPod := appv1alpha1.BuildJobName(svcApp.Name, appv1alpha1.BuildRevision(1)) + "-ccccc"

	loki := newFrontendLoki(t,
		fakeStream{
			labels: map[string]string{"namespace": ws, "app": carrier.Name, "app_id": bdsID, "type": LogTypeBuild, "pod": pod1},
			stamps: []int64{base.Add(15 * time.Second).UnixNano()},
			text:   func(int64) string { return "carrier build 1 output" },
		},
		fakeStream{
			labels: map[string]string{"namespace": ws, "app": carrier.Name, "app_id": bdsID, "type": LogTypeBuild, "pod": pod2},
			stamps: []int64{base.Add(45 * time.Second).UnixNano()},
			text:   func(int64) string { return "carrier build 2 output" },
		},
		fakeStream{
			labels: map[string]string{"namespace": ws, "app": svcApp.Name, "app_id": srvID, "type": LogTypeBuild, "pod": svcBuildPod},
			stamps: []int64{base.Add(25 * time.Second).UnixNano()},
			text:   func(int64) string { return "service build output" },
		},
		fakeStream{
			labels: map[string]string{"namespace": ws, "app": svcApp.Name, "app_id": srvID, "container": "app", "type": LogTypeApp, "pod": svcApp.Name + "-1", "level": "info"},
			stamps: []int64{base.Add(50 * time.Second).UnixNano()},
			text:   func(int64) string { return "service app output" },
		},
	)

	svc := newService(nil, carrier, svcApp, podFor(svcApp.Name, svcApp.Name+"-1"))
	svc.History = NewLokiSource(loki.srv.URL, loki.srv.Client())
	svc.LabelValues = func(_ context.Context, ns, label string, q LogQuery) ([]string, error) {
		if ns != ws || q.App != carrier.Name {
			if q.App == svcApp.Name && label == LabelType {
				return []string{LogTypeApp, LogTypeBuild}, nil
			}
			return nil, nil
		}
		switch label {
		case LabelType:
			return []string{LogTypeBuild}, nil
		case LabelInstance:
			var out []string
			for _, p := range []string{pod1, pod2} {
				if q.keepPod(p) {
					out = append(out, p)
				}
			}
			return out, nil
		}
		return nil, nil
	}
	svc.DeployProgress = func(_ context.Context, resource string, _ time.Time) ([]DeployProgress, error) {
		switch resource {
		case bdsID:
			return []DeployProgress{
				{ID: dep2, Generation: 2, Status: "live", Built: true, Commit: "2222222aaaa", CreatedAt: base.Add(40 * time.Second), StartedAt: base.Add(42 * time.Second), FinishedAt: base.Add(50 * time.Second)},
				{ID: dep1, Generation: 1, Status: "build_failed", Built: true, Commit: "1111111bbbb", FailureReason: appv1alpha1.FormatBuildFailed("build command", "exit 1"), CreatedAt: base.Add(10 * time.Second), StartedAt: base.Add(12 * time.Second), FinishedAt: base.Add(20 * time.Second)},
			}, nil
		case srvID:
			return []DeployProgress{
				{ID: svcDep, Generation: 1, Status: "live", Built: true, Commit: "3333333cccc", CreatedAt: base.Add(20 * time.Second), StartedAt: base.Add(22 * time.Second), FinishedAt: base.Add(30 * time.Second)},
			}, nil
		}
		return nil, nil
	}

	startStr := base.Format(time.RFC3339)
	endStr := base.Add(2 * time.Minute).Format(time.RFC3339)

	// 1. REST: untyped resource=bds-… returns both builds' pod + narration lines labeled with bld-… (never dep-…).
	allRec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+bdsID+"&startTime="+startStr+"&endTime="+endStr+"&limit=100")
	if allRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs?resource=bds: %d %s", allRec.Code, allRec.Body)
	}
	allEnv := decodeLogList(t, allRec)
	var allMsgs []string
	for _, l := range allEnv.Logs {
		allMsgs = append(allMsgs, displayText(l.Message))
		var gotRes, gotBuild, gotType string
		for _, lb := range l.Labels {
			switch lb.Name {
			case "resource":
				gotRes = lb.Value
			case LabelBuild:
				gotBuild = lb.Value
			case LabelType:
				gotType = lb.Value
			}
		}
		if gotRes != bdsID || gotType != LogTypeBuild || (gotBuild != bld1 && gotBuild != bld2) {
			t.Errorf("unfiltered bds line %q labels = %+v, want resource=%s type=build build in {%s,%s}", l.Message, l.Labels, bdsID, bld1, bld2)
		}
		if strings.Contains(gotBuild, "dep-") {
			t.Errorf("leaked internal deploy id in build label: %+v", l.Labels)
		}
	}
	for _, want := range []string{"carrier build 1 output", "carrier build 2 output", "==> Build succeeded 🎉", buildFailedLine} {
		if !slices.Contains(allMsgs, want) {
			t.Errorf("unfiltered bds logs missing %q: %v", want, allMsgs)
		}
	}

	// 2. REST: build=bld2 returns ONLY build 2's pod line and narration (never build 1, never dep2).
	b2Rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+bdsID+"&build="+bld2+"&startTime="+startStr+"&endTime="+endStr+"&limit=100")
	if b2Rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs?resource=bds&build=bld2: %d %s", b2Rec.Code, b2Rec.Body)
	}
	b2Env := decodeLogList(t, b2Rec)
	if len(b2Env.Logs) != 4 {
		t.Fatalf("build=bld2 returned %d lines (%+v), want 4 (queued, building, pod output, succeeded)", len(b2Env.Logs), b2Env.Logs)
	}
	for _, l := range b2Env.Logs {
		for _, lb := range l.Labels {
			if lb.Name == LabelBuild && lb.Value != bld2 {
				t.Errorf("build=bld2 line %q had build=%q, want %q", l.Message, lb.Value, bld2)
			}
		}
		if strings.Contains(l.Message, "build 1") || strings.Contains(l.Message, "Build failed") {
			t.Errorf("build=bld2 leaked build 1 line: %q", l.Message)
		}
	}

	// 3. Internal dep-… ID and unknown bld-… ID return empty, never leaking another build.
	for _, badBuild := range []string{dep2, ids.New(ids.Build)} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+bdsID+"&build="+badBuild+"&startTime="+startStr+"&endTime="+endStr)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/logs?build=%s: %d %s", badBuild, rec.Code, rec.Body)
		}
		if env := decodeLogList(t, rec); len(env.Logs) != 0 {
			t.Errorf("build=%s returned %d lines, want 0", badBuild, len(env.Logs))
		}
	}

	// 4. Non-build types on a Build Source answer empty; carrier cannot be addressed by display name or CR name.
	for _, lt := range []string{LogTypeApp, LogTypeRequest, LogTypePreDeploy} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+bdsID+"&type="+lt+"&startTime="+startStr+"&endTime="+endStr)
		if rec.Code != http.StatusOK || len(decodeLogList(t, rec).Logs) != 0 {
			t.Errorf("bds type=%s = %d %s, want 200 empty", lt, rec.Code, rec.Body)
		}
	}
	for _, nonID := range []string{"mono-builds", carrier.Name} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+nonID+"&startTime="+startStr+"&endTime="+endStr)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /v1/logs?resource=%s = %d %s, want 404 (carrier only addressable by explicit bds- id)", nonID, rec.Code, rec.Body)
		}
	}

	// 5. Label values: label=type -> ["build"], label=instance&build=bld2 -> projected pod2 + bld2 narration instances,
	//    and passing that projected narration instance back filters cleanly; label=build -> 400 (Render contract).
	typeValsRec := serveREST(svc, http.MethodGet, "/v1/logs/values?resource="+bdsID+"&label=type&startTime="+startStr+"&endTime="+endStr)
	if typeValsRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs/values?label=type: %d %s", typeValsRec.Code, typeValsRec.Body)
	}
	var typeVals []string
	if err := json.Unmarshal(typeValsRec.Body.Bytes(), &typeVals); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(typeVals, []string{LogTypeBuild}) {
		t.Errorf("label=type = %v, want [build]", typeVals)
	}

	instValsRec := serveREST(svc, http.MethodGet, "/v1/logs/values?resource="+bdsID+"&label=instance&build="+bld2+"&startTime="+startStr+"&endTime="+endStr)
	if instValsRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs/values?label=instance&build=bld2: %d %s", instValsRec.Code, instValsRec.Body)
	}
	var instVals []string
	if err := json.Unmarshal(instValsRec.Body.Bytes(), &instVals); err != nil {
		t.Fatal(err)
	}
	wantPod2Inst := projectLogInstance(bdsID, pod2)
	wantNarr2Inst := projectLogInstance(bdsID, bld2)
	if !slices.Contains(instVals, wantPod2Inst) || !slices.Contains(instVals, wantNarr2Inst) || len(instVals) != 2 {
		t.Errorf("label=instance&build=bld2 = %v, want [%s, %s]", instVals, wantPod2Inst, wantNarr2Inst)
	}
	// Filtering by the discovered projected narration instance returns bld2's narration lines only.
	narrRec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+bdsID+"&build="+bld2+"&instance="+wantNarr2Inst+"&startTime="+startStr+"&endTime="+endStr)
	narrEnv := decodeLogList(t, narrRec)
	if len(narrEnv.Logs) != 3 {
		t.Errorf("instance=%s returned %d lines (%+v), want 3 narration lines", wantNarr2Inst, len(narrEnv.Logs), narrEnv.Logs)
	}
	if badLabel := serveREST(svc, http.MethodGet, "/v1/logs/values?resource="+bdsID+"&label=build&startTime="+startStr+"&endTime="+endStr); badLabel.Code != http.StatusBadRequest {
		t.Errorf("GET /v1/logs/values?label=build = %d, want 400 (Render contract)", badLabel.Code)
	}

	// 6. GraphQL: logs(resource, build), logs(resource, deployId: bld2), logLabelValues, logRetention.
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gqlData := runQuery(t, schema, `{ logs(resource: "`+bdsID+`", build: ["`+bld2+`"], startTime: "`+startStr+`", endTime: "`+endStr+`") { logs { resource type build message } } }`)
	gqlLogs := gqlData["logs"].(map[string]any)["logs"].([]any)
	if len(gqlLogs) != len(b2Env.Logs) {
		t.Fatalf("GraphQL logs count = %d, REST count = %d", len(gqlLogs), len(b2Env.Logs))
	}
	for _, raw := range gqlLogs {
		row := raw.(map[string]any)
		if row["resource"] != bdsID || row["type"] != LogTypeBuild || row["build"] != bld2 {
			t.Errorf("GraphQL row = %+v, want resource=%s type=build build=%s", row, bdsID, bld2)
		}
	}
	gqlDeployScope := runQuery(t, schema, `{ logs(resource: "`+bdsID+`", type: "build", deployId: "`+bld2+`", startTime: "`+startStr+`", endTime: "`+endStr+`") { logs { build message } } }`)
	if rows := gqlDeployScope["logs"].(map[string]any)["logs"].([]any); len(rows) != len(b2Env.Logs) {
		t.Errorf("GraphQL deployId=%s count = %d, want %d", bld2, len(rows), len(b2Env.Logs))
	}
	gqlVals := runQuery(t, schema, `{ logLabelValues(resource: "`+bdsID+`", label: "instance", build: ["`+bld2+`"], startTime: "`+startStr+`", endTime: "`+endStr+`") }`)
	if got := gqlVals["logLabelValues"].([]any); len(got) != 2 {
		t.Errorf("GraphQL logLabelValues = %v, want 2 instances", got)
	}

	// 7. MCP: list_logs and list_log_label_values match REST identically.
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	svc.RegisterMCP(mcpSrv)
	ct, st := mcp.NewInMemoryTransports()
	if _, err := mcpSrv.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "1"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	mcpLogsRes, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_logs",
		Arguments: map[string]any{
			"resource":  []string{bdsID},
			"build":     []string{bld2},
			"startTime": startStr,
			"endTime":   endStr,
			"limit":     100,
		},
	})
	if err != nil || mcpLogsRes.IsError {
		t.Fatalf("MCP list_logs = %v %+v", err, mcpLogsRes)
	}
	var mcpEnv renderLogList
	if err := json.Unmarshal([]byte(mcpLogsRes.Content[0].(*mcp.TextContent).Text), &mcpEnv); err != nil {
		t.Fatal(err)
	}
	if len(mcpEnv.Logs) != len(b2Env.Logs) {
		t.Fatalf("MCP list_logs count = %d, REST count = %d", len(mcpEnv.Logs), len(b2Env.Logs))
	}
	for i := range mcpEnv.Logs {
		if mcpEnv.Logs[i].Message != b2Env.Logs[i].Message || !slices.Equal(mcpEnv.Logs[i].Labels, b2Env.Logs[i].Labels) {
			t.Errorf("MCP line %d = %+v, REST = %+v", i, mcpEnv.Logs[i], b2Env.Logs[i])
		}
	}

	mcpValsRes, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_log_label_values",
		Arguments: map[string]any{
			"resource":  []string{bdsID},
			"label":     LabelInstance,
			"build":     []string{bld2},
			"startTime": startStr,
			"endTime":   endStr,
		},
	})
	if err != nil || mcpValsRes.IsError {
		t.Fatalf("MCP list_log_label_values = %v %+v", err, mcpValsRes)
	}
	var mcpInstVals []string
	if err := json.Unmarshal([]byte(mcpValsRes.Content[0].(*mcp.TextContent).Text), &mcpInstVals); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mcpInstVals, instVals) {
		t.Errorf("MCP list_log_label_values = %v, REST = %v", mcpInstVals, instVals)
	}

	// 8. Service.Logs convenience verb on bds-… returns build logs labeled with bld-….
	tailEntries, err := svc.Logs(context.Background(), bdsID, 50)
	if err != nil || len(tailEntries) == 0 {
		t.Fatalf("Logs(%s) = %d, %v", bdsID, len(tailEntries), err)
	}
	for _, e := range tailEntries {
		if e.Labels[LabelBuild] != bld1 && e.Labels[LabelBuild] != bld2 {
			t.Errorf("Logs(%s) entry %q build=%q, want bld1/bld2", bdsID, e.Message, e.Labels[LabelBuild])
		}
	}

	// 9. Ordinary service build logs stay dep-… and never include carrier logs.
	srvRec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+srvID+"&build="+svcDep+"&startTime="+startStr+"&endTime="+endStr)
	srvEnv := decodeLogList(t, srvRec)
	if len(srvEnv.Logs) == 0 {
		t.Fatalf("service build=%s returned 0 lines", svcDep)
	}
	for _, l := range srvEnv.Logs {
		if strings.Contains(l.Message, "carrier build") {
			t.Errorf("service build logs leaked carrier line: %q", l.Message)
		}
		for _, lb := range l.Labels {
			if lb.Name == LabelBuild && lb.Value != svcDep {
				t.Errorf("service build label = %q, want %q", lb.Value, svcDep)
			}
		}
	}
}

func TestBuildSourceLiveTailSSEWebSocketAndWatchdog(t *testing.T) {
	ws := "tea-d0000000000000000002"
	bdsID := ids.New(ids.BuildSource)
	dep1 := ids.New(ids.Deploy)
	dep2 := ids.New(ids.Deploy)
	bld1, _ := ids.Rekind(ids.Build, dep1)

	carrier := sampleBuildSourceCarrier(ws, bdsID, "live-builds", "https://github.com/bex-co/monorepo")
	base := time.Date(2026, 11, 23, 22, 30, 0, 0, time.UTC)

	pod1 := buildPodFor(carrier.Name, appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(1))+"-older", "builds", "buildkit", base)
	pod2 := buildPodFor(carrier.Name, appv1alpha1.BuildJobName(carrier.Name, appv1alpha1.BuildRevision(2))+"-newer", "builds", "buildkit", base.Add(time.Second))

	svc := newService(map[string][]string{
		pod1.Name: {base.Add(5*time.Second).Format(time.RFC3339Nano) + " live build 1 step"},
		pod2.Name: {base.Add(6*time.Second).Format(time.RFC3339Nano) + " live build 2 step"},
	}, carrier, pod1, pod2)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	svc.BuildCloseWait = 50 * time.Millisecond
	svc.DeployProgress = func(_ context.Context, resource string, _ time.Time) ([]DeployProgress, error) {
		if resource != bdsID {
			return nil, nil
		}
		return []DeployProgress{
			{ID: dep2, Generation: 2, Status: "live", Built: true, Commit: "2222222", CreatedAt: base.Add(2 * time.Second), StartedAt: base.Add(3 * time.Second), FinishedAt: base.Add(10 * time.Second)},
			{ID: dep1, Generation: 1, Status: "live", Built: true, Commit: "1111111", CreatedAt: base, StartedAt: base.Add(time.Second), FinishedAt: base.Add(8 * time.Second)},
		}, nil
	}

	// 1. SSE live tail with build=bld1 follows ONLY pod1 + bld1 build-only narration.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/v1/logs/subscribe?resource="+bdsID+"&build="+bld1, nil)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("SSE subscribe = %d, want 200", resp.StatusCode)
	}
	var sseLines []renderLog
	for _, raw := range readSSEData(t, resp.Body) {
		var l renderLog
		if err := json.Unmarshal([]byte(raw), &l); err == nil && l.Message != "" {
			sseLines = append(sseLines, l)
		}
	}
	var sseMsgs []string
	for _, l := range sseLines {
		sseMsgs = append(sseMsgs, l.Message)
		for _, lb := range l.Labels {
			if lb.Name == "resource" && lb.Value != bdsID {
				t.Errorf("SSE resource = %q, want %q", lb.Value, bdsID)
			}
			if lb.Name == LabelBuild && lb.Value != bld1 {
				t.Errorf("SSE build = %q, want %q (line %q)", lb.Value, bld1, l.Message)
			}
		}
		if strings.Contains(l.Message, "build 2") || strings.Contains(l.Message, "Your service is live") {
			t.Errorf("SSE leaked unexpected line: %q", l.Message)
		}
	}
	if !slices.Contains(sseMsgs, "live build 1 step") || !slices.Contains(sseMsgs, "==> Build succeeded 🎉") {
		t.Errorf("SSE lines = %v, want pod1 output and Build succeeded narration", sseMsgs)
	}

	// 2. WebSocket live tail with build=bld1 streams pod1 + bld1 narration (never pod2).
	prevOpened := tailOpenedAt
	tailOpenedAt = func() time.Time { return time.Time{} }
	t.Cleanup(func() { tailOpenedAt = prevOpened })

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/v1/logs/subscribe?resource=" + bdsID + "&build=" + bld1 + "&startTime=" + url.QueryEscape(base.Add(-time.Second).Format(time.RFC3339))
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wsMsgs []string
	for len(wsMsgs) < 4 {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var l renderLog
		if err := json.Unmarshal(msg, &l); err != nil {
			continue
		}
		wsMsgs = append(wsMsgs, l.Message)
		for _, lb := range l.Labels {
			if lb.Name == LabelBuild && lb.Value != bld1 {
				t.Errorf("WS build = %q, want %q", lb.Value, bld1)
			}
		}
	}
	if !slices.Contains(wsMsgs, "live build 1 step") || !slices.Contains(wsMsgs, "==> Build succeeded 🎉") {
		t.Errorf("WS lines = %v, want pod1 output and Build succeeded narration", wsMsgs)
	}

	// 3. Revalidation watchdog terminates an open Build Source tail when access is revoked.
	checker := &roleAuthzChecker{}
	checker.Grant("viewer", core.RelCanViewLogs, core.WorkspaceObject(ws))
	watchBase := &core.Base{
		Client:    svc.Client,
		Namespace: ws,
		Authz:     checker,
		Workspace: fixedWorkspaces{"viewer": {ws}},
	}
	watchSvc := &Service{
		Base:                 watchBase,
		PodLogsFollow:        svc.PodLogsFollow,
		BuildNamespace:       "builds",
		BuildPodWaitInterval: 5 * time.Millisecond,
		RevalidateInterval:   10 * time.Millisecond,
	}
	var started atomic.Bool
	watchSvc.DeployProgress = func(_ context.Context, _ string, _ time.Time) ([]DeployProgress, error) {
		started.Store(true)
		return []DeployProgress{{ID: dep1, Generation: 1, Status: deployStatusBuildInProgress, CreatedAt: base}}, nil
	}
	// Hold pod1 pending so the tail stays open waiting until watchdog revokes access.
	pendingPod := pod1.DeepCopy()
	pendingPod.Status.Phase = corev1.PodPending
	pendingPod.Status.ContainerStatuses = nil
	if err := watchSvc.Client.Update(context.Background(), pendingPod); err != nil {
		t.Fatal(err)
	}

	vCtx := core.WithWorkspace(core.WithIdentity(context.Background(), core.Identity{Subject: "viewer", Method: "session"}), ws)
	ctx, cancel := context.WithTimeout(vCtx, 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- watchSvc.FollowLogs(ctx, LogQuery{App: bdsID, Build: []string{bld1}}, func(LogEntry) error { return nil })
	}()
	// Wait until the tail is admitted and polling, then revoke can_view_logs.
	deadline := time.Now().Add(time.Second)
	for !started.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	checker.Revoke("viewer", core.RelCanViewLogs, core.WorkspaceObject(ws))
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Errorf("revoked Build Source tail ended with %v, want context.Canceled from watchdog", err)
	}
}

func readSSEData(t *testing.T, r interface{ Read([]byte) (int, error) }) []string {
	t.Helper()
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			break
		}
	}
	var out []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if payload, ok := strings.CutPrefix(line, "data: "); ok {
			out = append(out, payload)
		}
	}
	return out
}

type staticProjectResources struct {
	ws        string
	resources []savedsearch.Resource
}

func (p staticProjectResources) ProjectLogResources(_ context.Context, _ string) (string, []savedsearch.Resource, error) {
	return p.ws, p.resources, nil
}

func (p staticProjectResources) ProjectScopeResources(_ context.Context, _, _ string) (string, []savedsearch.Resource, error) {
	return p.ws, p.resources, nil
}

func TestBuildSourceLogsRBACDeletingAndProjectNonLeakage(t *testing.T) {
	wsA := "tea-d0000000000000000010"
	wsB := "tea-d0000000000000000011"
	bdsA := ids.New(ids.BuildSource)
	bdsB := ids.New(ids.BuildSource)
	bdsDeleting := ids.New(ids.BuildSource)
	srvA := ids.New(ids.Service)
	depA := ids.New(ids.Deploy)
	bldA, _ := ids.Rekind(ids.Build, depA)

	carrierA := sampleBuildSourceCarrier(wsA, bdsA, "a-builds", "https://github.com/bex-co/a")
	carrierB := sampleBuildSourceCarrier(wsB, bdsB, "b-builds", "https://github.com/bex-co/b")
	carrierDel := deleting(sampleBuildSourceCarrier(wsA, bdsDeleting, "del-builds", "https://github.com/bex-co/del"))
	webA := sampleWorkspaceService(wsA, srvA, "web")

	checker := &roleAuthzChecker{}
	checker.Grant("viewer-a", core.RelCanViewLogs, core.WorkspaceObject(wsA))
	checker.Grant("viewer-b", core.RelCanViewLogs, core.WorkspaceObject(wsB))
	// "no-logs-a" is a member of wsA without can_view_logs.

	cl := newService(nil, carrierA, carrierB, carrierDel, webA).Client
	base := &core.Base{
		Client:    cl,
		Namespace: wsA,
		Authz:     checker,
		Workspace: fixedWorkspaces{
			"viewer-a":  {wsA},
			"no-logs-a": {wsA},
			"viewer-b":  {wsB},
		},
	}
	svc := &Service{
		Base:           base,
		History:        func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil },
		LabelValues:    func(context.Context, string, string, LogQuery) ([]string, error) { return []string{LogTypeBuild}, nil },
		PodLogsFollow:  staticLogStream(nil),
		BuildNamespace: "builds",
		Projects: staticProjectResources{
			ws:        wsA,
			resources: []savedsearch.Resource{{ID: srvA, Name: "web", Type: appv1alpha1.TypeWebService}},
		},
		DeployProgress: func(_ context.Context, resource string, _ time.Time) ([]DeployProgress, error) {
			if resource == bdsA {
				return []DeployProgress{{ID: depA, Generation: 1, Status: "live", Built: true, CreatedAt: scopeBase, StartedAt: scopeBase.Add(time.Second), FinishedAt: scopeBase.Add(5 * time.Second)}}, nil
			}
			return nil, nil
		},
	}

	ctxAs := func(subject, ws string) context.Context {
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
		if ws != "" {
			ctx = core.WithWorkspace(ctx, ws)
		}
		return ctx
	}

	// Viewer in wsA succeeds on bdsA across all read verbs.
	viewerCtx := ctxAs("viewer-a", wsA)
	entries, err := svc.QueryLogs(viewerCtx, LogQuery{App: bdsA, Build: []string{bldA}, Since: scopeBase.Add(-time.Minute), End: scopeBase.Add(time.Minute)})
	if err != nil || len(entries) == 0 {
		t.Fatalf("viewer QueryLogs(bdsA) = %d, %v", len(entries), err)
	}
	if vals, err := svc.LogLabelValues(viewerCtx, LabelType, LogQuery{App: bdsA}); err != nil || !slices.Equal(vals, []string{LogTypeBuild}) {
		t.Fatalf("viewer LogLabelValues(bdsA) = %v, %v", vals, err)
	}
	if _, err := svc.LogRetention(viewerCtx, bdsA); err != nil {
		t.Fatalf("viewer LogRetention(bdsA) = %v", err)
	}

	// Unauthorized caller (no can_view_logs) gets ErrForbidden on every verb.
	unauthCtx := ctxAs("no-logs-a", wsA)
	if _, err := svc.QueryLogs(unauthCtx, LogQuery{App: bdsA}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unauth QueryLogs(bdsA) = %v, want ErrForbidden", err)
	}
	if _, err := svc.LogLabelValues(unauthCtx, LabelType, LogQuery{App: bdsA}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unauth LogLabelValues(bdsA) = %v, want ErrForbidden", err)
	}
	if _, err := svc.LogRetention(unauthCtx, bdsA); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unauth LogRetention(bdsA) = %v, want ErrForbidden", err)
	}
	if err := svc.FollowLogs(unauthCtx, LogQuery{App: bdsA}, func(LogEntry) error { return nil }); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unauth FollowLogs(bdsA) = %v, want ErrForbidden", err)
	}

	// Cross-workspace caller (viewer-b -> bdsA) gets ErrForbidden, identical to srv-….
	foreignCtx := ctxAs("viewer-b", "")
	if _, err := svc.QueryLogs(foreignCtx, LogQuery{App: bdsA}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("foreign QueryLogs(bdsA) = %v, want ErrForbidden", err)
	}
	if _, err := svc.LogLabelValues(foreignCtx, LabelType, LogQuery{App: bdsA}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("foreign LogLabelValues(bdsA) = %v, want ErrForbidden", err)
	}
	if err := svc.FollowLogs(foreignCtx, LogQuery{App: bdsA}, func(LogEntry) error { return nil }); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("foreign FollowLogs(bdsA) = %v, want ErrForbidden", err)
	}

	// Deleting Build Source answers ErrNotFound (404).
	if _, err := svc.QueryLogs(viewerCtx, LogQuery{App: bdsDeleting}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleting QueryLogs = %v, want ErrNotFound", err)
	}
	if _, err := svc.LogLabelValues(viewerCtx, LabelType, LogQuery{App: bdsDeleting}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleting LogLabelValues = %v, want ErrNotFound", err)
	}
	if _, err := svc.LogRetention(viewerCtx, bdsDeleting); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("deleting LogRetention = %v, want ErrNotFound", err)
	}

	// Project-scoped query excludes carriers and refuses explicit smuggling via project+resource=bds-….
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/logs?project=prj-1&resource="+bdsA, nil).WithContext(viewerCtx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET /v1/logs?project=prj-1&resource=%s = %d %s, want 400", bdsA, rec.Code, rec.Body)
	}
	// Unscoped project query returns only service resources, never carrier narration.
	projReq := httptest.NewRequest(http.MethodGet, "/v1/logs?project=prj-1&type=build&startTime="+url.QueryEscape(scopeBase.Add(-time.Minute).Format(time.RFC3339))+"&endTime="+url.QueryEscape(scopeBase.Add(time.Minute).Format(time.RFC3339)), nil).WithContext(viewerCtx)
	projRec := httptest.NewRecorder()
	mux.ServeHTTP(projRec, projReq)
	if projRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs?project=prj-1&type=build = %d %s", projRec.Code, projRec.Body)
	}
	if env := decodeLogList(t, projRec); len(env.Logs) != 0 {
		t.Errorf("project build query leaked carrier logs: %+v", env.Logs)
	}
}

var _ client.Object = (*appv1alpha1.App)(nil)
