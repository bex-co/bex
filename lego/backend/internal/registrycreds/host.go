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

package registrycreds

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// dockerHubAliases are the other spellings of Docker Hub. registryHost resolves
// a Hub image to "docker.io", so a credential stored under an alias never
// matched one (w4/149).
var dockerHubAliases = map[string]bool{
	"index.docker.io":         true,
	"registry-1.docker.io":    true,
	"registry.hub.docker.com": true,
}

// registryHostPattern is a bare registry host with an optional port: DNS
// labels, or localhost.
var registryHostPattern = regexp.MustCompile(`^(localhost|[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*)(:[0-9]{1,5})?$`)

// foldRegistryHost lowercases a bare host and folds Docker Hub's aliases, the
// form every comparison uses.
func foldRegistryHost(host string) string {
	host = strings.ToLower(host)
	if dockerHubAliases[host] {
		return "docker.io"
	}
	return host
}

// canonicalRegistryHost turns what a user pastes for a registry into the host
// images are matched against: it drops a scheme and trailing slashes, lowercases,
// and folds Docker Hub aliases. "https://ghcr.io/" was stored verbatim and then
// never matched ghcr.io, so the credential was saved with a success toast and
// could never be used (w4/149). A path, query, or anything that still is not a
// host[:port] is a named 400, since a registry credential is per host.
func canonicalRegistryHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	lower := strings.ToLower(host)
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(lower, scheme) {
			host = host[len(scheme):]
			break
		}
	}
	host = strings.TrimRight(host, "/")
	host = foldRegistryHost(host)
	if host == "" || !registryHostPattern.MatchString(host) {
		return "", fmt.Errorf("%w: host must be a registry host such as ghcr.io or registry.example.com:5000, without a path (got %q)", core.ErrBadRequest, strings.TrimSpace(raw))
	}
	return host, nil
}

// sameRegistryHost reports whether a stored credential host serves host (a
// registryHost result). Rows stored before canonicalization may still carry a
// scheme or an alias, so the stored side is canonicalized too.
func sameRegistryHost(stored, host string) bool {
	canonical, err := canonicalRegistryHost(stored)
	if err != nil {
		return false
	}
	return canonical == foldRegistryHost(host)
}
