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
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// StartCause is why a sandbox's pod did not become ready within the
// OpenSandbox server's wait, as bex-api read it from the pod while the create
// waited (ADR047 § w10/m497). It rides SANDBOX_START_FAILED as params.cause
// and an agent session's failureCause.
type StartCause string

const (
	// StartCausePoolFull is a pod the scheduler could not place: no sandbox
	// machine has room for it, or none is ready. An agent session waits.
	StartCausePoolFull StartCause = "pool_full"
	// StartCauseImagePull is an image that did not pull.
	StartCauseImagePull StartCause = "image_pull"
	// StartCauseRuntime is a pod placed on a machine that could not start it:
	// the pod sandbox never came up (FailedCreatePodSandBox, a missing
	// RuntimeClass handler) or a container could not be created.
	StartCauseRuntime StartCause = "runtime"
	// StartCauseQuota is a pod the namespace's ResourceQuota refused.
	StartCauseQuota StartCause = "quota"
	// StartCauseUnknown is anything else, including a read that failed.
	StartCauseUnknown StartCause = "unknown"
)

// startCauseWords is the failure reason recorded for each cause. Unknown keeps
// the sentence every start failure carried before (w9/074).
var startCauseWords = map[StartCause]string{
	StartCauseImagePull: "The sandbox image could not be pulled, so the session did not start. Try again, and contact support if it keeps failing.",
	StartCauseRuntime:   "The sandbox runtime could not start this session's machine. Try again, and contact support if it keeps failing.",
	StartCauseQuota:     "The workspace's sandbox quota refused this session's machine. Stop a running sandbox or session, then try again.",
}

// startCauseParam is the SANDBOX_START_FAILED param that carries the cause.
const startCauseParam = "cause"

// StartFailureWords is the failure reason for a start that failed for cause.
// A full pool says how long the session waited for a machine, in whole
// minutes.
func StartFailureWords(cause StartCause, waited time.Duration) string {
	if cause == StartCausePoolFull {
		minutes := max(1, int(waited.Round(time.Minute)/time.Minute))
		unit := "minutes"
		if minutes == 1 {
			unit = "minute"
		}
		return fmt.Sprintf("No sandbox machine had room for this session for %d %s, so it did not start. Send the message again to retry.", minutes, unit)
	}
	if words, ok := startCauseWords[cause]; ok {
		return words
	}
	return StartFailureReason
}

// StartCauseOf is the cause a SANDBOX_START_FAILED error carries; "" for any
// other error.
func StartCauseOf(err error) StartCause {
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodeSandboxStartFailed {
		return ""
	}
	if cause, ok := coded.Params[startCauseParam].(string); ok && cause != "" {
		return StartCause(cause)
	}
	return StartCauseUnknown
}

// IsPoolFull reports a start that failed because the pool had no room: the
// one cause an agent session waits on.
func IsPoolFull(err error) bool { return StartCauseOf(err) == StartCausePoolFull }

// startPoll is how often a waiting create reads its pod. A var so tests poll
// fast.
var startPoll = 10 * time.Second

// startReadTimeout bounds one pod or quota read.
const startReadTimeout = 5 * time.Second

// imagePullReasons and runtimeReasons are the container waiting reasons that
// name a cause.
var (
	imagePullReasons = map[string]bool{"ErrImagePull": true, "ImagePullBackOff": true, "InvalidImageName": true, "ErrImageNeverPull": true}
	runtimeReasons   = map[string]bool{"CreateContainerError": true, "CreateContainerConfigError": true, "RunContainerError": true}
)

// podStartCause names why pod is not running from the pod alone: its
// scheduling condition, its containers' waiting reasons, and whether its pod
// sandbox started.
func podStartCause(pod *corev1.Pod) StartCause {
	scheduled := false
	readyToStart := corev1.ConditionUnknown
	for _, c := range pod.Status.Conditions {
		switch c.Type {
		case corev1.PodScheduled:
			if c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
				return StartCausePoolFull
			}
			scheduled = c.Status == corev1.ConditionTrue
		case corev1.PodReadyToStartContainers:
			readyToStart = c.Status
		}
	}
	var reasons []string
	for _, statuses := range [][]corev1.ContainerStatus{pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses} {
		for _, st := range statuses {
			if st.State.Waiting != nil {
				reasons = append(reasons, st.State.Waiting.Reason)
			}
		}
	}
	if slices.ContainsFunc(reasons, func(r string) bool { return imagePullReasons[r] }) {
		return StartCauseImagePull
	}
	if slices.ContainsFunc(reasons, func(r string) bool { return runtimeReasons[r] }) {
		return StartCauseRuntime
	}
	if pod.Status.Phase == corev1.PodFailed || (scheduled && readyToStart == corev1.ConditionFalse) {
		return StartCauseRuntime
	}
	return StartCauseUnknown
}

// startSelector is the labels that select one create's pod: a Render
// sandbox's public id, or an agent sandbox's principal and dispatch turn.
// nil when the create carries neither.
func startSelector(metadata map[string]string) map[string]string {
	if id := metadata[metadataPublicID]; id != "" {
		return map[string]string{metadataPublicID: id}
	}
	principal := metadata[agentsession.LabelSession]
	if principal == "" {
		return nil
	}
	selector := map[string]string{agentsession.LabelSession: principal}
	if turn := metadata[agentsession.LabelDispatchTurn]; turn != "" {
		selector[agentsession.LabelDispatchTurn] = turn
	}
	return selector
}

// startWatch reads a create's pod while the create waits, because the
// OpenSandbox server deletes the pod when its wait ends. The last read that
// found the pod names the cause; when no read ever found one, the namespace's
// quota decides between quota and unknown.
type startWatch struct {
	pods      client.Reader
	namespace string
	selector  map[string]string
	poolFull  func()

	// lastPod and logged belong to the run goroutine until done closes.
	lastPod StartCause
	logged  bool
	stop    context.CancelFunc
	done    chan struct{}
}

// watchStart starts reading the pod that selector names in workspace's
// sandbox namespace. poolFull, when set, is called once, on the first read
// that finds the pod waiting for room. nil when there is nothing to read.
func (s *Service) watchStart(ctx context.Context, workspace string, selector map[string]string, poolFull func()) *startWatch {
	if s.Pods == nil || len(selector) == 0 {
		return nil
	}
	ctx, stop := context.WithCancel(context.WithoutCancel(ctx))
	w := &startWatch{pods: s.Pods, namespace: store.SandboxNamespace(workspace), selector: selector, poolFull: poolFull, stop: stop, done: make(chan struct{})}
	go w.run(ctx)
	return w
}

func (w *startWatch) run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(startPoll)
	defer ticker.Stop()
	notified := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		cause, found := w.readPod(ctx)
		if !found {
			continue
		}
		w.lastPod = cause
		if cause == StartCausePoolFull && !notified && w.poolFull != nil {
			notified = true
			w.poolFull()
		}
	}
}

// end stops the reads. Safe on a nil watch.
func (w *startWatch) end() {
	if w == nil {
		return
	}
	w.stop()
	<-w.done
}

// cause stops the reads and names the cause. Safe on a nil watch, which has
// nothing to name.
func (w *startWatch) cause(ctx context.Context) StartCause {
	if w == nil {
		return StartCauseUnknown
	}
	w.end()
	if w.lastPod != "" {
		return w.lastPod
	}
	// No read found the pod (or the create answered before the first read):
	// read once more, then let the quota decide.
	if cause, found := w.readPod(ctx); found {
		return cause
	}
	if w.quotaRefuses(ctx) {
		return StartCauseQuota
	}
	return StartCauseUnknown
}

// readPod reads the create's pod. found is false when no pod matches or the
// read failed; a failed read is logged and never an error (it degrades to
// unknown).
func (w *startWatch) readPod(ctx context.Context) (StartCause, bool) {
	ctx, cancel := context.WithTimeout(ctx, startReadTimeout)
	defer cancel()
	var pods corev1.PodList
	if err := w.pods.List(ctx, &pods, client.InNamespace(w.namespace), client.MatchingLabels(w.selector)); err != nil {
		first := !w.logged
		w.logged = true
		if first && ctx.Err() == nil {
			log.Printf("sandbox create: read the pod's state failed (namespace=%s): %v", w.namespace, err)
		}
		return "", false
	}
	if len(pods.Items) == 0 {
		return "", false
	}
	return podStartCause(&pods.Items[0]), true
}

// quotaRefuses reports a namespace ResourceQuota with less room on some key
// than one sandbox pod charges at most, which is what refuses a pod before it
// exists.
func (w *startWatch) quotaRefuses(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, startReadTimeout)
	defer cancel()
	most := store.SandboxPodQuotaMax()
	for _, name := range []string{core.TenantComputeQuotaName, core.TenantQuotaName} {
		var quota corev1.ResourceQuota
		if err := w.pods.Get(ctx, client.ObjectKey{Namespace: w.namespace, Name: name}, &quota); err != nil {
			continue
		}
		for key, hard := range quota.Status.Hard {
			need, ok := most[key]
			if !ok {
				continue
			}
			room := hard.DeepCopy()
			room.Sub(quota.Status.Used[key])
			if room.Cmp(need) < 0 {
				return true
			}
		}
	}
	return false
}
