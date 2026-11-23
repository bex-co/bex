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

package controller

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The disk pre-pull (w6/m595): a disk service's code deploy pulls its new
// image onto the volume's node before Recreate stops the old instance.

// prePullTemplate is a rolled template that names its pull credentials and is
// selected for signature admission.
func prePullTemplate() *corev1.PodTemplateSpec {
	return &corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: "blog", execution.LabelVerifyImage: execution.VerifyImageEnabled}},
		Spec:       corev1.PodSpec{ImagePullSecrets: []corev1.LocalObjectReference{{Name: "reg-pull-blog"}, {Name: "linked-source"}}},
	}
}

func diskPrePullFor(t *testing.T, app *appv1alpha1.App, tier string, images ...string) *batchv1.Job {
	t.Helper()
	job, err := diskPrePullJob(app, tier, "node-a", images, prePullTemplate(), diskHoldScheme(t))
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestDiskPrePullJobIsTheAppsAndNoInstance(t *testing.T) {
	app := diskHoldApp()
	job := diskPrePullFor(t, app, "starter", "zot/blog@sha256:new")
	if job.Name != "dskpul-blog" || job.Namespace != app.Namespace {
		t.Errorf("job = %s/%s, want default/dskpul-blog", job.Namespace, job.Name)
	}
	long := diskPrePullFor(t, diskHoldApp(func(a *appv1alpha1.App) { a.Name = strings.Repeat("a", 63) }), "starter", "x")
	if len(long.Name) > 63 || !strings.HasPrefix(long.Name, diskPrePullPrefix) {
		t.Errorf("a long App's pre-pull is %q, want a dskpul- name within 63 characters", long.Name)
	}
	if owner := metav1.GetControllerOf(job); owner == nil || owner.Kind != "App" || owner.Name != app.Name || owner.UID != app.UID {
		t.Fatalf("controller = %+v, want the App, which bex-operator-workloads requires to admit it", owner)
	}
	for name, labels := range map[string]map[string]string{"job": job.Labels, "pod": job.Spec.Template.Labels} {
		if labels[labelApp] != app.Name || labels[execution.LabelAppUID] != string(app.UID) {
			t.Errorf("%s labels %v: want the App's name and uid, which admission and appPodRequests read", name, labels)
		}
		if labels[execution.LabelComponent] != diskPrePullComponent {
			t.Errorf("%s labels %v: want component %q, which keeps it off every instance list", name, labels, diskPrePullComponent)
		}
		for _, key := range []string{labelAppID, labelRevision, execution.LabelContainerPolicy} {
			if _, ok := labels[key]; ok {
				t.Errorf("%s labels %v carry %s: a Service, a revision reader or a widened container policy would take the pre-pull", name, labels, key)
			}
		}
	}
	if job.Spec.Template.Labels[execution.LabelVerifyImage] != execution.VerifyImageEnabled {
		t.Errorf("pod labels %v: want the template's verify-image label, so the webhook refuses an unsigned image before any node pulls it", job.Spec.Template.Labels)
	}
	if unsigned, err := diskPrePullJob(app, "starter", "node-a", []string{"x"}, &corev1.PodTemplateSpec{}, diskHoldScheme(t)); err != nil {
		t.Fatal(err)
	} else if _, ok := unsigned.Spec.Template.Labels[execution.LabelVerifyImage]; ok {
		t.Error("a template the webhook does not verify gave its pre-pull the verify-image label")
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Errorf("backoffLimit = %v, want 0: a failed pull is a rollout without one", job.Spec.BackoffLimit)
	}
	if d := job.Spec.ActiveDeadlineSeconds; d == nil || *d <= int64(rolloutBudgetSeconds) {
		t.Errorf("activeDeadlineSeconds = %v, want past the %ds rollout budget: an early deadline cancels a pull the new instance waits for", d, rolloutBudgetSeconds)
	}
	if job.Spec.TTLSecondsAfterFinished == nil {
		t.Error("no ttlSecondsAfterFinished: a pre-pull the operator never saw end would stay")
	}
}

func TestDiskPrePullPodPullsEachImageOnTheNodeAndRunsNone(t *testing.T) {
	images := []string{"zot/blog@sha256:new", "zot/bex-operator@sha256:probe"}
	pod := diskPrePullFor(t, diskHoldApp(), "starter", images...).Spec.Template.Spec
	if pod.NodeName != "" {
		t.Errorf("nodeName = %q: bex-operator-workloads refuses a pinned node", pod.NodeName)
	}
	if nodes, required := preferredNodes(pod); len(nodes) != 0 || !required {
		t.Errorf("preferred %v, required by name %v: want the node required, nothing preferred", nodes, required)
	}
	if pod.RestartPolicy != corev1.RestartPolicyNever || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken ||
		pod.ServiceAccountName != "" || pod.PriorityClassName != "" || len(pod.Tolerations) != 0 || len(pod.Volumes) != 0 || len(pod.InitContainers) != 0 {
		t.Errorf("pod = restart %s, automount %v, sa %q, priority %q, tolerations %v, volumes %v, init %d: want Never, no token, the default account, no class, toleration or volume",
			pod.RestartPolicy, pod.AutomountServiceAccountToken, pod.ServiceAccountName, pod.PriorityClassName, pod.Tolerations, pod.Volumes, len(pod.InitContainers))
	}
	if !slices.Equal(pod.ImagePullSecrets, prePullTemplate().Spec.ImagePullSecrets) {
		t.Errorf("imagePullSecrets = %v, want the template's: the kubelet pulls with the instance's credentials", pod.ImagePullSecrets)
	}
	if len(pod.Containers) != len(images) {
		t.Fatalf("containers = %d, want one per image", len(pod.Containers))
	}
	for i, c := range pod.Containers {
		checkPrePullContainer(t, i, c, images[i])
	}

	pooled := diskPrePullFor(t, diskHoldApp(), "pro-plus", images...)
	if tol := pooled.Spec.Template.Spec.Tolerations; len(tol) != 1 || tol[0].Key != execution.ServingPoolKey || tol[0].Value != tiers.Compute.ServingPool("pro-plus") {
		t.Errorf("pooled tolerations = %v, want the pro-plus pool's", tol)
	}
}

// checkPrePullContainer checks the pre-pull's container i, which pulls image.
func checkPrePullContainer(t *testing.T, i int, c corev1.Container, image string) {
	t.Helper()
	if c.Image != image || c.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("container %d pulls %s (%s), want %s re-checked as the instance pulls it", i, c.Image, c.ImagePullPolicy, image)
	}
	if c.Name == appContainerName || c.Name == "predeploy" {
		t.Errorf("container %d is named %q: log, exit and pull readers take that container for the tenant's", i, c.Name)
	}
	if !slices.Equal(c.Command, []string{"/"}) || len(c.Args) != 0 {
		t.Errorf("container %d runs %v %v, want the directory no image can execute", i, c.Command, c.Args)
	}
	if len(c.VolumeMounts) != 0 || len(c.Ports) != 0 || len(c.Env) != 0 || len(c.EnvFrom) != 0 {
		t.Errorf("container %d mounts %v, ports %v, env %v %v: want none", i, c.VolumeMounts, c.Ports, c.Env, c.EnvFrom)
	}
	sc := c.SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation ||
		sc.Capabilities == nil || len(sc.Capabilities.Add) != 0 || !slices.Equal(sc.Capabilities.Drop, []corev1.Capability{"ALL"}) ||
		sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault ||
		sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot || sc.RunAsUser == nil || *sc.RunAsUser == 0 ||
		sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Errorf("container %d security context %+v: want restricted, an explicit non-root user, read-only root", i, sc)
	}
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage} {
		if request, ok := c.Resources.Requests[name]; !ok || !request.IsZero() {
			t.Errorf("container %d %s request = %v (set %v), want an explicit 0", i, name, request.String(), ok)
		}
	}
	// runc's init runs under the CPU limit, as the holder's does.
	if !equality.Semantic.DeepEqual(c.Resources, diskHolderResources()) {
		t.Errorf("container %d resources = %+v, want the holder's", i, c.Resources)
	}
}

func TestNewTemplateImagesNamesOnlyWhatTheRunningTemplateDoesNot(t *testing.T) {
	spec := func(initImage string, images ...string) *corev1.PodSpec {
		s := &corev1.PodSpec{}
		if initImage != "" {
			s.InitContainers = []corev1.Container{{Name: "health-probe", Image: initImage}}
		}
		for _, image := range images {
			s.Containers = append(s.Containers, corev1.Container{Image: image})
		}
		return s
	}
	for _, tc := range []struct {
		name              string
		stored, projected *corev1.PodSpec
		want              []string
	}{
		{"a restart or plan change", spec("op:1", "app:1"), spec("op:1", "app:1"), nil},
		{"a code deploy", spec("op:1", "app:1"), spec("op:1", "app:2"), []string{"app:2"}},
		{"a rollback to an image the sidecar runs", spec("", "app:1", "app:2"), spec("", "app:2"), nil},
		{"a new init image and a new app image", spec("op:1", "app:1"), spec("op:2", "app:2", "app:2"), []string{"app:2", "op:2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := newTemplateImages(tc.stored, tc.projected); !slices.Equal(got, tc.want) {
				t.Errorf("new images = %v, want %v", got, tc.want)
			}
		})
	}
}

// The kubelet's states for a pre-pull container, as the CAPD cluster posted
// them (t002): waiting ContainerCreating through the pull, then terminated in
// StartError with the image resolved.
var (
	pullingState = corev1.ContainerStatus{Name: "pull-0", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}}
	pulledState  = corev1.ContainerStatus{Name: "pull-0", ImageID: "zot/blog@sha256:new", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
		ExitCode: 128, Reason: "StartError", Message: `exec: "/": is a directory`}}}
)

func waitingState(reason string) corev1.ContainerStatus {
	return corev1.ContainerStatus{Name: "pull-0", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}}
}

func TestDiskPrePullStateReadsTheKubeletsPull(t *testing.T) {
	app := diskHoldApp()
	type podState struct {
		phase      corev1.PodPhase
		node       string
		statuses   []corev1.ContainerStatus
		conditions []corev1.PodCondition
		deleting   bool
	}
	for _, tc := range []struct {
		name       string
		containers int
		pod        *podState
		jobAge     time.Duration
		jobFailed  bool
		want       prePullState
	}{
		{name: "no pod yet", containers: 1, want: prePullPulling},
		{name: "no pod past its bound: refused", containers: 1, jobAge: diskPrePullPodBound + time.Second, want: prePullFailed},
		{name: "no pod and the Job failed", containers: 1, jobFailed: true, want: prePullFailed},
		{name: "pending scheduling", containers: 1, pod: &podState{phase: corev1.PodPending}, want: prePullPulling},
		{name: "unschedulable", containers: 1, pod: &podState{phase: corev1.PodPending, conditions: []corev1.PodCondition{{
			Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable}}}, want: prePullFailed},
		{name: "pulling", containers: 1, pod: &podState{phase: corev1.PodPending, node: "node-a",
			statuses: []corev1.ContainerStatus{pullingState}}, want: prePullPulling},
		{name: "pulled", containers: 1, pod: &podState{phase: corev1.PodFailed, node: "node-a",
			statuses: []corev1.ContainerStatus{pulledState}}, want: prePullPulled},
		{name: "pulled, and the Job failed for it", containers: 1, jobFailed: true, pod: &podState{phase: corev1.PodFailed, node: "node-a",
			statuses: []corev1.ContainerStatus{pulledState}}, want: prePullPulled},
		{name: "one of two pulled", containers: 2, pod: &podState{phase: corev1.PodPending, node: "node-a",
			statuses: []corev1.ContainerStatus{pulledState, pullingState}}, want: prePullPulling},
		{name: "both pulled", containers: 2, pod: &podState{phase: corev1.PodFailed, node: "node-a",
			statuses: []corev1.ContainerStatus{pulledState, pulledState}}, want: prePullPulled},
		{name: "created but not started", containers: 1, pod: &podState{phase: corev1.PodPending, node: "node-a",
			statuses: []corev1.ContainerStatus{waitingState(createContainerConfigError)}}, want: prePullPulled},
		{name: "ErrImagePull", containers: 1, pod: &podState{phase: corev1.PodPending, node: "node-a",
			statuses: []corev1.ContainerStatus{waitingState("ErrImagePull")}}, want: prePullFailed},
		{name: "ImagePullBackOff beside a pulled image", containers: 2, pod: &podState{phase: corev1.PodPending, node: "node-a",
			statuses: []corev1.ContainerStatus{pulledState, waitingState(imagePullBackOff)}}, want: prePullFailed},
		{name: "InvalidImageName", containers: 1, pod: &podState{phase: corev1.PodPending, node: "node-a",
			statuses: []corev1.ContainerStatus{waitingState(reasonInvalidImageName)}}, want: prePullFailed},
		{name: "killed by its deadline mid-pull", containers: 1, jobFailed: true, pod: &podState{phase: corev1.PodFailed, node: "node-a",
			statuses: []corev1.ContainerStatus{{Name: "pull-0", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 137, Reason: "ContainerStatusUnknown"}}}}}, want: prePullFailed},
		{name: "ended with no status", containers: 1, pod: &podState{phase: corev1.PodFailed, node: "node-a"}, want: prePullFailed},
		{name: "being deleted", containers: 1, pod: &podState{phase: corev1.PodPending, node: "node-a", deleting: true,
			statuses: []corev1.ContainerStatus{pullingState}}, want: prePullFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			images := []string{"zot/blog@sha256:new", "zot/side@sha256:new"}[:tc.containers]
			job := diskPrePullFor(t, app, "starter", images...)
			job.CreationTimestamp = metav1.NewTime(time.Now().Add(-tc.jobAge))
			if tc.jobFailed {
				job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
			}
			var pod *corev1.Pod
			if tc.pod != nil {
				pod = &corev1.Pod{Spec: *job.Spec.Template.Spec.DeepCopy()}
				pod.Spec.NodeName = tc.pod.node
				pod.Status = corev1.PodStatus{Phase: tc.pod.phase, ContainerStatuses: tc.pod.statuses, Conditions: tc.pod.conditions}
				if tc.pod.deleting {
					pod.DeletionTimestamp = new(metav1.Now())
				}
			}
			if got := diskPrePullState(job, pod); got != tc.want {
				t.Errorf("state = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDiskPrePullWritesACodeDeployOnlyOnceTheNewImageIsOnTheNode(t *testing.T) {
	f := newDiskHoldFixture(t)
	pulled := diskPrePullCount(diskPrePullPulled)
	f.image = "nginx:2"

	// The pass starts the holder and the pre-pull on the volume's node.
	if wait := f.apply(); wait <= 0 {
		t.Fatalf("wait = %v, want the code deploy held", wait)
	}
	prePull := f.prePull()
	if prePull == nil || diskJobNode(prePull) != "node-a" || !slices.Equal(diskPrePullImages(prePull), []string{"nginx:2"}) {
		t.Fatalf("pre-pull = %v, want one of nginx:2 on node-a", prePull)
	}
	f.holderPod(f.holder(), true)
	pod := f.prePullPod(prePull, pullingState)

	// The holder has the volume, but the image is still pulling.
	if wait := f.apply(); wait <= 0 || wait > diskPrePullPoll {
		t.Fatalf("wait = %v with the pull running, want the rollout held for at most %v", wait, diskPrePullPoll)
	}
	f.expectImage("nginx:1", "with the new image still pulling")

	// Pulled: the template is written in this pass, and the pre-pull ends
	// on the next.
	f.endPull(pod, pulledState)
	if wait := f.apply(); wait != 0 {
		t.Fatalf("wait = %v with the image pulled, want the template written", wait)
	}
	f.expectImage("nginx:2", "once the new image is on the node")
	if got := diskPrePullCount(diskPrePullPulled) - pulled; got != 1 {
		t.Errorf("pulled count rose by %v, want 1", got)
	}
	f.apply()
	if f.prePull() != nil {
		t.Error("the pre-pull outlived its rollout's write and its pull")
	}
	if f.holder() == nil {
		t.Error("the holder ended with the pre-pull; it must outlive the old instance")
	}
}

func TestDiskPrePullNeverStopsTheRollout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result string
		setup  func(*diskHoldFixture)
		// pull runs once the holder is mounted, with the pre-pull's Job (nil
		// when it was refused).
		pull func(*diskHoldFixture, *batchv1.Job)
		// left is whether the pre-pull outlives the write: a pull still
		// running is never cancelled.
		left bool
	}{{
		name:   "the pull failed",
		result: diskPrePullFailed,
		pull: func(f *diskHoldFixture, job *batchv1.Job) {
			f.prePullPod(job, waitingState("ErrImagePull"))
		},
	}, {
		name:   "the pull is still running past its bound",
		result: diskPrePullTimeout,
		pull: func(f *diskHoldFixture, job *batchv1.Job) {
			f.prePullPod(job, pullingState)
			f.age(job, diskPrePullBound+time.Second)
		},
		left: true,
	}, {
		name:   "the pre-pull's pod was refused",
		result: diskPrePullFailed,
		pull: func(f *diskHoldFixture, job *batchv1.Job) {
			f.age(job, diskPrePullPodBound+time.Second)
		},
	}, {
		name:   "the pre-pull Job was refused",
		result: diskPrePullRefused,
		setup: func(f *diskHoldFixture) {
			f.refusePrePull = true
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDiskHoldFixture(t)
			before := diskPrePullCount(tc.result)
			if tc.setup != nil {
				tc.setup(f)
			}
			f.image = "nginx:2"
			if wait := f.apply(); wait <= 0 {
				t.Fatalf("wait = %v, want the rollout held for its holder", wait)
			}
			f.holderPod(f.holder(), true)
			prePull := f.prePull()
			if (prePull == nil) != f.refusePrePull {
				t.Fatalf("pre-pull = %v, refused %v", prePull, f.refusePrePull)
			}
			if tc.pull != nil {
				tc.pull(f, prePull)
			}
			if wait := f.apply(); wait != 0 {
				t.Fatalf("wait = %v, want the rollout to go on without the pull", wait)
			}
			f.expectImage("nginx:2", "without the pull")
			if got := diskPrePullCount(tc.result) - before; got != 1 {
				t.Errorf("%s count rose by %v, want 1", tc.result, got)
			}
			f.apply()
			if left := f.prePull() != nil; left != tc.left {
				t.Errorf("pre-pull left after the write: %v, want %v", left, tc.left)
			}
		})
	}
}

// A pull past its bound goes on in the kubelet, with the new instance's pull
// waiting behind it. Deleting the pre-pull would cancel it, so it ends once
// its pull has.
func TestDiskPrePullPastItsBoundEndsOnceItsPullHas(t *testing.T) {
	f := newDiskHoldFixture(t)
	f.image = "nginx:2"
	f.apply()
	f.holderPod(f.holder(), true)
	prePull := f.prePull()
	pod := f.prePullPod(prePull, pullingState)
	f.age(prePull, diskPrePullBound+time.Second)
	if wait := f.apply(); wait != 0 {
		t.Fatalf("wait = %v, want the template written at the bound", wait)
	}
	f.deleteInstance(f.instance)
	next := f.runInstance("new", "node-a", corev1.PodPending)
	f.mount(next)
	f.apply()
	if f.prePull() == nil {
		t.Fatal("the pre-pull ended while its pull ran: the new instance's pull waits for it")
	}
	f.endPull(pod, pulledState)
	f.apply()
	if f.prePull() != nil {
		t.Error("the pre-pull outlived its pull")
	}
}

func TestDiskPrePullIsStartedOnlyForANewImageOnAHeldNode(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*diskHoldFixture)
		held   bool
	}{
		{"a restart", func(f *diskHoldFixture) { f.app.Spec.RestartedAt = "2026-11-23T12:00:00Z" }, true},
		{"a plan change", func(f *diskHoldFixture) { f.app.Spec.Tier = "standard" }, true},
		{"a code deploy on a cordoned node", func(f *diskHoldFixture) { f.cordon(); f.image = "nginx:2" }, false},
		{"a code deploy onto another pool", func(f *diskHoldFixture) { f.app.Spec.Tier = "pro-plus"; f.image = "nginx:2" }, false},
		{"a code deploy with no running instance", func(f *diskHoldFixture) { f.deleteInstance(f.instance); f.image = "nginx:2" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDiskHoldFixture(t)
			tc.change(f)
			wait := f.apply()
			if f.prePull() != nil {
				t.Fatal("a pre-pull started")
			}
			if (f.holder() != nil) != tc.held || (wait > 0) != tc.held {
				t.Fatalf("holder %v, wait %v: want held %v", f.holder(), wait, tc.held)
			}
			if tc.held {
				f.holderPod(f.holder(), true)
				if wait := f.apply(); wait != 0 {
					t.Fatalf("wait = %v with the holder mounted, want the template written with no pull to wait for", wait)
				}
				if f.prePull() != nil {
					t.Error("a pre-pull started once the holder was mounted")
				}
			}
		})
	}
}

func TestDiskPrePullEndsWhenItsRolloutDoes(t *testing.T) {
	for _, tc := range []struct {
		name string
		// end ends the rollout, by a pass or by a call of its own.
		end func(*diskHoldFixture)
	}{
		{"the service is suspended", func(f *diskHoldFixture) { f.replicas = 0; f.apply() }},
		{"the node is cordoned", func(f *diskHoldFixture) { f.cordon(); f.apply() }},
		{"the disk is detached", func(f *diskHoldFixture) {
			f.r.releaseDiskHold(context.Background(), f.app, diskHoldEndStopped)
		}},
		{"the rollout outlived its budget", func(f *diskHoldFixture) {
			f.image = "nginx:1" // a cancel back to the running image
			f.age(f.prePull(), diskHoldMaxAge+time.Second)
			f.apply()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDiskHoldFixture(t)
			f.image = "nginx:2"
			f.apply()
			f.holderPod(f.holder(), true)
			f.prePullPod(f.prePull(), pullingState)
			if wait := f.apply(); wait <= 0 {
				t.Fatalf("wait = %v, want the rollout held for its pull", wait)
			}
			tc.end(f)
			if f.prePull() != nil {
				t.Error("the pre-pull is left with no rollout to pull for")
			}
		})
	}
}

// A second code deploy while the first one still waits for its pull pulls
// the second image instead.
func TestDiskPrePullFollowsTheImageBeingDeployed(t *testing.T) {
	f := newDiskHoldFixture(t)
	f.image = "nginx:2"
	f.apply()
	f.holderPod(f.holder(), true)
	f.prePullPod(f.prePull(), pullingState)
	f.apply()

	f.image = "nginx:3"
	if wait := f.apply(); wait <= 0 {
		t.Fatalf("wait = %v, want the second deploy held for its own pull", wait)
	}
	if got := f.prePull(); got == nil || !slices.Equal(diskPrePullImages(got), []string{"nginx:3"}) {
		t.Fatalf("pre-pull = %v, want one of nginx:3", got)
	}
	f.expectImage("nginx:1", "with the second image still pulling")
}

// A hold that falls back writes the template at once, and a pull already
// started goes on until it ends.
func TestDiskPrePullDoesNotHoldARolloutWhoseHoldFellBack(t *testing.T) {
	f := newDiskHoldFixture(t)
	f.image = "nginx:2"
	f.apply()
	pod := f.prePullPod(f.prePull(), pullingState)
	f.age(f.holder(), diskHoldMountBound+time.Second)
	if wait := f.apply(); wait != 0 {
		t.Fatalf("wait = %v, want the template written once the hold fell back", wait)
	}
	f.expectImage("nginx:2", "once the hold fell back")
	if f.prePull() == nil {
		t.Fatal("the pre-pull was cancelled mid-pull")
	}
	f.endPull(pod, pulledState)
	f.apply()
	if f.prePull() != nil {
		t.Error("the pre-pull outlived its pull")
	}
}

func TestDiskPrePullOfAnotherLifetimeIsLeftAlone(t *testing.T) {
	f := newDiskHoldFixture(t)
	refused := diskPrePullCount(diskPrePullRefused)
	other := diskPrePullFor(t, diskHoldApp(func(a *appv1alpha1.App) { a.UID = "earlier-uid" }), "starter", "nginx:0")
	if err := f.c.Create(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	f.image = "nginx:2"
	f.apply()
	f.holderPod(f.holder(), true)
	if wait := f.apply(); wait != 0 {
		t.Fatalf("wait = %v, want the template written without a pre-pull", wait)
	}
	if got := f.prePull(); got == nil || got.UID != other.UID {
		t.Error("another lifetime's Job was replaced")
	}
	if got := diskPrePullCount(diskPrePullRefused) - refused; got != 1 {
		t.Errorf("refused count rose by %v, want 1", got)
	}
}

// The pre-pull carries the App's label, so every operator reader that lists
// the App's pods by it sees the pre-pull too, ended in StartError by design.
// None may read it as an instance (t003).
func TestTheDiskPrePullIsNoInstanceOfTheRollout(t *testing.T) {
	scheme := diskHoldScheme(t)
	app := diskHoldApp()
	job := diskPrePullFor(t, app, "starter", "zot/blog@sha256:new")
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}}},
	}
	long := metav1.NewTime(time.Now().Add(-time.Hour))
	prePull := func(mutate func(*corev1.Pod)) *corev1.Pod {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-x", Namespace: app.Namespace, Labels: job.Spec.Template.Labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: "job-uid", Controller: new(true)}}},
			Spec: *job.Spec.Template.Spec.DeepCopy(),
		}
		mutate(pod)
		return pod
	}
	for name, pod := range map[string]*corev1.Pod{
		"ended in StartError": prePull(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-a"
			p.Status.Phase = corev1.PodFailed
			p.Status.ContainerStatuses = []corev1.ContainerStatus{pulledState}
		}),
		"backing off a pull": prePull(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-a"
			p.Status.Phase = corev1.PodPending
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: long}}
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "pull-0", Image: "zot/blog@sha256:new",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: imagePullBackOff, Message: "back-off"}}}}
		}),
		"pulling for long": prePull(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-a"
			p.Status.Phase = corev1.PodPending
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: long}}
			p.Status.ContainerStatuses = []corev1.ContainerStatus{pullingState}
		}),
		"unschedulable": prePull(func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodPending
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
				Reason: corev1.PodReasonUnschedulable, Message: "0/3 nodes are available", LastTransitionTime: long}}
		}),
		"lost": prePull(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-gone"
			p.Status.Phase = corev1.PodPending
		}),
	} {
		t.Run(name, func(t *testing.T) {
			r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build(), Scheme: scheme}
			ctx := context.Background()
			if reason, msg := r.stuckPodMessage(ctx, dep, 8080); msg != "" {
				t.Errorf("stuckPodMessage = %s: %q, want nothing: the pre-pull is no instance", reason, msg)
			}
			if msg := r.nodeLossMessage(ctx, dep); msg != "" {
				t.Errorf("nodeLossMessage = %q, want nothing: the pre-pull is no instance", msg)
			}
			if node := r.diskNode(ctx, app); node != "" {
				t.Errorf("diskNode = %q, want none: only an instance places the volume", node)
			}
			if servingInstance([]corev1.Pod{*pod}) != nil {
				t.Error("the pre-pull read as the instance a rollout replaces")
			}
		})
	}
}

func (f *diskHoldFixture) prePull() *batchv1.Job {
	job := &batchv1.Job{}
	if err := f.c.Get(context.Background(), client.ObjectKey{Namespace: f.app.Namespace, Name: diskPrePullName(f.app.Name)}, job); err != nil {
		if !apierrors.IsNotFound(err) {
			f.t.Fatal(err)
		}
		return nil
	}
	return job
}

// prePullPod is the Job controller's pod for job on node-a, its container in
// state.
func (f *diskHoldFixture) prePullPod(job *batchv1.Job, state corev1.ContainerStatus) *corev1.Pod {
	f.t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-p", Namespace: job.Namespace, Labels: job.Spec.Template.Labels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind("Job"))}},
		Spec: *job.Spec.Template.Spec.DeepCopy(),
	}
	pod.Spec.NodeName = "node-a"
	if err := f.c.Create(context.Background(), pod); err != nil {
		f.t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodPending
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{state}
	f.updateStatus(pod)
	return pod
}

// endPull is the kubelet posting pod's container in state, and the pod ended.
func (f *diskHoldFixture) endPull(pod *corev1.Pod, state corev1.ContainerStatus) {
	f.t.Helper()
	pod.Status.Phase = corev1.PodFailed
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{state}
	f.updateStatus(pod)
}

func (f *diskHoldFixture) expectImage(want, when string) {
	f.t.Helper()
	if got := appContainerImage(f.stored().Spec.Template); got != want {
		f.t.Fatalf("%s the template runs %s, want %s", when, got, want)
	}
}

func diskPrePullCount(result string) float64 {
	return testutil.ToFloat64(diskPrePullsTotal.WithLabelValues(result))
}
