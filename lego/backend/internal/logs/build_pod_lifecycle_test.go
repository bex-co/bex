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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m159 t008: a build's steps run as init containers, and Kubernetes reports
// the pod Pending until its one regular container (the push) starts. The tail
// used to attach only to a Running pod, so it reached the build at the push and
// never showed the clone, the build command or a failing command's error. Here
// the pod goes through its real lifecycle: Pending with the clone running,
// Pending with BuildKit running, then either Running with the push and
// Succeeded, or Failed with no regular container ever running; the build tail
// and the type-less tail (which carries build lines on this transport) alike.
func TestBuildTailFollowsABuildPodThroughItsLifecycle(t *testing.T) {
	for _, tc := range []struct {
		failing bool
		query   string
	}{{false, "&type=build"}, {true, "&type=build"}, {false, ""}} {
		failing := tc.failing
		t.Run(fmt.Sprintf("failing=%v,query=%q", failing, tc.query), func(t *testing.T) {
			soon := func(d time.Duration) string { return time.Now().Add(time.Hour + d).UTC().Format(time.RFC3339Nano) }
			lines := map[string]string{
				"clone":    soon(0) + " Cloning into 'src'...",
				"buildkit": soon(time.Second) + " yarn install v1.22.22",
				"push":     soon(2*time.Second) + " Copying blob sha256:abc",
			}
			if failing {
				lines["buildkit"] = soon(time.Second) + " error: build command 'yarn install && exit 3' exited with code 3"
			}
			svc := newService(nil, sampleApp("web"))
			svc.BuildNamespace = "builds"
			svc.BuildPodWaitInterval = 10 * time.Millisecond
			var mu sync.Mutex
			svc.PodLogsFollow = func(_ context.Context, _, _, container string, _ time.Time) (io.ReadCloser, error) {
				mu.Lock()
				defer mu.Unlock()
				return io.NopCloser(strings.NewReader(lines[container])), nil
			}
			srv := revalidationTestServer(svc)
			defer srv.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web"+tc.query, nil)
			if err != nil {
				t.Fatalf("WS dial: %v", err)
			}
			defer conn.Close()
			time.Sleep(50 * time.Millisecond)

			running := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
			waiting := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}
			ended := func(code int32) corev1.ContainerState {
				return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: code, FinishedAt: metav1.Now()}}
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "bld-web-gen-2-a", Namespace: "builds",
					Labels: map[string]string{appv1alpha1.LabelBuild: "web"}, CreationTimestamp: metav1.Now()},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{Name: "clone"}, {Name: "buildkit"}},
					Containers:     []corev1.Container{{Name: "push"}},
				},
				Status: corev1.PodStatus{
					Phase:                 corev1.PodPending,
					InitContainerStatuses: []corev1.ContainerStatus{{Name: "clone", State: running}, {Name: "buildkit", State: waiting}},
					ContainerStatuses:     []corev1.ContainerStatus{{Name: "push", State: waiting}},
				},
			}
			if err := svc.Client.Create(context.Background(), pod); err != nil {
				t.Fatal(err)
			}
			read := func() string {
				t.Helper()
				_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				_, payload, err := conn.ReadMessage()
				if err != nil {
					t.Fatalf("read: %v", err)
				}
				var line renderLog
				if err := json.Unmarshal(payload, &line); err != nil {
					t.Fatalf("frame %s is not a Log: %v", payload, err)
				}
				return line.Message
			}
			advance := func(mutate func(*corev1.Pod)) {
				t.Helper()
				mutate(pod)
				status := pod.Status
				if err := svc.Client.Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				pod.Status = status
				if err := svc.Client.Status().Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
			}
			var got []string
			got = append(got, read())
			advance(func(p *corev1.Pod) {
				p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "clone", State: ended(0)}, {Name: "buildkit", State: running}}
			})
			got = append(got, read())
			want := []string{"Cloning into 'src'...", "yarn install v1.22.22", "Copying blob sha256:abc"}
			if failing {
				advance(func(p *corev1.Pod) {
					p.Status.Phase = corev1.PodFailed
					p.Status.InitContainerStatuses[1].State = ended(3)
				})
				want = []string{"Cloning into 'src'...", "error: build command 'yarn install && exit 3' exited with code 3"}
			} else {
				advance(func(p *corev1.Pod) {
					p.Status.Phase = corev1.PodRunning
					p.Status.InitContainerStatuses[1].State = ended(0)
					p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "push", State: running}}
				})
				got = append(got, read())
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("lines = %q, want every phase's output in order: %q", got, want)
			}
		})
	}
}

// A tail opened while BuildKit already runs reads from that moment: the clone,
// which ended before the tail opened, is not attached. A tail with no lower
// bound reads every phase.
func TestBuildTailSkipsOnlyPhasesThatEndedBeforeItOpened(t *testing.T) {
	opened := time.Date(2026, 11, 12, 10, 58, 0, 0, time.UTC)
	ended := func(at time.Time) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: metav1.NewTime(at)}}
	}
	pod := corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{
			{Name: "clone", State: ended(opened.Add(-time.Minute))},
			{Name: "prepare", State: ended(opened.Add(time.Second))},
			{Name: "buildkit", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		},
	}}
	if got := terminatedBefore(pod, opened); len(got) != 1 || !got["clone"] {
		t.Fatalf("terminatedBefore = %v, want only the clone, which ended before the tail opened", got)
	}
	if got := terminatedBefore(pod, time.Time{}); len(got) != 0 {
		t.Fatalf("terminatedBefore with no lower bound = %v, want none", got)
	}
}
