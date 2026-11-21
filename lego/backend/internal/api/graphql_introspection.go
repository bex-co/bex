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
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// countGraphQLIntrospection charges budget one call for every resolver call
// the operations' introspection fields (__schema and __type) will make, and
// refuses the document once the budget is spent.
//
// graphql-go answers introspection from the schema in memory and never reads
// the request context there: the execution deadline ends the response, but
// the goroutine executing the document runs on to the end. The cost gate's
// field and depth budgets do not bound that work either, since every list in
// the schema's description multiplies the fields below it: a 28 KB document
// of 20 aliased copies of `types { fields { type { ofType { fields … } } } }`
// made 3.6 million resolver calls, 1.8 s and 3.5 GB of allocations, and
// returned 89 MB.
//
// An introspection answer depends on the schema alone, so its calls can be
// counted exactly before execution: each field costs one call on every
// object its selection set is read on, and the objects below a field are
// read from the schema. A selection set's objects are counted by kind and
// identity, so a count costs no more than the calls it charges. Arguments
// that narrow an answer (includeDeprecated: false) are taken at their widest.
// Only the query root type has __schema and __type, and no field returns it,
// so they are looked for at an operation's root and in the fragments spread
// there.
func countGraphQLIntrospection(schema *graphql.Schema, fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition, variables map[string]any, budget *gqlBudget) error {
	for _, op := range ops {
		c := introspectionCount{schema: schema, fragments: fragments, variables: map[string]any{}, budget: budget}
		// A variable the request does not set takes its default.
		for _, def := range op.VariableDefinitions {
			if def.Variable == nil || def.Variable.Name == nil {
				continue
			}
			if v, ok := variables[def.Variable.Name.Value]; ok {
				c.variables[def.Variable.Name.Value] = v
			} else if d, ok := def.DefaultValue.(*ast.StringValue); ok {
				c.variables[def.Variable.Name.Value] = d.Value
			}
		}
		if err := c.root(op.SelectionSet); err != nil {
			return err
		}
	}
	return nil
}

// introspectionKind is the introspection type a selection set is read on.
type introspectionKind int

const (
	introspectSchema introspectionKind = iota
	introspectType
	introspectField
	introspectInputValue
	introspectEnumValue
	introspectDirective
)

// introspected is the objects a selection set is read on, each with the
// number of times: schema types, fields, arguments and input fields, enum
// values and directives, or the schema itself (nil).
type introspected map[any]int

func (o introspected) calls() int {
	n := 0
	for _, times := range o {
		n += times
	}
	return n
}

type introspectionCount struct {
	schema    *graphql.Schema
	fragments map[string]*ast.FragmentDefinition
	variables map[string]any
	budget    *gqlBudget
}

// root walks an operation's root selection set for its introspection fields.
func (c *introspectionCount) root(set *ast.SelectionSet) error {
	if set == nil {
		return nil
	}
	for _, sel := range set.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			if s.Name == nil {
				continue
			}
			var objects introspected
			kind := introspectSchema
			switch s.Name.Value {
			case "__schema":
				objects = introspected{nil: 1}
			case "__type":
				kind = introspectType
				if t := c.namedType(s.Arguments); t != nil {
					objects = introspected{t: 1}
				}
			default:
				continue
			}
			if err := c.budget.spendIntrospection(1); err != nil {
				return err
			}
			if err := c.selection(kind, objects, s.SelectionSet); err != nil {
				return err
			}
		case *ast.InlineFragment:
			if err := c.root(s.SelectionSet); err != nil {
				return err
			}
		case *ast.FragmentSpread:
			if frag := c.fragment(s); frag != nil {
				if err := c.root(frag.SelectionSet); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// namedType is the type __type(name:) names, or nil when it names none.
func (c *introspectionCount) namedType(args []*ast.Argument) graphql.Type {
	for _, a := range args {
		if a.Name == nil || a.Name.Value != "name" {
			continue
		}
		var name string
		switch v := a.Value.(type) {
		case *ast.StringValue:
			name = v.Value
		case *ast.Variable:
			if v.Name != nil {
				name, _ = c.variables[v.Name.Value].(string)
			}
		}
		if t := c.schema.Type(name); t != nil {
			return t
		}
	}
	return nil
}

func (c *introspectionCount) fragment(s *ast.FragmentSpread) *ast.FragmentDefinition {
	if s.Name == nil {
		return nil
	}
	return c.fragments[s.Name.Value]
}

// selection charges set's fields on objects and walks the selection sets
// below them.
func (c *introspectionCount) selection(kind introspectionKind, objects introspected, set *ast.SelectionSet) error {
	if set == nil || len(objects) == 0 {
		return nil
	}
	calls := objects.calls()
	for _, sel := range set.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			if err := c.budget.spendIntrospection(calls); err != nil {
				return err
			}
			if s.Name == nil || s.SelectionSet == nil {
				continue
			}
			if to, below, ok := c.below(kind, s.Name.Value, objects); ok {
				if err := c.selection(to, below, s.SelectionSet); err != nil {
					return err
				}
			}
		case *ast.InlineFragment:
			if err := c.selection(kind, objects, s.SelectionSet); err != nil {
				return err
			}
		case *ast.FragmentSpread:
			if frag := c.fragment(s); frag != nil {
				if err := c.selection(kind, objects, frag.SelectionSet); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// below is the kind of the objects field answers with when read on objects,
// and those objects, or false when it answers with none (a leaf, or a field
// graphql-go's validation refuses).
func (c *introspectionCount) below(kind introspectionKind, field string, objects introspected) (introspectionKind, introspected, bool) {
	to, children, ok := introspectionChildren(c.schema, kind, field)
	if !ok {
		return 0, nil, false
	}
	out := introspected{}
	for object, times := range objects {
		for _, child := range children(object) {
			out[child] += times
		}
	}
	return to, out, true
}

// introspectionChildren is how a field of an introspection type answers: the
// kind of the objects it answers with, and those objects for one object it
// is read on.
func introspectionChildren(schema *graphql.Schema, kind introspectionKind, field string) (introspectionKind, func(any) []any, bool) {
	switch kind {
	case introspectSchema:
		switch field {
		case "types":
			return introspectType, func(any) []any {
				var out []any
				for _, t := range schema.TypeMap() {
					out = append(out, t)
				}
				return out
			}, true
		case "queryType", "mutationType", "subscriptionType":
			root := map[string]*graphql.Object{"queryType": schema.QueryType(), "mutationType": schema.MutationType(), "subscriptionType": schema.SubscriptionType()}[field]
			return introspectType, func(any) []any {
				if root == nil {
					return nil
				}
				return []any{root}
			}, true
		case "directives":
			return introspectDirective, func(any) []any { return asAny(schema.Directives()) }, true
		}
	case introspectType:
		switch field {
		case "fields":
			return introspectField, func(object any) []any {
				switch t := object.(type) {
				case *graphql.Object:
					return valuesAsAny(t.Fields())
				case *graphql.Interface:
					return valuesAsAny(t.Fields())
				}
				return nil
			}, true
		case "interfaces":
			return introspectType, func(object any) []any {
				if t, ok := object.(*graphql.Object); ok {
					return asAny(t.Interfaces())
				}
				return nil
			}, true
		case "possibleTypes":
			return introspectType, func(object any) []any {
				if t, ok := object.(graphql.Abstract); ok {
					return asAny(schema.PossibleTypes(t))
				}
				return nil
			}, true
		case "enumValues":
			return introspectEnumValue, func(object any) []any {
				if t, ok := object.(*graphql.Enum); ok {
					return asAny(t.Values())
				}
				return nil
			}, true
		case "inputFields":
			return introspectInputValue, func(object any) []any {
				if t, ok := object.(*graphql.InputObject); ok {
					return valuesAsAny(t.Fields())
				}
				return nil
			}, true
		case "ofType":
			return introspectType, func(object any) []any {
				switch t := object.(type) {
				case *graphql.List:
					return []any{t.OfType}
				case *graphql.NonNull:
					return []any{t.OfType}
				}
				return nil
			}, true
		}
	case introspectField:
		switch field {
		case "args":
			return introspectInputValue, func(object any) []any { return asAny(object.(*graphql.FieldDefinition).Args) }, true
		case "type":
			return introspectType, func(object any) []any { return []any{object.(*graphql.FieldDefinition).Type} }, true
		}
	case introspectInputValue:
		if field == "type" {
			return introspectType, func(object any) []any {
				switch v := object.(type) {
				case *graphql.Argument:
					return []any{v.Type}
				case *graphql.InputObjectField:
					return []any{v.Type}
				}
				return nil
			}, true
		}
	case introspectDirective:
		if field == "args" {
			return introspectInputValue, func(object any) []any { return asAny(object.(*graphql.Directive).Args) }, true
		}
	}
	return 0, nil, false
}

func asAny[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func valuesAsAny[K comparable, V any](m map[K]V) []any {
	out := make([]any, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
