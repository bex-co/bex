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
	"errors"
	"fmt"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func actionByID(t *testing.T, acts []core.ActionDecision, id string) core.ActionDecision {
	t.Helper()
	for _, a := range acts {
		if a.Action == id {
			return a
		}
	}
	t.Fatalf("no %q action in %+v", id, acts)
	return core.ActionDecision{}
}

// TestActionCapabilities_NoRestart (ADR087, w6/m136): the Key Value projection
// offers suspend/resume, the w4/m594 rows and delete — no invented restart.
func TestActionCapabilities_NoRestart(t *testing.T) {
	kv := keyValueForProtection("red-cache", "cache", true)
	svc, _, _ := protectedKeyValueService(kv)

	acts, err := svc.ActionCapabilities(context.Background(), kv.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	var ids []string
	for _, a := range acts {
		ids = append(ids, a.Action)
	}
	want := []string{core.ActionSuspend, core.ActionResume, core.ActionViewConnectionInfo, core.ActionUpdateIPAllowList, core.ActionDelete}
	if !slices.Equal(ids, want) {
		t.Fatalf("actions = %v, want exactly %v", ids, want)
	}
}

// TestActionCapabilities_SuspendResumeAgreeWithTheVerb (w4/m578): for an Admin
// and a Developer, on a member of a protected and of an unprotected
// environment, the projection's suspend and resume decisions are the verbs'
// own answers. Before w4/m578 the projection decided both on can_operate alone,
// so a Developer on a protected store was offered a Suspend and a Resume the
// verb then refused with 403 (requireUnprotected asks for the Admin role
// there). An Admin keeps the confirmation hint on suspend, and the Developer's
// refusal is named so a client can say only an Admin may.
func TestActionCapabilities_SuspendResumeAgreeWithTheVerb(t *testing.T) {
	roles := []struct {
		name  string
		authz core.Checker
		admin bool
	}{{"admin", allowAllRelations{}, true}, {"developer", denyWorkspaceManage{}, false}}
	for _, role := range roles {
		for _, protected := range []bool{true, false} {
			for _, action := range []string{core.ActionSuspend, core.ActionResume} {
				t.Run(fmt.Sprintf("%s/%s/protected=%t", role.name, action, protected), func(t *testing.T) {
					obj := keyValueForProtection("red-cache", "cache", true)
					if !protected {
						obj.Labels[core.LabelEnvironment] = "env-staging" // a member, of an unprotected environment
					}
					obj.Spec.Suspended = action == core.ActionResume // the state the verb acts on
					svc, cl, prot := protectedKeyValueService(obj)
					svc.Authz = role.authz
					ctx := ctxAsCaller()

					acts, err := svc.ActionCapabilities(ctx, obj.Name)
					if err != nil {
						t.Fatalf("ActionCapabilities: %v", err)
					}
					if prot.calls != 1 {
						t.Errorf("projection read protection %d times, want once for both verbs", prot.calls)
					}
					got := actionByID(t, acts, action)
					verb := svc.Suspend
					if action == core.ActionResume {
						verb = svc.Resume
					}
					_, verbErr := verb(ctx, obj.Name)

					allowed := got.Outcome == core.DecisionAllowed
					if want := role.admin || !protected; allowed != want || (verbErr == nil) != want {
						t.Fatalf("projection %+v and verb error %v disagree, want allowed=%t for both", got, verbErr, want)
					}
					switch {
					case !allowed:
						if got.Reason != core.ReasonProtectedEnvironmentAdmin || got.Precondition != "" || !errors.Is(verbErr, core.ErrForbidden) {
							t.Errorf("refused %s = %+v (verb %v), want denied/%s with no precondition and ErrForbidden",
								action, got, verbErr, core.ReasonProtectedEnvironmentAdmin)
						}
						var after appv1alpha1.KeyValue
						if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), &after); err != nil {
							t.Fatalf("get: %v", err)
						}
						if after.Spec.Suspended != obj.Spec.Suspended {
							t.Error("a refused verb must not change the store")
						}
					case action == core.ActionSuspend && protected:
						if got.Precondition != core.PrecondProtectedConfirmation {
							t.Errorf("an Admin's protected suspend = %+v, want the confirmation hint", got)
						}
					default:
						if got.Precondition != "" {
							t.Errorf("%s = %+v, want ready", action, got)
						}
					}
				})
			}
		}
	}
}

// TestActionCapabilities_ViewerDeniedWithoutGuardReads: a viewer is denied
// both actions without preconditions, and without the projection reading the
// protection store or asking the admin check.
func TestActionCapabilities_ViewerDeniedWithoutGuardReads(t *testing.T) {
	svc, _, prot := protectedKeyValueService(keyValueForProtection("red-locked", "locked", true))
	svc.Base.Authz = viewOnly{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "viewer-1", Method: "session"})
	acts, err := svc.ActionCapabilities(ctx, "red-locked")
	if err != nil {
		t.Fatalf("viewer ActionCapabilities: %v", err)
	}
	for _, a := range acts {
		if a.Outcome != core.DecisionDenied || a.Reason != core.ReasonInsufficientPermission || a.Precondition != "" {
			t.Errorf("viewer %s = %+v, want denied/insufficient_permission with no precondition", a.Action, a)
		}
	}
	if prot.calls != 0 {
		t.Errorf("viewer projection read the protection store %d times, want none", prot.calls)
	}
}

// viewOnly grants exactly can_view — the viewer shape.
type viewOnly struct{}

func (viewOnly) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation == core.RelCanView, nil
}

// TestActionCapabilities_SuspendResumeReadTheResourceState is w4/132, the Key
// Value half: both decisions used to compute protection and billing only, so a
// store in `status suspended, suspended suspended` projected the same
// suspend:allowed / resume:allowed as one that was running.
func TestActionCapabilities_SuspendResumeReadTheResourceState(t *testing.T) {
	running := keyValueForProtection("red-scratch", "scratch", false)
	svc, _, _ := protectedKeyValueService(running)
	acts, err := svc.ActionCapabilities(context.Background(), running.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities (running): %v", err)
	}
	if s := actionByID(t, acts, core.ActionSuspend); s.Precondition != "" {
		t.Errorf("running suspend precondition = %q, want none", s.Precondition)
	}
	if r := actionByID(t, acts, core.ActionResume); r.Precondition != core.PrecondNotSuspended {
		t.Errorf("running resume = %+v, want the not_suspended precondition", r)
	}

	asleep := keyValueForProtection("red-asleep", "asleep", true)
	asleep.Spec.Suspended = true
	svc2, _, _ := protectedKeyValueService(asleep)
	acts2, err := svc2.ActionCapabilities(context.Background(), asleep.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities (suspended): %v", err)
	}
	// Protected AND suspended: state wins, so the projection does not ask for a
	// confirmation phrase to perform a call that would no-op.
	if s := actionByID(t, acts2, core.ActionSuspend); s.Precondition != core.PrecondSuspended {
		t.Errorf("suspended suspend = %+v, want the suspended precondition", s)
	}
	if r := actionByID(t, acts2, core.ActionResume); r.Precondition != "" {
		t.Errorf("suspended resume precondition = %q, want none", r.Precondition)
	}
}
