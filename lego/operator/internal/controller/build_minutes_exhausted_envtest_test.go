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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m308 (docs/ADR030 §13): a workspace out of build minutes with no payment
// method starts no build this month; a build already running finishes.
var _ = Describe("Build minutes exhausted (w6/m308)", func() {
	ctx := context.Background()
	const ns = "tea-minutes"
	var buildNS string
	seq := 0
	BeforeEach(func() {
		seq++
		buildNS = fmt.Sprintf("build-minutes-%d", seq)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: buildNS}})).To(Succeed())
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
	})
	nn := func(name string) types.NamespacedName { return types.NamespacedName{Name: name, Namespace: ns} }
	r := func() *AppReconciler {
		return &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
	}
	create := func(name, month string) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: map[string]string{labelWorkspace: ns}},
			Spec:       appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile", Port: 3000},
		}
		if month != "" {
			app.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: month}
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })
	}
	reconcileN := func(rec *AppReconciler, name string, n int) {
		for range n {
			_, _ = rec.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name)})
		}
	}
	jobExists := func(name string) bool {
		a := envtestApp(nn(name))
		return k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: build.JobName(name, releaseBuildRevision(a))}, &batchv1.Job{}) == nil
	}
	thisMonth := time.Now().UTC().Format("2006-01")

	It("fails a new build with Build Minutes Exhausted and dispatches nothing", func() {
		create("out-of-minutes", thisMonth)
		reconcileN(r(), "out-of-minutes", 3)
		Expect(jobExists("out-of-minutes")).To(BeFalse(), "no build may start")
		a := envtestApp(nn("out-of-minutes"))
		Expect(a.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		ready := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready.Reason).To(Equal(appv1alpha1.ReasonBuildMinutesExhausted))
		Expect(ready.Message).To(HavePrefix("Build Minutes Exhausted"))
		Expect(appv1alpha1.IsBuildFailureReason(ready.Reason)).To(BeTrue(), "bex-api closes the deploy build_failed")
	})

	It("ignores a mark from an earlier month", func() {
		create("last-month", time.Now().UTC().AddDate(0, -1, 0).Format("2006-01"))
		reconcileN(r(), "last-month", 3)
		Expect(jobExists("last-month")).To(BeTrue(), "a new month brings new minutes")
	})

	It("starts no pre-deploy run, but lets one whose Job exists run on", func() {
		deploy := func(name, month string) types.NamespacedName {
			key := types.NamespacedName{Name: name, Namespace: "default"}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1, PreDeployCommand: "migrate"},
			}
			if month != "" {
				app.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: month}
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })
			return key
		}
		rec := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		pass := func(key types.NamespacedName) { _, _ = rec.Reconcile(ctx, reconcile.Request{NamespacedName: key}) }

		refused := deploy("predeploy-out", thisMonth)
		pass(refused)
		pass(refused)
		pd := envtestApp(refused).Status.PreDeploy
		Expect(pd).NotTo(BeNil())
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployFailed))
		Expect(pd.Message).To(HavePrefix("Build Minutes Exhausted"))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pd.Job, Namespace: "default"}, &batchv1.Job{})).NotTo(Succeed(),
			"no pre-deploy Job may start")

		// A Job already created for the release (its status write lost) keeps running.
		running := deploy("predeploy-running", "")
		for range 3 {
			pass(running)
		}
		Expect(envtestApp(running).Status.PreDeploy).NotTo(BeNil(), "precondition: the pre-deploy step started")
		a := envtestApp(running)
		a.Status.PreDeploy = nil
		Expect(k8sClient.Status().Update(ctx, a)).To(Succeed())
		a = envtestApp(running)
		a.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: thisMonth}
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
		pass(running)
		Expect(envtestApp(running).Status.PreDeploy.Status).NotTo(Equal(appv1alpha1.PreDeployFailed),
			"a pre-deploy step that already started is not refused")
	})

	It("lets a build that was already running finish", func() {
		create("running", "")
		rec := r()
		reconcileN(rec, "running", 3)
		Expect(jobExists("running")).To(BeTrue())
		a := envtestApp(nn("running"))
		a.Annotations = map[string]string{appv1alpha1.AnnotationBuildMinutesExhausted: thisMonth}
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
		reconcileN(rec, "running", 2)
		Expect(jobExists("running")).To(BeTrue(), "the running build is not stopped")
		ready := meta.FindStatusCondition(envtestApp(nn("running")).Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready.Reason).NotTo(Equal(appv1alpha1.ReasonBuildMinutesExhausted))
	})
})
