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
	"log"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The in-sandbox spawn path (ADR102 D9). A running agent starts a child session
// through its Pod-bound gateway URL; nothing about the caller is a bex
// credential. Authorization is decided in two layers:
//
//  1. The gateway proved which session's sandbox is calling, and
//     SpawnSession re-checks that the Pod is that session's current sandbox in
//     a working phase. That session IS the parent; the request body cannot
//     name another.
//  2. The child is created as the person who started the parent: the identity
//     recorded on the parent row (agent_sessions.created_by) goes through the
//     same Create as any client, so can_create on the workspace and
//     can_operate on the parent are re-checked now. A person who lost access
//     cannot keep spawning through a session they started earlier, and a
//     session created before the column existed (no recorded creator) cannot
//     spawn at all.

const (
	// SpawnNoCreatorCode: the parent records no creating subject.
	SpawnNoCreatorCode = "AGENT_SESSION_SPAWN_NO_CREATOR"
	// SpawnCreatorForbiddenCode: the person who started the parent can no
	// longer start sessions here.
	SpawnCreatorForbiddenCode = "AGENT_SESSION_SPAWN_CREATOR_FORBIDDEN"
	// ChildRepoMismatchCode: a child works in its parent's repository.
	ChildRepoMismatchCode = "AGENT_SESSION_CHILD_REPO_MISMATCH"
)

// SpawnRequest is what a session may say about its child. Everything else
// (agent, model, template, workspace, parent, egress) comes from the parent.
type SpawnRequest struct {
	Task string
	// Repo, when set, must be the parent's own repository: a child never
	// reaches a repository its parent could not.
	Repo   string
	Branch string
	Title  string
	Tags   []string
	// MaxSpendUSD, when set, must not exceed what the parent's tree has left.
	MaxSpendUSD *float64
	// SecretNames narrows the parent's secrets; nil inherits all of them and
	// an empty list gives the child none.
	SecretNames *[]string
	// IdempotencyKey is the caller's key for this one child; the create key
	// is derived from it and the parent, so a retried tool call returns the
	// first child. Empty means no idempotency.
	IdempotencyKey string
}

// forbiddenCode is the code of core.ErrForbidden, the bare authorization denial.
const forbiddenCode = "FORBIDDEN"

// spawnKeyPrefix keeps spawn keys apart from client and automation keys.
const spawnKeyPrefix = "spawn:"

// SpawnChild starts one child of parentSessionID. The caller must already
// have proven that the request comes from the parent's current sandbox
// (SpawnSession does); this method decides what the child may do and creates
// it through Create, so the limits, spend, secrets and persistence are the
// ones every client gets. The returned view has Created false on an
// idempotent replay.
func (s *Service) SpawnChild(ctx context.Context, parentSessionID string, req SpawnRequest) (View, error) {
	parent, err := s.Store.GetAgentSession(ctx, parentSessionID)
	if err != nil {
		return View{}, mapStoreError(parentSessionID, err)
	}
	if parent.CreatedBy == "" {
		return View{}, core.NewForbiddenError(SpawnNoCreatorCode,
			"this session does not record who started it, so it cannot start child sessions; start a new session to use this", nil)
	}
	var config AgentConfig
	if err := json.Unmarshal(parent.AgentConfig, &config); err != nil {
		return View{}, err
	}
	repo, err := childRepo(parent, req.Repo)
	if err != nil {
		return View{}, err
	}
	create := CreateRequest{
		OwnerID: parent.WorkspaceID, Repo: repo, Branch: req.Branch,
		AgentConfig: AgentConfig{Agent: config.Agent, Model: config.Model, Template: config.Template, Task: req.Task},
		Title:       req.Title, Tags: req.Tags, MaxSpendUSD: req.MaxSpendUSD,
		ParentSessionID: parent.ID,
	}
	if req.SecretNames != nil {
		create.SecretNames = append([]string{}, *req.SecretNames...)
	}
	if req.IdempotencyKey != "" {
		create.IdempotencyKey, create.platformKey = spawnKeyPrefix+parent.ID+":"+req.IdempotencyKey, true
	}
	// Act as the person who started the parent (ADR101 D4's rule).
	ctx = core.WithIdentity(ctx, core.Identity{Subject: parent.CreatedBy, Method: "session"})
	view, err := s.Create(ctx, create)
	var coded *core.CodedError
	if errors.Is(err, core.ErrForbidden) && errors.As(err, &coded) && coded.Code == forbiddenCode {
		// The bare authorization denial: a feature-specific forbidden keeps its own code.
		return View{}, core.NewForbiddenError(SpawnCreatorForbiddenCode,
			"the person who started this session can no longer start sessions in this workspace", nil)
	}
	return view, err
}

// childRepo is the repository a child works in: the parent's own.
func childRepo(parent store.AgentSession, requested string) (string, error) {
	if requested == "" {
		return parent.Repo, nil
	}
	normalized, err := agentsession.NormalizeRepository(requested)
	if err != nil || parent.Repo == "" || normalized != parent.Repo {
		return "", core.NewBadRequestError(ChildRepoMismatchCode,
			"a child session works in its parent's repository; leave repo empty to use it",
			map[string]any{"field": "repo", "parentSessionId": parent.ID})
	}
	return parent.Repo, nil
}

// verifiedParent re-reads the session the gateway named and holds the Pod to
// the same current-sandbox rule as every other gateway verb.
func (s *Service) verifiedParent(ctx context.Context, envelope agentsession.SpawnEnvelope) (store.AgentSession, error) {
	record, err := s.Store.GetAgentSession(ctx, envelope.SessionID)
	if err != nil || !agentsession.CurrentSandboxCaller(record, envelope.Workspace, envelope.PodName, envelope.PodUID) {
		return store.AgentSession{}, agentsession.NewSpawnError(agentsession.SpawnForbidden, "this session is not running a sandbox")
	}
	return record, nil
}

// SpawnSession is the internal spawn verb: SpawnChild for a gateway-verified
// Pod, with every refusal in the sandbox's vocabulary. Named refusals of the
// create path (depth, fan-out, spend, secrets, ...) pass through under their
// own code and message.
func (s *Service) SpawnSession(ctx context.Context, envelope agentsession.SpawnEnvelope) (agentsession.SpawnResponse, error) {
	parent, err := s.verifiedParent(ctx, envelope)
	if err != nil {
		return agentsession.SpawnResponse{}, err
	}
	content := envelope.Spawn
	view, err := s.SpawnChild(ctx, parent.ID, SpawnRequest{
		Task: content.Task, Repo: content.Repo, Branch: content.Branch, Title: content.Title, Tags: content.Tags,
		MaxSpendUSD: content.MaxSpendUSD, SecretNames: content.SecretNames, IdempotencyKey: content.IdempotencyKey,
	})
	if err != nil {
		return agentsession.SpawnResponse{}, spawnRefusal(parent.ID, err)
	}
	return agentsession.SpawnResponse{SessionID: view.ID, State: view.State, Created: view.Created == nil || *view.Created}, nil
}

// spawnRefusal puts a service error in the sandbox's vocabulary. A coded
// error keeps its code and message; anything else is logged and reported as
// unavailable, never relayed.
func spawnRefusal(parentID string, err error) *agentsession.SpawnError {
	return childRefusal(parentID, "started", err)
}

// childRefusal is spawnRefusal for any child verb; done names what failed
// ("started", "messaged", "canceled", "waited on"). The bare authorization
// denial reads as the creator's lost access, as on a spawn.
func childRefusal(parentID, done string, err error) *agentsession.SpawnError {
	var coded *core.CodedError
	if errors.As(err, &coded) && coded.Code == forbiddenCode {
		return agentsession.NewSpawnError(SpawnCreatorForbiddenCode, "the person who started this session can no longer act in this workspace")
	}
	if errors.As(err, &coded) && coded.Code != "" {
		return agentsession.NewSpawnError(coded.Code, coded.Error())
	}
	log.Printf("agent-session child: not %s (parent=%s): %v", done, parentID, err)
	return agentsession.NewSpawnError(agentsession.SpawnUnavailable, "the child session could not be "+done)
}

// ReadChildSession is the internal child-read verb: one descendant of the
// verified session (a child, a grandchild, ...), never a sibling, an ancestor
// or any other session. A session outside the descendants reads as not found,
// so the sandbox learns nothing about sessions it did not start.
func (s *Service) ReadChildSession(ctx context.Context, envelope agentsession.SpawnEnvelope) (agentsession.ChildView, error) {
	parent, err := s.verifiedParent(ctx, envelope)
	if err != nil {
		return agentsession.ChildView{}, err
	}
	if err := s.requireDescendants(ctx, parent, envelope.ChildID); err != nil {
		return agentsession.ChildView{}, err
	}
	return s.childView(ctx, envelope.ChildID)
}

// MessageChildSession starts a follow-up turn on one of the parent's
// descendants (w9/m346), as the parent's creator: Steer's own can_create
// check and every steer rule apply, and its refusals pass through by name.
func (s *Service) MessageChildSession(ctx context.Context, envelope agentsession.SpawnEnvelope) (agentsession.ChildView, error) {
	parent, creatorCtx, err := s.coordinatingParent(ctx, envelope, envelope.ChildID)
	if err != nil {
		return agentsession.ChildView{}, err
	}
	view, err := s.Steer(creatorCtx, SteerRequest{SessionID: envelope.ChildID, Prompt: envelope.Message.Prompt})
	if err != nil {
		return agentsession.ChildView{}, childRefusal(parent.ID, "messaged", err)
	}
	return childViewOf(view), nil
}

// CancelChildSession cancels one of the parent's descendants, as the
// parent's creator.
func (s *Service) CancelChildSession(ctx context.Context, envelope agentsession.SpawnEnvelope) (agentsession.ChildView, error) {
	parent, creatorCtx, err := s.coordinatingParent(ctx, envelope, envelope.ChildID)
	if err != nil {
		return agentsession.ChildView{}, err
	}
	view, err := s.Cancel(creatorCtx, envelope.ChildID)
	if err != nil {
		return agentsession.ChildView{}, childRefusal(parent.ID, "canceled", err)
	}
	return childViewOf(view), nil
}

// WaitChildSessions blocks until every named descendant has settled or the
// deadline passes (Wait's LISTEN-driven loop), as the parent's creator. It
// answers with each child's state; outcomes are read with ReadChildSession.
func (s *Service) WaitChildSessions(ctx context.Context, envelope agentsession.SpawnEnvelope) (agentsession.SpawnWaitResponse, error) {
	parent, creatorCtx, err := s.coordinatingParent(ctx, envelope, envelope.Wait.ChildIDs...)
	if err != nil {
		return agentsession.SpawnWaitResponse{}, err
	}
	result, err := s.Wait(creatorCtx, WaitRequest{IDs: envelope.Wait.ChildIDs, TimeoutSeconds: envelope.Wait.TimeoutSeconds}, nil)
	if err != nil {
		return agentsession.SpawnWaitResponse{}, childRefusal(parent.ID, "waited on", err)
	}
	out := agentsession.SpawnWaitResponse{Settled: result.Settled, TimedOut: result.TimedOut, Children: make([]agentsession.SpawnWaitChild, 0, len(result.Sessions))}
	if out.Settled == nil {
		out.Settled = []string{}
	}
	for _, session := range result.Sessions {
		out.Children = append(out.Children, agentsession.SpawnWaitChild{SessionID: session.ID, State: session.State})
	}
	return out, nil
}

// coordinatingParent is the verified parent of a coordinator verb, every
// named child checked to be its descendant, and a context acting as the
// person who started it (ADR101 D4's rule, as SpawnChild does).
func (s *Service) coordinatingParent(ctx context.Context, envelope agentsession.SpawnEnvelope, childIDs ...string) (store.AgentSession, context.Context, error) {
	parent, err := s.verifiedParent(ctx, envelope)
	if err != nil {
		return store.AgentSession{}, nil, err
	}
	if parent.CreatedBy == "" {
		return store.AgentSession{}, nil, agentsession.NewSpawnError(SpawnNoCreatorCode,
			"this session does not record who started it, so it cannot manage child sessions")
	}
	if err := s.requireDescendants(ctx, parent, childIDs...); err != nil {
		return store.AgentSession{}, nil, err
	}
	ctx = core.WithIdentity(ctx, core.Identity{Subject: parent.CreatedBy, Method: "session"})
	return parent, core.WithWorkspace(ctx, parent.WorkspaceID), nil
}

// childViewOf is a session view as the parent's agent reads a child, without
// its outcome (the child read carries that).
func childViewOf(v View) agentsession.ChildView {
	return agentsession.ChildView{SessionID: v.ID, ParentSessionID: v.ParentSessionID, State: v.State,
		Title: v.Title, Repo: v.Repo, Branch: v.Branch, Depth: v.Depth}
}

// requireDescendants refuses, as not found, any id outside the parent's tree.
func (s *Service) requireDescendants(ctx context.Context, parent store.AgentSession, childIDs ...string) error {
	for _, childID := range childIDs {
		descendant, err := s.Store.IsAgentSessionDescendant(ctx, parent.WorkspaceID, parent.ID, childID)
		if err != nil {
			log.Printf("agent-session child access: lineage check failed (parent=%s): %v", parent.ID, err)
			return agentsession.NewSpawnError(agentsession.SpawnUnavailable, "the child session could not be read")
		}
		if !descendant {
			return agentsession.NewSpawnError(agentsession.SpawnNotFound, "no such child session")
		}
	}
	return nil
}

// childView is a descendant as the agent reads it.
func (s *Service) childView(ctx context.Context, childID string) (agentsession.ChildView, error) {
	record, err := s.Store.GetAgentSession(ctx, childID)
	if err != nil {
		return agentsession.ChildView{}, agentsession.NewSpawnError(agentsession.SpawnNotFound, "no such child session")
	}
	view, err := s.toView(record)
	if err != nil {
		return agentsession.ChildView{}, agentsession.NewSpawnError(agentsession.SpawnUnavailable, "the child session could not be read")
	}
	s.fillOutcome(ctx, record, view.Outcome)
	outcome, err := json.Marshal(view.Outcome)
	if err != nil {
		return agentsession.ChildView{}, agentsession.NewSpawnError(agentsession.SpawnUnavailable, "the child session could not be read")
	}
	return agentsession.ChildView{
		SessionID: view.ID, ParentSessionID: view.ParentSessionID, State: view.State, Title: view.Title,
		Repo: view.Repo, Branch: view.Branch, Depth: view.Depth, Outcome: outcome,
	}, nil
}

// withSpawnEnv gives a session that can still spawn its Pod-bound spawn URL.
// A session at the depth limit gets none, so the agent is never offered a tool
// that can only fail. The URL carries no credential.
func (s *Service) withSpawnEnv(env map[string]string, record store.AgentSession) map[string]string {
	if record.Depth >= store.MaxAgentSessionDepth {
		return env
	}
	url, err := agentsession.SpawnURL(s.gitProxyURL(), store.SandboxNamespace(record.WorkspaceID), record.ID)
	if err != nil {
		log.Printf("agent-session dispatch: spawn URL invalid (session=%s): %v", record.ID, err)
		return env
	}
	env["BEX_AGENT_SPAWN_URL"] = url
	return env
}
