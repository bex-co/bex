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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Restart runs "the exact same Git commit and configuration as the
// running instance" (render.com/docs/deploys#restarting-a-service). A saved but
// undeployed port, shutdown delay or pre-deploy command therefore waits for the
// next deploy through a Restart, and a rollback runs its target's (w4/m433).
// Before, the selected record supplied only the command, health path,
// environment and instance count, so a Restart shipped the saved port: a port
// typo saved without deploying took the service down on the next Restart.
var _ = Describe("Restart and rollback run the recorded port, shutdown delay and pre-deploy command (w4/m433)", func() {
	// Each spec has its own App: envtest runs no garbage collector, so a
	// deleted App's release records outlive it.
	var name string
	var nn types.NamespacedName
	named := func(n string) {
		name, nn = n, types.NamespacedName{Name: n, Namespace: "default"}
	}
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
	record := func(gen int64) *runtimeConfigRecord {
		GinkgoHelper()
		rec, err := r.readRuntimeConfigRecord(ctx, getApp(), gen)
		Expect(err).NotTo(HaveOccurred())
		return rec
	}
	// release stamps a deploy verb's generation, selecting a recorded release
	// as bex-api's targetReleaseConfig does when from is positive.
	release := func(from int64, restart bool) int64 {
		GinkgoHelper()
		var ref *appv1alpha1.ReleaseConfigReference
		if from > 0 {
			ref = &appv1alpha1.ReleaseConfigReference{SourceGeneration: from, Image: getApp().Status.Image,
				PreserveGroupValues: restart, Runtime: record(from).spec.Runtime}
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
	preDeployCommand := func(gen int64) string {
		GinkgoHelper()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen))}, job)).To(Succeed())
		completeEnvtestJob(job, metav1.Now())
		return fmt.Sprint(job.Spec.Template.Spec.Containers[0].Command)
	}
	// serve runs gen's pre-deploy step and rollout to live and returns the
	// command its step ran.
	serve := func(gen int64) string {
		GinkgoHelper()
		pass(2)
		command := preDeployCommand(gen)
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
		return command
	}
	runs := func() (port string, grace int64) {
		GinkgoHelper()
		tmpl := envtestDeployment(nn).Spec.Template
		c := findContainer(tmpl.Spec.Containers, appContainerName)
		Expect(c).NotTo(BeNil())
		for _, env := range c.Env {
			if env.Name == portEnvName {
				port = env.Value
			}
		}
		return port, *tmpl.Spec.TerminationGracePeriodSeconds
	}
	servicePort := func() int32 {
		GinkgoHelper()
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		return svc.Spec.Ports[0].Port
	}
	undeployed := func() bool {
		GinkgoHelper()
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return getApp().Status.UndeployedChanges
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		background := metav1.DeletePropagationBackground
		for gen := range 8 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen)), Namespace: "default"}},
				&client.DeleteOptions{PropagationPolicy: &background})
		}
	})

	It("keeps the running values through a Restart, applies the saved ones on deploy, and rolls back to the target's", func() {
		named("restart-runtime-settings")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, PreDeployCommand: "echo one"},
		})).To(Succeed())
		Expect(serve(1)).To(ContainSubstring("echo one"))
		Expect(record(1).spec.Runtime).To(Equal(&appv1alpha1.ReleaseRuntimeSettings{Port: 3000, PreDeployCommand: "echo one"}),
			"a release records the settings it ran with")
		port, grace := runs()
		Expect(port).To(Equal("3000"))
		Expect(undeployed()).To(BeFalse())

		By("a save-only PATCH of the port, the shutdown delay and the pre-deploy command")
		update(func(app *appv1alpha1.App) {
			app.Spec.Port, app.Spec.MaxShutdownDelaySeconds, app.Spec.PreDeployCommand = 9000, new(int32(40)), "echo two"
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
		pass(2)
		Expect(undeployed()).To(BeTrue())

		By("a Restart keeps the running port, shutdown delay and pre-deploy command")
		restarted := release(1, true)
		Expect(serve(restarted)).To(ContainSubstring("echo one"), "the Restart reruns the running release's step, not the saved one")
		restartPort, restartGrace := runs()
		Expect(restartPort).To(Equal(port), "the Restart serves the running port")
		Expect(restartGrace).To(Equal(grace), "the Restart keeps the running shutdown delay")
		Expect(servicePort()).To(Equal(int32(3000)))
		Expect(getApp().ServingPort()).To(Equal(int32(3000)), "readers report the port the Restart serves")
		Expect(record(restarted).spec.Runtime.Port).To(Equal(int32(3000)), "the Restart records what it ran")
		Expect(undeployed()).To(BeTrue(), "the saved settings are still not deployed")
		Expect(getApp().Status.UndeployedChangesReason).To(Equal(appv1alpha1.UndeployedReasonHistoricalRelease))
		Expect(getApp().Spec.Port).To(Equal(int32(9000)), "the saved settings are untouched")

		By("the next deploy applies the saved values")
		deployed := release(0, false)
		Expect(serve(deployed)).To(ContainSubstring("echo two"))
		port, deployedGrace := runs()
		Expect(port).To(Equal("9000"))
		Expect(deployedGrace).To(Equal(grace + 10))
		Expect(servicePort()).To(Equal(int32(9000)))
		Expect(undeployed()).To(BeFalse(), "the deploy cleared the saved settings")

		By("a rollback to the first deploy runs its recorded values while the saved ones stay")
		rolledBack := release(1, false)
		Expect(serve(rolledBack)).To(ContainSubstring("echo one"))
		port, rollbackGrace := runs()
		Expect(port).To(Equal("3000"))
		Expect(rollbackGrace).To(Equal(grace))
		app := getApp()
		Expect(app.Spec.Port).To(Equal(int32(9000)))
		Expect(app.Spec.PreDeployCommand).To(Equal("echo two"))
		Expect(undeployed()).To(BeTrue())
	})

	It("restarts a release whose record predates the settings with the saved values", func() {
		named("restart-runtime-legacy")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, PreDeployCommand: "echo one"},
		})).To(Succeed())
		serve(1)
		By("rewriting generation 1's record as an operator before w4/m433 wrote it")
		legacy := record(1)
		legacy.spec.Runtime = nil
		Expect(r.persistRuntimeConfigRecord(ctx, getApp(), 1, legacy)).To(Succeed())
		update(func(app *appv1alpha1.App) {
			app.Spec.Port, app.Spec.PreDeployCommand = 9000, "echo two"
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
		restarted := release(1, true)
		Expect(serve(restarted)).To(ContainSubstring("echo two"))
		port, _ := runs()
		Expect(port).To(Equal("9000"), "no recorded port: the saved one applies, as before")
		Expect(record(restarted).spec.Runtime.Port).To(Equal(int32(9000)), "the Restart records what it ran")
	})
})
