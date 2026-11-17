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

	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A store accepts exactly its plan's connections, as on Render (w6/m456,
// ADR021 § Connection limits):
//
//   - kv-internal-proxy, in front of every tenant connection, holds the plan's
//     limit across its plain and TLS listeners and answers the next client
//     with Valkey's own refusal (kvinternalauth.Budget).
//   - valkey-server runs with the limit plus kvPlatformClientReserve, so the
//     platform's own clients (probes, the exporter, the operator, backups)
//     always fit beside a tenant at its limit, and a client that reaches the
//     server some other way still meets a bound.
//   - An existing template keeps the limit its server was started with when
//     that is the only change and the server can be lowered to the new one, so
//     the operator upgrade rolls no store; each pass then brings the running
//     process to the plan's value with CONFIG SET.

const (
	// kvPlatformClientReserve is the server's room above the plan's limit for
	// the platform's own clients, at most this many at once: the Valkey
	// readiness probe (1), the metrics exporter's scrape (1), the internal
	// proxy's readiness check (2 during its rollout), the operator's runtime
	// passes (1, 2 across a leader handoff) and a backup snapshot (1, 2 if a
	// manual one overlaps), plus one spare.
	kvPlatformClientReserve = 10
	// valkeyDefaultMaxclients is the limit a server started without
	// --maxclients runs. Valkey then lowers its own open-files limit to it
	// (plus 32), so CONFIG SET can lower that server's limit but not raise it.
	valkeyDefaultMaxclients = 10000
	// annotKVMaxclients records on the StatefulSet the limit CONFIG SET gave
	// its server and that server's process (persistenceProcess), so a settled
	// store is not dialed again until the server restarts from its template.
	// Not on the pod: the bex-operator-pod-labels admission policy lets the
	// operator change no pod annotation (w6/m456 t006).
	annotKVMaxclients = "app.bex.co/valkey-maxclients"
)

// valkeyMaxclients is the server's client limit for plan: the plan's
// connection limit plus the platform's reserve.
func valkeyMaxclients(plan tiers.ValkeyTier) string {
	return strconv.Itoa(int(plan.ConnectionLimit) + kvPlatformClientReserve)
}

// valkeyMaxclientsHoldable reports whether a template whose server was started
// with held (its --maxclients, "" for none) may keep it while want is applied
// at runtime: only when CONFIG SET can reach want from it, which means
// lowering. A raise needs the server restarted with the new flag.
func valkeyMaxclientsHoldable(held, want string) bool {
	if held == want {
		return false
	}
	started := valkeyDefaultMaxclients
	if held != "" {
		n, err := strconv.Atoi(held)
		if err != nil {
			return false
		}
		started = n
	}
	target, err := strconv.Atoi(want)
	return err == nil && target <= started
}

// reconcileKeyValueMaxclients brings the running server to its plan's client
// limit and reports whether a change is still pending. It never fails the
// store: a server that refuses keeps its previous limit, which the proxy's
// budget still holds the tenant below, and the next pass retries.
func (r *KeyValueReconciler) reconcileKeyValueMaxclients(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, platform *corev1.Secret, intent keyValueIntent) (pending bool) {
	want := valkeyMaxclients(intent.plan)
	if intent.replicas == 0 || valkeyArgValue(sts.Spec.Template.Spec.Containers, "--maxclients") == want {
		// No process runs, or the template starts every server with want
		// (an older pod mid-roll is replaced by one that does).
		return false
	}
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod); err != nil ||
		!metav1.IsControlledBy(pod, sts) || !pod.DeletionTimestamp.IsZero() || pod.Status.PodIP == "" ||
		persistenceProcess(pod) == "" || !valkeyHasPlatformUser(pod.Spec.Containers) || !podRunsStatefulSetRevision(pod, sts) {
		// No server from the current template runs yet; the pod watch
		// brings the next pass.
		return false
	}
	applied := want + "/" + persistenceProcess(pod)
	if sts.Annotations[annotKVMaxclients] == applied {
		return false
	}
	setMaxclients := r.setMaxclients
	if setMaxclients == nil {
		setMaxclients = setValkeyMaxclientsAtPod
	}
	if err := setMaxclients(ctx, pod, platform, want); err != nil {
		// The error can name the pod address; it goes to the log, not status.
		logf.FromContext(ctx).Info("Valkey client limit change did not complete; retrying", "name", kv.Name, "maxclients", want, "error", err.Error())
		return true
	}
	patch := client.MergeFrom(sts.DeepCopy())
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}
	sts.Annotations[annotKVMaxclients] = applied
	if err := r.Patch(ctx, sts, patch); err != nil {
		// The server runs want; without the record the next pass checks again.
		logf.FromContext(ctx).Info("Valkey client limit applied but not recorded", "name", kv.Name, "error", err.Error())
	}
	return false
}

// setValkeyMaxclientsAtPod runs setValkeyMaxclients as kvPlatformUser:
// CONFIG SET is an @admin verb the tenant's default user cannot run.
func setValkeyMaxclientsAtPod(ctx context.Context, pod *corev1.Pod, platform *corev1.Secret, maxclients string) error {
	_, err := withPlatformValkey(ctx, pod, platform, func(ctx context.Context, c *redis.Client) (struct{}, error) {
		return struct{}{}, setValkeyMaxclients(ctx, c, maxclients)
	})
	return err
}

// setValkeyMaxclients sets the running server's client limit. Lowering it
// below the clients already connected keeps every one of them and refuses new
// connections until the count drops below the limit.
func setValkeyMaxclients(ctx context.Context, c *redis.Client, maxclients string) error {
	current, err := c.ConfigGet(ctx, "maxclients").Result()
	if err != nil {
		return err
	}
	if current["maxclients"] == maxclients {
		return nil
	}
	return c.ConfigSet(ctx, "maxclients", maxclients).Err()
}
