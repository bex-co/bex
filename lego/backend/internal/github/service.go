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
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ConnectionStore is the Service's seam to the control-plane store — the narrow
// slice of Store it needs. *store.PGStore satisfies it; a fake backs the tests.
// nil => the control-plane store is off (BEX_CP_DB_URI unset) and every verb
// reports core.ErrGitHubUnavailable.
type ConnectionStore interface {
	UpsertGitConnection(ctx context.Context, c store.GitConnection) (store.GitConnection, error)
	GetGitConnection(ctx context.Context, workspaceID string) (store.GitConnection, error)
	// ListGitConnections returns a workspace's full connection set, oldest first
	// (ADR075) — the multi-account aggregate the repo picker and list surface read.
	ListGitConnections(ctx context.Context, workspaceID string) ([]store.GitConnection, error)
	// GetGitConnectionByOwner resolves the connection whose account login matches a
	// repo's owner — the exact installation to mint that repo's token from (ADR075 §4).
	GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (store.GitConnection, error)
	// GitConnectionByInstallation resolves which workspace (if any) already owns
	// an installation — the unique-binding gate (w1/m65 F2).
	GitConnectionByInstallation(ctx context.Context, installationID int64) (store.GitConnection, error)
	// CountGitConnections backs the per-workspace connection quota (ADR075 §2).
	CountGitConnections(ctx context.Context, workspaceID string) (int, error)
	DeleteGitConnection(ctx context.Context, workspaceID string, installationID int64) error
	// The subject-bound, single-use connect transaction (w1/m67 F3): the record
	// that ties "who started this flow" to "who came back from GitHub".
	CreateGitHubConnectTransaction(ctx context.Context, t store.GitHubConnectTransaction) error
	ConsumeGitHubConnectTransaction(ctx context.Context, nonce string) (store.GitHubConnectTransaction, error)
}

// APIClient is the GitHub REST surface the Service uses — *Client in production,
// a fake in tests. nil => the GitHub App is unconfigured (BEX_GITHUB_APP_* unset)
// and every verb reports core.ErrGitHubUnavailable.
type APIClient interface {
	InstallURL() string
	GetInstallation(ctx context.Context, installationID int64) (Installation, error)
	ListRepos(ctx context.Context, installationID int64) ([]Repo, error)
	ListBranches(ctx context.Context, installationID int64, owner, repo string) ([]string, error)
	MintInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error)
	RepoAccessible(ctx context.Context, token, owner, repo string) (bool, error)
	GetCommit(ctx context.Context, token, owner, repo, ref string) (Commit, error)
	GetFileContents(ctx context.Context, token, owner, repo, path, ref string) (FileContents, error)
	GetRepoCommitSHA(ctx context.Context, token, owner, repo, branch string) (string, error)
	OpenDraftPullRequest(ctx context.Context, installationID int64, owner, repo, head, base, title, body string) (PullRequest, error)
}

// InstallationVerifier proves the user completing a browser flow actually
// administers the installation being bound (F2), and powers the ADR075 §3a claim
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
	// (BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE, ADR075 §2; default 10, 0 disables).
	// Bounds one tenant's connection fan-out — and therefore the per-connection
	// GitHub round trips ListRepos makes.
	MaxConnections int
}

const maxGitHubInventoryFanout = 4

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
	// ADR075 §7: fail at start, not at the callback. With no verifier every
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
	// a NEW connect, so it is what "Connect another account" uses too (ADR075 §3).
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
// ADR075 §3a claim flow for an already-installed account.
type Claim struct {
	ClaimURL string `json:"claimUrl"`
}

// Bounded claim-callback failures (ADR075 §3a) — mapped to fixed git_error codes
// in rest.go; the messages are safe for the JSON (no-dashboard) mode.
var (
	errNoClaimableInstallation = fmt.Errorf("%w: no unconnected GitHub installation you administer was found; install the bex GitHub App on the account first", core.ErrBadRequest)
	errAmbiguousClaim          = fmt.Errorf("%w: several unconnected GitHub installations you administer were found; claiming requires exactly one", core.ErrBadRequest)
)

// verifierPreflight is ADR075 §7: connect/claim starts refuse immediately when
// the installation-admin verifier is unconfigured, because the callback would
// fail closed anyway — after the user completed the whole GitHub round trip.
func (s *Service) verifierPreflight() error {
	if s.Verifier == nil {
		return fmt.Errorf("%w: GitHub App OAuth verification is not configured (BEX_GITHUB_APP_CLIENT_ID/BEX_GITHUB_APP_CLIENT_SECRET); ask your platform operator", core.ErrGitHubUnavailable)
	}
	return nil
}

// StartClaim begins the ADR075 §3a claim flow: bind an installation that ALREADY
// exists on GitHub (the direct-install case) to ownerID's workspace ("" => the
// caller's default). GitHub strips the signed state from the install URL for
// already-installed accounts, so the claim rides the OAuth user-authorization
// flow instead — the one flow that always preserves state — and the callback
// resolves the installation server-side from the authorizing user's admin set.
// Admin-only, same transaction record as StartConnect (w1/m67 F3).
func (s *Service) StartClaim(ctx context.Context, ownerID string) (Claim, error) {
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
	token, err := s.mintConnectState(ctx, workspaceID, subject)
	if err != nil {
		return Claim{}, err
	}
	return Claim{ClaimURL: s.Verifier.AuthorizeURL() + "&state=" + url.QueryEscape(token)}, nil
}

// claimFromCallback is the claim flow's callback half: it arrives with code +
// state and NO installation_id (that absence is what selects this branch), runs
// the identical proof sequence as connectFromCallback — consume the single-use
// nonce, match the initiator, fresh can_manage — and then resolves the
// installation server-side: of this app's installations the code's user
// ADMINISTERS, exactly one must be unbound (or already bound to this same
// workspace — idempotent). Zero or several are bounded failures, never a guess;
// an installation bound to a DIFFERENT workspace is never a candidate. No
// client-supplied installation id exists on this path at all.
func (s *Service) claimFromCallback(ctx context.Context, nonce, caller, code string) (Connection, error) {
	txn, err := s.consumeCallbackProofs(ctx, nonce, caller, code)
	if err != nil {
		return Connection{}, err
	}
	admined, err := s.Verifier.ClaimableInstallations(ctx, code)
	if err != nil {
		return Connection{}, mapGitHubErr(err)
	}
	// Keep only installations not bound to a DIFFERENT workspace: unbound ones
	// are claimable, and one already bound to THIS workspace stays a candidate so
	// a repeated claim is idempotent rather than a confusing "nothing to claim".
	candidates := make([]Installation, 0, len(admined))
	for _, inst := range admined {
		existing, lookupErr := s.Store.GitConnectionByInstallation(ctx, inst.ID)
		switch {
		case errors.Is(lookupErr, store.ErrNotFound):
			candidates = append(candidates, inst)
		case lookupErr != nil:
			return Connection{}, lookupErr
		case existing.WorkspaceID == txn.TenantID:
			candidates = append(candidates, inst)
		}
	}
	switch len(candidates) {
	case 0:
		return Connection{}, errNoClaimableInstallation
	case 1:
		return s.connectWithWorkspace(ctx, txn.TenantID, candidates[0].ID)
	default:
		return Connection{}, errAmbiguousClaim
	}
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
	// SECURITY (w1/m65 F2): GetInstallation authenticates as the App, which can
	// look up EVERY installation of itself — so its success proves the
	// installation exists, NOT that the caller's workspace owns it. Enforce a
	// unique installation->workspace binding: an installation already connected by
	// a DIFFERENT workspace cannot be re-claimed here (a re-connect by the SAME
	// workspace is idempotent). This blocks a workspace admin from binding another
	// tenant's installation and minting tokens for its private repositories. (A
	// The mandatory user-OAuth proof has already established that the initiating
	// user administers this installation; this uniqueness check independently
	// prevents the same installation from being attached to two workspaces.)
	reconnect := false
	if existing, lookupErr := s.Store.GitConnectionByInstallation(ctx, installationID); lookupErr == nil {
		if existing.WorkspaceID != workspaceID {
			return Connection{}, fmt.Errorf("%w: this GitHub installation is already connected to another workspace", core.ErrConflict)
		}
		reconnect = true // same workspace re-binding: idempotent, quota-exempt
	} else if !errors.Is(lookupErr, store.ErrNotFound) {
		return Connection{}, lookupErr
	}
	// ADR075 §2: a NEW binding (not an idempotent re-connect) is subject to the
	// per-workspace connection cap, so one tenant cannot fan out unbounded
	// installations — each of which ListRepos then fans a GitHub round trip over.
	if !reconnect {
		if err := s.connectionQuota(ctx, workspaceID); err != nil {
			return Connection{}, err
		}
	}
	row, err := s.Store.UpsertGitConnection(ctx, store.GitConnection{
		WorkspaceID:    workspaceID,
		InstallationID: installationID,
		AccountLogin:   inst.AccountLogin,
	})
	if err != nil {
		return Connection{}, err
	}
	return s.connectedView(row), nil
}

// GetConnection returns ownerID's connection status ("" => the caller's default
// workspace, w6/m18) — the singular compatibility alias over the workspace's
// oldest connection (ADR075). "Not connected" is a valid state, not an error, and
// (ADR075 §3) carries NO install URL: the bare, stateless URL is no longer
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
// multi-account surface (ADR075 §5). An empty slice (never an error) means no
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

// connectionQuota refuses a NEW connection that would push a workspace past its
// per-workspace cap (BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE, ADR075 §2).
// MaxConnections <= 0 disables the cap (tests, store-off, self-host opt-out).
func (s *Service) connectionQuota(ctx context.Context, workspace string) error {
	if s.MaxConnections <= 0 {
		return nil
	}
	count, err := s.Store.CountGitConnections(ctx, workspace)
	if err != nil {
		return err
	}
	if count >= s.MaxConnections {
		return core.NewConflictError("GIT_CONNECTION_LIMIT",
			fmt.Sprintf("workspace already has %d connected GitHub installations (limit %d); disconnect one or raise the limit", count, s.MaxConnections),
			map[string]any{"count": count, "limit": s.MaxConnections})
	}
	return nil
}

// ListRepos returns the repositories across ALL of ownerID's connected
// installations ("" => the caller's default workspace, w6/m18; private included),
// each annotated with the GitHub account it came from so the picker can group by
// account (ADR075 §4). With no connection the list is empty (not an error). One
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
	// byte-identical to the pre-ADR075 behavior.
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

// WorkspaceForInstallation resolves the workspace bound to a GitHub App
// installation id so the git push webhook can CONFINE an app-signed delivery to
// its installation's workspace (codex #7). ok=false (no error) when no connection
// owns the installation or the control-plane store is off — the webhook then acts
// on nothing rather than falling back to a cross-tenant global match.
func (r installationResolver) WorkspaceForInstallation(ctx context.Context, installationID int64) (string, bool, error) {
	if r.s.Store == nil {
		return "", false, nil
	}
	conn, err := r.s.Store.GitConnectionByInstallation(ctx, installationID)
	if errors.Is(err, store.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return conn.WorkspaceID, true, nil
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
	// Resolve the workspace's connection for THIS repo's account (ADR075 §4): a
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
// points at, via workspaceID's GitHub connection — the provenance a deploy
// row is stamped with (w9/001). NOT authz-gated: the caller (a deploy
// trigger) has already authorized its own verb.
//
//   - ok=false, nil err: GitHub off, no connection, the repo isn't an
//     owner/repo URL or isn't in the grant, or the ref doesn't exist — the
//     deploy proceeds with no commit metadata (omitted, not faked).
//   - non-nil err: a GitHub failure. Unlike cloneToken, callers may treat
//     this the same as ok=false — commit metadata is provenance, never worth
//     failing a deploy over.
func (s *Service) resolveCommit(ctx context.Context, workspaceID, repoURL, ref string) (store.CommitInfo, bool, error) {
	if !s.configured() || ref == "" {
		return store.CommitInfo{}, false, nil
	}
	owner, repo, ok := ownerRepo(repoURL)
	if !ok {
		return store.CommitInfo{}, false, nil
	}
	row, err := s.Store.GetGitConnectionByOwner(ctx, workspaceID, owner)
	if errors.Is(err, store.ErrNotFound) {
		return store.CommitInfo{}, false, nil
	}
	if err != nil {
		return store.CommitInfo{}, false, err
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

// FetchBlueprintFile fetches the blueprint file at repo+branch+path using the
// workspace's GitHub connection (if available) or an anonymous fetch for public
// repos. Returns the contents and the head commit SHA, or an error.
func (f blueprintFetcher) FetchBlueprintFile(ctx context.Context, workspaceID, repoURL, branch, filePath string) (string, string, error) {
	return f.s.fetchBlueprintFile(ctx, workspaceID, repoURL, branch, filePath)
}

// BlueprintFileFetcher returns the blueprint-file-fetch seam wired in the
// composition root onto apps.Service.
func (s *Service) BlueprintFileFetcher() blueprintFetcher { return blueprintFetcher{s} }

func (s *Service) fetchBlueprintFile(ctx context.Context, workspaceID, repoURL, branch, filePath string) (string, string, error) {
	owner, repo, ok := ownerRepo(repoURL)
	if !ok {
		return "", "", fmt.Errorf("%w: repo URL %q is not an owner/repo URL", core.ErrBadRequest, repoURL)
	}
	var token string
	if s.configured() {
		row, err := s.Store.GetGitConnectionByOwner(ctx, workspaceID, owner)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return "", "", err
		}
		if err == nil {
			tok, err := s.GitHub.MintInstallationToken(ctx, row.InstallationID)
			if err != nil {
				return "", "", err
			}
			token = tok.Token
		}
	}
	if token == "" {
		// Anonymous fetch for public repos — re-use the client's base URL.
		if s.GitHub == nil {
			return "", "", fmt.Errorf("%w: GitHub App not configured; cannot fetch blueprint file from %q", core.ErrBadRequest, repoURL)
		}
	}
	// Use the GitHub raw-content endpoint.
	fc, err := s.GitHub.GetFileContents(ctx, token, owner, repo, filePath, branch)
	if err != nil {
		return "", "", err
	}
	// GetRepoCommitSHA gives us the true HEAD SHA (the raw-content response
	// does not carry it reliably for the repo HEAD).
	sha, shaErr := s.GitHub.GetRepoCommitSHA(ctx, token, owner, repo, branch)
	if shaErr != nil {
		sha = "" // best-effort; don't fail the fetch over missing provenance
	}
	if sha == "" {
		sha = fc.CommitSHA // fall back to the blob SHA if HEAD is unavailable
	}
	return fc.Contents, sha, nil
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
