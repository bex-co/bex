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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeEnvNames struct{ names map[string][]string }

func (f fakeEnvNames) ListEnvVarNames(_ context.Context, service string) ([]string, error) {
	return f.names[service], nil
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
			Name: "app-db", Plan: "basic-1gb", StorageGB: 10, Version: "16",
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
	v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest)
	if err != nil || !v.Valid {
		t.Fatalf("generated manifest must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}

	// (b) The reference forms and sync:false classification.
	for _, want := range []string{
		"fromDatabase", "app-db", "connectionString",
		"fromService", "app-cache", "keyvalue",
		"sync: false", "FOREIGN_SECRET", "SMTP_PASSWORD",
		"plan: pro plus", "dockerContext: apps/web", "dockerCommand: bin/server",
		"diskSizeGB: 10", "postgresMajorVersion: \"16\"", "highAvailability",
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
// past ValidAppName's 30-char cap — so exporting a.Name produced a render.yaml
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
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest); err != nil || !v.Valid {
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
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest); err != nil || !v.Valid {
		t.Fatalf("must self-validate: %+v err=%v\n%s", v, err, out.Manifest)
	}
	for _, want := range []string{
		"www.example.com", "example.com", // primary + additional domains
		"startCommand: bin/report", // cron command from Spec.Command
		"numInstances: 4",          // worker autoscaling → fixed upper bound
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
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest); err != nil || !v.Valid {
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
