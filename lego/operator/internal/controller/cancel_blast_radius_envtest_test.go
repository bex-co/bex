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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cancel_blast_radius_envtest_test.go is w1/m152 t004: the cancel rule — the
// last successful release stays live, no pod rolls — for EVERY kind of change
// that opens a config_change deploy, not only the service env var the defect
// was filed with. Each entry mutates what the backend's save mutates (the
// source Secret or the spec, plus spec.restartedAt), cancels that generation
// before it reaches the Deployment, and requires the served template back
// byte for byte.
var _ = Describe("Canceling any config_change (w1/m152 t004)", func() {
	// Every spec gets fresh names: the App finalizer keeps a deleted App around
	// until a reconcile releases it, so names cannot be reused between specs.
	var r *AppReconciler
	var group, svcA, svcB string
	seq := 0
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		seq++
		svcA, svcB = "blast-"+strconv.Itoa(seq)+"-a", "blast-"+strconv.Itoa(seq)+"-b"
		group = "evg-blast-" + strconv.Itoa(seq) + "-env"
	})

	nn := func(name string) types.NamespacedName { return types.NamespacedName{Name: name, Namespace: "default"} }
	pass := func(names ...string) {
		GinkgoHelper()
		for range 3 {
			for _, n := range names {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(n)})
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}
	getApp := func(name string) *appv1alpha1.App {
		GinkgoHelper()
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn(name), app)).To(Succeed())
		return app
	}
	getDep := func(name string) *appsv1.Deployment {
		GinkgoHelper()
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn(name), dep)).To(Succeed())
		return dep
	}
	markReady := func(name string) {
		GinkgoHelper()
		dep := getDep(name)
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas = 1, 1
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
	}
	putSecret := func(name, key, val string) {
		GinkgoHelper()
		s := &corev1.Secret{}
		if err := k8sClient.Get(ctx, nn(name), s); err != nil {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Data:       map[string][]byte{key: []byte(val)},
			})).To(Succeed())
			return
		}
		s.Data = map[string][]byte{key: []byte(val)}
		Expect(k8sClient.Update(ctx, s)).To(Succeed())
	}
	// serve creates a linked service reading its own env, its own files and the
	// shared group, and drives it to a served generation 1.
	serve := func(name string) *corev1.PodTemplateSpec {
		GinkgoHelper()
		putSecret(name+"-env", "MESSAGE", "OK")
		putSecret(name+"-files", "qa.txt", "v1-marker")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image: "nginx:1", Port: 3000, Replicas: 1,
				EnvFromSecret:    name + "-env",
				EnvFromSecrets:   []string{group},
				FilesFromSecrets: []string{name + "-files"},
				StartCommand:     "serve --v1",
				HealthCheckPath:  "/healthz",
				Tier:             "starter",
			},
		})).To(Succeed())
		pass(name)
		markReady(name)
		pass(name)
		Expect(getApp(name).Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")
		return getDep(name).Spec.Template.DeepCopy()
	}
	// save applies a spec change the way the backend's save does: with a
	// restartedAt bump, which is what opens the config_change deploy.
	save := func(name string, mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		app := getApp(name)
		mutate(&app.Spec)
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	cancel := func(name string) {
		GinkgoHelper()
		app := getApp(name)
		if app.Annotations == nil {
			app.Annotations = map[string]string{}
		}
		app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = strconv.FormatInt(app.Generation, 10)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	expectServed := func(name string, served *corev1.PodTemplateSpec) {
		GinkgoHelper()
		Expect(getDep(name).Spec.Template).To(Equal(*served),
			"a canceled change must leave the served pod template — and so every pod — exactly as it was")
		app := getApp(name)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.UndeployedChanges).To(BeTrue())
	}

	AfterEach(func() {
		for _, n := range []string{svcA, svcB} {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn(n), app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			}
			for _, s := range []string{n + "-env", n + "-files"} {
				_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: s, Namespace: "default"}})
			}
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: group, Namespace: "default"}})
	})

	DescribeTable("the served release stays live",
		func(change func(name string)) {
			putSecret(group, "SHARED", "g1")
			served := serve(svcA)
			change(svcA)
			cancel(svcA)
			pass(svcA)
			expectServed(svcA, served)
			// Nothing of the canceled release runs — including its pre-deploy
			// command, which is a migration more often than not.
			var jobs batchv1.JobList
			Expect(k8sClient.List(ctx, &jobs)).To(Succeed())
			for _, j := range jobs.Items {
				Expect(j.Name).NotTo(ContainSubstring(svcA), "a canceled release started Job %s", j.Name)
			}
		},
		Entry("secret file", func(n string) {
			putSecret(n+"-files", "qa.txt", "v2-should-not-ship")
			save(n, func(*appv1alpha1.AppSpec) {})
		}),
		Entry("linked env group value", func(n string) {
			putSecret(group, "SHARED", "g2-should-not-ship")
			save(n, func(*appv1alpha1.AppSpec) {})
		}),
		Entry("preDeployCommand", func(n string) {
			save(n, func(s *appv1alpha1.AppSpec) { s.PreDeployCommand = "migrate --should-not-run" })
		}),
		Entry("start command", func(n string) {
			save(n, func(s *appv1alpha1.AppSpec) { s.StartCommand = "serve --should-not-ship" })
		}),
		Entry("health check path", func(n string) {
			save(n, func(s *appv1alpha1.AppSpec) { s.HealthCheckPath = "/should-not-ship" })
		}),
		Entry("plan", func(n string) {
			save(n, func(s *appv1alpha1.AppSpec) { s.Tier = "standard" })
		}),
		Entry("code and config together", func(n string) {
			putSecret(n+"-env", "MESSAGE", "should-not-ship")
			save(n, func(s *appv1alpha1.AppSpec) { s.Image = "nginx:2" })
		}),
	)

	// Live on 2026-09-14 (pass 19) a canceled env change reached the very next
	// cron run: the CronJob template read the mutable Secret and was re-projected
	// from the saved spec. A run is a pod created from the CronJob's template, so
	// the served template staying put is what keeps every later run on the served
	// release.
	It("a cron job's runs stay on the served release", func() {
		putSecret(svcA+"-env", "MESSAGE", "OK")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: svcA, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeCronJob, Image: "busybox:1", Schedule: "*/5 * * * *",
				EnvFromSecret: svcA + "-env", StartCommand: "tick",
			},
		})).To(Succeed())
		pass(svcA)
		Expect(getApp(svcA).Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")
		cronTemplate := func() corev1.PodTemplateSpec {
			GinkgoHelper()
			cj := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, nn(appv1alpha1.CronJobName(svcA)), cj)).To(Succeed())
			return cj.Spec.JobTemplate.Spec.Template
		}
		served := cronTemplate()
		Expect(served.Spec.Containers[0].EnvFrom[0].SecretRef.Name).To(
			Equal(appv1alpha1.AppReleaseSnapshotName(svcA, svcA+"-env", 1)),
			"runs read the release's own copy, not the mutable Secret a save rewrites")

		putSecret(svcA+"-env", "MESSAGE", "should-not-ship")
		save(svcA, func(s *appv1alpha1.AppSpec) { s.StartCommand = "tick --should-not-ship" })
		cancel(svcA)
		pass(svcA)

		Expect(cronTemplate()).To(Equal(served))
		app := getApp(svcA)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.UndeployedChanges).To(BeTrue())
	})

	// The other direction of the same rule: a release's pre-deploy step runs as
	// part of THAT release, so it must read that release's configuration, not the
	// served one's. The step runs before the new release is snapshotted, while the
	// projection still points at the served generation's copies.
	It("the next release's pre-deploy command reads the next release's configuration", func() {
		putSecret(group, "SHARED", "g1")
		serve(svcA)
		putSecret(svcA+"-env", "DATABASE_URL", "postgres://new")
		save(svcA, func(s *appv1alpha1.AppSpec) { s.PreDeployCommand = "migrate" })
		pass(svcA)

		var jobs batchv1.JobList
		Expect(k8sClient.List(ctx, &jobs)).To(Succeed())
		var job *batchv1.Job
		for i := range jobs.Items {
			if jobs.Items[i].Labels[labelApp] == svcA {
				job = &jobs.Items[i]
			}
		}
		Expect(job).NotTo(BeNil(), "the pre-deploy Job for generation 2")
		ownRef := ""
		for _, from := range job.Spec.Template.Spec.Containers[0].EnvFrom {
			if name := from.SecretRef.Name; name == svcA+"-env" || strings.HasPrefix(name, svcA+"-env-r") {
				ownRef = name
			}
		}
		own := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, nn(ownRef), own)).To(Succeed(), "the migration's own env source %q", ownRef)
		Expect(own.Data).To(HaveKeyWithValue("DATABASE_URL", []byte("postgres://new")),
			"generation 2's migration reads %s — the served release's copy, without the saved DATABASE_URL", ownRef)
	})

	It("a group save canceled on every linked service changes none of them", func() {
		putSecret(group, "SHARED", "g1")
		servedA, servedB := serve(svcA), serve(svcB)

		putSecret(group, "SHARED", "g2-should-not-ship")
		for _, n := range []string{svcA, svcB} {
			save(n, func(*appv1alpha1.AppSpec) {})
			cancel(n)
		}
		pass(svcA, svcB)

		expectServed(svcA, servedA)
		expectServed(svcB, servedB)
		for _, n := range []string{svcA, svcB} {
			snap := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, nn(appv1alpha1.AppReleaseSnapshotName(n, group, 1)), snap)).To(Succeed())
			Expect(snap.Data).To(HaveKeyWithValue("SHARED", []byte("g1")),
				"each linked service keeps its own copy of the group as it served it")
			Expect(metav1.IsControlledBy(snap, getApp(n))).To(BeTrue())
		}
	})
})
