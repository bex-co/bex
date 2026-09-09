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
	"reflect"
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- harness ------------------------------------------------------------------

func fakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// sampleApp is store-managed (carries the app-id label the reconciler stamps)
// unless storeID is empty — the hand-applied case, which has no feed at all.
func sampleApp(name, storeID, tenant string) *appv1alpha1.App {
	a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{}}}
	if storeID != "" {
		a.Labels[store.LabelAppID] = storeID
	}
	if tenant != "" {
		a.Labels[core.LabelTenant] = tenant
	}
	return a
}

var now = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

type fakeStore struct {
	rows              []store.ServiceEventRow
	lookup            store.ServiceEventLookup
	lookupErr         error
	got               store.ServiceEventFilter
	gotOwner          string
	gotEventWorkspace string
	gotEventID        string
}

func (f *fakeStore) ListServiceEvents(_ context.Context, _, _, ownerWorkspace string, fil store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	f.gotOwner = ownerWorkspace
	f.got = fil
	return f.rows, nil
}

func (f *fakeStore) GetServiceEvent(_ context.Context, workspaceID, eventID string) (store.ServiceEventLookup, error) {
	f.gotEventWorkspace, f.gotEventID = workspaceID, eventID
	return f.lookup, f.lookupErr
}

func newService(st EventStore, objs ...client.Object) *Service {
	return &Service{Base: &core.Base{Client: fakeClient(objs...), Namespace: "default", Clock: func() time.Time { return now }}, Store: st}
}

// --- the vocabulary: every source row maps to the right event -----------------

func TestViewMapsEverySource(t *testing.T) {
	cases := []struct {
		name        string
		row         store.ServiceEventRow
		wantType    string
		wantDetails Details
	}{{
		name:        "deploy opened",
		row:         store.ServiceEventRow{Key: "dep-1:started", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerCreate},
		wantType:    TypeDeployStarted,
		wantDetails: Details{DeployID: "dep-1", Trigger: &Trigger{FirstBuild: true}},
	}, {
		name:        "deploy triggered through the API",
		row:         store.ServiceEventRow{Key: "dep-2:started", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-2", Trigger: store.TriggerAPI},
		wantType:    TypeDeployStarted,
		wantDetails: Details{DeployID: "dep-2", Trigger: &Trigger{Manual: true}},
	}, {
		name:        "deploy went live",
		row:         store.ServiceEventRow{Key: "dep-1:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, DeployID: "dep-1", Status: store.DeployLive},
		wantType:    TypeDeployEnded,
		wantDetails: Details{DeployID: "dep-1", DeployStatus: "succeeded"},
	}, {
		name:        "deploy failed",
		row:         store.ServiceEventRow{Key: "dep-3:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, DeployID: "dep-3", Status: store.DeployUpdateFailed},
		wantType:    TypeDeployEnded,
		wantDetails: Details{DeployID: "dep-3", DeployStatus: "failed"},
	}, {
		name:        "deploy failed on its pre-deploy step carries preDeployStatus (w1/m33)",
		row:         store.ServiceEventRow{Key: "dep-4:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, DeployID: "dep-4", Status: store.DeployUpdateFailed, PreDeployStatus: store.PreDeployFailed},
		wantType:    TypeDeployEnded,
		wantDetails: Details{DeployID: "dep-4", DeployStatus: "failed", PreDeployStatus: store.PreDeployFailed},
	}, {
		name:        "suspend names its actor",
		row:         store.ServiceEventRow{Key: "aud-1:", Source: store.EventSourceAudit, Verb: "apps.Suspend", Caller: "user-x"},
		wantType:    TypeSuspenderAdded,
		wantDetails: Details{Actor: "user-x"},
	}, {
		name:        "resume names its actor",
		row:         store.ServiceEventRow{Key: "aud-2:", Source: store.EventSourceAudit, Verb: "apps.Resume", Caller: "user-x"},
		wantType:    TypeSuspenderRemoved,
		wantDetails: Details{Actor: "user-x"},
	}, {
		name:        "restart names its actor in Render's field",
		row:         store.ServiceEventRow{Key: "aud-3:", Source: store.EventSourceAudit, Verb: "apps.Restart", Caller: "user-x"},
		wantType:    TypeServerRestarted,
		wantDetails: Details{TriggeredByUser: "user-x"},
	}, {
		name:     "scale carries no counts (structural redaction, not an oversight)",
		row:      store.ServiceEventRow{Key: "aud-4:", Source: store.EventSourceAudit, Verb: "apps.Scale", Caller: "user-x"},
		wantType: TypeInstanceCountChanged,
	}, {
		name:     "plan change carries no from/to",
		row:      store.ServiceEventRow{Key: "aud-5:", Source: store.EventSourceAudit, Verb: "apps.SetPlan", Caller: "user-x"},
		wantType: TypePlanChanged,
	}, {
		name:     "display-name change is recorded without leaking the label",
		row:      store.ServiceEventRow{Key: "aud-6:", Source: store.EventSourceAudit, Verb: "apps.SetDisplayName", Caller: "user-x"},
		wantType: TypeDisplayNameChanged,
	}, {
		name:     "command change is recorded without leaking the commands",
		row:      store.ServiceEventRow{Key: "aud-commands:", Source: store.EventSourceAudit, Verb: "apps.SetCommands", Caller: "user-x"},
		wantType: TypeCommandsChanged,
	}, {
		name:     "Dockerfile-path change is recorded without leaking the path",
		row:      store.ServiceEventRow{Key: "aud-dockerfile:", Source: store.EventSourceAudit, Verb: "apps.SetDockerfilePath", Caller: "user-x"},
		wantType: TypeDockerfilePathChanged,
	}, {
		name:     "source change is recorded without leaking the source",
		row:      store.ServiceEventRow{Key: "aud-source:", Source: store.EventSourceAudit, Verb: "apps.SetSource", Caller: "user-x"},
		wantType: TypeSourceChanged,
	}, {
		name:     "maintenance toggle uses Render vocabulary without leaking its value",
		row:      store.ServiceEventRow{Key: "aud-maintenance:", Source: store.EventSourceAudit, Verb: "apps.SetMaintenanceMode", Caller: "user-x"},
		wantType: TypeMaintenanceModeEnabled,
	}, {
		name:     "maintenance URI edit is a distinct event",
		row:      store.ServiceEventRow{Key: "aud-maintenance-uri:", Source: store.EventSourceAudit, Verb: "apps.SetMaintenanceModeURI", Caller: "user-x"},
		wantType: TypeMaintenanceModeURIUpdated,
	}, {
		name:     "env-var write carries neither key nor value",
		row:      store.ServiceEventRow{Key: "aud-7:", Source: store.EventSourceAudit, Verb: "secrets.SetEnvVar", Caller: "user-x"},
		wantType: TypeEnvVarsChanged,
	}, {
		name: "image pull fact carries only bounded failure details",
		row: store.ServiceEventRow{
			Key: "fact:image", Source: store.EventSourceFact, FactType: TypeImagePullFailed,
			DeployID: "dep-9", Image: "registry.example/web:bad", ReasonCode: store.EventReasonImagePullBackoff,
		},
		wantType: TypeImagePullFailed,
		wantDetails: Details{
			DeployID: "dep-9", Image: "registry.example/web:bad", ReasonCode: store.EventReasonImagePullBackoff,
		},
	}, {
		name: "autoscaling fact carries typed counts",
		row: func() store.ServiceEventRow {
			from, to := int32(1), int32(3)
			return store.ServiceEventRow{Key: "fact:scale", Source: store.EventSourceFact, FactType: TypeAutoscalingStarted, FromCount: &from, ToCount: &to}
		}(),
		wantType: TypeAutoscalingStarted,
		wantDetails: func() Details {
			from, to := int32(1), int32(3)
			return Details{FromCount: &from, ToCount: &to}
		}(),
	}, {
		name: "ignored commit fact carries no commit message",
		row: store.ServiceEventRow{
			Key: "fact:commit", Source: store.EventSourceFact, FactType: TypeCommitIgnored,
			CommitID: "abc123", CommitURL: "https://github.com/acme/web/commit/abc123", ReasonCode: store.EventReasonBuildFilter,
		},
		wantType: TypeCommitIgnored,
		wantDetails: Details{
			CommitID: "abc123", CommitURL: "https://github.com/acme/web/commit/abc123", ReasonCode: store.EventReasonBuildFilter,
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := view(tc.row, "web")
			if got.Type != tc.wantType {
				t.Errorf("type = %q, want %q", got.Type, tc.wantType)
			}
			if got.ServiceID != "web" {
				t.Errorf("serviceId = %q, want web", got.ServiceID)
			}
			if !ids.WellFormed(got.ID) {
				t.Errorf("id = %q, not a well-formed bex id", got.ID)
			}
			if kind, ok := ids.KindOf(got.ID); !ok || kind != ids.Event {
				t.Errorf("id = %q, want an evt- (Render-compatible) id", got.ID)
			}
			if got.Details.DeployID != tc.wantDetails.DeployID ||
				got.Details.DeployStatus != tc.wantDetails.DeployStatus ||
				got.Details.PreDeployStatus != tc.wantDetails.PreDeployStatus ||
				got.Details.Actor != tc.wantDetails.Actor ||
				got.Details.TriggeredByUser != tc.wantDetails.TriggeredByUser ||
				got.Details.Image != tc.wantDetails.Image ||
				got.Details.CommitID != tc.wantDetails.CommitID ||
				got.Details.CommitURL != tc.wantDetails.CommitURL ||
				got.Details.ReasonCode != tc.wantDetails.ReasonCode ||
				!equalInt32(got.Details.FromCount, tc.wantDetails.FromCount) ||
				!equalInt32(got.Details.ToCount, tc.wantDetails.ToCount) {
				t.Errorf("details = %+v, want %+v", got.Details, tc.wantDetails)
			}
			switch {
			case tc.wantDetails.Trigger == nil && got.Details.Trigger != nil:
				t.Errorf("trigger = %+v, want none (only deploy_started has one)", got.Details.Trigger)
			case tc.wantDetails.Trigger != nil && got.Details.Trigger == nil:
				t.Error("deploy_started must carry Render's trigger object")
			case tc.wantDetails.Trigger != nil && *got.Details.Trigger != *tc.wantDetails.Trigger:
				t.Errorf("trigger = %+v, want %+v", *got.Details.Trigger, *tc.wantDetails.Trigger)
			}
		})
	}
}

func equalInt32(a, b *int32) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// TestEventIDsAreDerivedNotMinted is the property the cursor and every client's
// dedupe depend on: the same source row is the same event, forever — and two
// different rows never collide.
func TestEventIDsAreDerivedNotMinted(t *testing.T) {
	a := view(store.ServiceEventRow{Key: "dep-1:started", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted}, "web")
	again := view(store.ServiceEventRow{Key: "dep-1:started", Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted}, "web")
	ended := view(store.ServiceEventRow{Key: "dep-1:ended", Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded}, "web")
	if a.ID != again.ID {
		t.Errorf("same row read twice = %s then %s; an event id must be stable", a.ID, again.ID)
	}
	if a.ID == ended.ID {
		t.Errorf("a deploy's start and end share the id %s; they are two events", a.ID)
	}
}

// --- cursor -------------------------------------------------------------------

func TestCursorRoundTrips(t *testing.T) {
	at := now.Add(123 * time.Nanosecond) // nanosecond precision must survive
	c := core.EncodeKeysetCursor(at, "aud-1:")
	got, err := core.DecodeKeysetCursor(c)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.At.Equal(at) || got.Key != "aud-1:" {
		t.Errorf("round-trip = %+v, want at=%s key=aud-1:", got, at)
	}
	if empty, err := core.DecodeKeysetCursor(""); err != nil || !empty.At.IsZero() {
		t.Errorf("empty cursor = %+v (err %v), want the head of the feed", empty, err)
	}
	for _, bad := range []string{"not-base64!!", "YWJj", "MjAyNi0wNy0xMHxrZXk"} { // junk, no separator, unparseable time
		if _, err := core.DecodeKeysetCursor(bad); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("core.DecodeKeysetCursor(%q) = %v, want core.ErrBadRequest (400)", bad, err)
		}
	}
}

// --- List ---------------------------------------------------------------------

func TestGetReturnsIndexedServiceEvent(t *testing.T) {
	eventID := ids.Derive(ids.Event, "dep-1:"+store.EventPhaseStarted)
	st := &fakeStore{lookup: store.ServiceEventLookup{
		Event:     store.ServiceEventRow{Key: "dep-1:" + store.EventPhaseStarted, At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: "dep-1", Trigger: store.TriggerAPI},
		ServiceID: "web",
	}}

	got, err := newService(st).Get(context.Background(), eventID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != eventID || got.Type != TypeDeployStarted || got.ServiceID != "web" {
		t.Errorf("Get = %+v, want the indexed deploy_started event", got)
	}
	if st.gotEventWorkspace != core.DefaultTenant || st.gotEventID != eventID {
		t.Errorf("store lookup = workspace %q id %q, want %q / %q", st.gotEventWorkspace, st.gotEventID, core.DefaultTenant, eventID)
	}
}

func TestGetReturnsIndexedPostgresEvent(t *testing.T) {
	eventID := ids.Derive(ids.Event, "aud-pg:")
	st := &fakeStore{lookup: store.ServiceEventLookup{
		Event:     store.ServiceEventRow{Key: "aud-pg:", At: now, Source: store.EventSourceAudit, Verb: core.AuditVerbPostgresCreated, Caller: "user-x"},
		ServiceID: "dpg-1",
	}}

	got, err := newService(st).Get(context.Background(), eventID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != eventID || got.Type != TypePostgresCreated || got.ServiceID != "dpg-1" {
		t.Errorf("Get = %+v, want the indexed postgres_created event", got)
	}
}

// TestGetReturnsIndexedDatastoreConfigEvents is w3/m82 t003's retrieval half:
// the value a datastore configuration effect recorded must come back in details
// under the same evt-… id a delivered webhook carried.
func TestGetReturnsIndexedDatastoreConfigEvents(t *testing.T) {
	enabled := true
	sizeGB := int32(40)
	policy, mode := "noeviction", "journal_snapshot"
	cases := []struct {
		name     string
		row      store.ServiceEventRow
		wantType string
		want     Details
	}{
		{
			name:     "ha status",
			row:      store.ServiceEventRow{Verb: core.AuditVerbPostgresHAChanged, HighAvailabilityEnabled: &enabled},
			wantType: TypePostgresHAStatusChanged,
			want:     Details{HighAvailabilityEnabled: &enabled},
		},
		{
			name:     "connection pool",
			row:      store.ServiceEventRow{Verb: core.AuditVerbPostgresPoolerChanged, ConnectionPoolEnabled: &enabled},
			wantType: TypePostgresConnectionPoolEnabledChanged,
			want:     Details{ConnectionPoolEnabled: &enabled},
		},
		{
			name:     "disk size",
			row:      store.ServiceEventRow{Verb: core.AuditVerbPostgresDiskSizeChanged, DiskSizeGB: &sizeGB},
			wantType: TypePostgresDiskSizeChanged,
			want:     Details{DiskSizeGB: &sizeGB},
		},
		{
			name:     "key value config restart",
			row:      store.ServiceEventRow{Verb: core.AuditVerbKeyValueConfigChanged, MaxmemoryPolicy: &policy, PersistenceMode: &mode},
			wantType: TypeKeyValueConfigRestart,
			want:     Details{MaxmemoryPolicy: &policy, PersistenceMode: &mode},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			row.Key, row.At, row.Source, row.Caller = "aud-cfg:", now, store.EventSourceAudit, "user-x"
			eventID := ids.Derive(ids.Event, row.Key)
			st := &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: "dpg-1"}}

			got, err := newService(st).Get(context.Background(), eventID)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != eventID || got.Type != tc.wantType || got.ServiceID != "dpg-1" {
				t.Fatalf("Get = %+v, want the indexed %s event", got, tc.wantType)
			}
			if !reflect.DeepEqual(got.Details, tc.want) {
				t.Errorf("details = %+v, want %+v", got.Details, tc.want)
			}
		})
	}
}

// TestGetReturnsObservedDatastoreFactEvents is w3/m82 t004's retrieval half for
// the reconciler-produced side of the vocabulary: an availability edge and a
// backup outcome both arrive as datastore_event_facts rows under
// EventSourceFact, and each must come back under the same evt-… id its webhook
// delivered — including the closed reason code an outage carries.
func TestGetReturnsObservedDatastoreFactEvents(t *testing.T) {
	cases := []struct {
		name     string
		row      store.ServiceEventRow
		service  string
		wantType string
		want     Details
	}{
		{
			name:     "postgres outage",
			row:      store.ServiceEventRow{FactType: TypePostgresUnavailable, ReasonCode: store.EventReasonReadinessFailed},
			service:  "dpg-1",
			wantType: TypePostgresUnavailable,
			want:     Details{ReasonCode: store.EventReasonReadinessFailed},
		},
		{
			name:     "key value recovery",
			row:      store.ServiceEventRow{FactType: TypeKeyValueAvailable},
			service:  "red-1",
			wantType: TypeKeyValueAvailable,
		},
		{
			name:     "backup outcome",
			row:      store.ServiceEventRow{FactType: TypePostgresBackupCompleted},
			service:  "dpg-1",
			wantType: TypePostgresBackupCompleted,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			row.Key, row.At, row.Source = "fact:observed:"+tc.service, now, store.EventSourceFact
			eventID := ids.Derive(ids.Event, row.Key)
			st := &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: tc.service}}

			got, err := newService(st).Get(context.Background(), eventID)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != eventID || got.Type != tc.wantType || got.ServiceID != tc.service {
				t.Fatalf("Get = %+v, want the indexed %s event", got, tc.wantType)
			}
			if !reflect.DeepEqual(got.Details, tc.want) {
				t.Errorf("details = %+v, want %+v", got.Details, tc.want)
			}
		})
	}
}

func TestGetErrorMatrix(t *testing.T) {
	validID := ids.Derive(ids.Event, "missing:")

	t.Run("malformed id", func(t *testing.T) {
		st := &fakeStore{}
		_, err := newService(st).Get(context.Background(), "not-an-event")
		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("Get malformed id = %v, want core.ErrBadRequest", err)
		}
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != EventIDInvalidCode {
			t.Errorf("Get malformed code = %v, want %s", err, EventIDInvalidCode)
		}
		if st.gotEventID != "" {
			t.Errorf("malformed id reached store as %q", st.gotEventID)
		}
	})

	t.Run("storeless", func(t *testing.T) {
		_, err := newService(nil).Get(context.Background(), validID)
		if !errors.Is(err, core.ErrEventsUnavailable) {
			t.Errorf("store-less Get = %v, want core.ErrEventsUnavailable", err)
		}
	})

	t.Run("missing is event-scoped not found", func(t *testing.T) {
		st := &fakeStore{lookupErr: store.ErrNotFound}
		_, err := newService(st).Get(context.Background(), validID)
		if !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("missing Get = %v, want core.ErrNotFound", err)
		}
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != EventNotFoundCode {
			t.Errorf("missing Get code = %v, want %s", err, EventNotFoundCode)
		}
	})

	t.Run("unmapped indexed row stays hidden", func(t *testing.T) {
		st := &fakeStore{lookup: store.ServiceEventLookup{
			Event:     store.ServiceEventRow{Key: "missing:", Source: store.EventSourceAudit, Verb: "unmapped.Verb"},
			ServiceID: "web",
		}}
		_, err := newService(st).Get(context.Background(), validID)
		if !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unmapped indexed event = %v, want core.ErrNotFound", err)
		}
	})
}

func TestListStoreLess503(t *testing.T) {
	svc := newService(nil, sampleApp("web", "srv-1", "tea-a"))
	if _, err := svc.List(context.Background(), "web", Filter{}); !errors.Is(err, core.ErrEventsUnavailable) {
		t.Errorf("store-less List = %v, want core.ErrEventsUnavailable (503, omitted not faked)", err)
	}
}

func TestListUnknownService404(t *testing.T) {
	svc := newService(&fakeStore{}, sampleApp("web", "srv-1", "tea-a"))
	if _, err := svc.List(context.Background(), "nope", Filter{}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown service = %v, want core.ErrNotFound", err)
	}
}

func TestListHandAppliedAppHasEmptyFeed(t *testing.T) {
	st := &fakeStore{rows: []store.ServiceEventRow{{Key: "aud-1:", Source: store.EventSourceAudit, Verb: "apps.Suspend"}}}
	svc := newService(st, sampleApp("manual", "", ""))
	got, err := svc.List(context.Background(), "manual", Filter{})
	if err != nil || len(got) != 0 {
		t.Errorf("hand-applied app = %+v (err %v), want an empty feed, never another service's rows", got, err)
	}
}

func TestListAppliesRenderDefaultWindow(t *testing.T) {
	st := &fakeStore{}
	svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
	if _, err := svc.List(context.Background(), "web", Filter{}); err != nil {
		t.Fatal(err)
	}
	if want := now.Add(-DefaultWindow); !st.got.Since.Equal(want) {
		t.Errorf("default Since = %s, want %s (Render's now-1h default for this endpoint)", st.got.Since, want)
	}
	// An explicit startTime wins.
	since := now.Add(-48 * time.Hour)
	if _, err := svc.List(context.Background(), "web", Filter{Since: since}); err != nil {
		t.Fatal(err)
	}
	if !st.got.Since.Equal(since) {
		t.Errorf("explicit Since = %s, want %s", st.got.Since, since)
	}
}

func TestListRejectsTooWideWindow(t *testing.T) {
	svc := newService(&fakeStore{}, sampleApp("web", "srv-1", "tea-a"))
	svc.MaxQueryHours = 24
	_, err := svc.List(context.Background(), "web", Filter{Since: now.Add(-25 * time.Hour)})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("25h window with a 24h cap = %v, want core.ErrBadRequest (400) — the logs/metrics bound", err)
	}
	if _, err := svc.List(context.Background(), "web", Filter{Since: now.Add(-23 * time.Hour)}); err != nil {
		t.Errorf("23h window with a 24h cap = %v, want it accepted", err)
	}
}

// TestListNamesTheServicesOwningWorkspace pins what the store needs to keep a
// stranger out of this feed: the tenant that OWNS the service. (The scoping rule
// itself lives in the store's one query — see store.serviceEventsQuery — so that
// a future reader of `target` inherits it instead of having to remember it.)
func TestListNamesTheServicesOwningWorkspace(t *testing.T) {
	st := &fakeStore{}
	svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
	if _, err := svc.List(context.Background(), "web", Filter{}); err != nil {
		t.Fatal(err)
	}
	if st.gotOwner != "tea-a" {
		t.Errorf("owner workspace = %q, want the App's tenant (tea-a) — the store scopes audit rows to it", st.gotOwner)
	}
	// The unfiltered query asks for every verb the vocabulary names and both deploy
	// transitions, so nothing the feed can render is left behind in SQL.
	if len(st.got.Verbs) != len(eventTypes) || len(st.got.Phases) != 2 {
		t.Errorf("unfiltered push-down = %d verbs / %d phases, want %d / 2", len(st.got.Verbs), len(st.got.Phases), len(eventTypes))
	}
}

// TestTypeFilterIsPushedDown is the property a paging client depends on: the type
// filter runs in SQL, BEFORE the LIMIT. Filtering the result in Go afterwards
// would return short — sometimes empty — pages, and an empty page is exactly how
// a cursor client is told the feed has ended, so it would stop early and miss
// every older matching event.
func TestTypeFilterIsPushedDown(t *testing.T) {
	cases := []struct {
		eventType  string
		wantVerbs  []string
		wantPhases []string
	}{
		{TypeDeployStarted, nil, []string{store.EventPhaseStarted}},
		{TypeDeployEnded, nil, []string{store.EventPhaseEnded}},
		{TypeSuspenderAdded, []string{"apps.Suspend"}, nil},
		{TypeCommandsChanged, []string{"apps.SetCommands"}, nil},
		{TypeDockerfilePathChanged, []string{"apps.SetDockerfilePath"}, nil},
		{TypeSourceChanged, []string{"apps.SetRegistryCredential", "apps.SetSource", "apps.SetSourceAndRegistryCredential"}, nil},
		// One type, four verbs — all must reach the query, or an env-var delete (or
		// a blueprint seed) would silently vanish from an env_vars_changed filter.
		{TypeEnvVarsChanged, []string{"secrets.DeleteEnvVar", "secrets.SeedEnvVars", "secrets.SetEnvVar", "secrets.SetEnvVars"}, nil},
		{TypeServiceEnvironmentChanged, []string{"secrets.PatchEnvironment"}, nil},
		// An unknown type asks the store for nothing at all: an empty feed, not a
		// page of zero items a client can't tell from the end of the feed.
		{"no_such_type", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			st := &fakeStore{}
			svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
			if _, err := svc.List(context.Background(), "web", Filter{Type: tc.eventType}); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(st.got.Verbs, tc.wantVerbs) {
				t.Errorf("verbs pushed down = %v, want %v", st.got.Verbs, tc.wantVerbs)
			}
			if !slices.Equal(st.got.Phases, tc.wantPhases) {
				t.Errorf("phases pushed down = %v, want %v", st.got.Phases, tc.wantPhases)
			}
		})
	}
}

// TestAutoDeployTypeFilterIsPushedDown verifies that the three auto-deploy event
// types push their SQL discrimination (auto_deploy_enabled = true/false/IS NULL)
// down into store.ServiceEventFilter.AutoDeploy rather than filtering in Go
// after the LIMIT — the same short-page hazard as the other type filters.
func TestAutoDeployTypeFilterIsPushedDown(t *testing.T) {
	cases := []struct {
		eventType      string
		wantAutoFilter store.AutoDeployFilter
	}{
		// Each type must push a distinct SQL predicate.
		{TypeAutoDeployEnabled, store.AutoDeployFilterEnabled},
		{TypeAutoDeployDisabled, store.AutoDeployFilterDisabled},
		{TypeAutoDeployChanged, store.AutoDeployFilterChanged},
	}
	for _, tc := range cases {
		t.Run(tc.eventType, func(t *testing.T) {
			st := &fakeStore{}
			svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
			if _, err := svc.List(context.Background(), "web", Filter{Type: tc.eventType}); err != nil {
				t.Fatal(err)
			}
			// The verb must be SetAutoDeploy — all three discriminate within the same verb.
			if !slices.Equal(st.got.Verbs, []string{"apps.SetAutoDeploy"}) {
				t.Errorf("verbs pushed down = %v, want [apps.SetAutoDeploy]", st.got.Verbs)
			}
			if st.got.AutoDeploy != tc.wantAutoFilter {
				t.Errorf("AutoDeploy filter = %v, want %v — filter runs in SQL before LIMIT, not in Go after", st.got.AutoDeploy, tc.wantAutoFilter)
			}
		})
	}
	// Unfiltered (type="") must carry AutoDeployFilterNone so all SetAutoDeploy
	// rows appear in the feed regardless of their auto_deploy_enabled value.
	t.Run("unfiltered passes all auto-deploy rows", func(t *testing.T) {
		st := &fakeStore{}
		svc := newService(st, sampleApp("web", "srv-1", "tea-a"))
		if _, err := svc.List(context.Background(), "web", Filter{}); err != nil {
			t.Fatal(err)
		}
		if st.got.AutoDeploy != store.AutoDeployFilterNone {
			t.Errorf("unfiltered AutoDeploy = %v, want AutoDeployFilterNone (no constraint)", st.got.AutoDeploy)
		}
	})
}
