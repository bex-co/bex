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

package events

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// byIDMembers resolves each subject's workspaces; the first is the default.
type byIDMembers map[string][]string

func (m byIDMembers) Tenant(_ context.Context, id core.Identity) (string, bool) {
	if ws := m[id.Subject]; len(ws) > 0 {
		return ws[0], true
	}
	return "", false
}

func (m byIDMembers) IsMember(_ context.Context, id core.Identity, workspace string) (bool, error) {
	for _, w := range m[id.Subject] {
		if w == workspace {
			return true, nil
		}
	}
	return false, nil
}

// byIDStore indexes each event id under a set of workspaces and answers
// GetServiceEvent only in one of them, like service_event_index's PK.
type byIDStore struct {
	*fakeStore
	owners  map[string][]string
	lookups []string
}

func (s *byIDStore) GetServiceEvent(ctx context.Context, workspace, eventID string) (store.ServiceEventLookup, error) {
	s.lookups = append(s.lookups, workspace)
	for _, w := range s.owners[eventID] {
		if w == workspace {
			return s.fakeStore.GetServiceEvent(ctx, workspace, eventID)
		}
	}
	return store.ServiceEventLookup{}, store.ErrNotFound
}

func (s *byIDStore) ServiceEventWorkspaces(_ context.Context, eventID string) ([]string, error) {
	if ws := s.owners[eventID]; len(ws) > 0 {
		return ws, nil
	}
	return nil, store.ErrNotFound
}

// TestGetRoutesByEventOwnWorkspace is w4/m172 for GET /v1/events/{eventId}: a
// member of [tea-a (default), tea-b] reaches a tea-b event with no ownerId; a
// non-member's id and an unknown id answer the same 404; an explicit ownerId
// keeps scoping the lookup exactly as before.
func TestGetRoutesByEventOwnWorkspace(t *testing.T) {
	row := store.ServiceEventRow{Key: "fact:byid-route", At: now, Source: store.EventSourceFact, FactType: TypePostgresUnavailable}
	eventID := ids.Derive(ids.Event, row.Key)
	missing := ids.Derive(ids.Event, "fact:byid-missing")
	newSvc := func() (*Service, *byIDStore) {
		st := &byIDStore{
			fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Postgres)}},
			owners:    map[string][]string{eventID: {"tea-b"}},
		}
		svc := newService(st)
		svc.Workspace = byIDMembers{"alice": {"tea-a", "tea-b"}, "mallory": {"tea-m"}}
		return svc, st
	}
	as := func(subject string) context.Context {
		return core.WithIdentity(t.Context(), core.Identity{Subject: subject, Method: "session"})
	}

	svc, st := newSvc()
	got, err := svc.Get(as("alice"), eventID)
	if err != nil || got.ID != eventID {
		t.Fatalf("member, no ownerId: %+v, %v; want the tea-b event", got, err)
	}
	if len(st.lookups) != 1 || st.lookups[0] != "tea-b" {
		t.Fatalf("lookup workspaces = %v, want [tea-b]", st.lookups)
	}

	// Explicit ownerId keeps today's behavior: a named workspace that does not
	// hold the event is a 404 even though the caller could reach it elsewhere.
	svc, _ = newSvc()
	if _, err := svc.Get(core.WithWorkspace(as("alice"), "tea-a"), eventID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("mismatched ownerId = %v, want not found", err)
	}
	svc, _ = newSvc()
	if got, err := svc.Get(core.WithWorkspace(as("alice"), "tea-b"), eventID); err != nil || got.ID != eventID {
		t.Fatalf("matching ownerId = %+v, %v", got, err)
	}

	// Non-member and unknown id: the same not-found, and the non-member's
	// probe never reads the foreign workspace's event row.
	svc, st = newSvc()
	foreignErr := func() error { _, err := svc.Get(as("mallory"), eventID); return err }()
	if len(st.lookups) != 0 {
		t.Fatalf("non-member read the event store in %v", st.lookups)
	}
	svc, _ = newSvc()
	unknownErr := func() error { _, err := svc.Get(as("mallory"), missing); return err }()
	if !errors.Is(foreignErr, core.ErrNotFound) || !errors.Is(unknownErr, core.ErrNotFound) {
		t.Fatalf("non-member = %v, unknown = %v; want not found for both", foreignErr, unknownErr)
	}
	if a, b := strings.ReplaceAll(foreignErr.Error(), eventID, "ID"), strings.ReplaceAll(unknownErr.Error(), missing, "ID"); a != b {
		t.Fatalf("non-member %q and unknown %q must be indistinguishable", a, b)
	}
}

// TestGetPrefersAWorkspaceTheCallerBelongsTo covers an evt-… id indexed under
// several workspaces (a workspace:default audit row is attributed to every
// matching owner): the caller's default wins when it is one of them, else the
// first member workspace by id.
func TestGetPrefersAWorkspaceTheCallerBelongsTo(t *testing.T) {
	row := store.ServiceEventRow{Key: "fact:byid-shared", At: now, Source: store.EventSourceFact, FactType: TypePostgresUnavailable}
	eventID := ids.Derive(ids.Event, row.Key)
	for _, tc := range []struct {
		name    string
		members []string
		want    string
	}{
		{"default among owners", []string{"tea-c", "tea-a"}, "tea-c"},
		{"first member owner", []string{"tea-x", "tea-c", "tea-b"}, "tea-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &byIDStore{
				fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Postgres)}},
				owners:    map[string][]string{eventID: {"tea-a", "tea-b", "tea-c"}},
			}
			svc := newService(st)
			svc.Workspace = byIDMembers{"alice": tc.members}
			ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "alice", Method: "session"})
			if _, err := svc.Get(ctx, eventID); err != nil {
				t.Fatal(err)
			}
			if len(st.lookups) != 1 || st.lookups[0] != tc.want {
				t.Fatalf("lookup workspaces = %v, want [%s]", st.lookups, tc.want)
			}
		})
	}
}
