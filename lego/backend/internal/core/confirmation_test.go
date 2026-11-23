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

// fakeProtection answers every environment's protection with one status.
type fakeProtection struct{ protected bool }

func (f fakeProtection) GetEnvironmentProtectedStatus(context.Context, string) (string, error) {
	if f.protected {
		return ProtectedStatusProtected, nil
	}
	return ProtectedStatusUnprotected, nil
}

// TestRestrictedDecision pins the projection of RequireEnvironmentAdmin
// (w4/m578): a restricted verb on a protected member reads as the fresh admin
// check's answer, with a role refusal named so a client can say only an Admin
// may, and every other case reads as the verb's own permission. A caller the
// verb refuses anyway pays neither read, and an unanswerable protection read
// is never a role refusal.
func TestRestrictedDecision(t *testing.T) {
	allowed := Decision{Outcome: DecisionAllowed}
	denied := Decision{Outcome: DecisionDenied, Reason: ReasonInsufficientPermission}
	cases := []struct {
		name          string
		verb          string
		operate       Decision
		protected     bool
		protectedErr  error
		adminErr      error
		want          Decision
		wantProtected bool
		wantReads     int // protection reads + admin checks
	}{
		{name: "a caller who may not operate pays no read", verb: "suspend", operate: denied, protected: true,
			want: denied},
		{name: "a verb Render leaves open is the operate answer", verb: "restart", operate: allowed, protected: true, adminErr: ErrForbidden,
			want: allowed},
		{name: "an unprotected member is the operate answer, with no admin check", verb: "resume", operate: allowed, adminErr: ErrForbidden,
			want: allowed, wantReads: 1},
		{name: "an Admin on a protected member is allowed", verb: "suspend", operate: allowed, protected: true,
			want: allowed, wantProtected: true, wantReads: 2},
		{name: "a Developer on a protected member is refused as admin-only", verb: "resume", operate: allowed, protected: true, adminErr: ErrForbidden,
			want: Decision{Outcome: DecisionDenied, Reason: ReasonProtectedEnvironmentAdmin}, wantProtected: true, wantReads: 2},
		{name: "a token missing the scope keeps its reason", verb: "suspend", operate: allowed, protected: true, adminErr: NewInsufficientScopeError(ScopeWrite),
			want: Decision{Outcome: DecisionDenied, Reason: ReasonMissingOAuthScope}, wantProtected: true, wantReads: 2},
		{name: "a coded refusal is not reported as admin-only", verb: "suspend", operate: allowed, protected: true,
			adminErr: NewForbiddenError("TWO_FACTOR_REQUIRED", "enable two-factor authentication", nil),
			want:     Decision{Outcome: DecisionDenied, Reason: ReasonInsufficientPermission}, wantProtected: true, wantReads: 2},
		{name: "an authorizer outage is unavailable", verb: "suspend", operate: allowed, protected: true, adminErr: fmt.Errorf("%w: down", ErrAuthzUnavailable),
			want: Decision{Outcome: DecisionUnavailable, Reason: ReasonAuthzUnavailable}, wantProtected: true, wantReads: 2},
		{name: "a failed protection read is unavailable, never a role refusal", verb: "suspend", operate: allowed, protectedErr: errors.New("db down"), adminErr: ErrForbidden,
			want: Decision{Outcome: DecisionUnavailable, Reason: ReasonAuthzUnavailable}, wantReads: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			got, isProtected := RestrictedDecision(tc.verb, tc.operate,
				func() (bool, error) { reads++; return tc.protected, tc.protectedErr },
				func() error { reads++; return tc.adminErr })
			if got != tc.want || isProtected != tc.wantProtected {
				t.Fatalf("RestrictedDecision = %+v protected=%t, want %+v protected=%t", got, isProtected, tc.want, tc.wantProtected)
			}
			if reads != tc.wantReads {
				t.Fatalf("reads = %d, want %d", reads, tc.wantReads)
			}
		})
	}
}

// TestRestrictedDecisionAgreesWithRequireEnvironmentAdmin: for every verb and
// every protection and admin answer, the projection is allowed exactly when
// the guard passes, so a client gating on it never offers a verb the guard
// refuses or hides one it admits; and the projection reads admin-only exactly
// when the guard answers the coded refusal (w4/728), so a raced click names
// the reason the disabled control showed.
func TestRestrictedDecisionAgreesWithRequireEnvironmentAdmin(t *testing.T) {
	verbs := []string{"create", "move", "delete", "suspend", "resume", "take offline", "shell",
		VerbViewCredentials, VerbChangeAccessControl, VerbEnvironmentVariables, "restart", "deploy"}
	for _, verb := range verbs {
		for _, protected := range []bool{true, false} {
			for _, adminErr := range []error{nil, ErrForbidden} {
				store := fakeProtection{protected: protected}
				guard := RequireEnvironmentAdmin(t.Context(), store, "env-1", verb, func() error { return adminErr })
				d, _ := RestrictedDecision(verb, Decision{Outcome: DecisionAllowed},
					func() (bool, error) { return EnvironmentProtected(t.Context(), store, "env-1") },
					func() error { return adminErr })
				if d.Allowed() != (guard == nil) {
					t.Errorf("%s protected=%t admin=%v: projection %+v, guard %v", verb, protected, adminErr, d, guard)
				}
				var coded *CodedError
				if adminOnly := d.Reason == ReasonProtectedEnvironmentAdmin; adminOnly != (errors.As(guard, &coded) && coded.Code == ProtectedPlacementCode) {
					t.Errorf("%s protected=%t admin=%v: projection %+v, guard %v", verb, protected, adminErr, d, guard)
				}
			}
		}
	}
}

// TestRequireEnvironmentAdminCodesTheRoleRefusal (w4/728): on a protected
// member every restricted verb answers a non-Admin with the coded 403 naming
// the environment by its public id and, in words, the action; a refusal coded
// with its own cause and an outage keep their own answer, so neither reads as
// a role refusal.
func TestRequireEnvironmentAdminCodesTheRoleRefusal(t *testing.T) {
	store := fakeProtection{protected: true}
	for verb, action := range map[string]string{
		"delete":                 "delete resources",
		"suspend":                "suspend resources",
		"resume":                 "resume resources",
		"take offline":           "turn on maintenance mode",
		"shell":                  "open a shell",
		VerbViewCredentials:      VerbViewCredentials,
		VerbChangeAccessControl:  VerbChangeAccessControl,
		VerbEnvironmentVariables: VerbEnvironmentVariables,
	} {
		err := RequireEnvironmentAdmin(t.Context(), store, "env-d00000000000000000a0", verb, func() error { return fmt.Errorf("check: %w", ErrForbidden) })
		var coded *CodedError
		if !errors.As(err, &coded) || coded.Code != ProtectedPlacementCode || !errors.Is(err, ErrForbidden) ||
			coded.Params["environmentId"] != "evm-d00000000000000000a0" {
			t.Errorf("%s: %#v, want the coded refusal naming evm-d00000000000000000a0", verb, err)
			continue
		}
		if want := "only workspace Admins can " + action + " in a protected environment"; coded.Error() != want {
			t.Errorf("%s: message %q, want %q", verb, coded.Error(), want)
		}
	}
	for _, own := range []error{
		NewForbiddenError("TWO_FACTOR_REQUIRED", "enable two-factor authentication", nil),
		NewInsufficientScopeError(ScopeWrite),
		fmt.Errorf("%w: down", ErrAuthzUnavailable),
	} {
		if err := RequireEnvironmentAdmin(t.Context(), store, "env-1", "suspend", func() error { return own }); err != own {
			t.Errorf("RequireEnvironmentAdmin with admin %v = %v, want it unchanged", own, err)
		}
	}
}
