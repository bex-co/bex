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
	"fmt"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The idle close of a finished turn's sandbox (w6/m563). A Devin session
// "sleeps automatically after 5 minutes of inactivity by default", and
// "Enterprise customers can … set a different timeout (between 5 and 120
// minutes)" (docs.devin.ai/admin/billing/usage, read 2026-11-22). bex closes
// the sandbox of a session a person started DefaultIdleTimeout after its last
// interaction, or after its Enterprise workspace's own idleTimeoutMinutes;
// an automation's and a child session's close at their harvest
// (Completer.idleGrace).
const (
	// DefaultIdleTimeout is BEX_AGENT_SANDBOX_IDLE_TTL's default.
	DefaultIdleTimeout = 5 * time.Minute
	// MinIdleTimeout and MaxIdleTimeout bound a workspace's own timeout.
	MinIdleTimeout = 5 * time.Minute
	MaxIdleTimeout = 120 * time.Minute
)

// idleTimeoutField is the wire name of the workspace's idle timeout.
const idleTimeoutField = "idleTimeoutMinutes"

// IdleTimeoutInvalidCode refuses an idleTimeoutMinutes outside 5 to 120.
const IdleTimeoutInvalidCode = "AGENT_ENVIRONMENT_IDLE_TIMEOUT_INVALID"

// validIdleTimeout refuses a workspace idle timeout outside the bounds.
func validIdleTimeout(minutes int) error {
	low, high := int(MinIdleTimeout/time.Minute), int(MaxIdleTimeout/time.Minute)
	if minutes >= low && minutes <= high {
		return nil
	}
	return core.NewBadRequestError(IdleTimeoutInvalidCode,
		fmt.Sprintf("idleTimeoutMinutes must be between %d and %d", low, high),
		map[string]any{"field": idleTimeoutField, "min": low, "max": high})
}

// idleTimeoutRefusal is the refusal of a write of workspaceID's own idle
// timeout: nil when its plan includes one, and where no plan applies (no
// plan reader, the default tenant), as every Base plan gate degrades.
func (s *Service) idleTimeoutRefusal(ctx context.Context, workspaceID string) (refusal, err error) {
	plan, ok, err := s.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok || core.PlanAllows(plan, core.FeatureAgentIdleTimeout) {
		return nil, err
	}
	return core.NewPlanFeatureError(idleTimeoutField, plan, core.FeatureAgentIdleTimeout), nil
}

// idleTimeoutConfigurable reports whether workspaceID's admins may set their
// own idle timeout, for a read: a plan read failure fails it closed.
func (s *Service) idleTimeoutConfigurable(ctx context.Context, workspaceID string) (bool, error) {
	refusal, err := s.idleTimeoutRefusal(ctx, workspaceID)
	if err != nil {
		log.Printf("agent environment: read plan failed (workspace=%s): %v", workspaceID, err)
		return false, core.NewAgentSessionDependencyUnavailableError()
	}
	return refusal == nil, nil
}

// IdleTimeouts is the trusted adapter the Completer reads workspaces' own
// idle timeouts through (Completer.IdleTimeouts). It is not a Service verb:
// the Completer acts as the platform on sessions it owns, and only the
// composition root hands it over.
type IdleTimeouts struct{ Service *Service }

// WorkspaceIdleTimeout is the workspace's own idle timeout for the sessions
// people start in it, which the Completer applies instead of its IdleTTL.
// ok is false when the workspace set none or its plan no longer includes it.
func (t IdleTimeouts) WorkspaceIdleTimeout(ctx context.Context, workspaceID string) (time.Duration, bool, error) {
	row, err := t.Service.workspaceEnvironment(ctx, workspaceID)
	if err != nil || row.IdleTimeoutMinutes == nil {
		return 0, false, err
	}
	refusal, err := t.Service.idleTimeoutRefusal(ctx, workspaceID)
	if err != nil {
		return 0, false, err
	}
	own, ok := ownIdleTimeout(row, refusal == nil)
	return own, ok, nil
}

// ownIdleTimeout is the workspace's own idle timeout while its plan lets it
// have one (configurable); ok is false when it applies none.
func ownIdleTimeout(row store.AgentEnvironment, configurable bool) (time.Duration, bool) {
	if !configurable || row.IdleTimeoutMinutes == nil {
		return 0, false
	}
	return time.Duration(*row.IdleTimeoutMinutes) * time.Minute, true
}

// withIdleTimeout fills the view's idle-timeout fields: the timeout the
// sessions people start in the workspace close after, in whole minutes, and
// whether its admins may change it.
func (s *Service) withIdleTimeout(view Environment, row store.AgentEnvironment, configurable bool) Environment {
	timeout := DefaultIdleTimeout
	if s.Completer != nil {
		timeout = s.Completer.idleTTL()
	}
	if own, ok := ownIdleTimeout(row, configurable); ok {
		timeout = own
	}
	view.IdleTimeoutMinutes = int((timeout + time.Minute - 1) / time.Minute)
	view.IdleTimeoutConfigurable = configurable
	return view
}
