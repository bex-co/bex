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

// ParseTime parses one optional RFC3339 field value and returns a named bad
// request for malformed input — QueryTime's string-argument core, for the
// GraphQL/MCP adapters that receive strings rather than query values. A
// malformed value must never be silently ignored: falling back to a default
// widens the effective window past caps like BEX_MAX_QUERY_HOURS.
//
// The message carries a worked example because a caller who spelled a
// timestamp wrong has no other way to see the expected shape.
func ParseTime(field, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be an RFC3339 timestamp (e.g. 2026-01-02T15:04:05Z)", ErrBadRequest, field)
	}
	return parsed, nil
}

// QueryTime parses one optional RFC3339 list-filter boundary and returns a
// named bad request for malformed input, so adapters never silently ignore it.
func QueryTime(q url.Values, key string) (time.Time, error) {
	return ParseTime(key, q.Get(key))
}

// CheckQueryWindow enforces a BEX_MAX_QUERY_HOURS-style cap on a query's
// start–end range — shared by the logs, events, and metrics services so the
// window budget cannot drift per feature (w9/004 → codex r7 #13). maxHours
// <= 0 disables the cap; an open start is unbounded history and passes; an
// open end is measured against now.
func CheckQueryWindow(maxHours int, now func() time.Time, start, end time.Time) error {
	if maxHours <= 0 || start.IsZero() {
		return nil
	}
	if end.IsZero() {
		end = now()
	}
	if end.Sub(start) > time.Duration(maxHours)*time.Hour {
		return fmt.Errorf("%w: query range exceeds %d hours", ErrBadRequest, maxHours)
	}
	return nil
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

// ErrLimitNotPositive is the 400 for a `limit` the caller explicitly supplied
// as zero or a negative number.
//
// Which of the two limit policies here a list wants is decided by ONE question:
// does its store read limit <= 0 as "absent" and substitute a cap of its own?
// If so it must REJECT (QueryLimit / this sentinel) — letting the value through
// answers with the full cap dressed as the page the caller asked for, and the
// response looks identical either way, so nothing surfaces the mistake. If
// instead the adapter substitutes a real default, CLAMP (PageParams/PageLimit).
//
// Only a surface that can still see PRESENCE may reject an explicit zero: by
// the time a list filter holds an int, zero is also how "omitted" arrives.
var ErrLimitNotPositive = fmt.Errorf("%w: limit must be a positive integer", ErrBadRequest)

// QueryLimit parses the `limit` query param for a REJECT-policy list (see
// ErrLimitNotPositive). Absent stays 0 — the store's own cap applies — while a
// present-but-unparseable or non-positive value is a 400 rather than a silent
// fall-back to a default page.
//
// This is the deliberate counterpart to PageParams, not a duplicate of it:
// PageParams' absent-means-DefaultPageLimit and unparseable-means-default
// readings are both wrong for these lists.
func QueryLimit(q url.Values) (int, error) {
	v := q.Get("limit")
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, ErrLimitNotPositive
	}
	return n, nil
}

// PageParams parses Render's `cursor` + `limit` query params. after is the
// opaque cursor to resume after (empty ⇒ first page); limit is clamped to
// [1, MaxPageLimit], defaulting to DefaultPageLimit when absent or unparseable.
// This is the CLAMP policy; see ErrLimitNotPositive for when to reject instead.
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

// PageLimitOrDefault is PageLimit for a typed adapter, where 0 means the
// argument was OMITTED rather than set to zero: absent ⇒ DefaultPageLimit, and
// anything else clamps through PageLimit.
//
// It exists because PageLimit alone is the wrong function for that job and
// fails quietly. PageLimit(0) is 1 — correct for a query string, where
// PageParams has already substituted the default before clamping, and wrong
// for an MCP/GraphQL int, where 0 IS the absent value. A list that reached for
// PageLimit directly therefore served one item per page while its own schema
// advertised twenty, and paged correctly, so only the page size gave it away
// (found in list_environments). Three sibling MCP tools had already hand-rolled
// this two-line branch to avoid that; this is the seam they were reinventing.
//
// Deliberately keyed on == 0, not < 1. The `pageLimit` helpers in events and
// apps look like more copies but treat any value below one — negatives
// included — as absent, which is a different and more forgiving rule; folding
// them in would silently turn their negative-limit answer from a default page
// into a one-item page. gqlutil.Page is likewise left alone: it already
// detects absence from the ARGUMENT MAP, which is strictly better than
// inferring it from a zero, and switching it would change what an explicit
// `limit: 0` returns.
func PageLimitOrDefault(limit int) int {
	if limit == 0 {
		return DefaultPageLimit
	}
	return PageLimit(limit)
}

// PageLimit clamps a caller-supplied page size to Render's public bounds.
// Adapters with typed arguments (GraphQL/MCP) use this to apply the same
// semantics as PageParams without round-tripping through URL strings. When 0
// means "omitted" rather than "zero", use PageLimitOrDefault instead.
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
