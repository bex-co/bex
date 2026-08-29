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
	"fmt"
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
	deliveries map[string][]store.WebhookAttempt
}

type recordingWebhookAudit struct {
	events []core.AuditEvent
}

func (r *recordingWebhookAudit) Record(_ context.Context, event core.AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

func newFakeEndpointStore() *fakeEndpointStore {
	return &fakeEndpointStore{rows: map[string]store.WebhookEndpoint{}, deliveries: map[string][]store.WebhookAttempt{}}
}

// redact mirrors the real store's reads, which never select the secret column.
func redact(e store.WebhookEndpoint) store.WebhookEndpoint {
	e.Secret = ""
	return e
}

func (f *fakeEndpointStore) CreateWebhookEndpoint(_ context.Context, tenantID, name, url, secret string, eventTypes []string, enabled bool, createdBy string) (store.WebhookEndpoint, error) {
	for _, existing := range f.rows {
		if existing.TenantID == tenantID && strings.EqualFold(existing.Name, name) {
			return store.WebhookEndpoint{}, store.ErrConflict
		}
	}
	now := time.Now().UTC()
	e := store.WebhookEndpoint{
		ID: ids.New(ids.Webhook), TenantID: tenantID, Name: name, URL: url, Secret: secret,
		EventTypes: eventTypes, Enabled: enabled, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	f.rows[e.ID] = e
	return e, nil
}

func (f *fakeEndpointStore) ListWebhookEndpoints(_ context.Context, tenantIDs []string, afterAt time.Time, afterKey string, limit int) ([]store.WebhookEndpoint, error) {
	var out []store.WebhookEndpoint
	for _, e := range f.rows {
		if slices.Contains(tenantIDs, e.TenantID) {
			out = append(out, redact(e))
		}
	}
	slices.SortFunc(out, func(a, b store.WebhookEndpoint) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	var page []store.WebhookEndpoint
	for _, e := range out {
		if !afterAt.IsZero() && (e.CreatedAt.After(afterAt) || (e.CreatedAt.Equal(afterAt) && e.ID >= afterKey)) {
			continue
		}
		page = append(page, e)
		if len(page) == limit {
			break
		}
	}
	return page, nil
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

func (f *fakeEndpointStore) UpdateWebhookEndpoint(_ context.Context, tenantID, id, name, url string, eventTypes []string, enabled bool) (store.WebhookEndpoint, error) {
	e, ok := f.rows[id]
	if !ok || e.TenantID != tenantID {
		return store.WebhookEndpoint{}, store.ErrNotFound
	}
	for otherID, existing := range f.rows {
		if otherID != id && existing.TenantID == tenantID && strings.EqualFold(existing.Name, name) {
			return store.WebhookEndpoint{}, store.ErrConflict
		}
	}
	e.Name = name
	e.URL = url
	e.EventTypes = eventTypes
	if enabled {
		e.DisabledReason = ""
	}
	e.Enabled = enabled
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
	delete(f.deliveries, id)
	return nil
}

func (f *fakeEndpointStore) ListWebhookAttempts(_ context.Context, filter store.WebhookAttemptFilter) ([]store.WebhookAttempt, error) {
	all := slices.Clone(f.deliveries[filter.EndpointID])
	slices.SortFunc(all, func(a, b store.WebhookAttempt) int {
		if a.SentAt == nil || b.SentAt == nil {
			return 0
		}
		if c := b.SentAt.Compare(*a.SentAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	var out []store.WebhookAttempt
	for _, d := range all {
		if d.SentAt == nil || (!filter.SentAfter.IsZero() && !d.SentAt.After(filter.SentAfter)) || (!filter.SentBefore.IsZero() && !d.SentAt.Before(filter.SentBefore)) {
			continue
		}
		if filter.Status != "" && d.Status != filter.Status {
			continue
		}
		if !filter.AfterAt.IsZero() {
			if d.SentAt.After(filter.AfterAt) || (d.SentAt.Equal(filter.AfterAt) && d.ID >= filter.AfterKey) {
				continue
			}
		}
		out = append(out, d)
		if len(out) == core.PageLimitOrAbsent(filter.Limit) {
			break
		}
	}
	return out, nil
}

func (f *fakeEndpointStore) QueueWebhookResend(_ context.Context, request store.WebhookResendRequest) (store.WebhookAttempt, error) {
	endpoint, ok := f.rows[request.EndpointID]
	if !ok || endpoint.TenantID != request.TenantID {
		return store.WebhookAttempt{}, store.ErrWebhookEndpointNotFound
	}
	var source *store.WebhookAttempt
	for i := range f.deliveries[request.EndpointID] {
		attempt := &f.deliveries[request.EndpointID][i]
		if attempt.Origin == store.WebhookAttemptManual && attempt.IdempotencyKey == request.IdempotencyKey {
			return *attempt, nil
		}
		if attempt.ID == request.SourceAttemptID {
			source = attempt
		}
		if attempt.Status == store.WebhookAttemptPending {
			return store.WebhookAttempt{}, store.ErrWebhookAttemptPending
		}
	}
	if !endpoint.Enabled {
		return store.WebhookAttempt{}, store.ErrWebhookEndpointDisabled
	}
	if source == nil {
		return store.WebhookAttempt{}, store.ErrNotFound
	}
	if source.Status == store.WebhookAttemptPending {
		return store.WebhookAttempt{}, store.ErrWebhookAttemptPending
	}
	next := request.RequestedAt
	attempt := store.WebhookAttempt{
		ID: ids.New(ids.WebhookDelivery), NotificationID: source.NotificationID,
		EndpointID: request.EndpointID, EventID: source.EventID, EventType: source.EventType, ServiceID: source.ServiceID,
		AttemptNumber: source.AttemptNumber + 1, Status: store.WebhookAttemptPending, Payload: source.Payload,
		Origin: store.WebhookAttemptManual, RequestedBy: request.RequestedBy, IdempotencyKey: request.IdempotencyKey,
		ParentStatus: source.ParentStatus, NextAttemptAt: &next, CreatedAt: request.RequestedAt,
	}
	f.deliveries[request.EndpointID] = append(f.deliveries[request.EndpointID], attempt)
	return attempt, nil
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

	created, err := s.Create(ctx, CreateRequest{Name: "primary", URL: "https://example.com/hook", EventTypes: []string{TypeDeployStarted, TypeDeployEnded}, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Secret == "" || !Verify(created.Secret, "m", "1614265330", []byte("b"), Sign(created.Secret, "m", time.Unix(1614265330, 0), []byte("b"))) {
		t.Errorf("Create must return a usable signing secret, got %q", created.Secret)
	}
	if created.Name != "primary" {
		t.Errorf("name = %q, want primary", created.Name)
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
		{Name: "x", URL: "not-a-url", EventTypes: []string{TypeDeployStarted}},
		{Name: "x", URL: "ftp://example.com/x", EventTypes: []string{TypeDeployStarted}},
		{Name: "x", URL: "http://example.com/x", EventTypes: []string{TypeDeployStarted}},
		{Name: "x", URL: "", EventTypes: []string{TypeDeployStarted}},
		{Name: "", URL: "https://example.com/hook", EventTypes: []string{TypeDeployStarted}},
		{Name: "x", URL: "https://example.com/hook", EventTypes: []string{"no_such_event"}},
		// Round-13 #7: embedded userinfo is refused outright — the repo-URL
		// invariant; a credential in the URL would be echoed to every viewer.
		{Name: "x", URL: "https://key:secret@hooks.slack.com/services/T000/B000/x", EventTypes: []string{TypeDeployStarted}},
		// Round-15 #2: an HTTPS URL still has to fit the destination length cap.
		{Name: "x", URL: "https://example.com/" + strings.Repeat("a", store.MaxWebhookURLBytes), EventTypes: []string{TypeDeployStarted}},
	} {
		if _, err := s.Create(ctx, tc); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("Create(%+v) = %v, want ErrBadRequest", tc, err)
		}
	}
}

// manageChecker models the workspace roles for the URL-redaction boundary:
// can_view for everyone, can_manage only for the admin subject.
type manageChecker struct{ admin string }

func (c manageChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	if relation == core.RelCanView {
		return true, nil
	}
	return relation == core.RelCanManage && subject == "user:"+c.admin, nil
}

// TestDestinationURLRedactedForNonAdminReaders (round-13 #7): the exact
// destination URL carries the integration capability (Slack/T000/B000/xxx,
// ?token=…), but list/get are member reads — so ordinary members see the
// origin only, while can_manage callers and the admin-gated write verbs still
// see exactly what was configured. The delivery worker reads the stored row,
// never this projection.
func TestDestinationURLRedactedForNonAdminReaders(t *testing.T) {
	const exact = "https://hooks.slack.com/services/T000/B000/0123456789abcdef"
	st := newFakeEndpointStore()
	viewer := &Service{Base: &core.Base{
		Namespace: "default", Workspace: fakeWorkspaceResolver{"tea-a"}, Authz: manageChecker{admin: "id-admin"},
	}, Store: st}
	admin := &Service{Base: &core.Base{
		Namespace: "default", Workspace: fakeWorkspaceResolver{"tea-a"}, Authz: manageChecker{admin: "id-admin"},
	}, Store: st}
	adminCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-admin", Method: "session"})
	viewerCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-viewer", Method: "session"})

	created, err := admin.Create(adminCtx, CreateRequest{Name: "slack", URL: exact, EventTypes: []string{TypeDeployEnded}, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.URL != exact {
		t.Fatalf("Create response URL = %q, want the exact destination (admin-gated verb)", created.URL)
	}

	if got, err := viewer.Get(viewerCtx, "", created.ID); err != nil {
		t.Fatalf("viewer Get: %v", err)
	} else if got.URL != "https://hooks.slack.com/…" {
		t.Fatalf("viewer Get URL = %q, want the redacted origin", got.URL)
	}
	list, err := viewer.List(viewerCtx, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("viewer List = %+v (err %v)", list, err)
	}
	if list[0].URL != "https://hooks.slack.com/…" {
		t.Fatalf("viewer List URL = %q, want the redacted origin", list[0].URL)
	}
	if got, err := admin.Get(adminCtx, "", created.ID); err != nil {
		t.Fatalf("admin Get: %v", err)
	} else if got.URL != exact {
		t.Fatalf("admin Get URL = %q, want the exact destination", got.URL)
	}
}

// TestDestinationURLRedactedForReadOnlyOAuthAdmin (capability composition on
// the audit-silent reveal): mayManageWorkspace composes can_manage's mapped
// OAuth capability, so a third-party human token delegated only bex.read by an
// admin gets the REDACTED origin-only URL on list/get even though OpenFGA still
// grants the admin's can_manage. bex.write — or a capability-exempt session /
// machine identity (covered by the admin leg of the test above) — reveals the
// exact destination.
func TestDestinationURLRedactedForReadOnlyOAuthAdmin(t *testing.T) {
	const exact = "https://hooks.slack.com/services/T000/B000/0123456789abcdef"
	const redacted = "https://hooks.slack.com/…"
	st := newFakeEndpointStore()
	s := &Service{Base: &core.Base{
		Namespace: "default", Workspace: fakeWorkspaceResolver{"tea-a"}, Authz: manageChecker{admin: "id-admin"},
	}, Store: st}
	oauthAdmin := func(scopes string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{
			Subject: "id-admin", Method: "oauth2", Human: true, CanonicalScopes: scopes,
		})
	}

	created, err := s.Create(oauthAdmin(core.ScopeWrite), CreateRequest{
		Name: "slack", URL: exact, EventTypes: []string{TypeDeployEnded}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	readOnly := oauthAdmin(core.ScopeRead)
	if got, err := s.Get(readOnly, "", created.ID); err != nil {
		t.Fatalf("read-only admin Get: %v", err)
	} else if got.URL != redacted {
		t.Fatalf("read-only admin Get URL = %q, want the redacted origin %q", got.URL, redacted)
	}
	list, err := s.List(readOnly, "")
	if err != nil || len(list) != 1 {
		t.Fatalf("read-only admin List = %+v (err %v)", list, err)
	}
	if list[0].URL != redacted {
		t.Fatalf("read-only admin List URL = %q, want the redacted origin %q", list[0].URL, redacted)
	}

	withWrite := oauthAdmin(core.ScopeRead + " " + core.ScopeWrite)
	if got, err := s.Get(withWrite, "", created.ID); err != nil {
		t.Fatalf("write-scoped admin Get: %v", err)
	} else if got.URL != exact {
		t.Fatalf("write-scoped admin Get URL = %q, want the exact destination", got.URL)
	}
}

func TestCreateSupportsDisabledAllEventsAndRequiresUniqueName(t *testing.T) {
	s, _ := newTestService()
	ctx := context.Background()

	created, err := s.Create(ctx, CreateRequest{
		Name: "All Events", URL: "https://example.com/all", EventTypes: []string{}, Enabled: false,
	})
	if err != nil {
		t.Fatalf("Create disabled all-events: %v", err)
	}
	if created.Enabled || created.EventTypes == nil || len(created.EventTypes) != 0 {
		t.Fatalf("created = %+v, want disabled with explicit empty all-events filter", created)
	}

	_, err = s.Create(ctx, CreateRequest{
		Name: " all events ", URL: "https://example.com/duplicate", EventTypes: []string{}, Enabled: true,
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != WebhookNameConflictCode || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate name error = %#v, want %s conflict", err, WebhookNameConflictCode)
	}
}

func TestListProjectsLatestAttemptWithoutAHistoryLookup(t *testing.T) {
	s, st := newTestService()
	created, err := s.Create(context.Background(), CreateRequest{
		Name: "latest", URL: "https://example.com/latest", EventTypes: []string{}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	row := st.rows[created.ID]
	row.LatestAttemptStatus = store.WebhookAttemptFailed
	row.LatestAttemptAt = &at
	row.LatestParentStatus = store.WebhookAttemptPending
	st.rows[created.ID] = row

	views, err := s.List(context.Background(), "")
	if err != nil || len(views) != 1 {
		t.Fatalf("List = %+v, %v", views, err)
	}
	if views[0].LatestStatus != DeliveryFailed || views[0].LatestParentStatus != DeliveryPending ||
		views[0].LatestSentAt != at.Format(time.RFC3339) {
		t.Fatalf("latest projection = %+v", views[0])
	}
}

func TestUpdateDistinguishesOmittedAndExplicitEmptyEventTypes(t *testing.T) {
	s, _ := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{
		Name: "events", URL: "https://example.com/events", EventTypes: []string{TypeDeployStarted}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	omitted, err := s.Update(ctx, "", created.ID, UpdateRequest{})
	if err != nil || !slices.Equal(omitted.EventTypes, []string{TypeDeployStarted}) {
		t.Fatalf("omitted filter update = %+v, %v", omitted, err)
	}
	empty := []string{}
	all, err := s.Update(ctx, "", created.ID, UpdateRequest{EventTypes: &empty})
	if err != nil || all.EventTypes == nil || len(all.EventTypes) != 0 {
		t.Fatalf("explicit empty filter update = %+v, %v", all, err)
	}

	blank := "   "
	if _, err := s.Update(ctx, "", created.ID, UpdateRequest{Name: &blank}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("blank rename = %v, want bad request", err)
	}
}

func TestMaintenanceEventTypesAreSubscribableAndMapped(t *testing.T) {
	for verb, want := range map[string]string{
		"apps.SetMaintenanceMode":    TypeMaintenanceModeEnabled,
		"apps.SetMaintenanceModeURI": TypeMaintenanceModeURIUpdated,
	} {
		if got := verbEvents[verb]; got != want {
			t.Errorf("verbEvents[%q] = %q, want %q", verb, got, want)
		}
		if !slices.Contains(EventTypes, want) {
			t.Errorf("EventTypes does not contain %q: %v", want, EventTypes)
		}
	}
	svc, _ := newTestService()
	created, err := svc.Create(context.Background(), CreateRequest{
		Name:       "maintenance",
		URL:        "https://example.com/maintenance-hook",
		EventTypes: []string{TypeMaintenanceModeURIUpdated, TypeMaintenanceModeEnabled},
		Enabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(created.EventTypes, []string{TypeMaintenanceModeEnabled, TypeMaintenanceModeURIUpdated}) {
		t.Fatalf("maintenance subscriptions = %v", created.EventTypes)
	}
}

func TestServiceMovedEventTypeIsSubscribableAndMapped(t *testing.T) {
	for _, verb := range []string{core.AuditVerbProjectServiceMoved, core.AuditVerbEnvironmentServiceMoved} {
		if got := verbEvents[verb]; got != TypeServiceMoved {
			t.Errorf("verbEvents[%q] = %q, want %q", verb, got, TypeServiceMoved)
		}
		if !slices.Contains(auditVerbs, verb) {
			t.Errorf("auditVerbs does not push %q down into the feed query", verb)
		}
	}
	if !slices.Contains(EventTypes, TypeServiceMoved) {
		t.Errorf("EventTypes does not contain %q: %v", TypeServiceMoved, EventTypes)
	}
	svc, _ := newTestService()
	created, err := svc.Create(context.Background(), CreateRequest{
		Name:       "moves",
		URL:        "https://example.com/moves-hook",
		EventTypes: []string{TypeServiceMoved},
		Enabled:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(created.EventTypes, []string{TypeServiceMoved}) {
		t.Fatalf("move subscriptions = %v", created.EventTypes)
	}
}

func TestDatastoreEventTypesAreSubscribableAndMapped(t *testing.T) {
	wantByVerb := map[string]string{
		core.AuditVerbSetPlan:                    TypePlanChanged,
		core.AuditVerbPostgresCreated:            TypePostgresCreated,
		core.AuditVerbPostgresRestarted:          TypePostgresRestarted,
		core.AuditVerbPostgresCredentialsCreated: TypePostgresCredentialsCreated,
		core.AuditVerbPostgresCredentialsDeleted: TypePostgresCredentialsDeleted,
		core.AuditVerbPostgresBackupStarted:      TypePostgresBackupStarted,
		core.AuditVerbPostgresPlanChanged:        TypePlanChanged,
		core.AuditVerbKeyValuePlanChanged:        TypePlanChanged,
	}
	for verb, want := range wantByVerb {
		if got := verbEvents[verb]; got != want {
			t.Errorf("verbEvents[%q] = %q, want %q", verb, got, want)
		}
		if !slices.Contains(EventTypes, want) {
			t.Errorf("EventTypes does not contain %q: %v", want, EventTypes)
		}
	}
	for _, verb := range []string{core.AuditVerbPostgresUpdated, core.AuditVerbKeyValueUpdated} {
		if got, ok := verbEvents[verb]; ok {
			t.Errorf("unrelated datastore verb %q unexpectedly maps to %q", verb, got)
		}
	}
}

func TestExistingLifecycleAndAutoDeployEventTypesAreSubscribable(t *testing.T) {
	want := []string{
		TypeBranchDeleted,
		TypeBuildStarted,
		TypeBuildEnded,
		TypePreDeployStarted,
		TypePreDeployEnded,
		TypeJobRunEnded,
		TypeAutoDeployEnabled,
		TypeAutoDeployDisabled,
	}
	for _, eventType := range want {
		if !slices.Contains(EventTypes, eventType) {
			t.Errorf("EventTypes does not contain %q: %v", eventType, EventTypes)
		}
	}
}

func TestCronWebhookEventsComeFromObservedFactsNotIntentVerbs(t *testing.T) {
	for _, verb := range []string{"apps.TriggerCronRun", "apps.CancelCronRun", "apps.CancelCurrentCronRun"} {
		if eventType, ok := verbEvents[verb]; ok {
			t.Errorf("intent verb %q unexpectedly maps to %q", verb, eventType)
		}
		if slices.Contains(auditVerbs, verb) {
			t.Errorf("intent verb %q unexpectedly included in webhook query", verb)
		}
	}
	for factType, want := range map[store.ServiceEventFactType]string{
		store.EventFactCronRunStarted: TypeCronJobRunStarted,
		store.EventFactCronRunEnded:   TypeCronJobRunEnded,
	} {
		if got := factEvents[string(factType)]; got != want {
			t.Errorf("factEvents[%q] = %q, want %q", factType, got, want)
		}
	}
}

func TestCreateDeduplicatesEventTypesInCanonicalOrder(t *testing.T) {
	s, _ := newTestService()
	created, err := s.Create(context.Background(), CreateRequest{
		Name:       "dedupe",
		URL:        "https://example.com/hook",
		EventTypes: []string{TypeServiceResumed, TypeDeployStarted, TypeServiceResumed},
		Enabled:    true,
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

	created, err := mine.Create(ctx, CreateRequest{Name: "mine", URL: "https://example.com/hook", EventTypes: []string{TypeDeployEnded}, Enabled: true})
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
	if _, err := other.Resend(ctx, "", created.ID, "whd-source", "foreign-resend-0001"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-workspace Resend = %v, want ErrNotFound", err)
	}
	if list, err := other.List(ctx, ""); err != nil || len(list) != 0 {
		t.Errorf("cross-workspace List = %+v (err %v), want empty", list, err)
	}
}

func TestStoreOffReports503Sentinel(t *testing.T) {
	s := &Service{Base: &core.Base{Namespace: "default"}}
	ctx := context.Background()
	if _, err := s.Create(ctx, CreateRequest{Name: "x", URL: "https://example.com/h", EventTypes: []string{TypeDeployStarted}, Enabled: true}); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("Create = %v, want ErrWebhooksUnavailable", err)
	}
	if _, err := s.List(ctx, ""); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("List = %v, want ErrWebhooksUnavailable", err)
	}
	if _, err := s.ListDeliveries(ctx, "", "whk-x", "", 0); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("ListDeliveries = %v, want ErrWebhooksUnavailable", err)
	}
	if _, err := s.Resend(ctx, "", "whk-x", "whd-x", "store-off-resend-0001"); !errors.Is(err, core.ErrWebhooksUnavailable) {
		t.Errorf("Resend = %v, want ErrWebhooksUnavailable", err)
	}
}

func TestSetEnabledClearsTheDisabledReason(t *testing.T) {
	s, st := newTestService()
	ctx := context.Background()
	created, err := s.Create(ctx, CreateRequest{Name: "history", URL: "https://example.com/hook", EventTypes: []string{TypeDeployEnded}, Enabled: true})
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
	created, err := s.Create(ctx, CreateRequest{Name: "update", URL: "https://example.com/hook", EventTypes: []string{TypeDeployEnded}, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	base := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	for i := range 5 {
		st.deliveries[created.ID] = append(st.deliveries[created.ID], store.WebhookAttempt{
			ID: ids.New(ids.WebhookDelivery), EndpointID: created.ID,
			EventID: "evt-x", EventType: TypeDeployEnded,
			Status: store.WebhookAttemptDelivered, AttemptNumber: i + 1,
			CreatedAt: base.Add(time.Duration(i) * time.Minute), SentAt: func() *time.Time { at := base.Add(time.Duration(i) * time.Minute); return &at }(),
		})
	}

	page1, err := s.ListDeliveries(ctx, "", created.ID, "", 3)
	if err != nil || len(page1) != 3 {
		t.Fatalf("page1 = %d items (err %v), want 3", len(page1), err)
	}
	if page1[0].SentAt < page1[2].SentAt {
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

func TestListDeliveriesFiltersTerminalStatusBeforePaging(t *testing.T) {
	s, st := newTestService()
	created, err := s.Create(t.Context(), CreateRequest{Name: "filtered", URL: "https://example.com/hook", EventTypes: []string{}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	for i, status := range []string{store.WebhookAttemptDelivered, store.WebhookAttemptPending, store.WebhookAttemptFailed} {
		sent := base.Add(time.Duration(i) * time.Minute)
		row := store.WebhookAttempt{ID: fmt.Sprintf("whd-%d", i), EndpointID: created.ID, SentAt: &sent, CreatedAt: sent, Status: status}
		st.deliveries[created.ID] = append(st.deliveries[created.ID], row)
	}

	rows, err := s.ListDeliveriesFiltered(t.Context(), "", created.ID, DeliveryFilter{Status: DeliveryFailed, Limit: 1})
	if err != nil || len(rows) != 1 || rows[0].Status != DeliveryFailed {
		t.Fatalf("failed history = %+v (err %v)", rows, err)
	}
	if _, err := s.ListDeliveriesFiltered(t.Context(), "", created.ID, DeliveryFilter{Status: "pending"}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid terminal status = %v, want bad request", err)
	}
}

func TestListDeliveriesPreservesEachAttemptAndParentRetryState(t *testing.T) {
	s, st := newTestService()
	created, err := s.Create(t.Context(), CreateRequest{Name: "attempts", URL: "https://example.com/hook", EventTypes: []string{}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	next := base.Add(10 * time.Minute)
	for i, statusCode := range []int{500, 502, 204} {
		sent := base.Add(time.Duration(i) * time.Minute)
		status := store.WebhookAttemptFailed
		parentStatus := store.WebhookAttemptPending
		if statusCode == 204 {
			status = store.WebhookAttemptDelivered
			parentStatus = "delivered"
		}
		st.deliveries[created.ID] = append(st.deliveries[created.ID], store.WebhookAttempt{
			ID: fmt.Sprintf("whd-attempt-%d", i+1), NotificationID: "whd-parent", EndpointID: created.ID,
			EventID: "evt-stable", EventType: TypeDeployEnded, ServiceID: "srv-1",
			AttemptNumber: i + 1, Status: status, StatusCode: statusCode,
			ResponseBody: fmt.Sprintf("response-%d", i+1), Payload: `{"type":"deploy_ended"}`,
			SentAt: &sent, NextAttemptAt: &next, ParentStatus: parentStatus, CreatedAt: sent,
		})
	}

	rows, err := s.ListDeliveriesFiltered(t.Context(), "", created.ID, DeliveryFilter{Status: DeliveryFailed})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID == rows[1].ID {
		t.Fatalf("failed attempts = %+v", rows)
	}
	for _, row := range rows {
		if row.EventID != "evt-stable" || row.Status != DeliveryFailed || row.ParentStatus != DeliveryPending ||
			row.NextAttemptAt != next.Format(time.RFC3339) || row.RequestBody != `{"type":"deploy_ended"}` || row.Cursor == "" {
			t.Fatalf("attempt evidence = %+v", row)
		}
	}
}

func TestResendIsAuthorizedAuditedAndIdempotent(t *testing.T) {
	s, st := newTestService()
	created, err := s.Create(t.Context(), CreateRequest{Name: "resend", URL: "https://example.com/hook", EventTypes: []string{}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.deliveries[created.ID] = []store.WebhookAttempt{{
		ID: "whd-source", NotificationID: "whd-parent", EndpointID: created.ID,
		EventID: "evt-stable", EventType: TypeDeployEnded, ServiceID: "srv-1",
		AttemptNumber: 1, Status: store.WebhookAttemptFailed, StatusCode: 502,
		Payload: `{"type":"deploy_ended"}`, SentAt: &sent, ParentStatus: store.WebhookAttemptPending, CreatedAt: sent,
	}}
	audit := &recordingWebhookAudit{}
	s.Base.Authz = manageChecker{admin: "admin-1"}
	s.Base.Audit = audit
	s.Base.Clock = func() time.Time { return sent.Add(time.Minute) }
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "admin-1", Method: "session"})

	first, err := s.Resend(ctx, "", created.ID, "whd-source", "service-resend-0001")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Resend(ctx, "", created.ID, "whd-source", "service-resend-0001")
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	row.Enabled = false
	st.rows[created.ID] = row
	third, err := s.Resend(ctx, "", created.ID, "whd-source", "service-resend-0001")
	if err != nil {
		t.Fatalf("ambiguous duplicate after disable: %v", err)
	}
	if first.ID == "" || first.ID != second.ID || first.Status != DeliveryPending || first.EventID != "evt-stable" ||
		first.ID != third.ID || first.AttemptNumber != 2 || first.RequestBody != `{"type":"deploy_ended"}` {
		t.Fatalf("idempotent resend = first %+v second %+v third %+v", first, second, third)
	}
	if len(st.deliveries[created.ID]) != 2 {
		t.Fatalf("duplicate resend created %d attempts, want source + one manual", len(st.deliveries[created.ID]))
	}
	manual := st.deliveries[created.ID][1]
	if manual.Origin != store.WebhookAttemptManual || manual.RequestedBy != "admin-1" || manual.IdempotencyKey != "service-resend-0001" {
		t.Fatalf("manual attempt metadata = %+v", manual)
	}
	if len(audit.events) != 3 {
		t.Fatalf("audit events = %d, want one per authorized request", len(audit.events))
	}
	for _, event := range audit.events {
		if event.Verb != "webhooks.Resend" || event.Caller != "admin-1" || event.Outcome != core.AuditAllowed ||
			event.Target != core.WebhookAttemptTarget(created.ID, "whd-source") {
			t.Fatalf("resend audit = %+v", event)
		}
	}
	viewerCtx := core.WithIdentity(t.Context(), core.Identity{Subject: "viewer-1", Method: "session"})
	if _, err := s.Resend(viewerCtx, "", created.ID, "whd-source", "denied-resend-0001"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unauthorized resend = %v, want forbidden", err)
	}
	denied := audit.events[len(audit.events)-1]
	if denied.Verb != "webhooks.Resend" || denied.Caller != "viewer-1" || denied.Outcome != core.AuditDenied ||
		denied.Target != core.WebhookAttemptTarget(created.ID, "whd-source") {
		t.Fatalf("denied resend audit = %+v", denied)
	}
}

func TestResendReturnsStableSafeRefusals(t *testing.T) {
	s, st := newTestService()
	created, err := s.Create(t.Context(), CreateRequest{Name: "resend-errors", URL: "https://example.com/hook", EventTypes: []string{}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.deliveries[created.ID] = []store.WebhookAttempt{{
		ID: "whd-source", NotificationID: "whd-parent", EndpointID: created.ID,
		EventID: "evt-1", EventType: TypeDeployEnded, AttemptNumber: 1,
		Status: store.WebhookAttemptFailed, Payload: `{}`, SentAt: &sent, CreatedAt: sent,
	}}
	assertCode := func(err error, code string, class error) {
		t.Helper()
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != code || !errors.Is(err, class) {
			t.Fatalf("error = %#v, want %s wrapping %v", err, code, class)
		}
	}

	_, err = s.Resend(t.Context(), "", created.ID, "whd-source", "short")
	assertCode(err, WebhookResendIdempotencyKeyInvalidCode, core.ErrBadRequest)
	_, err = s.Resend(t.Context(), "", created.ID, "whd-missing", "missing-attempt-0001")
	assertCode(err, WebhookDeliveryNotFoundCode, core.ErrNotFound)
	_, err = s.Resend(t.Context(), "", "whk-missing", "whd-source", "missing-endpoint-0001")
	assertCode(err, WebhookEndpointNotFoundCode, core.ErrNotFound)

	row := st.rows[created.ID]
	row.Enabled = false
	st.rows[created.ID] = row
	_, err = s.Resend(t.Context(), "", created.ID, "whd-source", "disabled-endpoint-0001")
	assertCode(err, WebhookEndpointDisabledCode, core.ErrConflict)
	row.Enabled = true
	st.rows[created.ID] = row

	if _, err := s.Resend(t.Context(), "", created.ID, "whd-source", "pending-first-0001"); err != nil {
		t.Fatal(err)
	}
	_, err = s.Resend(t.Context(), "", created.ID, "whd-source", "pending-second-0002")
	assertCode(err, WebhookDeliveryPendingCode, core.ErrConflict)
}
