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
	"encoding/json"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m129 end to end through the reconciler: Render's django example declares
// DATABASE_URL fromDatabase and runs `manage.py migrate` in its build command.
// The build waits for the connection Secret, then receives the value through
// its env Secret mount only, gets datastore reach into its own workspace while
// it runs, and loses it when it ends.
var _ = Describe("Build reach to the workspace's own datastores (w8/m129)", func() {
	ctx := context.Background()
	const (
		name    = "srv-m129-django"
		ws      = "tea-m129m129m129m129m129"
		buildNS = "m129-builds"
		secret  = "dpg-m129-app"
		uri     = "postgresql://mysite:m129-pw-literal@dpg-m129-rw.tea-m129m129m129m129m129.svc:5432/mysite"
	)
	nn := types.NamespacedName{Name: name, Namespace: ws}

	It("waits for the connection Secret, mounts DATABASE_URL, and scopes reach to the build", func() {
		for _, ns := range []string{ws, buildNS} {
			_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/render-examples/django", Branch: "main",
				Builder: "dockerfile", Port: 3000,
				Env: []appv1alpha1.EnvVar{{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{
					SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: secret, Key: "uri"}}}},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
		conn := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: ws},
			StringData: map[string]string{"uri": uri}}
		DeferCleanup(func() {
			a := &appv1alpha1.App{}
			if k8sClient.Get(ctx, nn, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r.Registry = ""
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}
			_ = k8sClient.Delete(ctx, conn)
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: buildNS}})
		})
		reconcileN := func(n int) {
			for range n {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		jobKey := client.ObjectKey{Namespace: buildNS, Name: build.JobName(name, "gen-1")}
		egressKey := client.ObjectKey{Namespace: buildNS, Name: buildEgressPolicyName(name)}
		ingressKey := client.ObjectKey{Namespace: ws, Name: buildIngressPolicyName(name)}

		By("holding the build while the database's connection Secret does not exist yet")
		reconcileN(3)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, jobKey, &batchv1.Job{}))).To(BeTrue(),
			"no build may start without DATABASE_URL — its migrate would hit localhost")
		held := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, held)).To(Succeed())
		Expect(held.Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))
		ready := meta.FindStatusCondition(held.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(reasonBuildQueued))
		Expect(ready.Message).NotTo(ContainSubstring(secret), "the wait message names no internal Secret")

		By("dispatching once CNPG has written the Secret")
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())
		reconcileN(2)
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
		raw, err := json.Marshal(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("m129-pw-literal"), "the connection string must never be a Job-spec literal")
		var mounted bool
		for _, v := range job.Spec.Template.Spec.Volumes {
			mounted = mounted || (v.Secret != nil && v.Secret.SecretName == nativeEnvSecretName(name))
		}
		Expect(mounted).To(BeTrue(), "the build mounts the merged env Secret")
		merged := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: nativeEnvSecretName(name)}, merged)).To(Succeed())
		Expect(string(merged.Data["DATABASE_URL"])).To(Equal(uri))

		By("granting reach only between this build's pods and its own workspace namespace")
		egress, ingress := &networkingv1.NetworkPolicy{}, &networkingv1.NetworkPolicy{}
		Expect(k8sClient.Get(ctx, egressKey, egress)).To(Succeed())
		Expect(k8sClient.Get(ctx, ingressKey, ingress)).To(Succeed())
		Expect(job.Spec.Template.Labels).To(Equal(mergeLabels(job.Spec.Template.Labels, egress.Spec.PodSelector.MatchLabels)),
			"the egress policy must select the build Job's own pods")
		Expect(egress.Spec.Egress).To(HaveLen(1))
		Expect(egress.Spec.Egress[0].To).To(HaveLen(1))
		Expect(egress.Spec.Egress[0].To[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{corev1.LabelMetadataName: ws}))
		Expect(ingress.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{corev1.LabelMetadataName: buildNS}))
		Expect(ingress.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(Equal(egress.Spec.PodSelector.MatchLabels))

		By("revoking both halves when the build ends")
		completeEnvtestJob(job, metav1.Now())
		reconcileN(2)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, egressKey, &networkingv1.NetworkPolicy{}))).To(BeTrue())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, ingressKey, &networkingv1.NetworkPolicy{}))).To(BeTrue())
	})
})

// mergeLabels returns base with extra laid over it; equal to base exactly when
// extra is a subset of base.
func mergeLabels(base, extra map[string]string) map[string]string {
	out := maps.Clone(base)
	maps.Copy(out, extra)
	return out
}
