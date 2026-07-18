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
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type patchCountingClient struct {
	client.Client
	patches int
	fail    error
}

func (c *patchCountingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	if c.fail != nil {
		return c.fail
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
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
	if !equalStringMaps(store.m[envPath("web")], map[string]string{"OLD": "env-before"}) ||
		!equalStringMaps(store.m[filesPath("web")], map[string]string{"old.pem": "file-before"}) {
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
		store := newFakeSecretStore()
		svc := newService(store, sampleApp("web"))
		field := svc.GraphQLMutation()["patchServiceEnvironment"]
		value, err := field.Resolve(graphql.ResolveParams{Context: context.Background(), Args: map[string]any{
			"serviceId": "web", "saveMode": "deploy",
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

	t.Run("MCP", func(t *testing.T) {
		store := newFakeSecretStore()
		cs := mcpSession(t, newService(store, sampleApp("web")))
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "patch_service_environment", Arguments: map[string]any{
			"serviceId": "web", "saveMode": "save_only",
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
}
