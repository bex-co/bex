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

package keyvalue

import (
	"context"
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ActionCapabilities projects the Key Value lifecycle verbs' decisions for
// ONE store in the acting workspace (ADR087, w6/m136): suspend and resume,
// both can_operate and, on a protected member, the Admin check setSuspended
// adds there (core.RestrictedDecision, w4/m578), with suspend carrying the
// protected-environment confirmation hint for that Admin and resume the
// dunning gate — the same core predicates setSuspended enforces. There is deliberately NO restart row: the
// verb does not exist for Key Value, and the projection must not invent one
// (an absent action means "does not exist for this type").
func (s *Service) ActionCapabilities(ctx context.Context, name string) ([]core.ActionDecision, error) {
	kv, err := s.fetchLiveKeyValue(ctx, core.RelCanView, name)
	if err != nil {
		return nil, err
	}
	if err := s.RequireResourceInActingWorkspace(ctx, kv.Labels); err != nil {
		return nil, err
	}
	// Bound to the acting workspace above — probe it directly, and compute
	// preconditions only for an allowed decision (a viewer pays no protection
	// or billing reads, and learns nothing from them).
	operate := s.CanDecisionOn(ctx, core.RelCanOperate, core.WorkspaceObject(s.WorkspaceOrDefault(ctx)))
	// Suspend and resume are restricted verbs: on a protected member they
	// read as requireUnprotected's own fresh workspace-manage check, one
	// protection read and one check shared by both (w4/m578).
	protected := sync.OnceValues(func() (bool, error) {
		return core.EnvironmentProtected(ctx, s.Protection, kv.Labels[core.LabelEnvironment])
	})
	admin := sync.OnceValue(func() error { return s.AuthorizeKeyValueFresh(ctx, core.RelCanManage, kv) })
	suspend, isProtected := core.RestrictedDecision("suspend", operate, protected, admin)
	resume, _ := core.RestrictedDecision("resume", operate, protected, admin)
	suspendPre, resumePre := core.LifecyclePreconditions(kv.Spec.Suspended, isProtected, resume, func() string {
		return s.BillingPreconditionFor(ctx, kv.Labels[core.LabelTenant])
	})
	return []core.ActionDecision{
		core.DecideAction(core.ActionSuspend, suspend, suspendPre),
		core.DecideAction(core.ActionResume, resume, resumePre),
	}, nil
}
