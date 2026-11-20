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
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql/language/ast"
)

// TestValidateGraphQLComplexity pins w1/m65 F9: over-budget documents are
// rejected BEFORE execution (no resolver runs), while ordinary queries pass and
// a genuine parse error is left for graphql.Do to report.
func TestValidateGraphQLComplexity(t *testing.T) {
	// Ordinary query — accepted.
	if err := validateGraphQLComplexity(`query { services { id name } }`); err != nil {
		t.Fatalf("ordinary query rejected: %v", err)
	}

	// Alias amplification — a near-limit document invoking one resolver hundreds
	// of times — is rejected.
	var aliases strings.Builder
	aliases.WriteString("{ ")
	for i := 0; i < gqlMaxAliases+5; i++ {
		fmt.Fprintf(&aliases, "a%d: services { id } ", i)
	}
	aliases.WriteString("}")
	if err := validateGraphQLComplexity(aliases.String()); err == nil {
		t.Error("alias-heavy document should be rejected before execution")
	}

	// Excessive nesting is rejected.
	inner := "id"
	for i := 0; i < gqlMaxDepth+5; i++ {
		inner = "node { " + inner + " }"
	}
	if err := validateGraphQLComplexity("{ " + inner + " }"); err == nil {
		t.Error("deeply nested document should be rejected")
	}

	// Too many operations in one document is rejected.
	var ops strings.Builder
	for i := 0; i < gqlMaxOperations+2; i++ {
		fmt.Fprintf(&ops, "query o%d { id } ", i)
	}
	if err := validateGraphQLComplexity(ops.String()); err == nil {
		t.Error("document with too many operations should be rejected")
	}

	// A syntactically invalid document is NOT rejected here — the execution
	// step produces the canonical parse error.
	if err := validateGraphQLComplexity("{ this is (((not valid"); err != nil {
		t.Errorf("parse error should pass through, got %v", err)
	}
}

// TestGraphQLPerFieldAliasBudget pins codex-security round 12, finding 4: the
// amplification shape is ONE expensive resolver aliased many times (projects —
// a full-workspace list with per-project enrichment per call). A document that
// aliases a single field name past gqlMaxAliasesPerField is rejected before
// execution even though it stays inside the total alias/field budgets, while a
// document spreading the same alias count across DISTINCT fields passes.
func TestGraphQLPerFieldAliasBudget(t *testing.T) {
	var oneField strings.Builder
	oneField.WriteString("{ ")
	for i := 0; i < gqlMaxAliasesPerField+1; i++ {
		fmt.Fprintf(&oneField, "p%d: projects { id } ", i)
	}
	oneField.WriteString("}")
	if err := validateGraphQLComplexity(oneField.String()); err == nil {
		t.Error("document aliasing one field past the per-field budget should be rejected")
	}

	// Distinct field names, same total alias count — diverse reads are not the
	// amplification shape and must stay accepted.
	var diverse strings.Builder
	diverse.WriteString("{ ")
	for i := 0; i < gqlMaxAliasesPerField; i++ {
		fmt.Fprintf(&diverse, "s%d: services { id } p%d: projects { id } ", i, i)
	}
	diverse.WriteString("}")
	if err := validateGraphQLComplexity(diverse.String()); err != nil {
		t.Errorf("diverse aliased reads within budgets should pass, got %v", err)
	}

	// The per-field cap is also enforced through a fragment spread — cost can't
	// be hidden behind fragments.
	frag := "fragment F on Query { f1: projects { id } f2: projects { id } f3: projects { id } f4: projects { id } f5: projects { id } f6: projects { id } f7: projects { id } f8: projects { id } f9: projects { id } f10: projects { id } f11: projects { id } }"
	if err := validateGraphQLComplexity("query { ...F } " + frag); err == nil {
		t.Error("per-field alias budget must hold through fragment spreads")
	}
}

// TestFragmentSpreadDepthParity pins codex-security target #8 as a NON-issue.
// A fragment spread does not introduce a nesting level in GraphQL — the fields
// inside a fragment sit at the same depth as the spread site — and the cost
// walker models exactly that: depth is charged on Field selection sets, and a
// spread is walked at the SAME depth. So a document nested through fragments is
// accepted/rejected at EXACTLY the same depth as the inline form: fragments can
// neither smuggle extra depth past gqlMaxDepth (a bypass) nor over-count a
// within-limit query (a false rejection). Incrementing depth on the spread, as
// the finding proposed, would break the second property.
func TestFragmentSpreadDepthParity(t *testing.T) {
	// inlineNest: `{ node { node { … id } } }` with k `node` levels.
	inlineNest := func(k int) string {
		inner := "id"
		for i := 0; i < k; i++ {
			inner = "node { " + inner + " }"
		}
		return "{ " + inner + " }"
	}
	// fragmentNest: the SAME k `node` levels, but every child selection set is
	// reached through a fragment spread (depth-transparent in GraphQL).
	fragmentNest := func(k int) string {
		var b strings.Builder
		b.WriteString("query { ...F0 } ")
		for i := 0; i < k; i++ {
			fmt.Fprintf(&b, "fragment F%d on T { node { ...F%d } } ", i, i+1)
		}
		fmt.Fprintf(&b, "fragment F%d on T { id } ", k)
		return b.String()
	}

	// Sweep across the depth boundary: the inline and fragment forms must agree
	// on accept/reject at every level.
	for k := gqlMaxDepth - 2; k <= gqlMaxDepth+2; k++ {
		inlineOK := validateGraphQLComplexity(inlineNest(k)) == nil
		fragOK := validateGraphQLComplexity(fragmentNest(k)) == nil
		if inlineOK != fragOK {
			t.Errorf("depth k=%d: inline accepted=%v but fragment accepted=%v — fragments must not change the depth verdict",
				k, inlineOK, fragOK)
		}
	}

	// Concretely: a fragment chain deeper than the limit is still rejected.
	if err := validateGraphQLComplexity(fragmentNest(gqlMaxDepth + 2)); err == nil {
		t.Error("a fragment-nested query past gqlMaxDepth must still be rejected")
	}
	// …and one within the limit still passes (no over-count).
	if err := validateGraphQLComplexity(fragmentNest(gqlMaxDepth - 2)); err != nil {
		t.Errorf("a fragment-nested query within gqlMaxDepth must pass, got %v", err)
	}
}

// fragmentFanOut is a document whose operation selects root, with "%s" in
// root standing for a spread of F0. Each fragment spreads the next one twice
// and the last selects leaf, so 2^n spread paths reach leaf while the
// document grows by one short fragment per step.
func fragmentFanOut(root, typ string, n int, leaf string) string {
	var b strings.Builder
	b.WriteString("query { " + fmt.Sprintf(root, "...F0") + " }")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, " fragment F%d on %s { ...F%d ...F%d }", i, typ, i+1, i+1)
	}
	fmt.Fprintf(&b, " fragment F%d on %s { %s }", n, typ, leaf)
	return b.String()
}

// finishesWithin fails t when f has not returned within ten seconds. The
// walks it bounds take microseconds; one that followed every spread path of
// a fan-out document would never return.
func finishesWithin(t *testing.T, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not finish within 10s", what)
	}
}

func parsedGraphQL(t *testing.T, doc string) (map[string]*ast.FragmentDefinition, []*ast.OperationDefinition) {
	t.Helper()
	fragments, ops, ok := parseGraphQLDocument(doc)
	if !ok {
		t.Fatalf("parse %q", doc)
	}
	return fragments, ops
}

// TestGraphQLFragmentFanOutIsLinear: a named fragment is the same at every
// spread of it, so each walk over a document's fragments reads each fragment
// once. The cost gate still charges a fragment's fields at every spread, so
// fan-out cannot hide cost, and the top-level and sensitive-selection walks
// still see every field. Each document here has 2^64 spread paths: a walk
// that re-read a fragment at every spread would never return.
func TestGraphQLFragmentFanOutIsLinear(t *testing.T) {
	const n = 64

	t.Run("cost gate", func(t *testing.T) {
		for leaf, want := range map[string]string{
			// Every spread is charged, a missing fragment's too.
			"...Missing": fmt.Sprintf("spreads fragments more than %d times", gqlMaxSpreads),
			// Every spread path selects __typename in the operation's
			// selection set.
			"__typename":         fmt.Sprintf("selects %q more than %d times in one selection set", "__typename", gqlMaxSameResponseName),
			"a: services { id }": fmt.Sprintf("aliases field %q more than %d times", "services", gqlMaxAliasesPerField),
			strings.Repeat("node { ", gqlMaxDepth) + "id" + strings.Repeat(" }", gqlMaxDepth): fmt.Sprintf("selection nesting exceeds %d", gqlMaxDepth),
		} {
			fragments, ops := parsedGraphQL(t, fragmentFanOut("%s", "Query", n, leaf))
			var err error
			finishesWithin(t, "cost gate, leaf "+leaf, func() { err = validateGraphQLComplexityParsed(fragments, ops) })
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("leaf %s: err = %v, want %q", leaf, err, want)
			}
		}
		fragments, ops := parsedGraphQL(t, `query { ...A } fragment A on Query { ...B } fragment B on Query { id ...A }`)
		if err := validateGraphQLComplexityParsed(fragments, ops); err == nil || !strings.Contains(err.Error(), "fragment cycle") {
			t.Errorf("fragment cycle: err = %v", err)
		}
	})

	t.Run("top-level fields", func(t *testing.T) {
		// These run before the cost gate, so they get no bound from it.
		fragments, ops := parsedGraphQL(t, fragmentFanOut("%s", "Query", n, "services { id }"))
		var top []string
		var label string
		finishesWithin(t, "top-level walk", func() {
			top = graphqlTopLevelOpsFrom(fragments, ops, "")
			label, _ = graphqlOperationDimsFrom(fragments, ops, "")
		})
		if strings.Join(top, ",") != "GQL Query.services" || label != "services" {
			t.Errorf("top-level fields = %v, label = %q; want [GQL Query.services], services", top, label)
		}
	})

	t.Run("sensitive selection", func(t *testing.T) {
		for leaf, want := range map[string]bool{
			`envVar(key:"FOO") { value }`:          true,
			`secretFile(name:"a.txt") { content }`: true,
			`envVarKeys { key }`:                   false,
			`...Missing`:                           false,
		} {
			fragments, ops := parsedGraphQL(t, fragmentFanOut(`service(id:"web") { %s }`, "Service", n, leaf))
			var got bool
			finishesWithin(t, "sensitive walk, leaf "+leaf, func() { got = graphQLSelectsSensitiveNested(fragments, ops, "") })
			if got != want {
				t.Errorf("leaf %s: sensitive = %v, want %v", leaf, got, want)
			}
		}
		// A spread cycle ends the walk, and a field beside it is still read.
		fragments, ops := parsedGraphQL(t, `query { service(id:"web") { ...A } } fragment A on Service { ...B } fragment B on Service { ...A envVar(key:"K") { value } }`)
		if !graphQLSelectsSensitiveNested(fragments, ops, "") {
			t.Error("a secret beside a spread cycle counted as not sensitive")
		}
	})

	t.Run("handler", func(t *testing.T) {
		// A Services Read key runs the top-level walk, then the cost gate,
		// which refuses the spreads before the grant check or graphql-go
		// reads the document.
		srv, grants := scopedKeyServer(t)
		key := keyOf("key-services-read")
		grants.set(key.Subject, grantOf("services:read"))
		doc := fragmentFanOut(`service(id:"web") { %s }`, "Service", n, "...Missing")
		var out map[string]any
		finishesWithin(t, "POST /graphql", func() { out = graphqlDo(t, srv, key, doc) })
		errs, _ := out["errors"].([]any)
		if len(errs) == 0 {
			t.Fatalf("answer = %v, want the spreads refused", out)
		}
		if msg, _ := errs[0].(map[string]any)["message"].(string); !strings.Contains(msg, "spreads fragments more than") {
			t.Errorf("answer = %v, want the spreads refused", out)
		}
	})
}
