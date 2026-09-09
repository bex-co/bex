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

// blueprint_test.go covers the /blueprints surface (w2/m15):
// validate (stateless) · list · sync — each verb over all three adapters.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- fake store ---

type fakeBlueprintStore struct {
	blueprints map[string]store.Blueprint // key: id
	// gotSyncLimit records the limit ListBlueprintSyncs was called with, so the
	// service's clamp is asserted where it is applied rather than re-derived.
	gotSyncLimit int
	// lastSyncUpdate records the most recent UpdateBlueprintSync call, so tests
	// can assert what error (if any) a sync run persisted.
	lastSyncUpdate store.BlueprintSync
	// syncsToReturn, when set, is what ListBlueprintSyncs returns — the wire
	// round-trip tests need a canned row rather than a live-recorded one.
	syncsToReturn []store.BlueprintSync
	// insertedSyncs records every InsertBlueprintSync call in order, so tests
	// can assert a run was (or was not) recorded and with what provenance.
	insertedSyncs []store.BlueprintSync
	// insertSyncErr / updateSyncErr, when set, fail the matching sync write —
	// the store-outage cases (w8/m36 t004).
	insertSyncErr error
	updateSyncErr error
}

func newFakeBlueprintStore(bs ...store.Blueprint) *fakeBlueprintStore {
	f := &fakeBlueprintStore{blueprints: make(map[string]store.Blueprint)}
	for _, b := range bs {
		f.blueprints[b.ID] = b
	}
	return f
}

func (f *fakeBlueprintStore) UpsertBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	for id, existing := range f.blueprints {
		if existing.TenantID == b.TenantID && existing.Repo == b.Repo && existing.Branch == b.Branch {
			existing.Name = b.Name
			existing.Manifest = b.Manifest
			existing.Status = "active"
			f.blueprints[id] = existing
			return existing, nil
		}
	}
	if b.ID == "" {
		b.ID = fmt.Sprintf("blp-fake-%d", len(f.blueprints))
	}
	f.blueprints[b.ID] = b
	return b, nil
}

func (f *fakeBlueprintStore) GetBlueprint(_ context.Context, id, tenantID string) (store.Blueprint, error) {
	b, ok := f.blueprints[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	return b, nil
}

func (f *fakeBlueprintStore) GetBlueprintByRepo(_ context.Context, tenantID, repo, branch string) (store.Blueprint, error) {
	for _, b := range f.blueprints {
		if b.TenantID == tenantID && b.Repo == repo && b.Branch == branch {
			return b, nil
		}
	}
	return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
}

func (f *fakeBlueprintStore) ListBlueprints(_ context.Context, tenantID string) ([]store.Blueprint, error) {
	var out []store.Blueprint
	for _, b := range f.blueprints {
		if b.TenantID == tenantID && b.Status != "disconnected" {
			out = append(out, b)
		}
	}
	return out, nil
}

func (f *fakeBlueprintStore) UpdateBlueprint(_ context.Context, id, tenantID string, name *string, autoSync *bool, bpPath *string, status *string, lastSyncAt *time.Time) (store.Blueprint, error) {
	b, ok := f.blueprints[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	if name != nil {
		b.Name = *name
	}
	if autoSync != nil {
		b.AutoSync = *autoSync
	}
	if bpPath != nil {
		b.Path = *bpPath
	}
	if status != nil {
		b.Status = *status
	}
	if lastSyncAt != nil {
		b.LastSyncAt = lastSyncAt
	}
	f.blueprints[id] = b
	return b, nil
}

func (f *fakeBlueprintStore) DisconnectBlueprint(_ context.Context, id, tenantID string) error {
	b, ok := f.blueprints[id]
	if !ok || b.TenantID != tenantID {
		return fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	b.Status = "disconnected"
	b.AutoSync = false
	f.blueprints[id] = b
	return nil
}

func (f *fakeBlueprintStore) InsertBlueprintSync(_ context.Context, run store.BlueprintSync) (store.BlueprintSync, error) {
	if f.insertSyncErr != nil {
		return store.BlueprintSync{}, f.insertSyncErr
	}
	if run.ID == "" {
		run.ID = fmt.Sprintf("bsr-fake-%d", len(f.insertedSyncs))
	}
	f.insertedSyncs = append(f.insertedSyncs, run)
	return run, nil
}

func (f *fakeBlueprintStore) UpdateBlueprintSync(_ context.Context, id, state string, completedAt *time.Time, errMsg *string) (store.BlueprintSync, error) {
	if f.updateSyncErr != nil {
		return store.BlueprintSync{}, f.updateSyncErr
	}
	out := store.BlueprintSync{ID: id, State: state, CompletedAt: completedAt, ErrorMessage: errMsg}
	f.lastSyncUpdate = out
	return out, nil
}

func (f *fakeBlueprintStore) ListBlueprintSyncs(_ context.Context, blueprintID, _ string, limit int) ([]store.BlueprintSync, error) {
	_ = blueprintID
	f.gotSyncLimit = limit
	return f.syncsToReturn, nil
}

// --- helpers ---

func newBlueprintService(fs *fakeBlueprintStore, ws core.WorkspaceResolver) *Service {
	cl := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: cl, Namespace: "default", Workspace: ws},
		Blueprints: fs,
	}
	return svc
}

func blueprintSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

// --- ValidateBlueprint ---

func TestValidateBlueprintValidYAML(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", stackManifest)
	if err != nil {
		t.Fatalf("ValidateBlueprint(valid): %v", err)
	}
	if !v.Valid || len(v.Errors) != 0 {
		t.Errorf("valid manifest: want Valid=true no errors, got %+v", v)
	}
	if v.Plan == nil || len(v.Plan.Services) != 3 || len(v.Plan.Databases) != 1 || v.Plan.TotalActions != 4 {
		t.Errorf("valid manifest: unexpected plan: %+v", v.Plan)
	}
	if v.Plan != nil && v.Plan.Mode != "current_state" {
		t.Errorf("validation plan mode = %q, want current_state", v.Plan.Mode)
	}
	if v.Plan == nil || len(v.Plan.Actions) != 4 {
		t.Errorf("validation action plan = %+v, want four create actions", v.Plan)
	} else {
		for _, action := range v.Plan.Actions {
			if action.Operation != BlueprintPlanCreate {
				t.Errorf("action = %+v, want create", action)
			}
		}
	}
}

func TestValidateBlueprintCurrentStateActionPlan(t *testing.T) {
	existing := sampleApp("web")
	existing.Spec.Image = "nginx:1"
	existing.Spec.Type = appv1alpha1.TypeWebService
	existing.Spec.Runtime = "image"
	svc := &Service{Base: &core.Base{Client: fakeClient(existing), Namespace: "default"}}
	manifest := `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
`
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !validation.Valid || validation.Plan == nil {
		t.Fatalf("ValidateBlueprint(noop): validation=%+v err=%v", validation, err)
	}
	if validation.Plan.Mode != "current_state" || len(validation.Plan.Actions) != 1 || validation.Plan.Actions[0].Operation != BlueprintPlanNoop {
		t.Fatalf("noop action plan = %+v", validation.Plan)
	}

	manifest = strings.Replace(manifest, "nginx:1", "nginx:2", 1)
	validation, err = svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !validation.Valid || validation.Plan == nil {
		t.Fatalf("ValidateBlueprint(update): validation=%+v err=%v", validation, err)
	}
	action := validation.Plan.Actions[0]
	if action.Operation != BlueprintPlanUpdate || action.ResourceID != "web" {
		t.Fatalf("update action = %+v", action)
	}
	for _, change := range action.ChangedFields {
		if change.Path == "nginx:2" {
			t.Fatal("current-state plan leaked a manifest value")
		}
	}
}

// round-21 finding 7: the pre-sync plan must resolve a store-managed service by
// its manifest-facing name, not its tenant-prefixed Kubernetes object name.
// Indexing by app.Name (CRName = "<tenant>-<name>") while looking up the bare
// manifest name missed every store-managed service, so the plan an approver
// reviews reported live services as fresh creates and skipped their field-change
// preflight. The existing current-state test seeds a bare-named service, so it
// cannot catch this; this one seeds a real tenant-prefixed object name.
func TestValidateBlueprintCurrentStatePlanResolvesStoreManagedService(t *testing.T) {
	existing := sampleApp(core.CRName("tea-a", "web")) // object name is tenant-prefixed
	existing.Namespace = "tea-a"
	existing.Labels = map[string]string{
		core.LabelTenant:      "tea-a",
		core.LabelServiceName: "web",
		store.LabelAppID:      "srv-web",
		store.LabelManagedBy:  store.ManagedByValue,
	}
	existing.Spec.Image = "nginx:1"
	existing.Spec.Type = appv1alpha1.TypeWebService
	existing.Spec.Runtime = "image"

	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{}, existing)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})
	manifest := `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
`
	validation, err := svc.ValidateBlueprint(ctx, "tea-a", manifest)
	if err != nil || !validation.Valid || validation.Plan == nil {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", validation, err)
	}
	if len(validation.Plan.Actions) != 1 {
		t.Fatalf("action plan = %+v, want one action", validation.Plan.Actions)
	}
	action := validation.Plan.Actions[0]
	if action.Operation == BlueprintPlanCreate {
		t.Fatalf("store-managed live service reported as a create: %+v", action)
	}
	if action.ResourceID != "srv-web" {
		t.Fatalf("resolved resource id = %q, want srv-web (the store-managed app id)", action.ResourceID)
	}
}

// planActionsForTest returns the authorized current-state actions for a
// manifest planned in workspace tea-a, failing the test on any other outcome.
func planActionsForTest(ctx context.Context, t *testing.T, svc *Service, manifest string) []BlueprintPlanAction {
	t.Helper()
	validation, err := svc.ValidateBlueprint(ctx, "tea-a", manifest)
	if err != nil || !validation.Valid || validation.Plan == nil {
		t.Fatalf("ValidateBlueprint: validation=%+v err=%v", validation, err)
	}
	if validation.Plan.Mode != "current_state" {
		t.Fatalf("plan mode = %q, want current_state", validation.Plan.Mode)
	}
	return validation.Plan.Actions
}

func parseBlueprintStackForTest(t *testing.T, manifest string) parsedStack {
	t.Helper()
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("CompileBlueprintIR() problems = %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parseCompiledStack: %v", err)
	}
	return st
}

// w6/m125: the live fixture that exposed round-21 finding 7, driven through
// the seams production uses. The service is created by the stack apply path
// (so the persisted spec is whatever createFromStack really stores, not a
// hand-built one), then re-planned: an unchanged manifest is a no-op, a
// changed one is an update naming the real field, a new sibling is a create in
// the same plan, and the update predicts the apply — the same App moves in
// place and no second one appears. Services report a true no-op rather than
// the conservative update env groups use: their spec is fully readable, so a
// zero diff is provable.
func TestValidateBlueprintCurrentStatePlanPredictsStoreManagedApply(t *testing.T) {
	const manifest = `services:
  - type: web
    name: qa-bp
    runtime: node
    plan: free
    buildCommand: npm install
    startCommand: npm start
    repo: https://github.com/render-examples/express-hello-world
    branch: main
`
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	if before := planActionsForTest(ctx, t, svc, manifest); len(before) != 1 || before[0].Operation != BlueprintPlanCreate {
		t.Fatalf("plan before apply = %+v, want one create", before)
	}

	st := parseBlueprintStackForTest(t, manifest)
	if _, err := svc.applyBlueprintCreate(ctx, st.services[0].req, st.services[0].fields); err != nil {
		t.Fatalf("apply: %v", err)
	}
	created := getTenantApp(t, cl, "tea-a", "qa-bp")
	if created.Labels[core.LabelServiceName] != "qa-bp" || created.Name == "qa-bp" {
		t.Fatalf("fixture must be a tenant-prefixed store-managed App, got name=%q labels=%v", created.Name, created.Labels)
	}

	unchanged := planActionsForTest(ctx, t, svc, manifest)
	if len(unchanged) != 1 || unchanged[0].Operation != BlueprintPlanNoop || unchanged[0].ResourceID != "srv-test" {
		t.Fatalf("re-plan of the unchanged manifest = %+v, want one noop on srv-test", unchanged)
	}

	changed := strings.Replace(manifest, "npm install", "npm ci", 1)
	updated := planActionsForTest(ctx, t, svc, changed)
	if len(updated) != 1 || updated[0].Operation != BlueprintPlanUpdate || updated[0].ResourceID != "srv-test" {
		t.Fatalf("re-plan of the changed manifest = %+v, want one update on srv-test", updated)
	}
	if reflect.DeepEqual(unchanged, updated) {
		t.Fatal("changed and unchanged manifests planned identically")
	}
	paths := map[string]bool{}
	for _, change := range updated[0].ChangedFields {
		paths[change.Path] = true
		if strings.Contains(change.Path, "npm") {
			t.Fatalf("plan leaked a manifest value: %q", change.Path)
		}
	}
	if !paths["buildCommand"] {
		t.Fatalf("update changedFields = %+v, want buildCommand", updated[0].ChangedFields)
	}

	mixed := changed + `  - type: web
    name: qa-bp-new
    runtime: image
    image: {url: nginx:1}
`
	ops := map[string]BlueprintPlanOperation{}
	for _, action := range planActionsForTest(ctx, t, svc, mixed) {
		ops[action.Name] = action.Operation
	}
	if len(ops) != 2 || ops["qa-bp"] != BlueprintPlanUpdate || ops["qa-bp-new"] != BlueprintPlanCreate {
		t.Fatalf("mixed plan operations = %v, want qa-bp update + qa-bp-new create", ops)
	}

	st = parseBlueprintStackForTest(t, changed)
	if _, err := svc.applyBlueprintCreate(ctx, st.services[0].req, st.services[0].fields); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	after := getTenantApp(t, cl, "tea-a", "qa-bp")
	if after.Spec.BuildCommand != "npm ci" || after.Labels[store.LabelAppID] != created.Labels[store.LabelAppID] {
		t.Fatalf("re-apply: buildCommand=%q appID=%q, want npm ci on the original app %q", after.Spec.BuildCommand, after.Labels[store.LabelAppID], created.Labels[store.LabelAppID])
	}
	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps, client.InNamespace("tea-a")); err != nil || len(apps.Items) != 1 {
		t.Fatalf("apps after re-apply = %d (err %v), want exactly one", len(apps.Items), err)
	}
	if settled := planActionsForTest(ctx, t, svc, changed); len(settled) != 1 || settled[0].Operation != BlueprintPlanNoop {
		t.Fatalf("plan after re-apply = %+v, want noop", settled)
	}
}

// w6/m125: the two neighbours of the store-managed case. A legacy hand-applied
// App carries no service-name label, so its object name IS the public name and
// must still resolve; an App labelled for another workspace must never match,
// even when it sits in the listed namespace.
func TestValidateBlueprintCurrentStatePlanLegacyAndForeignServices(t *testing.T) {
	legacy := sampleApp("web")
	legacy.Namespace = "tea-a"
	legacy.Labels = map[string]string{core.LabelTenant: "tea-a"}
	legacy.Spec.Image = "nginx:1"
	legacy.Spec.Type = appv1alpha1.TypeWebService
	legacy.Spec.Runtime = "image"

	foreign := sampleApp(core.CRName("tea-b", "api"))
	foreign.Namespace = "tea-a" // misfiled on purpose: only the tenant label may exclude it
	foreign.Labels = map[string]string{core.LabelTenant: "tea-b", core.LabelServiceName: "api"}
	foreign.Spec.Image = "nginx:1"
	foreign.Spec.Type = appv1alpha1.TypeWebService
	foreign.Spec.Runtime = "image"

	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{}, legacy, foreign)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	byName := map[string]BlueprintPlanAction{}
	for _, action := range planActionsForTest(ctx, t, svc, `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
`) {
		byName[action.Name] = action
	}
	if got := byName["web"]; got.Operation != BlueprintPlanNoop || got.ResourceID != "web" {
		t.Fatalf("legacy bare-named App = %+v, want noop resolved by object name", got)
	}
	if got := byName["api"]; got.Operation != BlueprintPlanCreate || got.ResourceID != "" {
		t.Fatalf("App labelled for another workspace = %+v, want create", got)
	}
}

// w6/m125: a legacy bare-named App beside its store-managed twin would leave
// list order deciding which spec the plan diffs against. The services loop
// refuses the way the datastore loops always have.
func TestValidateBlueprintCurrentStatePlanRejectsDuplicateServiceName(t *testing.T) {
	legacy := sampleApp("web")
	legacy.Namespace = "tea-a"
	legacy.Labels = map[string]string{core.LabelTenant: "tea-a"}

	managed := sampleApp(core.CRName("tea-a", "web"))
	managed.Namespace = "tea-a"
	managed.Labels = map[string]string{
		core.LabelTenant:      "tea-a",
		core.LabelServiceName: "web",
		store.LabelAppID:      "srv-web",
		store.LabelManagedBy:  store.ManagedByValue,
	}

	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{}, legacy, managed)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	_, err := svc.ValidateBlueprint(ctx, "tea-a", `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
`)
	if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), `service name "web"`) {
		t.Fatalf("duplicate service name: err = %v, want %v naming the service", err, core.ErrConflict)
	}
}

func TestValidateBlueprintBadYAML(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	const bad = `services:
  - name: ""
    type: web
    runtime: image
`
	v, err := svc.ValidateBlueprint(context.Background(), "", bad)
	if err != nil {
		t.Fatalf("ValidateBlueprint(bad): unexpected error %v", err)
	}
	if v.Valid || len(v.Errors) == 0 {
		t.Errorf("invalid manifest: want Valid=false with errors, got %+v", v)
	}
	if v.Errors[0].Error == "" || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].name" {
		t.Errorf("invalid manifest: want a structured services[0].name error, got %+v", v.Errors)
	}
}

func TestValidateBlueprintRejectsUnknownSourceField(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	validation, err := svc.ValidateBlueprint(context.Background(), "", `
services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx}
    typoThatWouldPreviouslyBeIgnored: true
`)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, "typoThatWouldPreviouslyBeIgnored") {
		t.Fatalf("unknown field validation = %+v", validation)
	}
	if validation.Errors[0].Path == nil || *validation.Errors[0].Path != "services[0].typoThatWouldPreviouslyBeIgnored" {
		t.Fatalf("unknown field path = %+v", validation.Errors[0])
	}
}

func TestValidateBlueprintRejectsUnknownNestedSourceField(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	validation, err := svc.ValidateBlueprint(context.Background(), "", `
services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx}
    scaling: {minInstances: 1, typo: true}
`)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if validation.Valid || len(validation.Errors) != 1 || validation.Errors[0].Path == nil || *validation.Errors[0].Path != "services[0].scaling.typo" {
		t.Fatalf("nested unknown validation = %+v", validation)
	}
}

func TestValidateBlueprintRejectsFieldsTheTargetServiceKindCannotApply(t *testing.T) {
	svc, _ := newService(nil)
	validation, err := svc.ValidateBlueprint(context.Background(), "", `services:
  - type: worker
    name: queue
    runtime: image
    image: {url: nginx:1.27}
    ipAllowList: [{source: 192.0.2.0/24}]
`)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if validation.Valid || len(validation.Errors) != 1 || validation.Errors[0].Path == nil || *validation.Errors[0].Path != "services[0].ipAllowList" {
		t.Fatalf("worker ipAllowList validation = %+v", validation)
	}
}

func TestValidateBlueprintSyntaxErrorIncludesLine(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	const bad = "services:\n  - name: web\n    envVars: [\n"
	v, err := svc.ValidateBlueprint(context.Background(), "", bad)
	if err != nil {
		t.Fatalf("ValidateBlueprint(syntax error): unexpected error %v", err)
	}
	if v.Valid || len(v.Errors) != 1 || v.Errors[0].Line == nil || *v.Errors[0].Line < 1 {
		t.Errorf("syntax error: want structured error with a source line, got %+v", v)
	}
}

func TestDecodeBlueprintValidationRequestAllowsTenMiBFileOnly(t *testing.T) {
	t.Parallel()
	requestFor := func(contents []byte) *http.Request {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("ownerId", "tea-test"); err != nil {
			t.Fatal(err)
		}
		part, err := writer.CreateFormFile("file", "render.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(contents); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/blueprints/validate", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		return req
	}

	valid := bytes.Repeat([]byte{'x'}, maxBlueprintValidationFileBytes)
	owner, contents, err := decodeBlueprintValidationRequest(httptest.NewRecorder(), requestFor(valid))
	if err != nil || owner != "tea-test" || len(contents) != len(valid) {
		t.Fatalf("10 MiB file = owner %q bytes %d err %v", owner, len(contents), err)
	}
	_, _, err = decodeBlueprintValidationRequest(httptest.NewRecorder(), requestFor(append(valid, 'x')))
	if err == nil || !strings.Contains(err.Error(), "10 MiB") {
		t.Fatalf("10 MiB + 1 file error = %v", err)
	}
}

func TestValidateBlueprintStateless(t *testing.T) {
	// validate must not touch the store — Blueprints=nil must not panic.
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: nil}
	if _, err := svc.ValidateBlueprint(context.Background(), "", stackManifest); err != nil {
		t.Fatalf("ValidateBlueprint with nil store: %v", err)
	}
}

func TestSyncFalseVarsUsesNormalizedAllLocationIR(t *testing.T) {
	t.Parallel()
	manifest := `
services:
  - name: root
    type: web
    runtime: image
    image: {url: nginx}
    envVars: [{key: ROOT_SECRET, sync: false}]
ungrouped:
  services:
    - name: ungrouped
      type: web
      runtime: image
      image: {url: nginx}
      envVars: [{key: UNGROUPED_SECRET, sync: false}]
projects:
  - name: app
    environments:
      - name: prod
        services:
          - name: nested
            type: web
            runtime: image
            image: {url: nginx}
            envVars: [{key: NESTED_SECRET, sync: false}]
`
	_, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("CompileBlueprintIR() problems = %#v", problems)
	}
	if got, want := strings.Join(syncFalseVarsFromBlueprintIR(ir), ","), "NESTED_SECRET,ROOT_SECRET,UNGROUPED_SECRET"; got != want {
		t.Fatalf("sync:false vars = %q, want %q", got, want)
	}
}

func TestValidateBlueprintBuildsPlanFromCompiledNestedIR(t *testing.T) {
	t.Parallel()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `
services:
  - name: root
    type: web
    runtime: image
    image: {url: nginx}
    envVars: [{key: ROOT_SECRET, sync: false}]
ungrouped:
  services:
    - name: ungrouped
      type: web
      runtime: image
      image: {url: nginx}
      envVars: [{key: UNGROUPED_SECRET, sync: false}]
projects:
  - name: app
    environments:
      - name: prod
        services:
          - name: nested
            type: web
            runtime: image
            image: {url: nginx}
            envVars: [{key: NESTED_SECRET, sync: false}]
`
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if !validation.Valid || validation.Plan == nil {
		t.Fatalf("validation = %#v, want a valid structural plan", validation)
	}
	if got, want := strings.Join(validation.Plan.Services, ","), "root,ungrouped,nested"; got != want {
		t.Errorf("plan services = %q, want %q", got, want)
	}
	if got, want := strings.Join(validation.Plan.SyncFalseVars, ","), "NESTED_SECRET,ROOT_SECRET,UNGROUPED_SECRET"; got != want {
		t.Errorf("plan sync:false vars = %q, want %q", got, want)
	}
}

func TestResolveBlueprintResourcesUsesNormalizedAllLocationIR(t *testing.T) {
	t.Parallel()
	labels := map[string]string{core.LabelTenant: "tea-test"}
	objects := []client.Object{
		&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "root", Namespace: "tea-test", Labels: labels}},
		&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "nested", Namespace: "tea-test", Labels: labels}},
		&appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: "tea-test", Labels: labels}, Spec: appv1alpha1.DatabaseSpec{Name: "orders"}},
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-cache", Namespace: "tea-test", Labels: labels}, Spec: appv1alpha1.KeyValueSpec{Name: "cache"}},
	}
	svc := &Service{
		Base:      &core.Base{Client: fakeClient(objects...), Namespace: "default"},
		EnvGroups: newFakeEnvGroups("shared"),
	}
	manifest := `
envVarGroups:
  - name: shared
    envVars:
      - {key: LOG_LEVEL, value: info}
services:
  - name: root
    type: web
    runtime: docker
ungrouped:
  databases:
    - name: orders
projects:
  - name: app
    environments:
      - name: prod
        services:
          - name: nested
            type: web
            runtime: docker
          - name: cache
            type: keyvalue
            ipAllowList: []
`
	if _, problems := CompileBlueprintSource(manifest); len(problems) > 0 {
		t.Fatalf("strict compiler rejected inventory fixture: %#v", problems)
	}
	resources := svc.resolveBlueprintResources(context.Background(), store.Blueprint{TenantID: "tea-test", Manifest: manifest})
	if got, want := len(resources), 5; got != want {
		t.Fatalf("resource inventory = %#v, want %d resources", resources, want)
	}
	got := map[string]string{}
	gotIDs := map[string]string{}
	for _, resource := range resources {
		got[resource.Name] = resource.Type
		gotIDs[resource.Name] = resource.ID
	}
	for name, wantType := range map[string]string{"root": "web_service", "nested": "web_service", "orders": "postgres", "cache": "key_value", "shared": "environment_group"} {
		if got[name] != wantType {
			t.Errorf("resource %q type = %q, want %q", name, got[name], wantType)
		}
	}
	if gotIDs["shared"] != "evg-shared" {
		t.Errorf("environment-group resource id = %q, want evg-shared", gotIDs["shared"])
	}
}

// --- PreviewBlueprint ---

// testCommitSHA is the canned immutable revision test fetchers pin to.
const testCommitSHA = "0123456789abcdef0123456789abcdef01234567"

// fakeBlueprintFetcher returns canned contents/sha or an error.
type fakeBlueprintFetcher struct {
	contents string
	sha      string
	err      error
	path     string
}

func (f fakeBlueprintFetcher) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.sha != "" {
		return f.sha, nil
	}
	return testCommitSHA, nil
}

func (f fakeBlueprintFetcher) FetchBlueprintFileAtCommit(_ context.Context, _ string, _ string, _ string, filePath string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	expectedPath := f.path
	if expectedPath == "" {
		expectedPath = CanonicalBlueprintFilename
	}
	if filePath != expectedPath {
		return "", fmt.Errorf("bad request: %s not found on main", filePath)
	}
	return f.contents, nil
}

func TestPreviewBlueprintFound(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: fakeBlueprintFetcher{contents: stackManifest, sha: "abc1234"},
	}
	p, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "main", "")
	if err != nil {
		t.Fatalf("PreviewBlueprint: %v", err)
	}
	if !p.Found || p.Manifest != stackManifest || p.CommitID != "abc1234" || p.Error != "" {
		t.Errorf("found preview: got %+v", p)
	}
	if p.Validation == nil || !p.Validation.Valid || p.Validation.Plan == nil || p.Validation.Plan.TotalActions != 4 {
		t.Errorf("found preview: want valid validation with plan, got %+v", p.Validation)
	}
}

type blueprintFilesFetcher map[string]string

func (f blueprintFilesFetcher) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	return testCommitSHA, nil
}

func (f blueprintFilesFetcher) FetchBlueprintFileAtCommit(_ context.Context, _ string, _ string, _ string, filePath string) (string, error) {
	contents, ok := f[filePath]
	if !ok {
		return "", fmt.Errorf("bad request: %s not found on main", filePath)
	}
	return contents, nil
}

func TestBlueprintFilenameDiscovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for name, test := range map[string]struct {
		files    blueprintFilesFetcher
		explicit string
		wantPath string
		wantErr  error
	}{
		"canonical":       {files: blueprintFilesFetcher{CanonicalBlueprintFilename: "canonical"}, wantPath: CanonicalBlueprintFilename},
		"legacy fallback": {files: blueprintFilesFetcher{LegacyBlueprintFilename: "legacy"}, wantPath: LegacyBlueprintFilename},
		"explicit legacy": {files: blueprintFilesFetcher{CanonicalBlueprintFilename: "canonical", LegacyBlueprintFilename: "legacy"}, explicit: LegacyBlueprintFilename, wantPath: LegacyBlueprintFilename},
		"ambiguous":       {files: blueprintFilesFetcher{CanonicalBlueprintFilename: "canonical", LegacyBlueprintFilename: "legacy"}, wantErr: ErrBlueprintFilenameAmbiguous},
	} {
		t.Run(name, func(t *testing.T) {
			contents, _, path, err := discoverBlueprintFile(ctx, test.files, "tea-test", "https://github.com/a/app", "main", test.explicit)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("discover error = %v, want %v", err, test.wantErr)
			}
			if err == nil && (path != test.wantPath || contents == "") {
				t.Fatalf("discover = contents %q path %q, want path %q", contents, path, test.wantPath)
			}
		})
	}
}

func TestBlueprintExplicitPathAllowsOnlyBlueprintFilenames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	files := blueprintFilesFetcher{"deploy/render.yaml": stackManifest, "infra/bex/stack.yaml": stackManifest}
	// Any clean repo-relative YAML path is fetchable since w8/m19 t006
	// (Render's custom Blueprint paths, 2026-02-09).
	for _, ok := range []string{"deploy/render.yaml", "infra/bex/stack.yaml"} {
		if _, _, gotPath, err := discoverBlueprintFile(ctx, files, "tea-test", "https://github.com/a/app", "main", ok); err != nil || gotPath != ok {
			t.Fatalf("approved Blueprint path %q = %q, %v", ok, gotPath, err)
		}
	}
	// Non-YAML files and escapes stay rejected before any repository fetch.
	for _, unsafePath := range []string{".env", "Makefile", "../render.yaml", "./render.yaml", `deploy\render.yaml`} {
		if _, _, _, err := discoverBlueprintFile(ctx, files, "tea-test", "https://github.com/a/app", "main", unsafePath); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("path %q error = %v, want ErrBadRequest before repository fetch", unsafePath, err)
		}
	}
}

// relationChecker authorizes exactly the allowed relations and records every
// relation asked — the role-ladder fake for response-shaping tests.
type relationChecker struct {
	allow map[string]bool
	asked []string
}

func (c *relationChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.asked = append(c.asked, relation)
	return c.allow[relation], nil
}

func TestPreviewBlueprintRequiresSensitiveRead(t *testing.T) {
	checker := &relationChecker{allow: map[string]bool{core.RelCanViewSensitive: true}}
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Authz: checker},
		GitFetcher: fakeBlueprintFetcher{contents: stackManifest, sha: "abc1234"},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "developer", Method: "session"})
	if _, err := svc.PreviewBlueprint(ctx, "", "https://github.com/a/app", "main", ""); err != nil {
		t.Fatalf("sensitive preview: %v", err)
	}
	if len(checker.asked) != 2 || checker.asked[0] != core.RelCanViewSensitive || checker.asked[1] != core.RelCanViewSensitive {
		t.Fatalf("preview relations = %v, want Authorize + AuthorizeFresh on can_view_sensitive", checker.asked)
	}
}

func TestPreviewBlueprintWarnsOnImplicitLegacyFilenameFallback(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: blueprintFilesFetcher{LegacyBlueprintFilename: stackManifest},
	}
	preview, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "main", "")
	if err != nil {
		t.Fatalf("PreviewBlueprint: %v", err)
	}
	if !preview.Found || preview.Warning == "" || !strings.Contains(preview.Warning, CanonicalBlueprintFilename) {
		t.Fatalf("legacy fallback preview = %+v, want a render.yaml migration warning", preview)
	}
}

func TestPreviewBlueprintInvalidManifest(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: fakeBlueprintFetcher{path: LegacyBlueprintFilename, contents: "services:\n  - name: \"\"\n    type: web\n"},
	}
	p, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "main", "bex.yml")
	if err != nil {
		t.Fatalf("PreviewBlueprint(invalid): %v", err)
	}
	if !p.Found || p.Validation == nil || p.Validation.Valid || len(p.Validation.Errors) == 0 {
		t.Errorf("invalid manifest: want found with validation errors, got %+v", p)
	}
}

func TestPreviewBlueprintFetchErrorIsNotFound(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: fakeBlueprintFetcher{err: fmt.Errorf("bad request: bex.yml not found on main")},
	}
	p, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "main", "")
	if err != nil {
		t.Fatalf("PreviewBlueprint(fetch error): want soft error, got %v", err)
	}
	if p.Found || p.Error != "bex.yml not found on main" || p.Validation != nil {
		t.Errorf("fetch error: want Found=false with message, got %+v", p)
	}
}

func TestPreviewBlueprintRequiresRepoAndBranch(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: fakeBlueprintFetcher{},
	}
	if _, err := svc.PreviewBlueprint(context.Background(), "", "", "main", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("missing repo: want ErrBadRequest, got %v", err)
	}
	if _, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("missing branch: want ErrBadRequest, got %v", err)
	}
}

func TestPreviewBlueprintNoFetcher(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	if _, err := svc.PreviewBlueprint(context.Background(), "", "https://github.com/a/app", "main", ""); !errors.Is(err, ErrBlueprintFetchUnavailable) {
		t.Errorf("nil fetcher: want ErrBlueprintFetchUnavailable, got %v", err)
	}
}

func TestPreviewBlueprintGraphQL(t *testing.T) {
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default"},
		GitFetcher: fakeBlueprintFetcher{contents: stackManifest, sha: "abc1234"},
	}
	schema := blueprintSchema(t, svc)
	res := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: `query {
			blueprintPreview(repo: "https://github.com/a/app", branch: "main") {
				found commitId error
				validation { valid plan { services databases totalActions } }
			}
		}`,
		Context: context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["blueprintPreview"].(map[string]any)
	if data["found"] != true || data["commitId"] != "abc1234" {
		t.Errorf("graphql preview: got %+v", data)
	}
	validation, _ := data["validation"].(map[string]any)
	if validation == nil || validation["valid"] != true {
		t.Errorf("graphql preview validation: got %+v", validation)
	}
}

// --- ListBlueprints ---

func TestListBlueprintsNoStore(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	if _, err := svc.ListBlueprints(context.Background(), "tea-a"); !errors.Is(err, ErrBlueprintsUnavailable) {
		t.Errorf("ListBlueprints no store: want ErrBlueprintsUnavailable, got %v", err)
	}
}

func TestListBlueprintsScopedToTenant(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(
		store.Blueprint{ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app", Branch: "main", Status: "active", Name: "app"},
		store.Blueprint{ID: "blp-2", TenantID: "tea-b", Repo: "https://github.com/b/other", Branch: "main", Status: "active", Name: "other"},
	)
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	views, err := svc.ListBlueprints(ctx, "tea-a")
	if err != nil {
		t.Fatalf("ListBlueprints: %v", err)
	}
	if len(views) != 1 || views[0].ID != "blp-1" {
		t.Errorf("ListBlueprints: want [blp-1], got %+v", views)
	}
}

// codex r7 #11 — the stored manifest is the same private repository content
// PreviewBlueprint gates on can_view_sensitive, so blueprint get/list must
// not hand it to a plain viewer while keeping the metadata viewer-readable.
func TestBlueprintManifestGatedOnSensitiveRead(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	newService := func(allow map[string]bool) *Service {
		fs := newFakeBlueprintStore(store.Blueprint{
			ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
			Branch: "main", Manifest: stackManifest, Status: "active", Name: "app",
		})
		return &Service{
			Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws, Authz: &relationChecker{allow: allow}},
			Blueprints: fs,
		}
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	viewer := newService(map[string]bool{core.RelCanView: true})
	if v, err := viewer.GetBlueprintByID(ctx, "blp-1", "tea-a"); err != nil || v.Manifest != "" || v.Name != "app" {
		t.Errorf("viewer get = manifest %q name %q (%v), want blank manifest with metadata intact", v.Manifest, v.Name, err)
	}
	if vs, err := viewer.ListBlueprints(ctx, "tea-a"); err != nil || len(vs) != 1 || vs[0].Manifest != "" {
		t.Errorf("viewer list = %+v (%v), want one view with a blank manifest", vs, err)
	}

	developer := newService(map[string]bool{core.RelCanView: true, core.RelCanViewSensitive: true})
	if v, err := developer.GetBlueprintByID(ctx, "blp-1", "tea-a"); err != nil || v.Manifest != stackManifest {
		t.Errorf("developer get manifest = %q (%v), want the stored manifest", v.Manifest, err)
	}
	if vs, err := developer.ListBlueprints(ctx, "tea-a"); err != nil || len(vs) != 1 || vs[0].Manifest != stackManifest {
		t.Errorf("developer list = %+v (%v), want the stored manifest", vs, err)
	}
}

func TestListBlueprintsEmpty(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: newFakeBlueprintStore()}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	views, err := svc.ListBlueprints(ctx, "tea-a")
	if err != nil {
		t.Fatalf("ListBlueprints empty: %v", err)
	}
	if len(views) != 0 {
		t.Errorf("ListBlueprints empty: want [], got %+v", views)
	}
}

// --- SyncBlueprint ---

func TestSyncBlueprintNoStore(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	if _, err := svc.SyncBlueprint(context.Background(), "blp-1", "tea-a", "", ""); !errors.Is(err, ErrBlueprintsUnavailable) {
		t.Errorf("SyncBlueprint no store: want ErrBlueprintsUnavailable, got %v", err)
	}
}

func TestSyncBlueprintNotFound(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	// core.ErrNotFound, not store.ErrNotFound: since w6/m96 every blueprint verb
	// runs its store error through store.MapError, which is what turns a missing
	// row into a 404 on REST/GraphQL/MCP instead of a 500 "internal error".
	// MapError deliberately keeps the store error in the message (%v) rather
	// than the chain, so the core sentinel is the one callers match on.
	if _, err := svc.SyncBlueprint(ctx, "blp-missing", "tea-a", "", ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("SyncBlueprint missing id: want core.ErrNotFound, got %v", err)
	}
}

func TestSyncBlueprintReappliesManifest(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Path:     CanonicalBlueprintFilename,
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
		// w8/m36 t003: a Git-backed sync with no fetcher is a failure, so the
		// reapply path fetches the unchanged manifest through the seam.
		GitFetcher: fakeBlueprintFetcher{contents: stackManifest},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	res, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "")
	if err != nil {
		t.Fatalf("SyncBlueprint: %v", err)
	}
	if res.Blueprint.ID != "blp-1" {
		t.Errorf("SyncBlueprint: blueprint id = %q, want blp-1", res.Blueprint.ID)
	}
	// w8/m36 t002: the run is stamped with the resolved commit the bytes were
	// read at — never an empty or fabricated provenance.
	if len(fs.insertedSyncs) != 1 || fs.insertedSyncs[0].CommitID != testCommitSHA {
		t.Errorf("SyncBlueprint run provenance = %+v, want one run at %q", fs.insertedSyncs, testCommitSHA)
	}
	if len(res.Stack.Services) == 0 {
		t.Errorf("SyncBlueprint: stack.services empty, expected stack to be deployed")
	}
}

func TestSyncBlueprintDoesNotApplyStoredManifestAfterInvalidGitUpdate(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Path:     CanonicalBlueprintFilename,
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	client := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: client, Namespace: "default", Workspace: ws},
		Blueprints: fs,
		GitFetcher: fakeBlueprintFetcher{contents: `
services:
  - name: app
    type: web
    runtime: image
    image: {url: nginx:latest}
    autoDeployTrigger: checksPass
`},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SyncBlueprint invalid fetched manifest: want ErrBadRequest, got %v", err)
	}
	if stored := fs.blueprints["blp-1"]; stored.Manifest != stackManifest || stored.Status != "active" {
		t.Errorf("invalid fetched manifest mutated stored blueprint: %+v", stored)
	}
	var apps appv1alpha1.AppList
	if err := client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("invalid fetched manifest applied stored blueprint: got %d Apps", len(apps.Items))
	}
}

// TestBlueprintCoreEntrypointsRefuseUnsupportedManifestBeforeWrites keeps the
// strict compiler as the one boundary for every core Blueprint route. REST,
// GraphQL, and MCP exercise ValidateBlueprint in TestValidateFiveFormsCrossSurface;
// this test covers the remaining preview/create/deploy/manual-sync/Git-sync
// routes and asserts that none can fall back to an older manifest or mutate.
func TestBlueprintCoreEntrypointsRefuseUnsupportedManifestBeforeWrites(t *testing.T) {
	const unsupported = `services:
  - name: app
    type: web
    runtime: image
    image: {url: nginx:latest}
    autoDeployTrigger: checksPass
`
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Name:     "app",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Path:     CanonicalBlueprintFilename,
		Manifest: stackManifest,
		Status:   "active",
	})
	client := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: client, Namespace: "default", Workspace: ws},
		Blueprints: fs,
		GitFetcher: fakeBlueprintFetcher{contents: unsupported, sha: "invalid-sha"},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	validation, err := svc.ValidateBlueprint(ctx, "tea-a", unsupported)
	if err != nil || validation.Valid || len(validation.Errors) != 1 || validation.Errors[0].Code != "BLUEPRINT_CAPABILITY_UNSUPPORTED" {
		t.Fatalf("ValidateBlueprint unsupported = %+v, %v", validation, err)
	}
	preview, err := svc.PreviewBlueprint(ctx, "tea-a", "https://github.com/a/app", "main", "")
	if err != nil || !preview.Found || preview.Validation == nil || preview.Validation.Valid {
		t.Fatalf("PreviewBlueprint unsupported = %+v, %v", preview, err)
	}

	cases := []struct {
		name string
		run  func() error
	}{
		{
			name: "deploy",
			run: func() error {
				_, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Repo: "https://github.com/a/app", Branch: "main", Manifest: unsupported})
				return err
			},
		},
		{
			name: "create",
			run: func() error {
				_, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/a/app", Branch: "main"})
				return err
			},
		},
		{
			name: "manual sync",
			run: func() error {
				_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", unsupported, "")
				return err
			},
		},
		{
			name: "Git sync",
			run: func() error {
				_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "")
				return err
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("%s accepted unsupported Blueprint: %v", test.name, err)
			}
		})
	}

	if stored := fs.blueprints["blp-1"]; stored.Manifest != stackManifest || stored.Status != "active" {
		t.Errorf("unsupported Blueprint mutated stored record: %+v", stored)
	}
	if len(fs.blueprints) != 1 {
		t.Errorf("unsupported Blueprint created a record: %d stored", len(fs.blueprints))
	}
	var apps appv1alpha1.AppList
	if err := client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("unsupported Blueprint created %d Apps", len(apps.Items))
	}
}

// --- w8/m36 t003/t004: Git-backed sync fails without falling back ---

// A Git-backed sync whose source cannot be fetched fails with zero workload
// mutations: the stored manifest is preserved, nothing is applied, and the
// attempt terminates in history with its sanitized reason (t003+t004).
func TestSyncBlueprintFetchFailureRecordsErrorRun(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	client := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: client, Namespace: "default", Workspace: ws},
		Blueprints: fs,
		GitFetcher: fakeBlueprintFetcher{err: fmt.Errorf("bad request: render.yaml not found on main")},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "")
	if err == nil {
		t.Fatal("SyncBlueprint unreachable source: want error, got success")
	}
	if stored := fs.blueprints["blp-1"]; stored.Manifest != stackManifest || stored.Status != "active" {
		t.Errorf("failed fetch mutated the stored blueprint: %+v", stored)
	}
	var apps appv1alpha1.AppList
	if err := client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("failed fetch applied %d Apps; want zero mutations", len(apps.Items))
	}
	if len(fs.insertedSyncs) != 1 {
		t.Fatalf("inserted runs = %d, want the one failed attempt", len(fs.insertedSyncs))
	}
	if fs.lastSyncUpdate.State != store.BlueprintSyncStateError {
		t.Fatalf("failed sync run state = %q, want error", fs.lastSyncUpdate.State)
	}
	if msg := fs.lastSyncUpdate.ErrorMessage; msg == nil || !strings.Contains(*msg, "not found") {
		t.Fatalf("failed sync run reason = %v, want the actionable fetch failure", msg)
	}
}

// A Git-backed sync with no fetcher configured fails rather than silently
// applying the stored manifest (t003).
func TestSyncBlueprintWithoutFetcherFailsGitBackedSync(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	client := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: client, Namespace: "default", Workspace: ws},
		Blueprints: fs,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", "")
	if !errors.Is(err, ErrBlueprintFetchUnavailable) {
		t.Fatalf("Git-backed sync without fetcher: want ErrBlueprintFetchUnavailable, got %v", err)
	}
	var apps appv1alpha1.AppList
	if err := client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("unavailable fetcher applied %d Apps; want zero mutations", len(apps.Items))
	}
	if fs.lastSyncUpdate.State != store.BlueprintSyncStateError {
		t.Fatalf("failed sync run state = %q, want error", fs.lastSyncUpdate.State)
	}
}

// A failed run insertion blocks every workload side effect: no App,
// datastore, grouping, environment, or ownership mutation without a recorded
// attempt (t004).
func TestSyncBlueprintFailedRunInsertBlocksApply(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	fs.insertSyncErr = errors.New("blueprints store unavailable")
	client := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: client, Namespace: "default", Workspace: ws},
		Blueprints: fs,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", stackManifest, ""); err == nil {
		t.Fatal("SyncBlueprint with failing run insert: want error, got success")
	}
	if stored := fs.blueprints["blp-1"]; stored.Manifest != stackManifest {
		t.Errorf("blocked sync mutated the stored manifest: %+v", stored)
	}
	var apps appv1alpha1.AppList
	if err := client.List(ctx, &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("blocked sync applied %d Apps; want zero mutations", len(apps.Items))
	}
}

// A failed completion write is not reported as verified success — even though
// the apply itself went through (t004).
func TestSyncBlueprintFailedCompletionWriteIsError(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	fs.updateSyncErr = errors.New("blueprints store unavailable")
	client := fakeClient()
	svc := &Service{
		Base:            &core.Base{Client: client, Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	_, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", stackManifest, "")
	if err == nil || !strings.Contains(err.Error(), "record completion") {
		t.Fatalf("SyncBlueprint with failing completion write: want record-completion error, got %v", err)
	}
}

// Explicit supplied-manifest sync stays usable and never claims a Git commit
// it did not consume (t003).
func TestSyncBlueprintSuppliedManifestClaimsNoCommit(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs, DomainOwnership: allowDomainOwnership{}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", stackManifest, ""); err != nil {
		t.Fatalf("SyncBlueprint supplied manifest: %v", err)
	}
	if len(fs.insertedSyncs) != 1 {
		t.Fatalf("inserted runs = %d, want 1", len(fs.insertedSyncs))
	}
	if got := fs.insertedSyncs[0].CommitID; got != "" {
		t.Fatalf("supplied-manifest run commit = %q, want no claimed commit", got)
	}
}

// Implicit canonical/legacy discovery resolves once and probes both filenames
// at that one revision — never two independently moving branch tips (t002).
type pinCountingFetcher struct {
	fakeBlueprintFetcher
	resolves int
	refs     []string
}

func (f *pinCountingFetcher) ResolveBlueprintCommit(ctx context.Context, ws, repo, branch string) (string, error) {
	f.resolves++
	return f.fakeBlueprintFetcher.ResolveBlueprintCommit(ctx, ws, repo, branch)
}

func (f *pinCountingFetcher) FetchBlueprintFileAtCommit(ctx context.Context, ws, repo, commit, filePath string) (string, error) {
	f.refs = append(f.refs, commit)
	return f.fakeBlueprintFetcher.FetchBlueprintFileAtCommit(ctx, ws, repo, commit, filePath)
}

func TestBlueprintDiscoveryPinsOneRevision(t *testing.T) {
	ctx := context.Background()
	fetcher := &pinCountingFetcher{fakeBlueprintFetcher: fakeBlueprintFetcher{contents: "canonical"}}
	contents, sha, filePath, err := discoverBlueprintFile(ctx, fetcher, "ws", "https://github.com/a/app", "main", "")
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if contents != "canonical" || sha != testCommitSHA || filePath != CanonicalBlueprintFilename {
		t.Fatalf("discover = %q %q %q, want canonical at the pinned commit", contents, sha, filePath)
	}
	if fetcher.resolves != 1 {
		t.Fatalf("discover resolved %d times, want exactly 1 pinned resolution", fetcher.resolves)
	}
	for _, ref := range fetcher.refs {
		if ref != testCommitSHA {
			t.Fatalf("discover probed ref %q, want every probe at %q", ref, testCommitSHA)
		}
	}
	if len(fetcher.refs) != 2 {
		t.Fatalf("discover probed %d files, want both filenames checked for ambiguity", len(fetcher.refs))
	}

	files := blueprintFilesFetcher{CanonicalBlueprintFilename: "c", LegacyBlueprintFilename: "l"}
	if _, _, _, err := discoverBlueprintFile(ctx, files, "ws", "https://github.com/a/app", "main", ""); !errors.Is(err, ErrBlueprintFilenameAmbiguous) {
		t.Fatalf("both filenames present: want ErrBlueprintFilenameAmbiguous, got %v", err)
	}
}

func TestSyncBlueprintReplacesManifest(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	const newManifest = `
services:
  - name: freshsvc
    type: web
    runtime: image
    image: {url: "nginx:latest"}
`
	res, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", newManifest, "")
	if err != nil {
		t.Fatalf("SyncBlueprint replace: %v", err)
	}
	// Manifest should be updated in the store.
	stored := fs.blueprints["blp-1"]
	if stored.Manifest != newManifest {
		t.Errorf("SyncBlueprint: stored manifest not updated")
	}
	if len(res.Stack.Services) == 0 {
		t.Errorf("SyncBlueprint replace: stack.services empty")
	}
}

// w6/m50 t001/t007: runSync (shared by manual SyncBlueprint and the
// webhook-triggered auto-sync) used to compute the real apply error and then
// discard it before UpdateBlueprintSync. groupedManifest compiles and passes
// the dry-run action plan cleanly (proven by TestBlueprintGroupingQuotaRefusesWithCodedError)
// but fails inside deployParsedStack's real grouping-apply step because no
// BlueprintGroups store is wired here — an error that only surfaces after the
// running sync row has already been inserted, exactly like a real quota hit
// or datastore-rename failure would.
func TestSyncBlueprintPersistsFailureReason(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", groupedManifest, ""); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("SyncBlueprint grouped manifest without BlueprintGroups: want ErrWorkspacesUnavailable, got %v", err)
	}
	if fs.lastSyncUpdate.State != store.BlueprintSyncStateError {
		t.Fatalf("failed sync run state = %q, want %q", fs.lastSyncUpdate.State, store.BlueprintSyncStateError)
	}
	if fs.lastSyncUpdate.ErrorMessage == nil || *fs.lastSyncUpdate.ErrorMessage == "" {
		t.Fatalf("failed sync run ErrorMessage = %v, want a non-empty reason", fs.lastSyncUpdate.ErrorMessage)
	}

	// A successful sync leaves the run's error message nil.
	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", stackManifest, ""); err != nil {
		t.Fatalf("SyncBlueprint valid manifest: %v", err)
	}
	if fs.lastSyncUpdate.State != store.BlueprintSyncStateSuccess || fs.lastSyncUpdate.ErrorMessage != nil {
		t.Fatalf("successful sync run = %+v, want success state with nil error", fs.lastSyncUpdate)
	}
}

// CreateBlueprint's own initial-sync path (a separate call site from runSync)
// gets the identical treatment: the Git-fetched manifest passes compile +
// action-plan preflight, then fails for real inside deployParsedStack once a
// sync run row already exists.
func TestCreateBlueprintPersistsFailureReason(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore()
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints: fs,
		GitFetcher: fakeBlueprintFetcher{contents: groupedManifest, sha: "deadbeef"},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	_, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/a/app", Branch: "main"})
	if !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("CreateBlueprint grouped manifest without BlueprintGroups: want ErrWorkspacesUnavailable, got %v", err)
	}
	if fs.lastSyncUpdate.State != store.BlueprintSyncStateError {
		t.Fatalf("initial sync run state = %q, want %q", fs.lastSyncUpdate.State, store.BlueprintSyncStateError)
	}
	if fs.lastSyncUpdate.ErrorMessage == nil || *fs.lastSyncUpdate.ErrorMessage == "" {
		t.Fatalf("initial sync run ErrorMessage = %v, want a non-empty reason", fs.lastSyncUpdate.ErrorMessage)
	}
}

// --- upsertBlueprint (auto-registration) ---

func TestUpsertBlueprintAutoRegistersOnDeploy(t *testing.T) {
	fs := newFakeBlueprintStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	ctx := core.WithWorkspace(context.Background(), "tea-a")

	svc.upsertBlueprint(ctx, DeployRequest{
		Repo:     "https://github.com/bex/myapp.git",
		Branch:   "main",
		Manifest: stackManifest,
	})
	if len(fs.blueprints) != 1 {
		t.Fatalf("upsertBlueprint: want 1 blueprint, got %d", len(fs.blueprints))
	}
	for _, b := range fs.blueprints {
		if b.Name != "myapp" {
			t.Errorf("upsertBlueprint: name = %q, want myapp", b.Name)
		}
		if b.TenantID != "tea-a" {
			t.Errorf("upsertBlueprint: tenantID = %q, want tea-a", b.TenantID)
		}
	}
}

func TestUpsertBlueprintNoopWithoutRepo(t *testing.T) {
	fs := newFakeBlueprintStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}

	svc.upsertBlueprint(context.Background(), DeployRequest{Manifest: stackManifest})
	if len(fs.blueprints) != 0 {
		t.Errorf("upsertBlueprint without repo: should not store anything")
	}
}

// --- repoName ---

func TestRepoName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/bex/myapp", "myapp"},
		{"https://github.com/bex/myapp.git", "myapp"},
		{"git@github.com:bex/repo.git", "repo"},
		{"https://github.com/bex/stack/", "stack"},
	}
	for _, c := range cases {
		if got := repoName(c.in); got != c.want {
			t.Errorf("repoName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- REST adapter ---

func TestRESTValidateBlueprint(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := fmt.Sprintf(`{"bexYaml":%q}`, stackManifest)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("validate valid => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Valid {
		t.Errorf("validate valid manifest: want valid=true, got %+v", out)
	}
	if out.Plan == nil || out.Plan.Mode != "current_state" || len(out.Plan.Actions) != 4 {
		t.Errorf("REST validation action plan = %+v, want four current-state actions", out.Plan)
	}
	if !strings.Contains(rec.Body.String(), `"operation"`) || strings.Contains(rec.Body.String(), `"Operation"`) {
		t.Errorf("REST action plan did not use lower-camel wire fields: %s", rec.Body.String())
	}
}

func TestRESTDeployBlueprintUsesStackCore(t *testing.T) {
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body := fmt.Sprintf(`{"bexYaml":%q}`, stackManifest)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/deploy", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("deploy Blueprint => 200, got %d: %s", rec.Code, rec.Body)
	}
	var result StackResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal deploy result: %v", err)
	}
	if len(result.Services) != 3 || len(result.Databases) != 1 {
		t.Fatalf("deploy result = %+v, want the full stack", result)
	}
}

func multipartBlueprintRequest(t *testing.T, ownerID, filename, manifest string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("ownerId", ownerID); err != nil {
		t.Fatalf("ownerId field: %v", err)
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("file field: %v", err)
	}
	if _, err := part.Write([]byte(manifest)); err != nil {
		t.Fatalf("file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest("POST", "/v1/blueprints/validate", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestRESTValidateBlueprintRenderCLIMultipart(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, multipartBlueprintRequest(t, "tea-workspace", "render.yaml", stackManifest))
	if rec.Code != http.StatusOK {
		t.Fatalf("multipart validate => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Valid || out.Plan == nil {
		t.Fatalf("multipart valid manifest: got %+v", out)
	}
	if got, want := out.Plan.Services, []string{"web", "api", "worker"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("plan services = %v, want %v", got, want)
	}
	if got, want := out.Plan.Databases, []string{"db"}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("plan databases = %v, want %v", got, want)
	}
	if out.Plan.TotalActions != 4 {
		t.Errorf("plan totalActions = %d, want 4", out.Plan.TotalActions)
	}
}

func TestRESTValidateBlueprintRenderCLIMultipartInvalidReturnsValidationResult(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, multipartBlueprintRequest(t, "tea-workspace", "render.yaml", "services:\n  - name: \"\"\n"))
	if rec.Code != http.StatusOK {
		t.Fatalf("invalid Blueprint is a validation result, not a transport error: got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid || len(out.Errors) != 1 || out.Errors[0].Error == "" {
		t.Fatalf("invalid multipart manifest: got %+v", out)
	}
}

func TestRESTValidateBlueprintRenderCLIMultipartRequiresOwnerAndFile(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, tc := range []struct {
		name     string
		ownerID  string
		manifest string
	}{{"missing owner", "", stackManifest}, {"empty file", "tea-workspace", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, multipartBlueprintRequest(t, tc.ownerID, "render.yaml", tc.manifest))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("got %d (%s), want 400", rec.Code, rec.Body)
			}
		})
	}
}

func TestRESTValidateBlueprintMultipartOwnerIDScopesAuthorization(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, tc := range []struct {
		ownerID  string
		wantCode int
	}{{"tea-a", http.StatusOK}, {"tea-b", http.StatusForbidden}} {
		req := multipartBlueprintRequest(t, tc.ownerID, "render.yaml", stackManifest)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-a", Method: "oauth2"}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.wantCode {
			t.Errorf("ownerId %q: got %d (%s), want %d", tc.ownerID, rec.Code, rec.Body, tc.wantCode)
		}
	}
}

func TestRESTValidateBlueprintBadYAML(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"bexYaml":"services:\n  - name: \"\"\n    type: web\n"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("validate bad => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid {
		t.Errorf("validate bad manifest: want valid=false, got %+v", out)
	}
}

func TestRESTListBlueprints(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/blueprints?ownerId=tea-a", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-a", Method: "oauth2"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list blueprints => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out []BlueprintView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 1 || out[0].ID != "blp-1" {
		t.Errorf("list blueprints: want [blp-1], got %+v", out)
	}
}

// w6/m50 t002/t007: the error_message a failed sync now persists must reach
// every read surface that already serves sync history — REST, GraphQL, MCP —
// with the same value for the same failed run.
func TestRESTListBlueprintSyncsIncludesErrorMessage(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	errMsg := "quota exceeded: workspace at service limit"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	fs.syncsToReturn = []store.BlueprintSync{
		{ID: "bsr-1", BlueprintID: "blp-1", State: store.BlueprintSyncStateError, ErrorMessage: &errMsg},
		{ID: "bsr-2", BlueprintID: "blp-1", State: store.BlueprintSyncStateSuccess},
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/blueprints/blp-1/syncs?ownerId=tea-a", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-a", Method: "oauth2"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list blueprint syncs => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out []BlueprintSyncView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("list blueprint syncs: want 2 rows, got %d", len(out))
	}
	if out[0].ErrorMessage == nil || *out[0].ErrorMessage != errMsg {
		t.Errorf("REST errored row ErrorMessage = %v, want %q", out[0].ErrorMessage, errMsg)
	}
	if out[1].ErrorMessage != nil {
		t.Errorf("REST successful row ErrorMessage = %v, want nil", *out[1].ErrorMessage)
	}
}

func TestGraphQLBlueprintSyncsIncludesErrorMessage(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	errMsg := "quota exceeded: workspace at service limit"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	fs.syncsToReturn = []store.BlueprintSync{
		{ID: "bsr-1", BlueprintID: "blp-1", State: store.BlueprintSyncStateError, ErrorMessage: &errMsg},
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	schema := blueprintSchema(t, svc)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ blueprintSyncs(id: "blp-1", ownerId: "tea-a") { id state errorMessage } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("blueprintSyncs query: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["blueprintSyncs"].([]any)
	if len(list) != 1 {
		t.Fatalf("blueprintSyncs query: want 1, got %d", len(list))
	}
	row := list[0].(map[string]any)
	if row["errorMessage"] != errMsg {
		t.Errorf("blueprintSyncs[0].errorMessage = %v, want %q", row["errorMessage"], errMsg)
	}
}

func TestMCPListBlueprintSyncsIncludesErrorMessage(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	errMsg := "quota exceeded: workspace at service limit"
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	fs.syncsToReturn = []store.BlueprintSync{
		{ID: "bsr-1", BlueprintID: "blp-1", State: store.BlueprintSyncStateError, ErrorMessage: &errMsg},
	}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithWorkspace(core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"}), "tea-a")
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_blueprint_syncs",
		Arguments: map[string]any{"id": "blp-1"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("list_blueprint_syncs tool error: %#v", res)
	}
	var out listBlueprintSyncsResult
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	if len(out.Syncs) != 1 {
		t.Fatalf("list_blueprint_syncs: want 1 sync, got %d", len(out.Syncs))
	}
	if out.Syncs[0].ErrorMessage == nil || *out.Syncs[0].ErrorMessage != errMsg {
		t.Errorf("MCP sync ErrorMessage = %v, want %q", out.Syncs[0].ErrorMessage, errMsg)
	}
}

// --- GraphQL adapter ---

func TestGraphQLValidateBlueprint(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	schema := blueprintSchema(t, svc)

	q := fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errors plan { mode services databases totalActions actions { operation kind name sourcePath } } } }`, stackManifest)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("validateBlueprint: %v", res.Errors)
	}
	v := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	if v["valid"] != true {
		t.Errorf("validateBlueprint valid manifest: want valid=true, got %v", v)
	}
	plan := v["plan"].(map[string]any)
	if plan["totalActions"] != 4 {
		t.Errorf("validateBlueprint plan = %+v, want totalActions=4", plan)
	}
	if plan["mode"] != "current_state" {
		t.Errorf("validateBlueprint plan = %+v, want mode=current_state", plan)
	}
	if actions, ok := plan["actions"].([]any); !ok || len(actions) != 4 {
		t.Errorf("validateBlueprint plan actions = %#v, want four actions", plan["actions"])
	}
}

func TestGraphQLValidateBlueprintPreservesStringErrorsAndAddsDetails(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	schema := blueprintSchema(t, svc)

	q := `{ validateBlueprint(bexYaml: "services:\n  - name: \"\"\n    type: web\n    runtime: image\n") { valid errors errorDetails { error path } } }`
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("validateBlueprint: %v", res.Errors)
	}
	v := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	if v["valid"] != false || len(v["errors"].([]any)) != 1 {
		t.Fatalf("validateBlueprint invalid result = %+v", v)
	}
	details := v["errorDetails"].([]any)
	if len(details) != 1 || details[0].(map[string]any)["path"] != "services[0].name" {
		t.Errorf("validateBlueprint errorDetails = %+v", details)
	}
}

func TestGraphQLListBlueprints(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	schema := blueprintSchema(t, svc)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ blueprints(ownerId: "tea-a") { id name repo branch status } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("blueprints query: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["blueprints"].([]any)
	if len(list) != 1 {
		t.Errorf("blueprints query: want 1, got %d", len(list))
	}
	b := list[0].(map[string]any)
	if b["id"] != "blp-1" {
		t.Errorf("blueprints[0].id = %v, want blp-1", b["id"])
	}
}

func TestGraphQLSyncBlueprint(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Path:     CanonicalBlueprintFilename,
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		DomainOwnership: allowDomainOwnership{},
		// w8/m36 t003: Git-backed sync fetches instead of reapplying stored.
		GitFetcher: fakeBlueprintFetcher{contents: stackManifest},
	}
	schema := blueprintSchema(t, svc)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { syncBlueprint(id: "blp-1", ownerId: "tea-a") { blueprint { id } services { id } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("syncBlueprint: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["syncBlueprint"].(map[string]any)
	bp := data["blueprint"].(map[string]any)
	if bp["id"] != "blp-1" {
		t.Errorf("syncBlueprint.blueprint.id = %v, want blp-1", bp["id"])
	}
}

// TestBlueprintAutoSyncPreservesTenantContext verifies that the auto-sync path
// called from webhooks preserves tenant context instead of dropping it with
// context.Background() (w9/001, the store-row half) — and, since w1/m69, that
// the binding reaches the CRs themselves: the apply pipeline runs as the
// blueprint row's tenant, so creates are tenant-labeled and tenant-namespaced,
// never unlabeled in the shared namespace (the round-15 scan's tenant-
// attribution break, which the w9/001 test never asserted).
func TestBlueprintAutoSyncPreservesTenantContext(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a", "user-b": "tea-b"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Path:     CanonicalBlueprintFilename,
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
		AutoSync: true,
	})
	cl := fakeClient()
	// w8/m36 t003: the webhook-triggered sync fetches the manifest through
	// the seam instead of reapplying the stored one.
	fetcher := fakeBlueprintFetcher{contents: stackManifest}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}, Blueprints: fs, DomainOwnership: allowDomainOwnership{}, GitFetcher: fetcher}

	tenantID := "tea-a"

	// Exactly what the webhook handler builds (webhook.go): workspace-NAMED but
	// identity-less — no core.Identity ever reaches the background sync.
	bgCtx := core.WithWorkspace(context.Background(), tenantID)

	// Store-row half (w9/001): the sync's store writes key to the named tenant.
	resolvedTenant := svc.resolveTenantID(bgCtx)
	if resolvedTenant != tenantID {
		t.Errorf("resolveTenantID(context with tenant) = %q, want %q", resolvedTenant, tenantID)
	}
	if bgTenant := svc.resolveTenantID(context.Background()); bgTenant != "" {
		t.Errorf("resolveTenantID(context.Background()) = %q, want empty string", bgTenant)
	}

	// CR half (w1/m69): the raw webhook ctx still resolves NO tenant — that is
	// precisely the gap — so triggerBlueprintSync binds the acting tenant from
	// the blueprint row before the apply runs (core.WithActingTenant; unit-
	// tested in core's workspace tests). The proof here is the created CRs:
	// they must be tenant-attributed, not the pre-fix unlabeled shared-ns
	// workload.

	svc.triggerBlueprintSync(bgCtx, tenantID, "https://github.com/a/app", "main")

	for _, name := range []string{"web", "api", "worker"} {
		var a appv1alpha1.App
		key := client.ObjectKey{Namespace: tenantID, Name: core.CRName(tenantID, name)}
		if err := cl.Get(context.Background(), key, &a); err != nil {
			t.Fatalf("auto-sync create %v: %v (must land tenant-labeled in the tenant namespace)", key, err)
		}
		if got := a.Labels[core.LabelTenant]; got != tenantID {
			t.Errorf("App %v: LabelTenant = %q, want %q", key, got, tenantID)
		}
		if got := a.Labels[core.LabelServiceName]; got != name {
			t.Errorf("App %v: LabelServiceName = %q, want %q", key, got, name)
		}
	}
	// No bare-named service may be left in the shared namespace — that is the
	// unattributed, unquota'd, unbilled workload the round-15 scan found.
	for _, name := range []string{"web", "api", "worker"} {
		var a appv1alpha1.App
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err == nil {
			t.Errorf("shared-namespace CR default/%s exists — auto-sync created an unattributed workload", name)
		}
	}
	// The manifest's database must be tenant-scoped too.
	var dbs appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &dbs, client.MatchingLabels{core.LabelTenant: tenantID}); err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if len(dbs.Items) != 1 {
		t.Fatalf("tenant-labeled databases = %d, want 1", len(dbs.Items))
	}
	if ns := dbs.Items[0].Namespace; ns != tenantID {
		t.Errorf("database namespace = %q, want %q", ns, tenantID)
	}
}

// foreignMatchManifest is the auto-sync manifest for the scoped-matching
// regression: one service and one database, both named to collide with
// pre-existing foreign resources in the shared namespace.
const foreignMatchManifest = `
services:
  - name: web
    type: web
    runtime: image
    image: {url: synced:latest}
databases:
  - name: db
    plan: basic-256mb
`

// TestBlueprintAutoSyncScopedMatchingIgnoresForeignResources pins the second
// half of the round-15 finding: an identity-less sync that resolved NO tenant
// used to match same-display-name datastores and bare-named Apps regardless of
// owner, merge-patching another tenant's (or a legacy unlabeled) resource.
// With the acting-tenant binding the sync resolves tea-a, so existing-resource
// resolution is tenant-scoped: the foreign CRs stay untouched and the sync
// creates its own.
func TestBlueprintAutoSyncScopedMatchingIgnoresForeignResources(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	bp := store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: foreignMatchManifest,
		Status: "active", Name: "app", AutoSync: true,
	}
	fs := newFakeBlueprintStore(bp)
	foreignApp := sampleApp("web") // bare-named, unlabeled, shared namespace
	foreignDB := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db-legacy", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "db", Plan: "basic-256mb", StorageGB: 5},
	}
	cl := fakeClient(foreignApp, foreignDB)
	// w8/m36 t003: the sync fetches the manifest through the seam.
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}, Blueprints: fs, DomainOwnership: allowDomainOwnership{}, GitFetcher: fakeBlueprintFetcher{contents: foreignMatchManifest}}

	// The ctx exactly as triggerBlueprintSync builds it (webhook-shaped
	// workspace-named ctx + the acting-tenant binding from the blueprint row).
	syncCtx := core.WithActingTenant(core.WithWorkspace(context.Background(), "tea-a"), "tea-a")
	if _, err := svc.runSync(syncCtx, bp, "", ""); err != nil {
		t.Fatalf("runSync: %v", err)
	}

	// The foreign bare-named App is nobody's — the sync must not adopt or patch it.
	var gotForeign appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &gotForeign); err != nil {
		t.Fatalf("get foreign app: %v", err)
	}
	if gotForeign.Spec.Image != foreignApp.Spec.Image {
		t.Errorf("foreign app image = %q, want untouched %q", gotForeign.Spec.Image, foreignApp.Spec.Image)
	}
	if _, labeled := gotForeign.Labels[core.LabelTenant]; labeled {
		t.Errorf("foreign app gained a tenant label — the sync adopted somebody else's resource")
	}

	// The foreign legacy Database keeps its spec and stays unlabeled.
	var gotLegacy appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "db-legacy"}, &gotLegacy); err != nil {
		t.Fatalf("get legacy database: %v", err)
	}
	if !reflect.DeepEqual(gotLegacy.Spec, foreignDB.Spec) {
		t.Errorf("legacy database spec changed: got %+v, want %+v", gotLegacy.Spec, foreignDB.Spec)
	}
	if _, labeled := gotLegacy.Labels[core.LabelTenant]; labeled {
		t.Errorf("legacy database gained a tenant label — the sync adopted somebody else's resource")
	}

	// The sync created its OWN tenant-attributed resources instead.
	var own appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: "tea-a-web"}, &own); err != nil {
		t.Fatalf("tenant's own app: %v", err)
	}
	if own.Spec.Image != "synced:latest" {
		t.Errorf("tenant's own app image = %q, want synced:latest", own.Spec.Image)
	}
	var dbs appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &dbs, client.MatchingLabels{core.LabelTenant: "tea-a"}); err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if len(dbs.Items) != 1 || dbs.Items[0].Name == "db-legacy" {
		t.Fatalf("tenant-labeled databases = %+v, want exactly one NEW (not db-legacy)", dbs.Items)
	}
}

// TestRunSyncFailsClosedWithoutResolvableTenant pins w1/m69's fail-closed
// guard: with the store on, a sync that cannot name its acting workspace
// refuses instead of applying identity-less — that path created unlabeled CRs
// in the shared namespace (round-15 scan).
func TestRunSyncFailsClosedWithoutResolvableTenant(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	bp := store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Manifest: stackManifest, Status: "active", Name: "app",
	}
	fs := newFakeBlueprintStore(bp)
	cl := fakeClient()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}, Blueprints: fs, DomainOwnership: allowDomainOwnership{}}

	if _, err := svc.runSync(context.Background(), bp, "", ""); !errors.Is(err, ErrBlueprintSyncWorkspaceUnresolved) {
		t.Fatalf("runSync(identity-less, workspace-less ctx) err = %v, want ErrBlueprintSyncWorkspaceUnresolved", err)
	}

	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("refused sync still wrote %d App CRs; want 0", len(apps.Items))
	}
}

// --- UpdateBlueprint path allowlist (codex-security round-6 #14) ---

// recordingBlueprintFetcher fails every fetch and records the paths asked for,
// proving a guard ran BEFORE the token-backed repository read. resolves counts
// commit resolutions, which are repository reads too.
type recordingBlueprintFetcher struct {
	paths    []string
	resolves int
}

func (f *recordingBlueprintFetcher) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	f.resolves++
	return "", fmt.Errorf("resolve should not have been reached")
}

func (f *recordingBlueprintFetcher) FetchBlueprintFileAtCommit(_ context.Context, _, _, _, filePath string) (string, error) {
	f.paths = append(f.paths, filePath)
	return "", fmt.Errorf("fetch should not have been reached")
}

// TestUpdateBlueprintEnforcesApprovedPath pins round-6 #14: the discovery-time
// YAML-bounded path rule must hold on the mutable update path too — otherwise
// a member with Blueprint create rights could point the stored path at any
// private-repository file and read it through the workspace installation token
// at the next sync.
func TestUpdateBlueprintEnforcesApprovedPath(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: CanonicalBlueprintFilename, Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	for _, bad := range []string{".env", "Makefile", "../render.yaml", "./render.yaml", `deploy\render.yaml`} {
		p := bad
		if _, err := svc.UpdateBlueprint(ctx, "blp-1", "tea-a", UpdateBlueprintRequest{Path: &p}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("UpdateBlueprint path %q: want ErrBadRequest, got %v", bad, err)
		}
	}
	if stored := fs.blueprints["blp-1"]; stored.Path != CanonicalBlueprintFilename {
		t.Fatalf("rejected update mutated the stored path: %q", stored.Path)
	}

	good := "deploy/render.yaml"
	if _, err := svc.UpdateBlueprint(ctx, "blp-1", "tea-a", UpdateBlueprintRequest{Path: &good}); err != nil {
		t.Fatalf("UpdateBlueprint approved nested path: %v", err)
	}
	if stored := fs.blueprints["blp-1"]; stored.Path != good {
		t.Errorf("approved path not stored: %q, want %q", stored.Path, good)
	}
}

// TestSyncBlueprintRefusesLegacyUnapprovedPath pins the sync-side guard: a row
// whose path predates the update-time allowlist (or was written by any path
// that forgot it) must be refused BEFORE the installation-token fetch.
func TestSyncBlueprintRefusesLegacyUnapprovedPath(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Path: ".env", Manifest: stackManifest,
		Status: "active", Name: "app",
	})
	fetcher := &recordingBlueprintFetcher{}
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints: fs,
		GitFetcher: fetcher,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if _, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("legacy unapproved path sync: want ErrBadRequest, got %v", err)
	}
	if len(fetcher.paths) != 0 {
		t.Errorf("sync fetched %v before validating the stored path", fetcher.paths)
	}
	if fetcher.resolves != 0 {
		t.Errorf("sync resolved the branch %d times before validating the stored path", fetcher.resolves)
	}
}

// TestValidateBlueprintStaticBuildCommand covers the w8/m19 t003 promotion:
// Render's near-universal static-site shape (runtime static + buildCommand +
// staticPublishPath) validates, and the projected App spec routes the build
// through the native static path so the declared command actually runs —
// builder native, runtime left empty (the build plane defaults the toolchain),
// no start command.
func TestValidateBlueprintStaticBuildCommand(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `services:
  - name: site
    type: web
    runtime: static
    repo: https://github.com/bex/site
    buildCommand: npm run build
    staticPublishPath: dist
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid {
		t.Fatalf("static buildCommand must validate: validation=%+v err=%v", v, err)
	}

	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, err := specFromCreate(st.services[0].req)
	if err != nil {
		t.Fatalf("specFromCreate: %v", err)
	}
	if spec.Type != appv1alpha1.TypeStaticSite || spec.BuildCommand != "npm run build" ||
		spec.Builder != "native" || spec.Runtime != "" || spec.PublishPath != "dist" || spec.StartCommand != "" {
		t.Errorf("static spec projection = type %q build %q builder %q runtime %q publish %q start %q",
			spec.Type, spec.BuildCommand, spec.Builder, spec.Runtime, spec.PublishPath, spec.StartCommand)
	}
}

// TestValidateBlueprintDockerContext covers the w8/m19 t004 promotion: the
// monorepo shape (dockerContext independent of rootDir) validates on server
// and cron services and threads to App.spec.dockerContext; traversal inputs
// are rejected at the exact field with the create boundary's error.
func TestValidateBlueprintDockerContext(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `services:
  - name: api
    type: web
    runtime: docker
    repo: https://github.com/bex/mono
    rootDir: apps/api
    dockerfilePath: Dockerfile
    dockerContext: apps/api/ctx
  - name: nightly
    type: cron
    runtime: docker
    repo: https://github.com/bex/mono
    dockerfilePath: Dockerfile
    dockerContext: apps/nightly
    schedule: "0 0 * * *"
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid {
		t.Fatalf("dockerContext must validate: validation=%+v err=%v", v, err)
	}

	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i, want := range []string{"apps/api/ctx", "apps/nightly"} {
		spec, err := specFromCreate(st.services[i].req)
		if err != nil {
			t.Fatalf("specFromCreate[%d]: %v", i, err)
		}
		if spec.DockerContext != want {
			t.Errorf("service %d DockerContext = %q, want %q", i, spec.DockerContext, want)
		}
	}

	escape := strings.Replace(manifest, "apps/api/ctx", "../escape", 1)
	v, err = svc.ValidateBlueprint(context.Background(), "", escape)
	if err != nil {
		t.Fatalf("ValidateBlueprint(escape): %v", err)
	}
	if v.Valid || len(v.Errors) == 0 || !strings.Contains(v.Errors[0].Error, "dockerContext") {
		t.Errorf("traversal dockerContext must be rejected by name, got %+v", v)
	}
}

// TestValidateBlueprintRegistryCredential covers the w8/m19 t005 promotion:
// render.yaml's by-name workspace registry-credential references — the
// service-level registryCredential and image.creds forms — resolve to the
// stored credential's id and bind App.spec.registryCredentialId; an unknown
// or self-contradictory reference fails before any write.
func TestValidateBlueprintRegistryCredential(t *testing.T) {
	svc := &Service{
		Base:          &core.Base{Client: fakeClient(), Namespace: "default"},
		RegistryCreds: &fakePullSecrets{credentialIDsByName: map[string]string{"acme-registry": "rgc-abc123"}},
	}
	manifest := `services:
  - name: api
    type: web
    runtime: image
    image:
      url: ghcr.io/acme/api:1
      creds: {fromRegistryCreds: {name: acme-registry}}
  - name: worker
    type: worker
    runtime: image
    image: {url: ghcr.io/acme/worker:1}
    registryCredential: {fromRegistryCreds: {name: acme-registry}}
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !v.Valid {
		t.Fatalf("registry credentials must validate: validation=%+v err=%v", v, err)
	}

	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := svc.resolveBlueprintRegistryCredentials(context.Background(), &st); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for i := range st.services {
		got := st.services[i].req.RegistryCredentialID
		if got == nil || *got != "rgc-abc123" {
			t.Errorf("service %d RegistryCredentialID = %v, want rgc-abc123", i, got)
		}
	}

	unknown := strings.ReplaceAll(manifest, "acme-registry", "no-such-credential")
	v, err = svc.ValidateBlueprint(context.Background(), "", unknown)
	if err != nil {
		t.Fatalf("ValidateBlueprint(unknown): %v", err)
	}
	if v.Valid || len(v.Errors) == 0 || !strings.Contains(v.Errors[0].Error, "no-such-credential") {
		t.Errorf("unknown credential name must fail by name, got %+v", v)
	}

	conflicting := `services:
  - name: api
    type: web
    runtime: image
    image:
      url: ghcr.io/acme/api:1
      creds: {fromRegistryCreds: {name: acme-registry}}
    registryCredential: {fromRegistryCreds: {name: other-registry}}
`
	v, err = svc.ValidateBlueprint(context.Background(), "", conflicting)
	if err != nil {
		t.Fatalf("ValidateBlueprint(conflicting): %v", err)
	}
	if v.Valid || len(v.Errors) == 0 || !strings.Contains(v.Errors[0].Error, "different registry credentials") {
		t.Errorf("conflicting references must fail, got %+v", v)
	}
}

// TestApprovedBlueprintPathCustomFilenames pins the w8/m19 t006 relaxation
// (Render's 2026-02-09 custom Blueprint paths): any clean repo-relative
// .yaml/.yml path is accepted; containment rejections and the implicit
// discovery defaults are unchanged.
func TestApprovedBlueprintPathCustomFilenames(t *testing.T) {
	for _, ok := range []string{
		"render.yaml", "bex.yml", "infra/bex/stack.yaml", "deploy/bex-beancount.yaml", "one.yml",
	} {
		got, err := approvedBlueprintPath(ok)
		if err != nil || got != ok {
			t.Errorf("approvedBlueprintPath(%q) = %q, %v; want accepted", ok, got, err)
		}
	}
	for _, bad := range []string{
		"", ".", "../escape.yaml", "/abs/render.yaml", "infra/../../up.yaml",
		"stack.json", "render.yaml.txt", `win\path.yaml`, "dir/",
	} {
		if _, err := approvedBlueprintPath(bad); err == nil {
			t.Errorf("approvedBlueprintPath(%q) must be rejected", bad)
		}
	}
}

// TestBlueprintByIDNotFoundIsIdenticalAcrossSurfaces is w6/m96's parity check.
// REST, GraphQL and MCP all call the same by-id verbs, so a missing blueprint
// must read the same on all three. Before this milestone every one of them
// returned the raw store error, which no surface classifies — REST answered
// `500 internal error`, and the other two answered their equivalent generic
// failure — so the id that does not exist and the id the caller may not see
// were indistinguishable from a genuine server fault. The REST validator fix is
// what made this reachable at all; without the mapping, "no longer 400" would
// merely have become "500 instead".
func TestBlueprintByIDNotFoundIsIdenticalAcrossSurfaces(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithWorkspace(core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"}), "tea-a")

	// The service layer all three share.
	for name, call := range map[string]func() error{
		"GetBlueprintByID": func() error {
			_, err := svc.GetBlueprintByID(ctx, "blp-absent", "tea-a")
			return err
		},
		"ListBlueprintSyncs": func() error {
			_, err := svc.ListBlueprintSyncs(ctx, "blp-absent", "tea-a", "", 0)
			return err
		},
		"UpdateBlueprint": func() error {
			// A real field: an empty patch is rejected as a bad request before
			// the lookup ever runs, which would prove nothing here.
			name := "renamed"
			_, err := svc.UpdateBlueprint(ctx, "blp-absent", "tea-a", UpdateBlueprintRequest{Name: &name})
			return err
		},
		"DisconnectBlueprint": func() error {
			return svc.DisconnectBlueprint(ctx, "blp-absent", "tea-a")
		},
		"SyncBlueprint": func() error {
			_, err := svc.SyncBlueprint(ctx, "blp-absent", "tea-a", "", "")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if !errors.Is(err, core.ErrNotFound) {
				t.Errorf("%s on a missing id = %v, want core.ErrNotFound — anything else surfaces as a 500", name, err)
			}
			// A classified error keeps its message on every surface; an
			// unclassified one is redacted to "internal error", which is exactly
			// what made this indistinguishable from a server fault.
			if !core.IsPublicError(err) {
				t.Errorf("%s error %v is redacted rather than reported as not-found", name, err)
			}
		})
	}

	// REST: 404 with a body, not 500.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	restReq := httptest.NewRequest("GET", "/v1/blueprints/blp-absent?ownerId=tea-a", nil)
	restReq = restReq.WithContext(ctx)
	restRec := httptest.NewRecorder()
	mux.ServeHTTP(restRec, restReq)
	if restRec.Code != http.StatusNotFound {
		t.Errorf("REST GET missing blueprint = %d %s, want 404", restRec.Code, restRec.Body)
	}

	// GraphQL: an error naming not-found, not a redacted internal failure.
	schema := blueprintSchema(t, svc)
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ blueprint(id: "blp-absent", ownerId: "tea-a") { id } }`})
	if len(res.Errors) != 1 {
		t.Fatalf("GraphQL blueprint(missing) errors = %v, want exactly one", res.Errors)
	}
	if strings.Contains(res.Errors[0].Message, "internal error") {
		t.Errorf("GraphQL error = %q, want the not-found message rather than a redacted internal error", res.Errors[0].Message)
	}

	// MCP: a tool error carrying the same classification.
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	toolRes, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_blueprint",
		Arguments: map[string]any{"id": "blp-absent"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !toolRes.IsError {
		t.Fatalf("get_blueprint on a missing id succeeded: %#v", toolRes)
	}

	// Control: the blueprint that DOES exist still reads identically on all
	// three, so the mapping above did not turn a working read into a failure.
	if _, err := svc.GetBlueprintByID(ctx, "blp-1", "tea-a"); err != nil {
		t.Errorf("GetBlueprintByID on the real row = %v, want success", err)
	}
}
