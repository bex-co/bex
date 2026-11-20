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
	"fmt"
	"time"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/lexer"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

// HTTPWriteTimeout is bex-api's http.Server WriteTimeout (cmd/api newHTTPServer).
// It lives here, beside the execution budget that must stay inside it.
const HTTPWriteTimeout = 30 * time.Second

// gqlExecTimeout bounds a single GraphQL document's execution so one expensive
// query can't tie up a resolver goroutine indefinitely (w1/m65 F9). It sits
// strictly inside HTTPWriteTimeout: when the two were equal, a resolver that ran
// to the deadline had its connection cut before the deadline error could be
// written, and the edge answered with a CORS-less 502 the browser can only call
// "Failed to fetch" (w4/m140). The margin covers auth, parsing, and writing the
// response.
const gqlExecTimeout = HTTPWriteTimeout - 5*time.Second

// GraphQL query-cost budgets (w1/m65 F9). The rate limiter charges one HTTP
// request per document, but one document can invoke many independent
// resolvers (an alias-heavy query calls an expensive provider-backed resolver N
// times). These bounds reject a document BEFORE execution — before any resolver,
// database, or provider call — so a near-limit request can't be amplified into a
// memory or upstream-work DoS against the shared API. Generous enough that no
// legitimate dashboard/CLI query trips them.
//
// codex-security round 12, finding 4 tightened the alias budgets: the original
// 500-alias allowance let ONE accepted document run a full-workspace list
// resolver (projects — all rows + per-project membership enrichment) hundreds
// of times while the limiter charged a single request token. Aliases are now
// bounded in total AND per field name, the shape of real amplification: no
// legitimate client aliases one field more than a handful of times.
const (
	gqlMaxDepth      = 15   // nesting depth
	gqlMaxAliases    = 50   // aliased fields in total (alias amplification)
	gqlMaxFields     = 2000 // total selected fields across the document
	gqlMaxOperations = 10   // operations in one document
	// gqlMaxAliasesPerField caps how many times ONE field name may be aliased
	// in a document — `p1: projects{…} p2: projects{…} …` is the amplification
	// shape; diverse aliased reads across different fields are not.
	gqlMaxAliasesPerField = 10
)

// Budgets on graphql-go's own validation, which runs inside the execution
// call after every bex gate and never looks at the request context, so the
// execution deadline does not bound it. Its overlap check compares every pair
// of fields that share a response name in one selection set, and follows each
// spread fragment down every spread path below it, so a small document of
// repeated fields or of fragments that spread two others each kept a CPU busy
// for minutes. Each budget sits far above what bex's clients send: the
// largest dashboard and mobile documents (2026-11-20) are under 3 KB, select
// one response name at most 3 times in a selection set and spread at most 3
// fragments, and the standard introspection query spreads 10 along its
// paths. Render's GraphQL API is not public, so no parity bound applies.
const (
	// gqlMaxDocumentBytes caps one document's text, checked before parsing.
	gqlMaxDocumentBytes = 32 << 10
	// gqlMaxValueDepth caps how deep a value nests lists and objects (an
	// argument, a variable's default, a variable's type), checked before
	// parsing: graphql-go's value checks cost the value's size times its
	// depth, and a 2,000-deep list took seconds. bex's deepest is the mobile
	// metrics filter's 4.
	gqlMaxValueDepth = 10
	// gqlMaxSpreads caps fragment spreads, counted along every spread path
	// as fields are.
	gqlMaxSpreads = 100
	// gqlMaxSameResponseName caps how many fields one selection set selects
	// under one response name, counting its inline fragments and the
	// fragments it spreads: the fields the overlap check compares pairwise.
	gqlMaxSameResponseName = 10
)

// checkGraphQLText refuses, before the document is parsed, one larger than
// gqlMaxDocumentBytes or with a value nested deeper than gqlMaxValueDepth.
// Values appear only between parentheses (arguments and variable
// definitions), which never nest, so a value's depth is the brackets opened
// since the last "(". A token the lexer refuses is left for the parser to
// report.
func checkGraphQLText(query string) error {
	if len(query) > gqlMaxDocumentBytes {
		return fmt.Errorf("query rejected: document is larger than %d bytes", gqlMaxDocumentBytes)
	}
	lex := lexer.Lex(&source.Source{Body: []byte(query)})
	depth := -1 // outside parentheses
	for {
		tok, err := lex(0)
		if err != nil || tok.Kind == lexer.EOF {
			return nil
		}
		switch tok.Kind {
		case lexer.PAREN_L:
			depth = 0
		case lexer.PAREN_R:
			depth = -1
		case lexer.BRACKET_L, lexer.BRACE_L:
			if depth >= 0 {
				depth++
				if depth > gqlMaxValueDepth {
					return fmt.Errorf("query rejected: a value nests lists and objects deeper than %d", gqlMaxValueDepth)
				}
			}
		case lexer.BRACKET_R, lexer.BRACE_R:
			if depth > 0 {
				depth--
			}
		}
	}
}

// parseGraphQL parses a request's document once, for the gates and for
// execution. The nodes keep their offsets but not the source: graphql-go
// reads the source to turn an error's offset into a line and column, scanning
// the whole document again for every error, so a document of many errors cost
// its length times their number. locateGraphQLErrors computes the lines and
// columns in one pass instead. A syntax error still carries its location.
func parseGraphQL(query string) (*ast.Document, error) {
	return parser.Parse(parser.ParseParams{
		Source:  &source.Source{Body: []byte(query), Name: "GraphQL request"},
		Options: parser.ParseOptions{NoSource: true},
	})
}

// graphqlDefinitions splits a document into its operations and its named
// fragments. A fragment name defined twice is refused, as graphql-go refuses
// it: the map would keep one definition, and the gates would never read the
// other, which graphql-go still validates.
func graphqlDefinitions(doc *ast.Document) (fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition, err error) {
	fragments = map[string]*ast.FragmentDefinition{}
	for _, def := range doc.Definitions {
		switch d := def.(type) {
		case *ast.OperationDefinition:
			ops = append(ops, d)
		case *ast.FragmentDefinition:
			if d.Name == nil {
				continue
			}
			if _, dup := fragments[d.Name.Value]; dup {
				return nil, nil, fmt.Errorf(`There can only be one fragment named "%s".`, d.Name.Value)
			}
			fragments[d.Name.Value] = d
		}
	}
	return fragments, ops, nil
}

// parseGraphQLDocument splits a query into operations and named fragments.
// Parse failures (and a fragment name defined twice) return ok=false so
// callers can defer to the canonical error.
func parseGraphQLDocument(query string) (fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition, ok bool) {
	doc, err := parseGraphQL(query)
	if err != nil {
		return nil, nil, false
	}
	fragments, ops, err = graphqlDefinitions(doc)
	return fragments, ops, err == nil
}

// validateGraphQLComplexity parses query and rejects it when it exceeds any
// cost budget, as the request path does. A parse failure is NOT rejected here
// — it returns nil so the execution step reports the canonical parse error
// the client expects. Fragment spreads are resolved (with cycle detection) so
// cost can't be hidden behind fragments.
func validateGraphQLComplexity(query string) error {
	if err := checkGraphQLText(query); err != nil {
		return err
	}
	doc, err := parseGraphQL(query)
	if err != nil {
		return nil // let the execution step report the parse error
	}
	fragments, ops, err := graphqlDefinitions(doc)
	if err != nil {
		return err
	}
	return validateGraphQLComplexityParsed(fragments, ops)
}

// validateGraphQLComplexityParsed is the AST form used when the document was
// already parsed for telemetry / scope (one parse per request).
func validateGraphQLComplexityParsed(fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition) error {
	if len(ops) > gqlMaxOperations {
		return fmt.Errorf("query rejected: too many operations (%d > %d)", len(ops), gqlMaxOperations)
	}
	c := gqlCost{fragments: fragments, tallied: map[string]*gqlTally{}, visiting: map[string]bool{}}
	var doc gqlTally
	for _, op := range ops {
		if op.SelectionSet == nil {
			continue
		}
		if _, err := c.walk(op.SelectionSet, 1, &doc); err != nil {
			return err
		}
	}
	// graphql-go validates every fragment, spread or not, and the walk
	// charged only the ones an operation spreads. A document with a fragment
	// nothing spreads is invalid anyway, so it is refused with graphql-go's
	// message rather than handed over unbudgeted.
	unused := ""
	for name := range fragments {
		if _, spread := c.tallied[name]; !spread && (unused == "" || name < unused) {
			unused = name
		}
	}
	if unused != "" {
		return fmt.Errorf(`Fragment "%s" is never used.`, unused)
	}
	return nil
}

// gqlCost walks a document's selection sets against the budgets. A named
// fragment costs the same at every spread of it, so its tally is taken once
// and added at each spread: the walk reads every selection set of the
// document once, however many spread paths lead to it, and its time is linear
// in the document's size.
type gqlCost struct {
	fragments map[string]*ast.FragmentDefinition
	// tallied holds each fragment's tally once it is complete.
	tallied map[string]*gqlTally
	// visiting holds the fragments whose tally is being taken: the spread
	// path from the operation down to the selection set being read.
	visiting map[string]bool
}

// gqlTally is what a selection set charges to the budgets: the fields,
// aliases and fragment spreads it selects, each counted once for every spread
// path that reaches it, and the deepest nesting level it reaches.
type gqlTally struct {
	fields  int
	aliases int
	spreads int
	// perField counts aliases by field name — the per-field-name budget's input.
	perField map[string]int
	// depth is the deepest level reached, the selection set's own level
	// being the level it was read at.
	depth int
	// names counts, in a fragment's tally, the response names its selection
	// set selects (see walk), which every selection set spreading it selects
	// too.
	names map[string]int
}

// add charges fields, aliases, spreads and per-name aliases to t and refuses
// the document once a budget is exceeded. Every count only grows, so a tally
// over a budget means the whole document is over it.
func (t *gqlTally) add(fields, aliases, spreads int, perField map[string]int) error {
	t.fields += fields
	t.aliases += aliases
	t.spreads += spreads
	over := ""
	for name, n := range perField {
		if t.perField == nil {
			t.perField = map[string]int{}
		}
		t.perField[name] += n
		if t.perField[name] > gqlMaxAliasesPerField && (over == "" || name < over) {
			over = name
		}
	}
	if over != "" {
		return fmt.Errorf("query rejected: aliases field %q more than %d times", over, gqlMaxAliasesPerField)
	}
	if t.fields > gqlMaxFields {
		return fmt.Errorf("query rejected: selects more than %d fields", gqlMaxFields)
	}
	if t.aliases > gqlMaxAliases {
		return fmt.Errorf("query rejected: uses more than %d aliases", gqlMaxAliases)
	}
	if t.spreads > gqlMaxSpreads {
		return fmt.Errorf("query rejected: spreads fragments more than %d times", gqlMaxSpreads)
	}
	return nil
}

// reach records that t's selection reaches nesting level depth.
func (t *gqlTally) reach(depth int) error {
	if depth > gqlMaxDepth {
		return fmt.Errorf("query rejected: selection nesting exceeds %d", gqlMaxDepth)
	}
	t.depth = max(t.depth, depth)
	return nil
}

// walk charges sel's selections to t and returns how many times sel selects
// each response name, counting its inline fragments and the fragments it
// spreads: graphql-go's overlap check compares those fields pairwise, so a
// name selected more than gqlMaxSameResponseName times is refused.
func (c *gqlCost) walk(sel *ast.SelectionSet, depth int, t *gqlTally) (map[string]int, error) {
	if err := t.reach(depth); err != nil {
		return nil, err
	}
	names := map[string]int{}
	for _, s := range sel.Selections {
		switch f := s.(type) {
		case *ast.Field:
			name := "(anonymous)"
			if f.Name != nil {
				name = f.Name.Value
			}
			var aliased map[string]int
			response := name
			if f.Alias != nil {
				aliased = map[string]int{name: 1}
				response = f.Alias.Value
			}
			if err := t.add(1, len(aliased), 0, aliased); err != nil {
				return nil, err
			}
			names[response]++
			if f.SelectionSet != nil {
				if _, err := c.walk(f.SelectionSet, depth+1, t); err != nil {
					return nil, err
				}
			}
		case *ast.InlineFragment:
			if f.SelectionSet != nil {
				inner, err := c.walk(f.SelectionSet, depth+1, t)
				if err != nil {
					return nil, err
				}
				for name, n := range inner {
					names[name] += n
				}
			}
		case *ast.FragmentSpread:
			if err := t.add(0, 0, 1, nil); err != nil {
				return nil, err
			}
			if f.Name == nil {
				continue
			}
			ft, err := c.fragment(f.Name.Value)
			if err != nil {
				return nil, err
			}
			if ft == nil {
				continue
			}
			// A spread adds no nesting: the fragment's own level 1 is the
			// spread's level.
			if err := t.reach(depth + ft.depth - 1); err != nil {
				return nil, err
			}
			if err := t.add(ft.fields, ft.aliases, ft.spreads, ft.perField); err != nil {
				return nil, err
			}
			for name, n := range ft.names {
				names[name] += n
			}
		}
	}
	over := ""
	for name, n := range names {
		if n > gqlMaxSameResponseName && (over == "" || name < over) {
			over = name
		}
	}
	if over != "" {
		return nil, fmt.Errorf("query rejected: selects %q more than %d times in one selection set", over, gqlMaxSameResponseName)
	}
	return names, nil
}

// fragment returns the tally of the named fragment's selection set read at
// level 1, taking it on the first spread and reusing it on every later one.
// A missing fragment costs nothing (graphql.Do reports it); a spread of a
// fragment whose tally is still being taken is a cycle, which is refused.
func (c *gqlCost) fragment(name string) (*gqlTally, error) {
	if ft, ok := c.tallied[name]; ok {
		return ft, nil
	}
	if c.visiting[name] { // cyclic spread — invalid GraphQL; refuse rather than loop
		return nil, fmt.Errorf("query rejected: fragment cycle via %q", name)
	}
	frag, ok := c.fragments[name]
	if !ok || frag.SelectionSet == nil {
		return nil, nil
	}
	c.visiting[name] = true
	ft := &gqlTally{}
	names, err := c.walk(frag.SelectionSet, 1, ft)
	if err != nil {
		return nil, err
	}
	ft.names = names
	delete(c.visiting, name)
	c.tallied[name] = ft
	return ft, nil
}
