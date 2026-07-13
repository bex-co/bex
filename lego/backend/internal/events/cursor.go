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

package events

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// cursor.go is the feed's pagination key. Every other bex list echoes a row's ID
// back as its cursor (deploys, audit, env-vars) — this one cannot, for two
// reasons that are properties of a COMPOSED feed:
//
//  1. There is no row to resume from. An event's id is a hash of its source row
//     (ids.Derive), and a hash cannot be turned back into "the (timestamp, key)
//     to page after". The id-as-cursor pattern works elsewhere because the store
//     can look the id up ("… < (SELECT at, id FROM audit_events WHERE id = $1)");
//     here the two sources have different key spaces and one of them yields two
//     events per row, so there is nothing to look up.
//  2. The row may be GONE. audit_events is swept by retention
//     (BEX_AUDIT_RETENTION_DAYS); an id-as-cursor whose row was purged between two
//     pages matches nothing, and a keyset subquery on it silently returns an EMPTY
//     page — pagination that stops early and looks like the end of the feed. A
//     cursor that CARRIES its (at, key) keeps paging correctly across a purge.
//
// So the cursor is the keyset itself, base64url-encoded to stay opaque — which is
// exactly what Render promises a cursor is ("the cursor of the last resource
// returned"), and what its own cursors look like on the wire.

// cursorSep separates the two keyset components. Safe: an RFC3339Nano timestamp
// contains no '|', and a row key is "<id>:<phase>" over the id alphabet.
const cursorSep = "|"

// keyset is a decoded cursor — the position to resume strictly after.
type keyset struct {
	At  time.Time
	Key string
}

// encodeCursor renders a row's keyset as the opaque cursor a client echoes back.
// Nanosecond precision: two events in the same microsecond must still order and
// resume deterministically, and the Key tiebreak only helps if the timestamp it
// is paired with round-trips exactly.
func encodeCursor(at time.Time, key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + cursorSep + key))
}

// decodeCursor parses a cursor back into its keyset. An empty cursor is the head
// of the feed (zero keyset), not an error. A malformed one is core.ErrBadRequest
// (400) rather than a silently empty page: a client that garbles its cursor is a
// bug worth surfacing, and unlike an id-as-cursor there is no legitimate
// "unknown cursor" case here — the value is self-describing, so if it doesn't
// parse, it was never one of ours.
func decodeCursor(cursor string) (keyset, error) {
	if cursor == "" {
		return keyset{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return keyset{}, fmt.Errorf("%w: malformed cursor", core.ErrBadRequest)
	}
	at, key, ok := strings.Cut(string(raw), cursorSep)
	if !ok {
		return keyset{}, fmt.Errorf("%w: malformed cursor", core.ErrBadRequest)
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return keyset{}, fmt.Errorf("%w: malformed cursor", core.ErrBadRequest)
	}
	return keyset{At: t, Key: key}, nil
}
