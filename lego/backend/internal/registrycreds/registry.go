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
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// renderRegistries is Render's registryCredentialRegistry enum (w4/420). bex
// stores a registry host, a superset of it: the enum maps onto a host, and a
// host maps back onto the enum when Render has a value for it.
var renderRegistries = []string{"GITHUB", "GITLAB", "DOCKER", "GOOGLE_ARTIFACT", "AWS_ECR"}

// renderRegistryHosts are the enum values with one fixed host. Google
// Artifact Registry and ECR hosts carry a project or account and region, so
// those need an explicit host.
var renderRegistryHosts = map[string]string{
	"DOCKER": "docker.io",
	"GITHUB": "ghcr.io",
	"GITLAB": "registry.gitlab.com",
}

var ecrHostPattern = regexp.MustCompile(`^[0-9]{12}\.dkr\.ecr\.[a-z0-9-]+\.amazonaws\.com$`)

// renderRegistry is the enum value for a stored host, "" for a self-hosted
// registry Render has no value for.
func renderRegistry(host string) string {
	canonical, err := canonicalRegistryHost(host)
	if err != nil {
		return ""
	}
	for registry, fixed := range renderRegistryHosts {
		if canonical == fixed {
			return registry
		}
	}
	switch {
	case canonical == "gcr.io" || strings.HasSuffix(canonical, ".gcr.io") || strings.HasSuffix(canonical, "-docker.pkg.dev"):
		return "GOOGLE_ARTIFACT"
	case ecrHostPattern.MatchString(canonical):
		return "AWS_ECR"
	}
	return ""
}

func checkRenderRegistry(field, registry string) error {
	if !slices.Contains(renderRegistries, registry) {
		return fmt.Errorf("%w: %s must be one of %s (got %q)", core.ErrBadRequest, field, strings.Join(renderRegistries, ", "), registry)
	}
	return nil
}

// createHost resolves a create's registry and host: either alone, or both
// when they agree.
func createHost(registry, host string) (string, error) {
	registry = strings.TrimSpace(registry)
	if registry == "" {
		return host, nil
	}
	if err := checkRenderRegistry("registry", registry); err != nil {
		return "", err
	}
	if strings.TrimSpace(host) == "" {
		if fixed, ok := renderRegistryHosts[registry]; ok {
			return fixed, nil
		}
		return "", fmt.Errorf("%w: registry %s needs host: its registry host names a project or account and region (e.g. us-docker.pkg.dev or 123456789012.dkr.ecr.us-east-1.amazonaws.com)", core.ErrBadRequest, registry)
	}
	if _, err := canonicalRegistryHost(host); err != nil {
		return "", err
	}
	if got := renderRegistry(host); got != registry {
		return "", fmt.Errorf("%w: registry %s does not match host %q", core.ErrBadRequest, registry, strings.TrimSpace(host))
	}
	return host, nil
}
