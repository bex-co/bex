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

package authz

// The model these tests pin, restated from authz.go:
//
//   - only positives are cached, and only for PositiveTTL;
//   - negatives and upstream errors are never cached, so a fresh grant or a
//     recovered upstream applies on the very next check;
//   - CheckFresh neither reads nor populates the cache;
//   - concurrent identical checks coalesce into one upstream call, distinct
//     checks do not;
//   - the store is resolved once;
//   - a revoke evicts exactly the revoked subject's cached decisions — and an
//     answer computed while the revoke was in flight must not be re-cached
//     afterwards (the round-6 #16 guarantee would otherwise expire one RTT
//     later than the eviction).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fgaStub is a scripted OpenFGA: the caller decides what /check answers, and
// the stub records every store lookup, check and membership write. beforeAnswer
// runs inside the /check handler before the answer is written — the seam the
// in-flight-revocation regression needs.
type fgaStub struct {
	mu           sync.Mutex
	storeGets    int
	checks       []tupleKey
	writes       []writeRequest
	answer       func(tupleKey) bool
	beforeAnswer func()
	failChecks   bool
	failWrites   bool
}

func (s *fgaStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fga-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/stores":
			s.mu.Lock()
			s.storeGets++
			s.mu.Unlock()
			_, _ = fmt.Fprint(w, `{"stores":[{"id":"store-1","name":"bex"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/stores/store-1/check":
			var in checkRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			s.mu.Lock()
			s.checks = append(s.checks, in.TupleKey)
			answer, before, fail := s.answer, s.beforeAnswer, s.failChecks
			s.mu.Unlock()
			if before != nil {
				before() // deliberately outside mu: it may block on the test
			}
			if fail {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			allowed := answer != nil && answer(in.TupleKey)
			_, _ = fmt.Fprintf(w, `{"allowed":%v}`, allowed)
		case r.Method == http.MethodPost && r.URL.Path == "/stores/store-1/write":
			var in writeRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			s.mu.Lock()
			fail := s.failWrites
			s.writes = append(s.writes, in)
			s.mu.Unlock()
			if fail {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			_, _ = fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (s *fgaStub) setAnswer(answer func(tupleKey) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.answer = answer
}

func (s *fgaStub) setFailChecks(fail bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failChecks = fail
}

func (s *fgaStub) checkCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.checks)
}

func (s *fgaStub) storeGetCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeGets
}

func (s *fgaStub) writeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writes)
}

func allowAll(tupleKey) bool { return true }
func denyAll(tupleKey) bool  { return false }

func newStubChecker(t *testing.T, stub *fgaStub) *openfgaChecker {
	t.Helper()
	return NewOpenFGAChecker(stub.server(t).URL, "fga-key").(*openfgaChecker)
}

func TestCheckDoesNotCacheUpstreamErrors(t *testing.T) {
	stub := &fgaStub{answer: allowAll, failChecks: true}
	chk := newStubChecker(t, stub)
	ctx := context.Background()

	if _, err := chk.Check(ctx, "user:a", core.RelCanView, core.WorkspaceObject("tea-a")); err == nil {
		t.Fatal("upstream error must surface, not be swallowed as a denial")
	}
	stub.setFailChecks(false)
	if ok, err := chk.Check(ctx, "user:a", core.RelCanView, core.WorkspaceObject("tea-a")); err != nil || !ok {
		t.Fatalf("check after recovery = %v/%v, want a fresh upstream answer", ok, err)
	}
	if got := stub.checkCount(); got != 2 {
		t.Fatalf("upstream checks = %d, want 2: an errored check must not be cached", got)
	}
}

func TestCheckFreshBypassesAndNeverPopulatesTheCache(t *testing.T) {
	stub := &fgaStub{answer: allowAll}
	chk := newStubChecker(t, stub)
	ctx := context.Background()
	key := core.WorkspaceObject("tea-a")

	if ok, err := chk.Check(ctx, "user:a", core.RelCanView, key); err != nil || !ok {
		t.Fatalf("warm check = %v/%v", ok, err)
	}
	// CheckFresh bypasses the cached positive and asks upstream again.
	if ok, err := chk.CheckFresh(ctx, "user:a", core.RelCanView, key); err != nil || !ok {
		t.Fatalf("fresh check = %v/%v", ok, err)
	}
	if got := stub.checkCount(); got != 2 {
		t.Fatalf("upstream checks = %d, want 2 (CheckFresh bypasses the cache)", got)
	}

	// A fresh check observes a just-revoked state the cache does not.
	stub.setAnswer(denyAll)
	if ok, err := chk.CheckFresh(ctx, "user:a", core.RelCanView, key); err != nil || ok {
		t.Fatalf("fresh check after revoke = %v/%v, want the upstream denial", ok, err)
	}
	// ...and CheckFresh did not overwrite or evict the cached positive, so the
	// cached Check path is unchanged (still one cached answer, still allowed).
	if ok, err := chk.Check(ctx, "user:a", core.RelCanView, key); err != nil || !ok {
		t.Fatalf("cached check = %v/%v, want the cached positive", ok, err)
	}
	if got := stub.checkCount(); got != 3 {
		t.Fatalf("upstream checks = %d, want 3 (Check is still cached)", got)
	}

	// CheckFresh must also not POPULATE: a fresh-only key checked twice costs
	// two upstream calls, and a following cached Check still goes upstream.
	other := core.WorkspaceObject("tea-b")
	for range 2 {
		if ok, err := chk.CheckFresh(ctx, "user:a", core.RelCanView, other); err != nil || ok {
			t.Fatalf("fresh check of uncached key = %v/%v", ok, err)
		}
	}
	if ok, err := chk.Check(ctx, "user:a", core.RelCanView, other); err != nil || ok {
		t.Fatalf("cached check of fresh-only key = %v/%v", ok, err)
	}
	if got := stub.checkCount(); got != 6 {
		t.Fatalf("upstream checks = %d, want 6: CheckFresh must never populate the cache", got)
	}
}

func TestConcurrentIdenticalChecksCoalesceIntoOneUpstreamCall(t *testing.T) {
	stub := &fgaStub{answer: allowAll}
	chk := newStubChecker(t, stub)
	ctx := context.Background()

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if ok, err := chk.Check(ctx, "user:a", core.RelCanView, core.WorkspaceObject("tea-a")); err != nil || !ok {
				errs <- fmt.Errorf("check = %v/%v", ok, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if got := stub.checkCount(); got != 1 {
		t.Fatalf("upstream checks = %d, want 1: identical in-flight checks must coalesce", got)
	}
}

func TestConcurrentDistinctChecksDoNotCoalesce(t *testing.T) {
	stub := &fgaStub{answer: allowAll}
	chk := newStubChecker(t, stub)
	ctx := context.Background()

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			obj := core.WorkspaceObject(fmt.Sprintf("tea-%d", i))
			if ok, err := chk.Check(ctx, "user:a", core.RelCanView, obj); err != nil || !ok {
				t.Errorf("check %s = %v/%v", obj, ok, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := stub.checkCount(); got != n {
		t.Fatalf("upstream checks = %d, want %d: distinct keys must not coalesce", got, n)
	}
}

func TestStoreResolvesOnceAcrossConcurrentColdStartChecks(t *testing.T) {
	stub := &fgaStub{answer: allowAll}
	chk := newStubChecker(t, stub)
	ctx := context.Background()

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			obj := core.WorkspaceObject(fmt.Sprintf("tea-%d", i))
			if _, err := chk.Check(ctx, "user:a", core.RelCanView, obj); err != nil {
				t.Errorf("cold check: %v", err)
			}
		}()
	}
	close(start)
	// A goroutine that read an empty storeID before the first lookup finished
	// and then joins singleflight only after it returned would re-execute the
	// lookup; give every goroutine a moment to enter the in-flight call so the
	// assertion below is about coalescing, not about scheduling.
	time.Sleep(50 * time.Millisecond)
	wg.Wait()

	if got := stub.storeGetCount(); got != 1 {
		t.Fatalf("store lookups = %d, want 1: cold-start lookups must coalesce", got)
	}
	// Warm checks never look the store up again.
	if _, err := chk.Check(ctx, "user:a", core.RelCanView, core.WorkspaceObject("tea-z")); err != nil {
		t.Fatalf("warm check: %v", err)
	}
	if got := stub.storeGetCount(); got != 1 {
		t.Fatalf("store lookups = %d after a warm check, want 1", got)
	}
}

func TestRevokeEvictsExactlyTheSubjectPrefix(t *testing.T) {
	stub := &fgaStub{answer: allowAll}
	chk := newStubChecker(t, stub)
	ctx := context.Background()

	pairs := []struct{ subject, object string }{
		{"user:mal", core.WorkspaceObject("tea-a")},
		{"user:mal", "service:srv-1"},
		{"user:mallory", core.WorkspaceObject("tea-a")}, // shares "user:mal" as a prefix
	}
	for _, p := range pairs {
		if ok, err := chk.Check(ctx, p.subject, core.RelCanView, p.object); err != nil || !ok {
			t.Fatalf("warm %s/%s = %v/%v", p.subject, p.object, ok, err)
		}
	}
	if got := stub.checkCount(); got != len(pairs) {
		t.Fatalf("warm upstream checks = %d, want %d", got, len(pairs))
	}

	if err := chk.RevokeWorkspaceMember(ctx, "tea-a", "user:mal", "developer"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	for _, p := range pairs {
		if ok, err := chk.Check(ctx, p.subject, core.RelCanView, p.object); err != nil || !ok {
			t.Fatalf("post-revoke %s/%s = %v/%v", p.subject, p.object, ok, err)
		}
	}
	// Both user:mal decisions were evicted (2 upstream calls); user:mallory's
	// decision was not (0) — eviction is by the exact subject prefix, not by
	// string prefix.
	if got := stub.checkCount(); got != len(pairs)+2 {
		t.Fatalf("upstream checks = %d, want %d: revoke must evict exactly one subject", got, len(pairs)+2)
	}
}

// TestRevokeDuringInFlightCheckDoesNotResurrectPositive is the regression for
// the eviction/in-flight-check race: a Check that read the pre-revoke state
// must not re-put its positive after RevokeWorkspaceMember has evicted the
// subject, or the revoked decision survives for the rest of PositiveTTL —
// exactly the window round-6 #16 closed. Same discipline as api/auth.go's
// revocationEpoch (a plain atomic re-check is not sufficient).
func TestRevokeDuringInFlightCheckDoesNotResurrectPositive(t *testing.T) {
	started := make(chan struct{})
	unblock := make(chan struct{})
	var gateOnce sync.Once
	stub := &fgaStub{
		answer: allowAll,
		beforeAnswer: func() {
			gateOnce.Do(func() {
				close(started)
				<-unblock
			})
		},
	}
	chk := newStubChecker(t, stub)
	ctx := context.Background()
	subject, object := "user:mal", core.WorkspaceObject("tea-a")

	type outcome struct {
		ok  bool
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		ok, err := chk.Check(ctx, subject, core.RelCanView, object)
		done <- outcome{ok, err}
	}()
	<-started // the check is now in flight, its answer not yet written

	if err := chk.RevokeWorkspaceMember(ctx, "tea-a", subject, "developer"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	close(unblock)
	if got := <-done; got.err != nil || !got.ok {
		t.Fatalf("in-flight check = %+v, want the upstream answer", got)
	}
	if got := stub.checkCount(); got != 1 {
		t.Fatalf("upstream checks = %d, want 1", got)
	}

	// The next check must consult OpenFGA: the revoked subject's decision was
	// evicted by the revoke and must not have been restored by the in-flight
	// answer landing afterwards.
	if ok, err := chk.Check(ctx, subject, core.RelCanView, object); err != nil || !ok {
		t.Fatalf("post-revoke check = %v/%v", ok, err)
	}
	if got := stub.checkCount(); got != 2 {
		t.Fatalf("upstream checks = %d, want 2: the in-flight positive was resurrected into the cache across the revoke", got)
	}
}

func TestRevokeDeletesTheTupleGrantWrote(t *testing.T) {
	stub := &fgaStub{answer: allowAll}
	chk := newStubChecker(t, stub)
	ctx := context.Background()

	if err := chk.GrantWorkspaceRole(ctx, "tea-a", "user:b", "billing"); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if err := chk.RevokeWorkspaceMember(ctx, "tea-a", "user:b", "billing"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := stub.writeCount(); got != 2 {
		t.Fatalf("writes = %d, want 2", got)
	}
	grant, revoke := stub.writes[0], stub.writes[1]
	if grant.Writes == nil || grant.Deletes != nil || len(grant.Writes.TupleKeys) != 1 {
		t.Fatalf("grant body = %+v, want one writes tuple and no deletes", grant)
	}
	if revoke.Deletes == nil || revoke.Writes != nil || len(revoke.Deletes.TupleKeys) != 1 {
		t.Fatalf("revoke body = %+v, want one deletes tuple and no writes", revoke)
	}
	granted, revoked := grant.Writes.TupleKeys[0], revoke.Deletes.TupleKeys[0]
	if granted != revoked {
		t.Fatalf("revoke deletes %+v, want the tuple grant wrote %+v", revoked, granted)
	}
	if granted.Relation != "billing" || granted.Object != core.WorkspaceObject("tea-a") || granted.User != "user:b" {
		t.Fatalf("tuple = %+v, want user:b billing workspace:tea-a", granted)
	}
}

func TestRevokeWriteFailureDoesNotEvict(t *testing.T) {
	// Eviction is only correct once the tuple is actually gone: if OpenFGA
	// rejects the delete, the warm positive must keep serving (fail back to the
	// known state, not to a fresh check that still sees the grant).
	stub := &fgaStub{answer: allowAll, failWrites: true}
	chk := newStubChecker(t, stub)
	ctx := context.Background()
	subject, object := "user:mal", core.WorkspaceObject("tea-a")
	if ok, err := chk.Check(ctx, subject, core.RelCanView, object); err != nil || !ok {
		t.Fatalf("warm check = %v/%v", ok, err)
	}
	if err := chk.RevokeWorkspaceMember(ctx, "tea-a", subject, "developer"); err == nil {
		t.Fatal("a failed delete must surface")
	}
	if ok, err := chk.Check(ctx, subject, core.RelCanView, object); err != nil || !ok {
		t.Fatalf("post-failed-revoke check = %v/%v", ok, err)
	}
	if got := stub.checkCount(); got != 1 {
		t.Fatalf("upstream checks = %d, want 1: a failed revoke must not evict", got)
	}
}
