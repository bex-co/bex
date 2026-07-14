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

package webhooks

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fakes ---

// fakeEndpointStore is an in-memory EndpointStore, workspace-scoped like the
// real Postgres rows (a lookup for the wrong workspace is ErrNotFound), and
// secret-omitting on reads like the real column lists.
type fakeEndpointStore struct {
	rows       map[string]store.WebhookEndpoint // by id, WITH secret (the table)
	deliveries map[string][]store.WebhookDelivery
}

func newFakeEndpointStore() *fakeEndpointStore {
	return &fakeEndpointStore{rows: map[string]store.WebhookEndpoint{}, deliveries: map[string][]store.WebhookDelivery{}}
}

// redact mirrors the real store's reads, which never select the secret column.
func redact(e store.WebhookEndpoint) store.WebhookEndpoint {
	e.Secret = ""
	return e
}

func (f *fakeEndpointStore) CreateWebhookEndpoint(_ context.Context, tenantID, name, url, secret string, eventTypes []string, createdBy string) (store.WebhookEndpoint, error) {
	if name == "" {
		name = url
	}
	now := time.Now().UTC()
	e := store.WebhookEndpoint{
		ID: ids.New(ids.Webhook), TenantID: tenantID, Name: name, URL: url, Secret: secret,
		EventTypes: eventTypes, Enabled: true, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	f.rows[e.ID] = e
	return e, nil
}

func (f *fakeEndpointStore) ListWebhookEndpoints(_ context.Context, tenantID string) ([]store.WebhookEndpoint, error) {
	var out []store.WebhookEndpoint
	for _, e := range f.rows {
		if e.TenantID == tenantID {
			out = append(out, redact(e))
		}
	}
	return out, nil
}

func (f *fakeEndpointStore) GetWebhookEndpoint(_ context.Context, tenantID, id string) (store.WebhookEndpoint, error) {
	e, ok := f.rows[id]
	if !ok || e.TenantID != tenantID {
		return store.WebhookEndpoint{}, store.ErrNotFound
	}
	return redact(e), nil
}

func (f *fakeEndpointStore) SetWebhookEndpointEnabled(_ context.Context, tenantID, id string, enabled bool, reason string) (store.WebhookEndpoint, error) {
	e, ok := f.rows[id]
	if !ok || e.TenantID != tenantID {
		return store.WebhookEndpoint{}, store.ErrNotFound
	}
	if enabled {
		reason = ""
	}
	e.Enabled = enabled
	e.DisabledReason = reason
	e.UpdatedAt = time.Now().UTC()
	f.rows[id] = e
	return redact(e), nil
}

func (f *fakeEndpointStore) DeleteWebhookEndpoint(_ context.Context, tenantID, id string) error {
	e, ok := f.rows[id]
	if !ok || e.TenantID != tenantID {
		return store.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeEndpointStore) ListWebhookDeliveries(_ context.Context, endpointID string, afterAt time.Time, afterKey string, limit int) ([]store.WebhookDelivery, error) {
	all := slices.Clone(f.deliveries[endpointID])
	slices.SortFunc(all, func(a, b store.WebhookDelivery) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	var out []store.WebhookDelivery
	for _, d := range all {
		if !afterAt.IsZero() {
			if d.CreatedAt.After(afterAt) || (d.CreatedAt.Equal(afterAt) && d.ID >= afterKey) {
				continue
			}
		}
		out = append(out, d)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// fakeWorkspaceResolver resolves every caller to a fixed tenant, for the
// cross-workspace scoping test.
type fakeWorkspaceResolver struct{ tenant string }

func (f fakeWorkspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return f.tenant, true
}

func (f fakeWorkspaceResolver) IsMember(context.Context, core.Identity, string) (bool, error) {
	return true, nil
}

func newTestService() (*Service, *fakeEndpointStore) {
	st := newFakeEndpointStore()
	return &Service{Base: &core.Base{Namespace: "default"}, Store: st}, st
}

// --- tests ---

func TestCreateReturnsTheSecretOnceAndReadsNeverDo(t *testing.T) {
	s, _ := newTestService()
	ctx := context.Background()

	created, err := s.Create(ctx, CreateRequest{URL: "https://example.com/hook", EventTypes: []string{TypeDeployStarted, TypeDeployEnded}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Secret == "" || !Verify(created.Secret, "m", "1614265330", []byte("b"), Sign(created.Secret, "m", time.Unix(1614265330, 0), []byte("b"))) {
		t.Errorf("Create must return a usable signing secret, got %q", created.Secret)
	}
	if created.Name != "https://example.com/hook" {
		t.Errorf("empty name should default to the URL, got %q", created.Name)
	}

	list, err := s.List(ctx, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v (err %v)", list, err)
	}
	if list[0].Secret != "" {
		t.Errorf("List leaked the secret: %q", list[0].Secret)
	}
	got, err := s.Get(ctx, "", created.ID)
	if err != nil || got.Secret != "" {
		t.Errorf("Get leaked the secret: %+v (err %v)", got, err)
	}
}

func TestCreateValidatesURLAndEventTypes(t *testing.T) {
	s, _ := newTestService()
	ctx := context.Background()

	for _, tc := range []CreateRequest{
		{URL: "not-a-url", EventTypes: []string{TypeDeployStarted}},
		{URL: "ftp://example.com/x", EventTypes: []string{TypeDeployStarted}},
		{URL: "", EventTypes: []string{TypeDeployStarted}},
		{URL: "https://example.com/hook", EventTypes: nil},
		{URL: "https://example.com/hook", EventTypes: []string{"no_such_event"}},
	} {
		if _, err := s.Create(ctx, tc); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("Create(%+v) = %v, want ErrBadRequest", tc, err)
		}
	}
}

func TestCreateDeduplicatesEventTypesInCanonicalOrder(t *testing.T) {
	s, _ := newTestService()
	created, err := s.Create(context.Background(), CreateRequest{
		URL:        "https://example.com/hook",
		EventTypes: []string{TypeServiceResumed, TypeDeployStarted, TypeServiceResumed},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	want := []string{TypeDeployStarted, TypeServiceResumed}
	if !slices.Equal(created.EventTypes, want) {
		t.Errorf("EventTypes = %v, want %v", created.EventTypes, want)
	}
}

func TestCrossWorkspaceAccessIsNotFound(t *testing.T) {
	st := newFakeEndpointStore()
	mine := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspaceResolver{"tea-mine"}}, Store: st}
	other := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspaceResolver{"tea-other"}}, Store: st}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "u1", Method: "session"})

	created, err := mine.Create(ctx, CreateRequest{URL: "https://example.com/hook", EventTypes: []string{TypeDeployEnded}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := other.Get(ctx, "", created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-workspace Get = %v, want ErrNotFound", err)
	}
	if err := other.Delete(ctx, "", created.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-workspace Delete = %v, want ErrNotFound", err)
	}
	if _, err := other.SetEnabled(ctx, "", created.ID, false); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-workspace SetEnabled = %v, want ErrNotFound", err)
	}
	if _, err := other.ListDeliveries(ctx, "", created.ID, "", 0); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-workspace ListDeliveries = %v, want ErrNotFound", err)
	}
	if list, err := other.List(ctx, ""); err != nil || len(list) != 0 {
		t.Errorf("cross-workspace List = %+v (err %v), want empty", list, err)
	}
}

func TestStoreOffReports503Sentinel(t *testing.T) {
	s := &Service{Base: &core.Base{Namespace: "default"}}
	ctx := context.Background()
	if _, err := s.Create(ctx, CreateRequest{URL: "https://example.com/h", EventTypes: []string{TypeDeployStarted}}); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("Create = %v, want ErrWebhooksUnavailable", err)
	}
	if _, err := s.List(ctx, ""); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("List = %v, want ErrWebhooksUnavailable", err)
	}
	if _, err := s.ListDeliveries(ctx, "", "whk-x", "", 0); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("ListDeliveries = %v, want ErrWebhooksUnavailable", err)
	}
}

func TestSetEnabledClearsTheDisabledReason(t *testing.T) {
	s, st := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{URL: "https://example.com/hook", EventTypes: []string{TypeDeployEnded}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Simulate the worker's auto-disable, then a manual re-enable.
	e := st.rows[created.ID]
	e.Enabled, e.DisabledReason = false, disabledReason
	st.rows[created.ID] = e

	v, err := s.SetEnabled(ctx, "", created.ID, true)
	if err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if !v.Enabled || v.DisabledReason != "" {
		t.Errorf("re-enabled endpoint = %+v, want enabled with no disabled reason", v)
	}
}

func TestListDeliveriesPagesNewestFirstByCursor(t *testing.T) {
	s, st := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{URL: "https://example.com/hook", EventTypes: []string{TypeDeployEnded}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	base := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	for i := range 5 {
		st.deliveries[created.ID] = append(st.deliveries[created.ID], store.WebhookDelivery{
			ID: ids.New(ids.WebhookDelivery), EndpointID: created.ID,
			EventID: "evt-x", EventType: TypeDeployEnded,
			CreatedAt: base.Add(time.Duration(i) * time.Minute), NextAttemptAt: base,
		})
	}

	page1, err := s.ListDeliveries(ctx, "", created.ID, "", 3)
	if err != nil || len(page1) != 3 {
		t.Fatalf("page1 = %d items (err %v), want 3", len(page1), err)
	}
	if page1[0].CreatedAt < page1[2].CreatedAt {
		t.Errorf("page1 not newest-first: %+v", page1)
	}
	page2, err := s.ListDeliveries(ctx, "", created.ID, page1[2].Cursor, 3)
	if err != nil || len(page2) != 2 {
		t.Fatalf("page2 = %d items (err %v), want 2", len(page2), err)
	}
	if _, err := s.ListDeliveries(ctx, "", created.ID, "garbage-cursor", 3); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("malformed cursor = %v, want ErrBadRequest", err)
	}
}
