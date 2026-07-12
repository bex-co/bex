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
	DeleteGitConnection(ctx context.Context, workspaceID string) error
}

// APIClient is the GitHub REST surface the Service uses — *Client in production,
// a fake in tests. nil => the GitHub App is unconfigured (BEX_GITHUB_APP_* unset)
// and every verb reports core.ErrGitHubUnavailable.
type APIClient interface {
	InstallURL() string
	GetInstallation(ctx context.Context, installationID int64) (Installation, error)
	ListRepos(ctx context.Context, installationID int64) ([]Repo, error)
	MintInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error)
	RepoAccessible(ctx context.Context, token, owner, repo string) (bool, error)
}

// Service manages a workspace's GitHub App connection and lists its repos over
// the injected client + store. Both seams must be present; either nil => 503.
type Service struct {
	*core.Base
	GitHub APIClient       // nil => GitHub App unconfigured
	Store  ConnectionStore // nil => control-plane store off
	// DashboardURL is BEX_DASHBOARD_URL — where the install callback redirects
	// the browser after recording the connection. Empty => the callback returns
	// JSON instead of redirecting.
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
// callback.
func (s *Service) StartConnect(ctx context.Context) (Connection, error) {
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
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

// Connect records (or replaces) the workspace's GitHub App installation. It
// validates the browser-supplied installation id by fetching it from GitHub
// with the app JWT first (a forged id 404s, mapped to ErrBadRequest), then
// upserts the connection. Admin-only.
func (s *Service) Connect(ctx context.Context, installationID int64) (Connection, error) {
	if err := s.Authorize(ctx, core.RelCanManage); err != nil {
		return Connection{}, err
	}
	if !s.configured() {
		return Connection{}, core.ErrGitHubUnavailable
	}
	if installationID <= 0 {
		return Connection{}, core.ErrBadRequest
	}
	inst, err := s.GitHub.GetInstallation(ctx, installationID)
	if err != nil {
		return Connection{}, mapGitHubErr(err)
	}
	row, err := s.Store.UpsertGitConnection(ctx, store.GitConnection{
		WorkspaceID:    s.workspaceID(ctx),
		InstallationID: installationID,
		AccountLogin:   inst.AccountLogin,
	})
	if err != nil {
		return Connection{}, err
	}
	return s.connectedView(row), nil
}

// GetConnection returns the workspace's connection status. "Not connected" is a
// valid state (Connected:false + the install URL), not an error. Member read.
func (s *Service) GetConnection(ctx context.Context) (Connection, error) {
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

// Disconnect removes the workspace's connection. Idempotent: disconnecting when
// not connected is a no-op success. Admin-only.
func (s *Service) Disconnect(ctx context.Context) error {
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

// ListRepos returns the connected installation's repositories (private included).
// With no connection the list is empty (disconnect "empties" the repos), not an
// error. Member read.
func (s *Service) ListRepos(ctx context.Context) ([]Repo, error) {
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
	owner, repo, ok := ownerRepo(repoURL)
	if !ok {
		return "", false, nil // not an owner/repo URL — nothing to match against
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
