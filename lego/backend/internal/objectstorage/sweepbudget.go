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

package objectstorage

import "time"

// The sweep's listing budget (ADR112 D8, w21/095).
//
// Wasabi counts the bytes a LIST reads as egress. Its documentation says "For
// utilization, a download also means reading any kind of data from storage
// servers or databases (metadata) that accounts for data READs" and names LIST
// as one of the operations (docs.wasabi.com, "Utilization: Seeing Downloads
// Without Performing a Data Download", updated 2026-02-17), and that its egress
// tool "Identifies GET (downloads) and LIST operations to measure egress bytes"
// ("Egress and Ingress Utilization: Calculating Using Bucket Logs", updated
// 2026-02-17). The free egress policy is monthly egress at or below the active
// storage volume, and the shared account stored 67 GB when measured on
// 2026-11-16 (ADR112 D4). A page of 1,000 keys is about 250 KB, so listing one
// object an hour costs 180 KB a month: one workspace at the 1,000,000-object cap
// listed every hour is about 180 GB a month, nearly three times the allowance by
// itself. A region is therefore listed less often the more objects it held when
// it was last listed, and the volume is measured (sweepmetrics.go) and alerted on
// (ObjectStorageSweepListingHigh).
const (
	// SweepHourlyObjects is how many objects a region may hold and still be
	// listed every hour. Each further SweepHourlyObjects adds an hour to the
	// wait, up to SweepMaxInterval, so a region is listed at most once an hour per
	// SweepHourlyObjects objects it held.
	SweepHourlyObjects int64 = 100

	// SweepMaxInterval is the longest a region goes unlisted, whatever its size:
	// the age of the usage the quota gate, `bucket info`, the storage meter and
	// the lifecycle rules (D5) work from is at most this plus one pass. It is
	// reached at 7,101 objects, and is what a workspace at the object cap is
	// listed at.
	SweepMaxInterval = 72 * time.Hour

	// SweepKeyBytes is the planning figure for what one listed object adds to a
	// ListObjectsV2 response: the S3 entry (193 bytes of XML) and a key of about 56
	// bytes, the length of a bex key <tea-id>/<obk-id>/k00000. One full page of
	// 1,000 such keys was 249,306 bytes from Wasabi's us-west-1 on 2026-11-16.
	SweepKeyBytes int64 = 250

	// SweepMonthlyBudgetBytes is the most listing the sweep may add to the shared
	// account's egress in a month at the planned scale: 10 GB, about 15% of the
	// 67 GB allowance measured on 2026-11-16. The egress alert fires at 30 GB
	// (warning) and 50 GB (critical) of what bex serves to tenants, so the three
	// together stay under the allowance (50 + 10 = 60 of 67).
	// ObjectStorageSweepListingHigh reads the same figure from the sweep's own
	// counter.
	SweepMonthlyBudgetBytes int64 = 10_000_000_000

	// listEntryOverheadBytes is the XML ListObjectsV2 wraps one object in (key
	// excluded) and listResponseOverheadBytes what a response carries beside its
	// entries, before the prefix it echoes: measured on Wasabi, 2026-11-16, and
	// held to a full page of 1,000 keys by TestListResponseBytesMatchesWasabi.
	listEntryOverheadBytes    = 193
	listResponseOverheadBytes = 250
)

// listResponseBytes estimates the body of the ListObjectsV2 response that named
// these objects under prefix, which is what Wasabi reads from its database for
// the listing and counts as egress. It is an estimate of the wire bytes, not a
// count: the response is never decompressed or measured here.
func listResponseBytes(prefix string, objects []ObjectInfo) int64 {
	n := int64(listResponseOverheadBytes + len(prefix))
	for _, o := range objects {
		n += int64(listEntryOverheadBytes + len(o.Key))
	}
	return n
}

// regionGap is how long a region whose last observation counted objects objects
// is left alone: MinGap up to HourlyObjects objects, one more hour for each
// further HourlyObjects, at most MaxGap. MinGap is 15 minutes under the pass
// interval, so a region observed at the end of one pass is due at the same
// moment of the next (a listing takes minutes), and the gap keeps that 15
// minutes however many hours it spans. A zero MinGap lists every pass, as the
// tests ask; a zero HourlyObjects or MaxGap leaves the scaling or the ceiling
// off.
func (sw *Sweeper) regionGap(objects int64) time.Duration {
	if sw.MinGap <= 0 {
		return 0
	}
	gap := sw.MinGap
	if sw.HourlyObjects > 0 && objects > sw.HourlyObjects {
		gap += time.Duration((objects-1)/sw.HourlyObjects) * SweepInterval
	}
	if sw.MaxGap > 0 {
		gap = min(gap, sw.MaxGap)
	}
	return gap
}
