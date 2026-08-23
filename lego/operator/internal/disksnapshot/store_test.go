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

package disksnapshot

import (
	"testing"
	"time"
)

// Snapshot names must sort lexically in time order: List and Prune decide what
// to delete from names alone, so an ordering that disagreed with time would
// delete the newest snapshot instead of the oldest.
func TestSnapshotKeysSortInTimeOrder(t *testing.T) {
	prefix := DiskPrefix("tea-1", "dsk-1")
	times := []time.Time{
		time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC),
		time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	var previous string
	for _, at := range times {
		key := SnapshotKey(prefix, at)
		if previous != "" && key <= previous {
			t.Fatalf("key %q does not sort after %q", key, previous)
		}
		previous = key
	}
}

// A non-UTC clock must not produce a key that sorts out of order against UTC
// keys taken moments earlier.
func TestSnapshotKeyNormalizesToUTC(t *testing.T) {
	zone := time.FixedZone("UTC+9", 9*3600)
	at := time.Date(2026, 5, 1, 12, 0, 0, 0, zone)
	key := SnapshotKey(DiskPrefix("tea-1", "dsk-1"), at)
	if want := DiskPrefix("tea-1", "dsk-1") + "2026-05-01T03:00:00Z" + SnapshotSuffix; key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
}

// Each disk's snapshots live under their own workspace/disk path, so a purge
// can never reach another tenant's objects.
func TestDiskPrefixIsScopedPerWorkspaceAndDisk(t *testing.T) {
	a := DiskPrefix("tea-1", "dsk-1")
	for _, other := range []string{
		DiskPrefix("tea-1", "dsk-2"),
		DiskPrefix("tea-2", "dsk-1"),
	} {
		if a == other {
			t.Fatalf("prefixes collide: %q", a)
		}
		if len(other) >= len(a) && other[:len(a)] == a {
			t.Fatalf("prefix %q contains %q; a purge would reach across", other, a)
		}
	}
}

func TestStoreJoinAppliesThePrefixExactlyOnce(t *testing.T) {
	for _, tc := range []struct{ prefix, key, want string }{
		{"", "a/b", "a/b"},
		{"snapshots", "a/b", "snapshots/a/b"},
		{"/snapshots/", "/a/b", "snapshots/a/b"},
		{"snapshots", "", "snapshots/"},
	} {
		if got := (Store{Prefix: tc.prefix}).join(tc.key); got != tc.want {
			t.Errorf("join(%q, %q) = %q, want %q", tc.prefix, tc.key, got, tc.want)
		}
	}
}

// Prune with a retain count below one would delete every snapshot a disk has;
// refusing it is cheaper than discovering it in a bucket.
func TestPruneRefusesARetainCountThatWouldDeleteEverything(t *testing.T) {
	for _, retain := range []int{0, -1} {
		if _, err := (Store{Bucket: "b"}).Prune(t.Context(), "p/", retain); err == nil {
			t.Errorf("Prune accepted retain=%d", retain)
		}
	}
}
