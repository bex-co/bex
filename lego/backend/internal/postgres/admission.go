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
	"fmt"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CheckDatabaseAdmission validates capacity and paid features against the full
// proposed spec. A nil current means creation. Updates preserve existing
// out-of-policy state through unrelated edits and permit disabling features;
// they cannot add capacity or strand features on an unsupported target plan.
// HA keeps its separate presence-aware gate from CheckHighAvailabilityPlan.
func CheckDatabaseAdmission(current *appv1alpha1.Database, desired appv1alpha1.DatabaseSpec) error {
	plan := admissionPlanID(desired.Plan)
	planChanged := current == nil || plan != admissionPlanID(current.Spec.Plan)
	storageChanged := current == nil || desired.StorageGB != current.Spec.StorageGB
	replicasChanged := current == nil || !slices.Equal(desired.ReadReplicas, current.Spec.ReadReplicas)
	checkReplicas := len(desired.ReadReplicas) > 0 && (planChanged || storageChanged || replicasChanged)
	checkAutoscaling := desired.DiskAutoscaling && (planChanged || current == nil || !current.Spec.DiskAutoscaling)
	checkPooler := desired.Pooler && (planChanged || current == nil || !current.Spec.Pooler)
	if !planChanged && !storageChanged && !checkReplicas && !checkAutoscaling && !checkPooler {
		return nil
	}
	tier, ok := tiers.Postgres.ByID(plan)
	if !ok {
		return core.NewBadRequestError("POSTGRES_PLAN_UNSUPPORTED",
			fmt.Sprintf("plan must be one of %s", strings.Join(tiers.Postgres.IDs(), "|")), map[string]any{"field": "plan", "plan": plan})
	}
	if desired.StorageGB < 0 {
		return core.NewBadRequestError("POSTGRES_STORAGE_INVALID", "diskSizeGB must not be negative", map[string]any{"field": "diskSizeGB"})
	}
	var highWater int32
	if current != nil {
		highWater = DatabaseStorageHighWater(current)
		if storageChanged {
			if err := validateDatabaseStorageResize(current, &desired.StorageGB); err != nil {
				return err
			}
		}
	}
	storage := tiers.Postgres.EffectiveStorageGB(plan, desired.StorageGB, highWater)
	if planChanged || storageChanged {
		if maximum := tiers.Postgres.MaxStorageGB(plan); storage > maximum {
			code := "POSTGRES_STORAGE_LIMIT_EXCEEDED"
			message := fmt.Sprintf("Postgres storage is limited to %d GB; the requested or allocated size is %d GB", maximum, storage)
			if !tier.SupportsDiskAutoscaling() {
				code = "POSTGRES_STORAGE_PLAN_UNSUPPORTED"
				message = fmt.Sprintf("free Postgres storage is fixed at %d GB; upgrade to a paid plan for %d GB of storage", maximum, storage)
			}
			return core.NewBadRequestError(code, message, map[string]any{"field": "diskSizeGB", "plan": plan, "maxStorageGB": maximum, "storageGB": storage})
		}
	}
	if checkReplicas {
		limit := tier.MaxReadReplicas()
		if limit == 0 {
			return core.NewBadRequestError("POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED",
				fmt.Sprintf("read replicas require a Postgres plan with at least %s CPU; plan %q has %s CPU; remove read replicas before selecting this plan", tiers.PostgresReadReplicaMinCPU, plan, tier.CPU),
				map[string]any{"field": "readReplicas", "plan": plan, "cpu": tier.CPU})
		}
		if len(desired.ReadReplicas) > limit {
			return core.NewBadRequestError("POSTGRES_READ_REPLICA_LIMIT_EXCEEDED",
				fmt.Sprintf("a Postgres database supports at most %d read replicas", limit),
				map[string]any{"field": "readReplicas", "maxReadReplicas": limit})
		}
		if storage < tiers.PostgresReadReplicaMinStorageGB {
			return core.NewBadRequestError("POSTGRES_READ_REPLICA_STORAGE_UNSUPPORTED",
				fmt.Sprintf("read replicas require at least %d GB of Postgres storage; effective storage is %d GB", tiers.PostgresReadReplicaMinStorageGB, storage),
				map[string]any{"field": "readReplicas", "minStorageGB": tiers.PostgresReadReplicaMinStorageGB, "storageGB": storage})
		}
	}
	if checkAutoscaling && !tier.SupportsDiskAutoscaling() {
		return core.NewBadRequestError("POSTGRES_DISK_AUTOSCALING_PLAN_UNSUPPORTED", "disk autoscaling requires a paid Postgres plan; disable disk autoscaling before selecting the free plan",
			map[string]any{"field": "enableDiskAutoscaling", "plan": plan})
	}
	if checkPooler && !tier.SupportsConnectionPooling() {
		return core.NewBadRequestError("POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED", "connection pooling requires a paid Postgres plan; disable connection pooling before selecting the free plan",
			map[string]any{"field": "connectionPool", "plan": plan})
	}
	return nil
}

func admissionPlanID(plan string) string {
	if plan == "" {
		return tiers.Postgres.Default().ID
	}
	return tiers.Postgres.CanonicalID(plan)
}

func checkPostgresPatchAdmission(current *appv1alpha1.Database, patch PostgresPatch) error {
	if err := validateDatabaseStorageResize(current, patch.DiskSizeGB); err != nil {
		return err
	}
	if err := checkHighAvailabilityPatch(current, patch.Plan, patch.EnableHighAvailability); err != nil {
		return err
	}
	desired := current.DeepCopy()
	patch.apply(desired)
	return CheckDatabaseAdmission(current, desired.Spec)
}
