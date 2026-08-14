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
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	databaseDiskAutoscaleThreshold = 0.90
	databaseDiskAutoscaleCooldown  = 12 * time.Hour
	databaseDiskAutoscaleInterval  = time.Minute
	databaseDiskResizeHistoryLimit = 20
	databaseDiskSampleFailureLimit = 3

	annotDiskAutoscaleAt       = "app.bex.co/disk-autoscale-at"
	annotDiskAutoscaleFromGB   = "app.bex.co/disk-autoscale-from-gb"
	annotDiskAutoscaleToGB     = "app.bex.co/disk-autoscale-to-gb"
	annotDiskAutoscaleUsed     = "app.bex.co/disk-autoscale-used-bytes"
	annotDiskAutoscaleCapacity = "app.bex.co/disk-autoscale-capacity-bytes"
	annotDiskSampleFailures    = "app.bex.co/disk-autoscale-sample-failures"

	// conditionDiskGrowthBlockedByQuota is set on Database.status when a disk
	// autoscale grow would exceed the namespace's requests.storage ResourceQuota
	// (ADR045 Finding 4, w7/m59). It is an observable, self-clearing block: the
	// operator backs off instead of stranding spec.storageGB ahead of a PVC that
	// cannot grow, and growth resumes once quota headroom returns.
	conditionDiskGrowthBlockedByQuota = "DiskGrowthBlockedByQuota"
)

// DatabaseDiskUsage is the fullest CNPG instance PVC's current kubelet sample.
// HA replicas each hold a complete copy, so the fullest replica drives growth.
type DatabaseDiskUsage struct {
	PVC           string
	UsedBytes     int64
	CapacityBytes int64
}

func (u DatabaseDiskUsage) ratio() float64 {
	if u.UsedBytes < 0 || u.CapacityBytes <= 0 {
		return 0
	}
	return float64(u.UsedBytes) / float64(u.CapacityBytes)
}

// DatabaseDiskUsageReader reads current PVC usage without depending on the
// optional control-plane store. Production uses Prometheus's already-scraped
// kubelet_volume_stats series; tests inject a deterministic reader.
type DatabaseDiskUsageReader func(ctx context.Context, namespace, database string) (DatabaseDiskUsage, error)

// NewPrometheusDatabaseDiskUsageReader returns a reader over the cluster's
// existing kubelet PVC series. No new scrape or Kubernetes RBAC is required.
func NewPrometheusDatabaseDiskUsageReader(base string, hc *http.Client) DatabaseDiskUsageReader {
	if hc == nil {
		hc = boundedhttp.Shared
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, namespace, database string) (DatabaseDiskUsage, error) {
		pattern := "^" + regexp.QuoteMeta(database) + `-[0-9]+$`
		selector := fmt.Sprintf(`{namespace=%s,persistentvolumeclaim=~%s}`, strconv.Quote(namespace), strconv.Quote(pattern))
		used, err := promInstantQuery(ctx, hc, base, "kubelet_volume_stats_used_bytes"+selector)
		if err != nil {
			return DatabaseDiskUsage{}, fmt.Errorf("query used bytes: %w", err)
		}
		capacity, err := promInstantQuery(ctx, hc, base, "kubelet_volume_stats_capacity_bytes"+selector)
		if err != nil {
			return DatabaseDiskUsage{}, fmt.Errorf("query capacity bytes: %w", err)
		}

		capacityByPVC := make(map[string]float64, len(capacity))
		for _, sample := range capacity {
			pvc := sample.labels["persistentvolumeclaim"]
			if pvc != "" && sample.value > capacityByPVC[pvc] {
				capacityByPVC[pvc] = sample.value
			}
		}
		var fullest DatabaseDiskUsage
		for _, sample := range used {
			pvc := sample.labels["persistentvolumeclaim"]
			capBytes := capacityByPVC[pvc]
			if pvc == "" || capBytes <= 0 {
				continue
			}
			candidate := DatabaseDiskUsage{
				PVC:           pvc,
				UsedBytes:     int64(sample.value),
				CapacityBytes: int64(capBytes),
			}
			if fullest.PVC == "" || candidate.ratio() > fullest.ratio() {
				fullest = candidate
			}
		}
		if fullest.PVC == "" {
			return DatabaseDiskUsage{}, fmt.Errorf("no complete PVC usage sample for %s/%s", namespace, database)
		}
		return fullest, nil
	}
}

// nextDatabaseDiskSize applies Render's captured rule: add 50%, round upward
// to a 5 GB multiple, and stop at 16 TB. It is strictly grow-only.
func nextDatabaseDiskSize(currentGB int32) int32 {
	capGB := tiers.Postgres.DiskAutoscalingCapGB()
	if currentGB <= 0 || currentGB >= capGB {
		return currentGB
	}
	// ceil(current * 1.5), followed by ceil-to-5GB.
	target := (currentGB*3 + 1) / 2
	target = ((target + 4) / 5) * 5
	if target > capGB {
		return capGB
	}
	return target
}

func databaseDiskAutoscaleDecision(currentGB int32, usage DatabaseDiskUsage, lastResize, now time.Time) (int32, bool) {
	if usage.ratio() < databaseDiskAutoscaleThreshold {
		return currentGB, false
	}
	if !lastResize.IsZero() && now.Sub(lastResize) < databaseDiskAutoscaleCooldown {
		return currentGB, false
	}
	next := nextDatabaseDiskSize(currentGB)
	return next, next > currentGB
}

func (r *DatabaseReconciler) databaseNow() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// applyDiskAutoscaling evaluates one sample and persists a resize intent. The
// last-resize annotation is patched atomically with spec.storageGB; that is the
// anti-flap source of truth while CNPG and kubelet catch up. Status history is
// reconstructed from those annotations if a later status write is interrupted.
func (r *DatabaseReconciler) applyDiskAutoscaling(ctx context.Context, db *appv1alpha1.Database) (time.Duration, error) {
	statusBefore := db.DeepCopy()
	if r.syncLastDiskResizeStatus(db) {
		if err := r.Status().Patch(ctx, db, client.MergeFrom(statusBefore)); err != nil {
			return databaseDiskAutoscaleInterval, fmt.Errorf("persist disk autoscale status: %w", err)
		}
	}
	if !db.Spec.DiskAutoscaling || db.Spec.Suspended {
		if err := r.resetDiskSampleFailures(ctx, db); err != nil {
			return 0, err
		}
		// Autoscaling off (or suspended) — a lingering quota block is now moot.
		return 0, r.clearDiskGrowthBlockedByQuota(ctx, db)
	}
	if r.DiskUsageReader == nil {
		return databaseDiskAutoscaleInterval, nil
	}

	usage, err := r.DiskUsageReader(ctx, db.Namespace, db.Name)
	if err != nil {
		logf.FromContext(ctx).Info("database disk autoscaling sample unavailable", "name", db.Name, "reason", err)
		return databaseDiskAutoscaleInterval, r.recordDiskSampleFailure(ctx, db, err)
	}
	if err := r.resetDiskSampleFailures(ctx, db); err != nil {
		return databaseDiskAutoscaleInterval, err
	}

	plan, currentGB := resolvePlan(db.Spec)
	currentGB = max(currentGB, db.Status.AllocatedStorageGB)
	now := r.databaseNow()
	lastResize, _ := time.Parse(time.RFC3339, db.Annotations[annotDiskAutoscaleAt])
	nextGB, grow := databaseDiskAutoscaleDecision(currentGB, usage, lastResize, now)
	if !grow {
		// Not growing this cycle — drop any stale quota-block condition.
		return databaseDiskAutoscaleInterval, r.clearDiskGrowthBlockedByQuota(ctx, db)
	}

	// Pre-flight the namespace ResourceQuota before growing. The operator patches
	// the CNPG Cluster CR, not the PVC — CNPG resizes the PVC — so a
	// requests.storage quota rejection would surface only inside CNPG as a
	// silently-unfulfilled resize, never as a Forbidden on our own write. Reading
	// the quota's headroom here turns that silent failure into an observable
	// Database condition and backs off, instead of stranding spec.storageGB ahead
	// of a PVC that cannot grow. CNPG grows one PVC per instance, so the aggregate
	// delta scales with the cluster size.
	instances := int64(plan.Instances)
	if db.Spec.HighAvailability && instances < 2 {
		instances = 2
	}
	blocked, quotaDetail, quotaErr := r.diskGrowthQuotaBlocks(ctx, db.Namespace, int64(nextGB-currentGB)*instances)
	if quotaErr != nil {
		// Could not read the quota — back off and retry rather than growing blind
		// (risking the silent CNPG resize failure) or hot-looping an error.
		logf.FromContext(ctx).Info("database disk autoscaling quota preflight unavailable", "name", db.Name, "reason", quotaErr)
		return databaseDiskAutoscaleInterval, nil
	}
	if blocked {
		return databaseDiskAutoscaleInterval, r.setDiskGrowthBlockedByQuota(ctx, db, nextGB, quotaDetail)
	}

	before := db.DeepCopy()
	db.Spec.StorageGB = nextGB
	metav1.SetMetaDataAnnotation(&db.ObjectMeta, annotDiskAutoscaleAt, now.Format(time.RFC3339))
	db.Annotations[annotDiskAutoscaleFromGB] = strconv.FormatInt(int64(currentGB), 10)
	db.Annotations[annotDiskAutoscaleToGB] = strconv.FormatInt(int64(nextGB), 10)
	db.Annotations[annotDiskAutoscaleUsed] = strconv.FormatInt(usage.UsedBytes, 10)
	db.Annotations[annotDiskAutoscaleCapacity] = strconv.FormatInt(usage.CapacityBytes, 10)
	if err := r.Patch(ctx, db, client.MergeFrom(before)); err != nil {
		return databaseDiskAutoscaleInterval, fmt.Errorf("persist disk autoscale intent: %w", err)
	}
	if r.Recorder != nil {
		r.Recorder.Eventf(db, "Normal", "DiskAutoscaled",
			"grew Postgres storage from %d GB to %d GB after PVC %s reached %.1f%% full",
			currentGB, nextGB, usage.PVC, usage.ratio()*100)
	}
	statusBefore = db.DeepCopy()
	if r.syncLastDiskResizeStatus(db) {
		if err := r.Status().Patch(ctx, db, client.MergeFrom(statusBefore)); err != nil {
			return databaseDiskAutoscaleInterval, fmt.Errorf("persist disk autoscale status: %w", err)
		}
	}
	logf.FromContext(ctx).Info("database disk autoscaled", "name", db.Name, "pvc", usage.PVC,
		"fromGB", currentGB, "toGB", nextGB, "usagePercent", usage.ratio()*100)
	return databaseDiskAutoscaleInterval, r.clearDiskGrowthBlockedByQuota(ctx, db)
}

// diskGrowthQuotaBlocks reports whether growing this namespace's aggregate
// requests.storage by deltaGi GiB would exceed any ResourceQuota in it, reading
// the live quotas the way the API server's admission does. A namespace with no
// requests.storage cap — or no quota at all, e.g. the shared-namespace runtime
// where per-tenant namespaces are disabled — never blocks, keeping those
// deployments byte-identical.
func (r *DatabaseReconciler) diskGrowthQuotaBlocks(ctx context.Context, namespace string, deltaGi int64) (bool, string, error) {
	if deltaGi <= 0 {
		return false, "", nil
	}
	var quotas corev1.ResourceQuotaList
	if err := r.List(ctx, &quotas, client.InNamespace(namespace)); err != nil {
		return false, "", err
	}
	delta := resource.NewQuantity(deltaGi*(1<<30), resource.BinarySI)
	for i := range quotas.Items {
		q := &quotas.Items[i]
		hard, ok := q.Status.Hard[corev1.ResourceRequestsStorage]
		if !ok {
			// Status may not be populated yet (freshly created quota); fall back
			// to the declared spec so a brand-new namespace still enforces.
			hard, ok = q.Spec.Hard[corev1.ResourceRequestsStorage]
		}
		if !ok {
			continue
		}
		used := q.Status.Used[corev1.ResourceRequestsStorage]
		projected := used.DeepCopy()
		projected.Add(*delta)
		if projected.Cmp(hard) > 0 {
			return true, fmt.Sprintf("ResourceQuota %q (requests.storage hard=%s, used=%s)", q.Name, hard.String(), used.String()), nil
		}
	}
	return false, "", nil
}

// setDiskGrowthBlockedByQuota records the quota block on Database.status and
// emits a single Warning event on the transition into blocked (subsequent
// blocked reconciles only refresh the message, never re-alert).
func (r *DatabaseReconciler) setDiskGrowthBlockedByQuota(ctx context.Context, db *appv1alpha1.Database, nextGB int32, quotaDetail string) error {
	message := fmt.Sprintf("disk autoscaling to %d GB is blocked by the namespace storage quota: %s; growth resumes when quota headroom is available (plan upgrade or freed storage)", nextGB, quotaDetail)
	prev := meta.FindStatusCondition(db.Status.Conditions, conditionDiskGrowthBlockedByQuota)
	wasBlocked := prev != nil && prev.Status == metav1.ConditionTrue
	statusBefore := db.DeepCopy()
	meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
		Type:               conditionDiskGrowthBlockedByQuota,
		Status:             metav1.ConditionTrue,
		Reason:             "ExceededNamespaceStorageQuota",
		Message:            message,
		ObservedGeneration: db.Generation,
	})
	if err := r.Status().Patch(ctx, db, client.MergeFrom(statusBefore)); err != nil {
		return fmt.Errorf("persist disk growth quota block: %w", err)
	}
	if !wasBlocked && r.Recorder != nil {
		r.Recorder.Eventf(db, "Warning", "DiskGrowthBlockedByQuota", "%s", message)
	}
	logf.FromContext(ctx).Info("database disk growth blocked by namespace quota", "name", db.Name, "toGB", nextGB, "quota", quotaDetail)
	return nil
}

// clearDiskGrowthBlockedByQuota removes the block condition once growth is no
// longer blocked; a no-op (no API write) when the condition is absent, so a
// healthy Database is never touched.
func (r *DatabaseReconciler) clearDiskGrowthBlockedByQuota(ctx context.Context, db *appv1alpha1.Database) error {
	if meta.FindStatusCondition(db.Status.Conditions, conditionDiskGrowthBlockedByQuota) == nil {
		return nil
	}
	statusBefore := db.DeepCopy()
	if meta.RemoveStatusCondition(&db.Status.Conditions, conditionDiskGrowthBlockedByQuota) {
		if err := r.Status().Patch(ctx, db, client.MergeFrom(statusBefore)); err != nil {
			return fmt.Errorf("clear disk growth quota block: %w", err)
		}
	}
	return nil
}

// recordDiskSampleFailure debounces transient/no-PVC startup noise. Only a
// Database already observed Ready accrues failures; the persisted annotation
// survives manager restarts. The Warning event fires once on the transition to
// the threshold and remains visible in kubectl describe.
func (r *DatabaseReconciler) recordDiskSampleFailure(ctx context.Context, db *appv1alpha1.Database, sampleErr error) error {
	if db.Status.Phase != appv1alpha1.DBPhaseReady {
		return nil
	}
	failures, _ := strconv.Atoi(db.Annotations[annotDiskSampleFailures])
	if failures >= databaseDiskSampleFailureLimit {
		return nil
	}
	failures++
	before := db.DeepCopy()
	metav1.SetMetaDataAnnotation(&db.ObjectMeta, annotDiskSampleFailures, strconv.Itoa(failures))
	if err := r.Patch(ctx, db, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("persist disk autoscale sample failure: %w", err)
	}
	if failures == databaseDiskSampleFailureLimit {
		if r.Recorder != nil {
			r.Recorder.Eventf(db, "Warning", "DiskAutoscalingSampleUnavailable",
				"could not sample Postgres disk usage for %d consecutive reconciles: %v",
				databaseDiskSampleFailureLimit, sampleErr)
		}
		logf.FromContext(ctx).Error(sampleErr, "database disk autoscaling sample persistently unavailable",
			"name", db.Name, "consecutiveFailures", failures)
	}
	return nil
}

func (r *DatabaseReconciler) resetDiskSampleFailures(ctx context.Context, db *appv1alpha1.Database) error {
	if db.Annotations == nil || db.Annotations[annotDiskSampleFailures] == "" {
		return nil
	}
	before := db.DeepCopy()
	delete(db.Annotations, annotDiskSampleFailures)
	if err := r.Patch(ctx, db, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("reset disk autoscale sample failures: %w", err)
	}
	return nil
}

func (r *DatabaseReconciler) syncLastDiskResizeStatus(db *appv1alpha1.Database) bool {
	annotations := db.Annotations
	if annotations == nil || annotations[annotDiskAutoscaleAt] == "" {
		return false
	}
	entry := appv1alpha1.DatabaseDiskResizeStatus{At: annotations[annotDiskAutoscaleAt]}
	from, errFrom := strconv.ParseInt(annotations[annotDiskAutoscaleFromGB], 10, 32)
	to, errTo := strconv.ParseInt(annotations[annotDiskAutoscaleToGB], 10, 32)
	used, errUsed := strconv.ParseInt(annotations[annotDiskAutoscaleUsed], 10, 64)
	capacity, errCapacity := strconv.ParseInt(annotations[annotDiskAutoscaleCapacity], 10, 64)
	if errFrom != nil || errTo != nil || errUsed != nil || errCapacity != nil || to <= from {
		return false
	}
	entry.FromGB = int32(from)
	entry.ToGB = int32(to)
	entry.UsedBytes = used
	entry.CapacityBytes = capacity
	for _, existing := range db.Status.DiskResizeHistory {
		if existing.At == entry.At && existing.ToGB == entry.ToGB {
			return false
		}
	}
	db.Status.DiskResizeHistory = append(db.Status.DiskResizeHistory, entry)
	if excess := len(db.Status.DiskResizeHistory) - databaseDiskResizeHistoryLimit; excess > 0 {
		db.Status.DiskResizeHistory = append([]appv1alpha1.DatabaseDiskResizeStatus(nil), db.Status.DiskResizeHistory[excess:]...)
	}
	return true
}
