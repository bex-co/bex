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
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ActionCapabilities projects the Postgres lifecycle verbs' decisions for ONE
// database in the acting workspace (ADR087, w6/m136): restart, suspend, and
// resume are all can_operate (the roleladder pins), and on a protected member
// suspend and resume read as the Admin check setSuspended adds there
// (core.RestrictedDecision, w4/m578); suspend carries the protected-environment
// confirmation hint for that Admin, resume carries the dunning gate, and
// restart carries the suspended / dunning preconditions — computed by the same
// core predicates setSuspended and Restart enforce. view_connection_info
// (can_view_sensitive) and update_ip_allow_list (can_create, the PATCH's own
// permission since w9/m619) read the same way as the verbs a protected
// environment keeps to Admins (w4/m594), so a client can say so before the
// click. Delete reads the same way with DeletePostgres's own permission
// (can_create), so a Developer on a protected database sees Delete disabled
// before typing its phrase (w4/724).
// Read-only; the verbs re-run their own guards at dispatch. Failover is
// deliberately absent: it is not a mobile-scope action (ADR087), and an absent
// action reads as "not offered here", never as permitted.
func (s *Service) ActionCapabilities(ctx context.Context, name string) ([]core.ActionDecision, error) {
	d, err := s.fetchLiveDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, d.Labels); err != nil {
		return nil, err
	}
	// Bound to the acting workspace above — probe it directly, and compute
	// preconditions only for an allowed decision (a viewer pays no protection
	// or billing reads, and learns nothing from them).
	workspace := core.WorkspaceObject(s.WorkspaceOrDefault(ctx))
	operate := s.CanDecisionOn(ctx, core.RelCanOperate, workspace)
	// Every row but restart is a restricted verb: on a protected member each
	// reads as the verb's own fresh workspace-manage check, one protection
	// read and one check shared by all (w4/m578, w4/m594).
	protected := sync.OnceValues(func() (bool, error) {
		return core.EnvironmentProtected(ctx, s.Protection, d.Labels[core.LabelEnvironment])
	})
	admin := sync.OnceValue(func() error { return s.AuthorizeDatabaseFresh(ctx, core.RelCanManage, d) })
	suspend, isProtected := core.RestrictedDecision("suspend", operate, protected, admin)
	resume, _ := core.RestrictedDecision("resume", operate, protected, admin)
	billing := func() string { return s.BillingPreconditionFor(ctx, d.Labels[core.LabelTenant]) }
	restartPre := ""
	if operate.Allowed() {
		if d.Spec.Suspended { // Restart's guard order: state, then billing
			restartPre = core.PrecondSuspended
		} else {
			restartPre = billing()
		}
	}
	suspendPre, resumePre := core.LifecyclePreconditions(d.Spec.Suspended, isProtected, resume, billing)
	credentials, _ := core.RestrictedDecision(core.VerbViewCredentials, s.CanDecisionOn(ctx, core.RelCanViewSensitive, workspace), protected, admin)
	create := s.CanDecisionOn(ctx, core.RelCanCreate, workspace)
	accessControl, _ := core.RestrictedDecision(core.VerbChangeAccessControl, create, protected, admin)
	remove, _ := core.RestrictedDecision("delete", create, protected, admin)
	return []core.ActionDecision{
		core.DecideAction(core.ActionRestart, operate, restartPre),
		core.DecideAction(core.ActionSuspend, suspend, suspendPre),
		core.DecideAction(core.ActionResume, resume, resumePre),
		core.DecideAction(core.ActionViewConnectionInfo, credentials, ""),
		core.DecideAction(core.ActionUpdateIPAllowList, accessControl, ""),
		core.DecideAction(core.ActionDelete, remove, ""),
	}, nil
}
