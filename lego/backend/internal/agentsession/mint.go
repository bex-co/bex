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

package agentsession

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	AuditVerbMintCredential  = "agentsessions.MintCredential"
	AuditVerbProxyCredential = "agentsessions.ProxyCredential"
	// AuditVerbMintGitHubReadCredential records each read-only GitHub API
	// credential the gateway's `gh` route asks for (w10/m513).
	AuditVerbMintGitHubReadCredential = "agentsessions.MintGitHubReadCredential"
	// AuditVerbMintGitHubWriteCredential records each credential of an
	// admitted `gh` write (w10/m522), allowed or denied, with its repository.
	AuditVerbMintGitHubWriteCredential = "agentsessions.MintGitHubWriteCredential"
)

const (
	// GitHubAPIProxySegment is the fifth path segment that turns a Git proxy
	// URL (ProxyRepositoryURL) into the session's read-only GitHub API route:
	// <repo URL>/github-api/<GitHub REST path> or /github-api/graphql. The
	// in-sandbox `gh` reaches it through the driver's local socket (w10/m513).
	GitHubAPIProxySegment = "github-api"
	// InternalGitHubReadMintPath lives only on bex-api's cluster-internal
	// :8091 listener, beside InternalMintPath, under the same gateway-only HMAC
	// and domain-separated from it by path, as the model mint is.
	InternalGitHubReadMintPath = "/v1/agent-session-github-read-credentials"
	// InternalGitHubWriteMintPath is the credential verb of one admitted `gh`
	// write (w10/m522), beside the read verb under the same HMAC and
	// domain-separated from it by path.
	InternalGitHubWriteMintPath = "/v1/agent-session-github-write-credentials"
)

type GitHubClient interface {
	MintSessionInstallationToken(context.Context, int64, string) (github.InstallationToken, error)
	// MintRepositoryReadToken is a repository environment build's clone
	// credential (ADR111): contents read-only.
	MintRepositoryReadToken(context.Context, int64, string) (github.InstallationToken, error)
	// MintRepositoryAPIReadToken is a session's `gh` route credential
	// (w10/m513): read-only Actions, checks, statuses, pull requests and
	// contents of the one repository.
	MintRepositoryAPIReadToken(context.Context, int64, string) (github.InstallationToken, error)
	// MintRepositoryAPIWriteToken is the credential of one admitted `gh`
	// write (w10/m522): those reads plus pull requests, issues and Actions
	// write of the one repository, never contents or workflows.
	MintRepositoryAPIWriteToken(context.Context, int64, string) (github.InstallationToken, error)
}

type ConnectionStore interface {
	// GetGitConnectionByOwner resolves the workspace's connection for a repo's
	// GitHub account (ADR078 §4). A workspace may hold several installations, so
	// the session token must be minted from the one that owns the target repo.
	GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (store.GitConnection, error)
}

// SessionStore reads the durable agent-session lifecycle record. Mint consults
// it to refuse a credential for a session that is no longer active (codex F12).
type SessionStore interface {
	GetAgentSession(context.Context, string) (store.AgentSession, error)
}

// credentialMintPhases is deliberately a fail-closed allowlist shared by the
// Git and model minters. A terminal row's old pod can still exist (idle grace,
// failed teardown); a denylist would make every future phase
// credential-eligible by default.
var credentialMintPhases = map[string]bool{
	"creating": true, "running": true, "resuming": true, "redispatching": true,
}

// CurrentSandboxCaller binds a gateway-resolved Pod to the exact durable
// sandbox generation. OpenSandbox names the session pod <sandbox_id>-0; a stale
// pod from a failed teardown carries the same session labels but a different
// immutable sandbox id after a redispatch. PodUID remains mandatory as the
// gateway's Kubernetes-derived identity, while the durable generation match is
// made through the current sandbox id.
//
// Exported so the ADR095 share verb, served outside this package, applies the
// same rule as every other gateway verb.
func CurrentSandboxCaller(session store.AgentSession, workspace, podName, podUID string) bool {
	return credentialMintPhases[session.Phase] && boundPod(session.WorkspaceID, session.SandboxID, workspace, podName, podUID)
}

// boundPod reports whether the gateway-resolved Pod is the sandbox a durable
// principal (a session, a build) records, in its workspace: OpenSandbox names
// a sandbox's Pod <sandbox_id>-0, and the Pod UID is the gateway's
// Kubernetes-derived identity.
func boundPod(principalWorkspace, sandboxID, workspace, podName, podUID string) bool {
	return principalWorkspace == workspace && sandboxID != "" && podUID != "" && podName == sandboxID+"-0"
}

// Minter is bex-api's internal-only credential verb. It trusts only a request
// authenticated by Handler's HMAC and still rechecks branch/repository policy.
type Minter struct {
	GitHub      GitHubClient
	Connections ConnectionStore
	Sessions    SessionStore
	// Builds authorizes a repository environment build's clone (ADR111); nil
	// refuses every build principal.
	Builds BuildStore
	// Environments reads the workspace's GitHub CLI setting for the `gh`
	// route's mints (w10/m524); nil refuses them.
	Environments EnvironmentStore
	Audit        core.AuditSink
	Now          func() time.Time
}

func (m *Minter) Mint(ctx context.Context, req MintRequest) (MintResponse, error) {
	return m.mint(ctx, req, mintGit)
}

// MintGitHubRead is the credential of a repo session's read-only GitHub API
// route (w10/m513): the same Pod, lifecycle and branch checks as Mint, then a
// token that can only read the session's one repository. A repository
// environment build has no `gh` route and gets none, and a workspace that
// turned `gh` off gets ErrGitHubCLIOff (w10/m524).
func (m *Minter) MintGitHubRead(ctx context.Context, req MintRequest) (MintResponse, error) {
	return m.mint(ctx, req, mintGitHubRead)
}

// MintGitHubWrite is the credential of one write the gateway's `gh` route
// admitted (w10/m522): MintGitHubRead's checks, then a token that can also
// comment, review, label, open issues and re-run CI on the one repository,
// and never change its code or refs.
func (m *Minter) MintGitHubWrite(ctx context.Context, req MintRequest) (MintResponse, error) {
	return m.mint(ctx, req, mintGitHubWrite)
}

// mintKind is which credential a mint verb issues.
type mintKind int

const (
	mintGit mintKind = iota
	mintGitHubRead
	mintGitHubWrite
)

func (k mintKind) auditVerb() string {
	switch k {
	case mintGitHubRead:
		return AuditVerbMintGitHubReadCredential
	case mintGitHubWrite:
		return AuditVerbMintGitHubWriteCredential
	}
	return AuditVerbMintCredential
}

// token is the kind's GitHub mint.
func (k mintKind) token(gh GitHubClient) func(context.Context, int64, string) (github.InstallationToken, error) {
	switch k {
	case mintGitHubRead:
		return gh.MintRepositoryAPIReadToken
	case mintGitHubWrite:
		return gh.MintRepositoryAPIWriteToken
	}
	return gh.MintSessionInstallationToken
}

func (m *Minter) mint(ctx context.Context, req MintRequest, kind mintKind) (response MintResponse, err error) {
	verb := kind.auditVerb()
	// Record both successful issuance and a refused mint. The event schema has no
	// arbitrary payload: target is the session id and targetName is the verified
	// non-secret repository identity; malformed input records an empty targetName
	// so raw attacker-controlled text, token, and branch never enter audit_events.
	auditRepository := ""
	defer func() {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
			// Log the failure reason (the token is never in the error) — the mint
			// was previously silent, which hid a GitHub-permission failure behind an
			// opaque 502 during the w3/m43 live E2E. Never logs the token.
			log.Printf("agent-session credential mint denied (session=%s repo=%s): %v", req.SessionID, auditRepository, err)
		}
		core.RecordAuditEvent(ctx, m.Audit, core.AuditEvent{
			Caller: req.PodName, CallerMethod: "sandbox", Verb: verb,
			Resource: core.WorkspaceObject(req.Workspace), Target: principalTarget(req.SessionID),
			TargetName: auditRepository, Outcome: outcome, At: m.now(),
		})
	}()
	repository, normalizeErr := NormalizeRepository(req.Repository)
	if normalizeErr != nil {
		return MintResponse{}, normalizeErr
	}
	req.Repository = repository
	auditRepository = repository

	if m.GitHub == nil || m.Connections == nil {
		return MintResponse{}, fmt.Errorf("agent credentials unavailable")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return MintResponse{}, fmt.Errorf("%w: incomplete verified identity", ErrInvalidRequest)
	}
	if IsBuildPrincipal(req.SessionID) {
		if kind != mintGit {
			return MintResponse{}, ErrForbidden
		}
		return m.mintBuild(ctx, req)
	}
	// Bind the mint to the session's CURRENT lifecycle, not just the requesting
	// pod's immutable identity (codex F12). A terminal session keeps its sandbox
	// pod through the ADR054 grace window, so pod-identity checks alone would let
	// a retained or compromised pod mint a fresh one-hour contents:write token
	// after the task was completed, failed, or canceled. Fail closed unless the
	// durable record is still active, in this workspace, and holds a live sandbox.
	if m.Sessions == nil {
		return MintResponse{}, fmt.Errorf("agent credentials unavailable")
	}
	session, sessErr := m.Sessions.GetAgentSession(ctx, req.SessionID)
	if sessErr != nil {
		if errors.Is(sessErr, store.ErrNotFound) {
			return MintResponse{}, ErrForbidden
		}
		return MintResponse{}, sessErr
	}
	if !CurrentSandboxCaller(session, req.Workspace, req.PodName, req.PodUID) {
		return MintResponse{}, ErrForbidden
	}
	if err := ValidateBranch(req.Branch); err != nil {
		return MintResponse{}, err
	}
	if kind != mintGit {
		if err := m.githubCLIAllowed(ctx, req.Workspace); err != nil {
			return MintResponse{}, err
		}
	}
	return m.installationToken(ctx, req.Workspace, repository, kind.token(m.GitHub))
}

// githubCLIAllowed refuses the `gh` route's credentials with ErrGitHubCLIOff
// while the workspace has its GitHub CLI setting off (w10/m524). The row is
// read on every call, so a running session's next `gh` call is refused once
// an admin saves the setting. A workspace with no row has the default, on.
// The Git mint never asks: pushes to the session branch keep working.
func (m *Minter) githubCLIAllowed(ctx context.Context, workspace string) error {
	if m.Environments == nil {
		return fmt.Errorf("agent credentials unavailable")
	}
	env, err := m.Environments.GetAgentEnvironment(ctx, workspace)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil
	case err != nil:
		return err
	case env.GitHubCLIDisabled:
		return ErrGitHubCLIOff
	}
	return nil
}

// mintBuild is a repository environment build's clone credential (ADR111): a
// contents-read token for the build's own repository, minted only for the
// running build's builder, on the build branch its proxy URL carries.
func (m *Minter) mintBuild(ctx context.Context, req MintRequest) (MintResponse, error) {
	build, err := currentBuilder(ctx, m.Builds, req.SessionID, req.Workspace, req.PodName, req.PodUID)
	if err != nil {
		return MintResponse{}, err
	}
	if req.Repository != build.Repo || req.Branch != BuildBranch(build.ID) {
		return MintResponse{}, ErrForbidden
	}
	return m.installationToken(ctx, req.Workspace, req.Repository, m.GitHub.MintRepositoryReadToken)
}

// installationToken mints a token for repository from the workspace
// connection that owns its account.
func (m *Minter) installationToken(ctx context.Context, workspace, repository string, mint func(context.Context, int64, string) (github.InstallationToken, error)) (MintResponse, error) {
	owner, name, _ := strings.Cut(repository, "/")
	// Resolve the connection that owns THIS repo's account (ADR078 §4). A
	// no-match — the repo's owner is not one of the workspace's connected
	// accounts — is a forbidden mint, exactly as the old owner-equality check was.
	connection, err := m.Connections.GetGitConnectionByOwner(ctx, workspace, owner)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return MintResponse{}, ErrForbidden
		}
		return MintResponse{}, err
	}
	token, err := mint(ctx, connection.InstallationID, name)
	if err != nil {
		return MintResponse{}, err
	}
	if token.Token == "" || token.ExpiresAt.IsZero() {
		return MintResponse{}, fmt.Errorf("github returned an incomplete installation token")
	}
	return MintResponse{Username: "x-access-token", Token: token.Token, ExpiresAt: token.ExpiresAt.UTC().Format(time.RFC3339)}, nil
}

// principalTarget is the audit target of a gateway principal: a session, or a
// repository environment build.
func principalTarget(principal string) string {
	if IsBuildPrincipal(principal) {
		return RepoEnvironmentBuildAuditTarget + ":" + principal
	}
	return "agent-session:" + principal
}

func (m *Minter) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}
