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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRenderServiceSupportsOfficialCLICloneAndRename(t *testing.T) {
	a := view(&appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "immutable-id",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)),
			Annotations: map[string]string{
				resourcemeta.UpdatedAtAnnotation: "2026-07-14T12:05:00Z",
			},
		},
		Spec: appv1alpha1.AppSpec{
			DisplayName: "Friendly API",
			Image:       "nginx:alpine",
			Runtime:     "image",
			Replicas:    2,
		},
	})

	got := toRenderService(a)
	if got.ID != "immutable-id" || got.Name != "Friendly API" {
		t.Fatalf("identity/name = %q/%q, want immutable-id/Friendly API", got.ID, got.Name)
	}
	if got.ImagePath != "nginx:alpine" {
		t.Fatalf("imagePath = %q, want configured image", got.ImagePath)
	}
	if got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Fatalf("CLI-required timestamps missing: created=%q updated=%q", got.CreatedAt, got.UpdatedAt)
	}
	if got.UpdatedAt == got.CreatedAt {
		t.Fatalf("updatedAt aliases createdAt: %q", got.UpdatedAt)
	}
	if n, ok := got.ServiceDetails["numInstances"].(int); !ok || n != 2 {
		t.Fatalf("serviceDetails.numInstances = %#v, want 2", got.ServiceDetails["numInstances"])
	}
}

func TestRESTUsesAndAcceptsTypedServiceIDAfterRename(t *testing.T) {
	app := sampleApp("tea-a-web")
	app.Labels = map[string]string{
		core.LabelTenant:      "tea-a",
		core.LabelServiceName: "web",
		store.LabelAppID:      "srv-d9example",
	}
	app.Spec.DisplayName = "Customer API"
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services?name=Customer+API", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET renamed service = %d: %s", rec.Code, rec.Body.String())
	}
	var listed []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Service.ID != "srv-d9example" {
		t.Fatalf("listed identity = %#v, want srv-d9example", listed)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/srv-d9example", strings.NewReader(`{"name":"Renamed Again"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH by typed id = %d: %s", rec.Code, rec.Body.String())
	}
	if got := getApp(t, cl, "tea-a-web").Spec.DisplayName; got != "Renamed Again" {
		t.Fatalf("display name = %q, want Renamed Again", got)
	}
	var updated renderService
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID != "srv-d9example" || updated.Name != "Renamed Again" {
		t.Fatalf("PATCH identity/name = %q/%q", updated.ID, updated.Name)
	}
}

func TestRESTPatchAcceptsOfficialCLIServiceBody(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Runtime = "node"
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{
		"name":"Customer API",
		"serviceDetails":{
			"plan":"standard",
			"healthCheckPath":"/ready",
			"preDeployCommand":"npm run migrate",
			"envSpecificDetails":{"buildCommand":"npm ci","startCommand":"npm start"}
		}
	}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH official CLI body = %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out["id"] != "web" || out["name"] != "Customer API" {
		t.Fatalf("PATCH response identity/name = %#v/%#v", out["id"], out["name"])
	}
	got := getApp(t, cl, "web")
	if got.Spec.DisplayName != "Customer API" || got.Spec.Tier != "standard" || got.Spec.HealthCheckPath != "/ready" || got.Spec.PreDeployCommand != "npm run migrate" || got.Spec.BuildCommand != "npm ci" || got.Spec.StartCommand != "npm start" {
		t.Fatalf("official CLI PATCH did not persist all supported fields: %+v", got.Spec)
	}
}

type recordingSecretFileSeeder struct {
	service string
	files   []core.SecretFile
}

type fixedEnvironmentResolver map[string]store.Environment

func (r fixedEnvironmentResolver) ResolveForCreate(_ context.Context, environmentID, workspaceID string) (core.EnvironmentAssignment, error) {
	e, ok := r[environmentID]
	if !ok {
		return core.EnvironmentAssignment{}, core.ErrNotFound
	}
	if e.TenantID != workspaceID {
		return core.EnvironmentAssignment{}, core.ErrForbidden
	}
	return core.EnvironmentAssignment{ID: e.ID, ProjectID: e.ProjectID, WorkspaceID: e.TenantID}, nil
}

func (s *recordingSecretFileSeeder) PrepareSecretFiles(_ context.Context, service string, app *appv1alpha1.App, files []core.SecretFile) error {
	s.service = service
	s.files = append([]core.SecretFile(nil), files...)
	app.Spec.FilesFromSecrets = []string{app.Name + "-files"}
	return nil
}

func (*recordingSecretFileSeeder) CommitSecretFiles(context.Context, string, *appv1alpha1.App) error {
	return nil
}

func (*recordingSecretFileSeeder) AbortSecretFiles(context.Context, string, *appv1alpha1.App) error {
	return nil
}

func TestRESTCreateSeedsOfficialCLISecretFiles(t *testing.T) {
	seeder := &recordingSecretFileSeeder{}
	svc, _ := newService(nil)
	svc.SecretFileSeeder = seeder
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","type":"web_service","image":{"imagePath":"nginx:alpine"},"secretFiles":[{"name":"app-secret","content":"top-secret"}]}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST with secretFiles = %d: %s", rec.Code, rec.Body.String())
	}
	if seeder.service != "web" || len(seeder.files) != 1 || seeder.files[0].Name != "app-secret" || seeder.files[0].Content != "top-secret" {
		t.Fatalf("seed call = service %q, files %#v", seeder.service, seeder.files)
	}
}

func TestRESTCreateRejectsSecretFilesBeforeWriteWhenUnavailable(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","type":"web_service","image":{"imagePath":"nginx:alpine"},"secretFiles":[{"name":"app-secret","content":"top-secret"}]}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("POST with unavailable secretFiles = %d: %s", rec.Code, rec.Body.String())
	}
	var app appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); !apierrors.IsNotFound(err) {
		t.Fatalf("service was written despite preflight failure: %v", err)
	}
}

func TestRESTRejectsUnsupportedOfficialCLIFieldsInsteadOfSilentlyDroppingThem(t *testing.T) {
	createBodies := map[string]string{
		"previews":            `{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{"previews":{"generation":"manual"}}}`,
		"registry credential": `{"name":"new","type":"web_service","image":{"imagePath":"nginx","registryCredentialId":"rgc-1"}}`,
	}
	for name, body := range createBodies {
		t.Run("create "+name, func(t *testing.T) {
			svc, _ := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
		})
	}

	patchBodies := map[string]string{
		"previews":            `{"serviceDetails":{"previews":{"generation":"manual"}}}`,
		"registry credential": `{"image":{"imagePath":"nginx","registryCredentialId":"rgc-1"}}`,
	}
	for name, body := range patchBodies {
		t.Run("patch "+name, func(t *testing.T) {
			svc, _ := newService(nil, sampleApp("web"))
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRESTPatchAcceptsOfficialCLISourceFields(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Branch = "main"
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{"image":{"imagePath":"nginx:stable"},"branch":"release"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH image/branch = %d: %s", rec.Code, rec.Body.String())
	}
	got := getApp(t, cl, "web")
	if got.Spec.Image != "nginx:stable" || got.Spec.Repo != "" || got.Spec.Branch != "release" {
		t.Fatalf("image source update = %+v", got.Spec)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{"repo":"https://github.com/acme/api","branch":"next"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH repo/branch = %d: %s", rec.Code, rec.Body.String())
	}
	got = getApp(t, cl, "web")
	if got.Spec.Repo != "https://github.com/acme/api" || got.Spec.Image != "" || got.Spec.Branch != "next" {
		t.Fatalf("repo source update = %+v", got.Spec)
	}
}

func TestRESTCreateDecodesOfficialCLITypeSpecificDetails(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		wantCheck func(*testing.T, appv1alpha1.AppSpec)
	}{
		{
			name: "cron command from env-specific start command",
			body: `{"name":"nightly","type":"cron_job","image":{"imagePath":"alpine:3.20"},` +
				`"serviceDetails":{"runtime":"image","schedule":"0 * * * *","envSpecificDetails":{"startCommand":"echo hourly"}}}`,
			wantCheck: func(t *testing.T, spec appv1alpha1.AppSpec) {
				if spec.Schedule != "0 * * * *" || spec.Command != "echo hourly" {
					t.Fatalf("cron details = schedule %q command %q", spec.Schedule, spec.Command)
				}
			},
		},
		{
			name: "static build and publish directories",
			body: `{"name":"site","type":"static_site","repo":"https://github.com/acme/site",` +
				`"serviceDetails":{"buildCommand":"npm run build","publishPath":"dist"}}`,
			wantCheck: func(t *testing.T, spec appv1alpha1.AppSpec) {
				if spec.BuildCommand != "npm run build" || spec.PublishPath != "dist" {
					t.Fatalf("static details = build %q publish %q", spec.BuildCommand, spec.PublishPath)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(tc.body)))
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
			}
			var list appv1alpha1.AppList
			if err := cl.List(context.Background(), &list); err != nil || len(list.Items) != 1 {
				t.Fatalf("created Apps = %d, err=%v", len(list.Items), err)
			}
			tc.wantCheck(t, list.Items[0].Spec)
		})
	}
}

func TestCreateAssignsOfficialCLIEnvironmentID(t *testing.T) {
	rec := &recordingStore{environments: map[string]store.Environment{
		"env-staging": {ID: "env-staging", ProjectID: "prj-platform", TenantID: "tea-a"},
	}}
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
	svc.Environments = fixedEnvironmentResolver(rec.environments)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

	got, err := svc.create(ctx, CreateRequest{Name: "web", Image: "nginx:alpine", EnvironmentID: "env-staging"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.appCreates) != 1 || rec.appCreates[0].ProjectID != "prj-platform" || rec.appCreates[0].EnvironmentID != "env-staging" {
		t.Fatalf("store association = %#v", rec.appCreates)
	}
	if got.ProjectID != "prj-platform" || got.EnvironmentID != "env-staging" {
		t.Fatalf("create response association = %+v", got)
	}
	rendered := toRenderService(got)
	if rendered.ProjectID != "prj-platform" || rendered.EnvironmentID != "env-staging" {
		t.Fatalf("REST response association = %+v", rendered)
	}
	app := getTenantApp(t, cl, "tea-a", "web")
	if app.Labels[core.LabelProject] != "prj-platform" || app.Labels[core.LabelEnvironment] != "env-staging" {
		t.Fatalf("projected association labels = %v", app.Labels)
	}
}

func TestCreateEnvironmentRejectsUnknownAndForeignBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name         string
		environments fixedEnvironmentResolver
		want         error
	}{
		{name: "unknown", environments: fixedEnvironmentResolver{}, want: core.ErrNotFound},
		{name: "foreign", environments: fixedEnvironmentResolver{
			"env-staging": {ID: "env-staging", ProjectID: "prj-platform", TenantID: "tea-b"},
		}, want: core.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingStore{}
			svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, rec)
			svc.Environments = tc.environments
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
			_, err := svc.create(ctx, CreateRequest{Name: "web", Image: "nginx:alpine", EnvironmentID: "env-staging"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("create error = %v, want %v", err, tc.want)
			}
			var apps appv1alpha1.AppList
			if listErr := cl.List(context.Background(), &apps); listErr != nil || len(apps.Items) != 0 || len(rec.appCreates) != 0 {
				t.Fatalf("failed resolution wrote apps=%d store rows=%d, err=%v", len(apps.Items), len(rec.appCreates), listErr)
			}
		})
	}
}

func TestRESTListServiceInstancesForOfficialCLI(t *testing.T) {
	app := sampleApp("web")
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:              "web-7c8d9",
		Namespace:         "default",
		Labels:            map[string]string{core.PodLabelApp: "web"},
		CreationTimestamp: metav1.NewTime(time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)),
	}}
	cl := fakeClient(app, pod)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/web/instances", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET instances = %d: %s", rec.Code, rec.Body.String())
	}
	var got []renderServiceInstance
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0].ID, "web-") || got[0].ID == "web-7c8d9" || got[0].CreatedAt == "" {
		t.Fatalf("instances = %#v", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/missing/instances", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing instances = %d, want 404", rec.Code)
	}
}

func TestRESTNameFilterUsesMutableRenderName(t *testing.T) {
	app := sampleApp("immutable-id")
	app.Spec.DisplayName = "Friendly API"
	svc, _ := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services?name=Friendly+API", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET by mutable name = %d: %s", rec.Code, rec.Body.String())
	}
	var got []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Service.ID != "immutable-id" || got[0].Service.Name != "Friendly API" {
		t.Fatalf("name-filter result = %#v", got)
	}
}

func TestRESTEnvironmentFilterUsesProjectedMembership(t *testing.T) {
	staging := sampleApp("staging")
	staging.Labels = map[string]string{core.LabelEnvironment: "env-staging"}
	production := sampleApp("production")
	production.Labels = map[string]string{core.LabelEnvironment: "env-production"}
	svc, _ := newService(nil, staging, production)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services?environmentId=env-staging", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET by environment = %d: %s", rec.Code, rec.Body.String())
	}
	var got []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Service.ID != "staging" || got[0].Service.EnvironmentID != "env-staging" {
		t.Fatalf("environment-filter result = %#v", got)
	}
}

func TestRESTEnvironmentFilterAcceptsOfficialCLIMultipleValueEncoding(t *testing.T) {
	staging := sampleApp("staging")
	staging.Labels = map[string]string{core.LabelEnvironment: "env-staging"}
	production := sampleApp("production")
	production.Labels = map[string]string{core.LabelEnvironment: "env-production"}
	preview := sampleApp("preview")
	preview.Labels = map[string]string{core.LabelEnvironment: "env-preview"}
	svc, _ := newService(nil, staging, production, preview)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services?environmentId=env-staging%2Cenv-production", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET by multiple environments = %d: %s", rec.Code, rec.Body.String())
	}
	var got []serviceWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Service.ID != "production" || got[1].Service.ID != "staging" {
		t.Fatalf("multi-environment result = %#v", got)
	}
}

func TestSetCommandsMapsCronStartCommand(t *testing.T) {
	app := sampleApp("nightly")
	app.Spec.Type = appv1alpha1.TypeCronJob
	svc, cl := newService(nil, app)
	cmd := "npm run report"
	if _, err := svc.SetCommands(context.Background(), "nightly", nil, &cmd); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "nightly").Spec.Command; got != cmd {
		t.Fatalf("cron command = %q, want %q", got, cmd)
	}
}
