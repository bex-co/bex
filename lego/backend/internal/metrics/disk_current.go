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
	"time"
)

// currentDiskWindow is how far back a current read looks for a volume's
// newest kubelet sample: a few missed scrapes, not a volume long gone.
const currentDiskWindow = 10 * time.Minute

// CurrentDiskUsed is the used bytes of the fullest volume req matches, each
// at its newest sample in the ten minutes before now, from the series the
// Metrics page's disk graph reads. ok is false when no volume has a sample
// (not mounted yet, parked, or not scraped). req's window is set here.
func CurrentDiskUsed(ctx context.Context, src DiskUsageSource, req DiskUsageRequest, now time.Time) (used float64, ok bool, err error) {
	req.Start, req.End, req.Resolution = now.Add(-currentDiskWindow), now, time.Minute
	series, err := src(ctx, req)
	if err != nil {
		return 0, false, err
	}
	for _, s := range series {
		if value, found := newestPoint(s.Points); found && (!ok || value > used) {
			used, ok = value, true
		}
	}
	return used, ok, nil
}

// newestPoint is the value of the latest well-formed point.
func newestPoint(points []MetricPoint) (float64, bool) {
	var latest time.Time
	var value float64
	found := false
	for _, p := range points {
		at, err := time.Parse(time.RFC3339, p.Timestamp)
		if err == nil && (!found || at.After(latest)) {
			latest, value, found = at, p.Value, true
		}
	}
	return value, found
}

// DatabaseStorageUsed reads a Postgres's current used bytes through src: the
// fullest of its instance volumes, since an HA standby and each read replica
// hold a full copy and the fullest fills first (the operator's disk
// autoscaler reads the same, w6/m467). It backs GraphQL's storageUsed on
// Database, Render's almost-full warning (w3/m656).
func DatabaseStorageUsed(src DiskUsageSource) func(ctx context.Context, namespace, database string) (float64, bool, error) {
	return func(ctx context.Context, namespace, database string) (float64, bool, error) {
		return CurrentDiskUsed(ctx, src, DiskUsageRequest{
			Namespace:  namespace,
			Resource:   database,
			PVCPattern: pvcPattern(DatastoreDatabase, database),
		}, time.Now())
	}
}
