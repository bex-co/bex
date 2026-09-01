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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastore.go is the managed-datastore (Database/KeyValue) sibling of the
// App-scoped Metrics verb (service.go): disk usage applies to both managed
// Postgres and Key Value instances (w1/m17, w1/m14 — both back onto a PVC);
// active-connections and replication-lag are Postgres-only (CNPG). metrics
// can't import internal/postgres or internal/keyvalue (features never import
// each other), so it resolves the Database/KeyValue CR by name through
// core.Base's AuthorizeDatabase/AuthorizeKeyValue (w6/m17) — the single
// authorize-and-fetch seam internal/postgres and internal/keyvalue's own verbs
// use, promoted onto the shared kernel (core.Base.AuthorizeApp's sibling) once
// a third caller needed it, rather than a second copy of the same logic.

// Datastore kinds a DatastoreMetricQuery can target.
const (
	DatastoreDatabase = "database"
	DatastoreKeyValue = "keyvalue"
	// DatastoreService reads a SERVICE's attached persistent disk (ADR082 D6),
	// not a managed datastore. It rides this verb rather than the App-metrics
	// verb because what it reads is a PVC — the same kubelet volume-stats
	// series, the same used/capacity pair, the same "there is no
	// metrics-server fallback" story. The App-metrics verb answers questions
	// about a workload (cpu, memory, instances) from a different source
	// entirely; a disk is not one of those questions. Only disk/disk_capacity
	// are valid here.
	DatastoreService = "service"
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
	// seconds behind its primary (CNPG's cnpg_pg_replication_lag). Gated on
	// Database.status.highAvailabilityEnabled — omitted (not a fake zero) for
	// a non-HA instance, since CNPG's own query returns 0 from a lone primary.
	MetricReplicationLag = "replication_lag"
	// MetricKVMemory/MetricKVConnections are a managed Key Value (Valkey)
	// instance's used-memory bytes and connected-client count (w5/011, the
	// redis_exporter sidecar the operator attaches). KeyValue-only — the
	// datastore siblings of the App metrics MEMORY/instance_count, distinct ids
	// so they can't be requested on a Postgres resource.
	MetricKVMemory      = "kv_memory"
	MetricKVConnections = "kv_connections"
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
// core.ErrMetricsUnavailable (only called when HighAvailabilityEnabled is true).
type ReplicationLagSource func(ctx context.Context, req ReplicationLagRequest) ([]MetricSeries, error)

// KeyValueStatsRequest is the backend-neutral memory/connections ask for one
// managed Key Value (Valkey) instance.
type KeyValueStatsRequest struct {
	Namespace  string
	Resource   string // the KeyValue name
	Dimension  string // "memory" | "connections"
	Start, End time.Time
	Resolution time.Duration
}

// KeyValueStatsSource reads a Valkey instance's used-memory / connected-clients
// history (the operator's redis_exporter sidecar, scraped by Prometheus's
// valkey-instances job). nil => kv_memory/kv_connections report
// core.ErrMetricsUnavailable.
type KeyValueStatsSource func(ctx context.Context, req KeyValueStatsRequest) ([]MetricSeries, error)

// DatastoreMetricQuery is the resolved request for the DatastoreMetrics verb —
// the Database/KeyValue-scoped sibling of MetricQuery.
type DatastoreMetricQuery struct {
	Kind       string // DatastoreDatabase | DatastoreKeyValue | DatastoreService
	Resource   string // Database, KeyValue, or service name
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
//
// A service's disk is the odd one out and has no ordinal: a service has at most
// one disk and cannot scale past one instance while it holds one (ADR082 D2),
// so the claim is a single name. That name is DERIVED, not spelled here —
// appv1alpha1.DiskPVCName is the same function the operator creates the claim
// with, so a long app name's truncated-plus-digest claim is matched rather than
// silently missed. QuoteMeta still applies: the result is going into a regex.
func pvcPattern(kind, resource string) string {
	name := regexp.QuoteMeta(resource)
	switch kind {
	case DatastoreKeyValue:
		return `^data-` + name + `-\d+$`
	case DatastoreService:
		return `^` + regexp.QuoteMeta(appv1alpha1.DiskPVCName(resource)) + `$`
	default:
		return `^` + name + `-\d+$`
	}
}

// DatastoreMetrics is the disk/db_connections/replication_lag read — the
// Database/KeyValue-scoped sibling of Metrics. It fails with core.ErrNotFound
// for an unknown resource, core.ErrForbidden for a metric the resource kind
// doesn't support (e.g. db_connections on a KeyValue), and returns
// Render-shaped series.
func (s *Service) DatastoreMetrics(ctx context.Context, q DatastoreMetricQuery) ([]MetricSeries, error) {
	// Same order as Metrics: window first, then authorization.
	if err := s.checkWindow(q.Start, q.End); err != nil {
		return nil, err
	}
	var isHA bool
	// Prometheus series are namespace-labeled, so every request below must carry
	// the datastore's OWN namespace (ADR043 D8). Querying the shared one for a
	// migrated datastore returns an empty series rather than an error — the same
	// silent-empty failure the App-side metrics path already had to fix.
	var namespace string
	// The same silent-empty class applies to the RESOURCE half of the matchers
	// (w5/m71): the caller's raw input only AUTHORIZES the read — every matcher
	// below is built from the resolved CR's metadata.name, so the identity the
	// query authorizes is the identity it measures. `var` (not := q.Resource)
	// keeps that structural: a branch that forgets the assignment measures "".
	var resource string
	switch q.Kind {
	case DatastoreDatabase:
		db, err := s.AuthorizeDatabase(ctx, core.RelCanView, q.Resource)
		if err != nil {
			return nil, err
		}
		namespace = db.Namespace
		resource = db.Name
		isHA = db.Status.HighAvailabilityEnabled
		if q.Metric == MetricKVMemory || q.Metric == MetricKVConnections {
			return nil, fmt.Errorf("metric %q is key-value-only, not valid for a database resource", q.Metric)
		}
	case DatastoreKeyValue:
		kv, err := s.AuthorizeKeyValue(ctx, core.RelCanView, q.Resource)
		if err != nil {
			return nil, err
		}
		namespace = kv.Namespace
		resource = kv.Name
		if q.Metric == MetricDBConnections || q.Metric == MetricReplicationLag {
			return nil, fmt.Errorf("metric %q is Postgres-only, not valid for a key-value resource", q.Metric)
		}
	case DatastoreService:
		// A service disk is read against the SERVICE — the App CR is the
		// resource that owns the claim, so canView on the app is the whole
		// authorization story (there is no separate disk object in the
		// cluster to gate on).
		app, err := s.AuthorizeApp(ctx, core.RelCanView, q.Resource)
		if err != nil {
			return nil, err
		}
		namespace = app.Namespace
		resource = app.Name
		// Every other datastore metric describes a datastore process. A
		// service has none — its cpu/memory/instances live on the App-metrics
		// verb — so anything but the two PVC series is a caller error, named
		// rather than silently answered with an empty series.
		if q.Metric != MetricDisk && q.Metric != MetricDiskCapacity {
			return nil, fmt.Errorf("metric %q is not valid for a service resource; only disk and disk_capacity are", q.Metric)
		}
		// A service without a disk has no claim to measure. Report absence as
		// an empty series, not an error: the Disk tab asks for the graph
		// before it knows whether one exists, and a 4xx there would render as
		// a failure rather than "no disk yet".
		if app.Spec.Disk == nil {
			return nil, nil
		}
	default:
		// An unknown kind names no resource to authorize against (there is no
		// AuthorizeDatabase/AuthorizeKeyValue call to make) — fall back to a
		// bare Authorize against the caller's own workspace so a malformed
		// request still 403s before revealing the kind was invalid.
		if err := s.Authorize(ctx, core.RelCanView); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unknown datastore kind %q", q.Kind)
	}

	q = q.normalized(s.Now())
	if err := checkPointBudget(q.Start, q.End, q.Resolution); err != nil {
		return nil, err
	}
	switch q.Metric {
	case MetricDisk, MetricDiskCapacity:
		if s.DiskUsage == nil {
			return nil, core.ErrMetricsUnavailable
		}
		return s.DiskUsage(ctx, DiskUsageRequest{
			Namespace:  namespace,
			Resource:   resource,
			PVCPattern: pvcPattern(q.Kind, resource),
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
			Namespace: namespace, Cluster: resource, Start: q.Start, End: q.End, Resolution: q.Resolution,
		})
	case MetricReplicationLag:
		// Gated on HighAvailabilityEnabled: a non-HA instance has no standby, and
		// CNPG's own lag query reports 0 (not absence) from a lone primary —
		// querying it would be the fake-zero this field is explicitly required to
		// avoid. An HA Postgres (w1/m22) reaches the real Prometheus source below.
		if !isHA {
			return nil, nil
		}
		if s.ReplicationLag == nil {
			return nil, core.ErrMetricsUnavailable
		}
		return s.ReplicationLag(ctx, ReplicationLagRequest{
			Namespace: namespace, Cluster: resource, Start: q.Start, End: q.End, Resolution: q.Resolution,
		})
	case MetricKVMemory, MetricKVConnections:
		if s.KeyValueStats == nil {
			return nil, core.ErrMetricsUnavailable
		}
		dimension := "memory"
		if q.Metric == MetricKVConnections {
			dimension = "connections"
		}
		return s.KeyValueStats(ctx, KeyValueStatsRequest{
			Namespace: namespace, Resource: resource, Dimension: dimension,
			Start: q.Start, End: q.End, Resolution: q.Resolution,
		})
	default:
		return nil, fmt.Errorf("unknown metric %q", q.Metric)
	}
}
