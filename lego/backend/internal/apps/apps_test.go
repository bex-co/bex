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
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
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

// getTenantApp fetches an App CR created for tenantID by its public name:
// core.CRName(tenantID, name) is the collision-free object name a
// tenant-scoped create stamps (w4/m19), landed in the workspace's own `<ws>`
// namespace (== tenantID, ADR043) — plain getApp only still works for a
// tenant-less (store-off) create, which stays in "default".
func getTenantApp(t *testing.T, cl client.Client, tenantID, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: tenantID, Name: core.CRName(tenantID, name)}, &a); err != nil {
		t.Fatalf("get %s/%s: %v", tenantID, name, err)
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

// TestInternalAddressOnAllThreeSurfaces is w9/m58's adapter-parity check
// (docs/ADR041-service-addresses.md D4): the Render-shaped private-network
// address "<slug>:<port>" reads back identically over REST, GraphQL and MCP
// for the addressable types, is absent for a worker, and — the one structural
// REST fix the capture forced — a private service's serviceDetails carries NO
// url (Render omits the field; bex used to leak the cluster-internal URL).
func TestInternalAddressOnAllThreeSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name        string
		svcType     string
		wantAddr    string // "" = internalAddress absent everywhere
		wantRESTURL bool
	}{
		{name: "web-service", svcType: appv1alpha1.TypeWebService, wantAddr: "web-a1b2:8080", wantRESTURL: true},
		{name: "private-service", svcType: appv1alpha1.TypePrivateService, wantAddr: "web-a1b2:8080", wantRESTURL: false},
		{name: "background-worker", svcType: appv1alpha1.TypeBackgroundWorker, wantAddr: "", wantRESTURL: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := sampleApp("web")
			app.Spec.Type = tc.svcType
			app.Spec.Subdomain = "web-a1b2"
			app.Spec.Port = 8080
			if tc.svcType == appv1alpha1.TypeBackgroundWorker {
				app.Status.URL = "" // a worker has no URL at all
			}
			svc, _ := newService(nil, app)
			ctx := context.Background()

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			var restBody struct {
				ServiceDetails map[string]any `json:"serviceDetails"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			gotAddr, addrPresent := restBody.ServiceDetails["internalAddress"]
			if tc.wantAddr == "" && addrPresent {
				t.Errorf("REST internalAddress present for %s, want absent", tc.svcType)
			}
			if tc.wantAddr != "" && gotAddr != tc.wantAddr {
				t.Errorf("REST internalAddress = %v, want %q", gotAddr, tc.wantAddr)
			}
			_, urlPresent := restBody.ServiceDetails["url"]
			if urlPresent != tc.wantRESTURL {
				t.Errorf("REST serviceDetails.url present = %v, want %v (Render omits url for pserv/worker)", urlPresent, tc.wantRESTURL)
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "web") { internalAddress } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			gqlAddr := res.Data.(map[string]any)["service"].(map[string]any)["internalAddress"]
			if tc.wantAddr == "" && gqlAddr != "" {
				t.Errorf("GraphQL internalAddress = %v, want empty", gqlAddr)
			}
			if tc.wantAddr != "" && gqlAddr != tc.wantAddr {
				t.Errorf("GraphQL internalAddress = %v, want %q", gqlAddr, tc.wantAddr)
			}

			// MCP (get_service's handler, the same toRenderService REST's GET uses)
			handler := svc.serviceTool(svc.Get)
			_, mcpService, err := handler(ctx, nil, serviceArgs{ServiceID: "web"})
			if err != nil {
				t.Fatalf("MCP get_service: %v", err)
			}
			mcpAddr, mcpPresent := mcpService.ServiceDetails["internalAddress"]
			if tc.wantAddr == "" && mcpPresent {
				t.Errorf("MCP internalAddress present, want absent")
			}
			if tc.wantAddr != "" && mcpAddr != tc.wantAddr {
				t.Errorf("MCP internalAddress = %v, want %q", mcpAddr, tc.wantAddr)
			}
		})
	}
}

// TestSlugPresentOnAllThreeSurfaces is w4/m20/t002's adapter-parity check:
// the globally-unique platform-host slug (spec.subdomain, minted w4/m19,
// falling back to the CR name when unset) reads back identically over
// REST, GraphQL and MCP — including the suffixed case a cross-tenant name
// collision produces. REST's GET and MCP's get_service both delegate to the
// same toRenderService(AppView) the assertions below share, so this also
// proves MCP without standing up a full mcp.Server transport.
func TestSlugPresentOnAllThreeSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name      string
		subdomain string
		want      string
	}{
		{name: "bare-name-free-platform-wide", subdomain: "", want: "web"},
		{name: "suffixed-on-cross-tenant-collision", subdomain: "web-a1b2", want: "web-a1b2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := sampleApp("web")
			app.Spec.Subdomain = tc.subdomain
			svc, _ := newService(nil, app)
			ctx := context.Background()

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			var restBody map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if got := restBody["slug"]; got != tc.want {
				t.Errorf("REST slug = %v, want %q", got, tc.want)
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "web") { slug } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			if got := res.Data.(map[string]any)["service"].(map[string]any)["slug"]; got != tc.want {
				t.Errorf("GraphQL slug = %v, want %q", got, tc.want)
			}

			// MCP (get_service's handler, the same toRenderService REST's GET uses)
			handler := svc.serviceTool(svc.Get)
			_, mcpService, err := handler(ctx, nil, serviceArgs{ServiceID: "web"})
			if err != nil {
				t.Fatalf("MCP get_service: %v", err)
			}
			if mcpService.Slug != tc.want {
				t.Errorf("MCP slug = %q, want %q", mcpService.Slug, tc.want)
			}
		})
	}
}

// TestSuspendersPresentOnAllThreeSurfaces is w4/014's adapter-parity check:
// Render's suspenders array reads ["user"] while an App is suspended (the
// user-driven suspend verb is bex's only suspend path) and [] otherwise —
// always an array, never null, and never a faked value bex has no source
// for (admin, billing, …). Same three-surface shape as the slug test above.
func TestSuspendersPresentOnAllThreeSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name      string
		suspended bool
		want      []string
	}{
		{name: "running-app-empty-array", suspended: false, want: []string{}},
		{name: "user-suspended-app", suspended: true, want: []string{"user"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := sampleApp("web")
			app.Spec.Suspended = tc.suspended
			svc, _ := newService(nil, app)
			ctx := context.Background()

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			var restBody map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			got, ok := restBody["suspenders"].([]any)
			if !ok {
				t.Fatalf("REST suspenders = %v (%T), want a JSON array (never null)", restBody["suspenders"], restBody["suspenders"])
			}
			if len(got) != len(tc.want) || (len(tc.want) == 1 && got[0] != tc.want[0]) {
				t.Errorf("REST suspenders = %v, want %v", got, tc.want)
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "web") { suspenders } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			gqlGot, ok := res.Data.(map[string]any)["service"].(map[string]any)["suspenders"].([]any)
			if !ok {
				t.Fatalf("GraphQL suspenders not a list")
			}
			if len(gqlGot) != len(tc.want) || (len(tc.want) == 1 && gqlGot[0] != tc.want[0]) {
				t.Errorf("GraphQL suspenders = %v, want %v", gqlGot, tc.want)
			}

			// MCP (get_service's handler, the same toRenderService REST's GET uses)
			handler := svc.serviceTool(svc.Get)
			_, mcpService, err := handler(ctx, nil, serviceArgs{ServiceID: "web"})
			if err != nil {
				t.Fatalf("MCP get_service: %v", err)
			}
			if mcpService.Suspenders == nil {
				t.Fatalf("MCP suspenders = nil, want an array (never null)")
			}
			if len(mcpService.Suspenders) != len(tc.want) || (len(tc.want) == 1 && mcpService.Suspenders[0] != tc.want[0]) {
				t.Errorf("MCP suspenders = %v, want %v", mcpService.Suspenders, tc.want)
			}
		})
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

// IsMember: a map-backed caller belongs to exactly the one workspace it
// resolves to — the single-membership case every pre-w6/m14 test is written
// against. Multi-membership callers use a richer fake (see the m14 tests).
func (f fakeWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	tid, ok := f[id.Subject]
	return ok && tid == tenantID, nil
}

// tenantApp mirrors what a store-managed create actually produces under
// per-tenant namespaces (ADR043): the App lives in its own workspace's `<ws>`
// namespace (== tenantID, store.WorkspaceNamespace) and carries LabelServiceName
// alongside LabelTenant (service.go's createNewApp) — both are load-bearing:
// List scopes InNamespace(tenantID), while GetApp's cluster-wide by-name
// fallback keys on LabelServiceName regardless of namespace.
func tenantApp(name, tenantID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Namespace = tenantID
	a.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelServiceName: name}
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
	a := getTenantApp(t, cl, "tea-a", "fresh")
	if a.Labels[core.LabelTenant] != "tea-a" {
		t.Fatalf("created App tenant label = %q, want tea-a", a.Labels[core.LabelTenant])
	}
	// The caller must be able to see/manage what it just created — the whole
	// point of stamping the label on create.
	if _, err := svc.Get(ctx, "fresh"); err != nil {
		t.Errorf("Get on just-created App: %v (label must make it visible to its own owner)", err)
	}
}

// A repo-backed create is accepted and validated (spec.repo set, branch
// defaulted), but must not pretend to be live — its URL/phase come from
// observed status, empty until w1/m5's builds converge it. The truthful
// superset rule: accept the intent, never fake the outcome.
func TestCreateRepoBackedIsAcceptedNotLive(t *testing.T) {
	svc, cl := newService(nil)
	v, err := svc.create(context.Background(), CreateRequest{Name: "from-git", Repo: "https://github.com/bex/hello"})
	if err != nil {
		t.Fatalf("repo-backed create: %v", err)
	}
	if v.URL != "" || v.Phase != "" {
		t.Errorf("a just-created repo App must not report a live URL/phase, got url=%q phase=%q", v.URL, v.Phase)
	}
	a := getApp(t, cl, "from-git")
	if a.Spec.Repo != "https://github.com/bex/hello" || a.Spec.Branch != "main" {
		t.Errorf("repo/branch not persisted: repo=%q branch=%q", a.Spec.Repo, a.Spec.Branch)
	}
	if a.Spec.Image != "" {
		t.Errorf("a repo-backed App must carry no image (build produces it), got %q", a.Spec.Image)
	}
}

func TestStoreOffListAndGetIgnoreTenantLabelsUnchanged(t *testing.T) {
	// Workspace nil (store off): byte-identical to before tenant onboarding —
	// every App in the namespace is visible regardless of label. Store-off
	// deployments have no NamespaceReconciler/per-tenant namespaces at all, so
	// (unlike tenantApp) these fixtures stay in the shared "default" namespace
	// a store-off apps.Service actually creates into.
	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelTenant: "tea-a"}
	other := sampleApp("other")
	other.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, _ := newService(nil, web, other)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	list, err := svc.List(ctx, "")
	if err != nil || len(list) != 2 {
		t.Fatalf("store-off List: %v len=%d, want 2 (unfiltered)", err, len(list))
	}
	if _, err := svc.Get(ctx, "other"); err != nil {
		t.Errorf("store-off Get across tenants: %v, want success", err)
	}
}

// --- Unified create path (w2/m11) ---

// newTenantStoreService builds a Service with both a workspace resolver and a
// recording store — the unified create path (store on + tenant resolved).
func newTenantStoreService(ws core.WorkspaceResolver, st IntentStore, apps ...*appv1alpha1.App) (*Service, client.Client) {
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fakeClient(objs...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}, Store: st}, cl
}

func TestCreateWritesStoreRowWhenStoreAndTenantResolved(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	if _, err := svc.create(ctx, CreateRequest{Name: "web", Image: "nginx:1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// A store row must have been written.
	if len(rec.appCreates) != 1 {
		t.Fatalf("want 1 store row, got %d", len(rec.appCreates))
	}
	got := rec.appCreates[0]
	if got.TenantID != "tea-a" || got.Name != "web" || got.Image != "nginx:1" {
		t.Errorf("store row = %+v", got)
	}
	// CR must carry the three store labels (managed-by, app-id, tenant).
	a := getTenantApp(t, cl, "tea-a", "web")
	if a.Labels[store.LabelManagedBy] != store.ManagedByValue {
		t.Errorf("CR missing managed-by label: %v", a.Labels)
	}
	if a.Labels[store.LabelAppID] != "srv-test" {
		t.Errorf("CR LabelAppID = %q, want srv-test", a.Labels[store.LabelAppID])
	}
	if a.Labels[core.LabelTenant] != "tea-a" {
		t.Errorf("CR LabelTenant = %q, want tea-a", a.Labels[core.LabelTenant])
	}
}

type createErrorClient struct {
	client.Client
	err error
}

func (c createErrorClient) Create(context.Context, client.Object, ...client.CreateOption) error {
	return c.err
}

func TestCreateRollsBackStoreRowWhenCRCreateFails(t *testing.T) {
	rec := &recordingStore{}
	wantErr := errors.New("CRD rejected App")
	svc := &Service{
		Base: &core.Base{
			Client:    createErrorClient{Client: fakeClient(), err: wantErr},
			Namespace: "default",
			Workspace: fakeWorkspace{"id-a": "tea-a"},
		},
		Store: rec,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	if _, err := svc.create(ctx, CreateRequest{Name: "web", Image: "nginx:1"}); !errors.Is(err, wantErr) {
		t.Fatalf("create error = %v, want wrapped CR create error", err)
	}
	if len(rec.appCreates) != 1 {
		t.Fatalf("store creates = %d, want 1", len(rec.appCreates))
	}
	if len(rec.deleteCalls) != 1 || rec.deleteCalls[0] != "srv-test" {
		t.Fatalf("store rollback deletes = %v, want [srv-test]", rec.deleteCalls)
	}
}

func TestCreateSkipsStoreRowWhenNoTenant(t *testing.T) {
	// Store wired but caller not bound to a tenant — must fall back to direct CR.
	rec := &recordingStore{}
	svc, cl := newTenantStoreService(fakeWorkspace{}, rec) // empty map = no tenant
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "unbound", Method: "oauth2"})

	if _, err := svc.create(ctx, CreateRequest{Name: "bare", Image: "nginx:1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(rec.appCreates) != 0 {
		t.Errorf("unbound caller must not write a store row, got %d", len(rec.appCreates))
	}
	// CR must still be created without store labels.
	a := getApp(t, cl, "bare")
	if a.Labels[store.LabelManagedBy] != "" {
		t.Errorf("unbound create must not stamp managed-by: %v", a.Labels)
	}
}

// TestCreateConflictsOnExistingStoreManagedAppWritesNoDeployRow is the w4/m19
// replacement for the old "create redeploys a store-managed app" contract:
// Create now rejects the existing name outright, and — unlike a real redeploy
// (Deploy/Restart) — must never open a deploy row for a create it refused.
func TestCreateConflictsOnExistingStoreManagedAppWritesNoDeployRow(t *testing.T) {
	rec := &recordingStore{}
	svc, _ := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.create(context.Background(), CreateRequest{Name: "web", Image: "nginx:2"}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("create on existing name: got %v, want ErrConflict", err)
	}
	if len(rec.deployCalls) != 0 {
		t.Errorf("a rejected create must not open a deploy row, got %d", len(rec.deployCalls))
	}
}

func TestCreateConflictsOnExistingUnmanagedApp(t *testing.T) {
	rec := &recordingStore{}
	svc, _ := newService(rec, sampleApp("hand"))

	if _, err := svc.create(context.Background(), CreateRequest{Name: "hand", Image: "nginx:2"}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("create on existing name: got %v, want ErrConflict", err)
	}
	if len(rec.deployCalls) != 0 {
		t.Errorf("a rejected create must not open a deploy row, got %d", len(rec.deployCalls))
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
	domainAdds  []struct{ id, host, redirectForName string }
	domainRems  []struct{ id, host string }
	deleteCalls []string
	appCreates  []store.App
	deployCalls []store.Deploy
	imageCalls  []struct{ id, image string }
	// notFoundOnDelete makes DeleteApp report the row is already gone, so a test
	// can assert the verb still deletes the CR (idempotent end state).
	notFoundOnDelete bool
	err              error
	// protectedStatus, keyed by app id, backs GetAppProtectedStatus (w6/m19) —
	// an id absent from the map reports "unprotected", matching the store's
	// own default for an App outside any environment.
	protectedStatus map[string]string
	environments    map[string]store.Environment
}

func (r *recordingStore) GetEnvironment(_ context.Context, id string) (store.Environment, error) {
	if r.err != nil {
		return store.Environment{}, r.err
	}
	if environment, ok := r.environments[id]; ok {
		return environment, nil
	}
	return store.Environment{}, fmt.Errorf("environment: %w", store.ErrNotFound)
}

func (r *recordingStore) GetAppProtectedStatus(_ context.Context, id string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if status, ok := r.protectedStatus[id]; ok {
		return status, nil
	}
	return "unprotected", nil
}

func (r *recordingStore) CreateApp(_ context.Context, a store.App) (store.App, error) {
	if r.err != nil {
		return store.App{}, r.err
	}
	a.ID = "srv-test"
	a.FirstDeployID = "dep-test"
	r.appCreates = append(r.appCreates, a)
	return a, nil
}

func (r *recordingStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error) {
	if r.err != nil {
		return store.Deploy{}, r.err
	}
	d := store.Deploy{ID: "dep-test", AppID: appID, Trigger: trigger, Image: image, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, Status: store.DeployUpdateInProgress}
	r.deployCalls = append(r.deployCalls, d)
	return d, nil
}

func (r *recordingStore) DeleteApp(_ context.Context, id string) error {
	if r.err != nil {
		return r.err
	}
	if r.notFoundOnDelete {
		return store.ErrNotFound
	}
	r.deleteCalls = append(r.deleteCalls, id)
	return nil
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

func (r *recordingStore) SetAppSource(_ context.Context, id, repo, image, branch string, registryCredentialID *string) error {
	return r.err
}

func (r *recordingStore) SetAppImage(_ context.Context, id string, image string) error {
	if r.err != nil {
		return r.err
	}
	r.imageCalls = append(r.imageCalls, struct{ id, image string }{id, image})
	return nil
}

func (r *recordingStore) AddDomain(_ context.Context, id, host, redirectForName string) error {
	if r.err != nil {
		return r.err
	}
	r.domainAdds = append(r.domainAdds, struct{ id, host, redirectForName string }{id, host, redirectForName})
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

// newBaseDomainService builds a store-less Service with BaseDomain/DashboardHost
// set, for the w7/m6 reserved-host guard tests.
func newBaseDomainService(baseDomain, dashboardHost string, apps ...*appv1alpha1.App) (*Service, client.Client) {
	svc, cl := newService(nil, apps...)
	svc.BaseDomain = baseDomain
	svc.DashboardHost = dashboardHost
	return svc, cl
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
	a := sampleApp("hand")
	a.Labels = map[string]string{core.LabelAppID: "srv-direct"}
	svc, cl := newService(rec, a)

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

func TestSetSourceUnmanagedAppWithPublicIDSkipsStore(t *testing.T) {
	rec := &recordingStore{err: errors.New("unexpected store write")}
	a := sampleApp("hand")
	a.Labels = map[string]string{core.LabelAppID: "srv-direct"}
	a.Spec.Image = "old:1"
	svc, cl := newService(rec, a)

	image := "new:1"
	if _, err := svc.SetSource(context.Background(), "hand", nil, &image, nil); err != nil {
		t.Fatalf("SetSource: %v", err)
	}
	if got := getApp(t, cl, "hand").Spec.Image; got != image {
		t.Fatalf("CR spec.image = %q, want %q", got, image)
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
	a := sampleApp("hand")
	a.Labels = map[string]string{core.LabelAppID: "srv-direct"}
	svc, cl := newService(rec, a)

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
	app := managedApp("web", "srv-1")
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/bex-co/private.git"
	app.Spec.CloneSecret = "deliberately-unusable-clone-secret"
	svc, cl := newService(rec, app)

	if _, err := svc.Scale(context.Background(), "web", 4); err != nil {
		t.Fatalf("Scale: %v", err)
	}
	if len(rec.replicasCalls) != 1 || rec.replicasCalls[0].id != "srv-1" || rec.replicasCalls[0].replicas != 4 {
		t.Fatalf("want row write [srv-1 4], got %v", rec.replicasCalls)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 4 {
		t.Errorf("CR spec.replicas = %d, want 4", got)
	}
	if len(rec.deployCalls) != 0 {
		t.Fatalf("manual scale must not open a deploy row, got %v", rec.deployCalls)
	}
	if got := getApp(t, cl, "web").Spec.CloneSecret; got != "deliberately-unusable-clone-secret" {
		t.Errorf("manual scale touched clone credential reference: %q", got)
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

func TestViewReportsDeletingWhileFinalizerHoldsApp(t *testing.T) {
	app := sampleApp("deleting-app")
	now := metav1.Now()
	app.DeletionTimestamp = &now
	if got := view(app).Phase; got != "Deleting" {
		t.Fatalf("deleting App phase = %q, want Deleting", got)
	}
}

// --- Delete (store-managed row-first vs hand-applied CR delete) ---

// gone asserts the App CR named name no longer exists.
func gone(t *testing.T, cl client.Client, name string) {
	t.Helper()
	var a appv1alpha1.App
	err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("App %s still exists (err=%v)", name, err)
	}
}

func TestDeleteStoreLessRemovesCR(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	gone(t, cl, "web")
}

func TestDeleteManagedAppDeletesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))
	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	// The row is the single writer of intent — it must be deleted (so a resync
	// can't resurrect the CR) — and the CR is removed directly for immediacy.
	if len(rec.deleteCalls) != 1 || rec.deleteCalls[0] != "srv-1" {
		t.Fatalf("want row delete [srv-1], got %v", rec.deleteCalls)
	}
	gone(t, cl, "web")
}

func TestDeleteUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	a := sampleApp("hand")
	a.Labels = map[string]string{core.LabelAppID: "srv-direct"}
	svc, cl := newService(rec, a)
	if err := svc.Delete(context.Background(), "hand"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(rec.deleteCalls) != 0 {
		t.Fatalf("hand-applied App must not touch the store, got %v", rec.deleteCalls)
	}
	gone(t, cl, "hand")
}

func TestDeleteUnknownIsNotFound(t *testing.T) {
	rec := &recordingStore{}
	svc, _ := newService(rec, managedApp("web", "srv-1"))
	err := svc.Delete(context.Background(), "nope")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Delete unknown => ErrNotFound, got %v", err)
	}
	if len(rec.deleteCalls) != 0 {
		t.Fatalf("an unknown id must not reach the store, got %v", rec.deleteCalls)
	}
}

// A row already deleted (a resync raced us) is the intended end state, not an
// error: Delete swallows store.ErrNotFound and still removes the orphaned CR.
func TestDeleteManagedRowAlreadyGoneStillDeletesCR(t *testing.T) {
	rec := &recordingStore{notFoundOnDelete: true}
	svc, cl := newService(rec, managedApp("web", "srv-1"))
	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete with an already-gone row => nil, got %v", err)
	}
	gone(t, cl, "web")
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

func TestRESTUsesTypedServiceIDForResponseAndLifecycle(t *testing.T) {
	app := managedApp(core.CRName("tea-a", "web"), "srv-c185th5c2rvvnhbfiltg")
	app.Labels[core.LabelServiceName] = "web"
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services", nil))
	var list []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if list[0].Service.ID != "srv-c185th5c2rvvnhbfiltg" || list[0].Service.Name != "web" {
		t.Fatalf("service identity = id %q name %q", list[0].Service.ID, list[0].Service.Name)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/srv-c185th5c2rvvnhbfiltg/restart", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("restart by typed id => %d: %s", rec.Code, rec.Body.String())
	}
	if got := getApp(t, cl, core.CRName("tea-a", "web")).Spec.RestartedAt; got == "" {
		t.Fatal("restart by typed id did not stamp restartedAt")
	}
}

func TestRESTListTypeFilter(t *testing.T) {
	// GET /v1/services?type= filters by Render serviceType (w2/m52).
	web := sampleApp("web")
	worker := sampleApp("worker")
	worker.Spec.Type = appv1alpha1.TypeBackgroundWorker
	cron := sampleApp("cron")
	cron.Spec.Type = appv1alpha1.TypeCronJob

	svc, _ := newService(nil, web, worker, cron)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	listNames := func(query string) []string {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services"+query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", query, rec.Code, rec.Body.String())
		}
		var page []serviceWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		names := make([]string, 0, len(page))
		for _, s := range page {
			names = append(names, s.Service.Name)
		}
		slices.Sort(names)
		return names
	}

	if got := listNames("?type=background_worker"); !slices.Equal(got, []string{"worker"}) {
		t.Errorf("type=background_worker = %v, want [worker]", got)
	}
	if got := listNames("?type=web_service"); !slices.Equal(got, []string{"web"}) {
		t.Errorf("type=web_service = %v, want [web]", got)
	}
	if got := listNames("?type=web_service,background_worker"); !slices.Equal(got, []string{"web", "worker"}) {
		t.Errorf("type=web_service,background_worker = %v, want [web, worker]", got)
	}
	if got := listNames("?type=cron_job&type=web_service"); !slices.Equal(got, []string{"cron", "web"}) {
		t.Errorf("repeated type= = %v, want [cron, web]", got)
	}
}

func TestRESTListSuspendedFilter(t *testing.T) {
	// GET /v1/services?suspended=true|false filters by suspension state (w2/m52).
	web := sampleApp("web")
	susp := sampleApp("susp")
	susp.Spec.Suspended = true

	svc, _ := newService(nil, web, susp)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services?suspended=true", nil))
	var page []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page) != 1 || page[0].Service.Name != "susp" {
		t.Errorf("suspended=true = %v, want [susp]", page)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services?suspended=false", nil))
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page) != 1 || page[0].Service.Name != "web" {
		t.Errorf("suspended=false = %v, want [web]", page)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services?suspended=maybe", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("suspended=maybe => 400, got %d", rec.Code)
	}
}

func TestRESTListTimeFilters(t *testing.T) {
	// GET /v1/services?createdBefore/After/updatedBefore/After (w2/m52).
	early := sampleApp("early")
	late := sampleApp("late")

	svc, _ := newService(nil, early, late)
	// Manually set CreatedAt on the views by patching the underlying CRs'
	// resource version timestamps — the fake client doesn't set CreationTimestamp,
	// so we rely on the fact that both views will have empty CreatedAt and thus
	// PASS any time filter (legacy-group rule: empty timestamp → include).
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Malformed timestamp → 400.
	for _, param := range []string{"createdBefore", "createdAfter", "updatedBefore", "updatedAfter"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services?"+param+"=yesterday", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s=yesterday => 400, got %d", param, rec.Code)
		}
	}

	// Empty CreatedAt on apps (fake client has no creation timestamp) → passes
	// any time window (same rule as matchesTimeWindow in envgroups).
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services?createdBefore=2020-01-01T00:00:00Z", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("createdBefore with empty timestamps => 200, got %d", rec.Code)
	}
	var page []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page) != 2 {
		t.Errorf("empty createdAt passes time filter: got %d results, want 2", len(page))
	}
}

// appUpdatedAt returns an App whose UpdatedAt resolves to the given timestamp,
// so a time-window filter has something real to place.
func appUpdatedAt(name, updatedAt string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Annotations = map[string]string{resourcemeta.UpdatedAtAnnotation: updatedAt}
	return a
}

func TestRESTListTimeWindowExcludes(t *testing.T) {
	// TestRESTListTimeFilters only proves the 400s and the legacy passthrough;
	// neither shows the window narrowing anything. Stamped services must
	// actually fall in or out of it.
	svc, _ := newService(nil,
		appUpdatedAt("old", "2026-01-01T00:00:00Z"),
		appUpdatedAt("new", "2026-12-01T00:00:00Z"),
		sampleApp("unstamped"),
	)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	listNames := func(query string) []string {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services"+query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/services%s = %d: %s", query, rec.Code, rec.Body.String())
		}
		var page []serviceWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		names := make([]string, 0, len(page))
		for _, s := range page {
			names = append(names, s.Service.Name)
		}
		slices.Sort(names)
		return names
	}

	cases := []struct {
		query string
		want  []string
	}{
		// "unstamped" has no timestamp, so it rides along with every window.
		{"?updatedBefore=2026-06-01T00:00:00Z", []string{"old", "unstamped"}},
		{"?updatedAfter=2026-06-01T00:00:00Z", []string{"new", "unstamped"}},
		{"?updatedAfter=2026-06-01T00:00:00Z&updatedBefore=2027-01-01T00:00:00Z", []string{"new", "unstamped"}},
		{"?updatedAfter=2027-01-01T00:00:00Z", []string{"unstamped"}},
	}
	for _, c := range cases {
		want := slices.Clone(c.want)
		slices.Sort(want)
		if got := listNames(c.query); !slices.Equal(got, want) {
			t.Errorf("GET /v1/services%s = %v, want %v", c.query, got, want)
		}
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
	app := sampleApp("web")
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/bex-co/private.git"
	app.Spec.CloneSecret = "deliberately-unusable-clone-secret"
	svc, cl := newService(nil, app)
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

func TestRESTDeleteService(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// delete => 204 empty body (Render's delete semantics), CR gone.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/services/web", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete => 204, got %d: %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("delete body must be empty, got %q", rec.Body)
	}
	gone(t, cl, "web")

	// A second delete of the now-unknown id => 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/services/web", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete unknown => 404, got %d", rec.Code)
	}

	// The canonical /v1/services route serves delete.
	svc2, cl2 := newService(nil, sampleApp("api"))
	mux2 := http.NewServeMux()
	svc2.RegisterREST(mux2)
	rec = httptest.NewRecorder()
	mux2.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/services/api", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete via /v1/services => 204, got %d", rec.Code)
	}
	gone(t, cl2, "api")
}

func TestGraphQLDeleteService(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { deleteService(id: "web") }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	if ok, _ := res.Data.(map[string]any)["deleteService"].(bool); !ok {
		t.Errorf("deleteService => true, got %v", res.Data)
	}
	gone(t, cl, "web")
}

func TestGraphQLScaleService(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/bex-co/private.git"
	app.Spec.CloneSecret = "deliberately-unusable-clone-secret"
	svc, cl := newService(nil, app)
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

// TestGraphQLCreateServiceEnvVars: createService(envVars:) lands on spec.Env
// identically to what REST produces from the same {key,value} pairs (w5/m20).
// Note: create-time envVars land on spec.Env, NOT on the OpenBao-backed secret
// store that the Environment tab reads — this is intentional and consistent
// across REST/GraphQL/MCP. Users who need env vars to appear in the Environment
// tab should add them there post-create (ADR006 §env-vars).
func TestGraphQLCreateServiceEnvVars(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{
		Schema:  schema,
		Context: context.Background(),
		RequestString: `mutation {
			createService(
				name: "svc-with-env"
				image: "ghcr.io/org/app:latest"
				envVars: [
					{key: "PORT", value: "8080"}
					{key: "LOG_LEVEL", value: "debug"}
					{key: "SESSION_SECRET", generateValue: true}
				]
			) { id }
		}`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql createService: %v", res.Errors)
	}

	a := getApp(t, cl, "svc-with-env")
	want := []appv1alpha1.EnvVar{{Name: "PORT", Value: "8080"}, {Name: "LOG_LEVEL", Value: "debug"}}
	if len(a.Spec.Env) != len(want)+1 {
		t.Fatalf("spec.Env len = %d, want %d", len(a.Spec.Env), len(want)+1)
	}
	for i, w := range want {
		if a.Spec.Env[i] != w {
			t.Errorf("spec.Env[%d] = %+v, want %+v", i, a.Spec.Env[i], w)
		}
	}
	generated := a.Spec.Env[len(want)]
	if generated.Name != "SESSION_SECRET" || len(generated.Value) != 44 {
		t.Fatalf("generated env = %+v, want SESSION_SECRET with a 44-char value", generated)
	}
}
