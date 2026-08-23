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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/snapshotticket"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeSnapshotLister struct {
	objects []DiskSnapshotObject
	err     error
	prefix  string // captured, so a test can assert what was listed
}

func (f *fakeSnapshotLister) ListSnapshots(_ context.Context, prefix string) ([]DiskSnapshotObject, error) {
	f.prefix = prefix
	return f.objects, f.err
}

var snapshotSecret = []byte("disk-snapshot-signing-key")

func newSnapshotService(objects ...DiskSnapshotObject) (*Service, *fakeSnapshotLister, string) {
	app := diskEligibleApp("web")
	app.Labels[core.LabelTenant] = "tea-snap"
	lister := &fakeSnapshotLister{objects: objects}
	svc, _, _ := newDiskService(app)
	svc.DiskSnapshots = lister
	svc.SnapshotSecret = snapshotSecret
	view, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
	if err != nil {
		panic(err)
	}
	return svc, lister, view.ID
}

func snapshotObject(day int) DiskSnapshotObject {
	at := time.Date(2026, 8, day, 2, 0, 0, 0, time.UTC)
	return DiskSnapshotObject{
		Key:       "tea-snap/web/" + at.Format(time.RFC3339) + ".tar.gz.age",
		CreatedAt: at,
	}
}

func TestListDiskSnapshotsReturnsRendersShapeNewestFirst(t *testing.T) {
	svc, lister, diskID := newSnapshotService(snapshotObject(1), snapshotObject(3), snapshotObject(2))

	views, err := svc.ListDiskSnapshots(context.Background(), diskID)
	if err != nil {
		t.Fatalf("ListDiskSnapshots: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("got %d snapshots, want 3", len(views))
	}
	// Newest first — the one a caller almost always wants is the last taken.
	if !views[0].CreatedAt.After(views[1].CreatedAt) || !views[1].CreatedAt.After(views[2].CreatedAt) {
		t.Fatalf("snapshots are not newest-first: %v %v %v", views[0].CreatedAt, views[1].CreatedAt, views[2].CreatedAt)
	}
	if views[0].CreatedAt != time.Date(2026, 8, 3, 2, 0, 0, 0, time.UTC) {
		t.Fatalf("createdAt = %v, want the time encoded in the object name", views[0].CreatedAt)
	}
	if views[0].SnapshotKey == "" || views[0].InstanceID == "" {
		t.Fatalf("view = %+v, want Render's snapshotKey and instanceId populated", views[0])
	}
	// The listing must be scoped to this disk's own prefix.
	if lister.prefix != "tea-snap/web/" {
		t.Fatalf("listed prefix %q, want this disk's own", lister.prefix)
	}
}

// A stray object in the same prefix must never be offered as restorable.
func TestListDiskSnapshotsIgnoresNonSnapshotObjects(t *testing.T) {
	svc, _, diskID := newSnapshotService(
		snapshotObject(1),
		DiskSnapshotObject{Key: "tea-snap/web/README.txt", CreatedAt: time.Now()},
	)

	views, err := svc.ListDiskSnapshots(context.Background(), diskID)
	if err != nil {
		t.Fatalf("ListDiskSnapshots: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d snapshots, want only the real one", len(views))
	}
}

func TestRestoreRequestsTheSnapshotOnTheSpec(t *testing.T) {
	svc, _, diskID := newSnapshotService(snapshotObject(1))
	views, err := svc.ListDiskSnapshots(context.Background(), diskID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if err := svc.RestoreDiskSnapshot(context.Background(), diskID, views[0].SnapshotKey); err != nil {
		t.Fatalf("RestoreDiskSnapshot: %v", err)
	}
	stored := &appv1alpha1.App{}
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, stored); err != nil {
		t.Fatalf("get app: %v", err)
	}
	if stored.Spec.Disk.RestoreSnapshot != snapshotObject(1).Key {
		t.Fatalf("spec.disk.restoreSnapshot = %q, want the requested object", stored.Spec.Disk.RestoreSnapshot)
	}
}

// Every rejection has to happen BEFORE the spec is touched: a bad key must not
// stop the service, because stopping it is the first step of a restore.
func TestRestoreRefusesBadKeysWithoutTouchingTheService(t *testing.T) {
	now := time.Now().UTC()
	expired, err := snapshotticket.Mint(snapshotSecret, "dsk-whatever", "tea-snap/web/old.tar.gz.age", now.Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	foreign, err := snapshotticket.Mint(snapshotSecret, "dsk-someone-else", "tea-other/app/x.tar.gz.age", now)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	forged, err := snapshotticket.Mint([]byte("not-our-secret"), "dsk-1", "tea-snap/web/x.tar.gz.age", now)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	for name, key := range map[string]string{
		"expired": expired, "another disk": foreign, "forged": forged, "empty": "", "garbage": "nonsense",
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, diskID := newSnapshotService(snapshotObject(1))

			err := svc.RestoreDiskSnapshot(context.Background(), diskID, key)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("RestoreDiskSnapshot(%s) = %v, want bad request", name, err)
			}
			stored := &appv1alpha1.App{}
			if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, stored); err != nil {
				t.Fatalf("get app: %v", err)
			}
			if stored.Spec.Disk.RestoreSnapshot != "" {
				t.Fatalf("a refused restore still requested %q", stored.Spec.Disk.RestoreSnapshot)
			}
		})
	}
}

// With no store configured the verbs must say so, not panic or pretend the disk
// has no snapshots (which would read as "nothing to restore").
func TestSnapshotVerbsReportUnavailableWithoutAStore(t *testing.T) {
	app := diskEligibleApp("web")
	svc, _, _ := newDiskService(app)
	view, err := svc.AddDisk(context.Background(), "web", "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("AddDisk: %v", err)
	}

	if _, err := svc.ListDiskSnapshots(context.Background(), view.ID); !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("ListDiskSnapshots = %v, want unavailable", err)
	}
	if err := svc.RestoreDiskSnapshot(context.Background(), view.ID, "any"); !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("RestoreDiskSnapshot = %v, want unavailable", err)
	}
}
