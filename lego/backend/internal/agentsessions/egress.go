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

package agentsessions

import (
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
)

// createEgress resolves the session's model endpoint for workspaceID (whose
// model route, if any, decides it now) and the session's tenant widening: the
// hosts the workspace network tier contributes (workspaceHosts, already
// validated when the environment was saved) merged with the request's explicit
// egressAllowlist. The request list is validated on its own first, so its
// errors keep naming the caller's entry; the merged list is capped at the same
// 32 destinations. The package-registry catalog is not part of the widening:
// sessionegress renders it for the registries and custom tiers, in both
// phases (the full tier renders no FQDN rule at all).
func createEgress(workspaceID string, config AgentConfig, workspaceHosts, requested []string) (string, []string, error) {
	endpoint, err := agentsession.RegisteredModelEndpoint(workspaceID, config.Agent, config.ModelEndpoint)
	if err != nil {
		return "", nil, err
	}
	if _, err := sessionegress.ModelEndpointHost(endpoint); err != nil {
		return "", nil, err
	}
	allowlist, err := sessionegress.ExtraDestinations(requested)
	if err != nil {
		return "", nil, err
	}
	merged := append(slices.Clone(workspaceHosts), allowlist...)
	slices.Sort(merged)
	allowlist, err = sessionegress.ExtraDestinations(slices.Compact(merged))
	if err != nil {
		return "", nil, err
	}
	return endpoint, allowlist, nil
}
