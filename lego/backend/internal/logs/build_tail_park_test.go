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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/020: a build tail started while the deploy is still queued — before the
// build Job (and so its pod) exists — waits for the pod instead of ending in a
// second as if the build were over.
func TestBuildTailParksWhileTheDeployIsQueuedWithNoPod(t *testing.T) {
	const podName = "bld-web-gen-1-late"
	app := waitingApp("web", appv1alpha1.ReasonRegistryCredsPending, "Waiting for the registry")
	svc := newService(map[string][]string{podName: {"#1 real build line"}}, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	polls := 0
	svc.DeployProgress = func(ctx context.Context, _ string, _ time.Time) ([]DeployProgress, error) {
		polls++
		if polls == 4 {
			pod := buildPodFor("web", podName, "builds", "buildkit", time.Unix(1, 0))
			if err := svc.Client.Create(ctx, pod); err != nil {
				t.Errorf("create build pod: %v", err)
			}
		}
		return []DeployProgress{queuedWaitDeploy()}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var msgs []string
	if err := svc.FollowLogs(ctx, LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(e LogEntry) error {
		msgs = append(msgs, e.Message)
		return nil
	}); err != nil {
		t.Fatalf("FollowLogs: %v (lines so far %v)", err, msgs)
	}
	if !slices.Contains(msgs, "#1 real build line") {
		t.Fatalf("tail = %v, want it to wait for the pod and stream its build", msgs)
	}
}

// With the deploy past its build (or no deploy at all), no pod is still the
// honest terminal answer.
func TestBuildTailEndsWhenTheDeployIsNotBuilding(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		d := queuedWaitDeploy()
		d.Status = "live"
		d.FinishedAt = d.CreatedAt.Add(time.Minute)
		return []DeployProgress{d}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := svc.FollowLogs(ctx, LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(LogEntry) error { return nil })
	if !errors.Is(err, ErrBuildNotRunning) {
		t.Fatalf("FollowLogs = %v, want ErrBuildNotRunning", err)
	}
}

// The WebSocket transport (the pinned Render CLI's) ends a refused build tail
// with one Log-shaped `==>` line carrying the same text as the SSE error event,
// instead of closing silently.
func TestWebSocketBuildTailSendsTheRefusalAsALogLine(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.BuildNamespace = "builds"
	srv := revalidationTestServer(svc)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?resource=web&type=build"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v — the tail closed without saying why", err)
	}
	var line renderLog
	if err := json.Unmarshal(payload, &line); err != nil {
		t.Fatalf("frame %s is not a Log: %v", payload, err)
	}
	if line.Message != "==> "+ErrBuildNotRunning.Error() {
		t.Errorf("message = %q, want %q", line.Message, "==> "+ErrBuildNotRunning.Error())
	}
	if !slices.ContainsFunc(line.Labels, func(l renderLabel) bool { return l.Name == "type" && l.Value == LogTypeBuild }) {
		t.Errorf("labels = %+v, want type=build", line.Labels)
	}
	if _, _, err := conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Errorf("after the refusal: %v, want a normal close", err)
	}
}
