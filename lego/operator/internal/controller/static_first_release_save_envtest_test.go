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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m460: a save of a static site's publish directory or root directory
// during its first build left the deploy update_in_progress until bex-api's
// health gate failed it ~21 minutes later. The first publish recorded rev-1
// and handed the host over on the next pass, but that pass found the save
// pending (pending-source-generation past the release stamp) with no served
// image (a static site records status.image only once its host is handed
// over), and resolveDeployImage's "nothing to reuse" halt returned before
// reconcileStaticSite could finish. The site served rev-1 through the
// activator, with no IP allow-list, and phase stayed Deploying.
var _ = Describe("A static site's first deploy with a save during its build (w8/m460)", func() {
	const ns = "default"
	const cidr = "203.0.113.0/24"

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: ns,
			BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore: publish.Store{
				Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3",
			},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
			ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
		}
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("a"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })
	})

	pass := func(nn types.NamespacedName) ctrl.Result {
		GinkgoHelper()
		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return res
	}
	passes := func(nn types.NamespacedName, n int) ctrl.Result {
		GinkgoHelper()
		var res ctrl.Result
		for range n {
			res = pass(nn)
		}
		return res
	}
	update := func(nn types.NamespacedName, mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := envtestApp(nn)
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	// save is a Render Update Service PATCH (bex services update): the change is
	// saved, and the next deploy ships it (rollout.Snapshot.Stamp, save-only).
	save := func(nn types.NamespacedName, mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(nn, func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, strconv.FormatInt(app.Generation+1, 10))
		})
	}
	// saveDirectory is a root or publish directory save. bex-api stamps a
	// rebuild on each (SetRootDir, SetPublishPath), so a set-and-clear leaves a
	// spec that differs from its release only in spec.restartedAt (w8/339).
	stamps := 0
	saveDirectory := func(nn types.NamespacedName, mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		stamps++
		save(nn, func(s *appv1alpha1.AppSpec) {
			mutate(s)
			s.RestartedAt = fmt.Sprintf("2026-11-19T09:00:%02dZ", stamps)
		})
	}
	// undeployed is the saved-configuration status the dashboard's "saved
	// changes aren't live yet" banner reads.
	undeployed := func(nn types.NamespacedName) (bool, string) {
		GinkgoHelper()
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		app := envtestApp(nn)
		return app.Status.UndeployedChanges, app.Status.UndeployedChangesReason
	}
	// operational is a save that rolls no release (an IP rule, a rename).
	operational := func(nn types.NamespacedName, mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(nn, func(app *appv1alpha1.App) { mutate(&app.Spec) })
	}
	// deploy is bex deploys create: a new release stamped at the generation it
	// writes.
	deploy := func(nn types.NamespacedName, at string) int64 {
		GinkgoHelper()
		update(nn, func(app *appv1alpha1.App) {
			app.Spec.RestartedAt = at
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation+1, 10))
		})
		return envtestApp(nn).Generation
	}
	create := func(name string, mutate ...func(*appv1alpha1.AppSpec)) types.NamespacedName {
		GinkgoHelper()
		spec := appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/octocat/Spoon-Knife", Branch: "main",
			BuildCommand: "mkdir -p out out2 && echo m-out > out/marker.txt && echo m-out2 > out2/marker.txt",
			PublishPath:  "out", Expose: true,
		}
		for _, m := range mutate {
			m(&spec)
		}
		// The creation's own deploy stamps release generation 1, as bex-api's
		// create does.
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: spec,
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: name, Namespace: ns}
		DeferCleanup(func() {
			if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				saved := r.Registry
				r.Registry = ""
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
				r.Registry = saved
			}
			background := metav1.DeletePropagationBackground
			for i := 1; i <= 8; i++ {
				gen := strconv.Itoa(i)
				for _, job := range []string{build.JobName(name, "gen-"+gen), publish.JobName(name, "rev-"+gen)} {
					_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job, Namespace: ns}},
						&client.DeleteOptions{PropagationPolicy: &background})
				}
			}
		})
		return nn
	}
	publishJob := func(name, rev string) *batchv1.Job {
		GinkgoHelper()
		j := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: publish.JobName(name, rev), Namespace: ns}, j)).To(Succeed())
		return j
	}
	// publishedPath is the directory a revision's publish Job extracts.
	publishedPath := func(name, rev string) string {
		GinkgoHelper()
		for _, c := range publishJob(name, rev).Spec.Template.Spec.InitContainers {
			if v := envValue(c.Env, "PUBLISH_PATH"); v != "" {
				return v
			}
		}
		return ""
	}
	ingress := func(nn types.NamespacedName) *networkingv1.Ingress {
		GinkgoHelper()
		ing := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed())
		return ing
	}
	backend := func(nn types.NamespacedName) string {
		GinkgoHelper()
		return ingress(nn).Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name
	}
	ipRuleRouted := func(nn types.NamespacedName) bool {
		GinkgoHelper()
		return strings.Contains(ingress(nn).Annotations[traefikRouterMiddlewaresAnnotation], nn.Name+"-ip-allow")
	}
	// build starts the first release and returns once its build Job runs.
	build1 := func(nn types.NamespacedName) {
		GinkgoHelper()
		passes(nn, 3)
		_, err := buildJobFor(nn.Name, "gen-1")
		Expect(err).NotTo(HaveOccurred(), "the first release must dispatch its build")
		Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))
	}
	// finishFirst completes the first build and publish, then the handover.
	finishFirst := func(nn types.NamespacedName) {
		GinkgoHelper()
		completeBuildJob(nn.Name, "gen-1")
		passes(nn, 2)
		completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
		res := pass(nn)
		Expect(res.RequeueAfter).To(Equal(staticHandoverDelay), "the first publish records rev-1 and hands the host over next pass")
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"))
	}
	// live is what bex-api's reconciler closes the first deploy row live on:
	// phase Running on release generation 1, whose revision serves.
	expectLive := func(nn types.NamespacedName) {
		GinkgoHelper()
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning), "status: %+v", app.Status)
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(backend(nn)).To(Equal(staticServerAliasName(nn.Name)), "the host is handed to the static-server")
		Expect(app.Status.ReleaseRollout).To(BeNil(), "a static site publishes; it rolls out no instance (w32/m560)")
	}

	It("goes live after a root directory set and cleared during the build", func() {
		nn := create("m460-rootdir")
		build1(nn)

		By("saving rootDir sub, then clearing it, while gen-1 builds")
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "sub" })
		pass(nn)
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "" })
		pass(nn)

		finishFirst(nn)
		res := passes(nn, 3)
		expectLive(nn)
		Expect(res.RequeueAfter).NotTo(Equal(pendingReleaseRequeue), "a settings save owes no release: no 1s requeue loop")
		Expect(envtestApp(nn).Status.PendingReleaseGeneration).To(BeZero())
		_, err := buildJobFor(nn.Name, "gen-2")
		Expect(err).To(HaveOccurred(), "a save must not build until a deploy requests it")
		different, reason := undeployed(nn)
		Expect(different).To(BeFalse(), "the saves net out: nothing waits for a deploy (w8/339), reason %q", reason)
	})

	// w8/339: production's set-and-clear during a later deploy's build read
	// "saved" too, until the next deploy.
	It("reads no undeployed change after a root directory set and cleared during a later deploy's build", func() {
		nn := create("m460-rootdir-later")
		build1(nn)
		finishFirst(nn)
		passes(nn, 3)
		expectLive(nn)

		gen := deploy(nn, "2026-11-19T08:00:00Z")
		passes(nn, 3)
		_, err := buildJobFor(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
		Expect(err).NotTo(HaveOccurred(), "precondition: the deploy builds")

		By("saving rootDir sub, then clearing it, while it builds")
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "sub" })
		pass(nn)
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "" })
		pass(nn)

		completeBuildJob(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
		passes(nn, 2)
		rev := "rev-" + strconv.FormatInt(gen, 10)
		completeEnvtestJob(publishJob(nn.Name, rev), metav1.Now())
		passes(nn, 3)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning), "status: %+v", app.Status)
		Expect(app.Status.ActiveRevision).To(Equal(rev))
		different, reason := undeployed(nn)
		Expect(different).To(BeFalse(), "the saves net out: nothing waits for a deploy, reason %q", reason)
	})

	// The same stamp rides a publish directory save, so one saved and then
	// reverted nets out too; a directory that stays saved still waits.
	It("reads no undeployed change after a publish directory saved and reverted during the build", func() {
		nn := create("m460-publish-revert")
		build1(nn)
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out2" })
		pass(nn)
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out" })
		pass(nn)
		finishFirst(nn)
		passes(nn, 3)
		expectLive(nn)
		different, reason := undeployed(nn)
		Expect(different).To(BeFalse(), "out, out2, out nets out, reason %q", reason)

		By("a publish directory that stays saved waits for a deploy")
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out2" })
		pass(nn)
		different, reason = undeployed(nn)
		Expect(different).To(BeTrue())
		Expect(reason).To(Equal(appv1alpha1.UndeployedReasonSaved))
	})

	// w32/053: production's bad branch (and failing build command) read false
	// once its deploy failed, though rev-1 still served the old settings, and
	// putting the live setting back read saved. A web service read both right.
	Context("after a deploy that never published (w32/053)", func() {
		// liveSite is the repro's step 1: rev-1 serves, nothing waits.
		liveSite := func(name string) (types.NamespacedName, appv1alpha1.AppSpec) {
			GinkgoHelper()
			nn := create(name)
			build1(nn)
			finishFirst(nn)
			passes(nn, 3)
			expectLive(nn)
			different, _ := undeployed(nn)
			Expect(different).To(BeFalse(), "A: nothing waits")
			return nn, envtestApp(nn).Spec
		}
		expectSaved := func(nn types.NamespacedName, why string) {
			GinkgoHelper()
			different, reason := undeployed(nn)
			Expect(different).To(BeTrue(), why)
			Expect(reason).To(Equal(appv1alpha1.UndeployedReasonSaved))
		}
		// putBack is step 4: every saved setting the live release's again.
		putBack := func(nn types.NamespacedName, live appv1alpha1.AppSpec) {
			GinkgoHelper()
			save(nn, func(s *appv1alpha1.AppSpec) { s.Branch, s.BuildCommand = live.Branch, live.BuildCommand })
			pass(nn)
			different, reason := undeployed(nn)
			Expect(different).To(BeFalse(), "D: the saved settings are the live release's, reason %q", reason)
			Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"))
		}

		for _, tc := range []struct {
			name string
			bad  func(*appv1alpha1.AppSpec)
		}{
			{"branch", func(s *appv1alpha1.AppSpec) { s.Branch = "qa-no-such-branch-4aa5" }},
			{"build command", func(s *appv1alpha1.AppSpec) { s.BuildCommand = "echo QA4AA5 fail; exit 3" }},
		} {
			It("reads a saved "+tc.name+" whose build failed as undeployed, and the live one put back as deployed", func() {
				nn, live := liveSite("w32-053-" + strings.ReplaceAll(tc.name, " ", "-"))
				save(nn, tc.bad)
				pass(nn)
				expectSaved(nn, "B: the save waits for a deploy")

				gen := strconv.FormatInt(deploy(nn, "2026-11-20T02:41:00Z"), 10)
				passes(nn, 3)
				failBuildJob(nn.Name, "gen-"+gen)
				// The pass that observes the failure returns it.
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
				app := envtestApp(nn)
				Expect(app.Status.ActiveRevision).To(Equal("rev-1"), "status: %+v", app.Status)
				Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning), "rev-1 keeps serving")
				expectSaved(nn, "C: the saved "+tc.name+" never served")

				putBack(nn, live)
			})
		}

		It("reads a saved branch whose deploy was canceled as undeployed, and the live one put back as deployed", func() {
			nn, live := liveSite("w32-053-cancel")
			save(nn, func(s *appv1alpha1.AppSpec) { s.Branch = "next" })
			pass(nn)
			gen := deploy(nn, "2026-11-20T02:41:00Z")
			passes(nn, 3)
			_, err := buildJobFor(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
			Expect(err).NotTo(HaveOccurred(), "precondition: the deploy builds")
			update(nn, func(app *appv1alpha1.App) {
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, strconv.FormatInt(gen, 10))
			})
			passes(nn, 3)
			Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"))
			expectSaved(nn, "the canceled deploy published nothing")

			putBack(nn, live)
		})
	})

	// w32/055: on production a rollback re-served an earlier publication, and
	// afterwards undeployedChanges read false, though the saved build command
	// that the next deploy publishes was not the target's. A rollback restores
	// the target's files (w4/m197), so it serves the target's settings.
	Context("after a rollback (w32/055)", func() {
		expectUndeployed := func(nn types.NamespacedName, want bool, why string) {
			GinkgoHelper()
			different, reason := undeployed(nn)
			Expect(different).To(Equal(want), "%s (reason %q)", why, reason)
			if want {
				Expect(reason).To(Equal(appv1alpha1.UndeployedReasonSaved))
			}
		}
		record := func(nn types.NamespacedName, gen int64) *corev1.Secret {
			GinkgoHelper()
			rec := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.StaticPublicationRecordName(nn.Name, gen), Namespace: ns}, rec)).To(Succeed())
			return rec
		}
		// liveSite serves rev-1 and returns the image its build produced.
		liveSite := func(name string) (types.NamespacedName, string) {
			GinkgoHelper()
			nn := create(name)
			build1(nn)
			finishFirst(nn)
			passes(nn, 3)
			expectLive(nn)
			return nn, envtestApp(nn).Status.Image
		}
		// redeploy is bex deploys create, built and published: bex-api's deploy
		// drops a rollback's selection.
		redeploy := func(nn types.NamespacedName, at string) {
			GinkgoHelper()
			var gen int64
			update(nn, func(app *appv1alpha1.App) {
				gen = app.Generation + 1
				app.Spec.ReleaseConfig, app.Spec.RestartedAt = nil, at
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(gen, 10))
			})
			passes(nn, 3)
			completeBuildJob(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
			passes(nn, 2)
			rev := "rev-" + strconv.FormatInt(gen, 10)
			completeEnvtestJob(publishJob(nn.Name, rev), metav1.Now())
			passes(nn, 3)
			Expect(envtestApp(nn).Status.ActiveRevision).To(Equal(rev), "status: %+v", envtestApp(nn).Status)
		}
		// rollback is bex-api's: a release that selects the target's retained
		// publication from its record and leaves the saved settings as saved.
		// The copy runs in a publish Job, with no build.
		rollback := func(nn types.NamespacedName, target int64, image, at string) int64 {
			GinkgoHelper()
			prefix := string(record(nn, target).Data[appv1alpha1.StaticPublicationPrefixKey])
			var gen int64
			update(nn, func(app *appv1alpha1.App) {
				gen = app.Generation + 1
				app.Spec.RestartedAt = at
				app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: gen, Image: image, StaticPrefix: prefix}
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(gen, 10))
			})
			passes(nn, 3)
			_, err := buildJobFor(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
			Expect(err).To(HaveOccurred(), "a rollback builds nothing")
			rev := "rev-" + strconv.FormatInt(gen, 10)
			completeEnvtestJob(publishJob(nn.Name, rev), metav1.Now())
			passes(nn, 3)
			app := envtestApp(nn)
			Expect(app.Status.ActiveRevision).To(Equal(rev), "status: %+v", app.Status)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			return gen
		}
		buildCommand := func(marker string) func(*appv1alpha1.AppSpec) {
			return func(s *appv1alpha1.AppSpec) { s.BuildCommand = "mkdir -p out && echo " + marker + " > out/marker.txt" }
		}

		It("reads a save the rollback did not publish as undeployed until a deploy publishes it", func() {
			nn, image := liveSite("w32-055-pending")
			live := envtestApp(nn).Spec
			redeploy(nn, "2026-11-20T05:39:29Z")
			expectUndeployed(nn, false, "rev-2 serves the saved settings")

			save(nn, buildCommand("PUB2"))
			pass(nn)
			expectUndeployed(nn, true, "the save waits for a deploy")

			gen := rollback(nn, 1, image, "2026-11-20T05:43:18Z")
			expectUndeployed(nn, true, "the rollback restored rev-1's files: the saved build command never published")
			Expect(record(nn, gen).Data[staticPublicationSettingsKey]).To(Equal(record(nn, 1).Data[staticPublicationSettingsKey]),
				"the restored publication is recorded with the target's settings")
			Expect(record(nn, 1).Data).To(HaveKey(staticPublicationSettingsKey))

			By("putting the target's build command back")
			save(nn, func(s *appv1alpha1.AppSpec) { s.BuildCommand = live.BuildCommand })
			pass(nn)
			expectUndeployed(nn, false, "the saved settings are the restored publication's")

			By("saving the build command again, then deploying it")
			save(nn, buildCommand("PUB2"))
			pass(nn)
			expectUndeployed(nn, true, "the save waits again")
			redeploy(nn, "2026-11-20T05:50:00Z")
			expectUndeployed(nn, false, "the deploy published the save")
		})

		It("reads saved settings that differ from the target's as undeployed, with no save pending", func() {
			nn, image := liveSite("w32-055-other")
			save(nn, buildCommand("PUB2"))
			pass(nn)
			redeploy(nn, "2026-11-20T05:39:29Z")
			expectUndeployed(nn, false, "rev-2 published PUB2")

			rollback(nn, 1, image, "2026-11-20T05:43:18Z")
			expectUndeployed(nn, true, "rev-1 was built with another build command, which the next deploy replaces")
			redeploy(nn, "2026-11-20T05:50:00Z")
			expectUndeployed(nn, false, "the deploy published the saved settings")
		})

		It("reads nothing to deploy after a rollback to a release built with the saved settings", func() {
			nn, image := liveSite("w32-055-same")
			redeploy(nn, "2026-11-20T05:39:29Z")
			rollback(nn, 1, image, "2026-11-20T05:43:18Z")
			expectUndeployed(nn, false, "rev-1 was built with the saved settings")
		})

		It("compares with the settings serving at the request when the target's record predates its settings", func() {
			nn, image := liveSite("w32-055-legacy")
			save(nn, buildCommand("PUB2"))
			pass(nn)
			redeploy(nn, "2026-11-20T05:39:29Z")
			served := envtestApp(nn).Spec.BuildCommand
			rec := record(nn, 1)
			delete(rec.Data, staticPublicationSettingsKey)
			Expect(k8sClient.Update(ctx, rec)).To(Succeed())

			save(nn, buildCommand("PUB3"))
			pass(nn)
			rollback(nn, 1, image, "2026-11-20T05:43:18Z")
			expectUndeployed(nn, true, "a save that was waiting keeps waiting")
			save(nn, func(s *appv1alpha1.AppSpec) { s.BuildCommand = served })
			pass(nn)
			expectUndeployed(nn, false, "the saved settings are the ones serving at the request")
		})
	})

	It("publishes the directory it started with, and the next deploy publishes the saved one", func() {
		nn := create("m460-publish")
		build1(nn)

		By("saving publish directory out2 while gen-1 builds")
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out2" })
		pass(nn)

		finishFirst(nn)
		Expect(publishedPath(nn.Name, "rev-1")).To(Equal("out"), "the first deploy publishes what it started with")
		res := passes(nn, 3)
		expectLive(nn)
		Expect(envtestApp(nn).Status.PendingReleaseGeneration).To(BeZero(), "the save coalesced while it built is owed nothing")
		Expect(res.RequeueAfter).NotTo(Equal(pendingReleaseRequeue))

		By("the saved directory waits for a deploy, and reads as an undeployed change")
		different, reason := undeployed(nn)
		Expect(different).To(BeTrue())
		Expect(reason).To(Equal(appv1alpha1.UndeployedReasonSaved))

		By("bex deploys create publishes out2")
		gen := deploy(nn, "2026-11-17T08:00:00Z")
		rev := "rev-" + strconv.FormatInt(gen, 10)
		passes(nn, 3)
		completeBuildJob(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
		passes(nn, 2)
		Expect(publishedPath(nn.Name, rev)).To(Equal("out2"))
		completeEnvtestJob(publishJob(nn.Name, rev), metav1.Now())
		passes(nn, 3)
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal(rev))
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		different, _ = undeployed(nn)
		Expect(different).To(BeFalse(), "the deploy shipped the saved directory")
	})

	It("enforces an IP rule saved during the build, before and after the deploy settles", func() {
		nn := create("m460-iprule")
		build1(nn)
		Expect(backend(nn)).To(Equal(activatorAliasName(nn.Name)), "precondition: the activator answers the first build")
		Expect(ipRuleRouted(nn)).To(BeFalse())

		By("saving an IP rule together with a publish directory while gen-1 builds")
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) {
			s.PublishPath = "out2"
			s.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: cidr, Description: "doc"}}
		})
		pass(nn)
		Expect(ipRuleRouted(nn)).To(BeTrue(), "the first build's route enforces the rule within a pass")

		By("publishing: the rule stays on the route")
		completeBuildJob(nn.Name, "gen-1")
		passes(nn, 2)
		Expect(ipRuleRouted(nn)).To(BeTrue())

		completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
		pass(nn)
		passes(nn, 3)
		expectLive(nn)
		Expect(ipRuleRouted(nn)).To(BeTrue(), "the served site enforces the rule")
	})

	// The cause is static-specific: a web service records its image before its
	// first release serves, so the same saves never stranded it. Its settled
	// release must not be requeued every second for the save either.
	It("leaves a web service's first deploy settling as before, owing nothing", func() {
		const name = "m460-web"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile", Port: 3000, Replicas: 1,
			},
		})).To(Succeed())
		DeferCleanup(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
			}
			background := metav1.DeletePropagationBackground
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: ns}},
				&client.DeleteOptions{PropagationPolicy: &background})
		})
		build1(nn)
		saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "sub" })
		pass(nn)
		completeBuildJob(name, "gen-1")
		passes(nn, 2)
		markEnvtestDeploymentReady(nn)
		res := passes(nn, 2)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning), "status: %+v", app.Status)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.PendingReleaseGeneration).To(BeZero())
		Expect(res.RequeueAfter).NotTo(Equal(pendingReleaseRequeue))
	})

	// t004: every release input a static site has, saved the way bex-api's
	// save-only surfaces write it (CLI and REST PATCH, the dashboard's Settings
	// save, REST env-var writes), and the operational fields, which stamp
	// nothing.
	for _, tc := range []struct {
		name   string
		rolls  bool
		mutate func(*appv1alpha1.AppSpec)
	}{
		{name: "environment variable", rolls: true, mutate: func(s *appv1alpha1.AppSpec) {
			s.Env = []appv1alpha1.EnvVar{{Name: "SITE_MODE", Value: "next"}}
		}},
		{name: "environment group link", rolls: true, mutate: func(s *appv1alpha1.AppSpec) { s.EnvFromSecrets = []string{"m460-group"} }},
		{name: "secret file", rolls: true, mutate: func(s *appv1alpha1.AppSpec) { s.FilesFromSecrets = []string{"m460-files"} }},
		{name: "branch", rolls: true, mutate: func(s *appv1alpha1.AppSpec) { s.Branch = "next" }},
		{name: "build command", rolls: true, mutate: func(s *appv1alpha1.AppSpec) { s.BuildCommand = "mkdir -p out && echo two > out/marker.txt" }},
		{name: "build filter", mutate: func(s *appv1alpha1.AppSpec) {
			s.BuildFilter = &appv1alpha1.BuildFilterSpec{Paths: []string{"site/**"}}
		}},
		{name: "auto-deploy", mutate: func(s *appv1alpha1.AppSpec) { s.AutoDeploy = !s.AutoDeploy }},
		{name: "headers and routes", mutate: func(s *appv1alpha1.AppSpec) {
			s.Headers = []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-Frame-Options", Value: "DENY"}}
			s.Routes = []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}}
		}},
	} {
		It("goes live with a mid-build save of its "+tc.name, func() {
			nn := create("m460-" + strings.ReplaceAll(strings.ReplaceAll(tc.name, " ", "-"), "-and-", "-"))
			build1(nn)
			Expect(appv1alpha1.SpecRollsRelease(envtestApp(nn).Spec, func() appv1alpha1.AppSpec {
				s := envtestApp(nn).Spec
				tc.mutate(&s)
				return s
			}())).To(Equal(tc.rolls), "the input's identity class")
			if tc.rolls {
				save(nn, tc.mutate)
			} else {
				operational(nn, tc.mutate)
			}
			pass(nn)
			finishFirst(nn)
			passes(nn, 3)
			expectLive(nn)
			_, err := buildJobFor(nn.Name, "gen-2")
			Expect(err).To(HaveOccurred(), "nothing builds until a deploy requests it")
		})
	}

	// A Blueprint sync, and an env-var write through GraphQL or MCP, deploy:
	// they stamp a release, which waits in the pending slot behind the first
	// one (ADR060 §D1a) and rolls once it settles.
	It("goes live, then rolls a deploy that arrived during the first build", func() {
		nn := create("m460-synced")
		build1(nn)
		update(nn, func(app *appv1alpha1.App) {
			app.Spec.PublishPath = "out2"
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation+1, 10))
		})
		gen := envtestApp(nn).Generation
		pass(nn)
		finishFirst(nn)
		Expect(publishedPath(nn.Name, "rev-1")).To(Equal("out"))
		passes(nn, 3)
		// A publish directory is release identity, not artifact identity: the
		// owed deploy republishes gen-1's image from out2, with no rebuild.
		_, err := buildJobFor(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
		Expect(err).To(HaveOccurred(), "a publish directory alone rebuilds nothing")
		rev := "rev-" + strconv.FormatInt(gen, 10)
		Expect(publishedPath(nn.Name, rev)).To(Equal("out2"), "the owed deploy publishes once the first release settles")
		completeEnvtestJob(publishJob(nn.Name, rev), metav1.Now())
		passes(nn, 3)
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal(rev))
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
	})

	// A direct publish (no build command) clones its repo in the publish Job,
	// and records no image at all, so a saved change halted every pass of a
	// served site too: its routing stopped following the App.
	Context("a direct publish", func() {
		direct := func(s *appv1alpha1.AppSpec) { s.BuildCommand = ""; s.PublishPath = "." }
		// clone runs the publish Job's pod to its upload step, or to the end.
		clone := func(nn types.NamespacedName, rev string, done bool) {
			GinkgoHelper()
			j := publishJob(nn.Name, rev)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: j.Name + "-abcde", Namespace: ns, Labels: map[string]string{"job-name": j.Name}},
				Spec: corev1.PodSpec{
					NodeName:       "node-a",
					InitContainers: []corev1.Container{{Name: "clone", Image: "x"}},
					Containers:     []corev1.Container{{Name: "upload", Image: "x"}},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod) })
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
				Name: "clone", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}},
			}}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
			if done {
				completeEnvtestJob(j, metav1.Now())
			}
		}

		It("keeps a served site's IP rule converging while a saved change waits", func() {
			nn := create("m460-direct-served", direct)
			passes(nn, 3)
			clone(nn, "rev-1", true)
			passes(nn, 4)
			Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseRunning))

			saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "public" })
			pass(nn)
			operational(nn, func(s *appv1alpha1.AppSpec) {
				s.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: cidr, Description: "doc"}}
			})
			pass(nn)
			Expect(ipRuleRouted(nn)).To(BeTrue(), "the rule takes effect while the saved directory waits")
			app := envtestApp(nn)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: publish.JobName(nn.Name, "rev-2"), Namespace: ns}, &batchv1.Job{}))).
				To(BeTrue(), "the saved directory publishes nothing until a deploy")
		})

		It("goes live after a save while its first publish uploads", func() {
			nn := create("m460-direct-first", direct)
			passes(nn, 3)
			clone(nn, "rev-1", false)
			pass(nn)
			Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseDeploying), "precondition: the clone ended, the upload runs")

			saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "public" })
			pass(nn)
			completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
			passes(nn, 4)
			expectLive(nn)
		})
	})

	// w32/101: a save-only update arriving while a static site's Restart or
	// Rollback copy Job is in flight (PhaseDeploying) used to wedge the site
	// in PhaseDeploying forever with zero requeue — blocking every subsequent
	// deploy in queued — for both no-build and built static sites and under
	// both Wait and Override overlap policies.
	Context("a save-only update during an in-flight static Restart or Rollback (w32/101)", func() {
		direct := func(s *appv1alpha1.AppSpec) { s.BuildCommand = ""; s.PublishPath = "." }
		startRestore := func(nn types.NamespacedName, targetPrefix, image, at string) int64 {
			GinkgoHelper()
			var gen int64
			update(nn, func(app *appv1alpha1.App) {
				gen = app.Generation + 1
				app.Spec.RestartedAt = at
				app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{
					Generation: gen, Image: image, StaticPrefix: targetPrefix,
				}
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(gen, 10))
			})
			res := passes(nn, 2)
			Expect(res.RequeueAfter).To(Equal(publish.ObserveInterval), "the restore copy Job is in flight and requeues")
			app := envtestApp(nn)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))
			Expect(app.Status.ReleaseGeneration).To(Equal(gen))
			Expect(app.Status.SelectedReleaseGeneration).To(Equal(gen))
			Expect(app.Status.ArtifactFingerprint).To(Equal(app.Status.ReleaseArtifactFingerprint),
				"staticRestoreSelection must stamp ArtifactFingerprint on adoption")
			Expect(app.Status.ArtifactImage).To(Equal(image))
			return gen
		}

		for _, policy := range []string{"", appv1alpha1.OverlappingDeployPolicyOverride} {
			policyName := "wait"
			if policy != "" {
				policyName = policy
			}

			It("finishes a no-build static Restart across a mid-copy save under "+policyName+" policy", func() {
				nn := create("w32-101-direct-"+policyName, direct)
				if policy != "" {
					update(nn, func(app *appv1alpha1.App) {
						if app.Labels == nil {
							app.Labels = map[string]string{}
						}
						app.Labels[appv1alpha1.LabelOverlappingDeployPolicy] = policy
					})
				}
				passes(nn, 3)
				completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
				passes(nn, 4)
				expectLive(nn)
				prefix := envtestApp(nn).Status.StaticPrefix
				Expect(prefix).NotTo(BeEmpty())

				By("starting a static Restart copy Job over rev-1")
				restartGen := startRestore(nn, prefix, "", "2026-11-23T19:46:35Z")
				restartRev := "rev-" + strconv.FormatInt(restartGen, 10)

				By("landing a save-only update mid-copy (changed publish directory)")
				saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "public" })
				res := pass(nn)
				Expect(res.RequeueAfter).To(Equal(publish.ObserveInterval),
					"a mid-copy save must not strand the in-flight copy with zero requeue")
				Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))

				By("completing the copy Job: the restart settles Running on rev-2 and leaves the save waiting")
				completeEnvtestJob(publishJob(nn.Name, restartRev), metav1.Now())
				passes(nn, 3)
				app := envtestApp(nn)
				Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning), "status: %+v", app.Status)
				Expect(app.Status.ActiveRevision).To(Equal(restartRev))
				Expect(app.Status.PendingReleaseGeneration).To(BeZero())
				different, reason := undeployed(nn)
				Expect(different).To(BeTrue(), "the saved publish directory was not published by the restart")
				Expect(reason).To(Equal(appv1alpha1.UndeployedReasonSaved))

				By("a follow-up deploy publishes the saved directory normally")
				var nextGen int64
				update(nn, func(app *appv1alpha1.App) {
					nextGen = app.Generation + 1
					app.Spec.ReleaseConfig = nil
					app.Spec.RestartedAt = "2026-11-23T19:50:00Z"
					metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(nextGen, 10))
				})
				passes(nn, 2)
				nextRev := "rev-" + strconv.FormatInt(nextGen, 10)
				completeEnvtestJob(publishJob(nn.Name, nextRev), metav1.Now())
				passes(nn, 3)
				app = envtestApp(nn)
				Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
				Expect(app.Status.ActiveRevision).To(Equal(nextRev))
				different, _ = undeployed(nn)
				Expect(different).To(BeFalse())
			})

			It("finishes a built static Rollback/Restart across a mid-copy save under "+policyName+" policy", func() {
				nn := create("w32-101-built-" + policyName)
				if policy != "" {
					update(nn, func(app *appv1alpha1.App) {
						if app.Labels == nil {
							app.Labels = map[string]string{}
						}
						app.Labels[appv1alpha1.LabelOverlappingDeployPolicy] = policy
					})
				}
				build1(nn)
				finishFirst(nn)
				passes(nn, 3)
				expectLive(nn)
				first := envtestApp(nn)
				prefix, image := first.Status.StaticPrefix, first.Status.Image
				Expect(image).NotTo(BeEmpty())

				By("starting a static restore copy Job over rev-1")
				restoreGen := startRestore(nn, prefix, image, "2026-11-23T19:46:35Z")
				restoreRev := "rev-" + strconv.FormatInt(restoreGen, 10)

				By("landing a save-only update mid-copy")
				saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out2" })
				res := pass(nn)
				Expect(res.RequeueAfter).To(Equal(publish.ObserveInterval),
					"a built static restore must keep observing its copy Job across a mid-copy save")

				By("completing the copy Job: the restore settles Running with its image and leaves out2 waiting")
				completeEnvtestJob(publishJob(nn.Name, restoreRev), metav1.Now())
				passes(nn, 3)
				app := envtestApp(nn)
				Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning), "status: %+v", app.Status)
				Expect(app.Status.ActiveRevision).To(Equal(restoreRev))
				Expect(app.Status.Image).To(Equal(image))
				different, reason := undeployed(nn)
				Expect(different).To(BeTrue())
				Expect(reason).To(Equal(appv1alpha1.UndeployedReasonSaved))
			})
		}

		It("holds a newer deploy behind an in-flight static restore under Wait, then deploys it", func() {
			nn := create("w32-101-overlap-wait", direct)
			passes(nn, 3)
			completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
			passes(nn, 4)
			expectLive(nn)
			prefix := envtestApp(nn).Status.StaticPrefix

			restartGen := startRestore(nn, prefix, "", "2026-11-23T19:46:35Z")
			restartRev := "rev-" + strconv.FormatInt(restartGen, 10)

			By("triggering a newer deploy while the restore copy is in flight")
			var deployGen int64
			update(nn, func(app *appv1alpha1.App) {
				deployGen = app.Generation + 1
				app.Spec.ReleaseConfig = nil
				app.Spec.RestartedAt = "2026-11-23T19:46:38Z"
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(deployGen, 10))
			})
			res := pass(nn)
			Expect(res.RequeueAfter).To(Equal(publish.ObserveInterval))
			app := envtestApp(nn)
			Expect(app.Status.ReleaseGeneration).To(Equal(restartGen), "Wait must pin the in-flight restore")
			Expect(app.Status.PendingReleaseGeneration).To(Equal(deployGen))
			deployRev := "rev-" + strconv.FormatInt(deployGen, 10)
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: publish.JobName(nn.Name, deployRev), Namespace: ns}, &batchv1.Job{}))).
				To(BeTrue(), "the newer deploy waits until the restore finishes")

			By("completing the restore copy: it goes live and immediately queues the waiting deploy")
			completeEnvtestJob(publishJob(nn.Name, restartRev), metav1.Now())
			res = pass(nn)
			Expect(res.RequeueAfter).To(Equal(pendingReleaseRequeue))
			Expect(envtestApp(nn).Status.ActiveRevision).To(Equal(restartRev))

			passes(nn, 2)
			completeEnvtestJob(publishJob(nn.Name, deployRev), metav1.Now())
			passes(nn, 3)
			app = envtestApp(nn)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			Expect(app.Status.ActiveRevision).To(Equal(deployRev))
		})

		It("lets a newer deploy supersede an in-flight static restore under Override", func() {
			nn := create("w32-101-overlap-override", direct)
			update(nn, func(app *appv1alpha1.App) {
				app.Labels = map[string]string{appv1alpha1.LabelOverlappingDeployPolicy: appv1alpha1.OverlappingDeployPolicyOverride}
			})
			passes(nn, 3)
			completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
			passes(nn, 4)
			expectLive(nn)
			prefix := envtestApp(nn).Status.StaticPrefix

			_ = startRestore(nn, prefix, "", "2026-11-23T19:46:35Z")

			var deployGen int64
			update(nn, func(app *appv1alpha1.App) {
				deployGen = app.Generation + 1
				app.Spec.ReleaseConfig = nil
				app.Spec.RestartedAt = "2026-11-23T19:46:38Z"
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(deployGen, 10))
			})
			passes(nn, 2)
			deployRev := "rev-" + strconv.FormatInt(deployGen, 10)
			Expect(envtestApp(nn).Status.ReleaseGeneration).To(Equal(deployGen), "Override adopts the newer deploy immediately")
			completeEnvtestJob(publishJob(nn.Name, deployRev), metav1.Now())
			passes(nn, 3)
			app := envtestApp(nn)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			Expect(app.Status.ActiveRevision).To(Equal(deployRev))
		})

		It("settles back to the served revision when an in-flight static restore is canceled", func() {
			nn := create("w32-101-cancel", direct)
			passes(nn, 3)
			completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
			passes(nn, 4)
			expectLive(nn)
			prefix := envtestApp(nn).Status.StaticPrefix

			restartGen := startRestore(nn, prefix, "", "2026-11-23T19:46:35Z")
			update(nn, func(app *appv1alpha1.App) {
				metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, strconv.FormatInt(restartGen, 10))
			})
			passes(nn, 3)
			app := envtestApp(nn)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
			Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		})

		It("control: still holds a built static site whose build has no image yet on a mid-build save under Override", func() {
			nn := create("w32-101-building-control")
			update(nn, func(app *appv1alpha1.App) {
				app.Labels = map[string]string{appv1alpha1.LabelOverlappingDeployPolicy: appv1alpha1.OverlappingDeployPolicyOverride}
			})
			build1(nn)
			finishFirst(nn)
			passes(nn, 3)
			expectLive(nn)

			gen := deploy(nn, "2026-11-23T19:48:00Z")
			passes(nn, 3)
			_, err := buildJobFor(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
			Expect(err).NotTo(HaveOccurred(), "precondition: build is in flight with no image yet")
			Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))

			saveDirectory(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out2" })
			res := pass(nn)
			Expect(res.RequeueAfter).To(BeZero(), "unpinned building static site holds rather than publishing the prior image")
			Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"))
			rev := "rev-" + strconv.FormatInt(gen, 10)
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: publish.JobName(nn.Name, rev), Namespace: ns}, &batchv1.Job{}))).
				To(BeTrue(), "must not dispatch a publish Job before the build produces its image")
		})
	})

	It("still goes live with an IP rule or a rename alone saved during the build", func() {
		nn := create("m460-operational")
		build1(nn)
		operational(nn, func(s *appv1alpha1.AppSpec) {
			s.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: cidr, Description: "doc"}}
		})
		pass(nn)
		operational(nn, func(s *appv1alpha1.AppSpec) { s.DisplayName = "renamed" })
		pass(nn)
		finishFirst(nn)
		passes(nn, 3)
		expectLive(nn)
		Expect(ipRuleRouted(nn)).To(BeTrue())
	})
})
