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
	// TypeStaticSite builds a repo and serves the built output (Spec.PublishPath)
	// from an object-store origin behind the shared static-server proxy — no
	// Deployment/Service/Ingress for the served content. Redirects/rewrites
	// (Spec.Routes) and custom response headers (Spec.Headers) apply at the edge.
	TypeStaticSite = "static_site"
)

// AppSpec is the desired state of a deploy-from-git App — the Render-like
// unit from strategy 211.09. Mirrors the Node MVP's service spec (src/api.js).
type AppSpec struct {
	// Type is the service kind, tracking Render's serviceType vocabulary:
	// web_service (default), private_service, background_worker, cron_job,
	// static_site. Empty is treated as web_service so existing Apps are unchanged.
	// A background_worker runs the image with no HTTP port/Service/Ingress; a
	// cron_job runs the image's command on Schedule as a Kubernetes CronJob (no
	// Deployment/Service/Ingress); a static_site builds the repo and serves its
	// PublishPath output from an object-store origin behind the shared
	// static-server (no Deployment/Service for the served content).
	// +optional
	// +kubebuilder:validation:Enum=web_service;private_service;background_worker;cron_job;static_site
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
	// no-op. Ignored for non-cron types. See docs/ADR006-bex-api.md (cron run trigger).
	// +optional
	RunAt string `json:"runAt,omitempty"`

	// PublishPath is the built output directory a static_site serves as its
	// document root (Render's "Publish Directory", e.g. "dist", "build",
	// "public"), relative to the built image's working directory. The publish
	// step uploads only this directory's contents to the object-store origin, so
	// the origin prefix root IS the site root. Required when Type is static_site;
	// ignored for every other type. See docs/ADR029-static-sites.md.
	// +optional
	PublishPath string `json:"publishPath,omitempty"`

	// Routes are the ordered redirect/rewrite rules a static_site applies at the
	// edge (Render's /routes), first match wins. Ignored for every other type.
	// +optional
	Routes []StaticRoute `json:"routes,omitempty"`

	// Headers are the custom response-header rules a static_site applies at the
	// edge (Render's /headers), scoped by request-path pattern. Ignored for every
	// other type.
	// +optional
	Headers []StaticHeader `json:"headers,omitempty"`

	// Repo is the git repository URL to deploy from (https://, ssh://, or git@
	// SCP form). Either Repo (build-from-git) or Image (prebuilt) must be set.
	// file:// and bare local paths are rejected at the CRD schema so a request
	// can never point a build at the build pod's own filesystem (w6/m6 t003).
	// +optional
	// +kubebuilder:validation:Pattern=`^(https?://|ssh://|git@)`
	// +kubebuilder:validation:MaxLength=2048
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
	// Image apps. Traversal ("..") is rejected at the API boundary (w6/m6 t003).
	// +optional
	// +kubebuilder:validation:MaxLength=512
	RootDir string `json:"rootDir,omitempty"`

	// Branch to track. Defaults to "main". A git ref (no shell metacharacters,
	// no leading dash) — enforced at the CRD schema (w6/m6 t003).
	// +optional
	// +kubebuilder:default=main
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._/@+-]*$`
	// +kubebuilder:validation:MaxLength=255
	Branch string `json:"branch,omitempty"`

	// CloneSecret names a Secret in the App's namespace holding a git credential
	// (key "token") used to clone a private Repo. When set, the build Job passes
	// it to BuildKit as the standard GIT_AUTH_TOKEN build secret so the https
	// git-context fetch authenticates. Empty means an unauthenticated (public)
	// clone — today's behavior, unchanged. The operator is GitHub-unaware: it
	// only mounts the Secret; bex-api mints and refreshes the token
	// (docs/ADR026-github-integration.md). An absent/expired Secret fails the build
	// with a clear condition (no silent public-clone fallback).
	// +optional
	CloneSecret string `json:"cloneSecret,omitempty"`

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

	// Env are environment variables set on the App's container, in the order
	// given. Each entry is either a plain literal (Value) or a single-key
	// reference into a Secret (ValueFrom.SecretKeyRef). Literal entries carry
	// non-secret configuration only; a secret-backed entry — the shape a bex.yml
	// `fromDatabase` reference resolves to — wires a service's DATABASE_URL at the
	// referenced Database's CNPG connection Secret without the credential ever
	// appearing in the App spec (docs/ADR013-secrets.md, w1/m24). Bulk secret
	// material still arrives via EnvFromSecret. The operator-owned PORT always
	// wins: a user Env entry named PORT is ignored, so it can never shadow the
	// injected value.
	// +optional
	Env []EnvVar `json:"env,omitempty"`

	// EnvFromSecret names a Secret in the App's namespace whose keys are injected
	// into the container as environment variables (envFrom). This is where the
	// env-vars API (docs/ADR013-secrets.md) materializes a service's OpenBao-backed
	// credentials — a per-app "<name>-env" Secret projected from the source of
	// truth. PORT still wins over any colliding key.
	// +optional
	EnvFromSecret string `json:"envFromSecret,omitempty"`

	// EnvFromSecrets names additional Secrets injected via envFrom, one per linked
	// environment group (docs/ADR013-secrets.md — environment groups: a "<evg-id>-env"
	// Secret the env-groups API materializes and shares across services). These are
	// wired BEFORE EnvFromSecret, so a service's own env var wins over a linked
	// group's on a key collision; the operator-owned PORT still wins over all. Each
	// is referenced as optional, so a group whose Secret is briefly absent never
	// wedges the pod. Empty => no group env. See EnvFromSecret for the per-service set.
	// +optional
	EnvFromSecrets []string `json:"envFromSecrets,omitempty"`

	// FilesFromSecrets names Secrets whose keys are projected as files under
	// /etc/secrets (one file per key) via a single projected volume: the service's
	// own secret files ("<name>-files", docs/ADR013-secrets.md — secret files) plus each
	// linked environment group's files ("<evg-id>-files"). All sources merge into
	// the one /etc/secrets mount; each is optional, so an absent source contributes
	// no files rather than failing the mount. Empty => no /etc/secrets volume.
	// +optional
	FilesFromSecrets []string `json:"filesFromSecrets,omitempty"`

	// HealthCheckPath is the HTTP path the operator GETs to gate pod readiness
	// (Render's health check — a 2xx/3xx makes a pod ready and routes traffic
	// to it). Wired as the app container's ReadinessProbe; defaults to "/".
	// Applies only to HTTP-serving service types (web/private) — workers and
	// cron jobs expose no port, so the path is unused there.
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
	// See docs/ADR007-restart-suspend-and-resume.md.
	// +optional
	RestartedAt string `json:"restartedAt,omitempty"`

	// Suspended parks the App without losing anything: the kubernetes runtime
	// scales the Deployment to 0 (Service, Ingress, TLS and Replicas are all
	// kept, so resume just scales back); the opensandbox runtime pauses the
	// sandbox. Phase becomes Hibernated. See docs/ADR007-restart-suspend-and-resume.md.
	// +optional
	Suspended bool `json:"suspended,omitempty"`

	// Autoscaling declares the per-service autoscaling policy. When enabled, the
	// operator adjusts spec.replicas within [minReplicas, maxReplicas] based on
	// live CPU/memory utilization. When nil or disabled, the service runs at a
	// fixed spec.replicas. Only applies to the kubernetes runtime.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Tier is the plan/size; the operator sets the pod's resources (requests==limits)
	// from it. Empty => no resource constraints (best-effort); the control plane sets
	// a tier explicitly. Resource ladder lives in docs/ADR003-control-plane.md.
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

// AutoscalingSpec declares the autoscaling policy for a service. When enabled,
// the operator adjusts spec.replicas within [minReplicas, maxReplicas] based on
// live CPU/memory utilization vs the declared targets. Mirrors Render's Scaling
// tab (minInstances / maxInstances / targetCPUPercent / targetMemoryPercent).
type AutoscalingSpec struct {
	// Enabled turns per-service autoscaling on. When false the service runs at
	// spec.replicas as today (a fixed count). Required.
	Enabled bool `json:"enabled"`

	// MinReplicas is the lower replica bound; the autoscaler never drives replicas
	// below this. Must be ≥ 0. Defaults to 1 when omitted and autoscaling is enabled.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the upper replica bound; the autoscaler never drives replicas
	// above this. Must be ≥ MinReplicas. Required when Enabled.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// TargetCPUPercent is the desired average CPU utilization as a percentage of
	// the tier's CPU limit across all running pods. The autoscaler adjusts
	// replicas to approach this target. Either TargetCPUPercent or
	// TargetMemoryPercent (or both) is required when Enabled.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUPercent *int32 `json:"targetCPUPercent,omitempty"`

	// TargetMemoryPercent is the desired average memory utilization as a
	// percentage of the tier's memory limit across all running pods.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty"`
}

// EnvVar is one environment variable for an App's container — either a plain
// literal (Value) or a single-key Secret reference (ValueFrom). Mirrors the
// two shapes a bex.yml envVars entry resolves to: a hardcoded {key,value} maps
// to a literal, and a fromDatabase reference maps to a ValueFrom.SecretKeyRef
// into the Database's CNPG connection Secret (w1/m24). Value and ValueFrom are
// mutually exclusive; ValueFrom is the only way a connection string reaches a
// container without its plaintext landing in the App spec (docs/ADR013-secrets.md).
type EnvVar struct {
	// Name of the environment variable.
	// +required
	Name string `json:"name"`

	// Value is the literal value; empty is allowed (sets the variable to "").
	// Mutually exclusive with ValueFrom.
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom sources this variable from a Secret key in the App's namespace.
	// The bex.yml fromDatabase form resolves here, so a service's DATABASE_URL
	// points at the CNPG "<database>-app" Secret without the credential ever
	// appearing in bex.yml or the App spec (survives credential rotation: a
	// redeploy reads the live Secret value). Mutually exclusive with Value.
	// +optional
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource selects a single Secret key to source an EnvVar from — the
// shape a fromDatabase reference resolves to. Mirrors Kubernetes' core EnvVar
// sourcing (a narrowed EnvVarSource with only the SecretKeyRef selector bex
// uses today).
type EnvVarSource struct {
	// SecretKeyRef names the Secret + key whose value populates this variable.
	// +optional
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// SecretKeySelector names one key of one Secret in the App's namespace.
type SecretKeySelector struct {
	// Name is the Secret name (e.g. a Database's "<name>-app" connection Secret).
	// +required
	Name string `json:"name"`

	// Key is the key within the Secret (e.g. "uri", "host", "port", "username",
	// "password", "dbname" — the CNPG app-Secret vocabulary, which Render's
	// fromDatabase property names map onto).
	// +required
	Key string `json:"key"`
}

// StaticRoute is one redirect/rewrite rule for a static_site, matching Render's
// route shape. Rules are evaluated in order (first match wins). A redirect
// answers with a 301 to Destination; a rewrite serves Destination's content
// with 200 (the SPA fallback is a rewrite of "/*" to "/index.html"). Source and
// Destination are request paths; "/*" is a trailing wildcard.
type StaticRoute struct {
	// Type is "redirect" (301 Location) or "rewrite" (200, serve another path).
	// +required
	// +kubebuilder:validation:Enum=redirect;rewrite
	Type string `json:"type"`

	// Source is the request path pattern to match (e.g. "/old", "/app/*").
	// +required
	Source string `json:"source"`

	// Destination is the target path ("/new", "/index.html"). A trailing "/*" in
	// Source is appended to a Destination that ends in "/*".
	// +required
	Destination string `json:"destination"`
}

// StaticHeader is one custom response-header rule for a static_site, matching
// Render's header shape: Name/Value is added to responses whose request path
// matches Path (e.g. "/*" for all paths, "/assets/*" for a subtree).
type StaticHeader struct {
	// Path is the request path pattern the header applies to (e.g. "/*").
	// +required
	Path string `json:"path"`

	// Name is the response header name (e.g. "X-Frame-Options").
	// +required
	Name string `json:"name"`

	// Value is the response header value (e.g. "DENY").
	// +required
	Value string `json:"value"`
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
