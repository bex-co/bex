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
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
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
			s.Meter.Observe(ctx, raw)
			if err := s.Client.Terminate(ctx, key, raw.ID); err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
				break
			}
			raw.Status.State = string(StatusTerminated)
			s.Meter.Observe(ctx, raw)
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
