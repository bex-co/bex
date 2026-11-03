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

package registry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeCopyRegistry is a registry with manifests by repo+digest and blobs by
// repo, enough to observe a server-side copy.
type fakeCopyRegistry struct {
	mu        sync.Mutex
	manifests map[string]string // repo@digest → body
	blobs     map[string]bool   // repo@digest
	declined  map[string]bool   // digest the registry will not mount
	auth      []string
	abandoned []string // upload sessions deleted
}

func (f *fakeCopyRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	path := strings.TrimPrefix(r.URL.Path, "/v2/")
	switch {
	case strings.Contains(path, "/manifests/"):
		repo, ref, _ := strings.Cut(path, "/manifests/")
		key := repo + "@" + ref
		switch r.Method {
		case http.MethodGet:
			body, ok := f.manifests[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			_, _ = io.WriteString(w, body)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			f.manifests[key] = string(body)
			w.WriteHeader(http.StatusCreated)
		}
	case strings.HasSuffix(path, "/blobs/uploads/") && r.Method == http.MethodPost:
		repo := strings.TrimSuffix(path, "/blobs/uploads/")
		digest, from := r.URL.Query().Get("mount"), r.URL.Query().Get("from")
		if f.declined[digest] || !f.blobs[from+"@"+digest] {
			w.Header().Set("Location", "/v2/"+repo+"/blobs/uploads/session-1")
			w.WriteHeader(http.StatusAccepted)
			return
		}
		f.blobs[repo+"@"+digest] = true
		w.WriteHeader(http.StatusCreated)
	case strings.Contains(path, "/blobs/uploads/") && r.Method == http.MethodDelete:
		f.abandoned = append(f.abandoned, path)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newFakeCopyRegistry(t *testing.T) (*fakeCopyRegistry, string) {
	t.Helper()
	f := &fakeCopyRegistry{
		manifests: map[string]string{
			"tea-a/source@" + digestIndex:  `{"manifests":[{"digest":"` + digestAMD64 + `"}]}`,
			"tea-a/source@" + digestAMD64:  `{"config":{"digest":"` + digestCfg + `"},"layers":[{"digest":"` + digestL1 + `"},{"digest":"` + digestL2 + `"}]}`,
			"tea-a/source@" + digestSingle: `{"config":{"digest":"` + digestCfg + `"},"layers":[{"digest":"` + digestL1 + `"}]}`,
		},
		blobs:    map[string]bool{"tea-a/source@" + digestCfg: true, "tea-a/source@" + digestL1: true, "tea-a/source@" + digestL2: true},
		declined: map[string]bool{},
	}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, strings.TrimPrefix(srv.URL, "http://")
}

// ADR097 §4: a linked service gets the build under the same digest in its own
// repository, every blob mounted rather than re-uploaded, with the platform
// builder's credential on every call.
func TestCopyManifestMountsBlobsAndKeepsTheDigest(t *testing.T) {
	f, host := newFakeCopyRegistry(t)
	if err := CopyManifest(context.Background(), nil, host, "tea-a/source", "tea-a/web", digestIndex, "build-1", "Basic YnVpbGRlcjpwdw=="); err != nil {
		t.Fatal(err)
	}
	if f.manifests["tea-a/web@build-1"] != f.manifests["tea-a/source@"+digestIndex] {
		t.Error("the copy is not tagged, so retention could reap it")
	}
	for _, key := range []string{"tea-a/web@" + digestIndex, "tea-a/web@" + digestAMD64} {
		if f.manifests[key] != f.manifests[strings.Replace(key, "web", "source", 1)] {
			t.Errorf("%s = %q, want the source manifest byte for byte", key, f.manifests[key])
		}
	}
	for _, blob := range []string{digestCfg, digestL1, digestL2} {
		if !f.blobs["tea-a/web@"+blob] {
			t.Errorf("blob %s was not mounted into tea-a/web", blob)
		}
	}
	for _, got := range f.auth {
		if got != "Basic YnVpbGRlcjpwdw==" {
			t.Fatalf("a request carried Authorization %q", got)
		}
	}
}

// A declined mount would leave the copied manifest naming a blob the
// destination lacks, so it fails the copy before the manifest is written.
func TestCopyManifestFailsWhenAMountIsDeclined(t *testing.T) {
	f, host := newFakeCopyRegistry(t)
	f.declined[digestL1] = true
	err := CopyManifest(context.Background(), nil, host, "tea-a/source", "tea-a/web", digestSingle, "build-1", "")
	if err == nil || !strings.Contains(err.Error(), "mount blob "+digestL1) {
		t.Fatalf("CopyManifest = %v, want the declined mount", err)
	}
	if _, written := f.manifests["tea-a/web@"+digestSingle]; written {
		t.Error("the manifest was written although a blob is missing")
	}
	if len(f.abandoned) != 1 {
		t.Errorf("abandoned uploads = %v, want the declined mount's session deleted", f.abandoned)
	}
}

func TestCopyManifestUnknownDigest(t *testing.T) {
	_, host := newFakeCopyRegistry(t)
	if err := CopyManifest(context.Background(), nil, host, "tea-a/source", "tea-a/web", "sha256:"+strings.Repeat("0", 64), "build-1", ""); err == nil {
		t.Fatal("copying an absent manifest succeeded")
	}
	// Nothing unchecked reaches a URL.
	for _, bad := range [][3]string{{"../x", "tea-a/web", digestSingle}, {"tea-a/source", "tea-a/web", "sha256:short"}} {
		if err := CopyManifest(context.Background(), nil, host, bad[0], bad[1], bad[2], "build-1", ""); err == nil || !strings.Contains(err.Error(), "invalid copy reference") {
			t.Errorf("CopyManifest(%v) = %v, want refused before any request", bad, err)
		}
	}
}

func fakeDigest(c string) string { return "sha256:" + strings.Repeat(c, 64) }

var (
	digestIndex  = fakeDigest("1")
	digestAMD64  = fakeDigest("2")
	digestSingle = fakeDigest("3")
	digestCfg    = fakeDigest("a")
	digestL1     = fakeDigest("b")
	digestL2     = fakeDigest("c")
)
