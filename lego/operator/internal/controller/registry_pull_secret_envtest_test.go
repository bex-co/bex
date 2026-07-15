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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Tenant registry imagePullSecrets (w6/m29)", func() {
	const (
		namespace  = "default"
		registry   = "zot.test:5000"
		pullSecret = "bex-registry-pull"
	)
	ctx := context.Background()

	newReconciler := func(secret string) *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			Registry: registry, RegistryPullSecret: secret,
		}
	}
	reconcileN := func(r *AppReconciler, nn types.NamespacedName, count int) {
		for range count {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	cleanupApp := func(r *AppReconciler, name string) {
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		app := &appv1alpha1.App{}
		if k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			configuredRegistry := r.Registry
			r.Registry = "" // teardown must not contact the test-only registry host
			reconcileN(r, nn, 1)
			r.Registry = configuredRegistry
		}
		// envtest has no garbage-collection controller.
		_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
		_ = k8sClient.Delete(ctx, &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
	}
	secretNames := func(refs []corev1.LocalObjectReference) []string {
		out := make([]string, len(refs))
		for i := range refs {
			out[i] = refs[i].Name
		}
		return out
	}

	It("attaches the secret to generated Deployments, CronJobs, and manual Jobs, and omits it when unset", func() {
		for _, tc := range []struct {
			name        string
			secret      string
			want        []string
			serviceType string
		}{
			{name: "pull-deploy-set", secret: pullSecret, want: []string{pullSecret}},
			{name: "pull-deploy-unset", want: []string{}},
			{name: "pull-cron-set", secret: pullSecret, want: []string{pullSecret}, serviceType: appv1alpha1.TypeCronJob},
			{name: "pull-cron-unset", want: []string{}, serviceType: appv1alpha1.TypeCronJob},
		} {
			r := newReconciler(tc.secret)
			nn := types.NamespacedName{Name: tc.name, Namespace: namespace}
			spec := appv1alpha1.AppSpec{Image: registry + "/" + tc.name + ":gen-1", Port: 3000, Type: tc.serviceType}
			if tc.serviceType == appv1alpha1.TypeCronJob {
				spec.Schedule = "*/5 * * * *"
				spec.RunAt = "2026-07-15T12:00:00Z"
			}
			Expect(k8sClient.Create(ctx, &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace}, Spec: spec,
			})).To(Succeed())
			reconcileN(r, nn, 2) // finalizer, then workload

			if tc.serviceType == appv1alpha1.TypeCronJob {
				cron := &batchv1.CronJob{}
				Expect(k8sClient.Get(ctx, nn, cron)).To(Succeed())
				Expect(secretNames(cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets)).To(Equal(tc.want))

				jobName := manualRunJobName(tc.name, spec.RunAt)
				job := &batchv1.Job{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, job)).To(Succeed())
				Expect(secretNames(job.Spec.Template.Spec.ImagePullSecrets)).To(Equal(tc.want))
				_ = k8sClient.Delete(ctx, job)
			} else {
				dep := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
				Expect(secretNames(dep.Spec.Template.Spec.ImagePullSecrets)).To(Equal(tc.want))
			}
			cleanupApp(r, tc.name)
		}
	})

	It("attaches the secret to generated pre-deploy Jobs and omits it when unset", func() {
		for _, tc := range []struct {
			name   string
			secret string
			want   []string
		}{
			{name: "pull-predeploy-set", secret: pullSecret, want: []string{pullSecret}},
			{name: "pull-predeploy-unset", want: []string{}},
		} {
			r := newReconciler(tc.secret)
			nn := types.NamespacedName{Name: tc.name, Namespace: namespace}
			Expect(k8sClient.Create(ctx, &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace},
				Spec: appv1alpha1.AppSpec{
					Image: registry + "/" + tc.name + ":gen-1", Port: 3000,
					PreDeployCommand: "echo migrate",
				},
			})).To(Succeed())
			reconcileN(r, nn, 2) // finalizer, then pre-deploy gate

			jobName := predeploy.JobName(tc.name, "gen-1")
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: namespace}, job)).To(Succeed())
			Expect(secretNames(job.Spec.Template.Spec.ImagePullSecrets)).To(Equal(tc.want))

			cleanupApp(r, tc.name)
			_ = k8sClient.Delete(ctx, job)
		}
	})

	It("backfills old App-owned workloads before a later build can fail", func() {
		const name = "pull-backfill"
		r := newReconciler(pullSecret)
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/repo"},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())

		image := registry + "/" + name + ":gen-1"
		dep := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: image}}},
				},
			},
		}
		cron := &batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: batchv1.CronJobSpec{
				Schedule: "*/5 * * * *",
				JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "app", Image: image}}},
				}}},
			},
		}
		Expect(controllerutil.SetControllerReference(app, dep, k8sClient.Scheme())).To(Succeed())
		Expect(controllerutil.SetControllerReference(app, cron, k8sClient.Scheme())).To(Succeed())
		Expect(k8sClient.Create(ctx, dep)).To(Succeed())
		Expect(k8sClient.Create(ctx, cron)).To(Succeed())

		Expect(r.backfillWorkloadPullSecrets(ctx, app)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, cron)).To(Succeed())
		Expect(secretNames(dep.Spec.Template.Spec.ImagePullSecrets)).To(Equal([]string{pullSecret}))
		Expect(secretNames(cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets)).To(Equal([]string{pullSecret}))

		depVersion, cronVersion := dep.ResourceVersion, cron.ResourceVersion
		Expect(r.backfillWorkloadPullSecrets(ctx, app)).To(Succeed(), "a converged backfill must be a no-op")
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, cron)).To(Succeed())
		Expect(dep.ResourceVersion).To(Equal(depVersion))
		Expect(cron.ResourceVersion).To(Equal(cronVersion))

		cleanupApp(r, name)
	})
})
