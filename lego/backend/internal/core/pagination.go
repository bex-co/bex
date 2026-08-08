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
	"cmp"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// QueryList parses Render's form/explode-false list filters while also
// accepting repeated query keys. Values are trimmed and deduplicated in first
// occurrence order, giving OR alternatives within one filter dimension.
func QueryList(q url.Values, key string) []string {
	var out []string
	for _, raw := range q[key] {
		for value := range strings.SplitSeq(raw, ",") {
			value = strings.TrimSpace(value)
			if value != "" && !slices.Contains(out, value) {
				out = append(out, value)
			}
		}
	}
	return out
}

// QueryTime parses one optional RFC3339 list-filter boundary and returns a
// named bad request for malformed input, so adapters never silently ignore it.
func QueryTime(q url.Values, key string) (time.Time, error) {
	raw := q.Get(key)
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be an RFC3339 timestamp", ErrBadRequest, key)
	}
	return parsed, nil
}

// TimeWindow is one Render `<field>Before`/`<field>After` list-filter range.
// The zero value admits everything.
type TimeWindow struct{ Before, After time.Time }

// QueryTimeWindow parses one before/after RFC3339 pair — Render's
// createdBefore/createdAfter shape — with QueryTime's named bad request.
func QueryTimeWindow(q url.Values, beforeKey, afterKey string) (TimeWindow, error) {
	before, err := QueryTime(q, beforeKey)
	if err != nil {
		return TimeWindow{}, err
	}
	after, err := QueryTime(q, afterKey)
	if err != nil {
		return TimeWindow{}, err
	}
	return TimeWindow{Before: before, After: after}, nil
}

// Contains reports whether an RFC3339 timestamp falls inside the window
// (exclusive on both ends, matching Render). A timestamp this cannot place —
// empty on records pre-dating timestamp stamping, or unparseable — is admitted
// rather than silently dropped: omitted data does not imply exclusion (w2/m51).
func (w TimeWindow) Contains(raw string) bool {
	if w.Before.IsZero() && w.After.IsZero() {
		return true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return true
	}
	return (w.Before.IsZero() || value.Before(w.Before)) && (w.After.IsZero() || value.After(w.After))
}

// ParseEnum validates one optional list-filter enum, returning it unchanged
// when empty (absent ⇒ unfiltered) or allowed, and a named bad request
// otherwise. Adapters with typed arguments (GraphQL/MCP) share it with REST so
// one vocabulary and one 400 message serve every surface.
func ParseEnum(key, value string, allowed ...string) (string, error) {
	if value == "" || slices.Contains(allowed, value) {
		return value, nil
	}
	return "", fmt.Errorf("%w: unknown %s %q (want %s)", ErrBadRequest, key, value, strings.Join(allowed, "|"))
}

// Filter returns the items satisfying keep, in order. It allocates a new slice
// rather than compacting in place so a caller's input is never mutated.
func Filter[T any](items []T, keep func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

// Render's `direction` ordering enum, shared by every list endpoint that
// honors it (logs and audit logs): backward is newest-first (the default),
// forward starts with the oldest item in the window.
const (
	DirectionBackward = "backward"
	DirectionForward  = "forward"
)

// ParseDirection validates Render's `direction` param — one vocabulary and
// one 400 message for every surface, so a second direction-honoring list
// can't drift from the first ("nothing accepted is ignored", w3/m8).
// "" or DirectionBackward is newest-first (oldestFirst=false),
// DirectionForward oldest-first; anything else is a named ErrBadRequest.
func ParseDirection(direction string) (oldestFirst bool, err error) {
	if direction != "" && direction != DirectionBackward && direction != DirectionForward {
		return false, fmt.Errorf("%w: unknown direction %q (want %s|%s)",
			ErrBadRequest, direction, DirectionBackward, DirectionForward)
	}
	return direction == DirectionForward, nil
}

// Render's cursor-pagination contract for list endpoints
// (docs/render-artifacts/owners-api.md, pagination.md): the `limit` query
// param defaults to 20 and is clamped to [1, 100]; `cursor` is the opaque
// position of the last item from the prior page.
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

// PageParams parses Render's `cursor` + `limit` query params. after is the
// opaque cursor to resume after (empty ⇒ first page); limit is clamped to
// [1, MaxPageLimit], defaulting to DefaultPageLimit when absent or unparseable.
func PageParams(q url.Values) (after string, limit int) {
	after = q.Get("cursor")
	limit = DefaultPageLimit
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	return after, PageLimit(limit)
}

// PageLimit clamps a caller-supplied page size to Render's public bounds.
// Adapters with typed arguments (GraphQL/MCP) use this to apply the same
// semantics as PageParams without round-tripping through URL strings.
func PageLimit(limit int) int {
	switch {
	case limit < 1:
		limit = 1
	case limit > MaxPageLimit:
		limit = MaxPageLimit
	}
	return limit
}

// Page returns the window of items after the `after` cursor (exclusive; empty
// starts at the head), capped at limit. cursorOf yields each item's opaque
// cursor — the value a client echoes back as `cursor` to fetch the next page.
// An `after` that matches no item yields an empty page (Render's end-of-list
// behavior: a shorter/empty page signals the client to stop). Implements the
// contract in docs/render-artifacts/owners-api.md; a nil/empty input is
// returned unchanged.
func Page[T any](items []T, after string, limit int, cursorOf func(T) string) []T {
	start := 0
	if after != "" {
		start = len(items) // unknown cursor ⇒ empty tail
		for i, it := range items {
			if cursorOf(it) == after {
				start = i + 1
				break
			}
		}
	}
	if start >= len(items) {
		return items[len(items):]
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// StablePage applies cursor pagination only when requested. An omitted
// cursor+limit returns items unchanged, preserving pre-pagination clients'
// complete-list response and ordering. A requested page is sorted by its
// stable unique cursor first, so separate requests cannot duplicate or skip an
// item merely because the backing store returned a different list order.
//
// Callers normalize limit with PageParams or PageLimit before calling.
func StablePage[T any](items []T, after string, limit int, requested bool, cursorOf func(T) string) []T {
	if !requested {
		return items
	}
	ordered := slices.Clone(items)
	slices.SortStableFunc(ordered, func(a, b T) int {
		return cmp.Compare(cursorOf(a), cursorOf(b))
	})
	return Page(ordered, after, limit, cursorOf)
}
