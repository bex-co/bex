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
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastore_activity.go sources Render's database-activity and disk graphs
// (w7/m430): network activity, transaction volume and data-volume I/O from
// Prometheus, and lock-delayed queries from the Postgres log lines in Loki.
// The chosen series, and why each, are recorded in
// docs/render-artifacts/metrics-page.md § Database activity and disk graphs.

// DatastoreActivityRequest asks for one resource's network, transaction or
// disk I/O series. Kind picks the pods: a datastore's numbered instances
// (<resource>-<n>) or a service's Deployment pods.
type DatastoreActivityRequest struct {
	Kind      string // DatastoreDatabase | DatastoreKeyValue | DatastoreService
	Namespace string
	Resource  string // the Database or KeyValue name, or the service's App name
	// NotBefore drops a same-named earlier service's pods (service only).
	NotBefore  time.Time
	Metric     string // network_activity | transaction_volume | disk_activity | disk_operations
	Start, End time.Time
	Resolution time.Duration
}

// DatastoreActivitySource returns per-instance, per-bucket series, each
// labelled resource, instance (the raw pod name) and, for the two-series
// graphs, direction (read | write).
type DatastoreActivitySource func(ctx context.Context, req DatastoreActivityRequest) ([]MetricSeries, error)

// LockWaitsRequest asks for one Postgres instance's lock-delayed queries.
type LockWaitsRequest struct {
	Namespace  string
	Database   string // the Database name: its primary CNPG cluster's, which names its replica clusters and their log streams' database labels too
	Start, End time.Time
	Resolution time.Duration
}

// LockWaitsSource returns per-instance counts of lock waits of at least one
// second that ended with the lock granted, per bucket.
type LockWaitsSource func(ctx context.Context, req LockWaitsRequest) ([]MetricSeries, error)

// LabelDirection splits a two-series graph: read and write are Render's words
// for both its Network Activity ("read from and written to the network") and
// its Disk Activity and Disk Operations graphs.
const (
	LabelDirection = "direction"
	directionRead  = "read"
	directionWrite = "write"
)

// nodeRootDisk is the block device every node boots from. On Hetzner Cloud
// that is the server's own disk, /dev/sda; a Hetzner volume attaches as a
// further sd device and its LUKS mapping (a service disk) as a dm device.
// Excluding the boot disk leaves a pod's data volume, so the disk graphs
// measure the volume rather than the container's writable layer. A cluster
// whose volumes live on the boot disk (the local mock's local-path) charts
// nothing.
const nodeRootDisk = "/dev/sda"

// cnpgCounterLookback floors the rate window over CNPG's exporter counters:
// the cnpg-tenant-db job scrapes every 60s (deploy/gitops/base/prometheus.yaml),
// and a rate needs two samples, so four scrape intervals ride out a skipped
// scrape as minRateLookback does for the 15s cAdvisor scrape.
const cnpgCounterLookback = 240

// NewPrometheusDatastoreActivitySource returns the production
// DatastoreActivitySource. base/hc as NewPrometheusRequestSource.
func NewPrometheusDatastoreActivitySource(base string, hc *http.Client) DatastoreActivitySource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req DatastoreActivityRequest) ([]MetricSeries, error) {
		query, unit := promDatastoreActivityQuery(req)
		if query == "" {
			return []MetricSeries{}, nil
		}
		// No point after now: a rate at a future step reads the last samples
		// again (w4/644).
		end := earlierTime(req.End, time.Now())
		if end.Before(req.Start) {
			return nil, nil
		}
		return promInstanceQuery(ctx, hc, base, query, req.Resource, unit, req.Start, end, stepSeconds(req.Resolution))
	}
}

// promDatastoreActivityQuery builds the PromQL and unit for one activity
// graph, or "" for a metric the kind has no source for. Every value is a
// per-bucket sum: the counter's rate over max(step, the scrape floor) times
// the step, which is increase() over the step whenever the step holds enough
// scrapes, and an evenly spread estimate below that, where increase() over the
// step alone would find a single sample and answer nothing.
func promDatastoreActivityQuery(req DatastoreActivityRequest) (string, string) {
	step := stepSeconds(req.Resolution)
	instances := fmt.Sprintf(`namespace=%q,pod=~%q`, req.Namespace, instancePods(req.Kind, req.Resource))
	switch req.Metric {
	case MetricNetworkActivity:
		if req.Kind != DatastoreDatabase {
			return "", ""
		}
		// cAdvisor reports a pod's network on its pod-level row (container="");
		// the cadvisor job keeps those rows for Postgres instance pods only.
		// max, not sum, so a recreated pod's last series does not add itself
		// to the new one's while it ages out of the window.
		window := max(step, minRateLookback)
		m := instances + `,container="",interface="eth0"`
		return fmt.Sprintf(`max by (pod, %s) (%s or %s) * %d`, LabelDirection,
			directionRate("container_network_receive_bytes_total", m, window, directionRead),
			directionRate("container_network_transmit_bytes_total", m, window, directionWrite), step), unitBytes
	case MetricTransactionVolume:
		if req.Kind != DatastoreDatabase {
			return "", ""
		}
		// Committed plus rolled-back transactions in the tenant's databases.
		// postgres and the templates carry the instance manager's and the
		// exporter's own queries, not the tenant's; the scrape drops them too.
		// rate() drops the metric name, so a database's commit and rollback
		// rates would share a label set; the rollback half is labelled apart
		// before the two are added up.
		window := max(step, cnpgCounterLookback)
		m := fmt.Sprintf(`namespace=%q,cnpg_io_cluster=~%q,datname!~"(postgres|template[01])?"`, req.Namespace, appv1alpha1.DatabaseClustersPattern(req.Resource))
		return fmt.Sprintf(`sum by (pod) (rate(cnpg_pg_stat_database_xact_commit{%s}[%ds]) or label_replace(rate(cnpg_pg_stat_database_xact_rollback{%[1]s}[%[2]ds]), "xact", "rollback", "", "")) * %d`,
			m, window, step), unitCount
	case MetricDiskActivity, MetricDiskOperations:
		reads, writes, unit := "container_fs_reads_bytes_total", "container_fs_writes_bytes_total", unitBytes
		if req.Metric == MetricDiskOperations {
			reads, writes, unit = "container_fs_reads_total", "container_fs_writes_total", unitCount
		}
		m := instances + `,container!=""`
		if req.Kind == DatastoreService {
			m = serviceContainers(req.Namespace, req.Resource)
		}
		m += fmt.Sprintf(`,device!=%q`, nodeRootDisk)
		// Per container the max over devices: a LUKS volume's I/O is charged
		// to both its dm mapping and the sd device under it, and a restart
		// briefly leaves the old container's series beside the new one's.
		// Every container's I/O on the volume then adds up per pod (Postgres
		// and its WAL-archiving sidecar both use the data volume).
		window := max(step, minRateLookback)
		perPod := fmt.Sprintf(`sum by (pod, %s) (max by (pod, container, %[1]s) (%s or %s)) * %d`, LabelDirection,
			directionRate(reads, m, window, directionRead), directionRate(writes, m, window, directionWrite), step)
		return excludePodsBefore(perPod, req.Namespace, req.NotBefore), unit
	default:
		return "", ""
	}
}

// directionRate is one counter's rate labelled with its direction, so the read
// and write halves of a graph stay apart through `or` and the aggregation.
func directionRate(counter, matchers string, window int64, direction string) string {
	return fmt.Sprintf(`label_replace(rate(%s{%s}[%ds]), %q, %q, "", "")`, counter, matchers, window, LabelDirection, direction)
}

// lockWaitThresholdMillis is Render's "one second or longer". PostgreSQL logs
// a wait's "acquired … after N ms" line only once the wait outlasted
// deadlock_timeout (1s unless a legacy parameter lowered it), so the threshold
// also holds a lowered deadlock_timeout to Render's second.
const lockWaitThresholdMillis = 1000

// lockWaitLine matches the line PostgreSQL's log_lock_waits writes when a
// waiting session is granted its lock, as the log shipper renders a CNPG
// record ("<time> [<pid>] LOG:  <message>", deploy/gitops/base/log-shipper.yaml):
// `process 228 acquired ShareLock on transaction 761 after 2500.040 ms`. The
// "still waiting" line of the same wait, written at deadlock_timeout, does not
// match, so a wait is counted once, after it ends, and never while it lasts.
// A wait that ends without the lock (a deadlock, a cancel) writes no such line.
const lockWaitLine = `\] LOG:  process [0-9]+ acquired .+ after [0-9]+\.[0-9]+ ms`

// NewLokiLockWaitsSource returns the production LockWaitsSource over the
// Postgres log streams in Loki. base is BEX_LOKI_URL; hc as
// NewLokiRequestMetricsSource.
func NewLokiLockWaitsSource(base string, hc *http.Client) LockWaitsSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req LockWaitsRequest) ([]MetricSeries, error) {
		series, err := lokiQueryRange(ctx, hc, base, lokiLockWaitQuery(req), req.Start, req.End, req.Resolution)
		if err != nil {
			return nil, err
		}
		relabelInstanceSeries(series, req.Database, unitCount)
		return series, nil
	}
}

// lokiLockWaitQuery counts, per instance pod and step bucket, the lock waits
// of at least lockWaitThresholdMillis that a Postgres's own log stream
// recorded as granted. The wait is parsed out of the line, compared, and
// dropped before counting, so each distinct wait does not become a series of
// its own. Only the namespace and database id are interpolated, %q-escaped.
func lokiLockWaitQuery(req LockWaitsRequest) string {
	selector := fmt.Sprintf(`{namespace=%q, database=~%q, type="postgres"}`, req.Namespace, appv1alpha1.DatabaseClustersPattern(req.Database))
	pipeline := fmt.Sprintf("|~ `%s` | regexp `after (?P<lock_wait_ms>[0-9]+)\\.[0-9]+ ms` | __error__=\"\" | lock_wait_ms >= %d | drop lock_wait_ms",
		lockWaitLine, lockWaitThresholdMillis)
	return fmt.Sprintf("sum by (pod) (count_over_time(%s %s%s [%ds]))", selector, pipeline,
		lokiEntryWindow(req.Start, req.End), stepSeconds(req.Resolution))
}
