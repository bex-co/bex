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
	"bytes"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	sigsyaml "sigs.k8s.io/yaml"
)

func TestApplicationContainerPolicies(t *testing.T) {
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1, appv1alpha1.ContainerPolicyRenderV1} {
		t.Run(policy, func(t *testing.T) {
			app := projectionApp(func(a *appv1alpha1.App) { a.Spec.ContainerPolicy = policy })
			dep := project(app, webParams())
			cron := (&AppReconciler{}).cronPodSpec(app, "image", 8123, nil, launchUnknown)
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
				// Literal path, not the shared helper: render-v1 runs bex's
				// Localhost profile (w6/m150); every other policy RuntimeDefault.
				if policy == appv1alpha1.ContainerPolicyRenderV1 {
					if security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost ||
						security.SeccompProfile.LocalhostProfile == nil || *security.SeccompProfile.LocalhostProfile != "profiles/bex-render-v1.json" {
						t.Fatalf("render-v1 seccomp = %+v, want the bex Localhost profile", security.SeccompProfile)
					}
				} else if security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
					t.Fatal("seccomp disabled")
				}
				if !reflect.DeepEqual(security.Capabilities.Drop, []corev1.Capability{"ALL"}) {
					t.Fatal("default capabilities retained")
				}
				// Literal lists, not the shared table: a table edit must fail here.
				var want []corev1.Capability
				switch policy {
				case appv1alpha1.ContainerPolicyImageV1:
					want = []corev1.Capability{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID"}
				case appv1alpha1.ContainerPolicyRenderV1:
					// Render's measured set (ADR089 § Privilege slice decisions).
					want = []corev1.Capability{"CHOWN", "DAC_OVERRIDE", "FOWNER", "SETGID", "SETUID", "SYS_CHROOT"}
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

// Admission keys on the policy label, so it must carry each widened policy's
// own name, and strict templates must stay byte-identical (no label at all).
func TestAppPolicyLabelsNameTheWidenedPolicy(t *testing.T) {
	for policy, want := range map[string]string{
		"":                                  "",
		appv1alpha1.ContainerPolicyStrictV1: "",
		appv1alpha1.ContainerPolicyImageV1:  appv1alpha1.ContainerPolicyImageV1,
		appv1alpha1.ContainerPolicyRenderV1: appv1alpha1.ContainerPolicyRenderV1,
	} {
		// Paid, so the free-plan label (w8/m127) stays out of this policy's view.
		app := projectionApp(func(a *appv1alpha1.App) {
			a.Spec.ContainerPolicy = policy
			a.Spec.Tier = "starter"
		})
		app.UID = "uid-1"
		base := map[string]string{"k": "v"}
		got := appPolicyLabels(app, base)
		if want == "" {
			if !reflect.DeepEqual(got, base) {
				t.Errorf("%q: strict labels changed: %v", policy, got)
			}
			continue
		}
		if got[execution.LabelContainerPolicy] != want || got[execution.LabelAppUID] != "uid-1" || got[execution.LabelApp] != app.Name {
			t.Errorf("%q: labels = %v", policy, got)
		}
		if _, leaked := base[execution.LabelContainerPolicy]; leaked {
			t.Errorf("%q: the caller's map was mutated", policy)
		}
	}
}

// The workload admission policy cannot import the capability table, so its
// containerPolicyCaps map is pinned to it here: a policy that gains a
// capability in one place and not the other fails this test instead of every
// deploy (w4/m142's failure mode, w6/m149).
func TestAdmissionCapabilityListsMatchTheTable(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/gitops/base/operator-workload-admission.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var expression, seccomp string
	for doc := range bytes.SplitSeq(raw, []byte("\n---")) {
		var policy admissionregistrationv1.ValidatingAdmissionPolicy
		if err := sigsyaml.Unmarshal(doc, &policy); err != nil {
			t.Fatal(err)
		}
		if policy.Kind != "ValidatingAdmissionPolicy" || policy.Name != "bex-operator-workloads" {
			continue
		}
		for _, v := range policy.Spec.Variables {
			switch v.Name {
			case "containerPolicyCaps":
				expression = v.Expression
			case "containerPolicySeccomp":
				seccomp = v.Expression
			}
		}
	}
	if expression == "" {
		t.Fatal("bex-operator-workloads has no containerPolicyCaps variable")
	}
	profiles := map[string]string{}
	for _, entry := range regexp.MustCompile(`'([a-z0-9-]+)':\s*'([^']+)'`).FindAllStringSubmatch(seccomp, -1) {
		profiles[entry[1]] = entry[2]
	}
	admitted := map[string][]string{}
	for _, entry := range regexp.MustCompile(`'([a-z0-9-]+)':\s*\[([^\]]*)\]`).FindAllStringSubmatch(expression, -1) {
		var caps []string
		for _, c := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(entry[2], -1) {
			caps = append(caps, c[1])
		}
		admitted[entry[1]] = caps
	}
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1, appv1alpha1.ContainerPolicyRenderV1} {
		want := appv1alpha1.ContainerPolicyCapabilities(policy)
		if got := admitted[policy]; !reflect.DeepEqual(got, want) {
			t.Errorf("%q: admission admits %v, the projection adds %v", policy, got, want)
		}
		delete(admitted, policy)
	}
	if len(admitted) != 0 {
		t.Errorf("admission admits capabilities for policies the table does not know: %v", admitted)
	}
	for _, policy := range []string{"", appv1alpha1.ContainerPolicyStrictV1, appv1alpha1.ContainerPolicyImageV1, appv1alpha1.ContainerPolicyRenderV1} {
		if got, want := profiles[policy], appv1alpha1.ContainerPolicySeccompProfile(policy); got != want {
			t.Errorf("%q: admission admits seccomp profile %q, the projection uses %q", policy, got, want)
		}
		delete(profiles, policy)
	}
	if len(profiles) != 0 {
		t.Errorf("admission admits seccomp profiles for policies the table does not know: %v", profiles)
	}
}
