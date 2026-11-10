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
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Sandbox groups (w22/m311, docs/ADR042-sandbox-cluster-substrate.md
// § Sandbox groups). Every Render sandbox belongs to a group; a group carries a
// concurrency limit, and a sandbox create or resume that would exceed it
// answers 429 (admission.go).

const (
	// RenderMaxSandboxGroups is how many groups Render lets a workspace have
	// (render.com/docs/sandboxes-limits, read 2026-11-10). bex's cap is
	// configurable (BEX_SANDBOX_MAX_GROUPS_PER_WORKSPACE); this is its default.
	RenderMaxSandboxGroups = 10
	// RenderMaxGroupConcurrency is Render's ceiling and default for a group's
	// concurrencyLimit: a higher value is capped, never refused. bex's ceiling is
	// configurable (BEX_SANDBOX_MAX_GROUP_CONCURRENCY); this is its default.
	RenderMaxGroupConcurrency = 1000

	maxGroupName = 100
)

// Codes. Where Render's own errorCode vocabulary names the refusal, bex answers
// with that code; the others are bex's, because Render documents none.
const (
	// CodeInvalidSandboxGroupID is Render's `invalid_sandbox_group_id`: an id that
	// is not a well-formed `sbg-` identifier.
	CodeInvalidSandboxGroupID = "invalid_sandbox_group_id"
	// CodeSandboxGroupLimitReached is Render's `sandbox_group_limit_reached`, a 400:
	// "a sandbox group create, or a sandbox create that needs a new default group,
	// for an owner already at its sandbox group limit" (pinned OpenAPI errorCode).
	CodeSandboxGroupLimitReached = "sandbox_group_limit_reached"
	// CodeSandboxGroupNotFound is a group id that names no group in the workspace.
	CodeSandboxGroupNotFound = "SANDBOX_GROUP_NOT_FOUND"
	// CodeSandboxGroupNameTaken is Render's 409 for a name another group has;
	// Render documents no code for it.
	CodeSandboxGroupNameTaken = "SANDBOX_GROUP_NAME_TAKEN"
	// CodeSandboxGroupNameInvalid refuses an empty, padded, over-long or
	// control-character group name.
	CodeSandboxGroupNameInvalid = "SANDBOX_GROUP_NAME_INVALID"
	// CodeSandboxGroupLimitInvalid refuses a concurrencyLimit below 1 or absent.
	CodeSandboxGroupLimitInvalid = "SANDBOX_GROUP_LIMIT_INVALID"
	// CodeSandboxGroupRegionInvalid refuses a region other than Render's one.
	CodeSandboxGroupRegionInvalid = "SANDBOX_REGION_INVALID"
)

// GroupStore is where sandbox groups and each sandbox's group live;
// *store.PGStore implements it. nil (store off) serves no group route, refuses
// a create that names a group, and enforces no concurrency limit.
type GroupStore interface {
	CreateSandboxGroup(ctx context.Context, workspaceID, name, region string, concurrencyLimit, maxGroups int) (store.SandboxGroup, error)
	EnsureDefaultSandboxGroup(ctx context.Context, workspaceID string, concurrencyLimit int) (store.SandboxGroup, error)
	SandboxGroups(ctx context.Context, workspaceID string) ([]store.SandboxGroup, error)
	SandboxGroup(ctx context.Context, workspaceID, id string) (store.SandboxGroup, error)
	SandboxGroupWorkspace(ctx context.Context, id string) (string, error)
	SetSandboxGroupLimit(ctx context.Context, workspaceID, id string, limit int) (store.SandboxGroup, error)
	SandboxGroupMembers(ctx context.Context, workspaceID string) (map[string]store.SandboxGroupMember, error)
	WithSandboxAdmission(ctx context.Context, workspaceID string, fn func(store.SandboxAdmission) error) error
	SetSandboxSlotLease(ctx context.Context, workspaceID, sandboxID string, until *time.Time) error
	ReleaseSandboxSlot(ctx context.Context, workspaceID, sandboxID string) error
	PruneSandboxGroupMembers(ctx context.Context, workspaceID string, live []string) error
}

// Group is Render's sandbox group, the shape of the pinned OpenAPI schema.
// EnvironmentID is always null: bex does not bind a group to a project
// environment (ADR042 § Sandbox groups).
type Group struct {
	ID            string  `json:"id"`
	OwnerID       string  `json:"ownerId"`
	Name          string  `json:"name"`
	Region        string  `json:"region"`
	IsDefault     bool    `json:"isDefault"`
	EnvironmentID *string `json:"environmentId"`
	// ConcurrencyLimit is the effective limit on active sandboxes: the configured
	// value capped by the platform's ceiling.
	ConcurrencyLimit int       `json:"concurrencyLimit"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// GroupEnvelope wraps a Group for Render's list surface, a cursor-paginated
// `[{sandboxGroup, cursor}]` array.
type GroupEnvelope struct {
	SandboxGroup Group  `json:"sandboxGroup"`
	Cursor       string `json:"cursor"`
}

// CreateGroupRequest is the caller's group create input. OwnerID binds the
// workspace; an empty Region is Render's default, oregon.
type CreateGroupRequest struct {
	OwnerID string
	Name    string
	Region  string
}

// CodeSandboxGroupsUnavailable is the 503 of a deployment with no control-plane
// store, where groups and their limits cannot be kept.
const CodeSandboxGroupsUnavailable = "SANDBOX_GROUPS_UNAVAILABLE"

func (s *Service) requireGroups() error {
	if s.Groups == nil {
		return core.NewUnavailableError(CodeSandboxGroupsUnavailable, "sandbox groups need the control-plane store", nil)
	}
	return nil
}

// groupConcurrencyCeiling is the most a group's limit can be (0: unbounded).
func (s *Service) groupConcurrencyCeiling() int { return s.MaxGroupConcurrency }

// newGroupLimit is the limit a group starts with: Render's 1,000, or the
// ceiling when that is lower.
func (s *Service) newGroupLimit() int {
	if c := s.groupConcurrencyCeiling(); c > 0 {
		return min(RenderMaxGroupConcurrency, c)
	}
	return RenderMaxGroupConcurrency
}

// clampLimit applies Render's rule that a concurrencyLimit above the ceiling is
// capped, not refused ("capped by Render's limit for this workspace").
func (s *Service) clampLimit(limit int) int {
	if c := s.groupConcurrencyCeiling(); c > 0 {
		return min(limit, c)
	}
	return min(limit, 1<<31-1)
}

// effectiveLimit is the limit a group is held to: its configured value capped by
// the ceiling, so lowering the ceiling lowers every group without a rewrite.
func (s *Service) effectiveLimit(g store.SandboxGroup) int {
	if c := s.groupConcurrencyCeiling(); c > 0 {
		return min(g.ConcurrencyLimit, c)
	}
	return g.ConcurrencyLimit
}

func (s *Service) groupView(g store.SandboxGroup) Group {
	return Group{
		ID: g.ID, OwnerID: g.WorkspaceID, Name: g.Name, Region: g.Region, IsDefault: g.IsDefault,
		ConcurrencyLimit: s.effectiveLimit(g), CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

func validateGroupID(id string) error {
	if kind, ok := ids.KindOf(id); ok && kind == ids.SandboxGroup {
		return nil
	}
	return core.NewBadRequestError(CodeInvalidSandboxGroupID,
		fmt.Sprintf("%q is not a well-formed sandbox group id; they look like %s-…", id, ids.SandboxGroup.Prefix()),
		map[string]any{"field": "sandboxGroupId"})
}

func groupNotFound(id string) error {
	return core.NewNotFoundError(CodeSandboxGroupNotFound, "sandbox group not found", map[string]any{"id": id})
}

// validateGroupName is the group name rule: Render documents only that a name is
// unique within the workspace, so bex asks for a non-empty, unpadded label with
// no control characters, at most 100 characters.
func validateGroupName(name string) error {
	bad := name == "" || strings.TrimSpace(name) != name || utf8.RuneCountInString(name) > maxGroupName ||
		strings.ContainsFunc(name, func(r rune) bool { return unicode.IsControl(r) || r == ' ' || r == ' ' })
	if !bad {
		return nil
	}
	return core.NewBadRequestError(CodeSandboxGroupNameInvalid,
		fmt.Sprintf("sandbox group name must be 1 to %d characters, with no leading or trailing space and no control characters", maxGroupName),
		map[string]any{"field": "name", "maxLength": maxGroupName})
}

// groupRegion resolves a group's region: Render has one, oregon, and the group
// carries it as a compatibility value that does not affect placement.
func groupRegion(region string) (string, error) {
	if region == "" || region == store.SandboxGroupRegion {
		return store.SandboxGroupRegion, nil
	}
	return "", core.NewBadRequestError(CodeSandboxGroupRegionInvalid,
		fmt.Sprintf("sandbox groups run in %s only, got %q", store.SandboxGroupRegion, region),
		map[string]any{"field": "region", "supported": []string{store.SandboxGroupRegion}})
}

// CreateGroup adds a group to the caller's workspace. The workspace's first
// group becomes its default.
func (s *Service) CreateGroup(ctx context.Context, req CreateGroupRequest) (Group, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Group{}, err
	}
	if err := s.requireGroups(); err != nil {
		return Group{}, err
	}
	if err := validateGroupName(req.Name); err != nil {
		return Group{}, err
	}
	region, err := groupRegion(req.Region)
	if err != nil {
		return Group{}, err
	}
	g, err := s.Groups.CreateSandboxGroup(ctx, s.WorkspaceOrDefault(ctx), req.Name, region, s.newGroupLimit(), s.MaxGroups)
	switch {
	case errors.Is(err, store.ErrSandboxGroupNameTaken):
		return Group{}, core.NewConflictError(CodeSandboxGroupNameTaken,
			fmt.Sprintf("a sandbox group named %q already exists in this workspace", req.Name), map[string]any{"field": "name"})
	case errors.Is(err, store.ErrSandboxGroupLimit):
		return Group{}, s.groupLimitReached()
	case err != nil:
		return Group{}, store.MapError(err)
	}
	return s.groupView(g), nil
}

// groupLimitReached is Render's `sandbox_group_limit_reached` refusal.
func (s *Service) groupLimitReached() error {
	return core.NewBadRequestError(CodeSandboxGroupLimitReached,
		fmt.Sprintf("this workspace is at its limit of %d sandbox groups", s.MaxGroups),
		map[string]any{"limit": s.MaxGroups})
}

// ListGroups returns the caller's workspace's groups, newest first, one page of
// at most limit after the cursor (Render's default of 20 when limit is zero).
func (s *Service) ListGroups(ctx context.Context, after string, limit int) ([]Group, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if err := s.requireGroups(); err != nil {
		return nil, err
	}
	rows, err := s.Groups.SandboxGroups(ctx, s.WorkspaceOrDefault(ctx))
	if err != nil {
		return nil, store.MapError(err)
	}
	groups := make([]Group, 0, len(rows))
	for _, g := range rows {
		groups = append(groups, s.groupView(g))
	}
	return core.Page(groups, after, core.PageLimitOrDefault(limit), func(g Group) string { return g.ID }), nil
}

// GetGroup returns one group. With no workspace named it acts in the group's
// own workspace when the caller is a member there; a group the caller cannot
// see answers the same not-found as one that does not exist.
func (s *Service) GetGroup(ctx context.Context, id string) (Group, error) {
	ctx, err := s.scopeGroup(ctx, id)
	if err != nil {
		return Group{}, err
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Group{}, err
	}
	g, err := s.storedGroup(ctx, id)
	if err != nil {
		return Group{}, err
	}
	return s.groupView(g), nil
}

// UpdateGroup sets a group's concurrency limit and returns the group with its
// effective limit. A value above the ceiling is capped, not refused, and
// lowering the limit stops no running sandbox: the group simply admits nothing
// new until its active count falls below the limit.
func (s *Service) UpdateGroup(ctx context.Context, id string, concurrencyLimit int) (Group, error) {
	ctx, err := s.scopeGroup(ctx, id)
	if err != nil {
		return Group{}, err
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Group{}, err
	}
	if err := s.requireGroups(); err != nil {
		return Group{}, err
	}
	if err := validateGroupID(id); err != nil {
		return Group{}, err
	}
	if concurrencyLimit < 1 {
		return Group{}, core.NewBadRequestError(CodeSandboxGroupLimitInvalid,
			"concurrencyLimit is required and must be at least 1", map[string]any{"field": "concurrencyLimit", "minimum": 1})
	}
	g, err := s.Groups.SetSandboxGroupLimit(ctx, s.WorkspaceOrDefault(ctx), id, s.clampLimit(concurrencyLimit))
	if errors.Is(err, store.ErrNotFound) {
		return Group{}, groupNotFound(id)
	}
	if err != nil {
		return Group{}, store.MapError(err)
	}
	return s.groupView(g), nil
}

// storedGroup loads one group of the request's workspace after the caller was
// authorized there.
func (s *Service) storedGroup(ctx context.Context, id string) (store.SandboxGroup, error) {
	if err := s.requireGroups(); err != nil {
		return store.SandboxGroup{}, err
	}
	if err := validateGroupID(id); err != nil {
		return store.SandboxGroup{}, err
	}
	g, err := s.Groups.SandboxGroup(ctx, s.WorkspaceOrDefault(ctx), id)
	if errors.Is(err, store.ErrNotFound) {
		return store.SandboxGroup{}, groupNotFound(id)
	}
	if err != nil {
		return store.SandboxGroup{}, store.MapError(err)
	}
	return g, nil
}

// scopeGroup resolves the workspace a by-id group verb acts in: an explicit
// workspace decides as before, otherwise the group's own does, so a member of
// several workspaces reaches a group by id as Render's by-id routes (which take
// no owner) allow. A group held in no workspace the caller belongs to, like one
// that does not exist, stays on the default path and answers not-found.
func (s *Service) scopeGroup(ctx context.Context, id string) (context.Context, error) {
	return s.ScopeByVisibleID(ctx, "", s.groupOwner(id))
}

// groupOwner is the unscoped routing read behind scopeGroup. It only routes:
// the verb still authorizes and loads the group in the workspace it picks.
func (s *Service) groupOwner(id string) core.ResourceOwner {
	return func(ctx context.Context) ([]string, error) {
		if s.Groups == nil || validateGroupID(id) != nil {
			return nil, nil
		}
		return store.OwnerWorkspaces(s.Groups.SandboxGroupWorkspace(ctx, id))
	}
}

// registerGroupREST mounts Render's four sandbox-group routes.
func (s *Service) registerGroupREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sandbox-groups", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OwnerID string `json:"ownerId"`
			Name    string `json:"name"`
			Region  string `json:"region"`
		}
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &body); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		owner := r.URL.Query().Get("ownerId")
		if owner == "" {
			owner = body.OwnerID
		}
		g, err := s.CreateGroup(r.Context(), CreateGroupRequest{OwnerID: owner, Name: body.Name, Region: body.Region})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, g)
	})
	mux.HandleFunc("GET /v1/sandbox-groups", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		after, limit := core.PageParams(query)
		groups, err := s.ListGroups(core.WithWorkspace(r.Context(), query.Get("ownerId")), after, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		envelopes := make([]GroupEnvelope, 0, len(groups))
		for _, g := range groups {
			envelopes = append(envelopes, GroupEnvelope{SandboxGroup: g, Cursor: g.ID})
		}
		core.WriteJSON(w, http.StatusOK, envelopes)
	})
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}", func(w http.ResponseWriter, r *http.Request) {
		g, err := s.GetGroup(r.Context(), r.PathValue("sandboxGroupId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, g)
	})
	mux.HandleFunc("PATCH /v1/sandbox-groups/{sandboxGroupId}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			// A pointer so an absent value is refused, not read as 0.
			ConcurrencyLimit *int `json:"concurrencyLimit"`
		}
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &body); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		limit := 0
		if body.ConcurrencyLimit != nil {
			limit = *body.ConcurrencyLimit
		}
		g, err := s.UpdateGroup(r.Context(), r.PathValue("sandboxGroupId"), limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, g)
	})
}
