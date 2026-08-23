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

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m46 t003 — the blast-radius sweep for the private-service exposure class.
//
// Every entrypoint that can set a service's Type/Expose/Host/Hosts is covered
// here, so a sibling surface is never left with the gap t001 closed. The full
// enumerated list, and why the count is what it is:
//
//  1. REST      POST /v1/services                     (rest.go, createServiceRequest.toCreateRequest)
//  2. GraphQL   mutation createService                (graphql.go)
//  3. MCP       create_web_service (type: private_service)  (mcp.go, createWebServiceArgs.toCreateRequest)
//  4. Blueprint applyCreate — first sync of a new service   (deploy.go, createFromStack)
//  5. Blueprint applyCreate — RE-sync of an existing one    (deploy.go, applyCreateToSpec)
//  6. Control-plane projector resync                   (store/reconciler.go — covered by its own
//     package's TestReconcileNeverExposesNonPublicServiceTypes; it is the write that caused the incident)
//
// MCP has no create_private_service tool: create_web_service carries the type,
// and create_cron_job / create_static_site are type-pinned by construction. No
// settings-update path computes Expose — after t001 the only assignments left
// in internal/apps are specFromCreate (type-aware) and applyCreateToSpec's
// pass-through of that same value, and the projector no longer owns the field.
//
// assertPrivate is the shared assertion: no platform host, no custom host, no
// public URL, on the persisted CR — the thing the operator actually reads.
func assertPrivate(t *testing.T, surface string, a *appv1alpha1.App) {
	t.Helper()
	if a.Spec.Type != appv1alpha1.TypePrivateService {
		t.Fatalf("%s: spec.type = %q, want private_service", surface, a.Spec.Type)
	}
	if a.Spec.Expose {
		t.Errorf("%s: spec.expose = true — a private service must carry no platform host", surface)
	}
	if a.Spec.Host != "" || len(a.Spec.Hosts) != 0 {
		t.Errorf("%s: spec.host = %q, spec.hosts = %v — want neither", surface, a.Spec.Host, a.Spec.Hosts)
	}
	if hosts := a.Spec.EffectiveHosts(a.Name, "onbex.co"); len(hosts) != 0 {
		t.Errorf("%s: effective hosts = %v, want none (this is what the operator turns into an Ingress)", surface, hosts)
	}
}

// assertNoPublicURLInRenderPayload fails if a Render-shaped payload carries a
// public URL anywhere — `serviceDetails.url` is the documented key, but the
// check is deliberately a substring sweep for the platform domain so a NEW
// field that starts echoing the host is caught too.
func assertNoPublicURLInRenderPayload(t *testing.T, surface string, payload []byte) {
	t.Helper()
	if strings.Contains(string(payload), "onbex.co") {
		t.Errorf("%s: response mentions the platform domain for a private service: %s", surface, payload)
	}
}

func TestPrivateServiceIsNeverPubliclyRoutableThroughRESTCreate(t *testing.T) {
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"pserv","type":"private_service","image":{"imagePath":"nginx:alpine"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services = %d: %s", rec.Code, rec.Body.String())
	}

	assertNoPublicURLInRenderPayload(t, "REST create", rec.Body.Bytes())
	assertPrivate(t, "REST", getApp(t, cl, "pserv"))

	// And on the read-back, not just the create echo.
	read := httptest.NewRecorder()
	mux.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/v1/services?name=pserv", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("GET /v1/services = %d: %s", read.Code, read.Body.String())
	}
	assertNoPublicURLInRenderPayload(t, "REST read", read.Body.Bytes())
}

func TestPrivateServiceIsNeverPubliclyRoutableThroughGraphQLCreate(t *testing.T) {
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	// The dashboard's own "New Service" mutation — the surface the live incident
	// was reproduced through.
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "pserv", type: "private_service", image: "nginx:alpine") { type url } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	created := res.Data.(map[string]any)["createService"].(map[string]any)
	if url, _ := created["url"].(string); url != "" {
		t.Errorf("GraphQL createService returned url %q for a private service", url)
	}
	assertPrivate(t, "GraphQL", getApp(t, cl, "pserv"))
}

func TestPrivateServiceIsNeverPubliclyRoutableThroughMCPCreate(t *testing.T) {
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"

	// create_web_service is MCP's typed create; private_service is one of the
	// types its `type` argument accepts (there is no create_private_service
	// tool). This drives the tool's own argument mapping and the Core verb
	// behind it, not the MCP transport — the tool body is exactly these two
	// calls (mcp.go registerServiceTools).
	args := createWebServiceArgs{Name: "pserv", Type: appv1alpha1.TypePrivateService, Image: "nginx:alpine"}
	view, err := svc.Create(context.Background(), args.toCreateRequest())
	if err != nil {
		t.Fatalf("MCP create_web_service(type=private_service): %v", err)
	}
	if view.URL != "" {
		t.Errorf("MCP create returned url %q for a private service", view.URL)
	}
	rendered, err := json.Marshal(toRenderService(view))
	if err != nil {
		t.Fatalf("marshal MCP result: %v", err)
	}
	assertNoPublicURLInRenderPayload(t, "MCP create", rendered)
	assertPrivate(t, "MCP", getApp(t, cl, "pserv"))
}

func TestPrivateServiceIsNeverPubliclyRoutableThroughBlueprintSync(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"
	req := CreateRequest{Name: "pserv", Type: appv1alpha1.TypePrivateService, Image: "nginx:alpine"}

	// First sync creates it.
	if _, err := svc.applyCreate(ctx, req); err != nil {
		t.Fatalf("blueprint first sync: %v", err)
	}
	assertPrivate(t, "blueprint create", getApp(t, cl, "pserv"))

	// A re-sync of the SAME service is the idempotent-upsert path
	// (applyCreateToSpec's straight `dst.Expose = want.Expose`): it must never
	// flip a private service public on the way through.
	if _, err := svc.applyCreate(ctx, req); err != nil {
		t.Fatalf("blueprint re-sync: %v", err)
	}
	assertPrivate(t, "blueprint re-sync", getApp(t, cl, "pserv"))
}

// TestCreateRecordsServiceTypeOnStoreRow is the other half of the t001 fix: the
// CR being right at create is not enough, because the control-plane projector
// rebuilds the spec from the ROW on every resync. A row that does not know the
// service type re-derives it as a web service and republishes it.
func TestCreateRecordsServiceTypeOnStoreRow(t *testing.T) {
	for _, serviceType := range []string{
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeWebService,
	} {
		t.Run(serviceType, func(t *testing.T) {
			rec := &recordingStore{}
			svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

			if _, err := svc.create(ctx, CreateRequest{Name: "svc", Type: serviceType, Image: "nginx:1"}); err != nil {
				t.Fatalf("create: %v", err)
			}
			if len(rec.appCreates) != 1 {
				t.Fatalf("store rows written = %d, want 1", len(rec.appCreates))
			}
			if got := rec.appCreates[0].Type; got != serviceType {
				t.Errorf("store row type = %q, want %q — the projector reads this to decide exposure", got, serviceType)
			}
		})
	}
}

// TestCreatePinsFirstDeployReleaseGeneration is the w6/m46 t004 regression.
// store.CreateApp opens the first deploy row against release generation 1
// before the App CR exists, so it can only assume the operator will settle on
// the same number. Every other deploy-opening path stamps the annotation that
// makes that agreement explicit; create did not, which left the first deploy at
// the mercy of any operational write that landed before the operator's first
// reconcile — the operator would then report a release the deploy row had never
// heard of, and the row closes 'canceled' with nothing having superseded it.
func TestCreatePinsFirstDeployReleaseGeneration(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "nginx:1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Asserted against the literal "1", not against the constant the stamp is
	// written from — the invariant is agreement with the generation the store
	// opens the first deploy row at, so a test that reads the same constant
	// back would pass however both drifted.
	a := getApp(t, cl, "web")
	if got := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != "1" {
		t.Fatalf("%s = %q, want %q — the generation store.CreateApp opens the first deploy row at",
			appv1alpha1.AnnotationReleaseGeneration, got, "1")
	}
}

// TestNonRoutableTypesRejectCustomDomains closes the other half of the exposure
// class (w6/m46 t002 step 2): a custom domain on a type the operator will never
// route is intent the platform cannot honor, so it is refused rather than
// stored. The Blueprint path already drew this line
// (validateManifestIngressFields); create named only worker/cron — letting a
// private_service through — and the AddDomain verb checked nothing at all, so a
// domain refused at create could be added a second later.
func TestNonRoutableTypesRejectCustomDomains(t *testing.T) {
	nonRoutable := []string{
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
	}
	for _, serviceType := range nonRoutable {
		t.Run("create/"+serviceType, func(t *testing.T) {
			svc, _ := newService(nil)
			req := CreateRequest{
				Name: "svc", Type: serviceType, Image: "nginx:1",
				Hosts: []string{"nope.example.com"},
			}
			if serviceType == appv1alpha1.TypeCronJob {
				req.Schedule = "0 * * * *"
			}
			_, err := svc.Create(context.Background(), req)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("create %s with a domain = %v, want ErrBadRequest", serviceType, err)
			}
		})

		t.Run("addDomain/"+serviceType, func(t *testing.T) {
			ctx := context.Background()
			svc, _ := newService(nil)
			req := CreateRequest{Name: "svc", Type: serviceType, Image: "nginx:1"}
			if serviceType == appv1alpha1.TypeCronJob {
				req.Schedule = "0 * * * *"
			}
			if _, err := svc.Create(ctx, req); err != nil {
				t.Fatalf("create %s: %v", serviceType, err)
			}
			if _, err := svc.AddDomain(ctx, "svc", "nope.example.com"); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("AddDomain on a %s = %v, want ErrBadRequest", serviceType, err)
			}
			if a := getApp(t, svc.Client, "svc"); a.Spec.Host != "" || len(a.Spec.Hosts) != 0 {
				t.Errorf("refused domain still landed on the spec: host=%q hosts=%v", a.Spec.Host, a.Spec.Hosts)
			}
		})
	}

	// The routable types are untouched — this must not become "no service can
	// have a custom domain".
	for _, serviceType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeStaticSite} {
		t.Run("allowed/"+serviceType, func(t *testing.T) {
			ctx := context.Background()
			svc, _ := newService(nil)
			req := CreateRequest{Name: "svc", Type: serviceType, Image: "nginx:1"}
			if serviceType == appv1alpha1.TypeStaticSite {
				req.Repo, req.Image, req.PublishPath = "https://github.com/bex-co/bex", "", "dist"
			}
			if _, err := svc.Create(ctx, req); err != nil {
				t.Fatalf("create %s: %v", serviceType, err)
			}
			if _, err := svc.AddDomain(ctx, "svc", "yes.example.com"); err != nil {
				t.Fatalf("AddDomain on a %s: %v", serviceType, err)
			}
		})
	}
}

// TestPrivateServiceURLNeverCarriesAPublicHost is w6/m46 t006's parity check.
// The three surfaces already differ here BY RECORD (ADR018 § Internal address,
// w9/m58): REST and MCP omit serviceDetails.url for a private service, matching
// Render's captured field table, while GraphQL keeps `url` as a bex-shaped
// surface carrying the cluster-internal address. That divergence is deliberate
// and is left standing.
//
// What all three must agree on — and what the exposure incident broke — is that
// none of them ever reports a PUBLIC host for a private service. status.url is
// the operator's own observation, so this drives the assertion off a running
// App's status rather than a create echo.
func TestPrivateServiceURLNeverCarriesAPublicHost(t *testing.T) {
	live := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "pserv", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypePrivateService, Image: "nginx:1", Port: 3000,
		},
		// What the operator stamps for a running private service once it builds
		// no Ingress: the in-cluster address, never an https host.
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning,
			URL:   "http://pserv.default.svc:3000",
		},
	}
	svc, _ := newService(nil, live)
	svc.BaseDomain = "onbex.co"

	view, err := svc.Get(context.Background(), "pserv")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// GraphQL reads AppView.URL straight through (graphql.go "url").
	if strings.HasPrefix(view.URL, "https://") || strings.Contains(view.URL, "onbex.co") {
		t.Errorf("GraphQL url = %q — a private service must never report a public host", view.URL)
	}
	if view.InternalAddress == "" {
		t.Error("the private-network address must still be reported, in internalAddress")
	}

	// REST + MCP share the Render projection, which omits url for this type.
	rendered, err := json.Marshal(toRenderService(view))
	if err != nil {
		t.Fatalf("marshal render service: %v", err)
	}
	assertNoPublicURLInRenderPayload(t, "REST/MCP read", rendered)
	if strings.Contains(string(rendered), "pserv.default.svc") {
		t.Errorf("Render projection carried the cluster-internal url, which w9/m58 removed: %s", rendered)
	}
}
