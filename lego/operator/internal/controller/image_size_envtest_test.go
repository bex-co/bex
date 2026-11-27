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
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/734: Render refuses a prebuilt image whose compressed size is over
// 10 GB ("The Docker image's compressed size cannot exceed 10 GB",
// render.com/docs/deploying-an-image § Image size). bex refuses it at the
// image metadata read, before any node pulls it, under ImagePullFailed (so
// bex-api records image_pull_failed), and a release that served keeps serving.
var _ = Describe("A prebuilt image over 10 GB compressed (w4/734)", func() {
	const repo = "docker.io/traefik/whoami"
	var (
		nn       types.NamespacedName
		r        *AppReconciler
		registry *movableTagRegistry
		digestA  string
	)
	run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
	pass := func() {
		GinkgoHelper()
		for range 4 {
			_, err := run()
			Expect(err).NotTo(HaveOccurred())
		}
	}
	goLive := func() {
		GinkgoHelper()
		if app := envtestApp(nn); app.Status.ImageNetwork != nil && app.Status.ImageNetwork.HTTPSelection == "" {
			// Stand in for the completed HTTP port probe (w8/m142).
			app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
			Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
			pass()
		}
		finishImagePortRollout(nn, run)
		pass()
	}
	// deploy stamps what bex-api's deploy of a saved image does: a release
	// generation and a restart stamp.
	deploy := func(image string) int64 {
		GinkgoHelper()
		app := envtestApp(nn)
		gen := app.Generation + 1
		app.Spec.Image = image
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(gen, 10))
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		pass()
		return gen
	}
	newReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BaseDomain: "onbex.co", ActivatorService: "bex-activator", ActivatorPort: 8888,
			MaintenanceService: "bex-activator", MaintenancePort: 8888,
			HTTPClient: registry.client(),
		}
	}

	BeforeEach(func() {
		registry = newMovableTagRegistry()
		digestA = registry.pushSized("v1", "A", 1_000_000_000)
		nn = types.NamespacedName{Name: fmt.Sprintf("image-size-%d", GinkgoParallelProcess()), Namespace: "default"}
		r = newReconciler()
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Image: repo + ":v1", Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1,
				Replicas: 1, Tier: "starter", Expose: true,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })

		By("serving :v1, a 1 GB image")
		pass()
		goLive()
		Expect(envtestApp(nn).Status.Image).To(Equal(repo + "@" + digestA))
	})

	It("fails the deploy of an image one byte over 10 GB before any pull, while the live release serves", func() {
		registry.pushSized("huge", "B", 6_000_000_000, 4_000_000_001)
		served := envtestDeployment(nn).Spec.Template.DeepCopy()
		gen := deploy(repo + ":huge")

		app := envtestApp(nn)
		rollout := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
		Expect(rollout).NotTo(BeNil())
		Expect(rollout.Status).To(Equal(metav1.ConditionFalse))
		Expect(rollout.Reason).To(Equal(appv1alpha1.ReasonImagePullFailed))
		Expect(rollout.ObservedGeneration).To(Equal(gen))
		Expect(rollout.Message).To(Equal(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageTooLarge,
			appv1alpha1.DeployReasonParam{Name: "image", Value: repo + ":huge"},
			appv1alpha1.DeployReasonParam{Name: "size", Value: "10.01"},
		)))
		Expect(rollout.Message).To(HavePrefix("the image " + repo + ":huge is 10.01 GB compressed, over the 10 GB limit"))
		Expect(app.Status.Phase).NotTo(Equal(appv1alpha1.PhaseDeploying))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.Image).To(Equal(repo + "@" + digestA))
		// No pod template ever named the oversized image, so no node pulls it.
		Expect(equality.Semantic.DeepEqual(*served, envtestDeployment(nn).Spec.Template)).To(BeTrue())

		By("the verdict is final: later passes do not read the registry again")
		registry.takeManifestReads()
		pass()
		Expect(registry.takeManifestReads()).To(BeEmpty())
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"))
	})

	It("deploys an image of exactly 10 GB as before", func() {
		digestB := registry.pushSized("big", "C", 6_000_000_000, 4_000_000_000)
		gen := deploy(repo + ":big")
		goLive()
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
		Expect(app.Status.Image).To(Equal(repo + "@" + digestB))
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(Equal(repo + "@" + digestB))
	})

	It("fails a new private service whose first image is over the limit, with no Deployment", func() {
		registry.pushSized("huge", "B", 12_345_678_901)
		private := types.NamespacedName{Name: fmt.Sprintf("image-size-private-%d", GinkgoParallelProcess()), Namespace: "default"}
		pr := newReconciler()
		runPrivate := func() (ctrl.Result, error) { return pr.Reconcile(ctx, reconcile.Request{NamespacedName: private}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: private.Name, Namespace: private.Namespace,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Image: repo + ":huge", Type: appv1alpha1.TypePrivateService, PortMode: appv1alpha1.PortModeImageV1,
				Replicas: 1, Tier: "starter",
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(private), runPrivate) })

		var err error
		for range 3 {
			_, err = runPrivate()
		}
		Expect(err).To(HaveOccurred())
		app := envtestApp(private)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(appv1alpha1.ReasonImagePullFailed))
		Expect(ready.Message).To(ContainSubstring("the image " + repo + ":huge is 12.35 GB compressed"))
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, private, &appsv1.Deployment{}))).To(BeTrue())
	})
})
