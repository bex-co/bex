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

package drainguard

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	capiNS   = "bex-capi"
	machName = "m1"
	doomed   = "tenant-a"
	spare    = "tenant-b"
	tenantNS = "tea-d98210cbbpdc73dcrkvg"
)

var deletedAt = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	s.AddKnownTypeWithName(MachineGVK, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(MachineGVK.GroupVersion().WithKind("MachineList"), &unstructured.UnstructuredList{})
	return s
}

// machine is a tenant-pool Machine on node (none for ""), shaped by opts.
func machine(node string, opts ...func(*unstructured.Unstructured)) *unstructured.Unstructured {
	m := NewMachine()
	m.SetNamespace(capiNS)
	m.SetName(machName)
	m.SetLabels(map[string]string{"nodepool": "bex-tenant-0"})
	m.SetFinalizers([]string{"machine.cluster.x-k8s.io"})
	if node != "" {
		_ = unstructured.SetNestedField(m.Object, node, "status", "nodeRef", "name")
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func withHook(m *unstructured.Unstructured) {
	m.SetAnnotations(map[string]string{HookAnnotation: hookOwner})
}

// deleting marks the Machine as CAPI does: a deletionTimestamp, its finalizer
// still held.
func deleting(m *unstructured.Unstructured) {
	ts := metav1.NewTime(deletedAt)
	m.SetDeletionTimestamp(&ts)
}

func node(name string, ready bool) *corev1.Node {
	status := corev1.ConditionTrue
	if !ready {
		status = corev1.ConditionUnknown
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}},
	}
}

// deployment is an App's rolled-out Deployment with replicas pods.
func deployment(app string, replicas int32, strategy appsv1.DeploymentStrategyType) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: tenantNS, Name: app, Generation: 3},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{appLabel: app}},
			Strategy: appsv1.DeploymentStrategy{Type: strategy},
		},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 3, Replicas: replicas, UpdatedReplicas: replicas, ReadyReplicas: replicas},
	}
}

func pod(app, name, onNode string, ready bool) *corev1.Pod {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	controller := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tenantNS, Name: name, Labels: map[string]string{appLabel: app},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: app + "-rs", UID: "rs", Controller: &controller}},
		},
		Spec:   corev1.PodSpec{NodeName: onNode},
		Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}},
	}
}

// jobPod is a Ready pre-deploy Job pod: it carries the App's label but does
// not serve the App.
func jobPod(app, name, onNode string) *corev1.Pod {
	p := pod(app, name, onNode, true)
	p.OwnerReferences[0].APIVersion, p.OwnerReferences[0].Kind = "batch/v1", "Job"
	return p
}

type fixture struct {
	r   *Reconciler
	now time.Time
}

func newFixture(t *testing.T, m *unstructured.Unstructured, objs ...client.Object) *fixture {
	t.Helper()
	return newFixtureWith(t, interceptor.Funcs{}, m, objs...)
}

// newFixtureWith wraps the workload client in funcs, to inject API failures.
func newFixtureWith(t *testing.T, funcs interceptor.Funcs, m *unstructured.Unstructured, objs ...client.Object) *fixture {
	t.Helper()
	s := testScheme(t)
	f := &fixture{now: deletedAt.Add(time.Minute)}
	f.r = &Reconciler{
		Machines: fake.NewClientBuilder().WithScheme(s).WithObjects(m).Build(),
		Workload: fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).
			WithIndex(&corev1.Pod{}, nodeNameField, func(o client.Object) []string { return []string{o.(*corev1.Pod).Spec.NodeName} }).
			WithInterceptorFuncs(funcs).
			Build(),
		Selector: labels.SelectorFromSet(labels.Set{"nodepool": "bex-tenant-0"}),
		MaxHold:  DefaultMaxHold, Now: func() time.Time { return f.now },
	}
	return f
}

func (f *fixture) reconcile(t *testing.T) ctrl.Result {
	t.Helper()
	res, err := f.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: capiNS, Name: machName}})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	return res
}

func (f *fixture) machine(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	m := NewMachine()
	if err := f.r.Machines.Get(context.Background(), types.NamespacedName{Namespace: capiNS, Name: machName}, m); err != nil {
		t.Fatal(err)
	}
	return m
}

func (f *fixture) hooked(t *testing.T) bool {
	t.Helper()
	_, ok := f.machine(t).GetAnnotations()[HookAnnotation]
	return ok
}

func (f *fixture) deployment(t *testing.T, app string) *appsv1.Deployment {
	t.Helper()
	var dep appsv1.Deployment
	if err := f.r.Workload.Get(context.Background(), types.NamespacedName{Namespace: tenantNS, Name: app}, &dep); err != nil {
		t.Fatal(err)
	}
	return &dep
}

func (f *fixture) evacuatedFrom(t *testing.T, app string) string {
	t.Helper()
	return f.deployment(t, app).Spec.Template.Annotations[EvacuatedFromAnnotation]
}

func (f *fixture) cordoned(t *testing.T, name string) bool {
	t.Helper()
	var n corev1.Node
	if err := f.r.Workload.Get(context.Background(), types.NamespacedName{Name: name}, &n); err != nil {
		t.Fatal(err)
	}
	return n.Spec.Unschedulable
}

func TestGuardedMachineGetsTheHook(t *testing.T) {
	f := newFixture(t, machine(doomed))
	f.reconcile(t)
	if !f.hooked(t) {
		t.Fatal("a guarded Machine has no pre-drain hook")
	}
}

func TestUnguardedMachineLosesTheHook(t *testing.T) {
	m := machine(doomed, withHook)
	m.SetLabels(map[string]string{"nodepool": "bex-platform"})
	f := newFixture(t, m)
	f.reconcile(t)
	if f.hooked(t) {
		t.Fatal("a Machine outside the selector kept the hook")
	}
}

// The incident shape: the only instance runs on the deleting Machine's node.
// The guard cordons, rolls the App off the node and holds the drain until the
// replacement is Ready elsewhere, then releases.
func TestSingleInstanceHandsOverBeforeTheDrain(t *testing.T) {
	f := newFixture(t, machine(doomed, withHook, deleting),
		node(doomed, true), node(spare, true),
		deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType),
		pod("web", "web-old", doomed, true))

	if res := f.reconcile(t); res.RequeueAfter == 0 {
		t.Fatal("the drain was not held for the only instance")
	}
	if !f.hooked(t) || !f.cordoned(t, doomed) {
		t.Fatalf("hooked=%v cordoned=%v, want the hook held and the node cordoned", f.hooked(t), f.cordoned(t, doomed))
	}
	if got := f.evacuatedFrom(t, "web"); got != doomed {
		t.Fatalf("evacuated-from = %q, want %q", got, doomed)
	}

	// The replacement is scheduled elsewhere but not Ready yet: keep holding.
	if err := f.r.Workload.Create(context.Background(), pod("web", "web-new", spare, false)); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t)
	if !f.hooked(t) {
		t.Fatal("released before the replacement was Ready")
	}

	var replacement corev1.Pod
	if err := f.r.Workload.Get(context.Background(), types.NamespacedName{Namespace: tenantNS, Name: "web-new"}, &replacement); err != nil {
		t.Fatal(err)
	}
	replacement.Status = pod("web", "web-new", spare, true).Status
	if err := f.r.Workload.Status().Update(context.Background(), &replacement); err != nil {
		t.Fatal(err)
	}
	if res := f.reconcile(t); res.RequeueAfter != 0 {
		t.Fatalf("still requeuing (%v) once the replacement was Ready", res.RequeueAfter)
	}
	if f.hooked(t) {
		t.Fatal("the hook was not released once the App had a Ready instance elsewhere")
	}
}

// Re-applying the same evacuation must not roll the App a second time.
func TestEvacuationIsAppliedOncePerNode(t *testing.T) {
	dep := deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType)
	dep.Spec.Template.Annotations = map[string]string{EvacuatedFromAnnotation: doomed}
	f := newFixture(t, machine(doomed, withHook, deleting), node(doomed, true), dep, pod("web", "web-old", doomed, true))
	before := f.deployment(t, "web").ResourceVersion
	f.reconcile(t)
	if f.deployment(t, "web").ResourceVersion != before {
		t.Fatal("an App already handing over off this node was patched again")
	}
}

// A Deployment mid-rollout is left alone: its new pods already avoid the
// cordoned node, and re-rolling it would restart that rollout.
func TestRollingDeploymentIsWaitedForNotRestarted(t *testing.T) {
	dep := deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType)
	dep.Status.UpdatedReplicas = 0
	f := newFixture(t, machine(doomed, withHook, deleting), node(doomed, true), dep, pod("web", "web-old", doomed, true))
	if res := f.reconcile(t); res.RequeueAfter == 0 || !f.hooked(t) {
		t.Fatal("the drain was not held for a rolling single instance")
	}
	if got := f.evacuatedFrom(t, "web"); got != "" {
		t.Fatalf("evacuated-from = %q on a Deployment already rolling, want untouched", got)
	}
}

func TestReleasesWithoutHolding(t *testing.T) {
	cases := []struct {
		name string
		m    *unstructured.Unstructured
		objs []client.Object
	}{
		{"no node yet", machine("", withHook, deleting), nil},
		{"node gone", machine(doomed, withHook, deleting), nil},
		{"node not Ready", machine(doomed, withHook, deleting), []client.Object{
			node(doomed, false), deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType), pod("web", "web-old", doomed, true),
		}},
		{"instance elsewhere", machine(doomed, withHook, deleting), []client.Object{
			node(doomed, true), deployment("web", 2, appsv1.RollingUpdateDeploymentStrategyType),
			pod("web", "web-a", doomed, true), pod("web", "web-b", spare, true),
		}},
		{"disk App (Recreate)", machine(doomed, withHook, deleting), []client.Object{
			node(doomed, true), deployment("db", 1, appsv1.RecreateDeploymentStrategyType), pod("db", "db-0", doomed, true),
		}},
		{"only a pre-deploy Job pod", machine(doomed, withHook, deleting), []client.Object{
			node(doomed, true), deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType), jobPod("web", "predeploy-web", doomed),
		}},
		{"not an App pod", machine(doomed, withHook, deleting), []client.Object{
			node(doomed, true), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "cni"}, Spec: corev1.PodSpec{NodeName: doomed}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.m, tc.objs...)
			if res := f.reconcile(t); res.RequeueAfter != 0 {
				t.Fatalf("requeued %v, want an immediate release", res.RequeueAfter)
			}
			if f.hooked(t) {
				t.Fatal("the hook was not released")
			}
		})
	}
}

// A replacement that never becomes Ready must not pin the node.
func TestHoldIsBounded(t *testing.T) {
	f := newFixture(t, machine(doomed, withHook, deleting),
		node(doomed, true), deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType), pod("web", "web-old", doomed, true))
	f.now = deletedAt.Add(DefaultMaxHold - time.Second)
	f.reconcile(t)
	if !f.hooked(t) {
		t.Fatal("released before the bound")
	}
	f.now = deletedAt.Add(DefaultMaxHold)
	f.reconcile(t)
	if f.hooked(t) {
		t.Fatal("the hook outlived the bound")
	}
}

// Another owner's hook, or a deleting Machine drain-guard never hooked, is not
// drain-guard's to touch.
func TestDeletingMachineWithoutTheHookIsLeftAlone(t *testing.T) {
	m := machine(doomed, deleting)
	m.SetAnnotations(map[string]string{"pre-drain.delete.hook.machine.cluster.x-k8s.io/other": "x"})
	f := newFixture(t, m, node(doomed, true))
	f.reconcile(t)
	if f.cordoned(t, doomed) {
		t.Fatal("cordoned a node drain-guard is not holding")
	}
	if f.machine(t).GetAnnotations()["pre-drain.delete.hook.machine.cluster.x-k8s.io/other"] != "x" {
		t.Fatal("another owner's hook was changed")
	}
}

// A pre-deploy Job pod Ready on another node is not an instance of the App: the
// only instance is still on the deleting node, so the drain is held.
func TestJobPodElsewhereIsNotAnInstance(t *testing.T) {
	f := newFixture(t, machine(doomed, withHook, deleting),
		node(doomed, true), node(spare, true),
		deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType),
		pod("web", "web-old", doomed, true), jobPod("web", "predeploy-web", spare))
	if res := f.reconcile(t); res.RequeueAfter == 0 || !f.hooked(t) {
		t.Fatal("released while the App's only instance was on the node")
	}
	if got := f.evacuatedFrom(t, "web"); got != doomed {
		t.Fatalf("evacuated-from = %q, want %q", got, doomed)
	}
}

// A failing workload API keeps the hook and retries at the hold's pace, never
// controller-runtime's growing backoff; the bound still releases on time, since
// it is checked before any workload call.
func TestFailingWorkloadAPIStillHonoursTheBound(t *testing.T) {
	down := errors.New("apiserver unavailable")
	f := newFixtureWith(t, interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error { return down },
	}, machine(doomed, withHook, deleting),
		node(doomed, true), deployment("web", 1, appsv1.RollingUpdateDeploymentStrategyType), pod("web", "web-old", doomed, true))

	if res := f.reconcile(t); res.RequeueAfter != recheck {
		t.Fatalf("requeue = %v, want the hold pace %v", res.RequeueAfter, recheck)
	}
	if !f.hooked(t) {
		t.Fatal("an API failure released the hook")
	}
	f.now = deletedAt.Add(DefaultMaxHold)
	f.reconcile(t)
	if f.hooked(t) {
		t.Fatal("the bound did not release while the API kept failing")
	}
}
