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
		validated(t, mux).ServeHTTP(rr, req)
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
	validated(t, legacyMux).ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/v1/services/legacy", nil))
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
	// A host that canonicalizes to nothing names no service, so it is refused
	// as no URL at all rather than checked against the platform's claims
	// (w5/094).
	if _, err := svc.ConfigureMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: "http://./"}); !strings.Contains(errString(err), "absolute HTTP(S) URL") {
		t.Fatalf("empty-host error = %v", err)
	}
}

func TestMaintenanceModeBlueprintOmissionAndPaidDefault(t *testing.T) {
	stack, err := parseStack(DeployRequest{Manifest: `services:
  - type: web
    name: web
    runtime: image
    image: {url: nginx:1}
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
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: true
      uri: https://status.example.com/maintenance
`
	validation, err := svc.ValidateBlueprint(context.Background(), "", custom, "")
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
    runtime: image
    image: {url: nginx:1}
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
	mixed := strings.Replace(custom, "image: {url: nginx:1}", "image: {url: nginx:2}", 1)
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
    runtime: image
    image: {url: nginx:1}
  - type: web
    name: invalid
    plan: starter
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: true
      uri: https://invalid.onbex.co/maintenance
`
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
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
    runtime: image
    image: {url: nginx:1}
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
    runtime: image
    image: {url: nginx:1}
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
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: true
      uri: ftp://status.example.com/page
`,
			want: "absolute HTTP(S) URL",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			validation, err := svc.ValidateBlueprint(context.Background(), "", tc.manifest, "")
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
		validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
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
	validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/services", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestMaintenanceModeRESTPlanTransitions(t *testing.T) {
	enabled := paidWebApp("web")
	// Free caps at 1 instance (w6/m118); paidWebApp inherits 2 from sampleApp, so
	// pin to 1 to isolate the maintenance-mode transition under test from the
	// separate downgrade-over-cap refusal.
	enabled.Spec.Replicas = 1
	enabled.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	svc, cl := newService(nil, enabled)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	patch := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
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

	upgrade := patch(`{"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":true,"uri":""}}}`)
	upgraded := getApp(t, cl, "web")
	if upgrade.Code != http.StatusOK || upgraded.Spec.Tier != "starter" || upgraded.Spec.MaintenanceMode == nil || !upgraded.Spec.MaintenanceMode.Enabled {
		t.Fatalf("upgrade+enable = %d %s; spec=%+v", upgrade.Code, upgrade.Body, upgraded.Spec)
	}
}

// TestMaintenanceModeMCPPlanTransitions is the MCP twin of
// TestMaintenanceModeRESTPlanTransitions (w1/073): update_service must apply
// disable-maintenance BEFORE the free downgrade, or SetPlan refuses. Before
// the fill armed MaintenanceBeforeFreeDowngrade, a combined call hit the
// late table row and 400'd.
func TestMaintenanceModeMCPPlanTransitions(t *testing.T) {
	enabled := paidWebApp("web")
	// Free caps at 1 instance (w6/m118); pin to 1 so the disable+downgrade flow
	// under test isn't refused by the separate over-cap downgrade guard.
	enabled.Spec.Replicas = 1
	enabled.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	svc, cl := newService(nil, enabled)
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

	rejected, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId": "web", "plan": "free",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !rejected.IsError || getApp(t, cl, "web").Spec.Tier != "starter" {
		t.Fatalf("enabled MCP downgrade isError=%v spec=%+v", rejected.IsError, getApp(t, cl, "web").Spec)
	}

	ok, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId":       "web",
		"plan":            "free",
		"maintenanceMode": map[string]any{"enabled": false, "uri": ""},
	}})
	if err != nil || ok.IsError {
		t.Fatalf("MCP disable+downgrade: result=%+v err=%v", ok, err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.Tier != "free" || got.Spec.MaintenanceMode == nil || got.Spec.MaintenanceMode.Enabled {
		t.Fatalf("MCP disable+downgrade spec=%+v", got.Spec)
	}
}

type maintenanceAuditSink struct{ events []core.AuditEvent }

func (s *maintenanceAuditSink) Record(_ context.Context, event core.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

type auditPatchClient struct {
	client.Client
	patches int
	err     error
}

func (c *auditPatchClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	var po client.PatchOptions
	for _, opt := range opts {
		opt.ApplyToPatch(&po)
	}
	if len(po.DryRun) == 0 {
		c.patches++
	}
	if c.err != nil {
		return c.err
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestMaintenanceModeChangedFieldsAuditOnceAndNoopDoesNotAudit(t *testing.T) {
	svc, _ := newService(nil, paidWebApp("web"))
	patches := &auditPatchClient{Client: svc.Client}
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
	// Render's Maintenance Mode URI Updated fromURI/toURI (w3/m317): from the
	// default page ("") to the new one; the toggle row carries no URI.
	if uri := sink.events[0]; uri.MaintenanceURIFrom == nil || *uri.MaintenanceURIFrom != "" ||
		uri.MaintenanceURITo == nil || *uri.MaintenanceURITo != mode.URI || uri.Relation != core.RelCanCreate {
		t.Fatalf("maintenance URI audit = %+v, want from \"\" to %s under %s", uri, mode.URI, core.RelCanCreate)
	}
	if toggle := sink.events[1]; toggle.MaintenanceURIFrom != nil || toggle.MaintenanceURITo != nil {
		t.Fatalf("maintenance toggle audit carries a URI: %+v", toggle)
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
	patches := &auditPatchClient{Client: svc.Client, err: errors.New("injected maintenance patch failure")}
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
	tool, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId": "web", "maintenanceMode": map[string]any{"enabled": false, "uri": "https://status.example.com/mcp"},
	}})
	if err != nil || tool.IsError {
		t.Fatalf("MCP update_service maintenanceMode: result=%+v err=%v", tool, err)
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
	badTool, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: map[string]any{
		"serviceId": "web", "maintenanceMode": map[string]any{"enabled": true, "uri": "https://web.onbex.co/self"},
	}})
	if err != nil || !badTool.IsError {
		t.Fatalf("invalid MCP update_service maintenanceMode: result=%+v err=%v", badTool, err)
	}
	if got := getApp(t, backing, "web").Spec.MaintenanceMode; got == nil || got.Enabled || got.URI != "https://status.example.com/mcp" {
		t.Fatalf("failed MCP mutation changed state: %+v", got)
	}
}

func TestMaintenanceModeDisabledURIRefusedOnFreeWebAndNonWebAcrossSurfaces(t *testing.T) {
	const customURI = "https://example.com/maint"
	newMCPClient := func(t *testing.T, svc *Service) *mcp.ClientSession {
		t.Helper()
		srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		ctx := context.Background()
		serverT, clientT := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(ctx, serverT, nil); err != nil {
			t.Fatal(err)
		}
		cl, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cl.Close() })
		return cl
	}
	newSchema := func(t *testing.T, svc *Service) graphql.Schema {
		t.Helper()
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		return schema
	}

	t.Run("free web service", func(t *testing.T) {
		freeApp := paidWebApp("free-web")
		freeApp.Spec.Tier = "free"
		freeApp.Spec.Replicas = 1
		svc, backing := newService(nil, freeApp)
		patches := &auditPatchClient{Client: svc.Client}
		svc.Client = patches
		sink := &maintenanceAuditSink{}
		svc.Audit = sink

		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		schema := newSchema(t, svc)
		mcpClient := newMCPClient(t, svc)
		ctx := context.Background()

		// Direct verb
		if _, err := svc.ConfigureMaintenanceMode(ctx, "free-web", MaintenanceModeView{Enabled: false, URI: customURI}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(errString(err), "maintenanceMode requires a paid web service plan") {
			t.Fatalf("direct ConfigureMaintenanceMode error = %v", err)
		}

		// REST PATCH (real + dryRun)
		for _, body := range []string{
			`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`,
			`{"dryRun":true,"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`,
		} {
			rr := httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/services/free-web", strings.NewReader(body)))
			if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "maintenanceMode requires a paid web service plan") {
				t.Fatalf("REST PATCH %s = %d %s", body, rr.Code, rr.Body.String())
			}
		}

		// REST POST create
		createBody := `{"name":"free-create","type":"web_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"free","maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`
		rr := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(createBody)))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "maintenanceMode requires a paid web service plan") {
			t.Fatalf("REST POST create = %d %s", rr.Code, rr.Body.String())
		}

		// GraphQL setMaintenanceMode
		gqlUpdate := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctx,
			RequestString: `mutation { setMaintenanceMode(id:"free-web", maintenanceMode:{enabled:false, uri:"` + customURI + `"}) { id } }`,
		})
		if len(gqlUpdate.Errors) != 1 || !strings.Contains(gqlUpdate.Errors[0].Message, "maintenanceMode requires a paid web service plan") {
			t.Fatalf("GraphQL setMaintenanceMode errors = %+v", gqlUpdate.Errors)
		}

		// GraphQL createService
		gqlCreate := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctx,
			RequestString: `mutation { createService(name:"free-gql", type:"web_service", image:"nginx:1", plan:"free", maintenanceMode:{enabled:false, uri:"` + customURI + `"}) { id } }`,
		})
		if len(gqlCreate.Errors) != 1 || !strings.Contains(gqlCreate.Errors[0].Message, "maintenanceMode requires a paid web service plan") {
			t.Fatalf("GraphQL createService errors = %+v", gqlCreate.Errors)
		}

		// MCP update_service
		mcpUpdate, err := mcpClient.CallTool(ctx, &mcp.CallToolParams{
			Name: "update_service",
			Arguments: map[string]any{
				"serviceId":       "free-web",
				"maintenanceMode": map[string]any{"enabled": false, "uri": customURI},
			},
		})
		if err != nil || !mcpUpdate.IsError {
			t.Fatalf("MCP update_service result=%+v err=%v", mcpUpdate, err)
		}

		// MCP create_web_service
		mcpCreate, err := mcpClient.CallTool(ctx, &mcp.CallToolParams{
			Name: "create_web_service",
			Arguments: map[string]any{
				"name":            "free-mcp",
				"runtime":         "image",
				"image":           "nginx:1",
				"plan":            "free",
				"maintenanceMode": map[string]any{"enabled": false, "uri": customURI},
			},
		})
		if err != nil || !mcpCreate.IsError {
			t.Fatalf("MCP create_web_service result=%+v err=%v", mcpCreate, err)
		}

		// Verify zero audit events before DeployStack (DeployStack records its
		// entry authorization audit before validating the manifest).
		if len(sink.events) != 0 {
			t.Fatalf("refused writes emitted audit events: %+v", sink.events)
		}

		// Blueprint ValidateBlueprint + DeployStack
		manifest := `services:
  - type: web
    name: free-bp
    plan: free
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: false
      uri: ` + customURI + `
`
		validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
		if err != nil || validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, "maintenanceMode requires a paid web service plan") {
			t.Fatalf("ValidateBlueprint = %+v, %v", validation, err)
		}
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(errString(err), "maintenanceMode requires a paid web service plan") {
			t.Fatalf("DeployStack error = %v", err)
		}

		if got := getApp(t, backing, "free-web").Spec.MaintenanceMode; got != nil {
			t.Fatalf("refused writes mutated free-web maintenanceMode: %+v", got)
		}
		if patches.patches != 0 {
			t.Fatalf("refused writes patched App %d times, want 0", patches.patches)
		}
		for _, event := range sink.events {
			if event.Verb == "apps.SetMaintenanceMode" || event.Verb == "apps.SetMaintenanceModeURI" {
				t.Fatalf("refused writes emitted maintenance audit event: %+v", event)
			}
		}
		var all appv1alpha1.AppList
		if err := backing.List(ctx, &all); err != nil {
			t.Fatal(err)
		}
		if len(all.Items) != 1 {
			t.Fatalf("refused creates wrote unexpected Apps: count=%d", len(all.Items))
		}
	})

	for _, tc := range []struct {
		name               string
		svcType            string
		manifest           string
		wantValidateErrSub string
		createReq          CreateRequest
		restCreate         string
	}{
		{
			name:               "static_site",
			svcType:            appv1alpha1.TypeStaticSite,
			wantValidateErrSub: "maintenanceMode",
			manifest: `services:
  - type: web
    name: nonweb-bp
    runtime: static
    repo: https://github.com/bex-co/bex
    staticPublishPath: dist
    maintenanceMode:
      enabled: false
      uri: ` + customURI + `
`,
			createReq: CreateRequest{
				Name:            "nonweb-create",
				Type:            appv1alpha1.TypeStaticSite,
				Repo:            "https://github.com/bex-co/bex",
				PublishPath:     "dist",
				MaintenanceMode: &MaintenanceModeView{Enabled: false, URI: customURI},
			},
			restCreate: `{"name":"nonweb-rest","type":"static_site","repo":"https://github.com/bex-co/bex","serviceDetails":{"publishPath":"dist","maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`,
		},
		{
			name:               "private_service",
			svcType:            appv1alpha1.TypePrivateService,
			wantValidateErrSub: "maintenanceMode is available only for web services",
			manifest: `services:
  - type: pserv
    name: nonweb-bp
    plan: starter
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: false
      uri: ` + customURI + `
`,
			createReq: CreateRequest{
				Name:            "nonweb-create",
				Type:            appv1alpha1.TypePrivateService,
				Plan:            "starter",
				Image:           "nginx:1",
				MaintenanceMode: &MaintenanceModeView{Enabled: false, URI: customURI},
			},
			restCreate: `{"name":"nonweb-rest","type":"private_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`,
		},
		{
			name:               "background_worker",
			svcType:            appv1alpha1.TypeBackgroundWorker,
			wantValidateErrSub: "maintenanceMode is available only for web services",
			manifest: `services:
  - type: worker
    name: nonweb-bp
    plan: starter
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: false
      uri: ` + customURI + `
`,
			createReq: CreateRequest{
				Name:            "nonweb-create",
				Type:            appv1alpha1.TypeBackgroundWorker,
				Plan:            "starter",
				Image:           "nginx:1",
				MaintenanceMode: &MaintenanceModeView{Enabled: false, URI: customURI},
			},
			restCreate: `{"name":"nonweb-rest","type":"background_worker","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`,
		},
		{
			name:               "cron_job",
			svcType:            appv1alpha1.TypeCronJob,
			wantValidateErrSub: "maintenanceMode",
			manifest: `services:
  - type: cron
    name: nonweb-bp
    plan: starter
    runtime: image
    image: {url: nginx:1}
    schedule: "*/5 * * * *"
    maintenanceMode:
      enabled: false
      uri: ` + customURI + `
`,
			createReq: CreateRequest{
				Name:            "nonweb-create",
				Type:            appv1alpha1.TypeCronJob,
				Plan:            "starter",
				Image:           "nginx:1",
				Schedule:        "*/5 * * * *",
				MaintenanceMode: &MaintenanceModeView{Enabled: false, URI: customURI},
			},
			restCreate: `{"name":"nonweb-rest","type":"cron_job","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"starter","schedule":"*/5 * * * *","maintenanceMode":{"enabled":false,"uri":"` + customURI + `"}}}`,
		},
	} {
		t.Run("non-web "+tc.name, func(t *testing.T) {
			app := paidWebApp("nonweb")
			app.Spec.Type = tc.svcType
			if tc.svcType == appv1alpha1.TypeStaticSite {
				app.Spec.Tier = "free"
				app.Spec.Repo = "https://github.com/bex-co/bex"
				app.Spec.Image = ""
				app.Spec.PublishPath = "dist"
			}
			if tc.svcType == appv1alpha1.TypeCronJob {
				app.Spec.Schedule = "*/5 * * * *"
			}
			svc, backing := newService(nil, app)
			patches := &auditPatchClient{Client: svc.Client}
			svc.Client = patches
			sink := &maintenanceAuditSink{}
			svc.Audit = sink

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			schema := newSchema(t, svc)
			mcpClient := newMCPClient(t, svc)
			ctx := context.Background()

			// Direct verb
			if _, err := svc.ConfigureMaintenanceMode(ctx, "nonweb", MaintenanceModeView{Enabled: false, URI: customURI}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(errString(err), "maintenanceMode is available only for web services") {
				t.Fatalf("direct ConfigureMaintenanceMode error = %v", err)
			}

			// Direct Create
			if _, err := svc.Create(ctx, tc.createReq); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(errString(err), "maintenanceMode is available only for web services") {
				t.Fatalf("direct Create error = %v", err)
			}

			// REST PATCH
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/services/nonweb", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":"`+customURI+`"}}}`)))
			if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "maintenanceMode is available only for web services") {
				t.Fatalf("REST PATCH = %d %s", rr.Code, rr.Body.String())
			}

			// REST POST create
			rr = httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(tc.restCreate)))
			if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "maintenanceMode is available only for web services") {
				t.Fatalf("REST POST create = %d %s", rr.Code, rr.Body.String())
			}

			// GraphQL setMaintenanceMode
			gqlUpdate := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       ctx,
				RequestString: `mutation { setMaintenanceMode(id:"nonweb", maintenanceMode:{enabled:false, uri:"` + customURI + `"}) { id } }`,
			})
			if len(gqlUpdate.Errors) != 1 || !strings.Contains(gqlUpdate.Errors[0].Message, "maintenanceMode is available only for web services") {
				t.Fatalf("GraphQL setMaintenanceMode errors = %+v", gqlUpdate.Errors)
			}

			// MCP update_service
			mcpUpdate, err := mcpClient.CallTool(ctx, &mcp.CallToolParams{
				Name: "update_service",
				Arguments: map[string]any{
					"serviceId":       "nonweb",
					"maintenanceMode": map[string]any{"enabled": false, "uri": customURI},
				},
			})
			if err != nil || !mcpUpdate.IsError {
				t.Fatalf("MCP update_service result=%+v err=%v", mcpUpdate, err)
			}

			if len(sink.events) != 0 {
				t.Fatalf("refused writes emitted audit events before DeployStack: %+v", sink.events)
			}

			// Blueprint ValidateBlueprint + DeployStack
			validation, err := svc.ValidateBlueprint(ctx, "", tc.manifest, "")
			if err != nil || validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, tc.wantValidateErrSub) {
				t.Fatalf("ValidateBlueprint = %+v, %v", validation, err)
			}
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(errString(err), tc.wantValidateErrSub) {
				t.Fatalf("DeployStack error = %v", err)
			}

			if got := getApp(t, backing, "nonweb").Spec.MaintenanceMode; got != nil {
				t.Fatalf("refused writes mutated nonweb maintenanceMode: %+v", got)
			}
			if patches.patches != 0 {
				t.Fatalf("refused writes patched App %d times, want 0", patches.patches)
			}
			for _, event := range sink.events {
				if event.Verb == "apps.SetMaintenanceMode" || event.Verb == "apps.SetMaintenanceModeURI" {
					t.Fatalf("refused writes emitted maintenance audit event: %+v", event)
				}
			}
		})
	}
}

func TestMaintenanceModeDisabledURIControlsAndDowngradeCompatibility(t *testing.T) {
	ctx := context.Background()

	// 1. Free web echo of inert {enabled:false, uri:""} on create & update succeeds (w9/068).
	t.Run("free web inert echo", func(t *testing.T) {
		freeApp := paidWebApp("free-web")
		freeApp.Spec.Tier = "free"
		freeApp.Spec.Replicas = 1
		svc, _ := newService(nil, freeApp)
		patches := &auditPatchClient{Client: svc.Client}
		svc.Client = patches
		sink := &maintenanceAuditSink{}
		svc.Audit = sink
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		rr := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/services/free-web", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":""}}}`)))
		if rr.Code != http.StatusOK {
			t.Fatalf("free echo PATCH = %d %s", rr.Code, rr.Body.String())
		}
		if patches.patches != 0 || len(sink.events) != 0 {
			t.Fatalf("free inert echo mutated or audited: patches=%d events=%+v", patches.patches, sink.events)
		}

		createRR := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(createRR, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(`{"name":"free-clone","type":"web_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"plan":"free","maintenanceMode":{"enabled":false,"uri":""}}}`)))
		if createRR.Code != http.StatusCreated {
			t.Fatalf("free inert create = %d %s", createRR.Code, createRR.Body.String())
		}
	})

	// 2. Paid web service setting {enabled:false, uri:"https://..."} succeeds and audits URI change.
	t.Run("paid web disabled custom URI", func(t *testing.T) {
		svc, backing := newService(nil, paidWebApp("paid-web"))
		sink := &maintenanceAuditSink{}
		svc.Audit = sink
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		rr := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rr, httptest.NewRequest(http.MethodPatch, "/v1/services/paid-web", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":"https://example.com/maint"}}}`)))
		if rr.Code != http.StatusOK {
			t.Fatalf("paid PATCH = %d %s", rr.Code, rr.Body.String())
		}
		got := getApp(t, backing, "paid-web").Spec.MaintenanceMode
		if got == nil || got.Enabled || got.URI != "https://example.com/maint" {
			t.Fatalf("paid maintenanceMode = %+v", got)
		}
		if len(sink.events) != 1 || sink.events[0].Verb != "apps.SetMaintenanceModeURI" ||
			sink.events[0].MaintenanceURIFrom == nil || *sink.events[0].MaintenanceURIFrom != "" ||
			sink.events[0].MaintenanceURITo == nil || *sink.events[0].MaintenanceURITo != "https://example.com/maint" {
			t.Fatalf("paid audit events = %+v", sink.events)
		}
	})

	// 3. Blueprint with omitted plan and disabled maintenanceMode + custom URI defaults to starter.
	t.Run("blueprint omitted plan defaults to starter", func(t *testing.T) {
		svc, backing := newService(nil)
		manifest := `services:
  - type: web
    name: bp-default-paid
    runtime: image
    image: {url: nginx:1}
    maintenanceMode:
      enabled: false
      uri: https://example.com/maint
`
		validation, err := svc.ValidateBlueprint(ctx, "", manifest, "")
		if err != nil || !validation.Valid {
			t.Fatalf("ValidateBlueprint = %+v, %v", validation, err)
		}
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
		got := getApp(t, backing, "bp-default-paid")
		if got.Spec.Tier != "starter" || got.Spec.MaintenanceMode == nil || got.Spec.MaintenanceMode.Enabled || got.Spec.MaintenanceMode.URI != "https://example.com/maint" {
			t.Fatalf("deployed app = tier:%q mode:%+v", got.Spec.Tier, got.Spec.MaintenanceMode)
		}
	})

	// 4. Downgraded free web service carrying a legacy stored URI:
	//    echoing unchanged succeeds with zero writes/events, changing to a new URI
	//    or enabling is refused, Blueprint resync omitting maintenanceMode still works,
	//    clearing to "" succeeds and audits, and upgrading + setting a new URI succeeds.
	t.Run("downgraded free web legacy URI", func(t *testing.T) {
		const legacyURI = "https://legacy.example.com/maint"
		downgraded := paidWebApp("downgraded")
		downgraded.Spec.Tier = "free"
		downgraded.Spec.Replicas = 1
		downgraded.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: false, URI: legacyURI}

		svc, backing := newService(nil, downgraded)
		patches := &auditPatchClient{Client: svc.Client}
		svc.Client = patches
		sink := &maintenanceAuditSink{}
		svc.Audit = sink
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		// Echo unchanged legacy URI -> 200, no patch, no audit
		echoRR := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(echoRR, httptest.NewRequest(http.MethodPatch, "/v1/services/downgraded", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":"`+legacyURI+`"}}}`)))
		if echoRR.Code != http.StatusOK {
			t.Fatalf("legacy echo PATCH = %d %s", echoRR.Code, echoRR.Body.String())
		}
		if patches.patches != 0 || len(sink.events) != 0 {
			t.Fatalf("legacy echo mutated or audited: patches=%d events=%+v", patches.patches, sink.events)
		}

		// Blueprint resync omitting maintenanceMode preserves legacy URI without failing
		omittedManifest := `services:
  - type: web
    name: downgraded
    plan: free
    runtime: image
    image: {url: nginx:1}
`
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: omittedManifest}); err != nil {
			t.Fatalf("Blueprint resync omitting maintenanceMode on downgraded free web: %v", err)
		}
		if got := getApp(t, backing, "downgraded").Spec.MaintenanceMode; got == nil || got.URI != legacyURI {
			t.Fatalf("Blueprint resync lost legacy URI: %+v", got)
		}
		sink.events = nil

		// Changing to a different URI on free -> 400
		changeRR := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(changeRR, httptest.NewRequest(http.MethodPatch, "/v1/services/downgraded", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":"https://other.example.com/maint"}}}`)))
		if changeRR.Code != http.StatusBadRequest || !strings.Contains(changeRR.Body.String(), "maintenanceMode requires a paid web service plan") {
			t.Fatalf("legacy URI change on free = %d %s", changeRR.Code, changeRR.Body.String())
		}

		// Enabling on free -> 400
		enableRR := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(enableRR, httptest.NewRequest(http.MethodPatch, "/v1/services/downgraded", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":"`+legacyURI+`"}}}`)))
		if enableRR.Code != http.StatusBadRequest || !strings.Contains(enableRR.Body.String(), "maintenanceMode requires a paid web service plan") {
			t.Fatalf("legacy enable on free = %d %s", enableRR.Code, enableRR.Body.String())
		}

		// Clearing legacy URI -> 200, emits URI update from legacyURI to ""
		clearRR := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(clearRR, httptest.NewRequest(http.MethodPatch, "/v1/services/downgraded", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":""}}}`)))
		if clearRR.Code != http.StatusOK {
			t.Fatalf("legacy clear PATCH = %d %s", clearRR.Code, clearRR.Body.String())
		}
		if got := getApp(t, backing, "downgraded").Spec.MaintenanceMode; got == nil || got.Enabled || got.URI != "" {
			t.Fatalf("cleared maintenanceMode = %+v", got)
		}
		if len(sink.events) != 1 || sink.events[0].Verb != "apps.SetMaintenanceModeURI" ||
			sink.events[0].MaintenanceURIFrom == nil || *sink.events[0].MaintenanceURIFrom != legacyURI ||
			sink.events[0].MaintenanceURITo == nil || *sink.events[0].MaintenanceURITo != "" {
			t.Fatalf("clear audit events = %+v", sink.events)
		}

		// Upgrading plan + setting custom URI in one PATCH -> 200
		upgradeRR := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(upgradeRR, httptest.NewRequest(http.MethodPatch, "/v1/services/downgraded", strings.NewReader(`{"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":false,"uri":"https://upgraded.example.com/maint"}}}`)))
		if upgradeRR.Code != http.StatusOK {
			t.Fatalf("upgrade + URI PATCH = %d %s", upgradeRR.Code, upgradeRR.Body.String())
		}
		got := getApp(t, backing, "downgraded")
		if got.Spec.Tier != "starter" || got.Spec.MaintenanceMode == nil || got.Spec.MaintenanceMode.Enabled || got.Spec.MaintenanceMode.URI != "https://upgraded.example.com/maint" {
			t.Fatalf("upgraded app = tier:%q mode:%+v", got.Spec.Tier, got.Spec.MaintenanceMode)
		}
	})
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
