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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Saved configuration notification", func() {
	It("observes existing-file saves through the manager without changing runtime or release identity", func() {
		ctx := context.Background()
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "saved-file-notification", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1, FilesFromSecrets: []string{"saved-file-notification-files"}},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, app))).To(Succeed()) })
		source := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: app.Name + "-files", Namespace: app.Namespace}, Data: map[string][]byte{"message": []byte("v1")}}
		Expect(k8sClient.Create(ctx, source)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, source))).To(Succeed()) })
		app.Status.ReleaseGeneration = app.Generation
		app.Status.Image = app.Spec.Image
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		Expect(r.ensureReleaseConfigSnapshot(ctx, app)).To(Succeed())
		app.Status.ActiveRevision = releaseRevision(app)
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		applyDeploymentSpec(dep, app, deploymentParams{image: app.Spec.Image, port: 3000, replicas: 1})
		Expect(controllerutil.SetControllerReference(app, dep, k8sClient.Scheme())).To(Succeed())
		Expect(k8sClient.Create(ctx, dep)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, dep))).To(Succeed()) })
		Expect(r.recordReleasePodTemplate(ctx, app, dep.Spec.Template)).To(Succeed())
		for _, name := range []string{appv1alpha1.ReleaseRecordName(app.Name, app.Generation), appv1alpha1.AppReleaseSnapshotName(app.Name, source.Name, app.Generation)} {
			DeferCleanup(func() {
				Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}))).To(Succeed())
			})
		}
		beforeApp, beforeDeployment := app.DeepCopy(), dep.DeepCopy()
		// Other specs start the same controller in independent managers.
		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		statusReconciler := &AppReconciler{Client: mgr.GetClient(), BuildClient: k8sClient, Scheme: mgr.GetScheme(), Mode: ModeKubernetes}
		Expect(statusReconciler.setupSavedConfigurationStatus(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		stopped := make(chan error, 1)
		go func() { stopped <- mgr.Start(managerCtx) }()
		DeferCleanup(func() { stop(); Eventually(stopped, 10*time.Second).Should(Receive(BeNil())) })
		Expect(mgr.GetCache().WaitForCacheSync(managerCtx)).To(BeTrue())

		for _, step := range []struct {
			value   string
			pending bool
		}{{"v2", true}, {"v1", false}, {"v3", true}} {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(source), source)).To(Succeed())
			source.Data["message"] = []byte(step.value)
			Expect(k8sClient.Update(ctx, source)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), app)).To(Succeed())
			base := client.MergeFrom(app.DeepCopy())
			app.Annotations = map[string]string{appv1alpha1.AnnotationSavedConfigRevision: step.value}
			Expect(k8sClient.Patch(ctx, app, base)).To(Succeed())
			Eventually(func(g Gomega) {
				current := &appv1alpha1.App{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), current)).To(Succeed())
				g.Expect(current.Status.UndeployedChanges).To(Equal(step.pending))
				g.Expect(current.Generation).To(Equal(beforeApp.Generation))
				g.Expect(current.Spec).To(Equal(beforeApp.Spec))
				g.Expect(current.Status.ActiveRevision).To(Equal(beforeApp.Status.ActiveRevision))
				g.Expect(current.Status.ReleaseGeneration).To(Equal(beforeApp.Status.ReleaseGeneration))
			}, 8*time.Second, 50*time.Millisecond).Should(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dep), dep)).To(Succeed())
			Expect(dep.ResourceVersion).To(Equal(beforeDeployment.ResourceVersion))
			Expect(dep.Spec.Template).To(Equal(beforeDeployment.Spec.Template))
			snapshot := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.AppReleaseSnapshotName(app.Name, source.Name, app.Generation)}, snapshot)).To(Succeed())
			Expect(string(snapshot.Data["message"])).To(Equal("v1"))
		}
	})
})
