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
	"crypto/sha256"
	"fmt"
)

// DiskPVCPrefix names the per-App claim backing spec.disk. A service has at
// most one disk, so the App's own name is enough to identify it.
const DiskPVCPrefix = "disk-"

// DiskLUKSSecretSuffix names the per-disk encryption-passphrase Secret the
// hcloud CSI driver reads at node-publish time.
const DiskLUKSSecretSuffix = "-luks"

// DiskPVCName and DiskLUKSSecretName derive a disk's child object names from
// the App's, truncating to Kubernetes' 63-character limit with a digest so two
// long App names cannot collide onto one volume.
//
// These live in the leaf contract module for the same reason BuildJobName does:
// both sides of the App CR boundary must derive them identically. The operator
// creates the PVC; bex-api names that exact claim when it queries kubelet's
// volume-stats series for the Disk tab's usage graph
// (docs/ADR082-persistent-disks.md D6). A second, hand-copied spelling of this
// rule would fail silently — an empty graph, not an error — for precisely the
// long-named apps the truncation exists to serve.
func DiskPVCName(appName string) string { return DiskChildName(DiskPVCPrefix, appName, "") }

// DiskLUKSSecretName is the disk's passphrase Secret. The digest is placed
// before the suffix so the "-luks" marker survives truncation.
func DiskLUKSSecretName(appName string) string {
	return DiskChildName(DiskPVCPrefix, appName, DiskLUKSSecretSuffix)
}

// DiskChildName is the naming rule every object in the disk plane shares:
// "<prefix><app><suffix>", truncated with an 8-hex digest of the full App name
// when it would exceed a DNS-1123 label. The digest sits before the suffix so a
// role marker like "-luks" survives truncation. The operator also names its
// backup/purge/restore workloads through it; only the PVC crosses the module
// boundary, but one rule for the whole plane means a long-named app's objects
// stay recognizably siblings.
func DiskChildName(prefix, appName, suffix string) string {
	if len(prefix)+len(appName)+len(suffix) <= 63 {
		return prefix + appName + suffix
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	keep := 63 - len(prefix) - len(suffix) - len(sum) - 1
	return prefix + appName[:keep] + "-" + sum + suffix
}
