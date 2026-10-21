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
	"strings"

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

// appHostUsers is a runtime pod's hostUsers: false (a pod user namespace) for
// render-v1 when BEX_RENDER_V1_USER_NAMESPACES is on (w6/m151), nil otherwise,
// for the Deployment and cron/manual-run paths (pre-deploy pods already run in
// one via execution.HardenPod),
// so every other template stays byte-identical. On-disk UIDs are unshifted
// under an idmapped volume, so turning it off needs no data migration (t002).
func appHostUsers(spec appv1alpha1.AppSpec, enabled bool) *bool {
	if !enabled || spec.ContainerPolicy != appv1alpha1.ContainerPolicyRenderV1 {
		return nil
	}
	return new(false)
}

// userNamespacesFor reports whether BEX_RENDER_V1_USER_NAMESPACES covers the
// App's workspace. A tenant App's namespace is its workspace id (ADR043), so a
// list scopes the switch to a canary workspace before "*" widens it.
func (r *AppReconciler) userNamespacesFor(app *appv1alpha1.App) bool {
	return r.RenderV1UserNamespaces["*"] || r.RenderV1UserNamespaces[app.Namespace]
}

// ParseWorkspaceSet reads a comma-separated workspace list; "on" and "*" mean
// every workspace, and blanks are ignored. Nil for an empty value.
func ParseWorkspaceSet(value string) map[string]bool {
	var set map[string]bool
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if item == "on" {
			item = "*"
		}
		if set == nil {
			set = map[string]bool{}
		}
		set[item] = true
	}
	return set
}
