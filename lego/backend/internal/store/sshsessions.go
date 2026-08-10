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

// HasOpenSSHSession reports whether a still-open SSH session (ended_at IS NULL)
// targets the given resource id and started at or after `since` (ADR054 D6). The
// agent-session Completer uses it to defer a finished session's sandbox teardown
// while an editor is connected; `since` (now − the SSH session cap) discards a
// leaked row a crashed gateway replica never closed, so deferral stays bounded.
func (s *PGStore) HasOpenSSHSession(ctx context.Context, resourceID string, since time.Time) (bool, error) {
	var open bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM ssh_sessions
			WHERE service_id = $1 AND ended_at IS NULL AND started_at >= $2)`,
		resourceID, since).Scan(&open)
	return open, err
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
