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

package apps

// blueprint_pricing_agreement_test.go pins the billing-gate/estimate agreement
// (ADR030 §7–§8, ADR046): on a valid stack, the payment-method gate trips
// exactly when the estimate shows something paid — a priced line or a
// variable entry. A priced line means paid provisioned capacity, so the card
// gate must have tripped; a tripped gate means paid intent, so the estimate
// must show it (a cron job as a variable cost, never as a monthly line).
// Invalid stacks (a free worker, an unknown plan) fail validation before
// either signal matters, so only valid stacks appear here.

import (
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestStackPaidPlanAgreesWithEstimate walks one valid stack per row: the
// gate's paid intent and the estimate's paid content must agree.
func TestStackPaidPlanAgreesWithEstimate(t *testing.T) {
	scaling := &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3}
	for _, tc := range []struct {
		name     string
		stack    parsedStack
		paidGate bool
		lines    int
		variable int
	}{
		{
			name:     "plan-less web service provisions paid",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "web"}}}},
			paidGate: true,
			lines:    1,
		},
		{
			name:     "explicit free web service",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Plan: "free"}}}},
			paidGate: false,
		},
		{
			name:     "plan-less worker provisions at the paid default",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "worker", Type: appv1alpha1.TypeBackgroundWorker}}}},
			paidGate: true,
			lines:    1,
		},
		{
			name:     "paid private service",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "api", Type: appv1alpha1.TypePrivateService, Plan: "standard"}}}},
			paidGate: true,
			lines:    1,
		},
		{
			name:     "paid cron is variable-only, still paid intent",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "nightly", Type: appv1alpha1.TypeCronJob, Plan: "starter"}}}},
			paidGate: true,
			variable: 1,
		},
		{
			name:     "static site runs no instance",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "site", Type: appv1alpha1.TypeStaticSite}}}},
			paidGate: false,
		},
		{
			name:     "autoscaled service prices one base instance",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "web", Plan: "starter", Autoscaling: scaling}}}},
			paidGate: true,
			lines:    1,
			variable: 1,
		},
		{
			name:     "multi-instance service prices one base instance",
			stack:    parsedStack{services: []parsedService{{req: CreateRequest{Name: "web", Plan: "starter", Replicas: 3}}}},
			paidGate: true,
			lines:    1,
			variable: 1,
		},
		{
			name:     "paid database",
			stack:    parsedStack{databases: []parsedDatabase{{name: "db", spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb"}}}},
			paidGate: true,
			lines:    1,
		},
		{
			name:     "database with standby and replica",
			stack:    parsedStack{databases: []parsedDatabase{{name: "db", spec: appv1alpha1.DatabaseSpec{Plan: "1c-2g", HighAvailability: true, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "r1"}}}}}},
			paidGate: true,
			lines:    3,
		},
		{
			name:     "free database",
			stack:    parsedStack{databases: []parsedDatabase{{name: "db", spec: appv1alpha1.DatabaseSpec{Plan: "free"}}}},
			paidGate: false,
		},
		{
			name:     "plan-less key value defaults to free",
			stack:    parsedStack{keyValues: []parsedKeyValue{{name: "cache"}}},
			paidGate: false,
		},
		{
			name:     "paid key value",
			stack:    parsedStack{keyValues: []parsedKeyValue{{name: "cache", spec: appv1alpha1.KeyValueSpec{Plan: "starter"}}}},
			paidGate: true,
			lines:    1,
		},
		{
			name:     "empty stack",
			stack:    parsedStack{},
			paidGate: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stackHasPaidPlan(tc.stack); got != tc.paidGate {
				t.Errorf("stackHasPaidPlan = %v, want %v", got, tc.paidGate)
			}
			est := blueprintEstimatedPricing(tc.stack)
			if est == nil {
				t.Fatal("blueprintEstimatedPricing returned nil")
			}
			if len(est.Lines) != tc.lines {
				t.Errorf("estimate lines = %+v, want %d", est.Lines, tc.lines)
			}
			if len(est.Variable) != tc.variable {
				t.Errorf("estimate variable = %+v, want %d entries", est.Variable, tc.variable)
			}
			// The agreement itself: paid intent ⇔ paid content.
			paidContent := len(est.Lines) > 0 || len(est.Variable) > 0
			if paidContent != tc.paidGate {
				t.Errorf("gate = %v but estimate shows lines=%d variable=%d; intent and content disagree",
					tc.paidGate, len(est.Lines), len(est.Variable))
			}
		})
	}
}
