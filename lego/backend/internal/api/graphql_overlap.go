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
	"github.com/graphql-go/graphql/language/ast"
)

// countGraphQLOverlap charges budget with what graphql-go's overlap check
// (OverlappingFieldsCanBeMergedRule, v0.8.1) will do with a document, and
// refuses the document once the budget is spent.
//
// The check compares every two fields one selection set selects under one
// response name, then their sub-selections the same way, and remembers no
// field comparison: fields that repeat a response name at each of several
// levels multiply its comparisons, and each comparison prints both fields'
// argument values with graphql-go's printer, which takes up to 16 µs a byte.
// A 23 KB document repeating one name 10 times at each of three levels, with
// a 12-byte argument on each leaf, kept it busy for 26 s: half a million
// comparisons. The per-selection cap on a response name
// (gqlMaxSameResponseName) does not bound that product, and the field budget
// does not either.
//
// gqlOverlap takes the check's steps without comparing anything. It follows
// every path the check can take and none of its early exits (a conflict ends
// a comparison there), so it never counts less than the check does, and it
// stops as soon as the budget is spent, so its own time is bounded by the
// budget too.
func countGraphQLOverlap(fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition, budget *gqlBudget) error {
	o := &gqlOverlap{fragments: fragments, sets: map[*ast.SelectionSet]*gqlOverlapSet{}, compared: map[[2]string]int{}, budget: budget}
	// The check runs on every selection set of the document, operations'
	// and fragments' alike, and follows no spread to get there.
	for _, op := range ops {
		if err := o.visit(op.SelectionSet); err != nil {
			return err
		}
	}
	for _, f := range fragments {
		if err := o.visit(f.SelectionSet); err != nil {
			return err
		}
	}
	return nil
}

type gqlOverlap struct {
	fragments map[string]*ast.FragmentDefinition
	// sets holds each selection set as the check reads it, read once as the
	// check caches it.
	sets map[*ast.SelectionSet]*gqlOverlapSet
	// compared counts how often two fragments were compared. The check
	// remembers that, and compares two fragments at most twice: once with
	// their parents mutually exclusive and once not.
	compared map[[2]string]int
	budget   *gqlBudget
}

// gqlOverlapSet is a selection set as the check reads it: its fields by
// response name in first-seen order, its inline fragments' fields merged in,
// and the fragments it spreads, each once.
type gqlOverlapSet struct {
	names     []string
	fields    map[string][]*ast.Field
	fragments []string
}

func (o *gqlOverlap) set(sel *ast.SelectionSet) *gqlOverlapSet {
	if s, ok := o.sets[sel]; ok {
		return s
	}
	s := &gqlOverlapSet{fields: map[string][]*ast.Field{}}
	spread := map[string]bool{}
	var collect func(sel *ast.SelectionSet)
	collect = func(sel *ast.SelectionSet) {
		if sel == nil {
			return
		}
		for _, selection := range sel.Selections {
			switch f := selection.(type) {
			case *ast.Field:
				name := ""
				if f.Name != nil {
					name = f.Name.Value
				}
				if f.Alias != nil {
					name = f.Alias.Value
				}
				if _, seen := s.fields[name]; !seen {
					s.names = append(s.names, name)
				}
				s.fields[name] = append(s.fields[name], f)
			case *ast.FragmentSpread:
				name := ""
				if f.Name != nil {
					name = f.Name.Value
				}
				if !spread[name] {
					spread[name] = true
					s.fragments = append(s.fragments, name)
				}
			case *ast.InlineFragment:
				collect(f.SelectionSet)
			}
		}
	}
	collect(sel)
	o.sets[sel] = s
	return s
}

// fragment is the named fragment's selection set, or nil when there is none
// (the check skips a spread of a fragment the document does not define).
func (o *gqlOverlap) fragment(name string) *gqlOverlapSet {
	f := o.fragments[name]
	if f == nil || f.SelectionSet == nil {
		return nil
	}
	return o.set(f.SelectionSet)
}

// visit takes the check's steps on sel and on every selection set below it:
// findConflictsWithinSelectionSet, which the check runs on each.
func (o *gqlOverlap) visit(sel *ast.SelectionSet) error {
	if sel == nil {
		return nil
	}
	s := o.set(sel)
	for _, name := range s.names {
		fields := s.fields[name]
		for i := range fields {
			for _, other := range fields[i+1:] {
				if err := o.compare(fields[i], other); err != nil {
					return err
				}
			}
		}
	}
	for i, name := range s.fragments {
		if err := o.fieldsAndFragment(s, name); err != nil {
			return err
		}
		for _, other := range s.fragments[i+1:] {
			if err := o.fragmentPair(name, other); err != nil {
				return err
			}
		}
	}
	for _, selection := range sel.Selections {
		switch f := selection.(type) {
		case *ast.Field:
			if err := o.visit(f.SelectionSet); err != nil {
				return err
			}
		case *ast.InlineFragment:
			if err := o.visit(f.SelectionSet); err != nil {
				return err
			}
		}
	}
	return nil
}

// compare is findConflict on two fields: a step that prints both fields'
// argument values, then their sub-selections compared as
// findConflictsBetweenSubSelectionSets does.
func (o *gqlOverlap) compare(f, g *ast.Field) error {
	if err := o.budget.spendOverlap(1, gqlArgumentBytes(f)+gqlArgumentBytes(g)); err != nil {
		return err
	}
	if f.SelectionSet == nil || g.SelectionSet == nil {
		return nil
	}
	s1, s2 := o.set(f.SelectionSet), o.set(g.SelectionSet)
	if err := o.between(s1, s2); err != nil {
		return err
	}
	for _, name := range s2.fragments {
		if err := o.fieldsAndFragment(s1, name); err != nil {
			return err
		}
	}
	for _, name := range s1.fragments {
		if err := o.fieldsAndFragment(s2, name); err != nil {
			return err
		}
	}
	for _, a := range s1.fragments {
		for _, b := range s2.fragments {
			if err := o.fragmentPair(a, b); err != nil {
				return err
			}
		}
	}
	return nil
}

// between is collectConflictsBetween: a step per response name of s1, and a
// comparison of each field s1 selects under it with each s2 selects.
func (o *gqlOverlap) between(s1, s2 *gqlOverlapSet) error {
	if err := o.budget.spendOverlap(len(s1.names), 0); err != nil {
		return err
	}
	for _, name := range s1.names {
		for _, f := range s1.fields[name] {
			for _, g := range s2.fields[name] {
				if err := o.compare(f, g); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// fieldsAndFragment is collectConflictsBetweenFieldsAndFragment: s's fields
// against the fragment's, then, as graphql-go does, the fragment's fields
// against each fragment it spreads, down its spreads. Nothing is remembered.
func (o *gqlOverlap) fieldsAndFragment(s *gqlOverlapSet, name string) error {
	if err := o.budget.spendOverlap(1, 0); err != nil {
		return err
	}
	f := o.fragment(name)
	if f == nil {
		return nil
	}
	if err := o.between(s, f); err != nil {
		return err
	}
	for _, inner := range f.fragments {
		if err := o.fieldsAndFragment(f, inner); err != nil {
			return err
		}
	}
	return nil
}

// fragmentPair is collectConflictsBetweenFragments: two different fragments'
// fields, then each against the fragments the other spreads.
func (o *gqlOverlap) fragmentPair(a, b string) error {
	if a == b {
		return nil
	}
	pair := [2]string{min(a, b), max(a, b)}
	if o.compared[pair] == 2 {
		return nil
	}
	o.compared[pair]++
	if err := o.budget.spendOverlap(1, 0); err != nil {
		return err
	}
	fa, fb := o.fragment(a), o.fragment(b)
	if fa == nil || fb == nil {
		return nil
	}
	if err := o.between(fa, fb); err != nil {
		return err
	}
	for _, inner := range fb.fragments {
		if err := o.fragmentPair(a, inner); err != nil {
			return err
		}
	}
	for _, inner := range fa.fragments {
		if err := o.fragmentPair(inner, b); err != nil {
			return err
		}
	}
	return nil
}

// gqlArgumentBytes is the length of a field's argument values in the
// document, variables included: the overlap check prints each to compare it.
func gqlArgumentBytes(f *ast.Field) int {
	n := 0
	for _, a := range f.Arguments {
		n += gqlValueBytes(a.Value)
	}
	return n
}
