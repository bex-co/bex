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

// parseGraphQLDocument splits a query into operations and named fragments.
// Parse failures return ok=false so callers can defer to graphql.Do's
// canonical parse error.
func parseGraphQLDocument(query string) (fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition, ok bool) {
	doc, err := parser.Parse(parser.ParseParams{Source: source.NewSource(&source.Source{Body: []byte(query)})})
	if err != nil {
		return nil, nil, false
	}
	fragments = map[string]*ast.FragmentDefinition{}
	for _, def := range doc.Definitions {
		switch d := def.(type) {
		case *ast.OperationDefinition:
			ops = append(ops, d)
		case *ast.FragmentDefinition:
			if d.Name != nil {
				fragments[d.Name.Value] = d
			}
		}
	}
	return fragments, ops, true
}

// validateGraphQLComplexity parses query and rejects it when it exceeds any
// cost budget. A parse failure is NOT rejected here — it returns nil so
// graphql.Do produces the canonical parse error the client expects. Fragment
// spreads are resolved (with cycle detection) so cost can't be hidden behind
// fragments.
func validateGraphQLComplexity(query string) error {
	fragments, ops, ok := parseGraphQLDocument(query)
	if !ok {
		return nil // let graphql.Do report the parse error
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
		if err := c.walk(op.SelectionSet, 1, &doc); err != nil {
			return err
		}
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

// gqlTally is what a selection set charges to the budgets: the fields and
// aliases it selects, each counted once for every spread path that reaches
// it, and the deepest nesting level it reaches.
type gqlTally struct {
	fields  int
	aliases int
	// perField counts aliases by field name — the per-field-name budget's input.
	perField map[string]int
	// depth is the deepest level reached, the selection set's own level
	// being the level it was read at.
	depth int
}

// add charges fields, aliases and per-name aliases to t and refuses the
// document once a budget is exceeded. Every count only grows, so a tally
// over a budget means the whole document is over it.
func (t *gqlTally) add(fields, aliases int, perField map[string]int) error {
	t.fields += fields
	t.aliases += aliases
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

func (c *gqlCost) walk(sel *ast.SelectionSet, depth int, t *gqlTally) error {
	if err := t.reach(depth); err != nil {
		return err
	}
	for _, s := range sel.Selections {
		switch f := s.(type) {
		case *ast.Field:
			var aliased map[string]int
			if f.Alias != nil {
				name := "(anonymous)"
				if f.Name != nil {
					name = f.Name.Value
				}
				aliased = map[string]int{name: 1}
			}
			if err := t.add(1, len(aliased), aliased); err != nil {
				return err
			}
			if f.SelectionSet != nil {
				if err := c.walk(f.SelectionSet, depth+1, t); err != nil {
					return err
				}
			}
		case *ast.InlineFragment:
			if f.SelectionSet != nil {
				if err := c.walk(f.SelectionSet, depth+1, t); err != nil {
					return err
				}
			}
		case *ast.FragmentSpread:
			if f.Name == nil {
				continue
			}
			ft, err := c.fragment(f.Name.Value)
			if err != nil {
				return err
			}
			if ft == nil {
				continue
			}
			// A spread adds no nesting: the fragment's own level 1 is the
			// spread's level.
			if err := t.reach(depth + ft.depth - 1); err != nil {
				return err
			}
			if err := t.add(ft.fields, ft.aliases, ft.perField); err != nil {
				return err
			}
		}
	}
	return nil
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
	if err := c.walk(frag.SelectionSet, 1, ft); err != nil {
		return nil, err
	}
	delete(c.visiting, name)
	c.tallied[name] = ft
	return ft, nil
}
