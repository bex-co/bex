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
	"k8s.io/apimachinery/pkg/types"
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

func TestPrepareSecretFilesMakesProjectionPartOfFirstAppRevision(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store)
	ctx := context.Background()
	app := sampleApp("web")
	app.UID = types.UID("app-uid")
	files := []core.SecretFile{{Name: "token", Content: "first-boot-secret"}}

	if err := svc.prepareSecretFiles(ctx, "web", app, files); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(app.Spec.FilesFromSecrets, "web-files") {
		t.Fatalf("App spec did not reference prepared projection: %#v", app.Spec.FilesFromSecrets)
	}
	secret := getSecret(t, svc.Client, "web-files")
	if string(secret.Data["token"]) != "first-boot-secret" || len(secret.OwnerReferences) != 0 {
		t.Fatalf("prepared Secret = data %#v owners %#v", secret.Data, secret.OwnerReferences)
	}
	if err := svc.Client.Create(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := svc.commitSecretFiles(ctx, "web", app); err != nil {
		t.Fatal(err)
	}
	secret = getSecret(t, svc.Client, "web-files")
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].UID != app.UID {
		t.Fatalf("committed Secret owner = %#v", secret.OwnerReferences)
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

// seedSecretFiles adds the given files (empty content) to a service in one loop,
// so a paging test has a stable name-sorted set to walk.
func seedSecretFiles(t *testing.T, svc *Service, service string, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := svc.SetSecretFile(context.Background(), service, name, "x"); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
}

func TestListSecretFilesPageVisitsEveryNameExactlyOnce(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	ctx := context.Background()
	// Insert out of order; the store is name-sorted so the cursor is stable.
	seedSecretFiles(t, svc, "web", "e.pem", "a.pem", "d.pem", "b.pem", "c.pem")

	var got []string
	cursor := ""
	for {
		page, err := svc.ListSecretFilesPage(ctx, "web", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 2 {
			t.Fatalf("page of %d exceeds the requested limit", len(page))
		}
		for _, f := range page {
			got = append(got, f.Name)
		}
		cursor = page[len(page)-1].Name
	}
	if want := []string{"a.pem", "b.pem", "c.pem", "d.pem", "e.pem"}; !slices.Equal(got, want) {
		t.Fatalf("paged names = %v, want %v", got, want)
	}
	// Omitting both params keeps the pre-pagination full-list behavior.
	all, err := svc.ListSecretFilesPage(ctx, "web", "", 0)
	if err != nil || len(all) != 5 {
		t.Fatalf("omitted pagination = %d files, %v; want all 5", len(all), err)
	}
}

// TestListSecretFilesPageStableUnderInterleavedWrites is the m10 stable-name
// property applied to secret files: because the cursor is the file NAME (not an
// index) over a name-sorted list, writes that land between two page fetches
// never re-emit an already-seen file nor skip a not-yet-seen one. A new file
// sorting *before* the cursor is correctly not re-returned; one sorting *after*
// is picked up; a delete of an unreached file is respected — all without
// duplicates.
func TestListSecretFilesPageStableUnderInterleavedWrites(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	ctx := context.Background()
	seedSecretFiles(t, svc, "web", "a.pem", "b.pem", "c.pem", "d.pem", "e.pem")

	// Page 1 (limit 2) => a.pem, b.pem; resume cursor is b.pem.
	page1, err := svc.ListSecretFilesPage(ctx, "web", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || page1[0].Name != "a.pem" || page1[1].Name != "b.pem" {
		t.Fatalf("page1 = %+v, want [a.pem b.pem]", page1)
	}
	cursor := page1[len(page1)-1].Name

	// Interleave writes before resuming: one insert in the already-passed range
	// (< cursor), one in the unseen range (> cursor), and a delete of an unseen
	// file. None of these must corrupt the resume.
	seedSecretFiles(t, svc, "web", "aa.pem") // sorts before the cursor
	seedSecretFiles(t, svc, "web", "cc.pem") // sorts after the cursor, unseen
	if err := svc.DeleteSecretFile(ctx, "web", "d.pem"); err != nil {
		t.Fatal(err)
	}

	// Resume to completion; collect every name the tail yields.
	var tail []string
	for {
		page, err := svc.ListSecretFilesPage(ctx, "web", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, f := range page {
			tail = append(tail, f.Name)
		}
		cursor = page[len(page)-1].Name
	}

	// c.pem, the freshly-inserted cc.pem, and e.pem — d.pem deleted; the
	// before-cursor aa.pem must never resurface; no duplicates.
	if want := []string{"c.pem", "cc.pem", "e.pem"}; !slices.Equal(tail, want) {
		t.Fatalf("resumed tail = %v, want %v (stable-name property violated)", tail, want)
	}
	if slices.Contains(tail, "aa.pem") {
		t.Fatal("a file inserted before the cursor was wrongly re-emitted")
	}
}

func TestListSecretFilesPageBoundaries(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	ctx := context.Background()
	seedSecretFiles(t, svc, "web", "a.pem", "b.pem", "c.pem")

	// A negative limit is a bad request (mirrors ListEnvVarsPage).
	if _, err := svc.ListSecretFilesPage(ctx, "web", "", -1); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("negative limit err = %v, want ErrBadRequest", err)
	}
	// An over-limit request is clamped to the API maximum, not rejected.
	big, err := svc.ListSecretFilesPage(ctx, "web", "", core.MaxPageLimit+50)
	if err != nil || len(big) != 3 {
		t.Fatalf("over-limit clamp = %d files, %v; want the whole 3-file set", len(big), err)
	}
	// An unknown/expired cursor yields an empty tail, never a wraparound.
	if tail, err := svc.ListSecretFilesPage(ctx, "web", "zzz.pem", 10); err != nil || len(tail) != 0 {
		t.Fatalf("unknown cursor = %d files, %v; want empty tail", len(tail), err)
	}
	// A cursor with no explicit limit falls back to Render's default page size.
	if page, err := svc.ListSecretFilesPage(ctx, "web", "", 0); err != nil || len(page) != 3 {
		t.Fatalf("cursor default page unexpected: %d, %v", len(page), err)
	}
	// The last item's cursor pages past the end to an empty tail.
	if tail, err := svc.ListSecretFilesPage(ctx, "web", "c.pem", 2); err != nil || len(tail) != 0 {
		t.Fatalf("last cursor = %d files, %v; want empty tail", len(tail), err)
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

	// Add a second file so the cursor/limit contract has something to page.
	seedSecretFiles(t, svc, "web", "db.pem")
	// Requested pagination is cursor-exclusive; omitting both params remains the
	// pre-pagination full-list behavior (the env-vars route's exact semantics).
	var firstPage, secondPage []secretFileWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/secret-files?limit=1", "").Body.Bytes(), &firstPage)
	if len(firstPage) != 1 {
		t.Fatalf("first page = %+v, want one item", firstPage)
	}
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/secret-files?limit=1&cursor="+firstPage[0].Cursor, "").Body.Bytes(), &secondPage)
	if len(secondPage) != 1 || secondPage[0].SecretFile.Name == firstPage[0].SecretFile.Name {
		t.Fatalf("second page = %+v after %+v", secondPage, firstPage)
	}
	var unpaged []secretFileWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/secret-files", "").Body.Bytes(), &unpaged)
	if len(unpaged) != 2 {
		t.Fatalf("unpaged list = %+v, want the complete two-item set", unpaged)
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
