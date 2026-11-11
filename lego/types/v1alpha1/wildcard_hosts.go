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

import "strings"

// WildcardHostPrefix leads a wildcard custom domain ("*.example.com", w4/m326),
// which serves every name one label below the rest of it.
const WildcardHostPrefix = "*."

// IsWildcardHost reports a wildcard custom domain.
func IsWildcardHost(host string) bool {
	return strings.HasPrefix(host, WildcardHostPrefix)
}

// WildcardHostFor is the wildcard custom domain that serves host
// ("*.example.com" for "api.example.com"), or "" for a single-label host.
// Host routers look a request up by its own name first and by this one when
// no App serves that name: a concrete host beats a wildcard over it.
func WildcardHostFor(host string) string {
	_, parent, ok := strings.Cut(host, ".")
	if !ok || parent == "" {
		return ""
	}
	return WildcardHostPrefix + parent
}

// PublicURLHost is the host a service's URL names: the first of hosts that
// is not a wildcard, which names no single site. "" when there is none.
func PublicURLHost(hosts []string) string {
	for _, host := range hosts {
		if !IsWildcardHost(host) {
			return host
		}
	}
	return ""
}
