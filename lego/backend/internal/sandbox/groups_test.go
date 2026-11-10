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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/routers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The sandbox group tests run against the real control-plane Postgres, because
// the properties they prove (one default group per workspace, the 429 that
// never overshoots under parallel creates) are the database's: they skip
// locally without BEX_TEST_DB_URI and fail in CI, like the store's own.

func openGroupsPG(t *testing.T) *store.PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return store.NewPGStore(pool)
}

// groupFixture is the idle fixture's fake OpenSandbox and exec gateway with a
// real workspace and the real group store behind the Service.
type groupFixture struct {
	*idleFixture
	st      *store.PGStore
	ws      string
	mux     *http.ServeMux
	handler http.Handler
}

func newGroupFixture(t *testing.T) *groupFixture {
	t.Helper()
	st := openGroupsPG(t)
	stamp := time.Now().UnixNano()
	tenant, err := st.CreateWorkspace(context.Background(), fmt.Sprintf("sbgroups-%d", stamp), store.PlanHobby, fmt.Sprintf("sbgroups-owner-%d", stamp))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })

	f := newIdleFixture(t)
	f.svc.Base.Workspace = fakeWorkspace{"id-a": tenant.ID}
	f.meter.keys = []store.SandboxTenantKey{{WorkspaceID: tenant.ID, APIKey: "key-a"}}
	f.svc.Templates = map[string]Template{"node": {Image: "node:20", Entrypoint: []string{"sh"}, CPU: "500m", Memory: "512Mi"}}
	f.svc.DefaultTemplate = "node"
	f.svc.Region = "oregon"
	f.svc.Groups = st
	f.svc.MaxGroups = 3
	f.svc.MaxGroupConcurrency = RenderMaxGroupConcurrency

	mux := http.NewServeMux()
	f.svc.RegisterREST(mux)
	// Every response to a group operation is checked against the pinned Render
	// schema. The sandbox responses are not: their 20-character ids differ from
	// Render's 25 on purpose (the request gate widens the pattern, ids.Sandbox),
	// which is a recorded divergence this feature does not touch.
	contract := rendertest.Load(t).MuxMatcher(mux)
	groupsOnly := func(r *http.Request) *routers.Route {
		if !strings.HasPrefix(r.URL.Path, "/v1/sandbox-groups") {
			return nil
		}
		return contract(r)
	}
	return &groupFixture{idleFixture: f, st: st, ws: tenant.ID, mux: mux, handler: rendertest.Wrap(t, mux, groupsOnly)}
}

// do calls the REST surface as the workspace's member.
func (f *groupFixture) do(t *testing.T, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(callerCtx())
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// addBox puts a sandbox of the workspace straight into OpenSandbox with no
// group binding, as a sandbox created before groups existed.
func (f *groupFixture) addBox(id, state string) {
	f.os.add(id, state, f.now.Add(-time.Minute), "0", map[string]string{metadataWorkspace: f.ws})
}

func (f *groupFixture) createGroup(t *testing.T, name string) Group {
	t.Helper()
	code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":%q}`, f.ws, name))
	var g Group
	if err := json.Unmarshal([]byte(body), &g); code != http.StatusCreated || err != nil {
		t.Fatalf("create group %q = %d %s", name, code, body)
	}
	return g
}

func (f *groupFixture) setLimit(t *testing.T, g Group, limit int) Group {
	t.Helper()
	code, body := f.do(t, http.MethodPatch, "/v1/sandbox-groups/"+g.ID, fmt.Sprintf(`{"concurrencyLimit":%d}`, limit))
	var out Group
	if err := json.Unmarshal([]byte(body), &out); code != http.StatusOK || err != nil {
		t.Fatalf("set limit %d = %d %s", limit, code, body)
	}
	return out
}

// create starts a sandbox in a group ("" = the default) and returns the status
// and body.
func (f *groupFixture) create(t *testing.T, groupID string) (int, string) {
	t.Helper()
	body := fmt.Sprintf(`{"ownerId":%q}`, f.ws)
	if groupID != "" {
		body = fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q}`, f.ws, groupID)
	}
	return f.do(t, http.MethodPost, "/v1/sandboxes", body)
}

func (f *groupFixture) mustCreate(t *testing.T, groupID string) Sandbox {
	t.Helper()
	code, body := f.create(t, groupID)
	var sb Sandbox
	if err := json.Unmarshal([]byte(body), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("create in %q = %d %s", groupID, code, body)
	}
	return sb
}

// suspend pauses a sandbox through the API, so its slot lease is cleared as in
// production, and lets the fake OpenSandbox settle it as Paused.
func (f *groupFixture) suspend(t *testing.T, sb Sandbox) {
	t.Helper()
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/pause?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("suspend %s = %d %s", sb.ID, code, body)
	}
	f.os.set(f.os.idOf(sb.ID), func(b *fakeBox) { b.state = "Paused" })
}

func errCode(body string) string {
	var out struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	return out.Code
}

// w22/m311: Render's group routes answer in Render's shape, the first group is
// the default, names are unique, the cap and the limit clamp hold.
func TestSandboxGroupRoutes(t *testing.T) {
	f := newGroupFixture(t)

	code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"CI agents"}`, f.ws))
	if code != http.StatusCreated {
		t.Fatalf("create = %d %s", code, body)
	}
	var raw map[string]any
	_ = json.Unmarshal([]byte(body), &raw)
	// Every field of the pinned schema has a value; environmentId is an explicit null.
	for _, key := range []string{"id", "ownerId", "name", "region", "isDefault", "environmentId", "concurrencyLimit", "createdAt", "updatedAt"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("create response lacks %q: %s", key, body)
		}
	}
	if raw["environmentId"] != nil || raw["ownerId"] != f.ws || raw["region"] != "oregon" || raw["isDefault"] != true || raw["concurrencyLimit"] != float64(1000) {
		t.Fatalf("first group = %s, want oregon, the workspace's default, limit 1000, no environment", body)
	}
	first := raw["id"].(string)

	second := f.createGroup(t, "Nightly")
	if second.IsDefault {
		t.Error("the second group became the default")
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"CI agents"}`, f.ws)); code != http.StatusConflict || errCode(body) != CodeSandboxGroupNameTaken {
		t.Errorf("duplicate name = %d %s, want 409 %s", code, body, CodeSandboxGroupNameTaken)
	}
	third := f.createGroup(t, "Third")
	// At the cap, the next create is Render's own 400 code.
	if code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"Fourth"}`, f.ws)); code != http.StatusBadRequest || errCode(body) != CodeSandboxGroupLimitReached {
		t.Errorf("create past the cap = %d %s, want 400 %s", code, body, CodeSandboxGroupLimitReached)
	}
	for name, tc := range map[string]string{
		"region":      `{"ownerId":"` + f.ws + `","name":"x","region":"frankfurt"}`,
		"empty name":  `{"ownerId":"` + f.ws + `","name":""}`,
		"padded name": `{"ownerId":"` + f.ws + `","name":" x"}`,
		"long name":   `{"ownerId":"` + f.ws + `","name":"` + strings.Repeat("a", 101) + `"}`,
		"no name":     `{"ownerId":"` + f.ws + `"}`,
	} {
		if code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", tc); code != http.StatusBadRequest {
			t.Errorf("%s = %d %s, want 400", name, code, body)
		}
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandbox-groups", fmt.Sprintf(`{"ownerId":%q,"name":"x","region":"oregon"}`, f.ws)); code != http.StatusBadRequest || errCode(body) != CodeSandboxGroupLimitReached {
		t.Errorf("explicit oregon at the cap = %d %s, want the cap, not a region error", code, body)
	}

	// The list is newest first, pages by cursor, and each item carries its cursor.
	list := func(query string) []GroupEnvelope {
		t.Helper()
		code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups?ownerId="+f.ws+query, "")
		var out []GroupEnvelope
		if err := json.Unmarshal([]byte(body), &out); code != http.StatusOK || err != nil {
			t.Fatalf("list %s = %d %s", query, code, body)
		}
		return out
	}
	all := list("")
	if len(all) != 3 || all[0].SandboxGroup.ID != third.ID || all[2].SandboxGroup.ID != first || all[0].Cursor != third.ID {
		t.Fatalf("list = %+v, want Third, Nightly, CI agents with cursors", all)
	}
	if page := list("&limit=1"); len(page) != 1 || page[0].SandboxGroup.ID != third.ID {
		t.Fatalf("first page = %+v", page)
	}
	if page := list("&limit=2&cursor=" + third.ID); len(page) != 2 || page[0].SandboxGroup.ID != second.ID || page[1].SandboxGroup.ID != first {
		t.Fatalf("second page = %+v", page)
	}
	if page := list("&cursor=" + first); len(page) != 0 {
		t.Fatalf("page after the last = %+v, want empty", page)
	}

	// Retrieve, and the not-found / malformed answers.
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+second.ID, ""); code != http.StatusOK || !strings.Contains(body, `"name":"Nightly"`) {
		t.Fatalf("retrieve = %d %s", code, body)
	}
	missing := "sbg-" + strings.Repeat("a", 20)
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+missing, ""); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound {
		t.Errorf("unknown group = %d %s, want 404 %s", code, body, CodeSandboxGroupNotFound)
	}
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/not-an-id", ""); code != http.StatusBadRequest || errCode(body) != CodeInvalidSandboxGroupID {
		t.Errorf("malformed id = %d %s, want 400 %s", code, body, CodeInvalidSandboxGroupID)
	}

	// PATCH answers the effective limit: a value above the ceiling is capped, a
	// value below 1 or an absent one is refused.
	if g := f.setLimit(t, second, 7); g.ConcurrencyLimit != 7 || !g.UpdatedAt.After(second.UpdatedAt) {
		t.Errorf("limit 7 = %+v", g)
	}
	if g := f.setLimit(t, second, 5000); g.ConcurrencyLimit != 1000 {
		t.Errorf("limit 5000 = %d, want it capped to 1000, not refused", g.ConcurrencyLimit)
	}
	for name, tc := range map[string]string{"zero": `{"concurrencyLimit":0}`, "negative": `{"concurrencyLimit":-3}`, "absent": `{}`} {
		if code, body := f.do(t, http.MethodPatch, "/v1/sandbox-groups/"+second.ID, tc); code != http.StatusBadRequest || errCode(body) != CodeSandboxGroupLimitInvalid {
			t.Errorf("limit %s = %d %s, want 400 %s", name, code, body, CodeSandboxGroupLimitInvalid)
		}
	}
	if code, body := f.do(t, http.MethodPatch, "/v1/sandbox-groups/"+missing, `{"concurrencyLimit":2}`); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound {
		t.Errorf("patch unknown group = %d %s, want 404", code, body)
	}
	if got := f.setLimit(t, second, 7); got.ConcurrencyLimit != 7 {
		t.Fatalf("limit = %d", got.ConcurrencyLimit)
	}
}

// A workspace with no group gets exactly one default the first time a sandbox
// needs one, without anyone creating it.
func TestFirstSandboxCreatesTheDefaultGroup(t *testing.T) {
	f := newGroupFixture(t)
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups?ownerId="+f.ws, ""); code != http.StatusOK || strings.TrimSpace(body) != "[]" {
		t.Fatalf("a workspace that never made a group lists %d %s, want []", code, body)
	}
	sb := f.mustCreate(t, "")
	if !strings.HasPrefix(sb.SandboxGroupID, "sbg-") {
		t.Fatalf("created sandbox = %+v, want a group", sb)
	}
	groups, err := f.st.SandboxGroups(context.Background(), f.ws)
	if err != nil || len(groups) != 1 || groups[0].ID != sb.SandboxGroupID || !groups[0].IsDefault || groups[0].Name != "Default" {
		t.Fatalf("groups = %+v (%v), want the one default, named Default", groups, err)
	}
	// More creates reuse it; the workspace still has exactly one.
	if again := f.mustCreate(t, ""); again.SandboxGroupID != sb.SandboxGroupID {
		t.Fatalf("second create in %s, want the default %s", again.SandboxGroupID, sb.SandboxGroupID)
	}
	if groups, _ = f.st.SandboxGroups(context.Background(), f.ws); len(groups) != 1 {
		t.Fatalf("%d groups after two creates, want 1", len(groups))
	}
}

// A sandbox created with a group lists under that group and not another; one
// created before groups existed belongs to the default group.
func TestSandboxesListUnderTheirGroup(t *testing.T) {
	f := newGroupFixture(t)
	def := f.createGroup(t, "default-one")
	other := f.createGroup(t, "other")
	f.addBox("os-legacy", "Running")

	inDefault := f.mustCreate(t, "")
	inOther := f.mustCreate(t, other.ID)
	if inDefault.SandboxGroupID != def.ID || inOther.SandboxGroupID != other.ID {
		t.Fatalf("created in %s and %s, want %s and %s", inDefault.SandboxGroupID, inOther.SandboxGroupID, def.ID, other.ID)
	}

	listIDs := func(query string) []string {
		t.Helper()
		code, body := f.do(t, http.MethodGet, "/v1/sandboxes?ownerId="+f.ws+query, "")
		var items []SandboxEnvelope
		if err := json.Unmarshal([]byte(body), &items); code != http.StatusOK || err != nil {
			t.Fatalf("list%s = %d %s", query, code, body)
		}
		var out []string
		for _, item := range items {
			out = append(out, item.Sandbox.ID+"@"+item.Sandbox.SandboxGroupID)
		}
		return out
	}
	if got := listIDs("&sandboxGroupId=" + other.ID); len(got) != 1 || got[0] != inOther.ID+"@"+other.ID {
		t.Errorf("list in the other group = %v, want only %s", got, inOther.ID)
	}
	got := listIDs("&sandboxGroupId=" + def.ID)
	if len(got) != 2 || !slices.Contains(got, inDefault.ID+"@"+def.ID) || !slices.Contains(got, "os-legacy@"+def.ID) {
		t.Errorf("list in the default group = %v, want the new sandbox and the pre-groups one", got)
	}
	if all := listIDs(""); len(all) != 3 {
		t.Errorf("unfiltered list = %v, want all three", all)
	}
	// A get carries the group too.
	if code, body := f.do(t, http.MethodGet, "/v1/sandboxes/"+inOther.ID+"?ownerId="+f.ws, ""); code != http.StatusOK || !strings.Contains(body, `"sandboxGroupId":"`+other.ID+`"`) {
		t.Errorf("get = %d %s, want its group", code, body)
	}

	// A group that is not this workspace's, or not a group id, is refused.
	if code, body := f.do(t, http.MethodGet, "/v1/sandboxes?ownerId="+f.ws+"&sandboxGroupId=sbg-"+strings.Repeat("a", 20), ""); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound {
		t.Errorf("list in an unknown group = %d %s, want 404 %s", code, body, CodeSandboxGroupNotFound)
	}
	if code, body := f.create(t, "sbg-"+strings.Repeat("a", 20)); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound {
		t.Errorf("create in an unknown group = %d %s, want 404", code, body)
	}
	if f.os.count("Running") != 3 {
		t.Errorf("%d sandboxes running, want 3: a refused create must start nothing", f.os.count("Running"))
	}
}

// Agent-session sandboxes are not Render sandboxes: no group, never counted.
func TestAgentSessionSandboxesHaveNoGroupAndDoNotCount(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "g")
	f.setLimit(t, g, 1)
	f.os.add("os-agent", "Running", f.now.Add(-time.Minute), "0", map[string]string{metadataWorkspace: f.ws, metadataAgentSession: "ags-1"})
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Fatalf("create with an agent-session sandbox running = %d %s, want it uncounted", code, body)
	}
	code, body := f.do(t, http.MethodGet, "/v1/sandboxes?ownerId="+f.ws, "")
	var items []SandboxEnvelope
	if err := json.Unmarshal([]byte(body), &items); code != http.StatusOK || err != nil {
		t.Fatal(code, body)
	}
	for _, item := range items {
		if item.Sandbox.ID == "os-agent" && item.Sandbox.SandboxGroupID != "" {
			t.Errorf("an agent-session sandbox reports group %s", item.Sandbox.SandboxGroupID)
		}
	}
}

// The scenario Render documents: at the limit a create answers 429; a suspended
// sandbox does not count, so suspending one makes room; resuming a sandbox
// needs room too; lowering the limit stops nothing.
func TestConcurrencyLimitOnCreateAndResume(t *testing.T) {
	f := newGroupFixture(t)
	f.svc.Metrics = NewMetrics(prometheus.NewRegistry())
	g := f.createGroup(t, "limited")
	f.setLimit(t, g, 2)

	a := f.mustCreate(t, g.ID)
	b := f.mustCreate(t, g.ID)
	code, body := f.create(t, g.ID)
	if code != http.StatusTooManyRequests || errCode(body) != CodeSandboxGroupConcurrencyLimit {
		t.Fatalf("third create = %d %s, want 429 %s", code, body, CodeSandboxGroupConcurrencyLimit)
	}
	if !strings.Contains(body, g.ID) || !strings.Contains(body, `"concurrencyLimit":2`) {
		t.Errorf("the 429 does not name the group and its limit: %s", body)
	}
	if n := f.os.count("Running"); n != 2 {
		t.Fatalf("%d sandboxes running after a refused create, want 2", n)
	}

	// Suspending one frees a slot: suspended sandboxes do not count.
	f.suspend(t, a)
	c := f.mustCreate(t, g.ID)

	// Resuming the suspended one now needs a slot, and there is none.
	code, body = f.do(t, http.MethodPost, "/v1/sandboxes/"+a.ID+"/resume?ownerId="+f.ws, "")
	if code != http.StatusTooManyRequests || errCode(body) != CodeSandboxGroupConcurrencyLimit {
		t.Fatalf("resume at the limit = %d %s, want 429 %s", code, body, CodeSandboxGroupConcurrencyLimit)
	}
	if got := f.os.state(f.os.idOf(a.ID)); got != "Paused" {
		t.Fatalf("the refused resume left the sandbox %s, want Paused", got)
	}
	for _, call := range f.os.lifecycleCalls() {
		if strings.HasPrefix(call, "resume") {
			t.Fatalf("the refused resume reached OpenSandbox: %v", f.os.lifecycleCalls())
		}
	}

	// Terminating another makes room, and the same resume succeeds.
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+b.ID+"/terminate?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("terminate = %d %s", code, body)
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+a.ID+"/resume?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("resume with room = %d %s, want it admitted", code, body)
	}
	_ = c

	// Lowering the limit below the running count stops nothing, and refuses
	// every new one until the count falls below it.
	running := f.os.count("Running", "Resuming")
	if running != 2 {
		t.Fatalf("%d sandboxes active, want 2", running)
	}
	f.setLimit(t, g, 1)
	if got := f.os.count("Running", "Resuming"); got != running {
		t.Fatalf("lowering the limit changed the active sandboxes from %d to %d", running, got)
	}
	if code, body := f.create(t, g.ID); code != http.StatusTooManyRequests {
		t.Fatalf("create above a lowered limit = %d %s, want 429", code, body)
	}
	// Each refusal was counted by what refused it, and none as a create failure.
	for labels, want := range map[[2]string]float64{{"group", "create"}: 2, {"group", "resume"}: 1} {
		if got := testutil.ToFloat64(f.svc.Metrics.refusals.WithLabelValues(labels[0], labels[1])); got != want {
			t.Errorf("refusals%v = %v, want %v", labels, got, want)
		}
	}
	if got := testutil.ToFloat64(f.svc.Metrics.creates.WithLabelValues(outcomeFailed)); got != 0 {
		t.Errorf("a refused create counted as %v failure(s)", got)
	}
}

// Every path that resumes a suspended sandbox is held to the limit: the
// explicit resume, and the exec and the two connect-token mints that wake it.
func TestImplicitResumeIsHeldToTheLimit(t *testing.T) {
	for _, route := range []struct{ name, method, path, body string }{
		{"exec", http.MethodPost, "/exec", `{"command":"true"}`},
		{"run token", http.MethodPost, "/runs/stream/token", `{"command":"true"}`},
		{"file token", http.MethodPost, "/files/upload/token?path=rel/f.txt", ""},
	} {
		t.Run(route.name, func(t *testing.T) {
			f := newGroupFixture(t)
			f.svc.Exec.FileGatewayURL = f.svc.Exec.GatewayURL + "/sandbox-files"
			g := f.createGroup(t, "limited")
			f.setLimit(t, g, 1)
			asleep := f.mustCreate(t, g.ID)
			f.suspend(t, asleep)
			f.mustCreate(t, g.ID) // takes the only slot

			path := "/v1/sandboxes/" + asleep.ID + route.path
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			code, body := f.do(t, route.method, path+sep+"ownerId="+f.ws, route.body)
			if code != http.StatusTooManyRequests || errCode(body) != CodeSandboxGroupConcurrencyLimit {
				t.Fatalf("%s on a suspended sandbox at the limit = %d %s, want 429 %s", route.name, code, body, CodeSandboxGroupConcurrencyLimit)
			}
			for _, call := range f.os.lifecycleCalls() {
				if strings.HasPrefix(call, "resume") {
					t.Fatalf("%s resumed a sandbox of a group at its limit: %v", route.name, f.os.lifecycleCalls())
				}
			}
			if got := f.os.state(f.os.idOf(asleep.ID)); got != "Paused" {
				t.Fatalf("%s left the sandbox %s, want Paused", route.name, got)
			}
		})
	}
}

// A sandbox that is not running does not make the exec route 429 when there is
// room: the same wake-on-exec succeeds once a slot is free.
func TestImplicitResumeSucceedsWithRoom(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "roomy")
	f.setLimit(t, g, 1)
	asleep := f.mustCreate(t, g.ID)
	f.suspend(t, asleep)
	code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+asleep.ID+"/exec?ownerId="+f.ws, `{"command":"true"}`)
	if code != http.StatusOK || !strings.Contains(body, "marker") {
		t.Fatalf("exec on a suspended sandbox with room = %d %s, want it woken and run", code, body)
	}
}

// However many creates race for the last slots, no more than the limit start.
func TestParallelCreatesNeverExceedTheLimit(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "racy")
	const limit, racers = 3, 24
	f.setLimit(t, g, limit)
	f.os.mu.Lock()
	f.os.createDelay = 20 * time.Millisecond // a pod takes time to start, so the list lags the admissions
	f.os.mu.Unlock()

	var created, refused atomic.Int32
	var wg sync.WaitGroup
	for range racers {
		wg.Go(func() {
			switch code, body := f.create(t, g.ID); {
			case code == http.StatusCreated:
				created.Add(1)
			case code == http.StatusTooManyRequests && errCode(body) == CodeSandboxGroupConcurrencyLimit:
				refused.Add(1)
			default:
				t.Errorf("create = %d %s", code, body)
			}
		})
	}
	wg.Wait()
	if created.Load() != limit || refused.Load() != racers-limit || f.os.count("Running") != limit {
		t.Fatalf("created %d, refused %d, running %d of %d racers; want exactly %d created", created.Load(), refused.Load(), f.os.count("Running"), racers, limit)
	}
}

// Parallel resumes are admitted the same way, including while OpenSandbox's
// list still reads an accepted resume as suspended.
func TestParallelResumesNeverExceedTheLimit(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "wake")
	f.setLimit(t, g, 3)
	f.mustCreate(t, g.ID) // one running, holding a slot
	var asleep []Sandbox
	for range 4 {
		sb := f.mustCreate(t, g.ID)
		f.suspend(t, sb)
		f.os.set(f.os.idOf(sb.ID), func(b *fakeBox) { b.lag = true })
		asleep = append(asleep, sb)
	}

	var admitted, refused atomic.Int32
	var wg sync.WaitGroup
	for _, sb := range asleep {
		wg.Go(func() {
			switch code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/resume?ownerId="+f.ws, ""); {
			case code == http.StatusNoContent:
				admitted.Add(1)
			case code == http.StatusTooManyRequests:
				refused.Add(1)
			default:
				t.Errorf("resume = %d %s", code, body)
			}
		})
	}
	wg.Wait()
	// The fake accepted every admitted resume but still lists those sandboxes as
	// Paused (lag): only the lease keeps them counted.
	if admitted.Load() != 2 || refused.Load() != 2 {
		t.Fatalf("admitted %d, refused %d resumes, want 2 admitted into the 2 free slots", admitted.Load(), refused.Load())
	}
	// A repeat of an admitted resume is not refused for holding its own slot.
	for _, sb := range asleep {
		code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/resume?ownerId="+f.ws, "")
		if code == http.StatusTooManyRequests {
			continue // the two refused ones stay refused
		}
		if code != http.StatusNoContent {
			t.Fatalf("repeat resume = %d %s", code, body)
		}
	}
}

// A sandbox that was accepted for resume but still reads suspended holds its
// slot for the lease, so the list's lag cannot let another create take it; and a
// resume that silently did nothing cannot hold the slot past the lease.
func TestSlotLeaseCoversListLagAndExpires(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "lease")
	f.setLimit(t, g, 1)
	sb := f.mustCreate(t, g.ID)
	f.suspend(t, sb)
	f.os.set(f.os.idOf(sb.ID), func(b *fakeBox) { b.lag = true })
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/resume?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("resume = %d %s", code, body)
	}
	// OpenSandbox accepted it but still lists it Paused: the slot is still taken.
	if got := f.os.state(f.os.idOf(sb.ID)); got != "Paused" {
		t.Fatalf("fixture: the box reads %s, want Paused (lag)", got)
	}
	if code, body := f.create(t, g.ID); code != http.StatusTooManyRequests {
		t.Fatalf("create while an accepted resume is not listed yet = %d %s, want 429", code, body)
	}
	// Past the lease with the list still saying Paused, nothing holds the slot.
	f.now = f.now.Add(settleLease + time.Second)
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Fatalf("create after the lease ended = %d %s, want the slot free", code, body)
	}
}

// An admitted create or resume that then fails gives its slot back.
func TestFailedCreateFreesItsSlot(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "failing")
	f.setLimit(t, g, 1)
	f.os.mu.Lock()
	f.os.createStatus, f.os.createBody = http.StatusInternalServerError, "boom"
	f.os.mu.Unlock()
	if code, body := f.create(t, g.ID); code < 500 {
		t.Fatalf("failing create = %d %s", code, body)
	}
	members, err := f.st.SandboxGroupMembers(context.Background(), f.ws)
	if err != nil || len(members) != 0 {
		t.Fatalf("a failed create left bindings %v (%v)", members, err)
	}
	f.os.mu.Lock()
	f.os.createStatus = 0
	f.os.mu.Unlock()
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Fatalf("create after a failed one = %d %s, want the slot free", code, body)
	}
}

// A create that timed out waiting for its pod may still come up, so its slot
// stays reserved until the lease ends; it is not handed to the next create.
func TestPodReadyTimeoutKeepsTheSlotUntilItsLeaseEnds(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "slow")
	f.setLimit(t, g, 1)
	f.os.mu.Lock()
	f.os.createStatus, f.os.createBody = http.StatusInternalServerError, podReadyTimeoutCode
	f.os.mu.Unlock()
	if code, body := f.create(t, g.ID); code < 400 {
		t.Fatalf("timed-out create = %d %s", code, body)
	}
	f.os.mu.Lock()
	f.os.createStatus = 0
	f.os.mu.Unlock()
	if code, body := f.create(t, g.ID); code != http.StatusTooManyRequests {
		t.Fatalf("create while a timed-out one may still start = %d %s, want 429", code, body)
	}
	f.now = f.now.Add(createLease + time.Minute)
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Fatalf("create after the lease ended = %d %s, want it admitted", code, body)
	}
}

// Terminating frees the slot at once, even inside the post-create lease.
func TestTerminateFreesTheSlotImmediately(t *testing.T) {
	f := newGroupFixture(t)
	g := f.createGroup(t, "term")
	f.setLimit(t, g, 1)
	sb := f.mustCreate(t, g.ID)
	if code, _ := f.create(t, g.ID); code != http.StatusTooManyRequests {
		t.Fatal("the second create was not refused")
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/terminate?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("terminate = %d %s", code, body)
	}
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Fatalf("create after terminating = %d %s, want the slot free", code, body)
	}
}

// The workspace cap spans groups; suspended sandboxes do not count toward it.
func TestWorkspaceCapSpansGroups(t *testing.T) {
	f := newGroupFixture(t)
	f.svc.MaxActive = 3
	a, b := f.createGroup(t, "a"), f.createGroup(t, "b")
	first := f.mustCreate(t, a.ID)
	f.mustCreate(t, a.ID)
	f.mustCreate(t, b.ID)
	for _, g := range []Group{a, b} {
		code, body := f.create(t, g.ID)
		if code != http.StatusTooManyRequests || errCode(body) != CodeSandboxWorkspaceConcurrencyLimit {
			t.Fatalf("create in %s at the workspace cap = %d %s, want 429 %s", g.Name, code, body, CodeSandboxWorkspaceConcurrencyLimit)
		}
	}
	// Suspending one frees a workspace slot for either group.
	if code, _ := f.do(t, http.MethodPost, "/v1/sandboxes/"+first.ID+"/pause?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatal("suspend failed")
	}
	f.os.set(f.os.idOf(first.ID), func(b *fakeBox) { b.state = "Paused" })
	f.mustCreate(t, b.ID)
	// And the sandbox that was suspended cannot come back while the workspace is full.
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+first.ID+"/resume?ownerId="+f.ws, ""); code != http.StatusTooManyRequests || errCode(body) != CodeSandboxWorkspaceConcurrencyLimit {
		t.Fatalf("resume at the workspace cap = %d %s, want 429 %s", code, body, CodeSandboxWorkspaceConcurrencyLimit)
	}
}

func TestCountSlots(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Minute), now.Add(-time.Minute)
	box := func(public, state string, extra map[string]string) osSandbox {
		md := map[string]string{
			metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
			metadataNetworkPolicy: string(NetworkPolicyDenyAll), metadataPublicID: public,
		}
		for k, v := range extra {
			md[k] = v
		}
		raw := osSandbox{ID: "os-" + public, Metadata: md}
		raw.Status.State = state
		return raw
	}
	members := map[string]store.SandboxGroupMember{
		"sbx-run":      {GroupID: "g1"},
		"sbx-asleep":   {GroupID: "g1"},
		"sbx-woken":    {GroupID: "g1", LeaseUntil: &future}, // resume accepted, list still Paused
		"sbx-stale":    {GroupID: "g1", LeaseUntil: &past},   // an expired lease holds nothing
		"sbx-err":      {GroupID: "g1", LeaseUntil: &future}, // errored never counts
		"sbx-pending":  {GroupID: "g2", LeaseUntil: &future}, // reserved, not listed yet
		"sbx-vanished": {GroupID: "g2", LeaseUntil: &past},
	}
	live := []osSandbox{
		box("sbx-run", "Running", nil),
		box("sbx-asleep", "Paused", nil),
		box("sbx-woken", "Paused", nil),
		box("sbx-stale", "Paused", nil),
		box("sbx-err", "Failed", nil),
		box("sbx-creating", "Creating", nil),                                      // bound to nothing: default group
		box("sbx-suspending", "Pausing", map[string]string{}),                     // still holds its pod
		box("sbx-resuming", "Resuming", nil),                                      // in flight
		box("sbx-agent", "Running", map[string]string{metadataAgentSession: "x"}), // never counted
		box("sbx-foreign", "Running", map[string]string{metadataWorkspace: "tea-b"}),
	}
	got := countSlots(live, "tea-a", members, "gdef", now)
	// g1: run + woken. g2: pending. gdef: creating + suspending + resuming.
	if got.byGroup["g1"] != 2 || got.byGroup["g2"] != 1 || got.byGroup["gdef"] != 3 || got.workspace != 6 {
		t.Fatalf("ledger = %+v, want g1=2 g2=1 gdef=3 across 6", got)
	}
	for _, id := range []string{"sbx-asleep", "sbx-stale", "sbx-err", "sbx-vanished", "sbx-agent", "sbx-foreign"} {
		if got.holders[id] {
			t.Errorf("%s holds a slot", id)
		}
	}
	for _, id := range []string{"sbx-run", "sbx-woken", "sbx-pending", "sbx-creating", "sbx-suspending", "sbx-resuming"} {
		if !got.holders[id] {
			t.Errorf("%s holds no slot", id)
		}
	}
}

// Which statuses count is the one decision Render's docs leave open; this pins
// bex's reading so a change to it is a visible one.
func TestHoldsSlotStatus(t *testing.T) {
	for status, want := range map[Status]bool{
		StatusCreating: true, StatusRunning: true, StatusSuspending: true, StatusResuming: true,
		StatusSuspended: false, StatusErrored: false, StatusTerminated: false,
	} {
		if got := holdsSlotStatus(status); got != want {
			t.Errorf("holdsSlotStatus(%s) = %v, want %v", status, got, want)
		}
	}
}

func TestGroupValidators(t *testing.T) {
	for _, name := range []string{"CI agents", "a", strings.Repeat("é", 100), "emoji ✨"} {
		if err := validateGroupName(name); err != nil {
			t.Errorf("name %q refused: %v", name, err)
		}
	}
	for _, name := range []string{"", " lead", "trail ", strings.Repeat("a", 101), "tab\there", "nul\x00", "line sep"} {
		err := validateGroupName(name)
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != CodeSandboxGroupNameInvalid {
			t.Errorf("name %q = %v, want %s", name, err, CodeSandboxGroupNameInvalid)
		}
	}
	svc := &Service{MaxGroupConcurrency: 1000}
	if svc.clampLimit(5000) != 1000 || svc.clampLimit(7) != 7 || svc.newGroupLimit() != 1000 {
		t.Error("clamp or default limit is wrong with a ceiling of 1000")
	}
	low := &Service{MaxGroupConcurrency: 50}
	if low.newGroupLimit() != 50 || low.clampLimit(1000) != 50 || low.effectiveLimit(store.SandboxGroup{ConcurrencyLimit: 400}) != 50 {
		t.Error("a ceiling below Render's 1,000 must cap the default, a requested value and a stored one")
	}
	if (&Service{}).clampLimit(1<<40) != 1<<31-1 {
		t.Error("with no ceiling a limit must still fit the column")
	}
}

// busyGroups is a group store whose admission lock is never free.
type busyGroups struct{ GroupStore }

func (busyGroups) WithSandboxAdmission(context.Context, string, func(store.SandboxAdmission) error) error {
	return store.ErrSandboxAdmissionBusy
}

// An admission that waited too long for its workspace's lock is a retryable 503,
// not a 500 and not a 429.
func TestBusyAdmissionIsARetryable503(t *testing.T) {
	svc := &Service{Groups: busyGroups{}}
	_, err := svc.admit(context.Background(), "", "tea-a", store.SandboxGroup{}, store.SandboxGroup{}, "sbx-1", "create", createLease)
	if !errors.Is(err, core.ErrUnavailable) || !hasCode(err, CodeSandboxAdmissionBusy) || errors.Is(err, core.ErrTooManyRequests) {
		t.Fatalf("admit = %v, want a 503 %s", err, CodeSandboxAdmissionBusy)
	}
}
