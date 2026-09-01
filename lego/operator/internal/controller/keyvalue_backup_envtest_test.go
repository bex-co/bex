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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("KeyValue backup reconciliation events", func() {
	BeforeEach(func() {
		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		reconciler := &KeyValueReconciler{
			Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Backup: testKeyValueBackupStore,
		}
		Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})
	})

	It("gates CronJobs by plan and completes the delete finalizer after purge", func() {
		freeNN := types.NamespacedName{Name: "backup-event-free", Namespace: "default"}
		paidNN := types.NamespacedName{Name: "backup-event-paid", Namespace: "default"}
		free := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: freeNN.Name, Namespace: freeNN.Namespace},
			Spec:       appv1alpha1.KeyValueSpec{Name: "backup-event-free", Plan: "free"},
		}
		paid := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: paidNN.Name, Namespace: paidNN.Namespace},
			Spec:       appv1alpha1.KeyValueSpec{Name: "backup-event-paid", Plan: "starter"},
		}
		Expect(k8sClient.Create(ctx, free)).To(Succeed())
		Expect(k8sClient.Create(ctx, paid)).To(Succeed())
		DeferCleanup(func() {
			for _, nn := range []types.NamespacedName{freeNN, paidNN} {
				_ = k8sClient.Delete(context.Background(), &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace}})
				_ = k8sClient.Delete(context.Background(), &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: keyValueBackupName(nn.Name), Namespace: nn.Namespace}})
			}
		})

		Eventually(func(g Gomega) {
			current := &appv1alpha1.KeyValue{}
			g.Expect(k8sClient.Get(ctx, paidNN, current)).To(Succeed())
			g.Expect(current.Finalizers).To(ContainElement(kvFinalizer))
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: keyValueBackupName(paidNN.Name), Namespace: paidNN.Namespace}, &batchv1.CronJob{})).To(Succeed())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		Consistently(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: keyValueBackupName(freeNN.Name), Namespace: freeNN.Namespace}, &batchv1.CronJob{})
			return apierrors.IsNotFound(err)
		}, time.Second, 100*time.Millisecond).Should(BeTrue())

		Expect(k8sClient.Delete(ctx, paid)).To(Succeed())
		var purge batchv1.Job
		Eventually(func(g Gomega) {
			var jobs batchv1.JobList
			g.Expect(k8sClient.List(ctx, &jobs, client.InNamespace(paidNN.Namespace), client.MatchingLabels{
				labelKeyValue: paidNN.Name, execution.LabelComponent: keyValueBackupPurgeComponent,
			})).To(Succeed())
			g.Expect(jobs.Items).To(HaveLen(1))
			purge = jobs.Items[0]
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		finishedAt := metav1.Now()
		purge.Status.StartTime = &finishedAt
		purge.Status.CompletionTime = &finishedAt
		purge.Status.Succeeded = 1
		purge.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue, LastTransitionTime: finishedAt},
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: finishedAt},
		}
		Expect(k8sClient.Status().Update(ctx, &purge)).To(Succeed())

		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, paidNN, &appv1alpha1.KeyValue{}))
		}, 15*time.Second, 100*time.Millisecond).Should(BeTrue())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(&purge), &batchv1.Job{}))
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())
	})

	It("preserves the backup prefix when the migration annotation is set", func() {
		nn := types.NamespacedName{Name: "backup-event-preserved", Namespace: "default"}
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{
				Name: nn.Name, Namespace: nn.Namespace,
				Annotations: map[string]string{appv1alpha1.AnnotationPreserveKeyValueBackups: "true"},
			},
			Spec: appv1alpha1.KeyValueSpec{Name: "backup-event-preserved", Plan: "starter"},
		}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(context.Background(), &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace}})
			_ = k8sClient.Delete(context.Background(), &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: keyValueBackupName(nn.Name), Namespace: nn.Namespace}})
		})

		Eventually(func(g Gomega) {
			current := &appv1alpha1.KeyValue{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Finalizers).To(ContainElement(kvFinalizer))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(ctx, &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace}})).To(Succeed())

		// The CR must finalize without the purge Job the unannotated path requires.
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, nn, &appv1alpha1.KeyValue{}))
		}, 15*time.Second, 100*time.Millisecond).Should(BeTrue())
		var jobs batchv1.JobList
		Expect(k8sClient.List(ctx, &jobs, client.InNamespace(nn.Namespace), client.MatchingLabels{
			labelKeyValue: nn.Name, execution.LabelComponent: keyValueBackupPurgeComponent,
		})).To(Succeed())
		Expect(jobs.Items).To(BeEmpty(), "a preserve-annotated delete must never create a purge Job")
	})
})
