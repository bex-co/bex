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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cancel_config_change_envtest_test.go reproduces m152's filed defect end to end:
// a saved config change whose deploy is canceled must not roll a pod.
//
// Live on 2026-09-14 the save rewrote `<name>-env` and bumped spec.restartedAt,
// the deploy was canceled while building, and a new pod appeared 4 s later
// serving the canceled value with no deploy row recording it. This models what the
// operator sees in that window: generation 2 (the save) plus the cancel for it,
// before generation 2 ever reached the Deployment.
var _ = Describe("Canceling a config-change deploy (w1/m152)", func() {
	const name = "cancel-config-change"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})

	pass := func() {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	getApp := func() *appv1alpha1.App {
		GinkgoHelper()
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		return app
	}
	getDep := func() *appsv1.Deployment {
		GinkgoHelper()
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		return dep
	}
	markReady := func() {
		GinkgoHelper()
		dep := getDep()
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas = 1, 1
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: "default"}})
	})

	It("leaves the serving pod template exactly as it was", func() {
		By("serving generation 1 with MESSAGE=OK")
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: "default"},
			Data:       map[string][]byte{"MESSAGE": []byte("OK")},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image: "nginx:1", Port: 3000, Replicas: 1, EnvFromSecret: name + "-env",
			},
		})).To(Succeed())
		for range 3 {
			pass()
		}
		markReady()
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")
		served := getDep().Spec.Template.DeepCopy()

		By("saving MESSAGE=should-not-ship, which rewrites the Secret and bumps restartedAt")
		env := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-env", Namespace: "default"}, env)).To(Succeed())
		env.Data = map[string][]byte{"MESSAGE": []byte("should-not-ship")}
		Expect(k8sClient.Update(ctx, env)).To(Succeed())
		app := getApp()
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())

		By("canceling that deploy before generation 2 reaches the Deployment")
		app = getApp()
		app.Annotations = map[string]string{
			appv1alpha1.AnnotationCanceledReleaseGeneration: strconv.FormatInt(app.Generation, 10),
		}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		for range 3 {
			pass()
		}

		By("the Deployment still carries the served template: no rollout, no new pod")
		Expect(getDep().Spec.Template).To(Equal(*served),
			"a canceled config change must not change the pod template — that is what rolled a pod live")

		By("the release still reads as the served one")
		app = getApp()
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))

		By("the saved change stays saved for the next deploy, and the service says so")
		Expect(app.Spec.RestartedAt).NotTo(BeEmpty())
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name + "-env", Namespace: "default"}, env)).To(Succeed())
		Expect(env.Data).To(HaveKeyWithValue("MESSAGE", []byte("should-not-ship")))
		Expect(app.Status.UndeployedChanges).To(BeTrue(),
			"the service is running an earlier release than its saved spec, and every surface reads this (t003)")

		By("the next deploy ships the saved change and clears the flag")
		app.Spec.RestartedAt = time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		for range 3 {
			pass()
		}
		app = getApp()
		Expect(app.Status.UndeployedChanges).To(BeFalse())
		Expect(app.Status.ReleaseGeneration).To(BeNumerically(">", 1))
		shipped := getDep().Spec.Template
		Expect(shipped).NotTo(Equal(*served), "the next deploy must actually roll")
		snap := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      appv1alpha1.ReleaseSnapshotName(name+"-env", app.Status.ReleaseGeneration),
			Namespace: "default",
		}, snap)).To(Succeed())
		Expect(snap.Data).To(HaveKeyWithValue("MESSAGE", []byte("should-not-ship")),
			"the new release snapshots and serves the value that was saved before the cancel")
		Expect(shipped.Spec.Containers[0].EnvFrom[0].SecretRef.Name).To(Equal(snap.Name))
	})
})
