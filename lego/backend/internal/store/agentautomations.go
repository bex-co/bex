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
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// AgentAutomation is one workspace agent automation (ADR101): triggers and a
// session template, both kept as the service layer's JSON.
type AgentAutomation struct {
	ID                  string
	WorkspaceID         string
	Name                string
	Enabled             bool
	CreatedBy           string
	Triggers            json.RawMessage
	Template            json.RawMessage
	OverlapPolicy       string
	NextFireAt          *time.Time
	LastFireAt          *time.Time
	LastSessionID       string
	PausedReason        string
	ConsecutiveFailures int
	// NotifyEmail is which run outcomes email the creator (w9/m343):
	// AutomationNotify*. Empty on a write means the default, on_failure.
	NotifyEmail string
	// NotifyFailureStreak counts consecutive failed runs for the email
	// backoff; the notice claim maintains it.
	NotifyFailureStreak int
	// PausedAt is when the platform last paused the automation (a pause with
	// a reason); PauseNotifiedAt when its creator was last told.
	PausedAt        *time.Time
	PauseNotifiedAt *time.Time
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Email notification settings of an automation (w9/m343).
const (
	AutomationNotifyAlways    = "always"
	AutomationNotifyOnFailure = "on_failure"
	AutomationNotifyOnSuccess = "on_success"
	AutomationNotifyNever     = "never"
)

// AutomationNotifyEmails are the email settings, in the order surfaces list them.
var AutomationNotifyEmails = []string{AutomationNotifyAlways, AutomationNotifyOnFailure, AutomationNotifyOnSuccess, AutomationNotifyNever}

// Reasons the platform pauses an automation (ADR101 D7); the migration 0209
// CHECK lists the same three.
const (
	AutomationPausedRepeatedFailures  = "repeated_failures"
	AutomationPausedCreatorLostAccess = "creator_lost_access"
	AutomationPausedTemplateInvalid   = "template_invalid"
)

// Outcomes of an AgentAutomationRun.
const (
	AutomationRunClaimed        = "claimed"
	AutomationRunStarted        = "started"
	AutomationRunSkippedOverlap = "skipped_overlap"
	AutomationRunFailedCreate   = "failed_create"
	AutomationRunDenied         = "denied"
)

// AgentAutomationRun is one fired tick of an automation.
type AgentAutomationRun struct {
	AutomationID string
	FireAt       time.Time
	Outcome      string
	SessionID    string
	MissedTicks  int
	Error        string
	ClaimedUntil *time.Time
	CreatedAt    time.Time
	SettledAt    *time.Time
}

// ClaimedRun is a run the caller now owns until its lease ends, with the
// automation row as it stood when claimed (after the schedule advanced).
type ClaimedRun struct {
	Run        AgentAutomationRun
	Automation AgentAutomation
}

// ClaimFailure is a due automation whose advance function failed; its row is
// left untouched.
type ClaimFailure struct {
	Automation AgentAutomation
	Err        error
}

// ClaimResult is what ClaimDueAutomations did: the runs claimed and the rows
// it skipped.
type ClaimResult struct {
	Runs     []ClaimedRun
	Failures []ClaimFailure
}

var (
	// ErrAgentAutomationVersion is an update whose expected version is stale.
	ErrAgentAutomationVersion = errors.New("agent automation version changed")
	// ErrAgentAutomationRunSettled is a settle of a run that is not claimed.
	ErrAgentAutomationRunSettled = errors.New("agent automation run already settled")
)

const agentAutomationColumns = `id, workspace_id, name, enabled, created_by, triggers, template, overlap_policy,
	next_fire_at, last_fire_at, coalesce(last_session_id, ''), coalesce(paused_reason, ''),
	consecutive_failures, notify_email, notify_failure_streak, paused_at, pause_notified_at,
	version, created_at, updated_at`

const agentAutomationRunColumns = `automation_id, fire_at, outcome, coalesce(session_id, ''), missed_ticks, error,
	claimed_until, created_at, settled_at`

func scanAgentAutomation(row pgx.Row) (AgentAutomation, error) {
	var a AgentAutomation
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.Name, &a.Enabled, &a.CreatedBy, &a.Triggers, &a.Template,
		&a.OverlapPolicy, &a.NextFireAt, &a.LastFireAt, &a.LastSessionID, &a.PausedReason,
		&a.ConsecutiveFailures, &a.NotifyEmail, &a.NotifyFailureStreak, &a.PausedAt, &a.PauseNotifiedAt,
		&a.Version, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func scanAgentAutomationRun(row pgx.Row) (AgentAutomationRun, error) {
	var r AgentAutomationRun
	err := row.Scan(&r.AutomationID, &r.FireAt, &r.Outcome, &r.SessionID, &r.MissedTicks, &r.Error,
		&r.ClaimedUntil, &r.CreatedAt, &r.SettledAt)
	return r, err
}

func collectWith[T any](scan func(pgx.Row) (T, error)) func(pgx.Rows, error) ([]T, error) {
	return func(rows pgx.Rows, err error) ([]T, error) {
		if err != nil {
			return nil, err
		}
		return pgx.CollectRows(rows, func(row pgx.CollectableRow) (T, error) { return scan(row) })
	}
}

func orEmptyJSON(raw json.RawMessage, empty string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(empty)
	}
	return raw
}

// CreateAgentAutomation inserts an automation at version 1. A taken name is
// ErrConflict.
func (s *PGStore) CreateAgentAutomation(ctx context.Context, in AgentAutomation) (AgentAutomation, error) {
	policy := in.OverlapPolicy
	if policy == "" {
		policy = "skip"
	}
	out, err := scanAgentAutomation(s.Pool.QueryRow(ctx, `
		INSERT INTO agent_automations (id, workspace_id, name, enabled, created_by, triggers, template,
			overlap_policy, next_fire_at, paused_reason, notify_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11)
		RETURNING `+agentAutomationColumns,
		in.ID, in.WorkspaceID, in.Name, in.Enabled, in.CreatedBy, orEmptyJSON(in.Triggers, "[]"),
		orEmptyJSON(in.Template, "{}"), policy, in.NextFireAt, in.PausedReason, cmp.Or(in.NotifyEmail, AutomationNotifyOnFailure)))
	if err != nil {
		return AgentAutomation{}, classify("agent automation", err)
	}
	return out, nil
}

// GetAgentAutomation reads one automation by id; ErrNotFound when there is none.
func (s *PGStore) GetAgentAutomation(ctx context.Context, id string) (AgentAutomation, error) {
	if !textBindable(id) {
		return AgentAutomation{}, ErrNotFound
	}
	out, err := scanAgentAutomation(s.Pool.QueryRow(ctx,
		`SELECT `+agentAutomationColumns+` FROM agent_automations WHERE id=$1`, id))
	if err != nil {
		return AgentAutomation{}, classify("agent automation", err)
	}
	return out, nil
}

// GetAgentAutomationByName reads one of the workspace's automations by name.
func (s *PGStore) GetAgentAutomationByName(ctx context.Context, workspaceID, name string) (AgentAutomation, error) {
	if !textBindable(name) {
		return AgentAutomation{}, ErrNotFound
	}
	out, err := scanAgentAutomation(s.Pool.QueryRow(ctx,
		`SELECT `+agentAutomationColumns+` FROM agent_automations WHERE workspace_id=$1 AND name=$2`, workspaceID, name))
	if err != nil {
		return AgentAutomation{}, classify("agent automation", err)
	}
	return out, nil
}

// ListAgentAutomations returns the workspace's automations newest first, at
// most limit of them, resuming strictly after the automation named by cursor
// (empty for the first page; an unknown cursor yields an empty page).
// createdBy, when set, keeps only that creator's automations.
func (s *PGStore) ListAgentAutomations(ctx context.Context, workspaceID, createdBy, cursor string, limit int) ([]AgentAutomation, error) {
	if (cursor != "" && !textBindable(cursor)) || !textBindable(createdBy) {
		return nil, nil
	}
	return collectWith(scanAgentAutomation)(s.Pool.Query(ctx, `
		SELECT `+agentAutomationColumns+` FROM agent_automations
		WHERE workspace_id=$1 AND ($2 = '' OR created_by=$2)
		  AND ($3 = '' OR (created_at, id) < (SELECT created_at, id FROM agent_automations WHERE id=$3))
		ORDER BY created_at DESC, id DESC LIMIT $4`,
		workspaceID, createdBy, cursor, clampLimit(limit)))
}

// CountAgentAutomations returns how many automations the workspace holds.
func (s *PGStore) CountAgentAutomations(ctx context.Context, workspaceID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_automations WHERE workspace_id=$1`, workspaceID).Scan(&n)
	return n, err
}

// clampLimit bounds a page: the services' largest page (200) plus the one
// look-ahead row they fetch to know whether a next page exists.
func clampLimit(limit int) int {
	if limit <= 0 || limit > 201 {
		return 201
	}
	return limit
}

// UpdateAgentAutomation replaces an automation's editable fields (name,
// enabled, triggers, template, overlap policy, email setting, next fire time) when its
// version is still ifVersion, and bumps the version. A stale version is
// ErrAgentAutomationVersion, a missing row ErrNotFound and a taken name
// ErrConflict. A non-empty CreatedBy replaces the creator (take-over, ADR101 D4).
// Pause state, counters and run history are not touched.
func (s *PGStore) UpdateAgentAutomation(ctx context.Context, in AgentAutomation, ifVersion int) (AgentAutomation, error) {
	policy := in.OverlapPolicy
	if policy == "" {
		policy = "skip"
	}
	out, err := scanAgentAutomation(s.Pool.QueryRow(ctx, `
		UPDATE agent_automations
		SET name=$3, enabled=$4, triggers=$5, template=$6, overlap_policy=$7, next_fire_at=$8,
		    created_by=COALESCE(NULLIF($10, ''), created_by), notify_email=$11,
		    version=version+1, updated_at=now()
		WHERE id=$1 AND version=$2 AND workspace_id=$9
		RETURNING `+agentAutomationColumns,
		in.ID, ifVersion, in.Name, in.Enabled, orEmptyJSON(in.Triggers, "[]"), orEmptyJSON(in.Template, "{}"),
		policy, in.NextFireAt, in.WorkspaceID, in.CreatedBy, in.NotifyEmail))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_automations WHERE id=$1 AND workspace_id=$2)`,
			in.ID, in.WorkspaceID).Scan(&exists); err != nil {
			return AgentAutomation{}, err
		}
		if !exists {
			return AgentAutomation{}, ErrNotFound
		}
		return AgentAutomation{}, ErrAgentAutomationVersion
	}
	if err != nil {
		return AgentAutomation{}, classify("agent automation", err)
	}
	return out, nil
}

// DeleteAgentAutomation removes one automation and its runs; the sessions it
// started stay. ErrNotFound when there is none.
func (s *PGStore) DeleteAgentAutomation(ctx context.Context, id string) error {
	if !textBindable(id) {
		return ErrNotFound
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM agent_automations WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAgentAutomationPaused pauses an automation (enabled=false, with reason)
// or resumes it (enabled=true, no reason, consecutive failures and the email
// failure streak reset), and bumps the version. A pause with a reason is a
// platform pause: it stamps paused_at once (a repeated pause keeps it) and
// leaves the pause to be notified (w9/m343). A manual pause or a resume clears
// both. The caller supplies a new next_fire_at through Update.
func (s *PGStore) SetAgentAutomationPaused(ctx context.Context, id string, paused bool, reason string) (AgentAutomation, error) {
	if !textBindable(id) {
		return AgentAutomation{}, ErrNotFound
	}
	out, err := scanAgentAutomation(s.Pool.QueryRow(ctx, `
		UPDATE agent_automations
		SET enabled = NOT $2,
		    paused_reason = CASE WHEN $2 THEN NULLIF($3, '') ELSE NULL END,
		    paused_at = CASE WHEN NOT $2 OR $3 = '' THEN NULL
		                     WHEN paused_reason IS NOT NULL AND paused_at IS NOT NULL THEN paused_at
		                     ELSE now() END,
		    pause_notified_at = CASE WHEN $2 AND $3 <> '' AND paused_reason IS NOT NULL AND paused_at IS NOT NULL
		                             THEN pause_notified_at END,
		    consecutive_failures = CASE WHEN $2 THEN consecutive_failures ELSE 0 END,
		    notify_failure_streak = CASE WHEN $2 THEN notify_failure_streak ELSE 0 END,
		    version=version+1, updated_at=now()
		WHERE id=$1
		RETURNING `+agentAutomationColumns, id, paused, reason))
	if err != nil {
		return AgentAutomation{}, classify("agent automation", err)
	}
	return out, nil
}

// ClaimDueAutomations is the scheduler's tick, in one transaction: it locks up
// to limit enabled automations whose next_fire_at is at or before now
// (FOR UPDATE SKIP LOCKED, so replicas never share a row), asks advance for
// each one's tick to run, its next future tick (nil when the schedule is
// finished, which also disables the automation) and the ticks skipped, writes
// the run row as 'claimed' with a lease of lease, and moves the automation's
// next_fire_at on. Because the row is advanced in the same transaction, no
// other replica sees it as due once this one commits. A run row that already
// exists for (automation, fire_at) is left alone and not returned. An advance
// error leaves that automation untouched and is reported in Failures.
func (s *PGStore) ClaimDueAutomations(ctx context.Context, now time.Time, limit int, lease time.Duration,
	advance func(a AgentAutomation) (fireAt time.Time, next *time.Time, missedTicks int, err error)) (ClaimResult, error) {
	var res ClaimResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		res = ClaimResult{}
		due, err := collectWith(scanAgentAutomation)(tx.Query(ctx, `
			SELECT `+agentAutomationColumns+` FROM agent_automations
			WHERE enabled AND next_fire_at <= $1
			ORDER BY next_fire_at, id
			FOR UPDATE SKIP LOCKED LIMIT $2`, now, clampLimit(limit)))
		if err != nil {
			return err
		}
		for _, a := range due {
			fireAt, next, missed, err := advance(a)
			if err != nil {
				res.Failures = append(res.Failures, ClaimFailure{Automation: a, Err: err})
				continue
			}
			// RETURNING hands back the new run row; a run that already exists
			// (ON CONFLICT DO NOTHING) returns none and is not fired again.
			run, err := scanAgentAutomationRun(tx.QueryRow(ctx, `
				INSERT INTO agent_automation_runs (automation_id, fire_at, outcome, missed_ticks, claimed_until)
				VALUES ($1, $2, 'claimed', $3, $4)
				ON CONFLICT (automation_id, fire_at) DO NOTHING
				RETURNING `+agentAutomationRunColumns, a.ID, fireAt, missed, now.Add(lease)))
			existed := errors.Is(err, pgx.ErrNoRows)
			if err != nil && !existed {
				return err
			}
			updated, err := scanAgentAutomation(tx.QueryRow(ctx, `
				UPDATE agent_automations
				SET next_fire_at=$2, last_fire_at=$3, enabled = enabled AND $2::timestamptz IS NOT NULL, updated_at=now()
				WHERE id=$1
				RETURNING `+agentAutomationColumns, a.ID, next, fireAt))
			if err != nil {
				return err
			}
			if existed {
				continue
			}
			res.Runs = append(res.Runs, ClaimedRun{Run: run, Automation: updated})
		}
		return nil
	})
	if err != nil {
		return ClaimResult{}, classify("agent automation", err)
	}
	return res, nil
}

// ClaimStaleRuns re-leases runs left 'claimed' past their lease (a replica
// died between claiming and settling): up to limit rows with claimed_until
// before now, locked SKIP LOCKED, each extended to now+lease, so a concurrent
// caller gets none of them.
func (s *PGStore) ClaimStaleRuns(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]ClaimedRun, error) {
	var out []ClaimedRun
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		out = nil
		runs, err := collectWith(scanAgentAutomationRun)(tx.Query(ctx, `
			SELECT `+agentAutomationRunColumns+` FROM agent_automation_runs
			WHERE outcome='claimed' AND claimed_until < $1
			ORDER BY claimed_until, automation_id, fire_at
			FOR UPDATE SKIP LOCKED LIMIT $2`, now, clampLimit(limit)))
		if err != nil {
			return err
		}
		until := now.Add(lease)
		for _, r := range runs {
			if _, err := tx.Exec(ctx, `UPDATE agent_automation_runs SET claimed_until=$3 WHERE automation_id=$1 AND fire_at=$2`,
				r.AutomationID, r.FireAt, until); err != nil {
				return err
			}
			r.ClaimedUntil = &until
			a, err := scanAgentAutomation(tx.QueryRow(ctx,
				`SELECT `+agentAutomationColumns+` FROM agent_automations WHERE id=$1`, r.AutomationID))
			if err != nil {
				return err
			}
			out = append(out, ClaimedRun{Run: r, Automation: a})
		}
		return nil
	})
	if err != nil {
		return nil, classify("agent automation run", err)
	}
	return out, nil
}

// SettleRun records a claimed run's outcome and, in the same transaction,
// updates the automation: 'started' stores sessionID as last_session_id and
// zeroes consecutive_failures, 'failed_create' and 'denied' add one to it,
// 'skipped_overlap' leaves it. It returns the new count. A run that is missing
// is ErrNotFound; one already settled is ErrAgentAutomationRunSettled.
func (s *PGStore) SettleRun(ctx context.Context, automationID string, fireAt time.Time, outcome, sessionID, errText string) (int, error) {
	if !textBindable(automationID) {
		return 0, ErrNotFound
	}
	var failures int
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE agent_automation_runs
			SET outcome=$3, session_id=NULLIF($4, ''), error=$5, claimed_until=NULL, settled_at=now()
			WHERE automation_id=$1 AND fire_at=$2 AND outcome='claimed'`,
			automationID, fireAt, outcome, sessionID, errText)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_automation_runs WHERE automation_id=$1 AND fire_at=$2)`,
				automationID, fireAt).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return ErrNotFound
			}
			return ErrAgentAutomationRunSettled
		}
		return tx.QueryRow(ctx, `
			UPDATE agent_automations
			SET consecutive_failures = CASE WHEN $2 = 'started' THEN 0
			                                WHEN $2 IN ('failed_create', 'denied') THEN consecutive_failures + 1
			                                ELSE consecutive_failures END,
			    last_session_id = CASE WHEN $2 = 'started' THEN NULLIF($3, '') ELSE last_session_id END,
			    updated_at = now()
			WHERE id=$1
			RETURNING consecutive_failures`, automationID, outcome, sessionID).Scan(&failures)
	})
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAgentAutomationRunSettled) {
		return 0, err
	}
	if err != nil {
		return 0, classify("agent automation run", err)
	}
	return failures, nil
}

// ListRuns returns an automation's runs newest first (by fire_at), at most
// limit of them, resuming strictly before cursor (zero for the first page).
func (s *PGStore) ListRuns(ctx context.Context, automationID string, cursor time.Time, limit int) ([]AgentAutomationRun, error) {
	if !textBindable(automationID) {
		return nil, nil
	}
	return collectWith(scanAgentAutomationRun)(s.Pool.Query(ctx, `
		SELECT `+agentAutomationRunColumns+` FROM agent_automation_runs
		WHERE automation_id=$1 AND ($2::timestamptz IS NULL OR fire_at < $2)
		ORDER BY fire_at DESC LIMIT $3`,
		automationID, nullTime(cursor), clampLimit(limit)))
}

// LatestStartedSessionPhase returns the session of the automation's most
// recent run that started one, with that session's phase; found is false when
// no run did. phase is empty when the session row no longer exists.
func (s *PGStore) LatestStartedSessionPhase(ctx context.Context, automationID string) (sessionID, phase string, found bool, err error) {
	if !textBindable(automationID) {
		return "", "", false, nil
	}
	err = s.Pool.QueryRow(ctx, `
		SELECT r.session_id, coalesce(g.phase, '')
		FROM (SELECT session_id FROM agent_automation_runs
		      WHERE automation_id=$1 AND outcome='started' AND session_id IS NOT NULL
		      ORDER BY fire_at DESC LIMIT 1) r
		LEFT JOIN agent_sessions g ON g.id = r.session_id`, automationID).Scan(&sessionID, &phase)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return sessionID, phase, true, nil
}

// InsertManualRun records a run-now fire (ADR101 D8): a 'claimed' run for the
// explicit fireAt, leased until now+lease so the recovery pass leaves it alone
// while its caller fires it. It does not touch the automation's schedule.
// inserted is false when a run already exists for (automation, fireAt); the
// existing row is not returned.
func (s *PGStore) InsertManualRun(ctx context.Context, automationID string, fireAt, now time.Time, lease time.Duration) (run AgentAutomationRun, inserted bool, err error) {
	if !textBindable(automationID) {
		return AgentAutomationRun{}, false, ErrNotFound
	}
	run, err = scanAgentAutomationRun(s.Pool.QueryRow(ctx, `
		INSERT INTO agent_automation_runs (automation_id, fire_at, outcome, missed_ticks, claimed_until)
		VALUES ($1, $2, 'claimed', 0, $3)
		ON CONFLICT (automation_id, fire_at) DO NOTHING
		RETURNING `+agentAutomationRunColumns, automationID, fireAt, now.Add(lease)))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentAutomationRun{}, false, nil
	}
	if err != nil {
		return AgentAutomationRun{}, false, classify("agent automation run", err)
	}
	return run, true, nil
}
