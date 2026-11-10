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
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The manifest of a generation-2 image read from a local Zot v2.1.22 on
// 2026-11-10 (ADR023 § Sandbox snapshot storage): the config blob and four
// layers, the last of which is 30 MB of zeros compressed to 29,310 bytes.
const gen2Manifest = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json",
"config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:aa","size":1375},
"layers":[{"size":4092319},{"size":20978113},{"size":5244676},{"size":29310}]}`

const gen2Bytes = 1375 + 4092319 + 20978113 + 5244676 + 29310 // 30,345,793

func fakeZot(t *testing.T, tags string, manifest string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "meter" || pass != "s3cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v2/snapshots/tea-a-sandbox/os-1-sandbox/tags/list":
			_, _ = fmt.Fprint(w, tags)
		case "/v2/snapshots/tea-a-sandbox/os-1-sandbox/manifests/snap-gen10":
			if !strings.Contains(r.Header.Get("Accept"), "application/vnd.oci.image.manifest.v1+json") {
				w.WriteHeader(http.StatusNotAcceptable)
				return
			}
			_, _ = fmt.Fprint(w, manifest)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRegistrySizerReadsTheNewestSnapshotManifest(t *testing.T) {
	// snap-gen10 is newer than snap-gen2 (a numeric order, not a string one), and
	// tags that are not the controller's are ignored.
	srv := fakeZot(t, `{"name":"x","tags":["snap-gen2","snap-gen10","latest","snap-gen-x","snap-gen1"]}`, gen2Manifest)
	sizer := &RegistrySizer{RegistryConfig{BaseURL: srv.URL + "/", User: "meter", Password: "s3cret"}}
	bytes, err := sizer.SnapshotBytes(context.Background(), "tea-a", "os-1")
	if err != nil {
		t.Fatal(err)
	}
	if bytes != gen2Bytes {
		t.Errorf("bytes = %d, want %d", bytes, gen2Bytes)
	}
}

func TestRegistrySizerRefusesWhatIsNotASingleSnapshotImage(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		tags, manifest, ref string
		sizer               func(*RegistrySizer)
	}{
		"no snapshot tag yet":       {tags: `{"tags":["latest"]}`, manifest: gen2Manifest, ref: "os-1"},
		"an image index":            {tags: `{"tags":["snap-gen10"]}`, manifest: `{"manifests":[{"digest":"sha256:a"}]}`, ref: "os-1"},
		"a manifest with no layers": {tags: `{"tags":["snap-gen10"]}`, manifest: `{"config":{"size":10},"layers":[]}`, ref: "os-1"},
		"a negative layer":          {tags: `{"tags":["snap-gen10"]}`, manifest: `{"config":{"size":10},"layers":[{"size":-5}]}`, ref: "os-1"},
		"an absurd layer":           {tags: `{"tags":["snap-gen10"]}`, manifest: `{"config":{"size":10},"layers":[{"size":17592186044416}]}`, ref: "os-1"},
		"bad tag json":              {tags: `not json`, manifest: gen2Manifest, ref: "os-1"},
		"a path in the sandbox id":  {tags: `{"tags":["snap-gen10"]}`, manifest: gen2Manifest, ref: "../os-1"},
		"a wrong password":          {tags: `{"tags":["snap-gen10"]}`, manifest: gen2Manifest, ref: "os-1", sizer: func(r *RegistrySizer) { r.Password = "wrong" }},
		"an unknown sandbox":        {tags: `{"tags":["snap-gen10"]}`, manifest: gen2Manifest, ref: "os-2"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := fakeZot(t, tc.tags, tc.manifest)
			sizer := &RegistrySizer{RegistryConfig{BaseURL: srv.URL, User: "meter", Password: "s3cret"}}
			if tc.sizer != nil {
				tc.sizer(sizer)
			}
			if bytes, err := sizer.SnapshotBytes(ctx, "tea-a", tc.ref); err == nil {
				t.Errorf("sized %d bytes, want an error", bytes)
			}
		})
	}
}

func TestRegistrySizerHonoursItsRepositoryLayout(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			_, _ = fmt.Fprint(w, `{"tags":["snap-gen3"]}`)
			return
		}
		_, _ = fmt.Fprint(w, gen2Manifest)
	}))
	t.Cleanup(srv.Close)
	sizer := &RegistrySizer{RegistryConfig{BaseURL: srv.URL, Prefix: "snaps", Container: "main"}}
	if _, err := sizer.SnapshotBytes(context.Background(), "tea-a", "os-1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"/v2/snaps/tea-a-sandbox/os-1-main/tags/list", "/v2/snaps/tea-a-sandbox/os-1-main/manifests/snap-gen3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("requests = %v, want %v", got, want)
	}
}

type fakeSizer struct {
	bytes map[string]int64
	calls []string
}

func (f *fakeSizer) SnapshotBytes(_ context.Context, ws, ref string) (int64, error) {
	f.calls = append(f.calls, ws+"/"+ref)
	if n, ok := f.bytes[ref]; ok {
		return n, nil
	}
	return 0, errors.New("registry down")
}

func TestMeterSizesRetainedImagesAndAccruesThem(t *testing.T) {
	started := time.Date(2026, 8, 1, 0, 3, 0, 0, time.UTC)
	fake := &fakeMeterStore{unsized: []store.SandboxStorageObject{
		{WorkspaceID: "tea-a", ObjectID: "sbx-1", Kind: store.StorageKindRootfs, Ref: "os-1", StartedAt: started},
		{WorkspaceID: "tea-a", ObjectID: "sbx-2", Kind: store.StorageKindRootfs, Ref: "os-2", StartedAt: started},
		// An object with no registry ref cannot be read, and is skipped.
		{WorkspaceID: "tea-a", ObjectID: "snp-1", Kind: store.StorageKindSnapshot, StartedAt: started},
	}}
	reg := prometheus.NewRegistry()
	metrics := NewStorageMetrics(reg)
	fake.stats = store.SandboxStorageStats{Open: 3, UnsizedOpen: 1}
	sizer := &fakeSizer{bytes: map[string]int64{"os-1": 10_000_000_000}} // os-2's registry read fails
	m := &Meter{Store: fake, Sizer: sizer, StorageMetrics: metrics, Now: func() time.Time { return started.Add(time.Hour) }}
	m.settleStorage(context.Background())

	if len(fake.sized) != 1 || fake.sized[0] != "tea-a/sbx-1=10000000000" {
		t.Errorf("sized = %v, want only sbx-1", fake.sized)
	}
	if strings.Join(sizer.calls, ",") != "tea-a/os-1,tea-a/os-2" {
		t.Errorf("registry reads = %v: an object with no ref must not be read", sizer.calls)
	}
	if fake.accrueCalls != 1 {
		t.Errorf("accrual passes = %d, want 1 (a registry failure must not stop the others)", fake.accrueCalls)
	}
	if got := testutil.ToFloat64(metrics.sizeFailures); got != 1 {
		t.Errorf("size failures = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.objects.WithLabelValues("unsized_open")); got != 1 {
		t.Errorf("unsized_open gauge = %v, want 1", got)
	}
}

// A registry that is down must not hold up the poll of every workspace: after
// three consecutive failures the round stops, and the rest wait for the next.
func TestMeterStopsSizingAfterConsecutiveRegistryFailures(t *testing.T) {
	fake := &fakeMeterStore{}
	for _, id := range []string{"1", "2", "3", "4", "5"} {
		fake.unsized = append(fake.unsized, store.SandboxStorageObject{
			WorkspaceID: "tea-a", ObjectID: "sbx-" + id, Kind: store.StorageKindRootfs, Ref: "os-" + id,
		})
	}
	sizer := &fakeSizer{bytes: map[string]int64{}} // every read fails
	m := &Meter{Store: fake, Sizer: sizer, StorageMetrics: NewStorageMetrics(prometheus.NewRegistry())}
	m.settleStorage(context.Background())
	if len(sizer.calls) != storageRegistryFailures {
		t.Errorf("registry reads = %v, want %d before the round gives up", sizer.calls, storageRegistryFailures)
	}
	if fake.accrueCalls != 1 || fake.pruneCalls != 1 {
		t.Errorf("accrual passes %d, prune passes %d: a registry outage must not stop accrual or pruning", fake.accrueCalls, fake.pruneCalls)
	}
	// A success resets the count: two failures, a success, two more all get read.
	sizer2 := &fakeSizer{bytes: map[string]int64{"os-3": 1}}
	m = &Meter{Store: fake, Sizer: sizer2}
	m.settleStorage(context.Background())
	if len(sizer2.calls) != 5 {
		t.Errorf("registry reads = %v, want all five: a success must reset the failure run", sizer2.calls)
	}
}

func TestMeterWithoutASizerNeverSizesButStillAccrues(t *testing.T) {
	fake := &fakeMeterStore{unsized: []store.SandboxStorageObject{
		{WorkspaceID: "tea-a", ObjectID: "sbx-1", Kind: store.StorageKindRootfs, Ref: "os-1"},
	}}
	m := &Meter{Store: fake}
	m.settleStorage(context.Background())
	if len(fake.sized) != 0 || fake.accrueCalls != 1 {
		t.Errorf("sized = %v accrual passes = %d: with no sizer nothing is sized and an already-sized object still accrues", fake.sized, fake.accrueCalls)
	}
}
