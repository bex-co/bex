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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/003: a built static site whose latest build failed froze. Its serving
// routing (the IP allow-list Middleware, the host Ingress) stopped following the
// App, because every pass halted at buildFromSource's failed-build gate and the
// prior-release hold (holdPendingArtifact) excludes static sites. The live
// reproduction then left the next deploy at "Build queued".
var _ = Describe("A built static site after a failed build (w32/003)", func() {
	const name = "site-failed-hold"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	mwNN := types.NamespacedName{Name: name + "-ip-allow", Namespace: "default"}

	var r *AppReconciler
	newReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
			BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore: publish.Store{
				Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3",
			},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
		}
	}
	reconcileN := func(n int) {
		for range n {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
	}
	getApp := func() *appv1alpha1.App {
		a := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, a)).To(Succeed())
		return a
	}
	jobExists := func(rev string) bool {
		_, err := buildJobFor(name, rev)
		return err == nil
	}
	completePublish := func(rev string) {
		GinkgoHelper()
		j := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "pub-" + name + "-" + rev, Namespace: "default"}, j)).To(Succeed())
		completeEnvtestJob(j, metav1.Now())
	}
	// update applies a spec edit; deploy also stamps the release generation the
	// way bex-api's deploy verbs do (patch, then annotate generation+1).
	update := func(mutate func(*appv1alpha1.App), deploy bool) {
		GinkgoHelper()
		a := getApp()
		mutate(a)
		if deploy {
			if a.Annotations == nil {
				a.Annotations = map[string]string{}
			}
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(a.Generation+1, 10)
		}
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
	}
	allowList := func() []any {
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		if err := k8sClient.Get(ctx, mwNN, o); err != nil {
			Expect(apierrors.IsNotFound(err)).To(BeTrue())
			return nil
		}
		ranges, _, _ := unstructured.NestedSlice(o.Object, "spec", "ipAllowList", "sourceRange")
		return ranges
	}

	BeforeEach(func() {
		r = newReconciler()
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: "default"},
			Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("a"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })
	})
	AfterEach(func() {
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			saved := r.Registry
			r.Registry = ""
			reconcileN(3)
			r.Registry = saved
		}
		for i := 1; i <= 8; i++ {
			rev := strconv.Itoa(i)
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-"+rev), Namespace: "default"}})
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "pub-" + name + "-rev-" + rev, Namespace: "default"}})
		}
	})

	It("keeps routing converging while the latest build is failed, and builds the next deploy", func() {
		By("publishing a built site: gen-1 build, then its publish")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/bex-canary-static",
				BuildCommand: "mkdir -p out && cp index.html out/", PublishPath: "out", Expose: true,
			},
		})).To(Succeed())
		reconcileN(3)
		Expect(jobExists("gen-1")).To(BeTrue())
		completeBuildJob(name, "gen-1")
		reconcileN(2)
		completePublish("rev-1")
		reconcileN(3)
		live := getApp()
		Expect(live.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(live.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))

		By("deploying a failing build command: gen-2 fails, rev-1 keeps serving")
		update(func(a *appv1alpha1.App) {
			a.Spec.BuildCommand = "echo qa-fail && exit 3"
			a.Spec.RestartedAt = "2026-11-04T11:05:00Z"
		}, true)
		reconcileN(2)
		Expect(jobExists("gen-2")).To(BeTrue())
		failBuildJob(name, "gen-2")
		reconcileN(3)
		failed := getApp()
		buildFailed := func(a *appv1alpha1.App) bool {
			c := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionBuild)
			return c != nil && appv1alpha1.IsBuildFailureReason(c.Reason) && c.ObservedGeneration == a.Status.ReleaseGeneration
		}
		Expect(buildFailed(failed)).To(BeTrue(), "status: %+v", failed.Status)
		Expect(failed.Status.ActiveRevision).To(Equal("rev-1"))

		By("adding an IP allow-list: the serving route enforces it although the latest build failed")
		update(func(a *appv1alpha1.App) {
			a.Spec.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: "198.51.100.0/24", Description: "qa-block"}}
		}, false)
		reconcileN(2)
		Expect(allowList()).To(Equal([]any{"198.51.100.0/24"}))
		ing := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed())
		Expect(ing.Annotations[traefikRouterMiddlewaresAnnotation]).To(ContainSubstring(name + "-ip-allow"))
		Expect(jobExists("gen-3")).To(BeFalse(), "an operational change must not rebuild")
		Expect(buildFailed(getApp())).To(BeTrue(), "the failed deploy stays failed")

		By("clearing it opens the route again")
		update(func(a *appv1alpha1.App) { a.Spec.IPAllowListEntries = nil }, false)
		reconcileN(2)
		Expect(allowList()).To(BeNil())

		By("re-adding it, fixing the build command and deploying: the fix builds and goes live")
		update(func(a *appv1alpha1.App) {
			a.Spec.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: "198.51.100.0/24", Description: "qa-block"}}
		}, false)
		reconcileN(2)
		update(func(a *appv1alpha1.App) {
			a.Spec.BuildCommand = "mkdir -p out && cp index.html out/ && echo built2 > out/marker.txt"
			a.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] = strconv.FormatInt(a.Generation+1, 10)
		}, false)
		reconcileN(2)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "pub-" + name + "-rev-2", Namespace: "default"}, &batchv1.Job{}))).
			To(BeTrue(), "a saved build command must not republish the serving image under the failed release's revision")
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"))
		Expect(allowList()).To(Equal([]any{"198.51.100.0/24"}))
		update(func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-04T11:09:34Z" }, true)
		deployGen := getApp().Generation
		reconcileN(3)
		rev := strconv.FormatInt(deployGen, 10)
		pending := getApp()
		Expect(jobExists("gen-"+rev)).To(BeTrue(), "the fix-forward deploy must dispatch its build; gen=%d ann=%v status=%+v", pending.Generation, pending.Annotations, pending.Status)
		completeBuildJob(name, "gen-"+rev)
		reconcileN(2)
		completePublish("rev-" + rev)
		reconcileN(3)
		done := getApp()
		Expect(done.Status.ActiveRevision).To(Equal("rev-" + rev))
		Expect(done.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(allowList()).To(Equal([]any{"198.51.100.0/24"}))
	})
})
