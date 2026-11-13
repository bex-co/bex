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

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w2/m358: imported sandbox images against a real control-plane Postgres, a
// source registry and the sandbox registry (both in-process), with every
// /v1/sandbox-groups response checked against Render's pinned schema.

type imageFixture struct {
	*groupFixture
	source, sink *httptest.Server
	group        Group
}

func newImageFixture(t *testing.T) *imageFixture {
	t.Helper()
	f := newGroupFixture(t)
	source := httptest.NewServer(registry.New())
	t.Cleanup(source.Close)
	sink := httptest.NewServer(registry.New())
	t.Cleanup(sink.Close)
	f.svc.Images = &Images{
		Store:    f.st,
		Registry: RegistryConfig{BaseURL: sink.URL, Prefix: DefaultSnapshotRepoPrefix, User: "bex-sandbox-importer", Password: "importer-pw"},
		// The in-process registries listen on loopback, which the default
		// transport's dial guard refuses (TestImportRefusesAPrivateSource).
		SourceTransport: http.DefaultTransport,
	}
	g := f.createGroup(t, "images")
	return &imageFixture{groupFixture: f, source: source, sink: sink, group: g}
}

func hostOf(s *httptest.Server) string { return strings.TrimPrefix(s.URL, "http://") }

// pushImage puts a random image with the given platform at ref on the source
// registry and returns its digest.
func pushImage(t *testing.T, ref string, platform v1.Platform, layerBytes int64) (v1.Image, string) {
	t.Helper()
	img, err := random.Image(layerBytes, 2)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = platform.OS, platform.Architecture
	if img, err = mutate.ConfigFile(img, cfg); err != nil {
		t.Fatal(err)
	}
	r, err := name.ParseReference(ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(r, img); err != nil {
		t.Fatal(err)
	}
	d, _ := img.Digest()
	return img, d.String()
}

func (f *imageFixture) importImage(t *testing.T, body string) (int, string) {
	t.Helper()
	return f.do(t, http.MethodPost, "/v1/sandbox-groups/"+f.group.ID+"/builds", body)
}

// waitBuild polls the build until it settles.
func (f *imageFixture) waitBuild(t *testing.T, id string) Build {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/builds/"+id, "")
		var b Build
		if err := json.Unmarshal([]byte(body), &b); code != http.StatusOK || err != nil {
			t.Fatalf("get build = %d %s", code, body)
		}
		if b.Status == store.SandboxBuildSucceeded || b.Status == store.SandboxBuildFailed {
			return b
		}
		if time.Now().After(deadline) {
			t.Fatalf("build %s still %s", id, b.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (f *imageFixture) mustImport(t *testing.T, body string) Build {
	t.Helper()
	code, body := f.importImage(t, body)
	var b Build
	if err := json.Unmarshal([]byte(body), &b); code != http.StatusCreated || err != nil {
		t.Fatalf("import = %d %s", code, body)
	}
	return b
}

func TestImportImageThenStartASandboxFromIt(t *testing.T) {
	f := newImageFixture(t)
	ref := hostOf(f.source) + "/library/py:3.13"
	_, digest := pushImage(t, ref, linuxAMD64, 512)

	b := f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	if b.SourceDigest != digest || b.Source != (BuildSource{Repository: hostOf(f.source), Image: "library/py", Reference: "3.13"}) ||
		b.SandboxGroupID != f.group.ID || (b.Status != store.SandboxBuildPending && b.Status != store.SandboxBuildRunning) {
		t.Fatalf("import = %+v, want source %s digest %s, pending or running", b, ref, digest)
	}
	done := f.waitBuild(t, b.ID)
	if done.Status != store.SandboxBuildSucceeded || done.FinishedAt == nil || done.FailureReason != nil {
		t.Fatalf("settled build = %+v", done)
	}

	// The copy is in the workspace's snapshot namespace in the sandbox registry.
	stored, err := name.ParseReference(fmt.Sprintf("%s/snapshots/%s/images/%s:image", hostOf(f.sink), store.SandboxNamespace(f.ws), b.ID), name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	desc, err := remote.Head(stored)
	if err != nil || desc.Digest.String() != digest {
		t.Fatalf("stored image = %+v, %v; want %s", desc, err, digest)
	}

	// The same name and tag returns the same build and copies nothing again.
	if again := f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref)); again.ID != b.ID {
		t.Fatalf("re-import = %s, want %s", again.ID, b.ID)
	}
	code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/builds?name=py", "")
	var list []BuildEnvelope
	if err := json.Unmarshal([]byte(body), &list); code != http.StatusOK || err != nil || len(list) != 1 || list[0].Cursor != b.ID {
		t.Fatalf("list = %d %s", code, body)
	}

	// A sandbox started by name or by id runs the stored digest.
	want := fmt.Sprintf("%s/snapshots/%s/images/%s@%s", hostOf(f.sink), store.SandboxNamespace(f.ws), b.ID, digest)
	for _, sel := range []string{`"buildName":"py"`, fmt.Sprintf(`"buildId":%q`, b.ID)} {
		code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,%s}`, f.ws, f.group.ID, sel))
		var sb Sandbox
		if err := json.Unmarshal([]byte(body), &sb); code != http.StatusCreated || err != nil {
			t.Fatalf("create with %s = %d %s", sel, code, body)
		}
		if got := f.os.boxes[f.os.idOf(sb.ID)].image; got != want {
			t.Fatalf("create with %s started %q, want %q", sel, got, want)
		}
	}
	// A read shows the build, never the sandbox registry's internal address.
	code, body = f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"buildName":"py"}`, f.ws))
	if code != http.StatusCreated || strings.Contains(body, hostOf(f.sink)) || !strings.Contains(body, `"image":"`+b.ID+`"`) {
		t.Fatalf("create in the default group (this one) = %d %s, want image %s and no registry address", code, body, b.ID)
	}
	// Another group has no image of that name.
	other := f.createGroup(t, "other")
	code, body = f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildName":"py"}`, f.ws, other.ID))
	if code != http.StatusNotFound || errCode(body) != CodeBuildNotFound {
		t.Fatalf("create by name in a group without it = %d %s", code, body)
	}
}

func TestImportPicksTheLinuxAMD64ImageOfAnIndex(t *testing.T) {
	f := newImageFixture(t)
	amd, armImg := mustRandom(t, linuxAMD64), mustRandom(t, v1.Platform{OS: "linux", Architecture: "arm64"})
	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{Add: armImg, Descriptor: v1.Descriptor{Platform: &v1.Platform{OS: "linux", Architecture: "arm64"}}},
		mutate.IndexAddendum{Add: amd, Descriptor: v1.Descriptor{Platform: &linuxAMD64}})
	ref := hostOf(f.source) + "/acme/tool:1"
	r, _ := name.ParseReference(ref)
	if err := remote.WriteIndex(r, idx); err != nil {
		t.Fatal(err)
	}
	idxDigest, _ := idx.Digest()
	amdDigest, _ := amd.Digest()

	b := f.mustImport(t, fmt.Sprintf(`{"name":"tool","sourceImage":%q}`, ref))
	if b.SourceDigest != idxDigest.String() {
		t.Fatalf("sourceDigest = %s, want the index's %s", b.SourceDigest, idxDigest)
	}
	if done := f.waitBuild(t, b.ID); done.Status != store.SandboxBuildSucceeded {
		t.Fatalf("index import = %+v", done)
	}
	stored, _ := f.st.SandboxBuild(context.Background(), f.ws, f.group.ID, b.ID)
	if stored.ImageDigest == nil || *stored.ImageDigest != amdDigest.String() {
		t.Fatalf("stored digest = %v, want the amd64 child's %s", stored.ImageDigest, amdDigest)
	}
}

func mustRandom(t *testing.T, p v1.Platform) v1.Image {
	t.Helper()
	img, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := img.ConfigFile()
	cfg = cfg.DeepCopy()
	cfg.OS, cfg.Architecture = p.OS, p.Architecture
	img, err = mutate.ConfigFile(img, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestImportRefusesImagesASandboxCannotRun(t *testing.T) {
	f := newImageFixture(t)
	f.svc.Images.MaxBytes = 4096
	arm := hostOf(f.source) + "/acme/arm:1"
	pushImage(t, arm, v1.Platform{OS: "linux", Architecture: "arm64"}, 256)
	big := hostOf(f.source) + "/acme/big:1"
	pushImage(t, big, linuxAMD64, 8192)

	for _, tc := range []struct{ body, code, mentions string }{
		{fmt.Sprintf(`{"name":"arm","sourceImage":%q}`, arm), CodeSandboxImageInvalid, "linux/amd64"},
		{fmt.Sprintf(`{"name":"big","sourceImage":%q}`, big), CodeSandboxImageInvalid, "limited to"},
		{fmt.Sprintf(`{"name":"gone","sourceImage":%q}`, hostOf(f.source)+"/acme/missing:1"), CodeSandboxImageUnavailable, "could not serve"},
		{`{"name":"bad","sourceImage":"UPPER/Case::x"}`, CodeSandboxImageInvalid, "not an image reference"},
		{fmt.Sprintf(`{"name":"","sourceImage":%q}`, arm), CodeSandboxImageInvalid, "name must be"},
	} {
		code, body := f.importImage(t, tc.body)
		if code != http.StatusBadRequest || errCode(body) != tc.code || !strings.Contains(body, tc.mentions) {
			t.Errorf("%s = %d %s, want 400 %s mentioning %q", tc.body, code, body, tc.code, tc.mentions)
		}
	}
	if rows, err := f.st.SandboxBuilds(context.Background(), f.ws, f.group.ID, "", nil); err != nil || len(rows) != 0 {
		t.Fatalf("refused imports left %d builds, %v", len(rows), err)
	}
}

// A private source is pulled with the request's credential, which neither the
// database nor bex-api's log ever holds.
func TestPrivateImportUsesTheCredentialOnceAndKeepsNone(t *testing.T) {
	f := newImageFixture(t)
	const password = "s3cret-registry-token"
	reg := registry.New()
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "bot" || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="private"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		reg.ServeHTTP(w, r)
	}))
	t.Cleanup(private.Close)
	img := mustRandom(t, linuxAMD64)
	ref := hostOf(private) + "/acme/agent:1"
	r, _ := name.ParseReference(ref)
	if err := remote.Write(r, img, remote.WithAuth(authn.FromConfig(authn.AuthConfig{Username: "bot", Password: password}))); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })

	code, body := f.importImage(t, fmt.Sprintf(`{"name":"agent","sourceImage":%q}`, ref))
	if code != http.StatusBadRequest || errCode(body) != CodeSandboxImageUnavailable {
		t.Fatalf("without a credential = %d %s, want 400 %s", code, body, CodeSandboxImageUnavailable)
	}
	b := f.mustImport(t, fmt.Sprintf(`{"name":"agent","sourceImage":%q,"registryCredential":{"username":"bot","password":%q}}`, ref, password))
	if done := f.waitBuild(t, b.ID); done.Status != store.SandboxBuildSucceeded {
		t.Fatalf("private import = %+v", done)
	}
	var row string
	if err := f.st.Pool.QueryRow(context.Background(), `SELECT row_to_json(b)::text FROM sandbox_builds b WHERE id = $1`, b.ID).Scan(&row); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(row, password) || strings.Contains(logs.String(), password) {
		t.Fatalf("the credential was kept: row %s, logs %q", row, logs.String())
	}
}

func TestImageReadsAndCreateRefusals(t *testing.T) {
	f := newImageFixture(t)
	ctx := context.Background()
	other := f.createGroup(t, "other")
	pending, _, err := f.st.CreateSandboxBuild(ctx, store.NewSandboxBuild{
		WorkspaceID: f.ws, SandboxGroupID: f.group.ID, Name: "slow",
		SourceRepository: "docker.io", SourceImage: "library/slow", SourceReference: "1", SourceDigest: "sha256:" + strings.Repeat("a", 64),
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"get malformed id", http.MethodGet, "/v1/sandbox-groups/" + f.group.ID + "/builds/nope", "", 400, CodeSandboxImageInvalid},
		{"get another group's build", http.MethodGet, "/v1/sandbox-groups/" + other.ID + "/builds/" + pending.ID, "", 404, CodeBuildNotFound},
		{"list unknown status", http.MethodGet, "/v1/sandbox-groups/" + f.group.ID + "/builds?status=done", "", 400, CodeInvalidBuildStatus},
		{"import into a malformed group", http.MethodPost, "/v1/sandbox-groups/nope/builds", `{"name":"x","sourceImage":"python:3.13"}`, 400, CodeInvalidSandboxGroupID},
		{"create from a pending build", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildId":%q}`, f.ws, f.group.ID, pending.ID), 409, CodeBuildNotSucceeded},
		// Render's 404 is for a name no build has; a name whose only build is
		// still importing is its 409 (w8/314).
		{"create from a name still importing", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildName":"slow"}`, f.ws, f.group.ID), 409, CodeBuildNotSucceeded},
		{"create from an unknown name", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildName":"nope"}`, f.ws, f.group.ID), 404, CodeBuildNotFound},
		{"create from another group's build", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildId":%q}`, f.ws, other.ID, pending.ID), 404, CodeBuildNotFound},
		{"create naming both", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"buildId":%q,"buildName":"slow"}`, f.ws, pending.ID), 400, CodeSandboxImageInvalid},
	} {
		code, body := f.do(t, tc.method, tc.path, tc.body)
		if code != tc.status || errCode(body) != tc.code {
			t.Errorf("%s = %d %s, want %d %s", tc.name, code, body, tc.status, tc.code)
		}
	}
	if n := f.os.count("Running"); n != 0 {
		t.Fatalf("a refused create started %d sandboxes", n)
	}
	// The group's in-flight limit answers 429 and names the limit.
	f.svc.Images.MaxInFlight = 1
	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	code, body := f.importImage(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	if code != http.StatusTooManyRequests || errCode(body) != CodeSandboxBuildLimit || !strings.Contains(body, "1 images importing") {
		t.Fatalf("import past the in-flight limit = %d %s", code, body)
	}
}

// The default source transport refuses a private destination, so an import
// cannot reach the cluster or cloud metadata (the in-process registry here
// listens on loopback).
func TestImportRefusesAPrivateSource(t *testing.T) {
	f := newImageFixture(t)
	f.svc.Images.SourceTransport = nil
	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	code, body := f.importImage(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	if code != http.StatusBadRequest || errCode(body) != CodeSandboxImageUnavailable {
		t.Fatalf("import from loopback = %d %s, want 400 %s", code, body, CodeSandboxImageUnavailable)
	}
}
