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

package s3gw

import (
	"context"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// batchSpy is an audit sink that takes batches, as the control-plane store does.
type batchSpy struct {
	batches [][]core.AuditEvent
	singles []core.AuditEvent
}

func (b *batchSpy) Record(_ context.Context, ev core.AuditEvent) error {
	b.singles = append(b.singles, ev)
	return nil
}

func (b *batchSpy) RecordAll(_ context.Context, evs []core.AuditEvent) error {
	b.batches = append(b.batches, evs)
	return nil
}

func TestTheDenialDedupeFiltersABatchOfRowsAsItFiltersOne(t *testing.T) {
	spy := &batchSpy{}
	now := time.Now()
	d := &denialDedupe{inner: spy, once: newOnceWindow(), now: func() time.Time { return now }}
	denied := core.AuditEvent{Outcome: core.AuditDenied, CallerMethod: core.MethodS3Key, Caller: "s3k-a", Verb: "objectstorage.PutObject", Target: "bucket/obj"}
	allowed := core.AuditEvent{Outcome: core.AuditAllowed, CallerMethod: core.MethodS3Key, Caller: "s3k-a", Verb: "objectstorage.PutObject", Target: "bucket/obj"}
	person := core.AuditEvent{Outcome: core.AuditDenied, Caller: "usr-a", Verb: "objectstorage.PutObject", Target: "bucket/obj"}
	if err := d.RecordAll(context.Background(), []core.AuditEvent{denied, denied, allowed, allowed, person, person}); err != nil {
		t.Fatal(err)
	}
	if len(spy.batches) != 1 || len(spy.batches[0]) != 5 {
		t.Fatalf("the batch sink got %v, want one batch of the first denied row, both allowed rows and both person rows (5)", spy.batches)
	}
	// The window holds: the same denial again, in a row or a batch, is dropped.
	_ = d.Record(context.Background(), denied)
	_ = d.RecordAll(context.Background(), []core.AuditEvent{denied})
	if len(spy.singles) != 0 || len(spy.batches[1]) != 0 {
		t.Errorf("a repeated denial inside the window reached the sink: %v, batch %v", spy.singles, spy.batches[1])
	}
	now = now.Add(AuditWindow + time.Minute)
	_ = d.Record(context.Background(), denied)
	if len(spy.singles) != 1 {
		t.Errorf("the denial after the window = %d rows, want one", len(spy.singles))
	}
}
