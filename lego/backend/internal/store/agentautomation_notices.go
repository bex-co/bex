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

package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// How a settled automation run ended, for its email decision (w9/m343). A
// run succeeds when its session completes and fails when its session fails
// or no session started (failed_create, denied). A canceled session and a
// session whose row is gone are neither. Overlap skips are never claimed.
const (
	AutomationRunResultSucceeded = "succeeded"
	AutomationRunResultFailed    = "failed"
	AutomationRunResultCanceled  = "canceled"
)

// AutomationRunNotice is one settled run claimed for its email decision,
// with the automation as it stands after this run moved its failure streak.
type AutomationRunNotice struct {
	Automation AgentAutomation
	Run        AgentAutomationRun
	Result     string
	// SessionFailureReason is the started session's failure reason, if any.
	SessionFailureReason string
	// EndedAt is when the run ended: its session's last update for a started
	// run, the settle time otherwise.
	EndedAt time.Time
}

func automationRunResult(outcome, phase string) string {
	switch {
	case outcome == AutomationRunFailedCreate || outcome == AutomationRunDenied:
		return AutomationRunResultFailed
	case phase == "completed":
		return AutomationRunResultSucceeded
	case phase == "failed":
		return AutomationRunResultFailed
	default:
		return AutomationRunResultCanceled
	}
}

// ClaimAutomationRunNotices claims up to limit settled runs that have not been
// considered for an email yet: every settled outcome except an overlap skip,
// and a started run only once its session has ended. It locks each run's
// automation too (SKIP LOCKED), so one caller handles an automation's pending
// runs in order, oldest settled first. In one transaction it marks the runs
// notified_at=now, so each is claimed exactly once across replicas, and moves
// each automation's notify_failure_streak run by run: a success resets it, a
// failure adds one, anything else leaves it.
func (s *PGStore) ClaimAutomationRunNotices(ctx context.Context, now time.Time, limit int) ([]AutomationRunNotice, error) {
	var out []AutomationRunNotice
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		out = nil
		rows, err := tx.Query(ctx, `
			SELECT r.automation_id, r.fire_at, r.outcome, coalesce(r.session_id, ''), r.missed_ticks, r.error,
			       r.claimed_until, r.created_at, r.settled_at,
			       coalesce(g.phase, ''), coalesce(g.failure_reason, ''), coalesce(g.updated_at, r.settled_at)
			FROM agent_automation_runs r
			JOIN agent_automations a ON a.id = r.automation_id
			LEFT JOIN agent_sessions g ON g.id = r.session_id
			WHERE r.notified_at IS NULL AND r.outcome NOT IN ('claimed', 'skipped_overlap')
			  AND (r.outcome <> 'started' OR g.id IS NULL OR g.phase = ANY($2))
			ORDER BY r.settled_at, r.automation_id, r.fire_at
			FOR UPDATE OF r, a SKIP LOCKED
			LIMIT $1`, clampLimit(limit), TerminalAgentSessionPhases)
		if err != nil {
			return err
		}
		var phases []string
		for rows.Next() {
			var n AutomationRunNotice
			var phase string
			if err := rows.Scan(&n.Run.AutomationID, &n.Run.FireAt, &n.Run.Outcome, &n.Run.SessionID, &n.Run.MissedTicks,
				&n.Run.Error, &n.Run.ClaimedUntil, &n.Run.CreatedAt, &n.Run.SettledAt,
				&phase, &n.SessionFailureReason, &n.EndedAt); err != nil {
				rows.Close()
				return err
			}
			out = append(out, n)
			phases = append(phases, phase)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(out) == 0 {
			return err
		}

		automationIDs := make([]string, 0, len(out))
		for _, n := range out {
			automationIDs = append(automationIDs, n.Run.AutomationID)
		}
		current, err := collectWith(scanAgentAutomation)(tx.Query(ctx,
			`SELECT `+agentAutomationColumns+` FROM agent_automations WHERE id = ANY($1)`, automationIDs))
		if err != nil {
			return err
		}
		byID := make(map[string]AgentAutomation, len(current))
		for _, a := range current {
			byID[a.ID] = a
		}
		fireAts := make([]time.Time, 0, len(out))
		for i := range out {
			a := byID[out[i].Run.AutomationID]
			out[i].Result = automationRunResult(out[i].Run.Outcome, phases[i])
			switch out[i].Result {
			case AutomationRunResultSucceeded:
				a.NotifyFailureStreak = 0
			case AutomationRunResultFailed:
				a.NotifyFailureStreak++
			}
			byID[a.ID] = a
			out[i].Automation = a
			fireAts = append(fireAts, out[i].Run.FireAt)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE agent_automation_runs r SET notified_at=$3
			FROM unnest($1::text[], $2::timestamptz[]) AS c(automation_id, fire_at)
			WHERE r.automation_id = c.automation_id AND r.fire_at = c.fire_at`, automationIDs, fireAts, now); err != nil {
			return err
		}
		streakIDs := make([]string, 0, len(byID))
		streaks := make([]int, 0, len(byID))
		for _, a := range current {
			if after := byID[a.ID].NotifyFailureStreak; after != a.NotifyFailureStreak {
				streakIDs = append(streakIDs, a.ID)
				streaks = append(streaks, after)
			}
		}
		if len(streakIDs) == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `
			UPDATE agent_automations a SET notify_failure_streak = c.streak
			FROM unnest($1::text[], $2::int[]) AS c(id, streak)
			WHERE a.id = c.id`, streakIDs, streaks)
		return err
	})
	if err != nil {
		return nil, classify("agent automation run", err)
	}
	return out, nil
}

// ClaimAutomationPauseNotices claims up to limit automations the platform
// paused whose creator has not been told about that pause yet, and marks each
// pause_notified_at=now in the same statement, so a pause is claimed once
// across replicas. It does not bump the version: a notice is not an edit.
func (s *PGStore) ClaimAutomationPauseNotices(ctx context.Context, now time.Time, limit int) ([]AgentAutomation, error) {
	out, err := collectWith(scanAgentAutomation)(s.Pool.Query(ctx, `
		UPDATE agent_automations SET pause_notified_at=$1
		WHERE id IN (
			SELECT id FROM agent_automations
			WHERE paused_reason IS NOT NULL AND paused_at IS NOT NULL AND pause_notified_at IS NULL
			ORDER BY paused_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $2)
		RETURNING `+agentAutomationColumns, now, clampLimit(limit)))
	if err != nil {
		return nil, classify("agent automation", err)
	}
	return out, nil
}

// ListPausedAutomationsForPush returns the automations the platform paused at
// or after since, oldest pause first, for the push worker's pause projection
// (deduped there by automation and pause instant). Trusted background read.
func (s *PGStore) ListPausedAutomationsForPush(ctx context.Context, since time.Time) ([]AgentAutomation, error) {
	return collectWith(scanAgentAutomation)(s.Pool.Query(ctx, `
		SELECT `+agentAutomationColumns+` FROM agent_automations
		WHERE paused_reason IS NOT NULL AND paused_at >= $1
		ORDER BY paused_at, id`, since))
}
