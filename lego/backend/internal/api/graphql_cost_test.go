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
