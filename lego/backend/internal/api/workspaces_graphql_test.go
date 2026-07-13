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
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// changeWorkspacePlan is GraphQL-only by design (workspaces/graphql.go: Render's
// REST owners surface has no plan mutation and its MCP has no workspace
// mutations — parity by absence), so GraphQL is the ONE surface a downgrade
// refusal has to be right on. These tests pin the wire shape the dashboard reads:
// the refusal arrives as a GraphQL error whose `message` carries the service's
// reason verbatim, which is what use-change-workspace-plan.ts renders inline in
// the change-plan dialog (graphQLErrorMessage → the Alert). A refusal that
// surfaced only as a null data field, or with the reason swallowed, would leave
// the dialog saying nothing actionable — which is the failure w6/m15/t003 exists
// to prevent.
func changePlanErr(t *testing.T, st *fakeWSStore, tenantID, plan string) string {
	t.Helper()
	base := serverBase(t, st)
	h, _ := serverWith(t, base, Deps{WorkspaceStore: st})

	q := fmt.Sprintf(`mutation { changeWorkspacePlan(id: %q, plan: %q) { id plan } }`, tenantID, plan)
	body, _ := json.Marshal(map[string]string{"query": q})
	w := do(t, h, "POST", "/graphql", testToken, string(body))
	if w.Code != 200 {
		t.Fatalf("graphql http %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if len(out.Errors) == 0 {
		t.Fatalf("want the downgrade refused, got a clean response: %s", w.Body.String())
	}
	return out.Errors[0].Message
}

func TestChangeWorkspacePlanGraphQL_RefusalNamesThePendingInvite(t *testing.T) {
	st := newFakeWSStore()
	w := mustCreate(t, st, "acme", "pro", "client-1") // creator is its 1 admin member
	st.addInvite(w.ID, "carol@example.com", "developer")

	msg := changePlanErr(t, st, w.ID, "hobby")

	// The dashboard renders this string as-is, so the email has to be IN it —
	// "member limit reached" alone would not tell the admin what to revoke.
	if !strings.Contains(msg, "carol@example.com") {
		t.Fatalf("refusal must name the pending invite, got %q", msg)
	}
	if !strings.Contains(msg, "hobby") {
		t.Fatalf("refusal must name the target plan, got %q", msg)
	}
}

// The role guard's refusal travels the same way (a pending invite holding a role
// the target plan doesn't offer), so the dialog's inline Alert is actionable for
// both downgrade guards, not just the seat one.
func TestChangeWorkspacePlanGraphQL_RefusalNamesTheOutOfPlanInvitedRole(t *testing.T) {
	st := newFakeWSStore()
	w := mustCreate(t, st, "acme", "scale", "client-1")
	st.addInvite(w.ID, "vic@example.com", "viewer") // viewer exists on scale, not on pro

	msg := changePlanErr(t, st, w.ID, "pro")

	if !strings.Contains(msg, "vic@example.com") || !strings.Contains(msg, "viewer") {
		t.Fatalf("refusal must name the invite and its role, got %q", msg)
	}
}
