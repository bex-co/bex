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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/testutil"
)

// introspectionCopies is a document of schemas aliased __schema fields, each
// selecting types aliased typesPer times, each with sub.
func introspectionCopies(schemas, typesPer int, sub string) string {
	var b strings.Builder
	b.WriteString("{ ")
	for i := range schemas {
		if i > 0 {
			fmt.Fprintf(&b, "s%d: ", i)
		}
		b.WriteString("__schema { ")
		for j := range typesPer {
			if j > 0 {
				fmt.Fprintf(&b, "t%d: ", j)
			}
			b.WriteString("types { " + sub + " } ")
		}
		b.WriteString("} ")
	}
	b.WriteString("}")
	return b.String()
}

// typeFields selects a __Type's fields, their types' fields and so on, depth
// times, through the wrappers a field's type may sit in, with every cheap
// field along the way: each costs a call on every type or field it is read on.
func typeFields(depth int) string {
	typ := "kind name description interfaces { __typename } possibleTypes { __typename } enumValues { __typename } inputFields { __typename }"
	field := "name description isDeprecated deprecationReason args { name }"
	if depth == 0 {
		return typ
	}
	return typ + " fields { " + field + " type { " + typeFields(depth-1) + " } } ofType { " + typ + " ofType { " + typ + " fields { " + field + " } } }"
}

// TestGraphQLIntrospectionIsBounded: graphql-go answers introspection from
// the schema without reading the request context, so the execution deadline
// ends the response but not the work. The cost gate admits this 22 KB
// document, which made 2 million resolver calls, a second of CPU and about
// 2 GB of allocations; it is now refused before execution.
func TestGraphQLIntrospectionIsBounded(t *testing.T) {
	srv, _ := scopedKeyServer(t)
	doc := introspectionCopies(6, 2, typeFields(3))
	if err := validateGraphQLComplexity(doc); err != nil {
		t.Fatalf("the cost gate refuses the document (%v); it no longer shows what the count adds", err)
	}
	var result *graphql.Result
	start := time.Now()
	finishesWithin(t, "the document", func() {
		_, _, _, result = srv.executeGraphQL(context.Background(), graphqlRequestBody{Query: doc}, &gqlBudget{})
	})
	if took := time.Since(start); took > time.Second {
		t.Errorf("took %v, want under a second", took)
	}
	want := fmt.Sprintf("query rejected: the request's introspection makes more than %d resolver calls", gqlMaxRequestIntrospectionCalls)
	if len(result.Errors) != 1 || result.Errors[0].Message != want || result.Data != nil {
		t.Fatalf("errors = %.300v, data returned %v, want only %q", result.Errors, result.Data != nil, want)
	}
	// A batch's documents share the budget: one document of these fits in
	// it, the second is refused.
	one := introspectionCopies(1, 1, typeFields(3))
	budget := &gqlBudget{}
	for i, wantRefused := range []bool{false, true} {
		_, _, _, r := srv.executeGraphQL(context.Background(), graphqlRequestBody{Query: one}, budget)
		if refused := len(r.Errors) == 1 && r.Errors[0].Message == want; refused != wantRefused {
			t.Errorf("document %d of a batch: errors = %.200v, want refused %v", i+1, r.Errors, wantRefused)
		}
	}
}

// resolveCounter counts the fields graphql-go resolves.
type resolveCounter struct{ calls atomic.Int64 }

func (*resolveCounter) Init(ctx context.Context, _ *graphql.Params) context.Context { return ctx }
func (*resolveCounter) Name() string                                                { return "resolveCounter" }
func (*resolveCounter) HasResult() bool                                             { return false }
func (*resolveCounter) GetResult(context.Context) any                               { return nil }
func (*resolveCounter) ParseDidStart(ctx context.Context) (context.Context, graphql.ParseFinishFunc) {
	return ctx, func(error) {}
}
func (*resolveCounter) ValidationDidStart(ctx context.Context) (context.Context, graphql.ValidationFinishFunc) {
	return ctx, func([]gqlerrors.FormattedError) {}
}
func (*resolveCounter) ExecutionDidStart(ctx context.Context) (context.Context, graphql.ExecutionFinishFunc) {
	return ctx, func(*graphql.Result) {}
}
func (c *resolveCounter) ResolveFieldDidStart(ctx context.Context, _ *graphql.ResolveInfo) (context.Context, graphql.ResolveFieldFinishFunc) {
	c.calls.Add(1)
	return ctx, func(any, error) {}
}

// TestGraphQLIntrospectionCountsTheCalls: the count is the number of fields
// graphql-go resolves, for every introspection type, through fragments and
// variables; and the standard introspection query, which codegen and
// GraphiQL send, stays far inside the budget.
func TestGraphQLIntrospectionCountsTheCalls(t *testing.T) {
	srv, _ := scopedKeyServer(t)
	counter := &resolveCounter{}
	srv.schema.AddExtensions(counter)
	for _, tc := range []struct {
		name, doc string
		variables map[string]any
		// data is the fields the document resolves besides introspection.
		data int64
	}{
		{name: "the standard introspection query", doc: testutil.IntrospectionQuery},
		{name: "nested fields", doc: introspectionCopies(1, 1, strings.ReplaceAll(typeFields(2), "fields {", "fields(includeDeprecated: true) {"))},
		{name: "two copies", doc: "{ a: __type(name: \"Service\") { name } b: __type(name: \"Service\") { kind } }"},
		{name: "a type a variable names", doc: "query($t: String!) { __type(name: $t) { fields(includeDeprecated: true) { name args { name type { name } } } } }", variables: map[string]any{"t": "Query"}},
		{name: "a type a default names", doc: "query($t: String = \"Mutation\") { __type(name: $t) { fields(includeDeprecated: true) { name } } }"},
		{name: "a type that does not exist", doc: "{ __type(name: \"Nope\") { name } }"},
		{name: "fragments", doc: "{ ...Root } fragment Root on Query { __schema { ...S } } fragment S on __Schema { queryType { name } mutationType { name } types { ... on __Type { kind enumValues(includeDeprecated: true) { name } } } directives { name locations args { name } } }"},
		{name: "beside data", doc: "{ __typename __schema { queryType { fields(includeDeprecated: true) { name } } } }", data: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fragments, ops, ok := parseGraphQLDocument(tc.doc)
			if !ok {
				t.Fatal("does not parse")
			}
			budget := &gqlBudget{}
			if err := countGraphQLIntrospection(&srv.schema, fragments, ops, tc.variables, budget); err != nil {
				t.Fatal(err)
			}
			counter.calls.Store(0)
			_, _, _, result := srv.executeGraphQL(context.Background(), graphqlRequestBody{Query: tc.doc, Variables: tc.variables}, &gqlBudget{})
			if len(result.Errors) > 0 {
				t.Fatalf("errors: %v", result.Errors)
			}
			if got, want := int64(budget.introspectionCalls), counter.calls.Load()-tc.data; got != want {
				t.Errorf("counted %d calls, graphql-go made %d", got, want)
			}
		})
	}
	fragments, ops, _ := parseGraphQLDocument(testutil.IntrospectionQuery)
	budget := &gqlBudget{}
	if err := countGraphQLIntrospection(&srv.schema, fragments, ops, nil, budget); err != nil || 3*budget.introspectionCalls > gqlMaxRequestIntrospectionCalls {
		t.Errorf("the introspection query makes %d calls (%v), over a third of the budget of %d: raise it before codegen is refused", budget.introspectionCalls, err, gqlMaxRequestIntrospectionCalls)
	}
}

// TestNoFieldReturnsTheQueryType: __schema and __type are counted at an
// operation's root only, which holds while no field returns the query root
// type, the one type that has them.
func TestNoFieldReturnsTheQueryType(t *testing.T) {
	srv, _ := scopedKeyServer(t)
	query := srv.schema.QueryType()
	for name, typ := range srv.schema.TypeMap() {
		var fields graphql.FieldDefinitionMap
		switch o := typ.(type) {
		case *graphql.Object:
			fields = o.Fields()
		case *graphql.Interface:
			fields = o.Fields()
		}
		for field, def := range fields {
			if named, ok := graphql.GetNamed(def.Type).(*graphql.Object); ok && named == query {
				t.Errorf("%s.%s returns the query type: count the introspection below it in countGraphQLIntrospection", name, field)
			}
		}
	}
}
