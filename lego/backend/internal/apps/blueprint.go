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
	UpdateBlueprintSync(ctx context.Context, id, state string, completedAt *time.Time, errMsg *string) (store.BlueprintSync, error)
	ListBlueprintSyncs(ctx context.Context, blueprintID, cursor string, limit int) ([]store.BlueprintSync, error)
	// AdmitBlueprintSyncRun atomically claims the active apply for an existing
	// Blueprint and records its running run (w8/m37 t002). Zero claimed rows
	// (disconnected between lookup and admission, or a lost admission race)
	// report store.ErrBlueprintSyncBusy.
	AdmitBlueprintSyncRun(ctx context.Context, blueprintID, tenantID string, run store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error)
	// AdmitBlueprintCreate atomically upserts the row for explicit creation and
	// claims its initial sync, reviving disconnected rows under a fresh
	// generation (w8/m37 t001). A conflicting live claim reports
	// store.ErrBlueprintSyncBusy.
	AdmitBlueprintCreate(ctx context.Context, b store.Blueprint, run store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error)
	// StageBlueprintManifest stores the admitted sync's preflighted manifest,
	// fencing on the admitted generation (w8/m37 t002/t005).
	StageBlueprintManifest(ctx context.Context, id, tenantID string, generation int64, runID, manifest string) (store.Blueprint, error)
	// CompleteBlueprintSync commits a run's terminal state together with the
	// status projected from the current row (w8/m37 t002/t005). A stale
	// completion reports store.ErrBlueprintSyncBusy without overwriting.
	CompleteBlueprintSync(ctx context.Context, id, tenantID, runID string, generation int64, state string, completedAt time.Time, errMsg *string) (store.Blueprint, error)
	// FailAdmittedSync settles an admitted run in error without projecting
	// Blueprint status: preflight/stage failures leave current settings and
	// status untouched while releasing the claim (w8/m37 t005).
	FailAdmittedSync(ctx context.Context, id, tenantID, runID string, generation int64, completedAt time.Time, errMsg *string) error
	// ListAbandonedBlueprintSyncs returns stale running runs for the recovery
	// sweep, oldest first, bounded per tick (w8/m37 t004).
	ListAbandonedBlueprintSyncs(ctx context.Context, before time.Time, limit int) ([]store.AbandonedBlueprintSync, error)
	// AbandonBlueprintSync settles one stale running run as interrupted,
	// flipping its Blueprint to error only while the abandoned generation
	// still owns it (w8/m37 t004). False means another writer settled first.
	AbandonBlueprintSync(ctx context.Context, runID string, now time.Time, reason string) (bool, error)
}

// errBlueprintSyncBusy is the one documented 409 for every lifecycle fencing
// outcome (admission contention, fenced stage/completion, disconnect-busy):
// one coded error through REST, GraphQL, MCP, and the dashboard (w8/m37 t002).
func errBlueprintSyncBusy(msg string) error {
	return core.NewConflictError("BLUEPRINT_SYNC_BUSY", msg, nil)
}

// isBlueprintBusy reports a lost (or never held) execution claim from the store.
func isBlueprintBusy(err error) bool {
	return errors.Is(err, store.ErrBlueprintSyncBusy)
}

// BlueprintFetcher fetches a blueprint file from its Git repository. The
// github.Service.BlueprintFileFetcher() adapter satisfies it. nil on
// apps.Service (s.GitFetcher) ⇒ create/sync cannot pull-from-repo.
//
// The revision contract (w8/m36 t002): resolve the branch to one immutable
// commit first, then read every manifest byte at that commit — a branch that
// advances mid-sync cannot mix revisions into one apply.
type BlueprintFetcher interface {
	// ResolveBlueprintCommit pins branch to its immutable HEAD commit.
	// Lookup failure or an empty/malformed commit ID is an actionable error,
	// never fabricated provenance.
	ResolveBlueprintCommit(ctx context.Context, workspaceID, repoURL, branch string) (commitSHA string, err error)
	// FetchBlueprintFileAtCommit reads filePath at an already-resolved commit.
	FetchBlueprintFileAtCommit(ctx context.Context, workspaceID, repoURL, commitSHA, filePath string) (contents string, err error)
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
//
// Every probe reads at one resolved commit (w8/m36 t002): the branch is pinned
// once up front, so canonical/legacy discovery examines a consistent revision
// instead of two independently moving branch tips.
func discoverBlueprintFile(ctx context.Context, fetcher BlueprintFetcher, workspaceID, repo, branch, explicitPath string) (contents, commitSHA, filePath string, err error) {
	commitSHA, err = fetcher.ResolveBlueprintCommit(ctx, workspaceID, repo, branch)
	if err != nil {
		return "", "", "", err
	}
	if explicitPath != "" {
		explicitPath, err = approvedBlueprintPath(explicitPath)
		if err != nil {
			return "", "", "", err
		}
		contents, err = fetcher.FetchBlueprintFileAtCommit(ctx, workspaceID, repo, commitSHA, explicitPath)
		return contents, commitSHA, explicitPath, err
	}
	canonicalContents, canonicalErr := fetcher.FetchBlueprintFileAtCommit(ctx, workspaceID, repo, commitSHA, CanonicalBlueprintFilename)
	legacyContents, legacyErr := fetcher.FetchBlueprintFileAtCommit(ctx, workspaceID, repo, commitSHA, LegacyBlueprintFilename)
	switch {
	case canonicalErr == nil && legacyErr == nil:
		return "", "", "", ErrBlueprintFilenameAmbiguous
	case canonicalErr == nil:
		return canonicalContents, commitSHA, CanonicalBlueprintFilename, nil
	case legacyErr == nil:
		return legacyContents, commitSHA, LegacyBlueprintFilename, nil
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
	ID           string  `json:"id"`
	CommitID     string  `json:"commitId,omitempty"`
	State        string  `json:"state"`
	StartedAt    string  `json:"startedAt"`
	CompletedAt  *string `json:"completedAt,omitempty"`
	ErrorMessage *string `json:"errorMessage,omitempty"`
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
	// RelCanViewSensitive is a read relation, so Authorize uses the decision
	// cache. Private repo contents are a sensitive sink: re-check uncached
	// before any Git fetch (codex round-15 #4).
	if err := s.AuthorizeFresh(ctx, core.RelCanViewSensitive); err != nil {
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

	// Admission claims the initial apply and records its run atomically (w8/m37
	// t002): a failed admission blocks every workload side effect, a competing
	// live claim loses with the documented busy conflict, and a disconnected
	// row is deliberately re-established under a fresh generation.
	now := s.Now().UTC()
	b, run, err := s.Blueprints.AdmitBlueprintCreate(ctx, store.Blueprint{
		TenantID: tenantID,
		Name:     req.Name,
		Repo:     req.Repo,
		Branch:   req.Branch,
		Path:     req.Path,
		AutoSync: true,
		Manifest: contents,
	}, store.BlueprintSync{
		CommitID:  commitSHA,
		State:     store.BlueprintSyncStateRunning,
		StartedAt: now,
	})
	if err != nil {
		if isBlueprintBusy(err) {
			return BlueprintView{}, errBlueprintSyncBusy("another sync is already running for this blueprint; retry after it settles")
		}
		return BlueprintView{}, err
	}

	prepareReq.Confirm = req.Confirm
	prepareReq.BlueprintID = b.ID
	prepareReq.BlueprintGeneration = b.ExecutionGeneration
	_, applyErr := s.deployParsedStack(ctx, prepareReq, parsed)

	b, cerr := s.completeAdmittedSync(ctx, b, run, applyErr, "create")
	if cerr != nil {
		return BlueprintView{}, cerr
	}
	v := toBlueprintView(b)
	v.Resources = s.resolveBlueprintResourcesFromIR(ctx, b, ir)
	return v, nil
}

// completeAdmittedSync commits the terminal state of an admitted run together
// with the status projected from the current row (w8/m37 t002/t005), and maps
// the outcome to the verb's return. applyErr is the apply outcome (nil on
// success); recordVerb names the verb for completion-persistence failures
// ("sync"/"create"). A fenced completion is never reported as success: on a
// failed apply the apply error is returned, on a successful apply the fencing
// error is.
func (s *Service) completeAdmittedSync(ctx context.Context, b store.Blueprint, run store.BlueprintSync, applyErr error, recordVerb string) (store.Blueprint, error) {
	state := store.BlueprintSyncStateSuccess
	if applyErr != nil {
		state = store.BlueprintSyncStateError
	}
	completedAt := s.Now().UTC()
	updated, err := s.Blueprints.CompleteBlueprintSync(ctx, b.ID, b.TenantID, run.ID, run.ExecutionGeneration, state, completedAt, errMsgPtr(applyErr))
	if err != nil {
		if isBlueprintBusy(err) {
			if applyErr != nil {
				return b, applyErr
			}
			return b, errBlueprintSyncBusy("this sync no longer owns blueprint execution (superseded or disconnected); start a new sync")
		}
		return b, fmt.Errorf("blueprint %s: record completion: %w", recordVerb, err)
	}
	if applyErr != nil {
		return updated, applyErr
	}
	return updated, nil
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
		return BlueprintView{}, store.MapError(err)
	}
	v := toBlueprintView(b)
	if !s.canReadManifest(ctx) {
		v.Manifest = ""
	}
	v.Resources = s.resolveBlueprintResources(ctx, b)
	return v, nil
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
	// RelCanViewSensitive is a read relation, so Can/Authorize would ride a
	// stale positive. Get/list blank Manifest rather than failing the whole
	// read: metadata stays viewer-readable (codex round-15 #4).
	return s.AuthorizeFresh(ctx, core.RelCanViewSensitive) == nil
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
		return SyncBlueprintResult{}, store.MapError(err)
	}
	return s.runSync(ctx, b, bexYAML, confirm)
}

// runSync is the shared sync engine: admits the apply, pulls-from-repo or uses
// the supplied/stored manifest, applies the stack, and commits status +
// lastSyncAt together with the run's terminal state.
// prepareSyncManifest validates one candidate manifest and stages it as the
// blueprint's syncing manifest under the admitted generation. Both sync
// sources — a caller-supplied YAML and a git-fetched file — run the identical
// compile → payment-gate → action-plan preflight before anything is persisted,
// so a manifest that cannot be applied never becomes the stored one. Only the
// manifest (the sync-owned field) is written: current name/path/autoSync
// settings survive the sync (w8/m37 t005).
func (s *Service) prepareSyncManifest(ctx context.Context, b store.Blueprint, run store.BlueprintSync, manifest string) (store.Blueprint, *parsedStack, error) {
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
	staged, err := s.Blueprints.StageBlueprintManifest(ctx, b.ID, b.TenantID, run.ExecutionGeneration, run.ID, manifest)
	if err != nil {
		if isBlueprintBusy(err) {
			return store.Blueprint{}, nil, errBlueprintSyncBusy("this sync no longer owns blueprint execution (superseded or disconnected); start a new sync")
		}
		return store.Blueprint{}, nil, err
	}
	return staged, &parsed, nil
}

// failSync records a failed attempt for an existing Blueprint and returns the
// source error (w8/m36 t004, w8/m37 t002). The run is inserted directly in its
// terminal error state — one statement, so a process dying mid-report cannot
// strand a running row. Source, validation, and admission-contention failures
// terminate in sync history with their sanitized reason instead of vanishing
// before any run is recorded. No workload mutation happens here; when the
// store itself is unavailable the source error is returned without promising
// a history row.
func (s *Service) failSync(ctx context.Context, b store.Blueprint, commitSHA string, startedAt time.Time, srcErr error) (SyncBlueprintResult, error) {
	completedAt := s.Now().UTC()
	_, _ = s.Blueprints.InsertBlueprintSync(ctx, store.BlueprintSync{
		BlueprintID:  b.ID,
		CommitID:     commitSHA,
		State:        store.BlueprintSyncStateError,
		StartedAt:    startedAt,
		CompletedAt:  &completedAt,
		ErrorMessage: errMsgPtr(srcErr),
	})
	return SyncBlueprintResult{}, srcErr
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
	var manifest string
	var prepared *parsedStack
	// useStored covers only legacy non-Git rows (no Repo to fetch from): the
	// one path that still applies the stored manifest. A Git-backed source
	// that cannot be fetched is a failure, never an implicit reapply (w8/m36
	// t003) — the stored manifest is preserved untouched for the next attempt.
	useStored := false

	if bexYAML != "" {
		// Explicit supplied-manifest sync (the documented bex extension): no
		// Git commit is consumed, so none is claimed.
		manifest = bexYAML
	} else if b.Repo != "" {
		if s.GitFetcher == nil {
			return s.failSync(ctx, b, "", now, ErrBlueprintFetchUnavailable)
		}
		// Re-validate the stored path before the token-backed fetch (round-6
		// #14): a legacy row written before UpdateBlueprint enforced the
		// allowlist must not become an arbitrary private-file read at sync time.
		if _, pathErr := approvedBlueprintPath(b.Path); pathErr != nil {
			return s.failSync(ctx, b, "", now, pathErr)
		}
		// Resolve-then-read at one immutable commit (w8/m36 t002): the commit
		// is known before the run is recorded, and any fetch failure below
		// fails the sync without touching the stored manifest.
		sha, resolveErr := s.GitFetcher.ResolveBlueprintCommit(ctx, tenantID, b.Repo, b.Branch)
		if resolveErr != nil {
			return s.failSync(ctx, b, "", now, resolveErr)
		}
		contents, fetchErr := s.GitFetcher.FetchBlueprintFileAtCommit(ctx, tenantID, b.Repo, sha, b.Path)
		if fetchErr != nil {
			return s.failSync(ctx, b, sha, now, fetchErr)
		}
		commitSHA, manifest = sha, contents
	} else {
		useStored = true
	}

	// Admission claims the active apply and records its run atomically (w8/m37
	// t002): at most one admitted apply per Blueprint across replicas, and a
	// failed admission blocks every workload side effect below. A competing
	// live claim loses with the documented busy conflict (recorded in history);
	// a disconnect that lands first fences here the same way.
	b, run, err := s.Blueprints.AdmitBlueprintSyncRun(ctx, b.ID, tenantID, store.BlueprintSync{
		CommitID:  commitSHA,
		State:     store.BlueprintSyncStateRunning,
		StartedAt: now,
	})
	if err != nil {
		if isBlueprintBusy(err) {
			return s.failSync(ctx, b, commitSHA, now, errBlueprintSyncBusy("another sync is already running for this blueprint; retry after it settles"))
		}
		return SyncBlueprintResult{}, err
	}

	// settleStage terminates an admitted run whose manifest could not be staged
	// (preflight refusal, or a lost fence on the legacy path) without touching
	// Blueprint status or settings (w8/m37 t005), releasing the claim. The
	// proximate error is what the caller can act on; a settle-persistence
	// failure is best-effort (the recovery sweep bounds a leaked claim).
	settleStage := func(stageErr error) (SyncBlueprintResult, error) {
		completedAt := s.Now().UTC()
		_ = s.Blueprints.FailAdmittedSync(ctx, b.ID, tenantID, run.ID, run.ExecutionGeneration, completedAt, errMsgPtr(stageErr))
		return SyncBlueprintResult{}, stageErr
	}
	if useStored {
		// Legacy non-Git rows apply the stored manifest: staging records the
		// same bytes and marks the admitted apply syncing under the fence.
		staged, err := s.Blueprints.StageBlueprintManifest(ctx, b.ID, tenantID, run.ExecutionGeneration, run.ID, b.Manifest)
		if err != nil {
			if isBlueprintBusy(err) {
				err = errBlueprintSyncBusy("this sync no longer owns blueprint execution (superseded or disconnected); start a new sync")
			}
			return settleStage(err)
		}
		b = staged
	} else {
		staged, parsed, err := s.prepareSyncManifest(ctx, b, run, manifest)
		if err != nil {
			return settleStage(err)
		}
		b, prepared = staged, parsed
	}

	deployReq := DeployRequest{
		BlueprintID:         b.ID,
		BlueprintGeneration: b.ExecutionGeneration,
		Repo:                b.Repo,
		Branch:              b.Branch,
		Manifest:            b.Manifest,
		Confirm:             confirm,
	}
	var stack StackResult
	var applyErr error
	if prepared != nil {
		stack, applyErr = s.deployParsedStack(ctx, deployReq, *prepared)
	} else {
		stack, applyErr = s.deployStack(ctx, deployReq)
	}

	b, cerr := s.completeAdmittedSync(ctx, b, run, applyErr, "sync")
	if cerr != nil {
		return SyncBlueprintResult{}, cerr
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
		return nil, store.MapError(err)
	}
	runs, err := s.Blueprints.ListBlueprintSyncs(ctx, bpID, cursor, core.PageLimitOrAbsent(limit))
	if err != nil {
		return nil, store.MapError(err)
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
		return BlueprintView{}, store.MapError(err)
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
		return BlueprintView{}, store.MapError(err)
	}
	return toBlueprintView(updated), nil
}

// DisconnectBlueprint stops syncing and hides the blueprint from the workspace.
// Resources remain untouched (Render semantics).
//
// Lifecycle coordination (w8/m37 t003): the store fences the transition on the
// execution boundary — a fresh apply still owning the claim refuses with the
// documented busy conflict (retry after it settles), a stale claim is settled
// inline, and late completions or ownership stamps from the fenced run cannot
// restore management afterwards. Cleanup failures are returned, never
// discarded: the row stays disconnected either way, and the error names the
// failed sweep for the operator.
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
	// reclaim (w8/m20 t004). A disconnected row reads as absent, so its
	// manifest is simply unavailable to the sweep.
	manifest := ""
	if b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID); err == nil {
		manifest = b.Manifest
	}
	if err := s.Blueprints.DisconnectBlueprint(ctx, bpID, tenantID); err != nil {
		if isBlueprintBusy(err) {
			return errBlueprintSyncBusy("a sync is currently applying for this blueprint; retry disconnect after it settles")
		}
		return store.MapError(err)
	}
	if err := s.reclaimBlueprintGroupings(ctx, tenantID, manifest); err != nil {
		log.Printf("blueprint %s: %v", bpID, err)
		return err
	}
	if err := s.clearBlueprintOwnership(ctx, tenantID, bpID); err != nil {
		log.Printf("blueprint %s: disconnect ownership cleanup failed: %v", bpID, err)
		return fmt.Errorf("blueprint disconnected but ownership cleanup failed: %w", err)
	}
	return nil
}

// GroupingReclaimer is the optional store seam DisconnectBlueprint sweeps
// orphaned grouping rows through (*store.PGStore satisfies it). nil ⇒ no
// reclaim, the pre-w8/m20 behavior.
type GroupingReclaimer interface {
	ReclaimEmptyBlueprintGroupings(ctx context.Context, tenantID string, pairs []store.GroupingPair, referencedEnvironments, referencedProjects map[string]bool) (removedEnvironments, removedProjects []string, err error)
}

// reclaimBlueprintGroupings deletes the empty grouping rows the disconnected
// blueprint's stored manifest declared. Skips (no manifest, unparseable
// manifest, no pairs, incomplete reference snapshot) are benign and return
// nil: disconnect has already succeeded, deployed resources are never touched
// (Render disconnect semantics), and a populated or externally-referenced
// grouping survives. Only a genuine reclaim-store failure is returned, so
// disconnect reports failed cleanup instead of inferring success from a
// discarded error (w8/m37 t003).
func (s *Service) reclaimBlueprintGroupings(ctx context.Context, tenantID, manifest string) error {
	if s.GroupingReclaim == nil || manifest == "" {
		return nil
	}
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		log.Printf("blueprint disconnect: stored manifest no longer compiles; skipping grouping reclaim (workspace %s)", tenantID)
		return nil
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		log.Printf("blueprint disconnect: stored manifest no longer parses; skipping grouping reclaim (workspace %s): %v", tenantID, err)
		return nil
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
		return nil
	}
	referencedEnvironments, referencedProjects, err := s.datastoreGroupingRefs(ctx, tenantID)
	if err != nil {
		// Fail closed: the CR labels are the ONLY guard for datastore-populated
		// groupings — an incomplete reference snapshot must skip the sweep
		// (recoverable) rather than risk deleting a populated grouping
		// (irreversible).
		log.Printf("blueprint disconnect: datastore reference scan failed; skipping grouping reclaim (workspace %s): %v", tenantID, err)
		return nil
	}
	removedEnvironments, removedProjects, err := s.GroupingReclaim.ReclaimEmptyBlueprintGroupings(ctx, tenantID, pairs, referencedEnvironments, referencedProjects)
	if err != nil {
		return fmt.Errorf("blueprint disconnect: grouping reclaim: %w", err)
	}
	var audits []groupingAuditEntry
	for _, name := range removedEnvironments {
		audits = append(audits, groupingAuditEntry{action: "environment_reclaimed", project: name})
	}
	for _, name := range removedProjects {
		audits = append(audits, groupingAuditEntry{action: "project_reclaimed", project: name})
	}
	s.emitGroupingAudits(ctx, tenantID, audits)
	return nil
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
		ID:           r.ID,
		CommitID:     r.CommitID,
		State:        r.State,
		StartedAt:    r.StartedAt.UTC().Format(time.RFC3339),
		ErrorMessage: r.ErrorMessage,
	}
	if r.CompletedAt != nil {
		s := r.CompletedAt.UTC().Format(time.RFC3339)
		v.CompletedAt = &s
	}
	return v
}

// errMsgPtr is nil on success, or err's message on failure — the shape
// UpdateBlueprintSync persists into blueprint_syncs.error_message.
func errMsgPtr(err error) *string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return &msg
}
