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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// envtest has no kubelet or Deployment controller. These specs write their
// observed readiness/deadline status, then exercise the real reconcile and API
// server, including release snapshots and selected historical configuration.
var _ = Describe("Image-port recovery after removing PORT (w8/210)", func() {
	DescribeTable("recovers the default port after a failed same-image candidate", func(rollback, keepSavedPORT bool) {
		name := "removed-port-fresh"
		if rollback {
			name = "removed-port-rollback"
		}
		if keepSavedPORT {
			name += "-saved"
		}
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			HTTPClient: imageConfigRegistry(GinkgoT(), `"80/tcp":{}`),
		}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		settle := func() { GinkgoHelper(); reconcileToFixedPoint(run) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/traefik/whoami:v1.10.3", Type: appv1alpha1.TypeWebService,
				PortMode: appv1alpha1.PortModeWebImageV1, Replicas: 1,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })

		By("serving the image's declared port 80")
		settle()
		app := envtestApp(nn)
		Expect(app.Status.ImageNetwork.PrimaryPort).To(Equal(int32(80)))
		// Stand in for the completed initial HTTP probe. Later recovery must
		// retain this verified serving choice without any test-side correction.
		app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		settle()
		markEnvtestDeploymentReady(nn)
		settle()
		app = envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.ServingNetwork.PrimaryPort).To(Equal(int32(80)))
		image := app.Status.ServingNetwork.Image
		assertImagePortRuntime(nn, 80)

		By("failing PORT=9999 while the same digest keeps serving on 80")
		app.Spec.Image, app.Spec.Env = image, []appv1alpha1.EnvVar{{Name: "PORT", Value: "9999"}}
		app.Spec.RestartedAt = "2026-10-28T01:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		settle()
		app = envtestApp(nn)
		Expect(app.Status.EnvPort).To(Equal(int32(9999)))
		Expect(app.Status.ImageNetwork.PrimaryPort).To(Equal(int32(9999)))
		Expect(app.Status.ImageNetwork.Image).To(Equal(image))
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].StartupProbe.TCPSocket.Port.IntVal).To(Equal(int32(9999)))
		assertImagePortService(nn, 80)
		failImagePortCandidate(nn)
		settle()
		app = envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(meta.IsStatusConditionFalse(app.Status.Conditions, appv1alpha1.ConditionRollout)).To(BeTrue())

		By("requesting recovery with the selected release's own PORT configuration")
		if !keepSavedPORT {
			app.Spec.Env = nil
		}
		app.Spec.RestartedAt = "2026-10-28T02:00:00Z"
		if rollback {
			generation := app.Generation + 1
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: generation, SourceGeneration: 1, Image: image, PreserveGroupValues: true}
			app.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: strconv.FormatInt(generation, 10)}
		}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		if keepSavedPORT {
			_, err := run()
			Expect(err).NotTo(HaveOccurred())
			Expect(envtestApp(nn).Status.EnvPort).To(BeZero(), "the first rollback pass must read the source record before its new copy exists")
			assertImagePortRuntime(nn, 80)
		}
		settle()
		app = envtestApp(nn)
		Expect(app.Status.EnvPort).To(BeZero(), "the release has no user PORT")
		if keepSavedPORT {
			Expect(app.Spec.Env).To(Equal([]appv1alpha1.EnvVar{{Name: "PORT", Value: "9999"}}), "rollback must leave saved settings unchanged")
		}
		Expect(app.Status.ImageNetwork.PrimaryPort).To(Equal(int32(80)))
		Expect(app.Status.ImageNetwork.ConfiguredPort).To(Equal(new(int32(0))), "zero provenance must survive the API server")
		assertImagePortRuntime(nn, 80)
		assertImagePortService(nn, 80)
		finishImagePortRollout(nn, run)
		app = envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", app.Status.ReleaseGeneration)))
		Expect(meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionReady)).To(BeTrue(), "recovery must finish, not retain the failed candidate's verdict")
		Expect(app.Status.ServingNetwork.PrimaryPort).To(Equal(int32(80)))
		assertImagePortService(nn, 80)
		if keepSavedPORT {
			By("keeping the selected PORT after its source release record is collected")
			Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: appv1alpha1.ReleaseRecordName(name, 1), Namespace: nn.Namespace,
			}})).To(Succeed())
			settle()
			Expect(envtestApp(nn).Status.EnvPort).To(BeZero())
			Expect(envtestApp(nn).Spec.Env).To(Equal([]appv1alpha1.EnvVar{{Name: "PORT", Value: "9999"}}))
			assertImagePortRuntime(nn, 80)
			assertImagePortService(nn, 80)

			By("using the still-saved PORT on the next ordinary deploy")
			app = envtestApp(nn)
			app.Spec.ReleaseConfig = nil
			delete(app.Annotations, appv1alpha1.AnnotationReleaseGeneration)
			app.Spec.RestartedAt = "2026-10-28T05:00:00Z"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			settle()
			Expect(envtestApp(nn).Status.EnvPort).To(Equal(int32(9999)))
			Expect(envtestApp(nn).Status.ImageNetwork.PrimaryPort).To(Equal(int32(9999)))
			assertImagePortRuntime(nn, 9999)
			assertImagePortService(nn, 80)
		}
	}, Entry("a fresh deploy", false, false),
		Entry("a rollback selecting the original release's configuration", true, false),
		Entry("a rollback while the failed PORT remains saved", true, true))

	It("moves PORT from 8080 to 9090 and returns to the image port when removed", func() {
		nn := types.NamespacedName{Name: "image-port-explicit-change", Namespace: "default"}
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			HTTPClient: imageConfigRegistry(GinkgoT(), `"80/tcp":{}`),
		}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/ealen/echo-server:0.9.2", Type: appv1alpha1.TypeWebService,
				PortMode: appv1alpha1.PortModeWebImageV1, Replicas: 1,
				Env: []appv1alpha1.EnvVar{{Name: "PORT", Value: "8080"}},
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })
		reconcileToFixedPoint(run)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		assertImagePortRuntime(nn, 8080)
		assertImagePortService(nn, 8080)
		app := envtestApp(nn)
		app.Spec.Image = app.Status.ServingNetwork.Image
		app.Spec.Env[0].Value, app.Spec.RestartedAt = "9090", "2026-10-28T03:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		assertImagePortRuntime(nn, 9090)
		assertImagePortService(nn, 8080)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		assertImagePortService(nn, 9090)
		finishImagePortRollout(nn, run)

		By("removing PORT from a process that previously honored it")
		app = envtestApp(nn)
		app.Spec.Env, app.Spec.RestartedAt = nil, "2026-10-28T04:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		Expect(app.Status.EnvPort).To(BeZero())
		Expect(app.Status.ImageNetwork.PrimaryPort).To(Equal(int32(80)), "fresh metadata must replace the injected 9090 before discovery")
		Expect(app.Status.ImageNetwork.ConfiguredPort).To(Equal(new(int32(0))))
		Expect(app.Status.ImageNetwork.HTTPSelection).To(BeEmpty())
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].StartupProbe.HTTPGet.Port.IntVal).To(Equal(int32(80)))
		assertImagePortService(nn, 9090)
		app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		assertImagePortRuntime(nn, 80)
		finishImagePortRollout(nn, run)
		app = envtestApp(nn)
		Expect(app.Status.ImageNetwork.ConfiguredPort).To(Equal(new(int32(0))))
		Expect(app.Status.ServingNetwork.ConfiguredPort).To(Equal(new(int32(0))))
		assertImagePortRuntime(nn, 80)
		assertImagePortService(nn, 80)
	})
})

func assertImagePortRuntime(nn types.NamespacedName, port int32) {
	GinkgoHelper()
	c := envtestDeployment(nn).Spec.Template.Spec.Containers[0]
	for _, probe := range probesOf(c) {
		Expect(probe).NotTo(BeNil())
		Expect(probe.TCPSocket).NotTo(BeNil())
		Expect(probe.TCPSocket.Port.IntVal).To(Equal(port))
	}
	Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: fmt.Sprint(port)}))
}

func assertImagePortService(nn types.NamespacedName, port int32) {
	GinkgoHelper()
	svc := &corev1.Service{}
	Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
	Expect(svc.Spec.Ports).To(ContainElement(And(HaveField("Port", port), HaveField("TargetPort.IntVal", port))))
}

func failImagePortCandidate(nn types.NamespacedName) {
	GinkgoHelper()
	dep := envtestDeployment(nn)
	dep.Status.ObservedGeneration = dep.Generation
	dep.Status.Replicas, dep.Status.UpdatedReplicas = 2, 1
	dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
	dep.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse, Reason: reasonProgressDeadlineExceeded}}
	Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
}

func finishImagePortRollout(nn types.NamespacedName, run func() (ctrl.Result, error)) {
	GinkgoHelper()
	// Completing a port change also releases minReadySeconds. Reflect the
	// Deployment controller observing that final spec update before declaring
	// readiness, as a real cluster does.
	for range 2 {
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
	}
	app := envtestApp(nn)
	Expect(app.Status.ActiveRevision).To(Equal(releaseRevision(app)))
	Expect(meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionReady)).To(BeTrue(), "%+v", app.Status.Conditions)
}
