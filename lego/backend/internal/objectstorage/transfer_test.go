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

package objectstorage_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// recordingSink collects what the meter flushes.
type recordingSink struct {
	mu      sync.Mutex
	fail    error
	flushes int
	totals  map[[2]string]int64
}

func (r *recordingSink) AddObjectStorageUsage(_ context.Context, deltas []store.ObjectStorageUsageDelta) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushes++
	if r.fail != nil {
		return r.fail
	}
	if r.totals == nil {
		r.totals = map[[2]string]int64{}
	}
	for _, d := range deltas {
		r.totals[[2]string{d.WorkspaceID, d.Kind}] += d.Quantity
	}
	return nil
}

func (r *recordingSink) total(ws, kind string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totals[[2]string{ws, kind}]
}

// flushOnce runs the meter's loop on a context that is already done: it skips
// the loop's passes and does the final flush a shutdown does.
func flushOnce(s *objectstorage.Service) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.RunMeter(ctx, time.Hour)
}

func TestTransferMetersNeverFailOrBlockWithoutASink(t *testing.T) {
	var nilService *objectstorage.Service
	nilService.RecordIngress(context.Background(), "tea-a", 10)
	nilService.RecordEgress(context.Background(), "tea-a", 10)
	s := objectstorage.NewService(nil, nil)
	s.RecordIngress(context.Background(), "tea-a", 10)
	s.RecordEgress(context.Background(), "tea-a", 10)
	flushOnce(s) // returns at once: there is nothing to flush to
}

func TestTransferMetersSumPerWorkspaceAndMeterThenFlush(t *testing.T) {
	sink := &recordingSink{}
	s := objectstorage.NewService(nil, nil)
	s.SetUsageSink(sink)
	ctx := context.Background()
	s.RecordIngress(ctx, "tea-a", 100)
	s.RecordIngress(ctx, "tea-a", 23)
	s.RecordEgress(ctx, "tea-a", 7)
	s.RecordIngress(ctx, "tea-b", 5)
	// Nothing to count: a failed put, a zero-byte object, a negative number, no workspace.
	s.RecordIngress(ctx, "tea-a", 0)
	s.RecordIngress(ctx, "tea-a", -50)
	s.RecordEgress(ctx, "", 9)

	if sink.flushes != 0 {
		t.Fatalf("recording flushed %d times: it must only sum in memory", sink.flushes)
	}
	flushOnce(s)

	for key, want := range map[[2]string]int64{
		{"tea-a", store.UsageKindObjectStorageIngressBytes}: 123,
		{"tea-a", store.UsageKindObjectStorageEgressBytes}:  7,
		{"tea-b", store.UsageKindObjectStorageIngressBytes}: 5,
	} {
		if got := sink.total(key[0], key[1]); got != want {
			t.Errorf("%v = %d, want %d", key, got, want)
		}
	}
	if sink.total("", store.UsageKindObjectStorageEgressBytes) != 0 || len(sink.totals) != 3 {
		t.Errorf("flushed %v, want exactly the three counted sums", sink.totals)
	}
	// What was flushed is not flushed again.
	flushOnce(s)
	if got := sink.total("tea-a", store.UsageKindObjectStorageIngressBytes); got != 123 {
		t.Errorf("a second flush added to what was already written: %d", got)
	}
}

func TestTransferMetersKeepTheirSumsWhenTheDatabaseFails(t *testing.T) {
	sink := &recordingSink{fail: errors.New("database is down")}
	s := objectstorage.NewService(nil, nil)
	s.SetUsageSink(sink)
	s.RecordEgress(context.Background(), "tea-a", 40)
	flushOnce(s) // fails, and must not lose the bytes
	s.RecordEgress(context.Background(), "tea-a", 2)
	sink.mu.Lock()
	sink.fail = nil
	sink.mu.Unlock()
	flushOnce(s)
	if got := sink.total("tea-a", store.UsageKindObjectStorageEgressBytes); got != 42 {
		t.Fatalf("egress after a failed flush and a retry = %d, want 42", got)
	}
}

func TestTransferMetersAreOffAgainWithANilSink(t *testing.T) {
	sink := &recordingSink{}
	s := objectstorage.NewService(nil, nil)
	s.SetUsageSink(sink)
	s.SetUsageSink(nil)
	s.RecordEgress(context.Background(), "tea-a", 1)
	flushOnce(s)
	if sink.flushes != 0 {
		t.Fatalf("a service whose sink was cleared flushed %d times", sink.flushes)
	}
}

// Render's routes mint a URL and the bytes bypass bex, so the egress meter takes
// the object's size from the HEAD the mint makes (ADR112 D2): an estimate.
func TestMintGetRecordsTheObjectSizeAsEstimatedEgress(t *testing.T) {
	e := newSweepEnv(t, "meter-mintget")
	e.Svc.SetUsageSink(e.PG)
	ctx := context.Background()
	sys, err := e.Svc.SystemBucket(ctx, e.WS, objectstorage.Oregon)
	if err != nil {
		t.Fatal(err)
	}
	e.put(sys, "photo.jpg", 4096, time.Minute)

	if _, err := e.Svc.MintGet(ctx, e.WS, objectstorage.Oregon, "missing.jpg"); !errors.Is(err, objectstorage.ErrObjectNotFound) {
		t.Fatalf("MintGet of a missing key = %v", err)
	}
	for range 2 {
		if _, err := e.Svc.MintGet(ctx, e.WS, objectstorage.Oregon, "photo.jpg"); err != nil {
			t.Fatal(err)
		}
	}
	flushOnce(e.Svc)

	var egress, ingress int64
	err = e.Pool.QueryRow(ctx, `SELECT
		COALESCE(sum(quantity) FILTER (WHERE kind = 'object_storage_egress_bytes'), 0)::bigint,
		COALESCE(sum(quantity) FILTER (WHERE kind = 'object_storage_ingress_bytes'), 0)::bigint
		FROM usage_hourly WHERE workspace_id = $1 AND resource_kind = 'object_storage' AND service_id = $1`, e.WS).Scan(&egress, &ingress)
	if err != nil {
		t.Fatal(err)
	}
	if egress != 2*4096 || ingress != 0 {
		t.Fatalf("egress = %d, ingress = %d; want 8192 (two mints of a 4096-byte object, none for the missing key) and 0", egress, ingress)
	}
}
