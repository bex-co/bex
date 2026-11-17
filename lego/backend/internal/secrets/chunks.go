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
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// chunks.go keeps a map whose encoded form would not fit one OpenBao entry
// out of that one entry (w4/m363 t009). OpenBao's integrated raft refuses a
// log entry over max_entry_size (1 MiB by default, unset in production), and
// a KV v2 write of a map is one entry: the JSON-encoded map plus version
// metadata. A secret-files map may hold 1 MiB of contents (Render's 1 MB,
// Kubernetes' MaxSecretSize), so stored inline it ran about 254 bytes past
// that cap and answered 500 below the stated bound.
//
// When a map's write body would pass maxInlineEntryBytes, its largest values
// move to chunk entries under <path>.chunks/<nonce>/<value>-<chunk>, each
// holding at most chunkRawBytes of the value, base64-encoded so JSON escaping
// can never grow it. The map entry keeps every key — offloaded ones with an
// empty value — plus chunkManifestKey, naming the nonce and each offloaded
// key's chunk count. Reads put the values back, so callers never see the
// layout.
//
// Chunks are written before the map entry that names them, under a nonce no
// other write uses, so a compare-and-set that loses leaves the current map
// untouched (its chunks are dropped). After a write lands, the chunks of the
// version it replaced are deleted: nothing reads a non-current version. A
// read that races that deletion re-reads the map. Delete and Retire remove
// every chunk under the path.
const (
	// maxInlineEntryBytes bounds a map entry's write body; anything larger
	// offloads values. Half the raft entry cap: the entry also carries KV v2
	// metadata, and OpenBao re-encodes the map with the same escaping.
	maxInlineEntryBytes = 512 << 10
	// chunkRawBytes is one chunk's raw share of a value: base64 makes it
	// 512 KiB, inside one entry with room to spare.
	chunkRawBytes = 384 << 10
	// chunkManifestKey cannot be a file name or an environment key (both
	// refuse NUL), so it never collides with a caller's key.
	chunkManifestKey = "\x00bex-chunks"
	chunkDirSuffix   = ".chunks"
	// chunkReadAttempts bounds the re-reads of a map whose chunks a
	// concurrent write deleted between the map read and the chunk read.
	chunkReadAttempts = 3
)

// chunkManifest is chunkManifestKey's value.
type chunkManifest struct {
	Version int            `json:"v"`
	Nonce   string         `json:"n"`
	Values  []chunkedValue `json:"k"`
}

type chunkedValue struct {
	Key    string `json:"key"`
	Chunks int    `json:"chunks"`
}

// chunkEntry is one chunk write: its logical path and its encoded share.
type chunkEntry struct {
	path string
	data string
}

var errChunkMissing = errors.New("openbao chunk missing")

func chunkRoot(path string) string { return path + chunkDirSuffix }

func chunkDir(path, nonce string) string { return chunkRoot(path) + "/" + nonce }

func chunkPath(path, nonce string, value, chunk int) string {
	return chunkDir(path, nonce) + "/" + strconv.Itoa(value) + "-" + strconv.Itoa(chunk)
}

// jsonLen is the length of v's JSON encoding.
func jsonLen(v string) int {
	b, _ := json.Marshal(v)
	return len(b)
}

// planChunks returns the map to store at path and the chunk entries to write
// first. A map whose write body fits maxInlineEntryBytes is returned as is,
// with no chunks.
func planChunks(path string, data map[string]string) (map[string]string, []chunkEntry, string, error) {
	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return nil, nil, "", err
	}
	if len(body) <= maxInlineEntryBytes {
		return data, nil, "", nil
	}
	nonceBytes := make([]byte, 12)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, nil, "", err
	}
	nonce := hex.EncodeToString(nonceBytes)

	type sized struct {
		key  string
		size int
	}
	keys := make([]sized, 0, len(data))
	for k, v := range data {
		keys = append(keys, sized{k, jsonLen(v)})
	}
	slices.SortFunc(keys, func(a, b sized) int {
		return cmp.Or(cmp.Compare(b.size, a.size), strings.Compare(a.key, b.key))
	})

	stored := maps.Clone(data)
	manifest := chunkManifest{Version: 1, Nonce: nonce}
	var chunks []chunkEntry
	// remaining tracks the stored body: each offloaded value leaves "" behind
	// and adds its key and count to the manifest, which is itself one value.
	remaining := len(body) + jsonLen(chunkManifestKey) + 64
	for _, k := range keys {
		if remaining <= maxInlineEntryBytes {
			break
		}
		value, index := data[k.key], len(manifest.Values)
		n := 0
		for start := 0; start < len(value); start += chunkRawBytes {
			end := min(start+chunkRawBytes, len(value))
			chunks = append(chunks, chunkEntry{
				path: chunkPath(path, nonce, index, n),
				data: base64.StdEncoding.EncodeToString([]byte(value[start:end])),
			})
			n++
		}
		stored[k.key] = ""
		manifest.Values = append(manifest.Values, chunkedValue{Key: k.key, Chunks: n})
		// The manifest is JSON inside a JSON string: its key is escaped twice.
		remaining -= k.size - 2
		remaining += 2*jsonLen(k.key) + 32
	}
	m, err := json.Marshal(manifest)
	if err != nil {
		return nil, nil, "", err
	}
	stored[chunkManifestKey] = string(m)
	return stored, chunks, nonce, nil
}

// writeMap writes data at path through write (a plain or compare-and-set
// data write that returns the version it created), offloading values first
// when the map would not fit one entry.
func (s *openBaoStore) writeMap(ctx context.Context, path string, data map[string]string, write func(map[string]string) (uint64, error)) (uint64, error) {
	stored, chunks, nonce, err := planChunks(path, data)
	if err != nil {
		return 0, err
	}
	for _, c := range chunks {
		body, _ := json.Marshal(map[string]any{"data": map[string]string{"c": c.data}})
		if err := s.kv(ctx, http.MethodPost, s.dataURL(ctx, c.path), body, nil); err != nil {
			_ = s.deleteChunkDir(ctx, path, nonce)
			return 0, sanitizeVersionedStoreError("write", err)
		}
	}
	version, err := write(stored)
	if err != nil {
		if nonce != "" {
			_ = s.deleteChunkDir(ctx, path, nonce)
		}
		return 0, err
	}
	s.dropReplacedChunks(ctx, path, version, nonce)
	return version, nil
}

// dropReplacedChunks deletes the chunks the version before version named, if
// any. Best effort: a failure leaves orphans that Delete or Retire removes.
func (s *openBaoStore) dropReplacedChunks(ctx context.Context, path string, version uint64, nonce string) {
	if version <= 1 {
		return
	}
	var out struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	url := s.dataURL(ctx, path) + "?version=" + strconv.FormatUint(version-1, 10)
	if err := s.kv(ctx, http.MethodGet, url, nil, &out); err != nil {
		return
	}
	if m, ok := parseManifest(out.Data.Data); ok && m.Nonce != nonce {
		_ = s.deleteChunkDir(ctx, path, m.Nonce)
	}
}

// parseManifest reads the manifest a stored map carries, if it carries one.
func parseManifest(stored map[string]string) (chunkManifest, bool) {
	raw, ok := stored[chunkManifestKey]
	if !ok {
		return chunkManifest{}, false
	}
	var m chunkManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil || m.Version != 1 || m.Nonce == "" {
		return chunkManifest{}, false
	}
	return m, true
}

// assemble returns the caller's map for a stored one: offloaded values read
// back from their chunks, the manifest key removed. A map with no manifest is
// returned unchanged.
func (s *openBaoStore) assemble(ctx context.Context, path string, stored map[string]string) (map[string]string, error) {
	m, ok := parseManifest(stored)
	if !ok {
		return stored, nil
	}
	out := maps.Clone(stored)
	delete(out, chunkManifestKey)
	for i, v := range m.Values {
		var b strings.Builder
		for n := range v.Chunks {
			var chunk struct {
				Data struct {
					Data map[string]string `json:"data"`
				} `json:"data"`
			}
			err := s.kv(ctx, http.MethodGet, s.dataURL(ctx, chunkPath(path, m.Nonce, i, n)), nil, &chunk)
			if isNotFound(err) {
				return nil, errChunkMissing
			}
			if err != nil {
				return nil, err
			}
			raw, err := base64.StdEncoding.DecodeString(chunk.Data.Data["c"])
			if err != nil {
				return nil, core.Err("openbao chunk is not base64")
			}
			b.Write(raw)
		}
		out[v.Key] = b.String()
	}
	return out, nil
}

// readAssembled reads path with read and assembles it, re-reading when a
// concurrent write deleted the chunks between the two reads.
func (s *openBaoStore) readAssembled(ctx context.Context, path string, read func() (map[string]string, uint64, error)) (map[string]string, uint64, error) {
	for attempt := 1; ; attempt++ {
		stored, version, err := read()
		if err != nil {
			return nil, 0, err
		}
		out, err := s.assemble(ctx, path, stored)
		if errors.Is(err, errChunkMissing) && attempt < chunkReadAttempts {
			continue
		}
		if err != nil {
			return nil, 0, err
		}
		return out, version, nil
	}
}

// deleteChunkDir deletes every chunk one write stored under path.
func (s *openBaoStore) deleteChunkDir(ctx context.Context, path, nonce string) error {
	dir := chunkDir(path, nonce)
	names, err := s.List(ctx, dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.deleteEntry(ctx, dir+"/"+name); err != nil {
			return err
		}
	}
	return nil
}

// deleteAllChunks deletes every chunk stored under path, whichever write
// stored it.
func (s *openBaoStore) deleteAllChunks(ctx context.Context, path string) error {
	nonces, err := s.List(ctx, chunkRoot(path))
	if err != nil {
		return err
	}
	for _, nonce := range nonces {
		if err := s.deleteChunkDir(ctx, path, nonce); err != nil {
			return fmt.Errorf("deleting chunks: %w", err)
		}
	}
	return nil
}
