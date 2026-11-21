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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/065: a service that went live on its image's detected port and was
// switched back to a Git build had its first build probed on the image's port,
// so its startup probe failed and it rolled a second time once detection found
// the port it binds. A release from the other source starts on its own port,
// in both directions, and a release from the same source still has a port its
// process moved to detected (w8/177). A private service is detected without an
// HTTP check (w8/m504); a web service takes the same path with one.
var _ = Describe("A source switch starts from the release's own port (w32/065)", func() {
	const name = "source-switch-port"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	var (
		r       *AppReconciler
		scanner *ipScanner
		run     func()
	)
	BeforeEach(func() {
		scanner = &ipScanner{open: map[string][]int{}}
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			Registry: "zot.test:5000", BuildNamespace: "default",
			PortScanner: scanner, portDetectEvery: time.Nanosecond,
		}
		run = func() {
			GinkgoHelper()
			for range 5 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})
	AfterEach(func() {
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			r.Registry = ""
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}
		_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}},
			client.PropagationPolicy(metav1.DeletePropagationBackground))
		for _, rev := range []string{"gen-2", "gen-3", "gen-4", "gen-5"} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, rev), Namespace: "default"}},
				client.PropagationPolicy(metav1.DeletePropagationBackground))
		}
	})

	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		a := envtestApp(nn)
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		mutate(a)
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
	}
	// deploy is a deploy as bex-api opens one: the release generation moves
	// to the generation the update reaches (a pending save's, when there is one).
	deploy := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		update(func(a *appv1alpha1.App) {
			mutate(a)
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(a.Generation+1, 10)
		})
	}
	toBuild := func(a *appv1alpha1.App) {
		a.Spec.Image, a.Spec.Repo, a.Spec.Branch, a.Spec.Builder = "", "https://github.com/bex-co/hello", "main", "dockerfile"
	}
	toImage := func(a *appv1alpha1.App) {
		a.Spec.Image, a.Spec.Repo, a.Spec.Branch, a.Spec.Builder = "docker.io/library/redis:7-alpine", "", "", ""
	}
	probedOn := func() int32 {
		GinkgoHelper()
		c := envtestDeployment(nn).Spec.Template.Spec.Containers[0]
		port := c.StartupProbe.TCPSocket.Port.IntVal
		for _, probe := range probesOf(c) {
			Expect(probe.TCPSocket.Port.IntVal).To(Equal(port))
		}
		Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "10000"}), "PORT stays the default (w8/267)")
		return port
	}
	deletePod := func(pod string) {
		GinkgoHelper()
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod, Namespace: nn.Namespace}},
			client.GracePeriodSeconds(0)))).To(Succeed())
	}
	// serve lets the Deployment's current template go live on a pod at ip
	// that binds port, after the earlier pods are gone.
	serve := func(pod, ip string, port int, earlier ...string) {
		GinkgoHelper()
		scanner.set(ip, port)
		readyEnvtestPod(nn, pod, ip)
		for _, p := range earlier {
			deletePod(p)
		}
		markEnvtestDeploymentReady(nn)
		run()
		app := envtestApp(nn)
		Expect(meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionReady)).To(BeTrue(), "%+v", app.Status.Conditions)
		Expect(app.Status.ActiveRevision).To(Equal(releaseRevision(app)), "the release in flight serves")
	}
	// detect has the current template's unready pod at ip bind port until
	// the operator adopts it.
	detect := func(pod, ip string, port int) {
		GinkgoHelper()
		scanner.set(ip, port)
		unreadyEnvtestPod(nn, pod, ip)
		run()
		Expect(envtestApp(nn).Status.DetectedPort).To(Equal(int32(port)))
		Expect(probedOn()).To(Equal(int32(port)))
	}

	It("probes each switched release on its own port first, in both directions", func() {
		By("an image service that goes live on the port its image binds")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/library/redis:7-alpine", Type: appv1alpha1.TypePrivateService,
				Port: appv1alpha1.DefaultPort, Replicas: 1,
			},
		})).To(Succeed())
		run()
		Expect(probedOn()).To(Equal(int32(appv1alpha1.DefaultPort)))
		detect(name+"-a", "10.8.0.1", 6379)
		Expect(envtestApp(nn).Status.DetectedPortSource).To(Equal(appv1alpha1.DetectedPortSourceImage))
		serve(name+"-b", "10.8.0.2", 6379, name+"-a")

		By("a save-only switch to a Git build keeps the image serving on its port")
		update(func(a *appv1alpha1.App) {
			toBuild(a)
			a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] = strconv.FormatInt(a.Generation+1, 10)
		})
		run()
		saved := envtestDeployment(nn)
		Expect(saved.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring("redis"))
		Expect(probedOn()).To(Equal(int32(6379)))
		Expect(envtestApp(nn).ServicePort()).To(Equal(int32(6379)))

		By("its deploy builds while the image keeps serving on its port")
		update(func(a *appv1alpha1.App) {
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(a.Generation, 10)
		})
		run()
		Expect(buildJobFor(name, "gen-2")).NotTo(BeNil())
		Expect(probedOn()).To(Equal(int32(6379)), "the image's pod template is untouched while the build runs")
		Expect(envtestApp(nn).Status.DetectedPort).To(Equal(int32(6379)))

		By("the built release is probed on its own 10000, not the image's 6379, and goes live in one roll")
		completeBuildJob(name, "gen-2")
		run()
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(":gen-2"))
		Expect(probedOn()).To(Equal(int32(appv1alpha1.DefaultPort)))
		Expect(envtestApp(nn).Status.DetectedPort).To(Equal(int32(6379)), "the image still serves on 6379")
		rolled := envtestDeployment(nn).Generation
		serve(name+"-c", "10.8.0.3", int(appv1alpha1.DefaultPort), name+"-b")
		app := envtestApp(nn)
		Expect(envtestDeployment(nn).Generation).To(Equal(rolled), "no second roll")
		Expect(probedOn()).To(Equal(int32(appv1alpha1.DefaultPort)))
		Expect(app.Status.DetectedPort).To(BeZero(), "the image's detection goes once the build serves")
		Expect(app.Status.DetectedPortSource).To(BeEmpty())
		Expect(app.ServicePort()).To(Equal(int32(appv1alpha1.DefaultPort)))
		Expect(app.ServingPortDetected()).To(BeFalse(), "its deploy names no detected port")

		By("a build whose process moves to another port still has it detected (w8/177)")
		deploy(func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-21T12:00:00Z" })
		run()
		completeBuildJob(name, "gen-3")
		run()
		Expect(probedOn()).To(Equal(int32(appv1alpha1.DefaultPort)))
		detect(name+"-d", "10.8.0.4", 8000)
		Expect(envtestApp(nn).Status.DetectedPortSource).To(Equal(appv1alpha1.DetectedPortSourceBuild))
		serve(name+"-e", "10.8.0.5", 8000, name+"-c", name+"-d")
		Expect(envtestApp(nn).Status.DetectedPort).To(Equal(int32(8000)))

		By("the next build of the same source starts on the port it found")
		deploy(func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-21T12:10:00Z" })
		run()
		completeBuildJob(name, "gen-4")
		run()
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(":gen-4"))
		Expect(probedOn()).To(Equal(int32(8000)))
		serve(name+"-f", "10.8.0.6", 8000, name+"-e")
		Expect(envtestApp(nn).Status.DetectedPort).To(Equal(int32(8000)))

		By("switched to the image, it is probed on 10000, not the build's 8000, and finds the image's port")
		deploy(toImage)
		run()
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(ContainSubstring("redis"))
		Expect(probedOn()).To(Equal(int32(appv1alpha1.DefaultPort)))
		Expect(envtestApp(nn).Status.DetectedPort).To(Equal(int32(8000)), "the build still serves on 8000")
		detect(name+"-g", "10.8.0.7", 6379)
		serve(name+"-h", "10.8.0.8", 6379, name+"-f", name+"-g")
		app = envtestApp(nn)
		Expect(app.Status.DetectedPort).To(Equal(int32(6379)))
		Expect(app.Status.DetectedPortSource).To(Equal(appv1alpha1.DetectedPortSourceImage))
		Expect(app.ServicePort()).To(Equal(int32(6379)))
	})
})
