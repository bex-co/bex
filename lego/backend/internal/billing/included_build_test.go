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

package billing

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m308 (docs/ADR030 §13): Stripe receives only the build minutes above the
// plan's included pipeline minutes.

// buildBeforeStore answers UsageBefore from the rows it was given, in the
// store's (window, resource kind, service) order, per meter kind.
type buildBeforeStore struct {
	fakeEmitterStore
	month []store.HourlyRow
	err   error
	reads int
}

func (b *buildBeforeStore) UsageBefore(_ context.Context, r store.HourlyRow) (int64, error) {
	b.reads++
	if b.err != nil {
		return 0, b.err
	}
	var total int64
	for _, m := range b.month {
		if m.WorkspaceID == r.WorkspaceID && m.Kind == r.Kind && (m.WindowStart.Before(r.WindowStart) ||
			(m.WindowStart.Equal(r.WindowStart) && m.ServiceID < r.ServiceID)) {
			total += m.Quantity
		}
	}
	return total, nil
}

// planReader answers every workspace with plan, or err.
type planReader struct {
	plan string
	err  error
}

func (p planReader) WorkspacePlanOf(context.Context, string) (string, bool, error) {
	return p.plan, p.err == nil, p.err
}

func TestOwedAboveSplitsTheRowAtTheAllowance(t *testing.T) {
	for _, tc := range []struct{ before, qty, allowance, want int64 }{
		{0, 100, 300, 0},     // inside the allowance
		{250, 100, 300, 50},  // straddles it
		{400, 100, 300, 100}, // past it
		{300, 0, 300, 0},
	} {
		if got := owedAbove(tc.before, tc.qty, tc.allowance); got != tc.want {
			t.Errorf("owedAbove(%d, %d, %d) = %d, want %d", tc.before, tc.qty, tc.allowance, got, tc.want)
		}
	}
}

func TestEmitterSendsOnlyBuildMinutesAboveTheAllowance(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	// Hobby includes 500 minutes, 30,000 seconds.
	inside := row("tea-a", "srv-1", store.UsageKindBuildSeconds, "", "service", 20000, w)
	straddle := row("tea-a", "srv-1", store.UsageKindBuildSeconds, "", "service", 20000, w.Add(time.Hour))
	past := row("tea-a", "srv-2", store.UsageKindBuildSeconds, "", "service", 5000, w.Add(time.Hour))
	compute := row("tea-a", "srv-1", store.UsageKindInstanceSeconds, "starter", "service", 3600, w)
	st := &buildBeforeStore{month: []store.HourlyRow{inside, straddle, past}}
	st.queue = [][]store.HourlyRow{{inside, straddle, past, compute}}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now)
	e.Plans = planReader{plan: "hobby"} // 500 included minutes

	e.emitOnce(context.Background(), false)

	at := func(r store.HourlyRow) string { return r.ServiceID + "@" + r.WindowStart.Format(time.RFC3339) }
	sent := map[string]string{}
	for _, ev := range ing.allEvents() {
		if name, value, skip := stripeMeterEvent(ev); !skip {
			sent[ev.Properties["service_id"]+"@"+ev.Timestamp.Format(time.RFC3339)+"/"+name] = value
		}
	}
	if _, ok := sent[at(inside)+"/build_seconds"]; ok {
		t.Error("an hour the allowance covers must not reach Stripe")
	}
	if got := sent[at(straddle)+"/build_seconds"]; got != "10000" {
		t.Errorf("straddling hour sent %q, want 10000 (20000 used, 10000 still inside the 30000)", got)
	}
	if got := sent[at(past)+"/build_seconds"]; got != "5000" {
		t.Errorf("hour past the allowance sent %q, want all 5000", got)
	}
	if got := sent[at(compute)+"/instance_seconds.service.starter"]; got != "3600" {
		t.Errorf("compute sent %q; the allowance must touch build minutes only", got)
	}
	billed := map[string]int64{}
	for _, batch := range st.attempts {
		for _, a := range batch {
			if a.BilledQuantity != nil {
				billed[at(a.Row)] = *a.BilledQuantity
			}
		}
	}
	if b, ok := billed[at(inside)]; !ok || b != 0 || billed[at(straddle)] != 10000 || billed[at(past)] != 5000 {
		t.Errorf("recorded billed quantities = %v, want inside 0, straddle 10000 and past 5000 (every build row frozen)", billed)
	}
	if st.stampedCount() != 4 {
		t.Errorf("stamped %d rows, want all 4 (a covered hour is accounted locally)", st.stampedCount())
	}
}

func TestEmitterHoldsBuildRowsWhenTheAllowanceCannotBeRead(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	build := row("tea-a", "srv-1", store.UsageKindBuildSeconds, "", "service", 200, now.Add(-72*time.Hour))
	build2 := row("tea-a", "srv-2", store.UsageKindBuildSeconds, "", "service", 300, now.Add(-72*time.Hour))
	for name, setup := range map[string]func(*Emitter, *buildBeforeStore){
		"plan read fails": func(e *Emitter, _ *buildBeforeStore) {
			e.Plans = planReader{err: errors.New("db down")}
		},
		"earlier minutes fail": func(e *Emitter, st *buildBeforeStore) {
			e.Plans = planReader{plan: "hobby"}
			st.err = errors.New("db down")
		},
	} {
		st := &buildBeforeStore{}
		st.queue = [][]store.HourlyRow{{build, build2}}
		ing := &fakeIngester{}
		e := newEmitter(st, ing, now)
		setup(e, st)
		e.emitOnce(context.Background(), false)
		if len(ing.allEvents()) != 0 || st.stampedCount() != 0 {
			t.Errorf("%s: the row was sent (%d events) or stamped (%d); it must wait, never go out whole", name, len(ing.allEvents()), st.stampedCount())
		}
	}
}

func TestEmitterReusesAFrozenBuildQuantityAndSkipsZeroRows(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	frozen := row("tea-a", "srv-1", store.UsageKindBuildSeconds, "", "service", 40000, w)
	first := int64(1234)
	frozen.BilledQuantity = &first // an earlier attempt sent this
	zero := row("tea-a", "srv-2", store.UsageKindBuildSeconds, "", "service", 0, w)
	st := &buildBeforeStore{err: errors.New("must not be read")}
	st.queue = [][]store.HourlyRow{{frozen, zero}}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now)
	e.Plans = planReader{plan: "pro"}

	e.emitOnce(context.Background(), false)

	var sentValue string
	for _, ev := range ing.allEvents() {
		if name, value, skip := stripeMeterEvent(ev); !skip && name == "build_seconds" {
			sentValue = value
		}
	}
	if sentValue != "1234" {
		t.Errorf("retry sent %q, want the frozen 1234 whatever the plan is now", sentValue)
	}
	if st.stampedCount() != 2 {
		t.Errorf("stamped %d rows, want both: a zero row needs no earlier-minutes read", st.stampedCount())
	}
}

// w6/m319: bandwidth follows the same split, in bytes, beside build minutes.
func TestEmitterSendsOnlyBandwidthAboveTheAllowance(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	// Hobby includes 5 GB, in the 2^30-byte GB egress is billed in.
	first := row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 4<<30, w)
	second := row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 2<<30, w.Add(time.Hour))
	build := row("tea-a", "srv-1", store.UsageKindBuildSeconds, "", "service", 600, w.Add(time.Hour))
	st := &buildBeforeStore{month: []store.HourlyRow{first, second, build}}
	st.queue = [][]store.HourlyRow{{first, second, build}}
	ing := &fakeIngester{}
	e := newEmitter(st, ing, now)
	e.Plans = planReader{plan: "hobby"}

	e.emitOnce(context.Background(), false)

	billed := map[string]int64{}
	for _, batch := range st.attempts {
		for _, a := range batch {
			if a.BilledQuantity != nil {
				billed[a.Row.Kind+"@"+a.Row.WindowStart.Format(time.RFC3339)] = *a.BilledQuantity
			}
		}
	}
	at := func(r store.HourlyRow) string { return r.Kind + "@" + r.WindowStart.Format(time.RFC3339) }
	if billed[at(first)] != 0 || billed[at(second)] != 1<<30 {
		t.Errorf("egress billed = %v, want the first hour covered and 1 GB of the second", billed)
	}
	if billed[at(build)] != 0 {
		t.Errorf("build billed %d; 600 s is inside Hobby's 500 minutes, whatever the egress", billed[at(build)])
	}
	for _, ev := range ing.allEvents() {
		if name, value, skip := stripeMeterEvent(ev); !skip && name == "egress_gib" && ev.Timestamp.Equal(first.WindowStart) {
			t.Errorf("a covered egress hour reached Stripe (%s)", value)
		}
	}
}

// Once a batch sees the allowance used up, rows of later hours owe their whole
// quantity without a read; a row of the same hour still reads, and the result
// is what reading every row would have sent.
func TestEmitterSkipsReadsAfterTheAllowanceIsSpent(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	w := now.Add(-72 * time.Hour)
	rows := []store.HourlyRow{
		row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 6<<30, w),                // spends Hobby's 5 GB
		row("tea-a", "srv-2", store.UsageKindEgressBytes, "", "service", 1<<30, w),                // same hour: reads
		row("tea-a", "srv-1", store.UsageKindEgressBytes, "", "service", 2<<30, w.Add(time.Hour)), // later: whole
		row("tea-a", "srv-2", store.UsageKindEgressBytes, "", "service", 3<<30, w.Add(2*time.Hour)),
	}
	st := &buildBeforeStore{month: rows}
	st.queue = [][]store.HourlyRow{rows}
	e := newEmitter(st, &fakeIngester{}, now)
	e.Plans = planReader{plan: "hobby"}

	e.emitOnce(context.Background(), false)

	if st.reads != 2 {
		t.Errorf("UsageBefore reads = %d, want 2: the spending hour's rows only", st.reads)
	}
	var billed []int64
	for _, batch := range st.attempts {
		for _, a := range batch {
			billed = append(billed, *a.BilledQuantity)
		}
	}
	want := []int64{1 << 30, 1 << 30, 2 << 30, 3 << 30}
	if fmt.Sprint(billed) != fmt.Sprint(want) {
		t.Errorf("billed = %v, want %v", billed, want)
	}
}
