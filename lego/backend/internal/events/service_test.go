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
	rows     []store.ServiceEventRow
	got      store.ServiceEventFilter
	gotOwner string
}

func (f *fakeStore) ListServiceEvents(_ context.Context, _, _, ownerWorkspace string, fil store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	f.gotOwner = ownerWorkspace
	f.got = fil
	return f.rows, nil
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
		name:     "env-var write carries neither key nor value",
		row:      store.ServiceEventRow{Key: "aud-7:", Source: store.EventSourceAudit, Verb: "secrets.SetEnvVar", Caller: "user-x"},
		wantType: TypeEnvVarsChanged,
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
				got.Details.TriggeredByUser != tc.wantDetails.TriggeredByUser {
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
		{TypeSourceChanged, []string{"apps.SetSource"}, nil},
		// One type, four verbs — all must reach the query, or an env-var delete (or
		// a blueprint seed) would silently vanish from an env_vars_changed filter.
		{TypeEnvVarsChanged, []string{"secrets.DeleteEnvVar", "secrets.SeedEnvVars", "secrets.SetEnvVar", "secrets.SetEnvVars"}, nil},
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
