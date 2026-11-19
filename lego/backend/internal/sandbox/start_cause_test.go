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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The pod shapes below are the ones Kubernetes 1.34 reports for each cause
// (ADR047 § w10/m497); the pool-full one is what production's sandbox pods
// showed on 2026-11-17 and 18.

func unschedulablePod() *corev1.Pod {
	return &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
		Message: "0/9 nodes are available: 1 Insufficient memory, 8 node(s) didn't match Pod's node affinity/selector.",
	}}}}
}

func scheduledPod(readyToStart corev1.ConditionStatus, waiting ...string) *corev1.Pod {
	pod := &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{
		{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
		{Type: corev1.PodReadyToStartContainers, Status: readyToStart},
	}}}
	for i, reason := range waiting {
		status := corev1.ContainerStatus{Name: "c", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}}
		if i == 0 {
			// The first reason is the init container's (execd-installer).
			pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, status)
			continue
		}
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, status)
	}
	return pod
}

func TestEachPodShapeNamesItsCause(t *testing.T) {
	failed := scheduledPod(corev1.ConditionTrue)
	failed.Status.Phase = corev1.PodFailed
	for _, tc := range []struct {
		name string
		pod  *corev1.Pod
		want StartCause
	}{
		{"unschedulable for memory", unschedulablePod(), StartCausePoolFull},
		{"init image does not pull", scheduledPod(corev1.ConditionTrue, "ErrImagePull"), StartCauseImagePull},
		{"image pull backing off", scheduledPod(corev1.ConditionTrue, "PodInitializing", "ImagePullBackOff"), StartCauseImagePull},
		{"invalid image name", scheduledPod(corev1.ConditionTrue, "PodInitializing", "InvalidImageName"), StartCauseImagePull},
		{"pod sandbox never started (FailedCreatePodSandBox, no runtime handler)", scheduledPod(corev1.ConditionFalse, "PodInitializing"), StartCauseRuntime},
		{"container cannot be created", scheduledPod(corev1.ConditionTrue, "PodInitializing", "CreateContainerError"), StartCauseRuntime},
		{"pod failed", failed, StartCauseRuntime},
		{"still initializing", scheduledPod(corev1.ConditionTrue, "PodInitializing"), StartCauseUnknown},
		{"no status at all", &corev1.Pod{}, StartCauseUnknown},
	} {
		if got := podStartCause(tc.pod); got != tc.want {
			t.Errorf("%s: cause = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestEachCauseHasItsOwnWords(t *testing.T) {
	seen := map[string]StartCause{}
	for _, cause := range []StartCause{StartCausePoolFull, StartCauseImagePull, StartCauseRuntime, StartCauseQuota, StartCauseUnknown} {
		words := StartFailureWords(cause, 90*time.Minute)
		if other, dup := seen[words]; dup {
			t.Errorf("%s and %s share the words %q", cause, other, words)
		}
		seen[words] = cause
	}
	for waited, want := range map[time.Duration]string{time.Hour + 20*time.Second: "for 60 minutes,", 40 * time.Second: "for 1 minute,", 0: "for 1 minute,"} {
		if got := StartFailureWords(StartCausePoolFull, waited); !strings.Contains(got, want) {
			t.Errorf("pool-full words after %s = %q, want %q", waited, got, want)
		}
	}
	if got := StartFailureWords(StartCauseUnknown, 0); got != StartFailureReason {
		t.Errorf("unknown words = %q, want the sentence every start failure carried before", got)
	}
}

func TestTheCauseRidesTheStartFailure(t *testing.T) {
	for _, cause := range []StartCause{StartCausePoolFull, StartCauseImagePull, StartCauseRuntime, StartCauseQuota, StartCauseUnknown} {
		err := sandboxStartError(cause)
		if !IsStartFailure(err) || StartCauseOf(err) != cause || IsPoolFull(err) != (cause == StartCausePoolFull) {
			t.Errorf("%s: start failure %v carries cause %q", cause, IsStartFailure(err), StartCauseOf(err))
		}
		if strings.Contains(err.Error(), "upgrade") {
			t.Errorf("%s: a platform start failure must not offer an upgrade: %s", cause, err)
		}
	}
	if StartCauseOf(sandboxCapacityError()) != "" || StartCauseOf(errors.New("x")) != "" {
		t.Error("only a start failure carries a cause")
	}
}

// podEnv is an OpenSandbox server that holds every create for hold before it
// answers the pod-ready timeout, and a Kubernetes API holding pods.
func podEnv(t *testing.T, hold time.Duration, objects ...client.Object) (*Service, *int32) {
	t.Helper()
	prev := startPoll
	startPoll = 5 * time.Millisecond
	t.Cleanup(func() { startPoll = prev })
	var reads int32
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	pods := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			atomic.AddInt32(&reads, 1)
			return c.List(ctx, list, opts...)
		},
	}).Build()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		time.Sleep(hold)
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = io.WriteString(w, `{"code":"KUBERNETES::POD_READY_TIMEOUT","message":"Last state: Pending"}`)
	}))
	t.Cleanup(upstream.Close)
	return &Service{
		Base:            &core.Base{Namespace: "default"},
		Client:          NewClient(upstream.URL),
		Templates:       map[string]Template{"node": {Image: "node:20", CPU: "1", Memory: "1Gi"}},
		DefaultTemplate: "node",
		Pods:            pods,
	}, &reads
}

// sessionPod is turn 1's pod of session ags-wait in tea-a's sandbox namespace.
func sessionPod(shape *corev1.Pod) *corev1.Pod {
	shape.ObjectMeta = metav1.ObjectMeta{Name: "box-0", Namespace: store.SandboxNamespace("tea-a"), Labels: map[string]string{
		agentsession.LabelSession: "ags-wait", agentsession.LabelDispatchTurn: "1",
	}}
	return shape
}

func waitingCreate(t *testing.T, svc *Service, poolFull func()) error {
	t.Helper()
	_, err := svc.createResolved(callerCtx(), "tea-a", "node", svc.Templates["node"], PlanStandard, 0,
		&NetworkPolicy{Default: NetworkPolicyDenyAll}, nil,
		map[string]string{agentsession.LabelSession: "ags-wait", agentsession.LabelDispatchTurn: "1"}, poolFull)
	return err
}

// The pod is read while the create waits, and the pool-full hook fires before
// the create ends, so the session can say it waits right away.
func TestATimeoutWhoseUnschedulablePodWasSeenIsPoolFull(t *testing.T) {
	svc, reads := podEnv(t, 60*time.Millisecond, sessionPod(unschedulablePod()))
	var hooked atomic.Int32
	err := waitingCreate(t, svc, func() { hooked.Add(1) })
	if StartCauseOf(err) != StartCausePoolFull {
		t.Fatalf("create = %v (cause %q), want SANDBOX_START_FAILED pool_full", err, StartCauseOf(err))
	}
	if hooked.Load() != 1 {
		t.Fatalf("pool-full hook fired %d times, want once", hooked.Load())
	}
	if atomic.LoadInt32(reads) < 2 {
		t.Fatalf("pod reads = %d, want reads while the create waited", *reads)
	}
}

// The server deletes the pod when its wait ends: the last read that found it
// names the cause, not the read after the 504 that finds nothing.
func TestTheLastPodSeenNamesTheCauseAfterTheServerDeletesIt(t *testing.T) {
	pod := sessionPod(scheduledPod(corev1.ConditionTrue, "ErrImagePull"))
	svc, _ := podEnv(t, 60*time.Millisecond, pod)
	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = svc.Pods.(client.Client).Delete(context.Background(), pod)
	}()
	if err := waitingCreate(t, svc, nil); StartCauseOf(err) != StartCauseImagePull {
		t.Fatalf("create = %v, want cause image_pull", err)
	}
}

// A pod placed on a machine was admitted by the quota, so a full plan budget
// does not turn it into SANDBOX_CAPACITY_LIMIT.
func TestAPodSeenOnAMachineNeverBlamesThePlan(t *testing.T) {
	svc, _ := podEnv(t, 40*time.Millisecond, sessionPod(scheduledPod(corev1.ConditionFalse)))
	quotaCPU, _ := store.SandboxComputeLimits("")
	svc.Templates["node"] = Template{Image: "node:20", CPU: quotaCPU.String(), Memory: "1Gi"}
	err := waitingCreate(t, svc, nil)
	if IsCapacityLimit(err) || StartCauseOf(err) != StartCauseRuntime {
		t.Fatalf("create = %v, want SANDBOX_START_FAILED runtime, not the plan limit", err)
	}
}

func TestNoPodAndAFullQuotaIsTheQuota(t *testing.T) {
	ns := store.SandboxNamespace("tea-a")
	quota := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: core.TenantComputeQuotaName, Namespace: ns},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{corev1.ResourceLimitsMemory: resource.MustParse("100Gi")},
			Used: corev1.ResourceList{corev1.ResourceLimitsMemory: resource.MustParse("98Gi")},
		}}
	svc, _ := podEnv(t, 20*time.Millisecond, quota)
	if err := waitingCreate(t, svc, nil); StartCauseOf(err) != StartCauseQuota {
		t.Fatalf("create = %v (cause %q), want quota", err, StartCauseOf(err))
	}
	// With room on every key and no pod, nothing names the cause.
	quota.Status.Used[corev1.ResourceLimitsMemory] = resource.MustParse("1Gi")
	svc, _ = podEnv(t, 20*time.Millisecond, quota)
	if err := waitingCreate(t, svc, nil); StartCauseOf(err) != StartCauseUnknown {
		t.Fatalf("create = %v (cause %q), want unknown", err, StartCauseOf(err))
	}
}

// A read the API refuses (no grant yet in the namespace) degrades to
// unknown, never to an error of its own.
func TestARefusedReadIsUnknown(t *testing.T) {
	svc, _ := podEnv(t, 30*time.Millisecond)
	svc.Pods = fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return errors.New(`pods is forbidden: User "system:serviceaccount:bex-system:bex-api" cannot list resource "pods"`)
		},
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return errors.New("forbidden")
		},
	}).Build()
	err := waitingCreate(t, svc, nil)
	if !IsStartFailure(err) || StartCauseOf(err) != StartCauseUnknown {
		t.Fatalf("create = %v (cause %q), want SANDBOX_START_FAILED unknown", err, StartCauseOf(err))
	}
}

// A Render sandbox create's 503 carries the cause too, as params.cause.
func TestARenderSandboxTimeoutAnswersTheCause(t *testing.T) {
	svc, _ := podEnv(t, 40*time.Millisecond)
	// The Render create mints the public id, so find its pod by any label: the
	// fake's pod carries a public id the test sets on the first read.
	svc.Pods = fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			var o client.ListOptions
			o.ApplyOptions(opts)
			if o.LabelSelector == nil || !strings.Contains(o.LabelSelector.String(), metadataPublicID) {
				t.Errorf("a Render sandbox's pod is read by %v, want its public id", o.LabelSelector)
			}
			pod := unschedulablePod()
			list.(*corev1.PodList).Items = []corev1.Pod{*pod}
			return nil
		},
	}).Build()
	rec := restCreate(t, svc)
	var body struct {
		Code   string         `json:"code"`
		Params map[string]any `json:"params"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusServiceUnavailable || body.Code != CodeSandboxStartFailed || body.Params["cause"] != "pool_full" {
		t.Fatalf("create = %d %s, want 503 SANDBOX_START_FAILED with params.cause pool_full", rec.Code, rec.Body.String())
	}
}
