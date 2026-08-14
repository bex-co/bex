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

import "testing"

// TestBillingExceptionTransition pins the pure exception-transition rules that
// SetBillingException persists: the toggle folds into the pre-toggle tenant
// flags, exclusion outranks comp, active/enforcement-shaped states route
// through the recovery worker toward the desired status, the reason is blanked
// only for healthy, and the version advances only on a real change. No DB.
func TestBillingExceptionTransition(t *testing.T) {
	cases := []struct {
		name      string
		state     BillingLifecycle
		exception string
		enabled   bool
		excluded  bool
		comped    bool
		active    bool
		reason    string
		want      billingExceptionOutcome
	}{
		{
			name:      "enable exclusion on healthy tenant",
			state:     BillingLifecycle{Status: BillingHealthy, TransitionVersion: 3},
			exception: BillingExcluded,
			enabled:   true,
			reason:    "ops request",
			want: billingExceptionOutcome{
				next: BillingExcluded, target: BillingHealthy,
				reason: "operator_excluded: ops request", version: 4, stateChanged: true,
			},
		},
		{
			name:      "enable comp on healthy tenant",
			state:     BillingLifecycle{Status: BillingHealthy, TransitionVersion: 7},
			exception: BillingComped,
			enabled:   true,
			reason:    "design partner",
			want: billingExceptionOutcome{
				next: BillingComped, target: BillingHealthy,
				reason: "operator_comped: design partner", version: 8, stateChanged: true,
			},
		},
		{
			name:      "enable exclusion with active enforcement routes through recovery",
			state:     BillingLifecycle{Status: BillingEnforced, Reason: "payment_failed", TransitionVersion: 5},
			exception: BillingExcluded,
			enabled:   true,
			active:    true,
			reason:    "vip",
			want: billingExceptionOutcome{
				next: BillingRecovering, target: BillingExcluded,
				reason: "operator_excluded: vip", version: 6, stateChanged: true,
			},
		},
		{
			name:      "enforcement-shaped status routes through recovery without active markers",
			state:     BillingLifecycle{Status: BillingEnforcing, Reason: "payment_failed", TransitionVersion: 2},
			exception: BillingExcluded,
			enabled:   true,
			reason:    "r",
			want: billingExceptionOutcome{
				next: BillingRecovering, target: BillingExcluded,
				reason: "operator_excluded: r", version: 3, stateChanged: true,
			},
		},
		{
			name:      "remove exclusion back to healthy blanks the reason",
			state:     BillingLifecycle{Status: BillingExcluded, Reason: "operator_excluded: was vip", TransitionVersion: 9},
			exception: BillingExcluded,
			enabled:   false,
			excluded:  true,
			reason:    "no longer vip",
			want: billingExceptionOutcome{
				next: BillingHealthy, target: BillingHealthy,
				reason: "", version: 10, stateChanged: true,
			},
		},
		{
			name:      "remove exclusion falls back to a still-set comp flag",
			state:     BillingLifecycle{Status: BillingExcluded, Reason: "operator_excluded: x", TransitionVersion: 1},
			exception: BillingExcluded,
			enabled:   false,
			excluded:  true,
			comped:    true,
			reason:    "r",
			want: billingExceptionOutcome{
				next: BillingComped, target: BillingHealthy,
				reason: "operator_excluded_removed: r", version: 2, stateChanged: true,
			},
		},
		{
			name:      "idempotent re-enable keeps version and reports no change",
			state:     BillingLifecycle{Status: BillingExcluded, Reason: "operator_excluded: r", TransitionVersion: 4},
			exception: BillingExcluded,
			enabled:   true,
			excluded:  true,
			reason:    "r",
			want: billingExceptionOutcome{
				next: BillingExcluded, target: BillingHealthy,
				reason: "operator_excluded: r", version: 4, stateChanged: false,
			},
		},
		{
			name:      "remove comp during recovery stays recovering toward healthy",
			state:     BillingLifecycle{Status: BillingRecovering, Reason: "payment_recovered", TransitionVersion: 6},
			exception: BillingComped,
			enabled:   false,
			comped:    true,
			reason:    "r",
			want: billingExceptionOutcome{
				next: BillingRecovering, target: BillingHealthy,
				reason: "operator_comped_removed: r", version: 7, stateChanged: true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := billingExceptionTransition(tc.state, tc.exception, tc.enabled, tc.excluded, tc.comped, tc.active, tc.reason)
			if got != tc.want {
				t.Fatalf("billingExceptionTransition() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
