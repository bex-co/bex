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
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// raftKV is a KV v2 fake with OpenBao integrated raft's entry cap: a data
// write whose body passes raftEntryCap answers 500, as production's
// openbao did for a files map holding 1 MiB of contents (w4/m363 t009:
// 1,048,322 bytes stored, 1,048,400 refused). It keeps versions, honours
// cas and ?version=, lists, and deletes metadata.
type raftKV struct {
	mu      sync.Mutex
	entries map[string]*raftEntry
}

type raftEntry struct {
	versions map[uint64]map[string]string
	current  uint64
}

// raftEntryCap is the request-body bound the fake applies. The real cap is
// on the raft log entry, which carries the same JSON plus metadata, so the
// body bound is a lenient stand-in: anything the fake refuses, OpenBao does.
const raftEntryCap = 1 << 20

func newRaftKV(t *testing.T) (*openBaoStore, *raftKV) {
	t.Helper()
	kv := &raftKV{entries: map[string]*raftEntry{}}
	srv := httptest.NewServer(kv)
	t.Cleanup(srv.Close)
	return &openBaoStore{addr: srv.URL, mount: baoMount, client: srv.Client(), token: "t", tokenExp: time.Now().Add(time.Hour)}, kv
}

func (kv *raftKV) chunkEntries() int {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	n := 0
	for p, e := range kv.entries {
		if strings.Contains(p, chunkDirSuffix+"/") && e.current > 0 {
			n++
		}
	}
	return n
}

func (kv *raftKV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	rest := strings.TrimPrefix(r.URL.Path, "/v1/"+baoMount+"/")
	kind, path, _ := strings.Cut(rest, "/")
	switch {
	case kind == "data" && r.Method == http.MethodGet:
		e := kv.entries[path]
		if e == nil || e.current == 0 {
			http.NotFound(w, r)
			return
		}
		v := e.current
		if q := r.URL.Query().Get("version"); q != "" {
			v, _ = strconv.ParseUint(q, 10, 64)
		}
		data, ok := e.versions[v]
		if !ok {
			http.NotFound(w, r)
			return
		}
		core.WriteJSON(w, 200, map[string]any{"data": map[string]any{"data": data, "metadata": map[string]any{"version": v}}})
	case kind == "data" && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		if len(body) > raftEntryCap {
			http.Error(w, "raft: entry too large", http.StatusInternalServerError)
			return
		}
		var in struct {
			Options *struct {
				CAS uint64 `json:"cas"`
			} `json:"options"`
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e := kv.entries[path]
		if e == nil {
			e = &raftEntry{versions: map[uint64]map[string]string{}}
			kv.entries[path] = e
		}
		if in.Options != nil && in.Options.CAS != e.current {
			http.Error(w, "check-and-set parameter did not match", http.StatusBadRequest)
			return
		}
		e.current++
		e.versions[e.current] = in.Data
		core.WriteJSON(w, 200, map[string]any{"data": map[string]any{"version": e.current}})
	case kind == "metadata" && r.Method == http.MethodGet && r.URL.Query().Get("list") == "true":
		prefix := path + "/"
		seen := map[string]bool{}
		for p, e := range kv.entries {
			if e.current == 0 || !strings.HasPrefix(p, prefix) {
				continue
			}
			child, sub, nested := strings.Cut(strings.TrimPrefix(p, prefix), "/")
			if nested && sub != "" {
				child += "/"
			}
			seen[child] = true
		}
		if len(seen) == 0 {
			http.NotFound(w, r)
			return
		}
		keys := make([]string, 0, len(seen))
		for k := range seen {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		core.WriteJSON(w, 200, map[string]any{"data": map[string]any{"keys": keys}})
	case kind == "metadata" && r.Method == http.MethodDelete:
		delete(kv.entries, path)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unsupported", http.StatusMethodNotAllowed)
	}
}

// filesAtBound is a files map whose contents total exactly
// corev1.MaxSecretSize: a 614,400-byte file (the DoD's) and a filler. pem
// makes the filler a PEM-like bundle whose newlines and quotes JSON escaping
// grows.
func filesAtBound(pem bool) map[string]string {
	big := strings.Repeat("a", 614400)
	n := corev1.MaxSecretSize - len(big)
	var fill string
	if pem {
		line := `MIIB"x\` + strings.Repeat("Q", 56) + "\n"
		fill = strings.Repeat(line, n/len(line)+1)[:n]
	} else {
		fill = strings.Repeat("b", n)
	}
	return map[string]string{"qa-big1.txt": big, "qa-fill.txt": fill}
}

// TestFilesMapAtTheBoundFitsTheRaftEntryCap is t009's regression: a files map
// at exactly the stated 1 MiB of contents, plain or PEM-like, is stored and
// read back whole through a store with raft's entry cap, by both writers.
func TestFilesMapAtTheBoundFitsTheRaftEntryCap(t *testing.T) {
	for _, pem := range []bool{false, true} {
		t.Run("pem="+strconv.FormatBool(pem), func(t *testing.T) {
			s, _ := newRaftKV(t)
			ctx := withTenant(context.Background(), "tea-x")
			files := filesAtBound(pem)
			if got := contentBytes(files); got != corev1.MaxSecretSize {
				t.Fatalf("fixture holds %d bytes, want %d", got, corev1.MaxSecretSize)
			}
			if err := s.Put(ctx, filesPath("web"), files); err != nil {
				t.Fatalf("Put at the bound: %v", err)
			}
			snap, err := s.GetVersioned(ctx, filesPath("web"))
			if err != nil {
				t.Fatalf("GetVersioned: %v", err)
			}
			if !maps.Equal(snap.Data, files) {
				t.Fatalf("read back %d keys / %d bytes, want the map written", len(snap.Data), contentBytes(snap.Data))
			}
			if _, err := s.PutCAS(ctx, filesPath("web"), files, snap.Version); err != nil {
				t.Fatalf("PutCAS at the bound: %v", err)
			}
			got, err := s.Get(ctx, filesPath("web"))
			if err != nil || !maps.Equal(got, files) {
				t.Fatalf("Get after PutCAS: %d bytes, err=%v", contentBytes(got), err)
			}
		})
	}
}

// TestChunkedMapLifecycle pins the layout's bookkeeping: a write that
// replaces a chunked map drops the replaced chunks, a losing compare-and-set
// leaves the current map and its chunks alone and stores none of its own,
// a small map goes back inline, and Delete removes every chunk.
func TestChunkedMapLifecycle(t *testing.T) {
	s, kv := newRaftKV(t)
	ctx := withTenant(context.Background(), "tea-x")
	path := filesPath("web")

	files := filesAtBound(false)
	v1, err := s.PutCAS(ctx, path, files, 0)
	if err != nil {
		t.Fatal(err)
	}
	first := kv.chunkEntries()
	if first == 0 {
		t.Fatal("a 1 MiB map stored no chunks")
	}

	// A stale writer loses and leaves nothing behind.
	if _, err := s.PutCAS(ctx, path, filesAtBound(true), v1+1); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("stale PutCAS: %v, want conflict", err)
	}
	if got := kv.chunkEntries(); got != first {
		t.Fatalf("after a lost CAS %d chunk entries, want %d", got, first)
	}

	// Replacing it keeps only the new write's chunks.
	files["qa-big1.txt"] = strings.Repeat("c", 614400)
	v2, err := s.PutCAS(ctx, path, files, v1)
	if err != nil {
		t.Fatal(err)
	}
	if got := kv.chunkEntries(); got != first {
		t.Fatalf("after a replacing write %d chunk entries, want %d", got, first)
	}
	got, err := s.Get(ctx, path)
	if err != nil || !maps.Equal(got, files) {
		t.Fatalf("read after replace: err=%v", err)
	}

	// A small map is stored inline and the old chunks go.
	small := map[string]string{"app.conf": "x=1\n"}
	if _, err := s.PutCAS(ctx, path, small, v2); err != nil {
		t.Fatal(err)
	}
	if got := kv.chunkEntries(); got != 0 {
		t.Fatalf("an inline map left %d chunk entries", got)
	}
	if got, err := s.Get(ctx, path); err != nil || !maps.Equal(got, small) {
		t.Fatalf("inline read: %v err=%v", got, err)
	}

	// Delete takes chunks with it.
	if err := s.Put(ctx, path, files); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, path); err != nil {
		t.Fatal(err)
	}
	if got := kv.chunkEntries(); got != 0 {
		t.Fatalf("Delete left %d chunk entries", got)
	}
	if got, err := s.Get(ctx, path); err != nil || len(got) != 0 {
		t.Fatalf("after Delete: %d keys err=%v", len(got), err)
	}
}

// TestFilesMapAtTheBoundOnRealOpenBao drives the production adapter against a
// real OpenBao (opt-in, like the batch suite). Against integrated raft
// storage — a local raft server, not CI's in-memory dev one — it is t009's
// live failure: without chunking the map at the bound answers 500. Service
// and environment-group maps share the adapter; both paths are written.
func TestFilesMapAtTheBoundOnRealOpenBao(t *testing.T) {
	bao := realBatchBao(t)
	ctx := withTenant(context.Background(), "tea-m363")
	for _, path := range []string{filesPath("web"), "env-groups/evg-m363/files"} {
		for _, pem := range []bool{false, true} {
			files := filesAtBound(pem)
			snap, err := bao.GetVersioned(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := bao.PutCAS(ctx, path, files, snap.Version); err != nil {
				t.Fatalf("%s pem=%v: PutCAS at the bound: %v", path, pem, err)
			}
			got, err := bao.Get(ctx, path)
			if err != nil || !maps.Equal(got, files) {
				t.Fatalf("%s pem=%v: read back %d bytes, err=%v", path, pem, contentBytes(got), err)
			}
		}
		if err := bao.Retire(ctx, path); err != nil {
			t.Fatalf("Retire: %v", err)
		}
		if left, err := bao.List(ctx, chunkRoot(path)); err != nil || len(left) != 0 {
			t.Fatalf("%s: chunks left after Retire: %v err=%v", path, left, err)
		}
	}
}
