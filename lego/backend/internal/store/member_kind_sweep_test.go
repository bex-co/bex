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

package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestEveryMemberQueryDecidesAboutKind is w5/m103's durable form of the
// person/machine split. The rule is not "always filter" — half these reads MUST
// see machine rows, or every API key stops working — it is "decide, and say so
// in the SQL".
//
// A tenant_members SELECT either mentions `kind` (it filters, or reads the
// column), or names itself in kindExemptQueries below with the reason it must
// see machines. Without this the failure mode is silent and is exactly the one
// m103 closed: a new count that forgets the predicate quietly starts charging
// for API keys again, or a new lookup that adds one quietly breaks key
// authorization.
//
// It parses the package's own AST rather than grepping, so backticks inside
// comments cannot be mistaken for SQL.
func TestEveryMemberQueryDecidesAboutKind(t *testing.T) {
	// Reads that must see BOTH kinds, and why. Each key is a distinctive
	// fragment of the query, matched with whitespace normalized (so reformatting
	// the SQL does not break it); a REWORDED query falls out of the list and has
	// to be re-justified, which is the point.
	kindExemptQueries := map[string]string{
		"SELECT role FROM tenant_members WHERE tenant_id = $1 AND subject = $2 FOR UPDATE": "" +
			"the in-transaction role read for ONE named subject — it must see a machine binding so " +
			"the member verbs can refuse it (members.Service guardMachine)",
		"SELECT EXISTS (SELECT 1 FROM tenant_members WHERE subject = $1 AND tenant_id = $2)": "" +
			"IsMember: may this subject act here — a bound key legitimately may",
		"SELECT EXISTS(SELECT 1 FROM tenant_members WHERE tenant_id = $1 AND subject = $2)": "" +
			"invite redemption's already-a-member check: a subject already bound here must not be " +
			"seated twice, whichever kind it is",
		"SELECT t.id, t.name, t.plan, t.created_at FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id WHERE m.subject = $1 ORDER BY m.created_at": "" +
			"TenantForIdentity: which workspace does this subject act in — the key's own binding is the answer",
		"FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id WHERE m.subject = $1 ORDER BY t.created_at": "" +
			"ListTenantsForSubject: the caller's own workspaces; an API key listing its workspace is correct",
		"SELECT count(*) FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id": "" +
			"the per-plan workspace cap counts the SUBJECT's workspaces, not a workspace's people",
		"SELECT tenant_id FROM tenant_members WHERE subject = $1 ORDER BY tenant_id": "" +
			"account deletion enumerates one subject's own memberships (a machine subject never runs this flow)",
		"SELECT tenant_id, subject, role, created_at, kind FROM tenant_members WHERE tenant_id = $1 AND subject = $2": "" +
			"GetTenantMember: the one-row read the member verbs use to DECIDE about a subject — it " +
			"returns the kind rather than filtering on it, which is how guardMachine can refuse",
		"LEFT JOIN tenant_members m ON m.tenant_id = c.tenant_id AND m.subject = c.subject": "" +
			"the push-delivery claim query reads the recipient's role; the recipient is already " +
			"constrained to a registered device/browser subscription, which only a human has",
		"JOIN tenant_members m ON m.tenant_id = d.tenant_id AND m.subject = d.subject": "" +
			"push destinations are reached through a device subscription, which only a human registers; " +
			"the join reads that human's role",
	}

	// `kind = 'user'`, `m.kind='user'`, … — the WHERE-clause form.
	kindPredicate := regexp.MustCompile(`(?i)\bkind\s*(=|<>|!=|\bIN\b)`)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	checked, exemptSeen := 0, map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				query, err := strconv.Unquote(lit.Value)
				if err != nil || !strings.Contains(query, "tenant_members") {
					return true
				}
				if !strings.Contains(strings.ToUpper(query), "SELECT") {
					return true // writes carry the kind explicitly or take the default
				}
				checked++
				// A PREDICATE, not merely the column in the select list:
				// ListTenantMembers reads `kind` into the row and still has to
				// filter, and an early version of this sweep accepted the read
				// alone — which let the filter be deleted silently.
				if kindPredicate.MatchString(query) {
					return true // a deliberate decision, stated in the SQL
				}
				flat := strings.Join(strings.Fields(query), " ")
				for fragment := range kindExemptQueries {
					if strings.Contains(flat, strings.Join(strings.Fields(fragment), " ")) {
						exemptSeen[fragment] = true
						return true
					}
				}
				t.Errorf("%s: a tenant_members SELECT that neither mentions kind nor is listed as "+
					"kind-exempt.\nDecide (w5/m103, docs/ADR024-members.md): a read answering \"who are the "+
					"people here\" filters kind = 'user'; a read answering \"may this subject act here\" must "+
					"NOT filter, and is listed in kindExemptQueries with its reason.\nQuery: %s",
					fset.Position(lit.Pos()), strings.Join(strings.Fields(query), " "))
				return true
			})
		}
	}
	for fragment := range kindExemptQueries {
		if !exemptSeen[fragment] {
			t.Errorf("kind-exempt entry matches no query any more — remove it or fix the fragment: %q", fragment)
		}
	}
	if checked < 10 {
		t.Fatalf("swept only %d tenant_members SELECTs — the AST walk stopped seeing them", checked)
	}
}
