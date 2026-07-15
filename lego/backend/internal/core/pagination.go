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
	"net/url"
	"slices"
	"strconv"
)

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
