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
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"sigs.k8s.io/yaml"
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
	Manifest  string    `json:"manifest"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// BlueprintValidationError is Render's validation-error shape. Error is always
// present; line/column/path are best-effort source locations and remain absent
// when the YAML decoder cannot identify one precisely.
type BlueprintValidationError struct {
	Error  string  `json:"error"`
	Line   *int    `json:"line,omitempty"`
	Column *int    `json:"column,omitempty"`
	Path   *string `json:"path,omitempty"`
}

// BlueprintValidationPlan is Render's dry-run resource summary for a Blueprint
// dry-run: services, managed Postgres databases, key-value stores, and env groups.
type BlueprintValidationPlan struct {
	Services     []string `json:"services,omitempty"`
	Databases    []string `json:"databases,omitempty"`
	KeyValue     []string `json:"keyValue,omitempty"`
	EnvGroups    []string `json:"envGroups,omitempty"`
	TotalActions int      `json:"totalActions"`
}

// BlueprintValidation is the result of a dry-run validate — no storage, no
// apply. Its JSON representation matches Render's ValidateBlueprintResponse so
// the official CLI can decode it directly.
type BlueprintValidation struct {
	Valid  bool                       `json:"valid"`
	Errors []BlueprintValidationError `json:"errors,omitempty"`
	Plan   *BlueprintValidationPlan   `json:"plan,omitempty"`
}

// SyncBlueprintResult is the result of a sync: the updated blueprint metadata
// plus the full stack result (services + databases converged by re-apply).
type SyncBlueprintResult struct {
	Blueprint BlueprintView `json:"blueprint"`
	Stack     StackResult   `json:"stack"`
}

// ValidateBlueprint parses a bex.yml and returns per-entry errors without
// applying anything (stateless: no store, no k8s writes). ownerID is Render's
// workspace selector; empty resolves to the caller's default. Requires can_view.
func (s *Service) ValidateBlueprint(ctx context.Context, ownerID, bexYAML string) (BlueprintValidation, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return BlueprintValidation{}, err
	}
	st, err := parseStack(DeployRequest{Manifest: bexYAML})
	if err == nil {
		err = s.validateBlueprintServices(st)
	}
	if err == nil {
		plan := blueprintValidationPlan(st)
		return BlueprintValidation{Valid: true, Plan: &plan}, nil
	}
	if !errors.Is(err, core.ErrBadRequest) {
		return BlueprintValidation{}, err
	}
	msg := err.Error()
	if after, ok := strings.CutPrefix(msg, "bad request: "); ok {
		msg = after
	}
	return BlueprintValidation{Errors: []BlueprintValidationError{blueprintValidationError(bexYAML, msg)}}, nil
}

func blueprintValidationPlan(st parsedStack) BlueprintValidationPlan {
	plan := BlueprintValidationPlan{
		Services:  make([]string, 0, len(st.services)),
		Databases: make([]string, 0, len(st.databases)),
		KeyValue:  make([]string, 0, len(st.keyValues)),
		EnvGroups: make([]string, 0, len(st.envGroups)),
	}
	for _, svc := range st.services {
		plan.Services = append(plan.Services, svc.req.Name)
	}
	for _, db := range st.databases {
		plan.Databases = append(plan.Databases, db.name)
	}
	for _, kv := range st.keyValues {
		plan.KeyValue = append(plan.KeyValue, kv.name)
	}
	for _, group := range st.envGroups {
		plan.EnvGroups = append(plan.EnvGroups, group.name)
	}
	plan.TotalActions = len(plan.Services) + len(plan.Databases) + len(plan.KeyValue) + len(plan.EnvGroups)
	return plan
}

var yamlLineRE = regexp.MustCompile(`(?i)\bline\s+(\d+)\b`)

func blueprintValidationError(manifest, message string) BlueprintValidationError {
	out := BlueprintValidationError{Error: message}
	if match := yamlLineRE.FindStringSubmatch(message); len(match) == 2 {
		if line, err := strconv.Atoi(match[1]); err == nil && line > 0 {
			out.Line = &line
		}
	}
	if fieldPath := blueprintErrorPath(manifest, message); fieldPath != "" {
		out.Path = &fieldPath
	}
	return out
}

// blueprintErrorPath supplies the optional Render path for semantic errors the
// shared parser can associate with a named entry. Syntax errors already carry a
// decoder line; an uncertain path is omitted rather than fabricated.
func blueprintErrorPath(manifest, message string) string {
	var parsed bexManifest
	if err := yaml.Unmarshal([]byte(manifest), &parsed); err != nil {
		return ""
	}
	services := parsed.Services
	for i, svc := range services {
		if svc.Name == "" {
			if strings.Contains(message, "service entry is missing its name") {
				return fmt.Sprintf("services[%d].name", i)
			}
			continue
		}
		if strings.Contains(message, fmt.Sprintf("%q", svc.Name)) || strings.Contains(message, svc.Name+" ") {
			return fmt.Sprintf("services[%d]%s", i, blueprintErrorField(message))
		}
	}
	for i, db := range parsed.Databases {
		if db.Name == "" {
			if strings.Contains(message, "database entry is missing its name") {
				return fmt.Sprintf("databases[%d].name", i)
			}
			continue
		}
		if strings.Contains(message, fmt.Sprintf("%q", db.Name)) || strings.Contains(message, db.Name+" ") {
			return fmt.Sprintf("databases[%d]%s", i, blueprintErrorField(message))
		}
	}
	for i, group := range parsed.EnvVarGroups {
		if group.Name == "" {
			if strings.Contains(message, "envVarGroups entry is missing its name") {
				return fmt.Sprintf("envVarGroups[%d].name", i)
			}
			continue
		}
		if strings.Contains(message, fmt.Sprintf("%q", group.Name)) || strings.Contains(message, group.Name+" ") {
			return fmt.Sprintf("envVarGroups[%d]%s", i, blueprintErrorField(message))
		}
	}
	return ""
}

func blueprintErrorField(message string) string {
	for _, field := range []string{"maintenanceMode", "plan", "domains", "schedule", "runtime", "type", "image", "name"} {
		if strings.Contains(strings.ToLower(message), strings.ToLower(field)) {
			return "." + field
		}
	}
	if strings.Contains(message, " env ") || strings.Contains(message, "env var") {
		return ".envVars"
	}
	return ""
}

// GetBlueprintByID returns a single blueprint by its opaque id, scoped to the
// caller's workspace (prevents cross-workspace reads). ownerID is optional.
func (s *Service) GetBlueprintByID(ctx context.Context, id, ownerID string) (BlueprintView, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return BlueprintView{}, err
	}
	if s.Blueprints == nil {
		return BlueprintView{}, ErrBlueprintsUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, id, tenantID)
	if err != nil {
		return BlueprintView{}, err
	}
	return toBlueprintView(b), nil
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
// confirm carries the exact protected-environment phrase for an existing
// service override; empty preserves the ordinary unprotected path.
func (s *Service) SyncBlueprint(ctx context.Context, id, ownerID, bexYAML, confirm string) (SyncBlueprintResult, error) {
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
		Confirm:  confirm,
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

// GetBlueprint returns a single blueprint by id, scoped to the caller's workspace.
// ownerID is optional — empty resolves to the caller's default workspace.
func (s *Service) GetBlueprint(ctx context.Context, id, ownerID string) (BlueprintView, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return BlueprintView{}, err
	}
	if s.Blueprints == nil {
		return BlueprintView{}, ErrBlueprintsUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, id, tenantID)
	if err != nil {
		return BlueprintView{}, err
	}
	return toBlueprintView(b), nil
}

func toBlueprintView(b store.Blueprint) BlueprintView {
	return BlueprintView{
		ID:        b.ID,
		Name:      b.Name,
		Repo:      b.Repo,
		Branch:    b.Branch,
		Manifest:  b.Manifest,
		Status:    b.Status,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}
