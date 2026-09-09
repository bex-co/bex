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
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const interruptedDispatchReason = "Sandbox provisioning was interrupted before it could be recorded. Send the prompt again to retry."

func (s *Service) abandonDispatch(ctx context.Context, row store.AgentSession, turn int, reason string) {
	d := store.AgentDispatch{SessionID: row.ID, WorkspaceID: row.WorkspaceID, Turn: turn}
	fact, err := s.Store.AbandonAgentDispatch(ctx, d, time.Now(), reason)
	if err != nil {
		log.Printf("agent-session dispatch: failure persistence failed (session=%s turn=%d): %v", row.ID, turn, err)
		return
	}
	s.Metrics.observeDispatchFailed(fact)
	if err := s.Sandbox.CleanupAgentDispatches(ctx, []store.AgentDispatch{d}); err != nil {
		log.Printf("agent-session dispatch: cleanup deferred (session=%s turn=%d): %v", row.ID, turn, err)
	}
}

// Recovery follows normal completion and spends at most ten seconds per tick.
// Successful tombstones are revisited hourly; failures rotate after a minute.
func (c *Completer) recoverDispatches(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	due, err := c.Store.ListAgentDispatchesDue(ctx, c.now())
	if err != nil {
		log.Printf("agent-session dispatch recovery: list: %v", err)
		return
	}
	groups := make(map[string][]store.AgentDispatch)
	for _, d := range due {
		if ctx.Err() != nil {
			return
		}
		fact, err := c.Store.AbandonAgentDispatch(ctx, d, c.now(), interruptedDispatchReason)
		if err != nil {
			continue
		}
		c.Metrics.observeDispatchFailed(fact)
		// Rotate before remote I/O so an unavailable workspace cannot starve others.
		if err := c.Store.DeferAgentDispatchCleanup(ctx, d, c.now().Add(time.Minute)); err != nil {
			continue
		}
		groups[d.WorkspaceID] = append(groups[d.WorkspaceID], d)
	}
	for workspace, dispatches := range groups {
		if ctx.Err() != nil {
			return
		}
		if err := c.Sandbox.CleanupAgentDispatches(ctx, dispatches); err != nil {
			log.Printf("agent-session dispatch recovery: cleanup failed (workspace=%s): %v", workspace, err)
			continue
		}
		for _, d := range dispatches {
			if err := c.Store.DeferAgentDispatchCleanup(ctx, d, c.now().Add(time.Hour)); err != nil {
				log.Printf("agent-session dispatch recovery: schedule cleanup: %v", err)
			}
		}
	}
}
