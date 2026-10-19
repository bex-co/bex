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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m120: a release the latest-pending slot holds through its rollout rolls
// out with its own settings, and its release record says so. A Docker build
// takes its start command from the pod, so a held release projected from the
// newer saved spec ran one release's image with another's command and env,
// and Restart, Rollback and undeployedChanges read a record that named the
// newer settings.
var _ = Describe("A release held through rollout keeps its own settings (w8/m120)", func() {
	const name = "held-settings"
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
	appContainer := func() corev1.Container {
		GinkgoHelper()
		c := findContainer(envtestDeployment(nn).Spec.Template.Spec.Containers, appContainerName)
		Expect(c).NotTo(BeNil())
		return *c
	}
	envValue := func(c corev1.Container) string {
		GinkgoHelper()
		for _, from := range c.EnvFrom {
			if from.SecretRef != nil && strings.HasPrefix(from.SecretRef.Name, envName) {
				s := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: from.SecretRef.Name}, s)).To(Succeed())
				return string(s.Data["K"])
			}
		}
		return ""
	}
	recordedStart := func(gen int64) string {
		GinkgoHelper()
		rec, err := r.readRuntimeConfigRecord(ctx, getApp(), gen)
		Expect(err).NotTo(HaveOccurred())
		return rec.spec.StartCommand
	}
	save := func(start, value string) {
		GinkgoHelper()
		env := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: envName}, env)).To(Succeed())
		env.Data = map[string][]byte{"K": []byte(value)}
		Expect(k8sClient.Update(ctx, env)).To(Succeed())
		app := getApp()
		app.Spec.StartCommand = start
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: envName, Namespace: "default"}})
		for gen := range 4 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: build.JobName(name, fmt.Sprintf("gen-%d", gen+1)), Namespace: "default"}})
		}
	})

	// heldAServes serves generation 1, saves A then B, and rolls A out while B
	// waits, asserting A runs and records its own settings.
	heldAServes := func() {
		GinkgoHelper()
		By("serving generation 1")
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: envName, Namespace: "default"},
			Data:       map[string][]byte{"K": []byte("v1")},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile", Port: 3000,
				Replicas: 1, StartCommand: "serve 1", EnvFromSecret: envName,
			},
		})).To(Succeed())
		for range 3 {
			pass()
		}
		completeBuildJob(name, "gen-1")
		pass()
		markEnvtestDeploymentReady(nn)
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")

		By("saving A, which starts building, then B, which waits behind it")
		save("serve A", "a")
		pass()
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)))
		adoptedA := getApp().Spec
		save("serve B", "b")
		pass()
		Expect(getApp().Status.PendingReleaseGeneration).To(Equal(int64(3)))

		By("A rolls out with A's start command and A's env, not B's")
		completeBuildJob(name, "gen-2")
		pass()
		Expect(appContainer().Command).To(Equal(shellCommand("serve A")))
		Expect(envValue(appContainer())).To(Equal("a"))
		Expect(recordedStart(2)).To(Equal("serve A"), "A's release record names A's settings")
		markEnvtestDeploymentReady(nn)
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-2"))
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(getApp().Status.UndeployedChanges).To(BeTrue(),
			"B's saved settings are not what A runs: a Restart of A must keep them pending (w8/m119)")
		rec, err := r.readRuntimeConfigRecord(ctx, getApp(), 2)
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.spec.SettingsFingerprint).To(Equal(releaseSettingsFingerprint(adoptedA)),
			"A's record keeps the settings fingerprint it was adopted with, which a cancel compares against")
	}

	It("rolls out and records its own start command and env while a newer save waits", func() {
		heldAServes()
		By("B then rolls out with its own settings, and A's record still names A's")
		pass()
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(3)))
		completeBuildJob(name, "gen-3")
		pass()
		Expect(appContainer().Command).To(Equal(shellCommand("serve B")))
		Expect(envValue(appContainer())).To(Equal("b"))
		markEnvtestDeploymentReady(nn)
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-3"))
		Expect(recordedStart(3)).To(Equal("serve B"))
		Expect(recordedStart(2)).To(Equal("serve A"))
	})

	It("a Restart of the held release re-releases its own settings and keeps the newer save pending", func() {
		heldAServes()
		pass() // B is adopted and starts building
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(3)))

		By("restarting A, as bex-api does: select A's record at a new release generation")
		app := getApp()
		image := strings.TrimPrefix(app.Status.Image, "/") // no registry in this suite
		app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, SourceGeneration: 2, Image: image, PreserveGroupValues: true}
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		app.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "4"}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		for range 2 {
			pass()
		}
		Expect(appContainer().Command).To(Equal(shellCommand("serve A")), "the restart runs A's command, not the saved B's")
		Expect(envValue(appContainer())).To(Equal("a"))
		markEnvtestDeploymentReady(nn)
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-4"))
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(getApp().Status.UndeployedChanges).To(BeTrue(), "B is still saved and not deployed")
	})
})
