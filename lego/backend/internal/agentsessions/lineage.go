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
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Parent and child sessions (ADR102). A child is an ordinary session with a
// parent: it is created through Create with CreateRequest.ParentSessionID, and
// this file holds the rules that make that safe.
const (
	// ParentNotFoundCode: the parent id names no session of the workspace.
	ParentNotFoundCode = "AGENT_SESSION_PARENT_NOT_FOUND"
	// ParentNotActiveCode: the parent is no longer working, so it cannot spawn.
	ParentNotActiveCode = "AGENT_SESSION_PARENT_NOT_ACTIVE"
	// DepthLimitCode: the child would sit deeper than store.MaxAgentSessionDepth.
	DepthLimitCode = "AGENT_SESSION_DEPTH_LIMIT"
	// ChildLimitCode: the parent already has store.MaxLiveAgentSessionChildren live children.
	ChildLimitCode = "AGENT_SESSION_CHILD_LIMIT"
	// ChildSpendExceedsParentCode: the child's maxSpendUSD is more than the
	// parent's tree has left under its ceiling.
	ChildSpendExceedsParentCode = "AGENT_SESSION_CHILD_SPEND_EXCEEDS_PARENT"
	// ChildSpendExhaustedCode: the parent's tree has spent its whole ceiling.
	ChildSpendExhaustedCode = "AGENT_SESSION_CHILD_SPEND_EXHAUSTED"
	// ChildSecretNotInheritedCode: the child asked for a secret its parent lacks.
	ChildSecretNotInheritedCode = "AGENT_SESSION_CHILD_SECRET_NOT_INHERITED"
	// ParentCanceledCode is the failureReasonCode of a session canceled because
	// an ancestor was.
	ParentCanceledCode = "AGENT_SESSION_PARENT_CANCELED"
	// CreatorRemovedCode is the failureReasonCode of a session canceled because
	// its creator's membership of the workspace ended (w5/186).
	CreatorRemovedCode = "AGENT_SESSION_CREATOR_REMOVED"
	// BranchInUseCode refuses a create or spawn naming a branch another
	// session in the workspace holds on the same repository (w5/m382).
	BranchInUseCode = "AGENT_SESSION_BRANCH_IN_USE"
)

func (s *Service) authorizeParent(ctx context.Context, parentID string) error {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(parentID)); err != nil {
		return err
	}
	return validateSessionID(parentID)
}

func parentNotFound(id string) error {
	return core.NewNotFoundError(ParentNotFoundCode, "parent agent session not found", map[string]any{"parentSessionId": id})
}

// childContext is what a child create needs to know about its parent.
type childContext struct {
	parent store.AgentSession
	// headroom is what the parent's tree may still spend under the tightest
	// ceiling on the parent's chain; bounded is false when no ceiling applies.
	headroom int64
	bounded  bool
	// secrets is the child's secret set as inherited and narrowed (D4).
	secrets SessionSecretsInput
}

// resolveParent loads the parent and applies the rules of D2 to D4 that
// need it: same workspace, still working, the spend it has left, and the
// secrets the child may take. The store re-checks the first two and the depth
// and fan-out limits atomically when it inserts the child.
func (s *Service) resolveParent(ctx context.Context, workspaceID string, req CreateRequest) (*childContext, error) {
	parent, err := s.Store.GetAgentSession(ctx, req.ParentSessionID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && parent.WorkspaceID != workspaceID) {
		return nil, parentNotFound(req.ParentSessionID)
	}
	if err != nil {
		return nil, mapStoreError(req.ParentSessionID, err)
	}
	if !slices.Contains(activeParentPhases, parent.Phase) {
		return nil, parentNotActive(parent.ID, parent.Phase)
	}
	headroom, bounded, err := s.Store.AgentSessionCeilingHeadroom(ctx, workspaceID, parent.ID)
	if err != nil {
		return nil, mapStoreError(parent.ID, err)
	}
	secrets, err := s.inheritSecrets(ctx, parent, req.SessionSecretsInput)
	if err != nil {
		return nil, err
	}
	return &childContext{parent: parent, headroom: headroom, bounded: bounded, secrets: secrets}, nil
}

// activeParentPhases are the phases a parent can spawn from: the working ones.
// A completed or canceled session is steered or restarted first.
var activeParentPhases = []string{PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching}

func parentNotActive(id, phase string) error {
	return core.NewConflictError(ParentNotActiveCode,
		"the parent agent session is not working; a session can only start children while it is running",
		map[string]any{"parentSessionId": id, "phase": phase})
}

// capCeiling applies D3 at creation: a child may not ask for more than its
// parent's tree has left, and a tree that has spent everything starts nothing.
// ceiling is the child's own parsed maxSpendUSD, nil when it set none (it is
// then bounded at run time by the tree alone).
func (c *childContext) capCeiling(requestedUSD *float64, ceiling *int64) error {
	if !c.bounded {
		return nil
	}
	if c.headroom <= 0 {
		return core.NewConflictError(ChildSpendExhaustedCode,
			"the parent's spend ceiling is used up, so it cannot start another child", map[string]any{"parentSessionId": c.parent.ID})
	}
	if ceiling != nil && *ceiling > c.headroom {
		return core.NewBadRequestError(ChildSpendExceedsParentCode,
			fmt.Sprintf("maxSpendUSD is more than the parent's remaining ceiling of $%s; a child's spend counts against its parent's", microUSDDecimal(c.headroom)),
			map[string]any{"field": spendLimitField, "parentSessionId": c.parent.ID, "remainingUSD": microUSDDecimal(c.headroom), "requestedUSD": requestedUSD})
	}
	return nil
}

// inheritSecrets gives a child its parent's secrets (D4). With secretNames
// omitted it takes all of them; with secretNames given (an empty list means
// none) it takes just those, each of which the parent must have. Inline
// secrets are refused: they would add a secret the parent lacks. A secret the
// parent holds inline is copied to the child as an inline secret of its own.
func (s *Service) inheritSecrets(ctx context.Context, parent store.AgentSession, in SessionSecretsInput) (SessionSecretsInput, error) {
	if len(in.Secrets) > 0 {
		return SessionSecretsInput{}, core.NewBadRequestError(ChildSecretNotInheritedCode,
			"a child session cannot take inline secrets; it inherits its parent's secrets and may only narrow them with secretNames",
			map[string]any{"field": "secrets", "name": in.Secrets[0].Name})
	}
	selected := parent.SecretNames
	if in.SecretNames != nil {
		for _, name := range in.SecretNames {
			if !slices.Contains(parent.SecretNames, name) {
				return SessionSecretsInput{}, core.NewBadRequestError(ChildSecretNotInheritedCode,
					fmt.Sprintf("secret %q is not one of the parent session's secrets; a child can use a subset of them, never more", name),
					map[string]any{"field": "secretNames", "name": name})
			}
		}
		selected = in.SecretNames
	}
	if len(selected) == 0 {
		return SessionSecretsInput{}, nil
	}
	if s.ModelKeys == nil {
		return SessionSecretsInput{}, errSecretsUnavailable()
	}
	inline, _, err := s.sessionInline(ctx, parent)
	if err != nil {
		return SessionSecretsInput{}, err
	}
	var out SessionSecretsInput
	for _, name := range selected {
		if value, ok := inline[name]; ok {
			out.Secrets = append(out.Secrets, SecretInput{Name: name, Value: value})
		} else {
			out.SecretNames = append(out.SecretNames, name)
		}
	}
	return out, nil
}

// lineageRefusal maps the store's refusals of a child insert to the named
// errors, nil for any other error (including none).
func lineageRefusal(parentID string, err error) error {
	switch {
	case errors.Is(err, store.ErrAgentSessionParentNotFound):
		return parentNotFound(parentID)
	case errors.Is(err, store.ErrAgentSessionParentInactive):
		return parentNotActive(parentID, "")
	case errors.Is(err, store.ErrAgentSessionDepthLimit):
		return core.NewConflictError(DepthLimitCode,
			fmt.Sprintf("agent sessions can be nested at most %d levels deep", store.MaxAgentSessionDepth),
			map[string]any{"parentSessionId": parentID, "maxDepth": store.MaxAgentSessionDepth})
	case errors.Is(err, store.ErrAgentSessionFanOutLimit):
		return core.NewConflictError(ChildLimitCode,
			fmt.Sprintf("a session can have at most %d live children; wait for one to finish or cancel one", store.MaxLiveAgentSessionChildren),
			map[string]any{"parentSessionId": parentID, "maxLiveChildren": store.MaxLiveAgentSessionChildren})
	}
	return nil
}

// cancelDescendants cancels every unfinished descendant of the canceling
// session, deepest first so a child is stopped before the session that could
// still be steering it. The caller holds can_operate on the root of the
// cancel, and descendants live in its workspace, so no per-session check is
// made. Every descendant is attempted; the first failure is returned, leaving
// that session canceling for a retry of the parent's cancel to converge.
func (s *Service) cancelDescendants(ctx context.Context, parent store.AgentSession) error {
	descendants, err := s.Store.ListUnfinishedAgentSessionDescendants(ctx, parent.WorkspaceID, parent.ID)
	if err != nil {
		return mapStoreError(parent.ID, err)
	}
	var first error
	for _, d := range descendants {
		if err := s.cancelDescendant(ctx, d, parent.ID); err != nil {
			log.Printf("agent-session cancel cascade: descendant not canceled (parent=%s session=%s): %v", parent.ID, d.ID, err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func (s *Service) cancelDescendant(ctx context.Context, d store.AgentSession, causeID string) error {
	canceled, err := s.cancelTree(ctx, d, false)
	if err != nil {
		return err
	}
	s.recordCancelCause(ctx, canceled,
		fmt.Sprintf("canceled because its parent session %s was canceled", causeID), ParentCanceledCode)
	return nil
}

// cancelTree takes a session to canceled. The flip to canceling comes first:
// it is what stops the session's git and model mints
// (agentsession.CurrentSandboxCaller), and a canceling session can start no
// new child, so with withDescendants the descendants listed next are the whole
// set (ADR102 D6). They are canceled before the session finishes, so a
// failure leaves it canceling and a retry converges the rest.
func (s *Service) cancelTree(ctx context.Context, record store.AgentSession, withDescendants bool) (store.AgentSession, error) {
	if record.Phase != PhaseCanceling {
		var err error
		if record, err = s.Store.SetAgentSessionLifecycle(ctx, record.ID, "", PhaseCanceling, "canceling", false); err != nil {
			return store.AgentSession{}, mapStoreError(record.ID, err)
		}
	}
	if withDescendants {
		if err := s.cancelDescendants(ctx, record); err != nil {
			return store.AgentSession{}, err
		}
	}
	return s.finishCancel(ctx, record)
}

// recordCancelCause records why a session the platform canceled was canceled;
// a person's own cancel records none.
func (s *Service) recordCancelCause(ctx context.Context, canceled store.AgentSession, reason, code string) {
	if canceled.Phase != PhaseCanceled {
		return
	}
	if err := s.Store.SetAgentSessionCancelCause(ctx, canceled.ID, reason, code); err != nil {
		log.Printf("agent-session cancel: record cause failed (session=%s): %v", canceled.ID, err)
	}
}

// memberSessionPage bounds one read of a member's unfinished sessions.
const memberSessionPage = 100

// MemberExit is the trusted adapter the members service ends a departing
// member's sessions through (w5/186). It is not a Service verb: only the
// composition root hands it to the members service, whose exit verbs did the
// authorization.
type MemberExit struct{ Service *Service }

// CancelMemberSessions cancels every unfinished session that creators (a
// departing member's subject and the client ids of the API keys they created
// here) started in workspaceID, with the sessions below each (w5/186, ADR107).
// Without it a removed member's running turn kept minting git and model
// credentials until its turn timeout; every later step (steer, spawn, resume,
// the next automation fire) was already refused. Automation and child
// sessions carry their creator's subject, so they are included.
//
// No per-session check is made. It runs in two passes:
//
//  1. Flip every session to canceling, which stops its mints. A failure here
//     is returned, so the membership change fails and its retry converges.
//  2. Tear each one down with its descendants. Best effort: a canceling
//     session mints nothing, and the sandbox inventory reconciler stops a
//     canceling session's sandbox, so a sandbox outage must not block the
//     exit. The first failure ends the pass, so an unreachable sandbox API
//     costs one timeout rather than one per session; the sessions left
//     canceling converge on a later Cancel.
func (m MemberExit) CancelMemberSessions(ctx context.Context, workspaceID string, creators []string) error {
	s := m.Service
	creators = slices.DeleteFunc(slices.Clone(creators), func(c string) bool { return c == "" })
	if s == nil || !s.enabled() || len(creators) == 0 {
		return nil
	}
	phases, _ := phasesOfState(StateWorking)
	query := store.AgentSessionListQuery{
		Archived:  store.ArchivedInclude, // archiving does not stop a running turn
		Phases:    phases,
		CreatedBy: creators,
		Limit:     memberSessionPage,
	}
	var stopped []store.AgentSession
	for {
		page, err := s.Store.ListAgentSessions(ctx, workspaceID, query)
		if err != nil {
			return fmt.Errorf("list the sessions %v started: %w", creators, err)
		}
		for _, listed := range page {
			// Re-read: its turn may have finished since the list, and the
			// lifecycle write would reopen a finished session as canceling.
			d, err := s.Store.GetAgentSession(ctx, listed.ID)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return mapStoreError(listed.ID, err)
			}
			if finishedPhase(d.Phase) {
				continue
			}
			if d.Phase != PhaseCanceling {
				if d, err = s.Store.SetAgentSessionLifecycle(ctx, d.ID, "", PhaseCanceling, "canceling", false); err != nil {
					return mapStoreError(listed.ID, err)
				}
			}
			stopped = append(stopped, d)
		}
		if len(page) < memberSessionPage {
			break
		}
		query.Cursor = page[len(page)-1].ID
	}
	reason := "canceled because its creator is no longer a member of the workspace"
	for _, d := range stopped {
		canceled, err := s.cancelTree(ctx, d, true)
		if err != nil {
			log.Printf("agent-session cancel: a departed member's sessions are left canceling (workspace=%s, from session %s): %v", workspaceID, d.ID, err)
			break
		}
		s.recordCancelCause(ctx, canceled, reason, CreatorRemovedCode)
	}
	return nil
}

// Tree is a session tree (ADR102 D8): the root and every session below it.
type Tree struct {
	RootSessionID string `json:"rootSessionId"`
	// Sessions is the root first, then by depth and creation time. Each view is
	// the list view of the session: state, phase and outcome, without the
	// summary and shared-file reads only a single-session Get pays.
	Sessions []View `json:"sessions"`
}

// Tree returns the tree the named session belongs to, whichever node it names.
// It is authorized as Get is: the caller must be able to operate the named
// session, and the tree is confined to that session's workspace.
func (s *Service) Tree(ctx context.Context, sessionID string) (Tree, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return Tree{}, err
	}
	if !s.enabled() {
		return Tree{}, core.ErrAgentSessionsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return Tree{}, err
	}
	record, err := s.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return Tree{}, mapStoreError(sessionID, err)
	}
	rootID := record.RootSessionID
	if rootID == "" {
		rootID = record.ID
	}
	rows, err := s.Store.AgentSessionSubtree(ctx, record.WorkspaceID, rootID)
	if err != nil {
		return Tree{}, mapStoreError(sessionID, err)
	}
	out := Tree{RootSessionID: rootID, Sessions: make([]View, 0, len(rows))}
	for _, row := range rows {
		view, err := s.toView(row)
		if err != nil {
			return Tree{}, err
		}
		out.Sessions = append(out.Sessions, view)
	}
	return out, nil
}

// validateLineageFilters refuses a parentSessionId or automationId that is not
// an id of its kind (a NUL byte or any other malformed text included) with a
// named 400 rather than quietly listing nothing.
func validateLineageFilters(q store.AgentSessionListQuery) error {
	if q.ParentSessionID != "" {
		if err := validateSessionID(q.ParentSessionID); err != nil {
			return core.NewBadRequestError("AGENT_SESSION_FILTER_INVALID",
				"parentSessionId must be an agent session id (ags-…)", map[string]any{"parentSessionId": q.ParentSessionID})
		}
	}
	if q.AutomationID != "" && !isAutomationID(q.AutomationID) {
		return core.NewBadRequestError("AGENT_SESSION_FILTER_INVALID",
			"automationId must be an agent automation id (aut-…)", map[string]any{"automationId": q.AutomationID})
	}
	return nil
}
