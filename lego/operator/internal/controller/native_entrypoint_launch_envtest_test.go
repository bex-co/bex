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
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A native service's pre-deploy command and a native cron job's own command
// start under the image's native-init entrypoint, as the service itself does,
// so each run finds the secret files mounted for it in the service root
// (w8/350). Before, both replaced the entrypoint and saw only the names the
// image was built with.
var _ = Describe("Native commands start under the image's entrypoint (w8/350)", func() {
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	native := func(name string, mutate func(*appv1alpha1.AppSpec)) (types.NamespacedName, func() (ctrl.Result, error)) {
		GinkgoHelper()
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		spec := appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Replicas: 1,
			Runtime: "node", Builder: build.BuilderNative, BuildCommand: "npm ci", StartCommand: "node server.js",
			FilesFromSecrets: []string{name + "-files"},
		}
		mutate(&spec)
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace}, Spec: spec})).To(Succeed())
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		DeferCleanup(func() {
			deleteOwner(ctx, envtestApp(nn), run)
			background := metav1.DeletePropagationBackground
			for _, job := range []string{build.JobName(name, "gen-1"), predeploy.JobName(name, "gen-1")} {
				_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job, Namespace: nn.Namespace}},
					&client.DeleteOptions{PropagationPolicy: &background})
			}
		})
		reconcileToFixedPoint(run)
		completeBuildJob(name, "gen-1")
		reconcileToFixedPoint(run)
		return nn, run
	}
	secretFiles := corev1.VolumeMount{Name: secretFilesVolumeName, MountPath: secretFilesMountPath, ReadOnly: true}

	It("runs a native service's pre-deploy command as arguments to its entrypoint", func() {
		nn, _ := native("w350-predeploy", func(s *appv1alpha1.AppSpec) { s.PreDeployCommand = "node migrate.js" })
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: predeploy.JobName(nn.Name, "gen-1"), Namespace: nn.Namespace}, job)).To(Succeed())
		c := job.Spec.Template.Spec.Containers[0]
		Expect(c.Command).To(BeEmpty(), "the image's entrypoint, native-init, must stay")
		Expect(c.Args).To(Equal([]string{"/bin/sh", "-c", "node migrate.js"}))
		Expect(c.VolumeMounts).To(ContainElement(secretFiles))
	})

	It("runs a native cron job's own command as arguments to its entrypoint", func() {
		nn, _ := native("w350-cron", func(s *appv1alpha1.AppSpec) {
			s.Type, s.Schedule, s.Command = appv1alpha1.TypeCronJob, "0 * * * *", "node report.js"
		})
		cj := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.CronJobName(nn.Name), Namespace: nn.Namespace}, cj)).To(Succeed())
		c := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
		Expect(c.Command).To(BeEmpty(), "the image's entrypoint, native-init, must stay")
		Expect(c.Args).To(Equal(nativeStartArgs("node report.js")))
		Expect(c.VolumeMounts).To(ContainElement(secretFiles))
	})
})
