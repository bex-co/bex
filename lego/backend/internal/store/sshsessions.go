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
)

// SSHSessionAudit contains connection metadata only. Its schema deliberately
// has no command, environment, or stream-content column, making terminal-data
// capture impossible through this persistence seam.
type SSHSessionAudit struct {
	ID            string
	Subject       string
	WorkspaceID   string
	ServiceID     string
	InstanceID    string
	RemoteAddress string
	StartedAt     time.Time
}

func (s *PGStore) StartSSHSession(ctx context.Context, session SSHSessionAudit) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO ssh_sessions
			(id, subject, workspace_id, service_id, instance_id, remote_address, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		session.ID, session.Subject, session.WorkspaceID, session.ServiceID,
		session.InstanceID, session.RemoteAddress, session.StartedAt)
	return err
}

func (s *PGStore) EndSSHSession(ctx context.Context, sessionID, result string, endedAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE ssh_sessions SET ended_at = $2, result = $3 WHERE id = $1`,
		sessionID, endedAt, result)
	return err
}

// AgentSessionSSHActivity summarizes a resource's editor-SSH activity for the
// agent-session idle reaper (ADR059 D2 / w2/m67): whether a still-open session
// (ended_at IS NULL) started at or after `freshSince` exists — a live editor
// that pins the sandbox and must never be reaped — and the most recent close
// time (ended_at), which feeds the idle clock `now − max(last turn end, last SSH
// disconnect)`. `freshSince` (now − the SSH session cap) discards a leaked row a
// crashed gateway replica never closed, so an editor pin stays bounded. It
// subsumes ADR054 D6's earlier open-only check.
func (s *PGStore) AgentSessionSSHActivity(ctx context.Context, resourceID string, freshSince time.Time) (hasFreshOpen bool, lastEnded *time.Time, err error) {
	// COALESCE bool_or so an empty set (no rows for this id) scans as false, not NULL.
	err = s.Pool.QueryRow(ctx, `
		SELECT
			COALESCE(bool_or(ended_at IS NULL AND started_at >= $2), false),
			max(ended_at)
		FROM ssh_sessions
		WHERE service_id = $1`,
		resourceID, freshSince).Scan(&hasFreshOpen, &lastEnded)
	return hasFreshOpen, lastEnded, err
}

// PurgeSSHSessions applies the platform audit-retention window to SSH session
// metadata as well as ordinary audit_events. Remote addresses and target ids
// must not become an indefinitely retained second audit trail.
func (s *PGStore) PurgeSSHSessions(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM ssh_sessions WHERE started_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ClaimShellNonce atomically claims a web-shell exec-ticket nonce across ALL
// gateway replicas (w1/042 L7): INSERT … ON CONFLICT DO NOTHING lets exactly
// one claimant win; a second redemption of the same ticket — on any replica —
// finds the row present and is refused. Expired rows are pruned here rather
// than by a janitor: shells are human-driven and rare, so the piggybacked
// DELETE keeps the table at "tickets minted in the last ~90s" for free.
func (s *PGStore) ClaimShellNonce(ctx context.Context, nonce string, expiresAt time.Time) (bool, error) {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM shell_ticket_nonces WHERE expires_at < now()`); err != nil {
		return false, err
	}
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO shell_ticket_nonces (nonce, expires_at)
		VALUES ($1, $2) ON CONFLICT (nonce) DO NOTHING`, nonce, expiresAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
