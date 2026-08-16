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

package usage

import (
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// period.go translates the `period` query param every usage surface accepts.
// Kept beside its own test rather than appended to service.go, whose sections
// are banner-organised and had no home for it.

// periodLayout is the wire format of the `period` param — Render-style
// "YYYY-MM". Spelled once so the parse and the echoed response cannot drift.
const periodLayout = "2006-01"

// ResolvePeriodEnd maps the optional "YYYY-MM" `period` param to the effective
// upper bound of a usage query — one translator for REST, GraphQL and MCP, so
// the three cannot read the same param differently.
//
// A MALFORMED period is a named 400. It used to be swallowed (`if err == nil`),
// which silently answered with the CURRENT month-to-date: a caller who typed
// `?period=july` or fat-fingered `2026-1x` got a different month's numbers than
// the one they asked for, on every surface. Usage figures drive billing
// conversations, so quietly substituting a month is the one thing this must not
// do — the same "nothing accepted is ignored" rule (w3/m8) the audit and deploy
// list filters already follow.
//
// A period that parses but is NOT in the past still returns now, deliberately:
// the current month is answered month-to-date, and a future month has no data
// to bound, so both collapse to "as of now". That is a semantic clamp on a
// value the caller spelled correctly, not an ignored input, and the response
// echoes the period actually used.
func ResolvePeriodEnd(period string, now time.Time) (time.Time, error) {
	if period == "" {
		return now, nil
	}
	end, err := parsePeriod(period)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: period must be a calendar month as YYYY-MM (e.g. 2026-07)", core.ErrBadRequest)
	}
	if end.Before(now) {
		return end, nil
	}
	return now, nil
}

// parsePeriod parses "YYYY-MM" and returns the last nanosecond of that month.
func parsePeriod(period string) (time.Time, error) {
	t, err := time.Parse(periodLayout, period)
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 1, 0).Add(-time.Nanosecond), nil
}
