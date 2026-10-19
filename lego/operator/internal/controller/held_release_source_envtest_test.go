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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A release in flight builds and runs what it was adopted with, even when a
// later save arrives before its build starts (w8/m121). Without its adopted
// view the build read the saved spec: an Update Source over a queued build
// rolled the previous image as the new release, a switch to a prebuilt image
// mid-build did the same, and a newer deploy's commit was built under the
// queued release's name.
var _ = Describe("A release in flight builds its own source after a later save (w8/m121)", func() {
	const name = "held-source"
	const blocker = "held-source-blocker"
	const commitX, commitY = "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	envName := name + "-env"
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func() {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	getApp := func() *appv1alpha1.App { GinkgoHelper(); return envtestApp(nn) }
	appImage := func() string {
		GinkgoHelper()
		c := findContainer(envtestDeployment(nn).Spec.Template.Spec.Containers, appContainerName)
		Expect(c).NotTo(BeNil())
		return c.Image
	}
	// cloneEnv is what the release's build Job fetches.
	cloneEnv := func(rev, key string) string {
		GinkgoHelper()
		j, err := buildJobFor(name, rev)
		Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
		for _, c := range append(j.Spec.Template.Spec.InitContainers, j.Spec.Template.Spec.Containers...) {
			if v := envValue(c.Env, key); v != "" {
				return v
			}
		}
		return ""
	}
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	// deploy saves a deploy verb's spec change, stamped with the generation the
	// update moves to, as bex-api opens one.
	deploy := func(mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
		})
	}
	// updateSource saves an Update Source: the next deploy consumes it.
	updateSource := func(mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) { mutate(&app.Spec) })
		update(func(app *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation))
		})
	}
	// holdBuilds fills the cluster build ceiling with another build, so a new
	// dispatch queues.
	holdBuilds := func() {
		GinkgoHelper()
		r.MaxActiveBuilds = 1
		Expect(k8sClient.Create(ctx, &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: blocker, Namespace: "default",
				Labels: map[string]string{execution.LabelComponent: "build"}},
			Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers:    []corev1.Container{{Name: "build", Image: "busybox"}},
			}}},
		})).To(Succeed())
	}
	releaseBuilds := func() { r.MaxActiveBuilds = 0 }
	serve := func() {
		GinkgoHelper()
		markEnvtestDeploymentReady(nn)
		pass()
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: envName, Namespace: "default"}})
		background := metav1.DeletePropagationBackground
		for _, job := range []string{blocker, build.JobName(name, "gen-1"), build.JobName(name, "gen-2"), build.JobName(name, "gen-3"),
			build.JobName(name, "gen-4"), predeploy.JobName(name, "gen-2")} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job, Namespace: "default"}},
				&client.DeleteOptions{PropagationPolicy: &background})
		}
	})

	// servesFirst serves generation 1 from main at commit X and returns its image.
	servesFirst := func(mutate ...func(*appv1alpha1.AppSpec)) string {
		GinkgoHelper()
		spec := appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", BuildCommit: commitX,
			Builder: "dockerfile", Port: 3000, Replicas: 1,
		}
		for _, m := range mutate {
			m(&spec)
		}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Spec: spec,
		})).To(Succeed())
		for range 3 {
			pass()
		}
		completeBuildJob(name, "gen-1")
		pass()
		serve()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")
		return getApp().Status.Image
	}

	It("builds its own branch when an Update Source lands before its build starts", func() {
		first := servesFirst()

		By("deploying generation 2 while the build ceiling is full")
		holdBuilds()
		deploy(func(spec *appv1alpha1.AppSpec) { spec.BuildCommit = "" })
		pass()
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)))
		_, err := buildJobFor(name, "gen-2")
		Expect(err).To(HaveOccurred(), "precondition: generation 2's build is still queued")

		By("an Update Source moves the branch; it waits for the next deploy")
		updateSource(func(spec *appv1alpha1.AppSpec) { spec.Branch = "next" })
		pass()
		serve()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"),
			"generation 2 must not roll generation 1's image as rev-2 before its own build")
		Expect(appImage()).To(Equal(first))

		By("generation 2 builds main, its own branch, and serves its own image")
		releaseBuilds()
		pass()
		Expect(cloneEnv("gen-2", "REF")).To(Equal("main"))
		completeBuildJob(name, "gen-2")
		pass()
		Expect(appImage()).NotTo(Equal(first))
		serve()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-2"))
		Expect(getApp().Spec.Branch).To(Equal("next"), "the saved source is untouched by the held passes")

		By("the saved branch still waits for the next deploy")
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-2"))
		_, err = buildJobFor(name, "gen-3")
		Expect(err).To(HaveOccurred(), "Update Source must not build until a deploy consumes it")
	})

	It("finishes its own repo build when an Update Source switches to a prebuilt image", func() {
		first := servesFirst()

		By("generation 2's build is running when the source switches to an image")
		deploy(func(spec *appv1alpha1.AppSpec) { spec.BuildCommit = "" })
		pass()
		_, err := buildJobFor(name, "gen-2")
		Expect(err).NotTo(HaveOccurred(), "precondition: generation 2 is building")
		updateSource(func(spec *appv1alpha1.AppSpec) {
			spec.Image, spec.Repo, spec.Branch, spec.Builder = "ghcr.io/example/prebuilt:v1", "", "", ""
		})
		pass()
		serve()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"),
			"generation 2 must not roll generation 1's image as rev-2")

		By("generation 2 rolls its own built image, not the prebuilt one")
		completeBuildJob(name, "gen-2")
		pass()
		Expect(appImage()).NotTo(Equal(first))
		Expect(appImage()).NotTo(Equal("ghcr.io/example/prebuilt:v1"))
		serve()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-2"))
		Expect(getApp().Spec.Image).To(Equal("ghcr.io/example/prebuilt:v1"))
	})

	It("builds its own commit when a newer deploy arrives while it is queued", func() {
		servesFirst()

		By("generation 2 for commit X queues; generation 3 for commit Y waits behind it")
		holdBuilds()
		deploy(func(spec *appv1alpha1.AppSpec) { spec.BuildCommit = commitX })
		pass()
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)))
		deploy(func(spec *appv1alpha1.AppSpec) { spec.BuildCommit = commitY })
		pass()
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)))
		Expect(getApp().Status.PendingReleaseGeneration).To(BeNumerically(">", 2))

		By("generation 2 builds commit X, then generation 3 builds commit Y")
		releaseBuilds()
		pass()
		Expect(cloneEnv("gen-2", "EXPECTED_COMMIT")).To(Equal(commitX))
		completeBuildJob(name, "gen-2")
		pass()
		serve()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-2"))
		pass()
		next := fmt.Sprintf("gen-%d", getApp().Status.ReleaseGeneration)
		Expect(next).NotTo(Equal("gen-2"), "the waiting deploy is adopted once generation 2 serves")
		Expect(cloneEnv(next, "EXPECTED_COMMIT")).To(Equal(commitY))
	})

	It("runs its own pre-deploy command with its own environment after a newer save", func() {
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: envName, Namespace: "default"},
			Data:       map[string][]byte{"K": []byte("v1")},
		})).To(Succeed())
		servesFirst(func(spec *appv1alpha1.AppSpec) { spec.EnvFromSecret = envName })
		saveEnv := func(value string) {
			GinkgoHelper()
			env := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: envName}, env)).To(Succeed())
			env.Data = map[string][]byte{"K": []byte(value)}
			Expect(k8sClient.Update(ctx, env)).To(Succeed())
		}

		By("A, with its own pre-deploy step and env value, builds; B then saves both")
		saveEnv("a")
		deploy(func(spec *appv1alpha1.AppSpec) { spec.BuildCommit, spec.PreDeployCommand = "", "migrate A" })
		pass()
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)))
		saveEnv("b")
		deploy(func(spec *appv1alpha1.AppSpec) { spec.PreDeployCommand = "migrate B" })
		pass()

		By("A's pre-deploy step runs A's command and reads A's env value")
		completeBuildJob(name, "gen-2")
		pass()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: predeploy.JobName(name, "gen-2")}, job)).To(Succeed(),
			"precondition: A's pre-deploy step started")
		c := job.Spec.Template.Spec.Containers[0]
		Expect(fmt.Sprint(c.Command, c.Args)).To(ContainSubstring("migrate A"))
		Expect(fmt.Sprint(c.Command, c.Args)).NotTo(ContainSubstring("migrate B"))
		value := ""
		for _, from := range c.EnvFrom {
			if from.SecretRef != nil && from.SecretRef.Name != envName && strings.HasPrefix(from.SecretRef.Name, envName) {
				s := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: from.SecretRef.Name}, s)).To(Succeed())
				value = string(s.Data["K"])
			}
		}
		Expect(value).To(Equal("a"), "A's pre-deploy step reads A's adoption snapshot, not the live Secret B saved")
	})
})
