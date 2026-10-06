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

package sandbox

import (
	"context"
	"errors"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// scopeSandbox resolves the workspace a by-id sandbox verb acts in (w4/m172):
// an explicit ownerID or a workspace the request already names decides as
// before; otherwise the sandbox's OWN workspace does, so a member of several
// workspaces reaches a sandbox by id without ?ownerId= (Render's by-id
// sandbox endpoints take no owner). A non-member's id — like a nonexistent
// one — answers the same SANDBOX_NOT_FOUND: ownership is only discoverable in
// the caller's own workspaces (ADR072's by-id matrix exemption).
func (s *Service) scopeSandbox(ctx context.Context, ownerID, id string) (context.Context, error) {
	return s.ScopeByVisibleID(ctx, ownerID, s.sandboxOwner(id))
}

// sandboxOwner is the unscoped routing read behind scopeSandbox. bex keeps no
// sandbox table (OpenSandbox is the source of truth, ADR042 D4), so it probes
// the caller's member workspaces through each one's EXISTING tenant key —
// looked up, never minted (a workspace with no key has never created a
// sandbox) — default first, stopping at the first whose namespace holds the id
// with matching workspace metadata: a sandbox in the caller's default never
// depends on another member namespace answering. It only routes:
// ownedSandbox still applies the owner/admin boundary in the chosen workspace.
// Anything it cannot decide (no membership listing) leaves the verb on its
// default-workspace path.
func (s *Service) sandboxOwner(id string) core.ResourceOwner {
	return func(ctx context.Context) ([]string, error) {
		if id == "" || !s.enabled() || s.MemberWorkspaceIDs == nil {
			return nil, nil
		}
		caller, ok := core.IdentityFrom(ctx)
		if !ok {
			return nil, nil
		}
		members, err := s.MemberWorkspaceIDs(ctx, caller)
		if err != nil || len(members) == 0 {
			return nil, err
		}
		def, _ := s.Workspace.Tenant(ctx, caller)
		if len(members) == 1 && members[0] == def {
			return nil, nil // the verb's default path already looks there
		}
		if s.Keys == nil {
			// Single-tenant OpenSandbox: one keyless namespace holds every
			// workspace's sandboxes, so one read names the owner.
			ws, found, err := s.probeSandbox(ctx, "", id)
			if err != nil || !found || !slices.Contains(members, ws) {
				return nil, err
			}
			return []string{ws}, nil
		}
		if i := slices.Index(members, def); i > 0 {
			members = slices.Concat([]string{def}, members[:i], members[i+1:])
		}
		for _, member := range members {
			key, minted, err := s.Keys.SandboxKeyLookup(ctx, member)
			if err != nil {
				return nil, err
			}
			if !minted {
				continue
			}
			ws, found, err := s.probeSandbox(ctx, key, id)
			if err != nil {
				return nil, err
			}
			if found && ws == member {
				return []string{member}, nil
			}
		}
		return nil, nil
	}
}

// probeSandbox reads one sandbox's workspace metadata through tenant key.
func (s *Service) probeSandbox(ctx context.Context, key, id string) (string, bool, error) {
	raw, err := s.resolveSandbox(ctx, key, id)
	if errors.Is(err, errOpenSandboxNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	ws := raw.Metadata[metadataWorkspace]
	return ws, ws != "", nil
}
