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

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/types/tiers"
)

// datastoreFootprint is what ONE datastore at its largest shape charges to the
// unscoped tenant-quota (w2/056). Pods are absent on purpose: datastore Pods
// run under the datastore PriorityClass and the pod count lives in the scoped
// tenant-compute-quota, so they are never counted there (baseResourceQuotas).
// The shapes are the operator's (lego/operator/internal/controller), which the
// backend cannot import, so each term names its source.
type datastoreFootprint struct {
	pvcs, storageGi, ephemeralRequestsGi, ephemeralLimitsGi, jobs int64
}

const (
	// limitRangeEphemeralRequestGi / LimitGi are baseLimitRange's defaults,
	// charged to every container that declares no ephemeral-storage: CNPG's
	// postgres and barman-cloud containers, PgBouncer, CNPG's bootstrap Jobs,
	// Valkey and its exporter, the Key Value proxy and the export Job.
	limitRangeEphemeralRequestGi, limitRangeEphemeralLimitGi = 1, 10

	// postgresHAInstances is a high-availability Cluster's primary + standby
	// (databaseInstanceCount); read replicas add tiers.PostgresMaxReadReplicas
	// more. Each instance is one Pod with one data PVC: cnpgClusterSpec sets
	// no walStorage, so there is no separate WAL volume.
	postgresHAInstances = 2
	// postgresInstanceContainers: postgres plus the barman-cloud plugin
	// sidecar CNPG injects when backups are on.
	postgresInstanceContainers = 2
	// postgresPoolerContainers: the single-instance PgBouncer Pooler
	// (poolerSpec), plus one RollingUpdate surge Pod of its Deployment.
	postgresPoolerContainers = 2
	// postgresCNPGJobContainers: the initdb/join Job CNPG runs while it
	// creates an instance, one at a time per Cluster and deleted afterwards.
	postgresCNPGJobContainers = 1
	// postgresExportJobs mirrors the operator's maxConcurrentExportsPerDB.
	// Each export Pod's pg-dump init container and upload container charge
	// max(init, sum) = one container's defaults.
	postgresExportJobs = 3
	// postgresJobs: the export Jobs, the expiry Job of each, the backup purge
	// Job a deletion runs, and CNPG's initdb/join Job.
	postgresJobs = 2*postgresExportJobs + 1 + 1

	// keyValueContainers: the StatefulSet Pod's valkey + metrics exporter
	// (its persistence init container charges max(init, sum), adding
	// nothing), plus the internal proxy Deployment and its RollingUpdate
	// surge Pod.
	keyValueContainers = 2 + 2
	// keyValueBackupJobs: the daily backup CronJob keeps 3 successful and 3
	// failed Jobs (keyvalue_backup.go) and runs at most one more
	// (ForbidConcurrent).
	keyValueBackupJobs = 3 + 3 + 1
	// keyValueBackupHeadroomGi mirrors keyValueBackupWorkHeadroomGiB: the
	// running backup Pod declares 2×storage+1 GiB ephemeral-storage on
	// requests and limits.
	keyValueBackupHeadroomGi = 1
)

// postgresFootprint is a Postgres at its largest shape: a high-availability
// pair plus every read replica, a pooler, a CNPG bootstrap Job and the
// maximum concurrent exports, each instance at the largest plan floor.
func postgresFootprint() datastoreFootprint {
	instances := int64(postgresHAInstances + tiers.PostgresMaxReadReplicas)
	containers := instances*postgresInstanceContainers + postgresPoolerContainers +
		postgresCNPGJobContainers + postgresExportJobs
	return datastoreFootprint{
		pvcs:                instances,
		storageGi:           instances * postgresFloorGi(),
		ephemeralRequestsGi: containers * limitRangeEphemeralRequestGi,
		ephemeralLimitsGi:   containers * limitRangeEphemeralLimitGi,
		jobs:                postgresJobs,
	}
}

// keyValueFootprint is a Key Value with backups on, at the largest plan floor,
// while its backup runs.
func keyValueFootprint() datastoreFootprint {
	floor := keyValueFloorGi()
	backup := 2*floor + keyValueBackupHeadroomGi
	return datastoreFootprint{
		pvcs:                1,
		storageGi:           floor,
		ephemeralRequestsGi: keyValueContainers*limitRangeEphemeralRequestGi + backup,
		ephemeralLimitsGi:   keyValueContainers*limitRangeEphemeralLimitGi + backup,
		jobs:                keyValueBackupJobs,
	}
}

// postgresFloorGi is the largest Postgres plan's included storage, and at
// least the read-replica minimum (a Cluster with readers has at least that).
func postgresFloorGi() int64 {
	floor := int64(tiers.PostgresReadReplicaMinStorageGB)
	for _, id := range tiers.Postgres.IDs() {
		if tier, ok := tiers.Postgres.ByID(id); ok {
			floor = max(floor, int64(tier.StorageGB))
		}
	}
	return floor
}

// keyValueFloorGi is the largest Key Value plan's included storage.
func keyValueFloorGi() int64 {
	var floor int64
	for _, id := range tiers.Valkey.IDs() {
		if tier, ok := tiers.Valkey.ByID(id); ok {
			floor = max(floor, int64(tier.StorageGB))
		}
	}
	return floor
}

// datastoreQuotaRoom is the unscoped tenant-quota room a plan's datastore
// count caps need when every Postgres and Key Value is at its largest shape,
// so that those caps bind before any shared dimension does.
func datastoreQuotaRoom(caps QuotaCaps) corev1.ResourceList {
	pg, kv := postgresFootprint(), keyValueFootprint()
	sum := func(field func(datastoreFootprint) int64) int64 {
		return caps.Postgres*field(pg) + caps.KeyValues*field(kv)
	}
	gi := func(field func(datastoreFootprint) int64) resource.Quantity {
		return *resource.NewQuantity(sum(field)<<30, resource.BinarySI)
	}
	count := func(field func(datastoreFootprint) int64) resource.Quantity {
		return *resource.NewQuantity(sum(field), resource.DecimalSI)
	}
	return corev1.ResourceList{
		corev1.ResourcePersistentVolumeClaims:   count(func(f datastoreFootprint) int64 { return f.pvcs }),
		corev1.ResourceRequestsStorage:          gi(func(f datastoreFootprint) int64 { return f.storageGi }),
		corev1.ResourceRequestsEphemeralStorage: gi(func(f datastoreFootprint) int64 { return f.ephemeralRequestsGi }),
		corev1.ResourceLimitsEphemeralStorage:   gi(func(f datastoreFootprint) int64 { return f.ephemeralLimitsGi }),
		"count/jobs.batch":                      count(func(f datastoreFootprint) int64 { return f.jobs }),
	}
}
