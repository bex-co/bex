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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type patchCountingClient struct {
	client.Client
	patches         int
	fail            error
	secretGets      int
	beforeSecretGet func()
}

type versionedFakeSecretStore struct {
	*fakeSecretStore
	versions    map[string]uint64
	afterCAS    func(path string, version uint64)
	afterGet    func() // fires once, after the NEXT GetVersioned captures its snapshot but before returning — simulates a concurrent writer landing in the read/write gap a CAS retry loop must survive
	reads       []string
	casCalls    int
	failCASCall int
}

type versionedTenantFakeSecretStore struct {
	*tenantFakeSecretStore
	versions map[string]uint64
}

func newVersionedTenantFakeSecretStore() *versionedTenantFakeSecretStore {
	return &versionedTenantFakeSecretStore{tenantFakeSecretStore: newTenantFakeSecretStore(), versions: map[string]uint64{}}
}

func (f *versionedTenantFakeSecretStore) GetVersioned(ctx context.Context, path string) (core.SecretKVSnapshot, error) {
	data, err := f.Get(ctx, path)
	return core.SecretKVSnapshot{Data: data, Version: f.versions[f.key(ctx, path)]}, err
}

func (f *versionedTenantFakeSecretStore) PutCAS(ctx context.Context, path string, data map[string]string, expectedVersion uint64) (uint64, error) {
	key := f.key(ctx, path)
	if f.versions[key] != expectedVersion {
		return 0, core.ErrConflict
	}
	if err := f.Put(ctx, path, data); err != nil {
		return 0, err
	}
	f.versions[key]++
	return f.versions[key], nil
}

func newVersionedFakeSecretStore() *versionedFakeSecretStore {
	return &versionedFakeSecretStore{
		fakeSecretStore: newFakeSecretStore(),
		versions:        map[string]uint64{},
	}
}

func (f *versionedFakeSecretStore) GetVersioned(ctx context.Context, path string) (core.SecretKVSnapshot, error) {
	data, err := f.Get(ctx, path)
	version := f.versions[path]
	if hook := f.afterGet; hook != nil {
		f.afterGet = nil
		hook()
	}
	return core.SecretKVSnapshot{Data: data, Version: version}, err
}

func (f *versionedFakeSecretStore) Get(ctx context.Context, path string) (map[string]string, error) {
	f.reads = append(f.reads, path)
	return f.fakeSecretStore.Get(ctx, path)
}

func (f *versionedFakeSecretStore) PutCAS(ctx context.Context, path string, data map[string]string, expectedVersion uint64) (uint64, error) {
	f.casCalls++
	if f.failCASCall == f.casCalls {
		return 0, errors.New("injected /openbao/private/path failed-writer-secret")
	}
	if f.versions[path] != expectedVersion {
		return 0, core.ErrConflict
	}
	if err := f.Put(ctx, path, data); err != nil {
		return 0, err
	}
	f.versions[path]++
	version := f.versions[path]
	if hook := f.afterCAS; hook != nil {
		f.afterCAS = nil
		hook(path, version)
	}
	return version, nil
}

func (c *patchCountingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	if c.fail != nil {
		return c.fail
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *patchCountingClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.Secret); ok && key.Name == "web-env" {
		c.secretGets++
		if c.secretGets == 2 && c.beforeSecretGet != nil {
			hook := c.beforeSecretGet
			c.beforeSecretGet = nil
			hook()
		}
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func TestPatchEnvironmentMixedSaveOnlyPreservesOmittedSecrets(t *testing.T) {
	store := newFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"KEEP": "unchanged-secret", "RENAME_ME": "rename-secret", "DROP": "remove-me"}
	store.m[filesPath("web")] = map[string]string{"keep.pem": "unchanged-file", "rename.pem": "rename-file", "drop.pem": "remove-file"}
	svc := newService(store, sampleApp("web"))

	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeOnly,
		EnvVars: []EnvVarPatch{
			{Key: "DROP", Delete: true},
			{Key: "RENAMED", FromKey: "RENAME_ME"},
			{Key: "ADDED", Value: "new-secret"},
			{Key: "GENERATED", GenerateValue: true},
		},
		SecretFiles: []SecretFilePatch{
			{Name: "drop.pem", Delete: true},
			{Name: "renamed.pem", FromName: "rename.pem"},
			{Name: "added.json", Content: "new-file"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RolledOut {
		t.Fatal("save_only reported a rollout")
	}
	if !slices.Equal(result.EnvVarKeys, []string{"ADDED", "GENERATED", "KEEP", "RENAMED"}) ||
		!slices.Equal(result.SecretFileNames, []string{"added.json", "keep.pem", "renamed.pem"}) {
		t.Fatalf("names-only result = %#v", result)
	}
	if store.m[envPath("web")]["KEEP"] != "unchanged-secret" || store.m[filesPath("web")]["keep.pem"] != "unchanged-file" {
		t.Fatalf("omitted secrets were not preserved: env=%#v files=%#v", store.m[envPath("web")], store.m[filesPath("web")])
	}
	if store.m[envPath("web")]["RENAMED"] != "rename-secret" || store.m[filesPath("web")]["renamed.pem"] != "rename-file" {
		t.Fatalf("opaque rename lost secret material: env=%#v files=%#v", store.m[envPath("web")], store.m[filesPath("web")])
	}
	if len(store.m[envPath("web")]["GENERATED"]) != 44 {
		t.Fatalf("generated secret length = %d", len(store.m[envPath("web")]["GENERATED"]))
	}
	app := getApp(t, svc.Client, "web")
	if app.Spec.RestartedAt != "" {
		t.Fatalf("save_only changed restartedAt to %q", app.Spec.RestartedAt)
	}
	if app.Spec.EnvFromSecret != "" || len(app.Spec.FilesFromSecrets) != 0 {
		t.Fatalf("save_only changed rollout-bearing spec references: %#v", app.Spec)
	}
	if app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] != "web-env" ||
		app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] != "web-files" {
		t.Fatalf("save_only pending projections = %#v", app.Annotations)
	}
	if string(getSecret(t, svc.Client, "web-env").Data["KEEP"]) != "unchanged-secret" ||
		string(getSecret(t, svc.Client, "web-files").Data["keep.pem"]) != "unchanged-file" {
		t.Fatal("projected Secrets lost omitted material")
	}
	encoded, _ := json.Marshal(result)
	for _, secret := range []string{"unchanged-secret", "new-secret", "unchanged-file", "new-file"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("batch result leaked %q: %s", secret, encoded)
		}
	}
}

func TestPatchEnvironmentDeployActivatesAllPendingSaveOnlyProjections(t *testing.T) {
	store := newFakeSecretStore()
	baseClient := fakeClient(sampleApp("web"))
	counting := &patchCountingClient{Client: baseClient}
	svc := &Service{Base: &core.Base{Client: counting, Namespace: "default", Clock: fixedNow}, Store: store}

	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode:    SaveModeOnly,
		EnvVars:     []EnvVarPatch{{Key: "A", Value: "one"}},
		SecretFiles: []SecretFilePatch{{Name: "token", Content: "two"}},
	}); err != nil {
		t.Fatal(err)
	}
	staged := getApp(t, counting, "web")
	if staged.Spec.EnvFromSecret != "" || len(staged.Spec.FilesFromSecrets) != 0 || counting.patches != 1 {
		t.Fatalf("save-only App = %#v, patches=%d", staged.Spec, counting.patches)
	}

	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars:  []EnvVarPatch{{Key: "A", Value: "updated"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	active := getApp(t, counting, "web")
	if !result.RolledOut || counting.patches != 2 || active.Spec.RestartedAt == "" ||
		active.Spec.EnvFromSecret != "web-env" || !slices.Contains(active.Spec.FilesFromSecrets, "web-files") {
		t.Fatalf("deploy did not activate pending projections once: result=%#v App=%#v patches=%d", result, active.Spec, counting.patches)
	}
	if active.Annotations[appv1alpha1.PendingEnvSecretAnnotation] != "" ||
		active.Annotations[appv1alpha1.PendingFilesSecretAnnotation] != "" {
		t.Fatalf("pending projection annotations survived deploy: %#v", active.Annotations)
	}
}

func TestPatchEnvironmentDeployRollsExactlyOnceAndNoopDoesNotRoll(t *testing.T) {
	store := newFakeSecretStore()
	baseClient := fakeClient(sampleApp("web"))
	counting := &patchCountingClient{Client: baseClient}
	clockCalls := 0
	svc := &Service{Base: &core.Base{
		Client: counting, Namespace: "default",
		Clock: func() time.Time { clockCalls++; return time.Unix(int64(clockCalls), 0).UTC() },
	}, Store: store}

	result, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars:  []EnvVarPatch{{Key: "A", Value: "one"}},
		SecretFiles: []SecretFilePatch{
			{Name: "token", Content: "two"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RolledOut || counting.patches != 1 {
		t.Fatalf("deploy = rolledOut %v, App patches %d; want true/1", result.RolledOut, counting.patches)
	}
	restartedAt := getApp(t, counting, "web").Spec.RestartedAt
	if restartedAt == "" {
		t.Fatal("deploy did not stamp restartedAt")
	}

	result, err = svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy,
		EnvVars:  []EnvVarPatch{{Key: "A", Value: "one"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RolledOut || counting.patches != 1 || getApp(t, counting, "web").Spec.RestartedAt != restartedAt {
		t.Fatalf("no-op rolled or patched: result=%#v patches=%d clocks=%d", result, counting.patches, clockCalls)
	}
}

func TestPatchEnvironmentValidatesBeforeWritingAndDoesNotLeakValues(t *testing.T) {
	store := newFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"KEEP": "original"}
	svc := newService(store, sampleApp("web"))
	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeOnly,
		EnvVars: []EnvVarPatch{
			{Key: "A", Value: "must-not-leak"},
			{Key: "A", Delete: true},
		},
	})
	if !errors.Is(err, core.ErrBadRequest) || strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("duplicate error = %v", err)
	}
	if len(store.m) != 1 || store.m[envPath("web")]["KEEP"] != "original" {
		t.Fatalf("validation mutated store: %#v", store.m)
	}
	if _, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{SaveMode: "later"}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid save mode = %v", err)
	}
}

func TestEnvVarRevisionIsCoherentAndMasked(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"A": "one-secret", "B": "two-secret"}
	svc := newService(store, sampleApp("web"))

	keys, err := svc.EnvVarKeys(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].Value != "" || keys[1].Value != "" || keys[0].Revision == "" || keys[0].Revision != keys[1].Revision {
		t.Fatalf("masked keys did not share one revision: %#v", keys)
	}
	if len(store.reads) != 1 || store.reads[0] != envPath("web") {
		t.Fatalf("versioned current-tenant read was not atomic/single: %v", store.reads)
	}
	revealed, err := svc.EnvVarValue(context.Background(), "web", "A")
	if err != nil {
		t.Fatal(err)
	}
	if revealed.Value != "one-secret" || revealed.Revision != keys[0].Revision {
		t.Fatalf("reveal did not use the coherent map revision: %#v", revealed)
	}
	views, err := svc.ListEnvVars(context.Background(), "web")
	if err != nil || len(views) != 2 || views[0].Revision != keys[0].Revision || views[1].Revision != keys[0].Revision {
		t.Fatalf("REST/MCP list revisions = %#v, %v", views, err)
	}
	view, err := svc.GetEnvVar(context.Background(), "web", "A")
	if err != nil || view.Revision != keys[0].Revision || view.Value != "one-secret" {
		t.Fatalf("REST/MCP reveal revision = %#v, %v", view, err)
	}
	encoded, _ := json.Marshal(keys)
	if strings.Contains(string(encoded), "one-secret") || strings.Contains(string(encoded), "two-secret") {
		t.Fatalf("masked list leaked values: %s", encoded)
	}
}

func TestVersionedEnvReadRefusesAmbiguousLegacyAndSanitizesStoreFailures(t *testing.T) {
	path := envPath("web")
	store := newVersionedTenantFakeSecretStore()
	legacyCtx := withTenant(context.Background(), baoTenant)
	if err := store.Put(legacyCtx, path, map[string]string{"TOKEN": "legacy-secret"}); err != nil {
		t.Fatal(err)
	}
	svc := newService(store, tenantApp("web", "tea-a"))
	keys, err := svc.EnvVarKeys(context.Background(), "web")
	if err != nil || len(keys) != 0 {
		t.Fatalf("versioned tenant read exposed legacy data = %#v, %v", keys, err)
	}
	if _, exists := store.m["tea-a/"+path]; exists {
		t.Fatalf("tenant path was populated from ambiguous legacy data: %#v", store.m)
	}
	if store.m[baoTenant+"/"+path]["TOKEN"] != "legacy-secret" {
		t.Fatalf("legacy tenant data must remain for explicit migration: %#v", store.m)
	}

	leaky := newVersionedFakeSecretStore()
	leaky.failGet = errors.New("GET https://bao.invalid/v1/secret/data/tenants/tea-private/services/web/env?token=revision-secret")
	_, err = newService(leaky, sampleApp("web")).EnvVarKeys(context.Background(), "web")
	if !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Fatalf("versioned read error = %v", err)
	}
	for _, material := range []string{"https://bao.invalid", "tea-private", "services/web/env", "revision-secret"} {
		if strings.Contains(err.Error(), material) {
			t.Fatalf("versioned read leaked %q: %v", material, err)
		}
	}
}

func TestPatchEnvironmentCASAllowsOneWinnerAndDoesNotLeakConflict(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "old-secret"}
	baseClient := fakeClient(sampleApp("web"))
	counting := &patchCountingClient{Client: baseClient}
	svc := &Service{Base: &core.Base{Client: counting, Namespace: "default", Clock: fixedNow}, Store: store}
	observed, err := svc.EnvVarValue(context.Background(), "web", "TOKEN")
	if err != nil {
		t.Fatal(err)
	}

	winner := EnvironmentPatch{
		SaveMode:            SaveModeDeploy,
		EnvVars:             []EnvVarPatch{{Key: "TOKEN", Value: "winner-secret"}},
		ExpectedEnvRevision: &observed.Revision,
	}
	result, err := svc.PatchEnvironment(context.Background(), "web", winner)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RolledOut || counting.patches != 1 || store.m[envPath("web")]["TOKEN"] != "winner-secret" {
		t.Fatalf("winner result=%#v patches=%d store=%#v", result, counting.patches, store.m)
	}
	projection := getSecret(t, counting, "web-env")
	if projection.Annotations[envProjectionRevisionAnnotation] != encodeEnvRevision(1) {
		t.Fatalf("projection owner annotation = %#v", projection.Annotations)
	}
	for _, path := range store.reads {
		if path == filesPath("web") {
			t.Fatalf("CAS path read secret files: %v", store.reads)
		}
	}
	restartedAt := getApp(t, counting, "web").Spec.RestartedAt

	loser := winner
	loser.EnvVars = []EnvVarPatch{{Key: "TOKEN", Value: "loser-secret"}}
	_, err = svc.PatchEnvironment(context.Background(), "web", loser)
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_REVISION_CONFLICT" {
		t.Fatalf("stale save error = %#v", err)
	}
	for _, material := range []string{"TOKEN", "old-secret", "winner-secret", "loser-secret", observed.Revision} {
		if strings.Contains(err.Error(), material) {
			t.Fatalf("conflict leaked %q: %v", material, err)
		}
	}
	if store.m[envPath("web")]["TOKEN"] != "winner-secret" || counting.patches != 1 || getApp(t, counting, "web").Spec.RestartedAt != restartedAt {
		t.Fatalf("stale save changed winner: store=%#v patches=%d", store.m, counting.patches)
	}
}

func TestPatchEnvironmentCASRejectsWideOrInvalidWrites(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "old-secret"}
	svc := newService(store, sampleApp("web"))
	revision := encodeEnvRevision(0)
	tests := []EnvironmentPatch{
		{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision},
		{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "A", Value: "one"}, {Key: "B", Value: "two"}}},
		{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "A", Value: "one"}}, SecretFiles: []SecretFilePatch{{Name: "x", Content: "secret"}}},
		{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "A", FromKey: "TOKEN"}}},
		{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "TOKEN", Delete: true}}},
		{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "TOKEN", GenerateValue: true}}},
	}
	for i, patch := range tests {
		_, err := svc.PatchEnvironment(context.Background(), "web", patch)
		var coded *core.CodedError
		if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != "INVALID_ENVIRONMENT_CAS_PATCH" {
			t.Fatalf("case %d error = %#v", i, err)
		}
	}
	malformed := "not-a-revision-secret"
	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeOnly, ExpectedEnvRevision: &malformed,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "must-not-write"}},
	})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_REVISION_INVALID" || strings.Contains(err.Error(), malformed) {
		t.Fatalf("malformed revision error = %#v", err)
	}
	if store.m[envPath("web")]["TOKEN"] != "old-secret" || store.versions[envPath("web")] != 0 {
		t.Fatalf("invalid writes mutated store: data=%#v version=%d", store.m, store.versions[envPath("web")])
	}

	_, err = svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "MISSING", Value: "must-not-create"}},
	})
	if !errors.Is(err, core.ErrNotFound) || !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_NOT_FOUND" {
		t.Fatalf("missing-key CAS error = %#v", err)
	}
	_, invalidErr := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "not valid", Value: "must-not-write"}},
	})
	if !errors.Is(invalidErr, core.ErrBadRequest) || !errors.As(invalidErr, &coded) || coded.Code != "ENVIRONMENT_VARIABLE_INVALID" {
		t.Fatalf("invalid-key CAS error = %#v", invalidErr)
	}
	if store.versions[envPath("web")] != 0 || store.m[envPath("web")]["TOKEN"] != "old-secret" {
		t.Fatalf("missing/invalid key mutated source: data=%#v version=%d", store.m, store.versions[envPath("web")])
	}
	var projection corev1.Secret
	if projectionErr := svc.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-env"}, &projection); !apierrors.IsNotFound(projectionErr) {
		t.Fatalf("missing/invalid key wrote projection: %v", projectionErr)
	}

	legacy := newService(newFakeSecretStore(), sampleApp("legacy"))
	_, err = legacy.PatchEnvironment(context.Background(), "legacy", EnvironmentPatch{
		SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "x"}},
	})
	if !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Fatalf("legacy CAS error = %v", err)
	}
}

func TestPatchEnvironmentCASCompensationDoesNotOverwriteNewerWinner(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
	revision := encodeEnvRevision(0)
	store.afterCAS = func(path string, version uint64) {
		if _, err := store.PutCAS(context.Background(), path, map[string]string{"TOKEN": "concurrent-winner"}, version); err != nil {
			t.Fatalf("inject concurrent winner: %v", err)
		}
	}
	failing := &patchCountingClient{Client: fakeClient(sampleApp("web")), fail: errors.New("injected App patch failure")}
	svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}

	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "failed-writer-secret"}},
	})
	if !errors.Is(err, core.ErrConflict) || store.m[envPath("web")]["TOKEN"] != "concurrent-winner" {
		t.Fatalf("compensation overwrote winner: err=%v store=%#v", err, store.m)
	}
	for _, material := range []string{"TOKEN", "before-secret", "failed-writer-secret", "concurrent-winner", revision} {
		if strings.Contains(err.Error(), material) {
			t.Fatalf("compensation error leaked %q: %v", material, err)
		}
	}
}

func TestPatchEnvironmentCASCompensationRestoresOwnedCreateAndUpdate(t *testing.T) {
	for _, existingProjection := range []bool{false, true} {
		name := "created"
		if existingProjection {
			name = "updated"
		}
		t.Run(name, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
			revision := encodeEnvRevision(0)
			app := sampleApp("web")
			objects := []client.Object{app}
			if existingProjection {
				app.Spec.EnvFromSecret = "web-env"
				objects = append(objects, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "web-env", Namespace: "default", UID: "old-uid", Annotations: map[string]string{envProjectionRevisionAnnotation: revision}},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{"TOKEN": []byte("before-secret")},
				})
			}
			failing := &patchCountingClient{Client: fakeClient(objects...), fail: errors.New("injected /sensitive/request/path")}
			svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}

			_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
				SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
				EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "failed-writer-secret"}},
			})
			var coded *core.CodedError
			if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_UPDATE_RESTORED" {
				t.Fatalf("compensated error = %v", err)
			}
			if store.m[envPath("web")]["TOKEN"] != "before-secret" || store.versions[envPath("web")] != 2 {
				t.Fatalf("source was not restored: data=%#v version=%d", store.m, store.versions[envPath("web")])
			}
			var projection corev1.Secret
			getErr := failing.Client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-env"}, &projection)
			if !existingProjection {
				if !apierrors.IsNotFound(getErr) {
					t.Fatalf("owned create was not deleted: %v", getErr)
				}
				return
			}
			if getErr != nil || string(projection.Data["TOKEN"]) != "before-secret" || projection.Annotations[envProjectionRevisionAnnotation] != encodeEnvRevision(2) {
				t.Fatalf("owned update was not restored: err=%v data=%q annotations=%#v", getErr, projection.Data["TOKEN"], projection.Annotations)
			}
		})
	}
}

func TestPatchEnvironmentCASCompensationFailureIsCodedAndRedacted(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
	store.failCASCall = 2 // initial write succeeds; source restoration fails
	revision := encodeEnvRevision(0)
	failing := &patchCountingClient{Client: fakeClient(sampleApp("web")), fail: errors.New("injected App /private/path failure")}
	svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}

	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "failed-writer-secret"}},
	})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_RESTORATION_FAILED" {
		t.Fatalf("restoration failure = %#v", err)
	}
	for _, material := range []string{"TOKEN", "before-secret", "failed-writer-secret", "/openbao/private/path", "/private/path", revision} {
		if strings.Contains(err.Error(), material) {
			t.Fatalf("restoration failure leaked %q: %v", material, err)
		}
	}
}

func TestPatchEnvironmentCASRollbackDoesNotClobberProjectionLandedAfterSourceRestore(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
	revision := encodeEnvRevision(0)
	app := sampleApp("web")
	app.Spec.EnvFromSecret = "web-env"
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "web-env",
			Namespace:   "default",
			UID:         "original-projection-uid",
			Annotations: map[string]string{envProjectionRevisionAnnotation: revision},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"TOKEN": []byte("before-secret")},
	}
	baseClient := fakeClient(app, existing)
	failing := &patchCountingClient{Client: baseClient, fail: errors.New("injected App patch failure")}
	failing.beforeSecretGet = func() {
		restoredVersion := store.versions[envPath("web")]
		winnerVersion, err := store.PutCAS(context.Background(), envPath("web"), map[string]string{"TOKEN": "concurrent-winner"}, restoredVersion)
		if err != nil {
			t.Fatalf("write newer source: %v", err)
		}
		var current corev1.Secret
		if err := baseClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-env"}, &current); err != nil {
			t.Fatalf("get projection for newer writer: %v", err)
		}
		current.Data = map[string][]byte{"TOKEN": []byte("concurrent-winner")}
		current.Annotations[envProjectionRevisionAnnotation] = encodeEnvRevision(winnerVersion)
		if err := baseClient.Update(context.Background(), &current); err != nil {
			t.Fatalf("land newer projection: %v", err)
		}
	}
	svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}

	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode: SaveModeDeploy, ExpectedEnvRevision: &revision,
		EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "failed-writer-secret"}},
	})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_REVISION_CONFLICT" {
		t.Fatalf("rollback ownership error = %#v", err)
	}
	if store.m[envPath("web")]["TOKEN"] != "concurrent-winner" {
		t.Fatalf("source winner overwritten: %#v", store.m)
	}
	projection := getSecret(t, baseClient, "web-env")
	if string(projection.Data["TOKEN"]) != "concurrent-winner" || projection.Annotations[envProjectionRevisionAnnotation] != encodeEnvRevision(3) {
		t.Fatalf("projection winner overwritten: data=%q annotations=%#v", projection.Data["TOKEN"], projection.Annotations)
	}
	for _, material := range []string{"TOKEN", "before-secret", "failed-writer-secret", "concurrent-winner", revision} {
		if strings.Contains(err.Error(), material) {
			t.Fatalf("rollback error leaked %q: %v", material, err)
		}
	}
}

func TestPatchEnvironmentCompensatesStoreAndProjectionsWhenAppPatchFails(t *testing.T) {
	store := newFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"OLD": "env-before"}
	store.m[filesPath("web")] = map[string]string{"old.pem": "file-before"}
	failing := &patchCountingClient{Client: fakeClient(sampleApp("web")), fail: errors.New("injected App patch failure")}
	svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}

	_, err := svc.PatchEnvironment(context.Background(), "web", EnvironmentPatch{
		SaveMode:    SaveModeDeploy,
		EnvVars:     []EnvVarPatch{{Key: "NEW", Value: "env-after"}},
		SecretFiles: []SecretFilePatch{{Name: "new.pem", Content: "file-after"}},
	})
	if err == nil || !strings.Contains(err.Error(), "injected App patch failure") {
		t.Fatalf("patch error = %v", err)
	}
	if !maps.Equal(store.m[envPath("web")], map[string]string{"OLD": "env-before"}) ||
		!maps.Equal(store.m[filesPath("web")], map[string]string{"old.pem": "file-before"}) {
		t.Fatalf("source compensation failed: env=%#v files=%#v", store.m[envPath("web")], store.m[filesPath("web")])
	}
	for _, name := range []string{"web-env", "web-files"} {
		var obj corev1.Secret
		err := failing.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &obj)
		if err == nil || !apierrors.IsNotFound(err) {
			t.Fatalf("projection %s remained after compensation: %v", name, err)
		}
	}
	if app := getApp(t, failing, "web"); app.Spec.RestartedAt != "" || app.Spec.EnvFromSecret != "" || len(app.Spec.FilesFromSecrets) != 0 {
		t.Fatalf("App changed despite failed patch: %#v", app.Spec)
	}
}

func TestPatchEnvironmentAuthorizationAndUnavailable(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user", Method: "oauth2"})
	checker := &fakeChecker{allow: false}
	svc := &Service{Base: &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: checker}, Store: newFakeSecretStore()}
	if _, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeOnly}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("forbidden patch = %v", err)
	}
	if checker.lastRelation != core.RelCanCreate {
		t.Fatalf("relation = %q", checker.lastRelation)
	}
	if _, err := newService(nil, sampleApp("web")).PatchEnvironment(context.Background(), "web", EnvironmentPatch{SaveMode: SaveModeOnly}); !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Fatalf("unavailable patch = %v", err)
	}
}

func TestPatchEnvironmentAdaptersShareTheContract(t *testing.T) {
	t.Run("REST", func(t *testing.T) {
		store := newFakeSecretStore()
		svc := newService(store, sampleApp("web"))
		response := serveREST(svc, http.MethodPatch, "/v1/services/web/environment", `{"saveMode":"save_only","envVars":[{"key":"TOKEN","value":"rest-secret"}],"secretFiles":[{"name":"key.pem","content":"rest-file"}]}`)
		if response.Code != http.StatusOK {
			t.Fatalf("PATCH = %d: %s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "rest-secret") || strings.Contains(response.Body.String(), "rest-file") {
			t.Fatalf("REST result leaked material: %s", response.Body.String())
		}
		if store.m[envPath("web")]["TOKEN"] != "rest-secret" || getApp(t, svc.Client, "web").Spec.RestartedAt != "" {
			t.Fatal("REST did not use save_only batch semantics")
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
		svc := newService(store, sampleApp("web"))
		field := svc.GraphQLMutation()["patchServiceEnvironment"]
		revision := encodeEnvRevision(0)
		value, err := field.Resolve(graphql.ResolveParams{Context: context.Background(), Args: map[string]any{
			"serviceId": "web", "saveMode": "deploy", "expectedEnvRevision": revision,
			"envVars": []any{map[string]any{"key": "TOKEN", "value": "graphql-secret"}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		result := value.(EnvironmentPatchResult)
		if !result.RolledOut || store.m[envPath("web")]["TOKEN"] != "graphql-secret" {
			t.Fatalf("GraphQL result=%#v store=%#v", result, store.m)
		}
	})

	t.Run("REST CAS", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
		svc := newService(store, sampleApp("web"))
		revision := encodeEnvRevision(0)
		response := serveREST(svc, http.MethodPatch, "/v1/services/web/environment", `{"saveMode":"deploy","expectedEnvRevision":"`+revision+`","envVars":[{"key":"TOKEN","value":"rest-cas-secret"}]}`)
		if response.Code != http.StatusOK || store.m[envPath("web")]["TOKEN"] != "rest-cas-secret" {
			t.Fatalf("REST CAS = %d: %s store=%#v", response.Code, response.Body.String(), store.m)
		}
		if strings.Contains(response.Body.String(), "rest-cas-secret") {
			t.Fatalf("REST CAS leaked value: %s", response.Body.String())
		}
	})

	t.Run("MCP", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
		cs := mcpSession(t, newService(store, sampleApp("web")))
		revision := encodeEnvRevision(0)
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
			"serviceId": "web", "saveMode": "save_only", "expectedEnvRevision": revision,
			"envVars": []map[string]any{{"key": "TOKEN", "value": "mcp-secret"}},
		}})
		if err != nil || res.IsError {
			t.Fatalf("MCP patch: err=%v result=%#v", err, res)
		}
		encoded, _ := json.Marshal(res.StructuredContent)
		if strings.Contains(string(encoded), "mcp-secret") || store.m[envPath("web")]["TOKEN"] != "mcp-secret" {
			t.Fatalf("MCP result/store = %s / %#v", encoded, store.m)
		}
	})

	t.Run("coded update-only refusal parity", func(t *testing.T) {
		revision := encodeEnvRevision(0)
		graphqlStore := newVersionedFakeSecretStore()
		graphqlSvc := newService(graphqlStore, sampleApp("web"))
		_, gqlErr := graphqlSvc.GraphQLMutation()["patchServiceEnvironment"].Resolve(graphql.ResolveParams{Context: context.Background(), Args: map[string]any{
			"serviceId": "web", "saveMode": "deploy", "expectedEnvRevision": revision,
			"envVars": []any{map[string]any{"key": "TOKEN", "value": "must-not-create"}},
		}})
		var gqlCoded *core.CodedError
		if !errors.As(gqlErr, &gqlCoded) || gqlCoded.Code != "ENVIRONMENT_VARIABLE_NOT_FOUND" {
			t.Fatalf("GraphQL refusal = %#v", gqlErr)
		}

		restStore := newVersionedFakeSecretStore()
		restResponse := serveREST(newService(restStore, sampleApp("web")), http.MethodPatch, "/v1/services/web/environment", `{"saveMode":"deploy","expectedEnvRevision":"`+revision+`","envVars":[{"key":"TOKEN","value":"must-not-create"}]}`)
		var restBody map[string]any
		_ = json.Unmarshal(restResponse.Body.Bytes(), &restBody)
		if restResponse.Code != http.StatusNotFound || restBody["code"] != "ENVIRONMENT_VARIABLE_NOT_FOUND" {
			t.Fatalf("REST refusal = %d %s", restResponse.Code, restResponse.Body.String())
		}

		mcpStore := newVersionedFakeSecretStore()
		cs := mcpSession(t, newService(mcpStore, sampleApp("web")))
		mcpResult, mcpErr := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
			"serviceId": "web", "saveMode": "deploy", "expectedEnvRevision": revision,
			"envVars": []map[string]any{{"key": "TOKEN", "value": "must-not-create"}},
		}})
		encoded, _ := json.Marshal(mcpResult)
		if mcpErr == nil && (mcpResult == nil || !mcpResult.IsError) {
			t.Fatalf("MCP refusal unexpectedly succeeded: %s", encoded)
		}
		if !strings.Contains(string(encoded), "ENVIRONMENT_VARIABLE_NOT_FOUND") && (mcpErr == nil || !strings.Contains(mcpErr.Error(), "ENVIRONMENT_VARIABLE_NOT_FOUND")) {
			t.Fatalf("MCP refusal lost code: err=%v result=%s", mcpErr, encoded)
		}
		for _, store := range []*versionedFakeSecretStore{graphqlStore, restStore, mcpStore} {
			if store.versions[envPath("web")] != 0 || len(store.m[envPath("web")]) != 0 {
				t.Fatalf("update-only refusal wrote source: %#v", store.m)
			}
		}
	})
}

func TestPatchEnvironmentCompensationCodesAreVisibleAndRedactedAcrossAdapters(t *testing.T) {
	revision := encodeEnvRevision(0)

	t.Run("REST restored", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
		failing := &patchCountingClient{Client: fakeClient(sampleApp("web")), fail: errors.New("injected /private/app/path failure")}
		svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}
		response := serveREST(svc, http.MethodPatch, "/v1/services/web/environment", `{"saveMode":"deploy","expectedEnvRevision":"`+revision+`","envVars":[{"key":"TOKEN","value":"failed-writer-secret"}]}`)
		var body map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &body)
		if response.Code != http.StatusConflict || body["code"] != "ENVIRONMENT_UPDATE_RESTORED" {
			t.Fatalf("REST restored outcome = %d %s", response.Code, response.Body.String())
		}
		for _, material := range []string{"TOKEN", "before-secret", "failed-writer-secret", "/private/app/path", revision} {
			if strings.Contains(response.Body.String(), material) {
				t.Fatalf("REST restored outcome leaked %q: %s", material, response.Body.String())
			}
		}
	})

	t.Run("MCP restoration failed", func(t *testing.T) {
		store := newVersionedFakeSecretStore()
		store.m[envPath("web")] = map[string]string{"TOKEN": "before-secret"}
		store.failCASCall = 2
		failing := &patchCountingClient{Client: fakeClient(sampleApp("web")), fail: errors.New("injected /private/app/path failure")}
		svc := &Service{Base: &core.Base{Client: failing, Namespace: "default", Clock: fixedNow}, Store: store}
		result, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
			"serviceId": "web", "saveMode": "deploy", "expectedEnvRevision": revision,
			"envVars": []map[string]any{{"key": "TOKEN", "value": "failed-writer-secret"}},
		}})
		encoded, _ := json.Marshal(result)
		if err == nil && (result == nil || !result.IsError) {
			t.Fatalf("MCP restoration failure unexpectedly succeeded: %s", encoded)
		}
		combined := string(encoded)
		if err != nil {
			combined += err.Error()
		}
		if !strings.Contains(combined, "ENVIRONMENT_RESTORATION_FAILED") {
			t.Fatalf("MCP restoration failure lost code: err=%v result=%s", err, encoded)
		}
		for _, material := range []string{"TOKEN", "before-secret", "failed-writer-secret", "/openbao/private/path", "/private/app/path", revision} {
			if strings.Contains(combined, material) {
				t.Fatalf("MCP restoration failure leaked %q: %s", material, combined)
			}
		}
	})
}

// TestDeleteEnvVarFinalKeyRaceSurvivesConcurrentSet pins codex-security round-19
// #5: deleting a service's LAST env var must not fall back to an unconditional
// OpenBao metadata Delete once the CAS retry loop's in-memory map goes empty —
// that ignored the read's observed version entirely, so a SetEnvVar that
// committed a newer revision in the read/write gap was wiped out along with
// every retained version. The delete must instead retry, exactly like any
// other CAS write, and pick up the concurrent writer's key.
func TestDeleteEnvVarFinalKeyRaceSurvivesConcurrentSet(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"OLD": "v"}
	svc := newService(store, sampleApp("web"))

	store.afterGet = func() {
		// A concurrent SetEnvVar lands after this delete's read but before its
		// write — committed directly (bypassing the deleter) with its own
		// version bump, exactly like a winning racing CAS write would.
		if err := store.Put(context.Background(), envPath("web"), map[string]string{"OLD": "v", "NEW": "concurrent"}); err != nil {
			t.Fatal(err)
		}
		store.versions[envPath("web")]++
	}

	if err := svc.DeleteEnvVar(context.Background(), "web", "OLD"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	final := store.m[envPath("web")]
	if _, ok := final["NEW"]; !ok {
		t.Fatalf("concurrent SetEnvVar was lost to the stale final-key delete: %#v", final)
	}
	if _, ok := final["OLD"]; ok {
		t.Fatalf("OLD should be gone once the delete retried against the fresh map: %#v", final)
	}
}

// TestPatchEnvironmentSparseSurvivesConcurrentSingleFieldWrite pins codex-security
// round-19 #7: a sparse (no ExpectedEnvRevision) PatchEnvironment must not read
// the whole env map once and unconditionally Put it back — a concurrent
// SetEnvVar landing in that gap was silently discarded. The sparse write now
// goes through the same GetVersioned/PutCAS retry loop as every single-field
// mutation, so it must retry and preserve the concurrent key instead of
// clobbering it.
func TestPatchEnvironmentSparseSurvivesConcurrentSingleFieldWrite(t *testing.T) {
	store := newVersionedFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"A": "1"}
	svc := newService(store, sampleApp("web"))

	store.afterGet = func() {
		if err := store.Put(context.Background(), envPath("web"), map[string]string{"A": "1", "B": "concurrent"}); err != nil {
			t.Fatal(err)
		}
		store.versions[envPath("web")]++
	}

	patch := EnvironmentPatch{SaveMode: SaveModeOnly, EnvVars: []EnvVarPatch{{Key: "C", Value: "3"}}}
	if _, err := svc.PatchEnvironment(context.Background(), "web", patch); err != nil {
		t.Fatalf("patch: %v", err)
	}
	final := store.m[envPath("web")]
	if final["B"] != "concurrent" {
		t.Fatalf("concurrent SetEnvVar was lost to the sparse patch's unconditional write: %#v", final)
	}
	if final["A"] != "1" || final["C"] != "3" {
		t.Fatalf("sparse patch did not apply cleanly alongside the concurrent write: %#v", final)
	}
}
