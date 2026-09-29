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
// restorable spec fields (restored into the saved spec by a rollback to it).
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
	StartCommand string `json:"startCommand,omitempty"`
}
