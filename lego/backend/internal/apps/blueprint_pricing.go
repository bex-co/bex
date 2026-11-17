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

import (
	"cmp"
	"context"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// blueprintEstimatedPricing projects a validated parsed stack to its always-on
// monthly cost (the Render blueprint "Estimated pricing" panel, on bex's
// price sheet). It prices what bex would actually provision: plans resolve
// through the same defaults the create path applies (omitted service plan ⇒
// the paid default, omitted database plan ⇒ basic-256mb, omitted Key Value
// plan ⇒ starter, omitted database storage ⇒ the plan's volume floor); the
// parse has already written both datastore plan defaults into each spec.
// Those are create defaults: callers pass stackAtKeptPlans, so an existing
// resource whose entry omits plan is priced on the plan it keeps.
// Static sites run no instance, so they carry no line; a persistent service
// disk rides its service's line as a provisioned-storage figure (ADR082 D8).
// Free tiers and runtime-dependent costs (cron, autoscaling, multi-instance)
// are handled by the estimator itself.
func blueprintEstimatedPricing(st parsedStack) *pricing.MonthlyEstimate {
	resources := make([]pricing.MonthlyResource, 0, len(st.services)+len(st.databases)+len(st.keyValues))
	for _, svc := range st.services {
		svcType := effectiveType(svc.req.Type)
		if svcType == appv1alpha1.TypeStaticSite {
			continue
		}
		tier, err := resolveTierForType(svcType, svc.req.Plan)
		if err != nil {
			continue // parse already rejected invalid plans; never reached on a valid stack
		}
		resources = append(resources, pricing.MonthlyResource{
			Name:         svc.req.Name,
			ResourceKind: store.ResourceKindService,
			Tier:         tier,
			TierLabel:    tierDisplayName(tier),
			Instances:    int(svc.req.Replicas),
			Autoscaling:  svc.req.Autoscaling != nil,
			Cron:         svcType == appv1alpha1.TypeCronJob,
			// Render's Blueprint panel shows a Disks group; bex's absence of one
			// used to be a recorded divergence (ADR018) because disks were a
			// non-goal. The disk rides its service's line as a storage figure,
			// the same shape a datastore's volume does.
			StorageGB: blueprintDiskSizeGB(svc.req.Disk),
		})
	}
	for _, db := range st.databases {
		resources = append(resources, pricing.MonthlyResource{
			Name:         db.name,
			ResourceKind: store.ResourceKindPostgres,
			Tier:         db.spec.Plan,
			// The Postgres family's own spelling, so the Blueprint panel names a
			// plan exactly as the Postgres plan picker does (1c-2g, Basic 256MB).
			TierLabel:        postgres.TierDisplayName(db.spec.Plan),
			StorageGB:        postgres.DatabaseStorageHighWater(&appv1alpha1.Database{Spec: db.spec}),
			HighAvailability: db.spec.HighAvailability,
			ReadReplicas:     len(db.spec.ReadReplicas),
		})
	}
	for _, kv := range st.keyValues {
		plan := kv.spec.Plan
		storage := kv.spec.StorageGB
		if t, ok := tiers.Valkey.ByID(plan); ok {
			storage = max(storage, t.StorageGB)
		}
		resources = append(resources, pricing.MonthlyResource{
			Name:         kv.name,
			ResourceKind: store.ResourceKindKeyValue,
			Tier:         plan,
			// The Key Value family's own spelling (3g → "3 GB",
			// pro_plus → "Pro Plus"), as its plan picker shows it — the
			// compute title-caser would render "3g" and "Pro_plus".
			TierLabel: keyvalue.TierDisplayName(plan),
			StorageGB: storage,
		})
	}
	est := pricing.Default.MonthlyEstimate(resources)
	return &est
}

// stackAtKeptPlans is st with each entry that names an existing resource and
// omits plan on that resource's own plan (w33/006). The parse writes each
// kind's create default into a plan-less entry, but a sync or adopt keeps an
// existing resource's plan when the file omits it, as Render "retains the
// current compute plan for an existing service" (the plan row of
// BlueprintServiceFieldPolicies, ApplyBlueprintDatabaseSpec,
// ApplyBlueprintKeyValueSpec). The default is what a create costs, not what
// this apply leaves running, so the estimate and the paid-intent gate both
// read this stack. Existence is the apply's own lookup: the snapshot's
// services by manifest name, a datastore by its unique display name. Only a
// kind with a plan-less entry is listed; without a cluster client nothing
// exists. st itself is not changed: the apply writes from the parse.
func (s *Service) stackAtKeptPlans(ctx context.Context, st parsedStack, snap *workspaceSnapshot) (parsedStack, error) {
	if s.Client == nil {
		return st, nil
	}
	omitsPlan := func(fields map[string]BlueprintField) bool {
		_, declared := fields["plan"]
		return !declared
	}
	// A service with no field record is re-applied whole, plan included
	// (applyBlueprintServiceSpec), so only a recorded omission keeps its plan.
	keepsServicePlan := func(svc parsedService) bool { return svc.fields != nil && omitsPlan(svc.fields) }
	if slices.ContainsFunc(st.services, keepsServicePlan) {
		services, err := snap.services(ctx)
		if err != nil {
			return parsedStack{}, err
		}
		st.services = slices.Clone(st.services)
		for i := range st.services {
			svc := &st.services[i]
			if live := services[svc.req.Name]; live != nil && keepsServicePlan(*svc) {
				// An untiered service runs on the catalog default, as
				// normalizeTierOrPlan reads an empty plan.
				svc.req.Plan = cmp.Or(live.Spec.Tier, tiers.Compute.Default().ID)
			}
		}
	}
	tenantID, scoped := s.Tenant(ctx)
	if slices.ContainsFunc(st.databases, func(db parsedDatabase) bool { return omitsPlan(db.fields) }) {
		databases, err := snap.databases(ctx)
		if err != nil {
			return parsedStack{}, err
		}
		st.databases = slices.Clone(st.databases)
		for i := range st.databases {
			db := &st.databases[i]
			if !omitsPlan(db.fields) {
				continue
			}
			if live, _ := uniqueDatabaseByDisplayName(databases.Items, scoped, tenantID, db.name); live != nil {
				db.spec.Plan = cmp.Or(live.Spec.Plan, tiers.Postgres.Default().ID)
			}
		}
	}
	if slices.ContainsFunc(st.keyValues, func(kv parsedKeyValue) bool { return omitsPlan(kv.fields) }) {
		keyValues, err := snap.keyValues(ctx)
		if err != nil {
			return parsedStack{}, err
		}
		st.keyValues = slices.Clone(st.keyValues)
		for i := range st.keyValues {
			kv := &st.keyValues[i]
			if !omitsPlan(kv.fields) {
				continue
			}
			if live, _ := uniqueKeyValueByDisplayName(keyValues.Items, scoped, tenantID, kv.name); live != nil {
				kv.spec.Plan = cmp.Or(live.Spec.Plan, tiers.Valkey.Default().ID)
			}
		}
	}
	return st, nil
}
