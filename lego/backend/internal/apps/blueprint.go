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

package apps

// blueprint.go adds the /blueprints surface (w2/m15): validate · list · sync.
// Blueprints are bex.yml stack sources — repo+branch+manifest records created
// automatically when deploy is called with a repo. The validate verb is
// stateless (no store); list/sync require the control-plane store.

import (
	"context"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// BlueprintStore is the persistence interface blueprints need.
// *store.PGStore satisfies it structurally.
type BlueprintStore interface {
	UpsertBlueprint(ctx context.Context, b store.Blueprint) (store.Blueprint, error)
	GetBlueprint(ctx context.Context, id, tenantID string) (store.Blueprint, error)
	ListBlueprints(ctx context.Context, tenantID string) ([]store.Blueprint, error)
}

// ErrBlueprintsUnavailable is returned when the control-plane store is not wired
// (BEX_CP_DB_URI unset). Validate is always available; list/sync need the store.
var ErrBlueprintsUnavailable = errors.New("blueprints store not configured (BEX_CP_DB_URI required)")

// BlueprintView is the API shape for a blueprint — all three surfaces return this.
type BlueprintView struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BlueprintValidation is the result of a dry-run validate — no storage, no apply.
type BlueprintValidation struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// SyncBlueprintResult is the result of a sync: the updated blueprint metadata
// plus the full stack result (services + databases converged by re-apply).
type SyncBlueprintResult struct {
	Blueprint BlueprintView `json:"blueprint"`
	Stack     StackResult   `json:"stack"`
}

// ValidateBlueprint parses a bex.yml and returns per-entry errors without
// applying anything (stateless: no store, no k8s writes). Requires can_view.
func (s *Service) ValidateBlueprint(ctx context.Context, bexYAML string) (BlueprintValidation, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return BlueprintValidation{}, err
	}
	_, err := parseStack(DeployRequest{Manifest: bexYAML})
	if err == nil {
		return BlueprintValidation{Valid: true}, nil
	}
	if !errors.Is(err, core.ErrBadRequest) {
		return BlueprintValidation{}, err
	}
	msg := err.Error()
	if after, ok := strings.CutPrefix(msg, "bad request: "); ok {
		msg = after
	}
	return BlueprintValidation{Errors: []string{msg}}, nil
}

// ListBlueprints returns all active blueprints for a workspace, newest first.
// ownerID is optional (Render's ownerId): empty resolves to the caller's default
// workspace (the same contract apps.List has).
func (s *Service) ListBlueprints(ctx context.Context, ownerID string) ([]BlueprintView, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Blueprints == nil {
		return nil, ErrBlueprintsUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	if tenantID == "" {
		return []BlueprintView{}, nil
	}
	bs, err := s.Blueprints.ListBlueprints(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]BlueprintView, len(bs))
	for i, b := range bs {
		views[i] = toBlueprintView(b)
	}
	return views, nil
}

// SyncBlueprint re-applies the stored blueprint by id, optionally replacing its
// manifest first. Idempotent: an unchanged bex.yml is a no-op re-apply.
// ownerID scopes the fetch to the caller's workspace (prevents cross-workspace
// sync). If bexYAML is non-empty the stored manifest is replaced before apply.
func (s *Service) SyncBlueprint(ctx context.Context, id, ownerID, bexYAML string) (SyncBlueprintResult, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return SyncBlueprintResult{}, err
	}
	if s.Blueprints == nil {
		return SyncBlueprintResult{}, ErrBlueprintsUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, id, tenantID)
	if err != nil {
		return SyncBlueprintResult{}, err
	}
	if bexYAML != "" {
		b.Manifest = bexYAML
		b, err = s.Blueprints.UpsertBlueprint(ctx, b)
		if err != nil {
			return SyncBlueprintResult{}, err
		}
	}
	stack, err := s.deployStack(ctx, DeployRequest{
		Repo:     b.Repo,
		Branch:   b.Branch,
		Manifest: b.Manifest,
	})
	if err != nil {
		return SyncBlueprintResult{}, err
	}
	return SyncBlueprintResult{Blueprint: toBlueprintView(b), Stack: stack}, nil
}

// upsertBlueprint persists a blueprint row after a successful deployStack call
// (called from deployStack when Blueprints store is set and req.Repo is non-empty).
// Errors are logged but not surfaced — the deploy already succeeded.
func (s *Service) upsertBlueprint(ctx context.Context, req DeployRequest) {
	if s.Blueprints == nil || req.Repo == "" {
		return
	}
	tenantID := s.resolveTenantID(ctx)
	if tenantID == "" {
		return
	}
	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	name := repoName(req.Repo)
	_, _ = s.Blueprints.UpsertBlueprint(ctx, store.Blueprint{
		TenantID: tenantID,
		Name:     name,
		Repo:     req.Repo,
		Branch:   branch,
		Manifest: req.Manifest,
		Status:   "active",
	})
}

// resolveTenantID returns the effective tenant id after Authorize has already
// validated workspace membership. It mirrors resolveWorkspace's precedence:
// an explicitly named workspace wins; otherwise the caller's default resolves.
func (s *Service) resolveTenantID(ctx context.Context) string {
	if named, ok := core.WorkspaceFrom(ctx); ok && named != "" {
		return named
	}
	if s.Workspace == nil {
		return ""
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return ""
	}
	tenantID, _ := s.Workspace.Tenant(ctx, id)
	return tenantID
}

// repoName extracts a human-friendly name from a repo URL — the last path
// component, stripping the ".git" suffix if present.
func repoName(repo string) string {
	base := path.Base(strings.TrimRight(repo, "/"))
	return strings.TrimSuffix(base, ".git")
}

func toBlueprintView(b store.Blueprint) BlueprintView {
	return BlueprintView{
		ID:        b.ID,
		Name:      b.Name,
		Repo:      b.Repo,
		Branch:    b.Branch,
		Status:    b.Status,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}
