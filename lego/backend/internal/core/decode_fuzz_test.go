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

package core

import (
	"testing"
	"time"
)

// seedCursorTime is a fixed instant for cursor seeds — deterministic so the
// corpus never depends on wall-clock time.
var seedCursorTime = time.Unix(1_700_000_000, 0).UTC()

// The opaque decoders below are fed straight from untrusted REST input — a
// list endpoint's `cursor` query param and a concurrency token in a request
// body or header. They must reject any malformed value with an error, never
// panic on a truncated base64 payload, a short byte run, or a garbled keyset.

// FuzzDecodeKeysetCursor drives the self-carrying list cursor decoder with
// arbitrary client-supplied strings. It must never panic; a value it accepts
// must survive a re-encode/re-decode round-trip so the resume position a client
// echoes back is stable.
func FuzzDecodeKeysetCursor(f *testing.F) {
	f.Add("")
	f.Add("not-base64!!")
	f.Add(EncodeKeysetCursor(seedCursorTime, "evt-000000000000000000000"))
	f.Add(EncodeKeysetCursor(seedCursorTime, "key|with|separators"))
	f.Add("MjAyNi0wMS0wMlQxNTowNDowNVo") // base64 of a timestamp with no separator
	f.Fuzz(func(t *testing.T, cursor string) {
		decoded, err := DecodeKeysetCursor(cursor)
		if err != nil {
			return
		}
		// A cursor we accepted must round-trip: re-encoding its keyset and
		// decoding again yields the same resume position.
		again, err := DecodeKeysetCursor(EncodeKeysetCursor(decoded.At, decoded.Key))
		if err != nil {
			t.Fatalf("re-decode of a re-encoded accepted cursor %q failed: %v", cursor, err)
		}
		if !again.At.Equal(decoded.At) || again.Key != decoded.Key {
			t.Fatalf("cursor keyset did not round-trip: %+v -> %+v", decoded, again)
		}
	})
}

// FuzzDecodeOpaqueRevision drives the namespaced concurrency-token decoder with
// arbitrary strings under a fixed prefix. It must never panic; a token it
// accepts must re-encode to the exact canonical form for that prefix+version.
func FuzzDecodeOpaqueRevision(f *testing.F) {
	const prefix = "rev-"
	f.Add("")
	f.Add(prefix)
	f.Add("rev-short")
	f.Add(EncodeOpaqueRevision(prefix, 0))
	f.Add(EncodeOpaqueRevision(prefix, 1<<63))
	f.Fuzz(func(t *testing.T, token string) {
		version, ok := DecodeOpaqueRevision(prefix, token)
		if !ok {
			return
		}
		if canonical := EncodeOpaqueRevision(prefix, version); canonical != token {
			t.Fatalf("accepted a non-canonical revision token %q (canonical %q)", token, canonical)
		}
	})
}
