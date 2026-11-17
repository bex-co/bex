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
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w33/m435: Render runs a Blueprint's initialDeployHook once, after the
// service's first successful deploy, and preDeployCommand before every deploy.
// bex ran the hook in place of the first release's pre-deploy command, before
// the rollout, so a seed ran instead of the migration it depends on and a
// failing hook failed the first deploy. These tests drive whole reconcile
// passes over the fake store.

const (
	hookMigrate = "echo migrate"
	hookSeed    = "echo seed"
)

// hookApp is a paid web service with a pre-deploy command and an initial
// deploy hook.
func hookApp() *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: defaultAppsNamespace, UID: "app-uid",
			// One annotation keeps the map through the store's round trips, for
			// the fixture's helpers that write into it.
			Annotations: map[string]string{"bex.co/display-name": "web"},
		},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "registry.example/app:1", Tier: "starter",
			Port: 3000, Replicas: 1, PreDeployCommand: hookMigrate, InitialDeployHook: hookSeed,
		},
	}
}

func stepJobKey(gen int64) types.NamespacedName {
	return types.NamespacedName{Namespace: defaultAppsNamespace, Name: predeploy.JobName("web", appv1alpha1.BuildRevision(gen))}
}

func hookJobKey(gen int64) types.NamespacedName {
	return types.NamespacedName{Namespace: defaultAppsNamespace, Name: predeploy.HookJobName("web", appv1alpha1.BuildRevision(gen))}
}

// storedJob is the Job at key and whether it exists.
func storedJob(t *testing.T, cl client.Client, key types.NamespacedName) (*batchv1.Job, bool) {
	t.Helper()
	job := &batchv1.Job{}
	if err := cl.Get(context.Background(), key, job); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false
		}
		t.Fatal(err)
	}
	return job, true
}

// jobScript is the command a pre-deploy plane Job runs, as its owner wrote it.
func jobScript(job *batchv1.Job) string {
	return strings.TrimPrefix(strings.Join(job.Spec.Template.Spec.Containers[0].Command, " "), "/bin/sh -c ")
}

// endJob reports the Job at key as ended; the fake store runs no Job controller.
func endJob(t *testing.T, cl client.Client, key types.NamespacedName, succeeded bool) {
	t.Helper()
	job, ok := storedJob(t, cl, key)
	if !ok {
		t.Fatalf("no Job %s to end", key.Name)
	}
	now := metav1.Now()
	job.Status.StartTime = &now
	if succeeded {
		job.Status.CompletionTime = &now
		job.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: now},
		}
	} else {
		job.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"},
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded", LastTransitionTime: now},
		}
	}
	if err := cl.Status().Update(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

// goLive passes release gen's pre-deploy step when it has one, rolls it out
// and reports it ready, so the pass that follows finds it live.
func goLive(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName, gen int64) {
	t.Helper()
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, stepJobKey(gen)); ok {
		endJob(t, cl, stepJobKey(gen), true)
		reconcileOnce(t, r, nn)
	}
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	live := liveApp(t, cl, nn)
	if live.Status.Phase != appv1alpha1.PhaseRunning || live.Status.ActiveRevision != "rev-"+strconv.FormatInt(gen, 10) {
		t.Fatalf("setup: release %d is not live (phase %q, active %q)", gen, live.Status.Phase, live.Status.ActiveRevision)
	}
}

func requireLive(t *testing.T, cl client.Client, nn types.NamespacedName, when string) appv1alpha1.App {
	t.Helper()
	live := liveApp(t, cl, nn)
	ready := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionReady)
	if live.Status.Phase != appv1alpha1.PhaseRunning || ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("%s: phase %q, Ready %+v; want the release live", when, live.Status.Phase, ready)
	}
	return live
}

func TestInitialDeployHookRunsOnceAfterTheFirstReleaseIsLive(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())

	// Before the rollout: the release's own pre-deploy command, never the hook.
	reconcileTwice(t, r, nn)
	step, ok := storedJob(t, cl, stepJobKey(1))
	if !ok || jobScript(step) != hookMigrate {
		t.Fatalf("first release's pre-deploy Job = %v (exists %v), want %q", step, ok, hookMigrate)
	}
	if step.Labels[predeploy.LabelStep] != string(predeploy.StepPreDeploy) {
		t.Fatalf("pre-deploy Job labels = %v, want the pre-deploy step label", step.Labels)
	}
	if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
		t.Fatal("the hook started before the release rolled out")
	}

	// Rolling, not yet live: still no hook.
	endJob(t, cl, stepJobKey(1), true)
	reconcileOnce(t, r, nn)
	if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
		t.Fatal("the hook started before the release was live")
	}

	// Live: the hook starts, as the instance runs.
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	hook, ok := storedJob(t, cl, hookJobKey(1))
	if !ok || jobScript(hook) != hookSeed {
		t.Fatalf("hook Job = %v (exists %v), want %q once the release is live", hook, ok, hookSeed)
	}
	if hook.Labels[predeploy.LabelStep] != string(predeploy.StepInitialDeployHook) ||
		hook.Labels[predeploy.LabelService] != "web" || hook.Labels[predeploy.LabelComponent] != predeploy.ComponentValue {
		t.Fatalf("hook Job labels = %v, want the pre-deploy plane's with the hook step", hook.Labels)
	}
	instance := liveDeployment(t, cl, nn).Spec.Template.Spec.Containers[0]
	container := hook.Spec.Template.Spec.Containers[0]
	if container.Image != instance.Image || len(container.Env) == 0 || len(container.Env) != len(instance.Env) {
		t.Fatalf("hook container image %q env %v, want the live instance's (%q, %v)", container.Image, container.Env, instance.Image, instance.Env)
	}
	live := requireLive(t, cl, nn, "while the hook runs")
	if got := live.Status.InitialDeployHook; got == nil || got.Status != appv1alpha1.PreDeployRunning || got.Generation != 1 || got.Job != hook.Name {
		t.Fatalf("status.initialDeployHook = %+v, want release 1's run recorded running", got)
	}
	if pd := live.Status.PreDeploy; pd == nil || pd.Status != appv1alpha1.PreDeploySucceeded || pd.Job != step.Name {
		t.Fatalf("status.preDeploy = %+v, want the pre-deploy step's own record untouched", pd)
	}
}

// A running hook is looked at again within seconds though the release itself
// is settled, and the pass that reads its end goes back to the steady cadence.
func TestInitialDeployHookIsPolledOnlyWhileItRuns(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	goLive(t, r, cl, nn, 1)
	pass := func() time.Duration {
		t.Helper()
		res, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
		if err != nil {
			t.Fatalf("reconcile: %v", err)
		}
		return res.RequeueAfter
	}
	if got := pass(); got != initialDeployHookPoll {
		t.Fatalf("pass over a running hook requeues in %v, want %v", got, initialDeployHookPoll)
	}
	endJob(t, cl, hookJobKey(1), true)
	if got := pass(); got <= initialDeployHookPoll {
		t.Fatalf("pass that read the hook's end requeues in %v, want the steady-state cadence", got)
	}
}

// Its one success completes the hook: a second release runs its pre-deploy
// command and no hook.
func TestInitialDeployHookSuccessEndsItForLaterReleases(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	goLive(t, r, cl, nn, 1)
	endJob(t, cl, hookJobKey(1), true)
	reconcileOnce(t, r, nn)
	live := requireLive(t, cl, nn, "after the hook succeeded")
	if !live.Status.InitialDeployHookCompleted || live.Status.InitialDeployHook.Status != appv1alpha1.PreDeploySucceeded ||
		live.Status.InitialDeployHook.FinishedAt == "" {
		t.Fatalf("after success: completed %v, run %+v", live.Status.InitialDeployHookCompleted, live.Status.InitialDeployHook)
	}

	deployImageAt(t, cl, nn, "registry.example/app:2", 2)
	goLive(t, r, cl, nn, 2)
	reconcileTwice(t, r, nn)
	if step, ok := storedJob(t, cl, stepJobKey(2)); !ok || jobScript(step) != hookMigrate {
		t.Fatalf("second release's pre-deploy Job = %v (exists %v), want %q", step, ok, hookMigrate)
	}
	if _, ok := storedJob(t, cl, hookJobKey(2)); ok {
		t.Fatal("the hook ran again for the second release")
	}
}

func TestInitialDeployHookFailureLeavesTheReleaseLiveAndRetriesAfterTheNextRelease(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	goLive(t, r, cl, nn, 1)

	endJob(t, cl, hookJobKey(1), false)
	reconcileOnce(t, r, nn)
	live := requireLive(t, cl, nn, "after the hook failed")
	run := live.Status.InitialDeployHook
	if live.Status.InitialDeployHookCompleted || run == nil || run.Status != appv1alpha1.PreDeployFailed || run.Generation != 1 {
		t.Fatalf("after failure: completed %v, run %+v; want release 1's failed run and the hook still owed", live.Status.InitialDeployHookCompleted, run)
	}
	if !strings.Contains(run.Message, "the initial deploy hook") || strings.Contains(run.Message, "pre-deploy") {
		t.Fatalf("failure message = %q, want it to name the hook, not the pre-deploy command", run.Message)
	}
	if pd := live.Status.PreDeploy; pd == nil || pd.Status != appv1alpha1.PreDeploySucceeded {
		t.Fatalf("status.preDeploy = %+v, want the release's passed step untouched by the hook", pd)
	}

	// Ended for this release: never started again for it, even once its Job is
	// gone (the TTL reap).
	if err := cl.Delete(context.Background(), &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: hookJobKey(1).Name, Namespace: nn.Namespace}}); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
		t.Fatal("a hook that failed for release 1 was started again for release 1")
	}
	requireLive(t, cl, nn, "after the failed Job was reaped")

	// The next release runs its pre-deploy command, goes live, then the hook.
	deployImageAt(t, cl, nn, "registry.example/app:2", 2)
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, hookJobKey(2)); ok {
		t.Fatal("the retry started before release 2 was live")
	}
	goLive(t, r, cl, nn, 2)
	if hook, ok := storedJob(t, cl, hookJobKey(2)); !ok || jobScript(hook) != hookSeed {
		t.Fatalf("release 2's hook Job = %v (exists %v), want the owed hook again", hook, ok)
	}
	endJob(t, cl, hookJobKey(2), true)
	reconcileOnce(t, r, nn)
	if live := requireLive(t, cl, nn, "after the retry succeeded"); !live.Status.InitialDeployHookCompleted {
		t.Fatalf("after the retry succeeded: run %+v, want the hook completed", live.Status.InitialDeployHook)
	}

	// Completed: release 3 runs none.
	deployImageAt(t, cl, nn, "registry.example/app:3", 3)
	goLive(t, r, cl, nn, 3)
	if _, ok := storedJob(t, cl, hookJobKey(3)); ok {
		t.Fatal("a completed hook ran again")
	}
}

// A hook still running when the next release goes live is left to finish, and
// no second one starts beside it.
func TestInitialDeployHookRunsOneJobAtATime(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	goLive(t, r, cl, nn, 1)

	deployImageAt(t, cl, nn, "registry.example/app:2", 2)
	goLive(t, r, cl, nn, 2)
	if _, ok := storedJob(t, cl, hookJobKey(1)); !ok {
		t.Fatal("release 2's pre-deploy step canceled release 1's running hook")
	}
	if _, ok := storedJob(t, cl, hookJobKey(2)); ok {
		t.Fatal("a second hook started while release 1's was still running")
	}

	// It fails: release 2 is live, so its own run starts.
	endJob(t, cl, hookJobKey(1), false)
	reconcileOnce(t, r, nn)
	if hook, ok := storedJob(t, cl, hookJobKey(2)); !ok || jobScript(hook) != hookSeed {
		t.Fatalf("release 2's hook Job = %v (exists %v), want it started once release 1's failed", hook, ok)
	}
	if run := liveApp(t, cl, nn).Status.InitialDeployHook; run == nil || run.Generation != 2 || run.Status != appv1alpha1.PreDeployRunning {
		t.Fatalf("status.initialDeployHook = %+v, want release 2's run", run)
	}
}

func TestInitialDeployHookWaitsOutASuspendedService(t *testing.T) {
	app := hookApp()
	app.Spec.Suspended = true
	r, cl, nn := lifecycleFixture(t, app)
	for range 4 {
		reconcileOnce(t, r, nn)
	}
	if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
		t.Fatal("the hook started for a suspended service")
	}

	setSuspendedAt(t, cl, nn, false, 2)
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, stepJobKey(1)); ok {
		endJob(t, cl, stepJobKey(1), true)
		reconcileOnce(t, r, nn)
	}
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	requireLive(t, cl, nn, "after the resume")
	if _, ok := storedJob(t, cl, hookJobKey(1)); !ok {
		t.Fatal("the hook did not start once the resumed release was live")
	}
}

func TestInitialDeployHookNeverStartsForACanceledFirstRelease(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	reconcileTwice(t, r, nn)
	endJob(t, cl, stepJobKey(1), true)
	reconcileOnce(t, r, nn)

	updateApp(t, cl, nn, func(a *appv1alpha1.App) {
		a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "1"
	})
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
		t.Fatal("the hook started for a release that was canceled before it went live")
	}
	if run := liveApp(t, cl, nn).Status.InitialDeployHook; run != nil {
		t.Fatalf("status.initialDeployHook = %+v, want none for a canceled first release", run)
	}
}

// A release canceled before it went live settles back onto the served one.
// That pass reaches the live path on the served release's restored template,
// with the canceled release's spec: a hook that release introduced must not
// start, on the served release's image, as if a release had gone live.
func TestInitialDeployHookNeverStartsForACancelOverTheServedRelease(t *testing.T) {
	app := hookApp()
	app.Spec.InitialDeployHook = ""
	r, cl, nn := lifecycleFixture(t, app)
	goLive(t, r, cl, nn, 1)

	updateApp(t, cl, nn, func(a *appv1alpha1.App) {
		a.Spec.Image, a.Spec.InitialDeployHook, a.Generation = "registry.example/app:2", hookSeed, 2
		a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = "2"
	})
	reconcileTwice(t, r, nn)
	updateApp(t, cl, nn, func(a *appv1alpha1.App) {
		a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "2"
	})
	for range 3 {
		reconcileOnce(t, r, nn)
	}
	live := requireLive(t, cl, nn, "after the cancel settled back")
	if live.OwedInitialDeployHook() == "" || !canceledOverServed(&live) {
		t.Fatalf("setup: want the canceled release's hook owed over the served release (owed %q, release generation %d, active %q)",
			live.OwedInitialDeployHook(), live.Status.ReleaseGeneration, live.Status.ActiveRevision)
	}
	for _, gen := range []int64{1, 2} {
		if _, ok := storedJob(t, cl, hookJobKey(gen)); ok {
			t.Fatalf("the hook started (gen-%d) though the release that introduced it was canceled before it went live", gen)
		}
	}
	if run := live.Status.InitialDeployHook; run != nil {
		t.Fatalf("status.initialDeployHook = %+v, want none", run)
	}
}

// The hook copies its container from the template the Deployment serves the
// release with. A Deployment still on another release's template is not that.
func TestInitialDeployHookNeverCopiesAnotherReleasesTemplate(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	goLive(t, r, cl, nn, 1)
	endJob(t, cl, hookJobKey(1), false)
	reconcileOnce(t, r, nn)

	// Release 2 is the App's release now, while the Deployment read still
	// carries release 1's template.
	live := liveApp(t, cl, nn)
	live.Status.ReleaseGeneration = 2
	live.Status.ActiveRevision = "rev-2"
	stale := liveDeployment(t, cl, nn)
	if got := r.reconcileInitialDeployHook(context.Background(), &live, &stale, "registry.example/app:2"); got != 0 {
		t.Fatalf("requeue = %v, want none", got)
	}
	if _, ok := storedJob(t, cl, hookJobKey(2)); ok {
		t.Fatal("the hook started from a template that is not the live release's")
	}
}

// Either completion marker ends the hook: the operator's status field, or the
// annotation bex-api set for services created before the operator owned it.
func TestInitialDeployHookCompletedByEitherMarkerNeverRuns(t *testing.T) {
	for name, mark := range map[string]func(*appv1alpha1.App){
		"status":     func(a *appv1alpha1.App) { a.Status.InitialDeployHookCompleted = true },
		"annotation": func(a *appv1alpha1.App) { a.Annotations[appv1alpha1.AnnotationInitialDeployHookRan] = "true" },
	} {
		t.Run(name, func(t *testing.T) {
			app := hookApp()
			mark(app)
			r, cl, nn := lifecycleFixture(t, app)
			goLive(t, r, cl, nn, 1)
			reconcileTwice(t, r, nn)
			if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
				t.Fatal("a hook already completed ran again")
			}
		})
	}
}

// Render: "seeding a database or downloading files to a persistent disk". The
// hook mounts the service's disk at its mount path, beside the instance.
func TestInitialDeployHookMountsTheServiceDisk(t *testing.T) {
	app := hookApp()
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	r, cl, nn := lifecycleFixture(t, app)
	goLive(t, r, cl, nn, 1)

	hook, ok := storedJob(t, cl, hookJobKey(1))
	if !ok {
		t.Fatal("no hook Job for a disk-bearing service")
	}
	pod := hook.Spec.Template.Spec
	var claim, path string
	for _, v := range pod.Volumes {
		if v.PersistentVolumeClaim != nil {
			claim = v.PersistentVolumeClaim.ClaimName
		}
	}
	for _, m := range pod.Containers[0].VolumeMounts {
		if m.Name == diskVolumeName {
			path = m.MountPath
		}
	}
	if claim != diskPVCName("web") || path != "/var/data" {
		t.Fatalf("hook mounts claim %q at %q, want the service's disk %q at /var/data", claim, path, diskPVCName("web"))
	}
	// The claim is ReadWriteOnce: the pod must land on the instance's node.
	if pod.Affinity == nil || pod.Affinity.PodAffinity == nil || len(pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution) != 1 {
		t.Fatalf("hook pod affinity = %+v, want it required beside the service's instance", pod.Affinity)
	}
	term := pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]
	instance := liveDeployment(t, cl, nn).Spec.Template.Labels
	selector, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
	if err != nil {
		t.Fatal(err)
	}
	if !selector.Matches(labels.Set(instance)) || selector.Matches(labels.Set(hook.Spec.Template.Labels)) || term.TopologyKey != corev1.LabelHostname {
		t.Fatalf("affinity term %+v must select the instance's pods (%v) on its node and never the hook's own (%v)", term, instance, hook.Spec.Template.Labels)
	}
	// It mounts the volume the way the instance does, wherever that node is.
	if pod.HostUsers != nil || len(pod.NodeSelector) != 0 {
		t.Fatalf("hook pod hostUsers %v nodeSelector %v, want the instance's own (unset) so the volume mounts as it does there", pod.HostUsers, pod.NodeSelector)
	}

	// The pre-deploy command has no disk access, as on Render.
	step, _ := storedJob(t, cl, stepJobKey(1))
	for _, v := range step.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			t.Fatalf("the pre-deploy Job mounts the disk: %+v", v)
		}
	}
}

// A service without a disk runs the hook as a pre-deploy pod runs: in a pod
// user namespace on the tenant pool, with no affinity.
func TestInitialDeployHookWithoutADiskKeepsThePreDeployPodShape(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, hookApp())
	goLive(t, r, cl, nn, 1)
	hook, _ := storedJob(t, cl, hookJobKey(1))
	pod := hook.Spec.Template.Spec
	if pod.HostUsers == nil || *pod.HostUsers || pod.NodeSelector[execution.NodePoolLabel] != execution.UntrustedNodePool || pod.Affinity != nil {
		t.Fatalf("hook pod hostUsers %v nodeSelector %v affinity %v, want the hardened pre-deploy pod shape", pod.HostUsers, pod.NodeSelector, pod.Affinity)
	}
	for _, v := range pod.Volumes {
		if v.PersistentVolumeClaim != nil {
			t.Fatalf("a disk-less service's hook mounts a claim: %+v", v)
		}
	}
}

// A restore the owner asked for, or a pending snapshot purge, owns the
// volume's contents: the hook stays off the disk until it is served.
func TestInitialDeployHookStaysOffADiskWithAPendingRestore(t *testing.T) {
	app := hookApp()
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10, RestoreSnapshot: "2026-11-15T03:00:00Z.tar.gz.age"}
	r, cl, nn := lifecycleFixture(t, app)
	goLive(t, r, cl, nn, 1)
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, hookJobKey(1)); ok {
		t.Fatal("the hook started against a disk with an unserved restore request")
	}

	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Spec.Disk.RestoreSnapshot = "" })
	reconcileTwice(t, r, nn)
	if _, ok := storedJob(t, cl, hookJobKey(1)); !ok {
		t.Fatal("the hook did not start once the restore request was gone")
	}
}

func TestDiskBusyNamesAPendingRestoreOrPurge(t *testing.T) {
	disk := func(mutate func(*appv1alpha1.App)) *appv1alpha1.App {
		app := hookApp()
		app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
		mutate(app)
		return app
	}
	for name, tc := range map[string]struct {
		app  *appv1alpha1.App
		want bool
	}{
		"no disk":       {hookApp(), false},
		"idle disk":     {disk(func(*appv1alpha1.App) {}), false},
		"purge pending": {disk(func(a *appv1alpha1.App) { a.Annotations[annotDiskPurgePending] = "app-uid" }), true},
		"restore asked": {disk(func(a *appv1alpha1.App) { a.Spec.Disk.RestoreSnapshot = "snap" }), true},
		"restore served": {disk(func(a *appv1alpha1.App) {
			a.Spec.Disk.RestoreSnapshot = "snap"
			a.Annotations[annotDiskRestored] = "snap"
		}), false},
	} {
		if got := diskBusy(tc.app); got != tc.want {
			t.Errorf("%s: diskBusy = %v, want %v", name, got, tc.want)
		}
	}
}

// A restore rewrites the volume, so it waits for a hook that still mounts it.
func TestRestoreWaitsForARunningInitialDeployHook(t *testing.T) {
	app := hookApp()
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	r, cl, nn := lifecycleFixture(t, app)
	goLive(t, r, cl, nn, 1)

	live := liveApp(t, cl, nn)
	if held, err := r.initialDeployHookHoldsDisk(context.Background(), &live); err != nil || !held {
		t.Fatalf("initialDeployHookHoldsDisk = %v, %v; want the running hook to hold the disk", held, err)
	}
	endJob(t, cl, hookJobKey(1), true)
	if held, err := r.initialDeployHookHoldsDisk(context.Background(), &live); err != nil || held {
		t.Fatalf("initialDeployHookHoldsDisk = %v, %v; want an ended hook to have let go", held, err)
	}
}
