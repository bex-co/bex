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

// CloneSecretName and ExternalRegistryPullSecretName derive the two
// build-plane Secrets bex-api mints for an App from the App's own name, and
// then writes into spec.cloneSecret / spec.externalRegistryPullSecret.
//
// They live in the leaf contract module for the same reason BuildJobName and
// DiskPVCName do: both sides of the App CR boundary must derive them
// identically. bex-api creates the Secrets and sets the spec fields; the
// operator recognizes those exact names as an App's OWN build-plane Secrets
// when it enforces LabelProtectedFromTenantMount (rejectProtectedSecretRefs).
// Hand-copied spellings drifting apart is exactly the failure w6/m97 shipped
// to production — the operator refused every App the deploy pipeline had just
// pointed at its own clone Secret — so there is one spelling, here.
//
// Unlike those two, these do NOT run through k8sname.Fit: they are plain
// suffixes on a CR name that already fits, and hash-truncating them would
// silently alias two Apps' credentials onto one Secret.
func CloneSecretName(appName string) string { return appName + "-clone" }

// ExternalRegistryPullSecretName is CloneSecretName's counterpart for the
// dockerconfigjson Secret materialized from a workspace's stored registry
// credential (w2/m14).
func ExternalRegistryPullSecretName(appName string) string { return appName + "-registry-pull" }
