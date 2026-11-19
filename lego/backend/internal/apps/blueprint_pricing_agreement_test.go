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
	"fmt"
	"math/rand"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
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
			// The parse turns a plan-less entry into the starter row below
			// (w33/m436), so a parsed stack's free store is an explicit one.
			name:     "free key value",
			stack:    parsedStack{keyValues: []parsedKeyValue{{name: "cache", spec: appv1alpha1.KeyValueSpec{Plan: "free"}}}},
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

// TestStackPaidPlanAgreesWithEstimateAcrossValidStacks is the generative
// companion to TestStackPaidPlanAgreesWithEstimate: instead of one row per
// shape, it walks deterministic pseudo-random cross-products of valid
// services, databases and key values and asserts the gate/estimate agreement
// on every stack. The generator only produces stacks validation would accept,
// so a disagreement is a real billing bug rather than an invalid-input
// artifact. Each rule is grounded in the validator it mirrors:
//
//   - a static site names no plan (the compiler schema rejects the key;
//     TestStaticSitePlanIsTheNamedOffender pins it)
//   - worker, private service and cron refuse the free plan
//     (normalizeTierForType's paid-only rule)
//   - unknown or paused plans are refused for a new service
//     (normalizeTierOrPlan, errPausedTier), so plans come from the catalog
//   - a free plan caps at one instance (checkInstanceCap) and refuses
//     autoscaling (autoscalingSpec is paid-only) and disks (validateDisk)
//   - a disk needs a web, private or worker service, a paid plan, at most one
//     replica and no autoscaling (validateDisk)
//
// A database or key value plan is always a catalog id (the parse writes the
// create default into a plan-less entry); HA and read replicas stay on the
// 1c-2g plan, the conservative subset CheckHighAvailabilityPlan admits.
func TestStackPaidPlanAgreesWithEstimateAcrossValidStacks(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var sawPaid, sawUnpaid, sawVariable bool
	for i := 0; i < 200; i++ {
		st, want := randomPricedStack(rng, i)
		est := blueprintEstimatedPricing(st)
		if est == nil {
			t.Fatalf("stack %d: blueprintEstimatedPricing returned nil", i)
		}
		if got := stackHasPaidPlan(st); got != want.paidGate {
			t.Errorf("stack %d: stackHasPaidPlan = %v, want %v", i, got, want.paidGate)
		}
		paidContent := len(est.Lines) > 0 || len(est.Variable) > 0
		if paidContent != want.paidGate {
			t.Errorf("stack %d: gate = %v but estimate shows lines=%d variable=%d",
				i, want.paidGate, len(est.Lines), len(est.Variable))
		}
		sawPaid = sawPaid || want.paidGate
		sawUnpaid = sawUnpaid || !want.paidGate
		sawVariable = sawVariable || len(est.Variable) > 0
		// Every priced line names a paid plan: the estimator's $0 filter and
		// the gate's PaidPlan must read the same tiers as paid.
		for _, line := range est.Lines {
			if !core.PaidPlan(line.Tier) {
				t.Errorf("stack %d: line %q priced at non-paid tier %q", i, line.Name, line.Tier)
			}
		}
		// Static sites run no instance, so they never surface anywhere.
		for _, name := range want.staticNames {
			for _, line := range est.Lines {
				if line.Name == name {
					t.Errorf("stack %d: static site %q priced in line %q", i, name, line.Name)
				}
			}
			for _, v := range est.Variable {
				if v.Name == name {
					t.Errorf("stack %d: static site %q listed as variable %q", i, name, v.Reason)
				}
			}
		}
		// Every variable entry is explained by its resource's declaration.
		for _, v := range est.Variable {
			decl, ok := want.byName[v.Name]
			if !ok {
				t.Errorf("stack %d: variable entry %q names no declared service", i, v.Name)
				continue
			}
			switch v.Reason {
			case pricing.VariableCron:
				if decl.svcType != appv1alpha1.TypeCronJob {
					t.Errorf("stack %d: %q listed as cron but is %q", i, v.Name, decl.svcType)
				}
			case pricing.VariableAutoscaling:
				if !decl.autoscaling {
					t.Errorf("stack %d: %q listed as autoscaled without autoscaling", i, v.Name)
				}
			case pricing.VariableMultiInstance:
				if decl.replicas <= 1 {
					t.Errorf("stack %d: %q listed as multi-instance with %d replicas", i, v.Name, decl.replicas)
				}
			default:
				t.Errorf("stack %d: unknown variable reason %q", i, v.Reason)
			}
		}
	}
	// The generator must exercise both sides of the agreement, or it would
	// pass vacuously.
	if !sawPaid || !sawUnpaid || !sawVariable {
		t.Errorf("generator coverage: paid=%v unpaid=%v variable=%v; want all true", sawPaid, sawUnpaid, sawVariable)
	}
}

// pricedServiceDecl is what a generated parsedService declared, the fields the
// estimate can observe.
type pricedServiceDecl struct {
	svcType     string
	replicas    int32
	autoscaling bool
}

type pricingAgreementOracle struct {
	paidGate    bool
	staticNames []string
	byName      map[string]pricedServiceDecl
}

// randomPricedStack builds a validation-acceptable stack plus its oracle: the
// gate verdict, the static-site names, and every service's estimate-visible
// declaration by name.
func randomPricedStack(rng *rand.Rand, seed int) (parsedStack, pricingAgreementOracle) {
	want := pricingAgreementOracle{byName: map[string]pricedServiceDecl{}}
	var st parsedStack
	svcTypes := []string{"", appv1alpha1.TypeWebService, appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypePrivateService, appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite}
	computePlans := []string{"", "free", "starter", "standard"}
	for j := 0; j < rng.Intn(4); j++ {
		svcType := svcTypes[rng.Intn(len(svcTypes))]
		plan := computePlans[rng.Intn(len(computePlans))]
		if svcType == appv1alpha1.TypeStaticSite {
			plan = ""
		} else if plan == "free" && (svcType == appv1alpha1.TypeBackgroundWorker ||
			svcType == appv1alpha1.TypePrivateService || svcType == appv1alpha1.TypeCronJob) {
			plan = "starter"
		}
		paid := plan != "free" && svcType != appv1alpha1.TypeStaticSite
		replicas := int32(rng.Intn(4))
		if !paid && replicas > 1 {
			replicas = 1
		}
		autoscaling := paid && rng.Intn(2) == 0 &&
			(svcType == "" || svcType == appv1alpha1.TypeWebService ||
				svcType == appv1alpha1.TypePrivateService || svcType == appv1alpha1.TypeBackgroundWorker)
		disk := paid && !autoscaling && replicas <= 1 && rng.Intn(2) == 0 &&
			(svcType == "" || svcType == appv1alpha1.TypeWebService ||
				svcType == appv1alpha1.TypePrivateService || svcType == appv1alpha1.TypeBackgroundWorker)
		req := CreateRequest{Name: fmt.Sprintf("svc-%d-%d", seed, j), Type: svcType, Plan: plan, Replicas: replicas}
		if autoscaling {
			req.Autoscaling = &SetAutoscalingRequest{MinInstances: 1, MaxInstances: 3}
		}
		if disk {
			size := int32(10 * rng.Intn(3))
			req.Disk = &ServiceDiskView{Name: "data", MountPath: "/data", SizeGB: size}
		}
		st.services = append(st.services, parsedService{req: req})
		want.byName[req.Name] = pricedServiceDecl{svcType: svcType, replicas: replicas, autoscaling: autoscaling}
		if paid {
			want.paidGate = true
		}
		if svcType == appv1alpha1.TypeStaticSite {
			want.staticNames = append(want.staticNames, req.Name)
		}
	}
	dbPlans := []string{"free", "basic-256mb", "basic-1gb", "1c-2g"}
	dbStorage := []int32{0, 1, 5, 10}
	for j := 0; j < rng.Intn(3); j++ {
		plan := dbPlans[rng.Intn(len(dbPlans))]
		spec := appv1alpha1.DatabaseSpec{Plan: plan, StorageGB: dbStorage[rng.Intn(len(dbStorage))]}
		if plan == "1c-2g" {
			spec.HighAvailability = rng.Intn(2) == 0
			for k := 0; k < rng.Intn(3); k++ {
				spec.ReadReplicas = append(spec.ReadReplicas,
					appv1alpha1.DatabaseReadReplica{Name: fmt.Sprintf("r%d", k)})
			}
		}
		st.databases = append(st.databases, parsedDatabase{name: fmt.Sprintf("db-%d-%d", seed, j), spec: spec})
		if core.PaidPlan(plan) {
			want.paidGate = true
		}
	}
	kvPlans := []string{"free", "starter"}
	for j := 0; j < rng.Intn(3); j++ {
		plan := kvPlans[rng.Intn(len(kvPlans))]
		spec := appv1alpha1.KeyValueSpec{Plan: plan, StorageGB: int32(5 * rng.Intn(2))}
		st.keyValues = append(st.keyValues, parsedKeyValue{name: fmt.Sprintf("kv-%d-%d", seed, j), spec: spec})
		if core.PaidPlan(plan) {
			want.paidGate = true
		}
	}
	return st, want
}
