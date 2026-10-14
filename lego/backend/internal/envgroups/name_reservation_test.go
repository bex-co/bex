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

package envgroups

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// clockedService is a service whose clock the test moves.
func clockedService(store core.SecretKV) (*Service, *time.Time) {
	now := time.Unix(1_000_000, 0).UTC()
	svc := newService(store)
	svc.Clock = func() time.Time { return now }
	return svc, &now
}

func conflictCode(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) && errors.Is(err, core.ErrConflict) {
		return coded.Code
	}
	return ""
}

func storedGroupNames(t *testing.T, svc *Service) []string {
	t.Helper()
	groups, err := svc.ListEnvGroups(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(groups))
	for i, group := range groups {
		names[i] = group.Name
	}
	return names
}

// w4/m206: a create that was interrupted after reserving its name left the
// reservation behind with no group: every retry answered ENV_GROUP_NAME_EXISTS
// for a name the workspace could neither find nor delete. The reservation is
// now named for what it is, and frees itself once its creator cannot still be
// running. This is the exact shape found in production: id, name and workspace,
// no group and no date.
func TestEnvGroup_AbandonedUndatedNameReservationFreesItself(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc, now := clockedService(store)
	path := envGroupNameClaimPath("", "qa-first")
	if _, err := store.PutCAS(ctx, path, map[string]string{"id": "evg-never-written", "name": "qa-first", "workspace": ""}, 0); err != nil {
		t.Fatal(err)
	}

	_, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "qa-first"})
	if code := conflictCode(err); code != "ENV_GROUP_NAME_RESERVED" {
		t.Fatalf("first retry = %v (code %q), want ENV_GROUP_NAME_RESERVED", err, code)
	}
	// The first sighting dates it; the creator is not displaced early.
	*now = now.Add(envGroupNameClaimGrace - time.Second)
	_, err = svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "qa-first"})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENV_GROUP_NAME_RESERVED" || coded.Params["retryAfterSeconds"] != 1 {
		t.Fatalf("retry inside the grace = %v (params %v), want reserved for one more second", err, coded)
	}
	if names := storedGroupNames(t, svc); len(names) != 0 {
		t.Fatalf("groups after refused retries = %v, want none", names)
	}

	*now = now.Add(time.Second)
	created, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "qa-first"})
	if err != nil {
		t.Fatalf("create after the grace: %v", err)
	}
	if names := storedGroupNames(t, svc); len(names) != 1 || names[0] != "qa-first" {
		t.Fatalf("groups = %v, want the one new qa-first", names)
	}
	claim, _ := store.Get(ctx, path)
	if claim["id"] != created.ID {
		t.Fatalf("reservation owner = %q, want the new group %q", claim["id"], created.ID)
	}
	// Now it is a real group's name: the ordinary conflict, however long ago.
	*now = now.Add(24 * time.Hour)
	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "qa-first"}); conflictCode(err) != "ENV_GROUP_NAME_EXISTS" {
		t.Fatalf("create over an existing group = %v, want ENV_GROUP_NAME_EXISTS", err)
	}
}

// A reservation made since w4/m206 carries its own date, so a creator that
// died is outlasted from the moment it claimed, with no first-sighting step —
// and one still inside its request is never displaced.
func TestEnvGroup_DatedNameReservationOutlivesItsCreatorOnly(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc, now := clockedService(store)
	if err := svc.claimGroupName(ctx, "", "shared", "evg-creator-died"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(30 * time.Second) // a creator may still be mid-request
	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"}); conflictCode(err) != "ENV_GROUP_NAME_RESERVED" {
		t.Fatalf("create beside a live reservation = %v, want ENV_GROUP_NAME_RESERVED", err)
	}
	*now = now.Add(envGroupNameClaimGrace)
	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"}); err != nil {
		t.Fatalf("create after the creator's grace: %v", err)
	}
}

// Two retries racing for an abandoned name still resolve to one winner.
func TestEnvGroup_AbandonedNameReservationHasOneTaker(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	first, now := clockedService(store)
	if err := first.claimGroupName(ctx, "", "shared", "evg-creator-died"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(envGroupNameClaimGrace + time.Second)
	second := newService(store)
	second.Clock = first.Clock
	errs := make(chan error, 2)
	start := make(chan struct{})
	for _, svc := range []*Service{first, second} {
		go func() {
			<-start
			_, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
			errs <- err
		}()
	}
	close(start)
	won := 0
	for range 2 {
		if err := <-errs; err == nil {
			won++
		} else if !errors.Is(err, core.ErrConflict) {
			t.Fatalf("racing create = %v, want success or a conflict", err)
		}
	}
	if names := storedGroupNames(t, first); won != 1 || len(names) != 1 {
		t.Fatalf("winners = %d, groups = %v; want exactly one of each", won, names)
	}
}

// uncertainClaimStore commits a name reservation and then reports a transport
// failure, as a write whose acknowledgment was lost does.
type uncertainClaimStore struct {
	*fakeStore
	fail bool
}

func (s *uncertainClaimStore) PutCAS(ctx context.Context, path string, data map[string]string, expectedVersion uint64) (uint64, error) {
	version, err := s.fakeStore.PutCAS(ctx, path, data, expectedVersion)
	if err == nil && s.fail && strings.HasPrefix(path, "env-group-name-claims/") && data["id"] != "" {
		return 0, errors.New("connection reset before the write was acknowledged")
	}
	return version, err
}

// A reservation write that may have landed is released before its failure is
// returned: the caller was never told it holds the name, so nothing else would.
// The failure stays what it was, and the name is free at once.
func TestEnvGroup_UnacknowledgedNameReservationIsReleased(t *testing.T) {
	ctx := context.Background()
	store := &uncertainClaimStore{fakeStore: newFakeStore(), fail: true}
	svc, _ := clockedService(store)

	_, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err == nil || errors.Is(err, core.ErrConflict) {
		t.Fatalf("create with a lost acknowledgment = %v, want the transport failure", err)
	}
	if claim, _ := store.Get(ctx, envGroupNameClaimPath("", "shared")); claim["id"] != "" {
		t.Fatalf("reservation left behind: %v", claim)
	}
	if names := storedGroupNames(t, svc); len(names) != 0 {
		t.Fatalf("groups after the failed create = %v, want none", names)
	}

	store.fail = false
	if _, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"}); err != nil {
		t.Fatalf("immediate retry: %v, want the name free with no waiting", err)
	}
}

// Renaming onto an abandoned name follows the same rule as creating under it.
func TestEnvGroup_RenameOntoAnAbandonedNameReservation(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc, now := clockedService(store)
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "current"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.claimGroupName(ctx, "", "wanted", "evg-creator-died"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RenameEnvGroup(ctx, group.ID, "wanted"); conflictCode(err) != "ENV_GROUP_NAME_RESERVED" {
		t.Fatalf("rename onto a live reservation = %v, want ENV_GROUP_NAME_RESERVED", err)
	}
	*now = now.Add(envGroupNameClaimGrace + time.Second)
	renamed, err := svc.RenameEnvGroup(ctx, group.ID, "wanted")
	if err != nil || renamed.Name != "wanted" {
		t.Fatalf("rename after the grace = %+v, %v", renamed, err)
	}
}
