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
	"maps"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// freePlanLabelValue is the App's execution.LabelFreePlan value: set on a
// free-plan App (isFreeApp: an empty tier is the free plan too), empty on a
// paid one (w8/m127).
func freePlanLabelValue(app *appv1alpha1.App) string {
	if isFreeApp(app) {
		return execution.FreePlanValue
	}
	return ""
}

// appPolicyLabels projects only persisted policy, never an App's arbitrary
// labels: the widened container policy's admission marker, and the free-plan
// label the SMTP egress deny selects (w8/m127). Keep strict paid templates
// byte-identical to avoid rolling legacy Apps.
func appPolicyLabels(app *appv1alpha1.App, labels map[string]string) map[string]string {
	widened := len(appv1alpha1.ContainerPolicyCapabilities(app.Spec.ContainerPolicy)) > 0
	free := freePlanLabelValue(app)
	if !widened && free == "" {
		return labels
	}
	labels = maps.Clone(labels)
	if labels == nil {
		labels = make(map[string]string)
	}
	if widened {
		labels[execution.LabelContainerPolicy] = app.Spec.ContainerPolicy
		labels[execution.LabelApp] = app.Name
		labels[execution.LabelAppUID] = string(app.UID)
	}
	if free != "" {
		labels[execution.LabelFreePlan] = free
	}
	return labels
}

// appSecCtx applies a service's durable policy to every path executing its
// image: Deployment, CronJob, one-off cron run and pre-deploy Job. Platform
// containers and managed datastores continue to use tenantSecCtx directly.
func appSecCtx(spec appv1alpha1.AppSpec) *corev1.SecurityContext {
	security := tenantSecCtx()
	for _, capability := range appv1alpha1.ContainerPolicyCapabilities(spec.ContainerPolicy) {
		security.Capabilities.Add = append(security.Capabilities.Add, corev1.Capability(capability))
	}
	if profile := appv1alpha1.ContainerPolicySeccompProfile(spec.ContainerPolicy); profile != "" {
		security.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeLocalhost, LocalhostProfile: &profile}
	}
	return security
}
