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
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/m521 t001: a new store's internal URL is credential-free from its first
// read, as Render's is. Against a real API server, through the whole
// Reconcile, with every write to the stable Service recorded: the Service a
// new store gets never selects the Valkey pod, not even in the pass that
// creates it, and it is created only after the proxy's NetworkPolicies exist,
// which are the same ones a cut-over store has. A store whose Service already
// selects Valkey keeps the w8/204 wait for a Ready proxy.
var _ = Describe("A new Key Value's first reconcile (w9/m521)", func() {
	const proxyImage = "ghcr.io/bex-co/bex-operator@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	var stores int

	type serviceWrite struct {
		verb     string
		selector map[string]string
		// policies is how many of the proxy's NetworkPolicies existed then.
		policies int
	}
	// recording wraps the suite's API server connection: every Create,
	// Update and Patch of the Service named name is recorded with its
	// selector and the proxy policies that existed when it was sent.
	recording := func(name string) (client.Client, func() []serviceWrite) {
		GinkgoHelper()
		base, err := client.NewWithWatch(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())
		var mu sync.Mutex
		var writes []serviceWrite
		record := func(ctx context.Context, verb string, obj client.Object) {
			svc, ok := obj.(*corev1.Service)
			if !ok || svc.Name != name {
				return
			}
			policies := 0
			for _, suffix := range []string{"-internal-proxy-environment-ingress", "-internal-proxy-backend-ingress"} {
				if base.Get(ctx, client.ObjectKey{Namespace: svc.Namespace, Name: name + suffix}, &networkingv1.NetworkPolicy{}) == nil {
					policies++
				}
			}
			mu.Lock()
			defer mu.Unlock()
			writes = append(writes, serviceWrite{verb: verb, selector: svc.Spec.Selector, policies: policies})
		}
		cl := interceptor.NewClient(base, interceptor.Funcs{
			Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
				record(ctx, "create", obj)
				return c.Create(ctx, obj, opts...)
			},
			Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
				record(ctx, "update", obj)
				return c.Update(ctx, obj, opts...)
			},
			Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				record(ctx, "patch", obj)
				return c.Patch(ctx, obj, patch, opts...)
			},
		})
		return cl, func() []serviceWrite {
			mu.Lock()
			defer mu.Unlock()
			return append([]serviceWrite(nil), writes...)
		}
	}

	newStore := func(internalAuth bool) *appv1alpha1.KeyValue {
		GinkgoHelper()
		stores++
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("red-first-read-%d", stores), Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Name: "first-read", Plan: "starter", InternalAuth: internalAuth},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		DeferCleanup(func() {
			// envtest runs no garbage collector: delete the owned objects too.
			bg := context.Background()
			for _, obj := range []client.Object{
				&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}},
				&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}},
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}},
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-auth", Namespace: kv.Namespace}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-platform", Namespace: kv.Namespace}},
				kv,
			} {
				_ = k8sClient.Delete(bg, obj)
			}
			policies := &networkingv1.NetworkPolicyList{}
			_ = k8sClient.List(bg, policies, client.InNamespace(kv.Namespace))
			for i := range policies.Items {
				if strings.HasPrefix(policies.Items[i].Name, kv.Name+"-") {
					_ = k8sClient.Delete(bg, &policies.Items[i])
				}
			}
		})
		return kv
	}
	reconcileStore := func(cl client.Client, kv *appv1alpha1.KeyValue) *appv1alpha1.KeyValue {
		GinkgoHelper()
		r := &KeyValueReconciler{Client: cl, Scheme: cl.Scheme(), InternalProxyImage: proxyImage}
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(kv)})
		Expect(err).NotTo(HaveOccurred())
		stored := &appv1alpha1.KeyValue{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kv), stored)).To(Succeed())
		return stored
	}
	connection := func(kv *appv1alpha1.KeyValue) map[string]string {
		GinkgoHelper()
		sec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kv), sec)).To(Succeed())
		data := map[string]string{}
		for key, value := range sec.Data {
			data[key] = string(value)
		}
		return data
	}
	proxySelector := func(kv *appv1alpha1.KeyValue) map[string]string {
		return map[string]string{labelKeyValueInternalProxy: kv.Name}
	}
	markProxyReady := func(kv *appv1alpha1.KeyValue) {
		GinkgoHelper()
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep)).To(Succeed())
		dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
	}
	// proxyPolicies is the store's NetworkPolicies, with its name and uid
	// taken out, so two stores' sets compare.
	proxyPolicies := func(kv *appv1alpha1.KeyValue) map[string]string {
		GinkgoHelper()
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(kv), kv)).To(Succeed())
		list := &networkingv1.NetworkPolicyList{}
		Expect(k8sClient.List(ctx, list, client.InNamespace(kv.Namespace))).To(Succeed())
		specs := map[string]string{}
		for _, np := range list.Items {
			suffix, ok := strings.CutPrefix(np.Name, kv.Name+"-")
			if !ok {
				continue
			}
			spec, err := json.Marshal(np.Spec)
			Expect(err).NotTo(HaveOccurred())
			specs[suffix] = strings.NewReplacer(kv.Name, "<id>", string(kv.UID), "<uid>").Replace(string(spec))
		}
		return specs
	}

	It("routes a new store's Service through its proxy from the pass that creates it, after the proxy's policies", func() {
		kv := newStore(false)
		cl, writes := recording(kv.Name)
		stored := reconcileStore(cl, kv)

		Expect(writes()).NotTo(BeEmpty())
		Expect(writes()[0].verb).To(Equal("create"))
		for _, w := range writes() {
			Expect(w.selector).To(Equal(proxySelector(kv)), "the stable Service of a new store selected the Valkey pod")
			Expect(w.policies).To(Equal(2), "the Service was written before the proxy's NetworkPolicies")
		}
		Expect(stored.Status.InternalAuthOptional).To(BeTrue(), "the first pass's status write carries the flag")
		Expect(stored.Status.Phase).To(Equal(appv1alpha1.KVPhaseProvisioning), "readiness still waits for the proxy")
		Expect(connection(kv)["uri"]).To(Equal("redis://"+kv.Name+":6379"), "the first connection Secret is credential-free")

		By("staying on the proxy, credential-free, while the proxy is not ready yet")
		stored = reconcileStore(cl, stored)
		for _, w := range writes() {
			Expect(w.selector).To(Equal(proxySelector(kv)))
		}
		Expect(stored.Status.InternalAuthOptional).To(BeTrue())

		By("having the NetworkPolicies of a store that cut over to its Ready proxy")
		legacy := newStore(false)
		Expect(k8sClient.Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: legacy.Name, Namespace: legacy.Namespace},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Selector: map[string]string{labelKeyValue: legacy.Name},
				Ports: []corev1.ServicePort{{Name: keyValueContainerName, Port: kvPort, TargetPort: intstr.FromInt(kvPort)}}},
		})).To(Succeed())
		legacyStored := reconcileStore(k8sClient, legacy)
		markProxyReady(legacy)
		legacyStored = reconcileStore(k8sClient, legacyStored)
		Expect(legacyStored.Status.InternalAuthOptional).To(BeTrue(), "precondition: the legacy store cut over")
		Expect(proxyPolicies(kv)).To(Equal(proxyPolicies(legacy)))
		Expect(proxyPolicies(kv)).To(HaveKey("internal-proxy-environment-ingress"))
		Expect(proxyPolicies(kv)).To(HaveKey("internal-proxy-backend-ingress"))
	})

	It("keeps a store whose Service selects Valkey on Valkey, credentialed, until its proxy is Ready (w8/204)", func() {
		kv := newStore(false)
		Expect(k8sClient.Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Selector: map[string]string{labelKeyValue: kv.Name},
				Ports: []corev1.ServicePort{{Name: keyValueContainerName, Port: kvPort, TargetPort: intstr.FromInt(kvPort)}}},
		})).To(Succeed())
		cl, writes := recording(kv.Name)
		stored := reconcileStore(cl, kv)
		for _, w := range writes() {
			Expect(w.selector).To(Equal(map[string]string{labelKeyValue: kv.Name}), "a store with direct clients moved before its proxy was ready")
		}
		Expect(stored.Status.InternalAuthOptional).To(BeFalse())
		data := connection(kv)
		Expect(data["uri"]).To(Equal("redis://"+kv.Name+":"+data["password"]+"@"+kv.Name+":6379"), "the URL keeps its credentials")

		markProxyReady(kv)
		stored = reconcileStore(cl, stored)
		Expect(writes()[len(writes())-1].selector).To(Equal(proxySelector(kv)))
		Expect(stored.Status.InternalAuthOptional).To(BeTrue())
	})

	It("routes a store created with internal auth through its proxy from birth and keeps its URL credentialed", func() {
		kv := newStore(true)
		cl, writes := recording(kv.Name)
		stored := reconcileStore(cl, kv)
		Expect(writes()).NotTo(BeEmpty())
		for _, w := range writes() {
			Expect(w.selector).To(Equal(proxySelector(kv)))
		}
		Expect(stored.Status.InternalAuthOptional).To(BeFalse())
		data := connection(kv)
		Expect(data["uri"]).To(Equal("redis://" + kv.Name + ":" + data["password"] + "@" + kv.Name + ":6379"))
		Expect(stored.Status.Username).To(Equal(kv.Name))
	})
})
