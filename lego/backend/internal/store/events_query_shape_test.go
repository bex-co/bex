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

package store

import (
	"strings"
	"testing"
)

// events_query_shape_test.go guards the one mistake serviceEventsQuery's shape
// invites and cannot survive: its `feed` CTE is four UNION ALL branches whose
// FIRST branch names the columns and whose other three are positional, and the
// outer SELECT then reads those names.
//
// w4/m112 added `stall_reason` to the outer SELECT and to the sibling by-id
// query, and missed the CTE — so every read of a service's activity feed failed
// with `column "stall_reason" does not exist`. Nothing in the default `go test
// ./...` run touches this SQL; it surfaced only when the Postgres-backed suite
// was run with a live database during w4/117.
//
// This is the cheap half of that check: no database, just the two things that
// must hold for the query to compile at all.

// selectListLength counts the top-level expressions in a SELECT list — commas
// outside parentheses, between "SELECT" and the branch's "FROM".
func selectListLength(t *testing.T, branch string) int {
	t.Helper()
	body := stripSQLComments(branch)
	if idx := strings.Index(body, "\n    FROM "); idx >= 0 {
		body = body[:idx]
	} else {
		t.Fatalf("branch has no FROM clause:\n%s", branch)
	}
	depth, count := 0, 1
	for _, r := range body {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				count++
			}
		}
	}
	return count
}

// stripSQLComments removes `-- …` to end of line. A comma inside a comment is
// not a select-list separator, and these branches are commented.
func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func TestServiceEventsFeedBranchesAgree(t *testing.T) {
	start := strings.Index(serviceEventsQuery, "WITH feed AS (")
	if start < 0 {
		t.Fatal("serviceEventsQuery no longer opens with the feed CTE — update this guard")
	}
	outerAt := strings.Index(serviceEventsQuery, "\nSELECT key, at, source,")
	if outerAt < 0 {
		t.Fatal("serviceEventsQuery no longer has the outer SELECT — update this guard")
	}
	// Drop the "WITH feed AS (" opener: it leaves an unclosed parenthesis that
	// would put the first branch's whole select list at depth 1.
	cte := serviceEventsQuery[start+len("WITH feed AS (") : outerAt]
	branches := strings.Split(cte, "UNION ALL")
	if len(branches) < 4 {
		t.Fatalf("expected at least four feed branches, got %d — update this guard", len(branches))
	}

	want := selectListLength(t, branches[0])
	for i, branch := range branches[1:] {
		if got := selectListLength(t, branch); got != want {
			t.Errorf("feed branch %d selects %d expressions, branch 0 selects %d — the branches are positional, so a column added to one must be added to all", i+1, got, want)
		}
	}

	// Every name the outer SELECT reads out of `feed` must be an alias the
	// first branch actually declares. This is the exact failure w4/m112 left.
	outer := serviceEventsQuery[outerAt:]
	outerList := outer[len("\nSELECT "):strings.Index(outer, "\nFROM feed")]
	for _, col := range strings.Split(outerList, ",") {
		col = strings.TrimSpace(strings.ReplaceAll(col, "\n", " "))
		if col == "" || strings.Contains(col, " ") || strings.Contains(col, "(") {
			continue // computed or already-qualified expressions are not feed names
		}
		if !strings.Contains(branches[0], " AS "+col+",") && !strings.Contains(branches[0], " AS "+col+"\n") {
			t.Errorf("the outer SELECT reads %q from feed, and the first branch declares no such alias", col)
		}
	}
}
