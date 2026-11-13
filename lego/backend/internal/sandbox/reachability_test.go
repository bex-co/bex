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

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
)

// podExecutor stands in for the gateway's KubeExecutor over one sandbox Pod:
// a Pod the gateway's target check classifies fails the exec with that class,
// as a real exec into it does; a Ready one runs and prints stdout. A nil Pod
// no longer exists.
type podExecutor struct {
	pod    *corev1.Pod
	stdout string
	ran    int
}

func (e *podExecutor) Execute(_ context.Context, target apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	e.ran++
	if e.pod == nil {
		return 126, &sshgateway.TargetTerminatedError{Detail: "pod no longer exists"}
	}
	if err := sshgateway.ClassifyTarget(e.pod, target.Container); err != nil {
		return 126, err
	}
	_, _ = io.WriteString(stdout, e.stdout)
	return 0, nil
}

// sandboxPod is the session sandbox's Pod in phase with its Ready condition
// and the sandbox container's state.
func sandboxPod(phase corev1.PodPhase, ready corev1.ConditionStatus, state corev1.ContainerState) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name, pod.Namespace = "os-agent-0", "tea-a-sandbox"
	pod.Status.Phase = phase
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: ready}}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "sandbox", State: state}}
	return pod
}

var runningContainer = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}

// sessionReadFixture is an agent-session lifecycle over a fake OpenSandbox
// server reporting state for os-agent and the real sandbox-exec gateway
// handler over exec.
func sessionReadFixture(t *testing.T, state string, exec sshgateway.Executor) *AgentSessionLifecycle {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sandboxes/os-agent" {
			http.NotFound(w, r)
			return
		}
		sb := osSandbox{ID: "os-agent", Metadata: map[string]string{
			metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
			metadataNetworkPolicy: string(NetworkPolicyDenyAll), agentsession.LabelSession: "ags-one",
		}}
		sb.Status.State = state
		_ = json.NewEncoder(w).Encode(sb)
	}))
	t.Cleanup(upstream.Close)
	secret := []byte("exec-secret")
	gateway := httptest.NewServer((&sandboxsse.Server{
		Secret: secret, Executor: exec,
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:  sshgateway.NewSessionLimiter(10, 5),
	}).Handler())
	t.Cleanup(gateway.Close)
	return &AgentSessionLifecycle{service: &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(upstream.URL),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: sshgateway.NewUpstreamClient(time.Minute)},
	}}
}

// A sandbox Pod on a NotReady node still reads Running. Its status read must
// come back transient — retried, never "sandbox runtime not configured" and
// never a dead sandbox — while a terminated Pod still reads NotFound, with how
// its container ended.
func TestReadSessionStatusTellsANodeStallFromADeadSandbox(t *testing.T) {
	stalled := sessionReadFixture(t, "Running", &podExecutor{pod: sandboxPod(corev1.PodRunning, corev1.ConditionFalse, runningContainer)})
	_, err := stalled.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
	if !Transient(err) || errors.Is(err, core.ErrSandboxesUnavailable) || errors.Is(err, core.ErrNotFound) {
		t.Fatalf("status read on a NotReady node = %v, want a transient error only", err)
	}
	if !hasCode(err, CodeSandboxUnreachable) || !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("stalled read = %v, want %s naming the not-ready pod", err, CodeSandboxUnreachable)
	}

	oom := sessionReadFixture(t, "Running", &podExecutor{pod: sandboxPod(corev1.PodFailed, corev1.ConditionFalse,
		corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}})})
	_, err = oom.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
	if !errors.Is(err, core.ErrNotFound) || Transient(err) {
		t.Fatalf("status read on a terminated pod = %v, want NotFound", err)
	}
	termination, ok := TerminationOf(err)
	if !ok || termination.Describe() != "the sandbox ran out of memory (exit 137)" {
		t.Fatalf("terminated read carries %+v (ok=%t), want OOMKilled exit 137", termination, ok)
	}

	live := sessionReadFixture(t, "Running", &podExecutor{pod: sandboxPod(corev1.PodRunning, corev1.ConditionTrue, runningContainer), stdout: `{"state":"running"}`})
	if raw, err := live.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent"); err != nil || raw != `{"state":"running"}` {
		t.Fatalf("live read = %q, %v", raw, err)
	}
}

// A lifecycle state bex does not know is not a dead sandbox: the read is
// transient, names the unknown state, and nothing is exec'd.
func TestSessionReadsTreatAnUnknownStateAsTransient(t *testing.T) {
	for _, state := range []string{"", "Stopping", "Hibernating"} {
		t.Run("state="+state, func(t *testing.T) {
			exec := &podExecutor{pod: sandboxPod(corev1.PodRunning, corev1.ConditionTrue, runningContainer)}
			lifecycle := sessionReadFixture(t, state, exec)
			_, statusErr := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
			_, transcriptErr := lifecycle.ReadSessionTranscript(context.Background(), "tea-a", "ags-one", "os-agent")
			for _, err := range []error{statusErr, transcriptErr} {
				if !Transient(err) || !StateUnknown(err) || errors.Is(err, core.ErrNotFound) {
					t.Fatalf("read in state %q = %v, want a transient unknown-state error", state, err)
				}
			}
			if exec.ran != 0 {
				t.Fatalf("a sandbox in an unknown state was exec'd %d times", exec.ran)
			}
			if got := mapOpenSandboxStatus(state); got != StatusErrored {
				t.Fatalf("views report state %q as %s, want errored as before", state, got)
			}
		})
	}
}

// A sandbox the lifecycle server reports Failed is gone, and its status read
// asks the gateway once how its container ended; a Pod already gone and a
// container still running record nothing. A terminated one is never exec'd.
func TestAnErroredSandboxReadCarriesHowItsContainerEnded(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state string
		exec  *podExecutor
		want  string
		ran   int
	}{
		{"OOM killed", "Failed", &podExecutor{pod: sandboxPod(corev1.PodFailed, corev1.ConditionFalse,
			corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}})},
			"the sandbox ran out of memory (exit 137)", 1},
		{"pod gone", "Failed", &podExecutor{}, "", 1},
		{"container running", "error", &podExecutor{pod: sandboxPod(corev1.PodRunning, corev1.ConditionTrue, runningContainer)}, "", 1},
		{"terminated", "Terminated", &podExecutor{pod: sandboxPod(corev1.PodFailed, corev1.ConditionFalse,
			corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}})}, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lifecycle := sessionReadFixture(t, tc.state, tc.exec)
			_, err := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
			if !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("read = %v, want NotFound", err)
			}
			termination, _ := TerminationOf(err)
			if got := termination.Describe(); got != tc.want || tc.exec.ran != tc.ran {
				t.Fatalf("termination %q after %d execs, want %q after %d", got, tc.exec.ran, tc.want, tc.ran)
			}
		})
	}
}

// A configured gateway that cannot be reached, or answers with a server
// error, is out of reach for now, not an unconfigured runtime; one that
// refuses bex-api's ticket is forbidden.
func TestAnUnreachableGatewayIsTransient(t *testing.T) {
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gone := "http://" + closed.Addr().String()
	_ = closed.Close()
	answering := func(status int) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, http.StatusText(status), status)
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}
	for name, url := range map[string]string{"refused": gone, "5xx": answering(http.StatusBadGateway)} {
		lifecycle := sessionReadFixture(t, "Running", &podExecutor{})
		lifecycle.service.Exec.GatewayURL = url
		_, err := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent")
		if !Transient(err) || errors.Is(err, core.ErrSandboxesUnavailable) {
			t.Fatalf("%s gateway = %v, want transient", name, err)
		}
	}
	lifecycle := sessionReadFixture(t, "Running", &podExecutor{})
	lifecycle.service.Exec.GatewayURL = answering(http.StatusForbidden)
	if _, err := lifecycle.ReadSessionStatus(context.Background(), "tea-a", "ags-one", "os-agent"); !errors.Is(err, core.ErrForbidden) || Transient(err) {
		t.Fatalf("gateway admission refusal = %v, want ErrForbidden", err)
	}
}

// How a lost sandbox's container ended, in the words a session's failure
// reason uses.
func TestTerminationDescribe(t *testing.T) {
	code := func(n int) *int { return &n }
	for _, tc := range []struct {
		termination Termination
		want        string
	}{
		{Termination{Reason: "OOMKilled", ExitCode: code(137)}, "the sandbox ran out of memory (exit 137)"},
		{Termination{Reason: "OOMKilled"}, "the sandbox ran out of memory (exit 137)"},
		{Termination{Reason: "Error", ExitCode: code(1)}, "the sandbox's agent exited with code 1"},
		{Termination{Reason: "StartError", ExitCode: code(128)}, "the sandbox's agent exited with code 128 (StartError)"},
		{Termination{Reason: "Error", ExitCode: code(137)}, "the sandbox's agent was killed by signal 9 (exit 137)"},
		{Termination{Reason: "Completed", ExitCode: code(0)}, "the sandbox's agent exited before finishing its turn (exit 0)"},
		{Termination{Reason: "Evicted"}, "the sandbox was evicted from its node"},
		{Termination{Reason: "EvictedEphemeralStorage"}, "the sandbox ran out of disk space (it wrote more than its ephemeral storage limit)"},
		{Termination{Reason: "EvictedEphemeralStorage", ExitCode: code(137)}, "the sandbox ran out of disk space (it wrote more than its ephemeral storage limit; exit 137)"},
		{Termination{Reason: "DeletionByTaintManager"}, "the sandbox was evicted from its node after the node stopped responding"},
		{Termination{Reason: "DeadlineExceeded"}, "the sandbox stopped (DeadlineExceeded)"},
		{Termination{Reason: "not one word; rm -rf"}, ""},
		{Termination{}, ""},
	} {
		if got := tc.termination.Describe(); got != tc.want {
			t.Errorf("%+v.Describe() = %q, want %q", tc.termination, got, tc.want)
		}
	}
}
