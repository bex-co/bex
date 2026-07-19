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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Release identity operational reconciliation", func() {
	const name = "release-identity-scale"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	build2NN := types.NamespacedName{Name: build.JobName(name, "gen-2"), Namespace: "default"}
	predeploy2NN := types.NamespacedName{Name: predeploy.JobName(name, "gen-2"), Namespace: "default"}

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			Registry: "zot.test:5000", BuildNamespace: "default",
		}
	})
	reconcile1 := func() (reconcile.Result, error) {
		return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
	}
	getApp := func() *appv1alpha1.App {
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		return app
	}
	getDeployment := func() *appsv1.Deployment {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		return dep
	}
	markReady := func(replicas int32) {
		dep := getDeployment()
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas = replicas
		dep.Status.UpdatedReplicas = replicas
		dep.Status.ReadyReplicas = replicas
		dep.Status.AvailableReplicas = replicas
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
	}

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build2NN.Name, Namespace: build2NN.Namespace}})
		_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: predeploy2NN.Name, Namespace: predeploy2NN.Namespace}})
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			_, _ = reconcile1()
		}
	})

	It("repairs the production failed-scale shape without source access, build, pre-deploy, or rollout", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/bex.git", RootDir: "examples/hello-go",
				Branch: "main", Runtime: "go", Builder: "native",
				BuildCommand: "go build -o app .", StartCommand: "./app",
				CloneSecret: "expired-clone-secret", PreDeployCommand: "echo migrate",
				Port: 3000, Replicas: 1,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		_, err := reconcile1() // finalizer
		Expect(err).NotTo(HaveOccurred())

		// Seed the completed generation-1 release. The named clone Secret is
		// deliberately absent: no operational reconciliation may try to read it.
		app = getApp()
		identity := desiredAppReleaseIdentity(app.Spec)
		app.Status = appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, Image: "zot.test:5000/" + name + ":gen-1",
			ArtifactImage:       "zot.test:5000/" + name + ":gen-1",
			ArtifactFingerprint: identity.artifact, ReleaseFingerprint: identity.release,
			ReleaseGeneration: 1, ActiveRevision: "rev-1", ObservedGeneration: 1,
			PreDeploy: &appv1alpha1.PreDeployStatus{
				Job: predeploy.JobName(name, "gen-1"), Generation: 1,
				Status: appv1alpha1.PreDeploySucceeded,
			},
		}
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		_, err = reconcile1()
		Expect(err).NotTo(HaveOccurred())
		markReady(1)
		_, err = reconcile1()
		Expect(err).NotTo(HaveOccurred())
		beforeTemplate := getDeployment().Spec.Template.DeepCopy()

		// Reproduce production: replicas advances metadata generation to 2, a
		// generation-2 build has already failed, while generation 1 still serves.
		app = getApp()
		app.Spec.Replicas = 2
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		app = getApp()
		app.Status.ArtifactFingerprint = ""
		app.Status.ReleaseFingerprint = ""
		app.Status.ReleaseGeneration = 0
		app.Status.Phase = appv1alpha1.PhaseFailed
		app.Status.ObservedGeneration = 1
		app.Status.Conditions = []metav1.Condition{{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "BuildFailed",
			Message: "Job has reached the specified backoff limit", ObservedGeneration: app.Generation,
			LastTransitionTime: metav1.Now(),
		}}
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		failedBuild := build.BuildJob(build.Options{
			Name: name, Revision: "gen-2", Registry: "zot.test:5000", Namespace: "default",
			Repo: app.Spec.Repo, Ref: "main", Builder: build.BuilderNative,
		}, "zot.test:5000/"+name+":gen-2")
		Expect(k8sClient.Create(ctx, failedBuild)).To(Succeed())
		now := metav1.Now()
		failedBuild.Status.StartTime = &now
		failedBuild.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"},
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"},
		}
		Expect(k8sClient.Status().Update(ctx, failedBuild)).To(Succeed())

		_, err = reconcile1()
		Expect(err).NotTo(HaveOccurred(), "scale must not observe/retry the failed generation-2 build")
		dep := getDeployment()
		Expect(*dep.Spec.Replicas).To(Equal(int32(2)))
		Expect(&dep.Spec.Template).To(Equal(beforeTemplate), "scale must preserve the pod template and ReplicaSet identity")
		Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelRevision, "rev-1"))
		err = k8sClient.Get(ctx, predeploy2NN, &batchv1.Job{})
		Expect(errors.IsNotFound(err)).To(BeTrue())

		markReady(2)
		_, err = reconcile1()
		Expect(err).NotTo(HaveOccurred())
		app = getApp()
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ObservedGeneration).To(Equal(app.Generation))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		Expect(app.Status.Image).To(Equal("zot.test:5000/" + name + ":gen-1"))
		ready := metaReadyCondition(app.Status.Conditions)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal("Deployed"))
	})
})

func metaReadyCondition(conditions []metav1.Condition) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == "Ready" {
			return &conditions[i]
		}
	}
	return nil
}
