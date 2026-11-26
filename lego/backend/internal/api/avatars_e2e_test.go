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

package api

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/avatars"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
)

// avatars_e2e_test.go walks w1/m648 the way bex-api serves it, on real
// Postgres and OpenFGA: Render's avatarPresignedUpload, the PUT to the URL it
// mints, the avatarDownload every type that shows a person or a workspace
// answers, and deleteAvatar. A person sets only their own avatar; a
// workspace's is its Admins'; a refused size or body stores nothing; and an
// avatar goes with its workspace.

// testPNG is a w x h PNG.
func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// avatarHTTP serves the two routes outside the gate, as composedMuxes mounts
// them, and answers a request's status and body.
type avatarHTTP struct {
	t   *testing.T
	mux *http.ServeMux
}

func (h avatarHTTP) do(method, url string, body []byte) (int, http.Header, []byte) {
	h.t.Helper()
	path := url[strings.Index(url, "/v1/"):]
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "image/jpeg") // what Render's dashboard sends, whatever the bytes
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	out, _ := io.ReadAll(rec.Result().Body)
	return rec.Code, rec.Result().Header, out
}

// avatarURL reads one avatarDownload { url } off a GraphQL object; "" for null.
func avatarURL(obj any) string {
	m, _ := obj.(map[string]any)
	d, _ := m["avatarDownload"].(map[string]any)
	url, _ := d["url"].(string)
	return url
}

func TestAvatarsE2E(t *testing.T) {
	e := newOrgE2E(t, func(_ *core.Base, d *Deps) {
		st := d.WorkspaceStore.(*store.PGStore)
		d.AvatarStore = st
		d.AppearanceStore = st
		d.DeployHookBaseURL = "https://api.bex.test"
	})
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.Handle(avatars.UploadPattern, e.srv.Avatars.UploadHandler())
	mux.Handle(avatars.DownloadPattern, e.srv.Avatars.DownloadHandler())
	h := avatarHTTP{t: t, mux: mux}

	alice, bob := e.person("alice"), e.person("bob")
	acme, err := e.st.CreateWorkspace(ctx, "avatars-"+e.stamp, store.PlanPro, alice.Subject)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.st.DeleteTenant(context.Background(), acme.ID) })
	if err := e.fga.(workspaces.WorkspaceGranter).GrantWorkspaceAdmin(ctx, acme.ID, "user:"+alice.Subject); err != nil {
		t.Fatal(err)
	}
	if err := e.st.AddMember(ctx, bob.Subject, acme.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	if err := e.fga.(members.RoleGranter).GrantWorkspaceRole(ctx, acme.ID, "user:"+bob.Subject, "developer"); err != nil {
		t.Fatal(err)
	}

	user := func(id core.Identity) map[string]any {
		u, _ := e.ok(id, `{ user { id avatarDownload { url } } }`)["user"].(map[string]any)
		return u
	}
	presign := func(id core.Identity, owner, kind string, size int) string {
		return fmt.Sprintf(`{ avatarPresignedUpload(avatarOwnerId: %q, avatarType: %s, avatarSizeBytes: %d) { url verb } }`, owner, kind, size)
	}
	upload := func(id core.Identity, owner, kind string, body []byte) (int, []byte) {
		t.Helper()
		got, _ := e.ok(id, presign(id, owner, kind, len(body)))["avatarPresignedUpload"].(map[string]any)
		url, _ := got["url"].(string)
		if got["verb"] != "PUT" || !strings.HasPrefix(url, "https://api.bex.test/v1/avatar-uploads/") {
			t.Fatalf("presigned upload = %v", got)
		}
		code, _, out := h.do(http.MethodPut, url, body)
		return code, out
	}

	// No avatar yet: avatarDownload is null and the client draws the default.
	me := user(alice)
	aliceID, _ := me["id"].(string)
	if !strings.HasPrefix(aliceID, "usr-") || avatarURL(me) != "" {
		t.Fatalf("user before any upload = %v", me)
	}

	// The round trip: presign, PUT, and the account's avatarDownload serves
	// the very bytes, typed by what they are, not what the PUT claimed.
	img := testPNG(t, 2, 2)
	if code, out := upload(alice, aliceID, "ACCOUNT", img); code != http.StatusOK {
		t.Fatalf("PUT = %d %s", code, out)
	}
	url := avatarURL(user(alice))
	if !strings.HasPrefix(url, "https://api.bex.test/v1/avatars/") {
		t.Fatalf("avatarDownload after upload = %q", url)
	}
	code, hdr, body := h.do(http.MethodGet, url, nil)
	if code != http.StatusOK || hdr.Get("Content-Type") != "image/png" || !bytes.Equal(body, img) ||
		!strings.Contains(hdr.Get("Cache-Control"), "immutable") || hdr.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("GET avatar = %d %v (%d bytes)", code, hdr, len(body))
	}

	// An upload URL is good once.
	got, _ := e.ok(alice, presign(alice, aliceID, "ACCOUNT", len(img)))["avatarPresignedUpload"].(map[string]any)
	once, _ := got["url"].(string)
	if code, _, _ := h.do(http.MethodPut, once, img); code != http.StatusOK {
		t.Fatalf("first PUT = %d", code)
	}
	if code, _, _ := h.do(http.MethodPut, once, img); code != http.StatusNotFound {
		t.Fatalf("second PUT on a spent URL = %d, want 404", code)
	}
	// A new upload is a new URL; the old one is gone.
	if replaced := avatarURL(user(alice)); replaced == url {
		t.Fatal("a new upload kept the old download URL")
	} else if code, _, _ := h.do(http.MethodGet, url, nil); code != http.StatusNotFound {
		t.Fatalf("GET replaced avatar = %d, want 404", code)
	} else {
		url = replaced
	}

	// Refused: a size out of bounds, a body that is not an image or not the
	// size asked for. None of them changes the avatar.
	e.refused(alice, presign(alice, aliceID, "ACCOUNT", 0), "AVATAR_SIZE_INVALID")
	e.refused(alice, presign(alice, aliceID, "ACCOUNT", store.MaxAvatarBytes+1), "AVATAR_SIZE_INVALID")
	if code, _ := upload(alice, aliceID, "ACCOUNT", []byte("<svg onload=alert(1)>")); code != http.StatusBadRequest {
		t.Fatalf("non-image PUT = %d, want 400", code)
	}
	got, _ = e.ok(alice, presign(alice, aliceID, "ACCOUNT", len(img)+5))["avatarPresignedUpload"].(map[string]any)
	short, _ := got["url"].(string)
	if code, _, _ := h.do(http.MethodPut, short, img); code != http.StatusBadRequest {
		t.Fatalf("short PUT = %d, want 400", code)
	}
	if avatarURL(user(alice)) != url {
		t.Fatal("a refused upload changed the avatar")
	}

	// Only your own account: bob cannot mint for alice, and an API key, which
	// is not a person, cannot mint at all.
	e.refused(bob, presign(bob, aliceID, "ACCOUNT", len(img)), core.NotFoundCode)
	e.refused(bob, fmt.Sprintf(`mutation { deleteAvatar(avatarOwnerId: %q, avatarType: ACCOUNT) }`, aliceID), core.NotFoundCode)
	machine := core.Identity{Subject: "key-client-" + e.stamp, Method: "oauth2", ClientID: "key-client-" + e.stamp}
	if code := graphqlCode(e.gql(machine, presign(machine, aliceID, "ACCOUNT", len(img)))); code != "FORBIDDEN" && code != "AVATAR_PERSON_REQUIRED" {
		t.Fatalf("an API key's account avatar upload answered %q, want a refusal", code)
	}

	// A workspace's avatar is its Admins': bob, a developer, sees the row but
	// may neither upload nor reset it.
	e.refused(bob, presign(bob, acme.ID, "WORKSPACE", len(img)), "FORBIDDEN")
	e.refused(bob, fmt.Sprintf(`mutation { deleteAvatar(avatarOwnerId: %q, avatarType: WORKSPACE) }`, acme.ID), "FORBIDDEN")
	if code, out := upload(alice, acme.ID, "WORKSPACE", testPNG(t, 3, 3)); code != http.StatusOK {
		t.Fatalf("workspace PUT = %d %s", code, out)
	}

	// Every type that shows a person or a workspace answers its avatar.
	listed, _ := e.ok(bob, `{ workspaces { id avatarDownload { url } } }`)["workspaces"].([]any)
	var wsURL string
	for _, w := range listed {
		if w.(map[string]any)["id"] == acme.ID {
			wsURL = avatarURL(w)
		}
	}
	if !strings.HasPrefix(wsURL, "https://api.bex.test/v1/avatars/") {
		t.Fatalf("workspaces list = %v, want acme's avatar", listed)
	}
	people, _ := e.ok(bob, fmt.Sprintf(`{ workspaceMembers(workspaceId: %q) { userId avatarDownload { url } } }`, acme.ID))["workspaceMembers"].([]any)
	seen := map[string]string{}
	for _, m := range people {
		id, _ := m.(map[string]any)["userId"].(string)
		seen[id] = avatarURL(m)
	}
	if seen[aliceID] != url || len(seen) != 2 {
		t.Fatalf("members = %v, want alice's avatar and bob's none", seen)
	}

	// Reset: both owners go back to their default, and the images are gone.
	e.ok(alice, fmt.Sprintf(`mutation { deleteAvatar(avatarOwnerId: %q, avatarType: ACCOUNT) }`, aliceID))
	if avatarURL(user(alice)) != "" {
		t.Fatal("avatarDownload after reset is not null")
	}
	if code, _, _ := h.do(http.MethodGet, url, nil); code != http.StatusNotFound {
		t.Fatalf("GET after reset = %d, want 404", code)
	}

	// The workspace's avatar goes with the workspace.
	if err := e.st.DeleteTenant(ctx, acme.ID); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := h.do(http.MethodGet, wsURL, nil); code != http.StatusNotFound {
		t.Fatalf("GET a deleted workspace's avatar = %d, want 404", code)
	}
}
