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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Update Service saves and deploys nothing (w8/m135): bex-api holds a
// Settings PATCH for the next deploy with the pending marker. The release
// serving keeps running the settings it was adopted with — probe, shutdown
// delay, command — and runs no pre-deploy; the next deploy applies every held
// field together. Before, the operator projected the saved spec straight into
// the template, so a held field rolled the pods with no deploy.
var _ = Describe("Saved settings wait for the next deploy (w8/m135)", func() {
	const name = "save-only-settings"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func(n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App { GinkgoHelper(); return envtestApp(nn) }
	template := func() *corev1.PodTemplateSpec {
		GinkgoHelper()
		return envtestDeployment(nn).Spec.Template.DeepCopy()
	}
	appContainer := func(t *corev1.PodTemplateSpec) corev1.Container {
		GinkgoHelper()
		c := findContainer(t.Spec.Containers, appContainerName)
		Expect(c).NotTo(BeNil())
		return *c
	}
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	// saveOnly is a Render Update Service PATCH as bex-api writes it: the
	// fields and the pending marker for the generation the write produces.
	saveOnly := func(mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
	}
	// deploy is a deploy verb: it stamps the generation it opens.
	deploy := func() int64 {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
		})
		return getApp().Generation
	}
	undeployed := func() bool {
		GinkgoHelper()
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return getApp().Status.UndeployedChanges
	}
	preDeployJob := func(gen int64) (*batchv1.Job, error) {
		job := &batchv1.Job{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen))}, job)
		return job, err
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		background := metav1.DeletePropagationBackground
		for gen := range 8 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen)), Namespace: "default"}},
				&client.DeleteOptions{PropagationPolicy: &background})
		}
	})

	It("holds every saved field until a deploy, which applies them together", func() {
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, HealthCheckPath: "/hz"},
		})).To(Succeed())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 serves")
		Expect(undeployed()).To(BeFalse())
		served := template()

		By("save-only PATCHes of the probe, the shutdown delay and the command")
		// The shutdown delay is outside the record's selected fields: only the
		// served release's settings fingerprint can tell it is not deployed.
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.MaxShutdownDelaySeconds = new(int32(40)) })
		pass(2)
		Expect(undeployed()).To(BeTrue(), "a saved shutdown delay is reported as not yet deployed")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.HealthCheckPath = "/hz2" })
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.StartCommand = "node index.js" })
		pass(3)
		app := getApp()
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)), "a save opens no release")
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue(),
			"the serving pods must keep the settings they were deployed with")

		By("a save-only PATCH of the pre-deploy command runs nothing")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.PreDeployCommand = "echo x" })
		pass(3)
		app = getApp()
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		_, err := preDeployJob(app.Generation)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a save runs no pre-deploy command")
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue())
		Expect(undeployed()).To(BeTrue(), "the saved settings are reported as not yet deployed")

		By("a deploy that is canceled leaves the served release and the saved settings pending")
		canceled := deploy()
		pass(2)
		_, err = preDeployJob(canceled)
		Expect(err).NotTo(HaveOccurred(), "the deploy runs the saved pre-deploy command")
		update(func(app *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, fmt.Sprint(canceled))
		})
		pass(3)
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue(), "the cancel keeps the served release")
		Expect(undeployed()).To(BeTrue(), "a canceled deploy leaves the saved settings pending")

		By("the next deploy applies every saved field together")
		next := deploy()
		pass(2)
		job, err := preDeployJob(next)
		Expect(err).NotTo(HaveOccurred())
		completeEnvtestJob(job, metav1.Now())
		pass(3)
		applied := template()
		c := appContainer(applied)
		Expect(probedHealthCheckPath(c)).To(HaveValue(Equal("/hz2")))
		Expect(fmt.Sprint(c.Command, c.Args)).To(ContainSubstring("node index.js"))
		Expect(*applied.Spec.TerminationGracePeriodSeconds).To(Equal(*served.Spec.TerminationGracePeriodSeconds+10),
			"the 40-second shutdown delay replaces the 30-second default")
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
		Expect(undeployed()).To(BeFalse(), "the deploy cleared the pending settings")

		By("a later save of the same values is no change")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.HealthCheckPath = "/hz2" })
		pass(2)
		Expect(undeployed()).To(BeFalse())
	})
})
