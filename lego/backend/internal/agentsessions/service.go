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

package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const ticketTTL = 90 * time.Second

type Store interface {
	CreateAgentSession(context.Context, store.AgentSession) (store.AgentSession, error)
	GetAgentSession(context.Context, string) (store.AgentSession, error)
	ListAgentSessions(context.Context, string) ([]store.AgentSession, error)
	SetAgentSessionLifecycle(context.Context, string, string, string, string, bool) (store.AgentSession, error)
	DeleteAgentSession(context.Context, string) error
}

// TupleWriter establishes the resource-parent edge in OpenFGA. The production
// checker implements it directly; keeping the seam here makes the domain tests
// prove that persistence alone is never treated as authorization.
type TupleWriter interface {
	GrantAgentSessionWorkspace(context.Context, string, string) error
}

type SandboxLifecycle interface {
	CreateAgentSessionSandbox(context.Context, string, string, string, string, string, string, []string) (sandbox.Sandbox, error)
	EnterAgentSessionPhase(context.Context, string, string, string) error
	ResumeAgentSessionSandbox(context.Context, string, string, string) error
	CancelAgentSessionSandbox(context.Context, string, string, string) error
}

type Service struct {
	*core.Base
	Store        Store
	Tuples       TupleWriter
	Sandbox      SandboxLifecycle
	TicketSecret []byte
	GatewayURL   string
}

func (s *Service) enabled() bool {
	return s.Store != nil && s.Tuples != nil && s.Sandbox != nil
}

func (s *Service) ticketEnabled() bool {
	return len(s.TicketSecret) > 0 && strings.TrimSpace(s.GatewayURL) != ""
}

func sessionObject(id string) string { return "agent_session:" + id }

func validateSessionID(value string) error {
	kind, ok := ids.KindOf(value)
	if !ok || kind != ids.AgentSession {
		return core.NewBadRequestError("AGENT_SESSION_ID_INVALID", "invalid agent session id", map[string]any{"id": value})
	}
	return nil
}

func validateCreate(req *CreateRequest) error {
	if strings.TrimSpace(req.Repo) == "" || strings.TrimSpace(req.AgentConfig.Agent) == "" || strings.TrimSpace(req.AgentConfig.Task) == "" {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "repo, agentConfig.agent, and agentConfig.task are required", nil)
	}
	if strings.TrimSpace(req.Branch) == "" {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "branch is required", map[string]any{"field": "branch"})
	}
	if len(req.Repo) > 2048 || len(req.Branch) > 255 || len(req.AgentConfig.Agent) > 128 || len(req.AgentConfig.Model) > 255 || len(req.AgentConfig.ModelEndpoint) > 2048 || len(req.AgentConfig.Task) > 100_000 || len(req.AgentConfig.Template) > 128 {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "agent session input exceeds its size limit", nil)
	}
	repository, err := agentsession.NormalizeRepository(req.Repo)
	if err != nil {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "repo must identify a GitHub owner/repository", map[string]any{"field": "repo"})
	}
	branch := strings.TrimSpace(req.Branch)
	if err := agentsession.ValidateBranch(branch); err != nil {
		return core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "branch must start with bex-agent/", map[string]any{"field": "branch"})
	}
	req.Repo = repository
	req.Branch = branch
	return nil
}

// Create starts a new session or resumes ResumeSessionID. New sessions check
// can_operate on the named workspace before persistence; resume checks the
// first-class agent_session object directly, avoiding a caller-default-workspace
// gate that could disagree with the resource's own parent tuple.
func (s *Service) Create(ctx context.Context, req CreateRequest) (View, error) {
	if req.ResumeSessionID != "" {
		return s.Resume(ctx, req.ResumeSessionID)
	}
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateCreate(&req); err != nil {
		return View{}, err
	}
	modelEndpoint, egressAllowlist, err := createEgress(req.AgentConfig, req.EgressAllowlist)
	if err != nil {
		return View{}, err
	}
	req.AgentConfig.ModelEndpoint = modelEndpoint
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return View{}, core.ErrForbidden
	}
	config, err := json.Marshal(req.AgentConfig)
	if err != nil {
		return View{}, err
	}
	record, err := s.Store.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: workspaceID,
		Repo:        req.Repo,
		Branch:      req.Branch,
		AgentConfig: config,
	})
	if err != nil {
		return View{}, mapStoreError(record.ID, err)
	}
	// The row is not a reachable resource until this edge exists. If OpenFGA is
	// unavailable, compensate the row and refuse before any sandbox is started.
	if err := s.Tuples.GrantAgentSessionWorkspace(ctx, record.ID, workspaceID); err != nil {
		_ = s.Store.DeleteAgentSession(ctx, record.ID)
		return View{}, fmt.Errorf("%w: establish agent session authorization: %v", core.ErrAuthzUnavailable, err)
	}
	template := strings.TrimSpace(req.AgentConfig.Template)
	sb, err := s.Sandbox.CreateAgentSessionSandbox(ctx, workspaceID, template, record.ID, record.Repo, record.Branch, modelEndpoint, egressAllowlist)
	if err != nil {
		_, _ = s.Store.SetAgentSessionLifecycle(ctx, record.ID, "", PhaseFailed, "sandbox create failed", false)
		return View{}, err
	}
	if err := s.Sandbox.EnterAgentSessionPhase(ctx, workspaceID, record.ID, sb.ID); err != nil {
		_ = s.Sandbox.CancelAgentSessionSandbox(ctx, workspaceID, record.ID, sb.ID)
		_, _ = s.Store.SetAgentSessionLifecycle(ctx, record.ID, sb.ID, PhaseFailed, "egress phase transition failed", false)
		return View{}, err
	}
	phase := PhaseCreating
	if sb.Status == sandbox.StatusRunning {
		phase = PhaseRunning
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, record.ID, sb.ID, phase, string(sb.Status), false)
	if err != nil {
		// A sandbox without a durable binding is unsafe and unmanageable. Best
		// effort termination contains it; the original store failure is returned.
		_ = s.Sandbox.CancelAgentSessionSandbox(ctx, workspaceID, record.ID, sb.ID)
		return View{}, mapStoreError(record.ID, err)
	}
	return s.withTicket(ctx, record)
}

func (s *Service) List(ctx context.Context, ownerID string) ([]View, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return nil, err
	}
	if !s.enabled() {
		return nil, core.ErrAgentSessionsUnavailable
	}
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return nil, core.ErrForbidden
	}
	rows, err := s.Store.ListAgentSessions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, row := range rows {
		view, err := viewOf(row)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, sessionID string) (View, error) {
	// Authorize the named object before validating/fetching it: unauthorized and
	// cross-workspace probes are a tuple denial, never a DB existence oracle.
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return viewOf(record)
}

func (s *Service) Resume(ctx context.Context, sessionID string) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if !s.ticketEnabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.Phase == PhaseCanceled || record.Phase == PhaseCanceling || record.SandboxID == "" {
		return View{}, core.NewConflictError("AGENT_SESSION_NOT_RESUMABLE", "agent session cannot be resumed from its current phase", map[string]any{"phase": record.Phase})
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseResuming, "resuming", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if err := s.Sandbox.ResumeAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
		_, _ = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseFailed, "sandbox resume failed", false)
		return View{}, err
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseRunning, "running", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return s.withTicket(ctx, record)
}

func (s *Service) Cancel(ctx context.Context, sessionID string) (View, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return View{}, err
	}
	if !s.enabled() {
		return View{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return View{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.Phase == PhaseCanceled {
		return viewOf(record) // idempotent public cancel
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseCanceling, "canceling", false)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	if record.SandboxID != "" {
		if err := s.Sandbox.CancelAgentSessionSandbox(ctx, record.WorkspaceID, record.ID, record.SandboxID); err != nil {
			return View{}, err // leave canceling: a retry converges it
		}
	}
	record, err = s.Store.SetAgentSessionLifecycle(ctx, sessionID, "", PhaseCanceled, "canceled", true)
	if err != nil {
		return View{}, mapStoreError(sessionID, err)
	}
	return viewOf(record)
}

func (s *Service) withTicket(ctx context.Context, record store.AgentSession) (View, error) {
	identity, ok := core.IdentityFrom(ctx)
	if !ok || identity.Subject == "" {
		return View{}, core.ErrForbidden
	}
	now := s.Now()
	expires := now.Add(ticketTTL)
	ticket, err := agentsessionticket.Mint(s.TicketSecret, agentsessionticket.Claims{
		Subject: identity.Subject, SessionID: record.ID, SandboxID: record.SandboxID,
		Pod: record.SandboxID + "-0", Workspace: record.WorkspaceID,
		Namespace: record.WorkspaceID + "-sandbox", IssuedAt: now.Unix(), ExpiresAt: expires.Unix(),
	})
	if err != nil {
		return View{}, err
	}
	view, err := viewOf(record)
	if err != nil {
		return View{}, err
	}
	view.Ticket, view.URL, view.ExpiresAt = ticket, s.GatewayURL, &expires
	return view, nil
}

func viewOf(record store.AgentSession) (View, error) {
	var config AgentConfig
	if err := json.Unmarshal(record.AgentConfig, &config); err != nil {
		return View{}, fmt.Errorf("decode persisted agent config: %w", err)
	}
	return View{ID: record.ID, OwnerID: record.WorkspaceID, Repo: record.Repo, Branch: record.Branch,
		AgentConfig: config, SandboxID: record.SandboxID, Phase: record.Phase, Status: record.Status,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, CanceledAt: record.CanceledAt}, nil
}

func mapStoreError(id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return core.NewNotFoundError("AGENT_SESSION_NOT_FOUND", "agent session not found", map[string]any{"id": id})
	}
	if errors.Is(err, store.ErrConflict) {
		return core.NewConflictError("AGENT_SESSION_CONFLICT", "agent session conflicts with existing state", map[string]any{"id": id})
	}
	return err
}
