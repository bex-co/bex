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

package postgres

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ActionCapabilities projects the Postgres lifecycle verbs' decisions for ONE
// database in the acting workspace (ADR087, w6/m136): restart, suspend, and
// resume are all can_operate (the roleladder pins); suspend additionally
// carries the protected-environment confirmation precondition and resume the
// dunning gate — computed by the same core predicates setSuspended enforces.
// Read-only; the verbs re-run their own guards at dispatch. Failover and
// delete are deliberately absent: neither is a mobile-scope action (ADR087),
// and an absent action reads as "not offered here", never as permitted.
func (s *Service) ActionCapabilities(ctx context.Context, name string) ([]core.ActionDecision, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, d.Labels); err != nil {
		return nil, err
	}
	// Bound to the acting workspace above — probe it directly, and compute
	// preconditions only for an allowed decision (a viewer pays no protection
	// or billing reads, and learns nothing from them).
	operate := s.CanDecisionOn(ctx, core.RelCanOperate, core.WorkspaceObject(s.WorkspaceOrDefault(ctx)))
	suspendPre, resumePre := "", ""
	if operate.Allowed() {
		suspendPre = core.EnvironmentProtectionPrecondition(ctx, s.Protection, d.Labels[core.LabelEnvironment])
		resumePre = s.BillingPreconditionFor(ctx, d.Labels[core.LabelTenant])
	}
	return []core.ActionDecision{
		core.DecideAction(core.ActionRestart, operate, ""),
		core.DecideAction(core.ActionSuspend, operate, suspendPre),
		core.DecideAction(core.ActionResume, operate, resumePre),
	}, nil
}
