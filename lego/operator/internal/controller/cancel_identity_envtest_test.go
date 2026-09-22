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
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Canceled release identity", func() {
	It("preserves the unfingerprinted legacy image fallback", func() {
		nn := types.NamespacedName{Name: "cancel-legacy-image", Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		app.Status = appv1alpha1.AppStatus{
			Image: "nginx:1", ObservedGeneration: app.Generation, Phase: appv1alpha1.PhaseDeploying,
		}
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		generation := app.Generation
		app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: strconv.FormatInt(generation, 10)}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		for range 2 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.ReleaseGeneration).To(Equal(generation))
		Expect(app.Status.ActiveRevision).To(BeEmpty())
		Expect(k8sClient.Delete(ctx, app)).To(Succeed())
	})

	It("never records a crash-looping first release as successful", func() {
		nn := types.NamespacedName{Name: "cancel-never-ready", Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		pass := func() {
			GinkgoHelper()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		}
		for range 3 {
			pass()
		}
		var dep appsv1.Deployment
		Expect(k8sClient.Get(ctx, nn, &dep)).To(Succeed())
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name + "-pod", Namespace: nn.Namespace, Labels: dep.Spec.Template.Labels},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		pod.Status.Phase = corev1.PodRunning
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name: "app", Image: "nginx:1", ImageID: "nginx:1",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		pass()
		Expect(app.Status.Image).To(Equal("nginx:1"))
		Expect(app.Status.ReleaseFingerprint).NotTo(BeEmpty())
		Expect(app.Status.ActiveRevision).To(BeEmpty())
		ready := meta.FindStatusCondition(app.Status.Conditions, "Ready")
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal("CrashLoopBackOff"))

		app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: strconv.FormatInt(app.Generation, 10)}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		for range 3 {
			pass()
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
			Expect(app.Status.ObservedGeneration).To(Equal(app.Generation))
			Expect(app.Status.ReleaseGeneration).To(BeZero())
			Expect(app.Status.ActiveRevision).To(BeEmpty())
			Expect(meta.IsStatusConditionTrue(app.Status.Conditions, "Ready")).To(BeFalse())
		}
		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		Expect(k8sClient.Delete(ctx, app)).To(Succeed())
	})
})
