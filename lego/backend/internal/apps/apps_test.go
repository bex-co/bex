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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func fakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 2},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + name + ".onbex.co"},
	}
}

func newService(st IntentStore, apps ...*appv1alpha1.App) (*Service, client.Client) {
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fakeClient(objs...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: st}, cl
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return &a
}

// --- Read + write verbs ---

func TestServiceListGetVerbs(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"), sampleApp("api"))

	list, err := svc.List(context.Background(), "")
	if err != nil || len(list) != 2 {
		t.Fatalf("List: %v len=%d", err, len(list))
	}

	v, err := svc.Get(context.Background(), "web")
	if err != nil || v.Name != "web" || v.Replicas != 2 {
		t.Fatalf("Get: %v %+v", err, v)
	}
	if _, err := svc.Get(context.Background(), "nope"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown => ErrNotFound, got %v", err)
	}

	// Suspend keeps replicas; Resume clears; Restart stamps restartedAt.
	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if a := getApp(t, cl, "web"); !a.Spec.Suspended || a.Spec.Replicas != 2 {
		t.Errorf("suspend must set suspended and keep replicas: %+v", a.Spec)
	}
	if _, err := svc.Resume(context.Background(), "web"); err != nil || getApp(t, cl, "web").Spec.Suspended {
		t.Errorf("resume should clear suspended: %v", err)
	}
	if _, err := svc.Restart(context.Background(), "web"); err != nil || getApp(t, cl, "web").Spec.RestartedAt == "" {
		t.Errorf("restart should stamp restartedAt: %v", err)
	}
}

// --- Workspace-scoped List/Get/Create (w1/m9) ---

// fakeWorkspace is a map-backed core.WorkspaceResolver: identities not in the
// map resolve ok=false — an unbound caller, or the store-on legacy path.
type fakeWorkspace map[string]string

func (f fakeWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

func tenantApp(name, tenantID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: tenantID}
	return a
}

func newTenantService(ws core.WorkspaceResolver, apps ...*appv1alpha1.App) (*Service, client.Client) {
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fakeClient(objs...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}}, cl
}

func TestListScopedToCallerTenant(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"},
		tenantApp("web", "tea-a"), tenantApp("db", "tea-a"), tenantApp("other", "tea-b"))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	list, err := svc.List(ctx, "")
	if err != nil || len(list) != 2 {
		t.Fatalf("List: %v len=%d, want 2 (tea-a's apps only)", err, len(list))
	}
	for _, a := range list {
		if a.Name != "web" && a.Name != "db" {
			t.Errorf("List leaked a cross-tenant App: %+v", a)
		}
	}
}

func TestListUnresolvedCallerSeesNothing(t *testing.T) {
	// Store on but the caller resolves to no tenant — must not silently fall
	// back to an unfiltered (every-tenant) list.
	svc, _ := newTenantService(fakeWorkspace{}, tenantApp("web", "tea-a"))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "unbound", Method: "oauth2"})

	list, err := svc.List(ctx, "")
	if err != nil || len(list) != 0 {
		t.Fatalf("List for unresolved caller: %v len=%d, want 0", err, len(list))
	}
}

func TestGetCrossTenantIsForbiddenNotNotFound(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, tenantApp("other", "tea-b"))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	if _, err := svc.Get(ctx, "other"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("cross-tenant Get: got %v, want ErrForbidden (not a 404 leak)", err)
	}
}

func TestGetSameTenantSucceeds(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, tenantApp("web", "tea-a"))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	if _, err := svc.Get(ctx, "web"); err != nil {
		t.Errorf("same-tenant Get: %v", err)
	}
}

func TestCreateStampsCallerTenantLabel(t *testing.T) {
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	if _, err := svc.create(ctx, CreateRequest{Name: "fresh", Image: "img:1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	a := getApp(t, cl, "fresh")
	if a.Labels[core.LabelTenant] != "tea-a" {
		t.Fatalf("created App tenant label = %q, want tea-a", a.Labels[core.LabelTenant])
	}
	// The caller must be able to see/manage what it just created — the whole
	// point of stamping the label on create.
	if _, err := svc.Get(ctx, "fresh"); err != nil {
		t.Errorf("Get on just-created App: %v (label must make it visible to its own owner)", err)
	}
}

func TestStoreOffListAndGetIgnoreTenantLabelsUnchanged(t *testing.T) {
	// Workspace nil (store off): byte-identical to before tenant onboarding —
	// every App in the namespace is visible regardless of label.
	svc, _ := newService(nil, tenantApp("web", "tea-a"), tenantApp("other", "tea-b"))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	list, err := svc.List(ctx, "")
	if err != nil || len(list) != 2 {
		t.Fatalf("store-off List: %v len=%d, want 2 (unfiltered)", err, len(list))
	}
	if _, err := svc.Get(ctx, "other"); err != nil {
		t.Errorf("store-off Get across tenants: %v, want success", err)
	}
}

// --- Single writer of intent (store-managed vs hand-applied) ---

type recordingStore struct {
	calls []struct {
		id        string
		suspended bool
	}
	tierCalls []struct {
		id   string
		tier string
	}
	replicasCalls []struct {
		id       string
		replicas int32
	}
	idleTTLCalls []struct {
		id      string
		seconds int32
	}
	domainAdds []struct{ id, host string }
	domainRems []struct{ id, host string }
	err        error
}

func (r *recordingStore) SetAppSuspended(_ context.Context, id string, suspended bool) error {
	if r.err != nil {
		return r.err
	}
	r.calls = append(r.calls, struct {
		id        string
		suspended bool
	}{id, suspended})
	return nil
}

func (r *recordingStore) SetAppTier(_ context.Context, id string, tier string) error {
	if r.err != nil {
		return r.err
	}
	r.tierCalls = append(r.tierCalls, struct {
		id   string
		tier string
	}{id, tier})
	return nil
}

func (r *recordingStore) SetAppReplicas(_ context.Context, id string, replicas int32) error {
	if r.err != nil {
		return r.err
	}
	r.replicasCalls = append(r.replicasCalls, struct {
		id       string
		replicas int32
	}{id, replicas})
	return nil
}

func (r *recordingStore) SetAppIdleTTL(_ context.Context, id string, seconds int32) error {
	if r.err != nil {
		return r.err
	}
	r.idleTTLCalls = append(r.idleTTLCalls, struct {
		id      string
		seconds int32
	}{id, seconds})
	return nil
}

func (r *recordingStore) AddDomain(_ context.Context, id, host string) error {
	if r.err != nil {
		return r.err
	}
	r.domainAdds = append(r.domainAdds, struct{ id, host string }{id, host})
	return nil
}

func (r *recordingStore) RemoveDomain(_ context.Context, id, host string) error {
	if r.err != nil {
		return r.err
	}
	r.domainRems = append(r.domainRems, struct{ id, host string }{id, host})
	return nil
}

func managedApp(name, appID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: appID}
	return a
}

func TestSuspendManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if _, err := svc.Resume(context.Background(), "web"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(rec.calls) != 2 || rec.calls[0].id != "srv-1" || !rec.calls[0].suspended ||
		rec.calls[1].id != "srv-1" || rec.calls[1].suspended {
		t.Fatalf("want row writes [srv-1 true, srv-1 false], got %v", rec.calls)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("CR should be resumed after the fast-path patch")
	}
}

func TestSuspendUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, sampleApp("hand"))

	if _, err := svc.Suspend(context.Background(), "hand"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.calls)
	}
	if !getApp(t, cl, "hand").Spec.Suspended {
		t.Fatal("CR should be suspended")
	}
}

func TestSuspendRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Suspend(context.Background(), "web"); err == nil {
		t.Fatal("want error when the row write fails")
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("CR must not be patched when the row write failed")
	}
}

func TestSuspendWithoutStorePatchesCR(t *testing.T) {
	svc, cl := newService(nil, managedApp("web", "srv-1"))
	if _, err := svc.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if !getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("CR should be suspended")
	}
}

// --- SetPlan (instance-tier changes) ---

func TestSetPlanValidatesAndMapsRenderSpelling(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	v, err := svc.SetPlan(context.Background(), "web", "pro_plus")
	if err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	if v.Plan != "pro_plus" {
		t.Errorf("view Plan = %q, want %q", v.Plan, "pro_plus")
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "pro-plus" {
		t.Errorf("spec.tier = %q, want %q (hyphenated id, not the Render spelling)", got, "pro-plus")
	}
}

func TestSetPlanUnknownPlanIsBadRequestAndNoOp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	if _, err := svc.SetPlan(context.Background(), "web", "gold"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown plan should be core.ErrBadRequest, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "" {
		t.Errorf("a rejected plan must not touch spec.tier, got %q", got)
	}
}

func TestSetPlanManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.SetPlan(context.Background(), "web", "standard"); err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	if len(rec.tierCalls) != 1 || rec.tierCalls[0].id != "srv-1" || rec.tierCalls[0].tier != "standard" {
		t.Fatalf("want row write [srv-1 standard], got %v", rec.tierCalls)
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "standard" {
		t.Errorf("CR spec.tier = %q, want %q", got, "standard")
	}
}

func TestSetPlanUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, sampleApp("hand"))

	if _, err := svc.SetPlan(context.Background(), "hand", "standard"); err != nil {
		t.Fatalf("SetPlan: %v", err)
	}
	if len(rec.tierCalls) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.tierCalls)
	}
	if got := getApp(t, cl, "hand").Spec.Tier; got != "standard" {
		t.Errorf("CR spec.tier = %q, want %q", got, "standard")
	}
}

func TestSetPlanRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.SetPlan(context.Background(), "web", "standard"); err == nil {
		t.Fatal("want error when the row write fails")
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "" {
		t.Errorf("spec.tier must be untouched when the row write failed, got %q", got)
	}
}

// --- Scale (manual instance-count changes) ---

func TestScaleSetsReplicas(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web")) // sampleApp starts at 2

	v, err := svc.Scale(context.Background(), "web", 3)
	if err != nil {
		t.Fatalf("Scale: %v", err)
	}
	if v.Replicas != 3 {
		t.Errorf("view Replicas = %d, want 3", v.Replicas)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 3 {
		t.Errorf("spec.replicas = %d, want 3", got)
	}

	// 3 -> 1 converges back down (the DoD's 1->3->1 round trip).
	if _, err := svc.Scale(context.Background(), "web", 1); err != nil {
		t.Fatalf("Scale down: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 1 {
		t.Errorf("spec.replicas = %d, want 1 after scale-down", got)
	}
}

func TestScaleOutOfRangeIsBadRequestAndNoOp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	for _, n := range []int32{0, -1, 101} {
		if _, err := svc.Scale(context.Background(), "web", n); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("Scale(%d) should be core.ErrBadRequest, got %v", n, err)
		}
	}
	// sampleApp's original count must be untouched by any rejected call.
	if got := getApp(t, cl, "web").Spec.Replicas; got != 2 {
		t.Errorf("a rejected scale must not touch spec.replicas, got %d", got)
	}
}

func TestScaleManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Scale(context.Background(), "web", 4); err != nil {
		t.Fatalf("Scale: %v", err)
	}
	if len(rec.replicasCalls) != 1 || rec.replicasCalls[0].id != "srv-1" || rec.replicasCalls[0].replicas != 4 {
		t.Fatalf("want row write [srv-1 4], got %v", rec.replicasCalls)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 4 {
		t.Errorf("CR spec.replicas = %d, want 4", got)
	}
}

func TestScaleUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, sampleApp("hand"))

	if _, err := svc.Scale(context.Background(), "hand", 5); err != nil {
		t.Fatalf("Scale: %v", err)
	}
	if len(rec.replicasCalls) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.replicasCalls)
	}
	if got := getApp(t, cl, "hand").Spec.Replicas; got != 5 {
		t.Errorf("CR spec.replicas = %d, want 5", got)
	}
}

func TestScaleRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newService(rec, managedApp("web", "srv-1")) // starts at 2

	if _, err := svc.Scale(context.Background(), "web", 7); err == nil {
		t.Fatal("want error when the row write fails")
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 2 {
		t.Errorf("spec.replicas must be untouched when the row write failed, got %d", got)
	}
}

// --- SetIdleTTL (free-tier auto-sleep window; "sleep = free") ---

func TestSetIdleTTLSetsAndReports(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	v, err := svc.SetIdleTTL(context.Background(), "web", 900)
	if err != nil {
		t.Fatalf("SetIdleTTL: %v", err)
	}
	if v.IdleTTLSeconds != 900 {
		t.Errorf("view IdleTTLSeconds = %d, want 900", v.IdleTTLSeconds)
	}
	if got := getApp(t, cl, "web").Spec.IdleTTLSeconds; got != 900 {
		t.Errorf("spec.idleTTLSeconds = %d, want 900", got)
	}
	// 0 restores the controller default and is a valid value (not rejected).
	if _, err := svc.SetIdleTTL(context.Background(), "web", 0); err != nil {
		t.Fatalf("SetIdleTTL(0): %v", err)
	}
	if got := getApp(t, cl, "web").Spec.IdleTTLSeconds; got != 0 {
		t.Errorf("spec.idleTTLSeconds = %d, want 0 (default)", got)
	}
}

func TestSetIdleTTLOutOfRangeIsBadRequestAndNoOp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web")) // sampleApp starts with idleTTL unset (0)

	for _, n := range []int32{-1, MaxIdleTTLSeconds + 1} {
		if _, err := svc.SetIdleTTL(context.Background(), "web", n); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetIdleTTL(%d) should be core.ErrBadRequest, got %v", n, err)
		}
	}
	if got := getApp(t, cl, "web").Spec.IdleTTLSeconds; got != 0 {
		t.Errorf("a rejected idle-TTL must not touch spec.idleTTLSeconds, got %d", got)
	}
}

func TestSetIdleTTLManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.SetIdleTTL(context.Background(), "web", 600); err != nil {
		t.Fatalf("SetIdleTTL: %v", err)
	}
	if len(rec.idleTTLCalls) != 1 || rec.idleTTLCalls[0].id != "srv-1" || rec.idleTTLCalls[0].seconds != 600 {
		t.Fatalf("want row write [srv-1 600], got %v", rec.idleTTLCalls)
	}
	if got := getApp(t, cl, "web").Spec.IdleTTLSeconds; got != 600 {
		t.Errorf("CR spec.idleTTLSeconds = %d, want 600", got)
	}
}

func TestSetIdleTTLUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, sampleApp("hand"))

	if _, err := svc.SetIdleTTL(context.Background(), "hand", 300); err != nil {
		t.Fatalf("SetIdleTTL: %v", err)
	}
	if len(rec.idleTTLCalls) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.idleTTLCalls)
	}
	if got := getApp(t, cl, "hand").Spec.IdleTTLSeconds; got != 300 {
		t.Errorf("CR spec.idleTTLSeconds = %d, want 300", got)
	}
}

func TestViewOmitsPlanForEmptyOrUnknownTier(t *testing.T) {
	svc, _ := newService(nil, sampleApp("untiered"))
	v, err := svc.Get(context.Background(), "untiered")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.Plan != "" {
		t.Errorf("an untiered App's view should omit Plan, got %q", v.Plan)
	}
}

// --- REST + GraphQL fragments (Render shape), without the auth gate ---

func TestRESTFragmentRenderShape(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// list is the {service, cursor} envelope.
	var list []serviceWithCursor
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if list[0].Service.Type != renderWebService || list[0].Service.Suspended != core.RenderNotSuspended {
		t.Errorf("render service shape wrong: %+v", list[0].Service)
	}
	// suspend => 202, restart => 200 (Render status codes).
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/suspend", nil))
	if rec.Code != http.StatusAccepted {
		t.Errorf("suspend => 202, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/restart", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("restart => 200, got %d", rec.Code)
	}
	// unknown => 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown => 404, got %d", rec.Code)
	}
}

func TestRESTPatchServicePlan(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"serviceDetails":{"plan":"pro_plus"}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH plan => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out renderService
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ServiceDetails["plan"] != "pro_plus" {
		t.Errorf("serviceDetails.plan = %v, want %q", out.ServiceDetails["plan"], "pro_plus")
	}

	// Unknown plan => 400, and it must not have changed anything.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(`{"serviceDetails":{"plan":"gold"}}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown plan => 400, got %d", rec.Code)
	}

	// A PATCH with no serviceDetails.plan is a no-op read, not an error.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(`{}`)))
	if rec.Code != http.StatusOK {
		t.Errorf("PATCH with no plan field => 200 (no-op), got %d", rec.Code)
	}
}

func TestRESTScaleService(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// scale => 202 with numInstances honored (Render's scale status code).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/scale", strings.NewReader(`{"numInstances":3}`)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("scale => 202, got %d: %s", rec.Code, rec.Body)
	}
	var out renderService
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Replicas != 3 {
		t.Errorf("replicas = %d, want 3", out.Replicas)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 3 {
		t.Errorf("spec.replicas = %d, want 3", got)
	}

	// Out-of-range numInstances => 400, and it must not change anything.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/scale", strings.NewReader(`{"numInstances":0}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("numInstances 0 => 400, got %d", rec.Code)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 3 {
		t.Errorf("a rejected scale must leave spec.replicas at 3, got %d", got)
	}
}

func TestGraphQLScaleService(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { scaleService(id: "web", numInstances: 3) { replicas } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["scaleService"].(map[string]any)["replicas"]
	if got != 3 {
		t.Errorf("replicas = %v, want 3", got)
	}
	if r := getApp(t, cl, "web").Spec.Replicas; r != 3 {
		t.Errorf("spec.replicas = %d, want 3", r)
	}

	// Out-of-range surfaces as a GraphQL error, not a silent no-op.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { scaleService(id: "web", numInstances: 0) { replicas } }`})
	if len(res.Errors) == 0 {
		t.Error("out-of-range numInstances should produce a GraphQL error")
	}
}

func TestGraphQLUpdateServicePlan(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { updateServicePlan(id: "web", plan: "standard") { plan } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["updateServicePlan"].(map[string]any)["plan"]
	if got != "standard" {
		t.Errorf("plan = %v, want %q", got, "standard")
	}

	// Unknown plan surfaces as a GraphQL error, not a silent no-op.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { updateServicePlan(id: "web", plan: "gold") { plan } }`})
	if len(res.Errors) == 0 {
		t.Error("unknown plan should produce a GraphQL error")
	}
}

// --- InstanceTypes (the plan picker's data source) ---

func TestInstanceTypesListsTheSharedCatalogInLadderOrder(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))

	types, err := svc.InstanceTypes(context.Background())
	if err != nil {
		t.Fatalf("InstanceTypes: %v", err)
	}
	if len(types) != len(tiers.Compute.IDs()) {
		t.Fatalf("want %d tiers, got %d", len(tiers.Compute.IDs()), len(types))
	}
	// Spot-check the entry whose display name and Render id both diverge from
	// its internal spec.tier spelling — the case tierDisplayName exists for.
	var proPlus *InstanceType
	for i := range types {
		if types[i].ID == "pro_plus" {
			proPlus = &types[i]
		}
	}
	if proPlus == nil {
		t.Fatal("pro_plus not found in InstanceTypes()")
	}
	if proPlus.Name != "Pro Plus" || proPlus.CPU != "4" || proPlus.Memory != "8Gi" {
		t.Errorf("pro_plus = %+v, want Name=Pro Plus CPU=4 Memory=8Gi", *proPlus)
	}
}

func TestTierDisplayName(t *testing.T) {
	cases := map[string]string{
		"free": "Free", "starter": "Starter", "standard": "Standard",
		"pro": "Pro", "pro-plus": "Pro Plus", "pro-max": "Pro Max", "pro-ultra": "Pro Ultra",
	}
	for id, want := range cases {
		if got := tierDisplayName(id); got != want {
			t.Errorf("tierDisplayName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestGraphQLInstanceTypes(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ instanceTypes { id name cpu memory } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["instanceTypes"].([]any)
	if len(list) != len(tiers.Compute.IDs()) {
		t.Fatalf("want %d instance types, got %d", len(tiers.Compute.IDs()), len(list))
	}
	first := list[0].(map[string]any)
	if first["id"] != "free" || first["name"] != "Free" {
		t.Errorf("first entry = %+v, want id=free name=Free (ladder order)", first)
	}
}
