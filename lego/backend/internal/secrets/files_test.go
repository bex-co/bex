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
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestSecretFiles_RoundTripAndProjection(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	// Set one file: stored, materialized into <svc>-files, App mounts it + rolls.
	if _, err := svc.SetSecretFile(ctx, "web", "ca.pem", "----CERT----"); err != nil {
		t.Fatalf("SetSecretFile: %v", err)
	}
	if store.m[filesPath("web")]["ca.pem"] != "----CERT----" {
		t.Fatalf("store not written: %+v", store.m[filesPath("web")])
	}
	sec := getSecret(t, svc.Client, "web-files")
	if string(sec.Data["ca.pem"]) != "----CERT----" {
		t.Fatalf("file not materialized: %+v", sec.Data)
	}
	if len(sec.OwnerReferences) != 1 || sec.OwnerReferences[0].Name != "web" {
		t.Fatalf("files Secret should be App-owned: %+v", sec.OwnerReferences)
	}
	app := getApp(t, svc.Client, "web")
	if !slices.Contains(app.Spec.FilesFromSecrets, "web-files") || app.Spec.RestartedAt == "" {
		t.Fatalf("app should mount <svc>-files and roll: %+v", app.Spec)
	}

	// List is names-only; single GET reveals content.
	list, err := svc.ListSecretFiles(ctx, "web")
	if err != nil || len(list) != 1 || list[0].Name != "ca.pem" || list[0].Content != "" {
		t.Fatalf("ListSecretFiles names-only: %+v err=%v", list, err)
	}
	one, err := svc.GetSecretFile(ctx, "web", "ca.pem")
	if err != nil || one.Content != "----CERT----" {
		t.Fatalf("GetSecretFile: %+v err=%v", one, err)
	}
	if _, err := svc.GetSecretFile(ctx, "web", "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown file => ErrNotFound, got %v", err)
	}

	// Deleting the last file removes the Secret and drops the mount reference.
	if err := svc.DeleteSecretFile(ctx, "web", "ca.pem"); err != nil {
		t.Fatalf("DeleteSecretFile: %v", err)
	}
	var gone corev1.Secret
	if err := svc.Client.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web-files"}, &gone); !apierrors.IsNotFound(err) {
		t.Errorf("<svc>-files Secret should be deleted once empty, got %v", err)
	}
	app = getApp(t, svc.Client, "web")
	if slices.Contains(app.Spec.FilesFromSecrets, "web-files") {
		t.Errorf("mount reference should be removed once empty: %+v", app.Spec.FilesFromSecrets)
	}
}

func TestSeedSecretFilesWritesAllFilesTogether(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()
	files := []core.SecretFile{
		{Name: "ca.pem", Content: "CERT"},
		{Name: "config.json", Content: `{"enabled":true}`},
	}
	if err := svc.SeedSecretFiles(ctx, "web", files); err != nil {
		t.Fatalf("SeedSecretFiles: %v", err)
	}
	got, err := store.Get(ctx, filesPath("web"))
	if err != nil {
		t.Fatal(err)
	}
	if got["ca.pem"] != "CERT" || got["config.json"] != `{"enabled":true}` {
		t.Fatalf("stored files = %#v", got)
	}
	app := getApp(t, svc.Client, "web")
	if len(app.Spec.FilesFromSecrets) != 1 || app.Spec.FilesFromSecrets[0] != "web-files" {
		t.Fatalf("files projection = %#v", app.Spec.FilesFromSecrets)
	}
}

func TestSecretFiles_ReaderSeamAndValidation(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	// Invalid name (a path) => ErrBadRequest, content never leaks, nothing stored.
	_, err := svc.SetSecretFile(ctx, "web", "etc/passwd", "topsecret")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("want ErrBadRequest, got %v", err)
	}
	if strings.Contains(err.Error(), "topsecret") {
		t.Errorf("error must not carry the content: %v", err)
	}
	if _, ok := store.m[filesPath("web")]; ok {
		t.Error("invalid write should not have stored anything")
	}

	// Reader seam: names-only list, content on demand.
	if _, err := svc.SetSecretFile(ctx, "web", "token", "abc"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	names, err := svc.SecretFileNames(ctx, "web")
	if err != nil || len(names) != 1 || names[0].Name != "token" || names[0].Content != "" || names[0].ID != "token" {
		t.Fatalf("SecretFileNames: %+v err=%v", names, err)
	}
	got, err := svc.SecretFileContent(ctx, "web", "token")
	if err != nil || got.Content != "abc" {
		t.Fatalf("SecretFileContent: %+v err=%v", got, err)
	}
}

func TestSecretFiles_Authorization(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "c1", Method: "oauth2"})
	chk := &fakeChecker{allow: false}
	svc := &Service{Base: &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}, Store: newFakeSecretStore()}

	if _, err := svc.ListSecretFiles(ctx, "web"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("list deny: %v", err)
	}
	if chk.lastRelation != core.RelCanViewSensitive {
		t.Errorf("read checked %s, want can_view_sensitive", chk.lastRelation)
	}
	if _, err := svc.SetSecretFile(ctx, "web", "f", "v"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("write deny: %v", err)
	}
	if chk.lastRelation != core.RelCanCreate {
		t.Errorf("write checked %s, want can_create", chk.lastRelation)
	}
}

func TestREST_SecretFiles(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))

	// PUT one => bare {name, content}; store + materialize.
	var one SecretFileView
	_ = json.Unmarshal(serveREST(svc, "PUT", "/v1/services/web/secret-files/ca.pem", `{"content":"X"}`).Body.Bytes(), &one)
	if one.Name != "ca.pem" || one.Content != "X" {
		t.Fatalf("PUT secret file shape: %+v", one)
	}
	// GET list => Render cursor envelope, names only.
	var list []secretFileWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/secret-files", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].SecretFile.Name != "ca.pem" || list[0].Cursor == "" || list[0].SecretFile.Content != "" {
		t.Fatalf("list envelope names-only: %+v", list)
	}
	// GET one => content; unknown => 404.
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/secret-files/ca.pem", "").Body.Bytes(), &one)
	if one.Content != "X" {
		t.Fatalf("single GET content: %+v", one)
	}
	if serveREST(svc, "GET", "/v1/services/web/secret-files/nope", "").Code != 404 {
		t.Error("unknown file => 404")
	}
	// DELETE => 204; /v1/apps alias works.
	if serveREST(svc, "DELETE", "/v1/apps/web/secret-files/ca.pem", "").Code != 204 {
		t.Error("delete via /v1/apps alias => 204")
	}
}

func TestREST_SecretFilesUnconfiguredIs503(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	if serveREST(svc, "GET", "/v1/services/web/secret-files", "").Code != 503 {
		t.Error("GET secret-files without a store => 503")
	}
}
