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
	"net"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A proxy pod drains on SIGTERM (w6/m512), but one that authenticated its
// clients for a store that now requires internal authentication must not:
// enabling it closes every connection that pod relays, as Render's restart
// does ("Any existing connections that use your unauthenticated URL will
// break", w6/m463). The pod cannot learn why it stops, since its env and grace
// period are fixed when it is created, so the operator closes them in Valkey:
// once such a pod is terminating, each pass kills every Valkey client
// connected from its IP, until the pod is gone. Until then it serves, as it
// did before w6/m512, so the store keeps an endpoint while the new pod starts.
//
// No other roll needs this. A credential rotation, a plan change and a
// suspension also restart Valkey, which closes every upstream connection and
// so ends the old pod's drain with it. Making a store private changes no proxy
// template: the proxy forwards its TLS port while the store's server can
// listen, and the kv-sni-proxy route decides who reaches it (ADR021 § 3).

// kvProxyCutRequeue polls a terminating pod whose clients must be cut: it
// accepts until its SIGTERM, a moment after the pass that saw it terminating.
const kvProxyCutRequeue = time.Second

// cutStaleKeyValueProxyClients closes, in Valkey, the connections of each
// terminating proxy pod whose internal-auth mode the store no longer has. It
// reports whether such a pod remains, so the caller polls again.
func (r *KeyValueReconciler) cutStaleKeyValueProxyClients(ctx context.Context, kv *appv1alpha1.KeyValue, platform *corev1.Secret) bool {
	if r.InternalProxyImage == "" {
		return false
	}
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(kv.Namespace), client.MatchingLabels{labelKeyValueInternalProxy: kv.Name}); err != nil {
		logf.FromContext(ctx).Info("listing the Key Value proxy's pods failed; retrying", "name", kv.Name, "error", err.Error())
		return kv.Spec.InternalAuth
	}
	want := ""
	if kv.Spec.InternalAuth {
		want = "client"
	}
	ips := map[string]bool{}
	stale := false
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp.IsZero() || keyValueProxyPodInternalAuth(pod) == want {
			continue
		}
		stale = true
		if pod.Status.PodIP != "" {
			ips[pod.Status.PodIP] = true
		}
	}
	if len(ips) == 0 {
		return stale
	}
	server := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, server); err != nil || server.Status.PodIP == "" {
		return true
	}
	cut := r.cutValkeyClients
	if cut == nil {
		cut = cutValkeyClientsAtPod
	}
	if err := cut(ctx, server, platform, ips); err != nil {
		// The error can name the pod address; it goes to the log, not status.
		logf.FromContext(ctx).Info("closing a stale Key Value proxy's connections failed; retrying", "name", kv.Name, "error", err.Error())
	}
	return true
}

// keyValueProxyPodInternalAuth is the BEX_KV_INTERNAL_AUTH a proxy pod runs.
func keyValueProxyPodInternalAuth(pod *corev1.Pod) string {
	for _, c := range pod.Spec.Containers {
		if c.Name != kvInternalProxyContainer {
			continue
		}
		for _, e := range c.Env {
			if e.Name == "BEX_KV_INTERNAL_AUTH" {
				return e.Value
			}
		}
	}
	return ""
}

// cutValkeyClientsAtPod runs killValkeyClientsFrom as kvPlatformUser: CLIENT
// KILL is an @admin verb the tenant's default user cannot run.
func cutValkeyClientsAtPod(ctx context.Context, pod *corev1.Pod, platform *corev1.Secret, ips map[string]bool) error {
	_, err := withPlatformValkey(ctx, pod, platform, func(ctx context.Context, c *redis.Client) (struct{}, error) {
		return struct{}{}, killValkeyClientsFrom(ctx, c, ips)
	})
	return err
}

// killValkeyClientsFrom closes every client of c's server connected from one
// of ips, of every type, pub/sub included.
func killValkeyClientsFrom(ctx context.Context, c *redis.Client, ips map[string]bool) error {
	list, err := c.ClientList(ctx).Result()
	if err != nil {
		return err
	}
	var ids []string
	for line := range strings.SplitSeq(list, "\n") {
		var id, addr string
		for field := range strings.FieldsSeq(line) {
			if v, ok := strings.CutPrefix(field, "id="); ok {
				id = v
			} else if v, ok := strings.CutPrefix(field, "addr="); ok {
				addr = v
			}
		}
		if host, _, err := net.SplitHostPort(addr); err == nil && id != "" && ips[host] {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	// The filter form answers how many it killed, so a client that left
	// meanwhile is no error.
	_, err = c.Pipelined(ctx, func(p redis.Pipeliner) error {
		for _, id := range ids {
			p.Do(ctx, "CLIENT", "KILL", "ID", id)
		}
		return nil
	})
	return err
}
