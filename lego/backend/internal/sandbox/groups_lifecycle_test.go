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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w22/m311 t010: the group behaviors the suites in groups_test.go and
// groups_surfaces_test.go did not pin. Same real-Postgres fixture.

// addWorkspace adds a second real workspace whose only member is subject, with
// its own OpenSandbox tenant key, and returns its id.
func (f *groupFixture) addWorkspace(t *testing.T, subject string) string {
	t.Helper()
	stamp := time.Now().UnixNano()
	tenant, err := f.st.CreateWorkspace(context.Background(), fmt.Sprintf("sbgroups-b-%d", stamp), store.PlanHobby, fmt.Sprintf("sbgroups-owner-b-%d", stamp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.st.DeleteTenant(context.Background(), tenant.ID) })
	f.svc.Base.Workspace = fakeWorkspace{"id-a": f.ws, subject: tenant.ID}
	f.meter.keys = append(f.meter.keys, store.SandboxTenantKey{WorkspaceID: tenant.ID, APIKey: "key-b"})
	return tenant.ID
}

// A group is reachable only inside its own workspace. A member of another
// workspace gets the same not-found for it as for an id that does not exist, on
// every verb that takes a group, and changes and starts nothing.
func TestGroupsAreIsolatedBetweenWorkspaces(t *testing.T) {
	f := newGroupFixture(t)
	wsB := f.addWorkspace(t, "id-b")
	secret := f.createGroup(t, "secret-roadmap")
	unknown := "sbg-" + strings.Repeat("a", 20)

	for name, call := range map[string]func(id string) (int, string){
		"retrieve": func(id string) (int, string) {
			return f.doAs(t, "id-b", http.MethodGet, "/v1/sandbox-groups/"+id, "")
		},
		"update": func(id string) (int, string) {
			return f.doAs(t, "id-b", http.MethodPatch, "/v1/sandbox-groups/"+id, `{"concurrencyLimit":1}`)
		},
		"create a sandbox in it": func(id string) (int, string) {
			return f.doAs(t, "id-b", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q}`, wsB, id))
		},
		"list its sandboxes": func(id string) (int, string) {
			return f.doAs(t, "id-b", http.MethodGet, "/v1/sandboxes?ownerId="+wsB+"&sandboxGroupId="+id, "")
		},
	} {
		foreignCode, foreignBody := call(secret.ID)
		unknownCode, unknownBody := call(unknown)
		if foreignCode != http.StatusNotFound || errCode(foreignBody) != CodeSandboxGroupNotFound {
			t.Errorf("%s of another workspace's group = %d %s, want 404 %s", name, foreignCode, foreignBody, CodeSandboxGroupNotFound)
		}
		if unknownCode != foreignCode || errCode(unknownBody) != errCode(foreignBody) {
			t.Errorf("%s: another workspace's group answers %d %s but an unknown one %d %s: the answers must not tell them apart", name, foreignCode, foreignBody, unknownCode, unknownBody)
		}
		if strings.Contains(foreignBody, secret.Name) || strings.Contains(foreignBody, f.ws) {
			t.Errorf("%s leaks the other workspace's group to a stranger: %s", name, foreignBody)
		}
	}
	if n := f.os.count("Running"); n != 0 {
		t.Errorf("%d sandboxes started by creates that named another workspace's group", n)
	}
	if got, err := f.st.SandboxGroup(context.Background(), f.ws, secret.ID); err != nil || got.ConcurrencyLimit != RenderMaxGroupConcurrency {
		t.Errorf("a stranger's update changed the group to %+v (%v)", got, err)
	}

	// The owner still reaches it, and the stranger reaches only their own.
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+secret.ID, ""); code != http.StatusOK {
		t.Fatalf("the owner's retrieve = %d %s", code, body)
	}
	code, body := f.doAs(t, "id-b", http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"secret-roadmap"}`, wsB))
	var mine Group
	if err := json.Unmarshal([]byte(body), &mine); code != http.StatusCreated || err != nil || !mine.IsDefault || mine.OwnerID != wsB {
		t.Fatalf("the same name in another workspace = %d %s, want that workspace's own default group", code, body)
	}
	if code, body := f.doAs(t, "id-b", http.MethodGet, "/v1/sandbox-groups/"+mine.ID, ""); code != http.StatusOK {
		t.Errorf("the stranger's own group = %d %s", code, body)
	}
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+mine.ID, ""); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound {
		t.Errorf("the first workspace reading the second's group = %d %s, want 404 %s", code, body, CodeSandboxGroupNotFound)
	}
	// Naming the workspace that holds it does not help a non-member.
	if code, body := f.doAs(t, "id-b", http.MethodGet, "/v1/sandboxes?ownerId="+f.ws+"&sandboxGroupId="+secret.ID, ""); code != http.StatusForbidden {
		t.Errorf("a non-member listing the owner's group by naming its workspace = %d %s, want 403", code, body)
	}
}

// Render lists 20 groups a call and at most 100, newest first, with a cursor. A
// workspace can reach more than 20 only with the group cap off, so this runs
// uncapped.
func TestGroupListPagesTwentyAtATime(t *testing.T) {
	f := newGroupFixture(t)
	f.svc.MaxGroups = 0
	var ids []string // oldest first
	for i := range 25 {
		ids = append(ids, f.createGroup(t, fmt.Sprintf("g%02d", i)).ID)
	}
	page := func(query string) []GroupEnvelope {
		t.Helper()
		code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups?ownerId="+f.ws+query, "")
		var out []GroupEnvelope
		if err := json.Unmarshal([]byte(body), &out); code != http.StatusOK || err != nil {
			t.Fatalf("list %q = %d %s", query, code, body)
		}
		return out
	}
	first := page("")
	if len(first) != 20 || first[0].SandboxGroup.ID != ids[24] || first[19].SandboxGroup.ID != ids[5] {
		t.Fatalf("default page = %d groups, want the 20 newest", len(first))
	}
	rest := page("&cursor=" + first[19].Cursor)
	if len(rest) != 5 || rest[0].SandboxGroup.ID != ids[4] || rest[4].SandboxGroup.ID != ids[0] {
		t.Fatalf("page after the cursor = %d groups, want the 5 oldest", len(rest))
	}
	if all := page("&limit=100"); len(all) != 25 {
		t.Fatalf("limit 100 = %d groups, want all 25", len(all))
	}
	if one := page("&limit=1"); len(one) != 1 || one[0].SandboxGroup.ID != ids[24] {
		t.Fatalf("limit 1 = %+v", one)
	}

	// GraphQL and MCP reach the same Service verb, so they page the same way.
	ctx := callerCtx()
	if got, err := f.svc.ListGroups(ctx, "", 0); err != nil || len(got) != 20 {
		t.Fatalf("ListGroups with no limit = %d groups (%v), want Render's default of 20", len(got), err)
	}
	if got, err := f.svc.ListGroups(ctx, ids[5], 500); err != nil || len(got) != 5 {
		t.Fatalf("ListGroups after a cursor with an oversized limit = %d groups (%v), want the 5 that remain", len(got), err)
	}
}

// A workspace whose sandboxes predate groups gets exactly one default group the
// first time they are read, and every one of them lists in it.
func TestListingLegacySandboxesCreatesTheOneDefaultGroup(t *testing.T) {
	f := newGroupFixture(t)
	f.svc.MaxGroups = 1
	f.addBox("os-legacy-1", "Running")
	f.addBox("os-legacy-2", "Paused")

	listGroups := func() []string {
		t.Helper()
		var out []string
		code, body := f.do(t, http.MethodGet, "/v1/sandboxes?ownerId="+f.ws+"&status=running,suspended", "")
		var items []SandboxEnvelope
		if err := json.Unmarshal([]byte(body), &items); code != http.StatusOK || err != nil || len(items) != 2 {
			t.Fatalf("list = %d %s (%v)", code, body, err)
		}
		for _, item := range items {
			out = append(out, item.Sandbox.SandboxGroupID)
		}
		return out
	}
	got := listGroups()
	groups, err := f.st.SandboxGroups(context.Background(), f.ws)
	if err != nil || len(groups) != 1 || !groups[0].IsDefault || groups[0].Name != store.DefaultSandboxGroupName {
		t.Fatalf("groups after the first read = %+v (%v), want the one default, named %s, made without the cap refusing it", groups, err, store.DefaultSandboxGroupName)
	}
	if got[0] != groups[0].ID || got[1] != groups[0].ID {
		t.Fatalf("legacy sandboxes list in %v, want the default %s", got, groups[0].ID)
	}
	if again := listGroups(); again[0] != groups[0].ID || again[1] != groups[0].ID {
		t.Fatalf("a second read lists them in %v", again)
	}
	if groups, _ = f.st.SandboxGroups(context.Background(), f.ws); len(groups) != 1 {
		t.Fatalf("%d groups after two reads, want 1", len(groups))
	}
	// The default counts toward the cap like any group.
	if code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"second"}`, f.ws)); code != http.StatusBadRequest || errCode(body) != CodeSandboxGroupLimitReached {
		t.Fatalf("a group past a cap of 1 = %d %s, want 400 %s", code, body, CodeSandboxGroupLimitReached)
	}
}

// A sandbox that predates groups counts in the default group's limit, and the
// resume that brings it back binds it there.
func TestLegacySandboxesCountInTheDefaultGroupAndResumeBindsThem(t *testing.T) {
	f := newGroupFixture(t)
	def := f.createGroup(t, "def")
	other := f.createGroup(t, "other")
	f.setLimit(t, def, 1)
	f.addBox("os-legacy", "Running")

	code, body := f.create(t, def.ID)
	if code != http.StatusTooManyRequests || errCode(body) != CodeSandboxGroupConcurrencyLimit || !strings.Contains(body, `"active":1`) {
		t.Fatalf("create in the default group beside a running legacy sandbox = %d %s, want 429 with 1 active", code, body)
	}
	f.mustCreate(t, other.ID) // another group is not affected

	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/os-legacy/pause?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("suspend = %d %s", code, body)
	}
	f.os.set("os-legacy", func(b *fakeBox) { b.state = "Paused" })
	taker := f.mustCreate(t, def.ID) // a suspended sandbox does not count

	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/os-legacy/resume?ownerId="+f.ws, ""); code != http.StatusTooManyRequests || errCode(body) != CodeSandboxGroupConcurrencyLimit {
		t.Fatalf("resume of the legacy sandbox into a full default group = %d %s, want 429", code, body)
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+taker.ID+"/terminate?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("terminate = %d %s", code, body)
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/os-legacy/resume?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("resume with room = %d %s", code, body)
	}
	members, err := f.st.SandboxGroupMembers(context.Background(), f.ws)
	if err != nil || members["os-legacy"].GroupID != def.ID {
		t.Fatalf("members = %+v (%v), want the resumed legacy sandbox bound to the default group %s", members, err, def.ID)
	}
}

// An admitted resume that then fails gives its slot back: the sandbox is still
// suspended, so it must not keep holding one.
func TestFailedResumeGivesItsSlotBack(t *testing.T) {
	for _, route := range []struct{ name, path, body string }{
		{"resume", "/resume", ""},
		{"exec", "/exec", `{"command":"true"}`},
	} {
		t.Run(route.name, func(t *testing.T) {
			f := newGroupFixture(t)
			g := f.createGroup(t, "flaky")
			f.setLimit(t, g, 1)
			asleep := f.mustCreate(t, g.ID)
			f.suspend(t, asleep)

			f.os.mu.Lock()
			f.os.resumeStatus = http.StatusInternalServerError
			f.os.mu.Unlock()
			if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+asleep.ID+route.path+"?ownerId="+f.ws, route.body); code < 500 {
				t.Fatalf("%s while the runtime fails = %d %s, want a server error", route.name, code, body)
			}
			if got := f.os.state(f.os.idOf(asleep.ID)); got != "Paused" {
				t.Fatalf("the sandbox is %s after a failed resume, want Paused", got)
			}
			f.os.mu.Lock()
			f.os.resumeStatus = 0
			f.os.mu.Unlock()
			if code, body := f.create(t, g.ID); code != http.StatusCreated {
				t.Fatalf("create after a failed %s = %d %s, want the slot free: a suspended sandbox holds none", route.name, code, body)
			}
		})
	}
}

// The idle sweeper suspends a sandbox without anyone calling the API, and that
// frees its slot at once, even inside the lease its create left.
func TestIdleSuspensionFreesTheSlot(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "idle")
	f.setLimit(t, g, 1)
	sb := f.mustCreate(t, g.ID)
	id := f.os.idOf(sb.ID)
	if code, _ := f.create(t, g.ID); code != http.StatusTooManyRequests {
		t.Fatal("fixture: the group is not full")
	}
	// Past its idle window, with the create's lease still running. Its age is
	// bex's own creation stamp (w32/020), which every read reports.
	f.os.set(id, func(b *fakeBox) {
		b.metadata[metadataIdleTimeout] = "60"
		b.metadata[metadataCreatedAt] = stampCreatedAt(f.now.Add(-time.Hour))
	})
	if n := f.sweeper.Sweep(context.Background()); n != 1 {
		t.Fatalf("the sweeper suspended %d sandboxes, want 1", n)
	}
	f.os.set(id, func(b *fakeBox) { b.state = "Paused" })
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Fatalf("create after the sweeper suspended the only sandbox = %d %s, want its slot free", code, body)
	}
}

// The sweeper also drops the binding of a sandbox that is gone from OpenSandbox,
// once it is old and holds no lease, and keeps the bindings of live ones.
func TestSweeperPrunesBindingsOfGoneSandboxes(t *testing.T) {
	ctx := context.Background()
	f := newGroupFixture(t)
	g := f.createGroup(t, "prune")
	live := f.mustCreate(t, g.ID)
	bind := func(id string, lease time.Time) {
		t.Helper()
		if err := f.st.WithSandboxAdmission(ctx, f.ws, func(adm store.SandboxAdmission) error { return adm.Bind(ctx, id, g.ID, lease) }); err != nil {
			t.Fatal(err)
		}
	}
	bind("sbx-gone-old", time.Now().Add(-time.Hour))
	bind("sbx-gone-leased", time.Now().Add(time.Hour))
	bind("sbx-gone-young", time.Now().Add(-time.Hour))
	if _, err := f.st.Pool.Exec(ctx, `UPDATE sandbox_group_members SET created_at = now() - interval '1 hour' WHERE workspace_id = $1 AND sandbox_id <> 'sbx-gone-young'`, f.ws); err != nil {
		t.Fatal(err)
	}

	f.sweeper.Sweep(ctx)
	members, err := f.st.SandboxGroupMembers(ctx, f.ws)
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := members["sbx-gone-old"]; kept {
		t.Error("the binding of a sandbox that is gone, old and unleased survived the sweep")
	}
	for _, want := range []string{live.ID, "sbx-gone-leased", "sbx-gone-young"} {
		if _, kept := members[want]; !kept {
			t.Errorf("the sweep removed the binding of %s", want)
		}
	}
}

// A hung OpenSandbox list cannot hold a workspace's admission lock for ever: the
// read is bounded, the create fails, and the next admission gets the lock at once.
func TestHungSandboxListCannotHoldTheAdmissionLock(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "hung")
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(hang.Close)
	t.Cleanup(func() { close(release) }) // runs first, so a failing run does not hang in hang.Close
	f.svc.Client = NewClient(hang.URL)
	defer func(old time.Duration) { admissionListTimeout = old }(admissionListTimeout)
	admissionListTimeout = 150 * time.Millisecond

	type answer struct {
		code int
		body string
	}
	created := make(chan answer, 1)
	go func() {
		code, body := f.create(t, g.ID)
		created <- answer{code, body}
	}()
	select {
	case got := <-created:
		if got.code < 500 {
			t.Fatalf("create against a hung OpenSandbox = %d %s, want a server error", got.code, got.body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a create against a hung OpenSandbox was still waiting after 5 s: the list read under the admission lock is not bounded")
	}
	done := make(chan error, 1)
	go func() {
		done <- f.st.WithSandboxAdmission(context.Background(), f.ws, func(store.SandboxAdmission) error { return nil })
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the admission lock was still held after the bounded list gave up")
	}
	if members, err := f.st.SandboxGroupMembers(context.Background(), f.ws); err != nil || len(members) != 0 {
		t.Fatalf("a create that never counted left bindings %v (%v)", members, err)
	}
}

// The platform's ceiling applies to a limit already stored: lowering it lowers
// what every group reports and is held to, with no rewrite.
func TestLoweredCeilingBindsStoredLimits(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "big")
	f.mustCreate(t, g.ID)
	f.mustCreate(t, g.ID)
	if g.ConcurrencyLimit != RenderMaxGroupConcurrency {
		t.Fatalf("fixture: the group stores %d", g.ConcurrencyLimit)
	}

	f.svc.MaxGroupConcurrency = 2 // the operator lowers BEX_SANDBOX_MAX_GROUP_CONCURRENCY
	code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+g.ID, "")
	var got Group
	if err := json.Unmarshal([]byte(body), &got); code != http.StatusOK || err != nil || got.ConcurrencyLimit != 2 {
		t.Fatalf("retrieve under a ceiling of 2 = %d %s, want an effective limit of 2", code, body)
	}
	code, body = f.create(t, g.ID)
	if code != http.StatusTooManyRequests || !strings.Contains(body, `"concurrencyLimit":2`) {
		t.Fatalf("third create under a ceiling of 2 = %d %s, want 429 naming the effective limit", code, body)
	}
	if updated := f.setLimit(t, g, 500); updated.ConcurrencyLimit != 2 {
		t.Fatalf("a PATCH above the ceiling answered %d, want the capped 2", updated.ConcurrencyLimit)
	}
}

// The 429 is bex's error envelope with Render's status-derived id, names the
// group, its limit and its active count, and carries no Retry-After: finishing
// or suspending other work clears it, not waiting.
func TestConcurrencyRefusalEnvelope(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "full")
	f.setLimit(t, g, 1)
	f.mustCreate(t, g.ID)

	rec := f.record("id-a", http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q}`, f.ws, g.ID))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("create at the limit = %d %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("a concurrency refusal carries Retry-After %q, but waiting does not clear it", got)
	}
	var envelope struct {
		Error, Message, ID, Code string
		Params                   struct {
			SandboxGroupID   string `json:"sandboxGroupId"`
			ConcurrencyLimit int    `json:"concurrencyLimit"`
			Active           int    `json:"active"`
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ID != "too_many_requests" || envelope.Code != CodeSandboxGroupConcurrencyLimit || envelope.Error == "" || envelope.Message != envelope.Error ||
		envelope.Params.SandboxGroupID != g.ID || envelope.Params.ConcurrencyLimit != 1 || envelope.Params.Active != 1 {
		t.Fatalf("429 envelope = %s", rec.Body)
	}
}
