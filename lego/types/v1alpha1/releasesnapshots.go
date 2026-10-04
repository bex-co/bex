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
// reads — never one flattened Secret — so each copy inherits the per-map 512 KiB
// quota that keeps it under Kubernetes' 1 MiB Secret ceiling.
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
	// operator's releaseSettingsFingerprint). Empty in older records.
	SettingsFingerprint string `json:"settingsFingerprint,omitempty"`
}

// ReleaseConfigReference selects one retained release without changing saved
// settings. The backend binds it to the newly requested release generation.
type ReleaseConfigReference struct {
	// +kubebuilder:validation:Minimum=1
	Generation int64 `json:"generation"`
	// SourceGeneration zero explicitly requests the legacy image-only fallback.
	// A positive generation must have a release record; its loss is an error.
	// +kubebuilder:validation:Minimum=0
	SourceGeneration int64 `json:"sourceGeneration"`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`
	Image string `json:"image"`
	// PreserveGroupValues is used by Restart. Rollback takes current values
	// from the target's surviving group associations; Restart retains its exact
	// running configuration, including already-snapshotted group values.
	// +optional
	PreserveGroupValues bool `json:"preserveGroupValues,omitempty"`
}

// ActiveReleaseConfig returns the selection belonging to the current or newly
// requested release. Operational metadata generations do not expire it; an
// actual later deployment does. The operator adopts direct CR release edits in
// status before consulting this helper.
func (a *App) ActiveReleaseConfig() *ReleaseConfigReference {
	ref := a.Spec.ReleaseConfig
	if ref == nil || ref.Generation <= 0 || ref.Image == "" {
		return nil
	}
	gen := a.Status.ReleaseGeneration
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
