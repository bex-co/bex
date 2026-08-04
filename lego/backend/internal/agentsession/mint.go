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
	GetGitConnection(context.Context, string) (store.GitConnection, error)
}

// Minter is bex-api's internal-only credential verb. It trusts only a request
// authenticated by Handler's HMAC and still rechecks branch/repository policy.
type Minter struct {
	GitHub      GitHubClient
	Connections ConnectionStore
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
	if err := ValidateBranch(req.Branch); err != nil {
		return MintResponse{}, err
	}
	connection, err := m.Connections.GetGitConnection(ctx, req.Workspace)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return MintResponse{}, fmt.Errorf("agent credentials unavailable")
		}
		return MintResponse{}, err
	}
	owner, name, _ := strings.Cut(repository, "/")
	if !strings.EqualFold(owner, connection.AccountLogin) {
		return MintResponse{}, ErrForbidden
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
