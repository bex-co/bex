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

// blueprint.go: Git-connected Blueprints (w2/m62, extends w2/m15 + w2/m41).
// Full surface: validate · create · list · get · sync (recorded) · list-syncs ·
// update · disconnect. Auto-sync on push is in webhook.go.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// BlueprintStore is the persistence interface blueprints need.
// *store.PGStore satisfies it structurally.
type BlueprintStore interface {
	UpsertBlueprint(ctx context.Context, b store.Blueprint) (store.Blueprint, error)
	GetBlueprint(ctx context.Context, id, tenantID string) (store.Blueprint, error)
	GetBlueprintByRepo(ctx context.Context, tenantID, repo, branch string) (store.Blueprint, error)
	ListBlueprints(ctx context.Context, tenantID string) ([]store.Blueprint, error)
	UpdateBlueprint(ctx context.Context, id, tenantID string, name *string, autoSync *bool, path *string, status *string, lastSyncAt *time.Time) (store.Blueprint, error)
	DisconnectBlueprint(ctx context.Context, id, tenantID string) error
	InsertBlueprintSync(ctx context.Context, run store.BlueprintSync) (store.BlueprintSync, error)
	UpdateBlueprintSync(ctx context.Context, id, state string, completedAt *time.Time) (store.BlueprintSync, error)
	ListBlueprintSyncs(ctx context.Context, blueprintID, cursor string, limit int) ([]store.BlueprintSync, error)
}

// BlueprintFetcher fetches a blueprint file from its Git repository. The
// github.Service.BlueprintFileFetcher() adapter satisfies it. nil on
// apps.Service (s.GitFetcher) ⇒ create/sync cannot pull-from-repo.
type BlueprintFetcher interface {
	FetchBlueprintFile(ctx context.Context, workspaceID, repoURL, branch, filePath string) (contents, commitSHA string, err error)
}

// ErrBlueprintsUnavailable is returned when the control-plane store is not wired.
var ErrBlueprintsUnavailable = core.Unavailable("blueprints store not configured (BEX_CP_DB_URI required)")

// ErrBlueprintFetchUnavailable is returned when no GitFetcher is configured.
var ErrBlueprintFetchUnavailable = core.Unavailable("blueprint file fetch unavailable (GitHub App not configured)")

// ErrBlueprintSyncWorkspaceUnresolved is returned when a sync cannot name the
// workspace it acts in (w1/m69). Fail closed: an identity-less apply would
// create unlabeled CRs in the shared namespace, invisible to workspace
// lists/purge/quota/billing — the round-15 scan's tenant-attribution break.
var ErrBlueprintSyncWorkspaceUnresolved = core.Unavailable("blueprint sync cannot resolve its acting workspace; refusing to apply")

// ErrBlueprintFilenameAmbiguous is returned only for implicit discovery when a
// repository contains both the canonical and legacy filename. Requiring a path
// avoids choosing a manifest based on a filename accident.
var ErrBlueprintFilenameAmbiguous = errors.New("both render.yaml and legacy bex.yml exist; specify the Blueprint path explicitly")

// CanonicalBlueprintFilename is the public discovery default. LegacyBexYAML is
// retained only as an identical-grammar discovery fallback and for existing
// explicit records; it is never a second dialect.
const (
	CanonicalBlueprintFilename = "render.yaml"
	LegacyBlueprintFilename    = "bex.yml"
)

// discoverBlueprintFile implements ADR049's filename rule. An explicit path is
// fetched as-is. For an implicit request, render.yaml wins only when the
// legacy alias is absent; legacy alone remains compatible; both are an error.
func discoverBlueprintFile(ctx context.Context, fetcher BlueprintFetcher, workspaceID, repo, branch, explicitPath string) (contents, commitSHA, filePath string, err error) {
	if explicitPath != "" {
		explicitPath, err = approvedBlueprintPath(explicitPath)
		if err != nil {
			return "", "", "", err
		}
		contents, commitSHA, err = fetcher.FetchBlueprintFile(ctx, workspaceID, repo, branch, explicitPath)
		return contents, commitSHA, explicitPath, err
	}
	canonicalContents, canonicalSHA, canonicalErr := fetcher.FetchBlueprintFile(ctx, workspaceID, repo, branch, CanonicalBlueprintFilename)
	legacyContents, legacySHA, legacyErr := fetcher.FetchBlueprintFile(ctx, workspaceID, repo, branch, LegacyBlueprintFilename)
	switch {
	case canonicalErr == nil && legacyErr == nil:
		return "", "", "", ErrBlueprintFilenameAmbiguous
	case canonicalErr == nil:
		return canonicalContents, canonicalSHA, CanonicalBlueprintFilename, nil
	case legacyErr == nil:
		return legacyContents, legacySHA, LegacyBlueprintFilename, nil
	default:
		return "", "", "", canonicalErr
	}
}

// approvedBlueprintPath keeps Blueprint discovery from becoming an arbitrary
// private-repository file reader. Since Render's 2026-02-09 custom Blueprint
// paths (w8/m19 t006), an explicit path may use any YAML filename in any
// subdirectory — the containment checks (clean, relative, no escapes) and the
// yaml/yml extension are what bound the readable surface; implicit discovery
// still looks only for render.yaml / the legacy bex.yml alias.
func approvedBlueprintPath(filePath string) (string, error) {
	filePath = strings.TrimSpace(filePath)
	clean := path.Clean(filePath)
	if filePath == "" || clean == "." || clean != filePath || path.IsAbs(clean) ||
		clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, `\`) {
		return "", fmt.Errorf("%w: Blueprint path must be a clean repository-relative path", core.ErrBadRequest)
	}
	switch path.Ext(clean) {
	case ".yaml", ".yml":
		return clean, nil
	default:
		return "", fmt.Errorf("%w: Blueprint path must be a .yaml or .yml file", core.ErrBadRequest)
	}
}

// BlueprintView is the API shape for a blueprint — all three surfaces return this.
type BlueprintView struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Repo      string              `json:"repo"`
	Branch    string              `json:"branch"`
	Path      string              `json:"path"`
	AutoSync  bool                `json:"autoSync"`
	Manifest  string              `json:"manifest"`
	Status    string              `json:"status"`
	LastSync  *string             `json:"lastSync,omitempty"`
	Resources []BlueprintResource `json:"resources,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

// BlueprintResource is one managed resource returned on by-id reads.
type BlueprintResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// BlueprintSyncView is the API shape for a sync run.
type BlueprintSyncView struct {
	ID          string  `json:"id"`
	CommitID    string  `json:"commitId,omitempty"`
	State       string  `json:"state"`
	StartedAt   string  `json:"startedAt"`
	CompletedAt *string `json:"completedAt,omitempty"`
}

// BlueprintValidationError is Render's validation-error shape.
type BlueprintValidationError struct {
	Code   string  `json:"code,omitempty"`
	Error  string  `json:"error"`
	Line   *int    `json:"line,omitempty"`
	Column *int    `json:"column,omitempty"`
	Path   *string `json:"path,omitempty"`
}

// BlueprintValidationPlan is a declaration-only dry-run summary. Validation
// does not resolve the caller's current resources, so TotalActions is the
// number of declared resources—not a create/update/no-op diff.
type BlueprintValidationPlan struct {
	Mode          string                `json:"mode"`
	Services      []string              `json:"services,omitempty"`
	Databases     []string              `json:"databases,omitempty"`
	KeyValue      []string              `json:"keyValue,omitempty"`
	EnvGroups     []string              `json:"envGroups,omitempty"`
	SyncFalseVars []string              `json:"syncFalseVars,omitempty"`
	TotalActions  int                   `json:"totalActions"`
	Actions       []BlueprintPlanAction `json:"actions,omitempty"`
}

// BlueprintValidation is the result of a dry-run validate.
type BlueprintValidation struct {
	Valid  bool                       `json:"valid"`
	Errors []BlueprintValidationError `json:"errors,omitempty"`
	Plan   *BlueprintValidationPlan   `json:"plan,omitempty"`
	// EstimatedPricing is the always-on monthly cost projection for the
	// declared resources, on bex's price sheet (internal/pricing). Present
	// only when the manifest is valid; an all-free stack carries empty lines
	// and a "0.00" total (the dashboard hides the panel).
	EstimatedPricing *pricing.MonthlyEstimate `json:"estimatedPricing,omitempty"`
}

// SyncBlueprintResult is returned by sync and create.
type SyncBlueprintResult struct {
	Blueprint BlueprintView `json:"blueprint"`
	Stack     StackResult   `json:"stack"`
}

// BlueprintPreview is the pre-create dry-run result: the manifest fetched from
// Git plus its validation. A fetch failure (file missing, bad branch) is
// reported in Error with Found=false rather than as a verb error, so the
// dashboard can render Render's "Blueprint file not found on branch" + Retry
// state instead of a toast.
type BlueprintPreview struct {
	Found      bool                 `json:"found"`
	Manifest   string               `json:"manifest,omitempty"`
	CommitID   string               `json:"commitId,omitempty"`
	Warning    string               `json:"warning,omitempty"`
	Error      string               `json:"error,omitempty"`
	Validation *BlueprintValidation `json:"validation,omitempty"`
}

// CreateBlueprintRequest is the input to CreateBlueprint.
type CreateBlueprintRequest struct {
	Repo         string
	Branch       string
	Path         string            // defaults to render.yaml
	Name         string            // defaults to repo basename
	EnvVarValues map[string]string // values for sync:false env vars
	Confirm      string
}

// UpdateBlueprintRequest is the partial-update input to UpdateBlueprint.
type UpdateBlueprintRequest struct {
	Name     *string
	AutoSync *bool
	Path     *string
}

// ValidateBlueprint parses a Render Blueprint and returns per-entry errors without
// applying anything (stateless: no store, no k8s writes). Requires can_view.
func (s *Service) ValidateBlueprint(ctx context.Context, ownerID, bexYAML string) (BlueprintValidation, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return BlueprintValidation{}, err
	}
	return s.blueprintValidationFor(ctx, "", "", bexYAML)
}

// blueprintValidationFor is the stateless dry-run core shared by
// ValidateBlueprint and PreviewBlueprint. repo/branch feed the same parse a
// create would run; both empty for a manifest-only validate.
func (s *Service) blueprintValidationFor(ctx context.Context, repo, branch, bexYAML string) (BlueprintValidation, error) {
	source, ir, problems := CompileBlueprintIR(bexYAML)
	if len(problems) > 0 {
		return BlueprintValidation{Errors: blueprintCompilerValidationErrors(problems)}, nil
	}
	st, err := parseCompiledStack(blueprintParseOverrides{repo: repo, branch: branch}, source, ir)
	if err == nil {
		err = s.resolveBlueprintRegistryCredentials(ctx, &st)
	}
	if err == nil {
		err = s.validateBlueprintServices(ctx, st)
	}
	if err == nil && repo != "" && s.Blueprints != nil {
		// Ownership conflicts surface in the preview/validation result (the
		// dashboard's create review + pre-sync dialog) so nobody discovers
		// them at apply time; the apply-path preflight still enforces.
		if entries := s.previewOwnershipConflicts(ctx, repo, branch, st); len(entries) > 0 {
			return BlueprintValidation{Errors: entries}, nil
		}
	}
	if err == nil {
		plan := blueprintValidationPlanFromIR(ir, st)
		if actionPlan, available, planErr := s.blueprintActionPlan(ctx, ir, st); planErr != nil {
			if !errors.Is(planErr, core.ErrBadRequest) {
				return BlueprintValidation{}, planErr
			}
			msg := strings.TrimPrefix(planErr.Error(), "bad request: ")
			return BlueprintValidation{Errors: []BlueprintValidationError{blueprintValidationError(ir, msg)}}, nil
		} else if available {
			plan.Mode = "current_state"
			plan.Actions = actionPlan.Actions
			plan.TotalActions = len(actionPlan.Actions)
		}
		return BlueprintValidation{Valid: true, Plan: &plan, EstimatedPricing: blueprintEstimatedPricing(st)}, nil
	}
	if !errors.Is(err, core.ErrBadRequest) {
		return BlueprintValidation{}, err
	}
	msg := err.Error()
	if after, ok := strings.CutPrefix(msg, "bad request: "); ok {
		msg = after
	}
	return BlueprintValidation{Errors: []BlueprintValidationError{blueprintValidationError(ir, msg)}}, nil
}

func blueprintCompilerValidationErrors(problems []BlueprintSourceProblem) []BlueprintValidationError {
	errors := make([]BlueprintValidationError, 0, len(problems))
	for _, problem := range problems {
		path := blueprintDisplayPath(problem.Path)
		entry := BlueprintValidationError{Code: problem.Code, Error: problem.Message, Path: &path}
		if problem.Line > 0 {
			line := problem.Line
			entry.Line = &line
		}
		if problem.Column > 0 {
			column := problem.Column
			entry.Column = &column
		}
		errors = append(errors, entry)
	}
	return errors
}

func blueprintDisplayPath(pointer string) string {
	if pointer == "" || pointer == "#" {
		return ""
	}
	pointer = strings.TrimPrefix(pointer, "#/")
	var path string
	for _, segment := range strings.Split(pointer, "/") {
		segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~1", "/"), "~0", "~")
		if _, err := strconv.Atoi(segment); err == nil {
			path += "[" + segment + "]"
			continue
		}
		if path != "" {
			path += "."
		}
		path += segment
	}
	return path
}

// PreviewBlueprint fetches repo/branch/path from Git and dry-run validates the
// manifest without creating or applying anything — Render's pre-create
// "Review Blueprint configurations" step. Repository contents are private
// source material, so preview requires the sensitive-read role.
func (s *Service) PreviewBlueprint(ctx context.Context, ownerID, repo, branch, filePath string) (BlueprintPreview, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return BlueprintPreview{}, err
	}
	if repo == "" || branch == "" {
		return BlueprintPreview{}, fmt.Errorf("%w: repo and branch are required", core.ErrBadRequest)
	}
	if s.GitFetcher == nil {
		return BlueprintPreview{}, ErrBlueprintFetchUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	contents, commitSHA, discoveredPath, err := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, repo, branch, filePath)
	if err != nil {
		msg := err.Error()
		if after, ok := strings.CutPrefix(msg, "bad request: "); ok {
			msg = after
		}
		return BlueprintPreview{Error: msg}, nil
	}
	validation, err := s.blueprintValidationFor(ctx, repo, branch, contents)
	if err != nil {
		return BlueprintPreview{}, err
	}
	preview := BlueprintPreview{Found: true, Manifest: contents, CommitID: commitSHA, Validation: &validation}
	if filePath == "" && discoveredPath == LegacyBlueprintFilename {
		preview.Warning = "bex.yml is a deprecated filename-only alias; rename it to render.yaml"
	}
	return preview, nil
}

// CreateBlueprint creates a new Git-connected Blueprint instance by fetching
// the manifest from Git, validating it, applying the stack, and recording an
// initial sync run.
func (s *Service) CreateBlueprint(ctx context.Context, ownerID string, req CreateBlueprintRequest) (BlueprintView, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return BlueprintView{}, err
	}
	if s.Blueprints == nil {
		return BlueprintView{}, ErrBlueprintsUnavailable
	}
	if req.Repo == "" || req.Branch == "" {
		return BlueprintView{}, fmt.Errorf("%w: repo and branch are required", core.ErrBadRequest)
	}
	if req.Name == "" {
		req.Name = repoName(req.Repo)
	}
	tenantID := s.resolveTenantID(ctx)

	if s.GitFetcher == nil {
		return BlueprintView{}, ErrBlueprintFetchUnavailable
	}
	contents, commitSHA, discoveredPath, err := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, req.Repo, req.Branch, req.Path)
	if err != nil {
		return BlueprintView{}, fmt.Errorf("blueprint fetch: %w", err)
	}
	req.Path = discoveredPath

	prepareReq := DeployRequest{Repo: req.Repo, Branch: req.Branch, Manifest: contents, EnvVarValues: req.EnvVarValues}
	parsed, ir, parseErr := compileStack(prepareReq)
	if parseErr != nil {
		return BlueprintView{}, parseErr
	}
	if err := s.requireStackPaymentMethod(ctx, parsed); err != nil {
		return BlueprintView{}, err
	}
	if _, _, err := s.blueprintActionPlan(ctx, ir, parsed); err != nil {
		return BlueprintView{}, err
	}

	now := s.Now().UTC()
	b, err := s.Blueprints.UpsertBlueprint(ctx, store.Blueprint{
		TenantID: tenantID,
		Name:     req.Name,
		Repo:     req.Repo,
		Branch:   req.Branch,
		Path:     req.Path,
		AutoSync: true,
		Manifest: contents,
		Status:   store.BlueprintStatusSyncing,
	})
	if err != nil {
		return BlueprintView{}, err
	}

	run, _ := s.Blueprints.InsertBlueprintSync(ctx, store.BlueprintSync{
		BlueprintID: b.ID,
		CommitID:    commitSHA,
		State:       store.BlueprintSyncStateRunning,
		StartedAt:   now,
	})

	prepareReq.Confirm = req.Confirm
	prepareReq.BlueprintID = b.ID
	_, applyErr := s.deployParsedStack(ctx, prepareReq, parsed)

	finalStatus := store.BlueprintStatusInSync
	syncState := store.BlueprintSyncStateSuccess
	if applyErr != nil {
		finalStatus = store.BlueprintStatusError
		syncState = store.BlueprintSyncStateError
	}
	completedAt := s.Now().UTC()
	if updated, updErr := s.Blueprints.UpdateBlueprint(ctx, b.ID, tenantID, nil, nil, nil, &finalStatus, &completedAt); updErr == nil {
		b = updated
	}
	if run.ID != "" {
		_, _ = s.Blueprints.UpdateBlueprintSync(ctx, run.ID, syncState, &completedAt)
	}
	if applyErr != nil {
		return BlueprintView{}, applyErr
	}
	v := toBlueprintView(b)
	v.Resources = s.resolveBlueprintResourcesFromIR(ctx, b, ir)
	return v, nil
}

// GetBlueprintByID returns a single blueprint by its opaque id.
func (s *Service) GetBlueprintByID(ctx context.Context, bpID, ownerID string) (BlueprintView, error) {
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
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID)
	if err != nil {
		return BlueprintView{}, err
	}
	v := toBlueprintView(b)
	if !s.canReadManifest(ctx) {
		v.Manifest = ""
	}
	v.Resources = s.resolveBlueprintResources(ctx, b)
	return v, nil
}

// GetBlueprint is an alias for GetBlueprintByID.
func (s *Service) GetBlueprint(ctx context.Context, bpID, ownerID string) (BlueprintView, error) {
	return s.GetBlueprintByID(ctx, bpID, ownerID)
}

// ListBlueprints returns all non-disconnected blueprints for a workspace.
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
	if len(views) > 0 && !s.canReadManifest(ctx) {
		for i := range views {
			views[i].Manifest = ""
		}
	}
	return views, nil
}

// canReadManifest is the one place the raw-manifest sensitivity rule lives:
// the stored manifest is the same private repository content PreviewBlueprint
// gates on RelCanViewSensitive, and its envVars may carry literal values the
// env-vars API protects behind the same relation — serving it at can_view
// would be a read-around of both (codex r7 #11). Get/list blank Manifest for
// callers below that role; blueprint metadata stays viewer-readable.
func (s *Service) canReadManifest(ctx context.Context) bool {
	return s.Can(ctx, core.RelCanViewSensitive)
}

// SyncBlueprint re-applies the blueprint by id.
func (s *Service) SyncBlueprint(ctx context.Context, bpID, ownerID, bexYAML, confirm string) (SyncBlueprintResult, error) {
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
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID)
	if err != nil {
		return SyncBlueprintResult{}, err
	}
	return s.runSync(ctx, b, bexYAML, confirm)
}

// runSync is the shared sync engine: records a run, pulls-from-repo or uses
// the supplied/stored manifest, applies the stack, and stamps status + lastSyncAt.
// prepareSyncManifest validates one candidate manifest and records it as the
// blueprint's syncing manifest. Both sync sources — a caller-supplied YAML and
// a git-fetched file — run the identical compile → payment-gate → action-plan
// preflight before anything is persisted, so a manifest that cannot be applied
// never becomes the stored one.
func (s *Service) prepareSyncManifest(ctx context.Context, b store.Blueprint, manifest string) (store.Blueprint, *parsedStack, error) {
	parsed, ir, err := compileStack(DeployRequest{Repo: b.Repo, Branch: b.Branch, Manifest: manifest})
	if err != nil {
		return store.Blueprint{}, nil, err
	}
	if err := s.requireStackPaymentMethod(ctx, parsed); err != nil {
		return store.Blueprint{}, nil, err
	}
	if _, _, err := s.blueprintActionPlan(ctx, ir, parsed); err != nil {
		return store.Blueprint{}, nil, err
	}
	updated, err := s.Blueprints.UpsertBlueprint(ctx, store.Blueprint{
		ID:       b.ID,
		TenantID: b.TenantID,
		Name:     b.Name,
		Repo:     b.Repo,
		Branch:   b.Branch,
		Path:     b.Path,
		AutoSync: b.AutoSync,
		Manifest: manifest,
		Status:   store.BlueprintStatusSyncing,
	})
	if err != nil {
		return store.Blueprint{}, nil, err
	}
	return updated, &parsed, nil
}

func (s *Service) runSync(ctx context.Context, b store.Blueprint, bexYAML, confirm string) (SyncBlueprintResult, error) {
	tenantID := b.TenantID
	// w1/m69: a sync that cannot name the workspace it acts in must refuse
	// rather than apply identity-less (ErrBlueprintSyncWorkspaceUnresolved's
	// doc has the why). Store-off mode is unaffected: there is no tenant to
	// resolve and one shared namespace by design. The refusal also covers the
	// acting/named-conflict programming error, which Tenant collapses into
	// ok=false alongside a plain unresolved workspace.
	if s.Workspace != nil {
		if _, ok := s.Tenant(ctx); !ok {
			return SyncBlueprintResult{}, ErrBlueprintSyncWorkspaceUnresolved
		}
	}
	now := s.Now().UTC()
	var commitSHA string
	var prepared *parsedStack

	if bexYAML != "" {
		updated, parsed, err := s.prepareSyncManifest(ctx, b, bexYAML)
		if err != nil {
			return SyncBlueprintResult{}, err
		}
		b, prepared = updated, parsed
	} else if s.GitFetcher != nil && b.Repo != "" {
		// Re-validate the stored path before the token-backed fetch (round-6
		// #14): a legacy row written before UpdateBlueprint enforced the
		// allowlist must not become an arbitrary private-file read at sync time.
		if _, pathErr := approvedBlueprintPath(b.Path); pathErr != nil {
			return SyncBlueprintResult{}, pathErr
		}
		// A fetch failure is deliberately swallowed: the sync falls through to
		// the stored manifest rather than failing the whole run.
		if contents, sha, fetchErr := s.GitFetcher.FetchBlueprintFile(ctx, tenantID, b.Repo, b.Branch, b.Path); fetchErr == nil {
			commitSHA = sha
			updated, parsed, err := s.prepareSyncManifest(ctx, b, contents)
			if err != nil {
				return SyncBlueprintResult{}, err
			}
			b, prepared = updated, parsed
		}
	}

	if b.Status != store.BlueprintStatusSyncing {
		syncingStatus := store.BlueprintStatusSyncing
		if updated, err := s.Blueprints.UpdateBlueprint(ctx, b.ID, tenantID, nil, nil, nil, &syncingStatus, nil); err == nil {
			b = updated
		}
	}

	run, _ := s.Blueprints.InsertBlueprintSync(ctx, store.BlueprintSync{
		BlueprintID: b.ID,
		CommitID:    commitSHA,
		State:       store.BlueprintSyncStateRunning,
		StartedAt:   now,
	})

	deployReq := DeployRequest{
		BlueprintID: b.ID,
		Repo:        b.Repo,
		Branch:      b.Branch,
		Manifest:    b.Manifest,
		Confirm:     confirm,
	}
	var stack StackResult
	var applyErr error
	if prepared != nil {
		stack, applyErr = s.deployParsedStack(ctx, deployReq, *prepared)
	} else {
		stack, applyErr = s.deployStack(ctx, deployReq)
	}

	finalStatus := store.BlueprintStatusInSync
	syncState := store.BlueprintSyncStateSuccess
	if applyErr != nil {
		finalStatus = store.BlueprintStatusError
		syncState = store.BlueprintSyncStateError
		if !b.AutoSync {
			finalStatus = store.BlueprintStatusPaused
		}
	}
	completedAt := s.Now().UTC()
	if updated, updErr := s.Blueprints.UpdateBlueprint(ctx, b.ID, tenantID, nil, nil, nil, &finalStatus, &completedAt); updErr == nil {
		b = updated
	}
	if run.ID != "" {
		_, _ = s.Blueprints.UpdateBlueprintSync(ctx, run.ID, syncState, &completedAt)
	}
	if applyErr != nil {
		return SyncBlueprintResult{}, applyErr
	}
	return SyncBlueprintResult{Blueprint: toBlueprintView(b), Stack: stack}, nil
}

// triggerBlueprintSync is called by the push-webhook auto-sync path
// (webhook.go). Errors are intentionally dropped (webhook already responded).
func (s *Service) triggerBlueprintSync(ctx context.Context, tenantID, repo, branch string) {
	if s.Blueprints == nil {
		return
	}
	b, err := s.Blueprints.GetBlueprintByRepo(ctx, tenantID, repo, branch)
	if err != nil || !b.AutoSync {
		return
	}
	// w1/m69: bind the sync to the blueprint row's tenant. ctx arrives carrying
	// only WithWorkspace(tenantID) — workspace-NAMED but identity-less — which
	// resolveWorkspace treats as no tenant at all, so the apply pipeline used to
	// run with Tenant(ctx)=="". The acting tenant is derived from this store
	// row, never from the push payload.
	ctx = core.WithActingTenant(ctx, b.TenantID)
	_, _ = s.runSync(ctx, b, "", "")
}

// ListBlueprintSyncs returns recorded sync runs for a blueprint, newest first.
func (s *Service) ListBlueprintSyncs(ctx context.Context, bpID, ownerID, cursor string, limit int) ([]BlueprintSyncView, error) {
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
	if _, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID); err != nil {
		return nil, err
	}
	runs, err := s.Blueprints.ListBlueprintSyncs(ctx, bpID, cursor, core.PageLimitOrAbsent(limit))
	if err != nil {
		return nil, err
	}
	out := make([]BlueprintSyncView, len(runs))
	for i, r := range runs {
		out[i] = toBlueprintSyncView(r)
	}
	return out, nil
}

// UpdateBlueprint applies a partial update to name/autoSync/path.
func (s *Service) UpdateBlueprint(ctx context.Context, bpID, ownerID string, req UpdateBlueprintRequest) (BlueprintView, error) {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return BlueprintView{}, err
	}
	if s.Blueprints == nil {
		return BlueprintView{}, ErrBlueprintsUnavailable
	}
	if req.Name == nil && req.AutoSync == nil && req.Path == nil {
		return BlueprintView{}, fmt.Errorf("%w: at least one of name/autoSync/path must be provided", core.ErrBadRequest)
	}
	if req.Name != nil && *req.Name == "" {
		return BlueprintView{}, fmt.Errorf("%w: name cannot be empty", core.ErrBadRequest)
	}
	if req.Path != nil && *req.Path == "" {
		return BlueprintView{}, fmt.Errorf("%w: path cannot be empty", core.ErrBadRequest)
	}
	// The same allowlist discovery enforces (codex-security round-6 #14): a
	// later sync fetches this stored path with the workspace's GitHub
	// installation token and echoes the parsed content in BlueprintView, so an
	// unvalidated update would turn Blueprint create rights into an arbitrary
	// private-repository file probe. Store only the normalized result.
	if req.Path != nil {
		clean, err := approvedBlueprintPath(*req.Path)
		if err != nil {
			return BlueprintView{}, err
		}
		req.Path = &clean
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID)
	if err != nil {
		return BlueprintView{}, err
	}
	var newStatus *string
	if req.AutoSync != nil && !*req.AutoSync && b.Status != store.BlueprintStatusPaused {
		st := store.BlueprintStatusPaused
		newStatus = &st
	}
	if req.AutoSync != nil && *req.AutoSync && b.Status == store.BlueprintStatusPaused {
		st := store.BlueprintStatusInSync
		newStatus = &st
	}
	updated, err := s.Blueprints.UpdateBlueprint(ctx, bpID, tenantID, req.Name, req.AutoSync, req.Path, newStatus, nil)
	if err != nil {
		return BlueprintView{}, err
	}
	return toBlueprintView(updated), nil
}

// DisconnectBlueprint stops syncing and hides the blueprint from the workspace.
// Resources remain untouched (Render semantics).
func (s *Service) DisconnectBlueprint(ctx context.Context, bpID, ownerID string) error {
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Blueprints == nil {
		return ErrBlueprintsUnavailable
	}
	tenantID := s.resolveTenantID(ctx)
	// Read the stored manifest before the row disappears — it names the
	// groupings this blueprint minted, which the post-disconnect sweep may
	// reclaim (w8/m20 t004).
	manifest := ""
	if b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID); err == nil {
		manifest = b.Manifest
	}
	if err := s.Blueprints.DisconnectBlueprint(ctx, bpID, tenantID); err != nil {
		return err
	}
	s.reclaimBlueprintGroupings(ctx, tenantID, manifest)
	s.clearBlueprintOwnership(ctx, tenantID, bpID)
	return nil
}

// GroupingReclaimer is the optional store seam DisconnectBlueprint sweeps
// orphaned grouping rows through (*store.PGStore satisfies it). nil ⇒ no
// reclaim, the pre-w8/m20 behavior.
type GroupingReclaimer interface {
	ReclaimEmptyBlueprintGroupings(ctx context.Context, tenantID string, pairs []store.GroupingPair, referencedEnvironments, referencedProjects map[string]bool) (removedEnvironments, removedProjects []string, err error)
}

// reclaimBlueprintGroupings deletes the empty grouping rows the disconnected
// blueprint's stored manifest declared. Best-effort by design: disconnect has
// already succeeded, deployed resources are never touched (Render disconnect
// semantics), a populated or externally-referenced grouping survives with a
// log line, and a stale/unparseable manifest simply skips the sweep.
func (s *Service) reclaimBlueprintGroupings(ctx context.Context, tenantID, manifest string) {
	if s.GroupingReclaim == nil || manifest == "" {
		return
	}
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		log.Printf("blueprint disconnect: stored manifest no longer compiles; skipping grouping reclaim (workspace %s)", tenantID)
		return
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		log.Printf("blueprint disconnect: stored manifest no longer parses; skipping grouping reclaim (workspace %s): %v", tenantID, err)
		return
	}
	seen := map[store.GroupingPair]bool{}
	var pairs []store.GroupingPair
	for _, grouping := range st.groupings {
		pair := store.GroupingPair{Project: grouping.projectName, Environment: grouping.environmentName}
		if !seen[pair] {
			seen[pair] = true
			pairs = append(pairs, pair)
		}
	}
	if len(pairs) == 0 {
		return
	}
	referencedEnvironments, referencedProjects, err := s.datastoreGroupingRefs(ctx, tenantID)
	if err != nil {
		// Fail closed: the CR labels are the ONLY guard for datastore-populated
		// groupings — an incomplete reference snapshot must skip the sweep
		// (recoverable) rather than risk deleting a populated grouping
		// (irreversible).
		log.Printf("blueprint disconnect: datastore reference scan failed; skipping grouping reclaim (workspace %s): %v", tenantID, err)
		return
	}
	removedEnvironments, removedProjects, err := s.GroupingReclaim.ReclaimEmptyBlueprintGroupings(ctx, tenantID, pairs, referencedEnvironments, referencedProjects)
	if err != nil {
		log.Printf("blueprint disconnect: grouping reclaim failed (workspace %s): %v", tenantID, err)
		return
	}
	var audits []groupingAuditEntry
	for _, name := range removedEnvironments {
		audits = append(audits, groupingAuditEntry{action: "environment_reclaimed", project: name})
	}
	for _, name := range removedProjects {
		audits = append(audits, groupingAuditEntry{action: "project_reclaimed", project: name})
	}
	s.emitGroupingAudits(ctx, tenantID, audits)
}

// datastoreGroupingRefs collects the environment/project ids the workspace's
// Database and KeyValue CRs still reference through their membership labels —
// a datastore-populated grouping must survive the reclaim sweep.
func (s *Service) datastoreGroupingRefs(ctx context.Context, tenantID string) (map[string]bool, map[string]bool, error) {
	environments, projects := map[string]bool{}, map[string]bool{}
	databases, err := s.listWorkspaceDatabases(ctx, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing workspace databases: %w", err)
	}
	for _, db := range databases.Items {
		if id := db.Labels[core.LabelEnvironment]; id != "" {
			environments[id] = true
		}
		if id := db.Labels[core.LabelProject]; id != "" {
			projects[id] = true
		}
	}
	keyValues, err := s.listWorkspaceKeyValues(ctx, tenantID)
	if err != nil {
		return nil, nil, fmt.Errorf("listing workspace key values: %w", err)
	}
	for _, kv := range keyValues.Items {
		if id := kv.Labels[core.LabelEnvironment]; id != "" {
			environments[id] = true
		}
		if id := kv.Labels[core.LabelProject]; id != "" {
			projects[id] = true
		}
	}
	return environments, projects, nil
}

// upsertBlueprint auto-registers a blueprint row after a successful deployStack
// call. Called from deploy.go when req.Repo is non-empty.
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
		branch = appv1alpha1.DefaultBranch
	}
	_, _ = s.Blueprints.UpsertBlueprint(ctx, store.Blueprint{
		TenantID: tenantID,
		Name:     repoName(req.Repo),
		Repo:     req.Repo,
		Branch:   branch,
		Path:     CanonicalBlueprintFilename,
		AutoSync: true,
		Manifest: req.Manifest,
		Status:   store.BlueprintStatusInSync,
	})
}

// resolveTenantID returns the effective tenant id.
// An explicitly named workspace in the context wins; otherwise the caller's
// default is resolved through the store (Workspace). When the store is off
// (Workspace == nil), only an explicitly named workspace is available.
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

// resolveBlueprintResources resolves the manifest's declared resources to their
// current live App / Database CRs within the workspace.
func (s *Service) resolveBlueprintResources(ctx context.Context, b store.Blueprint) []BlueprintResource {
	if b.Manifest == "" {
		return nil
	}
	_, ir, problems := CompileBlueprintIR(b.Manifest)
	if len(problems) > 0 || len(ir.Resources) == 0 {
		return nil
	}
	return s.resolveBlueprintResourcesFromIR(ctx, b, ir)
}

func (s *Service) resolveBlueprintResourcesFromIR(ctx context.Context, b store.Blueprint, ir BlueprintIR) []BlueprintResource {
	if len(ir.Resources) == 0 {
		return nil
	}

	tenantID := b.TenantID
	ns := s.AppNamespace(tenantID)
	opts := []client.ListOption{
		client.InNamespace(ns),
		client.MatchingLabels{core.LabelTenant: tenantID},
	}

	appByName := map[string]*appv1alpha1.App{}
	dbByName := map[string]*appv1alpha1.Database{}
	keyValueByName := map[string]*appv1alpha1.KeyValue{}
	if s.Client != nil {
		var appList appv1alpha1.AppList
		if err := s.Client.List(ctx, &appList, opts...); err == nil {
			for i := range appList.Items {
				a := &appList.Items[i]
				appByName[a.Name] = a
			}
		}

		var dbList appv1alpha1.DatabaseList
		if err := s.Client.List(ctx, &dbList, opts...); err == nil {
			for i := range dbList.Items {
				d := &dbList.Items[i]
				dbByName[d.Spec.Name] = d
			}
		}
		var keyValueList appv1alpha1.KeyValueList
		if err := s.Client.List(ctx, &keyValueList, opts...); err == nil {
			for i := range keyValueList.Items {
				kv := &keyValueList.Items[i]
				keyValueByName[kv.Spec.Name] = kv
			}
		}
	}
	envGroupByName := map[string]string{}
	hasEnvGroups := false
	for _, resource := range ir.Resources {
		if resource.Kind == BlueprintResourceEnvVarGroup {
			hasEnvGroups = true
			break
		}
	}
	if hasEnvGroups && s.EnvGroups != nil {
		if groups, err := s.EnvGroups.GroupIDsByName(ctx); err == nil {
			envGroupByName = groups
		}
	}

	var resources []BlueprintResource
	for _, resource := range ir.Resources {
		switch resource.Kind {
		case BlueprintResourceService:
			a, ok := appByName[resource.Name]
			if !ok {
				continue
			}
			resourceID := store.ManagedAppID(a.Labels)
			if resourceID == "" {
				resourceID = a.Name
			}
			resources = append(resources, BlueprintResource{ID: resourceID, Name: resource.Name, Type: effectiveType(a.Spec.Type)})
		case BlueprintResourcePostgres:
			d, ok := dbByName[resource.Name]
			if !ok {
				continue
			}
			resources = append(resources, BlueprintResource{ID: d.Name, Name: resource.Name, Type: "postgres"})
		case BlueprintResourceKeyValue:
			kv, ok := keyValueByName[resource.Name]
			if !ok {
				continue
			}
			resources = append(resources, BlueprintResource{ID: kv.Name, Name: resource.Name, Type: "key_value"})
		case BlueprintResourceEnvVarGroup:
			groupID, ok := envGroupByName[resource.Name]
			if !ok {
				continue
			}
			resources = append(resources, BlueprintResource{ID: groupID, Name: resource.Name, Type: "environment_group"})
		}
	}
	return resources
}

// blueprintValidationPlanFromIR builds the structural validation summary from
// the exact normalized declarations that were compiled for this request. It
// deliberately does not touch the raw manifest: this prevents a second YAML
// parse from disagreeing with the strict compiler on aliases, duplicate keys,
// scalar types, or nested resource placement.
func blueprintValidationPlanFromIR(ir BlueprintIR, st parsedStack) BlueprintValidationPlan {
	plan := BlueprintValidationPlan{
		Mode:      "structural",
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
	plan.SyncFalseVars = syncFalseVarsFromBlueprintIR(ir)
	plan.TotalActions = len(plan.Services) + len(plan.Databases) + len(plan.KeyValue) + len(plan.EnvGroups)
	return plan
}

func syncFalseVarsFromBlueprintIR(ir BlueprintIR) []string {
	seen := map[string]bool{}
	var names []string
	for _, resource := range ir.Resources {
		if resource.Kind != BlueprintResourceService {
			continue
		}
		envVars, _ := resource.Fields["envVars"].Value.([]any)
		for _, rawEnv := range envVars {
			env, _ := rawEnv.(map[string]any)
			key, _ := env["key"].(string)
			sync, declared := env["sync"].(bool)
			if declared && !sync && key != "" && !seen[key] {
				seen[key] = true
				names = append(names, key)
			}
		}
	}
	return names
}

var (
	yamlLineRE              = regexp.MustCompile(`(?i)\bline\s+(\d+)\b`)
	unknownBlueprintFieldRE = regexp.MustCompile(`^(.+) contains unknown field "([^"]+)"$`)
)

func blueprintValidationError(ir BlueprintIR, message string) BlueprintValidationError {
	out := BlueprintValidationError{Error: message}
	if match := yamlLineRE.FindStringSubmatch(message); len(match) == 2 {
		if line, err := strconv.Atoi(match[1]); err == nil && line > 0 {
			out.Line = &line
		}
	}
	if fieldPath := blueprintErrorPath(ir, message); fieldPath != "" {
		out.Path = &fieldPath
	}
	return out
}

func blueprintErrorPath(ir BlueprintIR, message string) string {
	if match := unknownBlueprintFieldRE.FindStringSubmatch(message); len(match) == 3 {
		return match[1] + "." + match[2]
	}
	for _, resource := range ir.Resources {
		if strings.Contains(message, fmt.Sprintf("%q", resource.Name)) || strings.Contains(message, resource.Name+" ") {
			return blueprintDisplayPath(resource.SourcePath) + blueprintErrorField(message)
		}
	}
	return ""
}

func blueprintErrorField(message string) string {
	for _, field := range []string{"maintenanceMode", "plan", "domains", "schedule", "runtime", "type", "image", "name", "ipAllowList", "renderSubdomainPolicy", "scaling"} {
		if strings.Contains(strings.ToLower(message), strings.ToLower(field)) {
			return "." + field
		}
	}
	if strings.Contains(message, " env ") || strings.Contains(message, "env var") {
		return ".envVars"
	}
	return ""
}

// repoName extracts a human-friendly name from a repo URL.
func repoName(repo string) string {
	base := path.Base(strings.TrimRight(repo, "/"))
	return strings.TrimSuffix(base, ".git")
}

func toBlueprintView(b store.Blueprint) BlueprintView {
	status := b.Status
	if status == "active" {
		status = store.BlueprintStatusInSync
	}
	v := BlueprintView{
		ID:        b.ID,
		Name:      b.Name,
		Repo:      b.Repo,
		Branch:    b.Branch,
		Path:      b.Path,
		AutoSync:  b.AutoSync,
		Manifest:  b.Manifest,
		Status:    status,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
	if b.LastSyncAt != nil {
		s := b.LastSyncAt.UTC().Format(time.RFC3339)
		v.LastSync = &s
	}
	return v
}

func toBlueprintSyncView(r store.BlueprintSync) BlueprintSyncView {
	v := BlueprintSyncView{
		ID:        r.ID,
		CommitID:  r.CommitID,
		State:     r.State,
		StartedAt: r.StartedAt.UTC().Format(time.RFC3339),
	}
	if r.CompletedAt != nil {
		s := r.CompletedAt.UTC().Format(time.RFC3339)
		v.CompletedAt = &s
	}
	return v
}
