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
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m157. While a newer release had no image yet (its build
// failed, or was still queued or running), buildFromSource halted every
// reconcile before the runtime, so the serving release could not resume, sleep,
// wake or scale. On production a resume over a failed build answered 503 service
// hibernated for minutes while the service read Running.

// releaseTwoFromSource makes release 2 a repo build: it has no image until its
// build succeeds.
func releaseTwoFromSource(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Image = ""
	live.Spec.Repo = "https://example.invalid/repo.git"
	live.Generation = 2
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
}

// storeReleaseTwoBuildFailure records release 2's failed build in the named
// condition: Build, attributed to the release generation, is how r.fail leaves
// it; Ready alone is the marker an operator older than w6/m100 left.
func storeReleaseTwoBuildFailure(t *testing.T, cl client.Client, nn types.NamespacedName, conditionType string) {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	meta.SetStatusCondition(&live.Status.Conditions, metav1.Condition{
		Type: conditionType, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonBuildFailedUserError,
		Message: errBuildBroke.Error(), ObservedGeneration: 2,
	})
	if err := cl.Status().Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}
}

func assertNoBuildJobs(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	var jobs batchv1.JobList
	if err := cl.List(context.Background(), &jobs, client.InNamespace(nn.Namespace), client.MatchingLabels{"app.bex.co/build": nn.Name}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("build jobs = %d, want none — a recorded build failure must not be dispatched again", len(jobs.Items))
	}
}

// assertPriorReleaseKept checks that the unbuilt release never reached the pod:
// the template and status.image are release 1's, release 2's failed verdict is
// still stored, and no build Job was dispatched for it.
func assertPriorReleaseKept(t *testing.T, cl client.Client, nn types.NamespacedName, revision, image string) {
	t.Helper()
	if got := deploymentTemplateRevision(t, cl, nn); got != revision {
		t.Fatalf("pod template revision = %q, want the prior release's %q — the unbuilt release must not roll", got, revision)
	}
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	if live.Status.Image != image {
		t.Fatalf("status.image = %q, want the serving release's %q", live.Status.Image, image)
	}
	if build := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild); build == nil ||
		build.Reason != appv1alpha1.ReasonBuildFailedUserError || build.ObservedGeneration != 2 {
		t.Fatalf("Build condition = %+v, want release 2's failed verdict kept", build)
	}
	assertNoBuildJobs(t, cl, nn)
}

// buildSlotTaken is another App's active build in the workspace. With
// MaxConcurrentBuilds at 1 it holds the only slot, so a release waits in
// BuildQueued without a build plane.
func buildSlotTaken(namespace string) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "build-other", Namespace: namespace, Labels: map[string]string{
		execution.LabelComponent: "build", execution.LabelWorkspace: namespace, "app.bex.co/build": "other",
	}}}
}

func TestResumeOverFailedBuildRestoresPriorRelease(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	priorRevision, priorImage := serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	storeReleaseTwoBuildFailure(t, cl, nn, appv1alpha1.ConditionBuild)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("setup: replicas = %d over the failed build, want release 1 still serving", got)
	}

	// Suspend parks release 1.
	setSuspendedAt(t, cl, nn, true, 3)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("suspended replicas = %d, want 0", got)
	}

	// Resume brings it back, and the route follows once a pod is ready.
	setSuspendedAt(t, cl, nn, false, 4)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("resumed replicas = %d, want 1: a recorded build failure must not keep a resumed service at 0", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("resumed phase = %q, want Running", got)
	}
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("resumed backend = %q, want the App's own Service %q", got, app.Name)
	}
	assertPriorReleaseKept(t, cl, nn, priorRevision, priorImage)
}

func TestIdleSleepAndWakeOverFailedBuildKeepPriorRelease(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	priorRevision, priorImage := serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	storeReleaseTwoBuildFailure(t, cl, nn, appv1alpha1.ConditionBuild)
	reconcileTwice(t, r, nn)

	// It goes idle: the route moves to the activator, then the pods drain.
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("idle replicas = %d, want 0: a recorded build failure must not stop an idle free service from sleeping", got)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("hibernated backend = %q, want the activator alias", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("parked phase = %q, want Hibernated", got)
	}

	// A public request wakes it.
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("woken replicas = %d, want 1: a recorded build failure must not keep a woken service at 0", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase on the wake pass = %q, want Running", got)
	}
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("woken backend = %q, want the App's own Service %q once a pod is ready", got, app.Name)
	}
	assertPriorReleaseKept(t, cl, nn, priorRevision, priorImage)
}

// A free service that sleeps while its next release waits for a build slot must
// still sleep and wake on the release it serves, and the build must keep being
// polled: nothing watches build Jobs, so a parked pass that dropped the build's
// requeue could leave a finished build unobserved.
func TestParkedServiceWithQueuedBuildKeepsBuildPollAndWakesOnPriorRelease(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	app.UID = "uid-web" // the fake client assigns none; build admission is keyed on it
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, buildSlotTaken(app.Namespace)).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{}).Build()
	r := wakeReconciler(cl, scheme)
	r.MaxConcurrentBuilds = 1
	readerCalls := 0
	r.ActivityReader = func(_ context.Context, _ *appv1alpha1.App, since time.Time) (time.Time, error) {
		readerCalls++
		return since, nil // no traffic since the stamp
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}

	priorRevision, _ := serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	awake, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseBuilding {
		t.Fatalf("setup: phase = %q, want Building while release 2 waits for a build slot", got)
	}
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("awake replicas while queued = %d, want the prior release still serving", got)
	}
	if awake.RequeueAfter <= 0 || awake.RequeueAfter > 30*time.Second {
		t.Fatalf("awake requeue while queued = %v, want the queued build's own poll", awake.RequeueAfter)
	}

	// It goes idle while queued.
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	var res reconcile.Result
	for range 2 {
		var err error
		if res, err = r.Reconcile(ctx, req); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("idle replicas = %d, want 0: a queued build must not stop an idle free service from sleeping", got)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("parked backend = %q, want the activator alias", got)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter > 30*time.Second {
		t.Fatalf("parked requeue = %v, want the queued build's own poll (at most 30s) so the build is still observed", res.RequeueAfter)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseBuilding {
		t.Fatalf("parked phase = %q, want Building: a build in flight owns the phase", got)
	}

	// While it sleeps, further polls of the queued build keep its requeue and
	// read no traffic: the phase stays Building, so recentlyActive's Hibernated
	// shortcut does not apply.
	callsWhenParked := readerCalls
	for range 3 {
		var err error
		if res, err = r.Reconcile(ctx, req); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	if extra := readerCalls - callsWhenParked; extra != 0 {
		t.Fatalf("activity reads while parked = %d, want none: a sleeping service's build poll must not query its traffic", extra)
	}
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas on later parked polls = %d, want 0", got)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter > 30*time.Second {
		t.Fatalf("later parked requeue = %v, want the queued build's own poll", res.RequeueAfter)
	}

	// A request wakes it while the build is still queued.
	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("woken replicas = %d, want 1 while release 2 waits for its build", got)
	}
	markDeploymentRolledOut(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("woken backend = %q, want the prior release's Service %q", got, app.Name)
	}
	if got := deploymentTemplateRevision(t, cl, nn); got != priorRevision {
		t.Fatalf("template revision = %q while the build waits, want the prior release's %q", got, priorRevision)
	}
}

// A build failure recorded only in the Ready condition, by an operator older
// than w6/m100, must stay on the old halt: the hold's status writes replace
// Ready, and without its marker the failed build would be dispatched again.
func TestLegacyReadyOnlyBuildFailureIsNotHeld(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	storeReleaseTwoBuildFailure(t, cl, nn, appv1alpha1.ConditionReady)

	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	reconcileTwice(t, r, nn)

	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	if ready := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionReady); ready == nil ||
		ready.Reason != appv1alpha1.ReasonBuildFailedUserError {
		t.Fatalf("Ready condition = %+v, want the legacy build-failure marker kept", ready)
	}
	assertNoBuildJobs(t, cl, nn)
}

// A disk restore owns the volume until it finishes. A wake over a failed build
// must not scale the service back up onto a filesystem the restore Job is still
// rewriting.
func TestWakeOverFailedBuildWaitsForDiskRestore(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{}).Build()
	r := wakeReconciler(cl, scheme)
	r.DiskSnapshots = DiskSnapshotStore{Endpoint: "https://s3.example.invalid", Bucket: "snapshots", S3Secret: "s3", AgePublicKey: "age1test", AgeSecret: "age"}
	r.BackupHelperImage = "ghcr.io/bex-co/bex:test"
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	storeReleaseTwoBuildFailure(t, cl, nn, appv1alpha1.ConditionBuild)
	reconcileTwice(t, r, nn)
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("setup: replicas = %d, want 0 once hibernated", got)
	}

	// A restore is requested and its Job is running.
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Disk.RestoreSnapshot = "snap-1"
	live.Generation = 3
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	restore := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: diskRestoreName(app.Name), Namespace: app.Namespace}}
	if err := cl.Create(ctx, restore); err != nil {
		t.Fatal(err)
	}

	stampLastActiveAt(t, cl, nn, time.Now())
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas during the restore = %d, want 0: the hold must not scale up onto a volume the restore owns", got)
	}
}

// A deploy that lands while the service is suspended has no image until it
// builds, so the suspended pass reuses the serving image. It must not write the
// new release's config onto the parked template, or a resume would start the
// prior image under the new release's settings.
func TestDeployWhileSuspendedKeepsTemplateOnServingRelease(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	_, priorImage := serveReleaseOne(t, r, cl, nn)
	setSuspendedAt(t, cl, nn, true, 2)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("setup: suspended replicas = %d, want 0", got)
	}
	parkedRevision := deploymentTemplateRevision(t, cl, nn)

	// Release 3 moves to a repo build while suspended.
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Image = ""
	live.Spec.Repo = "https://example.invalid/repo.git"
	live.Generation = 3
	live.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "3"
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)

	if got := deploymentTemplateRevision(t, cl, nn); got != parkedRevision {
		t.Fatalf("parked template revision = %q, want the parked release's %q — an unbuilt release must not reach the parked template", got, parkedRevision)
	}
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want 0 while suspended", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want Hibernated while suspended", got)
	}
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	if live.Status.Image != priorImage {
		t.Fatalf("status.image = %q, want the serving release's %q", live.Status.Image, priorImage)
	}
	assertNoBuildJobs(t, cl, nn)
}

// An IP allow-list edit made while a failed build is held must reach the
// Ingress. The held pass used to skip the rewrite whenever the route already
// matched, and the route never changes for an allow-list edit, so the old list
// kept being enforced until a new release shipped.
func TestAllowListEditOverFailedBuildReachesIngress(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := activeApp("tea-m157")
	app.Labels[labelAppID] = "srv-m157"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	storeReleaseTwoBuildFailure(t, cl, nn, appv1alpha1.ConditionBuild)
	reconcileTwice(t, r, nn)

	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24"}}
	live.Generation = 3
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)

	mw := &unstructured.Unstructured{}
	mw.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
	if err := cl.Get(ctx, types.NamespacedName{Name: app.Name + "-ip-allow", Namespace: app.Namespace}, mw); err != nil {
		t.Fatalf("allow-list Middleware = %v, want it created while the failed build is held", err)
	}
	ranges, _, _ := unstructured.NestedSlice(mw.Object, "spec", "ipAllowList", "sourceRange")
	if len(ranges) != 1 || ranges[0] != "203.0.113.0/24" {
		t.Fatalf("Middleware sourceRange = %v, want [203.0.113.0/24]", ranges)
	}
}

// A routing failure on a pass that observes a build must not replace the
// Building phase with Failed: that phase pins the release to its build, and
// without it a newer push could start a second build beside the running one.
func TestRoutingFailureWhileBuildQueuedKeepsBuildingPhase(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-m157")
	app.UID = "uid-web"
	refuseAlias := false
	errRefused := errors.New("service write refused")
	aliasWrite := func(obj client.Object) error {
		if svc, ok := obj.(*corev1.Service); ok && refuseAlias && svc.Name == activatorAliasName(app.Name) {
			return errRefused
		}
		return nil
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, buildSlotTaken(app.Namespace)).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if err := aliasWrite(obj); err != nil {
					return err
				}
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				if err := aliasWrite(obj); err != nil {
					return err
				}
				return c.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				if err := aliasWrite(obj); err != nil {
					return err
				}
				return c.Patch(ctx, obj, patch, opts...)
			},
		}).Build()
	r := wakeReconciler(cl, scheme)
	r.MaxConcurrentBuilds = 1
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}

	serveReleaseOne(t, r, cl, nn)
	releaseTwoFromSource(t, cl, nn)
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseBuilding {
		t.Fatalf("setup: phase = %q, want Building while release 2 waits for a build slot", got)
	}

	// It goes idle, and the activator alias its route needs cannot be written.
	// The alias exists from release 1's first pass, before its pod was ready.
	alias := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: activatorAliasName(app.Name), Namespace: app.Namespace}}
	if err := cl.Delete(ctx, alias); client.IgnoreNotFound(err) != nil {
		t.Fatal(err)
	}
	refuseAlias = true
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	if _, err := r.Reconcile(ctx, req); err == nil {
		t.Fatal("reconcile error = nil, want the refused alias write returned")
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseBuilding {
		t.Fatalf("phase after the routing failure = %q, want Building: the queued build still owns the release", got)
	}
}
