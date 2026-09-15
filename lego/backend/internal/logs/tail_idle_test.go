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
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w1/m146: a live app tail stays open for as long as it is authorized and the
// App exists. It used to end the moment every pod present at subscribe time had
// exited, so a cron job between runs dropped its stream ~300 ms after replaying
// the last run and the dashboard reconnected every ~3 s behind a banner.

var (
	containerRunning    = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	containerTerminated = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}
	containerWaiting    = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}
)

func appPodIn(app, name string, phase corev1.PodPhase, state corev1.ContainerState, restarts int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelApp: app}},
		Status: corev1.PodStatus{
			Phase: phase,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: core.AppContainer, State: state, RestartCount: restarts,
			}},
		},
	}
}

// scriptedStream serves each pod's canned logs in order, one per follow opened,
// so a restarted container's second follow reads its second life. It also counts
// the follows opened per pod.
type scriptedStream struct {
	mu     sync.Mutex
	logs   map[string][]string
	opened map[string]int
}

func newScriptedStream(logs map[string][]string) *scriptedStream {
	return &scriptedStream{logs: logs, opened: map[string]int{}}
}

func (s *scriptedStream) follow(_ context.Context, _, pod, _ string, _ time.Time) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.opened[pod]
	s.opened[pod]++
	if n >= len(s.logs[pod]) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	return io.NopCloser(strings.NewReader(s.logs[pod][n])), nil
}

func (s *scriptedStream) openedFor(pod string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opened[pod]
}

func tailService(stream PodLogStream, objs ...client.Object) *Service {
	return &Service{
		Base:                  &core.Base{Client: fakeClientWith(objs...), Namespace: "default"},
		PodLogsFollow:         stream,
		TailRelistInterval:    10 * time.Millisecond,
		TailHeartbeatInterval: 10 * time.Millisecond,
	}
}

type tailRun struct {
	messages   chan string
	heartbeats chan struct{}
	done       chan error
}

// startTail runs followLogs until the test ends, delivering every emitted
// message and heartbeat.
func startTail(t *testing.T, svc *Service, q LogQuery) *tailRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &tailRun{
		messages:   make(chan string, 64),
		heartbeats: make(chan struct{}, 64),
		done:       make(chan error, 1),
	}
	go func() {
		run.done <- svc.followLogs(ctx, q, func(e LogEntry) error {
			run.messages <- e.Message
			return nil
		}, func() error {
			select {
			case run.heartbeats <- struct{}{}:
			default:
			}
			return nil
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-run.done:
		case <-time.After(5 * time.Second):
			t.Error("tail did not end after its context was cancelled")
		}
	})
	return run
}

func (r *tailRun) expectMessage(t *testing.T, want string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-r.messages:
			if got == want {
				return
			}
		case err := <-r.done:
			t.Fatalf("tail ended with %v while waiting for %q", err, want)
		case <-deadline:
			t.Fatalf("timed out waiting for %q", want)
		}
	}
}

// expectOpen fails if the tail ends within d — several re-list ticks at the
// test cadence.
func (r *tailRun) expectOpen(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case err := <-r.done:
		t.Fatalf("tail ended with %v; it must stay open while the App exists", err)
	case <-time.After(d):
	}
}

func setPodStatus(t *testing.T, svc *Service, name string, phase corev1.PodPhase, state corev1.ContainerState, restarts int32) {
	t.Helper()
	var pod corev1.Pod
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &pod); err != nil {
		t.Fatalf("get pod %s: %v", name, err)
	}
	pod.Status = appPodIn("", name, phase, state, restarts).Status
	if err := svc.Client.Status().Update(context.Background(), &pod); err != nil {
		t.Fatalf("update pod %s status: %v", name, err)
	}
}

func createPod(t *testing.T, svc *Service, pod *corev1.Pod) {
	t.Helper()
	status := pod.Status
	if err := svc.Client.Create(context.Background(), pod); err != nil {
		t.Fatalf("create pod %s: %v", pod.Name, err)
	}
	pod.Status = status
	if err := svc.Client.Status().Update(context.Background(), pod); err != nil {
		t.Fatalf("set pod %s status: %v", pod.Name, err)
	}
}

// The README's reproduction: a cron job whose one run has finished. The tail
// replays that run once, stays open, and streams the next run as it happens.
func TestAppTailOutlivesAFinishedRunAndStreamsTheNextOne(t *testing.T) {
	stream := newScriptedStream(map[string][]string{
		"nightly-run-1": {"2026-09-14T08:56:55Z qa-cron-ran"},
		"nightly-run-2": {"2026-09-14T09:56:55Z second run"},
	})
	svc := tailService(stream.follow, sampleApp("nightly"),
		appPodIn("nightly", "nightly-run-1", corev1.PodSucceeded, containerTerminated, 0))

	run := startTail(t, svc, LogQuery{App: "nightly"})
	run.expectMessage(t, "qa-cron-ran")
	run.expectOpen(t, 150*time.Millisecond)
	if n := stream.openedFor("nightly-run-1"); n != 1 {
		t.Fatalf("a finished run was followed %d times; it must replay once per subscription", n)
	}

	createPod(t, svc, appPodIn("nightly", "nightly-run-2", corev1.PodRunning, containerRunning, 0))
	run.expectMessage(t, "second run")
}

// A service with no pod at all — suspended, hibernated, or before its first
// pod — keeps an open, heartbeating tail and streams the first pod that starts.
func TestAppTailWithNoPodsStaysOpenHeartbeatsAndStreamsTheFirstPod(t *testing.T) {
	stream := newScriptedStream(map[string][]string{"web-1": {"2026-09-14T10:00:00Z woke up"}})
	svc := tailService(stream.follow, sampleApp("web"))

	run := startTail(t, svc, LogQuery{App: "web"})
	for i := 0; i < 3; i++ {
		select {
		case <-run.heartbeats:
		case err := <-run.done:
			t.Fatalf("a zero-pod tail ended with %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("a zero-pod tail sent no heartbeat")
		}
	}

	createPod(t, svc, appPodIn("web", "web-1", corev1.PodRunning, containerRunning, 0))
	run.expectMessage(t, "woke up")
}

// A pod whose container is still waiting has no log yet: it is attached once
// its container starts, not dropped for the rest of the subscription.
func TestAppTailAttachesAPendingPodOnceItsContainerStarts(t *testing.T) {
	stream := newScriptedStream(map[string][]string{"nightly-run-2": {"2026-09-14T11:00:00Z pulled and ran"}})
	svc := tailService(stream.follow, sampleApp("nightly"),
		appPodIn("nightly", "nightly-run-2", corev1.PodPending, containerWaiting, 0))

	run := startTail(t, svc, LogQuery{App: "nightly"})
	run.expectOpen(t, 100*time.Millisecond)
	if n := stream.openedFor("nightly-run-2"); n != 0 {
		t.Fatalf("a waiting container was followed %d times before it started", n)
	}

	setPodStatus(t, svc, "nightly-run-2", corev1.PodRunning, containerRunning, 0)
	run.expectMessage(t, "pulled and ran")
}

// A container that restarts keeps its pod name. The follow of its first life
// ends; the tail attaches to the new life, which is what the client reconnect
// used to provide when the whole stream ended.
func TestAppTailReattachesARestartedContainer(t *testing.T) {
	stream := newScriptedStream(map[string][]string{
		"web-1": {"2026-09-14T12:00:00Z first life", "2026-09-14T12:00:05Z second life"},
	})
	svc := tailService(stream.follow, sampleApp("web"),
		appPodIn("web", "web-1", corev1.PodRunning, containerRunning, 0))

	run := startTail(t, svc, LogQuery{App: "web"})
	run.expectMessage(t, "first life")
	run.expectOpen(t, 100*time.Millisecond)
	if n := stream.openedFor("web-1"); n != 1 {
		t.Fatalf("an unrestarted container was followed %d times", n)
	}

	setPodStatus(t, svc, "web-1", corev1.PodRunning, containerRunning, 1)
	run.expectMessage(t, "second life")
}

// A pod that starts after subscribe is still subject to the caller's instance
// filter, resolved against the pods that exist when it appears.
func TestAppTailAppliesTheInstanceFilterToLaterPods(t *testing.T) {
	stream := newScriptedStream(map[string][]string{
		"web-2": {"2026-09-14T13:00:00Z not this one"},
		"web-3": {"2026-09-14T13:00:01Z this one"},
	})
	svc := tailService(stream.follow, sampleApp("web"))

	run := startTail(t, svc, LogQuery{App: "web", Instance: []string{"web-3"}})
	run.expectOpen(t, 50*time.Millisecond)
	createPod(t, svc, appPodIn("web", "web-2", corev1.PodRunning, containerRunning, 0))
	createPod(t, svc, appPodIn("web", "web-3", corev1.PodRunning, containerRunning, 0))
	run.expectMessage(t, "this one")
	if n := stream.openedFor("web-2"); n != 0 {
		t.Fatalf("a pod outside the instance filter was followed %d times", n)
	}
}

// Keeping idle tails open must not turn "this query type has no live producer"
// into a parked stream: that is still refused immediately (codex #3), unlike
// "this App has no pod right now".
func TestAppTailStillRefusesAQueryWithNoLiveProducer(t *testing.T) {
	svc := tailService(newScriptedStream(nil).follow, sampleApp("web"))
	done := make(chan error, 1)
	go func() {
		done <- svc.FollowLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypePreDeploy}}, func(LogEntry) error { return nil })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("no-producer query => ErrBadRequest, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a query with no live producer parked instead of being refused")
	}
}

// Both HTTP transports write a keepalive their decoders ignore, so an idle tail
// survives the proxies in front of bex-api.
func TestSubscribeWritesAKeepaliveOnAnIdleTail(t *testing.T) {
	for _, tc := range []struct {
		accept    string
		keepalive string
	}{
		{"text/event-stream", ": keepalive"},
		{"application/x-ndjson", ""},
	} {
		t.Run(tc.accept, func(t *testing.T) {
			svc := tailService(newScriptedStream(nil).follow, sampleApp("web"))
			srv := revalidationTestServer(svc)
			defer srv.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/logs/subscribe?resource=web", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept", tc.accept)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("subscribe: %v", err)
			}
			defer resp.Body.Close()

			got := make(chan string, 1)
			go func() {
				sc := bufio.NewScanner(resp.Body)
				if sc.Scan() {
					got <- sc.Text()
				}
				close(got)
			}()
			select {
			case line, ok := <-got:
				if !ok {
					t.Fatal("an idle tail ended instead of sending a keepalive")
				}
				if line != tc.keepalive {
					t.Fatalf("first line on an idle tail = %q, want the keepalive %q", line, tc.keepalive)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no keepalive on an idle tail")
			}
		})
	}
}

// The WebSocket transport (the Render CLI's) pings an idle tail.
func TestSubscribeWebSocketPingsAnIdleTail(t *testing.T) {
	svc := tailService(newScriptedStream(nil).follow, sampleApp("web"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?resource=web"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()

	pinged := make(chan struct{}, 1)
	conn.SetPingHandler(func(string) error {
		select {
		case pinged <- struct{}{}:
		default:
		}
		return nil
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	select {
	case <-pinged:
	case <-time.After(5 * time.Second):
		t.Fatal("an idle WebSocket tail sent no ping")
	}
}

// An idle tail still ends when its authorization is revoked or its App is
// deleted, and releases its subscription slot.
func TestIdleTailStillEndsOnRevocationAndDeletion(t *testing.T) {
	t.Run("revocation", func(t *testing.T) {
		checker := &freshGateChecker{cached: true, fresh: true}
		svc := revalidationService(checker, newScriptedStream(nil).follow, sampleApp("nightly"))
		srv := revalidationTestServer(svc)
		defer srv.Close()

		resp := subscribeSSE(t, srv, "nightly")
		defer resp.Body.Close()
		done := streamDone(resp.Body)
		waitForFreshCalls(t, checker, 1)
		checker.setFresh(false, nil)
		waitForStreamEnd(t, done)
		waitForSlotRelease(t, svc)
	})
	t.Run("deletion", func(t *testing.T) {
		checker := &freshGateChecker{cached: true, fresh: true}
		app := sampleApp("nightly")
		svc := revalidationService(checker, newScriptedStream(nil).follow, app)
		srv := revalidationTestServer(svc)
		defer srv.Close()

		resp := subscribeSSE(t, srv, "nightly")
		defer resp.Body.Close()
		done := streamDone(resp.Body)
		waitForFreshCalls(t, checker, 1)
		if err := svc.Client.Delete(context.Background(), app); err != nil {
			t.Fatalf("delete App: %v", err)
		}
		waitForStreamEnd(t, done)
		waitForSlotRelease(t, svc)
	})
}
