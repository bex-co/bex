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

package github

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"golang.org/x/sync/singleflight"
)

// ConnectionStore is the Service's seam to the control-plane store — the narrow
// slice of Store it needs. *store.PGStore satisfies it; a fake backs the tests.
// nil => the control-plane store is off (BEX_CP_DB_URI unset) and every verb
// reports core.ErrGitHubUnavailable.
type ConnectionStore interface {
	// BindGitConnection atomically enforces BOTH quotas — the workspace's
	// connection fan-in and the installation's workspace fan-out (ADR078 §2) —
	// while inserting or refreshing one binding.
	BindGitConnection(ctx context.Context, c store.GitConnection, maxConnections, maxWorkspaces int) (store.GitConnection, error)
	GetGitConnection(ctx context.Context, workspaceID string) (store.GitConnection, error)
	// ListGitConnections returns a workspace's full connection set, oldest first
	// (ADR078) — the multi-account aggregate the repo picker and list surface read.
	ListGitConnections(ctx context.Context, workspaceID string) ([]store.GitConnection, error)
	// GetGitConnectionByOwner resolves the connection whose account login matches a
	// repo's owner — the exact installation to mint that repo's token from (ADR078 §4).
	GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (store.GitConnection, error)
	// GitConnectionsByInstallation resolves every workspace that has proved a
	// binding for an installation (ADR078 §2, N:N). The push webhook's reverse
	// lookup; empty means act on nothing (§4a).
	GitConnectionsByInstallation(ctx context.Context, installationID int64) ([]store.GitConnection, error)
	// CountGitConnections backs the per-workspace connection quota (ADR078 §2).
	CountGitConnections(ctx context.Context, workspaceID string) (int, error)
	DeleteGitConnection(ctx context.Context, workspaceID string, installationID int64) error
	// The subject-bound, single-use connect transaction (w1/m67 F3): the record
	// that ties "who started this flow" to "who came back from GitHub".
	CreateGitHubConnectTransaction(ctx context.Context, t store.GitHubConnectTransaction) error
	ConsumeGitHubConnectTransaction(ctx context.Context, nonce string) (store.GitHubConnectTransaction, error)
	// The deferred claim selector (ADR078 §3a): an ambiguous claim's already-proved
	// candidate set, held single-use for the few minutes the human needs to choose.
	CreateGitHubClaimSelection(ctx context.Context, sel store.GitHubClaimSelection) error
	GetGitHubClaimSelection(ctx context.Context, id string) (store.GitHubClaimSelection, error)
	ConsumeGitHubClaimSelection(ctx context.Context, id string) (store.GitHubClaimSelection, error)
}

// APIClient is the GitHub REST surface the Service uses — *Client in production,
// a fake in tests. nil => the GitHub App is unconfigured (BEX_GITHUB_APP_* unset)
// and every verb reports core.ErrGitHubUnavailable.
type APIClient interface {
	InstallURL() string
	GetInstallation(ctx context.Context, installationID int64) (Installation, error)
	ListRepos(ctx context.Context, installationID int64) ([]Repo, error)
	ListBranches(ctx context.Context, installationID int64, owner, repo string) ([]string, error)
	ListRepoTree(ctx context.Context, token, owner, repo, path, ref string) ([]RepoTreeEntry, error)
	MintInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error)
	RepoAccessible(ctx context.Context, token, owner, repo string) (bool, error)
	GetCommit(ctx context.Context, token, owner, repo, ref string) (Commit, error)
	GetFileContents(ctx context.Context, token, owner, repo, path, ref string) (FileContents, error)
	GetRepoCommitSHA(ctx context.Context, token, owner, repo, branch string) (string, error)
	OpenDraftPullRequest(ctx context.Context, installationID int64, owner, repo, head, base, title, body string) (PullRequest, error)
}

// InstallationVerifier proves the user completing a browser flow actually
// administers the installation being bound (F2), and powers the ADR078 §3a claim
// flow for already-installed accounts. Implemented by *Client when the App's
// OAuth credentials are configured; nil => connect/claim starts refuse up front
// (§7) and the callback fails closed. Kept an interface so the fake in tests can
// drive accept/reject.
type InstallationVerifier interface {
	VerifyInstallationAdmin(ctx context.Context, code string, installationID int64) (bool, error)
	// AuthorizeURL is the app's OAuth user-authorization endpoint — the claim
	// flow's start, the one GitHub flow that always preserves `state` (§3a).
	AuthorizeURL() string
	// ClaimableInstallations resolves the claim callback's missing installation
	// id: this app's installations the code's user ADMINISTERS.
	ClaimableInstallations(ctx context.Context, code string) ([]Installation, error)
}

// Service manages a workspace's GitHub App connection and lists its repos over
// the injected client + store. Both seams must be present; either nil => 503.
type Service struct {
	*core.Base
	GitHub APIClient       // nil => GitHub App unconfigured
	Store  ConnectionStore // nil => control-plane store off
	// Verifier proves installation administration on the browser connect callback
	// (F2). nil => not configured; the unique-binding gate still applies.
	Verifier InstallationVerifier
	// StateSecret signs the short-lived workspace credential carried through the
	// browser install redirect. Production reuses BEX_GITHUB_APP_PRIVATE_KEY's
	// PEM bytes, so no second platform secret or replica-local state is needed.
	StateSecret []byte
	// DashboardURL is BEX_DASHBOARD_URL — where the install callback redirects
	// the browser after success or with a bounded failure code. Empty => the
	// callback returns JSON instead of redirecting.
	DashboardURL string
	// MaxConnections caps how many GitHub installations ONE workspace may connect
	// (BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE, ADR078 §2; default 10, 0 disables).
	// Bounds one tenant's connection fan-in — and therefore the per-connection
	// GitHub round trips ListRepos makes.
	MaxConnections int
	// MaxWorkspacesPerInstallation is MaxConnections' mirror under N:N
	// (BEX_MAX_WORKSPACES_PER_GIT_INSTALLATION, ADR078 §2; default 10, 0
	// disables): how many workspaces ONE installation may serve, which is also
	// how wide a single push delivery can fan out (§4a).
	MaxWorkspacesPerInstallation int
	runtimeDetectionOnce         sync.Once
	runtimeDetectionCache        *core.TTLCache[RuntimeDetection]
	runtimeDetectionFlight       singleflight.Group
	// Public-repo commit resolve (w4/m108 t004): unauthenticated GitHub
	// GET /commits/{ref} when no App installation exists for the owner.
	// Cached aggressively — GitHub's anonymous limit is 60 req/h/IP.
	publicCommitOnce   sync.Once
	publicCommitCache  *core.TTLCache[cachedPublicCommit]
	publicCommitFlight singleflight.Group
}

// cachedPublicCommit is one (repo, ref) resolve outcome. ok=false is a
// negative cache entry (404/403/422/outage) so private and missing look
// identical and do not burn the anonymous quota on every deploy open.
type cachedPublicCommit struct {
	info store.CommitInfo
	ok   bool
}

const (
	maxGitHubInventoryFanout   = 4
	runtimeDetectionCacheTTL   = 30 * time.Second
	runtimeDetectionUnknownTTL = 5 * time.Second
	repoTreeProbeTimeout       = 5 * time.Second
	// Public commit resolve (w4/m108 t004): short TTLs under GitHub's
	// unauthenticated 60 req/h/IP budget; a hung call must not delay deploy open.
	publicCommitCacheTTL       = 5 * time.Minute
	publicCommitMissTTL        = 2 * time.Minute
	publicCommitResolveTimeout = 5 * time.Second
)

// Connection is the neutral connection view every adapter renders. InstallURL is
// always populated (the connect CTA the human clicks); the rest are set only
// when Connected.
type Connection struct {
	Connected      bool   `json:"connected"`
	AccountLogin   string `json:"accountLogin,omitempty"`
	InstallationID int64  `json:"installationId,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
	InstallURL     string `json:"installUrl"`
}

// configured reports whether both the GitHub App and the store are wired.
func (s *Service) configured() bool { return s.GitHub != nil && s.Store != nil }

// workspaceID is the store key for the caller's connection: the caller's tenant
// when the resolver finds one, else the single-workspace default.
// installURL is the app's install URL, or "" when the app is unconfigured.
func (s *Service) installURL() string {
	if s.GitHub == nil {
		return ""
	}
	return s.GitHub.InstallURL()
}

// StartConnect returns the current connection state plus the install URL the
// admin clicks to install the app (and grant repos). Admin-only — connecting a
// workspace's GitHub is an admin action even though the record lands at the
// callback. ownerID ("" => the caller's default workspace, w6/m18) names the
// workspace to check/connect, membership-checked via core.WithWorkspace like
// every other explicit-target verb. The returned install URL carries that
// resolved workspace in a short-lived signed state credential, so GitHub's
// identity-less callback can safely record against the same workspace.
func (s *Service) StartConnect(ctx context.Context, ownerID string) (Connection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return Connection{}, err
	}
	if !s.configured() {
		return Connection{}, core.ErrGitHubUnavailable
	}
	// ADR078 §7: fail at start, not at the callback. With no verifier every
	// binding is guaranteed to 503 AFTER the user walks the whole GitHub flow —
	// refuse before minting a transaction for an unfinishable attempt.
	if err := s.verifierPreflight(); err != nil {
		return Connection{}, err
	}
	workspaceID := s.WorkspaceOrDefault(ctx)
	// Record WHO is starting this flow (w1/m67 F3). Without it the install URL is
	// a portable bearer credential for "bind an installation to this workspace":
	// an attacker could hand its own URL to a victim GitHub org admin, whose
	// genuine installation would then land in the attacker's workspace.
	subject := ""
	if id, ok := core.IdentityFrom(ctx); ok {
		subject = id.Subject
	}
	if subject == "" {
		return Connection{}, core.ErrForbidden
	}
	installURL, err := s.statefulInstallURL(ctx, workspaceID, subject)
	if err != nil {
		return Connection{}, err
	}
	// The install URL is the only bindable (stateful) URL bex produces — it starts
	// a NEW connect, so it is what "Connect another account" uses too (ADR078 §3).
	// The returned Connected flag reflects whether the workspace already holds any
	// connection, but the URL always adds one.
	rows, err := s.Store.ListGitConnections(ctx, workspaceID)
	if err != nil {
		return Connection{}, err
	}
	if len(rows) == 0 {
		return Connection{Connected: false, InstallURL: installURL}, nil
	}
	conn := s.connectedView(rows[0])
	conn.InstallURL = installURL
	return conn, nil
}

// connectFromCallback is the sole installation-binding path, and it now requires
// THREE proofs that all name the same attempt (w1/m67 F3):
//
//  1. the signed state, carrying an opaque nonce;
//  2. a server-side transaction row for that nonce, atomically consumed here, that
//     records the bex subject and workspace the flow started with; and
//  3. the GitHub user OAuth code, proving the browser principal administers the
//     installation.
//
// Before (3) was tied to (2) by a shared subject, proofs (1) and (3) belonged to
// unrelated principals and were individually portable: an attacker's signed
// install URL, completed by a victim GitHub admin, bound the victim's
// installation to the attacker's workspace. caller is the authenticated bex
// identity presenting the callback; it must equal the transaction's initiator.
// GitHub authorization codes are single-use, and the nonce now is too.
func (s *Service) connectFromCallback(ctx context.Context, nonce, caller string, installationID int64, code string) (Connection, error) {
	txn, err := s.consumeCallbackProofs(ctx, nonce, caller, code)
	if err != nil {
		return Connection{}, err
	}
	ok, err := s.Verifier.VerifyInstallationAdmin(ctx, code, installationID)
	if err != nil {
		return Connection{}, mapGitHubErr(err)
	}
	if !ok {
		return Connection{}, fmt.Errorf("%w: could not verify you administer this GitHub installation", core.ErrForbidden)
	}
	return s.connectWithWorkspace(ctx, txn.TenantID, installationID)
}

// consumeCallbackProofs is the shared, order-sensitive proof prologue of BOTH
// callback branches (install and claim) — one copy so a future fix to any proof
// cannot silently miss the other path:
//
//  1. configured + verifier guards (fail closed);
//  2. code and caller presence — the callback is a top-level GET navigation, so
//     the Lax-scoped bex session cookie does travel with it; no session means we
//     cannot know who is completing the flow and must refuse;
//  3. atomic nonce consumption FIRST — single-use whatever happens next, so a
//     failed or probed callback cannot be retried against a different target;
//  4. initiator == caller (w1/m67 F3);
//  5. fresh can_manage on the transaction's workspace — the callback can arrive
//     minutes after StartConnect/StartClaim, and a demotion inside that window
//     must not still bind (codex round-15 #3). Authz nil still allows (local/dev).
//
// The caller keeps only its installation-resolution tail (verify the browser-
// supplied id for install; resolve server-side for claim).
func (s *Service) consumeCallbackProofs(ctx context.Context, nonce, caller, code string) (store.GitHubConnectTransaction, error) {
	if !s.configured() || s.Verifier == nil {
		return store.GitHubConnectTransaction{}, core.ErrGitHubUnavailable
	}
	if strings.TrimSpace(code) == "" {
		return store.GitHubConnectTransaction{}, fmt.Errorf("%w: GitHub user authorization code is required", core.ErrBadRequest)
	}
	if caller == "" {
		return store.GitHubConnectTransaction{}, fmt.Errorf("%w: sign in to bex before completing the GitHub connection", core.ErrForbidden)
	}
	txn, err := s.Store.ConsumeGitHubConnectTransaction(ctx, nonce)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.GitHubConnectTransaction{}, fmt.Errorf("%w: this GitHub connect link has expired or was already used; start again from bex", core.ErrForbidden)
		}
		return store.GitHubConnectTransaction{}, err
	}
	if txn.Subject != caller {
		return store.GitHubConnectTransaction{}, fmt.Errorf("%w: this GitHub connect link was started by a different bex user", core.ErrForbidden)
	}
	freshCtx := core.WithIdentity(ctx, core.Identity{Subject: txn.Subject, Method: "session"})
	if err := s.AuthorizeFreshOn(freshCtx, core.RelCanManage, core.WorkspaceObject(txn.TenantID)); err != nil {
		return store.GitHubConnectTransaction{}, err
	}
	return txn, nil
}

// Claim is StartClaim's result: the GitHub OAuth authorize URL that starts the
// ADR078 §3a claim flow for an already-installed account.
type Claim struct {
	ClaimURL string `json:"claimUrl"`
}

// claimSelectionTTL bounds how long an ambiguous claim's proved candidate set
// stays offered. Short: the human is mid-flow, sitting in front of the picker.
const claimSelectionTTL = 5 * time.Minute

// Bounded claim-callback failures (ADR078 §3a) — mapped to fixed git_error codes
// in rest.go; the messages are safe for the JSON (no-dashboard) mode.
//
// Both messages were rewritten for N:N (w2/m162). The old no-claimable copy said
// "install the bex GitHub App on the account first", which after §2 is advice for
// a state that can no longer arise from being bound elsewhere — the App is
// installed; the claim just found nothing this user administers. The old
// ambiguity copy told the user to uninstall the extras; the remedy is now to pick
// one, and errAmbiguousClaim survives only for the case where the selection could
// not be recorded at all.
var (
	errNoClaimableInstallation = fmt.Errorf("%w: no GitHub account you administer was found; check that you authorized the right GitHub user, or install the bex GitHub App on the account first", core.ErrBadRequest)
	errAmbiguousClaim          = fmt.Errorf("%w: several GitHub accounts you administer were found and the choice could not be recorded; start the claim again", core.ErrBadRequest)
	errClaimSelectionGone      = fmt.Errorf("%w: this GitHub account choice has expired or was already used; start the claim again", core.ErrForbidden)
)

// claimSelectionRequiredError is not a failure — it is the ambiguous branch
// SUCCEEDING into a deferred choice. It carries the selection id so the callback
// can redirect the browser to the picker instead of a dead-end error code.
type claimSelectionRequiredError struct{ SelectionID string }

func (e *claimSelectionRequiredError) Error() string {
	return "github claim: several administered installations; selection " + e.SelectionID + " is pending"
}

// verifierPreflight is ADR078 §7: connect/claim starts refuse immediately when
// the installation-admin verifier is unconfigured, because the callback would
// fail closed anyway — after the user completed the whole GitHub round trip.
func (s *Service) verifierPreflight() error {
	if s.Verifier == nil {
		return fmt.Errorf("%w: GitHub App OAuth verification is not configured (BEX_GITHUB_APP_CLIENT_ID/BEX_GITHUB_APP_CLIENT_SECRET); ask your platform operator", core.ErrGitHubUnavailable)
	}
	return nil
}

// StartClaim begins the ADR078 §3a claim flow: bind an installation that ALREADY
// exists on GitHub (the direct-install case) to ownerID's workspace ("" => the
// caller's default). GitHub strips the signed state from the install URL for
// already-installed accounts, so the claim rides the OAuth user-authorization
// flow instead — the one flow that always preserves state — and the callback
// resolves the installation server-side from the authorizing user's admin set.
// Admin-only, same transaction record as StartConnect (w1/m67 F3).
//
// installationID (0 = unspecified) is the optional start-time selector: it
// NARROWS the candidate set the callback proves. It is not trusted as an input —
// an installation the authorizing GitHub user does not administer never becomes a
// candidate no matter what is named here.
func (s *Service) StartClaim(ctx context.Context, ownerID string, installationID int64) (Claim, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return Claim{}, err
	}
	if !s.configured() {
		return Claim{}, core.ErrGitHubUnavailable
	}
	if err := s.verifierPreflight(); err != nil {
		return Claim{}, err
	}
	workspaceID := s.WorkspaceOrDefault(ctx)
	subject := ""
	if id, ok := core.IdentityFrom(ctx); ok {
		subject = id.Subject
	}
	if subject == "" {
		return Claim{}, core.ErrForbidden
	}
	token, err := s.mintConnectState(ctx, workspaceID, subject, installationID)
	if err != nil {
		return Claim{}, err
	}
	return Claim{ClaimURL: s.Verifier.AuthorizeURL() + "&state=" + url.QueryEscape(token)}, nil
}

// claimFromCallback is the claim flow's callback half: it arrives with code +
// state and NO installation_id (that absence is what selects this branch), runs
// the identical proof sequence as connectFromCallback — consume the single-use
// nonce, match the initiator, fresh can_manage — and then resolves the
// installation server-side from this app's installations the code's user
// ADMINISTERS.
//
// ADR078 §3a (2026-09-16): the old "and not already bound to ANY workspace"
// filter is GONE. Under N:N (§2) being bound elsewhere no longer disqualifies an
// installation, because this callback carries a full, independent proof that the
// human administers it — so the previous behaviour (drop it, then report
// "install the App first") was both wrong and actively misleading. What survives
// is scoped to this transaction's own workspace: a binding this workspace
// already holds stays a candidate, making a repeated claim idempotent.
//
// A start-time installation id NARROWS the proved set and can never add to it.
// Ambiguity is no longer a dead end: the proved set is handed to the human as a
// single-use selection (errClaimSelectionRequired) rather than discarded.
func (s *Service) claimFromCallback(ctx context.Context, nonce, caller, code string) (Connection, error) {
	txn, err := s.consumeCallbackProofs(ctx, nonce, caller, code)
	if err != nil {
		return Connection{}, err
	}
	admined, err := s.Verifier.ClaimableInstallations(ctx, code)
	if err != nil {
		return Connection{}, mapGitHubErr(err)
	}
	candidates := make([]Installation, 0, len(admined))
	for _, inst := range admined {
		// The client-supplied id intersects the server-proved set; it is never a
		// source of candidates, only a filter over them.
		if txn.InstallationID > 0 && inst.ID != txn.InstallationID {
			continue
		}
		candidates = append(candidates, inst)
	}
	switch len(candidates) {
	case 0:
		return Connection{}, errNoClaimableInstallation
	case 1:
		return s.connectWithWorkspace(ctx, txn.TenantID, candidates[0].ID)
	default:
		return Connection{}, s.offerClaimSelection(ctx, txn, candidates)
	}
}

// offerClaimSelection persists an ambiguous claim's ALREADY-PROVED candidate set
// and returns the sentinel that routes the browser to the picker (ADR078 §3a).
//
// The dashboard cannot know any installation id before the OAuth round trip — the
// set is only discoverable with the user token minted from the single-use code —
// so discarding it here is what made ambiguity unresolvable. Every proof has
// already run for every member: state, nonce, initiator, fresh can_manage, and
// VerifyInstallationAdmin. The row is a memo of that, not a substitute for it:
// subject-bound, workspace-bound, single-use, short-lived, and closed to ids
// outside the set. The OAuth code is spent and deliberately not stored.
//
// A failure to persist degrades to the ordinary bounded ambiguity error rather
// than inventing a binding.
func (s *Service) offerClaimSelection(ctx context.Context, txn store.GitHubConnectTransaction, candidates []Installation) error {
	selectionID := id.New(id.GitClaimSelection)
	rows := make([]store.GitHubClaimCandidate, 0, len(candidates))
	for _, inst := range candidates {
		rows = append(rows, store.GitHubClaimCandidate{InstallationID: inst.ID, AccountLogin: inst.AccountLogin})
	}
	if err := s.Store.CreateGitHubClaimSelection(ctx, store.GitHubClaimSelection{
		ID:          selectionID,
		WorkspaceID: txn.TenantID,
		Subject:     txn.Subject,
		Candidates:  rows,
		ExpiresAt:   s.Now().Add(claimSelectionTTL),
	}); err != nil {
		log.Printf("github claim: could not record selection for workspace %s: %v", txn.TenantID, err)
		return errAmbiguousClaim
	}
	return &claimSelectionRequiredError{SelectionID: selectionID}
}

// ClaimCandidate is one option the picker renders.
type ClaimCandidate struct {
	InstallationID int64  `json:"installationId"`
	AccountLogin   string `json:"accountLogin"`
}

// ClaimSelection is the picker's view of an outstanding ambiguous claim.
type ClaimSelection struct {
	ID         string           `json:"id"`
	Candidates []ClaimCandidate `json:"candidates"`
	ExpiresAt  string           `json:"expiresAt"`
}

// GetClaimSelection renders an outstanding selection for the picker WITHOUT
// consuming it. Admin-gated on ownerID's workspace ("" => the caller's default,
// ADR078 §6) and subject-matched on top: a selection id is a name, not a
// capability, so it reveals its candidates only to the bex user who started the
// claim, inside the workspace that claim was for. Unknown, expired, foreign-
// workspace and foreign-subject selections are all refused identically.
func (s *Service) GetClaimSelection(ctx context.Context, ownerID, selectionID string) (ClaimSelection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return ClaimSelection{}, err
	}
	sel, err := s.loadSelection(ctx, selectionID, false)
	if err != nil {
		return ClaimSelection{}, err
	}
	out := ClaimSelection{ID: sel.ID, ExpiresAt: sel.ExpiresAt.UTC().Format(time.RFC3339)}
	for _, c := range sel.Candidates {
		out.Candidates = append(out.Candidates, ClaimCandidate{InstallationID: c.InstallationID, AccountLogin: c.AccountLogin})
	}
	return out, nil
}

// SelectClaim completes an ambiguous claim by binding one installation the
// callback already proved. Admin-gated on ownerID's workspace.
//
// SECURITY: this grants nothing the callback had not established. The selection
// is consumed atomically (so a replay finds nothing), can_manage is re-checked
// NOW rather than inherited from the callback (a demotion inside the selection
// window must not still bind), the presenting subject must equal the initiator,
// the selection's workspace must be the authorized one, and the installation must
// be a member of the stored set — so the client chooses among proved options and
// can never introduce a new one.
func (s *Service) SelectClaim(ctx context.Context, ownerID, selectionID string, installationID int64) (Connection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return Connection{}, err
	}
	sel, err := s.loadSelection(ctx, selectionID, true)
	if err != nil {
		return Connection{}, err
	}
	for _, c := range sel.Candidates {
		if c.InstallationID == installationID {
			return s.connectWithWorkspace(ctx, sel.WorkspaceID, c.InstallationID)
		}
	}
	return Connection{}, fmt.Errorf("%w: that GitHub account is not one of this claim's options", core.ErrBadRequest)
}

// loadSelection is the shared guard of both selection verbs — one copy so the
// read and the write cannot drift on who may see a pending choice. consume
// distinguishes them: peek (render the picker) vs spend it (bind). Unknown,
// expired, foreign-subject and foreign-workspace all collapse to one
// indistinguishable refusal, so a selection id cannot be probed.
func (s *Service) loadSelection(ctx context.Context, selectionID string, consume bool) (store.GitHubClaimSelection, error) {
	if !s.configured() {
		return store.GitHubClaimSelection{}, core.ErrGitHubUnavailable
	}
	caller := ""
	if ident, ok := core.IdentityFrom(ctx); ok {
		caller = ident.Subject
	}
	if caller == "" {
		return store.GitHubClaimSelection{}, core.ErrForbidden
	}
	load := s.Store.GetGitHubClaimSelection
	if consume {
		load = s.Store.ConsumeGitHubClaimSelection
	}
	sel, err := load(ctx, selectionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.GitHubClaimSelection{}, errClaimSelectionGone
		}
		return store.GitHubClaimSelection{}, err
	}
	if sel.Subject != caller || sel.WorkspaceID != s.WorkspaceOrDefault(ctx) {
		return store.GitHubClaimSelection{}, errClaimSelectionGone
	}
	return sel, nil
}

// connectWithWorkspace records a connection for the workspace authenticated by
// a verified state credential. It deliberately is not an exported service verb:
// it has no caller Identity to authorize, and must only be called by the callback
// after the initiator, installation-admin, and current can_manage proofs succeed.
func (s *Service) connectWithWorkspace(ctx context.Context, workspaceID string, installationID int64) (Connection, error) {
	if !s.configured() {
		return Connection{}, core.ErrGitHubUnavailable
	}
	if workspaceID == "" || installationID <= 0 {
		return Connection{}, core.ErrBadRequest
	}
	inst, err := s.GitHub.GetInstallation(ctx, installationID)
	if err != nil {
		return Connection{}, mapGitHubErr(err)
	}
	// SECURITY: the same-pair reconnect exemption, BOTH quota admissions, and the
	// insert are one store transaction. A standalone count here lets two callbacks
	// at limit-1 both pass and exceed the configured bound.
	row, err := s.Store.BindGitConnection(ctx, store.GitConnection{
		WorkspaceID:    workspaceID,
		InstallationID: installationID,
		AccountLogin:   inst.AccountLogin,
	}, s.MaxConnections, s.MaxWorkspacesPerInstallation)
	if err != nil {
		var limit *store.GitConnectionLimitError
		if errors.As(err, &limit) {
			return Connection{}, core.NewConflictError("GIT_CONNECTION_LIMIT",
				fmt.Sprintf("workspace already has %d connected GitHub installations (limit %d); disconnect one or raise the limit", limit.Count, limit.Limit),
				map[string]any{"count": limit.Count, "limit": limit.Limit})
		}
		// The mirror cap (ADR078 §2): how many workspaces one installation serves,
		// which is also how wide a single push delivery can fan out (§4a).
		var wsLimit *store.GitInstallationWorkspaceLimitError
		if errors.As(err, &wsLimit) {
			return Connection{}, core.NewConflictError("GIT_INSTALLATION_WORKSPACE_LIMIT",
				fmt.Sprintf("this GitHub account already serves %d workspaces (limit %d); disconnect it from one or raise the limit", wsLimit.Count, wsLimit.Limit),
				map[string]any{"count": wsLimit.Count, "limit": wsLimit.Limit})
		}
		return Connection{}, err
	}
	return s.connectedView(row), nil
}

// GetConnection returns ownerID's connection status ("" => the caller's default
// workspace, w6/m18) — the singular compatibility alias over the workspace's
// oldest connection (ADR078). "Not connected" is a valid state, not an error, and
// (ADR078 §3) carries NO install URL: the bare, stateless URL is no longer
// advertised as a connect CTA — only the connectGit mutation mints a bindable
// (stateful) one. A connected row keeps its install URL as a "configure grants on
// GitHub" deep link. Member read.
func (s *Service) GetConnection(ctx context.Context, ownerID string) (Connection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Connection{}, err
	}
	if !s.configured() {
		return Connection{}, core.ErrGitHubUnavailable
	}
	row, err := s.Store.GetGitConnection(ctx, s.WorkspaceOrDefault(ctx))
	if errors.Is(err, store.ErrNotFound) {
		return Connection{Connected: false}, nil
	}
	if err != nil {
		return Connection{}, err
	}
	return s.connectedView(row), nil
}

// ListConnections returns every GitHub installation ownerID's workspace has
// connected ("" => the caller's default workspace), oldest first — the
// multi-account surface (ADR078 §5). An empty slice (never an error) means no
// connection; the caller starts one through the connectGit mutation. Each row's
// InstallURL is the bare "configure grants on GitHub" deep link. Member read.
func (s *Service) ListConnections(ctx context.Context, ownerID string) ([]Connection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, core.ErrGitHubUnavailable
	}
	rows, err := s.Store.ListGitConnections(ctx, s.WorkspaceOrDefault(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]Connection, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.connectedView(row))
	}
	return out, nil
}

// Disconnect removes one of ownerID's connections ("" => the caller's default
// workspace, w6/m18). installationID names the exact connection to remove; 0
// targets the sole connection (the singular-alias behavior) and is refused with
// ErrConflict when the workspace holds several — an ambiguous "disconnect" must
// not silently pick one. Idempotent: disconnecting when not connected is a no-op
// success. Admin-only.
func (s *Service) Disconnect(ctx context.Context, ownerID string, installationID int64) error {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return err
	}
	if !s.configured() {
		return core.ErrGitHubUnavailable
	}
	workspace := s.WorkspaceOrDefault(ctx)
	if installationID <= 0 {
		rows, err := s.Store.ListGitConnections(ctx, workspace)
		if err != nil {
			return err
		}
		switch len(rows) {
		case 0:
			return nil // idempotent no-op
		case 1:
			installationID = rows[0].InstallationID
		default:
			return fmt.Errorf("%w: this workspace has multiple GitHub connections; specify which installation to disconnect", core.ErrConflict)
		}
	}
	err := s.Store.DeleteGitConnection(ctx, workspace, installationID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}

// ListRepos returns the repositories across ALL of ownerID's connected
// installations ("" => the caller's default workspace, w6/m18; private included),
// each annotated with the GitHub account it came from so the picker can group by
// account (ADR078 §4). With no connection the list is empty (not an error). One
// GitHub round trip per connection, run through a fixed worker pool; a single
// connection's failure degrades that account's slice (logged) rather than
// failing the whole list. Member read.
func (s *Service) ListRepos(ctx context.Context, ownerID string) ([]Repo, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, core.ErrGitHubUnavailable
	}
	rows, err := s.Store.ListGitConnections(ctx, s.WorkspaceOrDefault(ctx))
	if err != nil {
		return nil, err
	}
	perConn := make([][]Repo, len(rows))
	errs := make([]error, len(rows))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := min(maxGitHubInventoryFanout, len(rows))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if err := ctx.Err(); err != nil {
					errs[i] = err
					continue
				}
				row := rows[i]
				repos, err := s.GitHub.ListRepos(ctx, row.InstallationID)
				if err != nil {
					errs[i] = mapGitHubErr(err)
					continue
				}
				for j := range repos {
					repos[j].AccountLogin = row.AccountLogin
					repos[j].InstallationID = row.InstallationID
				}
				perConn[i] = repos
			}
		}()
	}
	for i := range rows {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	// A dead/permission-changed installation must not blank out the OTHER
	// accounts' repos: when at least one connection returned repos, degrade the
	// failed ones (logged) and serve what we have. But if EVERY connection failed
	// (the single-connection GitHub-outage case included), surface the error
	// rather than a misleading empty list — this keeps a one-connection workspace
	// byte-identical to the pre-ADR078 behavior.
	failed, total := 0, 0
	var firstErr error
	for i, e := range errs {
		if e != nil {
			failed++
			if firstErr == nil {
				firstErr = e
			}
			log.Printf("github ListRepos: connection account=%s failed: %v", rows[i].AccountLogin, e)
			continue
		}
		total += len(perConn[i])
	}
	// len(rows) > 0 guards the zero-connection case: with no rows, failed == 0 ==
	// len(rows) would otherwise read as "all failed" and return a nil error+slice.
	if len(rows) > 0 && failed == len(rows) {
		return nil, firstErr
	}
	out := make([]Repo, 0, total)
	for _, repos := range perConn {
		out = append(out, repos...)
	}
	return out, nil
}

// ListBranches returns the branch names of repoURL for ownerID's connected
// installation ("" => the caller's default workspace). It degrades to an empty
// list — never an error — for a non-github.com repo, no connection, or a repo
// the installation can't see, so the dashboard falls back to free-text branch
// entry (w5/m54). Member read.
func (s *Service) ListBranches(ctx context.Context, ownerID, repoURL string) ([]string, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, core.ErrGitHubUnavailable
	}
	owner, repo, ok := githubOwnerRepo(repoURL)
	if !ok {
		return []string{}, nil // non-GitHub repo => free-text fallback
	}
	row, err := s.Store.GetGitConnectionByOwner(ctx, s.WorkspaceOrDefault(ctx), owner)
	if errors.Is(err, store.ErrNotFound) {
		return []string{}, nil // no connection for this repo's account => free-text fallback
	}
	if err != nil {
		return nil, err
	}
	branches, err := s.GitHub.ListBranches(ctx, row.InstallationID, owner, repo)
	if err != nil {
		return nil, mapGitHubErr(err)
	}
	if branches == nil {
		branches = []string{}
	}
	return branches, nil
}

// RepoTreeProbe is the typed result of the best-effort repository listing used
// by runtime detection. Unknown is deliberately data, not an error: a missing
// directory, empty repository, rate limit, or GitHub outage must leave the
// create wizard on its existing manual runtime selection path.
type RepoTreeProbe struct {
	Entries []RepoTreeEntry
	Unknown bool
}

type repoTreeTarget struct {
	workspaceID string
	owner       string
	repo        string
	branch      string
	rootDir     string
}

func (t repoTreeTarget) cacheKey() string {
	return strings.Join([]string{t.workspaceID, t.owner, t.repo, t.branch, t.rootDir}, "\x00")
}

func (s *Service) repoTreeTarget(ctx context.Context, repoURL, branch, rootDir string) (repoTreeTarget, bool, error) {
	if !s.configured() {
		return repoTreeTarget{}, false, nil
	}
	rootDir = strings.TrimSpace(rootDir)
	if !store.ValidRootDir(rootDir) {
		return repoTreeTarget{}, false, fmt.Errorf("%w: rootDir must be a relative path with no '..' components", core.ErrBadRequest)
	}
	owner, repo, ok := githubOwnerRepo(repoURL)
	branch = strings.TrimSpace(branch)
	if !ok || branch == "" {
		return repoTreeTarget{}, false, nil
	}
	return repoTreeTarget{
		workspaceID: s.WorkspaceOrDefault(ctx),
		owner:       owner,
		repo:        repo,
		branch:      branch,
		rootDir:     rootDir,
	}, true, nil
}

// ProbeRepoTree returns the immediate files at rootDir on branch for a repo in
// ownerID's connected GitHub installation. It is member-readable like ListRepos
// and ListBranches. Expected probe failures collapse to Unknown so transport
// adapters never need to understand GitHub's rate-limit or contents dialect.
func (s *Service) ProbeRepoTree(ctx context.Context, ownerID, repoURL, branch, rootDir string) (RepoTreeProbe, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return RepoTreeProbe{}, err
	}
	target, ok, err := s.repoTreeTarget(ctx, repoURL, branch, rootDir)
	if err != nil {
		return RepoTreeProbe{}, err
	}
	if !ok {
		return RepoTreeProbe{Unknown: true}, nil
	}
	return s.probeRepoTree(ctx, target)
}

func (s *Service) probeRepoTree(ctx context.Context, target repoTreeTarget) (RepoTreeProbe, error) {
	ctx, cancel := context.WithTimeout(ctx, repoTreeProbeTimeout)
	defer cancel()
	row, err := s.Store.GetGitConnectionByOwner(ctx, target.workspaceID, target.owner)
	if errors.Is(err, store.ErrNotFound) {
		return RepoTreeProbe{Unknown: true}, nil
	}
	if err != nil {
		return RepoTreeProbe{}, err
	}
	tok, err := s.GitHub.MintInstallationToken(ctx, row.InstallationID)
	if err != nil {
		return RepoTreeProbe{Unknown: true}, nil
	}
	entries, err := s.GitHub.ListRepoTree(ctx, tok.Token, target.owner, target.repo, target.rootDir, target.branch)
	if err != nil {
		return RepoTreeProbe{Unknown: true}, nil
	}
	files := make([]RepoTreeEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Type == "file" && entry.Name != "" {
			files = append(files, entry)
		}
	}
	if len(files) == 0 {
		return RepoTreeProbe{Unknown: true}, nil
	}
	return RepoTreeProbe{Entries: files}, nil
}

// githubOwnerRepo extracts owner + repo when repoURL is a real github.com origin.
// ok=false for anything else, so the caller degrades to free-text branch entry
// (ListBranches) or a public/anonymous clone with no token (cloneToken).
//
// SECURITY (w1/m65 F1, hardened): this is an ORIGIN-VALIDATION control, not a
// string-comparison key, so it must not reuse core.CanonicalRepo — that
// normalizer strips everything through the first '@' to drop scp userinfo, which
// a crafted path like `https://evil.example/@github.com/owner/repo` abuses to
// masquerade as github.com and mint a token the build's credential helper would
// then send to evil.example. Instead parse the URL structurally: an HTTPS URL
// must have Hostname exactly github.com, no userinfo, and the default port; the
// scp form is matched against a fixed `git@github.com:` prefix. Non-HTTPS
// schemes (ssh/git) never mint a token — those clones authenticate with keys,
// not the x-access-token password, so an HTTP installation token is both useless
// and a leak risk on the wrong host.
func githubOwnerRepo(repoURL string) (owner, repo string, ok bool) {
	s := strings.TrimSpace(repoURL)
	// scp-like syntax has no scheme: git@github.com:owner/repo(.git).
	if !strings.Contains(s, "://") {
		rest, found := strings.CutPrefix(s, "git@github.com:")
		if !found {
			return "", "", false
		}
		return splitOwnerRepo(rest)
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", "", false
	}
	switch {
	case !strings.EqualFold(u.Scheme, "https"):
		return "", "", false
	case u.User != nil: // no userinfo — reject the `x@github.com` / `github.com@evil` tricks
		return "", "", false
	case !strings.EqualFold(u.Hostname(), "github.com"):
		return "", "", false
	case u.Port() != "": // only the default HTTPS port
		return "", "", false
	case u.RawQuery != "" || u.Fragment != "":
		return "", "", false
	}
	return splitOwnerRepo(u.EscapedPath())
}

// installationResolver adapts Service to apps.InstallationResolver. Like
// tokenSource, it is a SEPARATE type (not a Service method) on purpose: the git
// push webhook authenticates by HMAC signature, not an Authorize call, so
// exposing this as an exported Service verb would (rightly) trip
// TestAuthzGuardsEveryVerb. The adapter keeps that trust boundary explicit.
type installationResolver struct{ s *Service }

// WorkspacesForInstallation resolves every workspace that has PROVED a binding
// for a GitHub App installation id, so the git push webhook can confine an
// app-signed delivery to exactly that set (codex #7, ADR057 round-6 #9, restated
// for N:N in ADR078 §4a).
//
// An EMPTY result — no bindings, or the control-plane store is off — means the
// webhook acts on nothing. It must never be read as "unscoped": that is the
// fail-closed property, and it is unchanged by N:N, which alters only how many
// non-empty scopes there can be.
//
// This is the ONLY place an installation resolves to workspaces. Every other
// consumer runs the opposite direction (workspace → its connections), which is
// why the N:N blast radius is this function and its one caller.
func (r installationResolver) WorkspacesForInstallation(ctx context.Context, installationID int64) ([]string, error) {
	if r.s.Store == nil {
		return nil, nil
	}
	conns, err := r.s.Store.GitConnectionsByInstallation(ctx, installationID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(conns))
	for _, c := range conns {
		if c.WorkspaceID != "" {
			out = append(out, c.WorkspaceID)
		}
	}
	return out, nil
}

// InstallationResolver returns the webhook's installation→workspace seam (wired
// onto apps.GitWebhook in the composition root, codex #7).
func (s *Service) InstallationResolver() installationResolver { return installationResolver{s} }

// splitOwnerRepo reduces a repo path ("/owner/repo.git", "owner/repo") to its
// exactly-two non-empty segments, lowercased; ok=false otherwise.
func splitOwnerRepo(path string) (owner, repo string, ok bool) {
	p := strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	parts := strings.Split(p, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return strings.ToLower(parts[0]), strings.ToLower(parts[1]), true
}

// tokenSource adapts the Service to apps' CloneTokenSource seam. It exists as a
// separate type (not a Service method) on purpose: cloneToken authenticates via
// the deploy path's own authorization, not an Authorize call, so exposing it as
// an exported Service verb would (rightly) trip TestAuthzGuardsEveryVerb. The
// adapter keeps that trust boundary explicit — the same reason the git webhook's
// redeploy is unexported.
type tokenSource struct{ s *Service }

// CloneToken satisfies apps.CloneTokenSource.
func (t tokenSource) CloneToken(ctx context.Context, workspaceID, repoURL string) (string, bool, error) {
	return t.s.cloneToken(ctx, workspaceID, repoURL)
}

// RepoGranted satisfies apps.CloneTokenSource's read-path half: whether the repo
// belongs to the workspace's connection, with no token handed back. It is
// cloneToken with the credential dropped — literally the same call, so the
// deliverability the product REPORTS can never disagree with what a real deploy
// trigger DOES (w6/m99).
func (t tokenSource) RepoGranted(ctx context.Context, workspaceID, repoURL string) (bool, error) {
	_, granted, err := t.s.cloneToken(ctx, workspaceID, repoURL)
	return granted, err
}

// ValidateRepo checks a source edit before any App or clone Secret is changed.
// An unconnected GitHub repo is usable only when GitHub serves it anonymously;
// other Git hosts retain the create flow's public-Git behavior.
func (t tokenSource) ValidateRepo(ctx context.Context, workspaceID, repoURL string) error {
	owner, repo, ok := githubOwnerRepo(repoURL)
	if !ok || !t.s.configured() {
		return nil
	}
	granted, err := t.RepoGranted(ctx, workspaceID, repoURL)
	if err != nil {
		return fmt.Errorf("checking repository access: %w", err)
	}
	if granted {
		return nil
	}
	public, err := t.s.GitHub.RepoAccessible(ctx, "", owner, repo)
	if err != nil {
		return fmt.Errorf("checking public repository access: %w", err)
	}
	if !public {
		return fmt.Errorf("%w: repository is not accessible through this workspace's GitHub connections or as public Git; connect its owner and grant repository access, or use a public repository", core.ErrBadRequest)
	}
	return nil
}

// DeployTokenSource returns the deploy path's clone-token seam (wired onto
// apps.Service in the composition root).
func (s *Service) DeployTokenSource() tokenSource { return tokenSource{s} }

// cloneToken returns a fresh installation token to clone repoURL, if that repo
// belongs to the workspace's GitHub connection. NOT authz-gated — the caller
// (Create/redeploy) has already authorized its own verb, and the webhook
// redeploy carries no identity.
//
//   - ok=false, nil err: GitHub off, no connection, or the repo isn't in the
//     grant — the caller keeps today's public-clone behavior.
//   - non-nil err: a GitHub failure — the caller must fail the deploy, never
//     silently public-clone what might be a private repo.
//
// The read path's grant test is this same call, via RepoGranted (w6/m99): a
// second grant-check implementation would be free to drift from what a deploy
// actually does, which is the gap that let the product claim GitHub-app
// delivery for a repo the installation never granted.
func (s *Service) cloneToken(ctx context.Context, workspaceID, repoURL string) (string, bool, error) {
	if !s.configured() {
		return "", false, nil
	}
	// SECURITY (w1/m65 F1, hardened): bind the token to a structurally verified
	// github.com origin, not just an owner/repo path suffix. The minted
	// installation token flows into a Secret the operator's build Job hands to
	// git's credential helper, which sends it to whatever host the App's repo URL
	// names. githubOwnerRepo parses the URL with net/url and requires Hostname
	// exactly github.com with no userinfo and the default port, so a crafted host
	// (`https://evil.example/@github.com/org/repo`, `x@github.com`, a subdomain,
	// or a non-default port) never mints a token: the build then clones
	// anonymously and no credential reaches the attacker origin. The build's
	// credential helper is independently host-bound (answers only for github.com)
	// as defense in depth.
	owner, repo, ok := githubOwnerRepo(repoURL)
	if !ok {
		return "", false, nil // not a github.com owner/repo URL — nothing to match against
	}
	// Resolve the workspace's connection for THIS repo's account (ADR078 §4): a
	// workspace may hold several installations, and the token must come from the
	// one that owns the repo — never account A's token for account B's repo.
	row, err := s.Store.GetGitConnectionByOwner(ctx, workspaceID, owner)
	if errors.Is(err, store.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	// Mint once, then ask whether the installation can see this one repo — a
	// single GET, versus paginating the whole installation repo list. An
	// installation token can only reach granted repos, so a 404 means "not in
	// the grant" (RepoAccessible reports ok=false, no error).
	tok, err := s.GitHub.MintInstallationToken(ctx, row.InstallationID)
	if err != nil {
		return "", false, err
	}
	inGrant, err := s.GitHub.RepoAccessible(ctx, tok.Token, owner, repo)
	if err != nil {
		return "", false, err
	}
	if !inGrant {
		return "", false, nil
	}
	return tok.Token, true, nil
}

// commitSource adapts the Service to the deploys/apps CommitResolver seam
// (w9/001), the tokenSource precedent: resolveCommit authenticates via the
// deploy path's own authorization rather than an Authorize call, so exposing
// it as an exported Service verb would (rightly) trip TestAuthzGuardsEveryVerb.
type commitSource struct{ s *Service }

// ResolveCommit satisfies deploys.CommitResolver and apps.CommitResolver.
func (t commitSource) ResolveCommit(ctx context.Context, workspaceID, repoURL, ref string) (store.CommitInfo, bool, error) {
	return t.s.resolveCommit(ctx, workspaceID, repoURL, ref)
}

// DeployCommitSource returns the deploy path's commit-resolution seam (wired
// onto deploys.Service and apps.Service in the composition root).
func (s *Service) DeployCommitSource() commitSource { return commitSource{s} }

// resolveCommit resolves ref (a branch, tag, or SHA) to the exact commit it
// points at — the provenance a deploy row is stamped with at open time
// (w9/001, extended w4/m108 t004). NOT authz-gated: the caller (a deploy
// trigger) has already authorized its own verb.
//
// Decision (w4/m108 t004, also docs/ADR004): when the workspace has no GitHub
// App installation for the repo owner, fall back to an unauthenticated
// GET /repos/{owner}/{repo}/commits/{ref} for github.com public repos —
// reuses this seam so create / Trigger / blueprint all get hash+message+
// authorAt at deploy-open, cheaper than an operator→backend write-back.
// Limits: GitHub-only public repos; 60 req/h/IP so results are cached per
// (repo, ref); every failure stays ok=false and never blocks a deploy.
// Non-GitHub URLs, private-without-connection, and unknown refs remain
// commit-less (404/403 collapse so this path is not an existence oracle).
//
//   - ok=false, nil err: GitHub off, unparseable URL, out-of-grant /
//     private / unknown ref, non-github.com without a connection, or any
//     public-fallback failure — the deploy proceeds with no commit metadata
//     (omitted, not faked).
//   - non-nil err: an installation-path GitHub failure. Callers may treat
//     this the same as ok=false — commit metadata is provenance, never worth
//     failing a deploy over. The public fallback never returns a non-nil err.
func (s *Service) resolveCommit(ctx context.Context, workspaceID, repoURL, ref string) (store.CommitInfo, bool, error) {
	if !s.configured() || ref == "" {
		return store.CommitInfo{}, false, nil
	}
	owner, repo, ok := ownerRepo(repoURL)
	if !ok {
		return store.CommitInfo{}, false, nil
	}
	row, err := s.Store.GetGitConnectionByOwner(ctx, workspaceID, owner)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.CommitInfo{}, false, err
	}
	if errors.Is(err, store.ErrNotFound) {
		return s.resolvePublicCommit(ctx, repoURL, ref)
	}
	tok, err := s.GitHub.MintInstallationToken(ctx, row.InstallationID)
	if err != nil {
		return store.CommitInfo{}, false, err
	}
	c, err := s.GitHub.GetCommit(ctx, tok.Token, owner, repo, ref)
	if err != nil {
		// 404 = repo not in the grant; 422 = no such ref. Both mean "nothing to
		// resolve", not a GitHub failure.
		var apiErr *APIError
		if errors.As(err, &apiErr) && (apiErr.Status == 404 || apiErr.Status == 422) {
			return store.CommitInfo{}, false, nil
		}
		return store.CommitInfo{}, false, err
	}
	return store.CommitInfo{Hash: c.SHA, Message: c.Message, AuthorAt: c.AuthorAt}, true, nil
}

func (s *Service) publicCommitMemo() *core.TTLCache[cachedPublicCommit] {
	s.publicCommitOnce.Do(func() {
		s.publicCommitCache = core.NewTTLCache[cachedPublicCommit]()
	})
	return s.publicCommitCache
}

// resolvePublicCommit is the no-installation path for Public Git URL services:
// unauthenticated GitHub commit lookup, cached per (owner/repo, ref), bounded
// by a short timeout. Every failure is ok=false — never an error a deploy
// opener could treat as fatal, and never a fabricated partial commit.
func (s *Service) resolvePublicCommit(ctx context.Context, repoURL, ref string) (store.CommitInfo, bool, error) {
	owner, repo, ok := githubOwnerRepo(repoURL)
	if !ok {
		return store.CommitInfo{}, false, nil
	}
	key := strings.ToLower(owner) + "/" + strings.ToLower(repo) + "@" + ref
	if cached, hit := s.publicCommitMemo().Get(key); hit {
		return cached.info, cached.ok, nil
	}
	v, _, _ := s.publicCommitFlight.Do(key, func() (any, error) {
		if cached, hit := s.publicCommitMemo().Get(key); hit {
			return cached, nil
		}
		rctx, cancel := context.WithTimeout(ctx, publicCommitResolveTimeout)
		defer cancel()
		c, err := s.GitHub.GetCommit(rctx, "", owner, repo, ref)
		entry := cachedPublicCommit{}
		ttl := publicCommitMissTTL
		// Collapse every failure — including 404 vs 403 — into a miss so this
		// path cannot distinguish private from missing (no existence oracle).
		if err == nil && c.SHA != "" {
			entry = cachedPublicCommit{
				info: store.CommitInfo{Hash: c.SHA, Message: c.Message, AuthorAt: c.AuthorAt},
				ok:   true,
			}
			ttl = publicCommitCacheTTL
		}
		s.publicCommitMemo().Put(key, entry, time.Now().Add(ttl))
		return entry, nil
	})
	entry, _ := v.(cachedPublicCommit)
	return entry.info, entry.ok, nil
}

// ownerRepo extracts the "owner"/"repo" pair from a git URL of any form
// (https/ssh/scp), reusing core.CanonicalRepo's "host/owner/repo" normalization.
// ok=false when the URL doesn't carry both a host and an owner/repo.
func ownerRepo(raw string) (owner, repo string, ok bool) {
	parts := strings.Split(core.CanonicalRepo(raw), "/")
	if len(parts) < 3 {
		return "", "", false // need host/owner/repo
	}
	return parts[len(parts)-2], parts[len(parts)-1], true
}

func (s *Service) connectedView(c store.GitConnection) Connection {
	return Connection{
		Connected:      true,
		AccountLogin:   c.AccountLogin,
		InstallationID: c.InstallationID,
		CreatedAt:      c.CreatedAt.UTC().Format(time.RFC3339),
		InstallURL:     s.installURL(),
	}
}

// blueprintFetcher adapts Service to the apps.BlueprintFetcher seam (w2/m62).
// Not an exported verb — only the deploy path logic calls it, not end-users.
type blueprintFetcher struct{ s *Service }

// ResolveBlueprintCommit pins branch to its immutable HEAD commit before any
// manifest bytes are read, so branch movement cannot mix revisions (w8/m36
// t002). A lookup failure or an empty/malformed commit ID is an actionable
// error — provenance is never fabricated from a blob SHA.
func (f blueprintFetcher) ResolveBlueprintCommit(ctx context.Context, workspaceID, repoURL, branch string) (string, error) {
	return f.s.resolveBlueprintCommit(ctx, workspaceID, repoURL, branch)
}

// FetchBlueprintFileAtCommit reads the blueprint file at an already-resolved
// immutable commit (see ResolveBlueprintCommit). The commit is re-validated so
// a caller can never smuggle a branch name or garbage into sync provenance.
func (f blueprintFetcher) FetchBlueprintFileAtCommit(ctx context.Context, workspaceID, repoURL, commitSHA, filePath string) (string, error) {
	return f.s.fetchBlueprintFileAtCommit(ctx, workspaceID, repoURL, commitSHA, filePath)
}

// BlueprintFileFetcher returns the blueprint-file-fetch seam wired in the
// composition root onto apps.Service.
func (s *Service) BlueprintFileFetcher() blueprintFetcher { return blueprintFetcher{s} }

// blueprintToken mints the workspace installation token for a repo owner, or
// "" for the anonymous public-repo path. Shared by commit resolution and
// content fetch so private and public repositories use the same revision
// contract (w8/m36 t002).
func (s *Service) blueprintToken(ctx context.Context, workspaceID, owner, repoURL string) (string, error) {
	var token string
	if s.configured() {
		row, err := s.Store.GetGitConnectionByOwner(ctx, workspaceID, owner)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return "", err
		}
		if err == nil {
			tok, err := s.GitHub.MintInstallationToken(ctx, row.InstallationID)
			if err != nil {
				return "", err
			}
			token = tok.Token
		}
	}
	if token == "" {
		// Anonymous fetch for public repos — re-use the client's base URL.
		if s.GitHub == nil {
			return "", fmt.Errorf("%w: GitHub App not configured; cannot fetch blueprint file from %q", core.ErrBadRequest, repoURL)
		}
	}
	return token, nil
}

// blueprintRepoToken parses the repo URL and mints the workspace token for its
// owner — the shared preamble of commit resolution and content fetch.
func (s *Service) blueprintRepoToken(ctx context.Context, workspaceID, repoURL string) (owner, repo, token string, err error) {
	owner, repo, ok := ownerRepo(repoURL)
	if !ok {
		return "", "", "", fmt.Errorf("%w: repo URL %q is not an owner/repo URL", core.ErrBadRequest, repoURL)
	}
	token, err = s.blueprintToken(ctx, workspaceID, owner, repoURL)
	if err != nil {
		return "", "", "", err
	}
	return owner, repo, token, nil
}

// ErrBranchNotFound and ErrRepoNotFoundOrNoAccess name which half of a
// blueprint commit lookup failed. GitHub answers 404 for a missing branch and
// for a repository the installation cannot see, and the status alone cannot
// tell them apart — so callers that want to say "check the branch name" rather
// than "check the repository" need this, and guessing from message text is not
// a contract (w2/m97 t001).
//
// The two are distinguished by the upstream response BODY, which stays
// internal: it is read here to pick a sentinel and is never interpolated into
// any error that reaches a client (w6/005). A private repository and a missing
// repository are deliberately one sentinel — GitHub makes them
// indistinguishable on purpose, and so must bex.
var (
	ErrBranchNotFound         = errors.New("blueprint branch not found")
	ErrRepoNotFoundOrNoAccess = errors.New("blueprint repository not found or not accessible")
)

func (s *Service) resolveBlueprintCommit(ctx context.Context, workspaceID, repoURL, branch string) (string, error) {
	owner, repo, token, err := s.blueprintRepoToken(ctx, workspaceID, repoURL)
	if err != nil {
		return "", err
	}
	sha, err := s.GitHub.GetRepoCommitSHA(ctx, token, owner, repo, branch)
	if err != nil {
		return "", classifyBlueprintCommitLookup(err)
	}
	if !validCommitSHA(sha) {
		return "", fmt.Errorf("github: branch %q returned an empty or malformed commit id", branch)
	}
	return sha, nil
}

func (s *Service) fetchBlueprintFileAtCommit(ctx context.Context, workspaceID, repoURL, commitSHA, filePath string) (string, error) {
	if !validCommitSHA(commitSHA) {
		return "", fmt.Errorf("github: refusing to fetch blueprint file at an empty or malformed commit id")
	}
	owner, repo, token, err := s.blueprintRepoToken(ctx, workspaceID, repoURL)
	if err != nil {
		return "", err
	}
	// GitHub's contents endpoint accepts a commit as the ref parameter, so the
	// bytes read here are exactly the resolved revision — never a moved tip.
	fc, err := s.GitHub.GetFileContents(ctx, token, owner, repo, filePath, commitSHA)
	if err != nil {
		return "", err
	}
	return fc.Contents, nil
}

// mapGitHubErr turns a GitHub client error into a clean bex error: a 4xx (e.g. a
// forged/unknown installation) is caller error → ErrBadRequest; anything else
// (5xx, network) is surfaced as-is so it never masquerades as success.
func mapGitHubErr(err error) error {
	if errors.Is(err, errInventoryBound) {
		return core.NewBadRequestError(
			"GITHUB_INVENTORY_LIMIT",
			"GitHub repository or branch inventory exceeds the per-request safety limit; narrow the connected installation or repository and retry",
			nil,
		)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		return core.ErrBadRequest
	}
	return err
}

// classifyBlueprintCommitLookup labels a branch-info 404 as branch-missing or
// repo-missing, leaving every other failure exactly as it was. GitHub answers
// `{"message":"Branch not found"}` when the repository resolved but the ref did
// not, and a bare `Not Found` when it did not resolve the repository at all.
func classifyBlueprintCommitLookup(err error) error {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		return err
	}
	if strings.Contains(strings.ToLower(apiErr.Body), "branch not found") {
		return fmt.Errorf("%w: %w", ErrBranchNotFound, err)
	}
	return fmt.Errorf("%w: %w", ErrRepoNotFoundOrNoAccess, err)
}
