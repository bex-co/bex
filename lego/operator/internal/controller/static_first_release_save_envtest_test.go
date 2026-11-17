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
	}

	It("goes live after a root directory set and cleared during the build", func() {
		nn := create("m460-rootdir")
		build1(nn)

		By("saving rootDir sub, then clearing it, while gen-1 builds")
		save(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "sub" })
		pass(nn)
		save(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "" })
		pass(nn)

		finishFirst(nn)
		res := passes(nn, 3)
		expectLive(nn)
		Expect(res.RequeueAfter).NotTo(Equal(pendingReleaseRequeue), "a settings save owes no release: no 1s requeue loop")
		Expect(envtestApp(nn).Status.PendingReleaseGeneration).To(BeZero())
		_, err := buildJobFor(nn.Name, "gen-2")
		Expect(err).To(HaveOccurred(), "a save must not build until a deploy requests it")
	})

	It("publishes the directory it started with, and the next deploy publishes the saved one", func() {
		nn := create("m460-publish")
		build1(nn)

		By("saving publish directory out2 while gen-1 builds")
		save(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "out2" })
		pass(nn)

		finishFirst(nn)
		Expect(publishedPath(nn.Name, "rev-1")).To(Equal("out"), "the first deploy publishes what it started with")
		res := passes(nn, 3)
		expectLive(nn)
		Expect(envtestApp(nn).Status.PendingReleaseGeneration).To(BeZero(), "the save coalesced while it built is owed nothing")
		Expect(res.RequeueAfter).NotTo(Equal(pendingReleaseRequeue))

		By("the saved directory waits for a deploy, and reads as an undeployed change")
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		app := envtestApp(nn)
		Expect(app.Status.UndeployedChanges).To(BeTrue())
		Expect(app.Status.UndeployedChangesReason).To(Equal(appv1alpha1.UndeployedReasonSaved))

		By("bex deploys create publishes out2")
		gen := deploy(nn, "2026-11-17T08:00:00Z")
		rev := "rev-" + strconv.FormatInt(gen, 10)
		passes(nn, 3)
		completeBuildJob(nn.Name, "gen-"+strconv.FormatInt(gen, 10))
		passes(nn, 2)
		Expect(publishedPath(nn.Name, rev)).To(Equal("out2"))
		completeEnvtestJob(publishJob(nn.Name, rev), metav1.Now())
		passes(nn, 3)
		app = envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal(rev))
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		_, err = r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(envtestApp(nn).Status.UndeployedChanges).To(BeFalse(), "the deploy shipped the saved directory")
	})

	It("enforces an IP rule saved during the build, before and after the deploy settles", func() {
		nn := create("m460-iprule")
		build1(nn)
		Expect(backend(nn)).To(Equal(activatorAliasName(nn.Name)), "precondition: the activator answers the first build")
		Expect(ipRuleRouted(nn)).To(BeFalse())

		By("saving an IP rule together with a publish directory while gen-1 builds")
		save(nn, func(s *appv1alpha1.AppSpec) {
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
		save(nn, func(s *appv1alpha1.AppSpec) { s.RootDir = "sub" })
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

			save(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "public" })
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

			save(nn, func(s *appv1alpha1.AppSpec) { s.PublishPath = "public" })
			pass(nn)
			completeEnvtestJob(publishJob(nn.Name, "rev-1"), metav1.Now())
			passes(nn, 4)
			expectLive(nn)
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
