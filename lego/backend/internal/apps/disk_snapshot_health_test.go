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
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func utcAt(day, hour, minute int) time.Time {
	return time.Date(2026, 10, day, hour, minute, 0, 0, time.UTC)
}

// The snapshot of the first 02:00–02:59 UTC window opening after the reference
// must exist by 10:00: the window, the 1h starting deadline and a 6h run.
func TestDiskSnapshotDueBy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ref, due time.Time
	}{
		{"created before the window", utcAt(21, 1, 0), utcAt(21, 10, 0)},
		{"created in the afternoon", utcAt(21, 14, 49), utcAt(22, 10, 0)},
		{"created at the window's start", utcAt(21, 2, 0), utcAt(22, 10, 0)},
		{"snapshot late in the window", utcAt(22, 2, 59), utcAt(23, 10, 0)},
		{"non-UTC reference", time.Date(2026, 10, 21, 20, 0, 0, 0, time.FixedZone("PDT", -7*3600)), utcAt(23, 10, 0)},
	} {
		if got := diskSnapshotDueBy(tc.ref); !got.Equal(tc.due) {
			t.Errorf("%s: due %v, want %v", tc.name, got, tc.due)
		}
	}
}

func newHealthService(t *testing.T, created time.Time, now time.Time, cond *metav1.Condition, objects ...DiskSnapshotObject) (*Service, string) {
	t.Helper()
	app := diskEligibleApp("web")
	app.Labels[core.LabelTenant] = "tea-snap"
	if cond != nil {
		app.Status.Conditions = []metav1.Condition{*cond}
	}
	svc, _, st := newDiskService(app)
	svc.DiskSnapshots = &fakeSnapshotLister{objects: objects}
	svc.SnapshotSecret = snapshotSecret
	svc.Base.Clock = func() time.Time { return now }
	view, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
	if err != nil {
		t.Fatal(err)
	}
	d := st.disks[view.ID]
	d.CreatedAt = created
	st.disks[view.ID] = d
	return svc, view.ID
}

func snapshotAt(ts time.Time) DiskSnapshotObject {
	return DiskSnapshotObject{Key: "tea-snap/web/" + ts.Format(time.RFC3339) + ".tar.gz.age", CreatedAt: ts}
}

func TestDiskSnapshotHealthStates(t *testing.T) {
	ctx := context.Background()
	t.Run("a new disk before its first window is ok", func(t *testing.T) {
		svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(22, 3, 29), nil)
		h, err := svc.DiskSnapshotHealth(ctx, id)
		if err != nil || h.State != DiskSnapshotHealthOK || h.LastSnapshotAt != nil || !h.DueBy.Equal(utcAt(22, 10, 0)) {
			t.Fatalf("health = %+v, %v; want ok, due 22 10:00", h, err)
		}
	})
	t.Run("a missed first window is overdue", func(t *testing.T) {
		// The w6/089 case: created 14:49, still nothing at 10:01 the next day.
		svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(22, 10, 1), nil)
		if h, _ := svc.DiskSnapshotHealth(ctx, id); h.State != DiskSnapshotHealthOverdue {
			t.Fatalf("state = %q, want overdue", h.State)
		}
	})
	t.Run("last night's snapshot keeps it ok, and the newest one is reported", func(t *testing.T) {
		svc, id := newHealthService(t, utcAt(1, 9, 0), utcAt(22, 12, 0), nil, snapshotAt(utcAt(21, 2, 40)), snapshotAt(utcAt(22, 2, 40)))
		h, _ := svc.DiskSnapshotHealth(ctx, id)
		if h.State != DiskSnapshotHealthOK || h.LastSnapshotAt == nil || !h.LastSnapshotAt.Equal(utcAt(22, 2, 40)) || !h.DueBy.Equal(utcAt(23, 10, 0)) {
			t.Fatalf("health = %+v, want ok with last 22 02:40, due 23 10:00", h)
		}
	})
	t.Run("a snapshot stopped two nights ago is overdue", func(t *testing.T) {
		svc, id := newHealthService(t, utcAt(1, 9, 0), utcAt(22, 10, 1), nil, snapshotAt(utcAt(20, 2, 40)))
		if h, _ := svc.DiskSnapshotHealth(ctx, id); h.State != DiskSnapshotHealthOverdue {
			t.Fatalf("state = %q, want overdue", h.State)
		}
	})
	t.Run("an operator scheduling failure is unavailable, with only its code", func(t *testing.T) {
		cond := &metav1.Condition{Type: appv1alpha1.ConditionDiskReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.DiskReasonSnapshotCredentialUnavailable, Message: "run scripts/disk-snapshot-secret.sh install"}
		svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(21, 15, 0), cond)
		h, _ := svc.DiskSnapshotHealth(ctx, id)
		if h.State != DiskSnapshotHealthUnavailable || h.Reason != "SnapshotCredentialUnavailable" {
			t.Fatalf("health = %+v, want unavailable SnapshotCredentialUnavailable", h)
		}
	})
	// A resize, or a restore that could not run, says nothing about the
	// nightly schedule.
	for _, reason := range []string{"DiskResizePending", "SnapshotStoreUnavailable", "DiskRestoreFailed"} {
		t.Run(reason+" does not mean snapshots stopped", func(t *testing.T) {
			cond := &metav1.Condition{Type: appv1alpha1.ConditionDiskReady, Status: metav1.ConditionFalse, Reason: reason}
			svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(21, 15, 0), cond)
			if h, _ := svc.DiskSnapshotHealth(ctx, id); h.State != DiskSnapshotHealthOK || h.Reason != "" {
				t.Fatalf("health = %+v, want ok", h)
			}
		})
	}
	t.Run("the operator's verdict survives a failing store listing", func(t *testing.T) {
		cond := &metav1.Condition{Type: appv1alpha1.ConditionDiskReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.DiskReasonSnapshotImageUnresolved}
		svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(21, 15, 0), cond)
		svc.DiskSnapshots = &fakeSnapshotLister{err: errors.New("bucket unreachable")}
		h, err := svc.DiskSnapshotHealth(ctx, id)
		if err != nil || h.State != DiskSnapshotHealthUnavailable {
			t.Fatalf("health = %+v, %v; want unavailable", h, err)
		}
	})
	t.Run("a failing listing is an error when there is no verdict", func(t *testing.T) {
		svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(21, 15, 0), nil)
		svc.DiskSnapshots = &fakeSnapshotLister{err: errors.New("bucket unreachable")}
		if _, err := svc.DiskSnapshotHealth(ctx, id); err == nil {
			t.Fatal("want the listing error, got nil")
		}
	})
}

func TestDiskSnapshotHealthWithoutAStoreIsNotConfigured(t *testing.T) {
	svc, id := newHealthService(t, utcAt(21, 14, 49), utcAt(22, 10, 1), nil)
	svc.DiskSnapshots = nil
	_, err := svc.DiskSnapshotHealth(context.Background(), id)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != DiskSnapshotsNotConfiguredCode {
		t.Fatalf("err = %v, want code %s", err, DiskSnapshotsNotConfiguredCode)
	}
}
