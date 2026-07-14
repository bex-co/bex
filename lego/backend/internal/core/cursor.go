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
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// The SELF-CARRYING keyset cursor, shared by every list whose rows cannot be
// resumed from by id (w3/m7 events, w3/m11 webhook deliveries). Most bex lists
// echo a row's ID back as the cursor (deploys, audit, env-vars); that pattern
// fails for two classes of list:
//
//  1. There is no row to look up. A composed feed's entry id is a hash of its
//     source row (ids.Derive) — nothing the store can turn back into "the
//     (timestamp, key) to page after".
//  2. The row may be GONE or CHANGED under the reader. A retention sweep
//     (audit_events) purges rows between two pages; a delivery row is updated
//     in place by retries. An id-as-cursor keyset subquery then silently
//     returns an empty page — pagination that stops early and looks like the
//     end of the list. A cursor that CARRIES its (at, key) keeps paging
//     correctly regardless.
//
// So the cursor is the keyset itself, base64url-encoded to stay opaque — which
// is exactly what Render promises a cursor is ("the cursor of the last
// resource returned"), and what its own cursors look like on the wire.

// cursorSep separates the two keyset components. Safe: an RFC3339Nano
// timestamp contains no '|', and row keys are over the id alphabet (plus ':').
const cursorSep = "|"

// KeysetCursor is a decoded cursor — the position to resume strictly after.
// The zero value is the head of the list.
type KeysetCursor struct {
	At  time.Time
	Key string
}

// EncodeKeysetCursor renders a row's keyset as the opaque cursor a client
// echoes back. Nanosecond precision: two rows in the same microsecond must
// still order and resume deterministically, and the Key tiebreak only helps if
// the timestamp it is paired with round-trips exactly.
func EncodeKeysetCursor(at time.Time, key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + cursorSep + key))
}

// DecodeKeysetCursor parses a cursor back into its keyset. An empty cursor is
// the head of the list (zero keyset), not an error. A malformed one is
// ErrBadRequest (400) rather than a silently empty page: a client that garbles
// its cursor is a bug worth surfacing, and unlike an id-as-cursor there is no
// legitimate "unknown cursor" case — the value is self-describing, so if it
// doesn't parse, it was never one of ours.
func DecodeKeysetCursor(cursor string) (KeysetCursor, error) {
	if cursor == "" {
		return KeysetCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return KeysetCursor{}, fmt.Errorf("%w: malformed cursor", ErrBadRequest)
	}
	at, key, ok := strings.Cut(string(raw), cursorSep)
	if !ok {
		return KeysetCursor{}, fmt.Errorf("%w: malformed cursor", ErrBadRequest)
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return KeysetCursor{}, fmt.Errorf("%w: malformed cursor", ErrBadRequest)
	}
	return KeysetCursor{At: t, Key: key}, nil
}
