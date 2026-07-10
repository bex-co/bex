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

// Service types, tracking Render's serviceType vocabulary. An empty spec.type is
// treated as TypeWebService, so pre-existing Apps keep their behavior.
const (
	// TypeWebService is an HTTP service exposed at a URL (Deployment + Service +
	// optional Ingress). The default.
	TypeWebService = "web_service"
	// TypePrivateService is an HTTP service reachable only in-cluster (Deployment +
	// ClusterIP Service, no platform Ingress).
	TypePrivateService = "private_service"
	// TypeBackgroundWorker runs the built image with no HTTP port: a bare
	// Deployment, no Service, no Ingress, no URL.
	TypeBackgroundWorker = "background_worker"
	// TypeCronJob runs the built image's command on Spec.Schedule as a Kubernetes
	// CronJob — no Deployment/Service/Ingress; run history lands in Status.Runs.
	TypeCronJob = "cron_job"
)

// AppSpec is the desired state of a deploy-from-git App — the Render-like
// unit from strategy 211.09. Mirrors the Node MVP's service spec (src/api.js).
type AppSpec struct {
	// Type is the service kind, tracking Render's serviceType vocabulary:
	// web_service (default), private_service, background_worker, cron_job. Empty is
	// treated as web_service so existing Apps are unchanged. A background_worker
	// runs the image with no HTTP port/Service/Ingress; a cron_job runs the image's
	// command on Schedule as a Kubernetes CronJob (no Deployment/Service/Ingress).
	// +optional
	// +kubebuilder:validation:Enum=web_service;private_service;background_worker;cron_job
	Type string `json:"type,omitempty"`

	// Schedule is the cron expression (standard 5-field crontab) a cron_job runs
	// on. Required when Type is cron_job; ignored for every other type.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Command overrides the built image's default entrypoint/cmd for a cron_job's
	// run (Render's cron "Command" field, e.g. "npm run report"); empty runs the
	// image's own command unmodified. Ignored for every other type.
	// +optional
	Command string `json:"command,omitempty"`

	// RunAt requests a one-off run of a cron_job now (verb-as-timestamp, like
	// RestartedAt): when it changes, the operator creates a single Job from the
	// cron's template. Empty = never requested; re-setting the same value is a
	// no-op. Ignored for non-cron types. See docs/bex-api.md (cron run trigger).
	// +optional
	RunAt string `json:"runAt,omitempty"`

	// Repo is the git repository URL (or local path) to deploy from. Either Repo
	// (build-from-git) or Image (prebuilt) must be set.
	// +optional
	Repo string `json:"repo,omitempty"`

	// Image is a prebuilt OCI image to run directly, skipping the build plane.
	// +optional
	Image string `json:"image,omitempty"`

	// RootDir scopes build-from-git to a subdirectory of Repo (Render's Root
	// Directory setting, for monorepos): the build-context git ref gets a
	// ":<RootDir>" suffix so BuildKit builds only that subdirectory's
	// Dockerfile, and the git-push auto-deploy webhook only redeploys when the
	// pushed diff touches paths under it. Empty means the repo root (today's
	// behavior, unchanged). Dockerfile builder only; ignored for prebuilt
	// Image apps.
	// +optional
	RootDir string `json:"rootDir,omitempty"`

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

	// Env are plain (literal) environment variables set on the App's container,
	// in the order given. These carry non-secret configuration only — secret
	// material is delivered out-of-band via EnvFromSecret (docs/secrets.md), never
	// inlined here where it would sit in plaintext in etcd. The operator-owned PORT
	// always wins: a user Env entry named PORT is ignored, so it can never shadow
	// the injected value.
	// +optional
	Env []EnvVar `json:"env,omitempty"`

	// EnvFromSecret names a Secret in the App's namespace whose keys are injected
	// into the container as environment variables (envFrom). This is where the
	// env-vars API (docs/secrets.md) materializes a service's OpenBao-backed
	// credentials — a per-app "<name>-env" Secret projected from the source of
	// truth. PORT still wins over any colliding key.
	// +optional
	EnvFromSecret string `json:"envFromSecret,omitempty"`

	// EnvFromSecrets names additional Secrets injected via envFrom, one per linked
	// environment group (docs/secrets.md — environment groups: a "<evg-id>-env"
	// Secret the env-groups API materializes and shares across services). These are
	// wired BEFORE EnvFromSecret, so a service's own env var wins over a linked
	// group's on a key collision; the operator-owned PORT still wins over all. Each
	// is referenced as optional, so a group whose Secret is briefly absent never
	// wedges the pod. Empty => no group env. See EnvFromSecret for the per-service set.
	// +optional
	EnvFromSecrets []string `json:"envFromSecrets,omitempty"`

	// FilesFromSecrets names Secrets whose keys are projected as files under
	// /etc/secrets (one file per key) via a single projected volume: the service's
	// own secret files ("<name>-files", docs/secrets.md — secret files) plus each
	// linked environment group's files ("<evg-id>-files"). All sources merge into
	// the one /etc/secrets mount; each is optional, so an absent source contributes
	// no files rather than failing the mount. Empty => no /etc/secrets volume.
	// +optional
	FilesFromSecrets []string `json:"filesFromSecrets,omitempty"`

	// HealthCheckPath is the HTTP path intended for revision health checks.
	// Declared for the bex.yml/Render contract; the operator does not read it
	// yet — Running is gated on Deployment replica readiness instead.
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

// EnvVar is a single literal name/value environment variable for an App's
// container (the plain half of Render's envVars shape). Only literal values are
// carried here; a secret reference belongs in AppSpec.EnvFromSecret.
type EnvVar struct {
	// Name of the environment variable.
	// +required
	Name string `json:"name"`

	// Value is the literal value; empty is allowed (sets the variable to "").
	// +optional
	Value string `json:"value,omitempty"`
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

// CronRun is one execution of a cron_job — a Kubernetes Job spawned either by the
// CronJob's schedule or by a one-off RunAt trigger. Recorded in AppStatus.Runs so
// the run history is visible over the API without listing Jobs.
type CronRun struct {
	// Name is the Kubernetes Job name backing this run.
	// +required
	Name string `json:"name"`

	// StartedAt is when the run began (RFC3339); empty if it has not started yet.
	// +optional
	StartedAt string `json:"startedAt,omitempty"`

	// FinishedAt is when the run completed or failed (RFC3339); empty while running.
	// +optional
	FinishedAt string `json:"finishedAt,omitempty"`

	// Status is the run outcome: Running, Succeeded, or Failed.
	// +required
	Status string `json:"status"`
}

// AppStatus is the observed state of a App.
type AppStatus struct {
	// Phase is the high-level lifecycle state.
	// +optional
	Phase AppPhase `json:"phase,omitempty"`

	// Runs is the recent run history of a cron_job (newest first), populated from
	// the Jobs the CronJob schedule and one-off RunAt triggers create. Empty for
	// every other service type.
	// +optional
	Runs []CronRun `json:"runs,omitempty"`

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
