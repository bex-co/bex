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
// the catalog default, omitted database storage ⇒ the plan's volume floor).
// Static sites run no instance, so they carry no line; persistent service
// disks are a platform non-goal and are never priced. Free tiers and
// runtime-dependent costs (cron, autoscaling, multi-instance) are handled by
// the estimator itself.
func blueprintEstimatedPricing(st parsedStack) *pricing.MonthlyEstimate {
	resources := make([]pricing.MonthlyResource, 0, len(st.services)+len(st.databases)+len(st.keyValues))
	for _, svc := range st.services {
		svcType := effectiveType(svc.req.Type)
		if svcType == appv1alpha1.TypeStaticSite {
			continue
		}
		tier, err := normalizeTierForType(svcType, svc.req.Plan)
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
			Name:             db.name,
			ResourceKind:     store.ResourceKindPostgres,
			Tier:             db.spec.Plan,
			TierLabel:        tierDisplayName(db.spec.Plan),
			StorageGB:        postgres.DatabaseStorageHighWater(&appv1alpha1.Database{Spec: db.spec}),
			HighAvailability: db.spec.HighAvailability,
			ReadReplicas:     len(db.spec.ReadReplicas),
		})
	}
	for _, kv := range st.keyValues {
		plan := kv.spec.Plan
		if plan == "" {
			plan = tiers.Valkey.Default().ID
		}
		storage := kv.spec.StorageGB
		if t, ok := tiers.Valkey.ByID(plan); ok {
			storage = max(storage, t.StorageGB)
		}
		resources = append(resources, pricing.MonthlyResource{
			Name:         kv.name,
			ResourceKind: store.ResourceKindKeyValue,
			Tier:         plan,
			TierLabel:    tierDisplayName(plan),
			StorageGB:    storage,
		})
	}
	est := pricing.Default.MonthlyEstimate(resources)
	return &est
}
