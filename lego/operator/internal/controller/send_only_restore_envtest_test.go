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
	"encoding/json"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A deploy canceled over a served release restores that release's recorded
// template on every pass. A record from before w6/m640 has no send-only label,
// so on production a free web service whose last deploy was canceled woke
// without it (2026-11-26). The restored template takes the labels of the plan
// it runs, and a background worker keeps refusing private traffic.
var _ = Describe("A canceled deploy restores a served template from before the send-only label (w6/m640)", func() {
	const ws = "tea-m640m640m640m640m640"
	var (
		name string // each spec's own: a deleted App's finalizer outlives its spec
		nn   types.NamespacedName
		r    *AppReconciler
	)
	BeforeEach(func() {
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ws}})
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		}
	})

	pass := func() {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	getApp := func() *appv1alpha1.App {
		GinkgoHelper()
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		return app
	}
	// stripRecord rewrites generation 1's record as the operator before w6/m640
	// recorded it, without the given labels.
	stripRecord := func(keys ...string) {
		GinkgoHelper()
		rec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.ReleaseRecordName(name, 1), Namespace: ws}, rec)).To(Succeed())
		var recorded corev1.PodTemplateSpec
		Expect(json.Unmarshal(rec.Data[appv1alpha1.ReleaseRecordPodTemplateKey], &recorded)).To(Succeed())
		for _, key := range keys {
			Expect(recorded.Labels).To(HaveKey(key), "precondition: generation 1 recorded %s", key)
			delete(recorded.Labels, key)
		}
		raw, err := json.Marshal(recorded)
		Expect(err).NotTo(HaveOccurred())
		rec.Data[appv1alpha1.ReleaseRecordPodTemplateKey] = raw
		Expect(k8sClient.Update(ctx, rec)).To(Succeed())
	}
	// cancelAChange saves a change (mutate) and cancels its deploy.
	cancelAChange := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		app = getApp()
		app.Annotations = map[string]string{
			appv1alpha1.AnnotationCanceledReleaseGeneration: strconv.FormatInt(app.Generation, 10),
		}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		for range 3 {
			pass()
		}
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(1)), "the served release is restored")
	}
	getDep := func() *appsv1.Deployment {
		GinkgoHelper()
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		return dep
	}

	It("gives a background worker's restored template the send-only label", func() {
		name = "send-only-restore-worker"
		nn = types.NamespacedName{Name: name, Namespace: ws}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeBackgroundWorker, Tier: "starter", Image: "nginx:1", Replicas: 1,
			},
		})).To(Succeed())
		for range 3 {
			pass()
		}
		dep := getDep()
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas = 1, 1
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		pass()
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")
		Expect(sendOnly(getDep().Spec.Template.Labels)).To(BeTrue(), "precondition: the worker sends only")

		stripRecord(execution.LabelPrivateNetwork)
		cancelAChange(func(app *appv1alpha1.App) { app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano) })
		Expect(sendOnly(getDep().Spec.Template.Labels)).To(BeTrue(),
			"a restored template must not open the worker to private traffic: %v", getDep().Spec.Template.Labels)
	})

	It("gives a cron job's restored template its workspace and send-only labels", func() {
		name = "send-only-restore-cron"
		nn = types.NamespacedName{Name: name, Namespace: ws}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeCronJob, Tier: "starter", Image: "nginx:1", Schedule: "0 3 * * *", Command: "echo served",
			},
		})).To(Succeed())
		for range 3 {
			pass()
		}
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(1)), "precondition: generation 1 served")
		stripRecord(labelWorkspace, execution.LabelPrivateNetwork)
		cancelAChange(func(app *appv1alpha1.App) { app.Spec.Command = "echo canceled" })

		cron := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.CronJobName(name), Namespace: ws}, cron)).To(Succeed())
		tpl := cron.Spec.JobTemplate.Spec.Template
		Expect(tpl.Spec.Containers[0].Command).NotTo(ContainElement(ContainSubstring("canceled")), "precondition: the served template is restored")
		Expect(tpl.Labels).To(HaveKeyWithValue(labelWorkspace, ws))
		Expect(sendOnly(tpl.Labels)).To(BeTrue(), "a restored cron template: %v", tpl.Labels)
	})
})
