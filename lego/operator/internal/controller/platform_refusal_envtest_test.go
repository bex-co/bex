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
	"errors"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/090 against a real API server: a ValidatingAdmissionPolicy refusing the
// operator's Deployment write, the 2026-11-12 shape, reads on the App's Ready
// condition as the platform-error sentence; the policy's text is only in the
// operator log, under the sentence's reference.
var _ = Describe("an admission policy refusing the operator's Deployment write (w9/090)", func() {
	const (
		ns         = "tea-platform-refusal"
		policyName = "test-refuse-tenant-deployments"
		denial     = "operator-authored containers outside the execution boundary must be restricted"
	)

	refusesDeployments := func() error {
		probe := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "probe", Namespace: ns},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "probe"}},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "probe"}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "probe", Image: "nginx:1"}}},
				},
			},
		}
		err := k8sClient.Create(ctx, probe, client.DryRunAll)
		if err == nil {
			return errors.New("the policy does not refuse Deployments yet")
		}
		if !apierrors.IsForbidden(err) {
			return err
		}
		return nil
	}

	BeforeEach(func() {
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		scope := &metav1.LabelSelector{MatchLabels: map[string]string{corev1.LabelMetadataName: ns}}
		policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: policyName},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
				FailurePolicy: new(admissionregistrationv1.Fail),
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: scope,
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"},
							},
						},
					}},
				},
				Validations: []admissionregistrationv1.Validation{{
					Expression: "false", Message: denial, Reason: new(metav1.StatusReasonForbidden),
				}},
			},
		}
		binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{Name: policyName},
			Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
				PolicyName:        policyName,
				ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
			},
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, policy) })
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, binding) })
		// The API server enforces a new policy once its informer has synced it.
		Eventually(refusesDeployments, "60s", "250ms").Should(Succeed())
	})

	It("records the platform-error sentence and logs the raw refusal under its reference", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: ns, Labels: map[string]string{labelWorkspace: ns}},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeWebService, Image: "nginx:1", Port: 3000, Replicas: 1, Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })
		nn := types.NamespacedName{Name: app.Name, Namespace: ns}
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, BaseDomain: "onbex.co"}

		var log capturedLog
		var refused error
		Eventually(func(g Gomega) {
			_, refused = r.Reconcile(log.into(ctx), reconcile.Request{NamespacedName: nn})
			ready := meta.FindStatusCondition(envtestApp(nn).Status.Conditions, appv1alpha1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal("DeployFailed"))
		}, "30s", "250ms").Should(Succeed())
		Expect(apierrors.IsForbidden(refused)).To(BeTrue(), "the pass returns the raw refusal, so it requeues: %v", refused)

		live := envtestApp(nn)
		Expect(live.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		message := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionReady).Message
		code, params, ok := appv1alpha1.ParseDeployReason(message)
		Expect(ok).To(BeTrue(), message)
		Expect(code).To(Equal(appv1alpha1.DeployReasonPlatformError))
		Expect(params).To(HaveLen(1))
		ref := params[0].Value
		Expect(ref).To(MatchRegexp(`^g\d+-[0-9a-f]{8}$`))
		for _, internal := range []string{"forbidden", "ValidatingAdmissionPolicy", policyName, denial, ns} {
			Expect(message).NotTo(ContainSubstring(internal))
		}

		logged := log.String()
		Expect(logged).To(ContainSubstring(ref))
		Expect(logged).To(ContainSubstring("ValidatingAdmissionPolicy '" + policyName + "'"))
		Expect(logged).To(ContainSubstring(denial))
	})
})
