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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cancel_build_envtest_test.go is the live m152 DoD shape (2026-09-29): a
// repo-backed service whose config_change deploy is canceled WHILE ITS BUILD
// RUNS. The operator has already dispatched generation 2 — status names it as
// the release in flight — before the cancel lands, which the image-backed specs
// never reach.
var _ = Describe("Canceling a config_change mid-build (w1/m152)", func() {
	const name = "cancel-mid-build"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
		}
	})
	pass := func(n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App {
		GinkgoHelper()
		a := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, a)).To(Succeed())
		return a
	}
	getDep := func() *appsv1.Deployment {
		GinkgoHelper()
		d := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, d)).To(Succeed())
		return d
	}
	AfterEach(func() {
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			saved := r.Registry
			r.Registry = ""
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
			r.Registry = saved
		}
		for _, rev := range []string{"gen-1", "gen-2"} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, rev), Namespace: "default"}})
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: "default"}})
	})

	It("keeps the served release and says the change is not deployed", func() {
		By("serving generation 1, built from the repo, with no environment")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile",
				Port: 3000, Replicas: 1,
			},
		})).To(Succeed())
		pass(3)
		completeBuildJob(name, "gen-1")
		pass(2)
		dep := getDep()
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas = 1, 1
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")
		served := getDep().Spec.Template.DeepCopy()

		By("the first env save: the Secret, envFromSecret and restartedAt in one patch")
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: "default"},
			Data:       map[string][]byte{"MESSAGE": []byte("should-not-ship")},
		})).To(Succeed())
		app := getApp()
		app.Spec.EnvFromSecret = name + "-env"
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())

		By("the operator dispatches generation 2's build before the cancel lands")
		pass(1)
		Expect(buildJobFor(name, "gen-2")).NotTo(BeNil())
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)), "precondition: generation 2 is in flight")

		By("canceling it mid-build")
		app = getApp()
		app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: strconv.FormatInt(app.Generation, 10)}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		pass(3)

		Expect(getDep().Spec.Template).To(Equal(*served), "the served pod template must be untouched — no new pod")
		app = getApp()
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.UndeployedChanges).To(BeTrue(), "the saved change must read as not deployed")
	})
})
