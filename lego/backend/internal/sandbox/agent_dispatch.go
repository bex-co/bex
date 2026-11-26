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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CleanupAgentDispatches deletes only the abandoned generation, never a newer
// sandbox or its session-wide egress policy. Legacy intents predate turn labels.
func (l *AgentSessionLifecycle) CleanupAgentDispatches(ctx context.Context, dispatches []store.AgentDispatch) error {
	if len(dispatches) == 0 {
		return nil
	}
	s := l.service
	workspace := dispatches[0].WorkspaceID
	key, err := s.agentSessionKey(ctx, workspace)
	if err != nil {
		return err
	}
	rows, err := s.Client.List(ctx, key)
	if err != nil {
		return err
	}
	intents := make(map[string][]store.AgentDispatch)
	for _, d := range dispatches {
		if d.WorkspaceID == workspace {
			intents[d.SessionID] = append(intents[d.SessionID], d)
		}
	}
	var cleanupErr error
	for _, raw := range rows {
		if err := ctx.Err(); err != nil {
			return errors.Join(cleanupErr, err)
		}
		if !validOwnedSandbox(raw, workspace) {
			continue
		}
		for _, d := range intents[raw.Metadata[agentsession.LabelSession]] {
			turn := raw.Metadata[agentsession.LabelDispatchTurn]
			if raw.ID != d.PreviousSandboxID && turn != strconv.Itoa(d.Turn) && !(d.Legacy && turn == "") {
				continue
			}
			if err := s.terminateObserved(ctx, key, &raw); err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
				break
			}
			break
		}
	}
	// Session IDs are never reused. Only a deleted session permits deleting its
	// shared policy without racing a newer turn. The tombstone also revisits
	// this deletion if a delayed setup call writes the policy afterward.
	if s.SessionEgress != nil {
		for sessionID, ds := range intents {
			for _, d := range ds {
				if d.SessionDeleted {
					cleanupErr = errors.Join(cleanupErr, s.SessionEgress.Delete(ctx, store.SandboxNamespace(workspace), sessionID))
					break
				}
			}
		}
	}
	return cleanupErr
}

// earlierAttempt is what an earlier create of this session turn left, for a
// repeated create (AgentSessionSandbox.Repeat, w3/m405). OpenSandbox's create
// is not idempotent, and one whose answer was lost may have made the sandbox:
// a running one is the repeat's answer, and every other one, half created or
// failed, is ended so the new create leaves the turn exactly one.
func (s *Service) earlierAttempt(ctx context.Context, workspace string, bindings map[string]string) (Sandbox, bool, error) {
	session, turn := bindings[agentsession.LabelSession], bindings[agentsession.LabelDispatchTurn]
	if turn == "" {
		return Sandbox{}, false, fmt.Errorf("%w: a repeated agent sandbox create needs its dispatch turn", core.ErrBadRequest)
	}
	key, err := s.agentSessionKey(ctx, workspace)
	if err != nil {
		return Sandbox{}, false, err
	}
	rows, err := s.Client.ListByMetadata(ctx, key, map[string]string{agentsession.LabelSession: session, agentsession.LabelDispatchTurn: turn})
	if err != nil {
		return Sandbox{}, false, err
	}
	var kept *osSandbox
	for i := range rows {
		raw := &rows[i]
		if !validOwnedSandbox(*raw, workspace) || raw.Metadata[agentsession.LabelSession] != session || raw.Metadata[agentsession.LabelDispatchTurn] != turn {
			continue
		}
		if kept == nil && mapOpenSandboxStatus(raw.Status.State) == StatusRunning {
			kept = raw
			continue
		}
		log.Printf("sandbox create: a repeat ends sandbox %s (%s) an earlier attempt left (workspace=%s session=%s turn=%s)", raw.ID, raw.Status.State, workspace, session, turn)
		if err := s.terminateObserved(ctx, key, raw); err != nil {
			return Sandbox{}, false, err
		}
	}
	if kept == nil {
		return Sandbox{}, false, nil
	}
	log.Printf("sandbox create: a repeat takes running sandbox %s an earlier attempt made (workspace=%s session=%s turn=%s)", kept.ID, workspace, session, turn)
	s.Meter.Observe(ctx, *kept)
	s.Meter.Label(ctx, *kept)
	return s.sandboxView(*kept, workspace), true, nil
}
