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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ageRule is a rule in the wrangler shape that deletes after maxAge seconds.
func ageRule(id, prefix string, maxAge int, enabled bool) string {
	return fmt.Sprintf(`{"id":%q,"enabled":%t,"conditions":{"prefix":%q},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":%d}}}`,
		id, enabled, prefix, maxAge)
}

func dateRule(id, prefix, date string) string {
	return fmt.Sprintf(`{"id":%q,"enabled":true,"conditions":{"prefix":%q},"deleteObjectsTransition":{"condition":{"type":"Date","date":%q}}}`,
		id, prefix, date)
}

// remaining is the customer keys left in a bucket, sorted.
func (e *sweepEnv) remaining(b store.ObjectBucket) []string {
	e.t.Helper()
	prefix := objectstorage.Prefix(e.WS, b.ID)
	var keys []string
	for _, k := range e.Fakes[objectstorage.Region(b.Region)].Keys(objectstorage.Region(b.Region).Bucket(), prefix) {
		keys = append(keys, strings.TrimPrefix(k, prefix))
	}
	return keys
}

func wantKeys(t *testing.T, got []string, want ...string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("remaining keys = %v, want %v", got, want)
	}
}

func TestLifecycleAgeDeletesOnlyObjectsOlderThanMaxAge(t *testing.T) {
	e := newSweepEnv(t, "life-age")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old", 10, 2*time.Hour)
	e.put(b, "exactly", 20, time.Hour) // not older than maxAge
	e.put(b, "fresh", 40, 30*time.Minute)
	e.setLifecycle(b, "["+ageRule("a", "", 3600, true)+"]")

	e.mustSweep()

	wantKeys(t, e.remaining(b), "exactly", "fresh")
	e.wantUsage(b.ID, 60, 2) // the usage excludes what the rule deleted
}

// maxAge 0 is a rule the route accepts (Cloudflare's API allows it) and means
// every matching object is already older than the rule; a rule with no maxAge at
// all is not that rule and deletes nothing.
func TestLifecycleAgeZeroDeletesEverythingAndAMissingAgeNothing(t *testing.T) {
	e := newSweepEnv(t, "life-age0")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "fresh", 1, time.Second)
	missing := `{"id":"m","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age"}}}`
	e.setLifecycle(b, "["+missing+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b), "fresh")
	e.setLifecycle(b, "["+ageRule("zero", "", 0, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b))
}

func TestLifecycleDateDeletesEveryMatchingObjectOncePassed(t *testing.T) {
	e := newSweepEnv(t, "life-date")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old", 1, 48*time.Hour)
	e.put(b, "fresh", 1, time.Minute) // newer than the date's passing, and still deleted

	// A date still ahead deletes nothing, however old the objects.
	e.setLifecycle(b, "["+dateRule("later", "", "2026-12-01T00:00:00Z")+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b), "fresh", "old")

	// Once it has passed, like S3's and R2's date expiration, everything matching goes.
	e.setLifecycle(b, "["+dateRule("passed", "", "2026-11-10T11:59:59Z")+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b))
	e.wantUsage(b.ID, 0, 0)
}

func TestLifecycleDateAcceptsABareDate(t *testing.T) {
	e := newSweepEnv(t, "life-baredate")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "a", 1, time.Minute)
	e.setLifecycle(b, "["+dateRule("d", "", "2026-11-01")+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b))
}

func TestLifecyclePrefixLimitsARuleToItsKeys(t *testing.T) {
	e := newSweepEnv(t, "life-prefix")
	b := e.bucket("data", objectstorage.Oregon)
	for _, k := range []string{"logs/a", "logs/sub/b", "data/c", "logs", "logs-archive/d", "x/logs/e"} {
		e.put(b, k, 1, 24*time.Hour)
	}
	e.setLifecycle(b, "["+ageRule("logs", "logs/", 60, true)+"]")

	e.mustSweep()

	wantKeys(t, e.remaining(b), "data/c", "logs", "logs-archive/d", "x/logs/e")
	// The prefix is the bucket's key, not the Wasabi key: a rule cannot reach
	// into the workspace or bucket segments.
	e.setLifecycle(b, "["+ageRule("wasabi", e.WS+"/", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b), "data/c", "logs", "logs-archive/d", "x/logs/e")
}

func TestLifecycleDisabledRuleDoesNothing(t *testing.T) {
	e := newSweepEnv(t, "life-disabled")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old", 1, 24*time.Hour)
	e.setLifecycle(b, "["+ageRule("off", "", 60, false)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b), "old")
	if deletes(e.Fakes[objectstorage.Oregon]) != 0 {
		t.Fatal("a disabled rule sent a delete")
	}
	e.setLifecycle(b, "["+ageRule("on", "", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b))
}

// deletes counts the delete requests, batch or single, a fake saw.
func deletes(fake *fakes3.Server) int {
	n := 0
	for _, r := range fake.Requests() {
		if (r.Method == "POST" && r.Query.Has("delete")) || r.Method == "DELETE" {
			n++
		}
	}
	return n
}

func TestLifecycleRuleThatMatchesNothingDeletesNothing(t *testing.T) {
	e := newSweepEnv(t, "life-nothing")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "a", 5, time.Minute)
	e.put(b, "b", 6, 100*time.Hour)
	e.setLifecycle(b, "["+ageRule("nope", "nothing/", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b), "a", "b")
	e.wantUsage(b.ID, 11, 2)
	if n := deletes(e.Fakes[objectstorage.Oregon]); n != 0 {
		t.Fatalf("a rule that matched nothing sent %d delete requests", n)
	}
}

func TestLifecycleRulesBelongToTheirOwnBucket(t *testing.T) {
	e := newSweepEnv(t, "life-isolated")
	a, b := e.bucket("with-rule", objectstorage.Oregon), e.bucket("without", objectstorage.Oregon)
	for _, bk := range []store.ObjectBucket{a, b} {
		e.put(bk, "old", 1, 24*time.Hour)
	}
	e.setLifecycle(a, "["+ageRule("a", "", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(a))
	wantKeys(t, e.remaining(b), "old")
	e.wantUsage(b.ID, 1, 1)
}

func TestLifecycleDeletesAtMostCapObjectsPerRulePerSweep(t *testing.T) {
	e := newSweepEnv(t, "life-cap")
	e.Sweeper.LifecycleCap = 3
	b := e.bucket("logs", objectstorage.Oregon)
	for i := range 7 {
		e.put(b, fmt.Sprintf("o%d", i), 1, 24*time.Hour)
	}
	e.setLifecycle(b, "["+ageRule("a", "", 60, true)+"]")

	e.mustSweep()
	if got := len(e.remaining(b)); got != 4 {
		t.Fatalf("%d objects remain after one sweep under a cap of 3, want 4", got)
	}
	e.wantUsage(b.ID, 4, 4)
	e.mustSweep()
	e.mustSweep()
	if got := len(e.remaining(b)); got != 0 {
		t.Fatalf("%d objects remain after three sweeps, want the cap to spread the work over sweeps and finish it", got)
	}
}

func TestLifecycleCapIsPerRuleNotPerBucketOrSweep(t *testing.T) {
	e := newSweepEnv(t, "life-caps")
	e.Sweeper.LifecycleCap = 3
	a, b := e.bucket("first", objectstorage.Oregon), e.bucket("second", objectstorage.Oregon)
	for i := range 5 {
		e.put(a, fmt.Sprintf("x/%d", i), 1, 24*time.Hour)
		e.put(a, fmt.Sprintf("y/%d", i), 1, 24*time.Hour)
		e.put(b, fmt.Sprintf("z/%d", i), 1, 24*time.Hour)
	}
	// Two rules on one bucket, each with its own cap, and a rule on another.
	e.setLifecycle(a, "["+ageRule("x", "x/", 60, true)+","+ageRule("y", "y/", 60, true)+"]")
	e.setLifecycle(b, "["+ageRule("z", "z/", 60, true)+"]")

	e.mustSweep()

	if got := len(e.remaining(a)); got != 4 { // 3 of 5 deleted under x/, 3 of 5 under y/
		t.Fatalf("bucket a keeps %d objects, want 4: each of its two rules deletes up to 3", got)
	}
	if got := len(e.remaining(b)); got != 2 {
		t.Fatalf("bucket b keeps %d objects, want 2: another bucket's rules do not spend its cap", got)
	}
}

func TestLifecycleOverlappingRulesDeleteAnObjectOnceAndSpendOneCap(t *testing.T) {
	e := newSweepEnv(t, "life-overlap")
	e.Sweeper.LifecycleCap = 3
	b := e.bucket("logs", objectstorage.Oregon)
	for i := range 5 {
		e.put(b, fmt.Sprintf("o%d", i), 1, 24*time.Hour)
	}
	// Both rules match everything. The first takes three; the second has room
	// for the other two, so one sweep clears all five.
	e.setLifecycle(b, "["+ageRule("one", "", 60, true)+","+ageRule("two", "", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b))
	e.wantUsage(b.ID, 0, 0)
}

func TestLifecycleSkipsABadRuleAndRunsTheRestWithoutFailingTheSweep(t *testing.T) {
	e := newSweepEnv(t, "life-bad")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old", 3, 24*time.Hour)
	e.put(b, "fresh", 4, time.Second)
	rules := "[" + strings.Join([]string{
		`{"id":"wrong-type","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age","maxAge":"soon"}}}`,
		`{"id":"unknown-kind","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Weekly","maxAge":60}}}`,
		`{"id":"no-max-age","enabled":true,"conditions":{"prefix":""},"deleteObjectsTransition":{"condition":{"type":"Age"}}}`,
		ageRule("good", "", 3600, true),
	}, ",") + "]"
	e.setLifecycle(b, rules)

	if err := e.sweep(); err != nil {
		t.Fatalf("a bad rule failed the sweep: %v", err)
	}
	wantKeys(t, e.remaining(b), "fresh")
	e.wantUsage(b.ID, 4, 1)
	for _, want := range []string{"rule 0", "Weekly", "maxAge in seconds"} {
		if !e.logged(want) {
			t.Errorf("no log line mentions %q: %q", want, e.Logs)
		}
	}
}

func TestLifecycleSkipsAStoredDocumentThatIsNotARuleList(t *testing.T) {
	e := newSweepEnv(t, "life-garbage")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old", 3, 24*time.Hour)
	// The column is jsonb, so any JSON value can be in it.
	e.setLifecycle(b, `{"rules":[`+ageRule("a", "", 60, true)+`]}`)
	if err := e.sweep(); err != nil {
		t.Fatalf("a malformed lifecycle document failed the sweep: %v", err)
	}
	wantKeys(t, e.remaining(b), "old")
	e.wantUsage(b.ID, 3, 1) // the bucket is still counted
	if !e.logged(b.ID) {
		t.Fatalf("the bucket with the bad document was not named in the log: %q", e.Logs)
	}
}

func TestLifecycleToleratesUnknownFieldsAndRunsNothingItDoesNotImplement(t *testing.T) {
	e := newSweepEnv(t, "life-unknown")
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "cold/old", 1, 24*time.Hour)
	e.put(b, "gone/old", 1, 24*time.Hour)
	e.setLifecycle(b, `[
		{"id":"ia","enabled":true,"conditions":{"prefix":"cold/"},
		 "storageClassTransitions":[{"condition":{"type":"Age","maxAge":60},"storageClass":"InfrequentAccess"}],
		 "abortMultipartUploadsTransition":{"condition":{"type":"Age","maxAge":60}}},
		{"id":"future","enabled":true,"conditions":{"prefix":"gone/","somethingNew":true},"newField":[1],
		 "deleteObjectsTransition":{"condition":{"type":"Age","maxAge":60,"alsoNew":1}}}
	]`)
	e.mustSweep()
	// Storage-class transitions and multipart aborts are stored and do nothing.
	wantKeys(t, e.remaining(b), "cold/old")
}

// failingBatchDelete is a store whose multi-object delete fails.
type failingBatchDelete struct {
	objectstorage.Store
	calls int
}

func (f *failingBatchDelete) DeleteKeys(_ context.Context, keys []string) ([]string, error) {
	f.calls++
	return keys, errors.New("wasabi refused the delete")
}

func TestLifecycleDeleteFailureKeepsTheObjectsCountedAndDoesNotFailTheSweep(t *testing.T) {
	var failing *failingBatchDelete
	e := newSweepEnv(t, "life-fails", func(r objectstorage.Region, st objectstorage.Store) objectstorage.Store {
		if r != objectstorage.Oregon {
			return st
		}
		failing = &failingBatchDelete{Store: st}
		return failing
	})
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old1", 10, 24*time.Hour)
	e.put(b, "old2", 20, 24*time.Hour)
	e.setLifecycle(b, "["+ageRule("a", "", 60, true)+"]")

	if err := e.sweep(); err != nil {
		t.Fatalf("a failed lifecycle delete failed the sweep: %v", err)
	}
	wantKeys(t, e.remaining(b), "old1", "old2")
	e.wantUsage(b.ID, 30, 2) // still stored, still counted
	if failing.calls != 1 || !e.logged("lifecycle delete failed") {
		t.Fatalf("delete calls = %d, logged = %v; want one attempt and a log line", failing.calls, e.logged("lifecycle delete failed"))
	}
}

// singleDeleteOnly hides the batch delete a real store has.
type singleDeleteOnly struct{ objectstorage.Store }

func TestLifecycleWorksWithAStoreThatCanOnlyDeleteOneKeyAtATime(t *testing.T) {
	e := newSweepEnv(t, "life-single", func(_ objectstorage.Region, st objectstorage.Store) objectstorage.Store {
		return singleDeleteOnly{st}
	})
	b := e.bucket("logs", objectstorage.Oregon)
	e.put(b, "old1", 10, 24*time.Hour)
	e.put(b, "old2", 20, 24*time.Hour)
	e.put(b, "fresh", 40, time.Second)
	e.setLifecycle(b, "["+ageRule("a", "", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b), "fresh")
	e.wantUsage(b.ID, 40, 1)
}

func TestLifecycleDeletesInOneRequestPerPageNotOnePerObject(t *testing.T) {
	e := newSweepEnv(t, "life-batch")
	b := e.bucket("logs", objectstorage.Oregon)
	for i := range 50 {
		e.put(b, fmt.Sprintf("o%02d", i), 1, 24*time.Hour)
	}
	e.setLifecycle(b, "["+ageRule("a", "", 60, true)+"]")
	e.mustSweep()
	wantKeys(t, e.remaining(b))
	var batches, singles int
	for _, r := range e.Fakes[objectstorage.Oregon].Requests() {
		switch {
		case r.Method == "POST" && r.Query.Has("delete"):
			batches++
		case r.Method == "DELETE":
			singles++
		}
	}
	if batches != 1 || singles != 0 {
		t.Fatalf("50 deletions took %d batch and %d single requests, want 1 and 0", batches, singles)
	}
}
