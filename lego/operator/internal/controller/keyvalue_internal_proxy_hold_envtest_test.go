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
	"slices"

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

// A store's kv-internal-proxy closes every client it relays when it rolls,
// after the w6/m512 drain, so its pod template holds exactly as it runs until
// the store's own change rolls it (w6/m517): no operator deploy moves it,
// whatever its image, binary or rendering (w6/m469 held only the image).
// Against a real API server, so "rolls" is a template change and a generation
// bump, exactly what the Deployment controller acts on.
var _ = Describe("Key Value proxy template across operator deploys", func() {
	const (
		imageA  = "ghcr.io/bex-co/bex-operator@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		imageB  = "ghcr.io/bex-co/bex-operator@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		imageC  = "ghcr.io/bex-co/bex-operator@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
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
		intent = keyValueIntent{plan: plan, authSecretName: "auth", credentialRevision: "rev-1", tlsSecretName: keyValueTLSName(kv), labels: map[string]string{labelKeyValue: kv.Name}}
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
	get := func() *appsv1.Deployment {
		GinkgoHelper()
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep)).To(Succeed())
		return dep
	}
	reconcile := func(r *KeyValueReconciler) *appsv1.Deployment {
		GinkgoHelper()
		_, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent)
		Expect(err).NotTo(HaveOccurred())
		dep := get()
		for _, key := range []string{kvInternalProxyBinaryAnnotation, kvInternalProxyInputsAnnotation, kvInternalProxyRollRevisionAnnotation} {
			Expect(dep.Spec.Template.Annotations).NotTo(HaveKey(key), "a record on the pod template would roll every proxy once")
		}
		Expect(dep.Annotations).To(HaveKeyWithValue(kvInternalProxyInputsAnnotation, keyValueInternalProxyInputs(kv, intent)))
		Expect(dep.Annotations).To(HaveKey(kvInternalProxyRollRevisionAnnotation))
		return dep
	}
	// edit changes the stored Deployment as an older operator would have
	// written it.
	edit := func(change func(*appsv1.Deployment)) *appsv1.Deployment {
		GinkgoHelper()
		dep := get()
		change(dep)
		Expect(k8sClient.Update(ctx, dep)).To(Succeed())
		return get()
	}
	// olderRendering drops what w6/m512 added, as a template written before it.
	olderRendering := func(dep *appsv1.Deployment) {
		c := &dep.Spec.Template.Spec.Containers[0]
		c.Env = slices.DeleteFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "BEX_KV_DRAIN_SECONDS" })
		dep.Spec.Template.Spec.TerminationGracePeriodSeconds = new(int64(corev1.DefaultTerminationGracePeriodSeconds))
	}
	// beforeM517 drops the records, as a Deployment written before w6/m517.
	beforeM517 := func(dep *appsv1.Deployment) {
		delete(dep.Annotations, kvInternalProxyInputsAnnotation)
		delete(dep.Annotations, kvInternalProxyRollRevisionAnnotation)
	}
	image := func(dep *appsv1.Deployment) string { return dep.Spec.Template.Spec.Containers[0].Image }
	env := func(dep *appsv1.Deployment) map[string]string {
		values := map[string]string{}
		for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
			values[e.Name] = e.Value
		}
		return values
	}
	expectHeld := func(dep, held *appsv1.Deployment) {
		GinkgoHelper()
		Expect(dep.Generation).To(Equal(held.Generation))
		Expect(equality.Semantic.DeepEqual(dep.Spec.Template, held.Spec.Template)).To(BeTrue(),
			"the pod template changed: %v", dep.Spec.Template)
	}
	expectCurrent := func(dep *appsv1.Deployment, r *KeyValueReconciler) {
		GinkgoHelper()
		Expect(image(dep)).To(Equal(r.InternalProxyImage))
		Expect(env(dep)).To(HaveKeyWithValue("BEX_KV_DRAIN_SECONDS", fmt.Sprint(kvInternalProxyDrainSeconds)))
		Expect(dep.Spec.Template.Spec.TerminationGracePeriodSeconds).To(HaveValue(BeEquivalentTo(kvInternalProxyGraceSeconds)))
		Expect(dep.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, r.InternalProxyBinary))
	}

	It("keeps the template through deploys that change the image, the binary and the rendering", func() {
		first := reconcile(operator(imageA, binary1))
		Expect(image(first)).To(Equal(imageA))
		Expect(first.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		expectHeld(reconcile(operator(imageB, binary1)), first)
		expectHeld(reconcile(operator(imageB, binary2)), first)

		// A template an older rendering wrote: the current one adds a field.
		older := edit(olderRendering)
		for range 2 {
			expectHeld(reconcile(operator(imageC, binary2)), older)
		}
		Expect(image(older)).To(Equal(imageA))
	})

	DescribeTable("rolls once onto the manager's image and the whole rendering when the store's own change rolls it",
		func(change func()) {
			reconcile(operator(imageA, binary1))
			held := edit(olderRendering)
			r := operator(imageB, binary2)
			expectHeld(reconcile(r), held)

			change()
			rolled := reconcile(r)
			Expect(rolled.Generation).To(Equal(held.Generation + 1))
			expectCurrent(rolled, r)
			expectHeld(reconcile(r), rolled)
		},
		Entry("a plan change", func() { intent.plan, _ = resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "pro"}) }),
		Entry("enabling internal authentication", func() { kv.Spec.InternalAuth = true }),
		Entry("a credential rotation", func() { intent.credentialRevision = "rev-2" }),
		Entry("making an older store with a static listener public", func() { intent.tlsListener = true }),
	)

	It("carries what each change writes into the rolled template", func() {
		reconcile(operator(imageA, binary1))
		r := operator(imageB, binary2)
		kv.Spec.InternalAuth = true
		intent.tlsListener = true
		dep := reconcile(r)
		Expect(env(dep)).To(HaveKeyWithValue("BEX_KV_INTERNAL_AUTH", "client"))
		Expect(env(dep)).To(HaveKeyWithValue("BEX_KV_TLS_ADDR", ":6380"))
		Expect(dep.Spec.Template.Spec.Volumes).To(HaveLen(1))
		Expect(dep.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal(keyValueTLSName(kv)))
	})

	It("takes the current rendering on resume", func() {
		reconcile(operator(imageA, binary1))
		held := edit(olderRendering)

		kv.Spec.Suspended = true
		suspended := reconcile(operator(imageB, binary2))
		Expect(suspended.Spec.Replicas).To(HaveValue(BeEquivalentTo(0)))

		kv.Spec.Suspended = false
		r := operator(imageC, binary2)
		resumed := reconcile(r)
		Expect(resumed.Spec.Replicas).To(HaveValue(BeEquivalentTo(1)))
		Expect(resumed.Generation).To(BeNumerically(">", held.Generation))
		expectCurrent(resumed, r)
		expectHeld(reconcile(operator(imageA, binary1)), resumed)
	})

	It("rolls every held proxy once when the override revision is above the recorded one", func() {
		reconcile(operator(imageA, binary1))
		// What a bump of kvInternalProxyRollRevision leaves: a lower record.
		held := edit(func(dep *appsv1.Deployment) {
			dep.Annotations[kvInternalProxyRollRevisionAnnotation] = fmt.Sprint(kvInternalProxyRollRevision - 1)
		})
		r := operator(imageB, binary2)
		rolled := reconcile(r)
		Expect(rolled.Generation).To(BeNumerically(">", held.Generation))
		expectCurrent(rolled, r)
		Expect(rolled.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		expectHeld(reconcile(operator(imageC, binary1)), rolled)

		// A record a newer operator left, before a rollback, stays and holds.
		newer := edit(func(dep *appsv1.Deployment) {
			dep.Annotations[kvInternalProxyRollRevisionAnnotation] = fmt.Sprint(kvInternalProxyRollRevision + 1)
		})
		kept := reconcile(operator(imageA, binary1))
		expectHeld(kept, newer)
		Expect(kept.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision+1)))
	})

	It("adopts a Deployment written before the hold without a roll when its template is current", func() {
		reconcile(operator(imageA, binary1))
		before := edit(beforeM517)
		Expect(before.Annotations).NotTo(HaveKey(kvInternalProxyInputsAnnotation))

		// w6/m469's image hold is part of the comparison: imageA carries binary1.
		adopted := reconcile(operator(imageB, binary1))
		// Recording bumps the generation (the API server copies a Deployment's
		// annotations to its ReplicaSet), but only a template change makes a
		// new ReplicaSet and replaces the pod.
		Expect(equality.Semantic.DeepEqual(adopted.Spec.Template, before.Spec.Template)).To(BeTrue(),
			"the pod template changed: %v", adopted.Spec.Template)
		Expect(adopted.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		expectHeld(reconcile(operator(imageC, binary2)), adopted)
	})

	It("rolls a Deployment written before the hold once when its template predates w6/m512, then holds it", func() {
		reconcile(operator(imageA, binary1))
		preM512 := edit(func(dep *appsv1.Deployment) { olderRendering(dep); beforeM517(dep) })
		Expect(env(preM512)).NotTo(HaveKey("BEX_KV_DRAIN_SECONDS"))

		r := operator(imageB, binary1)
		rolled := reconcile(r)
		Expect(rolled.Generation).To(BeNumerically(">", preM512.Generation))
		Expect(env(rolled)).To(HaveKeyWithValue("BEX_KV_DRAIN_SECONDS", fmt.Sprint(kvInternalProxyDrainSeconds)))
		Expect(rolled.Spec.Template.Spec.TerminationGracePeriodSeconds).To(HaveValue(BeEquivalentTo(kvInternalProxyGraceSeconds)))
		// The adoption kept imageA (same binary) and rolled only the rendering.
		Expect(image(rolled)).To(Equal(imageA))
		expectHeld(reconcile(operator(imageC, binary2)), rolled)
	})

	It("adopts onto the operator's image when the operator has no hash of its binary", func() {
		reconcile(operator(imageA, binary1))
		edit(beforeM517)
		dep := reconcile(operator(imageB, ""))
		Expect(image(dep)).To(Equal(imageB))
		Expect(dep.Annotations).NotTo(HaveKey(kvInternalProxyBinaryAnnotation),
			"a hash left behind would claim imageB carries binary1")
		expectHeld(reconcile(operator(imageA, "")), dep)
	})

	It("adopts a proxy written before the hash existed when its image's registry shows the same binary", func() {
		reconcile(operator(imageA, "")) // production's proxies before w6/m469
		before := edit(beforeM517)
		Expect(before.Annotations).NotTo(HaveKey(kvInternalProxyBinaryAnnotation))

		var asked []string
		r := operator(imageB, binary1)
		r.InternalProxyBinaryOf = func(_ context.Context, image string) (string, error) {
			asked = append(asked, image)
			return binary1, nil
		}
		adopted := reconcile(r)
		Expect(asked).To(Equal([]string{imageA}))
		Expect(equality.Semantic.DeepEqual(adopted.Spec.Template, before.Spec.Template)).To(BeTrue(),
			"the pod template changed: %v", adopted.Spec.Template)
		Expect(adopted.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, binary1))

		reconcile(r)
		Expect(asked).To(HaveLen(1), "a held proxy asked the registry again")
	})

	It("rolls a proxy written before the hash existed when its image's binary differs or is unknown", func() {
		for _, answer := range []struct {
			sum string
			err error
		}{{sum: binary2}, {err: errors.New("registry unreachable")}} {
			reconcile(operator(imageA, ""))
			before := edit(beforeM517)
			r := operator(imageB, binary1)
			r.InternalProxyBinaryOf = func(context.Context, string) (string, error) { return answer.sum, answer.err }
			rolled := reconcile(r)
			Expect(rolled.Generation).To(BeNumerically(">", before.Generation))
			Expect(image(rolled)).To(Equal(imageB))
			Expect(rolled.Annotations).To(HaveKeyWithValue(kvInternalProxyBinaryAnnotation, binary1))
			// Back to a proxy written before w6/m469 for the next answer.
			edit(beforeM517)
			reconcile(operator(imageA, ""))
		}
	})
})
