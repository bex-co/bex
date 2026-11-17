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

package sshgateway

import (
	"cmp"
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// ShellSession is what a shell session's workspace audit rows say about it
// (w9/m451): who opened it, on which instance, and from where. It carries no
// command, environment or terminal content, by construction: there is no
// field for any of them.
type ShellSession struct {
	// ID is the session's ssn- id; each phase's row id derives from it, so a
	// retried write records once.
	ID      string
	Subject string
	// Method is the gateway identity the session authenticated with
	// (core.MethodSSH, core.MethodShell).
	Method string
	Target apps.SSHInstanceTarget
	// SourceIP is the client's address; UserAgent the browser's, which native
	// SSH has none of (Render: "when available").
	SourceIP  string
	UserAgent string
}

// RecordShellEvent writes s's verb row, core.AuditVerbStartShell or
// core.AuditVerbEndShell (Render's StartShellEvent and EndShellEvent), in the
// target's workspace audit log. An agent-session sandbox is not a service
// shell (its reference product is Devin, not Render) and records nothing. The
// write is bounded and fail-open (core.RecordAuditEvent): a failed write is
// logged and never blocks or ends the session. A nil sink records nothing.
func RecordShellEvent(sink core.AuditSink, verb string, s ShellSession, at time.Time) {
	if sink == nil || s.Target.Sandbox || s.Target.ServiceID == "" {
		return
	}
	workspace := cmp.Or(s.Target.OwnerID, core.DefaultTenant)
	core.RecordAuditEvent(context.Background(), sink, core.AuditEvent{
		ID:     ids.Derive(ids.Audit, s.ID, verb),
		Caller: s.Subject, CallerMethod: s.Method, Verb: verb,
		Resource: core.WorkspaceObject(workspace), Target: core.ServiceTarget(s.Target.ServiceID),
		Outcome: core.AuditAllowed, At: at,
		SourceIP: s.SourceIP, UserAgent: core.AuditUserAgent(s.UserAgent), InstanceID: s.Target.ID,
	})
}
