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
	"strconv"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m172. A free web service whose newest rollout failed over a
// served release kept the failed pod template on its Deployment. Awake, the old
// ReplicaSet's pod kept serving; once the service parked, both ReplicaSets sat at
// 0, and the next wake scaled the failed template, so no pod became ready, the
// public route stayed on the activator (503 "service hibernated") and the phase
// read Running from the desired scale.

const failingImage = "memcached:1.6-alpine"

// deployImageAt requests a new release that changes the image.
func deployImageAt(t *testing.T, cl client.Client, nn types.NamespacedName, image string, generation int64) {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Image = image
	live.Generation = generation
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(generation, 10)
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
}

// markRolloutFailed reports the Deployment's newest template past its progress
// deadline with none of its pods updated, and ready pods of the old ReplicaSet:
// 1 while the service stayed awake through the rollout, 0 when it rolled from
// parked. The fake client runs no Deployment controller.
func markRolloutFailed(t *testing.T, cl client.Client, nn types.NamespacedName, ready int32) {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	dep.Status = appsv1.DeploymentStatus{
		ObservedGeneration: dep.Generation, Replicas: ready, ReadyReplicas: ready, AvailableReplicas: ready,
		Conditions: []appsv1.DeploymentCondition{{
			Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
		}},
	}
	if err := cl.Status().Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
}

// markServedPodReady reports one ready pod that is not of the Deployment's newest
// template: the served release's pod while a newer release rolls, or once it
// alone has been started.
func markServedPodReady(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	if err := cl.Status().Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
}

// markDeploymentDrained reports the parked Deployment with no pods.
func markDeploymentDrained(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation}
	if err := cl.Status().Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
}

func deploymentTemplate(t *testing.T, cl client.Client, nn types.NamespacedName) corev1.PodTemplateSpec {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	return dep.Spec.Template
}

func liveApp(t *testing.T, cl client.Client, nn types.NamespacedName) appv1alpha1.App {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	return live
}

// parkIdle lets the service go idle: the route moves to the activator, then the
// pods drain.
func parkIdle(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("setup: replicas = %d, want 0 once hibernated", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("parked phase = %q, want Hibernated", got)
	}
	markDeploymentDrained(t, cl, nn)
}

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

func failedRolloutFixture(t *testing.T, app *appv1alpha1.App) (*AppReconciler, client.Client, types.NamespacedName) {
	t.Helper()
	// The fake client assigns no generation, and a release is recorded under its
	// own: start at 1 so release 1 has a record to restore.
	app.Generation = 1
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	return wakeReconciler(cl, scheme), cl, types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
}

// The filed ordering minus the park: the rollout fails while the service is
// awake, the service then sleeps, and a request wakes it.
func TestWakeAfterFailedRolloutServesPriorRelease(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := failedRolloutFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)

	// Release 2 rolls an image that never becomes healthy; the old pod serves on.
	deployImageAt(t, cl, nn, failingImage, 2)
	markRolloutFailed(t, cl, nn, 1)
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
	markDeploymentRolledOut(t, cl, nn)
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
	r, cl, nn := failedRolloutFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	parkIdle(t, r, cl, nn)

	// Release 2 is deployed onto the parked service: it stays off the template.
	deployImageAt(t, cl, nn, failingImage, 2)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(got, served) {
		t.Fatalf("parked template = revision %q, want the served release's: an unserved release must not be parked into the template", got.Labels[labelRevision])
	}
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("parked replicas = %d, want 0", got)
	}

	// A request wakes it: the served release starts first, behind the activator.
	stampLastActiveAt(t, cl, nn, time.Now())
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
	markServedPodReady(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplate(t, cl, nn).Labels[labelRevision]; got != "rev-2" {
		t.Fatalf("template revision = %q, want release 2 rolling over the ready served pod", got)
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend = %q, want the App's own Service %q while the served pod is ready", got, app.Name)
	}
	markRolloutFailed(t, cl, nn, 1)
	reconcileTwice(t, r, nn)

	assertServedReleaseHeld(t, cl, nn, served, "after the rollout failed")
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want Running", got)
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend = %q, want the App's own Service %q", got, app.Name)
	}
}

// A park that lands while a newer release is still rolling takes that release off
// the template too, so the next wake cannot start it alone.
func TestParkMidRolloutPutsServedTemplateBack(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := failedRolloutFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	markServedPodReady(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := deploymentTemplate(t, cl, nn).Labels[labelRevision]; got != "rev-2" {
		t.Fatalf("setup: template revision = %q, want release 2 rolling", got)
	}

	parkIdle(t, r, cl, nn)
	if got := deploymentTemplate(t, cl, nn); !equality.Semantic.DeepEqual(got, served) {
		t.Fatalf("parked template = revision %q, want the served release's", got.Labels[labelRevision])
	}
	if live := liveApp(t, cl, nn); live.Status.ActiveRevision != "rev-1" {
		t.Fatalf("activeRevision = %q, want rev-1", live.Status.ActiveRevision)
	}
}

// A served release that can no longer start must not block the release that
// might fix it: past servedWakeBudget the new release rolls anyway.
func TestWakeRollsNewReleaseWhenServedPodStaysUnready(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := failedRolloutFixture(t, app)
	ctx := context.Background()

	serveReleaseOne(t, r, cl, nn)
	parkIdle(t, r, cl, nn)
	deployImageAt(t, cl, nn, "nginx:2", 2)
	reconcileTwice(t, r, nn)
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentTemplate(t, cl, nn).Labels[labelRevision]; got != "rev-1" {
		t.Fatalf("setup: template revision = %q, want the served release starting first", got)
	}

	var dep appsv1.Deployment
	if err := cl.Get(ctx, nn, &dep); err != nil {
		t.Fatal(err)
	}
	stuck := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "web-stuck", Namespace: nn.Namespace, Labels: dep.Spec.Template.Labels,
		CreationTimestamp: metav1.NewTime(time.Now().Add(-servedWakeBudget - time.Minute)),
	}}
	if err := cl.Create(ctx, stuck); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentTemplate(t, cl, nn).Labels[labelRevision]; got != "rev-2" {
		t.Fatalf("template revision = %q, want release 2 to roll once the served pod overran its budget", got)
	}
}

// The hold ends with the next release: a later deploy rolls and is promoted.
func TestDeployAfterFailedRolloutRollsNormally(t *testing.T) {
	app := activeApp("tea-m172")
	r, cl, nn := failedRolloutFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	markRolloutFailed(t, cl, nn, 1)
	reconcileTwice(t, r, nn)
	assertServedReleaseHeld(t, cl, nn, served, "after the rollout failed")

	deployImageAt(t, cl, nn, "nginx:2", 3)
	markServedPodReady(t, cl, nn) // the served pod serves; release 3's is not up yet
	reconcileTwice(t, r, nn)
	got := deploymentTemplate(t, cl, nn)
	if got.Labels[labelRevision] != "rev-3" || got.Spec.Containers[0].Image != "nginx:2" {
		t.Fatalf("template = revision %q image %q, want release 3 (nginx:2) to roll", got.Labels[labelRevision], got.Spec.Containers[0].Image)
	}
	if phase := appPhase(t, cl, nn); phase != appv1alpha1.PhaseDeploying {
		t.Fatalf("phase while release 3 rolls = %q, want Deploying", phase)
	}
	markDeploymentRolledOut(t, cl, nn)
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
	r, cl, nn := failedRolloutFixture(t, app)
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
	markRolloutFailed(t, cl, nn, 1)
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
	r, cl, nn := failedRolloutFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	markServedPodReady(t, cl, nn)
	reconcileTwice(t, r, nn) // release 2 rolls over the served pod
	deleteServedRecord(t, cl, nn)
	markRolloutFailed(t, cl, nn, 0)
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
	r, cl, nn := failedRolloutFixture(t, app)

	serveReleaseOne(t, r, cl, nn)
	served := deploymentTemplate(t, cl, nn)
	deployImageAt(t, cl, nn, failingImage, 2)
	markRolloutFailed(t, cl, nn, 1)
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
