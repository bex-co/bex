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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m469: a store's kv-internal-proxy closes every tenant connection when it
// rolls, so an operator deploy whose /kv-internal-proxy is byte-identical must
// leave its pod template alone. Against a real API server, so "rolls" is a
// generation bump, exactly what the Deployment controller acts on.
var _ = Describe("Key Value proxy image across operator deploys", func() {
	const (
		imageA  = "ghcr.io/bex-co/bex-operator@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		imageB  = "ghcr.io/bex-co/bex-operator@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		binary1 = "1111111111111111111111111111111111111111111111111111111111111111"
		binary2 = "2222222222222222222222222222222222222222222222222222222222222222"
	)
	var (
		kv     *appv1alpha1.KeyValue
		intent keyValueIntent
		stores int
	)

	BeforeEach(func() {
		stores++
		kv = &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("red-proxy-hold-%d", stores), Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Name: "proxy-hold", Plan: "starter"},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		plan, _ := resolveKVPlan(kv.Spec)
		intent = keyValueIntent{plan: plan, authSecretName: "auth", credentialRevision: "rev-1", labels: map[string]string{labelKeyValue: kv.Name}}
		DeferCleanup(func() {
			// envtest runs no garbage collector: delete the owned objects too.
			bg := context.Background()
			_ = k8sClient.Delete(bg, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}})
			_ = k8sClient.Delete(bg, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}})
			_ = k8sClient.DeleteAllOf(bg, &networkingv1.NetworkPolicy{}, client.InNamespace(kv.Namespace))
			_ = k8sClient.Delete(bg, kv)
		})
	})

	operator := func(image, binary string) *KeyValueReconciler {
		return &KeyValueReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), InternalProxyImage: image, InternalProxyBinary: binary}
	}
	reconcile := func(r *KeyValueReconciler) *appsv1.Deployment {
		GinkgoHelper()
		_, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent)
		Expect(err).NotTo(HaveOccurred())
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep)).To(Succeed())
		Expect(dep.Spec.Template.Annotations).NotTo(HaveKey(kvInternalProxyBinaryAnnotation),
			"the hash on the pod template would roll every proxy once")
		return dep
	}
	image := func(dep *appsv1.Deployment) string { return dep.Spec.Template.Spec.Containers[0].Image }
	env := func(dep *appsv1.Deployment) map[string]string {
		values := map[string]string{}
		for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
			values[e.Name] = e.Value
		}
		return values
	}

	It("keeps the proxy's image and generation through a deploy that leaves the binary alone", func() {
		first := reconcile(operator(imageA, binary1))
		Expect(image(first)).To(Equal(imageA))
		Expect(first.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, binary1))

		held := reconcile(operator(imageB, binary1))
		Expect(held.Generation).To(Equal(first.Generation))
		Expect(equality.Semantic.DeepEqual(held.Spec.Template, first.Spec.Template)).To(BeTrue(),
			"the pod template changed: %v", held.Spec.Template)
		Expect(image(held)).To(Equal(imageA))
	})

	It("rolls the proxy onto the new image when the binary changes, and records the new hash", func() {
		first := reconcile(operator(imageA, binary1))
		rolled := reconcile(operator(imageB, binary2))
		Expect(rolled.Generation).To(BeNumerically(">", first.Generation))
		Expect(image(rolled)).To(Equal(imageB))
		Expect(rolled.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, binary2))

		// The next deploy with the same binary holds the new image.
		Expect(image(reconcile(operator(imageA, binary2)))).To(Equal(imageB))
	})

	It("still rolls a held proxy for internal auth or a credential change, on the held image", func() {
		reconcile(operator(imageA, binary1))
		r := operator(imageB, binary1)
		held := reconcile(r)
		Expect(env(held)).NotTo(HaveKey("BEX_KV_INTERNAL_AUTH"))

		kv.Spec.InternalAuth = true
		auth := reconcile(r)
		Expect(auth.Generation).To(BeNumerically(">", held.Generation), "enabling internal auth must roll the proxy (w6/m463)")
		Expect(env(auth)).To(HaveKeyWithValue("BEX_KV_INTERNAL_AUTH", "client"))
		Expect(image(auth)).To(Equal(imageA))

		intent.credentialRevision = "rev-2"
		rotated := reconcile(r)
		Expect(rotated.Generation).To(BeNumerically(">", auth.Generation))
		Expect(rotated.Spec.Template.Annotations).To(HaveKeyWithValue(credentialRevisionAnnotation, "rev-2"))
		Expect(image(rotated)).To(Equal(imageA))
	})

	It("follows the operator's image as before when the operator has no hash of its binary", func() {
		reconcile(operator(imageA, binary1))
		dep := reconcile(operator(imageB, ""))
		Expect(image(dep)).To(Equal(imageB))
		Expect(dep.Annotations).NotTo(HaveKey(kvInternalProxyBinaryAnnotation),
			"a hash left behind would claim imageB carries binary1")
		Expect(image(reconcile(operator(imageA, "")))).To(Equal(imageA))
	})

	It("keeps a proxy written before the hash existed when its image's registry shows the same binary", func() {
		before := reconcile(operator(imageA, "")) // today's production proxies
		Expect(before.Annotations).NotTo(HaveKey(kvInternalProxyBinaryAnnotation))

		var asked []string
		r := operator(imageB, binary1)
		r.InternalProxyBinaryOf = func(_ context.Context, image string) (string, error) {
			asked = append(asked, image)
			return binary1, nil
		}
		adopted := reconcile(r)
		Expect(asked).To(Equal([]string{imageA}))
		// Recording the hash bumps the generation (the API server copies a
		// Deployment's annotations to its ReplicaSet), but only a template
		// change makes a new ReplicaSet and replaces the pod.
		Expect(equality.Semantic.DeepEqual(adopted.Spec.Template, before.Spec.Template)).To(BeTrue(),
			"the pod template changed: %v", adopted.Spec.Template)
		Expect(adopted.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, binary1))

		reconcile(r)
		Expect(asked).To(HaveLen(1), "a recorded match asked the registry again")
	})

	It("rolls a proxy written before the hash existed when its image's binary differs or is unknown", func() {
		for _, answer := range []struct {
			sum string
			err error
		}{{sum: binary2}, {err: errors.New("registry unreachable")}} {
			before := reconcile(operator(imageA, ""))
			r := operator(imageB, binary1)
			r.InternalProxyBinaryOf = func(context.Context, string) (string, error) { return answer.sum, answer.err }
			rolled := reconcile(r)
			Expect(rolled.Generation).To(BeNumerically(">", before.Generation))
			Expect(image(rolled)).To(Equal(imageB))
			Expect(rolled.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, binary1))
		}
	})
})
