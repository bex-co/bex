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

// Package execution owns the common identity and pod-level defaults for every
// short-lived workload that executes tenant-controlled source or images.
package execution

import corev1 "k8s.io/api/core/v1"

const (
	LabelApp           = "app.bex.co/app"
	LabelAppUID        = "app.bex.co/app-uid"
	LabelComponent     = "app.bex.co/component"
	LabelWorkspace     = "app.bex.co/workspace"
	LabelAppNamespace  = "app.bex.co/app-namespace"
	LabelVerifyImage   = "app.bex.co/verify-image"
	VerifyImageEnabled = "enabled"
	NodePoolLabel      = "bex.co/pool"
	UntrustedNodePool  = "tenant"
)

// PodLabels returns the common labels that let logging, admission, and network
// policy select every execution variant consistently. Extra mechanism-specific
// labels can be added by the caller.
func PodLabels(app, appUID, component, workspace, appNamespace string, verifyImage bool) map[string]string {
	labels := map[string]string{
		LabelApp:       app,
		LabelComponent: component,
	}
	if appUID != "" {
		labels[LabelAppUID] = appUID
	}
	if workspace != "" {
		labels[LabelWorkspace] = workspace
	}
	if appNamespace != "" {
		labels[LabelAppNamespace] = appNamespace
	}
	if verifyImage {
		labels[LabelVerifyImage] = VerifyImageEnabled
	}
	return labels
}

// HardenPod applies the defaults shared by all untrusted execution pods. A
// mechanism may add a narrower container security context, but it must not
// restore the ambient Kubernetes API token or broaden node placement.
func HardenPod(spec *corev1.PodSpec) {
	spec.AutomountServiceAccountToken = ptr(false)
	spec.HostUsers = ptr(false)
	spec.NodeSelector = map[string]string{NodePoolLabel: UntrustedNodePool}
	if spec.SecurityContext == nil {
		spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if spec.SecurityContext.SeccompProfile == nil {
		spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	}
}

func ptr[T any](v T) *T { return &v }
