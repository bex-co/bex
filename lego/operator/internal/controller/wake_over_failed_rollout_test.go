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
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m172. A free web service whose newest rollout failed over a
// served release kept the failed pod template on its Deployment. Awake, the old
// ReplicaSet's pod kept serving; once the service parked, both ReplicaSets sat at
// 0, and the next wake scaled the failed template, so no pod became ready, the
// public route stayed on the activator (503 "service hibernated") and the phase
// read Running from the desired scale.

const failingImage = "memcached:1.6-alpine"

// assertServedReleaseHeld checks the state a failed rollout over release 1 must
// leave: release 1's template, release 1 active, release 2's verdict kept.
func assertServedReleaseHeld(t *testing.T, cl client.Client, nn types.NamespacedName, served corev1.PodTemplateSpec, when string) {
	t.Helper()
	if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(got, served) {
		t.Fatalf("%s: pod template is not the served release's (revision %q, image %q); want revision %q, image %q",
			when, got.Labels[labelRevision], got.Spec.Containers[0].Image, served.Labels[labelRevision], served.Spec.Containers[0].Image)
	}
	live := liveApp(t, cl, nn)
	if live.Status.ActiveRevision != "rev-1" {
		t.Fatalf("%s: activeRevision = %q, want rev-1 — the failed release must never be promoted", when, live.Status.ActiveRevision)
	}
	rollout := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionRollout)
	if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.ObservedGeneration != 2 {
		t.Fatalf("%s: Rollout = %+v, want release 2's failed verdict kept", when, rollout)
	}
}

// The filed ordering minus the park: the rollout fails while the service is
// awake, the service then sleeps, and a request wakes it.
func TestWakeAfterFailedRolloutServesPriorRelease(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)

	// Release 2 rolls an image that never becomes healthy; the old pod serves on.
	deployImageAt(t, cl, nn, failingImage, 2)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now()))
	reconcileTwice(t, r, nn)
	assertServedReleaseHeld(t, cl, nn, served, "after the rollout failed")
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase after the failed rollout = %q, want Running on the prior release", got)
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend after the failed rollout = %q, want the App's own Service %q", got, app.Name)
	}

	parkIdle(t, r, cl, nn)
	assertServedReleaseHeld(t, cl, nn, served, "parked")
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("hibernated backend = %q, want the activator alias", got)
	}

	// A public request wakes it onto the served release.
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("woken replicas = %d, want 1", got)
	}
	assertServedReleaseHeld(t, cl, nn, served, "woken")
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("woken backend = %q, want the App's own Service %q once the prior release's pod is ready", got, app.Name)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("woken phase = %q, want Running", got)
	}
	assertServedReleaseHeld(t, cl, nn, served, "serving again")
}

// The observed ordering: the failing deploy starts while the service is
// hibernated. Scaling a Deployment up is not a rollout, so a failing template
// written onto the parked Deployment would be started by the wake with no progress
// deadline and no verdict: live, the service answered 503 for 22 minutes and
// counting. The parked Deployment keeps the served template instead, the wake
// starts the served release, and only then does the new release roll over it.
func TestFailedRolloutStartedWhileParkedServesPriorRelease(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	parkIdle(t, r, cl, nn)

	// Release 2 is deployed onto the parked service, which wakes it with no
	// request (w6/076): the served release starts first, behind the activator.
	deployImageAt(t, cl, nn, failingImage, 2)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("woken replicas = %d, want 1", got)
	}
	if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(got, served) {
		t.Fatalf("woken template = revision %q, want the served release's until its pod is ready", got.Labels[labelRevision])
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseDeploying {
		t.Fatalf("phase while the served release starts = %q, want Deploying", got)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("backend = %q, want the activator alias until a pod is ready", got)
	}

	// Its pod is ready: release 2 rolls over it, and fails.
	setDeploymentStatus(t, cl, nn, statusServedPodReady)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("template revision = %q, want release 2 rolling over the ready served pod", got)
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend = %q, want the App's own Service %q while the served pod is ready", got, app.Name)
	}
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now()))
	reconcileTwice(t, r, nn)

	assertServedReleaseHeld(t, cl, nn, served, "after the rollout failed")
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want Running", got)
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend = %q, want the App's own Service %q", got, app.Name)
	}
}

// rollFailingReleaseTwo serves release 1, then starts rolling release 2 over it
// with the served pod still ready, and returns release 1's template.
func rollFailingReleaseTwo(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) corev1.PodTemplateSpec {
	t.Helper()
	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	setDeploymentStatus(t, cl, nn, statusServedPodReady)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("setup: template revision = %q, want release 2 rolling", got)
	}
	return served
}

// rolloutPod is a pod of the Deployment's newest template in the given
// container state: what the rollout's stall scan reads.
func rolloutPod(t *testing.T, cl client.Client, nn types.NamespacedName, status corev1.ContainerStatus) {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	status.Name = appContainerName
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: nn.Name + "-new", Namespace: nn.Namespace, Labels: dep.Spec.Template.Labels},
		Spec:       *dep.Spec.Template.Spec.DeepCopy(),
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{status}},
	}
	if err := cl.Create(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

// w6/m147: the idle window equals the rollout budget, so with no traffic a park
// landed before the progress deadline, overwrote Ready with AutoHibernated, and
// the rollout never settled: the deploy closed with the generic health-gate line
// though the operator had named the crash or the failing probe. An idle App now
// stays up until its rollout has a verdict, which carries the diagnosis, and
// parks on the served release after it.
func TestIdleMidRolloutWaitsForItsVerdict(t *testing.T) {
	longAgo := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	notStarted := false
	for _, tc := range []struct {
		name   string
		status corev1.ContainerStatus
		want   string
	}{
		{"crash loop", corev1.ContainerStatus{
			State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 127}},
		}, "exit code 127"},
		{"tcp probe", corev1.ContainerStatus{
			Started: &notStarted,
			State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: longAgo}},
		}, "a TCP connect to port"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := activeApp("tea-m147")
			r, cl, nn := lifecycleFixture(t, app)

			served := rollFailingReleaseTwo(t, r, cl, nn)
			rolloutPod(t, cl, nn, tc.status)

			// Idle past the window mid-rollout: no park.
			stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
			reconcileTwice(t, r, nn)
			reconcileTwice(t, r, nn)
			if got := deploymentReplicas(t, cl, nn); got != 1 {
				t.Fatalf("replicas = %d, want the rollout kept running until its verdict", got)
			}
			if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
				t.Fatalf("template revision = %q, want release 2 still rolling", got)
			}
			if got := appPhase(t, cl, nn); got == appv1alpha1.PhaseHibernated {
				t.Fatal("parked mid-rollout")
			}

			// The deadline lands: the verdict carries the diagnosis, then it parks.
			setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now()))
			reconcileTwice(t, r, nn)
			reconcileTwice(t, r, nn)
			rollout := meta.FindStatusCondition(liveApp(t, cl, nn).Status.Conditions, appv1alpha1.ConditionRollout)
			if rollout == nil || rollout.ObservedGeneration != 2 || !strings.Contains(rollout.Message, tc.want) {
				t.Fatalf("Rollout = %+v, want release 2's verdict naming %q", rollout, tc.want)
			}
			if got := deploymentReplicas(t, cl, nn); got != 0 {
				t.Fatalf("replicas = %d, want parked once the rollout settled", got)
			}
			if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
				t.Fatalf("phase = %q, want Hibernated after the verdict", got)
			}
			assertServedReleaseHeld(t, cl, nn, served, "parked after the verdict")
		})
	}
}

// The deferral is bounded: a deadline that passed rolloutVerdictGrace ago with
// no verdict recorded no longer keeps a free service awake, and it parks on the
// served release.
func TestIdleMidRolloutDeferralIsBounded(t *testing.T) {
	app := activeApp("tea-m147")
	r, cl, nn := lifecycleFixture(t, app)

	served := rollFailingReleaseTwo(t, r, cl, nn)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now().Add(-rolloutVerdictGrace-time.Minute)))
	// Release 2 already had its one wake (w6/076): only the deferral could keep
	// the service up.
	live := liveApp(t, cl, nn)
	live.Annotations[annotReleaseWakeGeneration] = "2"
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want parked: a deadline past the grace with no verdict no longer defers the park", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want Hibernated", got)
	}
	if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(got, served) {
		t.Fatalf("parked template = revision %q, want the served release's", got.Labels[labelRevision])
	}
}

// A manual suspend still parks mid-rollout: the served template goes back so a
// resume does not start the unsettled release alone (w1/m172).
func TestSuspendMidRolloutPutsServedTemplateBack(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)

	served := rollFailingReleaseTwo(t, r, cl, nn)

	live := liveApp(t, cl, nn)
	live.Spec.Suspended = true
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want 0 once suspended", got)
	}
	if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(got, served) {
		t.Fatalf("parked template = revision %q, want the served release's", got.Labels[labelRevision])
	}
	if live := liveApp(t, cl, nn); live.Status.ActiveRevision != "rev-1" {
		t.Fatalf("activeRevision = %q, want rev-1", live.Status.ActiveRevision)
	}
}

// wakeOntoServedRelease serves release 1, parks it, and deploys release 2, which
// wakes the service onto release 1's template first: the served release starts
// before the newer one rolls.
func wakeOntoServedRelease(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	serveReleaseOne(t, r, cl, nn)
	parkIdle(t, r, cl, nn)
	deployImageAt(t, cl, nn, "nginx:2", 2)
	reconcileTwice(t, r, nn)
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-1" {
		t.Fatalf("setup: template revision = %q, want the served release starting first", got)
	}
}

// A served release that can no longer start must not block the release that
// might fix it: once it has been unavailable for servedWakeBudget the new release
// rolls anyway. The wait counts from when the Deployment lost availability, so a
// pod that was just replaced does not restart it (w5/m123).
func TestWakeRollsNewReleaseWhenServedPodStaysUnready(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)
	wakeOntoServedRelease(t, r, cl, nn)

	setDeploymentStatus(t, cl, nn, statusUnavailableSince(time.Now().Add(-time.Minute), ""))
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-1" {
		t.Fatalf("template revision = %q, want the served release given its budget", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseDeploying {
		t.Fatalf("phase while the served release starts = %q, want Deploying", got)
	}

	rolloutPod(t, cl, nn, corev1.ContainerStatus{}) // replaced just now
	setDeploymentStatus(t, cl, nn, statusUnavailableSince(time.Now().Add(-servedWakeBudget-time.Minute), ""))
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("template revision = %q, want release 2 to roll once the served release overran its budget", got)
	}
}

// Pods the served release's ReplicaSet cannot create (the workspace quota, the
// image-signature check after a key rotation) never exist to age. Ready names
// the cause while the newer release waits, and the wait ends with the budget
// (w5/m123).
func TestServedReleaseThatCannotCreatePodsIsDiagnosedThenRolledOver(t *testing.T) {
	app := activeApp("tea-m123")
	r, cl, nn := lifecycleFixture(t, app)
	wakeOntoServedRelease(t, r, cl, nn)

	const forbidden = `pods "web-5d8f9-" is forbidden: exceeded quota: tenant-quota, requested: pods=1, used: pods=20, limited: pods=20`
	setDeploymentStatus(t, cl, nn, statusUnavailableSince(time.Now().Add(-time.Minute), forbidden))
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-1" {
		t.Fatalf("template revision = %q, want the served release given its budget", got)
	}
	ready := meta.FindStatusCondition(liveApp(t, cl, nn).Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != appv1alpha1.ReasonServedReleaseCannotStart || !strings.Contains(ready.Message, forbidden) {
		t.Fatalf("Ready = %+v, want %s naming the ReplicaSet's failure", ready, appv1alpha1.ReasonServedReleaseCannotStart)
	}

	setDeploymentStatus(t, cl, nn, statusUnavailableSince(time.Now().Add(-servedWakeBudget-time.Minute), forbidden))
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("template revision = %q, want release 2 to roll once the budget passed", got)
	}
}

// The wait is a paid service's too: a newer release, perhaps the hotfix, waits
// for a service with no ready pod at most servedWakeBudget after it lost
// availability, however young its pod (w5/m123).
func TestHotfixWaitsAtMostTheBudgetFromLostAvailability(t *testing.T) {
	app := activeApp("tea-m123")
	app.Spec.Tier = "starter"
	r, cl, nn := lifecycleFixture(t, app)
	serveReleaseOne(t, r, cl, nn)

	setDeploymentStatus(t, cl, nn, statusUnavailableSince(time.Now().Add(-time.Minute), ""))
	deployImageAt(t, cl, nn, "nginx:2", 2)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-1" {
		t.Fatalf("template revision = %q, want the hotfix to wait while the served release has its budget", got)
	}

	rolloutPod(t, cl, nn, corev1.ContainerStatus{}) // replaced just now
	setDeploymentStatus(t, cl, nn, statusUnavailableSince(time.Now().Add(-servedWakeBudget-time.Minute), ""))
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("template revision = %q, want the hotfix to roll once the service was unavailable past the budget", got)
	}
}

// The hold ends with the next release: a later deploy rolls and is promoted.
func TestDeployAfterFailedRolloutRollsNormally(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now()))
	reconcileTwice(t, r, nn)
	assertServedReleaseHeld(t, cl, nn, served, "after the rollout failed")

	deployImageAt(t, cl, nn, "nginx:2", 3)
	setDeploymentStatus(t, cl, nn, statusServedPodReady) // the served pod serves; release 3's is not up yet
	reconcileTwice(t, r, nn)
	got := deploymentTemplate(t, cl, nn)
	if got.Labels[labelRevision] != "rev-3" || got.Spec.Containers[0].Image != "nginx:2" {
		t.Fatalf("template = revision %q image %q, want release 3 (nginx:2) to roll", got.Labels[labelRevision], got.Spec.Containers[0].Image)
	}
	if phase := appPhase(t, cl, nn); phase != appv1alpha1.PhaseDeploying {
		t.Fatalf("phase while release 3 rolls = %q, want Deploying", phase)
	}
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live := liveApp(t, cl, nn)
	if live.Status.ActiveRevision != "rev-3" || live.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("after release 3: phase %q activeRevision %q, want Running on rev-3", live.Status.Phase, live.Status.ActiveRevision)
	}
}

func deleteServedRecord(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName(nn.Name, 1), Namespace: nn.Namespace}}
	if err := cl.Delete(context.Background(), rec); err != nil {
		t.Fatalf("setup: release 1 has no recorded template to delete: %v", err)
	}
}

// A served release with no record (it predates w1/m152, or GC reclaimed it) is
// restored from the ReplicaSet the Deployment still retains for it.
func TestFailedRolloutWithoutRecordRestoresFromServedReplicaSet(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)
	ctx := context.Background()

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	var dep appsv1.Deployment
	if err := cl.Get(ctx, nn, &dep); err != nil {
		t.Fatal(err)
	}
	controller := true
	replicaSet := func(name, revision string, tmpl corev1.PodTemplateSpec) *appsv1.ReplicaSet {
		tmpl = *tmpl.DeepCopy()
		tmpl.Labels[appsv1.DefaultDeploymentUniqueLabelKey] = name
		return &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: nn.Namespace, Labels: tmpl.Labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: dep.Name, UID: dep.UID, Controller: &controller}},
				Annotations:     map[string]string{deploymentRevisionAnnotation: revision},
			},
			Spec: appsv1.ReplicaSetSpec{Selector: dep.Spec.Selector, Template: tmpl},
		}
	}
	// An older ReplicaSet of the same release (before a re-projection) must lose to the
	// newest one, and another Deployment's must be ignored.
	stale := *served.DeepCopy()
	stale.Annotations = map[string]string{"app.bex.co/restarted-at": "earlier"}
	foreign := replicaSet("web-foreign", "9", stale)
	foreign.OwnerReferences[0].UID = "someone-else"
	for _, rs := range []*appsv1.ReplicaSet{replicaSet("web-old", "1", stale), replicaSet("web-served", "2", served), foreign} {
		if err := cl.Create(ctx, rs); err != nil {
			t.Fatal(err)
		}
	}

	deployImageAt(t, cl, nn, failingImage, 2)
	deleteServedRecord(t, cl, nn)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now()))
	reconcileTwice(t, r, nn)

	assertServedReleaseHeld(t, cl, nn, served, "restored from the ReplicaSet")
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want Running on the prior release", got)
	}
}

// With neither a record nor a ReplicaSet there is nothing that can serve, and
// the phase must say so instead of reading Running from the desired scale.
func TestFailedRolloutWithNothingToRestoreSettlesFailed(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	setDeploymentStatus(t, cl, nn, statusServedPodReady)
	reconcileTwice(t, r, nn) // release 2 rolls over the served pod
	deleteServedRecord(t, cl, nn)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(0, time.Now()))
	reconcileTwice(t, r, nn)

	live := liveApp(t, cl, nn)
	if live.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want Failed: the Deployment runs the failed template and nothing is ready", live.Status.Phase)
	}
	ready := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason == appv1alpha1.ReasonPriorReleaseServing {
		t.Fatalf("Ready = %+v, want False with the rollout's own reason", ready)
	}
	rollout := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionRollout)
	if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.ObservedGeneration != 2 {
		t.Fatalf("Rollout = %+v, want release 2's failed verdict for the deploy row", rollout)
	}
	if live.Status.ActiveRevision != "rev-1" {
		t.Fatalf("activeRevision = %q, want rev-1 kept", live.Status.ActiveRevision)
	}
}

// Resume is the same wake by a different door, and a background worker has no
// route at all: its replicas alone must come back on the served release.
func TestSuspendAndResumeWorkerOverFailedRolloutKeepsPriorRelease(t *testing.T) {
	app := heldWorkerApp("tea-m172")
	r, cl, nn := lifecycleFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(1, time.Now()))
	reconcileTwice(t, r, nn)
	assertServedReleaseHeld(t, cl, nn, served, "after the rollout failed")

	setSuspendedAt(t, cl, nn, true, 3)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("suspended worker replicas = %d, want 0", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("suspended worker phase = %q, want Hibernated", got)
	}

	setSuspendedAt(t, cl, nn, false, 4)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("resumed worker replicas = %d, want 1", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("resumed worker phase = %q, want Running", got)
	}
	assertServedReleaseHeld(t, cl, nn, served, "resumed")
}
