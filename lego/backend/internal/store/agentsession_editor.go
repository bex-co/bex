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

import "context"

// EditorSandbox reports whether the session's sandbox is the one Resume
// started, or kept, for a person's editor (w6/m564, ADR054 D8).
func (s AgentSession) EditorSandbox() bool {
	return s.SandboxID != "" && s.SandboxID == s.EditorSandboxID
}

// BeginAgentSessionEditorStart takes a completed or failed session with no
// sandbox to resuming, the phase it reads while Resume starts a sandbox for a
// person's editor. ErrNotFound when the session is no longer in that state: a
// turn began, a start already runs, or it was archived or canceled.
func (s *PGStore) BeginAgentSessionEditorStart(ctx context.Context, id string) (AgentSession, error) {
	return s.updateAgentSession(ctx, `
		UPDATE agent_sessions SET phase='resuming', status='resuming', updated_at=now()
		WHERE id=$1 AND phase IN ('completed','failed') AND sandbox_id='' AND archived_at IS NULL
		RETURNING `+agentSessionColumns, id)
}

// BindAgentSessionEditorSandbox records the sandbox an editor start made, on
// a session still resuming with none and not archived. ErrNotFound when it is
// not: a cancel, an archive or the Completer's convergence came first, and the
// caller ends the sandbox it made.
func (s *PGStore) BindAgentSessionEditorSandbox(ctx context.Context, id, sandboxID string) (AgentSession, error) {
	return s.updateAgentSession(ctx, `
		UPDATE agent_sessions SET sandbox_id=$2, editor_sandbox_id=$2, updated_at=now()
		WHERE id=$1 AND phase='resuming' AND sandbox_id='' AND archived_at IS NULL
		RETURNING `+agentSessionColumns, id, sandboxID)
}

// SettleAgentSessionEditorStart returns a resuming session to the terminal
// phase it was resumed from, keeping whatever sandbox it has: failed when it
// carries a failure reason (a completed turn records none), else completed.
// updated_at, the idle clock, starts now. ErrNotFound when it is not resuming.
func (s *PGStore) SettleAgentSessionEditorStart(ctx context.Context, id string) (AgentSession, error) {
	return s.updateAgentSession(ctx, `
		UPDATE agent_sessions
		SET phase = CASE WHEN failure_reason <> '' THEN 'failed' ELSE 'completed' END,
		    status = CASE WHEN failure_reason <> '' THEN 'failed' ELSE 'completed' END,
		    updated_at=now()
		WHERE id=$1 AND phase='resuming'
		RETURNING `+agentSessionColumns, id)
}

// KeepAgentSessionSandboxForEditor marks a finished session's live sandbox as
// opened for an editor and restarts its idle clock, the Resume of a session
// whose sandbox is still up. ErrNotFound when the session no longer has that
// sandbox in a completed or failed phase, or was archived.
func (s *PGStore) KeepAgentSessionSandboxForEditor(ctx context.Context, id, sandboxID string) (AgentSession, error) {
	return s.updateAgentSession(ctx, `
		UPDATE agent_sessions SET editor_sandbox_id=sandbox_id, updated_at=now()
		WHERE id=$1 AND sandbox_id=$2 AND sandbox_id <> '' AND phase IN ('completed','failed') AND archived_at IS NULL
		RETURNING `+agentSessionColumns, id, sandboxID)
}

func (s *PGStore) updateAgentSession(ctx context.Context, sql string, args ...any) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, sql, args...))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}
