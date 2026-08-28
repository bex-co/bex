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

	// A syntactically invalid document is NOT rejected here — graphql.Do produces
	// the canonical parse error.
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
