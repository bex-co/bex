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
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type maxmemoryCall struct{ maxmemory, policy string }

// newMaxmemoryFixture is a running Persistence Off store with an eviction
// policy, its pod on the StatefulSet's current revision, and the runtime
// CONFIG SET stubbed to record each call (or fail with *fail).
func newMaxmemoryFixture(t *testing.T, policy string) (*runtimeTLSFixture, *[]maxmemoryCall, *error) {
	f := newRuntimeTLSFixture(t)
	var calls []maxmemoryCall
	var fail error
	f.r.setMaxmemory = func(_ context.Context, pod *corev1.Pod, platform *corev1.Secret, maxmemory, policy string) error {
		if pod.Status.PodIP != "10.0.0.7" || platform.Name != "cache-kv-platform" {
			t.Fatalf("CONFIG SET dialed pod %q as %q", pod.Status.PodIP, platform.Name)
		}
		if fail != nil {
			return fail
		}
		calls = append(calls, maxmemoryCall{maxmemory, policy})
		return nil
	}
	// The fake API server bumps no generation; a template change must, as the
	// StatefulSet controller's view of a rollout depends on it.
	f.r.Client = interceptor.NewClient(f.cl.(client.WithWatch), interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				stored := &appsv1.StatefulSet{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(sts), stored); err == nil &&
					!equality.Semantic.DeepEqual(stored.Spec.Template, sts.Spec.Template) {
					sts.Generation = stored.Generation + 1
				}
			}
			return c.Update(ctx, obj, opts...)
		},
	})
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.MaxmemoryPolicy = policy })
	f.startPod()
	f.settleRollout()
	return f, &calls, &fail
}

func (f *runtimeTLSFixture) updateSpec(edit func(*appv1alpha1.KeyValueSpec)) {
	f.t.Helper()
	if err := f.cl.Get(f.ctx, f.nn, f.kv); err != nil {
		f.t.Fatal(err)
	}
	edit(&f.kv.Spec)
	if err := f.cl.Update(f.ctx, f.kv); err != nil {
		f.t.Fatal(err)
	}
	f.reconcileTwice()
}

// settleRollout stands in for the StatefulSet controller finishing a rollout:
// the pod runs the current template's revision.
func (f *runtimeTLSFixture) settleRollout() {
	f.t.Helper()
	sts := f.statefulSet()
	revision := "cache-kv-" + sts.ResourceVersion
	sts.Status.ObservedGeneration, sts.Status.UpdateRevision, sts.Status.CurrentRevision = sts.Generation, revision, revision
	if err := f.cl.Status().Update(f.ctx, sts); err != nil {
		f.t.Fatal(err)
	}
	pod := &corev1.Pod{}
	if err := f.cl.Get(f.ctx, client.ObjectKey{Namespace: f.nn.Namespace, Name: f.nn.Name + "-0"}, pod); err != nil {
		f.t.Fatal(err)
	}
	pod.Labels[appsv1.StatefulSetRevisionLabel] = revision
	pod.Spec = *sts.Spec.Template.Spec.DeepCopy()
	if err := f.cl.Update(f.ctx, pod); err != nil {
		f.t.Fatal(err)
	}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: keyValueContainerName, ContainerID: "containerd://" + revision,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
	if err := f.cl.Status().Update(f.ctx, pod); err != nil {
		f.t.Fatal(err)
	}
	f.reconcileTwice()
	sts = f.statefulSet()
	f.template, f.version = sts.Spec.Template.DeepCopy(), sts.ResourceVersion
}

func (f *runtimeTLSFixture) templatePolicy() string {
	f.t.Helper()
	return valkeyArgsMaxmemoryPolicy(f.statefulSet().Spec.Template.Spec.Containers)
}

// TestKeyValueMaxmemoryPolicyChangeRestartsNothing is w4/m330: a policy change
// on a running Persistence Off store reaches the running process with CONFIG
// SET and leaves the StatefulSet alone, so the pod is not replaced and no key
// is lost. A change that restarts the store anyway (a plan change) starts the
// new server with the current policy.
func TestKeyValueMaxmemoryPolicyChangeRestartsNothing(t *testing.T) {
	f, calls, fail := newMaxmemoryFixture(t, "allkeys-lru")
	free, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
	budget := valkeyMaxmemory(free)
	if f.kv.Status.MaxmemoryPolicy != "allkeys-lru" || !slices.Contains(*calls, maxmemoryCall{budget, "allkeys-lru"}) {
		t.Fatalf("a running store: status policy %q, CONFIG SET calls %v", f.kv.Status.MaxmemoryPolicy, *calls)
	}

	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.MaxmemoryPolicy = "volatile-lru" })
	f.unrolled("a maxmemory policy change")
	if got := (*calls)[len(*calls)-1]; got != (maxmemoryCall{budget, "volatile-lru"}) {
		t.Fatalf("CONFIG SET %v, want the plan's budget and volatile-lru", got)
	}
	if f.kv.Status.MaxmemoryPolicy != "volatile-lru" || f.templatePolicy() != "allkeys-lru" {
		t.Fatalf("status policy %q, template policy %q; want the running volatile-lru and the held allkeys-lru",
			f.kv.Status.MaxmemoryPolicy, f.templatePolicy())
	}

	// A server that refuses the change keeps its policy on record and is
	// retried soon; the store keeps serving.
	*fail = errors.New("LOADING Valkey is loading the dataset in memory")
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.MaxmemoryPolicy = "noeviction" })
	f.unrolled("a refused maxmemory policy change")
	if f.kv.Status.MaxmemoryPolicy != "volatile-lru" || f.kv.Status.Phase == appv1alpha1.KVPhaseFailed {
		t.Fatalf("a refused change: status %+v", f.kv.Status)
	}
	if f.result.RequeueAfter == 0 || f.result.RequeueAfter > kvRuntimePendingRequeue {
		t.Fatalf("a pending policy change requeues after %v", f.result.RequeueAfter)
	}
	*fail = nil
	f.reconcileTwice()
	if f.kv.Status.MaxmemoryPolicy != "noeviction" {
		t.Fatalf("the retry left status policy %q", f.kv.Status.MaxmemoryPolicy)
	}

	// A plan change rolls the pod anyway; the new server starts with the
	// current policy, and the old pod is not given the new plan's budget.
	before := len(*calls)
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.Plan = "starter" })
	if f.templatePolicy() != "noeviction" {
		t.Fatalf("the plan change's template starts the server with %q, want noeviction", f.templatePolicy())
	}
	if len(*calls) != before {
		t.Fatalf("the old pod was reconfigured mid-roll: %v", (*calls)[before:])
	}
}

// A suspended store has no process: nothing is dialed, its template takes the
// new policy, and the resumed server starts with it.
func TestKeyValueMaxmemoryPolicyWhileSuspended(t *testing.T) {
	f, calls, _ := newMaxmemoryFixture(t, "allkeys-lru")
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.Suspended = true })
	before := len(*calls)
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.MaxmemoryPolicy = "allkeys-lfu" })
	if len(*calls) != before || f.templatePolicy() != "allkeys-lfu" {
		t.Fatalf("suspended: CONFIG SET calls %v, template policy %q", (*calls)[before:], f.templatePolicy())
	}
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.Suspended = false })
	f.settleRollout()
	if f.kv.Status.MaxmemoryPolicy != "allkeys-lfu" {
		t.Fatalf("resumed: status policy %q, CONFIG SET calls %v", f.kv.Status.MaxmemoryPolicy, *calls)
	}
}

// Once the template and the process both run the store's policy nothing is
// dialed; while the template holds an older one, every pass re-applies the
// current one, so a server restarted from that template is corrected.
func TestKeyValueMaxmemoryPolicyDialsOnlyWhileHeld(t *testing.T) {
	f, calls, _ := newMaxmemoryFixture(t, "allkeys-lru")
	before := len(*calls)
	f.reconcileTwice()
	if len(*calls) != before {
		t.Fatalf("a store running its template's policy was dialed: %v", (*calls)[before:])
	}
	f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.MaxmemoryPolicy = "volatile-lru" })
	before = len(*calls)
	f.reconcileTwice()
	if len(*calls) != before+2 {
		t.Fatalf("a held template's store was dialed %d times in two passes, want 2", len(*calls)-before)
	}
}

// Only a running server of the current template, with the platform user,
// takes the CONFIG SET.
func TestKeyValueMaxmemoryPolicyDialsOnlyTheServingProcess(t *testing.T) {
	for name, edit := range map[string]func(*corev1.Pod){
		"not running":       func(p *corev1.Pod) { p.Status.ContainerStatuses = nil },
		"no address":        func(p *corev1.Pod) { p.Status.PodIP = "" },
		"another revision":  func(p *corev1.Pod) { p.Labels[appsv1.StatefulSetRevisionLabel] = "cache-kv-old" },
		"not the set's pod": func(p *corev1.Pod) { p.OwnerReferences = nil },
		"no platform user": func(p *corev1.Pod) {
			c := &p.Spec.Containers[0]
			at := slices.Index(c.Args, kvPlatformUser)
			c.Args = slices.Delete(slices.Clone(c.Args), at-1, at+len(valkeyPlatformUserRule("")))
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, calls, _ := newMaxmemoryFixture(t, "allkeys-lru")
			f.updateSpec(func(spec *appv1alpha1.KeyValueSpec) { spec.MaxmemoryPolicy = "volatile-lru" })
			pod := &corev1.Pod{}
			if err := f.cl.Get(f.ctx, client.ObjectKey{Namespace: f.nn.Namespace, Name: f.nn.Name + "-0"}, pod); err != nil {
				t.Fatal(err)
			}
			// The status write returns the stored spec and the spec write the
			// stored status, so each takes the edit.
			edit(pod)
			if err := f.cl.Status().Update(f.ctx, pod); err != nil {
				t.Fatal(err)
			}
			edit(pod)
			if err := f.cl.Update(f.ctx, pod); err != nil {
				t.Fatal(err)
			}
			before := len(*calls)
			f.reconcileTwice()
			if len(*calls) != before {
				t.Fatalf("CONFIG SET reached a pod that is %s: %v", name, (*calls)[before:])
			}
		})
	}
}

// The template holds its policy only while that is the whole difference, and
// only for a server that has the platform user to take the CONFIG SET.
func TestKeyValueTemplateHoldsItsMaxmemoryPolicy(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "hold-kv", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", MaxmemoryPolicy: "allkeys-lru"},
	}
	plan, storage := resolveKVPlan(kv.Spec)
	intent := keyValueIntentFor(kv, plan, storage, "", false, true)
	existing := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(existing, kv, intent)
	existing.ResourceVersion = "7"

	write := func(edit func(*appv1alpha1.KeyValueSpec), sts *appsv1.StatefulSet) string {
		changed := kv.DeepCopy()
		edit(&changed.Spec)
		next := sts.DeepCopy()
		applyKeyValueStatefulSet(next, changed, keyValueIntentFor(changed, plan, storage, "", false, true))
		return valkeyArgsMaxmemoryPolicy(next.Spec.Template.Spec.Containers)
	}
	if got := write(func(s *appv1alpha1.KeyValueSpec) { s.MaxmemoryPolicy = "volatile-ttl" }, existing); got != "allkeys-lru" {
		t.Fatalf("a policy-only change wrote %q into the template", got)
	}
	if got := write(func(s *appv1alpha1.KeyValueSpec) { s.MaxmemoryPolicy = "volatile-ttl"; s.Version = "7" }, existing); got != "volatile-ttl" {
		t.Fatalf("a change that rolls anyway kept %q", got)
	}
	legacy := existing.DeepCopy()
	c := &legacy.Spec.Template.Spec.Containers[0]
	at := slices.Index(c.Args, kvPlatformUser)
	c.Args = slices.Delete(c.Args, at-1, at+len(valkeyPlatformUserRule(""))) // "--user" and the rule
	if valkeyHasPlatformUser(legacy.Spec.Template.Spec.Containers) {
		t.Fatal("the legacy fixture still defines the platform user")
	}
	if got := write(func(s *appv1alpha1.KeyValueSpec) { s.MaxmemoryPolicy = "volatile-ttl" }, legacy); got != "volatile-ttl" {
		t.Fatalf("a template without the platform user kept %q", got)
	}
	created := &appsv1.StatefulSet{}
	if got := write(func(s *appv1alpha1.KeyValueSpec) { s.MaxmemoryPolicy = "volatile-ttl" }, created); got != "volatile-ttl" {
		t.Fatalf("a new StatefulSet starts with %q", got)
	}
}
