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

package keyvalue

import appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

// unavailableReasons maps the operator's Ready-condition reason for a failed
// Key Value to a sentence safe for any reader (w5/m129). The condition's own
// message can carry raw API-server text, so it is not published;
// StorageShrinkRejected's message is operator-authored and is.
var unavailableReasons = map[string]string{
	appv1alpha1.ReasonStatefulSetReadFailed:       "The Key Value's workload status could not be checked.",
	appv1alpha1.ReasonStatefulSetFailed:           "The Key Value's workload could not be updated, so the latest change was not applied.",
	appv1alpha1.ReasonPVCReadFailed:               "The storage volume's status could not be checked.",
	appv1alpha1.ReasonPVCResizeFailed:             "The Key Value's storage volume could not be resized.",
	appv1alpha1.ReasonSecretFailed:                "The Key Value's connection details could not be saved.",
	appv1alpha1.ReasonCredentialSecretFailed:      "The Key Value's credentials could not be saved.",
	appv1alpha1.ReasonSecretRecreateFailed:        "The Key Value's connection details could not be rebuilt.",
	appv1alpha1.ReasonServiceFailed:               "The Key Value's internal address could not be set up.",
	appv1alpha1.ReasonNetworkPolicyFailed:         "The Key Value's network policy could not be applied.",
	appv1alpha1.ReasonBackupNetworkPolicyFailed:   "The backup job's network policy could not be applied.",
	appv1alpha1.ReasonBackupCronJobFailed:         "The daily backup schedule could not be updated.",
	appv1alpha1.ReasonTLSIssuerMissing:            "Public access can't be enabled: no certificate issuer is configured.",
	appv1alpha1.ReasonCertificateFailed:           "The certificate for public access could not be issued.",
	appv1alpha1.ReasonCertificateCleanupFailed:    "The certificate for public access could not be removed.",
	appv1alpha1.ReasonStorageClassMissing:         "The storage volume has no storage class, so it cannot grow.",
	appv1alpha1.ReasonStorageClassNotFound:        "The storage volume's storage class was not found, so it cannot grow.",
	appv1alpha1.ReasonStorageClassNotExpandable:   "The storage volume's storage class does not allow it to grow.",
	appv1alpha1.ReasonStorageBlockedByQuota:       "The storage volume can't grow: the workspace's storage quota has no room for it.",
	appv1alpha1.ReasonPersistenceTransitionFailed: "Preparing the stored data for startup failed; the data is kept and this will retry.",
	appv1alpha1.ReasonPersistenceSourceUnknown:    "The stored data's persistence mode is unknown, so the Key Value can't start until a platform operator confirms it. No data was changed.",
}
