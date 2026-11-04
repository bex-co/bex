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

package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A WAL archive that hangs instead of failing (w5/175): with the object store
// silently dropping packets, barman-cloud-wal-archive blocked for over 20
// minutes, pg_stat_archiver.failed_count stayed 0 and CNPG kept
// ContinuousArchiving=True, so postgres_wal_archive_failed never fired while
// the recovery window stopped advancing. The stall is visible only from the
// instance itself: segments waiting in archive_status/*.ready while nothing
// has been archived for as long.
const (
	// reasonWALArchiveStalled is the WALArchiving=False reason for a hang CNPG
	// does not report.
	reasonWALArchiveStalled = "WALArchiveStalled"
	// walArchiveStallAfter is how long segments must wait, with no archival at
	// all, before the archive counts as stalled. A healthy archive ships a
	// segment within seconds, and an idle database has no .ready segments.
	walArchiveStallAfter = 15 * time.Minute
	// walArchiveStallProbeInterval re-probes a Ready backup-enabled database:
	// a stall changes nothing CNPG reports, so no watch event would wake us.
	walArchiveStallProbeInterval = 5 * time.Minute
)

// WALArchiveStall is one instance whose archive has stalled.
type WALArchiveStall struct {
	Pod              string
	ReadySegments    int64
	SinceLastArchive time.Duration
}

// WALArchiveStallReader reports a stalled WAL archive for one Database, or nil
// when archiving is advancing. Production reads CNPG's already-scraped
// instance metrics from Prometheus; tests inject a deterministic reader.
type WALArchiveStallReader func(ctx context.Context, namespace, database string) (*WALArchiveStall, error)

// NewPrometheusWALArchiveStallReader returns a reader over CNPG's
// cnpg_collector_pg_wal_archive_status and pg_stat_archiver series, scraped by
// the cnpg-tenant-db job. A segment count that never dropped to zero across
// the window, together with no archival in the same window, is a stall; a
// busy database keeps .ready segments too, but keeps archiving them.
func NewPrometheusWALArchiveStallReader(base string, hc *http.Client) WALArchiveStallReader {
	if hc == nil {
		hc = boundedhttp.Shared
	}
	base = strings.TrimRight(base, "/")
	window := fmt.Sprintf("%dm", int(walArchiveStallAfter/time.Minute))
	threshold := fmt.Sprintf("%d", int(walArchiveStallAfter/time.Second))
	return func(ctx context.Context, namespace, database string) (*WALArchiveStall, error) {
		selector := fmt.Sprintf(`namespace=%s,cnpg_io_cluster=%s`, strconv.Quote(namespace), strconv.Quote(database))
		ready, err := promInstantQuery(ctx, hc, base, fmt.Sprintf(
			`min by (pod) (min_over_time(cnpg_collector_pg_wal_archive_status{%s,value="ready"}[%s])) > 0`, selector, window))
		if err != nil {
			return nil, fmt.Errorf("query waiting segments: %w", err)
		}
		since, err := promInstantQuery(ctx, hc, base, fmt.Sprintf(
			`max by (pod) (cnpg_pg_stat_archiver_seconds_since_last_archival{%s}) > %s`, selector, threshold))
		if err != nil {
			return nil, fmt.Errorf("query last archival: %w", err)
		}
		sinceByPod := make(map[string]float64, len(since))
		for _, sample := range since {
			sinceByPod[sample.labels["pod"]] = sample.value
		}
		var stall *WALArchiveStall
		for _, sample := range ready {
			pod := sample.labels["pod"]
			seconds, ok := sinceByPod[pod]
			if pod == "" || !ok {
				continue
			}
			if stall == nil || pod < stall.Pod {
				stall = &WALArchiveStall{Pod: pod, ReadySegments: int64(sample.value), SinceLastArchive: time.Duration(seconds) * time.Second}
			}
		}
		return stall, nil
	}
}

// walArchiveStallRequeue keeps a backup-enabled database probed while a reader
// is configured; zero otherwise.
func (r *DatabaseReconciler) walArchiveStallRequeue(backupEnabled bool) time.Duration {
	if !backupEnabled || r.WALArchiveStallReader == nil {
		return 0
	}
	return walArchiveStallProbeInterval
}

// stampWALArchiving mirrors CNPG's ContinuousArchiving onto WALArchiving, then
// turns a True into False with reasonWALArchiveStalled when the reader sees a
// hang. A probe that cannot read keeps a stall already reported rather than
// flapping it back to True, which would re-arm and re-send
// postgres_wal_archive_failed on the next good read. A continuing stall keeps
// its transition time and message, so it costs no status write per pass.
func (r *DatabaseReconciler) stampWALArchiving(ctx context.Context, db *appv1alpha1.Database, cluster *unstructured.Unstructured, backupEnabled bool) {
	var previous *metav1.Condition
	if cond := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving); cond != nil {
		previous = cond.DeepCopy()
	}
	stampWALArchivingCondition(db, cluster, backupEnabled)
	if r.WALArchiveStallReader == nil {
		return
	}
	current := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving)
	if current == nil || current.Status != metav1.ConditionTrue {
		return
	}
	wasStalled := previous != nil && previous.Reason == reasonWALArchiveStalled
	stall, err := r.WALArchiveStallReader(ctx, db.Namespace, db.Name)
	if err != nil {
		logf.FromContext(ctx).Info("WAL archive stall probe unavailable", "name", db.Name, "reason", err)
		if wasStalled {
			meta.SetStatusCondition(&db.Status.Conditions, *previous)
		}
		return
	}
	if stall == nil {
		return
	}
	stalled := metav1.Condition{
		Type: appv1alpha1.ConditionWALArchiving, Status: metav1.ConditionFalse, Reason: reasonWALArchiveStalled,
		Message: fmt.Sprintf("WAL archiving has stalled: segments have waited on %s for over %s with nothing archived",
			stall.Pod, walArchiveStallAfter),
		ObservedGeneration: db.Generation,
	}
	if wasStalled {
		stalled.LastTransitionTime = previous.LastTransitionTime
	}
	meta.SetStatusCondition(&db.Status.Conditions, stalled)
}
