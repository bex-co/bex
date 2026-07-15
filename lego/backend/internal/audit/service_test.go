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

package audit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeAllowChecker allows everything.
type fakeAllowChecker struct{}

func (fakeAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// fakeStore is a spy AuditStore: it records the arguments ListAuditEvents/
// PurgeAuditEvents were called with and returns canned results/errors.
// Mutex-guarded because TestRunWithIntervalSweepsOnStartupAndOnTick reads its
// fields from the test goroutine while the sweep loop writes them from its own.
type fakeStore struct {
	mu           sync.Mutex
	gotWorkspace string
	gotFilter    store.AuditFilter
	listRows     []store.AuditRow
	listErr      error

	gotPurgeBefore time.Time
	purgeN         int64
	purgeErr       error
	purgeCalls     int

	gotSessionPurgeBefore time.Time
	sessionPurgeN         int64
	sessionPurgeErr       error
	sessionPurgeCalls     int
}

func (f *fakeStore) ListAuditEvents(_ context.Context, workspaceID string, filter store.AuditFilter) ([]store.AuditRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotWorkspace, f.gotFilter = workspaceID, filter
	return f.listRows, f.listErr
}

func (f *fakeStore) PurgeAuditEvents(_ context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotPurgeBefore = before
	f.purgeCalls++
	return f.purgeN, f.purgeErr
}

func (f *fakeStore) PurgeSSHSessions(_ context.Context, before time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotSessionPurgeBefore = before
	f.sessionPurgeCalls++
	return f.sessionPurgeN, f.sessionPurgeErr
}

func (f *fakeStore) calls() (int, time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.purgeCalls, f.gotPurgeBefore
}

// TestListPassesFilterThroughAndScopesToOwner proves List forwards the
// caller-supplied filter unchanged and scopes the store query to ownerID —
// not the caller's own workspace, which is what would leak another
// workspace's trail if List silently substituted it.
func TestListPassesFilterThroughAndScopesToOwner(t *testing.T) {
	fs := &fakeStore{listRows: []store.AuditRow{{ID: "aud-1", WorkspaceID: "tea-owner"}}}
	base := &core.Base{Authz: fakeAllowChecker{}}
	svc := &Service{Base: base, Store: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})

	since := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	events, err := svc.List(ctx, "tea-owner", Filter{Since: since, Until: until, Cursor: "aud-cursor", Limit: 7})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if fs.gotWorkspace != "tea-owner" {
		t.Errorf("store queried workspace %q, want tea-owner", fs.gotWorkspace)
	}
	if fs.gotFilter.Since != since || fs.gotFilter.Until != until || fs.gotFilter.Cursor != "aud-cursor" || fs.gotFilter.Limit != 7 {
		t.Errorf("filter not forwarded unchanged: got %+v", fs.gotFilter)
	}
	if len(events) != 1 || events[0].ID != "aud-1" {
		t.Errorf("events = %+v, want the store's single row projected", events)
	}
}

// TestListStoreLessIsUnavailable proves the store-off degrade is
// core.ErrAuditUnavailable, not a nil-pointer panic or a silent empty list —
// the DoD's "store-less mode ... 503 reads (omitted, not faked)".
func TestListStoreLessIsUnavailable(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: nil}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	if _, err := svc.List(ctx, "tea-owner", Filter{}); !errors.Is(err, core.ErrAuditUnavailable) {
		t.Fatalf("List with no store: err = %v, want ErrAuditUnavailable", err)
	}
}

func TestRenderMaintenanceAuditTypesAndMetadata(t *testing.T) {
	disabled := false
	toggle := toRenderAuditLog(Event{Verb: "apps.SetMaintenanceMode", MaintenanceModeTo: &disabled})
	if toggle.Action != "MaintenanceModeEnabledEvent" || toggle.Metadata == nil || toggle.Metadata.To == nil || *toggle.Metadata.To {
		t.Fatalf("toggle audit = %+v, want MaintenanceModeEnabledEvent metadata.to=false", toggle)
	}
	uri := toRenderAuditLog(Event{Verb: "apps.SetMaintenanceModeURI"})
	if uri.Action != "MaintenanceModeURIUpdatedEvent" || uri.Metadata == nil || uri.Metadata.To != nil {
		t.Fatalf("URI audit = %+v, want MaintenanceModeURIUpdatedEvent with empty metadata", uri)
	}
}

// TestListPropagatesStoreError proves a store failure surfaces to the caller
// rather than being swallowed into an empty list (which would look
// indistinguishable from "no events").
func TestListPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("connection reset")
	fs := &fakeStore{listErr: wantErr}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
	if _, err := svc.List(ctx, "tea-owner", Filter{}); !errors.Is(err, wantErr) {
		t.Fatalf("List: err = %v, want %v", err, wantErr)
	}
}

// TestPurgeUsesDefaultRetention proves an unset/invalid RetentionDays falls
// back to DefaultRetentionDays (90) rather than purging with a zero/negative
// window (which would delete everything or nothing depending on the SQL
// driver's handling of a zero time.Time).
func TestPurgeUsesDefaultRetention(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: fs}

	svc.purge(context.Background())

	want := now.AddDate(0, 0, -DefaultRetentionDays)
	if !fs.gotPurgeBefore.Equal(want) {
		t.Errorf("purge before = %s, want %s (now - %d default days)", fs.gotPurgeBefore, want, DefaultRetentionDays)
	}
	if !fs.gotSessionPurgeBefore.Equal(want) {
		t.Errorf("SSH-session purge before = %s, want %s", fs.gotSessionPurgeBefore, want)
	}
}

// TestPurgeUsesConfiguredRetention proves a caller-set RetentionDays actually
// changes the purge boundary, not just the default.
func TestPurgeUsesConfiguredRetention(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{Clock: func() time.Time { return now }}, Store: fs, RetentionDays: 7}

	svc.purge(context.Background())

	want := now.AddDate(0, 0, -7)
	if !fs.gotPurgeBefore.Equal(want) {
		t.Errorf("purge before = %s, want %s (now - 7 configured days)", fs.gotPurgeBefore, want)
	}
	if !fs.gotSessionPurgeBefore.Equal(want) {
		t.Errorf("SSH-session purge before = %s, want %s", fs.gotSessionPurgeBefore, want)
	}
}

// TestPurgeErrorDoesNotPanicOrRetryImmediately proves a store error during
// the sweep is handled gracefully (logged, loop continues) rather than
// crashing the background goroutine it runs in.
func TestPurgeErrorDoesNotPanicOrRetryImmediately(t *testing.T) {
	fs := &fakeStore{purgeErr: errors.New("db unreachable")}
	svc := &Service{Base: &core.Base{}, Store: fs}
	svc.purge(context.Background()) // must not panic
	if fs.purgeCalls != 1 {
		t.Errorf("purge called the store %d times, want exactly 1", fs.purgeCalls)
	}
	if fs.sessionPurgeCalls != 0 {
		t.Errorf("SSH-session purge called %d times after event purge failed, want 0", fs.sessionPurgeCalls)
	}
}

func TestSSHSessionPurgeErrorDoesNotPanicOrRetryImmediately(t *testing.T) {
	fs := &fakeStore{sessionPurgeErr: errors.New("db unreachable")}
	svc := &Service{Base: &core.Base{}, Store: fs}
	svc.purge(context.Background())
	if fs.purgeCalls != 1 || fs.sessionPurgeCalls != 1 {
		t.Errorf("event/session purge calls = %d/%d, want 1/1", fs.purgeCalls, fs.sessionPurgeCalls)
	}
}

// TestRunWithIntervalSweepsOnStartupAndOnTick proves the loop purges once
// immediately (so a restart doesn't defer the first sweep a full interval)
// and again on every tick, stopping cleanly when ctx is cancelled.
func TestRunWithIntervalSweepsOnStartupAndOnTick(t *testing.T) {
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{}, Store: fs}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		svc.RunWithInterval(ctx, 10*time.Millisecond)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		if n, _ := fs.calls(); n >= 3 {
			break
		}
		select {
		case <-deadline:
			n, _ := fs.calls()
			t.Fatalf("only %d purge calls after 2s, want at least 3 (startup + ticks)", n)
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunWithInterval did not return after ctx cancellation")
	}
}
