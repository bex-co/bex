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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// AgentAutomationRun is one fire of an automation as every surface shows it
// (ADR101 D3, D7).
type AgentAutomationRun struct {
	AutomationID string     `json:"automationId"`
	FireAt       time.Time  `json:"fireAt"`
	Outcome      string     `json:"outcome"`
	SessionID    string     `json:"sessionId,omitempty"`
	MissedTicks  int        `json:"missedTicks"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	SettledAt    *time.Time `json:"settledAt,omitempty"`
}

// AgentAutomationRunList is a newest-first page of runs. NextCursor is the
// last run's fireAt as RFC 3339 with nanoseconds, empty on the last page.
type AgentAutomationRunList struct {
	AutomationID string               `json:"automationId"`
	Runs         []AgentAutomationRun `json:"runs"`
	NextCursor   string               `json:"nextCursor,omitempty"`
}

// RunAgentAutomationNow fires the automation immediately as a normal run
// (fire_at = now), under the same authorization, overlap and spend rules as a
// scheduled fire, and returns the settled run. The creator or a workspace admin
// may; the run executes as the creator (ADR101 D4), and the schedule is not
// advanced. An automation paused for a reason must be resumed first. A
// transient failure returns the error and leaves the run claimed: the recovery
// pass finishes it after the lease.
func (s *Service) RunAgentAutomationNow(ctx context.Context, ownerID, ref string) (AgentAutomationRun, error) {
	row, _, err := s.scopeAutomationManaged(ctx, ownerID, ref)
	if err != nil {
		return AgentAutomationRun{}, err
	}
	if row.PausedReason != "" {
		return AgentAutomationRun{}, core.NewConflictError(CodeAutomationPaused,
			"the automation is paused ("+row.PausedReason+"); resume it first", map[string]any{"pausedReason": row.PausedReason})
	}
	now := s.Now().UTC().Truncate(time.Microsecond)
	run, inserted, err := s.Automations.InsertManualRun(ctx, row.ID, now, now, automationLease)
	if errors.Is(err, store.ErrNotFound) {
		return AgentAutomationRun{}, errAutomationNotFound(ref)
	}
	if err != nil {
		return AgentAutomationRun{}, err
	}
	if !inserted {
		return AgentAutomationRun{}, errAutomationConflict("a run for this instant already exists; try again")
	}
	if err := s.fireAutomationRun(ctx, store.ClaimedRun{Run: run, Automation: row}); err != nil {
		return AgentAutomationRun{}, err
	}
	settled, err := s.storedRun(ctx, run)
	if err != nil {
		return AgentAutomationRun{}, err
	}
	return automationRunView(settled), nil
}

// AgentAutomationRuns lists an automation's runs, newest first, a page at a
// time: cursor is the previous page's nextCursor (RFC 3339 with nanoseconds).
func (s *Service) AgentAutomationRuns(ctx context.Context, ownerID, ref, cursor string, limit int) (AgentAutomationRunList, error) {
	row, _, err := s.scopeAutomation(ctx, ownerID, ref, core.RelCanCreate, "")
	if err != nil {
		return AgentAutomationRunList{}, err
	}
	var after time.Time
	if cursor != "" {
		if after, err = time.Parse(time.RFC3339Nano, cursor); err != nil {
			return AgentAutomationRunList{}, automationInvalid("cursor", "cursor must be a value returned as nextCursor (RFC 3339 time)")
		}
	}
	if limit <= 0 {
		limit = defaultAutomationPageSize
	}
	limit = min(limit, 200)
	rows, err := s.Automations.ListRuns(ctx, row.ID, after, limit+1)
	if err != nil {
		return AgentAutomationRunList{}, err
	}
	out := AgentAutomationRunList{AutomationID: row.ID, Runs: make([]AgentAutomationRun, 0, len(rows))}
	if len(rows) > limit {
		rows = rows[:limit]
		out.NextCursor = rows[limit-1].FireAt.UTC().Format(time.RFC3339Nano)
	}
	for _, r := range rows {
		out.Runs = append(out.Runs, automationRunView(r))
	}
	return out, nil
}

func automationRunView(r store.AgentAutomationRun) AgentAutomationRun {
	return AgentAutomationRun{
		AutomationID: r.AutomationID, FireAt: r.FireAt, Outcome: r.Outcome, SessionID: r.SessionID,
		MissedTicks: r.MissedTicks, Error: r.Error, CreatedAt: r.CreatedAt, SettledAt: r.SettledAt,
	}
}

// storedRun re-reads a run row (its settled state).
func (s *Service) storedRun(ctx context.Context, run store.AgentAutomationRun) (store.AgentAutomationRun, error) {
	rows, err := s.Automations.ListRuns(ctx, run.AutomationID, run.FireAt.Add(time.Microsecond), 1)
	if err != nil {
		return store.AgentAutomationRun{}, err
	}
	if len(rows) == 0 || !rows[0].FireAt.Equal(run.FireAt) {
		return run, nil
	}
	return rows[0], nil
}
