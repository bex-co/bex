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
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/kinds"
	"github.com/graphql-go/graphql/language/location"
	"github.com/graphql-go/graphql/language/visitor"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// runGraphQL is graphql.Do over the document the gates already parsed:
// graphql-go's validation, then execution. graphql.Do adds only schema
// extensions, which bex's schema has none of. The validation runs
// graphqlValidationRules, and the errors are located afterwards (see
// parseGraphQL).
func (s *Server) runGraphQL(ctx context.Context, body graphqlRequestBody, doc *ast.Document, parseErr error) *graphql.Result {
	var result *graphql.Result
	if parseErr != nil {
		result = &graphql.Result{Errors: gqlerrors.FormatErrors(parseErr)}
	} else if v := graphql.ValidateDocument(&s.schema, doc, graphqlValidationRules); !v.IsValid {
		result = &graphql.Result{Errors: v.Errors}
	} else {
		result = graphql.Execute(graphql.ExecuteParams{
			Schema:        s.schema,
			AST:           doc,
			OperationName: body.OperationName,
			Args:          body.Variables,
			// One document is one API call: a read its fields share (a sandbox
			// group's listing, w1/m658) is made once (core.Memoized).
			Context: core.WithRequestMemo(ctx),
		})
	}
	locateGraphQLErrors(body.Query, result.Errors)
	return result
}

// graphqlValidationRules is graphql-go's specified rule set with its two
// rules that suggest names replaced. For each unknown field or type name they
// compute an edit distance to every field of the parent type, or to every type
// in the schema, so their cost grew with the bytes of unknown names times the
// schema's: 2,000 unknown 25-character fields, which the cost gate admits,
// took two seconds, and one 10 KB type name most of one. The replacements
// report the same error without the "Did you mean" list.
var graphqlValidationRules = func() []graphql.ValidationRuleFn {
	replace := map[uintptr]graphql.ValidationRuleFn{
		reflect.ValueOf(graphql.FieldsOnCorrectTypeRule).Pointer(): fieldsOnCorrectTypeRule,
		reflect.ValueOf(graphql.KnownTypeNamesRule).Pointer():      knownTypeNamesRule,
	}
	rules := slices.Clone(graphql.SpecifiedRules)
	for i, rule := range rules {
		if r, ok := replace[reflect.ValueOf(rule).Pointer()]; ok {
			rules[i] = r
		}
	}
	return rules
}()

// fieldsOnCorrectTypeRule is graphql.FieldsOnCorrectTypeRule without its
// suggestions: a field its parent type does not define is refused.
func fieldsOnCorrectTypeRule(context *graphql.ValidationContext) *graphql.ValidationRuleInstance {
	return &graphql.ValidationRuleInstance{VisitorOpts: &visitor.VisitorOptions{
		KindFuncMap: map[string]visitor.NamedVisitFuncs{
			kinds.Field: {Kind: func(p visitor.VisitFuncParams) (string, any) {
				node, ok := p.Node.(*ast.Field)
				parent := context.ParentType()
				if !ok || parent == nil || reflect.ValueOf(parent).IsNil() || context.FieldDef() != nil {
					return visitor.ActionNoChange, nil
				}
				name := ""
				if node.Name != nil {
					name = node.Name.Value
				}
				context.ReportError(validationError(graphql.UndefinedFieldMessage(name, parent.Name(), nil, nil), node))
				return visitor.ActionNoChange, nil
			}},
		},
	}}
}

// knownTypeNamesRule is graphql.KnownTypeNamesRule without its suggestions:
// a type name the schema does not define is refused, outside type system
// definitions, which name types of their own.
func knownTypeNamesRule(context *graphql.ValidationContext) *graphql.ValidationRuleInstance {
	skip := visitor.NamedVisitFuncs{Kind: func(visitor.VisitFuncParams) (string, any) {
		return visitor.ActionSkip, nil
	}}
	return &graphql.ValidationRuleInstance{VisitorOpts: &visitor.VisitorOptions{
		KindFuncMap: map[string]visitor.NamedVisitFuncs{
			kinds.ObjectDefinition:      skip,
			kinds.InterfaceDefinition:   skip,
			kinds.UnionDefinition:       skip,
			kinds.InputObjectDefinition: skip,
			kinds.Named: {Kind: func(p visitor.VisitFuncParams) (string, any) {
				node, ok := p.Node.(*ast.Named)
				if !ok {
					return visitor.ActionNoChange, nil
				}
				name := ""
				if node.Name != nil {
					name = node.Name.Value
				}
				if context.Schema().Type(name) == nil {
					context.ReportError(validationError(fmt.Sprintf(`Unknown type "%s".`, name), node))
				}
				return visitor.ActionNoChange, nil
			}},
		},
	}}
}

// validationError is a validation error at node, as graphql-go builds one.
func validationError(message string, node ast.Node) error {
	return gqlerrors.NewError(message, []ast.Node{node}, "", nil, []int{}, nil)
}

// graphqlLineBreak is what graphql-go's location.GetLocation counts as a
// line break.
var graphqlLineBreak = regexp.MustCompile("\r\n|[\n\r]")

// locateGraphQLErrors gives each error location its line and column. The
// document was parsed without its source (parseGraphQL), so graphql-go left
// every location it computed from a node at line 1, column offset+1; this
// finds the document's line breaks once and computes what
// location.GetLocation would have, for every error. A location already on a
// later line is a syntax error's, located by the parser; one genuinely on line
// 1 maps to itself.
func locateGraphQLErrors(query string, errs []gqlerrors.FormattedError) {
	var breaks [][]int
	read := false
	for i := range errs {
		for j, loc := range errs[i].Locations {
			if loc.Line != 1 {
				continue
			}
			if !read {
				breaks, read = graphqlLineBreak.FindAllStringIndex(query, -1), true
			}
			offset := loc.Column - 1
			line := sort.Search(len(breaks), func(k int) bool { return breaks[k][0] >= offset })
			if line > 0 {
				errs[i].Locations[j] = location.SourceLocation{Line: line + 1, Column: offset + 1 - breaks[line-1][1]}
			}
		}
	}
}
