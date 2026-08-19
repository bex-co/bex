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
)

type GitHubClient interface {
	MintSessionInstallationToken(context.Context, int64, string) (github.InstallationToken, error)
}

type ConnectionStore interface {
	// GetGitConnectionByOwner resolves the workspace's connection for a repo's
	// GitHub account (ADR075 §4). A workspace may hold several installations, so
	// the session token must be minted from the one that owns the target repo.
	GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (store.GitConnection, error)
}

// SessionStore reads the durable agent-session lifecycle record. Mint consults
// it to refuse a credential for a session that is no longer active (codex F12).
type SessionStore interface {
	GetAgentSession(context.Context, string) (store.AgentSession, error)
}

// credentialMintPhases is deliberately a fail-closed allowlist shared by the
// Git and model minters. Hibernation temporarily moves a terminal row through
// new lifecycle values while its old pod can still exist; a denylist would make
// every future phase credential-eligible by default.
var credentialMintPhases = map[string]bool{
	"creating": true, "running": true, "resuming": true, "redispatching": true,
}

// currentSandboxCaller binds a gateway-resolved Pod to the exact durable
// sandbox generation. OpenSandbox names the session pod <sandbox_id>-0; a stale
// pod from a failed teardown carries the same session labels but a different
// immutable sandbox id after rehydration. PodUID remains mandatory as the
// gateway's Kubernetes-derived identity, while the durable generation match is
// made through the current sandbox id.
func currentSandboxCaller(session store.AgentSession, workspace, podName, podUID string) bool {
	return session.WorkspaceID == workspace &&
		credentialMintPhases[session.Phase] &&
		session.SandboxID != "" &&
		podUID != "" &&
		podName == session.SandboxID+"-0"
}

// Minter is bex-api's internal-only credential verb. It trusts only a request
// authenticated by Handler's HMAC and still rechecks branch/repository policy.
type Minter struct {
	GitHub      GitHubClient
	Connections ConnectionStore
	Sessions    SessionStore
	Audit       core.AuditSink
	Now         func() time.Time
}

func (m *Minter) Mint(ctx context.Context, req MintRequest) (response MintResponse, err error) {
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
			Caller: req.PodName, CallerMethod: "sandbox", Verb: AuditVerbMintCredential,
			Resource: core.WorkspaceObject(req.Workspace), Target: "agent-session:" + req.SessionID,
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
	if !currentSandboxCaller(session, req.Workspace, req.PodName, req.PodUID) {
		return MintResponse{}, ErrForbidden
	}
	if err := ValidateBranch(req.Branch); err != nil {
		return MintResponse{}, err
	}
	owner, name, _ := strings.Cut(repository, "/")
	// Resolve the connection that owns THIS repo's account (ADR075 §4). A
	// no-match — the repo's owner is not one of the workspace's connected
	// accounts — is a forbidden mint, exactly as the old owner-equality check was.
	connection, err := m.Connections.GetGitConnectionByOwner(ctx, req.Workspace, owner)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return MintResponse{}, ErrForbidden
		}
		return MintResponse{}, err
	}
	token, err := m.GitHub.MintSessionInstallationToken(ctx, connection.InstallationID, name)
	if err != nil {
		return MintResponse{}, err
	}
	if token.Token == "" || token.ExpiresAt.IsZero() {
		return MintResponse{}, fmt.Errorf("github returned an incomplete installation token")
	}
	return MintResponse{Username: "x-access-token", Token: token.Token, ExpiresAt: token.ExpiresAt.UTC().Format(time.RFC3339)}, nil
}

func (m *Minter) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}
