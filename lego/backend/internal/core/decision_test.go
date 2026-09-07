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

package core

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// decisionChecker is a stub Checker with a fixed answer per call.
type decisionChecker struct {
	allowed bool
	err     error
}

func (c decisionChecker) Check(context.Context, string, string, string) (bool, error) {
	return c.allowed, c.err
}

// TestClassifyDecision pins the error→tri-state mapping (ADR087, w6/m136),
// including the precedence trap: the insufficient-scope CodedError WRAPS
// ErrForbidden, so a naive Is(ErrForbidden)-first classifier would report a
// scope problem as a role refusal — exactly the wrong message to show a
// narrowed first-party token.
func TestClassifyDecision(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantOutcome string
		wantReason  string
	}{
		{"nil is allowed", nil, DecisionAllowed, ""},
		{"insufficient scope beats its ErrForbidden sentinel",
			NewInsufficientScopeError(ScopeWrite), DecisionDenied, ReasonMissingOAuthScope},
		{"wrapped insufficient scope still classifies",
			fmt.Errorf("gate: %w", NewInsufficientScopeError("")), DecisionDenied, ReasonMissingOAuthScope},
		{"forbidden is an affirmative permission refusal",
			ErrForbidden, DecisionDenied, ReasonInsufficientPermission},
		{"checker outage is unavailable, not a verdict",
			fmt.Errorf("%w: dial tcp: connection refused", ErrAuthzUnavailable), DecisionUnavailable, ReasonAuthzUnavailable},
		{"an unrecognized error fails closed as unavailable",
			errors.New("unclassified"), DecisionUnavailable, ReasonAuthzUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ClassifyDecision(tc.err)
			if d.Outcome != tc.wantOutcome || d.Reason != tc.wantReason {
				t.Fatalf("ClassifyDecision(%v) = {%s %s}, want {%s %s}",
					tc.err, d.Outcome, d.Reason, tc.wantOutcome, tc.wantReason)
			}
			if d.Allowed() != (tc.wantOutcome == DecisionAllowed) {
				t.Fatalf("Allowed() disagrees with outcome %q", d.Outcome)
			}
		})
	}
}

// TestCanDecisionThroughSeam runs the probe end to end at the Base seam: the
// OAuth capability half (a narrowed token) and the OpenFGA half (deny, error)
// must classify distinctly, and Can must remain exactly the collapsed form of
// CanDecision so the legacy boolean surfaces cannot drift from the tri-state.
func TestCanDecisionThroughSeam(t *testing.T) {
	session := WithIdentity(context.Background(), Identity{Subject: "u1", Method: "session"})

	t.Run("allowed", func(t *testing.T) {
		b := &Base{Authz: decisionChecker{allowed: true}}
		if d := b.CanDecision(session, RelCanOperate); !d.Allowed() || d.Reason != "" {
			t.Fatalf("allowed probe = %+v", d)
		}
	})

	t.Run("FGA deny is denied/insufficient_permission", func(t *testing.T) {
		b := &Base{Authz: decisionChecker{allowed: false}}
		d := b.CanDecision(session, RelCanOperate)
		if d.Outcome != DecisionDenied || d.Reason != ReasonInsufficientPermission {
			t.Fatalf("deny probe = %+v", d)
		}
	})

	t.Run("checker outage is unavailable, never a role verdict", func(t *testing.T) {
		b := &Base{Authz: decisionChecker{err: errors.New("fga down")}}
		d := b.CanDecision(session, RelCanOperate)
		if d.Outcome != DecisionUnavailable || d.Reason != ReasonAuthzUnavailable {
			t.Fatalf("outage probe = %+v", d)
		}
		if d.Allowed() {
			t.Fatal("outage must not read as allowed")
		}
	})

	t.Run("narrowed OAuth token is denied/missing_oauth_scope before FGA", func(t *testing.T) {
		// A read-only human OAuth delegation probing a write relation: the scope
		// gate must answer (and classify) without consulting the checker — the
		// checker here would ALLOW, proving the scope refusal wins.
		readOnly := WithIdentity(context.Background(), Identity{
			Subject: "u1", Method: "oauth2", Human: true, CanonicalScopes: ScopeRead,
		})
		b := &Base{Authz: decisionChecker{allowed: true}}
		d := b.CanDecision(readOnly, RelCanOperate)
		if d.Outcome != DecisionDenied || d.Reason != ReasonMissingOAuthScope {
			t.Fatalf("narrowed-token probe = %+v", d)
		}
	})

	t.Run("Can is exactly the collapsed CanDecision", func(t *testing.T) {
		for _, b := range []*Base{
			{Authz: decisionChecker{allowed: true}},
			{Authz: decisionChecker{allowed: false}},
			{Authz: decisionChecker{err: errors.New("down")}},
		} {
			if got, want := b.Can(session, RelCanOperate), b.CanDecision(session, RelCanOperate).Allowed(); got != want {
				t.Fatalf("Can=%v disagrees with CanDecision.Allowed=%v", got, want)
			}
		}
	})
}

// TestRequireResourceInActingWorkspace pins the explicit-context binding
// (ADR087, w6/m136/t003): a capability projection answers about "this resource
// in the ACTING workspace", so a target owned elsewhere reads as absent —
// indistinguishable from nonexistent — even though the verbs' own
// cross-workspace fallback might have honored it.
func TestRequireResourceInActingWorkspace(t *testing.T) {
	member := WithIdentity(context.Background(), Identity{Subject: "u1", Method: "session"})

	t.Run("own-workspace resource binds", func(t *testing.T) {
		b := &Base{Workspace: fakeWorkspace{"u1": "tea-a"}}
		if err := b.RequireResourceInActingWorkspace(member, map[string]string{LabelTenant: "tea-a"}); err != nil {
			t.Fatalf("own resource = %v, want nil", err)
		}
	})

	t.Run("foreign resource reads as absent", func(t *testing.T) {
		b := &Base{Workspace: fakeWorkspace{"u1": "tea-a"}}
		if err := b.RequireResourceInActingWorkspace(member, map[string]string{LabelTenant: "tea-b"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign resource = %v, want ErrNotFound", err)
		}
	})

	t.Run("store off: unlabeled binds to default, labeled does not", func(t *testing.T) {
		b := &Base{}
		if err := b.RequireResourceInActingWorkspace(context.Background(), nil); err != nil {
			t.Fatalf("unlabeled in store-off mode = %v, want nil", err)
		}
		if err := b.RequireResourceInActingWorkspace(context.Background(), map[string]string{LabelTenant: "tea-a"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("labeled resource against the default workspace = %v, want ErrNotFound", err)
		}
	})
}
