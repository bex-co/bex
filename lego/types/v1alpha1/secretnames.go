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
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

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

// TLSSecretName is the Secret the operator issues a host's certificate into,
// and the one bex-api looks for to report the host verified (w5/m121). Each
// host has its own, so one domain's failed issuance or renewal (a customer's
// deleted CNAME) cannot block the others. The App's first effective host keeps
// the legacy "<app>-tls": renaming it would point the Ingress at an empty
// Secret until cert-manager re-issued. Any other is "<app>-tls-<host>" with
// "*" spelled "wildcard", or a hash of the host past the 253-character Secret
// name limit, so a long host still gets a name of its own.
func TLSSecretName(app string, first bool, host string) string {
	if first {
		return app + "-tls"
	}
	name := app + "-tls-" + strings.ReplaceAll(host, "*", "wildcard")
	if len(name) > validation.DNS1123SubdomainMaxLength {
		sum := sha256.Sum256([]byte(host))
		name = fmt.Sprintf("%s-tls-%x", app, sum[:8])
	}
	return name
}

// TLSSecretNameFor is TLSSecretName for host, first when it leads the hosts
// EffectiveHosts serves: a custom primary in spec.host, or spec.hosts[0] when
// no primary and no platform host precede it. baseDomain must be the
// operator's BEX_BASE_DOMAIN, which bex-api shares.
func (s AppSpec) TLSSecretNameFor(name, baseDomain, host string) string {
	hosts := s.EffectiveHosts(name, baseDomain)
	return TLSSecretName(name, len(hosts) > 0 && hosts[0] == host, host)
}
