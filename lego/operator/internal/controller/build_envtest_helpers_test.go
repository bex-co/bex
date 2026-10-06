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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Shared build-Job fixtures for the envtest suites that drive a build to a
// terminal state (supersede, terminal-requeue, queued-behind-failure). envtest
// runs no kubelet, so a build only ever finishes because a test writes the Job
// status the operator observes — and that status has grammar the API server
// enforces, which is exactly the kind of knowledge that must not live in three
// hand-copied places.

// buildJobFor reads the build Job the operator dispatches for one App revision.
func buildJobFor(appName, rev string) (*batchv1.Job, error) {
	j := &batchv1.Job{}
	err := k8sClient.Get(context.Background(),
		client.ObjectKey{Namespace: "default", Name: build.JobName(appName, rev)}, j)
	return j, err
}

// failBuildJob marks a dispatched build Job Failed with the tenant-classified
// reason (exit 90 → PodFailurePolicy), the envtest stand-in for a real broken
// tenant build. That classification is what faultFromJob keys on.
func failBuildJob(appName, rev string) {
	GinkgoHelper()
	j, err := buildJobFor(appName, rev)
	Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
	failEnvtestJob(j, batchv1.JobReasonPodFailurePolicy, "container exit code 90")
}

// completeBuildJob marks a dispatched build Job Complete — the stand-in for a
// finished in-cluster build.
func completeBuildJob(appName, rev string) {
	GinkgoHelper()
	j, err := buildJobFor(appName, rev)
	Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
	completeEnvtestJob(j, metav1.Now())
}

// failEnvtestJob writes a Failed Job status. The API server's Job status
// grammar requires FailureTarget before Failed and no completionTime on a
// failed Job.
func failEnvtestJob(j *batchv1.Job, reason, message string) {
	GinkgoHelper()
	now := metav1.Now()
	j.Status.StartTime = &now
	j.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: reason, Message: message},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: reason, Message: message},
	}
	Expect(k8sClient.Status().Update(context.Background(), j)).To(Succeed())
}

// completeEnvtestJob writes a Complete Job status that finished at at. Its
// grammar mirrors failEnvtestJob's: SuccessCriteriaMet precedes Complete, and a
// completed Job carries a completionTime.
func completeEnvtestJob(j *batchv1.Job, at metav1.Time) {
	GinkgoHelper()
	j.Status.StartTime = &at
	j.Status.CompletionTime = &at
	j.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}
	Expect(k8sClient.Status().Update(context.Background(), j)).To(Succeed())
}

// envtestApp and envtestDeployment read an App and its Deployment by name.
func envtestApp(nn types.NamespacedName) *appv1alpha1.App {
	GinkgoHelper()
	app := &appv1alpha1.App{}
	Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
	return app
}

func envtestDeployment(nn types.NamespacedName) *appsv1.Deployment {
	GinkgoHelper()
	dep := &appsv1.Deployment{}
	Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
	return dep
}

// markEnvtestDeploymentReady writes the status a kubelet and the Deployment
// controller would: every desired replica updated and ready.
func markEnvtestDeploymentReady(nn types.NamespacedName) {
	GinkgoHelper()
	dep := envtestDeployment(nn)
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	dep.Status.ObservedGeneration = dep.Generation
	dep.Status.Replicas, dep.Status.UpdatedReplicas = replicas, replicas
	dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = replicas, replicas
	Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
}
