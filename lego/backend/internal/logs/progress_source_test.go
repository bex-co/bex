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
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/types/renderlog"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/m608: historical deploy progress lines and their deterministic log IDs
// stay immutable across --repo, --branch, and --image switches, while
// pre-migration rows (empty Repo and Image) fall back to the App's current source.
func TestProgressLinesPreservePerDeploySourceAcrossSwitches(t *testing.T) {
	const (
		repoA = "https://github.com/bex-co/bex"
		repoB = "https://github.com/expressjs/serve-static"
		img   = "docker.io/library/nginx:1.27-alpine"
	)
	base := time.Date(2026, 11, 24, 1, 0, 0, 0, time.UTC)
	at := func(sec int) time.Time { return base.Add(time.Duration(sec) * time.Second) }
	deployingLine := renderlog.PlatformStep.Line("Deploying...")

	messagesAndIDs := func(entries []LogEntry) ([]string, []string) {
		msgs := make([]string, len(entries))
		ids := make([]string, len(entries))
		for i, e := range entries {
			msgs[i], ids[i] = e.Message, logID(e)
		}
		return msgs, ids
	}

	t.Run("repo A to repo B switch keeps historical Building from and IDs", func(t *testing.T) {
		row := DeployProgress{
			ID: "dep-repo-a", Status: "deactivated", Built: true,
			Repo: repoA, Branch: "main", Commit: "abc1234def5678",
			CreatedAt: at(0), StartedAt: at(2), RolloutStartedAt: at(30), FinishedAt: at(45),
		}
		beforeMsgs, beforeIDs := messagesAndIDs(progressLines(row, progressContext{repo: repoA, branch: "main", serviceType: appv1alpha1.TypeWebService}))
		afterMsgs, afterIDs := messagesAndIDs(progressLines(row, progressContext{repo: repoB, branch: "master", serviceType: appv1alpha1.TypeWebService}))
		if !slices.Equal(beforeMsgs, afterMsgs) || !slices.Equal(beforeIDs, afterIDs) {
			t.Fatalf("repo switch altered historical lines/IDs:\nbefore=%q %q\nafter =%q %q", beforeMsgs, beforeIDs, afterMsgs, afterIDs)
		}
		if !slices.Contains(afterMsgs, "==> Build queued") ||
			!slices.Contains(afterMsgs, "==> Building from "+repoA+"@abc1234") ||
			!slices.Contains(afterMsgs, deployingLine) {
			t.Fatalf("missing expected repo A build story: %q", afterMsgs)
		}
	})

	t.Run("branch A to branch B switch keeps unresolved-commit branch and IDs", func(t *testing.T) {
		row := DeployProgress{
			ID: "dep-branch-a", Status: "deactivated", Built: true,
			Repo: repoA, Branch: "feature-a", Commit: "",
			CreatedAt: at(0), StartedAt: at(2), FinishedAt: at(45),
		}
		beforeMsgs, beforeIDs := messagesAndIDs(progressLines(row, progressContext{repo: repoA, branch: "feature-a", serviceType: appv1alpha1.TypeWebService}))
		afterMsgs, afterIDs := messagesAndIDs(progressLines(row, progressContext{repo: repoA, branch: "feature-b", serviceType: appv1alpha1.TypeWebService}))
		if !slices.Equal(beforeMsgs, afterMsgs) || !slices.Equal(beforeIDs, afterIDs) {
			t.Fatalf("branch switch altered historical lines/IDs:\nbefore=%q %q\nafter =%q %q", beforeMsgs, beforeIDs, afterMsgs, afterIDs)
		}
		if !slices.Contains(afterMsgs, "==> Building from "+repoA+"@feature-a") {
			t.Fatalf("expected branch feature-a, got %q", afterMsgs)
		}
	})

	t.Run("recorded repo with empty branch never borrows current App branch", func(t *testing.T) {
		row := DeployProgress{
			ID: "dep-no-branch", Status: "deactivated", Built: true,
			Repo: repoA, Branch: "", Commit: "",
			CreatedAt: at(0), StartedAt: at(2), FinishedAt: at(45),
		}
		msgs, _ := messagesAndIDs(progressLines(row, progressContext{repo: repoB, branch: "other-branch", serviceType: appv1alpha1.TypeWebService}))
		if !slices.Contains(msgs, "==> Building from "+repoA) {
			t.Fatalf("expected bare repo without borrowed branch, got %q", msgs)
		}
	})

	t.Run("Git to prebuilt image switch keeps Build queued, Building from, Deploying... and IDs", func(t *testing.T) {
		row := DeployProgress{
			ID: "dep-git-then-img", Status: "deactivated", Built: true,
			Repo: repoA, Branch: "main", Commit: "abc1234def5678",
			CreatedAt: at(0), StartedAt: at(2), RolloutStartedAt: at(30), FinishedAt: at(45),
		}
		beforeMsgs, beforeIDs := messagesAndIDs(progressLines(row, progressContext{repo: repoA, branch: "main", serviceType: appv1alpha1.TypeWebService}))
		afterMsgs, afterIDs := messagesAndIDs(progressLines(row, progressContext{repo: "", branch: "", serviceType: appv1alpha1.TypeWebService}))
		if !slices.Equal(beforeMsgs, afterMsgs) || !slices.Equal(beforeIDs, afterIDs) {
			t.Fatalf("Git->image switch altered historical lines/IDs:\nbefore=%q %q\nafter =%q %q", beforeMsgs, beforeIDs, afterMsgs, afterIDs)
		}
		for _, forbidden := range []string{"==> Deploy queued", "==> Rolling out the current release"} {
			if slices.Contains(afterMsgs, forbidden) {
				t.Fatalf("Git deploy rewritten to %q: %q", forbidden, afterMsgs)
			}
		}
		if !slices.Contains(afterMsgs, "==> Build queued") ||
			!slices.Contains(afterMsgs, "==> Building from "+repoA+"@abc1234") ||
			!slices.Contains(afterMsgs, deployingLine) {
			t.Fatalf("expected full Git build story after image switch, got %q", afterMsgs)
		}
	})

	t.Run("prebuilt image to Git switch keeps Deploy queued and Deploying image", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			status   string
			finished time.Time
		}{
			{"finished image deploy", "deactivated", at(45)},
			{"in-flight image deploy", "update_in_progress", time.Time{}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				row := DeployProgress{
					ID: "dep-img-then-git", Status: tc.status, Image: img,
					CreatedAt: at(0), StartedAt: at(2), FinishedAt: tc.finished,
				}
				beforeMsgs, beforeIDs := messagesAndIDs(progressLines(row, progressContext{serviceType: appv1alpha1.TypeWebService}))
				afterMsgs, afterIDs := messagesAndIDs(progressLines(row, progressContext{repo: repoA, branch: "main", serviceType: appv1alpha1.TypeWebService}))
				if !slices.Equal(beforeMsgs, afterMsgs) || !slices.Equal(beforeIDs, afterIDs) {
					t.Fatalf("image->Git switch altered historical lines/IDs:\nbefore=%q %q\nafter =%q %q", beforeMsgs, beforeIDs, afterMsgs, afterIDs)
				}
				if !slices.Contains(afterMsgs, "==> Deploy queued") || !slices.Contains(afterMsgs, "==> Deploying image "+img) {
					t.Fatalf("expected image deploy lines, got %q", afterMsgs)
				}
			})
		}
	})

	t.Run("pre-migration fallback uses current App source when Repo and Image are empty", func(t *testing.T) {
		withCommit := DeployProgress{
			ID: "dep-legacy-sha", Status: "live", Built: true, Commit: "def567890123",
			CreatedAt: at(0), StartedAt: at(2), RolloutStartedAt: at(20), FinishedAt: at(30),
		}
		msgs, _ := messagesAndIDs(progressLines(withCommit, progressContext{repo: repoA, branch: "main", serviceType: appv1alpha1.TypeWebService}))
		if !slices.Contains(msgs, "==> Build queued") ||
			!slices.Contains(msgs, "==> Building from "+repoA+"@def5678") ||
			!slices.Contains(msgs, deployingLine) {
			t.Fatalf("legacy commit fallback = %q", msgs)
		}

		unresolved := DeployProgress{
			ID: "dep-legacy-branch", Status: "live", Built: true, Commit: "",
			CreatedAt: at(0), StartedAt: at(2), FinishedAt: at(30),
		}
		msgs, _ = messagesAndIDs(progressLines(unresolved, progressContext{repo: repoA, branch: "develop", serviceType: appv1alpha1.TypeWebService}))
		if !slices.Contains(msgs, "==> Building from "+repoA+"@develop") {
			t.Fatalf("legacy branch fallback = %q", msgs)
		}
	})
}

// w32/m608: REST /v1/logs, GraphQL logs, MCP list_logs, SSE /v1/logs/subscribe,
// and WebSocket /v1/logs/subscribe all narrate a historical Git deploy from its
// own recorded Repo/Branch with identical log IDs after the App CR has switched
// to a prebuilt image.
func TestPerDeploySourceReadsTheSameAcrossSurfaces(t *testing.T) {
	const (
		repoA = "https://github.com/bex-co/bex"
		img   = "docker.io/library/nginx:1.27-alpine"
	)
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }
	created, started, rolloutAt, finished := soon(30*time.Minute), soon(31*time.Minute), soon(65*time.Minute), soon(72*time.Minute)
	wantLines := []string{
		"==> Build queued",
		"==> Building from " + repoA + "@abc1234",
		renderlog.PlatformStep.Line("Deploying..."),
	}

	row := DeployProgress{
		ID: "dep-src-immut", Generation: 4, Status: "live", Built: true,
		Repo: repoA, Branch: "main", Commit: "abc1234def5678",
		CreatedAt: created, StartedAt: started, RolloutStartedAt: rolloutAt, FinishedAt: finished,
	}

	// First read via REST while the App CR still points at repoA@main to
	// capture the pre-switch wire IDs (including projected instance labels).
	app := sampleApp("web")
	app.Spec.Repo = repoA
	app.Spec.Branch = "main"

	svc := newService(map[string][]string{
		"bld-web-gen-4-a": {soon(time.Hour).Format(time.RFC3339Nano) + " ==> Build successful 🎉"},
	}, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}

	start, end := created.Add(-time.Second).Format(time.RFC3339), finished.Add(time.Second).Format(time.RFC3339)
	preRec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end)
	var preEnv renderLogList
	if err := json.Unmarshal(preRec.Body.Bytes(), &preEnv); err != nil || preRec.Code != http.StatusOK {
		t.Fatalf("pre-switch REST = %d %s (%v)", preRec.Code, preRec.Body, err)
	}
	wantIDs := make([]string, len(wantLines))
	for i, want := range wantLines {
		idx := slices.IndexFunc(preEnv.Logs, func(l renderLog) bool { return l.Message == want })
		if idx < 0 {
			t.Fatalf("pre-switch REST missing %q in %+v", want, preEnv.Logs)
		}
		wantIDs[i] = preEnv.Logs[idx].ID
	}

	// Now switch the App CR away from Git to a prebuilt image.
	app.Spec.Repo = ""
	app.Spec.Branch = ""
	app.Spec.Image = img
	if err := svc.Client.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	assertSurface := func(surface string, messages, ids []string) {
		t.Helper()
		for i, want := range wantLines {
			idx := slices.Index(messages, want)
			if idx < 0 {
				t.Fatalf("%s missing %q in %q", surface, want, messages)
			}
			if ids[idx] != wantIDs[i] {
				t.Fatalf("%s id for %q = %q, want pre-switch %q", surface, want, ids[idx], wantIDs[i])
			}
		}
	}

	// REST GET /v1/logs
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end)
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("REST = %d %s (%v)", rec.Code, rec.Body, err)
	}
	var messages, ids []string
	for _, l := range env.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	assertSurface("REST", messages, ids)

	// GraphQL logs
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", type:"build", startTime:"`+start+`", endTime:"`+end+`") { logs { id message } } }`)
	messages, ids = nil, nil
	for _, r := range data["logs"].(map[string]any)["logs"].([]any) {
		m := r.(map[string]any)
		messages, ids = append(messages, m["message"].(string)), append(ids, m["id"].(string))
	}
	assertSurface("GraphQL", messages, ids)

	// MCP list_logs
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpSrv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "type": []string{LogTypeBuild}, "startTime": start, "endTime": end,
	}})
	if err != nil || res.IsError {
		t.Fatalf("list_logs: %v %+v", err, res)
	}
	var mcpGot renderLogList
	rendertest.DecodeMCPText(t, res, &mcpGot)
	messages, ids = nil, nil
	for _, l := range mcpGot.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	assertSurface("MCP", messages, ids)

	// SSE GET /v1/logs/subscribe
	if err := svc.Client.Create(ctx, buildPodFor("web", "bld-web-gen-4-a", "builds", "buildkit", opened)); err != nil {
		t.Fatal(err)
	}
	httpSrv := revalidationTestServer(svc)
	defer httpSrv.Close()
	sseReq, err := http.NewRequestWithContext(ctx, http.MethodGet, httpSrv.URL+"/v1/logs/subscribe?resource=web&type=build", nil)
	if err != nil {
		t.Fatal(err)
	}
	sseReq.Header.Set("Accept", "text/event-stream")
	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("SSE get: %v", err)
	}
	defer sseResp.Body.Close()
	if sseResp.StatusCode != http.StatusOK || sseResp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("SSE = %d %q", sseResp.StatusCode, sseResp.Header.Get("Content-Type"))
	}
	messages, ids = nil, nil
	scanner := bufio.NewScanner(sseResp.Body)
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var item renderLog
		if err := json.Unmarshal([]byte(line), &item); err != nil || item.ID == "" {
			continue
		}
		messages, ids = append(messages, item.Message), append(ids, item.ID)
	}
	assertSurface("SSE", messages, ids)

	// WebSocket GET /v1/logs/subscribe (CLI --tail)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpSrv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build", nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	messages, ids = nil, nil
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for !slices.Contains(messages, rendersLive) {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("WS waiting after %q: %v", messages, err)
		}
		var item renderLog
		if err := json.Unmarshal(payload, &item); err != nil {
			t.Fatalf("WS frame %s: %v", payload, err)
		}
		messages, ids = append(messages, item.Message), append(ids, item.ID)
	}
	assertSurface("WebSocket", messages, ids)
}
