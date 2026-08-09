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
	"net/url"
	"strings"
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
	// GitConnectionByInstallation resolves which workspace (if any) already owns
	// an installation — the unique-binding gate (w1/m65 F2).
	GitConnectionByInstallation(ctx context.Context, installationID int64) (store.GitConnection, error)
	DeleteGitConnection(ctx context.Context, workspaceID string) error
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

// InstallationVerifier proves the user completing a browser install actually
// administers the installation (F2). Implemented by *Client when the App's OAuth
// credentials are configured; nil => the connect callback relies on the unique
// installation->workspace binding alone. Kept a narrow interface so the fake in
// tests can drive accept/reject.
type InstallationVerifier interface {
	VerifyInstallationAdmin(ctx context.Context, code string, installationID int64) (bool, error)
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
}

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
func (s *Service) workspaceID(ctx context.Context) string {
	if tid, ok := s.Tenant(ctx); ok && tid != "" {
		return tid
	}
	return core.DefaultTenant
}

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
	workspaceID := s.workspaceID(ctx)
	installURL, err := s.statefulInstallURL(workspaceID)
	if err != nil {
		return Connection{}, err
	}
	row, err := s.Store.GetGitConnection(ctx, workspaceID)
	if errors.Is(err, store.ErrNotFound) {
		return Connection{Connected: false, InstallURL: installURL}, nil
	}
	if err != nil {
		return Connection{}, err
	}
	conn := s.connectedView(row)
	conn.InstallURL = installURL
	return conn, nil
}

// connectFromCallback is the sole installation-binding path. The signed state
// has already selected an authorized bex workspace; the GitHub user OAuth code
// independently proves that the browser principal administers the selected
// installation. Both proofs are mandatory. GitHub authorization codes are
// single-use, which also prevents callback replay after a successful exchange.
func (s *Service) connectFromCallback(ctx context.Context, workspaceID string, installationID int64, code string) (Connection, error) {
	if !s.configured() {
		return Connection{}, core.ErrGitHubUnavailable
	}
	if s.Verifier == nil {
		return Connection{}, core.ErrGitHubUnavailable
	}
	if strings.TrimSpace(code) == "" {
		return Connection{}, fmt.Errorf("%w: GitHub user authorization code is required", core.ErrBadRequest)
	}
	ok, err := s.Verifier.VerifyInstallationAdmin(ctx, code, installationID)
	if err != nil {
		return Connection{}, mapGitHubErr(err)
	}
	if !ok {
		return Connection{}, fmt.Errorf("%w: could not verify you administer this GitHub installation", core.ErrForbidden)
	}
	return s.connectWithWorkspace(ctx, workspaceID, installationID)
}

// connectWithWorkspace records a connection for the workspace authenticated by
// a verified state credential. It deliberately is not an exported service verb:
// it has no caller Identity to authorize, and must only be called by the callback
// after both verifyConnectState and the installation-admin proof succeed.
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
	if existing, lookupErr := s.Store.GitConnectionByInstallation(ctx, installationID); lookupErr == nil {
		if existing.WorkspaceID != workspaceID {
			return Connection{}, fmt.Errorf("%w: this GitHub installation is already connected to another workspace", core.ErrConflict)
		}
	} else if !errors.Is(lookupErr, store.ErrNotFound) {
		return Connection{}, lookupErr
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

// GetConnection returns ownerID's connection status ("" => the caller's
// default workspace, w6/m18). "Not connected" is a valid state (Connected:false
// + the install URL), not an error. Member read.
func (s *Service) GetConnection(ctx context.Context, ownerID string) (Connection, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Connection{}, err
	}
	if !s.configured() {
		return Connection{}, core.ErrGitHubUnavailable
	}
	row, err := s.Store.GetGitConnection(ctx, s.workspaceID(ctx))
	if errors.Is(err, store.ErrNotFound) {
		return Connection{Connected: false, InstallURL: s.installURL()}, nil
	}
	if err != nil {
		return Connection{}, err
	}
	return s.connectedView(row), nil
}

// Disconnect removes ownerID's connection ("" => the caller's default
// workspace, w6/m18). Idempotent: disconnecting when not connected is a no-op
// success. Admin-only.
func (s *Service) Disconnect(ctx context.Context, ownerID string) error {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return err
	}
	if !s.configured() {
		return core.ErrGitHubUnavailable
	}
	err := s.Store.DeleteGitConnection(ctx, s.workspaceID(ctx))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}

// ListRepos returns ownerID's connected installation's repositories ("" => the
// caller's default workspace, w6/m18; private included). With no connection the
// list is empty (disconnect "empties" the repos), not an error. Member read.
func (s *Service) ListRepos(ctx context.Context, ownerID string) ([]Repo, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, core.ErrGitHubUnavailable
	}
	row, err := s.Store.GetGitConnection(ctx, s.workspaceID(ctx))
	if errors.Is(err, store.ErrNotFound) {
		return []Repo{}, nil
	}
	if err != nil {
		return nil, err
	}
	repos, err := s.GitHub.ListRepos(ctx, row.InstallationID)
	if err != nil {
		return nil, mapGitHubErr(err)
	}
	if repos == nil {
		repos = []Repo{}
	}
	return repos, nil
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
	row, err := s.Store.GetGitConnection(ctx, s.workspaceID(ctx))
	if errors.Is(err, store.ErrNotFound) {
		return []string{}, nil
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
	row, err := s.Store.GetGitConnection(ctx, workspaceID)
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
	row, err := s.Store.GetGitConnection(ctx, workspaceID)
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
		row, err := s.Store.GetGitConnection(ctx, workspaceID)
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
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		return core.ErrBadRequest
	}
	return err
}
