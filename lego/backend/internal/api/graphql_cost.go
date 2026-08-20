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

// gqlExecTimeout bounds a single GraphQL document's execution so one expensive
// query can't tie up a resolver goroutine indefinitely (w1/m65 F9).
const gqlExecTimeout = 30 * time.Second

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
// fragments; the total-field cap also bounds the validator's own work.
func validateGraphQLComplexity(query string) error {
	fragments, ops, ok := parseGraphQLDocument(query)
	if !ok {
		return nil // let graphql.Do report the parse error
	}
	if len(ops) > gqlMaxOperations {
		return fmt.Errorf("query rejected: too many operations (%d > %d)", len(ops), gqlMaxOperations)
	}
	c := gqlCost{fragments: fragments}
	for _, op := range ops {
		if op.SelectionSet == nil {
			continue
		}
		if err := c.walk(op.SelectionSet, 1, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

// gqlCost accumulates field/alias counts while walking a document's selection
// sets and enforces the depth/breadth budgets.
type gqlCost struct {
	fragments map[string]*ast.FragmentDefinition
	fields    int
	aliases   int
	// perField counts aliases by field name — the per-field-name budget's input.
	perField map[string]int
}

func (c *gqlCost) walk(sel *ast.SelectionSet, depth int, visiting map[string]bool) error {
	if depth > gqlMaxDepth {
		return fmt.Errorf("query rejected: selection nesting exceeds %d", gqlMaxDepth)
	}
	for _, s := range sel.Selections {
		switch f := s.(type) {
		case *ast.Field:
			c.fields++
			if f.Alias != nil {
				c.aliases++
				name := "(anonymous)"
				if f.Name != nil {
					name = f.Name.Value
				}
				if c.perField == nil {
					c.perField = map[string]int{}
				}
				c.perField[name]++
				if c.perField[name] > gqlMaxAliasesPerField {
					return fmt.Errorf("query rejected: aliases field %q more than %d times", name, gqlMaxAliasesPerField)
				}
			}
			if c.fields > gqlMaxFields {
				return fmt.Errorf("query rejected: selects more than %d fields", gqlMaxFields)
			}
			if c.aliases > gqlMaxAliases {
				return fmt.Errorf("query rejected: uses more than %d aliases", gqlMaxAliases)
			}
			if f.SelectionSet != nil {
				if err := c.walk(f.SelectionSet, depth+1, visiting); err != nil {
					return err
				}
			}
		case *ast.InlineFragment:
			if f.SelectionSet != nil {
				if err := c.walk(f.SelectionSet, depth+1, visiting); err != nil {
					return err
				}
			}
		case *ast.FragmentSpread:
			if f.Name == nil {
				continue
			}
			name := f.Name.Value
			if visiting[name] { // cyclic spread — invalid GraphQL; refuse rather than loop
				return fmt.Errorf("query rejected: fragment cycle via %q", name)
			}
			frag, ok := c.fragments[name]
			if !ok || frag.SelectionSet == nil {
				continue
			}
			visiting[name] = true
			if err := c.walk(frag.SelectionSet, depth, visiting); err != nil {
				return err
			}
			delete(visiting, name)
		}
	}
	return nil
}
