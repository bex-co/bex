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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A pending disk restore is a lifecycle verb on the CR, not a row column. A
// resync must keep it, or the operator sees no restore two seconds into one and
// brings the service back up over the volume the restore Job is rewriting
// (w6/089, production 2026-10-24). The projector still owns the disk itself.
func TestApplyOwnedSpecKeepsAPendingDiskRestore(t *testing.T) {
	disk := func(size int32, restore string) *appv1alpha1.DiskSpec {
		return &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/lib/clickhouse", SizeGB: size, RestoreSnapshot: restore}
	}
	const object = "tea-x/app/2026-10-24T22:39:24Z.tar.gz.age"

	t.Run("an unchanged disk keeps the request and writes nothing", func(t *testing.T) {
		dst := appv1alpha1.AppSpec{Disk: disk(10, object)}
		if changed := applyOwnedSpec(&dst, appv1alpha1.AppSpec{Disk: disk(10, "")}); changed {
			t.Error("resync of an unchanged disk reported a change")
		}
		if dst.Disk.RestoreSnapshot != object {
			t.Fatalf("restoreSnapshot = %q, want the pending request kept", dst.Disk.RestoreSnapshot)
		}
	})
	t.Run("a grown disk converges and keeps the request", func(t *testing.T) {
		dst := appv1alpha1.AppSpec{Disk: disk(10, object)}
		if changed := applyOwnedSpec(&dst, appv1alpha1.AppSpec{Disk: disk(20, "")}); !changed {
			t.Error("a size change was not applied")
		}
		if dst.Disk.SizeGB != 20 || dst.Disk.RestoreSnapshot != object {
			t.Fatalf("disk = %+v, want size 20 with the request kept", *dst.Disk)
		}
	})
	t.Run("a detached disk clears everything", func(t *testing.T) {
		dst := appv1alpha1.AppSpec{Disk: disk(10, object)}
		if changed := applyOwnedSpec(&dst, appv1alpha1.AppSpec{}); !changed || dst.Disk != nil {
			t.Fatalf("disk = %+v, changed=%v; want it removed", dst.Disk, changed)
		}
	})
	t.Run("an attached disk starts with no request", func(t *testing.T) {
		dst := appv1alpha1.AppSpec{}
		if changed := applyOwnedSpec(&dst, appv1alpha1.AppSpec{Disk: disk(10, "")}); !changed || dst.Disk == nil || dst.Disk.RestoreSnapshot != "" {
			t.Fatalf("disk = %+v, changed=%v; want the new disk with no restore", dst.Disk, changed)
		}
	})
}
