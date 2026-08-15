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

package gqlutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// findGraphQLFiles returns the internal/**/graphql.go fragments whose source
// satisfies match, relative to internal/. gqlutil itself is excluded — it is
// where the shared implementation is supposed to live.
func findGraphQLFiles(t *testing.T, match func(source string) bool) []string {
	t.Helper()
	root, err := filepath.Abs("..") // internal/gqlutil -> internal/
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Base(path) != "graphql.go" {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if strings.HasPrefix(rel, "gqlutil"+string(filepath.Separator)) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if match(string(source)) {
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

type pageItem struct{ ID string }

// pageSchema builds a one-query schema whose resolver pages `count` items
// through Page, exactly as a feature fragment's list resolver does. Driving the
// helper through a real executed schema (rather than calling it with a
// hand-built ResolveParams) is what makes these tests meaningful: graphql-go's
// own argument handling — an omitted optional argument is absent from the map,
// not zero — is half of the contract under test.
func pageSchema(t *testing.T, count int) graphql.Schema {
	t.Helper()
	items := make([]pageItem, 0, count)
	for i := range count {
		items = append(items, pageItem{ID: fmt.Sprintf("id-%03d", i)})
	}
	itemType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Item",
		Fields: graphql.Fields{"id": &graphql.Field{Type: graphql.String}},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"items": &graphql.Field{
					Type: graphql.NewList(itemType),
					Args: graphql.FieldConfigArgument{
						"cursor": &graphql.ArgumentConfig{Type: graphql.String},
						"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
					},
					Resolve: func(p graphql.ResolveParams) (any, error) {
						return Page(p, items, func(i pageItem) string { return i.ID }), nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return schema
}

// queryIDs executes the query and returns the resolved ids.
func queryIDs(t *testing.T, schema graphql.Schema, query string) []string {
	t.Helper()
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: query})
	if len(result.Errors) > 0 {
		t.Fatalf("query %s: %v", query, result.Errors)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("query %s: data = %#v", query, result.Data)
	}
	raw, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("query %s: items = %#v", query, data["items"])
	}
	ids := make([]string, 0, len(raw))
	for _, entry := range raw {
		ids = append(ids, entry.(map[string]any)["id"].(string))
	}
	return ids
}

// TestPageWithNoArgumentsReturnsTheWholeListUnpaged pins the subtlest half of
// the contract. Page passes `cursorSet || limitSet` to StablePage rather than a
// constant true, so a caller naming NEITHER argument still receives every item
// — the behavior these list queries shipped with before pagination existed.
// Hardcoding that flag to true would silently truncate every unpaged caller at
// 20 items, and nothing else in the suite would notice.
func TestPageWithNoArgumentsReturnsTheWholeListUnpaged(t *testing.T) {
	schema := pageSchema(t, 50)
	if got := queryIDs(t, schema, `{ items { id } }`); len(got) != 50 {
		t.Fatalf("unpaged query returned %d items, want all 50", len(got))
	}
}

func TestPageAppliesTheDefaultLimitOnceEitherArgumentIsNamed(t *testing.T) {
	schema := pageSchema(t, 50)
	// Naming only the cursor opts into the windowed contract, so the default
	// limit applies even though no limit was given.
	got := queryIDs(t, schema, `{ items(cursor: "id-000") { id } }`)
	if len(got) != core.DefaultPageLimit {
		t.Fatalf("cursor-only query returned %d items, want the default %d", len(got), core.DefaultPageLimit)
	}
	// The cursor is exclusive: the window starts at the item after it.
	if got[0] != "id-001" {
		t.Fatalf("first item = %q, want id-001 (cursor is exclusive)", got[0])
	}
}

func TestPageClampsTheLimitToRendersBounds(t *testing.T) {
	schema := pageSchema(t, 200)
	for _, tc := range []struct {
		name  string
		query string
		want  int
	}{
		{name: "in range", query: `{ items(limit: 5) { id } }`, want: 5},
		{name: "above max clamps down", query: `{ items(limit: 500) { id } }`, want: core.MaxPageLimit},
		{name: "at max", query: `{ items(limit: 100) { id } }`, want: core.MaxPageLimit},
		// limit:0 is a degenerate input. core.PageLimit floors it at 1 rather
		// than treating it as "unset", so an explicit zero yields one item, not
		// the default page. Pinned because it is exactly where internal/
		// envgroups' own paging helper still disagrees (it keys on limit == 0
		// and returns the default 20 instead).
		{name: "explicit zero floors at one", query: `{ items(limit: 0) { id } }`, want: 1},
		{name: "negative floors at one", query: `{ items(limit: -7) { id } }`, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := queryIDs(t, schema, tc.query); len(got) != tc.want {
				t.Fatalf("%s returned %d items, want %d", tc.query, len(got), tc.want)
			}
		})
	}
}

func TestPageWalksTheListWithSuccessiveCursors(t *testing.T) {
	schema := pageSchema(t, 25)
	first := queryIDs(t, schema, `{ items(limit: 10) { id } }`)
	if len(first) != 10 || first[0] != "id-000" {
		t.Fatalf("first page = %v", first)
	}
	second := queryIDs(t, schema, fmt.Sprintf(`{ items(limit: 10, cursor: %q) { id } }`, first[len(first)-1]))
	if len(second) != 10 || second[0] != "id-010" {
		t.Fatalf("second page = %v, want 10 items starting at id-010", second)
	}
	last := queryIDs(t, schema, fmt.Sprintf(`{ items(limit: 10, cursor: %q) { id } }`, second[len(second)-1]))
	if len(last) != 5 || last[0] != "id-020" {
		t.Fatalf("last page = %v, want the 5-item tail starting at id-020", last)
	}
	// An unknown cursor yields an empty page — Render's end-of-list signal.
	if got := queryIDs(t, schema, `{ items(limit: 10, cursor: "id-999") { id } }`); len(got) != 0 {
		t.Fatalf("unknown cursor returned %d items, want an empty page", len(got))
	}
}

// TestPageIsTheOnlyPagingBlockLeftInTheGraphQLFragments guards the dedupe: the
// eight-line cursor/limit/StablePage block Page replaced existed in six
// verbatim copies, and a new list query is far more likely to be written by
// copying a neighbour than by reaching for this helper.
func TestPageIsTheOnlyPagingBlockLeftInTheGraphQLFragments(t *testing.T) {
	offenders := findGraphQLFiles(t, func(source string) bool {
		return strings.Contains(source, `p.Args["limit"].(int)`) &&
			strings.Contains(source, "core.StablePage(")
	})
	if len(offenders) > 0 {
		t.Fatalf("these GraphQL fragments re-implement the cursor/limit paging block instead of "+
			"calling gqlutil.Page:\n\t%s", strings.Join(offenders, "\n\t"))
	}
}
