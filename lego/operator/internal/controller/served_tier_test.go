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
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func templateRunning(tier string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{
		{Name: "sidecar", Resources: resourcesForTier("pro")},
		{Name: appContainerName, Resources: resourcesForTier(tier)},
	}}}
}

// A restored template names its plan only through its app container's CPU and
// memory, so no two compute plans may share both (w6/m609).
func TestComputePlansDifferInCPUOrMemory(t *testing.T) {
	for _, id := range tiers.Compute.IDs() {
		if got, ok := templateTier(templateRunning(id)); !ok || got != id {
			t.Errorf("a template running %q reads as %q (%v)", id, got, ok)
		}
	}
}

// The pass records the plan of the template it writes: the projected spec's,
// or a restored template's own (w6/m609).
func TestRecordServedTier(t *testing.T) {
	app := &appv1alpha1.App{}
	recordServedTier(app, "standard", nil)
	if app.Status.ServedTier != "standard" {
		t.Fatalf("projected: servedTier %q, want standard", app.Status.ServedTier)
	}
	restored := templateRunning("starter")
	recordServedTier(app, "standard", &restored)
	if app.Status.ServedTier != "starter" {
		t.Fatalf("restored: servedTier %q, want the restored template's starter", app.Status.ServedTier)
	}
	unknown := corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName}}}}
	recordServedTier(app, "standard", &unknown)
	if app.Status.ServedTier != "starter" {
		t.Fatalf("a restored template no plan matches moved servedTier to %q", app.Status.ServedTier)
	}
}
