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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The build package proves the cache phases are shaped correctly; this proves
// the operator actually asks for them. BEX_BUILD_CACHE reaching the Job spec is
// the one seam whose failure is completely silent — an unwired gate produces
// builds that work, cost nothing extra, and never cache, which is
// indistinguishable from a cache that is merely not hitting yet.
//
// It also submits the cached Job to a REAL API server, which the unit tests
// structurally cannot: two containers in a Job template and a second emptyDir
// are schema-visible changes, and a spec the server prunes or rejects would
// stop every build from dispatching.
var _ = Describe("build cache dispatch", func() {
	// Canonical ADR043 placement: namespace == workspace label (a labeled App
	// in the shared namespace is refused since codex-security 2026-08 F11).
	const ns = "tea-w1"

	dispatch := func(name string, cache bool) *batchv1.Job {
		ctx := context.Background()
		_ = k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}) // AlreadyExists is fine across specs
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: ns,
				Labels: map[string]string{labelWorkspace: "tea-w1"},
			},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })

		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			Registry: "zot.test:5000", BuildCache: cache,
		}
		nn := types.NamespacedName{Name: name, Namespace: ns}
		// Several passes: the build is only dispatched once the App has settled
		// through its earlier reconcile steps. The last error is kept so a
		// reconcile that fails every pass reports its reason instead of surfacing
		// as an unexplained Eventually timeout.
		var lastErr error
		for range 5 {
			if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
				lastErr = err
			}
		}
		var job batchv1.Job
		jobNN := types.NamespacedName{Name: build.JobName(name, "gen-1"), Namespace: ns}
		Eventually(func() error {
			return k8sClient.Get(ctx, jobNN, &job)
		}, "30s", "250ms").Should(Succeed(),
			"the reconcile never dispatched a build Job; last reconcile error: %v", lastErr)
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, &job) })
		return &job
	}

	names := func(cs []corev1.Container) []string {
		out := make([]string, 0, len(cs))
		for _, c := range cs {
			out = append(out, c.Name)
		}
		return out
	}

	It("dispatches the cache phases when BEX_BUILD_CACHE is on, and the API server keeps them", func() {
		job := dispatch("cache-on-app", true)
		spec := job.Spec.Template.Spec
		Expect(names(spec.InitContainers)).To(ContainElement("cache-restore"))
		Expect(names(spec.Containers)).To(ContainElement("cache-save"))

		volumes := make([]string, 0, len(spec.Volumes))
		for _, v := range spec.Volumes {
			volumes = append(volumes, v.Name)
		}
		Expect(volumes).To(ContainElements("cache-in", "cache-out"))

		// The cache must address this App's OWN repository column.
		var restore *corev1.Container
		for i := range spec.InitContainers {
			if spec.InitContainers[i].Name == "cache-restore" {
				restore = &spec.InitContainers[i]
			}
		}
		Expect(restore).NotTo(BeNil())
		Expect(restore.Args).To(ContainElement(ContainSubstring("tea-w1/cache-on-app-cache:cache")))
	})

	It("dispatches an unchanged build Job when the gate is off", func() {
		job := dispatch("cache-off-app", false)
		spec := job.Spec.Template.Spec
		Expect(names(spec.InitContainers)).NotTo(ContainElement("cache-restore"))
		Expect(names(spec.Containers)).NotTo(ContainElement("cache-save"))
		Expect(spec.Containers).To(HaveLen(1))
		for _, v := range spec.Volumes {
			Expect(v.Name).NotTo(Or(Equal("cache-in"), Equal("cache-out")))
		}
	})
})
