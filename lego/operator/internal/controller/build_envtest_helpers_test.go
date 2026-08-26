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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/build"
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
// tenant build. That classification is what faultFromJob keys on. The API
// server's Job status grammar requires FailureTarget before Failed and no
// completionTime on a failed Job.
func failBuildJob(appName, rev string) {
	GinkgoHelper()
	j, err := buildJobFor(appName, rev)
	Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
	now := metav1.Now()
	j.Status.StartTime = &now
	j.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue,
			Reason: batchv1.JobReasonPodFailurePolicy, Message: "container exit code 90"},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
			Reason: batchv1.JobReasonPodFailurePolicy, Message: "container exit code 90"},
	}
	Expect(k8sClient.Status().Update(context.Background(), j)).To(Succeed())
}

// completeBuildJob marks a dispatched build Job Complete — the stand-in for a
// finished in-cluster build. Its grammar mirrors failBuildJob's:
// SuccessCriteriaMet precedes Complete, and a completed Job carries a
// completionTime.
func completeBuildJob(appName, rev string) {
	GinkgoHelper()
	j, err := buildJobFor(appName, rev)
	Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
	now := metav1.Now()
	j.Status.StartTime = &now
	j.Status.CompletionTime = &now
	j.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
		{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
	}
	Expect(k8sClient.Status().Update(context.Background(), j)).To(Succeed())
}
