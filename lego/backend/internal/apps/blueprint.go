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
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// BlueprintStore is the persistence interface blueprints need.
// *store.PGStore satisfies it structurally.
type BlueprintStore interface {
	UpsertBlueprint(ctx context.Context, b store.Blueprint) (store.Blueprint, error)
	GetBlueprint(ctx context.Context, id, tenantID string) (store.Blueprint, error)
	// BlueprintNameTaken reports whether another live, approved Blueprint in
	// the workspace has name, ignoring case; every create and a rename ask
	// it (w8/250).
	BlueprintNameTaken(ctx context.Context, tenantID, name, exceptID string) (bool, error)
	// BlueprintWorkspace resolves the workspace owning a live Blueprint id —
	// routing only; the caller authorizes against it (w4/m169).
	BlueprintWorkspace(ctx context.Context, id string) (string, error)
	// BlueprintCreateRetry returns the Blueprint a one-step create of this
	// file and mode would retry — a live one no sync has brought in sync — or
	// store.ErrNotFound when the create is a new Blueprint (w8/m153).
	BlueprintCreateRetry(ctx context.Context, tenantID, repo, branch, path, mode string) (store.Blueprint, error)
	// ListAutoSyncBlueprints returns auto-sync-enabled, non-disconnected
	// Blueprints on branch (w8/m38). tenantScope "" = all workspaces;
	// otherwise confine to that installation-bound workspace. Repo URL
	// matching is the caller's responsibility (repoURLsMatch).
	ListAutoSyncBlueprints(ctx context.Context, branch, tenantScope string) ([]store.Blueprint, error)
	ListBlueprints(ctx context.Context, tenantID string) ([]store.Blueprint, error)
	// EnqueueBlueprintAutoSyncIntent persists accepted automatic work before
	// webhook acknowledgment (w8/m38). inserted=false on duplicate delivery.
	EnqueueBlueprintAutoSyncIntent(ctx context.Context, intent store.BlueprintAutoSyncIntent) (inserted bool, err error)
	// ClaimBlueprintAutoSyncIntents leases a bounded pending batch for the
	// durable auto-sync worker (w8/m38).
	ClaimBlueprintAutoSyncIntents(ctx context.Context, limit int, lease time.Duration) ([]store.BlueprintAutoSyncIntent, error)
	CompleteBlueprintAutoSyncIntent(ctx context.Context, id string, claimGeneration int64, reason string) error
	FailBlueprintAutoSyncIntent(ctx context.Context, id string, claimGeneration int64, errMsg string) error
	AssertBlueprintAutoSyncIntent(ctx context.Context, id string, claimGeneration int64) error
	AdmitBlueprintAutoSyncRun(ctx context.Context, intent store.BlueprintAutoSyncIntent, source store.Blueprint, run store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error)
	UpdateBlueprint(ctx context.Context, id, tenantID string, name *string, autoSync *bool, path, branch *string, status *string, lastSyncAt *time.Time) (store.Blueprint, error)
	DisconnectBlueprint(ctx context.Context, id, tenantID string) error
	InsertBlueprintSync(ctx context.Context, run store.BlueprintSync) (store.BlueprintSync, error)
	UpdateBlueprintSync(ctx context.Context, id, state string, completedAt *time.Time, errMsg *string) (store.BlueprintSync, error)
	ListBlueprintSyncs(ctx context.Context, blueprintID, cursor string, limit int) ([]store.BlueprintSync, error)
	// AdmitBlueprintSyncRun atomically claims the active apply for an existing
	// Blueprint and records its running run (w8/m37 t002). Zero claimed rows
	// (disconnected between lookup and admission, or a lost admission race)
	// report store.ErrBlueprintSyncBusy.
	AdmitBlueprintSyncRun(ctx context.Context, blueprintID, tenantID string, run store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error)
	// AdmitBlueprintCreate atomically records an explicit one-step create and
	// claims its initial sync (w8/m37 t001). It inserts a new Blueprint, or,
	// when retryOf names BlueprintCreateRetry's answer, continues on that row
	// under a bumped generation (w8/m153). store.ErrBlueprintSyncBusy reports
	// a create of the same file already applying, or a retry target that
	// changed since the caller looked.
	AdmitBlueprintCreate(ctx context.Context, b store.Blueprint, run store.BlueprintSync, retryOf string) (store.Blueprint, store.BlueprintSync, error)
	// SetBlueprintSyncNote annotates an already-admitted run (w4/120).
	SetBlueprintSyncNote(ctx context.Context, runID, note string) error
	// StageBlueprintManifest stores the admitted sync's preflighted manifest,
	// fencing on the admitted generation (w8/m37 t002/t005).
	StageBlueprintManifest(ctx context.Context, id, tenantID string, generation int64, runID, manifest string) (store.Blueprint, error)
	// AssertBlueprintExecution confirms the admitted (generation, runID) still
	// owns the claim before a resource-mutation family (w8/m39).
	AssertBlueprintExecution(ctx context.Context, id, tenantID string, generation int64, runID string) error
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
	// flipping its Blueprint to error and bumping generation only while the
	// abandoned generation still owns it (w8/m37 t004 + w8/m39). False means
	// another writer settled first.
	AbandonBlueprintSync(ctx context.Context, runID string, now time.Time, reason string) (bool, error)
	// ClaimBlueprintResource takes (or keeps) the durable workspace claim for
	// one resource name (w8/m40). expectedOwner "" adopts only when absent or
	// already ours; non-empty transfers only while that owner still holds it.
	ClaimBlueprintResource(ctx context.Context, tenantID string, kind store.BlueprintClaimKind, name, blueprintID, expectedOwner string) error
	// ReleaseBlueprintResourceClaims drops every durable claim for a
	// disconnected Blueprint (w8/m40).
	ReleaseBlueprintResourceClaims(ctx context.Context, tenantID, blueprintID string) error
	// ListBlueprintResourceClaims lists durable claims owned by blueprintID —
	// also what a sync diffs against the manifest's declarations (w4/m125).
	ListBlueprintResourceClaims(ctx context.Context, tenantID, blueprintID string) ([]store.BlueprintResourceClaim, error)
	// GetBlueprintResourceOwner returns the owning blueprint id, or "" if none.
	GetBlueprintResourceOwner(ctx context.Context, tenantID string, kind store.BlueprintClaimKind, name string) (string, error)
	// BlueprintResourceOwners maps every claimed resource in the workspace to
	// its owning blueprint id (w5/114).
	BlueprintResourceOwners(ctx context.Context, tenantID string) (map[store.BlueprintResourceKey]string, error)
	// ReleaseBlueprintResourceClaim drops one claim, and only while this
	// blueprint still holds it (w4/m125).
	ReleaseBlueprintResourceClaim(ctx context.Context, tenantID string, kind store.BlueprintClaimKind, name, blueprintID string) error
}

// errBlueprintSyncBusy is the one documented 409 for every lifecycle fencing
// outcome (admission contention, fenced stage/completion/assert, disconnect-
// busy): one coded error through REST, GraphQL, MCP, and the dashboard
// (w8/m37 t002 + w8/m39).
func errBlueprintSyncBusy(msg string) error {
	return core.NewConflictError("BLUEPRINT_SYNC_BUSY", msg, nil)
}

// errBlueprintSourceChanged is the documented 409 when a manual sync's
// reviewed-source precondition no longer matches the Blueprint's configured
// repo/path (w8/m41). Callers must refresh the preview and confirm again —
// the apply never silently follows a moved branch tip.
func errBlueprintSourceChanged(msg string, params map[string]any) error {
	return core.NewConflictError("BLUEPRINT_SOURCE_CHANGED", msg, params)
}

// ReviewedBlueprintSource is the optional manual-sync precondition (w8/m41):
// when set, sync fetches that immutable commit and refuses repo/path drift
// instead of re-resolving the branch HEAD. Omit (nil / empty CommitID) to
// keep today's HEAD-resolve semantics — auto-sync and older clients.
type ReviewedBlueprintSource struct {
	Repo     string // optional; CanonicalRepo-must-match the Blueprint row
	Path     string // required with CommitID; must match the configured path
	CommitID string // immutable SHA from preview.commitId
	// PlannedRunID and EnvVarValues are set only by approving a planned sync
	// (w2/m256): admission turns the planned row into the run, and the manual
	// env var values approval supplied are seeded once by the apply.
	PlannedRunID string
	EnvVarValues map[string]string
}

// optionalReviewedBlueprintSource builds the pin from adapter fields; nil when
// every field is empty (omit-precondition / old clients).
func optionalReviewedBlueprintSource(repo, path, commitID string) *ReviewedBlueprintSource {
	if commitID == "" && repo == "" && path == "" {
		return nil
	}
	return &ReviewedBlueprintSource{Repo: repo, Path: path, CommitID: commitID}
}

// matchReviewedBlueprintSource rejects incomplete pins and settings that
// drifted between preview and confirm. It does not fetch Git.
func matchReviewedBlueprintSource(b store.Blueprint, reviewed *ReviewedBlueprintSource) error {
	if reviewed == nil || reviewed.CommitID == "" {
		return fmt.Errorf("%w: reviewed commitId is required", core.ErrBadRequest)
	}
	if !core.ValidCommitSHA(reviewed.CommitID) {
		return fmt.Errorf("%w: reviewed commitId must be a full git object id", core.ErrBadRequest)
	}
	if reviewed.Path == "" {
		return fmt.Errorf("%w: reviewed path is required with commitId", core.ErrBadRequest)
	}
	cleanConfigured, err := approvedBlueprintPath(b.Path)
	if err != nil {
		return err
	}
	cleanReviewed, err := approvedBlueprintPath(reviewed.Path)
	if err != nil {
		return fmt.Errorf("%w: reviewed path must be a clean repository-relative .yaml/.yml path", core.ErrBadRequest)
	}
	if cleanReviewed != cleanConfigured {
		return errBlueprintSourceChanged(
			"blueprint path no longer matches the reviewed source; refresh the preview and confirm again",
			map[string]any{
				"expectedPath":   cleanConfigured,
				"reviewedPath":   cleanReviewed,
				"expectedCommit": reviewed.CommitID,
			})
	}
	if reviewed.Repo != "" && core.CanonicalRepo(reviewed.Repo) != core.CanonicalRepo(b.Repo) {
		return errBlueprintSourceChanged(
			"blueprint repository no longer matches the reviewed source; refresh the preview and confirm again",
			map[string]any{
				"expectedRepo":   b.Repo,
				"reviewedRepo":   reviewed.Repo,
				"expectedCommit": reviewed.CommitID,
			})
	}
	return nil
}

// errBlueprintExecutionLost is the busy conflict for a mid-apply assert that
// lost authority — actionable: start an explicit new sync; partial work is
// never replayed automatically.
func errBlueprintExecutionLost() error {
	return errBlueprintSyncBusy("this sync no longer owns blueprint execution (superseded, interrupted, or disconnected); start a new sync — partial work is never replayed automatically")
}

// isBlueprintBusy reports a lost (or never held) execution claim from the store.
func isBlueprintBusy(err error) bool {
	return errors.Is(err, store.ErrBlueprintSyncBusy)
}

// requireBlueprintExecution asserts the DeployRequest's admitted claim still
// holds before a mutation family. Non-blueprint / unguarded deploys (empty id
// or zero generation) are no-ops.
func (s *Service) requireBlueprintExecution(ctx context.Context, req DeployRequest) error {
	if req.BlueprintID == "" || req.BlueprintGeneration == 0 || req.BlueprintRunID == "" || s.Blueprints == nil {
		return nil
	}
	tenantID, ok := s.Tenant(ctx)
	if !ok {
		tenantID = s.resolveTenantID(ctx)
	}
	if tenantID == "" {
		return nil
	}
	if err := s.Blueprints.AssertBlueprintExecution(ctx, req.BlueprintID, tenantID, req.BlueprintGeneration, req.BlueprintRunID); err != nil {
		if isBlueprintBusy(err) {
			return errBlueprintExecutionLost()
		}
		return err
	}
	return nil
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

// classifyBlueprintFetchError turns a Blueprint fetch failure into a coded
// reason plus a sentence an author can act on.
//
// This is the whole point of w2/m97: every failure used to arrive as the raw
// internal error, so "you pushed to a branch that does not exist" and "your
// GitHub app was uninstalled" both read as `github: unexpected status 404`, and
// a missing file quoted the pinned commit SHA at the user. Nothing this
// function returns contains upstream vocabulary or a SHA — the message is built
// here from the caller's own inputs, never from err.Error().
func classifyBlueprintFetchError(err error, requestedPath, branch string) (BlueprintPreviewReason, string) {
	displayPath := requestedPath
	if displayPath == "" {
		displayPath = CanonicalBlueprintFilename
	}
	switch {
	case errors.Is(err, ErrBlueprintFilenameAmbiguous):
		return BlueprintFetchAmbiguousFilename, fmt.Sprintf(
			"This repository has both %s and %s on branch %q. Remove one, or name the file explicitly.",
			CanonicalBlueprintFilename, LegacyBlueprintFilename, branch)
	case errors.Is(err, github.ErrBranchNotFound):
		return BlueprintFetchBranchNotFound, fmt.Sprintf("Branch %q does not exist in this repository.", branch)
	case errors.Is(err, github.ErrRepoNotFoundOrNoAccess):
		return BlueprintFetchRepoNotFound,
			"Repository not found, or bex's GitHub app cannot access it."
	}
	// approvedBlueprintPath's refusals are the only ErrBadRequest that reaches
	// here, and they are about the caller's path, not the repository.
	if errors.Is(err, core.ErrBadRequest) {
		message := err.Error()
		if after, ok := strings.CutPrefix(message, "bad request: "); ok {
			message = after
		}
		return BlueprintFetchInvalidPath, message
	}
	var apiErr *github.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusNotFound:
			// The commit resolved (a branch/repo 404 is caught above), so the
			// only remaining 404 is the file itself.
			return BlueprintFetchFileNotFound, fmt.Sprintf(
				"Blueprint file %q not found on branch %q.", displayPath, branch)
		case http.StatusUnauthorized, http.StatusForbidden:
			return BlueprintFetchAccessDenied,
				"bex's GitHub app is not authorized to read this repository. Re-check the installation and its repository access."
		case http.StatusTooManyRequests:
			return BlueprintFetchRateLimited,
				"GitHub is rate-limiting bex right now. Try again in a few minutes."
		}
	}
	return BlueprintFetchUnavailable,
		"Could not reach GitHub to read the Blueprint file. Try again in a few minutes."
}

// blueprintFetchFailure is classifyBlueprintFetchError for the sync paths
// (w2/m97 t003). A sync run records its failure in `blueprint_syncs.error_message`,
// which REST, GraphQL, MCP and the Blueprint detail page all surface — so the
// raw `github:` text and the pinned SHA were leaking there too, not just into
// the preview.
//
// The message is `"<reason>: <sentence>"`. The leading token is a stable,
// machine-keyable code so a client does not parse prose, and it needs no new
// column or migration: the existing error_message carries it. Split on the
// first ": " to read the code.
func blueprintFetchFailure(err error, requestedPath, branch string) error {
	reason, message := classifyBlueprintFetchError(err, requestedPath, branch)
	return fmt.Errorf("%s: %s", reason, message)
}

// blueprintCreateFetchError is classifyBlueprintFetchError for the create
// path's HTTP answer (w8/134). blueprintFetchFailure's plain error is right for
// sync history but carried no status, so a create answered every fetch failure
// as a bare 500 and threw the classified sentence away. The caller's repo,
// branch, path or GitHub access is a 400; GitHub being rate-limited or
// unreachable a 503. The reason is the error code, the same token the preview
// and sync history carry.
func blueprintCreateFetchError(err error, requestedPath, branch string) error {
	reason, message := classifyBlueprintFetchError(err, requestedPath, branch)
	if reason == BlueprintFetchRateLimited || reason == BlueprintFetchUnavailable {
		return core.NewUnavailableError(string(reason), message, nil)
	}
	return core.NewBadRequestError(string(reason), message, nil)
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
	if !isContainedRepositoryPath(filePath, clean) {
		return "", fmt.Errorf("%w: Blueprint path must be a clean repository-relative path", core.ErrBadRequest)
	}
	switch path.Ext(clean) {
	case ".yaml", ".yml":
		return clean, nil
	default:
		return "", fmt.Errorf("%w: Blueprint path must be a .yaml or .yml file", core.ErrBadRequest)
	}
}

// isContainedRepositoryPath reports whether filePath is already its own clean
// form and stays inside the repository: non-empty, relative, and free of ".."
// and backslash segments. clean is path.Clean(filePath).
func isContainedRepositoryPath(filePath, clean string) bool {
	return filePath != "" && clean != "." && clean == filePath &&
		!path.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, "../") &&
		!strings.Contains(clean, `\`)
}

// BlueprintView is the API shape for a blueprint — all three surfaces return this.
type BlueprintView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// OwnerID is the Blueprint's workspace, required on Render's blueprint
	// objects since the 2026-11 contract (w2/065).
	OwnerID   string              `json:"ownerId"`
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
	// ResourceSuffix is the suffix a create_new Blueprint appends to its
	// resources' names, empty for one that manages the names its file gives.
	// It tells apart several Blueprints made from one file (w8/m153). GraphQL
	// only: Render's blueprint object has no such field.
	ResourceSuffix string `json:"-"`
}

// BlueprintResource is one managed resource returned on by-id reads.
type BlueprintResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// BlueprintSyncView is the API shape for a sync run. ID is the exe- public
// alias of the stored bsr- key (w2/m288).
type BlueprintSyncView struct {
	ID           string  `json:"id"`
	CommitID     string  `json:"commitId,omitempty"`
	State        string  `json:"state"`
	StartedAt    string  `json:"startedAt"`
	CompletedAt  *string `json:"completedAt,omitempty"`
	ErrorMessage *string `json:"errorMessage,omitempty"`
	// Note says what the run did when its state cannot: today, the resources
	// a sync detached from the Blueprint. Absent when there is nothing to
	// report.
	Note string `json:"note,omitempty"`
}

// BlueprintValidationError is Render's validation-error shape.
type BlueprintValidationError struct {
	Code   string  `json:"code,omitempty"`
	Error  string  `json:"error"`
	Line   *int    `json:"line,omitempty"`
	Column *int    `json:"column,omitempty"`
	Path   *string `json:"path,omitempty"`
	// conflict is the ownership conflict this entry reports, if it reports
	// one. Error words it for a surface that takes `confirm`; a planned sync,
	// which has none, words it again from this (w8/251).
	conflict *blueprintOwnershipConflict
}

// BlueprintValidationPlan is structural when current state is unavailable,
// otherwise it reports create/update/noop actions and, with an explicit
// blueprint scope, detach actions for claims the new manifest drops.
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
	Blueprint         BlueprintView       `json:"blueprint"`
	Stack             StackResult         `json:"stack"`
	DetachedResources []BlueprintResource `json:"detachedResources"`
}

// BlueprintPreview is the pre-create dry-run result: the manifest fetched from
// Git plus its validation. A fetch failure (file missing, bad branch) is
// reported in Error with Found=false rather than as a verb error, so the
// dashboard can render Render's "Blueprint file not found on branch" + Retry
// state instead of a toast.
type BlueprintPreview struct {
	Found bool `json:"found"`
	// Branch is the branch previewed: the one asked for, or the repository's
	// default branch when none was (w4/532).
	Branch   string `json:"branch,omitempty"`
	Manifest string `json:"manifest,omitempty"`
	CommitID string `json:"commitId,omitempty"`
	Warning  string `json:"warning,omitempty"`
	// Error is a human-readable sentence for API clients. It never carries
	// upstream error text (`github:`, `unexpected status NNN`) or the pinned
	// commit SHA — see classifyBlueprintFetchError.
	Error string `json:"error,omitempty"`
	// Reason is the machine-readable cause, so a client (and the dashboard)
	// picks its own copy instead of parsing prose. Empty on success.
	Reason     BlueprintPreviewReason `json:"reason,omitempty"`
	Validation *BlueprintValidation   `json:"validation,omitempty"`
	// ExistingResources is the mode the file was reviewed in (adopt or
	// create_new) and ResourceSuffix the suffix a create_new review used; a
	// create passes both back so it makes exactly what was reviewed.
	// MatchedResources are the resources the file declares whose names
	// already exist in the workspace — when Render's creation flow shows its
	// notice and suffixes the new Blueprint's resources (w8/m153). All three
	// are set only for a not-yet-created Blueprint.
	ExistingResources string                 `json:"existingResources,omitempty"`
	ResourceSuffix    string                 `json:"resourceSuffix,omitempty"`
	MatchedResources  []BlueprintResourceRef `json:"matchedResources,omitempty"`
	// SuggestedName is set when an approved Blueprint already has the name
	// the create would use (the repository's, or the one the caller named):
	// a free name derived from it (defaultBlueprintName), for a form to
	// propose instead (w8/250). "" when none is free.
	SuggestedName string `json:"suggestedName,omitempty"`
}

// BlueprintPreviewReason is the closed set of reasons a Blueprint fetch failed.
// It exists because the preview used to pass internal error text straight to
// the client — `github: file "render.yaml" not found at ref "<40-hex>"` — which
// told an author nothing actionable, leaked the upstream vocabulary, and left
// the dashboard rendering every failure as "Blueprint file not found" (w2/m97).
type BlueprintPreviewReason string

const (
	// BlueprintFetchFileNotFound: the repo and branch resolved; the file did not.
	BlueprintFetchFileNotFound BlueprintPreviewReason = "file_not_found"
	// BlueprintFetchBranchNotFound: the repo resolved; the branch did not.
	BlueprintFetchBranchNotFound BlueprintPreviewReason = "branch_not_found"
	// BlueprintFetchRepoNotFound: missing repo and invisible-private repo are
	// one reason on purpose — GitHub answers 404 for both, and distinguishing
	// them would be an existence oracle for private repositories.
	BlueprintFetchRepoNotFound BlueprintPreviewReason = "repo_not_found_or_no_access"
	// BlueprintFetchAccessDenied: 401/403 — the installation exists but is not
	// permitted.
	BlueprintFetchAccessDenied BlueprintPreviewReason = "access_denied"
	// BlueprintFetchRateLimited: 429. Retryable.
	BlueprintFetchRateLimited BlueprintPreviewReason = "rate_limited"
	// BlueprintFetchInvalidPath: the caller's path failed approvedBlueprintPath.
	BlueprintFetchInvalidPath BlueprintPreviewReason = "invalid_path"
	// BlueprintFetchAmbiguousFilename: both render.yaml and the legacy alias exist.
	BlueprintFetchAmbiguousFilename BlueprintPreviewReason = "ambiguous_filename"
	// BlueprintFetchUnavailable: any other upstream status or transport
	// failure. Retryable.
	BlueprintFetchUnavailable BlueprintPreviewReason = "unavailable"
)

// RetryableBlueprintFetch reports whether retrying the same request could
// plausibly succeed without the author changing anything. It is what decides
// whether the dashboard offers a Retry button.
func RetryableBlueprintFetch(reason BlueprintPreviewReason) bool {
	return reason == BlueprintFetchRateLimited || reason == BlueprintFetchUnavailable
}

// CreateBlueprintRequest is the input to CreateBlueprint.
type CreateBlueprintRequest struct {
	Repo         string
	Branch       string
	Path         string            // defaults to render.yaml
	Name         string            // defaults to repo basename
	EnvVarValues map[string]string // values for sync:false env vars
	Confirm      string
	// ExistingResources is Render's existingResources for this one-step
	// create: "" or adopt manages the resources the file names, create_new a
	// separate set whose names carry this Blueprint's suffix (w8/m153).
	ExistingResources string
	// ResourceSuffix is the create_new suffix the caller reviewed in the
	// preview, so the create makes the names the review showed. Empty mints
	// one. It only names resources, so a caller-chosen value grants nothing.
	ResourceSuffix string
}

// blueprintPreviewOptions are the optional inputs of previewBlueprintWith.
type blueprintPreviewOptions struct {
	// ForBlueprintID previews on behalf of an existing Blueprint (its pre-sync
	// dialog), with that Blueprint's own mode and suffix.
	ForBlueprintID string
	// ExistingResources is the mode a not-yet-created Blueprint is reviewed
	// in: adopt (the default) or create_new (w8/m153).
	ExistingResources string
	// Name is the name the caller's create would send, when that is not the
	// repository's (a form that proposes its own default); SuggestedName is
	// worked out against it (w8/250).
	Name string
}

// BlueprintResourceRef names one resource a Blueprint file declares. Kind is
// a BlueprintResourceKind value, the name its plan actions use.
type BlueprintResourceRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

var errBlueprintExistingResourcesMode = fmt.Errorf("%w: existingResources must be adopt or create_new", core.ErrBadRequest)

// blueprintExistingResourcesMode reads Render's existingResources: empty or
// adopt, or create_new.
func blueprintExistingResourcesMode(v string) (string, error) {
	switch v {
	case "", store.BlueprintExistingAdopt:
		return store.BlueprintExistingAdopt, nil
	case store.BlueprintExistingCreateNew:
		return store.BlueprintExistingCreateNew, nil
	}
	return "", errBlueprintExistingResourcesMode
}

// UpdateBlueprintRequest is the partial-update input to UpdateBlueprint.
type UpdateBlueprintRequest struct {
	Name     *string
	AutoSync *bool
	Path     *string
	// Branch moves the Blueprint to another branch of its repository (Render's
	// update-blueprint branch, w2/065). The branch must hold the Blueprint file
	// at the (new or current) path; changing it plans no sync.
	Branch *string
}

// ValidateBlueprint parses a Render Blueprint and returns per-entry errors without
// applying anything. An optional blueprint ID includes owned-resource detachments.
// Reads current state when available and requires can_view.
func (s *Service) ValidateBlueprint(ctx context.Context, ownerID, bexYAML, blueprintID string) (BlueprintValidation, error) {
	ctx, blueprintID, err := s.blueprintRequestScope(ctx, core.RelCanView, blueprintID, ownerID)
	if err != nil {
		return BlueprintValidation{}, err
	}
	return s.blueprintValidationFor(ctx, "", "", bexYAML, blueprintID)
}

// blueprintRequestScope authorizes a validate/preview: on behalf of an existing
// Blueprint it acts in that Blueprint's own workspace (blueprintScope, w4/m172)
// unless ownerID names one; for a new manifest, in the named or default one.
// Like blueprintScope it returns the storage (blp-) form of bpID.
func (s *Service) blueprintRequestScope(ctx context.Context, relation, bpID, ownerID string) (context.Context, string, error) {
	if bpID != "" {
		return s.blueprintScope(ctx, relation, bpID, ownerID)
	}
	if ownerID != "" {
		ctx = core.WithWorkspace(ctx, ownerID)
	}
	return ctx, bpID, s.Authorize(ctx, relation)
}

// unresolvedRepoKey marks a read-only Blueprint validation that does not know
// the repository containing the manifest (the CLI's validate uploads only
// ownerId and the file).
type unresolvedRepoKey struct{}

// blueprintReadSpec is specFromCreate for the read-only validation and plan
// stages. Render defaults an omitted repo to the repository containing the
// Blueprint, so where that is unknown a Git-backed service without one has an
// unresolved source, not a missing one; a runtime: image service still needs
// its image. Creation and apply call specFromCreate and stay strict.
func blueprintReadSpec(ctx context.Context, req CreateRequest) (appv1alpha1.AppSpec, error) {
	if req.Repo == "" && req.Image == "" && req.blueprintBuildSource == "" && !strings.EqualFold(strings.TrimSpace(req.Runtime), buildImage) {
		req.implicitRepo, _ = ctx.Value(unresolvedRepoKey{}).(bool)
	}
	return specFromCreate(req)
}

// withCurrentTier records the tier an existing service already runs on a
// Blueprint request that re-applies it, so a paused tier (w2/m250) is refused
// only as a new selection. existing nil (a create) leaves req unchanged.
func withCurrentTier(req CreateRequest, existing *appv1alpha1.App) CreateRequest {
	if existing != nil {
		req.currentTier = existing.Spec.Tier
	}
	return req
}

// blueprintValidationFor is the read-only dry-run core shared by
// ValidateBlueprint and PreviewBlueprint. repo/branch feed the same parse a
// create would run; both empty for a manifest-only validate.
func (s *Service) blueprintValidationFor(ctx context.Context, repo, branch, bexYAML, blueprintID string) (BlueprintValidation, error) {
	var suffix string
	if blueprintID != "" {
		if s.Blueprints == nil {
			return BlueprintValidation{}, ErrBlueprintsUnavailable
		}
		b, err := s.Blueprints.GetBlueprint(ctx, blueprintID, s.resolveTenantID(ctx))
		if err != nil {
			return BlueprintValidation{}, store.MapError(err)
		}
		suffix = b.ResourceSuffix
	}
	return s.blueprintValidationWithSuffix(ctx, repo, branch, bexYAML, blueprintID, suffix, nil)
}

// blueprintValidationWithSuffix validates bexYAML as the Blueprint blueprintID
// ("" for one not yet created) that manages its resources under suffix. snap
// is the caller's workspace snapshot to read through, nil to take a new one.
func (s *Service) blueprintValidationWithSuffix(ctx context.Context, repo, branch, bexYAML, blueprintID, suffix string, snap *workspaceSnapshot) (BlueprintValidation, error) {
	// One memo for the whole validation: it writes nothing, so every check of
	// the platform's host claims (domains, maintenance URIs) reads one index.
	ctx = withRequestMemo(ctx)
	if repo == "" {
		ctx = context.WithValue(ctx, unresolvedRepoKey{}, true)
	}
	source, ir, problems := compileBlueprintIRFor(bexYAML, suffix)
	if len(problems) > 0 {
		return BlueprintValidation{Errors: blueprintCompilerValidationErrors(problems)}, nil
	}
	st, err := parseCompiledStack(blueprintParseOverrides{repo: repo, branch: branch, containing: true}, source, ir)
	// The validation writes nothing, so its checks share one workspace snapshot.
	if snap == nil {
		snap = s.newWorkspaceSnapshot(ctx)
	}
	var refused blueprintResourceErrors
	if errors.As(err, &refused) {
		// Some declarations were refused; the services that parsed still get
		// their own checks, which do not depend on the refused ones.
		if svcErr := s.validateBlueprintServices(ctx, st, snap); svcErr != nil {
			var more blueprintResourceErrors
			if !errors.As(svcErr, &more) {
				return BlueprintValidation{}, svcErr
			}
			refused = append(refused, more...)
		}
		return BlueprintValidation{Errors: blueprintResourceValidationErrors(source, ir, refused)}, nil
	}
	if err == nil {
		err = s.resolveBlueprintRegistryCredentials(ctx, &st)
	}
	if err == nil {
		err = s.validateBlueprintServices(ctx, st, snap)
	}
	if err == nil {
		err = s.validateBlueprintBuildSources(ctx, st, snap)
	}
	if err == nil {
		// w4/118: fromDatabase and fromService→KeyValue resolve against the
		// WORKSPACE rather than the file, and their checks ran only at apply —
		// so a manifest naming a database that exists nowhere validated
		// `valid: true` with a clean plan, while the same manifest's
		// fromService→service (a same-file-only reference) was rejected. The
		// resolvers, their wording and their ambiguous-name case already
		// existed; only the step was wrong.
		//
		// fromGroup stays deferred, deliberately. Its check is gated on an
		// uncached can_view_sensitive re-assert (preflightBlueprintEnv) because
		// a group link materializes secret values to workload code, and
		// validateBlueprint is a can_view verb — running it here would either
		// refuse a viewer's validate or weaken that gate. Recorded in ADR018's
		// Blueprint row so a manifest author knows which checks are which.
		if entries := s.validateWorkspaceReferences(ctx, source, ir, st, snap); len(entries) > 0 {
			return BlueprintValidation{Errors: entries}, nil
		}
		// The workspace plan's environment cap and network-isolation rule,
		// which the apply enforces (w8/m138 t013).
		if entries := s.blueprintPlanLimitErrors(ctx, source, st.groupings); len(entries) > 0 {
			return BlueprintValidation{Errors: entries}, nil
		}
		// A non-Admin's create in, or move into or out of, a protected
		// environment, which the apply refuses before any write (w3/m281).
		var placed blueprintResourceErrors
		if placed, _, err = s.blueprintProtectedPlacementProblems(ctx, st, snap); len(placed) > 0 {
			return BlueprintValidation{Errors: blueprintResourceValidationErrors(source, ir, placed)}, nil
		}
	}
	if err == nil && repo != "" && s.Blueprints != nil {
		// Ownership conflicts surface in the preview/validation result (the
		// dashboard's create review + pre-sync dialog) so nobody discovers
		// them at apply time; the apply-path preflight still enforces.
		if entries := s.previewOwnershipConflicts(ctx, blueprintID, st, snap); len(entries) > 0 {
			return BlueprintValidation{Errors: entries}, nil
		}
	}
	if err == nil {
		plan := blueprintValidationPlanFromIR(ir, st)
		if actionPlan, available, planErr := s.blueprintActionPlan(ctx, ir, st, blueprintID, snap); planErr != nil {
			if !errors.Is(planErr, core.ErrBadRequest) {
				return BlueprintValidation{}, planErr
			}
			if errors.As(planErr, &refused) {
				return BlueprintValidation{Errors: blueprintResourceValidationErrors(source, ir, refused)}, nil
			}
			msg := blueprintManifestCreateMessage(blueprintValidationMessage(planErr))
			return BlueprintValidation{Errors: []BlueprintValidationError{blueprintValidationError(ir, msg)}}, nil
		} else if available {
			creates, createErr := s.previewStackCreates(ctx, st, actionPlan, snap)
			if createErr != nil {
				return BlueprintValidation{}, createErr
			}
			if len(creates) > 0 {
				return BlueprintValidation{Errors: blueprintResourceValidationErrors(source, ir, creates)}, nil
			}
			plan.Mode = "current_state"
			plan.Actions = actionPlan.Actions
			plan.TotalActions = len(actionPlan.Actions)
		}
		return BlueprintValidation{Valid: true, Plan: &plan, EstimatedPricing: blueprintEstimatedPricing(st)}, nil
	}
	// Per-resource refusals (a host another site serves is a 409, w8/055) are
	// located entries whatever their sentinel.
	if errors.As(err, &refused) {
		return BlueprintValidation{Errors: blueprintResourceValidationErrors(source, ir, refused)}, nil
	}
	if !errors.Is(err, core.ErrBadRequest) {
		return BlueprintValidation{}, err
	}
	return BlueprintValidation{Errors: []BlueprintValidationError{blueprintValidationError(ir, blueprintManifestCreateMessage(blueprintValidationMessage(err)))}}, nil
}

// blueprintValidationMessage drops the ErrBadRequest/ErrConflict sentinel text wherever a
// wrap put it — "service \"x\": bad request: …" as well as a leading one. The
// entry is already a validation error; the sentinel is transport detail.
func blueprintValidationMessage(err error) string {
	msg := strings.ReplaceAll(err.Error(), core.ErrBadRequest.Error()+": ", "")
	return strings.ReplaceAll(msg, core.ErrConflict.Error()+": ", "")
}

// blueprintResourceValidationErrors turns per-resource refusals into entries
// located from the IR: the failing field's line/column when the message names
// one, else the declaration's own.
func blueprintResourceValidationErrors(source *BlueprintSource, ir BlueprintIR, refused blueprintResourceErrors) []BlueprintValidationError {
	out := make([]BlueprintValidationError, 0, len(refused))
	for _, problem := range refused {
		msg := blueprintManifestCreateMessage(blueprintValidationMessage(problem.err))
		var entry BlueprintValidationError
		if resource, ok := blueprintIRResource(ir, problem.kind, problem.name); ok {
			pointer := resource.SourcePath + strings.ReplaceAll(blueprintErrorField(msg), ".", "/")
			if i, ok := blueprintEnvEntryIndex(resource, problem.err, msg); ok {
				pointer = fmt.Sprintf("%s/envVars/%d", resource.SourcePath, i) + blueprintEnvRefField(resource, i, msg)
			} else if i, ok := blueprintDomainIndex(resource, msg); ok {
				pointer = fmt.Sprintf("%s/domains/%d", resource.SourcePath, i)
			} else if rule, ok := blueprintStaticRulePointer(resource, msg); ok {
				pointer = resource.SourcePath + rule
			} else if field, ok := blueprintCodedField(problem.err, resource); ok {
				pointer = resource.SourcePath + "/" + field
			}
			entry = blueprintLocatedError(source, msg, pointer)
		} else {
			entry = blueprintValidationError(ir, msg)
		}
		entry.Code = blueprintRefusalCode(problem.err)
		out = append(out, entry)
	}
	// Parsing visits resource kinds separately; present refusals in source order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line == nil {
			return false
		}
		if out[j].Line == nil {
			return true
		}
		return *out[i].Line < *out[j].Line
	})
	return out
}

// blueprintCodedField is the declared field a coded refusal names. It locates
// the refusal where the message alone can mislead: a Key Value named
// Plan_Cache is refused for its name, not its plan.
func blueprintCodedField(err error, resource BlueprintResourceIR) (string, bool) {
	var coded *core.CodedError
	if !errors.As(err, &coded) {
		return "", false
	}
	field, ok := coded.Params["field"].(string)
	if !ok {
		return "", false
	}
	_, declared := resource.Fields[field]
	return field, declared
}

// blueprintRefusalCode is the code the apply's own refusal carries, so
// preview and apply name a refusal alike (w5/m126).
func blueprintRefusalCode(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// blueprintEnvEntryIndex is the envVars entry a refusal belongs to: the index
// the parser recorded (exact, even for a repeated key, w8/100), else the first
// entry with the key the message names.
func blueprintEnvEntryIndex(resource BlueprintResourceIR, err error, msg string) (int, bool) {
	var entry *envEntryError
	if errors.As(err, &entry) {
		if envVars, _ := resource.Fields["envVars"].Value.([]any); entry.index < len(envVars) {
			return entry.index, true
		}
	}
	return blueprintEnvVarIndex(resource, msg)
}

var blueprintEnvKeyRE = regexp.MustCompile(`envVars\["([^"]+)"\]`)

// blueprintEnvVarIndex resolves a message naming envVars["KEY"] to that entry.
func blueprintEnvVarIndex(resource BlueprintResourceIR, msg string) (int, bool) {
	match := blueprintEnvKeyRE.FindStringSubmatch(msg)
	if match == nil {
		return 0, false
	}
	envVars, _ := resource.Fields["envVars"].Value.([]any)
	for i, raw := range envVars {
		if env, _ := raw.(map[string]any); env["key"] == match[1] {
			return i, true
		}
	}
	return 0, false
}

// blueprintEnvRefField narrows an env-entry refusal that names its
// fromService.type to that authored field (w8/096), when the entry declares it.
func blueprintEnvRefField(resource BlueprintResourceIR, i int, msg string) string {
	if !strings.Contains(msg, "fromService.type") {
		return ""
	}
	envVars, _ := resource.Fields["envVars"].Value.([]any)
	entry, _ := envVars[i].(map[string]any)
	if ref, _ := entry["fromService"].(map[string]any); ref != nil {
		if _, declared := ref["type"]; declared {
			return "/fromService/type"
		}
	}
	return ""
}

var blueprintStaticRuleRE = regexp.MustCompile(`\b(routes|headers)\[(\d+)\]\.(type|source|destination|path|name|value)\b`)

// blueprintStaticRulePointer resolves a static-site rule refusal naming
// routes[i].field or headers[i].field to that authored rule (w8/101): the
// field when the entry declares it, else the entry. An index the resource
// does not declare falls back to the keyword locator.
func blueprintStaticRulePointer(resource BlueprintResourceIR, msg string) (string, bool) {
	match := blueprintStaticRuleRE.FindStringSubmatch(msg)
	if match == nil {
		return "", false
	}
	rules, _ := resource.Fields[match[1]].Value.([]any)
	i, err := strconv.Atoi(match[2])
	if err != nil || i >= len(rules) {
		return "", false
	}
	pointer := fmt.Sprintf("/%s/%d", match[1], i)
	if entry, _ := rules[i].(map[string]any); entry != nil {
		if _, declared := entry[match[3]]; declared {
			pointer += "/" + match[3]
		}
	}
	return pointer, true
}

var blueprintQuotedRE = regexp.MustCompile(`"([^"]+)"`)

// blueprintDomainIndex resolves a hostname refusal to the domains entry it
// quotes (w8/055) — compared case- and trailing-dot-insensitively, as hosts
// are canonicalized before the check that quotes them.
func blueprintDomainIndex(resource BlueprintResourceIR, msg string) (int, bool) {
	domains, _ := resource.Fields["domains"].Value.([]any)
	for _, match := range blueprintQuotedRE.FindAllStringSubmatch(msg, -1) {
		quoted := strings.TrimSuffix(strings.ToLower(match[1]), ".")
		for i, raw := range domains {
			if host, _ := raw.(string); strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".") == quoted {
				return i, true
			}
		}
	}
	return 0, false
}

func blueprintIRResource(ir BlueprintIR, kind BlueprintResourceKind, name string) (BlueprintResourceIR, bool) {
	name = strings.TrimSpace(name)
	for _, resource := range ir.Resources {
		if resource.Kind == kind && resource.Name == name {
			return resource, true
		}
	}
	return BlueprintResourceIR{}, false
}

// blueprintLocatedError is an entry for the JSON pointer: its display path and
// the nearest recorded source location at or above it.
func blueprintLocatedError(source *BlueprintSource, msg, pointer string) BlueprintValidationError {
	entry := BlueprintValidationError{Error: msg}
	if path := blueprintDisplayPath(pointer); path != "" {
		entry.Path = &path
	}
	if source == nil {
		return entry
	}
	location := lookupBlueprintLocation(pointer, source.Locations)
	if location.Line > 0 {
		line := location.Line
		entry.Line = &line
	}
	if location.Column > 0 {
		column := location.Column
		entry.Column = &column
	}
	return entry
}

// blueprintManifestCreateMessage maps create-API field names onto the keys
// a render.yaml author can actually write. publishPath is the only current
// split: REST/MCP create keep that spelling; the pinned schema rejects it.
func blueprintManifestCreateMessage(message string) string {
	if !strings.Contains(message, "publishPath") || strings.Contains(message, "staticPublishPath") {
		return message
	}
	return strings.ReplaceAll(message, "publishPath", "staticPublishPath")
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
// forBlueprintID names the blueprint this preview is on behalf of — the detail
// page's pre-sync dialog passes its own id, `/blueprints/new` passes "".
func (s *Service) PreviewBlueprint(ctx context.Context, ownerID, repo, branch, filePath, forBlueprintID string) (BlueprintPreview, error) {
	return s.previewBlueprintWith(ctx, ownerID, repo, branch, filePath, blueprintPreviewOptions{ForBlueprintID: forBlueprintID})
}

// previewBlueprintWith is PreviewBlueprint with its optional inputs.
//
// For a Blueprint that does not exist yet it also reports which of the file's
// resources already exist in the workspace (w8/m153). Render's creation flow
// "displays a notice if your new Blueprint matches existing resources in your
// workspace" and "appends a suffix to the name of each new resource to prevent
// collisions": a caller that sees matches reviews again with
// existingResources create_new, which validates and plans under a suffix
// minted here and returns it for the create to pass back. The default stays
// adopt, the review of managing the resources the file names.
func (s *Service) previewBlueprintWith(ctx context.Context, ownerID, repo, branch, filePath string, opts blueprintPreviewOptions) (BlueprintPreview, error) {
	ctx, forBlueprintID, err := s.blueprintRequestScope(ctx, core.RelCanViewSensitive, opts.ForBlueprintID, ownerID)
	if err != nil {
		return BlueprintPreview{}, err
	}
	// RelCanViewSensitive is a read relation, so Authorize uses the decision
	// cache. Private repo contents are a sensitive sink: re-check uncached
	// before any Git fetch (codex round-15 #4).
	if err := s.AuthorizeFresh(ctx, core.RelCanViewSensitive); err != nil {
		return BlueprintPreview{}, err
	}
	if s.GitFetcher == nil {
		return BlueprintPreview{}, ErrBlueprintFetchUnavailable
	}
	if opts.ExistingResources != "" && forBlueprintID != "" {
		return BlueprintPreview{}, fmt.Errorf("%w: existingResources is fixed when a Blueprint is created; it cannot be set for an existing Blueprint", core.ErrBadRequest)
	}
	mode, err := blueprintExistingResourcesMode(opts.ExistingResources)
	if err != nil {
		return BlueprintPreview{}, err
	}
	tenantID := s.resolveTenantID(ctx)
	branch, err = s.blueprintBranch(ctx, tenantID, repo, branch)
	if err != nil {
		if reason, message, ok := blueprintBranchRefusal(err); ok {
			return BlueprintPreview{Reason: reason, Error: message}, nil
		}
		return BlueprintPreview{}, err
	}
	contents, commitSHA, discoveredPath, err := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, repo, branch, filePath)
	if err != nil {
		reason, message := classifyBlueprintFetchError(err, filePath, branch)
		return BlueprintPreview{Branch: branch, Reason: reason, Error: message}, nil
	}
	preview := BlueprintPreview{Found: true, Branch: branch, Manifest: contents, CommitID: commitSHA}
	var validation BlueprintValidation
	if forBlueprintID != "" {
		validation, err = s.blueprintValidationFor(ctx, repo, branch, contents, forBlueprintID)
	} else {
		// One workspace snapshot serves the match list and the validation.
		snap := s.newWorkspaceSnapshot(ctx)
		preview.MatchedResources = s.blueprintMatchedResources(ctx, contents, snap)
		preview.ExistingResources = mode
		// Review what the create would do. A create that would continue a
		// failed one (BlueprintCreateRetry) is reviewed as that Blueprint,
		// with its suffix and with its own resources not in conflict with it.
		selfID := ""
		if s.Blueprints != nil {
			switch retry, lookupErr := s.Blueprints.BlueprintCreateRetry(ctx, tenantID, repo, branch, discoveredPath, mode); {
			case lookupErr == nil:
				selfID, preview.ResourceSuffix = retry.ID, retry.ResourceSuffix
			case !errors.Is(lookupErr, store.ErrNotFound):
				return BlueprintPreview{}, lookupErr
			}
		}
		if selfID == "" && mode == store.BlueprintExistingCreateNew {
			preview.ResourceSuffix = store.NewBlueprintResourceSuffix()
		}
		if selfID == "" {
			base := strings.TrimSpace(opts.Name)
			if base == "" {
				base = repoName(repo)
			}
			name, err := s.defaultBlueprintName(ctx, tenantID, base, preview.ResourceSuffix)
			if err != nil {
				return BlueprintPreview{}, err
			}
			if name != base {
				preview.SuggestedName = name
			}
		}
		validation, err = s.blueprintValidationWithSuffix(ctx, repo, branch, contents, selfID, preview.ResourceSuffix, snap)
	}
	if err != nil {
		return BlueprintPreview{}, err
	}
	preview.Validation = &validation
	if filePath == "" && discoveredPath == LegacyBlueprintFilename {
		preview.Warning = "bex.yml is a deprecated filename-only alias; rename it to render.yaml"
	}
	return preview, nil
}

// blueprintMatchedResources lists the services, datastores and environment
// groups manifest declares whose names already exist in the caller's
// workspace, in the file's order, read through snap. A manifest that does not
// compile, or a workspace that cannot be read, matches nothing: the validation
// that follows reports the first, and the create's own checks do not depend on
// this list.
func (s *Service) blueprintMatchedResources(ctx context.Context, manifest string, snap *workspaceSnapshot) []BlueprintResourceRef {
	if s.Client == nil {
		return nil
	}
	source, ir, problems := compileBlueprintIRFor(manifest, "")
	if len(problems) > 0 {
		return nil
	}
	// A partly refused file still names the resources that parsed.
	st, _ := parseCompiledStack(blueprintParseOverrides{containing: true}, source, ir)
	tenantID, scoped := s.Tenant(ctx)
	var matched []BlueprintResourceRef
	if len(st.services) > 0 {
		services, err := snap.services(ctx)
		if err != nil {
			return nil
		}
		for _, svc := range st.services {
			if services[svc.req.Name] != nil {
				matched = append(matched, BlueprintResourceRef{Kind: string(BlueprintResourceService), Name: svc.req.Name})
			}
		}
	}
	if len(st.databases) > 0 {
		databases, err := snap.databases(ctx)
		if err != nil {
			return nil
		}
		for _, db := range st.databases {
			// Any database of that name is a match, a duplicated one included.
			if found, unique := uniqueDatabaseByDisplayName(databases.Items, scoped, tenantID, db.name); found != nil || !unique {
				matched = append(matched, BlueprintResourceRef{Kind: string(BlueprintResourcePostgres), Name: db.name})
			}
		}
	}
	if len(st.keyValues) > 0 {
		keyValues, err := snap.keyValues(ctx)
		if err != nil {
			return nil
		}
		for _, kv := range st.keyValues {
			if found, unique := uniqueKeyValueByDisplayName(keyValues.Items, scoped, tenantID, kv.name); found != nil || !unique {
				matched = append(matched, BlueprintResourceRef{Kind: string(BlueprintResourceKeyValue), Name: kv.name})
			}
		}
	}
	if len(st.envGroups) > 0 && s.EnvGroups != nil {
		groups, err := s.EnvGroups.GroupIDsByName(ctx)
		if err != nil {
			return nil
		}
		for _, group := range st.envGroups {
			if _, ok := groups[group.name]; ok {
				matched = append(matched, BlueprintResourceRef{Kind: string(BlueprintResourceEnvVarGroup), Name: group.name})
			}
		}
	}
	return matched
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
	req.Name = strings.TrimSpace(req.Name)
	tenantID := s.resolveTenantID(ctx)

	if s.GitFetcher == nil {
		return BlueprintView{}, ErrBlueprintFetchUnavailable
	}
	mode, err := blueprintExistingResourcesMode(req.ExistingResources)
	if err != nil {
		return BlueprintView{}, err
	}
	if req.ResourceSuffix != "" && (mode != store.BlueprintExistingCreateNew || !store.ValidBlueprintResourceSuffix(req.ResourceSuffix)) {
		return BlueprintView{}, fmt.Errorf("%w: resourceSuffix is four lowercase letters or digits and applies only to existingResources create_new", core.ErrBadRequest)
	}
	if req.Branch, err = s.blueprintBranch(ctx, tenantID, req.Repo, req.Branch); err != nil {
		return BlueprintView{}, err
	}
	contents, commitSHA, discoveredPath, err := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, req.Repo, req.Branch, req.Path)
	if err != nil {
		return BlueprintView{}, blueprintCreateFetchError(err, req.Path, req.Branch)
	}
	req.Path = discoveredPath

	// A create is a new Blueprint, whatever else tracks this file (w8/m153;
	// Render: "You can create multiple Blueprints from a single YAML file").
	// The one reuse is a retry: a create whose apply failed left its row, and
	// repeating it continues there, with that row's suffix, instead of
	// leaving a second half-made Blueprint beside the first. The fetch above
	// is read-only, which is why the lookup sits after it: the path a create
	// lands on is the DISCOVERED one.
	retryOf, suffix := "", ""
	switch retry, err := s.Blueprints.BlueprintCreateRetry(ctx, tenantID, req.Repo, req.Branch, req.Path, mode); {
	case err == nil:
		// The names a caller reviewed are the ones it gets: a suffix that is
		// not the retried Blueprint's would silently be replaced by it.
		if req.ResourceSuffix != "" && req.ResourceSuffix != retry.ResourceSuffix {
			return BlueprintView{}, fmt.Errorf("%w: this create continues Blueprint %s, whose resources use the suffix %q, not %q; preview again and retry",
				core.ErrConflict, ids.BlueprintPublicID(retry.ID), retry.ResourceSuffix, req.ResourceSuffix)
		}
		retryOf, suffix = retry.ID, retry.ResourceSuffix
	case !errors.Is(err, store.ErrNotFound):
		// Fail closed. An unreadable blueprint table must not be read as "a
		// new Blueprint", which would leave a duplicate beside a failed one.
		return BlueprintView{}, err
	case mode == store.BlueprintExistingCreateNew:
		if suffix = req.ResourceSuffix; suffix == "" {
			suffix = store.NewBlueprintResourceSuffix()
		}
	}
	// A retry keeps the name its Blueprint already has (admission writes a
	// retried row's manifest only), so only a new Blueprint is named here.
	if retryOf == "" {
		if req.Name, err = s.newBlueprintName(ctx, tenantID, req.Name, req.Repo, suffix); err != nil {
			return BlueprintView{}, err
		}
	}

	prepareReq := DeployRequest{Repo: req.Repo, Branch: req.Branch, ContainingSource: true, Manifest: contents, EnvVarValues: req.EnvVarValues, resourceSuffix: suffix}
	parsed, ir, parseErr := compileStack(prepareReq)
	if parseErr != nil {
		return BlueprintView{}, parseErr
	}
	if err := s.requireStackBilling(ctx, parsed); err != nil {
		return BlueprintView{}, err
	}
	preflight := s.newWorkspaceSnapshot(ctx)
	if _, _, err := s.blueprintActionPlan(ctx, ir, parsed, "", preflight); err != nil {
		return BlueprintView{}, err
	}
	// The per-service create checks the apply would run first, now, so a
	// manifest `blueprints validate` refuses answers 400 with no row behind it
	// (w8/166) instead of leaving an auto-syncing Blueprint in error.
	checked := parsed
	if err := s.resolveBlueprintRegistryCredentials(ctx, &checked); err != nil {
		return BlueprintView{}, err
	}
	if err := s.validateBlueprintServices(withRequestMemo(ctx), checked, preflight); err != nil {
		return BlueprintView{}, err
	}
	// One manager per resource, asked before anything is recorded: the apply
	// asks again under the claim it then holds.
	if err := s.refuseUnconfirmedTakeover(ctx, retryOf, req.Confirm, checked, preflight); err != nil {
		return BlueprintView{}, err
	}

	// Admission claims the initial apply and records its run atomically (w8/m37
	// t002): a failed admission blocks every workload side effect, and a
	// create of the same file that is already applying loses with the
	// documented busy conflict.
	now := s.Now().UTC()
	b, run, err := s.Blueprints.AdmitBlueprintCreate(ctx, store.Blueprint{
		TenantID:          tenantID,
		Name:              req.Name,
		Repo:              req.Repo,
		Branch:            req.Branch,
		Path:              req.Path,
		AutoSync:          true,
		Manifest:          contents,
		ExistingResources: mode,
		ResourceSuffix:    suffix,
	}, store.BlueprintSync{
		CommitID:  commitSHA,
		State:     store.BlueprintSyncStateRunning,
		StartedAt: now,
	}, retryOf)
	if err != nil {
		if isBlueprintBusy(err) {
			return BlueprintView{}, errBlueprintSyncBusy("a create of this Blueprint file is already running in this workspace; retry after it settles")
		}
		return BlueprintView{}, err
	}

	prepareReq.Confirm = req.Confirm
	prepareReq.BlueprintID = b.ID
	prepareReq.BlueprintGeneration = b.ExecutionGeneration
	prepareReq.BlueprintRunID = run.ID
	// The apply reads the workspace again: admission serializes this
	// Blueprint's applies, and one admitted before it may have created a
	// resource since the plan read them.
	snap := s.newWorkspaceSnapshot(ctx)
	_, applyErr := s.deployParsedStack(ctx, prepareReq, parsed, snap)

	b, cerr := s.completeAdmittedSync(ctx, b, run, applyErr, "create")
	if cerr != nil {
		return BlueprintView{}, cerr
	}
	// The apply succeeded, so its last write, the ownership stamp, left snap
	// as the apply left the workspace.
	v := toBlueprintView(b)
	v.Resources = s.resolveBlueprintResourcesFromIR(ctx, b, ir, snap)
	return v, nil
}

// newBlueprintName is the name a one-step create gives a new Blueprint. Render
// keeps names unique among approved Blueprints, which its own create and the
// approval already enforce here; this create did not, so two copies of one
// file both took the repository's name (w8/250). A name the caller sent is
// refused when it is taken. A create that sends none gets the default, and is
// refused only when no default is free.
func (s *Service) newBlueprintName(ctx context.Context, tenantID, name, repo, suffix string) (string, error) {
	if name == "" {
		free, err := s.defaultBlueprintName(ctx, tenantID, repoName(repo), suffix)
		if err != nil {
			return "", err
		}
		if free == "" {
			return "", errBlueprintNameTaken(repoName(repo))
		}
		return free, nil
	}
	taken, err := s.blueprintNameTaken(ctx, tenantID, name, "")
	if err != nil {
		return "", err
	}
	if taken {
		return "", errBlueprintNameTaken(name)
	}
	return name, nil
}

// defaultBlueprintNameTries bounds the numbered names defaultBlueprintName
// tries for a Blueprint that has no resource suffix.
const defaultBlueprintNameTries = 20

// defaultBlueprintName is the free name nearest base, the name a new
// Blueprint would otherwise take (the repository's, for a create that names
// none): base itself, or, when an approved Blueprint already has that, base
// with the Blueprint's resource suffix, which is how copies of one file are
// told apart (w8/m153). A Blueprint without a suffix (adopt) gets the first
// free of `<base>-2`, `<base>-3`, …. "" when none is free.
func (s *Service) defaultBlueprintName(ctx context.Context, tenantID, base, suffix string) (string, error) {
	candidates := []string{base}
	if suffix != "" {
		candidates = append(candidates, base+"-"+suffix)
	} else {
		for n := 2; n <= defaultBlueprintNameTries; n++ {
			candidates = append(candidates, fmt.Sprintf("%s-%d", base, n))
		}
	}
	for _, candidate := range candidates {
		taken, err := s.blueprintNameTaken(ctx, tenantID, candidate, "")
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", nil
}

// blueprintNameTaken reports whether an approved Blueprint in the workspace,
// other than exceptID, has name. Without a store there are no Blueprints.
func (s *Service) blueprintNameTaken(ctx context.Context, tenantID, name, exceptID string) (bool, error) {
	if s.Blueprints == nil {
		return false, nil
	}
	return s.Blueprints.BlueprintNameTaken(ctx, tenantID, name, exceptID)
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
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanView, bpID, ownerID)
	if err != nil {
		return BlueprintView{}, err
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

// SyncBlueprint re-applies the blueprint by id. reviewed is optional (w8/m41):
// when non-nil with CommitID set, the apply pins that revision and never
// re-resolves the branch tip; omit for HEAD-resolve (auto-sync / old clients).
func (s *Service) SyncBlueprint(ctx context.Context, bpID, ownerID, bexYAML, confirm string, reviewed *ReviewedBlueprintSource) (SyncBlueprintResult, error) {
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
	if err != nil {
		return SyncBlueprintResult{}, err
	}
	tenantID := s.resolveTenantID(ctx)
	b, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID)
	if err != nil {
		return SyncBlueprintResult{}, store.MapError(err)
	}
	// A Blueprint created through Render's plan-then-approve flow (w2/m256)
	// applies only by approving its plan, which is also where the plan's
	// manual env var values are supplied. A manual sync would skip both.
	if b.Status == store.BlueprintStatusCreated {
		return SyncBlueprintResult{}, core.NewConflictError(BlueprintAwaitingApprovalCode,
			"this Blueprint is waiting for its planned sync to be approved; approve it (or plan a new sync) instead of syncing",
			map[string]any{"blueprintId": ids.BlueprintPublicID(b.ID)})
	}
	return s.runSync(ctx, b, bexYAML, confirm, reviewed, nil)
}

// BlueprintAwaitingApprovalCode refuses a manual sync of a Blueprint that has
// not had a planned sync approved yet.
const BlueprintAwaitingApprovalCode = "BLUEPRINT_AWAITING_APPROVAL"

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
func (s *Service) prepareSyncManifest(ctx context.Context, b store.Blueprint, run store.BlueprintSync, manifest string, envVarValues map[string]string, snap *workspaceSnapshot) (store.Blueprint, *parsedStack, error) {
	parsed, ir, err := compileStack(DeployRequest{Repo: b.Repo, Branch: b.Branch, ContainingSource: true, Manifest: manifest, EnvVarValues: envVarValues, resourceSuffix: b.ResourceSuffix})
	if err != nil {
		return store.Blueprint{}, nil, err
	}
	if err := s.requireStackBilling(ctx, parsed); err != nil {
		return store.Blueprint{}, nil, err
	}
	if _, _, err := s.blueprintActionPlan(ctx, ir, parsed, "", snap); err != nil {
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

// Automatic delivery is a wake-up for the current branch, not a manual
// reviewed-commit pin. Its durable claim also fences admission and each later
// mutation through the existing Blueprint execution authority.
func (s *Service) runSync(ctx context.Context, b store.Blueprint, bexYAML, confirm string, reviewed *ReviewedBlueprintSource, auto *store.BlueprintAutoSyncIntent) (SyncBlueprintResult, error) {
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
	if auto != nil {
		if b.Repo == "" {
			return SyncBlueprintResult{}, fmt.Errorf("%w: automatic Blueprint sync requires a Git source", core.ErrBadRequest)
		}
		if err := s.Blueprints.AssertBlueprintAutoSyncIntent(ctx, auto.ID, auto.ClaimGeneration); err != nil {
			return SyncBlueprintResult{}, errBlueprintExecutionLost()
		}
	}
	now := s.Now().UTC()
	// Only pre-apply, retryable provider failures retain the automatic intent.
	// The public failure stays sanitized through the shared source classifier.
	failSource := func(commit string, sourceErr error) (SyncBlueprintResult, error) {
		reason, _ := classifyBlueprintFetchError(sourceErr, b.Path, b.Branch)
		result, err := s.failSync(ctx, b, commit, now, blueprintFetchFailure(sourceErr, b.Path, b.Branch))
		if auto != nil && RetryableBlueprintFetch(reason) {
			return result, &blueprintAutoSyncRetry{err: err}
		}
		return result, err
	}
	var commitSHA string
	var manifest string
	var prepared *parsedStack
	// useStored covers only legacy non-Git rows (no Repo to fetch from): the
	// one path that still applies the stored manifest. A Git-backed source
	// that cannot be fetched is a failure, never an implicit reapply (w8/m36
	// t003) — the stored manifest is preserved untouched for the next attempt.
	useStored := false

	// Empty reviewed struct is treated as omitted (compat with zero values).
	if reviewed != nil && reviewed.CommitID == "" && reviewed.Repo == "" && reviewed.Path == "" {
		reviewed = nil
	}

	if bexYAML != "" {
		// Explicit supplied-manifest sync (the documented bex extension): no
		// Git commit is consumed, so none is claimed. A reviewed pin would
		// contradict that provenance claim (w8/m41).
		if reviewed != nil {
			return SyncBlueprintResult{}, fmt.Errorf("%w: bexYaml and reviewed commitId cannot both be set", core.ErrBadRequest)
		}
		manifest = bexYAML
	} else if b.Repo != "" {
		if s.GitFetcher == nil {
			if auto != nil {
				return SyncBlueprintResult{}, &blueprintAutoSyncRetry{err: ErrBlueprintFetchUnavailable}
			}
			return s.failSync(ctx, b, "", now, ErrBlueprintFetchUnavailable)
		}
		// Re-validate the stored path before the token-backed fetch (round-6
		// #14): a legacy row written before UpdateBlueprint enforced the
		// allowlist must not become an arbitrary private-file read at sync time.
		if _, pathErr := approvedBlueprintPath(b.Path); pathErr != nil {
			return s.failSync(ctx, b, "", now, pathErr)
		}
		if reviewed != nil {
			// Manual sync with reviewed-source pin (w8/m41): never Resolve the
			// branch tip — fetch exactly the previewed commit. Settings that
			// drifted since review fail before any Git read or admission.
			if matchErr := matchReviewedBlueprintSource(b, reviewed); matchErr != nil {
				// Precondition refusal — no Git read yet; do not invent a history row.
				return SyncBlueprintResult{}, matchErr
			}
			contents, fetchErr := s.GitFetcher.FetchBlueprintFileAtCommit(ctx, tenantID, b.Repo, reviewed.CommitID, b.Path)
			if fetchErr != nil {
				return failSource(reviewed.CommitID, fetchErr)
			}
			commitSHA, manifest = reviewed.CommitID, contents
		} else {
			// Resolve-then-read at one immutable commit (w8/m36 t002): the commit
			// is known before the run is recorded, and any fetch failure below
			// fails the sync without touching the stored manifest. Auto-sync and
			// callers that omit the reviewed pin keep this HEAD-resolve path.
			sha, resolveErr := s.GitFetcher.ResolveBlueprintCommit(ctx, tenantID, b.Repo, b.Branch)
			if resolveErr != nil {
				return failSource("", resolveErr)
			}
			if auto != nil && (!validGitObjectID(sha) || isZeroSHA(sha)) {
				return failSource("", errors.New("GitHub returned an invalid Blueprint source commit"))
			}
			contents, fetchErr := s.GitFetcher.FetchBlueprintFileAtCommit(ctx, tenantID, b.Repo, sha, b.Path)
			if fetchErr != nil {
				return failSource(sha, fetchErr)
			}
			commitSHA, manifest = sha, contents
		}
	} else {
		if reviewed != nil {
			return SyncBlueprintResult{}, fmt.Errorf("%w: reviewed commitId requires a Git-backed blueprint", core.ErrBadRequest)
		}
		useStored = true
	}

	// Admission claims the active apply and records its run atomically (w8/m37
	// t002): at most one admitted apply per Blueprint across replicas, and a
	// failed admission blocks every workload side effect below. A competing
	// live claim loses with the documented busy conflict (recorded in history);
	// a disconnect that lands first fences here the same way.
	run := store.BlueprintSync{CommitID: commitSHA, State: store.BlueprintSyncStateRunning, StartedAt: now}
	var envVarValues map[string]string
	if reviewed != nil {
		run.ID, envVarValues = reviewed.PlannedRunID, reviewed.EnvVarValues
		if reviewed.PlannedRunID != "" {
			ctx = withPlannedApply(ctx)
		}
	}
	var admitted store.Blueprint
	var err error
	if auto != nil {
		run.Note = fmt.Sprintf("Automatic delivery accepted commit %s; consumed current branch commit %s.", auto.CommitSHA, commitSHA)
		admitted, run, err = s.Blueprints.AdmitBlueprintAutoSyncRun(ctx, *auto, b, run)
	} else {
		admitted, run, err = s.Blueprints.AdmitBlueprintSyncRun(ctx, b.ID, tenantID, run)
	}
	if err != nil {
		if isBlueprintBusy(err) {
			return s.failSync(ctx, b, commitSHA, now, errBlueprintSyncBusy("another sync is already running for this blueprint; retry after it settles"))
		}
		if auto != nil {
			return SyncBlueprintResult{}, &blueprintAutoSyncRetry{err: err}
		}
		return SyncBlueprintResult{}, err
	}
	b = admitted
	// The admitted sync's reads until it writes a resource share one snapshot;
	// staging writes only the store.
	snap := s.newWorkspaceSnapshot(ctx)

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
	if auto != nil {
		// A slow immutable fetch may have finished after the branch moved. The
		// admitted execution serializes applies, so revalidate before staging
		// or any workload mutation. A retry resolves the new head afresh.
		head, err := s.GitFetcher.ResolveBlueprintCommit(ctx, tenantID, b.Repo, b.Branch)
		if err != nil {
			reason, _ := classifyBlueprintFetchError(err, b.Path, b.Branch)
			failure := blueprintFetchFailure(err, b.Path, b.Branch)
			if RetryableBlueprintFetch(reason) {
				return settleStage(&blueprintAutoSyncRetry{err: failure})
			}
			return settleStage(failure)
		}
		if head != commitSHA {
			return settleStage(&blueprintAutoSyncRetry{err: errors.New("automatic Blueprint source advanced before apply; retry current branch")})
		}
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
		staged, parsed, err := s.prepareSyncManifest(ctx, b, run, manifest, envVarValues, snap)
		if err != nil {
			return settleStage(err)
		}
		b, prepared = staged, parsed
	}

	deployReq := DeployRequest{
		BlueprintID:         b.ID,
		BlueprintGeneration: b.ExecutionGeneration,
		BlueprintRunID:      run.ID,
		Repo:                b.Repo,
		Branch:              b.Branch,
		ContainingSource:    true,
		Manifest:            b.Manifest,
		Confirm:             confirm,
		EnvVarValues:        envVarValues,
		resourceSuffix:      b.ResourceSuffix,
	}
	var applyErr error
	if prepared == nil {
		parsed, ir, err := compileStack(deployReq)
		applyErr = err
		if applyErr == nil {
			prepared = &parsed
			_, _, applyErr = s.blueprintActionPlan(ctx, ir, parsed, "", snap)
		}
	}
	baseline := []BlueprintResource{}
	var stack StackResult
	if applyErr == nil {
		var detached []blueprintDetachment
		if detached, err = s.blueprintDetachments(ctx, b.TenantID, b.ID, *prepared, func() (*blueprintActionResolver, error) {
			return newBlueprintActionResolver(ctx, s, parsedStack{}, snap)
		}); err != nil {
			return settleStage(err)
		}
		baseline = make([]BlueprintResource, 0, len(detached))
		for _, resource := range detached {
			baseline = append(baseline, resource.BlueprintResource)
		}
		stack, applyErr = s.deployParsedStack(ctx, deployReq, *prepared, snap)
	}

	b, cerr := s.completeAdmittedSync(ctx, b, run, applyErr, "sync")
	if cerr != nil {
		return SyncBlueprintResult{}, cerr
	}
	if len(baseline) > 0 {
		names := make([]string, 0, len(baseline))
		for _, resource := range baseline {
			names = append(names, fmt.Sprintf("%s %q (%s)", resource.Type, resource.Name, resource.ID))
		}
		note := "Detached from this blueprint: " + strings.Join(names, ", ") + ". These resources continue running and may incur charges."
		if run.Note != "" {
			note = run.Note + "\n" + note
		}
		if err := s.Blueprints.SetBlueprintSyncNote(ctx, run.ID, note); err != nil {
			return SyncBlueprintResult{}, fmt.Errorf("sync applied but recording detached resources failed: %w", err)
		}
	}
	return SyncBlueprintResult{Blueprint: toBlueprintView(b), Stack: stack, DetachedResources: baseline}, nil
}

// ListBlueprintSyncs returns recorded sync runs for a blueprint, newest first.
// cursor is a sync id in either spelling (exe- public, bsr- durable, w2/m288).
func (s *Service) ListBlueprintSyncs(ctx context.Context, bpID, ownerID, cursor string, limit int) ([]BlueprintSyncView, error) {
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanView, bpID, ownerID)
	if err != nil {
		return nil, err
	}
	tenantID := s.resolveTenantID(ctx)
	if _, err := s.Blueprints.GetBlueprint(ctx, bpID, tenantID); err != nil {
		return nil, store.MapError(err)
	}
	runs, err := s.Blueprints.ListBlueprintSyncs(ctx, bpID, ids.BlueprintSyncStorageID(cursor), core.PageLimitOrAbsent(limit))
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
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
	if err != nil {
		return BlueprintView{}, err
	}
	if req.Name == nil && req.AutoSync == nil && req.Path == nil && req.Branch == nil {
		return BlueprintView{}, fmt.Errorf("%w: at least one of name/autoSync/path/branch must be provided", core.ErrBadRequest)
	}
	if req.Branch != nil {
		trimmed := strings.TrimSpace(*req.Branch)
		if trimmed == "" {
			return BlueprintView{}, fmt.Errorf("%w: branch cannot be empty", core.ErrBadRequest)
		}
		req.Branch = &trimmed
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return BlueprintView{}, fmt.Errorf("%w: name cannot be empty", core.ErrBadRequest)
		}
		req.Name = &trimmed
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
	// A rename follows the create's rule (w8/250). A Blueprint that awaits its
	// first approval is checked when it is approved, and a rename that only
	// changes the case of its own name takes no one's.
	if req.Name != nil && !strings.EqualFold(*req.Name, b.Name) && b.Status != store.BlueprintStatusCreated {
		taken, err := s.blueprintNameTaken(ctx, tenantID, *req.Name, b.ID)
		if err != nil {
			return BlueprintView{}, err
		}
		if taken {
			return BlueprintView{}, errBlueprintNameTaken(*req.Name)
		}
	}
	if req.Branch != nil && *req.Branch != b.Branch {
		if s.GitFetcher == nil {
			return BlueprintView{}, ErrBlueprintFetchUnavailable
		}
		path := b.Path
		if req.Path != nil {
			path = *req.Path
		}
		if _, _, _, err := discoverBlueprintFile(ctx, s.GitFetcher, tenantID, b.Repo, *req.Branch, path); err != nil {
			return BlueprintView{}, blueprintCreateFetchError(err, path, *req.Branch)
		}
	}
	var newStatus *string
	// A Blueprint whose first sync awaits approval stays created, as on
	// Render ("status remains created until its first sync has been
	// approved"); approving it with autoSync off settles paused (w5/m283).
	if req.AutoSync != nil && !*req.AutoSync && b.Status != store.BlueprintStatusPaused && b.Status != store.BlueprintStatusCreated {
		st := store.BlueprintStatusPaused
		newStatus = &st
	}
	if req.AutoSync != nil && *req.AutoSync && b.Status == store.BlueprintStatusPaused {
		st := store.BlueprintStatusInSync
		newStatus = &st
	}
	updated, err := s.Blueprints.UpdateBlueprint(ctx, bpID, tenantID, req.Name, req.AutoSync, req.Path, req.Branch, newStatus, nil)
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
	ctx, bpID, err := s.blueprintScope(ctx, core.RelCanCreate, bpID, ownerID)
	if err != nil {
		return err
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
	if err := s.Blueprints.ReleaseBlueprintResourceClaims(ctx, tenantID, bpID); err != nil {
		log.Printf("blueprint %s: disconnect claim release failed: %v", bpID, err)
		return fmt.Errorf("blueprint disconnected but claim release failed: %w", err)
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
	// Used only when the upsert inserts: an existing row keeps its name.
	name := repoName(req.Repo)
	if free, err := s.defaultBlueprintName(ctx, tenantID, name, ""); err == nil && free != "" {
		name = free
	}
	b, err := s.Blueprints.UpsertBlueprint(ctx, store.Blueprint{
		TenantID: tenantID,
		Name:     name,
		Repo:     req.Repo,
		Branch:   branch,
		Path:     CanonicalBlueprintFilename,
		AutoSync: true,
		Manifest: req.Manifest,
		Status:   store.BlueprintStatusInSync,
	})
	if err != nil || b.ID == "" || b.Status == "disconnected" {
		return
	}
	// The deploy that reached here just applied this manifest, so the row's
	// in_sync claim has exactly one justification — record it (w4/368):
	// a terminal success run plus last_sync_at, through the same seams the
	// git-connected lifecycle uses (InsertBlueprintSync as failSync writes
	// its terminal row; UpdateBlueprint for the stamp). Without them the
	// detail page said In Sync over "No syncs yet" / "Last synced —"
	// forever, and later deploys never repaired the history.
	now := s.Now().UTC()
	if _, err := s.Blueprints.InsertBlueprintSync(ctx, store.BlueprintSync{
		BlueprintID: b.ID,
		State:       store.BlueprintSyncStateSuccess,
		StartedAt:   now,
		CompletedAt: &now,
	}); err == nil {
		_, _ = s.Blueprints.UpdateBlueprint(ctx, b.ID, tenantID, nil, nil, nil, nil, nil, &now)
	}
}

// blueprintScope scopes ctx to the workspace that owns Blueprint bpID and
// authorizes relation there (core.Base.ScopeByID, ADR072's by-id matrix). An
// explicit ownerID (or a workspace already named on ctx) keeps the caller's
// choice. Without one the id resolves its own workspace, so a caller who
// belongs to several needs no ownerId, as Render's owner-less
// GET /blueprints/{id} implies (w4/m169). A caller outside that workspace gets
// the typed-id 403, audited once like any refusal (w5/m115).
//
// bpID may be the public exs- alias or the durable blp- key (w2/m269); the
// returned id is always the storage form, which callers use for every lookup,
// comparison and write downstream.
func (s *Service) blueprintScope(ctx context.Context, relation, bpID, ownerID string) (context.Context, string, error) {
	bpID = ids.BlueprintStorageID(bpID)
	var owner core.ResourceOwner
	if s.Blueprints != nil {
		owner = func(ctx context.Context) ([]string, error) {
			return store.OwnerWorkspaces(s.Blueprints.BlueprintWorkspace(ctx, bpID))
		}
	}
	ctx, err := s.ScopeByID(ctx, ownerID, owner)
	if err != nil {
		return ctx, bpID, store.MapError(err)
	}
	if err := s.Authorize(ctx, relation); err != nil {
		return ctx, bpID, err
	}
	if s.Blueprints == nil {
		return ctx, bpID, ErrBlueprintsUnavailable
	}
	return ctx, bpID, nil
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
	_, ir, problems := compileBlueprintIRFor(b.Manifest, b.ResourceSuffix)
	if len(problems) > 0 || len(ir.Resources) == 0 {
		return nil
	}
	return s.resolveBlueprintResourcesFromIR(ctx, b, ir, &workspaceSnapshot{s: s, tenantID: b.TenantID})
}

// resolveBlueprintResourcesFromIR resolves ir's declared resources through
// snap, which must postdate any write the view should show.
func (s *Service) resolveBlueprintResourcesFromIR(ctx context.Context, b store.Blueprint, ir BlueprintIR, snap *workspaceSnapshot) []BlueprintResource {
	if len(ir.Resources) == 0 {
		return nil
	}

	tenantID := b.TenantID
	declared := map[BlueprintResourceKind]bool{}
	for _, resource := range ir.Resources {
		declared[resource.Kind] = true
	}
	appByName := map[string]*appv1alpha1.App{}
	dbByName := map[string]*appv1alpha1.Database{}
	keyValueByName := map[string]*appv1alpha1.KeyValue{}
	buildSourceByName := map[string]*appv1alpha1.App{}
	// Each kind the manifest declares is read as the apply reads it, across
	// namespaces, since a workspace's resources may still sit in the shared
	// namespace (ADR043 D8, w5/125); a service as the apply resolves it
	// (w5/m133). A failed read leaves its kind unreported.
	if s.Client != nil {
		if declared[BlueprintResourceService] {
			if services, err := snap.services(ctx); err == nil {
				appByName = services
			}
		}
		if declared[BlueprintResourcePostgres] {
			if dbList, err := snap.databases(ctx); err == nil {
				for i := range dbList.Items {
					d := &dbList.Items[i]
					dbByName[d.Spec.Name] = d
				}
			}
		}
		if declared[BlueprintResourceBuildSource] {
			if carriers, err := snap.buildSources(ctx); err == nil {
				buildSourceByName = carriers
			}
		}
		if declared[BlueprintResourceKeyValue] {
			if keyValueList, err := snap.keyValues(ctx); err == nil {
				for i := range keyValueList.Items {
					kv := &keyValueList.Items[i]
					keyValueByName[kv.Spec.Name] = kv
				}
			}
		}
	}
	envGroupByName := map[string]string{}
	if declared[BlueprintResourceEnvVarGroup] && s.EnvGroups != nil {
		if groups, err := s.EnvGroups.GroupIDsByName(ctx); err == nil {
			envGroupByName = groups
		}
	}

	// The durable claim (preferred) or CR label attributes a resource to b
	// (w8/m40). Unclaimed resources that only match the manifest by name are
	// not reported as managed.
	owners := s.blueprintResourceOwners(ctx, tenantID)
	owned := func(kind store.BlueprintClaimKind, name, labelOwner string) bool {
		return blueprintResourceOwner(owners, kind, name, labelOwner) == b.ID
	}
	var resources []BlueprintResource
	for _, resource := range ir.Resources {
		switch resource.Kind {
		case BlueprintResourceService:
			a, ok := appByName[resource.Name]
			if !ok {
				continue
			}
			if !owned(store.BlueprintClaimService, resource.Name, a.Labels[core.LabelBlueprint]) {
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
			if !owned(store.BlueprintClaimDatabase, resource.Name, d.Labels[core.LabelBlueprint]) {
				continue
			}
			resources = append(resources, BlueprintResource{ID: d.Name, Name: resource.Name, Type: string(BlueprintResourcePostgres)})
		case BlueprintResourceKeyValue:
			kv, ok := keyValueByName[resource.Name]
			if !ok {
				continue
			}
			if !owned(store.BlueprintClaimKeyValue, resource.Name, kv.Labels[core.LabelBlueprint]) {
				continue
			}
			resources = append(resources, BlueprintResource{ID: kv.Name, Name: resource.Name, Type: string(BlueprintResourceKeyValue)})
		case BlueprintResourceEnvVarGroup:
			groupID, ok := envGroupByName[resource.Name]
			if !ok {
				continue
			}
			resources = append(resources, BlueprintResource{ID: groupID, Name: resource.Name, Type: "environment_group"})
		case BlueprintResourceBuildSource:
			carrier, ok := buildSourceByName[resource.Name]
			if !ok || !owned(store.BlueprintClaimBuildSource, resource.Name, carrier.Labels[core.LabelBlueprint]) {
				continue
			}
			resources = append(resources, BlueprintResource{ID: core.AppPublicID(carrier), Name: resource.Name, Type: string(BlueprintResourceBuildSource)})
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
	if strings.Contains(message, "health check path") {
		return ".healthCheckPath"
	}
	if strings.Contains(message, "pre-deploy command") {
		return ".preDeployCommand"
	}
	// A paid-only refusal on a free plan names "instance type", which is not
	// the type field: the declared feature, else the plan, is what changes
	// (w4/416).
	if strings.Contains(message, "paid instance type") {
		switch {
		case strings.Contains(message, "autoscaling"):
			return ".scaling"
		case strings.Contains(message, "disk"):
			return ".disk"
		}
		return ".plan"
	}
	// Hostname refusals contain "name"; they belong to domains (w8/055).
	for _, marker := range []string{"hostname", "wildcard", "this domain", "custom domains"} {
		if strings.Contains(strings.ToLower(message), marker) {
			return ".domains"
		}
	}
	for _, field := range []string{"maintenanceMode", "highAvailability", "readReplicas", "diskSizeGB", "storageAutoscalingEnabled", "connectionPool", "plan", "domains", "schedule", "runtime", "type", "image", "databaseName", "name", "ipAllowList", "renderSubdomainPolicy", "scaling", "staticPublishPath", "publishPath"} {
		if strings.Contains(strings.ToLower(message), strings.ToLower(field)) {
			return "." + field
		}
	}
	if strings.Contains(message, " env ") || strings.Contains(message, "env var") || strings.Contains(message, "envVars[") {
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
	// in_sync claims a recorded apply converged the manifest. Every path
	// that legitimately lands there also stamps last_sync_at — a completed
	// sync (CompleteBlueprintSync settles success and error alike), a
	// git-connected create's first completion, deploy auto-registration
	// since w4/368 — so an in_sync row with no stamp has genuinely never
	// applied (deploy-auto-registered before w4/368, pre-m62 'active'
	// rows) and reads created, agreeing with its empty sync history.
	if status == store.BlueprintStatusInSync && b.LastSyncAt == nil {
		status = store.BlueprintStatusCreated
	}
	v := BlueprintView{
		ID:             ids.BlueprintPublicID(b.ID),
		Name:           b.Name,
		OwnerID:        b.TenantID,
		Repo:           b.Repo,
		Branch:         b.Branch,
		Path:           b.Path,
		AutoSync:       b.AutoSync,
		Manifest:       b.Manifest,
		Status:         status,
		ResourceSuffix: b.ResourceSuffix,
		CreatedAt:      b.CreatedAt,
		UpdatedAt:      b.UpdatedAt,
	}
	if b.LastSyncAt != nil {
		s := b.LastSyncAt.UTC().Format(time.RFC3339)
		v.LastSync = &s
	}
	return v
}

func toBlueprintSyncView(r store.BlueprintSync) BlueprintSyncView {
	v := BlueprintSyncView{
		ID:           ids.BlueprintSyncPublicID(r.ID),
		CommitID:     r.CommitID,
		State:        r.State,
		StartedAt:    r.StartedAt.UTC().Format(time.RFC3339),
		ErrorMessage: r.ErrorMessage,
		Note:         r.Note,
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

// blueprintBranch is the branch a Blueprint preview or create reads: the one
// asked for, or else the repository's default branch, resolved as a service
// create resolves it (ValidateRepo, anonymous for a public repo) (w4/532).
// With no opinion — GitHub unconfigured, or not a github.com repository — a
// branch stays required.
func (s *Service) blueprintBranch(ctx context.Context, tenantID, repo, branch string) (string, error) {
	if repo == "" {
		return "", errBlueprintBranchRequired
	}
	if branch != "" {
		return branch, nil
	}
	if s.GitHub != nil {
		defaultBranch, err := s.GitHub.ValidateRepo(ctx, tenantID, repo)
		if err != nil {
			return "", err
		}
		if defaultBranch != "" {
			return defaultBranch, nil
		}
	}
	return "", errBlueprintBranchRequired
}

var errBlueprintBranchRequired = fmt.Errorf("%w: repo and branch are required", core.ErrBadRequest)

// blueprintBranchRefusal turns a failed default-branch lookup into the
// preview's own reason, so the Review step keeps saying why in one place.
// A missing branch with no default to offer stays a 400.
func blueprintBranchRefusal(err error) (BlueprintPreviewReason, string, bool) {
	var coded *core.CodedError
	switch {
	case errors.As(err, &coded) && coded.Code == github.CodeGitHubRateLimited:
		return BlueprintFetchRateLimited, coded.Error(), true
	case errors.As(err, &coded) && coded.Code == github.CodeGitHubAccessDenied:
		return BlueprintFetchAccessDenied, coded.Error(), true
	case errors.Is(err, core.ErrUnavailable):
		return BlueprintFetchUnavailable, "Could not reach GitHub to read the Blueprint file. Try again in a few minutes.", true
	case errors.Is(err, core.ErrBadRequest) && !errors.Is(err, errBlueprintBranchRequired):
		return BlueprintFetchRepoNotFound, "Repository not found, or bex's GitHub app cannot access it.", true
	}
	return "", "", false
}
