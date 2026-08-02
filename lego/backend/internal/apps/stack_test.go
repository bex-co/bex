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
	"errors"
	"strconv"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
    runtime: docker
    repo: https://github.com/bex/stack
    plan: starter
    numInstances: 2
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
    type: pserv
    runtime: image
    image: {url: api:latest}
  - name: worker
    type: worker
    runtime: docker
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
	if db.Name != "db" || db.Plan != "basic-256mb" || db.StorageGB != 5 || db.Version != "16" {
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

func TestParseStackDatabasePhysicalIdentifiers(t *testing.T) {
	const manifest = `
databases:
  - name: orders
    databaseName: orders_data
    user: orders_owner
`
	st, err := parseStack(DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.databases) != 1 || st.databases[0].spec.DatabaseName != "orders_data" || st.databases[0].spec.DatabaseUser != "orders_owner" {
		t.Fatalf("physical identifier spec = %+v", st.databases)
	}

	bad := strings.Replace(manifest, "orders_data", "Orders-Data", 1)
	if _, err := parseStack(DeployRequest{Manifest: bad}); err == nil || !strings.Contains(err.Error(), "databaseName") {
		t.Fatalf("invalid databaseName error = %v", err)
	}
}

func TestParseStackScalingBlockPopulatesAutoscaling(t *testing.T) {
	// render.yaml scaling: block (w2/m49) must flow through parseStack →
	// CreateRequest.Autoscaling and then onto spec.autoscaling via specFromCreate.
	const manifest = `
services:
  - name: web
    type: web
    image: {url: nginx:1}
    scaling:
      minInstances: 2
      maxInstances: 10
      targetCPUPercent: 70
`
	st, err := parseStack(DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	req := findSvc(t, st, "web").req
	if req.Autoscaling == nil {
		t.Fatal("Autoscaling is nil, want non-nil")
	}
	if req.Autoscaling.MinInstances != 2 {
		t.Errorf("MinInstances = %d, want 2", req.Autoscaling.MinInstances)
	}
	if req.Autoscaling.MaxInstances != 10 {
		t.Errorf("MaxInstances = %d, want 10", req.Autoscaling.MaxInstances)
	}
	if req.Autoscaling.TargetCPUPercent == nil || *req.Autoscaling.TargetCPUPercent != 70 {
		t.Errorf("TargetCPUPercent = %v, want 70", req.Autoscaling.TargetCPUPercent)
	}
	// specFromCreate must materialize spec.autoscaling with Enabled:true.
	spec, err := specFromCreate(req)
	if err != nil {
		t.Fatalf("specFromCreate: %v", err)
	}
	if spec.Autoscaling == nil || !spec.Autoscaling.Enabled {
		t.Fatal("spec.Autoscaling not enabled after create")
	}
	if spec.Autoscaling.MinReplicas != 2 || spec.Autoscaling.MaxReplicas != 10 {
		t.Errorf("spec.Autoscaling bounds = %d/%d, want 2/10", spec.Autoscaling.MinReplicas, spec.Autoscaling.MaxReplicas)
	}
}

func TestParseStackScalingBlockValidation(t *testing.T) {
	// An invalid scaling block (no target set) is rejected by specFromCreate,
	// not silently accepted.
	const manifest = `
services:
  - name: web
    type: web
    image: {url: nginx:1}
    scaling:
      minInstances: 1
      maxInstances: 5
`
	st, err := parseStack(DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	req := findSvc(t, st, "web").req
	if req.Autoscaling == nil {
		t.Fatal("Autoscaling is nil")
	}
	if _, err := specFromCreate(req); err == nil {
		t.Error("specFromCreate with no target should fail, got nil")
	}
}

func TestParseStackCanonicalServices(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: sampleManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	if len(st.services) != 1 || st.services[0].req.Name != "hello" {
		t.Fatalf("canonical parse = %+v", st.services)
	}
	if len(st.databases) != 0 {
		t.Errorf("file must declare no databases, got %d", len(st.databases))
	}
}

func TestParseStackDefersFromDatabaseUntilStableIDIsKnown(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: stackManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	web := findSvc(t, st, "web")
	if len(web.databaseRefs) != 2 {
		t.Fatalf("database refs = %+v, want 2 deferred refs", web.databaseRefs)
	}
	if got := web.databaseRefs[0].FromDatabase.Name; got != "db" {
		t.Errorf("deferred database name = %q, want db", got)
	}
	for _, name := range []string{"DATABASE_URL", "DB_PASSWORD"} {
		for _, env := range web.req.Env {
			if env.Name == name {
				t.Errorf("%s was resolved before the immutable database id was known", name)
			}
		}
	}
}

func TestParseStackDefersFromServiceHostUntilSlugIsKnown(t *testing.T) {
	// host defers to apply — the injected hostname is the sibling's slug,
	// minted at create (ADR041 D3; the same deferral shape as fromDatabase
	// above). port still resolves at parse from the platform default.
	st, _ := parseStack(DeployRequest{Manifest: stackManifest})
	web := findSvc(t, st, "web")
	if len(web.hostRefs) != 1 {
		t.Fatalf("hostRefs = %+v, want the one deferred api host ref", web.hostRefs)
	}
	// The port half resolves at parse; only the slug hostname waits for apply.
	if ref := web.hostRefs[0]; ref.target != "api" || ref.key != "API_HOST" || ref.port != 3000 || ref.hostport {
		t.Errorf("hostRef = %+v, want {key API_HOST, target api, port 3000, host-only}", ref)
	}
	for _, env := range web.req.Env {
		if env.Name == "API_HOST" {
			t.Errorf("API_HOST was resolved before the sibling's slug was known")
		}
	}
	if p := findEnv(t, web.req.Env, "API_PORT"); p.Value != "3000" {
		t.Errorf("fromService port = %q, want the platform default 3000", p.Value)
	}
}

// --- parse: all-or-nothing validation ---

func TestParseStackRejectsDuplicateNames(t *testing.T) {
	dup := `
services:
  - {name: web, image: {url: a}}
  - {name: web, image: {url: b}}
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
    image: {url: x}
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
    image: {url: x}
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
		"fromGroup with a key":  "services:\n  - {name: web, image: {url: x}, envVars: [{key: S, fromGroup: g}]}\n",
		"value + generateValue": "services:\n  - {name: web, image: {url: x}, envVars: [{key: S, value: v, generateValue: true}]}\n",
		"dangling envVarKey":    "services:\n  - {name: web, image: {url: x}, envVars: [{key: S, fromService: {name: web, envVarKey: NOPE}}]}\n",
		"envVarKey + property":  "services:\n  - {name: web, image: {url: x}, envVars: [{key: S, fromService: {name: web, envVarKey: X, property: host}}]}\n",
		"bad db property":       "services:\n  - {name: web, image: {url: x}, envVars: [{key: S, fromDatabase: {name: db, property: url}}]}\ndatabases:\n  - {name: db}\n",
		"group var referencing": "envVarGroups:\n  - name: g\n    envVars: [{key: S, fromService: {name: web, property: host}}]\nservices:\n  - {name: web, image: {url: x}}\n",
		"group var sync false":  "envVarGroups:\n  - name: g\n    envVars: [{key: S, value: v, sync: false}]\nservices:\n  - {name: web, image: {url: x}}\n",
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseStack(DeployRequest{Manifest: manifest}); err == nil {
				t.Errorf("%s: want a validation error, got nil", name)
			}
		})
	}
}

func TestParseStackRejectsRetiredBlueprintDialect(t *testing.T) {
	cases := map[string]string{
		"apps":        "apps: [{name: web, image: {url: x}}]",
		"tier":        "services: [{name: web, tier: free}]",
		"replicas":    "services: [{name: web, replicas: 2}]",
		"port":        "services: [{name: web, port: 8080}]",
		"imagePath":   "services: [{name: web, imagePath: nginx:1}]",
		"publishPath": "services: [{name: web, publishPath: dist}]",
		"bare image":  "services: [{name: web, image: nginx:1}]",
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseStack(DeployRequest{Manifest: manifest})
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), name) {
				t.Fatalf("retired %s => actionable ErrBadRequest, got %v", name, err)
			}
		})
	}
}

func TestParseStackRejectsFromServiceToPortlessService(t *testing.T) {
	// fromService must target a web/private service (which exposes a k8s Service /
	// DNS name); a worker has no Service, so referencing its host is invalid.
	bad := `
services:
  - name: web
    image: {url: x}
    envVars:
      - {key: H, fromService: {name: w, property: host}}
  - {name: w, type: worker, image: {url: "w:1"}}
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
	if !strings.HasPrefix(res.Databases[0].ID, "dpg-") {
		t.Fatalf("database id = %q, want dpg-...", res.Databases[0].ID)
	}
	if len(res.Services) != 3 {
		t.Fatalf("services = %d, want 3", len(res.Services))
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), key(res.Databases[0].ID), &db); err != nil {
		t.Fatalf("database %s not created: %v", res.Databases[0].ID, err)
	}
	if db.Spec.Name != "db" || db.Spec.Plan != "basic-256mb" {
		t.Errorf("db spec = %+v", db.Spec)
	}
	for _, name := range []string{"web", "api", "worker"} {
		if getApp(t, cl, name).Name != name {
			t.Errorf("app %s not created", name)
		}
	}
	// The web service's fromDatabase env landed as a secretRef in the App spec.
	du := findEnv(t, getApp(t, cl, "web").Spec.Env, "DATABASE_URL")
	wantSecret := res.Databases[0].ID + "-app"
	if du.ValueFrom == nil || du.ValueFrom.SecretKeyRef == nil || du.ValueFrom.SecretKeyRef.Name != wantSecret {
		t.Errorf("web DATABASE_URL not a %s secretRef: %+v", wantSecret, du)
	}
	// Legacy (no tenant store): the CR/Service name IS the bare name, so the
	// fromService host literal stays byte-identical to the pre-m57 behavior.
	if h := findEnv(t, getApp(t, cl, "web").Spec.Env, "API_HOST"); h.Value != "api" {
		t.Errorf("legacy fromService host = %q, want the bare name api", h.Value)
	}
}

func TestDeployStackFromServiceHostResolvesToTheSlug(t *testing.T) {
	// Store-managed path (w4/m19 CR names `<tenant>-<name>`): the injected
	// hostname must be the sibling's resolvable slug — never the bare bex.yml
	// name, which matches no Service (ADR041 gap 1 / D3). `web` references
	// `api` declared AFTER it, so this also exercises the forward-reference
	// second pass (api's slug exists only after its create).
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	res, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: stackManifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	var apiSlug string
	for _, v := range res.Services {
		if v.Name == "api" {
			apiSlug = v.Slug
		}
	}
	if apiSlug == "" || apiSlug == "api" {
		t.Fatalf("api slug = %q, want a store-managed slug distinct from the bare name", apiSlug)
	}
	web := getTenantApp(t, cl, "tea-a", "web")
	if h := findEnv(t, web.Spec.Env, "API_HOST"); h.Value != apiSlug {
		t.Errorf("fromService host = %q, want the sibling's slug %q", h.Value, apiSlug)
	}
	if p := findEnv(t, web.Spec.Env, "API_PORT"); p.Value != "3000" {
		t.Errorf("fromService port = %q, want the platform default 3000", p.Value)
	}

	// Idempotence: a re-apply resolves every ref up front (all siblings exist)
	// and converges to the same env — no churn, no duplicate vars.
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: stackManifest}); err != nil {
		t.Fatalf("second DeployStack: %v", err)
	}
	web = getTenantApp(t, cl, "tea-a", "web")
	seen := 0
	for _, env := range web.Spec.Env {
		if env.Name == "API_HOST" {
			seen++
			if env.Value != apiSlug {
				t.Errorf("re-apply fromService host = %q, want %q", env.Value, apiSlug)
			}
		}
	}
	if seen != 1 {
		t.Errorf("API_HOST appears %d times after re-apply, want exactly 1", seen)
	}
}

func TestDeployStackAllOrNothingCreatesNothing(t *testing.T) {
	// An invalid third service (unknown fromDatabase) must reject the whole apply
	// before any resource is written — nothing partially created.
	bad := `
services:
  - {name: a, image: {url: x}}
  - {name: b, image: {url: y}}
  - name: c
    image: {url: z}
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
	first := listDBs(t, cl)
	if len(first) != 1 {
		t.Fatalf("database count = %d, want 1", len(first))
	}
	firstDBID := first[0].Name
	firstDB := first[0].ResourceVersion

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
	if getDB(t, cl, firstDBID).ResourceVersion != firstDB {
		t.Error("Database was patched on a no-op re-apply")
	}
}

func TestDeployStackCustomPhysicalIdentifiersAreCreateOnlyAndIdempotent(t *testing.T) {
	const manifest = `
databases:
  - name: orders
    databaseName: orders_data
    user: orders_owner
`
	svc, cl := newService(nil)
	ctx := context.Background()
	first, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("first DeployStack: %v", err)
	}
	db := getDB(t, cl, first.Databases[0].ID)
	if db.Spec.DatabaseName != "orders_data" || db.Spec.DatabaseUser != "orders_owner" {
		t.Fatalf("physical identifiers = %q/%q", db.Spec.DatabaseName, db.Spec.DatabaseUser)
	}
	resourceVersion := db.ResourceVersion

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("idempotent DeployStack: %v", err)
	}
	if got := getDB(t, cl, db.Name).ResourceVersion; got != resourceVersion {
		t.Fatalf("idempotent reapply patched Database: resourceVersion %q -> %q", resourceVersion, got)
	}

	changed := strings.Replace(manifest, "orders_data", "other_data", 1)
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err == nil || !strings.Contains(err.Error(), "databaseName is immutable") {
		t.Fatalf("changed physical identifier error = %v", err)
	}
	unchanged := getDB(t, cl, db.Name)
	if unchanged.Spec.DatabaseName != "orders_data" || unchanged.Spec.DatabaseUser != "orders_owner" {
		t.Fatalf("rejected reapply mutated identifiers = %q/%q", unchanged.Spec.DatabaseName, unchanged.Spec.DatabaseUser)
	}
}

func TestDeployStackReapplyAfterRenameKeepsDatabaseIdentity(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	first, err := svc.DeployStack(ctx, DeployRequest{Manifest: stackManifest})
	if err != nil {
		t.Fatalf("first DeployStack: %v", err)
	}
	databaseID := first.Databases[0].ID
	db := getDB(t, cl, databaseID)
	db.Spec.Name = "renamed-db"
	if err := cl.Update(ctx, db); err != nil {
		t.Fatalf("rename database: %v", err)
	}

	renamedManifest := strings.ReplaceAll(stackManifest, "fromDatabase: {name: db,", "fromDatabase: {name: renamed-db,")
	renamedManifest = strings.Replace(renamedManifest, "databases:\n  - name: db\n", "databases:\n  - name: renamed-db\n", 1)
	second, err := svc.DeployStack(ctx, DeployRequest{Manifest: renamedManifest})
	if err != nil {
		t.Fatalf("reapply renamed stack: %v", err)
	}
	if len(second.Databases) != 1 || second.Databases[0].ID != databaseID || second.Databases[0].Name != "renamed-db" {
		t.Fatalf("renamed database = %+v, want id %s", second.Databases, databaseID)
	}
	if got := len(listDBs(t, cl)); got != 1 {
		t.Fatalf("database count after rename reapply = %d, want 1", got)
	}
	du := findEnv(t, getApp(t, cl, "web").Spec.Env, "DATABASE_URL")
	if got := du.ValueFrom.SecretKeyRef.Name; got != databaseID+"-app" {
		t.Errorf("database secret after rename = %q, want %q", got, databaseID+"-app")
	}
}

func TestDeployStackChangedServiceRedeploys(t *testing.T) {
	rec := &recordingStore{}
	// A store-managed App (carries app-id): a changed re-apply opens a deploy
	// record and bumps RestartedAt; the idempotent path only fires on a real diff.
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)

	changed := "services:\n  - {name: web, image: {url: new:1}}\n"
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
	if got := rec.deployCalls[0].Generation; got != existing.Generation+1 {
		t.Errorf("blueprint deploy generation = %d, want %d", got, existing.Generation+1)
	}
	if got := a.Annotations[appv1alpha1.AnnotationReleaseGeneration]; got != strconv.FormatInt(existing.Generation+1, 10) {
		t.Errorf("release-generation annotation = %q, want %d", got, existing.Generation+1)
	}
}

func TestDeployStackPreservesNonOwnedFieldsOnReapply(t *testing.T) {
	// applyCreate re-applies only the create-owned fields. A non-owned field like
	// EnvFromSecret (owned by the secrets feature) must survive a stack re-apply.
	existing := sampleApp("web")
	existing.Spec.EnvFromSecret = "web-env"
	svc, cl := newService(nil, existing)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: "services:\n  - {name: web, image: {url: x}}"}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.EnvFromSecret; got != "web-env" {
		t.Errorf("EnvFromSecret = %q, want web-env (must survive re-apply)", got)
	}
}

// --- keyvalue Blueprint tests (w2/m41) ---

// kvManifest is a minimal stack with one keyvalue store and a web service that
// references it via fromService (connectionString + host + port + password).
const kvManifest = `
services:
  - name: cache
    type: redis
    plan: free
  - name: web
    image: {url: nginx}
    envVars:
      - key: REDIS_URL
        fromService: {name: cache, type: redis, property: connectionString}
      - key: REDIS_HOST
        fromService: {name: cache, type: redis, property: host}
      - key: REDIS_PORT
        fromService: {name: cache, type: redis, property: port}
      - key: REDIS_PASS
        fromService: {name: cache, type: redis, property: password}
`

func TestDeployStackKeyValueProvisionedAndValidationPlan(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: kvManifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(res.KeyValues) != 1 || res.KeyValues[0].Name != "cache" {
		t.Fatalf("keyValues = %+v, want [{Name:cache}]", res.KeyValues)
	}
	kvID := res.KeyValues[0].ID
	if !strings.HasPrefix(kvID, "red-") {
		t.Errorf("keyvalue id = %q, want red-...", kvID)
	}
	// The keyvalue CR must exist.
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, key(kvID), &kv); err != nil {
		t.Fatalf("keyvalue %s not created: %v", kvID, err)
	}
	// The web service must have 4 SecretKeyRef env vars pointing at the KV secret.
	app := getApp(t, cl, "web")
	for _, tc := range []struct {
		key  string
		want string
	}{
		{"REDIS_URL", "uri"},
		{"REDIS_HOST", "host"},
		{"REDIS_PORT", "port"},
		{"REDIS_PASS", "password"},
	} {
		e := findEnv(t, app.Spec.Env, tc.key)
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			t.Errorf("%s: want SecretKeyRef, got %+v", tc.key, e)
			continue
		}
		if e.ValueFrom.SecretKeyRef.Name != kvID {
			t.Errorf("%s: secret name = %q, want %q", tc.key, e.ValueFrom.SecretKeyRef.Name, kvID)
		}
		if e.ValueFrom.SecretKeyRef.Key != tc.want {
			t.Errorf("%s: secret key = %q, want %q", tc.key, e.ValueFrom.SecretKeyRef.Key, tc.want)
		}
	}
	// Validation plan must include the keyvalue name.
	st, err := parseStack(DeployRequest{Manifest: kvManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	plan := blueprintValidationPlan("", st)
	if len(plan.KeyValue) != 1 || plan.KeyValue[0] != "cache" {
		t.Errorf("plan.KeyValue = %v, want [cache]", plan.KeyValue)
	}
	if plan.TotalActions != 2 { // 1 service + 1 keyvalue
		t.Errorf("plan.TotalActions = %d, want 2", plan.TotalActions)
	}
}

func TestDeployStackFromServiceKeyValueUnknownTarget(t *testing.T) {
	manifest := `
services:
  - name: web
    image: {url: nginx}
    envVars:
      - key: REDIS_URL
        fromService: {name: missing, type: redis, property: connectionString}
`
	svc, _ := newService(nil)
	_, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("unknown keyvalue fromService => error naming the target, got %v", err)
	}
}

func TestDeployStackFromServiceKeyValueInvalidProperty(t *testing.T) {
	bad := []string{
		// no property
		"services:\n  - name: cache\n    type: redis\n    plan: free\n  - name: web\n    image: {url: x}\n    envVars:\n      - {key: K, fromService: {name: cache, type: redis}}\n",
		// unsupported property
		"services:\n  - name: cache\n    type: redis\n    plan: free\n  - name: web\n    image: {url: x}\n    envVars:\n      - {key: K, fromService: {name: cache, type: redis, property: user}}\n",
	}
	for _, m := range bad {
		svc, _ := newService(nil)
		if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil {
			t.Errorf("invalid keyvalue fromService => error, got nil for: %s", m)
		}
	}
}

// --- initialDeployHook: parse + ran-once semantics (w2/m45) ---

const hookManifest = `
services:
  - name: web
    image: {url: nginx:1.26}
    initialDeployHook: npm run db:setup
    preDeployCommand: npm run db:migrate
`

// TestParseStackInitialDeployHookParsed verifies the field is read from bex.yml.
func TestParseStackInitialDeployHookParsed(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: hookManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	req := findSvc(t, st, "web").req
	if req.InitialDeployHook != "npm run db:setup" {
		t.Errorf("InitialDeployHook = %q, want npm run db:setup", req.InitialDeployHook)
	}
	if req.PreDeployCommand != "npm run db:migrate" {
		t.Errorf("PreDeployCommand = %q, want npm run db:migrate", req.PreDeployCommand)
	}
}

// TestDeployStackInitialHookSetsPreDeployCommandOnFirstCreate verifies that on
// first create the hook overrides spec.preDeployCommand and the annotation is set.
func TestDeployStackInitialHookSetsPreDeployCommandOnFirstCreate(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: hookManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.PreDeployCommand != "npm run db:setup" {
		t.Errorf("preDeployCommand = %q, want npm run db:setup (hook on first deploy)", app.Spec.PreDeployCommand)
	}
	if app.Annotations[initialDeployHookAnnotation] != "npm run db:setup" {
		t.Errorf("annotation %s = %q, want npm run db:setup", initialDeployHookAnnotation, app.Annotations[initialDeployHookAnnotation])
	}
	if app.Annotations[initialDeployHookRanAnnotation] != "" {
		t.Errorf("ran-once annotation must be absent before hook runs; got %q", app.Annotations[initialDeployHookRanAnnotation])
	}
}

// TestDeployStackInitialHookMarkedRanWhenPreDeploySucceeds verifies that a
// re-sync after the first pre-deploy Job succeeds writes the ran-once annotation
// and restores the regular preDeployCommand.
func TestDeployStackInitialHookMarkedRanWhenPreDeploySucceeds(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: hookManifest}); err != nil {
		t.Fatalf("first DeployStack: %v", err)
	}
	// Simulate the operator: first pre-deploy Job succeeded.
	app := getApp(t, cl, "web")
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Status: appv1alpha1.PreDeploySucceeded}
	if err := cl.Update(ctx, app); err != nil {
		t.Fatalf("update status: %v", err)
	}
	// Re-sync: hook should not rerun; ran-once annotation should appear.
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: hookManifest}); err != nil {
		t.Fatalf("second DeployStack: %v", err)
	}
	app = getApp(t, cl, "web")
	if app.Annotations[initialDeployHookRanAnnotation] != "true" {
		t.Errorf("ran-once annotation = %q, want true", app.Annotations[initialDeployHookRanAnnotation])
	}
	if app.Spec.PreDeployCommand != "npm run db:migrate" {
		t.Errorf("preDeployCommand after hook ran = %q, want npm run db:migrate (regular command)", app.Spec.PreDeployCommand)
	}
}

// TestDeployStackInitialHookSkippedWhenAlreadyRan verifies that once the
// ran-once annotation is set, subsequent syncs never re-apply the hook.
func TestDeployStackInitialHookSkippedWhenAlreadyRan(t *testing.T) {
	svc, cl := newService(nil)
	ctx := context.Background()
	// Create with ran-once annotation already set (simulates a fully-converged state).
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default",
			Annotations: map[string]string{
				initialDeployHookAnnotation:    "npm run db:setup",
				initialDeployHookRanAnnotation: "true",
			},
		},
		Spec: appv1alpha1.AppSpec{
			Image:            "nginx:1.26",
			PreDeployCommand: "npm run db:migrate",
		},
	}
	if err := cl.Create(ctx, app); err != nil {
		t.Fatalf("create app: %v", err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: hookManifest}); err != nil {
		t.Fatalf("DeployStack with ran-once set: %v", err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.PreDeployCommand != "npm run db:migrate" {
		t.Errorf("preDeployCommand = %q, want npm run db:migrate (hook must not rerun)", got.Spec.PreDeployCommand)
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

func listDBs(t *testing.T, cl client.Client) []appv1alpha1.Database {
	t.Helper()
	var dbs appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &dbs); err != nil {
		t.Fatalf("list databases: %v", err)
	}
	return dbs.Items
}
