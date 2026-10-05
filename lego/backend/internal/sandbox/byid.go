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
// one — answers the same SANDBOX_NOT_FOUND.
func (s *Service) scopeSandbox(ctx context.Context, ownerID, id string) (context.Context, error) {
	return s.ScopeByID(ctx, ownerID, s.sandboxOwner(id), sandboxNotFound(id))
}

// sandboxOwner is the unscoped routing read behind scopeSandbox. bex keeps no
// sandbox table (OpenSandbox is the source of truth, ADR042 D4), so it probes
// each of the caller's member workspaces through that workspace's EXISTING
// tenant key — looked up, never minted (a workspace with no key has never
// created a sandbox) — and answers the first whose namespace holds the id with
// matching workspace metadata. It only routes: ownedSandbox still applies the
// owner/admin boundary in the chosen workspace. Anything it cannot decide
// (no membership listing, a key provider without a lookup-only seam) answers
// found=false, which leaves the verb on its default-workspace path.
func (s *Service) sandboxOwner(id string) core.ResourceOwner {
	return func(ctx context.Context) (string, bool, error) {
		if id == "" || !s.enabled() || s.MemberWorkspaceIDs == nil {
			return "", false, nil
		}
		caller, ok := core.IdentityFrom(ctx)
		if !ok {
			return "", false, nil
		}
		members, err := s.MemberWorkspaceIDs(ctx, caller)
		if err != nil || len(members) == 0 {
			return "", false, err
		}
		if s.Keys == nil {
			// Single-tenant OpenSandbox: one keyless namespace holds every
			// workspace's sandboxes, so one read names the owner.
			ws, found, err := s.probeSandbox(ctx, "", id)
			if err != nil || !found || !slices.Contains(members, ws) {
				return "", false, err
			}
			return ws, true, nil
		}
		lookup, ok := s.Keys.(PurgeKeyLookup)
		if !ok {
			return "", false, nil
		}
		for _, member := range members {
			key, minted, err := lookup.SandboxKeyLookup(ctx, member)
			if err != nil {
				return "", false, err
			}
			if !minted {
				continue
			}
			ws, found, err := s.probeSandbox(ctx, key, id)
			if err != nil {
				return "", false, err
			}
			if found && ws == member {
				return member, true, nil
			}
		}
		return "", false, nil
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
