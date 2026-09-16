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
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m156. A free web service whose newest release failed its
// pre-deploy step kept serving its prior release, went idle and hibernated, and
// then never woke: the stored failed verdict halted every reconcile before the
// Deployment and Ingress writes, so replicas stayed 0 and the public route stayed
// on the activator while the service read Running.

// markDeploymentRolledOut reports the Deployment's current revision fully rolled
// out and ready, the state deploymentRolloutReady waits for before markRunning
// records a release as active. markDeploymentReady stops short of that (no
// updated replicas), which is enough for routing but never activates a release.
func markDeploymentRolledOut(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	dep.Status.ObservedGeneration = dep.Generation
	dep.Status.Replicas = 1
	dep.Status.UpdatedReplicas = 1
	dep.Status.ReadyReplicas = 1
	dep.Status.AvailableReplicas = 1
	if err := cl.Status().Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
}

func stampLastActiveAt(t *testing.T, cl client.Client, nn types.NamespacedName, at time.Time) {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Annotations[annotLastActive] = at.UTC().Format(time.RFC3339)
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
}

func appPhase(t *testing.T, cl client.Client, nn types.NamespacedName) appv1alpha1.AppPhase {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	return live.Status.Phase
}

func deploymentTemplateRevision(t *testing.T, cl client.Client, nn types.NamespacedName) string {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	return dep.Spec.Template.Labels[labelRevision]
}

<<<<<<< Updated upstream
// serveReleaseOne reconciles the fixture's prebuilt release until it is the
// active release, and returns its pod template revision and image.
func serveReleaseOne(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) (string, string) {
	t.Helper()
	reconcileTwice(t, r, nn)
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	if live.Status.ActiveRevision == "" || live.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("setup: release 1 never became active (phase %q, activeRevision %q)", live.Status.Phase, live.Status.ActiveRevision)
	}
	return deploymentTemplateRevision(t, cl, nn), live.Status.Image
}

// jobsIn lists the Jobs in namespace: pre-deploy steps in these tests.
func jobsIn(t *testing.T, cl client.Client, namespace string) []batchv1.Job {
	t.Helper()
	var jobs batchv1.JobList
	if err := cl.List(context.Background(), &jobs, client.InNamespace(namespace)); err != nil {
		t.Fatal(err)
	}
	return jobs.Items
}

// heldWorkerApp is a background worker: no Service, Ingress or auto-sleep, and
// no plan instance cap, so a manual scale takes effect.
func heldWorkerApp(namespace string) *appv1alpha1.App {
	app := activeApp(namespace)
	app.Spec.Type = appv1alpha1.TypeBackgroundWorker
	app.Spec.Tier = ""
	app.Spec.Expose = false
	return app
}

// storeFailedPreDeploy makes release 2 add a pre-deploy command whose step
// failed, with the verdict stored against that release.
func storeFailedPreDeploy(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	ctx := context.Background()
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.PreDeployCommand = "exit 3"
	live.Generation = 2
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Job: predeploy.JobName(nn.Name, appv1alpha1.BuildRevision(2)), Generation: 2, Status: appv1alpha1.PreDeployFailed, Message: exitThree,
	}
	if err := cl.Status().Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
}

func setSuspendedAt(t *testing.T, cl client.Client, nn types.NamespacedName, suspended bool, generation int64) {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Suspended = suspended
	live.Generation = generation
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
}

=======
>>>>>>> Stashed changes
func TestWakeOverFailedPreDeployRestoresPriorRelease(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m156")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	// Release 1 serves.
<<<<<<< Updated upstream
	priorRevision, _ := serveReleaseOne(t, r, cl, nn)

	// A config change adds a failing pre-deploy command as release 2, and the
	// step's failed verdict is stored against that release.
	storeFailedPreDeploy(t, cl, nn)
=======
	reconcileTwice(t, r, nn)
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	if live.Status.ActiveRevision == "" || live.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("setup: release 1 never became active (phase %q, activeRevision %q)", live.Status.Phase, live.Status.ActiveRevision)
	}
	priorRevision := deploymentTemplateRevision(t, cl, nn)

	// A config change adds a failing pre-deploy command as release 2, and the
	// step's failed verdict is stored against that release.
	live.Spec.PreDeployCommand = "exit 3"
	live.Generation = 2
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Job: predeploy.JobName(app.Name, appv1alpha1.BuildRevision(2)), Generation: 2, Status: appv1alpha1.PreDeployFailed, Message: exitThree,
	}
	if err := cl.Status().Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
>>>>>>> Stashed changes
	reconcileTwice(t, r, nn)

	// It goes idle: the route moves to the activator, then the pods drain. The
	// pre-deploy gate is skipped while auto-hibernating, so this half always worked.
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("setup: replicas = %d, want 0 once hibernated", got)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("setup: hibernated backend = %q, want the activator alias", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("parked phase = %q, want Hibernated", got)
	}

	// A public request wakes it: the activator stamps last-active.
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("woken replicas = %d, want 1: the stored failed pre-deploy verdict must not keep a woken service at 0", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase on the wake pass = %q, want Running: settle from the scale this pass wrote", got)
	}
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("woken backend = %q, want the App's own Service %q once a pod is ready", got, app.Name)
	}

	// The prior release serves; the failed release never rolled and never re-ran.
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("pod template revision = %q, want the prior release's %q — the failed release must not roll", got, priorRevision)
	}
<<<<<<< Updated upstream
	if jobs := jobsIn(t, cl, app.Namespace); len(jobs) != 0 {
		t.Fatalf("jobs = %d, want none — a stored verdict must not re-run the pre-deploy command", len(jobs))
	}
	var live appv1alpha1.App
=======
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs, client.InNamespace(app.Namespace)); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("jobs = %d, want none — a stored verdict must not re-run the pre-deploy command", len(jobs.Items))
	}
>>>>>>> Stashed changes
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	if pd := live.Status.PreDeploy; pd == nil || pd.Status != appv1alpha1.PreDeployFailed || pd.Generation != 2 {
		t.Fatalf("status.preDeploy = %+v, want release 2's failed verdict kept", pd)
	}
}

// A release whose pre-deploy step lands while the service is parked must not
// reach the pod template before the step runs: before w1/m156 the parking pass
// skipped the gate and wrote the unmigrated release onto the parked Deployment,
// so the next wake started it. After the fix the wake starts the step while the
// prior release serves, and the new release rolls only once the step passes.
func TestParkedPendingPreDeployServesPriorReleaseUntilTheStepPasses(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m156")
	app.UID = "uid-web" // the fake client assigns none; the pre-deploy Job is keyed on it
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
<<<<<<< Updated upstream

	// Release 1 serves, then goes idle and parks.
	priorRevision, _ := serveReleaseOne(t, r, cl, nn)
=======
	listJobs := func() []batchv1.Job {
		t.Helper()
		var jobs batchv1.JobList
		if err := cl.List(ctx, &jobs, client.InNamespace(app.Namespace)); err != nil {
			t.Fatal(err)
		}
		return jobs.Items
	}

	// Release 1 serves, then goes idle and parks.
	reconcileTwice(t, r, nn)
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	priorRevision := deploymentTemplateRevision(t, cl, nn)
>>>>>>> Stashed changes
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("setup: replicas = %d, want 0 once parked", got)
	}

	// Release 2 adds a pre-deploy command while parked.
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.PreDeployCommand = "echo migrate"
	live.Generation = 2
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("parked template revision = %q, want the prior release's %q — an unmigrated release must not be parked into the template", got, priorRevision)
	}
<<<<<<< Updated upstream
	if jobs := jobsIn(t, cl, app.Namespace); len(jobs) != 0 {
=======
	if jobs := listJobs(); len(jobs) != 0 {
>>>>>>> Stashed changes
		t.Fatalf("jobs = %d, want none while parked", len(jobs))
	}

	// A request wakes it: the step starts and the prior release serves meanwhile.
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("woken replicas = %d, want 1 while the pre-deploy step runs", got)
	}
<<<<<<< Updated upstream
	jobs := jobsIn(t, cl, app.Namespace)
=======
	jobs := listJobs()
>>>>>>> Stashed changes
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want the release 2 pre-deploy Job", len(jobs))
	}
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend = %q, want the prior release's Service %q while the step runs", got, app.Name)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("template revision = %q while the step runs, want the prior release's %q", got, priorRevision)
	}

	// The step passes: release 2 rolls.
	job := jobs[0]
	job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue})
	if err := cl.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got == priorRevision {
		t.Fatalf("template revision = %q, want release 2 to roll once its step passed", got)
	}
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	if pd := live.Status.PreDeploy; pd == nil || pd.Status != appv1alpha1.PreDeploySucceeded || pd.Generation != 2 {
		t.Fatalf("status.preDeploy = %+v, want release 2's succeeded step", pd)
	}
}

// Resume is the same wake by a different door: before w1/m156 a service
// suspended while a failed pre-deploy verdict stood could not be resumed,
// because the stored verdict halted the reconcile before the replicas were
// restored. Suspend and resume now move only replicas and routing, and the
// prior release's template stays.
func TestSuspendAndResumeOverFailedPreDeployKeepPriorRelease(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m156")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
<<<<<<< Updated upstream
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	// Release 1 serves; release 2's pre-deploy step failed.
	priorRevision, _ := serveReleaseOne(t, r, cl, nn)
	storeFailedPreDeploy(t, cl, nn)
	reconcileTwice(t, r, nn)

	// Suspend: scaled to 0 on the prior release's template.
	setSuspendedAt(t, cl, nn, true, 3)
=======
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	setSuspended := func(suspended bool, generation int64) {
		t.Helper()
		var live appv1alpha1.App
		if err := cl.Get(ctx, nn, &live); err != nil {
			t.Fatal(err)
		}
		live.Spec.Suspended = suspended
		live.Generation = generation
		if err := cl.Update(ctx, &live); err != nil {
			t.Fatal(err)
		}
	}

	// Release 1 serves; release 2's pre-deploy step failed.
	reconcileTwice(t, r, nn)
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	priorRevision := deploymentTemplateRevision(t, cl, nn)
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.PreDeployCommand = "exit 3"
	live.Generation = 2
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Job: predeploy.JobName(app.Name, appv1alpha1.BuildRevision(2)), Generation: 2, Status: appv1alpha1.PreDeployFailed, Message: exitThree,
	}
	if err := cl.Status().Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)

	// Suspend: scaled to 0 on the prior release's template.
	setSuspended(true, 3)
>>>>>>> Stashed changes
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("suspended replicas = %d, want 0", got)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("suspended template revision = %q, want the prior release's %q", got, priorRevision)
	}

	// Resume: replicas come back, and the route follows once a pod is ready.
<<<<<<< Updated upstream
	setSuspendedAt(t, cl, nn, false, 4)
=======
	setSuspended(false, 4)
>>>>>>> Stashed changes
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("resumed replicas = %d, want 1: a failed pre-deploy verdict must not block resume", got)
	}
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("resumed backend = %q, want the App's own Service %q", got, app.Name)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("resumed template revision = %q, want the prior release's %q", got, priorRevision)
	}
<<<<<<< Updated upstream
	if jobs := jobsIn(t, cl, app.Namespace); len(jobs) != 0 {
		t.Fatalf("jobs = %d, want none — resume must not re-run a failed step", len(jobs))
	}
}

// A background worker's replicas follow suspend and resume too. Before w1/m158
// suspending one still parked it (the gate is skipped while suspended), but that
// pass also wrote the unmigrated release onto the parked template, and a resume
// waited behind the stored failed verdict and stayed at 0 until a new release
// shipped.
func TestSuspendAndResumeWorkerOverFailedPreDeployKeepPriorRelease(t *testing.T) {
	scheme := wakeScheme()
	app := heldWorkerApp("tea-m158")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	priorRevision, _ := serveReleaseOne(t, r, cl, nn)
	storeFailedPreDeploy(t, cl, nn)
	reconcileTwice(t, r, nn)

	setSuspendedAt(t, cl, nn, true, 3)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("suspended worker replicas = %d, want 0: a failed pre-deploy verdict must not keep a suspended worker running", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("suspended worker phase = %q, want Hibernated", got)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("suspended worker template revision = %q, want the prior release's %q — an unmigrated release must not be parked into the template", got, priorRevision)
	}

	setSuspendedAt(t, cl, nn, false, 4)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("resumed worker replicas = %d, want 1: a failed pre-deploy verdict must not keep a resumed worker at 0", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("resumed worker phase = %q, want Running", got)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("worker template revision = %q, want the prior release's %q", got, priorRevision)
	}
	if jobs := jobsIn(t, cl, app.Namespace); len(jobs) != 0 {
		t.Fatalf("jobs = %d, want none — resume must not re-run a failed step", len(jobs))
	}
}

// A worker's manual scale while its newer release's pre-deploy step runs takes
// effect on the prior release, and the new release rolls once the step passes.
func TestWorkerScaleWhilePreDeployRunsTakesEffect(t *testing.T) {
	scheme := wakeScheme()
	app := heldWorkerApp("tea-m158")
	app.UID = "uid-worker" // the fake client assigns none; the pre-deploy Job is keyed on it
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	priorRevision, _ := serveReleaseOne(t, r, cl, nn)
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.PreDeployCommand = "echo migrate"
	live.Generation = 2
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	jobs := jobsIn(t, cl, app.Namespace)
	if len(jobs) != 1 {
		t.Fatalf("setup: jobs = %d, want release 2's running pre-deploy Job", len(jobs))
	}

	// Scale to 2 while the step runs.
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Replicas = 2
	live.Generation = 3
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 2 {
		t.Fatalf("worker replicas while the step runs = %d, want 2: a manual scale must not wait for the pre-deploy step", got)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("worker template revision while the step runs = %q, want the prior release's %q", got, priorRevision)
	}

	// The step passes: release 2 rolls at the scaled count.
	job := jobs[0]
	job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue})
	if err := cl.Status().Update(ctx, &job); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got == priorRevision {
		t.Fatalf("worker template revision = %q, want release 2 to roll once its step passed", got)
	}
	if got := deploymentReplicas(t, cl, nn); got != 2 {
		t.Fatalf("worker replicas after the rollout = %d, want 2", got)
=======
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs, client.InNamespace(app.Namespace)); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("jobs = %d, want none — resume must not re-run a failed step", len(jobs.Items))
>>>>>>> Stashed changes
	}
}
