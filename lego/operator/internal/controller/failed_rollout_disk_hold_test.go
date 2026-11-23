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
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/165: the swap back to the served release after a failed rollout is a
// Recreate rollout too. Without a hold, nothing used a disk service's volume
// between the failed instance's teardown and the restored instance's binding,
// and a node-status sync in that gap detached the volume and attached it again
// to the same node (about 25 s, w6/161).

// failedDiskRollout is a disk-bearing Starter web service whose release 1
// served on node-a and whose release 2 was rolled there and is crash-looping:
// its instance runs on node-a with the volume mounted, and its rollout has
// just passed its progress deadline.
type failedDiskRollout struct {
	*diskHoldFixture
	nn     types.NamespacedName
	served corev1.PodTemplateSpec
	failed *corev1.Pod
}

func newFailedDiskRollout(t *testing.T) *failedDiskRollout {
	t.Helper()
	app := activeApp("tea-w6165")
	app.UID = "w6165-uid"
	app.Spec.Tier = "starter"
	app.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}
	var uids atomic.Int32
	r, cl, nn := lifecycleFixture(t, app,
		withObjects(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}),
		withInterceptor(interceptor.Funcs{
			// The API server's uid, which the holder's pod and the instances'
			// controller references are matched by.
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if obj.GetUID() == "" {
					obj.SetUID(types.UID(fmt.Sprintf("%s-uid-%d", obj.GetName(), uids.Add(1))))
				}
				return c.Create(ctx, obj, opts...)
			},
		}))
	f := &failedDiskRollout{diskHoldFixture: &diskHoldFixture{t: t, c: cl, r: r, app: app, replicas: 1}, nn: nn}

	serveReleaseOne(t, r, cl, nn)
	f.served = deploymentTemplate(t, cl, nn)
	if !mountsDiskClaim(f.served.Spec, app.Name) {
		t.Fatal("setup: release 1's template does not mount the disk")
	}
	// Release 2 rolls with no instance running, so the setup takes no hold, and
	// its instance then crash-loops on node-a with the volume mounted.
	deployImageAt(t, cl, nn, "nginx:2", 2)
	setDeploymentStatus(t, cl, nn, statusServedPodReady)
	reconcileTwice(t, r, nn)
	if got := appContainerImage(deploymentTemplate(t, cl, nn)); got != "nginx:2" {
		t.Fatalf("setup: template runs %s, want release 2's nginx:2", got)
	}
	f.failed = f.runInstance("two", "node-a", corev1.PodRunning)
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(0, time.Now()))
	return f
}

func (f *failedDiskRollout) expectNotRestored(when string) {
	f.t.Helper()
	if got := deploymentTemplateRevision(f.t, f.c, f.nn); got != "rev-2" {
		f.t.Fatalf("%s the template is %q: the served release was written before its volume was held, and the failed instance would stop with nothing holding it", when, got)
	}
}

func TestFailedRolloutRestoreHoldsTheDiskThroughTheSwap(t *testing.T) {
	f := newFailedDiskRollout(t)
	held := diskHoldCount(diskHoldHeld)
	mounted := diskHoldEndCount(diskHoldEndMounted)

	// The pass that settles the verdict starts the holder on the volume's
	// node, and the pre-pull of the served image beside it, and keeps the
	// failed template.
	reconcileOnce(t, f.r, f.nn)
	rollout := meta.FindStatusCondition(liveApp(t, f.c, f.nn).Status.Conditions, appv1alpha1.ConditionRollout)
	if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.ObservedGeneration != 2 {
		t.Fatalf("Rollout = %+v, want release 2's failed verdict stored before the restore waits", rollout)
	}
	holder := f.holder()
	if holder == nil || diskJobNode(holder) != "node-a" {
		t.Fatalf("holder = %v, want one on node-a, where the failed instance has the volume", holder)
	}
	prePull := f.prePull()
	if prePull == nil || diskJobNode(prePull) != "node-a" || !slices.Equal(diskPrePullImages(prePull), []string{"nginx:1"}) {
		t.Fatalf("pre-pull = %v, want one of the served nginx:1 on node-a", prePull)
	}
	f.expectNotRestored("with the holder just created")

	// The failed-rollout hold keeps waiting while the holder is not mounted.
	pod := f.holderPod(holder, false)
	pull := f.prePullPod(prePull, pulledState)
	f.endPull(pull, pulledState)
	reconcileOnce(t, f.r, f.nn)
	f.expectNotRestored("with the holder bound but not mounted")
	if got := appPhase(t, f.c, f.nn); got == appv1alpha1.PhaseRunning {
		t.Errorf("phase = %q while the failed template still runs, want it settled only once the served one is back", got)
	}

	// Mounted: the served template goes back in this pass, and the holder
	// outlives the failed instance.
	f.mount(pod)
	reconcileOnce(t, f.r, f.nn)
	if got := deploymentTemplate(t, f.c, f.nn); !equality.Semantic.DeepEqual(got, f.served) {
		t.Fatalf("template revision %q, want release 1's recorded template once the holder has the volume", got.Labels[labelRevision])
	}
	if f.holder() == nil {
		t.Fatal("the holder ended with the restore; it must outlive the failed instance")
	}
	if got := diskHoldCount(diskHoldHeld) - held; got != 1 {
		t.Errorf("held count rose by %v, want 1 for the restore", got)
	}
	if got := appPhase(t, f.c, f.nn); got != appv1alpha1.PhaseRunning {
		t.Errorf("phase = %q, want Running on the served release", got)
	}

	// The restored instance is bound to node-a: the holder stays until it has
	// the volume mounted, then ends.
	f.deleteInstance(f.failed)
	restored := f.runInstance("restored", "node-a", corev1.PodPending)
	reconcileOnce(t, f.r, f.nn)
	if f.holder() == nil {
		t.Fatal("the holder ended before the restored instance had the volume mounted")
	}
	f.mount(restored)
	reconcileOnce(t, f.r, f.nn)
	if f.holder() != nil {
		t.Fatal("the holder outlived the restored instance's mount")
	}
	if got := diskHoldEndCount(diskHoldEndMounted) - mounted; got != 1 {
		t.Errorf("mounted ends rose by %v, want 1", got)
	}
}

// A restore the hold cannot cover goes on at once, as a rollout does: the
// failed rollout never waits on a node the volume must leave.
func TestFailedRolloutRestoreGoesOnWithoutAHoldItCannotTake(t *testing.T) {
	f := newFailedDiskRollout(t)
	before := diskHoldCount(diskHoldSkippedUnschedulable)
	f.cordon()
	reconcileOnce(t, f.r, f.nn)
	if got := deploymentTemplate(t, f.c, f.nn); !equality.Semantic.DeepEqual(got, f.served) {
		t.Fatalf("template revision %q, want release 1's restored at once on a cordoned node", got.Labels[labelRevision])
	}
	if f.holder() != nil {
		t.Error("a holder is left on a cordoned node: the volume must be free to move")
	}
	if got := diskHoldCount(diskHoldSkippedUnschedulable) - before; got != 1 {
		t.Errorf("skipped_unschedulable rose by %v, want 1", got)
	}
}
