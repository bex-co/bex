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

// blueprint_generate_test.go covers w8/m22: exporting live resources as a
// render.yaml that bex's own validator accepts, with no secret values, the
// reference forms where derivable, and byte-identical output across surfaces.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeEnvNames struct{ names map[string][]string }

func (f fakeEnvNames) ListEnvVarNames(_ context.Context, service string) ([]string, error) {
	return f.names[service], nil
}

type fakeEnvGroupExport struct {
	groups map[string]struct {
		name string
		keys []string
	}
}

func (f fakeEnvGroupExport) ExportEnvGroup(_ context.Context, gid string) (string, []string, error) {
	g, ok := f.groups[gid]
	if !ok {
		return "", nil, core.ErrNotFound
	}
	return g.name, g.keys, nil
}

func generateFixtureService() *Service {
	web := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type:            appv1alpha1.TypeWebService,
			Repo:            "https://github.com/acme/web",
			Branch:          "release",
			Runtime:         "docker",
			DockerfilePath:  "Dockerfile",
			DockerContext:   "apps/web",
			StartCommand:    "bin/server",
			Tier:            "pro-plus",
			HealthCheckPath: "/healthz",
			Replicas:        1,
			Env: []appv1alpha1.EnvVar{
				{Name: "LOG_LEVEL", Value: "info"},
				{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "dpg-abc123-app", Key: "uri"}}},
				{Name: "REDIS_URL", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "red-xyz789", Key: "uri"}}},
				{Name: "FOREIGN_SECRET", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "some-other-secret", Key: "token"}}},
			},
		},
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-abc123", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "app-db", Plan: "1c-2g", StorageGB: 20, Version: "16",
			HighAvailability: true,
		},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-xyz789", Namespace: "default"},
		Spec: appv1alpha1.KeyValueSpec{
			Name: "app-cache", Plan: "standard", MaxmemoryPolicy: "allkeys-lru",
		},
	}
	cl := fakeClient(web, db, kv)
	return &Service{
		Base:     &core.Base{Client: cl, Namespace: "default"},
		EnvNames: fakeEnvNames{names: map[string][]string{"web": {"SMTP_PASSWORD"}}},
	}
}

func TestGenerateBlueprintRoundTrip(t *testing.T) {
	svc := generateFixtureService()
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		ServiceIDs:  []string{"web"},
		PostgresIDs: []string{"dpg-abc123"},
		KeyValueIDs: []string{"red-xyz789"},
	})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if out.Filename != "render.yaml" {
		t.Errorf("filename = %q", out.Filename)
	}

	// (a) The platform's own validator accepts the generated manifest.
	v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("generated manifest must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}

	// (b) The reference forms and sync:false classification.
	for _, want := range []string{
		"fromDatabase", "app-db", "connectionString",
		"fromService", "app-cache", "keyvalue",
		"sync: false", "FOREIGN_SECRET", "SMTP_PASSWORD",
		"plan: pro plus", "dockerContext: apps/web", "dockerCommand: bin/server",
		"diskSizeGB: 20", "postgresMajorVersion: \"16\"", "highAvailability",
		"maxmemoryPolicy: allkeys-lru",
	} {
		if !strings.Contains(out.Manifest, want) {
			t.Errorf("generated manifest missing %q:\n%s", want, out.Manifest)
		}
	}

	// The literal env var keeps its value; secret-backed ones never carry one.
	if !strings.Contains(out.Manifest, "value: info") {
		t.Errorf("literal env value missing:\n%s", out.Manifest)
	}
	for _, secretByte := range []string{"dpg-abc123-app", "some-other-secret", "red-xyz789"} {
		if strings.Contains(out.Manifest, secretByte) {
			t.Errorf("generated manifest leaks internal secret/CR name %q:\n%s", secretByte, out.Manifest)
		}
	}
}

// TestGenerateBlueprintServiceNameIsPublicNotCRName covers w6/m114: a
// store-managed App's object name is CRName(tenant, name) — tenant-prefixed and
// past ValidResourceName's 30-char cap — so exporting a.Name produced a render.yaml
// that bex's own validateBlueprint rejected AND leaked the workspace tenant id
// into a file the user is told to commit. Every prior fixture used a bare CR
// name (LabelServiceName absent), the legacy path where a.Name already is the
// public name, so none of them exercised this. The generator's self-check runs
// only compile+parse, not the create boundary validateBlueprint runs, which is
// why the bad name reached users.
func TestGenerateBlueprintServiceNameIsPublicNotCRName(t *testing.T) {
	const (
		publicName = "qa-20260826-webhook-svc"
		tenantID   = "tea-d98210cbbpdc73dcrkvg"
		appID      = "srv-da7o6ovvqdcc73bpn9hg"
		crName     = tenantID + "-" + publicName // 48 chars: past the 30-char cap
	)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: "default",
			Labels: map[string]string{
				core.LabelServiceName: publicName,
				core.LabelTenant:      tenantID,
				core.LabelAppID:       appID,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:    appv1alpha1.TypeWebService,
			Repo:    "https://github.com/bex-co/bex-hello-go-live.git",
			Runtime: "go", BuildCommand: "go build -o app .", StartCommand: "./app",
		},
	}
	cl := fakeClient(app)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{appID}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	// The exporter must emit the public name a user would type to recreate it.
	if !strings.Contains(out.Manifest, "name: "+publicName) {
		t.Errorf("manifest must carry the public name %q:\n%s", publicName, out.Manifest)
	}
	// Not the tenant-prefixed CR object name, and no tenant id anywhere: this is
	// a file the user commits to a repo.
	if strings.Contains(out.Manifest, "tea-") {
		t.Errorf("manifest leaks the tenant id:\n%s", out.Manifest)
	}
	if strings.Contains(out.Manifest, crName) {
		t.Errorf("manifest carries the CR object name:\n%s", out.Manifest)
	}
	// The loop is closed: the platform's own validator accepts what it produced.
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("generated manifest must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}
}

func TestGenerateBlueprintDomainsCronAndWorkerScaling(t *testing.T) {
	cpu := int32(70)
	web := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "nginx:1",
			Host: "www.example.com", Hosts: []string{"example.com"},
		},
	}
	cron := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeCronJob, Image: "job:1",
			Schedule: "0 0 * * *", Command: "bin/report",
		},
	}
	worker := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "crunch", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			// A paid tier: free caps at 1 instance (w6/m118), so autoscaling to 4
			// is only valid on a plan without the cap.
			Type: appv1alpha1.TypeBackgroundWorker, Image: "worker:1", Tier: "standard",
			Autoscaling: &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 4, TargetCPUPercent: &cpu},
		},
	}
	cl := fakeClient(web, cron, worker)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		ServiceIDs: []string{"site", "nightly", "crunch"},
	})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}
	for _, want := range []string{
		"www.example.com", "example.com", // primary + additional domains
		"dockerCommand: bin/report", // an image cron's command from Spec.Command (w5/080)
		"numInstances: 4",           // worker autoscaling → fixed upper bound
	} {
		if !strings.Contains(out.Manifest, want) {
			t.Errorf("manifest missing %q:\n%s", want, out.Manifest)
		}
	}
	if strings.Contains(out.Manifest, "scaling:") {
		t.Errorf("worker must not emit a scaling block:\n%s", out.Manifest)
	}
}

func TestGenerateBlueprintEmptySelectionRejected(t *testing.T) {
	svc := generateFixtureService()
	if _, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{}); err == nil {
		t.Fatal("empty selection must be rejected")
	}
}

func TestGenerateBlueprintEnvGroupsRoundTrip(t *testing.T) {
	svc := generateFixtureService()
	web := &appv1alpha1.App{}
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, web); err != nil {
		t.Fatal(err)
	}
	web.Spec.EnvFromSecrets = []string{"evg-shared-env"}
	if err := svc.Client.Update(context.Background(), web); err != nil {
		t.Fatal(err)
	}
	svc.EnvGroupExport = fakeEnvGroupExport{groups: map[string]struct {
		name string
		keys []string
	}{
		"evg-shared": {name: "shared-config", keys: []string{"API_TOKEN", "LOG_LEVEL"}},
	}}

	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		ServiceIDs:  []string{"web"},
		PostgresIDs: []string{"dpg-abc123"},
		KeyValueIDs: []string{"red-xyz789"},
		EnvGroupIDs: []string{"evg-shared"},
	})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	for _, want := range []string{
		"envVarGroups:", "name: shared-config", "fromGroup: shared-config",
		"key: API_TOKEN", "generateValue: true",
	} {
		if !strings.Contains(out.Manifest, want) {
			t.Errorf("generated manifest missing %q:\n%s", want, out.Manifest)
		}
	}
	if strings.Contains(out.Manifest, "evg-") {
		t.Errorf("manifest must not leak env-group ids:\n%s", out.Manifest)
	}
	if strings.Contains(out.Manifest, "secret-value") || strings.Contains(out.Manifest, "sk_live") {
		t.Errorf("manifest must never emit secret values:\n%s", out.Manifest)
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("generated env-group manifest must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}
}

func TestGenerateBlueprintUnselectedEnvGroupFallsBackToSyncFalse(t *testing.T) {
	svc := generateFixtureService()
	web := &appv1alpha1.App{}
	if err := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, web); err != nil {
		t.Fatal(err)
	}
	web.Spec.EnvFromSecrets = []string{"evg-shared-env"}
	if err := svc.Client.Update(context.Background(), web); err != nil {
		t.Fatal(err)
	}
	svc.EnvGroupExport = fakeEnvGroupExport{groups: map[string]struct {
		name string
		keys []string
	}{
		"evg-shared": {name: "shared-config", keys: []string{"API_TOKEN"}},
	}}

	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		ServiceIDs: []string{"web"},
	})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if strings.Contains(out.Manifest, "fromGroup") || strings.Contains(out.Manifest, "envVarGroups") {
		t.Errorf("unselected env group must not emit fromGroup/envVarGroups:\n%s", out.Manifest)
	}
	if !strings.Contains(out.Manifest, "API_TOKEN") || !strings.Contains(out.Manifest, "sync: false") {
		t.Errorf("unselected group keys must degrade to sync:false:\n%s", out.Manifest)
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("degraded env-group manifest must still validate: %+v err=%v", v, err)
	}
}

func TestGenerateBlueprintEnvGroupsOnlySelection(t *testing.T) {
	svc := generateFixtureService()
	svc.EnvGroupExport = fakeEnvGroupExport{groups: map[string]struct {
		name string
		keys []string
	}{
		"evg-shared": {name: "shared-config", keys: []string{"API_TOKEN"}},
	}}
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		EnvGroupIDs: []string{"evg-shared"},
	})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if !strings.Contains(out.Manifest, "envVarGroups:") || strings.Contains(out.Manifest, "services:") {
		t.Errorf("env-group-only selection should emit only envVarGroups:\n%s", out.Manifest)
	}
}

func TestGenerateBlueprintUnselectedTargetFallsBackToSyncFalse(t *testing.T) {
	svc := generateFixtureService()
	// Database not selected: its reference cannot parse in the generated file,
	// so the var must degrade to sync:false rather than emit a dangling ref.
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		ServiceIDs: []string{"web"},
	})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if strings.Contains(out.Manifest, "fromDatabase") || strings.Contains(out.Manifest, "fromService") {
		t.Errorf("unselected targets must not emit references:\n%s", out.Manifest)
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("degraded manifest must still validate: %+v err=%v", v, err)
	}
}

func TestGenerateBlueprintCrossSurface(t *testing.T) {
	svc := generateFixtureService()
	want, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
		ServiceIDs: []string{"web"}, PostgresIDs: []string{"dpg-abc123"}, KeyValueIDs: []string{"red-xyz789"},
	})
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/generate", strings.NewReader(
		`{"serviceIds":["web"],"postgresIds":["dpg-abc123"],"keyValueIds":["red-xyz789"]}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST generate => %d: %s", rec.Code, rec.Body)
	}
	var restOut GenerateBlueprintResult
	if err := json.Unmarshal(rec.Body.Bytes(), &restOut); err != nil || restOut.Manifest != want.Manifest {
		t.Fatalf("REST manifest differs (err %v)", err)
	}

	schema := blueprintSchema(t, svc)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(
		`{ generateBlueprint(serviceIds: ["web"], postgresIds: ["dpg-abc123"], keyValueIds: ["red-xyz789"]) { manifest filename } }`)})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	gql := res.Data.(map[string]any)["generateBlueprint"].(map[string]any)
	if gql["manifest"] != want.Manifest || gql["filename"] != "render.yaml" {
		t.Fatalf("GraphQL manifest differs")
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpOut := call("generate_blueprint", map[string]any{
		"serviceIds": []string{"web"}, "postgresIds": []string{"dpg-abc123"}, "keyValueIds": []string{"red-xyz789"},
	})
	if mcpOut["manifest"] != want.Manifest {
		t.Fatalf("MCP manifest differs: %v", mcpOut)
	}
}

// w4/193: hello-go's exact shape — a Dockerfile build whose spec.runtime is
// empty (the builder says it) — failed the generator's own self-check with
// "missing property 'runtime'" and surfaced as a bare 500.
func TestGenerateBlueprintDockerfileServiceWithoutExplicitRuntime(t *testing.T) {
	const appID = "srv-daif6dsmg29s73d1umvg"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "hello-go", Namespace: "default", Labels: map[string]string{
			core.LabelServiceName: "hello-go", core.LabelAppID: appID,
		}},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Repo: "https://github.com/bex-co/bex", Branch: "main",
			RootDir: "examples/hello-go", DockerContext: "examples/hello-go", HealthCheckPath: "/",
			Env: []appv1alpha1.EnvVar{{Name: "MESSAGE", Value: "hi"}},
		},
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default"}}
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{appID}})
	if err != nil {
		t.Fatalf("GenerateBlueprint(hello-go) = %v", err)
	}
	if !strings.Contains(out.Manifest, "runtime: docker") {
		t.Fatalf("manifest must carry the effective runtime:\n%s", out.Manifest)
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("generated manifest must self-validate: %+v err=%v", v, err)
	}
}

// TestGenerateBlueprintKeepsEverySelectedBranch (w8/m76): an omitted branch
// means the repository's default to Render and to a direct create, so the
// export must name the branch each service actually builds — a resolved
// non-main default and an explicit main alike. Dropping main (the pre-m76
// export) turned "main on a master-default repo" into "master" on re-create.
func TestGenerateBlueprintKeepsEverySelectedBranch(t *testing.T) {
	for _, branch := range []string{"main", "master", "trunk"} {
		t.Run(branch, func(t *testing.T) {
			const appID = "srv-branch-export"
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{
					core.LabelServiceName: "web", core.LabelAppID: appID,
				}},
				Spec: appv1alpha1.AppSpec{
					Type: appv1alpha1.TypeWebService, Repo: "https://github.com/expressjs/serve-static",
					Branch: branch, Runtime: "docker",
				},
			}
			svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default"}}
			out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{appID}})
			if err != nil {
				t.Fatalf("GenerateBlueprint: %v", err)
			}
			if !strings.Contains(out.Manifest, "branch: "+branch+"\n") {
				t.Fatalf("export lost the selected branch %q:\n%s", branch, out.Manifest)
			}
			if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
				t.Fatalf("generated manifest must self-validate: %+v err=%v", v, err)
			}
		})
	}
}

// A shape the generator cannot express fails with a coded error naming the
// service, never an anonymous "internal error". Buildpack builds and stored
// build commands were such shapes until w5/m117; no App a create or the CRD
// accepts is known to fail now, so this stores a runtime the CRD's enum would
// refuse (the fake client runs no CRD validation).
func TestGenerateBlueprintSelfCheckFailureNamesTheService(t *testing.T) {
	const appID = "srv-unknown-runtime"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "bp", Namespace: "default", Labels: map[string]string{
			core.LabelServiceName: "bp", core.LabelAppID: appID,
		}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Repo: "https://github.com/acme/app", Runtime: "cobol"},
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default"}}
	_, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{appID}})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_GENERATE_FAILED" || coded.Params["resourceId"] != appID {
		t.Fatalf("self-check failure = %v, want BLUEPRINT_GENERATE_FAILED naming %s", err, appID)
	}
	if !core.IsPublicError(err) || !strings.Contains(err.Error(), appID) {
		t.Fatalf("the refusal must reach the caller and name the service: %v", err)
	}
}

// A malformed generate body is the caller's error, a 400 like every sibling
// Blueprint route's, never the bare 500 a raw decode error became (w8/133).
func TestGenerateBlueprintRefusesAMalformedBodyAs400(t *testing.T) {
	svc := generateFixtureService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, body := range []string{``, `{`, `"x"`, `[]`, `{"serviceIds":"x"}`} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/generate", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("generate %q = %d %s, want 400", body, rec.Code, rec.Body.String())
		}
	}
	// null decodes, and reaches the named empty-selection refusal as before.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/generate", strings.NewReader(`null`)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "select at least one") {
		t.Errorf("generate null = %d %s, want the empty-selection 400", rec.Code, rec.Body.String())
	}
}

// TestGenerateBlueprintExportsNonDefaultPort is w4/411: a service listening
// on a non-default port exports a literal PORT, so the copy a fresh apply
// creates listens on the same port instead of falling back to 10000.
func TestGenerateBlueprintExportsNonDefaultPort(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		spec appv1alpha1.AppSpec
		want bool
	}{
		{"git service on 3000", appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Repo: "https://github.com/acme/web", Runtime: "docker", Port: 3000}, true},
		{"image service on 3000", appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "docker.io/traefik/whoami:v1.10.1", Port: 3000}, true},
		{"default port", appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Repo: "https://github.com/acme/web", Runtime: "docker"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}, Spec: tc.spec}
			svc := &Service{Base: &core.Base{Client: fakeClient(app), Namespace: "default"}}
			out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{"web"}})
			if err != nil {
				t.Fatalf("GenerateBlueprint: %v", err)
			}
			if got := strings.Contains(out.Manifest, "key: PORT"); got != tc.want {
				t.Fatalf("PORT exported = %v, want %v:\n%s", got, tc.want, out.Manifest)
			}
			if !tc.want {
				return
			}
			if !strings.Contains(out.Manifest, `value: "3000"`) {
				t.Fatalf("manifest must carry the literal port:\n%s", out.Manifest)
			}
			// Compiling the manifest for a fresh workspace yields the PORT
			// the create path derives the listening port from.
			stack := parseBlueprintStackForTest(t, out.Manifest)
			parsed, err := specFromCreate(stack.services[0].req)
			if err != nil {
				t.Fatalf("specFromCreate: %v", err)
			}
			found := false
			for _, env := range parsed.Env {
				if env.Name == "PORT" && env.Value == "3000" {
					found = true
				}
			}
			if !found {
				t.Fatalf("compiled spec env = %+v, want PORT=3000", parsed.Env)
			}
		})
	}
}

// TestGenerateBlueprintExportsPendingDomain is w4/411's second gap: a
// claimed-but-unverified domain is configured, so it exports — and the
// generated manifest still validates as noop against its own source.
func TestGenerateBlueprintExportsPendingDomain(t *testing.T) {
	ctx := context.Background()
	claims := newMemoryDomainClaimStore()
	claims.claims["pending.example.com"] = store.Domain{ID: "cdm-1", AppID: "srv-1", Host: "pending.example.com", ClaimState: "pending"}
	app := managedApp("web", "srv-1")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Repo = "https://github.com/acme/web"
	app.Spec.Runtime = "docker"
	app.Spec.Image = ""
	svc, _ := newService(claims, app)

	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{"srv-1"}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if !strings.Contains(out.Manifest, "pending.example.com") {
		t.Fatalf("manifest must list the pending domain:\n%s", out.Manifest)
	}
	v, err := svc.ValidateBlueprint(ctx, "", out.Manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("generated manifest must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}
	for _, action := range v.Plan.Actions {
		if action.Operation != BlueprintPlanNoop {
			t.Errorf("action for %s = %s, want noop (pending domains must not re-plan as update)", action.Name, action.Operation)
		}
	}
}

func TestGenerateBlueprintExportsPublicRegionWhenConfigured(t *testing.T) {
	for _, tc := range []struct{ placement, public string }{{"fsn1", "frankfurt"}, {"", ""}} {
		t.Run("placement="+tc.placement, func(t *testing.T) {
			svc := generateFixtureService()
			svc.Metadata = resourcemeta.Config{Region: tc.placement}
			static := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "static", Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/static", Runtime: "static", BuildCommand: "make build", PublishPath: "dist"},
			}
			if err := svc.Client.Create(context.Background(), static); err != nil {
				t.Fatal(err)
			}
			out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{
				ServiceIDs: []string{"web", "static"}, PostgresIDs: []string{"dpg-abc123"}, KeyValueIDs: []string{"red-xyz789"},
			})
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Services  []map[string]any `json:"services"`
				Databases []map[string]any `json:"databases"`
			}
			if err := yaml.Unmarshal([]byte(out.Manifest), &manifest); err != nil {
				t.Fatal(err)
			}
			if len(manifest.Services) != 3 || len(manifest.Databases) != 1 {
				t.Fatalf("missing selected resources: %+v", manifest)
			}
			for _, entry := range append(manifest.Services, manifest.Databases...) {
				got, present := entry["region"]
				if tc.public == "" || entry["runtime"] == "static" {
					if present {
						t.Errorf("%s exported an unavailable/static region %v", entry["name"], got)
					}
				} else if !present || got != tc.public {
					t.Errorf("%s region = %v (present=%v), want %s", entry["name"], got, present, tc.public)
				}
			}
			if svc.Metadata.PlatformRegion() != tc.placement {
				t.Fatal("export mutated internal placement")
			}
		})
	}
}
