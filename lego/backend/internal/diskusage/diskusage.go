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

// Package diskusage watches service disks fill up, as Render does: "A
// service's persistent disk usage exceeds 80% of the disk's allocated space"
// and "drops back below 80%" are both events (service_disk_usage_high /
// service_disk_usage_recovered) and emails (render.com/docs/notifications).
// Usage is the kubelet's used bytes for the disk's volume against the disk's
// own size (not the larger physical volume, ADR082). A crossing flips only
// when two consecutive sweeps agree, so a disk hovering at the threshold does
// not flap (w2/m255).
package diskusage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ThresholdPercent is Render's 80%.
const ThresholdPercent = 80.0

// Store is what the sweeper persists through; *store.PGStore satisfies it.
type Store interface {
	ListDisks(ctx context.Context, tenantID, appID string) ([]store.Disk, error)
	GetDiskUsageState(ctx context.Context, diskID string) (store.DiskUsageState, error)
	SetDiskUsagePending(ctx context.Context, diskID string, pendingHigh *bool, at time.Time) error
	RecordDiskUsageCrossing(ctx context.Context, fact store.ServiceEventFact) (bool, error)
}

// Notifier emails a disk crossing.
type Notifier interface {
	NotifyDiskUsage(ctx context.Context, tenantID, serviceID, name string, high bool, d store.DiskUsageDetails, sourceKey string, at time.Time) error
}

// Sweeper evaluates every service disk each Interval.
type Sweeper struct {
	Apps     client.Client
	Store    Store
	Usage    metrics.DiskUsageSource
	Notifier Notifier
	Clock    func() time.Time
	// Interval between sweeps; non-positive means five minutes.
	Interval time.Duration
}

// Run sweeps until ctx is canceled; an unwired sweeper is a no-op.
func (w *Sweeper) Run(ctx context.Context) {
	if w == nil || w.Apps == nil || w.Store == nil || w.Usage == nil {
		return
	}
	core.Poll(ctx, "service disk usage", w.interval(), w.SweepOnce)
}

func (w *Sweeper) interval() time.Duration {
	if w.Interval > 0 {
		return w.Interval
	}
	return 5 * time.Minute
}

func (w *Sweeper) now() time.Time {
	if w.Clock != nil {
		return w.Clock()
	}
	return time.Now()
}

// SweepOnce evaluates every disk of every running service.
func (w *Sweeper) SweepOnce(ctx context.Context) error {
	var list appv1alpha1.AppList
	if err := w.Apps.List(ctx, &list); err != nil {
		return fmt.Errorf("list apps: %w", err)
	}
	now := w.now()
	var errs []error
	for i := range list.Items {
		a := &list.Items[i]
		if a.Spec.Disk == nil || a.Spec.Suspended || a.Status.Phase == appv1alpha1.PhaseHibernated {
			continue
		}
		if err := w.evaluate(ctx, a, now); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", a.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (w *Sweeper) evaluate(ctx context.Context, a *appv1alpha1.App, now time.Time) error {
	tenantID, appID := a.Labels[core.LabelTenant], store.ManagedAppID(a.Labels)
	disks, err := w.Store.ListDisks(ctx, tenantID, appID)
	if err != nil {
		return err
	}
	var disk *store.Disk
	for i := range disks {
		if disks[i].DeletedAt == nil {
			disk = &disks[i]
			break
		}
	}
	if disk == nil || disk.SizeGB <= 0 {
		return nil
	}
	used, ok, err := w.usedBytes(ctx, a, now)
	if err != nil || !ok {
		return err // no series: the volume is not mounted, nothing to judge
	}
	percent := used / (float64(disk.SizeGB) * metrics.BytesPerGiB) * 100
	high := percent >= ThresholdPercent
	state, err := w.Store.GetDiskUsageState(ctx, disk.ID)
	if err != nil {
		return err
	}
	if high == state.High {
		if state.PendingHigh != nil {
			return w.Store.SetDiskUsagePending(ctx, disk.ID, nil, now)
		}
		return nil
	}
	// A sighting older than two sweeps is stale (the service was parked, or
	// its series missing, in between): start again rather than confirm.
	if state.PendingHigh == nil || *state.PendingHigh != high || state.PendingSince == nil ||
		now.Sub(*state.PendingSince) > 2*w.interval() {
		if state.PendingHigh != nil && *state.PendingHigh == high {
			if err := w.Store.SetDiskUsagePending(ctx, disk.ID, nil, now); err != nil {
				return err
			}
		}
		return w.Store.SetDiskUsagePending(ctx, disk.ID, &high, now)
	}
	// A second sweep saw the same crossing: confirm it.
	details := store.DiskUsageDetails{
		DiskID: disk.ID, DiskName: disk.Name, MountPath: disk.MountPath,
		UsagePercent: roundTenth(percent), ThresholdPercent: ThresholdPercent,
	}
	typ := store.EventFactServiceDiskUsageRecovered
	if high {
		typ = store.EventFactServiceDiskUsageHigh
	}
	sourceKey := fmt.Sprintf("%s:%s:%d", typ, disk.ID, state.PendingSince.Unix())
	serviceID := core.AppPublicID(a)
	if w.Notifier != nil {
		// Mailed before the state flips: the alert is keyed by the crossing, so
		// a failed write retries without a second email, and a failed email is
		// retried before the crossing is recorded. Accepted residual: if the
		// write keeps failing and usage returns before it succeeds, the email
		// has no event in the feed.
		if err := w.Notifier.NotifyDiskUsage(ctx, tenantID, serviceID, core.AppPublicName(a), high, details, sourceKey, now); err != nil {
			return fmt.Errorf("notify: %w", err)
		}
	}
	_, err = w.Store.RecordDiskUsageCrossing(ctx, store.ServiceEventFact{
		SourceKey: sourceKey, AppID: appID, Type: typ, At: now, DiskUsage: &details,
	})
	return err
}

// usedBytes is the volume's most recent used-bytes sample within the last ten
// minutes.
func (w *Sweeper) usedBytes(ctx context.Context, a *appv1alpha1.App, now time.Time) (float64, bool, error) {
	series, err := w.Usage(ctx, metrics.DiskUsageRequest{
		Namespace:  a.Namespace,
		Resource:   a.Name,
		PVCPattern: metrics.ServiceDiskPVCPattern(a.Name),
		Start:      now.Add(-10 * time.Minute),
		End:        now,
		Resolution: time.Minute,
	})
	if err != nil {
		return 0, false, err
	}
	var latest time.Time
	var value float64
	found := false
	for _, s := range series {
		for _, p := range s.Points {
			at, err := time.Parse(time.RFC3339, p.Timestamp)
			if err == nil && (!found || at.After(latest)) {
				latest, value, found = at, p.Value, true
			}
		}
	}
	return value, found, nil
}

func roundTenth(v float64) float64 { return math.Round(v*10) / 10 }
