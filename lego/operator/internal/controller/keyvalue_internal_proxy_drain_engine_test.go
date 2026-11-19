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
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/kvinternalauth"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// serveEngineProxy runs a kv-internal-proxy listener in front of e's server
// until the test ends, and reports when Serve returns.
func serveEngineProxy(t *testing.T, config kvinternalauth.Config) (string, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- kvinternalauth.Serve(t.Context(), listener, config) }()
	return listener.Addr().String(), served
}

// TestADrainingProxyLeavesTheReserveToThePlatform (w6/m512): a free store's
// tenants hold the plan's 50 clients through the old proxy pod when it starts
// draining, and reconnect all 50 through the new pod, which has a budget of
// its own. Valkey admits 60, so without KeepReserve the tenants would take the
// platform's 10. With it, the old pod gives up its draining streams as the new
// pod admits, and a new platform-user dial and the new pod's readiness check
// still connect, on both pinned images (BEX_TEST_VALKEY=1).
func TestADrainingProxyLeavesTheReserveToThePlatform(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker proxy drain tests")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			e := startTLSSwitchEngine(t, major)
			free, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
			limit := int(free.ConnectionLimit)
			upstream := e.hostPort("6379")
			oldBudget := kvinternalauth.NewBudget(limit)
			drain := make(chan struct{})
			old, _ := serveEngineProxy(t, kvinternalauth.Config{Upstream: upstream, Password: fixturePassword, Budget: oldBudget, Drain: drain, DrainTimeout: time.Minute})
			newer, _ := serveEngineProxy(t, kvinternalauth.Config{Upstream: upstream, Password: fixturePassword, Budget: kvinternalauth.NewBudget(limit)})
			var held []net.Conn
			t.Cleanup(func() {
				for _, c := range held {
					_ = c.Close()
				}
			})
			for i := range limit {
				c, reply := dialPing(t, old)
				held = append(held, c)
				if reply != "+PONG" {
					t.Fatalf("tenant client %d of %d through the old pod read %q", i+1, limit, reply)
				}
			}

			close(drain)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			go kvinternalauth.KeepReserve(ctx, kvinternalauth.Config{Upstream: upstream, Password: fixturePassword}, oldBudget)
			moved := 0
			for deadline := time.Now().Add(5 * time.Second); moved < limit && time.Now().Before(deadline); {
				c, reply := dialPing(t, newer)
				if reply == "+PONG" {
					held = append(held, c)
					moved++
					continue
				}
				// A burst can meet the server's refusal before the old pod gives
				// a stream back.
				_ = c.Close()
				time.Sleep(20 * time.Millisecond)
			}

			platform := platformValkeyClient(upstream, &corev1.Secret{Data: map[string][]byte{kvPasswordKey: []byte(fixturePlatformPassword)}})
			defer func() { _ = platform.Close() }()
			if err := platform.Ping(ctx).Err(); err != nil {
				t.Fatalf("a platform-user dial during the drain: %v", err)
			}
			if err := kvinternalauth.Check(ctx, kvinternalauth.Config{Upstream: upstream, Password: fixturePassword}); err != nil {
				t.Fatalf("the new pod's readiness check during the drain: %v", err)
			}
			if moved != limit {
				t.Fatalf("only %d of the %d tenants reconnected through the new pod", moved, limit)
			}
		})
	}
}

// TestEnablingInternalAuthCutsADrainingProxysClients (w6/m512): the old proxy
// pod of a store that just enabled internal authentication drains on SIGTERM
// like any other, so the operator closes its clients in Valkey. A held
// unauthenticated client closes within seconds, not at the end of the 30 s
// drain, and the old pod's drain ends with it. A roll that leaves the mode
// alone (a binary change) closes nothing. Both pinned images
// (BEX_TEST_VALKEY=1).
func TestEnablingInternalAuthCutsADrainingProxysClients(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker proxy drain tests")
	}
	for _, major := range []string{"7", "8"} {
		for _, internalAuth := range []bool{true, false} {
			t.Run(major+"/internalAuth="+map[bool]string{true: "enabled", false: "unchanged"}[internalAuth], func(t *testing.T) {
				e := startTLSSwitchEngine(t, major)
				upstream := e.hostPort("6379")
				drain := make(chan struct{})
				old, served := serveEngineProxy(t, kvinternalauth.Config{Upstream: upstream, Password: fixturePassword, Drain: drain, DrainTimeout: 30 * time.Second})
				held, reply := dialPing(t, old)
				t.Cleanup(func() { _ = held.Close() })
				if reply != "+PONG" {
					t.Fatalf("the unauthenticated client read %q", reply)
				}
				close(drain)

				// Valkey sees the proxy's connections from this host's address,
				// as it sees the platform client's own.
				info, err := e.platform.Do(t.Context(), "CLIENT", "INFO").Text()
				if err != nil {
					t.Fatal(err)
				}
				var source string
				for field := range strings.FieldsSeq(info) {
					if addr, ok := strings.CutPrefix(field, "addr="); ok {
						source, _, _ = net.SplitHostPort(addr)
					}
				}
				kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-cut", Namespace: "tea-test"}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", InternalAuth: internalAuth}}
				r := &KeyValueReconciler{
					Client:             fake.NewClientBuilder().WithScheme(keyValueBackupTestScheme(t)).WithObjects(terminatingProxyPod(kv, "", source), valkeyServerPod(kv)).Build(),
					InternalProxyImage: "bex@sha256:fixture",
					cutValkeyClients: func(ctx context.Context, _ *corev1.Pod, _ *corev1.Secret, ips map[string]bool) error {
						return killValkeyClientsFrom(ctx, e.platform, ips)
					},
				}
				began := time.Now()
				pending := r.cutStaleKeyValueProxyClients(t.Context(), kv, nil)
				if !internalAuth {
					if pending {
						t.Fatal("a roll that keeps the proxy's mode polls a cut")
					}
					time.Sleep(time.Second)
					if reply := ping(t, held); reply != "+PONG" {
						t.Fatalf("a roll that keeps the proxy's mode closed the held client: %q", reply)
					}
					return
				}
				if !pending {
					t.Fatal("the stale pod is still terminating, but no poll is pending")
				}
				_ = held.SetReadDeadline(time.Now().Add(5 * time.Second))
				var b [1]byte
				if n, err := held.Read(b[:]); n != 0 || err == nil {
					t.Fatalf("the held unauthenticated client was not closed: n=%d err=%v", n, err)
				}
				if elapsed := time.Since(began); elapsed > 5*time.Second {
					t.Fatalf("the held client closed %v after the cut", elapsed)
				}
				select {
				case err := <-served:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("the old pod's drain outlived its last stream")
				}
			})
		}
	}
}

// terminatingProxyPod is a store's proxy pod that has been told to stop,
// running internalAuth as its BEX_KV_INTERNAL_AUTH, at ip.
func terminatingProxyPod(kv *appv1alpha1.KeyValue, internalAuth, ip string) *corev1.Pod {
	env := []corev1.EnvVar{{Name: "BEX_KV_MAX_CONNECTIONS", Value: "50"}}
	if internalAuth != "" {
		env = append(env, corev1.EnvVar{Name: "BEX_KV_INTERNAL_AUTH", Value: internalAuth})
	}
	now := metav1.Now()
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: kv.Name + "-internal-old", Namespace: kv.Namespace, Labels: keyValueInternalProxyPodLabels(kv),
			DeletionTimestamp: &now, Finalizers: []string{"test/hold"},
		},
		Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: kvInternalProxyContainer, Env: env}}},
		Status: corev1.PodStatus{PodIP: ip},
	}
}

func valkeyServerPod(kv *appv1alpha1.KeyValue) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-0", Namespace: kv.Namespace, Labels: map[string]string{labelKeyValue: kv.Name}},
		Status:     corev1.PodStatus{PodIP: "10.0.0.9"},
	}
}
