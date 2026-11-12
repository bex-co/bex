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
	"cmp"
	"context"
	"net"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A maxmemory-policy change restarts nothing, as on Render (w4/m330):
//
//   - The pod template keeps the policy its server was started with when the
//     policy is the only thing that would change, so the StatefulSet does not
//     roll. Any other template change rolls the pod anyway, and that roll
//     starts the server with the current policy.
//   - Each pass brings the running process to the current policy with CONFIG
//     SET as the platform user, as soon as the server runs (Valkey takes CONFIG
//     while it loads), including a process restarted for another reason from
//     a template that still holds an older policy. Once the template and the
//     process both run the current policy, nothing is dialed.
//   - A suspended store has no process, so its template takes the current
//     policy and the resumed server starts with it.

// valkeyArgsMaxmemoryPolicy is the policy a Valkey container's args start the
// server with, "" when they set none.
func valkeyArgsMaxmemoryPolicy(containers []corev1.Container) string {
	return valkeyArgValue(containers, "--maxmemory-policy")
}

// applyKeyValueStatefulSet writes the desired StatefulSet shape, keeping the
// template's own maxmemory policy when that is the only change, so a policy
// edit does not roll the pod. A new template, or one without the platform user
// that could take the CONFIG SET, differs anyway and takes the current policy.
// It is a pure function of (sts, kv, intent) so the workload shape is testable
// without a client.
func applyKeyValueStatefulSet(sts *appsv1.StatefulSet, kv *appv1alpha1.KeyValue, intent keyValueIntent) {
	held := valkeyArgsMaxmemoryPolicy(sts.Spec.Template.Spec.Containers)
	if held == kv.Spec.MaxmemoryPolicy || intent.replicas == 0 {
		writeKeyValueStatefulSet(sts, kv, intent)
		return
	}
	before := sts.DeepCopy()
	holding := kv.DeepCopy()
	holding.Spec.MaxmemoryPolicy = held
	writeKeyValueStatefulSet(sts, holding, intent)
	if apiequality.Semantic.DeepEqual(sts.Spec.Template, before.Spec.Template) {
		return
	}
	before.DeepCopyInto(sts)
	writeKeyValueStatefulSet(sts, kv, intent)
}

// valkeyMaxmemoryConfig is the memory budget and policy a store's server runs:
// the plan's budget and the chosen policy, or, with none chosen, Valkey's
// defaults (no budget, noeviction), which a server started without the flags
// already runs.
func valkeyMaxmemoryConfig(spec appv1alpha1.KeyValueSpec, plan tiers.ValkeyTier) (maxmemory, policy string) {
	if spec.MaxmemoryPolicy == "" {
		return "0", "noeviction"
	}
	return valkeyMaxmemory(plan), spec.MaxmemoryPolicy
}

// reconcileKeyValueMaxmemory brings the running process to the store's
// maxmemory policy and reports the policy it is confirmed to run, plus whether
// a change is still pending. It never fails the store: a server that refuses
// the change keeps serving its previous policy, and the next pass retries.
func (r *KeyValueReconciler) reconcileKeyValueMaxmemory(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, platform *corev1.Secret, intent keyValueIntent) (applied string, pending bool) {
	applied = kv.Status.MaxmemoryPolicy
	if intent.replicas == 0 {
		// No process runs, and the template starts the resumed one right.
		return applied, false
	}
	if applied == kv.Spec.MaxmemoryPolicy && valkeyArgsMaxmemoryPolicy(sts.Spec.Template.Spec.Containers) == applied {
		// Any restart starts the server with the policy it already runs.
		return applied, false
	}
	reader := cmp.Or(r.APIReader, client.Reader(r.Client))
	pod := &corev1.Pod{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod); err != nil ||
		!metav1.IsControlledBy(pod, sts) || !pod.DeletionTimestamp.IsZero() || pod.Status.PodIP == "" ||
		persistenceProcess(pod) == "" || !valkeyHasPlatformUser(pod.Spec.Containers) || !podRunsStatefulSetRevision(pod, sts) {
		// No server from the current template runs yet (starting, or an older
		// pod mid-roll, which must not take a new plan's budget); the pod and
		// StatefulSet watches bring the next pass.
		return applied, false
	}
	setMaxmemory := r.setMaxmemory
	if setMaxmemory == nil {
		setMaxmemory = setValkeyMaxmemoryAtPod
	}
	maxmemory, policy := valkeyMaxmemoryConfig(kv.Spec, intent.plan)
	if err := setMaxmemory(ctx, pod, platform, maxmemory, policy); err != nil {
		// The error can name the pod address; it goes to the log, not status.
		logf.FromContext(ctx).Info("Valkey maxmemory policy change did not complete; retrying", "name", kv.Name, "policy", policy, "error", err.Error())
		return applied, true
	}
	return kv.Spec.MaxmemoryPolicy, false
}

// podRunsStatefulSetRevision reports whether pod runs the StatefulSet's
// current template: the controller has observed the latest spec and the pod
// carries its update revision.
func podRunsStatefulSetRevision(pod *corev1.Pod, sts *appsv1.StatefulSet) bool {
	return sts.Status.ObservedGeneration >= sts.Generation && sts.Status.UpdateRevision != "" &&
		pod.Labels[appsv1.StatefulSetRevisionLabel] == sts.Status.UpdateRevision
}

// setValkeyMaxmemoryAtPod runs setValkeyMaxmemory as kvPlatformUser: CONFIG
// SET is an @admin verb the tenant's default user cannot run.
func setValkeyMaxmemoryAtPod(ctx context.Context, pod *corev1.Pod, platform *corev1.Secret, maxmemory, policy string) error {
	_, err := withPlatformValkey(ctx, pod, platform, func(ctx context.Context, c *redis.Client) (struct{}, error) {
		return struct{}{}, setValkeyMaxmemory(ctx, c, maxmemory, policy)
	})
	return err
}

// withPlatformValkey runs fn against pod's Valkey as kvPlatformUser, within
// the 8-second budget every runtime switch gets.
func withPlatformValkey[T any](ctx context.Context, pod *corev1.Pod, platform *corev1.Secret, fn func(context.Context, *redis.Client) (T, error)) (T, error) {
	timeout, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c := platformValkeyClient(net.JoinHostPort(pod.Status.PodIP, strconv.Itoa(kvPort)), platform)
	defer func() { _ = c.Close() }()
	return fn(timeout, c)
}

// setValkeyMaxmemory sets the memory budget and eviction policy in the
// running process, in one CONFIG SET so the server never runs one without the
// other. Neither changes the keyspace by itself.
func setValkeyMaxmemory(ctx context.Context, c *redis.Client, maxmemory, policy string) error {
	current, err := c.ConfigGet(ctx, "maxmemory*").Result()
	if err != nil {
		return err
	}
	if current["maxmemory"] == maxmemory && current["maxmemory-policy"] == policy {
		return nil
	}
	return c.Do(ctx, "CONFIG", "SET", "maxmemory", maxmemory, "maxmemory-policy", policy).Err()
}
