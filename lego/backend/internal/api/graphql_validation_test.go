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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/printer"
	"github.com/graphql-go/graphql/testutil"
)

// TestGraphQLValidationIsBounded: graphql-go validates a document after every
// bex gate and never reads the request context, so the execution deadline does
// not bound it. Each document here kept it busy for seconds to hours (the
// first two are the 20,000-fragment and 20,000-link documents measured at
// 4m52s and 11.5s); each is now refused by a gate or answered within a second.
func TestGraphQLValidationIsBounded(t *testing.T) {
	srv, _ := scopedKeyServer(t)
	repeat := func(n int, format string) string {
		var b strings.Builder
		for i := range n {
			fmt.Fprintf(&b, format, i, i+1)
		}
		return b.String()
	}
	unknownFields := strings.Repeat("x\n", 15000)
	manyErrors := "{ __typename\n" + strings.Repeat("@a\n", 10000) + "}"
	for _, tc := range []struct {
		name, doc string
		// want is the first error's message; errors, when set, how many.
		want   string
		errors int
	}{{
		name: "20,000 unused fragments",
		doc:  "{ __typename }\n" + repeat(20000, "fragment F%[1]d on Query { __typename }\n"),
		want: "query rejected: document is larger than 32768 bytes",
	}, {
		name: "a 20,000-link spread chain",
		doc:  "{ ...F0 }\n" + repeat(20000, "fragment F%[1]d on Query { ...F%[2]d }\n") + "fragment F20000 on Query { __typename }",
		want: "query rejected: document is larger than 32768 bytes",
	}, {
		// 2^30 spread paths in 3 KB.
		name: "fragments that each spread two others",
		doc: "{ ...F0 }\n" + repeat(30, "fragment F%[1]d on Query { ...A%[1]d ...B%[1]d }\nfragment A%[1]d on Query { ...F%[2]d }\nfragment B%[1]d on Query { ...F%[2]d }\n") +
			"fragment F30 on Query { ...Missing }",
		want: fmt.Sprintf("query rejected: spreads fragments more than %d times", gqlMaxSpreads),
	}, {
		// Half a million conflicting pairs in 29 KB.
		name: "one response name selected 1,000 times",
		doc:  "{ " + repeat(1000, "__type(name: \"T%[1]d\") { name }\n") + "}",
		want: fmt.Sprintf(`query rejected: selects "__type" more than %d times in one selection set`, gqlMaxSameResponseName),
	}, {
		name: "a fragment nothing spreads",
		doc:  "{ __typename }\nfragment F on Query { " + unknownFields + "}",
		want: `Fragment "F" is never used.`,
	}, {
		name: "a fragment defined twice",
		doc:  "{ ...F }\nfragment F on Query { __typename }\nfragment F on Query { " + unknownFields + "}",
		want: `There can only be one fragment named "F".`,
	}, {
		name: "a list nested 2,000 deep",
		doc:  "{ __type(name: " + strings.Repeat("[", 2000) + strings.Repeat("]", 2000) + ") { name } }",
		want: fmt.Sprintf("query rejected: a value nests lists and objects deeper than %d", gqlMaxValueDepth),
	}, {
		// graphql-go re-read the whole document for each error's line.
		name:   "10,000 errors, one per line",
		doc:    manyErrors,
		want:   `Unknown directive "a".`,
		errors: 10000,
	}, {
		// graphql-go measured its edit distance to every type in the schema.
		name:   "a 30,000-character type name",
		doc:    "query($a: " + strings.Repeat("X", 30000) + ") { __typename }",
		want:   `Unknown type "` + strings.Repeat("X", 30000) + `".`,
		errors: 2,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			var result *graphql.Result
			start := time.Now()
			finishesWithin(t, tc.name, func() {
				_, _, _, result = srv.executeGraphQL(context.Background(), graphqlRequestBody{Query: tc.doc})
			})
			if took := time.Since(start); took > time.Second {
				t.Errorf("took %v, want under a second", took)
			}
			if len(result.Errors) == 0 || result.Errors[0].Message != tc.want {
				t.Fatalf("errors = %.300v, want first %q", result.Errors, tc.want)
			}
			if tc.errors != 0 && len(result.Errors) != tc.errors {
				t.Errorf("%d errors, want %d", len(result.Errors), tc.errors)
			}
		})
	}
	// The 10,000 errors are each on their own line.
	_, _, _, result := srv.executeGraphQL(context.Background(), graphqlRequestBody{Query: manyErrors})
	if loc := result.Errors[9999].Locations; len(loc) != 1 || loc[0].Line != 10001 || loc[0].Column != 1 {
		t.Errorf("the last error's location = %v, want line 10001, column 1", loc)
	}
}

// TestGraphQLAnswersMatchGraphQLDo: the request path validates and executes
// the document it parsed for the gates, without its source, and locates the
// errors afterwards. Its answer is graphql.Do's, line breaks of every kind and
// characters of any width included, but for the names graphql-go suggests
// after an unknown field or type.
func TestGraphQLAnswersMatchGraphQLDo(t *testing.T) {
	srv, _ := scopedKeyServer(t)
	suggestion := regexp.MustCompile(` Did you mean .*\?$`)
	invalid := "query Q($unused: Int) {\n  __typename @skip(if: true) @nope\n  service(id: \"x\", bogus: 1) { id }\n}"
	for _, doc := range []string{
		"{ __typename }",
		invalid,
		strings.ReplaceAll(invalid, "\n", "\r\n"),
		strings.ReplaceAll(invalid, "\n", "\r"),
		"{\n  servics { id }\n  __typename\n}",
		"query($a: Strin) {\n  __typename\n}",
		"# é日本\n{ __typename @x(a: \"é日本\") @nope\n  serviceX }",
		"{\n  service(id: \"nope\") {\n    id\n  }\n}",
		"{\n  service(id: \"x\") {\n    id\n\n",
		"{ ...F }\nfragment F on Query {\n  __typename\n  nope\n}",
	} {
		parsed, parseErr := parseGraphQL(doc)
		got := srv.runGraphQL(context.Background(), graphqlRequestBody{Query: doc}, parsed, parseErr)
		want := graphql.Do(graphql.Params{Schema: srv.schema, RequestString: doc, Context: context.Background()})
		for i := range want.Errors {
			want.Errors[i].Message = suggestion.ReplaceAllString(want.Errors[i].Message, "")
		}
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("document %q:\n got %s\nwant %s", doc, gotJSON, wantJSON)
		}
	}
}

// TestGraphQLDocumentBudgets pins each budget on graphql-go's validation at
// its edge, and each count through inline fragments and fragment spreads.
func TestGraphQLDocumentBudgets(t *testing.T) {
	nest := func(open, close string, n int) string { return strings.Repeat(open, n) + strings.Repeat(close, n) }
	query := "{ __typename }"
	for doc, want := range map[string]string{
		strings.Repeat(" ", gqlMaxDocumentBytes-len(query)) + query:   "",
		strings.Repeat(" ", gqlMaxDocumentBytes-len(query)+1) + query: "document is larger than",
		// Values nest inside parentheses only.
		"{ __type(name: " + nest("[", "]", gqlMaxValueDepth) + ") { name } }":                                                                       "",
		"{ __type(name: " + nest("[", "]", gqlMaxValueDepth+1) + ") { name } }":                                                                     "a value nests lists and objects deeper than",
		"{ __type(name: " + nest("{a: ", "}", gqlMaxValueDepth+1) + ") { name } }":                                                                  "a value nests lists and objects deeper than",
		"query($a: " + nest("[", "]", gqlMaxValueDepth+1) + ") { __typename }":                                                                      "a value nests lists and objects deeper than",
		"{ a: __type(name: " + nest("[", "]", gqlMaxValueDepth) + ") { name } b: __type(name: " + nest("[", "]", gqlMaxValueDepth) + ") { name } }": "",
		"{ " + nest("__schema { ", "} ", gqlMaxValueDepth+2) + "}":                                                                                  "",
		// One response name per selection set, counting what it merges.
		"{ " + strings.Repeat("__typename ", gqlMaxSameResponseName) + "}":                                         "",
		"{ " + strings.Repeat("__typename ", gqlMaxSameResponseName+1) + "}":                                       `selects "__typename" more than`,
		"{ __typename ... on Query { " + strings.Repeat("__typename ", gqlMaxSameResponseName) + "} }":             `selects "__typename" more than`,
		"{ __typename ...F } fragment F on Query { " + strings.Repeat("__typename ", gqlMaxSameResponseName) + "}": `selects "__typename" more than`,
		"{ " + strings.Repeat("a: __typename ", 6) + strings.Repeat("a: __schema { __typename } ", 5) + "}":        `selects "a" more than`,
		"{ " + strings.Repeat("__schema { __typename } ", gqlMaxSameResponseName) + "}":                            "",
		// Spreads, along every path.
		fragmentSpreadChain(gqlMaxSpreads, "id"):   "",
		fragmentSpreadChain(gqlMaxSpreads+1, "id"): "spreads fragments more than",
		strings.Replace(fragmentSpreadChain(gqlMaxSpreads/2+1, "id"), "{ ...F1 }", "{ ... on Service { ...F1 } ...F1 }", 1): "spreads fragments more than",
		// Every fragment is read, once.
		"{ __typename } fragment F on Query { __typename }":                        `Fragment "F" is never used.`,
		"{ ...F } fragment F on Query { __typename } fragment G on Query { ...F }": `Fragment "G" is never used.`,
		"{ ...F } fragment F on Query { __typename } fragment F on Query { id }":   `There can only be one fragment named "F".`,
	} {
		err := validateGraphQLComplexity(doc)
		if want == "" && err != nil {
			t.Errorf("%.80q refused: %v", doc, err)
		}
		if want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%.80q: err = %v, want %q", doc, err, want)
		}
	}
}

// TestGraphQLClientDocumentsPassTheGates: every operation the dashboard and
// the mobile app send, and the introspection query codegen and GraphiQL send,
// passes the gates. A client document is its operation with every fragment it
// spreads, from any file, and Apollo adds __typename to each selection set
// below the operation's.
func TestGraphQLClientDocumentsPassTheGates(t *testing.T) {
	comment := regexp.MustCompile(`#[^\n\r]*`)
	fragments := map[string]*ast.FragmentDefinition{}
	var ops []*ast.OperationDefinition
	where := map[*ast.OperationDefinition]string{}
	for _, dir := range []string{"../../../../dashboard/src", "../../../../mobile/src"} {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".graphql") {
				return err
			}
			text, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			// Apollo prints the document it sends, without comments (which
			// hold the only characters graphql-go v0.8.1 misreads).
			doc, err := parseGraphQL(comment.ReplaceAllString(string(text), ""))
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			for _, def := range doc.Definitions {
				switch d := def.(type) {
				case *ast.OperationDefinition:
					ops = append(ops, d)
					where[d] = path
				case *ast.FragmentDefinition:
					fragments[d.Name.Value] = d
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(ops) < 100 {
		t.Fatalf("read %d client operations, want the dashboard's and the mobile app's", len(ops))
	}
	for _, def := range fragments {
		addTypename(def.SelectionSet)
	}
	for _, op := range ops {
		for _, sel := range op.SelectionSet.Selections {
			if f, ok := sel.(*ast.Field); ok && f.SelectionSet != nil {
				addTypename(f.SelectionSet)
			}
		}
		defs := []ast.Node{op}
		for _, name := range spreadFragments(op.SelectionSet, fragments, map[string]bool{}) {
			defs = append(defs, fragments[name])
		}
		text, _ := printer.Print(&ast.Document{Kind: kinds.Document, Definitions: defs}).(string)
		if err := validateGraphQLComplexity(text); err != nil {
			t.Errorf("%s: operation %s refused: %v", where[op], op.Name.Value, err)
		}
	}
	if err := validateGraphQLComplexity(testutil.IntrospectionQuery); err != nil {
		t.Errorf("the introspection query refused: %v", err)
	}
}

// addTypename adds __typename to sel and to every selection set below it
// that lacks one, as Apollo Client does.
func addTypename(sel *ast.SelectionSet) {
	has := false
	for _, s := range sel.Selections {
		switch f := s.(type) {
		case *ast.Field:
			has = has || (f.Alias == nil && f.Name.Value == "__typename")
			if f.SelectionSet != nil {
				addTypename(f.SelectionSet)
			}
		case *ast.InlineFragment:
			addTypename(f.SelectionSet)
		}
	}
	if !has {
		sel.Selections = append(sel.Selections, &ast.Field{Kind: kinds.Field, Name: &ast.Name{Kind: kinds.Name, Value: "__typename"}})
	}
}

// spreadFragments is the names of the fragments sel spreads, directly or
// through other fragments, in a fixed order.
func spreadFragments(sel *ast.SelectionSet, fragments map[string]*ast.FragmentDefinition, seen map[string]bool) []string {
	var names []string
	for _, s := range sel.Selections {
		var inner *ast.SelectionSet
		switch f := s.(type) {
		case *ast.Field:
			inner = f.SelectionSet
		case *ast.InlineFragment:
			inner = f.SelectionSet
		case *ast.FragmentSpread:
			if frag := fragments[f.Name.Value]; frag != nil && !seen[f.Name.Value] {
				seen[f.Name.Value] = true
				names = append(names, f.Name.Value)
				inner = frag.SelectionSet
			}
		}
		if inner != nil {
			names = append(names, spreadFragments(inner, fragments, seen)...)
		}
	}
	sort.Strings(names)
	return names
}
