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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AppSpec is the desired state of a deploy-from-git App — the Render-like
// unit from strategy 211.09. Mirrors the Node MVP's service spec (src/api.js).
type AppSpec struct {
	// Repo is the git repository URL (or local path) to deploy from. Either Repo
	// (build-from-git) or Image (prebuilt) must be set.
	// +optional
	Repo string `json:"repo,omitempty"`

	// Image is a prebuilt OCI image to run directly, skipping the build plane.
	// +optional
	Image string `json:"image,omitempty"`

	// Branch to track. Defaults to "main".
	// +optional
	// +kubebuilder:default=main
	Branch string `json:"branch,omitempty"`

	// Builder selects how the image is built:
	// "auto" (Dockerfile if present, else Cloud Native Buildpacks), "buildpack", or "dockerfile".
	// +optional
	// +kubebuilder:validation:Enum=auto;buildpack;dockerfile
	// +kubebuilder:default=auto
	Builder string `json:"builder,omitempty"`

	// Replicas for the kubernetes runtime (pods bin-packed across machines/nodes).
	// +optional
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// Port the application listens on (PORT is injected).
	// +optional
	// +kubebuilder:default=3000
	Port int32 `json:"port,omitempty"`

	// HealthCheckPath polled for 2xx before traffic is shifted to a new revision.
	// +optional
	// +kubebuilder:default=/
	HealthCheckPath string `json:"healthCheckPath,omitempty"`

	// AutoDeploy triggers a deploy on each push to Branch.
	// +optional
	AutoDeploy bool `json:"autoDeploy,omitempty"`

	// IdleTTLSeconds before the service hibernates ("sleep = free"). 0 = controller default.
	// +optional
	IdleTTLSeconds int32 `json:"idleTTLSeconds,omitempty"`

	// RestartedAt requests a rolling restart when set or changed (verb-as-timestamp,
	// e.g. RFC3339 now): the operator copies it to the pod template annotation
	// "app.bex.co/restarted-at"; the changed template rolls new pods with no gap.
	// Empty = never requested; re-setting the same value is a no-op.
	// See docs/restart-suspend-and-resume.md.
	// +optional
	RestartedAt string `json:"restartedAt,omitempty"`

	// Suspended parks the App without losing anything: the kubernetes runtime
	// scales the Deployment to 0 (Service, Ingress, TLS and Replicas are all
	// kept, so resume just scales back); the opensandbox runtime pauses the
	// sandbox. Phase becomes Hibernated. See docs/restart-suspend-and-resume.md.
	// +optional
	Suspended bool `json:"suspended,omitempty"`

	// Tier is the plan/size; the operator sets the pod's resources (requests==limits)
	// from it. Empty => no resource constraints (best-effort); the control plane sets
	// a tier explicitly. Resource ladder lives in docs/control-plane.md.
	// +optional
	// +kubebuilder:validation:Enum=free;starter;standard;pro;pro-plus;pro-max;pro-ultra
	Tier string `json:"tier,omitempty"`

	// Host is the primary external FQDN to expose this App at (e.g.
	// "beancount.1.2.3.4.sslip.io", or a tenant's custom domain). On the kubernetes
	// runtime the operator creates an Ingress (+ TLS via cert-manager) routing this
	// host to the App's Service. When neither Host, Expose nor Hosts yields a host =>
	// in-cluster only (ClusterIP). Host keeps first position in the effective host
	// list, so existing Apps keep their "<name>-tls" certificate secret untouched.
	// +optional
	Host string `json:"host,omitempty"`

	// Expose, when true and the controller's BEX_BASE_DOMAIN env is set, serves the
	// App at the platform hostname "<name>.<BEX_BASE_DOMAIN>" (in addition to Host
	// and Hosts, if given). Requires wildcard DNS for the base domain.
	// +optional
	Expose bool `json:"expose,omitempty"`

	// Hosts are additional external FQDNs to serve this App at — typically tenants'
	// custom domains, CNAME'd to the platform hostname. Every effective host gets its
	// own Ingress rule and its own cert-manager certificate/secret, so one domain's
	// broken DNS can never block another's issuance or renewal.
	// +optional
	Hosts []string `json:"hosts,omitempty"`
}

// AppPhase mirrors the lifecycle state machine (211.09 §Agent Lifecycle).
// +kubebuilder:validation:Enum=Pending;Building;Deploying;Running;Hibernated;Failed
type AppPhase string

const (
	PhasePending    AppPhase = "Pending"
	PhaseBuilding   AppPhase = "Building"
	PhaseDeploying  AppPhase = "Deploying"
	PhaseRunning    AppPhase = "Running"
	PhaseHibernated AppPhase = "Hibernated"
	PhaseFailed     AppPhase = "Failed"
)

// AppStatus is the observed state of a App.
type AppStatus struct {
	// Phase is the high-level lifecycle state.
	// +optional
	Phase AppPhase `json:"phase,omitempty"`

	// URL is the canonical serving URL (first effective host).
	// +optional
	URL string `json:"url,omitempty"`

	// URLs are all serving URLs when the App has multiple hosts (canonical first).
	// +optional
	URLs []string `json:"urls,omitempty"`

	// ActiveRevision currently serving traffic (e.g. "rev_5").
	// +optional
	ActiveRevision string `json:"activeRevision,omitempty"`

	// Image is the OCI image of the active revision.
	// +optional
	Image string `json:"image,omitempty"`

	// SandboxID is the runtime handle of the active revision (OpenSandbox sandbox id).
	// +optional
	SandboxID string `json:"sandboxID,omitempty"`

	// Endpoint is the raw runtime target for the edge ("host:port/prefix").
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// ObservedGeneration is the .metadata.generation the controller last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state (Ready / Progressing / Degraded).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.activeRevision`
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="Repo",type=string,JSONPath=`.spec.repo`,priority=1

// App is the Schema for the services API
type App struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of App
	// +required
	Spec AppSpec `json:"spec"`

	// status defines the observed state of App
	// +optional
	Status AppStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AppList contains a list of App
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []App `json:"items"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
