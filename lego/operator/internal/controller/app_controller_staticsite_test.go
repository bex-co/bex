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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("reconciling a static_site App", func() {
	const ns = "default"

	// A reconciler with the static-site plane configured.
	staticReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore: publish.Store{
				Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3",
			},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
		}
	}

	reconcileN := func(r *AppReconciler, nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	// reconcileFailing drives the reconcile for an invalid spec: the finalizer pass
	// requeues cleanly, then the reconcile that reaches the static-site path calls
	// r.fail — which sets Phase=Failed and returns the error (so controller-runtime
	// requeues). We tolerate that error and assert on the recorded phase instead.
	reconcileFailing := func(r *AppReconciler, nn types.NamespacedName) {
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
	}

	It("serves from the static-server without a Deployment/Service", func() {
		const name = "site-serve"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Image:       "site:v1", // prebuilt => no build path
				PublishPath: "dist",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())

		// Pre-mark this generation as already published so the reconcile skips the
		// publish Job (which can't complete under envtest — no kubelet) and takes
		// the serving path.
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Status.ActiveRevision = revFor(app)
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		reconcileN(staticReconciler(), nn)

		By("creating no Deployment and no Service for the served content")
		var dep appsv1.Deployment
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, nn, &dep))).To(BeTrue())

		By("routing the host Ingress at the shared static-server")
		var ing networkingv1.Ingress
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		Expect(ing.Spec.Rules).To(HaveLen(1))
		Expect(ing.Spec.Rules[0].Host).To(Equal(name + ".onbex.co"))
		backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
		aliasName := staticServerAliasName(name)
		Expect(backend.Name).To(Equal(aliasName))
		Expect(backend.Port.Number).To(Equal(int32(8080)))
		var alias corev1.Service
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: aliasName, Namespace: ns}, &alias)).To(Succeed())
		Expect(alias.Spec.Type).To(Equal(corev1.ServiceTypeExternalName))
		Expect(alias.Spec.ExternalName).To(Equal("bex-static-server.bex-system.svc.cluster.local"))
		Expect(alias.Spec.Ports).To(HaveLen(1))
		Expect(alias.Spec.Ports[0].Name).To(Equal("http"))
		Expect(alias.Spec.Ports[0].Port).To(Equal(int32(8080)))
		Expect(alias.Labels).To(HaveKeyWithValue(labelApp, name))
		Expect(alias.Labels).To(HaveKeyWithValue(labelPlatformAliasPurpose, platformAliasStatic))
		Expect(alias.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "bex-operator"))
		Expect(alias.OwnerReferences).To(HaveLen(1))
		Expect(alias.OwnerReferences[0].UID).To(Equal(app.UID))

		By("reporting Running with the served URL")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.URL).To(Equal("https://" + name + ".onbex.co"))
		Expect(app.Status.StaticPrefix).To(BeEmpty())
	})

	It("keeps the host Ingress + certificate on the static-server when suspended (w3/m46)", func() {
		const name = "site-suspend"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Image:       "site:v1",
				PublishPath: "dist",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Status.ActiveRevision = revFor(app)
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		By("serving while running")
		reconcileN(staticReconciler(), nn)
		var ing networkingv1.Ingress
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		Expect(ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).To(Equal(staticServerAliasName(name)))
		Expect(ing.Spec.TLS).NotTo(BeEmpty())

		By("suspending the site")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.Suspended = true
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(staticReconciler(), nn)

		By("keeping the Ingress + TLS pointed at the static-server (not deleting it) so the managed cert survives")
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		Expect(ing.Spec.Rules).To(HaveLen(1))
		Expect(ing.Spec.Rules[0].Host).To(Equal(name + ".onbex.co"))
		Expect(ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).To(Equal(staticServerAliasName(name)))
		// The per-host TLS entry (cert-manager Certificate + Traefik-served cert)
		// is retained — the crux of the fix: a suspended site keeps its managed
		// cert instead of falling back to Traefik's default self-signed cert.
		Expect(ing.Spec.TLS).NotTo(BeEmpty())

		By("reporting Hibernated with the URL kept")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
		Expect(app.Status.URL).To(Equal("https://" + name + ".onbex.co"))
	})

	It("keeps an already-published labeled site on the empty/legacy prefix across reconcile", func() {
		const name = "site-upgrade-prefix"
		const ws = "tea-aaaaaaaaaaaaaaaaaaaa"
		// Canonical ADR043 placement: namespace == workspace (a labeled App in
		// the shared namespace is refused since codex-security 2026-08 F11).
		_ = k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ws},
		}) // AlreadyExists is fine: two specs share this workspace id
		nn := types.NamespacedName{Name: name, Namespace: ws}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: ws,
				Labels: map[string]string{labelWorkspace: ws},
			},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Image:       "site:v1",
				PublishPath: "dist",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Status.ActiveRevision = revFor(app)
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		reconcileN(staticReconciler(), nn)

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.StaticPrefix).To(BeEmpty(),
			"inventing W/A/<rev>/ without a publish would serve an empty prefix")
	})

	It("direct-publishes a no-build static_site by cloning the repo (w9/010)", func() {
		const name = "site-direct"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Repo:        "https://github.com/bex-co/bex",
				RootDir:     "examples/static-site",
				PublishPath: ".",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data: map[string][]byte{
				"AWS_ACCESS_KEY_ID":     []byte("test-access"),
				"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
			},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })

		// Round-13 #6: the reconcile no longer blocks inside publish — Ensure
		// dispatches the Job and returns RequeueAfter, so the Job is completed by
		// hand between two reconcile rounds (envtest has no kubelet to run it).
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			r := staticReconciler()
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}()

		By("dispatching a clone-mode publish Job and no build Job")
		var job batchv1.Job
		jobNN := types.NamespacedName{Name: "pub-" + name + "-rev-1", Namespace: ns}
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &job) }, "30s", "250ms").Should(Succeed())
		Eventually(done, "30s").Should(BeClosed())
		Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(1))
		clone := job.Spec.Template.Spec.InitContainers[0]
		Expect(clone.Name).To(Equal("clone"))
		Expect(clone.Image).To(Equal(publish.DefaultGitImage))
		env := map[string]string{}
		for _, e := range clone.Env {
			env[e.Name] = e.Value
		}
		Expect(env["REPO"]).To(Equal("https://github.com/bex-co/bex"))
		Expect(env["SRC_DIR"]).To(Equal("examples/static-site"))
		var bld batchv1.Job
		Expect(apierrors.IsNotFound(
			k8sClient.Get(ctx, types.NamespacedName{Name: "bld-" + name + "-gen-1", Namespace: ns}, &bld),
		)).To(BeTrue())

		By("completing the publish Job by hand and reconciling again")
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.CompletionTime = &now
		job.Status.Conditions = append(job.Status.Conditions,
			batchv1.JobCondition{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		)
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
		r := staticReconciler()
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})

		By("serving from the static-server with no image recorded")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.Image).To(BeEmpty())
		Expect(app.Status.StaticPrefix).To(Equal("site-direct/rev-1/"))
		var ing networkingv1.Ingress
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		Expect(ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).To(Equal(staticServerAliasName(name)))
	})

	// w6/038: a failed publish used to reach the user as "BackoffLimitExceeded:
	// Job has reached the specified backoff limit" — true and useless, with no
	// mention anywhere of the wrong Publish Directory that actually caused it.
	// This pins the whole seam: what the failing container recorded becomes the
	// App's own Ready-condition message, which is what bex-api stamps on the
	// deploy row's failureReason and the dashboard renders.
	It("explains a failed publish in the failing container's own words, not Kubernetes' backoff reason", func() {
		const name = "site-publish-failed"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Repo:        "https://github.com/bex-co/bex",
				PublishPath: "totally-nonexistent-output-dir-xyz",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data: map[string][]byte{
				"AWS_ACCESS_KEY_ID":     []byte("test-access"),
				"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
			},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			r := staticReconciler()
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}()
		var job batchv1.Job
		jobNN := types.NamespacedName{Name: "pub-" + name + "-rev-1", Namespace: ns}
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &job) }, "30s", "250ms").Should(Succeed())
		Eventually(done, "30s").Should(BeClosed())

		By("failing the Job the way Kubernetes does, with the container's own words on its pod")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: job.Name + "-abcde", Namespace: ns,
				Labels: map[string]string{"job-name": job.Name},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "upload", Image: "x"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod) })
		pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
			Name: "clone",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1,
				Message:  `the publish directory "totally-nonexistent-output-dir-xyz" does not exist in the repository at "HEAD"` + "\n",
			}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.Conditions = append(job.Status.Conditions,
			batchv1.JobCondition{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue,
				Reason: "BackoffLimitExceeded", Message: "Job has reached the specified backoff limit"},
			batchv1.JobCondition{Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
				Reason: "BackoffLimitExceeded", Message: "Job has reached the specified backoff limit"},
		)
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())

		r := staticReconciler()
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Reason).To(Equal("PublishFailed"))
		Expect(cond.Message).To(ContainSubstring("totally-nonexistent-output-dir-xyz"),
			"the user must learn WHICH directory is missing")
		Expect(cond.Message).NotTo(ContainSubstring("BackoffLimitExceeded"),
			"Kubernetes' generic backoff reason must not be what the user is left with")
	})

	It("records a workspace-scoped staticPrefix after a completed labeled publish", func() {
		const name = "site-labeled-publish"
		const ws = "tea-aaaaaaaaaaaaaaaaaaaa"
		// Canonical ADR043 placement (see the site-upgrade-prefix spec above).
		_ = k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ws},
		}) // AlreadyExists is fine: two specs share this workspace id
		nn := types.NamespacedName{Name: name, Namespace: ws}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: ws,
				Labels: map[string]string{labelWorkspace: ws},
			},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Repo:        "https://github.com/bex-co/bex",
				RootDir:     "examples/static-site",
				PublishPath: ".",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3-labeled", Namespace: ws},
			Data: map[string][]byte{
				"AWS_ACCESS_KEY_ID":     []byte("test-access"),
				"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
			},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })
		r := staticReconciler()
		r.StaticStore.Secret = "static-s3-labeled"

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}()
		var job batchv1.Job
		jobNN := types.NamespacedName{Name: "pub-" + name + "-rev-1", Namespace: ws}
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &job) }, "30s", "250ms").Should(Succeed())
		Eventually(done, "30s").Should(BeClosed())
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.CompletionTime = &now
		job.Status.Conditions = append(job.Status.Conditions,
			batchv1.JobCondition{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		)
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.StaticPrefix).To(Equal(ws + "/" + name + "/rev-1/"))
	})

	It("fails before dispatch when the publish credential Secret is missing", func() {
		const name = "site-no-publish-credential"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Image: "site:v1", PublishPath: "dist", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())

		reconcileFailing(staticReconciler(), nn)

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		Expect(app.Status.Conditions).To(ContainElement(And(
			HaveField("Type", "Ready"),
			HaveField("Reason", "StaticCredentialUnavailable"),
			HaveField("Message", ContainSubstring("static publish credential Secret default/static-s3 is unavailable")),
		)))
		var publishJob batchv1.Job
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx,
			types.NamespacedName{Name: "pub-" + name + "-rev-1", Namespace: ns}, &publishJob))).To(BeTrue())
	})

	It("fails a static_site with no publishPath", func() {
		const name = "site-nopublish"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Image: "site:v1", Expose: true},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileFailing(staticReconciler(), nn)

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
	})

	It("fails a static_site when the object store is unconfigured", func() {
		const name = "site-nostore"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Image: "site:v1", PublishPath: "dist", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())

		// A reconciler with no StaticStore/StaticServerService configured.
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
		}
		reconcileFailing(r, nn)

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
	})
})

// revFor is the revision string reconcileStaticSite computes for an App.
func revFor(app *appv1alpha1.App) string {
	return fmt.Sprintf("rev-%d", app.Generation)
}
