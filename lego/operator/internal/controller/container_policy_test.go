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
	"reflect"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplicationContainerPolicies(t *testing.T) {
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1} {
		t.Run(policy, func(t *testing.T) {
			app := projectionApp(func(a *appv1alpha1.App) { a.Spec.ContainerPolicy = policy })
			dep := project(app, webParams())
			cron := (&AppReconciler{}).cronPodSpec(app, "image", 8123, nil)
			app.Spec.PreDeployCommand = "true"
			app.UID = "policy-test"
			scheme := datastoreSecretScheme(t)
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: scheme}
			if _, _, err := r.reconcilePreDeploy(t.Context(), app, "image", 8123); err != nil {
				t.Fatal(err)
			}
			var jobs batchv1.JobList
			if err := cl.List(t.Context(), &jobs); err != nil || len(jobs.Items) != 1 {
				t.Fatalf("pre-deploy jobs = %d, error = %v", len(jobs.Items), err)
			}
			for _, pod := range []corev1.PodSpec{dep.Spec.Template.Spec, cron.Spec, jobs.Items[0].Spec.Template.Spec} {
				if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
					t.Fatal("service account token exposed")
				}
				if pod.HostNetwork || pod.HostPID || pod.HostIPC {
					t.Fatal("host namespace exposed")
				}
				security := pod.Containers[0].SecurityContext
				if security.AllowPrivilegeEscalation == nil || *security.AllowPrivilegeEscalation {
					t.Fatal("privilege escalation enabled")
				}
				if security.Privileged != nil && *security.Privileged {
					t.Fatal("privileged container")
				}
				if security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
					t.Fatal("seccomp disabled")
				}
				if !reflect.DeepEqual(security.Capabilities.Drop, []corev1.Capability{"ALL"}) {
					t.Fatal("default capabilities retained")
				}
				var want []corev1.Capability
				if policy == appv1alpha1.ContainerPolicyImageV1 {
					want = []corev1.Capability{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID"}
				}
				if !reflect.DeepEqual(security.Capabilities.Add, want) {
					t.Fatalf("capabilities = %v, want %v", security.Capabilities.Add, want)
				}
			}
		})
	}
	// Platform helpers must not inherit application-image compatibility.
	if len(tenantSecCtx().Capabilities.Add) != 0 {
		t.Fatal("platform security policy widened")
	}
}
