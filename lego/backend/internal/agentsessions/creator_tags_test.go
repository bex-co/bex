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

package agentsessions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// --- w7/m296: who started a session, the creator filter, replaceable tags ---

// twoMemberFixture is a workspace with two members, alice and carol, each
// owning one session, so "mine" has something to exclude.
func twoMemberFixture(t *testing.T) (*Service, *fakeStore, string, string) {
	t.Helper()
	svc, st, fga, _ := fixture()
	fga.members["carol"] = fga.members["alice"]
	alice, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	st.now = st.now.Add(1)
	carol, err := svc.Create(caller("carol"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, alice.ID, carol.ID
}

func listIDs(t *testing.T, svc *Service, ctx context.Context, req ListRequest) []string {
	t.Helper()
	views, err := svc.List(ctx, req)
	if err != nil {
		t.Fatalf("list %+v: %v", req, err)
	}
	out := make([]string, 0, len(views))
	for _, v := range views {
		out = append(out, v.ID)
	}
	return out
}

// The creator recorded at create (ADR102 D9) is the read model's createdBy,
// and a row without one omits it rather than inventing a value.
func TestViewCarriesTheCreator(t *testing.T) {
	svc, st, aliceID, _ := twoMemberFixture(t)
	got, err := svc.Get(caller("carol"), aliceID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreatedBy != "alice" {
		t.Fatalf("createdBy = %q, want the subject that created it", got.CreatedBy)
	}
	row := st.rows[aliceID]
	row.CreatedBy = ""
	st.rows[aliceID] = row
	if got, _ := svc.Get(caller("alice"), aliceID); got.CreatedBy != "" {
		t.Fatalf("a row with no recorded creator must not claim one: %q", got.CreatedBy)
	}
}

// mine keeps the caller's own sessions (Devin's sidebar default); createdBy
// keeps any listed subject's (Devin v3 user_ids); both together union.
func TestListFiltersByCreator(t *testing.T) {
	svc, _, aliceID, carolID := twoMemberFixture(t)
	if got := listIDs(t, svc, caller("alice"), ListRequest{Mine: true}); !reflect.DeepEqual(got, []string{aliceID}) {
		t.Fatalf("alice mine = %v, want only her session %s", got, aliceID)
	}
	if got := listIDs(t, svc, caller("alice"), ListRequest{CreatedBy: []string{"carol"}}); !reflect.DeepEqual(got, []string{carolID}) {
		t.Fatalf("createdBy=carol = %v, want %s", got, carolID)
	}
	got := listIDs(t, svc, caller("alice"), ListRequest{Mine: true, CreatedBy: []string{" carol ", "carol"}})
	if len(got) != 2 {
		t.Fatalf("mine+createdBy=carol = %v, want both sessions", got)
	}
	if got := listIDs(t, svc, caller("alice"), ListRequest{CreatedBy: []string{"nobody"}}); len(got) != 0 {
		t.Fatalf("an unknown creator must match nothing, got %v", got)
	}
}

func TestListCreatorFilterRefusals(t *testing.T) {
	svc, _, _, _ := twoMemberFixture(t)
	many := make([]string, maxCreatedByFilter+1)
	for i := range many {
		many[i] = "subject-" + strings.Repeat("x", i+1)
	}
	if _, err := svc.List(caller("alice"), ListRequest{CreatedBy: many}); !isCode(err, "AGENT_SESSION_FILTER_INVALID") {
		t.Fatalf("21 createdBy values = %v, want AGENT_SESSION_FILTER_INVALID", err)
	}
	// An identity with no subject has no "mine": refused, never widened to all.
	noSubject := core.WithIdentity(context.Background(), core.Identity{Method: "session"})
	if _, err := svc.List(noSubject, ListRequest{Mine: true}); err == nil {
		t.Fatal("mine with no caller subject must be refused")
	}
}

// Tags are replaced whole (Devin's PUT …/tags), [] clears them, the create
// rules apply, and the list's tag filter sees the new set.
func TestReplaceTags(t *testing.T) {
	svc, _, _, id := steerableFixture(t)
	view, err := svc.ReplaceTags(caller("alice"), id, []string{"bug", "ui", "bug"})
	if err != nil || !reflect.DeepEqual(view.Tags, []string{"bug", "ui"}) {
		t.Fatalf("replace = %v err=%v, want [bug ui] (duplicates collapsed)", view.Tags, err)
	}
	view, err = svc.ReplaceTags(caller("alice"), id, []string{"triage"})
	if err != nil || !reflect.DeepEqual(view.Tags, []string{"triage"}) {
		t.Fatalf("second replace = %v err=%v, want exactly [triage]", view.Tags, err)
	}
	if got := listIDs(t, svc, caller("alice"), ListRequest{Tag: "triage"}); !reflect.DeepEqual(got, []string{id}) {
		t.Fatalf("tag filter after replace = %v", got)
	}
	if got := listIDs(t, svc, caller("alice"), ListRequest{Tag: "bug"}); len(got) != 0 {
		t.Fatalf("a replaced tag must stop matching, got %v", got)
	}
	view, err = svc.ReplaceTags(caller("alice"), id, []string{})
	if err != nil || len(view.Tags) != 0 {
		t.Fatalf("clear = %v err=%v", view.Tags, err)
	}
	if _, err := svc.ReplaceTags(caller("alice"), id, []string{"Not Valid"}); !isCode(err, "AGENT_SESSION_TAGS_INVALID") {
		t.Fatalf("invalid tag = %v, want AGENT_SESSION_TAGS_INVALID", err)
	}
	if _, err := svc.ReplaceTags(caller("alice"), id, make([]string, maxTags+1)); !isCode(err, "AGENT_SESSION_TAGS_INVALID") {
		t.Fatalf("21 tags = %v, want AGENT_SESSION_TAGS_INVALID", err)
	}
}

// An archived session can be viewed but not modified (ADR065 D1, Devin): the
// tag edit is refused and the stored tags are untouched.
func TestReplaceTagsRefusedWhenArchived(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	if _, err := svc.ReplaceTags(caller("alice"), id, []string{"keep"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReplaceTags(caller("alice"), id, []string{"changed"}); !isCode(err, "AGENT_SESSION_ARCHIVED") {
		t.Fatalf("tag edit on archived = %v, want AGENT_SESSION_ARCHIVED", err)
	}
	if got := st.rows[id].Tags; !reflect.DeepEqual(got, []string{"keep"}) {
		t.Fatalf("archived tags changed to %v", got)
	}
}

// Another workspace's member cannot retag a session they cannot operate.
func TestReplaceTagsNeedsAccess(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	if _, err := svc.ReplaceTags(caller("bob"), id, []string{"x"}); err == nil {
		t.Fatal("bob (another workspace) retagged alice's session")
	}
	if len(st.rows[id].Tags) != 0 {
		t.Fatalf("refused edit still wrote tags: %v", st.rows[id].Tags)
	}
}

// REST: the body must name tags (a missing key is not a silent clear), and
// mine must parse as a boolean.
func TestRESTTagsAndMineValidation(t *testing.T) {
	svc, _, _, id := steerableFixture(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, target, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(caller("alice"))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	if rec := serve(http.MethodPut, "/v1/agent-sessions/"+id+"/tags", `{}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "AGENT_SESSION_TAGS_INVALID") {
		t.Fatalf("PUT tags without tags = %d %s", rec.Code, rec.Body)
	}
	if rec := serve(http.MethodPut, "/v1/agent-sessions/"+id+"/tags", `{"tags":["a"]}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"tags":["a"]`) {
		t.Fatalf("PUT tags = %d %s", rec.Code, rec.Body)
	}
	if rec := serve(http.MethodGet, "/v1/agent-sessions?mine=maybe", ""); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "AGENT_SESSION_FILTER_INVALID") {
		t.Fatalf("mine=maybe = %d %s", rec.Code, rec.Body)
	}
	rec := serve(http.MethodGet, "/v1/agent-sessions?mine=true", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"createdBy":"alice"`) {
		t.Fatalf("mine=true = %d %s", rec.Code, rec.Body)
	}
}

func TestNormalizeTagsSharedWithCreate(t *testing.T) {
	req := createInput()
	req.Tags = []string{"a", "a", "b"}
	if err := validateTitleAndTags(&req); err != nil || !reflect.DeepEqual(req.Tags, []string{"a", "b"}) {
		t.Fatalf("create tags = %v err=%v", req.Tags, err)
	}
	if _, err := normalizeTags([]string{""}); !isCode(err, "AGENT_SESSION_TAGS_INVALID") {
		t.Fatalf("empty tag = %v, want AGENT_SESSION_TAGS_INVALID", err)
	}
}

// MCP agrees with REST: null tags are refused rather than read as a clear,
// a list replaces the set, and mine keeps the caller's sessions.
func TestMCPSetTagsAndMine(t *testing.T) {
	svc, _, aliceID, carolID := twoMemberFixture(t)
	// The server acts as the identity its connection was opened with.
	alice := newMCPClient(t, caller("alice"), svc)
	carol := newMCPClient(t, caller("carol"), svc)

	res := callTool(t, alice, "alice", "set_agent_session_tags", map[string]any{"id": aliceID, "tags": nil})
	if !res.IsError || !strings.Contains(toolText(res, 0), "tags is required") {
		t.Fatalf("null tags = %+v %q, want a refusal", res.IsError, toolText(res, 0))
	}
	res = callTool(t, alice, "alice", "set_agent_session_tags", map[string]any{"id": aliceID, "tags": []string{"ci"}})
	if res.IsError || !strings.Contains(toolText(res, 0), `"tags":["ci"]`) || !strings.Contains(toolText(res, 0), `"createdBy":"alice"`) {
		t.Fatalf("set tags = %q", toolText(res, 0))
	}
	res = callTool(t, carol, "carol", "list_agent_sessions", map[string]any{"ownerId": "tea-a", "mine": true})
	if res.IsError || !strings.Contains(toolText(res, 0), carolID) || strings.Contains(toolText(res, 0), aliceID) {
		t.Fatalf("carol mine = %q", toolText(res, 0))
	}
}
