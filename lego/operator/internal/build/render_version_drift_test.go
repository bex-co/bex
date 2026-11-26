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

package build

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/types/toolchains"
)

// Render moves the default of a runtime for services created from a day on,
// and back-dates rows, without a changelog entry: on 2026-09-09, 09-17, 10-12,
// 11-03, 11-19 and 11-20 (w3/m637). scripts/render-version-drift.sh reads its
// seven version pages every day into the JSON this test reads from
// BEX_RENDER_VERSION_PAGES, and TestRenderVersionDrift compares every row a
// service bex runs can fall in with bex's own dated tables.

// renderVersionPagesEnv names the JSON file of Render's pages. Unset, the live
// comparison skips; the daily job sets it.
const renderVersionPagesEnv = "BEX_RENDER_VERSION_PAGES"

// bexFirstService is when bex's first service could be created: no service
// bex runs is older, so the comparison starts at the row in force that day.
var bexFirstService = time.Date(2026, time.June, 5, 0, 0, 0, 0, time.UTC)

// renderVersionPages is scripts/lib/render_version_pages.py's output.
type renderVersionPages struct {
	Source string                       `json:"source"`
	Pages  map[string]renderVersionPage `json:"pages"`
}

type renderVersionPage struct {
	URL string `json:"url"`
	// Defaults are the current defaults the page's header gives, by tool.
	Defaults map[string]string `json:"defaults"`
	// Since is the day the header dates them from, "" when it names none.
	Since string `json:"since"`
	// Tool is the one whose defaults Rows give, newest first.
	Tool string             `json:"tool"`
	Rows []renderVersionRow `json:"rows"`
}

type renderVersionRow struct {
	From    string `json:"from"`   // YYYY-MM-DD; "" on the oldest row,
	Before  string `json:"before"` // which says "Before YYYY-MM-DD"
	Version string `json:"version"`
}

// renderComparison is how much of a page a bex table follows.
type renderComparison int

const (
	// renderRows: every row from the one in force on bexFirstService.
	renderRows renderComparison = iota
	// renderHeaderDefault: the current default and the day the header
	// dates it from, against the table's newest step. For a default the
	// page has no history table for.
	renderHeaderDefault
	// renderCurrentDefault: the current default alone.
	renderCurrentDefault
)

// renderDefaultTable is a bex table that follows one default on a Render page.
type renderDefaultTable struct {
	page, tool string
	name       string // the bex table, as a reader finds it
	steps      []toolchains.DatedDefault
	compare    renderComparison
	same       func(render, bex string) bool // version equality; nil is ==
}

// renderDefaultTables are bex's tables, one per default Render dates. A page
// Render adds needs one here, and a WATCHED entry in the parser.
func renderDefaultTables() []renderDefaultTable {
	return []renderDefaultTable{
		{page: "node-version", tool: "Node.js", name: "exactDefaults[nativeNodeRuntime] (nativeversion.go)", steps: exactDefaults[nativeNodeRuntime]},
		{page: "python-version", tool: "Python", name: "toolchains.PythonDefaults (lego/types/toolchains/python.go)", steps: toolchains.PythonDefaults},
		{page: "ruby-version", tool: "Ruby", name: "exactDefaults[nativeRubyRuntime] (nativeversion.go)", steps: exactDefaults[nativeRubyRuntime]},
		{page: "bun-version", tool: "Bun", name: "bunDefaults (nativeversion.go)", steps: bunDefaults},
		{page: "uv-version", tool: "uv", name: "uvDefaults (nativeversion.go)", steps: uvDefaults},
		{page: "elixir-erlang-versions", tool: "Elixir", name: "exactDefaults[nativeElixirRuntime] (nativeversion.go)", steps: exactDefaults[nativeElixirRuntime]},
		// The page's only history table is Elixir's; its header dates both
		// defaults. bex writes the release in the four-part form (28.0.2.0).
		{page: "elixir-erlang-versions", tool: "Erlang/OTP", name: "erlangDefaults (nativeversion.go)", steps: erlangDefaults, compare: renderHeaderDefault,
			same: func(render, bex string) bool { return otpTag(render) == otpTag(bex) }},
		// Render's older Poetry rows name series (1.1.x (various)), and bex
		// carries one Poetry whatever a service's age.
		{page: "poetry-version", tool: "Poetry", name: "defaultPoetryVersion (nativeversion.go)", steps: []toolchains.DatedDefault{{Version: defaultPoetryVersion}}, compare: renderCurrentDefault},
	}
}

// renderVersionException is a reviewed difference bex keeps from Render.
type renderVersionException struct {
	page, tool, row string // as the difference names them
	adr018          string // the ADR018 row that records it with the user's sign-off
}

// renderVersionDriftExceptions are the differences bex keeps on purpose. Empty
// since w3/m636 gave every table Render's rows back to bexFirstService; an
// entry needs the user's sign-off recorded in its ADR018 row (AGENTS.md).
var renderVersionDriftExceptions []renderVersionException

// renderVersionDifference is one way a page and a bex table disagree.
type renderVersionDifference struct {
	page, tool string
	row        string // a row's day, "current", or "page"
	message    string
}

func (d renderVersionDifference) String() string {
	where := "render.com/docs/" + d.page
	if d.tool != "" {
		where += ", " + d.tool
	}
	switch d.row {
	case "page":
	case "current":
		where += " current default"
	default:
		where += " row " + d.row
	}
	return where + ": " + d.message
}

func (t renderDefaultTable) differs(row, format string, args ...any) renderVersionDifference {
	return renderVersionDifference{page: t.page, tool: t.tool, row: row, message: fmt.Sprintf(format, args...)}
}

func (t renderDefaultTable) equal(render, bex string) bool {
	if t.same != nil {
		return t.same(render, bex)
	}
	return render == bex
}

// renderVersionDrift is every difference between Render's pages and bex's
// tables, in table order.
func renderVersionDrift(pages renderVersionPages, tables []renderDefaultTable) []renderVersionDifference {
	var diffs []renderVersionDifference
	followed := map[string]bool{}
	for _, table := range tables {
		followed[table.page] = true
		page, ok := pages.Pages[table.page]
		if !ok {
			diffs = append(diffs, table.differs("page", "not among the pages read from %s; %s follows it", pages.Source, table.name))
			continue
		}
		diffs = append(diffs, table.differences(page)...)
	}
	for _, name := range slices.Sorted(maps.Keys(pages.Pages)) {
		if !followed[name] {
			diffs = append(diffs, renderVersionDifference{page: name, row: "page",
				message: "no bex table follows this page: give it one in renderDefaultTables (render_version_drift_test.go)"})
		}
	}
	return diffs
}

func (t renderDefaultTable) differences(page renderVersionPage) []renderVersionDifference {
	current, ok := page.Defaults[t.tool]
	if !ok {
		return []renderVersionDifference{t.differs("current", "%s gives no current %s default; %s follows one", page.URL, t.tool, t.name)}
	}
	newest := t.steps[len(t.steps)-1]
	switch t.compare {
	case renderCurrentDefault:
		if !t.equal(current, newest.Version) {
			return []renderVersionDifference{t.differs("current", "Render's current default is %s, %s is %s", current, t.name, newest.Version)}
		}
		return nil
	case renderHeaderDefault:
		since, err := time.Parse(time.DateOnly, page.Since)
		if err != nil {
			return []renderVersionDifference{t.differs("current", "the header dates the current %s default from no day (%q)", t.tool, page.Since)}
		}
		if !t.equal(current, newest.Version) || !since.Equal(newest.From) {
			return []renderVersionDifference{t.differs("current", "Render's current default is %s from %s, %s's newest step is %s from %s",
				current, page.Since, t.name, newest.Version, newest.From.Format(time.DateOnly))}
		}
		return nil
	}
	if page.Tool != t.tool {
		return []renderVersionDifference{t.differs("page", "the page's history table is %s's, not %s's, which %s follows", page.Tool, t.tool, t.name)}
	}
	render, err := page.steps()
	if err != nil {
		return []renderVersionDifference{t.differs("page", "%v", err)}
	}
	var diffs []renderVersionDifference
	if top := render[len(render)-1]; !t.equal(current, newest.Version) || !top.From.Equal(newest.From) {
		diffs = append(diffs, t.differs("current", "Render's current default moved: %s from %s, where %s's newest step is %s from %s",
			current, top.From.Format(time.DateOnly), t.name, newest.Version, newest.From.Format(time.DateOnly)))
	}
	return append(diffs, t.rowDifferences(inForceSince(render, bexFirstService), inForceSince(t.steps, bexFirstService))...)
}

// rowDifferences compares two tables of steps from bexFirstService on.
func (t renderDefaultTable) rowDifferences(render, bex []toolchains.DatedDefault) []renderVersionDifference {
	var diffs []renderVersionDifference
	label := func(from time.Time) string {
		if from.Equal(bexFirstService) {
			return from.Format(time.DateOnly) + " (the row in force when bex's first service could be created)"
		}
		return from.Format(time.DateOnly)
	}
	onDay := func(steps []toolchains.DatedDefault, day time.Time) (string, bool) {
		i := slices.IndexFunc(steps, func(s toolchains.DatedDefault) bool { return s.From.Equal(day) })
		if i < 0 {
			return "", false
		}
		return steps[i].Version, true
	}
	newestBex := bex[len(bex)-1].From // inForceSince of a non-empty table is never empty
	for _, r := range render {
		day := r.From.Format(time.DateOnly)
		version, ok := onDay(bex, r.From)
		switch {
		case !ok && r.From.After(newestBex):
			diffs = append(diffs, t.differs(day, "new row: Render gives %s to services created from %s; %s has no step that day", r.Version, label(r.From), t.name))
		case !ok:
			diffs = append(diffs, t.differs(day, "back-dated row: Render gives %s to services created from %s; %s has no step that day (its newest is %s)",
				r.Version, label(r.From), t.name, newestBex.Format(time.DateOnly)))
		case !t.equal(r.Version, version):
			diffs = append(diffs, t.differs(day, "Render gives %s to services created from %s, %s gives %s", r.Version, label(r.From), t.name, version))
		}
	}
	for _, b := range bex {
		if _, ok := onDay(render, b.From); !ok {
			diffs = append(diffs, t.differs(b.From.Format(time.DateOnly), "%s has a step %s from %s that Render's page no longer has", t.name, b.Version, label(b.From)))
		}
	}
	return diffs
}

// steps is the page's history table oldest first, as bex writes its tables:
// the oldest "Before" row from the zero time.
func (p renderVersionPage) steps() ([]toolchains.DatedDefault, error) {
	if len(p.Rows) == 0 {
		return nil, fmt.Errorf("%s has no history rows", p.URL)
	}
	steps := make([]toolchains.DatedDefault, 0, len(p.Rows))
	for _, row := range slices.Backward(p.Rows) {
		var from time.Time
		if row.From != "" {
			var err error
			if from, err = time.Parse(time.DateOnly, row.From); err != nil {
				return nil, fmt.Errorf("%s has a row dated %q", p.URL, row.From)
			}
		}
		steps = append(steps, toolchains.DatedDefault{From: from, Version: row.Version})
	}
	return steps, nil
}

// inForceSince is steps (oldest first) from the one in force at since on,
// that one dated since: what services created from since on get.
func inForceSince(steps []toolchains.DatedDefault, since time.Time) []toolchains.DatedDefault {
	var out []toolchains.DatedDefault
	for _, step := range steps {
		if step.From.After(since) {
			out = append(out, step)
			continue
		}
		out = []toolchains.DatedDefault{{From: since, Version: step.Version}}
	}
	return out
}

// applyRenderVersionExceptions drops the differences an exception covers. An
// exception that covers none, or names no ADR018 row, is a problem.
func applyRenderVersionExceptions(diffs []renderVersionDifference, exceptions []renderVersionException) (left []renderVersionDifference, problems []string) {
	used := make([]bool, len(exceptions))
	for _, d := range diffs {
		i := slices.IndexFunc(exceptions, func(e renderVersionException) bool {
			return e.page == d.page && e.tool == d.tool && e.row == d.row
		})
		if i < 0 {
			left = append(left, d)
			continue
		}
		used[i] = true
	}
	for i, e := range exceptions {
		if !strings.Contains(e.adr018, "ADR018") {
			problems = append(problems, fmt.Sprintf("exception %s %s %s names no ADR018 row recording the user's sign-off", e.page, e.tool, e.row))
		}
		if !used[i] {
			problems = append(problems, fmt.Sprintf("exception %s %s %s covers no difference any more: remove it", e.page, e.tool, e.row))
		}
	}
	return left, problems
}

func TestRenderVersionDrift(t *testing.T) {
	path := os.Getenv(renderVersionPagesEnv)
	if path == "" {
		t.Skipf("%s is unset; scripts/render-version-drift.sh sets it to Render's pages, read live", renderVersionPagesEnv)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", renderVersionPagesEnv, err)
	}
	var pages renderVersionPages
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields() // the parser and this test share one shape
	if err := decoder.Decode(&pages); err != nil {
		t.Fatalf("%s is not the parser's JSON: %v", path, err)
	}
	left, problems := applyRenderVersionExceptions(renderVersionDrift(pages, renderDefaultTables()), renderVersionDriftExceptions)
	for _, d := range left {
		t.Error(d)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
	if t.Failed() {
		t.Log(`Render's default versions no longer match bex's tables. For each difference, follow Render in the named table
with a dated step (w3/m636: never a line-pin bump, so no older service moves), then refresh the page fixtures in
scripts/fixtures/render-version-pages/. Keeping a difference needs the user's sign-off in its ADR018 row and an
entry in renderVersionDriftExceptions.`)
	}
}

// renderPagesLikeBex is Render's pages as they read when they agree with every
// bex table, older rows than any bex service included.
func renderPagesLikeBex() renderVersionPages {
	pages := renderVersionPages{Source: "https://render.com/docs/llms.txt", Pages: map[string]renderVersionPage{}}
	for _, table := range renderDefaultTables() {
		page, ok := pages.Pages[table.page]
		if !ok {
			page = renderVersionPage{URL: "https://render.com/docs/" + table.page + ".md", Defaults: map[string]string{}}
		}
		newest := table.steps[len(table.steps)-1]
		switch table.compare {
		case renderRows:
			page.Defaults[table.tool] = newest.Version
			page.Since, page.Tool = newest.From.Format(time.DateOnly), table.tool
			for _, step := range slices.Backward(table.steps) {
				page.Rows = append(page.Rows, renderVersionRow{From: step.From.Format(time.DateOnly), Version: step.Version})
			}
			page.Rows = append(page.Rows, renderVersionRow{Before: "1970-01-01", Version: "0.0.1"})
		case renderHeaderDefault:
			page.Defaults[table.tool] = otpName(newest.Version) // Render writes 28.0.2
		case renderCurrentDefault:
			page.Defaults[table.tool] = newest.Version
			page.Since, page.Tool = "2025-06-12", table.tool
			page.Rows = []renderVersionRow{{From: "2025-06-12", Version: newest.Version}, {From: "2021-04-27", Version: "1.1.x (various)"}}
		}
		pages.Pages[table.page] = page
	}
	return pages
}

func editPage(pages renderVersionPages, name string, edit func(*renderVersionPage)) {
	page := pages.Pages[name]
	edit(&page)
	pages.Pages[name] = page
}

// TestRenderVersionDriftNamesEachKindOfMove: each way Render has moved, or
// could move, a default fails the comparison with the page, the row and the
// bex table named.
func TestRenderVersionDriftNamesEachKindOfMove(t *testing.T) {
	if diffs := renderVersionDrift(renderPagesLikeBex(), renderDefaultTables()); len(diffs) != 0 {
		t.Fatalf("pages built from bex's own tables differ from them: %v", diffs)
	}
	cases := []struct {
		name string
		edit func(renderVersionPages)
		want []string // each in some difference
	}{
		{"a current default moved in place", func(p renderVersionPages) {
			editPage(p, "node-version", func(page *renderVersionPage) {
				page.Rows[0].Version, page.Defaults["Node.js"] = "24.22.2", "24.22.2"
			})
		}, []string{
			"render.com/docs/node-version, Node.js current default: Render's current default moved: 24.22.2 from 2026-10-21, where exactDefaults[nativeNodeRuntime] (nativeversion.go)'s newest step is 24.22.1",
			"render.com/docs/node-version, Node.js row 2026-10-21: Render gives 24.22.2 to services created from 2026-10-21, exactDefaults[nativeNodeRuntime] (nativeversion.go) gives 24.22.1",
		}},
		{"a new row", func(p renderVersionPages) {
			editPage(p, "ruby-version", func(page *renderVersionPage) {
				page.Rows = slices.Insert(page.Rows, 0, renderVersionRow{From: "2026-12-01", Version: "3.4.14"})
				page.Defaults["Ruby"], page.Since = "3.4.14", "2026-12-01"
			})
		}, []string{
			"render.com/docs/ruby-version, Ruby current default: Render's current default moved: 3.4.14 from 2026-12-01",
			"render.com/docs/ruby-version, Ruby row 2026-12-01: new row: Render gives 3.4.14 to services created from 2026-12-01; exactDefaults[nativeRubyRuntime] (nativeversion.go) has no step that day",
		}},
		{"a back-dated row", func(p renderVersionPages) {
			editPage(p, "uv-version", func(page *renderVersionPage) {
				i := slices.IndexFunc(page.Rows, func(r renderVersionRow) bool { return r.From == "2026-09-09" })
				page.Rows = slices.Insert(page.Rows, i, renderVersionRow{From: "2026-09-20", Version: "0.12.15"})
			})
		}, []string{
			"render.com/docs/uv-version, uv row 2026-09-20: back-dated row: Render gives 0.12.15 to services created from 2026-09-20; uvDefaults (nativeversion.go) has no step that day (its newest is 2026-11-03)",
		}},
		{"a removed row", func(p renderVersionPages) {
			editPage(p, "bun-version", func(page *renderVersionPage) {
				page.Rows = slices.DeleteFunc(page.Rows, func(r renderVersionRow) bool { return r.Version == "1.4.3" })
			})
		}, []string{"render.com/docs/bun-version, Bun row 2026-10-12: bunDefaults (nativeversion.go) has a step 1.4.3 from 2026-10-12 that Render's page no longer has"}},
		{"the row in force on bex's first day changed", func(p renderVersionPages) {
			editPage(p, "python-version", func(page *renderVersionPage) {
				i := slices.IndexFunc(page.Rows, func(r renderVersionRow) bool { return r.Version == "3.14.3" })
				page.Rows[i].Version = "3.14.2"
			})
		}, []string{"render.com/docs/python-version, Python row 2026-06-05: Render gives 3.14.2 to services created from 2026-06-05 (the row in force when bex's first service could be created), toolchains.PythonDefaults (lego/types/toolchains/python.go) gives 3.14.3"}},
		{"a row back-dated to just before bex's first day", func(p renderVersionPages) {
			editPage(p, "node-version", func(page *renderVersionPage) {
				i := slices.IndexFunc(page.Rows, func(r renderVersionRow) bool { return r.Version == "24.14.1" })
				page.Rows = slices.Insert(page.Rows, i, renderVersionRow{From: "2026-05-20", Version: "24.15.0"})
			})
		}, []string{"render.com/docs/node-version, Node.js row 2026-06-05: Render gives 24.15.0 to services created from 2026-06-05 (the row in force when bex's first service could be created), exactDefaults[nativeNodeRuntime] (nativeversion.go) gives 24.14.1"}},
		{"Erlang/OTP's default moved", func(p renderVersionPages) {
			editPage(p, "elixir-erlang-versions", func(page *renderVersionPage) { page.Defaults["Erlang/OTP"] = "28.1" })
		}, []string{"render.com/docs/elixir-erlang-versions, Erlang/OTP current default: Render's current default is 28.1 from 2025-06-12, erlangDefaults (nativeversion.go)'s newest step is 28.0.2.0 from 2025-06-12"}},
		{"Erlang/OTP's default re-dated", func(p renderVersionPages) {
			editPage(p, "elixir-erlang-versions", func(page *renderVersionPage) { page.Since = "2026-12-01" })
		}, []string{"Erlang/OTP current default: Render's current default is 28.0.2 from 2026-12-01, erlangDefaults (nativeversion.go)'s newest step is 28.0.2.0 from 2025-06-12"}},
		{"Poetry's default moved", func(p renderVersionPages) {
			editPage(p, "poetry-version", func(page *renderVersionPage) { page.Defaults["Poetry"] = "2.2.1" })
		}, []string{"render.com/docs/poetry-version, Poetry current default: Render's current default is 2.2.1, defaultPoetryVersion (nativeversion.go) is 2.1.3"}},
		{"a removed page", func(p renderVersionPages) { delete(p.Pages, "uv-version") },
			[]string{"render.com/docs/uv-version, uv: not among the pages read from https://render.com/docs/llms.txt; uvDefaults (nativeversion.go) follows it"}},
		{"an eighth page", func(p renderVersionPages) { p.Pages["go-version"] = p.Pages["poetry-version"] },
			[]string{"render.com/docs/go-version: no bex table follows this page"}},
		{"a history table for another tool", func(p renderVersionPages) {
			editPage(p, "elixir-erlang-versions", func(page *renderVersionPage) { page.Tool = "Erlang/OTP" })
		}, []string{"render.com/docs/elixir-erlang-versions, Elixir: the page's history table is Erlang/OTP's, not Elixir's"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pages := renderPagesLikeBex()
			tc.edit(pages)
			diffs := renderVersionDrift(pages, renderDefaultTables())
			got := make([]string, 0, len(diffs))
			for _, d := range diffs {
				got = append(got, d.String())
			}
			for _, want := range tc.want {
				if !slices.ContainsFunc(got, func(line string) bool { return strings.Contains(line, want) }) {
					t.Errorf("no difference says %q; got:\n%s", want, strings.Join(got, "\n"))
				}
			}
		})
	}
}

// TestRenderVersionDriftIgnoresRowsNoBexServiceFallsIn: Render's rows from
// before bex's first service, and Poetry's older series, move nothing bex
// builds.
func TestRenderVersionDriftIgnoresRowsNoBexServiceFallsIn(t *testing.T) {
	pages := renderPagesLikeBex()
	editPage(pages, "node-version", func(page *renderVersionPage) {
		page.Rows[len(page.Rows)-1].Version = "14.17.0"
		// Older than the row in force on bex's first day, which still is.
		page.Rows = slices.Insert(page.Rows, len(page.Rows)-1, renderVersionRow{From: "2026-03-01", Version: "24.13.0"})
	})
	editPage(pages, "poetry-version", func(page *renderVersionPage) { page.Rows[1].Version = "1.7.1" })
	if diffs := renderVersionDrift(pages, renderDefaultTables()); len(diffs) != 0 {
		t.Errorf("rows no bex service falls in made differences: %v", diffs)
	}
}

func TestRenderVersionExceptions(t *testing.T) {
	pages := renderPagesLikeBex()
	editPage(pages, "ruby-version", func(page *renderVersionPage) {
		page.Rows = slices.Insert(page.Rows, 0, renderVersionRow{From: "2026-12-01", Version: "3.4.14"})
		page.Defaults["Ruby"] = "3.4.14"
	})
	diffs := renderVersionDrift(pages, renderDefaultTables())
	signed := []renderVersionException{
		{page: "ruby-version", tool: "Ruby", row: "current", adr018: "ADR018 services-lifecycle.md Runtime row"},
		{page: "ruby-version", tool: "Ruby", row: "2026-12-01", adr018: "ADR018 services-lifecycle.md Runtime row"},
	}
	if left, problems := applyRenderVersionExceptions(diffs, signed); len(left) != 0 || len(problems) != 0 {
		t.Errorf("signed exceptions left %v, problems %v", left, problems)
	}
	if left, _ := applyRenderVersionExceptions(diffs, signed[:1]); len(left) != 1 || left[0].row != "2026-12-01" {
		t.Errorf("an exception for the current default alone left %v, want the new row", left)
	}
	unsigned := []renderVersionException{{page: "ruby-version", tool: "Ruby", row: "current", adr018: "agreed in chat"}}
	if _, problems := applyRenderVersionExceptions(diffs, unsigned); !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "names no ADR018 row") }) {
		t.Errorf("an exception with no ADR018 row passed: %v", problems)
	}
	stale := []renderVersionException{{page: "node-version", tool: "Node.js", row: "current", adr018: "ADR018 Runtime row"}}
	if _, problems := applyRenderVersionExceptions(diffs, stale); !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, "covers no difference any more") }) {
		t.Errorf("an exception covering no difference passed: %v", problems)
	}
	for _, e := range renderVersionDriftExceptions {
		if !strings.Contains(e.adr018, "ADR018") {
			t.Errorf("committed exception %s %s %s names no ADR018 row", e.page, e.tool, e.row)
		}
	}
}
