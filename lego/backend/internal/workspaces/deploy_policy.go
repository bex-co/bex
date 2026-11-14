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

package workspaces

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A workspace's Overlapping Deploy Policy (w4/653): Render's Workspace
// Settings → Overlapping Deploy Policy (render.com/docs/deploys). Wait, the
// default for workspaces created on or after 2025-07-14 and so for every bex
// workspace, lets the deploy in progress finish and then runs the most recently
// triggered one; Override cancels the deploy in progress and starts the new
// one. Render's public API and MCP neither read nor write it (its OpenAPI owner
// has no such field), so it is GraphQL and dashboard only (parity by absence).
// Every member reads it, as on the settings page; Admins change it.

type deployPolicyStore interface {
	OverlappingDeployPolicy(ctx context.Context, workspaceID string) (string, error)
	SetOverlappingDeployPolicy(ctx context.Context, workspaceID, policy string) error
}

// overlappingDeployPolicy is w's policy, read only when a surface selects
// it, and refused, as the inbound IP rules are, to a member the workspace's
// 2FA requirement blocks. A store without the setting runs Wait, as bex did
// before it.
func (s *Service) overlappingDeployPolicy(ctx context.Context, w WorkspaceView) (string, error) {
	// Like the inbound IP rules, this is workspace configuration the 2FA-exempt
	// workspaces list must not hand a member the workspace blocks (w22/m402).
	if w.RequireTwoFactor {
		if err := s.CheckTwoFactor(ctx, w.ID); err != nil {
			return "", err
		}
	}
	if w.OverlappingDeployPolicy != "" {
		return w.OverlappingDeployPolicy, nil
	}
	st, ok := s.Store.(deployPolicyStore)
	if !ok {
		return store.OverlapWait, nil
	}
	policy, err := st.OverlappingDeployPolicy(ctx, w.ID)
	return policy, mapStoreErr(err)
}

// SetOverlappingDeployPolicy sets a workspace's Overlapping Deploy Policy,
// "wait" or "override". Admin only (can_manage on the exact workspace). It
// applies to the next trigger; the operator follows within one projector pass.
func (s *Service) SetOverlappingDeployPolicy(ctx context.Context, ownerID, policy string) (WorkspaceView, error) {
	ownerID, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, ownerID)
	if err != nil {
		return WorkspaceView{}, err
	}
	if policy != store.OverlapWait && policy != store.OverlapOverride {
		return WorkspaceView{}, core.NewBadRequestError("INVALID_OVERLAPPING_DEPLOY_POLICY",
			`overlappingDeployPolicy must be "wait" or "override"`, map[string]any{"policy": policy})
	}
	st, ok := s.Store.(deployPolicyStore)
	if !ok {
		return WorkspaceView{}, core.ErrWorkspacesUnavailable
	}
	if err := st.SetOverlappingDeployPolicy(ctx, ownerID, policy); err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	t, err := s.Store.GetTenant(ctx, ownerID)
	if err != nil {
		return WorkspaceView{}, mapStoreErr(err)
	}
	v := view(t, "admin")
	v.OverlappingDeployPolicy = policy
	return v, nil
}
