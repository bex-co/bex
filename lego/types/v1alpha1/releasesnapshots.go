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
	"regexp"
	"strconv"
	"strings"
)

// releasesnapshots.go is the cross-boundary contract for per-release configuration
// snapshots (w1/m152; docs/ADR004-app-deployment.md §Per-release configuration
// snapshots). The operator writes them when a release dispatches; bex-api reads them
// to restore a rollback target's configuration. Like BuildJobName, the names have to
// agree exactly on both sides, so they are derived here and nowhere else.

// ReleaseSnapshotSuffix marks a snapshot and carries its release generation.
const ReleaseSnapshotSuffix = "-r"

// ReleaseSnapshotName is the immutable per-release copy of a configuration source
// Secret: `<source>-r<generation>`. The operator writes one per source a release
// reads — never one flattened Secret — so each copy inherits its source map's
// quota (512 KiB of env vars, 1 MiB of secret-file contents) that keeps it
// within Kubernetes' 1 MiB Secret limit.
func ReleaseSnapshotName(source string, generation int64) string {
	return source + ReleaseSnapshotSuffix + strconv.FormatInt(generation, 10)
}

// AppReleaseSnapshotName is app's snapshot of source at generation — the name every
// reader and writer must use. A source the App owns (`<app>-env`, `<app>-files`)
// keeps `<source>-r<gen>`. A SHARED source is scoped by the app:
// `<app>-<source>-r<gen>`. An env group's `<evg-id>-env` lives in the workspace
// namespace and is read by every linked service, while release generations are
// counted PER APP — so an unscoped `<evg-id>-env-r3` was the same object for every
// linked service at its own generation 3. Copy-once then handed the second service
// the first one's copy (possibly stale values), owned and garbage-collected by the
// first (w1/m152 t004).
func AppReleaseSnapshotName(app, source string, generation int64) string {
	if strings.HasPrefix(source, app+"-") {
		return ReleaseSnapshotName(source, generation)
	}
	return ReleaseSnapshotName(app+"-"+source, generation)
}

// IsReleaseSnapshotName reports whether name is itself a snapshot. The suffix must
// be followed by digits only and preceded by a non-empty source, so a tenant Secret
// ending in "-r12x", "-rollback", or a bare "-r3" is not mistaken for one.
func IsReleaseSnapshotName(name string) bool {
	idx := strings.LastIndex(name, ReleaseSnapshotSuffix)
	if idx <= 0 {
		return false
	}
	digits := name[idx+len(ReleaseSnapshotSuffix):]
	if digits == "" {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ReleaseRecordName is the per-release record of what a generation ran with beyond
// its configuration Secrets: the exact pod template the operator applied (restored
// verbatim when a cancel settles back onto that release) and the release's
// restorable spec fields (selected for runtime by a rollback to it).
func ReleaseRecordName(app string, generation int64) string {
	return ReleaseSnapshotName(app+"-podtemplate", generation)
}

const (
	// ReleaseRecordPodTemplateKey holds the JSON-encoded corev1.PodTemplateSpec.
	ReleaseRecordPodTemplateKey = "podTemplate"
	// ReleaseRecordSpecKey holds the JSON-encoded ReleaseRecordSpec.
	ReleaseRecordSpecKey = "releaseSpec"
	// ReleaseRecordAppContainer names the pod template's container that runs the
	// release's image.
	ReleaseRecordAppContainer = "app"
	// ReleaseRecordAdoptedSpecKey holds the JSON-encoded AppSpec the release was
	// adopted with (w8/m121). bex-api reads only its registry binding, for a
	// record written before ReleaseRecordSpec.Registry (w8/m529).
	ReleaseRecordAdoptedSpecKey = "adoptedSpec"
)

// ReleaseRecordSpec is the part of a release's spec that Render restores on a
// rollback besides its image and environment ("the start command matches the target
// deploy"; render.com/docs/rollbacks). Plan, custom domains and disks are
// deliberately absent: Render keeps the current ones, and so does bex.
type ReleaseRecordSpec struct {
	// Version 1 records source membership and literal env in addition to the
	// original start command. Version 0 derives those from its pod template.
	Version      int    `json:"version,omitempty"`
	StartCommand string `json:"startCommand,omitempty"`
	// Pointers distinguish an explicitly empty value from a legacy record that
	// predates these fields. Legacy replicas cannot be reconstructed from a pod.
	Command         *string `json:"command,omitempty"`
	HealthCheckPath *string `json:"healthCheckPath,omitempty"`
	Replicas        *int32  `json:"replicas,omitempty"`
	// SavedReplicas is the saved count when this template was applied. It lets
	// a later operational scale supersede a rollback's initial historical count.
	SavedReplicas    *int32   `json:"savedReplicas,omitempty"`
	Env              []EnvVar `json:"env,omitempty"`
	EnvFromSecret    string   `json:"envFromSecret,omitempty"`
	EnvFromSecrets   []string `json:"envFromSecrets,omitempty"`
	FilesFromSecrets []string `json:"filesFromSecrets,omitempty"`
	// SettingsFingerprint identifies the saved settings the release ran with,
	// so a cancel can tell a retained change from an identical redeploy (the
	// operator's releaseSettingsFingerprint). A Restart or rollback runs its
	// target's build, and an environment "Save and deploy" or group deploy the
	// served one, so its fingerprint names that build's settings
	// (BuildSettingsFingerprint) with its own release settings. Empty in older
	// records.
	SettingsFingerprint string `json:"settingsFingerprint,omitempty"`
	// BuildSettingsFingerprint identifies the build settings of the image the
	// release runs (repository, branch, root directory, build command,
	// Dockerfile, runtime, but not the start command and environment a release
	// applies when it starts the image: the operator's
	// buildSettingsFingerprint). A Restart or rollback copies its target's
	// (w32/058), and an environment "Save and deploy" or group deploy the
	// served release's (w32/060). Empty in records written before it, and in
	// the record of a Restart or rollback of one.
	BuildSettingsFingerprint string `json:"buildSettingsFingerprint,omitempty"`
	// ImageSource is the reference the release's image digest was resolved
	// from, such as an image-port service's tag. A rollback or Restart serves
	// the digest itself and inherits this from the release it selects, so a
	// release serving the saved tag's digest is not an undeployed change
	// (w4/531). Empty in older records and for images run as written.
	ImageSource string `json:"imageSource,omitempty"`
	// Runtime is the port, shutdown delay and pre-deploy command the release
	// ran with (w4/m433). nil in records written before them: a Restart or
	// rollback to such a record runs the saved values.
	Runtime *ReleaseRuntimeSettings `json:"runtime,omitempty"`
	// Registry is the registry credential the release pulled its image with
	// (w8/m529). nil in records written before it.
	Registry *ReleaseRegistryBinding `json:"registry,omitempty"`
}

// ReleaseRegistryBinding is the external registry credential a release pulls
// its image with: the workspace credential and the per-App pull Secret bex-api
// materialized from it (ExternalRegistryPullSecretName). Both empty: the
// release pulls anonymously. A Restart pulls the serving release with its own
// binding and a rollback its target's, as each went live, whatever credential
// the service binds now (w8/m529).
type ReleaseRegistryBinding struct {
	// +optional
	// +kubebuilder:validation:MaxLength=63
	CredentialID string `json:"credentialId,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=253
	PullSecret string `json:"pullSecret,omitempty"`
}

// RegistryBinding is the binding a release dispatched from s pulls with.
func (s AppSpec) RegistryBinding() *ReleaseRegistryBinding {
	binding := &ReleaseRegistryBinding{PullSecret: s.ExternalRegistryPullSecret}
	if s.RegistryCredentialID != nil {
		binding.CredentialID = *s.RegistryCredentialID
	}
	return binding
}

// ApplyTo runs spec with the recorded binding; nil leaves the saved one.
func (b *ReleaseRegistryBinding) ApplyTo(spec *AppSpec) {
	if b == nil {
		return
	}
	spec.ExternalRegistryPullSecret, spec.RegistryCredentialID = b.PullSecret, nil
	if b.CredentialID != "" {
		spec.RegistryCredentialID = new(b.CredentialID)
	}
}

// ReleasePullSecret is the external registry pull Secret the current release
// pulls with: the active selection's recorded binding (Restart, rollback) over
// the saved spec's. A selection without one (legacy) pulls with the saved.
func (a *App) ReleasePullSecret() string {
	if ref := a.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 && ref.Registry != nil {
		return ref.Registry.PullSecret
	}
	return a.Spec.ExternalRegistryPullSecret
}

// ReleaseRuntimeSettings are the saved settings outside the pod's command and
// environment that a release runs with, and that Render's Restart keeps
// ("the exact same Git commit and configuration as the running instance";
// render.com/docs/deploys#restarting-a-service). A rollback selects the
// target's, as it does its start command and health check path (w4/m433).
type ReleaseRuntimeSettings struct {
	// +optional
	Port int32 `json:"port,omitempty"`
	// +optional
	// +kubebuilder:validation:Enum=configured-v1;image-v1;image-configured-v1;web-image-v1;web-image-configured-v1
	PortMode string `json:"portMode,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	MaxShutdownDelaySeconds *int32 `json:"maxShutdownDelaySeconds,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxLength=4096
	PreDeployCommand string `json:"preDeployCommand,omitempty"`
}

// RuntimeSettings is the spec's ReleaseRuntimeSettings, as a release
// dispatched from it records them.
func (s AppSpec) RuntimeSettings() *ReleaseRuntimeSettings {
	settings := &ReleaseRuntimeSettings{Port: s.Port, PortMode: s.PortMode, PreDeployCommand: s.PreDeployCommand}
	if s.MaxShutdownDelaySeconds != nil {
		settings.MaxShutdownDelaySeconds = new(*s.MaxShutdownDelaySeconds)
	}
	return settings
}

// ApplyTo runs spec with the recorded settings; nil leaves the saved ones. A
// recorded port mode applies only within the saved mode's family (configured
// or image ports): what discovers a service's ports is its source's, not a
// setting a release selects.
func (r *ReleaseRuntimeSettings) ApplyTo(spec *AppSpec) {
	if r == nil {
		return
	}
	probe := *spec
	probe.PortMode = r.PortMode
	if probe.HasImagePorts() == spec.HasImagePorts() {
		spec.Port, spec.PortMode = r.Port, r.PortMode
	}
	spec.MaxShutdownDelaySeconds = nil
	if r.MaxShutdownDelaySeconds != nil {
		spec.MaxShutdownDelaySeconds = new(*r.MaxShutdownDelaySeconds)
	}
	spec.PreDeployCommand = r.PreDeployCommand
}

// RuntimeSpec is the spec the current release runs its port, shutdown delay
// and pre-deploy command from: the active selection's recorded settings
// (Restart, rollback) over the saved spec, else the saved spec itself.
func (a *App) RuntimeSpec() AppSpec {
	spec := a.Spec
	if ref := a.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 {
		ref.Runtime.ApplyTo(&spec)
	}
	return spec
}

// StaticPublicationRecordName is the per-release record of where a static_site
// generation's successful publication actually lives in the object store
// (w4/m197). The operator writes it once the publication succeeds; bex-api
// reads it to roll back to that publication without re-extracting the target's
// image through today's saved publish directory. Distinct from
// ReleaseRecordName: a static release has no pod template or runtime inputs.
func StaticPublicationRecordName(app string, generation int64) string {
	return ReleaseSnapshotName(app+"-static", generation)
}

// StaticPublicationPrefixKey holds the recorded object-key prefix, e.g.
// "<workspace>/<app>/rev-7/".
const StaticPublicationPrefixKey = "prefix"

// staticPublicationPrefix matches a recorded static revision prefix:
// "[<workspace>/]<app>/rev-<generation>/".
var staticPublicationPrefix = regexp.MustCompile(`^(?:[a-z0-9][a-z0-9-]*/)?[a-z0-9][a-z0-9-]*/rev-[1-9][0-9]*/$`)

// ValidStaticPublicationPrefix reports whether prefix has the shape of a
// recorded static revision prefix. Ownership (that it is this App's) is checked
// by the operator against the App's own identity.
func ValidStaticPublicationPrefix(prefix string) bool {
	return staticPublicationPrefix.MatchString(prefix)
}

// ReleaseConfigReference selects one retained release without changing saved
// settings. The backend binds it to the newly requested release generation.
// A static_site rollback selects the target's retained publication
// (StaticPrefix); its Image is empty when the target published straight from
// its repository.
// +kubebuilder:validation:XValidation:rule="has(self.image) || has(self.staticPrefix)",message="releaseConfig selects an image or a static publication"
type ReleaseConfigReference struct {
	// +kubebuilder:validation:Minimum=1
	Generation int64 `json:"generation"`
	// SourceGeneration zero explicitly requests the legacy image-only fallback.
	// A positive generation must have a release record; its loss is an error.
	// +kubebuilder:validation:Minimum=0
	SourceGeneration int64 `json:"sourceGeneration"`
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`
	Image string `json:"image,omitempty"`
	// StaticPrefix is the retained object-store prefix of the target's static
	// publication (StaticPublicationRecordName). When set, a static_site
	// publishes this release by copying that prefix — never by extracting Image
	// through the saved publish directory or rebuilding from saved settings.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^(?:[a-z0-9][a-z0-9-]*/)?[a-z0-9][a-z0-9-]*/rev-[1-9][0-9]*/$`
	StaticPrefix string `json:"staticPrefix,omitempty"`
	// PreserveGroupValues is used by Restart. Rollback takes current values
	// from the target's surviving group associations; Restart retains its exact
	// running configuration, including already-snapshotted group values.
	// +optional
	PreserveGroupValues bool `json:"preserveGroupValues,omitempty"`
	// Runtime is the selected release's recorded port, shutdown delay and
	// pre-deploy command (ReleaseRecordSpec.Runtime). nil when its record
	// predates them: the release then runs the saved values.
	// +optional
	Runtime *ReleaseRuntimeSettings `json:"runtime,omitempty"`
	// Registry is the selected release's recorded registry binding
	// (ReleaseRecordSpec.Registry), refreshed from its credential's stored
	// token. nil when its record predates it: the release then pulls with the
	// saved binding.
	// +optional
	Registry *ReleaseRegistryBinding `json:"registry,omitempty"`
}

// ActiveReleaseConfig returns the selection belonging to the current or newly
// requested release. Operational metadata generations do not expire it; an
// actual later deployment does, once the operator adopts it: a selection the
// operator adopted and still runs keeps resolving while a newer requested
// release waits behind it (Render's Wait, w4/661). The operator adopts direct
// CR release edits in status before consulting this helper.
func (a *App) ActiveReleaseConfig() *ReleaseConfigReference {
	ref := a.Spec.ReleaseConfig
	if ref == nil || ref.Generation <= 0 || (ref.Image == "" && ref.StaticPrefix == "") {
		return nil
	}
	gen := a.Status.ReleaseGeneration
	if gen == ref.Generation && a.Status.SelectedReleaseGeneration == gen {
		return ref
	}
	if requested, err := strconv.ParseInt(a.Annotations[AnnotationReleaseGeneration], 10, 64); err == nil && requested > gen {
		gen = requested
	}
	if gen <= 0 {
		gen = a.Generation
	}
	if gen != ref.Generation {
		return nil
	}
	return ref
}

// RevisionGeneration parses an active revision ("rev-N", AppStatus.ActiveRevision)
// to the release generation N it serves.
func RevisionGeneration(rev string) (int64, bool) {
	raw, ok := strings.CutPrefix(rev, "rev-")
	if !ok {
		return 0, false
	}
	generation, err := strconv.ParseInt(raw, 10, 64)
	return generation, err == nil && generation > 0
}
