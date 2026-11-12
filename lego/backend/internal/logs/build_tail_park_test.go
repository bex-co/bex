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
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/020: a build tail started while the deploy is still queued — before the
// build Job (and so its pod) exists — waits for the pod instead of ending in a
// second as if the build were over.
func TestBuildTailParksWhileTheDeployIsQueuedWithNoPod(t *testing.T) {
	const podName = "bld-web-gen-1-late"
	app := waitingApp("web", appv1alpha1.ReasonRegistryCredsPending, "Waiting for the registry")
	svc := newService(map[string][]string{podName: {"2026-07-05T00:00:01Z #1 real build line"}}, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	// The row never leaves its build here; the close wait is not under test.
	svc.BuildCloseWait = 20 * time.Millisecond
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

// The WebSocket transport is the pinned Render CLI's, and Render's
// subscription follows what is generated from now on (w8/m159): a build tail
// opened with no build running stays open, prints nothing from earlier, and
// streams the next build once it starts. It used to answer "no running build"
// and close 4000, so `bex logs --tail --type build` opened before a deploy
// never showed its build.
func TestWebSocketBuildTailWaitsForTheNextBuild(t *testing.T) {
	earlier := buildPodFor("web", "bld-web-gen-1-old", "builds", "buildkit", time.Now().Add(-time.Hour))
	earlier.Status.Phase = corev1.PodSucceeded
	// Every pod's lines are known up front: the fake stream reads the map
	// from other goroutines. The next build's pod appears later.
	lines := map[string][]string{
		earlier.Name: {time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano) + " an earlier build"},
		"bld-web-gen-2-next": {
			time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano) + " next build one",
			time.Now().Add(time.Hour+time.Second).UTC().Format(time.RFC3339Nano) + " next build two",
		},
	}
	svc := newService(lines, sampleApp("web"), earlier)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	srv := revalidationTestServer(svc)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?resource=web&type=build"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, payload, err := conn.ReadMessage(); err == nil {
		t.Fatalf("a build tail with no build sent %s; want it to wait", payload)
	} else if ne, ok := err.(interface{ Timeout() bool }); !ok || !ne.Timeout() {
		t.Fatalf("a build tail with no build ended: %v; want it to stay open", err)
	}

	// The next deploy's build starts. A read deadline that fired leaves the
	// connection unusable, so read on a second subscription opened now.
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn2.Close()
	time.Sleep(50 * time.Millisecond)
	next := buildPodFor("web", "bld-web-gen-2-next", "builds", "buildkit", time.Now())
	if err := svc.Client.Create(context.Background(), next); err != nil {
		t.Fatalf("start the next build: %v", err)
	}
	var got []string
	_ = conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
	for len(got) < 2 {
		_, payload, err := conn2.ReadMessage()
		if err != nil {
			t.Fatalf("read after %v: %v", got, err)
		}
		var line renderLog
		if err := json.Unmarshal(payload, &line); err != nil {
			t.Fatalf("frame %s is not a Log: %v", payload, err)
		}
		got = append(got, line.Message)
	}
	if fmt.Sprint(got) != "[next build one next build two]" {
		t.Fatalf("lines = %v, want the next build's two lines and nothing earlier", got)
	}
	// The build pod still reads Running in the fake, so later passes read its
	// lines again: none is sent twice.
	_ = conn2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, payload, err := conn2.ReadMessage(); err == nil {
		t.Errorf("a line was sent twice: %s", payload)
	}
}

// A tail that cannot run at all still ends with one Log-shaped `==>` line
// carrying the reason, then a terminal close: Render CLI v2.29+ reconnects
// after a normal close, so a 1000 would retry forever (w8/m67).
func TestWebSocketTailSendsTheRefusalAsALogLine(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.PodLogsFollow = nil
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
	if line.Message != "==> "+core.ErrLogsUnavailable.Error() {
		t.Errorf("message = %q, want %q", line.Message, "==> "+core.ErrLogsUnavailable.Error())
	}
	_, _, err = conn.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != CloseTailRefused {
		t.Errorf("after the refusal: %v, want close %d", err, CloseTailRefused)
	}
}

// A close frame's reason holds 123 bytes; a longer refusal is cut on a rune
// boundary instead of failing the close.
func TestCloseReasonFitsTheFrame(t *testing.T) {
	long := errors.New(strings.Repeat("é", 100))
	got := closeReason(long)
	if len(got) > 123 || !utf8.ValidString(got) {
		t.Fatalf("closeReason = %d bytes, valid=%v", len(got), utf8.ValidString(got))
	}
	if short := closeReason(ErrBuildNotRunning); short != ErrBuildNotRunning.Error() {
		t.Errorf("closeReason(short) = %q", short)
	}
}
