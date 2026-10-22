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
	"context"
	"time"

	"github.com/graphql-go/graphql"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Disk snapshot health states (w6/m152). An empty snapshot list cannot tell
// "the first nightly window has not come yet" from "snapshots are not
// running", so the dashboard asks for this verdict alongside the list.
const (
	DiskSnapshotHealthOK          = "ok"
	DiskSnapshotHealthOverdue     = "overdue"
	DiskSnapshotHealthUnavailable = "unavailable"
)

// diskSnapshotDueAfterWindowStart is the latest a nightly snapshot can land
// after its window opens: the hour-long window, the CronJob's starting
// deadline, then a full run. Anything later did not happen.
const diskSnapshotDueAfterWindowStart = time.Hour + appv1alpha1.DiskBackupStartingDeadline + appv1alpha1.DiskBackupActiveDeadline

// DiskSnapshotHealth is whether a disk's nightly snapshots are happening.
type DiskSnapshotHealth struct {
	State string
	// Reason is the operator's DiskReady reason code when State is
	// unavailable; never its message, which names operator internals.
	Reason         string
	LastSnapshotAt *time.Time
	// DueBy is when the next snapshot should exist; zero when unavailable and
	// the store could not be listed.
	DueBy time.Time
}

// DiskSnapshotHealth reports whether a disk's nightly snapshots are being
// taken: unavailable when the operator could not schedule them, overdue when
// the first nightly snapshot due after the newer of the last snapshot and the
// disk's creation has not landed, else ok.
func (s *Service) DiskSnapshotHealth(ctx context.Context, diskID string) (DiskSnapshotHealth, error) {
	disk, a, err := s.authorizeDiskSnapshots(ctx, diskID)
	if err != nil {
		return DiskSnapshotHealth{}, err
	}
	health := DiskSnapshotHealth{State: DiskSnapshotHealthOK}
	if reason, ok := diskSnapshotUnavailableReason(a); ok {
		health.State, health.Reason = DiskSnapshotHealthUnavailable, reason
	}
	objects, err := s.diskSnapshotObjects(ctx, a)
	if err != nil {
		// The operator's verdict stands on its own; only overdue needs the list.
		if health.State == DiskSnapshotHealthUnavailable {
			return health, nil
		}
		return DiskSnapshotHealth{}, err
	}
	for _, obj := range objects {
		if at := snapshotCreatedAt(obj); health.LastSnapshotAt == nil || at.After(*health.LastSnapshotAt) {
			health.LastSnapshotAt = &at
		}
	}
	reference := disk.CreatedAt
	if health.LastSnapshotAt != nil && health.LastSnapshotAt.After(reference) {
		reference = *health.LastSnapshotAt
	}
	health.DueBy = diskSnapshotDueBy(reference)
	if health.State == DiskSnapshotHealthOK && s.Now().After(health.DueBy) {
		health.State = DiskSnapshotHealthOverdue
	}
	return health, nil
}

// diskSnapshotDueBy is when the snapshot of the first nightly window opening
// after ref must exist.
func diskSnapshotDueBy(ref time.Time) time.Time {
	ref = ref.UTC()
	start := time.Date(ref.Year(), ref.Month(), ref.Day(), appv1alpha1.DiskBackupHourUTC, 0, 0, 0, time.UTC)
	if !start.After(ref) {
		start = start.AddDate(0, 0, 1)
	}
	return start.Add(diskSnapshotDueAfterWindowStart)
}

// diskSnapshotUnavailableReason reads the operator's verdict that it could not
// schedule this disk's snapshots. Other DiskReady reasons, including a failed
// restore's, say nothing about the nightly schedule.
func diskSnapshotUnavailableReason(a *appv1alpha1.App) (string, bool) {
	cond := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionDiskReady)
	if cond == nil || cond.Status != metav1.ConditionFalse {
		return "", false
	}
	switch cond.Reason {
	case appv1alpha1.DiskReasonSnapshotImageUnresolved, appv1alpha1.DiskReasonSnapshotCredentialUnavailable:
		return cond.Reason, true
	}
	return "", false
}

var diskSnapshotHealthGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "DiskSnapshotHealth",
	Description: "Whether a disk's nightly snapshots are being taken (bex extension).",
	Fields: graphql.Fields{
		"state":          gqlutil.StrField(func(v DiskSnapshotHealth) any { return v.State }),
		"reason":         gqlutil.StrField(func(v DiskSnapshotHealth) any { return v.Reason }),
		"lastSnapshotAt": gqlutil.TimeField(func(v DiskSnapshotHealth) any { return v.LastSnapshotAt }),
		"dueBy":          gqlutil.TimeField(func(v DiskSnapshotHealth) any { return v.DueBy }),
	},
})
