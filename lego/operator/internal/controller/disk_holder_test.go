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
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The disk holder (w6/m559): a disk service's Recreate rollout keeps its
// volume on its node through a same-node swap.

// diskHolderFor is app's holder Job for node at tier.
func diskHolderFor(t *testing.T, app *appv1alpha1.App, tier string) *batchv1.Job {
	t.Helper()
	job, err := diskHolderJob(app, tier, "node-a", diskHoldScheme(t))
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestDiskHolderJobIsTheAppsAndNoServiceSelectsIt(t *testing.T) {
	app := diskHoldApp()
	job := diskHolderFor(t, app, "starter")
	if job.Name != "dskhld-blog" || job.Namespace != app.Namespace {
		t.Errorf("job = %s/%s, want default/dskhld-blog", job.Namespace, job.Name)
	}
	long := diskHolderFor(t, diskHoldApp(func(a *appv1alpha1.App) { a.Name = strings.Repeat("a", 63) }), "starter")
	if len(long.Name) > 63 || !strings.HasPrefix(long.Name, diskHolderPrefix) {
		t.Errorf("a long App's holder is %q, want a dskhld- name within 63 characters", long.Name)
	}
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.Kind != "App" || owner.Name != app.Name || owner.UID != app.UID {
		t.Fatalf("controller = %+v, want the App, which bex-operator-workloads requires to admit it", owner)
	}
	for name, labels := range map[string]map[string]string{"job": job.Labels, "pod": job.Spec.Template.Labels} {
		if labels[labelApp] != app.Name || labels[execution.LabelAppUID] != string(app.UID) {
			t.Errorf("%s labels %v: want the App's name and uid, which admission and appPodRequests read", name, labels)
		}
		if labels[execution.LabelComponent] != diskHolderComponent {
			t.Errorf("%s labels %v: want component %q, which keeps it off every instance list", name, labels, diskHolderComponent)
		}
		if _, ok := labels[labelAppID]; ok {
			t.Errorf("%s labels %v carry %s: the Services would select the holder", name, labels, labelAppID)
		}
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Errorf("backoffLimit = %v, want 0: a failed holder is a rollout without one", job.Spec.BackoffLimit)
	}
	if d := job.Spec.ActiveDeadlineSeconds; d == nil || *d <= int64(rolloutBudgetSeconds) {
		t.Errorf("activeDeadlineSeconds = %v, want just past the %ds rollout budget", d, rolloutBudgetSeconds)
	}
	if job.Spec.TTLSecondsAfterFinished == nil {
		t.Error("no ttlSecondsAfterFinished: a holder that hit its deadline would stay")
	}
}

func TestDiskHolderPodHoldsTheClaimOnTheVolumesNode(t *testing.T) {
	app := diskHoldApp()
	job := diskHolderFor(t, app, "starter")
	pod := job.Spec.Template.Spec
	if pod.NodeName != "" {
		t.Errorf("nodeName = %q: bex-operator-workloads refuses a pinned node", pod.NodeName)
	}
	if got := diskHolderNode(job); got != "node-a" {
		t.Errorf("required node = %q, want node-a", got)
	}
	if nodes, required := preferredNodes(pod); len(nodes) != 0 || !required {
		t.Errorf("preferred %v, required by name %v: want the node required, nothing preferred", nodes, required)
	}
	if pod.RestartPolicy != corev1.RestartPolicyNever || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken ||
		pod.ServiceAccountName != "" || pod.PriorityClassName != "" || len(pod.Tolerations) != 0 {
		t.Errorf("pod = restart %s, automount %v, sa %q, priority %q, tolerations %v: want Never, no token, the default account, no class, no toleration on a standard tier",
			pod.RestartPolicy, pod.AutomountServiceAccountToken, pod.ServiceAccountName, pod.PriorityClassName, pod.Tolerations)
	}
	if len(pod.Volumes) != 1 || pod.Volumes[0].PersistentVolumeClaim == nil ||
		pod.Volumes[0].PersistentVolumeClaim.ClaimName != diskPVCName(app.Name) || pod.Volumes[0].PersistentVolumeClaim.ReadOnly {
		t.Fatalf("volumes = %+v, want the disk claim with a read-write source (a read-only one fails the LUKS publish with EBUSY)", pod.Volumes)
	}
	if len(pod.Containers) != 1 || len(pod.InitContainers) != 0 {
		t.Fatalf("containers = %d, init = %d, want one container", len(pod.Containers), len(pod.InitContainers))
	}

	// A node of the App's dedicated serving pool takes it.
	pooled := diskHolderFor(t, app, "pro-plus")
	if pool := tiers.Compute.ServingPool("pro-plus"); pool == "" {
		t.Fatal("pro-plus names no serving pool; pick a tier that does")
	} else if tol := pooled.Spec.Template.Spec.Tolerations; len(tol) != 1 || tol[0].Key != execution.ServingPoolKey || tol[0].Value != pool {
		t.Errorf("pooled tolerations = %v, want the %s pool's", tol, pool)
	}
}

func TestDiskHolderContainerRunsPauseRestrictedAndRequestsNothing(t *testing.T) {
	c := diskHolderFor(t, diskHoldApp(), "starter").Spec.Template.Spec.Containers[0]
	if c.Image != diskHolderImage || c.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("image %s (%s), want the node's own %s, never pulled when present", c.Image, c.ImagePullPolicy, diskHolderImage)
	}
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].Name != diskVolumeName || !c.VolumeMounts[0].ReadOnly {
		t.Errorf("mounts = %+v, want the disk, read-only", c.VolumeMounts)
	}
	if len(c.Ports) != 0 {
		t.Errorf("ports = %v, want none", c.Ports)
	}
	sc := c.SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation ||
		sc.Capabilities == nil || len(sc.Capabilities.Add) != 0 || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" ||
		sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault ||
		sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot || sc.RunAsUser == nil || *sc.RunAsUser == 0 ||
		sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Errorf("security context %+v: want restricted, non-root, read-only root", sc)
	}
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage} {
		request, ok := c.Resources.Requests[name]
		if !ok || !request.IsZero() {
			t.Errorf("%s request = %v (set %v), want an explicit 0 so tenant-limits adds nothing and a full node fits it", name, request.String(), ok)
		}
		if limit, ok := c.Resources.Limits[name]; !ok || limit.Cmp(resource.MustParse("64Mi")) > 0 {
			t.Errorf("%s limit = %v (set %v), want a small one", name, limit.String(), ok)
		}
	}
}

// diskHolderStartCPU is the CPU time a pause pod shaped like the holder spends
// starting, measured on a production tenant node (w6/m559 t005, 2026-11-22).
// Under a 10m CPU limit its runc inits took 13.9 s for the sandbox and 11.4 s
// for the container, 25.4 s at 1/100 of a CPU. It started in 2.4 s at 100m
// and in 1.4 s at 500m.
const diskHolderStartCPU = 254 * time.Millisecond

// The holder's CPU limit is the CFS quota runc's init runs under, so it sets
// how long the holder takes to start, and the rollout waits for that start
// with the old instance still serving. At the 10m first shipped, every holder
// on production took longer than diskHoldMountBound and every rollout went on
// without a hold.
func TestDiskHolderStartsWellWithinItsMountBound(t *testing.T) {
	c := diskHolderFor(t, diskHoldApp(), "starter").Spec.Template.Spec.Containers[0]
	limit, ok := c.Resources.Limits[corev1.ResourceCPU]
	if !ok || limit.IsZero() {
		t.Fatalf("cpu limit = %v (set %v); tenant-limits would give the holder a 2-CPU default", limit.String(), ok)
	}
	throttled := time.Duration(float64(diskHolderStartCPU) / limit.AsApproximateFloat64())
	if throttled > time.Second {
		t.Errorf("at a %s CPU limit the holder spends %v starting, want at most 1s of the %v bound: the wait delays every disk deploy", limit.String(), throttled.Round(10*time.Millisecond), diskHoldMountBound)
	}
	cpu, _, _, ok := tiers.Compute.Resources("starter")
	if !ok {
		t.Fatal("no starter tier")
	}
	if limit.Cmp(resource.MustParse(cpu)) > 0 {
		t.Errorf("cpu limit = %s, want at most a Starter instance's %s, the least a disk service runs on", limit.String(), cpu)
	}
}

func TestDiskHoldWritesTheTemplateOnlyOnceTheHolderHasTheVolume(t *testing.T) {
	f := newDiskHoldFixture(t)
	held := diskHoldCount(diskHoldHeld)
	f.app.Spec.RestartedAt = "2026-11-21T12:00:00Z"

	// The pass starts the holder on the volume's node and writes nothing.
	if wait := f.apply(); wait <= 0 {
		t.Fatalf("wait = %v, want the rollout held for its holder", wait)
	}
	holder := f.holder()
	if holder == nil || diskHolderNode(holder) != "node-a" {
		t.Fatalf("holder = %v, want one on node-a, the volume's node", holder)
	}
	f.expectUnrolled("with the holder just created")

	// The holder's pod is bound but its volume not mounted yet.
	pod := f.holderPod(holder, "node-a", false)
	if wait := f.apply(); wait <= 0 {
		t.Fatalf("wait = %v with the holder unmounted, want the rollout still held", wait)
	}
	f.expectUnrolled("with the holder bound but not mounted")

	// Mounted: the template is written in this pass.
	f.mount(pod)
	if wait := f.apply(); wait != 0 {
		t.Fatalf("wait = %v with the holder mounted, want the template written", wait)
	}
	if got := f.stored().Spec.Template.Annotations["app.bex.co/restarted-at"]; got != f.app.Spec.RestartedAt {
		t.Fatalf("restarted-at = %q, want the restart written once the holder has the volume", got)
	}
	if f.holder() == nil {
		t.Fatal("the holder ended with the template write; it must outlive the old instance")
	}
	if got := diskHoldCount(diskHoldHeld) - held; got != 1 {
		t.Errorf("held count rose by %v, want 1", got)
	}
}

func TestDiskHoldEndsOnceTheNewInstanceHasTheVolumeOnItsNode(t *testing.T) {
	f := newDiskHoldFixture(t)
	f.holdThroughRestart()
	mounted := diskHoldEndCount(diskHoldEndMounted)

	// The old instance is gone and the new one is bound to node-a, still
	// waiting for its volume: the holder stays.
	f.deleteInstance(f.instance)
	next := f.runInstance("new", "node-a", corev1.PodPending)
	f.apply()
	if f.holder() == nil {
		t.Fatal("the holder ended before the new instance had the volume mounted")
	}

	f.mount(next)
	f.apply()
	if f.holder() != nil {
		t.Fatal("the holder outlived the new instance's mount")
	}
	if got := diskHoldEndCount(diskHoldEndMounted) - mounted; got != 1 {
		t.Errorf("mounted ends rose by %v, want 1", got)
	}
}

func TestDiskHoldEndsAtOnceWhenTheNewInstanceIsBoundElsewhere(t *testing.T) {
	f := newDiskHoldFixture(t)
	f.holdThroughRestart()
	moved := diskHoldEndCount(diskHoldEndMoved)
	f.deleteInstance(f.instance)
	// Bound to node-b, not mounted: the volume must move, and the holder on
	// node-a would hold it hostage.
	f.runInstance("new", "node-b", corev1.PodPending)
	f.apply()
	if f.holder() != nil {
		t.Fatal("the holder stayed on node-a while the new instance waits for the volume on node-b")
	}
	if got := diskHoldEndCount(diskHoldEndMoved) - moved; got != 1 {
		t.Errorf("moved ends rose by %v, want 1", got)
	}
}

// A second deploy while the first one's new instance is still starting keeps
// the holder, and the first one's instance mounting the volume is not the end
// of the second rollout.
func TestDiskHoldCarriesThroughARolloutThatFollowsAnother(t *testing.T) {
	f := newDiskHoldFixture(t)
	f.holdThroughRestart()
	f.deleteInstance(f.instance)
	first := f.runInstance("first", "node-a", corev1.PodPending)

	f.app.Spec.RestartedAt = "2026-11-21T12:05:00Z"
	if wait := f.apply(); wait != 0 {
		t.Fatalf("wait = %v, want the second template written at once over the mounted holder", wait)
	}
	if got := f.stored().Spec.Template.Annotations["app.bex.co/restarted-at"]; got != f.app.Spec.RestartedAt {
		t.Fatalf("restarted-at = %q, want the second restart", got)
	}
	f.mount(first)
	f.apply()
	if f.holder() == nil {
		t.Fatal("the first rollout's instance mounting the volume ended the second rollout's hold")
	}
}

func TestDiskHoldFallsBackWithoutBlockingTheRollout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result string
		setup  func(*diskHoldFixture)
		// during runs between the holder's creation and the pass that falls
		// back; nil for a fallback the first pass takes.
		during func(*diskHoldFixture, *batchv1.Job)
	}{{
		name:   "the holder is refused",
		result: diskHoldFallbackRefused,
		setup: func(f *diskHoldFixture) {
			f.refuseJobs = true
		},
	}, {
		name:   "the holder is not mounted in time",
		result: diskHoldFallbackTimeout,
		during: func(f *diskHoldFixture, holder *batchv1.Job) {
			f.age(holder, diskHoldMountBound+time.Second)
		},
	}, {
		name:   "the holder's pod failed",
		result: diskHoldFallbackFailed,
		during: func(f *diskHoldFixture, holder *batchv1.Job) {
			pod := f.holderPod(holder, "node-a", false)
			pod.Status.Phase = corev1.PodFailed
			f.updateStatus(pod)
		},
	}, {
		name:   "the node is cordoned before the rollout",
		result: diskHoldSkippedUnschedulable,
		setup: func(f *diskHoldFixture) {
			f.cordon("node-a")
		},
	}, {
		name:   "the node is cordoned while the rollout waits",
		result: diskHoldSkippedUnschedulable,
		during: func(f *diskHoldFixture, _ *batchv1.Job) {
			f.cordon("node-a")
		},
	}, {
		name:   "the node is being removed by the autoscaler",
		result: diskHoldSkippedUnschedulable,
		setup: func(f *diskHoldFixture) {
			f.taint("node-a", corev1.Taint{Key: "ToBeDeletedByClusterAutoscaler", Effect: corev1.TaintEffectNoSchedule})
		},
	}, {
		name:   "the node is gone",
		result: diskHoldSkippedUnschedulable,
		setup: func(f *diskHoldFixture) {
			f.deleteNode("node-a")
		},
	}, {
		name:   "the plan moves the service to another pool",
		result: diskHoldSkippedOtherPool,
		setup: func(f *diskHoldFixture) {
			f.app.Spec.Tier = "pro-plus"
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDiskHoldFixture(t)
			before := diskHoldCount(tc.result)
			if tc.setup != nil {
				tc.setup(f)
			}
			f.app.Spec.RestartedAt = "2026-11-21T12:00:00Z"
			if tc.during != nil {
				if wait := f.apply(); wait <= 0 {
					t.Fatalf("wait = %v, want the rollout held first", wait)
				}
				holder := f.holder()
				if holder == nil {
					t.Fatal("no holder started")
				}
				tc.during(f, holder)
			}
			if wait := f.apply(); wait != 0 {
				t.Fatalf("wait = %v, want the rollout to go on without the hold", wait)
			}
			if got := f.stored().Spec.Template.Annotations["app.bex.co/restarted-at"]; got != f.app.Spec.RestartedAt {
				t.Fatalf("restarted-at = %q, want the restart written as before the hold existed", got)
			}
			if f.holder() != nil {
				t.Error("a holder is left: the volume must be free to move")
			}
			if got := diskHoldCount(tc.result) - before; got != 1 {
				t.Errorf("%s count rose by %v, want 1", tc.result, got)
			}
		})
	}
}

func TestDiskHoldEndsWithNoRolloutInProgress(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		change func(*diskHoldFixture)
	}{{
		name:   "the Deployment settled",
		reason: diskHoldEndSettled,
		change: func(f *diskHoldFixture) {
			dep := f.stored()
			dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1}
			f.updateStatus(dep)
		},
	}, {
		name:   "the service is suspended",
		reason: diskHoldEndStopped,
		change: func(f *diskHoldFixture) {
			f.replicas = 0
		},
	}, {
		name:   "the holder hit its deadline",
		reason: diskHoldEndFinished,
		change: func(f *diskHoldFixture) {
			holder := f.holder()
			holder.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
			f.updateStatus(holder)
		},
	}, {
		name:   "the rollout outlived its budget",
		reason: diskHoldEndExpired,
		change: func(f *diskHoldFixture) {
			f.age(f.holder(), diskHoldMaxAge+time.Second)
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDiskHoldFixture(t)
			f.holdThroughRestart()
			before := diskHoldEndCount(tc.reason)
			f.apply()
			if f.holder() == nil {
				t.Fatal("the holder ended before anything said so")
			}
			tc.change(f)
			f.apply()
			if f.holder() != nil {
				t.Fatal("the holder is left with no rollout to hold for")
			}
			if got := diskHoldEndCount(tc.reason) - before; got != 1 {
				t.Errorf("%s ends rose by %v, want 1", tc.reason, got)
			}
		})
	}
}

func TestDiskHoldIsNeverStartedWithoutASameNodeSwapToCover(t *testing.T) {
	t.Run("a steady pass", func(t *testing.T) {
		f := newDiskHoldFixture(t)
		f.writes.Store(0)
		if wait := f.apply(); wait != 0 {
			t.Fatalf("wait = %v, want none", wait)
		}
		if f.holder() != nil {
			t.Error("a steady pass started a holder")
		}
		if n := f.writes.Load(); n != 0 {
			t.Errorf("a steady pass made %d writes, want none", n)
		}
	})
	t.Run("a first deploy", func(t *testing.T) {
		f := newDiskHoldFixture(t, withoutDeployment)
		if wait := f.apply(); wait != 0 || f.holder() != nil {
			t.Errorf("wait = %v, holder %v: a first deploy has no volume to hold", wait, f.holder())
		}
	})
	t.Run("no running instance", func(t *testing.T) {
		f := newDiskHoldFixture(t)
		f.deleteInstance(f.instance)
		f.app.Spec.RestartedAt = "2026-11-21T12:00:00Z"
		if wait := f.apply(); wait != 0 || f.holder() != nil {
			t.Errorf("wait = %v, holder %v: nothing on a node holds the volume", wait, f.holder())
		}
	})
	t.Run("a service without a disk", func(t *testing.T) {
		f := newDiskHoldFixture(t, func(a *appv1alpha1.App) {
			a.Spec.Disk = nil
			delete(a.Annotations, annotDiskProvisioned)
		})
		f.app.Spec.RestartedAt = "2026-11-21T12:00:00Z"
		if wait := f.apply(); wait != 0 || f.holder() != nil {
			t.Errorf("wait = %v, holder %v: a rolling service shares no volume", wait, f.holder())
		}
		if got := f.stored().Spec.Strategy.Type; got == appsv1.RecreateDeploymentStrategyType {
			t.Fatalf("strategy = %s, want the stateless rolling update", got)
		}
	})
}

// diskHoldFixture is a disk-bearing App serving one instance on node-a, its
// Deployment, the ReplicaSet that runs its template and the nodes, on a fake
// client that counts writes. envtest runs no kubelet or scheduler, so pod
// placement and status are set by hand either way.
type diskHoldFixture struct {
	t          *testing.T
	c          client.Client
	r          *AppReconciler
	app        *appv1alpha1.App
	instance   *corev1.Pod
	replicas   int32
	refuseJobs bool
	writes     atomic.Int32
	rsCount    int
}

func withoutDeployment(a *appv1alpha1.App) { a.Annotations["test/no-deployment"] = "true" }

func newDiskHoldFixture(t *testing.T, mutate ...func(*appv1alpha1.App)) *diskHoldFixture {
	t.Helper()
	f := &diskHoldFixture{t: t, app: diskHoldApp(mutate...), replicas: 1}
	scheme := diskHoldScheme(t)
	write := func() { f.writes.Add(1) }
	f.c = fake.NewClientBuilder().WithScheme(scheme).WithObjects(f.app,
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-b"}}).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}, &batchv1.Job{}, &corev1.Pod{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				if _, job := obj.(*batchv1.Job); job && f.refuseJobs {
					return apierrors.NewForbidden(schema.GroupResource{Group: "batch", Resource: "jobs"}, obj.GetName(), nil)
				}
				write()
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				write()
				return c.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				write()
				return c.Patch(ctx, obj, patch, opts...)
			},
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				write()
				return c.Delete(ctx, obj, opts...)
			},
		}).Build()
	f.r = &AppReconciler{Client: f.c, BuildClient: f.c, Scheme: scheme}
	if f.app.Annotations["test/no-deployment"] != "" {
		return f
	}
	// The first deploy creates the Deployment; no instance holds a volume yet.
	if wait := f.apply(); wait != 0 {
		t.Fatalf("first deploy wait = %v, want none", wait)
	}
	f.instance = f.runInstance("old", "node-a", corev1.PodRunning)
	return f
}

func diskHoldScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func diskHoldApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	app := diskApp("blog", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1})
	app.UID = "blog-uid"
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Tier = "starter"
	app.Annotations = map[string]string{annotDiskProvisioned: diskProvisionedMarker}
	for _, m := range mutate {
		m(app)
	}
	return app
}

// apply runs the serving pass's Deployment write and returns its hold wait.
func (f *diskHoldFixture) apply() time.Duration {
	f.t.Helper()
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: f.app.Name, Namespace: f.app.Namespace}}
	wait, err := f.r.applyServingDeployment(context.Background(), f.app, dep,
		deploymentParams{image: "nginx:1", port: 8080, replicas: f.replicas}, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	return wait
}

func (f *diskHoldFixture) stored() *appsv1.Deployment {
	f.t.Helper()
	dep := &appsv1.Deployment{}
	if err := f.c.Get(context.Background(), client.ObjectKey{Namespace: f.app.Namespace, Name: f.app.Name}, dep); err != nil {
		f.t.Fatal(err)
	}
	return dep
}

func (f *diskHoldFixture) expectUnrolled(when string) {
	f.t.Helper()
	if got, ok := f.stored().Spec.Template.Annotations["app.bex.co/restarted-at"]; ok {
		f.t.Fatalf("%s the template already carries restarted-at %q: the old instance would stop with nothing holding the volume", when, got)
	}
}

func (f *diskHoldFixture) holder() *batchv1.Job {
	job := &batchv1.Job{}
	if err := f.c.Get(context.Background(), client.ObjectKey{Namespace: f.app.Namespace, Name: diskHolderName(f.app.Name)}, job); err != nil {
		if !apierrors.IsNotFound(err) {
			f.t.Fatal(err)
		}
		return nil
	}
	return job
}

// runInstance starts an instance of the Deployment's current template on
// node: its ReplicaSet, as the Deployment controller would make it, and its
// pod.
func (f *diskHoldFixture) runInstance(name, node string, phase corev1.PodPhase) *corev1.Pod {
	f.t.Helper()
	ctx := context.Background()
	dep := f.stored()
	f.rsCount++
	hash := strings.Repeat(string(rune('a'+f.rsCount)), 6)
	tmpl := dep.Spec.Template.DeepCopy()
	tmpl.Labels[appsv1.DefaultDeploymentUniqueLabelKey] = hash
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: dep.Name + "-" + hash, Namespace: dep.Namespace, Labels: tmpl.Labels,
			UID:             types.UID("rs-" + hash),
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(dep, appsv1.SchemeGroupVersion.WithKind("Deployment"))}},
		Spec: appsv1.ReplicaSetSpec{Selector: dep.Spec.Selector, Template: *tmpl},
	}
	if err := f.c.Create(ctx, rs); err != nil {
		f.t.Fatal(err)
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: rs.Name + "-" + name, Namespace: dep.Namespace, Labels: tmpl.Labels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(rs, appsv1.SchemeGroupVersion.WithKind(kindReplicaSet))}},
		Spec: corev1.PodSpec{NodeName: node, Containers: tmpl.Spec.Containers, Volumes: tmpl.Spec.Volumes},
	}
	if err := f.c.Create(ctx, pod); err != nil {
		f.t.Fatal(err)
	}
	pod.Status.Phase = phase
	f.updateStatus(pod)
	return pod
}

// holderPod is the Job controller's pod for holder, bound to node, mounted
// or not.
func (f *diskHoldFixture) holderPod(holder *batchv1.Job, node string, mounted bool) *corev1.Pod {
	f.t.Helper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: holder.Name + "-x", Namespace: holder.Namespace, Labels: holder.Spec.Template.Labels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(holder, batchv1.SchemeGroupVersion.WithKind("Job"))}},
		Spec: *holder.Spec.Template.Spec.DeepCopy(),
	}
	pod.Spec.NodeName = node
	if err := f.c.Create(context.Background(), pod); err != nil {
		f.t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodPending
	f.updateStatus(pod)
	if mounted {
		f.mount(pod)
	}
	return pod
}

// mount is the kubelet having pod's volumes mounted and its sandbox ready.
func (f *diskHoldFixture) mount(pod *corev1.Pod) {
	f.t.Helper()
	pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{Type: corev1.PodReadyToStartContainers, Status: corev1.ConditionTrue})
	f.updateStatus(pod)
}

// holdThroughRestart runs a restart to the point where its template is
// written over a mounted holder on node-a.
func (f *diskHoldFixture) holdThroughRestart() {
	f.t.Helper()
	f.app.Spec.RestartedAt = "2026-11-21T12:00:00Z"
	if wait := f.apply(); wait <= 0 {
		f.t.Fatalf("wait = %v, want the restart held", wait)
	}
	f.holderPod(f.holder(), "node-a", true)
	if wait := f.apply(); wait != 0 {
		f.t.Fatalf("wait = %v, want the restart written over the mounted holder", wait)
	}
}

func (f *diskHoldFixture) deleteInstance(pod *corev1.Pod) {
	f.t.Helper()
	if err := f.c.Delete(context.Background(), pod); err != nil {
		f.t.Fatal(err)
	}
}

func (f *diskHoldFixture) updateStatus(obj client.Object) {
	f.t.Helper()
	if err := f.c.Status().Update(context.Background(), obj); err != nil {
		f.t.Fatal(err)
	}
}

// age moves holder's creation into the past.
func (f *diskHoldFixture) age(holder *batchv1.Job, by time.Duration) {
	f.t.Helper()
	holder.CreationTimestamp = metav1.NewTime(time.Now().Add(-by))
	if err := f.c.Update(context.Background(), holder); err != nil {
		f.t.Fatal(err)
	}
	if got := f.holder(); got == nil || time.Since(got.CreationTimestamp.Time) < by-time.Second {
		f.t.Fatalf("the fake client kept the holder's creation at %v", got.CreationTimestamp)
	}
}

func (f *diskHoldFixture) node(name string) *corev1.Node {
	f.t.Helper()
	node := &corev1.Node{}
	if err := f.c.Get(context.Background(), client.ObjectKey{Name: name}, node); err != nil {
		f.t.Fatal(err)
	}
	return node
}

func (f *diskHoldFixture) cordon(name string) {
	f.t.Helper()
	node := f.node(name)
	node.Spec.Unschedulable = true
	if err := f.c.Update(context.Background(), node); err != nil {
		f.t.Fatal(err)
	}
}

func (f *diskHoldFixture) taint(name string, taint corev1.Taint) {
	f.t.Helper()
	node := f.node(name)
	node.Spec.Taints = append(node.Spec.Taints, taint)
	if err := f.c.Update(context.Background(), node); err != nil {
		f.t.Fatal(err)
	}
}

func (f *diskHoldFixture) deleteNode(name string) {
	f.t.Helper()
	if err := f.c.Delete(context.Background(), f.node(name)); err != nil {
		f.t.Fatal(err)
	}
}

func diskHoldCount(result string) float64 {
	return testutil.ToFloat64(diskHoldsTotal.WithLabelValues(result))
}

func diskHoldEndCount(reason string) float64 {
	return testutil.ToFloat64(diskHoldEndsTotal.WithLabelValues(reason))
}

// The holder carries the App's label, so every operator reader that lists the
// App's pods by it sees the holder too. None may read it as an instance (t003).
func TestTheDiskHolderIsNoInstanceOfTheRollout(t *testing.T) {
	scheme := diskHoldScheme(t)
	app := diskHoldApp()
	job, err := diskHolderJob(app, "starter", "node-a", scheme)
	if err != nil {
		t.Fatal(err)
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}}},
	}
	holder := func(mutate func(*corev1.Pod)) *corev1.Pod {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-x", Namespace: app.Namespace, Labels: job.Spec.Template.Labels,
				OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: "job-uid", Controller: new(true)}}},
			Spec: *job.Spec.Template.Spec.DeepCopy(),
		}
		mutate(pod)
		return pod
	}
	long := metav1.NewTime(time.Now().Add(-time.Hour))
	for name, pod := range map[string]*corev1.Pod{
		// The pause image failing to pull on a node that runs another
		// sandbox version.
		"pulling": holder(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-a"
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: diskHolderContainer, Image: diskHolderImage,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: imagePullBackOff, Message: "back-off"}}}}
		}),
		// Unschedulable for longer than the stall grace.
		"unschedulable": holder(func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodPending
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
				Reason: corev1.PodReasonUnschedulable, Message: "0/3 nodes are available", LastTransitionTime: long}}
		}),
		// Stuck creating its container for longer than the stall bound.
		"creating": holder(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-a"
			p.Status.Phase = corev1.PodPending
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: long}}
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: diskHolderContainer,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}}}
		}),
		// On a node that went away.
		"lost": holder(func(p *corev1.Pod) {
			p.Spec.NodeName = "node-gone"
			p.Status.Phase = corev1.PodRunning
		}),
	} {
		t.Run(name, func(t *testing.T) {
			r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build(), Scheme: scheme}
			if reason, msg := r.stuckPodMessage(context.Background(), dep, 8080); msg != "" {
				t.Errorf("stuckPodMessage = %s: %q, want nothing: the holder is no instance", reason, msg)
			}
			if msg := r.nodeLossMessage(context.Background(), dep); msg != "" {
				t.Errorf("nodeLossMessage = %q, want nothing: the holder is no instance", msg)
			}
			if node := r.diskNode(context.Background(), app); node != "" {
				t.Errorf("diskNode = %q, want none: only an instance places the volume", node)
			}
		})
	}
}
