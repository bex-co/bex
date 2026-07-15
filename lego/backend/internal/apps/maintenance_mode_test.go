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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func paidWebApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Tier = "starter"
	return a
}

func TestMaintenanceModeRESTCanonicalShapeAndPresence(t *testing.T) {
	svc, cl := newService(nil, paidWebApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	missing := patch(`{"serviceDetails":{"maintenanceMode":{"enabled":true}}}`)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing uri status = %d body=%s", missing.Code, missing.Body.String())
	}
	if got := getApp(t, cl, "web").Spec.MaintenanceMode; got != nil {
		t.Fatalf("invalid request mutated maintenanceMode: %+v", got)
	}

	ok := patch(`{"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":""}}}`)
	if ok.Code != http.StatusOK {
		t.Fatalf("patch status = %d body=%s", ok.Code, ok.Body.String())
	}
	got := getApp(t, cl, "web").Spec.MaintenanceMode
	if got == nil || !got.Enabled || got.URI != "" {
		t.Fatalf("persisted maintenanceMode = %+v", got)
	}
	var wire struct {
		ServiceDetails struct {
			MaintenanceMode map[string]any `json:"maintenanceMode"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.ServiceDetails.MaintenanceMode) != 2 || wire.ServiceDetails.MaintenanceMode["enabled"] != true || wire.ServiceDetails.MaintenanceMode["uri"] != "" {
		t.Fatalf("REST maintenanceMode = %#v", wire.ServiceDetails.MaintenanceMode)
	}

	legacySvc, legacyClient := newService(nil, paidWebApp("legacy"))
	legacyMux := http.NewServeMux()
	legacySvc.RegisterREST(legacyMux)
	legacy := httptest.NewRecorder()
	legacyMux.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/v1/services/legacy", nil))
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy read status = %d body=%s", legacy.Code, legacy.Body.String())
	}
	if err := json.Unmarshal(legacy.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.ServiceDetails.MaintenanceMode) != 2 || wire.ServiceDetails.MaintenanceMode["enabled"] != false || wire.ServiceDetails.MaintenanceMode["uri"] != "" {
		t.Fatalf("legacy REST maintenanceMode = %#v", wire.ServiceDetails.MaintenanceMode)
	}
	if got := getApp(t, legacyClient, "legacy").Spec.MaintenanceMode; got != nil {
		t.Fatalf("canonical read persisted a default: %+v", got)
	}
}

func TestMaintenanceModeEligibilityAndSelfURLAreMutationFree(t *testing.T) {
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
	}{
		{name: "free", app: func() *appv1alpha1.App { a := paidWebApp("free"); a.Spec.Tier = "free"; return a }()},
		{name: "private", app: func() *appv1alpha1.App {
			a := paidWebApp("private")
			a.Spec.Type = appv1alpha1.TypePrivateService
			return a
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil, tc.app)
			if _, err := svc.ConfigureMaintenanceMode(context.Background(), tc.app.Name, MaintenanceModeView{Enabled: true}); !strings.Contains(errString(err), "maintenanceMode") {
				t.Fatalf("error = %v", err)
			}
			if got := getApp(t, cl, tc.app.Name).Spec.MaintenanceMode; got != nil {
				t.Fatalf("rejected write mutated App: %+v", got)
			}
		})
	}

	svc, cl := newService(nil, paidWebApp("web"))
	if _, err := svc.ConfigureMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: "https://web.onbex.co/maintenance"}); !strings.Contains(errString(err), "same service") {
		t.Fatalf("same-service error = %v", err)
	}
	if got := getApp(t, cl, "web").Spec.MaintenanceMode; got != nil {
		t.Fatalf("same-service rejection mutated App: %+v", got)
	}
}

func TestMaintenanceModeBlueprintOmissionAndPaidDefault(t *testing.T) {
	stack, err := parseStack(DeployRequest{Manifest: `services:
  - type: web
    name: web
    image: nginx:1
    maintenanceMode:
      enabled: true
`})
	if err != nil {
		t.Fatal(err)
	}
	req := stack.services[0].req
	if req.Plan != "starter" || req.MaintenanceMode == nil || !req.MaintenanceMode.Enabled || req.MaintenanceMode.URI != "" {
		t.Fatalf("Blueprint request = %+v", req)
	}

	dst := appv1alpha1.AppSpec{MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true, URI: "https://status.example.com"}}
	applyCreateToSpec(&dst, appv1alpha1.AppSpec{})
	if dst.MaintenanceMode == nil || dst.MaintenanceMode.URI != "https://status.example.com" {
		t.Fatalf("omitted Blueprint field reset API-owned state: %+v", dst.MaintenanceMode)
	}
	want := appv1alpha1.AppSpec{MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: false, URI: ""}}
	applyCreateToSpec(&dst, want)
	if dst.MaintenanceMode == nil || dst.MaintenanceMode.Enabled || dst.MaintenanceMode.URI != "" {
		t.Fatalf("explicit Blueprint value not applied: %+v", dst.MaintenanceMode)
	}
}

func TestMaintenanceModeBlueprintValidateApplyAndResync(t *testing.T) {
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"
	custom := `services:
  - type: web
    name: web
    plan: starter
    image: nginx:1
    maintenanceMode:
      enabled: true
      uri: https://status.example.com/maintenance
`
	validation, err := svc.ValidateBlueprint(context.Background(), "", custom)
	if err != nil || !validation.Valid {
		t.Fatalf("ValidateBlueprint = %+v, %v", validation, err)
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: custom}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	created := getApp(t, cl, "web")
	if created.Spec.MaintenanceMode == nil || !created.Spec.MaintenanceMode.Enabled || created.Spec.MaintenanceMode.URI != "https://status.example.com/maintenance" {
		t.Fatalf("created maintenanceMode = %+v", created.Spec.MaintenanceMode)
	}
	restartedAt := created.Spec.RestartedAt

	omitted := `services:
  - type: web
    name: web
    plan: starter
    image: nginx:1
`
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: omitted}); err != nil {
		t.Fatalf("omitted re-sync: %v", err)
	}
	preserved := getApp(t, cl, "web")
	if preserved.Spec.MaintenanceMode == nil || preserved.Spec.MaintenanceMode.URI != "https://status.example.com/maintenance" {
		t.Fatalf("omitted re-sync reset maintenanceMode: %+v", preserved.Spec.MaintenanceMode)
	}

	disabled := strings.Replace(custom, "enabled: true", "enabled: false", 1)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: disabled}); err != nil {
		t.Fatalf("maintenance-only re-sync: %v", err)
	}
	updated := getApp(t, cl, "web")
	if updated.Spec.MaintenanceMode == nil || updated.Spec.MaintenanceMode.Enabled {
		t.Fatalf("disabled maintenanceMode = %+v", updated.Spec.MaintenanceMode)
	}
	if updated.Spec.RestartedAt != restartedAt {
		t.Fatalf("maintenance-only re-sync changed restartedAt: %q -> %q", restartedAt, updated.Spec.RestartedAt)
	}

	// A mixed Blueprint change keeps its normal deploy while routing the
	// maintenance portion through the same typed effects as every other surface.
	sink := &maintenanceAuditSink{}
	svc.Audit = sink
	mixed := strings.Replace(custom, "image: nginx:1", "image: nginx:2", 1)
	mixed = strings.Replace(mixed, "https://status.example.com/maintenance", "https://status.example.com/next", 1)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: mixed}); err != nil {
		t.Fatalf("mixed Blueprint update: %v", err)
	}
	mixedApp := getApp(t, cl, "web")
	if mixedApp.Spec.Image != "nginx:2" || mixedApp.Spec.RestartedAt == restartedAt || mixedApp.Spec.MaintenanceMode == nil ||
		!mixedApp.Spec.MaintenanceMode.Enabled || mixedApp.Spec.MaintenanceMode.URI != "https://status.example.com/next" {
		t.Fatalf("mixed Blueprint state = %+v", mixedApp.Spec)
	}
	var maintenanceEffects []core.AuditEvent
	for _, event := range sink.events {
		if event.Verb == "apps.SetMaintenanceModeURI" || event.Verb == "apps.SetMaintenanceMode" {
			maintenanceEffects = append(maintenanceEffects, event)
		}
	}
	if len(maintenanceEffects) != 2 || maintenanceEffects[0].Verb != "apps.SetMaintenanceModeURI" ||
		maintenanceEffects[1].Verb != "apps.SetMaintenanceMode" || maintenanceEffects[1].MaintenanceModeTo == nil ||
		!*maintenanceEffects[1].MaintenanceModeTo {
		t.Fatalf("mixed Blueprint maintenance effects = %+v", maintenanceEffects)
	}
}

func TestMaintenanceModeBlueprintInvalidStackWritesNothing(t *testing.T) {
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"
	manifest := `services:
  - type: web
    name: valid
    plan: starter
    image: nginx:1
  - type: web
    name: invalid
    plan: starter
    image: nginx:1
    maintenanceMode:
      enabled: true
      uri: https://invalid.onbex.co/maintenance
`
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || validation.Valid || len(validation.Errors) == 0 || !strings.Contains(validation.Errors[0].Error, "same service") {
		t.Fatalf("ValidateBlueprint = %+v, %v", validation, err)
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err == nil || !strings.Contains(err.Error(), "same service") {
		t.Fatalf("DeployStack error = %v", err)
	}
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 0 {
		t.Fatalf("invalid stack partially wrote Apps: %+v", apps.Items)
	}
}

func TestMaintenanceModeBlueprintRejectsInvalidPlacementAndURIWithoutWrites(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name: "free web service",
			manifest: `services:
  - type: web
    name: web
    plan: free
    image: nginx:1
    maintenanceMode:
      enabled: true
`,
			want: "paid web service",
		},
		{
			name: "non-web service",
			manifest: `services:
  - type: worker
    name: worker
    plan: starter
    image: nginx:1
    maintenanceMode:
      enabled: true
`,
			want: "only for web services",
		},
		{
			name: "unsupported URI scheme",
			manifest: `services:
  - type: web
    name: web
    plan: starter
    image: nginx:1
    maintenanceMode:
      enabled: true
      uri: ftp://status.example.com/page
`,
			want: "absolute HTTP(S) URL",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			validation, err := svc.ValidateBlueprint(context.Background(), "", tc.manifest)
			if err != nil || validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, tc.want) {
				t.Fatalf("ValidateBlueprint = %+v, %v", validation, err)
			}
			if validation.Errors[0].Path == nil {
				t.Fatal("validation path is nil")
			}
			if got := *validation.Errors[0].Path; got != "services[0].maintenanceMode" {
				t.Fatalf("validation path = %q, want services[0].maintenanceMode", got)
			}
			if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: tc.manifest}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DeployStack error = %v", err)
			}
			var apps appv1alpha1.AppList
			if err := cl.List(context.Background(), &apps); err != nil {
				t.Fatal(err)
			}
			if len(apps.Items) != 0 {
				t.Fatalf("invalid Blueprint wrote Apps: %+v", apps.Items)
			}
		})
	}
}

func TestMaintenanceModeCreateRESTRequiresBothKeys(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, body := range []string{
		`{"name":"missing-uri","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":true}}}`,
		`{"name":"missing-enabled","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","maintenanceMode":{"uri":""}}}`,
		`{"name":"null-mode","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","maintenanceMode":null}}`,
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "requires enabled and uri") {
			t.Fatalf("invalid create = %d %s", rr.Code, rr.Body.String())
		}
	}
	var before appv1alpha1.AppList
	if err := cl.List(context.Background(), &before); err != nil {
		t.Fatal(err)
	}
	if len(before.Items) != 0 {
		t.Fatalf("invalid REST create wrote Apps: %+v", before.Items)
	}

	body := []byte(`{"name":"web","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":true,"uri":""}}}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/services", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMaintenanceModeRESTPlanTransitions(t *testing.T) {
	enabled := paidWebApp("web")
	enabled.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	svc, cl := newService(nil, enabled)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	patch := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
		return rr
	}

	rejected := patch(`{"serviceDetails":{"plan":"free"}}`)
	if rejected.Code != http.StatusBadRequest || getApp(t, cl, "web").Spec.Tier != "starter" {
		t.Fatalf("enabled downgrade = %d %s; spec=%+v", rejected.Code, rejected.Body, getApp(t, cl, "web").Spec)
	}

	downgrade := patch(`{"serviceDetails":{"plan":"free","maintenanceMode":{"enabled":false,"uri":""}}}`)
	got := getApp(t, cl, "web")
	if downgrade.Code != http.StatusOK || got.Spec.Tier != "free" || got.Spec.MaintenanceMode == nil || got.Spec.MaintenanceMode.Enabled {
		t.Fatalf("disable+downgrade = %d %s; spec=%+v", downgrade.Code, downgrade.Body, got.Spec)
	}

	upgrade := patch(`{"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":false,"uri":""}}}`)
	if upgrade.Code != http.StatusOK || getApp(t, cl, "web").Spec.Tier != "starter" {
		t.Fatalf("upgrade+disabled object = %d %s; spec=%+v", upgrade.Code, upgrade.Body, getApp(t, cl, "web").Spec)
	}
}

type maintenanceAuditSink struct{ events []core.AuditEvent }

func (s *maintenanceAuditSink) Record(_ context.Context, event core.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

type maintenancePatchClient struct {
	client.Client
	patches int
	fail    bool
}

func (c *maintenancePatchClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	if c.fail {
		return errors.New("injected maintenance patch failure")
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestMaintenanceModeChangedFieldsAuditOnceAndNoopDoesNotAudit(t *testing.T) {
	svc, _ := newService(nil, paidWebApp("web"))
	patches := &maintenancePatchClient{Client: svc.Client}
	svc.Client = patches
	sink := &maintenanceAuditSink{}
	svc.Audit = sink
	mode := MaintenanceModeView{Enabled: true, URI: "https://status.example.com/maintenance"}
	if _, err := svc.ConfigureMaintenanceMode(context.Background(), "web", mode); err != nil {
		t.Fatal(err)
	}
	want := []string{"apps.SetMaintenanceModeURI", "apps.SetMaintenanceMode"}
	got := make([]string, len(sink.events))
	for i, event := range sink.events {
		got[i] = event.Verb
		if event.Target != core.ServiceTarget("web") || event.Outcome != core.AuditAllowed {
			t.Fatalf("audit[%d] = %+v", i, event)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("audit verbs = %v, want %v", got, want)
	}
	if patches.patches != 1 {
		t.Fatalf("combined maintenance update used %d patches, want one atomic patch", patches.patches)
	}
	if sink.events[0].MaintenanceModeTo != nil || sink.events[1].MaintenanceModeTo == nil || !*sink.events[1].MaintenanceModeTo {
		t.Fatalf("maintenance audit metadata = [%v, %v], want [nil, true]", sink.events[0].MaintenanceModeTo, sink.events[1].MaintenanceModeTo)
	}
	if _, err := svc.ConfigureMaintenanceMode(context.Background(), "web", mode); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("no-op emitted audit effects: %+v", sink.events)
	}
	if patches.patches != 1 {
		t.Fatalf("no-op wrote App: patches=%d", patches.patches)
	}
	if _, err := svc.ConfigureMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: "https://web.onbex.co/self"}); err == nil {
		t.Fatal("same-service update unexpectedly succeeded")
	}
	if len(sink.events) != 2 {
		t.Fatalf("failed write emitted audit effects: %+v", sink.events)
	}
	if _, err := svc.ConfigureMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: false, URI: mode.URI}); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) != 3 || sink.events[2].Verb != "apps.SetMaintenanceMode" || sink.events[2].MaintenanceModeTo == nil || *sink.events[2].MaintenanceModeTo {
		t.Fatalf("disable audit metadata = %+v, want metadata.to=false", sink.events)
	}
}

func TestMaintenanceModePatchFailureIsAtomicAndEmitsNothing(t *testing.T) {
	svc, backing := newService(nil, paidWebApp("web"))
	patches := &maintenancePatchClient{Client: svc.Client, fail: true}
	svc.Client = patches
	sink := &maintenanceAuditSink{}
	svc.Audit = sink

	_, err := svc.ConfigureMaintenanceMode(context.Background(), "web", MaintenanceModeView{
		Enabled: true,
		URI:     "https://status.example.com/maintenance",
	})
	if err == nil || !strings.Contains(err.Error(), "injected maintenance patch failure") {
		t.Fatalf("ConfigureMaintenanceMode error = %v", err)
	}
	if patches.patches != 1 {
		t.Fatalf("failed maintenance update used %d patches, want one", patches.patches)
	}
	if got := getApp(t, backing, "web").Spec.MaintenanceMode; got != nil {
		t.Fatalf("failed maintenance update partially mutated App: %+v", got)
	}
	if len(sink.events) != 0 {
		t.Fatalf("failed maintenance update emitted effects: %+v", sink.events)
	}
}

func TestMaintenanceModeDeniedAuditUsesCanonicalVerb(t *testing.T) {
	svc, _ := newService(nil, paidWebApp("web"))
	svc.Authz = &fakeChecker{allow: false}
	sink := &maintenanceAuditSink{}
	svc.Audit = sink

	_, err := svc.ConfigureMaintenanceMode(ctxAs("user-x"), "web", MaintenanceModeView{Enabled: true})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("ConfigureMaintenanceMode error = %v, want ErrForbidden", err)
	}
	if len(sink.events) != 1 || sink.events[0].Verb != core.AuditVerbMaintenanceModeEnabled || sink.events[0].Outcome != core.AuditDenied {
		t.Fatalf("denied audit = %+v, want one denied %s event", sink.events, core.AuditVerbMaintenanceModeEnabled)
	}
}

func TestMaintenanceModeGraphQLAndMCPRoundTrip(t *testing.T) {
	svc, backing := newService(nil, paidWebApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { setMaintenanceMode(id:"web", maintenanceMode:{enabled:true, uri:"https://status.example.com/gql"}) { maintenanceMode { enabled uri } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL mutation: %v", res.Errors)
	}
	gqlMode := res.Data.(map[string]any)["setMaintenanceMode"].(map[string]any)["maintenanceMode"].(map[string]any)
	if gqlMode["enabled"] != true || gqlMode["uri"] != "https://status.example.com/gql" {
		t.Fatalf("GraphQL maintenanceMode = %#v", gqlMode)
	}
	badGQL := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { setMaintenanceMode(id:"web", maintenanceMode:{enabled:false, uri:"https://web.onbex.co/self"}) { id } }`})
	if len(badGQL.Errors) != 1 || !strings.Contains(badGQL.Errors[0].Message, "same service") {
		t.Fatalf("GraphQL validation errors = %+v", badGQL.Errors)
	}
	if got := getApp(t, backing, "web").Spec.MaintenanceMode; got == nil || !got.Enabled || got.URI != "https://status.example.com/gql" {
		t.Fatalf("failed GraphQL mutation changed state: %+v", got)
	}

	createReq := (createWebServiceArgs{Name: "new", Image: "nginx:1", Plan: "starter", MaintenanceMode: &maintenanceModeArg{Enabled: true}}).toCreateRequest()
	if createReq.MaintenanceMode == nil || !createReq.MaintenanceMode.Enabled || createReq.MaintenanceMode.URI != "" {
		t.Fatalf("MCP create request = %+v", createReq)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	tool, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "set_maintenance_mode", Arguments: map[string]any{
		"serviceId": "web", "maintenanceMode": map[string]any{"enabled": false, "uri": "https://status.example.com/mcp"},
	}})
	if err != nil || tool.IsError {
		t.Fatalf("MCP set_maintenance_mode: result=%+v err=%v", tool, err)
	}
	data, err := json.Marshal(tool.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var mcpView map[string]any
	if err := json.Unmarshal(data, &mcpView); err != nil {
		t.Fatal(err)
	}
	details := mcpView["serviceDetails"].(map[string]any)
	mcpMode := details["maintenanceMode"].(map[string]any)
	if mcpMode["enabled"] != false || mcpMode["uri"] != "https://status.example.com/mcp" {
		t.Fatalf("MCP maintenanceMode = %#v", mcpMode)
	}
	badTool, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "set_maintenance_mode", Arguments: map[string]any{
		"serviceId": "web", "maintenanceMode": map[string]any{"enabled": true, "uri": "https://web.onbex.co/self"},
	}})
	if err != nil || !badTool.IsError {
		t.Fatalf("invalid MCP set_maintenance_mode: result=%+v err=%v", badTool, err)
	}
	if got := getApp(t, backing, "web").Spec.MaintenanceMode; got == nil || got.Enabled || got.URI != "https://status.example.com/mcp" {
		t.Fatalf("failed MCP mutation changed state: %+v", got)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
