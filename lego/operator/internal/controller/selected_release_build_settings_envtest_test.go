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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Restart runs "the exact same Git commit and configuration as the running
// instance" (render.com/docs/deploys), and a rollback reuses its target's build
// artifact and settings while Render's next standard deploy "uses the service's
// current configuration" (render.com/docs/rollbacks). A build command or branch
// saved without a deploy therefore waits through either, and the target's put
// back waits for nothing. The selection's record named the saved build
// settings: the pending one read as deployed once the selection served, and the
// target's put back as undeployed (w32/058).
var _ = Describe("A Restart or rollback keeps a saved build setting undeployed (w32/058)", func() {
	const name = "selected-build-settings"
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
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	// saveOnly saves a build setting as a Render Update Service PATCH does,
	// without a deploy (w8/m135).
	saveOnly := func(mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
		pass(2)
	}
	// release stamps a deploy verb's generation: a standard deploy, or for a
	// positive from a Restart or rollback selecting that release, as bex-api's
	// targetReleaseConfig does.
	release := func(from int64, restart bool) int64 {
		GinkgoHelper()
		var ref *appv1alpha1.ReleaseConfigReference
		if from > 0 {
			rec, err := r.readRuntimeConfigRecord(ctx, getApp(), from)
			Expect(err).NotTo(HaveOccurred())
			ref = &appv1alpha1.ReleaseConfigReference{SourceGeneration: from, Image: strings.TrimPrefix(getApp().Status.Image, "/"),
				PreserveGroupValues: restart, Runtime: rec.spec.Runtime}
		}
		update(func(app *appv1alpha1.App) {
			gen := app.Generation + 1
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			app.Spec.ReleaseConfig = nil
			if ref != nil {
				ref.Generation = gen
				app.Spec.ReleaseConfig = ref
			}
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(gen))
		})
		return getApp().Generation
	}
	// serve rolls gen out to live, completing its build when it builds.
	serve := func(gen int64, builds bool) {
		GinkgoHelper()
		pass(3)
		if builds {
			completeBuildJob(name, fmt.Sprintf("gen-%d", gen))
			pass(2)
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
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		background := metav1.DeletePropagationBackground
		for gen := range 16 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, fmt.Sprintf("gen-%d", gen)), Namespace: "default"}},
				&client.DeleteOptions{PropagationPolicy: &background})
		}
	})

	It("reads a saved build command or branch undeployed through a Restart and a rollback until a deploy builds it", func() {
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/hello", Branch: "main", Runtime: "node", Builder: build.BuilderNative,
				BuildCommand: "npm ci", StartCommand: "node server.js", Port: 3000, Replicas: 1, Tier: "starter"},
		})).To(Succeed())
		serve(1, true)
		expectUndeployed(false, "", "the first deploy is what is saved")

		By("an identical Restart")
		serve(release(1, true), false)
		expectUndeployed(false, "", "a Restart of what is saved changes nothing")

		By("a plan change deploys")
		update(func(app *appv1alpha1.App) { app.Spec.Tier = "standard" })
		planned := release(0, false)
		serve(planned, true)
		expectUndeployed(false, "", "the plan change deployed")

		By("a saved build command waits through a Restart")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "npm run build" })
		expectUndeployed(true, appv1alpha1.UndeployedReasonSaved, "the saved build command is not deployed")
		serve(release(planned, true), false)
		expectUndeployed(true, appv1alpha1.UndeployedReasonHistoricalRelease,
			"the Restart runs the serving release's build, not the saved build command")

		By("the build command the Restart runs, put back")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "npm ci" })
		expectUndeployed(false, "", "the Restart runs the build command now saved")

		By("a rollback past the plan change keeps the current plan")
		rolledBack := release(1, false)
		serve(rolledBack, false)
		expectUndeployed(false, "", "Render's rollback keeps the current plan, so the plan changed since the target is no change")

		By("a saved branch waits through a Restart of the rollback, and through a rollback")
		saveOnly(func(s *appv1alpha1.AppSpec) { s.Branch = "dev" })
		expectUndeployed(true, appv1alpha1.UndeployedReasonHistoricalRelease, "the saved branch is not deployed")
		serve(release(rolledBack, true), false)
		expectUndeployed(true, appv1alpha1.UndeployedReasonHistoricalRelease,
			"the Restart runs the rollback target's build, not the saved branch")
		serve(release(planned, false), false)
		expectUndeployed(true, appv1alpha1.UndeployedReasonHistoricalRelease, "the rollback runs its target's branch")

		By("the next deploy builds the saved branch")
		serve(release(0, false), true)
		expectUndeployed(false, "", "the deploy built the saved settings")
	})
})
