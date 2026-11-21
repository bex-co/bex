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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's environment page separates "Save and deploy", which deploys the
// existing build with the saved variables, from "Save, rebuild, and deploy"
// (render.com/docs/configure-environment-variables). A build command saved
// without a deploy therefore waits through a "Save and deploy" and through an
// environment group's deploy, both of which rerun the serving build (w4/m180,
// w4/708), while the start command and variables they run read deployed
// (w32/048). The rerun's record named the saved build settings, so the pending
// build command read as deployed once the rerun served (w32/060).
var _ = Describe("An environment Save and deploy keeps a saved build setting undeployed (w32/060)", func() {
	const name = "rerun-build-settings"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	envSecret, groupSecret := name+"-env", "evg-"+name+"-env"
	var r *AppReconciler
	BeforeEach(func() {
		// A registry, so the served image is a reference bex-api can select.
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, Registry: "zot.test:5000"}
	})
	pass := func(n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App { GinkgoHelper(); return envtestApp(nn) }
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	setSecret := func(secretName, value string) {
		GinkgoHelper()
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: "default"}}
		_, err := ctrl.CreateOrUpdate(ctx, k8sClient, s, func() error {
			s.Data = map[string][]byte{"QA_MARK": []byte(value)}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	}
	// saveOnly saves a setting as a Render Update Service PATCH does, without
	// a deploy (w8/m135).
	saveOnly := func(mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
		pass(2)
	}
	// rerun is a deploy that reruns the serving build, as bex-api writes an
	// environment "Save and deploy" or a group link's deploy: mutate applies
	// the environment change, the release bumps restartedAt and selects the
	// image the serving release runs (rollout.SelectReleaseImage, from the
	// deploy row's resolved image, the operator's status.image), and the
	// deploy stamps its generation.
	rerun := func(mutate func(*appv1alpha1.AppSpec)) int64 {
		GinkgoHelper()
		image := getApp().Status.Image
		Expect(appContainerImage(envtestDeployment(nn).Spec.Template)).To(Equal(image), "precondition: the serving release runs status.image")
		update(func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			gen := app.Generation + 1
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: gen, Image: image}
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(gen))
		})
		return getApp().Generation
	}
	// deploy is a standard deploy, which builds the saved settings.
	deploy := func() int64 {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			app.Spec.ReleaseConfig = nil
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
		})
		return getApp().Generation
	}
	// serve rolls gen out to live, completing its build when it builds; a
	// rerun dispatches none.
	serve := func(gen int64, builds bool) {
		GinkgoHelper()
		pass(3)
		if builds {
			completeBuildJob(name, fmt.Sprintf("gen-%d", gen))
			pass(2)
		} else {
			_, err := buildJobFor(name, fmt.Sprintf("gen-%d", gen))
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the rerun builds nothing")
		}
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
	}
	// expectUndeployed runs the saved-configuration status and reads it.
	expectUndeployed := func(want bool, reason, why string) {
		GinkgoHelper()
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		status := getApp().Status
		Expect(status.UndeployedChanges).To(Equal(want), why)
		Expect(status.UndeployedChangesReason).To(Equal(reason), why)
	}

	AfterEach(func() {
		r.Registry = "" // nothing to delete from a registry that does not exist
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		background := client.PropagationPolicy(metav1.DeletePropagationBackground)
		// envtest runs no garbage collector.
		_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}, background)
		for _, s := range []string{envSecret, groupSecret} {
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: s, Namespace: "default"}})
		}
		for gen := range 16 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, fmt.Sprintf("gen-%d", gen)), Namespace: "default"}}, background)
		}
	})

	It("reads a saved build command undeployed through a Save and deploy and a group link until a deploy builds it", func() {
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/hello", Branch: "main", Runtime: "node", Builder: build.BuilderNative,
				BuildCommand: "npm ci", StartCommand: "node server.js", Port: 3000, Replicas: 1, Tier: "starter"},
		})).To(Succeed())
		serve(1, true)
		expectUndeployed(false, "", "the first deploy is what is saved")

		By("a Save and deploy of the service's first variable, with nothing else saved")
		setSecret(envSecret, "v1")
		serve(rerun(func(s *appv1alpha1.AppSpec) { s.EnvFromSecret = envSecret }), false)
		expectUndeployed(false, "", "the rerun deploys the variable, and nothing else is saved")

		By("a saved build command waits through a Save and deploy")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "npm run build" })
		expectUndeployed(true, appv1alpha1.UndeployedReasonSaved, "the saved build command is not deployed")
		setSecret(envSecret, "v2")
		serve(rerun(func(*appv1alpha1.AppSpec) {}), false)
		expectUndeployed(true, appv1alpha1.UndeployedReasonSaved, "the Save and deploy runs the serving build, not the saved build command")

		By("and through an environment group's deploy")
		setSecret(groupSecret, "g1")
		serve(rerun(func(s *appv1alpha1.AppSpec) { s.EnvFromSecrets = append(s.EnvFromSecrets, groupSecret) }), false)
		expectUndeployed(true, appv1alpha1.UndeployedReasonSaved, "the group's deploy runs the serving build, not the saved build command")

		By("the build command the reruns run, put back")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "npm ci" })
		expectUndeployed(false, "", "the reruns run the build command now saved")

		By("a saved start command, which a Save and deploy runs (w32/048)")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.StartCommand = "node other.js" })
		expectUndeployed(true, appv1alpha1.UndeployedReasonSaved, "the saved start command is not deployed")
		serve(rerun(func(*appv1alpha1.AppSpec) {}), false)
		expectUndeployed(false, "", "the rerun runs the saved start command on the serving build")

		By("the next deploy builds the saved build command")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "npm run build" })
		expectUndeployed(true, appv1alpha1.UndeployedReasonSaved, "the saved build command is not deployed")
		serve(deploy(), true)
		expectUndeployed(false, "", "the deploy built the saved settings")
	})
})
