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
func DiskPVCName(appName string) string { return DiskChildName(DiskPVCPrefix, appName) }

// DiskLUKSSecretName is the disk's passphrase Secret: the PVC's name plus
// "-luks", by construction and NOT via DiskChildName. The encrypted
// StorageClass (deploy/gitops/base/disk-storageclass.yaml) templates its
// node-publish secret as `${pvc.name}-luks`, so the kubelet fetches exactly
// pvc+suffix — a Secret name derived any other way (an earlier version ran the
// truncation with the suffix inside it, shifting the cut point) silently
// diverges for long App names and every mount of their disks fails. Secret
// names are DNS-1123 subdomains (253 chars), so appending to a 63-char claim
// name is always legal. Verified against the live class on production,
// 2026-09-02 (w2/m86).
func DiskLUKSSecretName(appName string) string {
	return DiskPVCName(appName) + DiskLUKSSecretSuffix
}

// DiskChildName is the naming rule every workload object in the disk plane
// shares: "<prefix><app>", truncated with an 8-hex digest of the full App name
// when it would exceed a DNS-1123 label. The operator names its
// backup/purge/restore workloads through it; only the PVC crosses the module
// boundary, but one rule for the whole plane means a long-named app's objects
// stay recognizably siblings. (The LUKS Secret deliberately does NOT use this
// rule — see DiskLUKSSecretName.)
func DiskChildName(prefix, appName string) string {
	if len(prefix)+len(appName) <= 63 {
		return prefix + appName
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	keep := 63 - len(prefix) - len(sum) - 1
	return prefix + appName[:keep] + "-" + sum
}
