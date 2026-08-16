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
	LabelApp    = "app.bex.co/app"
	LabelAppUID = "app.bex.co/app-uid"
	// LabelDatabaseUID and LabelKeyValueUID are LabelAppUID's siblings for the
	// two other parent kinds that own cleanup Jobs. All three bind a Job to one
	// exact parent lifetime, so a recreated parent never adopts the old Job.
	LabelDatabaseUID   = "app.bex.co/database-uid"
	LabelKeyValueUID   = "app.bex.co/keyvalue-uid"
	LabelComponent     = "app.bex.co/component"
	LabelWorkspace     = "app.bex.co/workspace"
	LabelAppNamespace  = "app.bex.co/app-namespace"
	LabelVerifyImage   = "app.bex.co/verify-image"
	VerifyImageEnabled = "enabled"
	NodePoolLabel      = "bex.co/pool"
	UntrustedNodePool  = "tenant"

	// BuildPoolTaintKey/Value name the taint the dedicated build node pools
	// (bex-tenant-lg, bex-tenant-burst) register with at bootstrap
	// (docs/ADR060 § dedicated build pool). Only build pods tolerate it:
	// serving, pre-deploy, and publish pods must keep being repelled so a
	// long-lived workload can never land on — and pin — an elastic build node
	// (observed 2026-08: single-instance CNPG PDBs kept bex-tenant-burst
	// undrainable for 6 days).
	BuildPoolTaintKey   = "bex.co/build-only"
	BuildPoolTaintValue = "true"

	// LabelProtectedFromTenantMount marks an operator-projected Secret that a
	// tenant workload must never mount (codex F1) — e.g. the shared S3 backup
	// credential the Database/KeyValue reconcilers copy into a datastore's own
	// namespace. Kubelet secret projection needs no pod-side API token, so pod
	// hardening alone does not stop a co-located tenant App from naming such a
	// Secret in EnvFrom/volume/env; the App reconcile rejects any reference to a
	// Secret carrying this label.
	LabelProtectedFromTenantMount = "app.bex.co/protected-from-tenant-mount"
	ProtectedFromTenantMount      = "true"
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
	spec.HostUsers = ptr(false) // pod user namespace; requires containerd 2.x + UserNamespacesSupport
	spec.NodeSelector = map[string]string{NodePoolLabel: UntrustedNodePool}
	if spec.SecurityContext == nil {
		spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if spec.SecurityContext.SeccompProfile == nil {
		spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	}
}

// TolerateBuildPool lets a build pod schedule onto the tainted dedicated
// build node pools in addition to the untainted tenant pool. It widens
// eligibility within the untrusted `bex.co/pool=tenant` selector set by
// HardenPod, never beyond it.
func TolerateBuildPool(spec *corev1.PodSpec) {
	spec.Tolerations = append(spec.Tolerations, corev1.Toleration{
		Key:      BuildPoolTaintKey,
		Operator: corev1.TolerationOpEqual,
		Value:    BuildPoolTaintValue,
		Effect:   corev1.TaintEffectNoSchedule,
	})
}

func ptr[T any](v T) *T { return &v }
