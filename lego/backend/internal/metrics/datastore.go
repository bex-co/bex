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

package metrics

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// datastore.go is the managed-datastore (Database/KeyValue) sibling of the
// App-scoped Metrics verb (service.go): disk usage applies to both managed
// Postgres and Key Value instances (w1/m17, w1/m14 — both back onto a PVC);
// active-connections and replication-lag are Postgres-only (CNPG). metrics
// can't import internal/postgres or internal/keyvalue (features never import
// each other), so it resolves the Database/KeyValue CR by name through
// core.Base's GetDatabase/GetKeyValue — the same shared fetch-by-name +
// AuthorizeLabeled gate internal/postgres and internal/keyvalue's own verbs
// use, promoted onto the shared kernel (core.Base.GetApp's sibling) once a
// third caller needed it, rather than a second copy of the Get+gate logic.

// Datastore kinds a DatastoreMetricQuery can target.
const (
	DatastoreDatabase = "database"
	DatastoreKeyValue = "keyvalue"
)

// Datastore metric ids (w3/m10) — bex extensions, no Render equivalent.
const (
	// MetricDisk/MetricDiskCapacity are the used/capacity bytes of a managed
	// datastore's backing PVC (Database or KeyValue). Capacity is a slow-moving
	// config-shaped value, reported the same way cpu_limit/memory_limit are.
	MetricDisk         = "disk"
	MetricDiskCapacity = "disk_capacity"
	// MetricDBConnections is a managed Postgres instance's live active-connection
	// count (CNPG's cnpg_backends_total, summed across states).
	MetricDBConnections = "db_connections"
	// MetricReplicationLag is a managed Postgres instance's replication lag in
	// seconds behind its primary (CNPG's cnpg_pg_replication_lag). Omitted
	// (not a fake zero) for every instance until w1/m22 (Postgres HA) ships —
	// gated on Database.status.highAvailabilityEnabled, which today is always
	// false, so the field is present in the API contract and inert until then.
	MetricReplicationLag = "replication_lag"
)

// DiskUsageRequest is the backend-neutral disk-usage ask for a managed
// datastore's backing PVC(s).
type DiskUsageRequest struct {
	Namespace  string
	Resource   string // Database or KeyValue name (the "resource" label)
	PVCPattern string // regex matching the resource's PVC name(s)
	Metric     string // MetricDisk | MetricDiskCapacity
	Start, End time.Time
	Resolution time.Duration
}

// DiskUsageSource reads PVC usage/capacity history (kubelet volume stats). nil
// => disk/disk_capacity report core.ErrMetricsUnavailable — there is no
// metrics-server fallback for PVC stats (unlike cpu/memory).
type DiskUsageSource func(ctx context.Context, req DiskUsageRequest) ([]MetricSeries, error)

// DBConnectionsRequest is the backend-neutral active-connections ask for one
// managed Postgres instance.
type DBConnectionsRequest struct {
	Namespace  string
	Cluster    string // the Database name == the CNPG cluster name
	Start, End time.Time
	Resolution time.Duration
}

// DBConnectionsSource reads a Postgres instance's active-connection history
// (CNPG's postgres_exporter, cnpg_backends_total). nil => db_connections
// reports core.ErrMetricsUnavailable.
type DBConnectionsSource func(ctx context.Context, req DBConnectionsRequest) ([]MetricSeries, error)

// ReplicationLagRequest is the backend-neutral replication-lag ask for one
// managed Postgres instance.
type ReplicationLagRequest struct {
	Namespace  string
	Cluster    string
	Start, End time.Time
	Resolution time.Duration
}

// ReplicationLagSource reads a Postgres instance's replication-lag history
// (CNPG's cnpg_pg_replication_lag). nil => replication_lag reports
// core.ErrMetricsUnavailable once HA makes it reachable (today the verb never
// calls it — see MetricReplicationLag).
type ReplicationLagSource func(ctx context.Context, req ReplicationLagRequest) ([]MetricSeries, error)

// DatastoreMetricQuery is the resolved request for the DatastoreMetrics verb —
// the Database/KeyValue-scoped sibling of MetricQuery.
type DatastoreMetricQuery struct {
	Kind       string // DatastoreDatabase | DatastoreKeyValue
	Resource   string // Database or KeyValue name
	Metric     string
	Start, End time.Time
	Resolution time.Duration
}

func (q DatastoreMetricQuery) normalized(now time.Time) DatastoreMetricQuery {
	if q.End.IsZero() {
		q.End = now
	}
	if q.Start.IsZero() {
		q.Start = q.End.Add(-defaultMetricSpan)
	}
	if q.Resolution <= 0 {
		q.Resolution = defaultResolution
	}
	return q
}

// pvcPattern anchors a regex matching every PVC ordinal a resource's workload
// can own: CNPG names a Database's PVCs "<name>-<n>" (per-instance, HA-ready);
// the KeyValue StatefulSet's volumeClaimTemplate is named "data", so its PVCs
// are "data-<name>-<n>" (single instance today, per lego/types/tiers).
func pvcPattern(kind, resource string) string {
	name := regexp.QuoteMeta(resource)
	if kind == DatastoreKeyValue {
		return `^data-` + name + `-\d+$`
	}
	return `^` + name + `-\d+$`
}

// DatastoreMetrics is the disk/db_connections/replication_lag read — the
// Database/KeyValue-scoped sibling of Metrics. It fails with core.ErrNotFound
// for an unknown resource, core.ErrForbidden for a metric the resource kind
// doesn't support (e.g. db_connections on a KeyValue), and returns
// Render-shaped series.
func (s *Service) DatastoreMetrics(ctx context.Context, q DatastoreMetricQuery) ([]MetricSeries, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}

	var isHA bool
	switch q.Kind {
	case DatastoreDatabase:
		db, err := s.GetDatabase(ctx, core.RelCanView, q.Resource)
		if err != nil {
			return nil, err
		}
		isHA = db.Status.HighAvailabilityEnabled
	case DatastoreKeyValue:
		if _, err := s.GetKeyValue(ctx, core.RelCanView, q.Resource); err != nil {
			return nil, err
		}
		if q.Metric == MetricDBConnections || q.Metric == MetricReplicationLag {
			return nil, fmt.Errorf("metric %q is Postgres-only, not valid for a key-value resource", q.Metric)
		}
	default:
		return nil, fmt.Errorf("unknown datastore kind %q", q.Kind)
	}

	q = q.normalized(s.Now())
	switch q.Metric {
	case MetricDisk, MetricDiskCapacity:
		if s.DiskUsage == nil {
			return nil, core.ErrMetricsUnavailable
		}
		return s.DiskUsage(ctx, DiskUsageRequest{
			Namespace:  s.Namespace,
			Resource:   q.Resource,
			PVCPattern: pvcPattern(q.Kind, q.Resource),
			Metric:     q.Metric,
			Start:      q.Start,
			End:        q.End,
			Resolution: q.Resolution,
		})
	case MetricDBConnections:
		if s.DBConnections == nil {
			return nil, core.ErrMetricsUnavailable
		}
		return s.DBConnections(ctx, DBConnectionsRequest{
			Namespace: s.Namespace, Cluster: q.Resource, Start: q.Start, End: q.End, Resolution: q.Resolution,
		})
	case MetricReplicationLag:
		// Gated on HighAvailabilityEnabled, not on ReplicationLag being wired:
		// pre-w1/m22 there is no standby, and CNPG's own lag query reports 0 (not
		// absence) from a lone primary — querying it would be the fake-zero this
		// field is explicitly required to avoid. Once HA ships this reaches the
		// real source below, unconditionally, no second milestone needed.
		if !isHA {
			return nil, nil
		}
		if s.ReplicationLag == nil {
			return nil, core.ErrMetricsUnavailable
		}
		return s.ReplicationLag(ctx, ReplicationLagRequest{
			Namespace: s.Namespace, Cluster: q.Resource, Start: q.Start, End: q.End, Resolution: q.Resolution,
		})
	default:
		return nil, fmt.Errorf("unknown metric %q", q.Metric)
	}
}
