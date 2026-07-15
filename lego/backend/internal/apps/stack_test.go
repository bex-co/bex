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
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// stack_test.go covers the multi-service bex.yml (render.yaml Blueprint) path
// added in w1/m24: parsing services:/databases:, fromDatabase secretRef
// resolution, fromService literals, all-or-nothing validation, DB-first
// ordering, and idempotent re-apply. The single-service regression coverage
// stays in deploy_test.go.

// stackManifest is a web + private-service + worker stack wired to one postgres
// via fromDatabase, with a fromService host/port link between web and api.
const stackManifest = `
services:
  - name: web
    type: web
    repo: https://github.com/bex/stack
    plan: starter
    numInstances: 2
    port: 3000
    healthCheckPath: /healthz
    domains: [web.example.com]
    envVars:
      - key: APP_ENV
        value: prod
      - key: DATABASE_URL
        fromDatabase: {name: db, property: connectionString}
      - key: DB_PASSWORD
        fromDatabase: {name: db, property: password}
      - key: API_HOST
        fromService: {name: api, property: host}
      - key: API_PORT
        fromService: {name: api, property: port}
  - name: api
    type: private
    image: {url: api:latest}
    port: 8080
  - name: worker
    type: worker
    repo: https://github.com/bex/stack
    rootDir: worker
databases:
  - name: db
    plan: basic-256mb
    diskSizeGB: 5
    postgresMajorVersion: "16"
    readReplicas:
      - name: db-ro
`

func key(name string) client.ObjectKey { return client.ObjectKey{Namespace: "default", Name: name} }

// --- parse: structure + resolution ---

func TestParseStackProjectsServicesAndDatabases(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: stackManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	if len(st.databases) != 1 || st.databases[0].name != "db" {
		t.Fatalf("databases = %+v", st.databases)
	}
	if len(st.services) != 3 {
		t.Fatalf("services = %d, want 3", len(st.services))
	}
	db := st.databases[0].spec
	if db.Plan != "basic-256mb" || db.StorageGB != 5 || db.Version != "16" {
		t.Errorf("db spec = %+v", db)
	}
	if len(db.ReadReplicas) != 1 || db.ReadReplicas[0].Name != "db-ro" {
		t.Errorf("readReplicas = %+v", db.ReadReplicas)
	}
	// render.yaml plan -> spec.tier; numInstances -> replicas.
	web := findSvc(t, st, "web").req
	if web.Plan != "starter" || web.Replicas != 2 {
		t.Errorf("web plan/replicas = %q/%d (render.yaml plan+numInstances)", web.Plan, web.Replicas)
	}
	if web.Type != appv1alpha1.TypeWebService {
		t.Errorf("web type = %q, want web_service", web.Type)
	}
	if w := findSvc(t, st, "worker").req; w.Type != appv1alpha1.TypeBackgroundWorker {
		t.Errorf("worker type = %q, want background_worker", w.Type)
	}
}

func TestParseStackLegacyAppsUnchanged(t *testing.T) {
	// A legacy single-service `apps:` file must parse into exactly one service,
	// byte-identical to the pre-m24 behavior (deploy_test's sampleManifest).
	st, err := parseStack(DeployRequest{Manifest: sampleManifest})
	if err != nil {
		t.Fatalf("parseStack legacy: %v", err)
	}
	if len(st.services) != 1 || st.services[0].req.Name != "hello" {
		t.Fatalf("legacy parse = %+v", st.services)
	}
	if len(st.databases) != 0 {
		t.Errorf("legacy file must declare no databases, got %d", len(st.databases))
	}
}

func TestParseStackResolvesFromDatabaseAsSecretRef(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: stackManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	web := findSvc(t, st, "web").req
	du := findEnv(t, web.Env, "DATABASE_URL")
	// fromDatabase resolves to a secretRef into the CNPG "<name>-app" Secret,
	// mapped by property -> key (connectionString -> uri); the credential NEVER
	// appears as a plaintext value (survives rotation, nothing sensitive in spec).
	if du.Value != "" {
		t.Errorf("DATABASE_URL must carry no plaintext value, got %q", du.Value)
	}
	if du.ValueFrom == nil || du.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("DATABASE_URL must be a secretKeyRef, got %+v", du)
	}
	if ref := du.ValueFrom.SecretKeyRef; ref.Name != "db-app" || ref.Key != "uri" {
		t.Errorf("secretRef = %+v, want {db-app, uri}", ref)
	}
	pw := findEnv(t, web.Env, "DB_PASSWORD").ValueFrom.SecretKeyRef
	if pw.Name != "db-app" || pw.Key != "password" {
		t.Errorf("password ref = %+v, want {db-app, password}", pw)
	}
}

func TestParseStackResolvesFromServiceAsLiteral(t *testing.T) {
	st, _ := parseStack(DeployRequest{Manifest: stackManifest})
	web := findSvc(t, st, "web").req
	if h := findEnv(t, web.Env, "API_HOST"); h.Value != "api" {
		t.Errorf("fromService host = %q, want api", h.Value)
	}
	if p := findEnv(t, web.Env, "API_PORT"); p.Value != "8080" {
		t.Errorf("fromService port = %q, want 8080 (api's declared port)", p.Value)
	}
}

// --- parse: all-or-nothing validation ---

func TestParseStackRejectsDuplicateNames(t *testing.T) {
	dup := `
services:
  - {name: web, image: a}
  - {name: web, image: b}
`
	_, err := parseStack(DeployRequest{Manifest: dup})
	if err == nil || !strings.Contains(err.Error(), `duplicate name "web"`) {
		t.Errorf("duplicate name => error naming it, got %v", err)
	}
}

func TestParseStackRejectsUnknownFromDatabase(t *testing.T) {
	bad := `
services:
  - name: web
    image: x
    envVars:
      - key: DATABASE_URL
        fromDatabase: {name: ghost, property: connectionString}
`
	_, err := parseStack(DeployRequest{Manifest: bad})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("unknown fromDatabase => error naming the offender, got %v", err)
	}
}

func TestParseStackRejectsUnknownFromService(t *testing.T) {
	bad := `
services:
  - name: web
    image: x
    envVars:
      - key: H
        fromService: {name: ghost, property: host}
`
	_, err := parseStack(DeployRequest{Manifest: bad})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("unknown fromService => error naming the offender, got %v", err)
	}
}

func TestParseStackRejectsBadEnvForms(t *testing.T) {
	// The five w1/m35 forms (envVarGroups/fromGroup/sync:false/generateValue/
	// fromService.envVarKey) now WORK; these are the genuinely-malformed shapes
	// that must still reject per-entry (all-or-nothing).
	cases := map[string]string{
		"fromGroup with a key":  "services:\n  - {name: web, image: x, envVars: [{key: S, fromGroup: g}]}\n",
		"value + generateValue": "services:\n  - {name: web, image: x, envVars: [{key: S, value: v, generateValue: true}]}\n",
		"dangling envVarKey":    "services:\n  - {name: web, image: x, envVars: [{key: S, fromService: {name: web, envVarKey: NOPE}}]}\n",
		"envVarKey + property":  "services:\n  - {name: web, image: x, envVars: [{key: S, fromService: {name: web, envVarKey: X, property: host}}]}\n",
		"bad db property":       "services:\n  - {name: web, image: x, envVars: [{key: S, fromDatabase: {name: db, property: url}}]}\ndatabases:\n  - {name: db}\n",
		"group var referencing": "envVarGroups:\n  - name: g\n    envVars: [{key: S, fromService: {name: web, property: host}}]\nservices:\n  - {name: web, image: x}\n",
		"group var sync false":  "envVarGroups:\n  - name: g\n    envVars: [{key: S, value: v, sync: false}]\nservices:\n  - {name: web, image: x}\n",
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseStack(DeployRequest{Manifest: manifest}); err == nil {
				t.Errorf("%s: want a validation error, got nil", name)
			}
		})
	}
}

func TestParseStackRejectsBothAppsAndServices(t *testing.T) {
	both := `
services: [{name: a, image: x}]
apps: [{name: b, image: y}]
`
	if _, err := parseStack(DeployRequest{Manifest: both}); err == nil {
		t.Error("apps: + services: together must be rejected")
	}
}

func TestParseStackRejectsFromServiceToPortlessService(t *testing.T) {
	// fromService must target a web/private service (which exposes a k8s Service /
	// DNS name); a worker has no Service, so referencing its host is invalid.
	bad := `
services:
  - name: web
    image: x
    envVars:
      - {key: H, fromService: {name: w, property: host}}
  - {name: w, type: worker, image: "w:1"}
`
	_, err := parseStack(DeployRequest{Manifest: bad})
	if err == nil || !strings.Contains(err.Error(), "no network address") {
		t.Errorf("fromService to a worker => error, got %v", err)
	}
}

// --- apply: ordering, all-or-nothing, idempotency ---

func TestDeployStackAppliesDatabasesFirstThenServices(t *testing.T) {
	svc, cl := newService(nil)
	res, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: stackManifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(res.Databases) != 1 || res.Databases[0].Name != "db" {
		t.Fatalf("databases = %+v", res.Databases)
	}
	if len(res.Services) != 3 {
		t.Fatalf("services = %d, want 3", len(res.Services))
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), key("db"), &db); err != nil {
		t.Fatalf("database db not created: %v", err)
	}
	if db.Spec.Plan != "basic-256mb" {
		t.Errorf("db plan = %q", db.Spec.Plan)
	}
	for _, name := range []string{"web", "api", "worker"} {
		if getApp(t, cl, name).Name != name {
			t.Errorf("app %s not created", name)
		}
	}
	// The web service's fromDatabase env landed as a secretRef in the App spec.
	du := findEnv(t, getApp(t, cl, "web").Spec.Env, "DATABASE_URL")
	if du.ValueFrom == nil || du.ValueFrom.SecretKeyRef == nil || du.ValueFrom.SecretKeyRef.Name != "db-app" {
		t.Errorf("web DATABASE_URL not a db-app secretRef: %+v", du)
	}
}

func TestDeployStackAllOrNothingCreatesNothing(t *testing.T) {
	// An invalid third service (unknown fromDatabase) must reject the whole apply
	// before any resource is written — nothing partially created.
	bad := `
services:
  - {name: a, image: x}
  - {name: b, image: y}
  - name: c
    image: z
    envVars:
      - {key: U, fromDatabase: {name: nope, property: connectionString}}
databases:
  - {name: db}
`
	svc, cl := newService(nil)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: bad}); err == nil {
		t.Fatal("invalid third entry => error, got nil")
	}
	var apps appv1alpha1.AppList
	_ = cl.List(context.Background(), &apps)
	if len(apps.Items) != 0 {
		t.Errorf("all-or-nothing: %d apps created, want zero", len(apps.Items))
	}
	var dbs appv1alpha1.DatabaseList
	_ = cl.List(context.Background(), &dbs)
	if len(dbs.Items) != 0 {
		t.Errorf("all-or-nothing: %d databases created, want zero", len(dbs.Items))
	}
}

func TestDeployStackIdempotentReapplyIsNoOp(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec)
	ctx := context.Background()

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: stackManifest}); err != nil {
		t.Fatalf("first DeployStack: %v", err)
	}
	firstRA := getApp(t, cl, "web").Spec.RestartedAt
	firstDB := getDB(t, cl, "db").ResourceVersion

	// Re-applying the SAME file converges with zero spec change, zero new deploy
	// records, and no RestartedAt bump (the DoD's idempotency contract).
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: stackManifest}); err != nil {
		t.Fatalf("second DeployStack: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.RestartedAt; got != firstRA {
		t.Errorf("RestartedAt changed on no-op re-apply: %q -> %q", firstRA, got)
	}
	if len(rec.deployCalls) != 0 {
		t.Errorf("idempotent re-apply opened %d deploy records, want 0", len(rec.deployCalls))
	}
	if getDB(t, cl, "db").ResourceVersion != firstDB {
		t.Error("Database was patched on a no-op re-apply")
	}
}

func TestDeployStackChangedServiceRedeploys(t *testing.T) {
	rec := &recordingStore{}
	// A store-managed App (carries app-id): a changed re-apply opens a deploy
	// record and bumps RestartedAt; the idempotent path only fires on a real diff.
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)

	changed := "services:\n  - {name: web, image: new:1}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: changed}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	a := getApp(t, cl, "web")
	if a.Spec.Image != "new:1" {
		t.Errorf("image not updated: %q", a.Spec.Image)
	}
	if a.Spec.RestartedAt == "" {
		t.Error("a changed re-apply must bump RestartedAt")
	}
	if len(rec.deployCalls) != 1 || rec.deployCalls[0].Trigger != "blueprint" {
		t.Errorf("changed re-apply => 1 blueprint deploy record, got %+v", rec.deployCalls)
	}
}

func TestDeployStackPreservesNonOwnedFieldsOnReapply(t *testing.T) {
	// applyCreate re-applies only the create-owned fields. A non-owned field like
	// EnvFromSecret (owned by the secrets feature) must survive a stack re-apply.
	existing := sampleApp("web")
	existing.Spec.EnvFromSecret = "web-env"
	svc, cl := newService(nil, existing)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: "services:\n  - {name: web, image: x}"}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.EnvFromSecret; got != "web-env" {
		t.Errorf("EnvFromSecret = %q, want web-env (must survive re-apply)", got)
	}
}

func TestDeployRejectsStackViaSingleServiceDeploy(t *testing.T) {
	// Deploy is the single-service convenience; a multi-resource manifest must
	// point at the stack form rather than silently returning the first service.
	svc, _ := newService(nil)
	if _, err := svc.Deploy(context.Background(), DeployRequest{Manifest: stackManifest}); err == nil {
		t.Error("Deploy on a stack manifest must error (use DeployStack)")
	}
}

// --- helpers ---

func findSvc(t *testing.T, st parsedStack, name string) parsedService {
	t.Helper()
	for _, s := range st.services {
		if s.req.Name == name {
			return s
		}
	}
	t.Fatalf("service %q not in stack", name)
	return parsedService{}
}

func findEnv(t *testing.T, env []appv1alpha1.EnvVar, name string) appv1alpha1.EnvVar {
	t.Helper()
	for _, e := range env {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("env %q not found", name)
	return appv1alpha1.EnvVar{}
}

func getDB(t *testing.T, cl client.Client, name string) *appv1alpha1.Database {
	t.Helper()
	var d appv1alpha1.Database
	if err := cl.Get(context.Background(), key(name), &d); err != nil {
		t.Fatalf("get db %s: %v", name, err)
	}
	return &d
}
