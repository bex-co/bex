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
		imageA = "ghcr.io/bex-co/bex-operator@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		imageB = "ghcr.io/bex-co/bex-operator@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		imageC = "ghcr.io/bex-co/bex-operator@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
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

	operator := func(image string) *KeyValueReconciler {
		return &KeyValueReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), InternalProxyImage: image}
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
		for _, key := range []string{kvInternalProxyInputsAnnotation, kvInternalProxyRollRevisionAnnotation} {
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
	// olderRendering drops what w6/m512 and w9/135 added (the drain, the
	// preStop pause and the grace period covering them), as a template
	// written before both.
	olderRendering := func(dep *appsv1.Deployment) {
		c := &dep.Spec.Template.Spec.Containers[0]
		c.Env = slices.DeleteFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "BEX_KV_DRAIN_SECONDS" })
		c.Lifecycle = nil
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
		Expect(dep.Spec.Template.Spec.Containers[0].Lifecycle).To(Equal(&corev1.Lifecycle{PreStop: &corev1.LifecycleHandler{
			Sleep: &corev1.SleepAction{Seconds: kvInternalProxyPauseSeconds},
		}}))
		Expect(dep.Spec.Template.Spec.TerminationGracePeriodSeconds).To(HaveValue(BeEquivalentTo(kvInternalProxyGraceSeconds)))
	}

	It("keeps the template through deploys that change the image, the binary and the rendering", func() {
		first := reconcile(operator(imageA))
		Expect(image(first)).To(Equal(imageA))
		Expect(first.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		expectHeld(reconcile(operator(imageB)), first)
		expectHeld(reconcile(operator(imageC)), first)

		// A template an older rendering wrote: the current one adds a field.
		older := edit(olderRendering)
		for range 2 {
			expectHeld(reconcile(operator(imageC)), older)
		}
		Expect(image(older)).To(Equal(imageA))
	})

	DescribeTable("rolls once onto the manager's image and the whole rendering when the store's own change rolls it",
		func(change func()) {
			reconcile(operator(imageA))
			held := edit(olderRendering)
			r := operator(imageB)
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
		reconcile(operator(imageA))
		r := operator(imageB)
		kv.Spec.InternalAuth = true
		intent.tlsListener = true
		dep := reconcile(r)
		Expect(env(dep)).To(HaveKeyWithValue("BEX_KV_INTERNAL_AUTH", "client"))
		Expect(env(dep)).To(HaveKeyWithValue("BEX_KV_TLS_ADDR", ":6380"))
		Expect(dep.Spec.Template.Spec.Volumes).To(HaveLen(1))
		Expect(dep.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal(keyValueTLSName(kv)))
	})

	It("takes the current rendering on resume", func() {
		reconcile(operator(imageA))
		held := edit(olderRendering)

		kv.Spec.Suspended = true
		suspended := reconcile(operator(imageB))
		Expect(suspended.Spec.Replicas).To(HaveValue(BeEquivalentTo(0)))

		kv.Spec.Suspended = false
		r := operator(imageC)
		resumed := reconcile(r)
		Expect(resumed.Spec.Replicas).To(HaveValue(BeEquivalentTo(1)))
		Expect(resumed.Generation).To(BeNumerically(">", held.Generation))
		expectCurrent(resumed, r)
		expectHeld(reconcile(operator(imageA)), resumed)
	})

	It("rolls every held proxy once when the override revision is above the recorded one", func() {
		reconcile(operator(imageA))
		// What a bump of kvInternalProxyRollRevision leaves: a lower record.
		held := edit(func(dep *appsv1.Deployment) {
			dep.Annotations[kvInternalProxyRollRevisionAnnotation] = fmt.Sprint(kvInternalProxyRollRevision - 1)
		})
		r := operator(imageB)
		rolled := reconcile(r)
		Expect(rolled.Generation).To(BeNumerically(">", held.Generation))
		expectCurrent(rolled, r)
		Expect(rolled.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		expectHeld(reconcile(operator(imageC)), rolled)

		// A record a newer operator left, before a rollback, stays and holds.
		newer := edit(func(dep *appsv1.Deployment) {
			dep.Annotations[kvInternalProxyRollRevisionAnnotation] = fmt.Sprint(kvInternalProxyRollRevision + 1)
		})
		kept := reconcile(operator(imageA))
		expectHeld(kept, newer)
		Expect(kept.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision+1)))
	})

	It("rolls a held proxy whose override record is missing or unreadable", func() {
		for _, record := range []string{"", "one"} {
			reconcile(operator(imageA))
			held := edit(func(dep *appsv1.Deployment) {
				olderRendering(dep)
				dep.Annotations[kvInternalProxyRollRevisionAnnotation] = record
				if record == "" {
					delete(dep.Annotations, kvInternalProxyRollRevisionAnnotation)
				}
			})
			r := operator(imageB)
			rolled := reconcile(r)
			Expect(rolled.Generation).To(BeNumerically(">", held.Generation))
			expectCurrent(rolled, r)
			Expect(rolled.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		}
	})

	It("adopts a Deployment written before the hold without a roll when its template is current", func() {
		reconcile(operator(imageA))
		before := edit(beforeM517)
		Expect(before.Annotations).NotTo(HaveKey(kvInternalProxyInputsAnnotation))

		adopted := reconcile(operator(imageA))
		// Recording bumps the generation (the API server copies a Deployment's
		// annotations to its ReplicaSet), but only a template change makes a
		// new ReplicaSet and replaces the pod.
		Expect(equality.Semantic.DeepEqual(adopted.Spec.Template, before.Spec.Template)).To(BeTrue(),
			"the pod template changed: %v", adopted.Spec.Template)
		Expect(adopted.Annotations).To(HaveKeyWithValue(kvInternalProxyRollRevisionAnnotation, fmt.Sprint(kvInternalProxyRollRevision)))
		expectHeld(reconcile(operator(imageC)), adopted)
	})

	DescribeTable("rolls a Deployment written before the hold once onto the manager's image and rendering, then holds it",
		func(older func(*appsv1.Deployment)) {
			reconcile(operator(imageA))
			before := edit(func(dep *appsv1.Deployment) { older(dep); beforeM517(dep) })

			r := operator(imageB)
			rolled := reconcile(r)
			Expect(rolled.Generation).To(BeNumerically(">", before.Generation))
			expectCurrent(rolled, r)
			expectHeld(reconcile(operator(imageC)), rolled)
		},
		Entry("when it runs another image", func(*appsv1.Deployment) {}),
		Entry("when its template predates w6/m512", olderRendering),
	)
})
