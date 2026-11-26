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
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m671: Render "periodically checks image availability for active
// services. If an image is unavailable, Render can notify you even if your
// running services are unaffected" (render.com/docs/deploying-an-image). The
// app-image-check controller asks the registry for a running image service's
// served image with one manifest HEAD and records ImageAvailable; bex-api
// turns its False transition into one image_pull_failed event.
var _ = Describe("Checking a running image service's image (w4/m671)", func() {
	const repo = "docker.io/traefik/whoami"
	var (
		nn       types.NamespacedName
		r        *AppReconciler
		registry *movableTagRegistry
		now      time.Time
		digest   string
		served   *appsv1.Deployment
	)
	run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
	check := func() time.Duration {
		GinkgoHelper()
		res, err := r.reconcileImageCheck(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return res.RequeueAfter
	}
	// advance moves the check's clock (and the read gate's) past wait, to a
	// whole second as a stored status time is.
	advance := func(wait time.Duration) {
		now = now.Add(wait + time.Second).Truncate(time.Second)
	}
	available := func() *metav1.Condition {
		GinkgoHelper()
		return meta.FindStatusCondition(envtestApp(nn).Status.Conditions, appv1alpha1.ConditionImageAvailable)
	}
	// deploymentUntouched: the check never writes the running Deployment.
	deploymentUntouched := func() {
		GinkgoHelper()
		dep := envtestDeployment(nn)
		Expect(dep.Generation).To(Equal(served.Generation))
		Expect(equality.Semantic.DeepEqual(dep.Spec, served.Spec)).To(BeTrue())
	}

	BeforeEach(func() {
		registry = newMovableTagRegistry()
		digest = registry.push("v1", "A")
		nn = types.NamespacedName{Name: fmt.Sprintf("image-check-%d", GinkgoParallelProcess()), Namespace: "default"}
		now = time.Now().UTC().Truncate(time.Second)
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BaseDomain: "onbex.co", ActivatorService: "bex-activator", ActivatorPort: 8888,
			MaintenanceService: "bex-activator", MaintenancePort: 8888,
			HTTPClient:         registry.client(),
			ImageCheckInterval: DefaultImageCheckInterval,
			imageCheckNow:      func() time.Time { return now },
		}
		r.imageCheckReads().now = func() time.Time { return now }
		r.imageCheckPace.spacing = time.Nanosecond
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Image: repo + ":v1", Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1,
				Replicas: 1, Tier: "starter", Expose: true,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })
		for range 4 {
			_, err := run()
			Expect(err).NotTo(HaveOccurred())
		}
		if app := envtestApp(nn); app.Status.ImageNetwork != nil && app.Status.ImageNetwork.HTTPSelection == "" {
			app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
			Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		}
		finishImagePortRollout(nn, run)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.Image).To(Equal(repo + "@" + digest))
		served = envtestDeployment(nn)
		registry.takeManifestReads()
	})

	It("reads the served digest with one HEAD, records it as served and comes back an interval later", func() {
		wait := check()
		Expect(registry.takeManifestReads()).To(Equal([]string{"HEAD " + digest}))
		app := envtestApp(nn)
		Expect(app.Status.ImageCheck).NotTo(BeNil())
		Expect(app.Status.ImageCheck.Image).To(Equal(repo + "@" + digest))
		Expect(app.Status.ImageCheck.CheckedAt.Time).To(BeTemporally("==", now))
		Expect(app.Status.ImageCheck.Refusals).To(BeZero())
		c := available()
		Expect(c).NotTo(BeNil())
		Expect(c.Status).To(Equal(metav1.ConditionTrue))
		Expect(c.Reason).To(Equal(appv1alpha1.ReasonImageAvailable))
		Expect(wait).To(BeNumerically(">=", DefaultImageCheckInterval*9/10))
		Expect(wait).To(BeNumerically("<", DefaultImageCheckInterval*11/10))

		By("not reading the registry again before the interval")
		now = now.Add(time.Hour)
		Expect(check()).To(BeNumerically("<", wait))
		Expect(registry.takeManifestReads()).To(BeEmpty())
		deploymentUntouched()
	})

	It("reports a deleted image after the second refusal in a row, once per outage, and a recovery clears it", func() {
		advance(check())
		registry.takeManifestReads()
		registry.forget(digest)

		By("a first not-found keeps the condition and checks again within minutes")
		wait := check()
		Expect(registry.takeManifestReads()).To(Equal([]string{"HEAD " + digest}))
		Expect(available().Status).To(Equal(metav1.ConditionTrue))
		Expect(envtestApp(nn).Status.ImageCheck.Refusals).To(Equal(int32(1)))
		Expect(wait).To(BeNumerically("<=", imageCheckConfirm*11/10))

		By("the second turns it False with Render's cause")
		advance(wait)
		check()
		c := available()
		Expect(c.Status).To(Equal(metav1.ConditionFalse))
		Expect(c.Reason).To(Equal(appv1alpha1.ReasonImageNotFound))
		Expect(c.Message).To(Equal(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageNotFound,
			appv1alpha1.DeployReasonParam{Name: "image", Value: repo + "@" + digest})))
		Expect(c.LastTransitionTime.Time).To(BeTemporally("==", now))
		outage := c.LastTransitionTime

		By("the instances keep serving: the Deployment and the phase are untouched")
		deploymentUntouched()
		Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseRunning))

		By("a further refusal keeps the same transition, so no second event")
		wait = check()
		Expect(wait).To(BeNumerically(">=", DefaultImageCheckInterval*9/10))
		advance(wait)
		check()
		Expect(available().LastTransitionTime).To(Equal(outage))
		Expect(envtestApp(nn).Status.ImageCheck.Refusals).To(Equal(int32(3)))

		By("the image pushed again: one served answer turns it True")
		Expect(registry.push("v1", "A")).To(Equal(digest))
		advance(DefaultImageCheckInterval * 11 / 10)
		check()
		c = available()
		Expect(c.Status).To(Equal(metav1.ConditionTrue))
		Expect(c.LastTransitionTime.Time).To(BeTemporally("==", now))
		Expect(envtestApp(nn).Status.ImageCheck.Refusals).To(BeZero())
		deploymentUntouched()
	})

	It("names an image the registry refuses without a credential", func() {
		registry.setAnswer(http.StatusForbidden)
		advance(check())
		check()
		c := available()
		Expect(c.Status).To(Equal(metav1.ConditionFalse))
		Expect(c.Reason).To(Equal(appv1alpha1.ReasonImageAccessRefused))
		Expect(c.Message).To(HavePrefix("the registry refused access to the image " + repo + "@" + digest))
	})

	It("never turns False on a rate limit, a server error or an unreachable registry", func() {
		for _, fail := range []func(){
			func() { registry.setAnswer(http.StatusTooManyRequests) },
			func() { registry.setAnswer(http.StatusServiceUnavailable) },
			func() { registry.setAnswer(0); registry.setUnreachable(true) },
		} {
			fail()
			for range 3 {
				wait := check()
				Expect(wait).To(BeNumerically(">=", imageCheckRetry))
				advance(wait)
			}
			Expect(available()).To(BeNil())
			Expect(envtestApp(nn).Status.ImageCheck).To(BeNil())
		}
		Expect(registry.takeManifestReads()).To(HaveLen(9))
		deploymentUntouched()
	})

	It("does not report an image that is missing once between answers that serve it", func() {
		for range 3 {
			registry.setAnswer(http.StatusNotFound)
			advance(check())
			registry.setAnswer(0)
			advance(check())
		}
		Expect(available().Status).To(Equal(metav1.ConditionTrue))
		Expect(envtestApp(nn).Status.ImageCheck.Refusals).To(BeZero())
	})

	It("never checks a suspended service or an image in bex's own registry", func() {
		app := envtestApp(nn)
		app.Spec.Suspended = true
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		Expect(check()).To(BeZero())

		app = envtestApp(nn)
		app.Spec.Suspended = false
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		r.Registry = "zot.bex-registry.svc:5000"
		app = envtestApp(nn)
		app.Status.Image = r.Registry + "/" + appIdentity(app).Repo() + "@" + digest
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		Expect(check()).To(BeZero())

		Expect(registry.takeManifestReads()).To(BeEmpty())
		Expect(envtestApp(nn).Status.ImageCheck).To(BeNil())
		Expect(available()).To(BeNil())
	})

	It("forgets the outage of an image that no longer serves", func() {
		registry.setAnswer(http.StatusNotFound)
		advance(check())
		check()
		Expect(available().Status).To(Equal(metav1.ConditionFalse))

		app := envtestApp(nn)
		app.Status.Phase = appv1alpha1.PhaseDeploying
		app.Status.Image = repo + ":v2"
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		Expect(check()).To(BeZero())
		Expect(registry.takeManifestReads()).To(HaveLen(2))
		Expect(envtestApp(nn).Status.ImageCheck).To(BeNil())
		Expect(available()).To(BeNil())
	})

	It("keeps its answer when a runtime status write retries over it", func() {
		stale := envtestApp(nn)
		check()
		Expect(available()).NotTo(BeNil())
		stale.Status.URL = "https://retried.onbex.co"
		r.updateStatusRetrying(ctx, stale, "test")
		app := envtestApp(nn)
		Expect(app.Status.URL).To(Equal("https://retried.onbex.co"))
		Expect(app.Status.ImageCheck).NotTo(BeNil())
		Expect(meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionImageAvailable)).NotTo(BeNil())
		Expect(strings.Contains(app.Status.ImageCheck.Image, digest)).To(BeTrue())
	})
})
