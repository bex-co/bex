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

package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ip_rules_test.go covers w9/m388's workspace inbound IP rules: who may set
// them (Admin, Scale and up), what a change projects onto which resources,
// the audit row, and the reads on REST (Render's owner.ipAllowList), GraphQL
// and MCP.

// ipStore is fakeStore plus the rules table.
type ipStore struct {
	*fakeStore
	mu     sync.Mutex
	lists  map[string][]core.IPAllowListEntry
	writes int
	reads  int
	// afterWrite, when set, replaces the stored list after the next write: a
	// second Admin's save landing during the first one's projection.
	afterWrite []core.IPAllowListEntry
}

func newIPStore() *ipStore {
	return &ipStore{fakeStore: newFakeStore(), lists: map[string][]core.IPAllowListEntry{}}
}

func (s *ipStore) WorkspaceIPAllowList(ctx context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	lists, err := s.WorkspaceIPAllowLists(ctx, []string{id})
	entries, ok := lists[id]
	return entries, ok, err
}

func (s *ipStore) WorkspaceIPAllowLists(_ context.Context, ids []string) (map[string][]core.IPAllowListEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	out := map[string][]core.IPAllowListEntry{}
	for _, id := range ids {
		if l, ok := s.lists[id]; ok {
			out[id] = slices.Clone(l)
		}
	}
	return out, nil
}

func (s *ipStore) SetWorkspaceIPAllowList(_ context.Context, id string, entries []core.IPAllowListEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	s.lists[id] = slices.Clone(entries)
	if s.afterWrite != nil {
		s.lists[id] = s.afterWrite
		s.afterWrite = nil
	}
	return nil
}

type recordingAudit struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (r *recordingAudit) Record(_ context.Context, ev core.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return nil
}

func (r *recordingAudit) verbs(verb string) []core.AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []core.AuditEvent
	for _, ev := range r.events {
		if ev.Verb == verb {
			out = append(out, ev)
		}
	}
	return out
}

// layerTarget records the layers one datastore kind was asked to project.
type layerTarget struct {
	mu  sync.Mutex
	set map[string][]string // workspace → cidrs
}

func (l *layerTarget) SetWorkspaceIPAllowList(_ context.Context, workspaceID string, cidrs []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.set == nil {
		l.set = map[string][]string{}
	}
	l.set[workspaceID] = cidrs
	return nil
}

type ipFixture struct {
	svc       *Service
	st        *ipStore
	cl        client.Client
	audit     *recordingAudit
	databases *layerTarget
	keyValues *layerTarget
	checker   *fakeChecker
	ws        string
}

func app(ws, name, typ string) *appv1alpha1.App {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{core.LabelTenant: ws}},
		Spec:       appv1alpha1.AppSpec{Type: typ, Image: "nginx:1"},
	}
	if typ == appv1alpha1.TypeCronJob {
		a.Spec.Schedule = "0 * * * *"
	}
	return a
}

// newIPFixture is a workspace on plan with one App of every type, a
// Postgres, a Key Value, and an App of another workspace.
func newIPFixture(t *testing.T, plan string) ipFixture {
	t.Helper()
	st := newIPStore()
	checker := &fakeChecker{allow: true}
	audit := &recordingAudit{}
	base := &core.Base{
		Authz: checker, Audit: audit,
		Plans: core.WorkspacePlanFunc(func(ctx context.Context, id string) (string, error) {
			t, err := st.GetTenant(ctx, id)
			return t.Plan, err
		}),
	}
	svc := &Service{Base: base, Store: st, Granter: &fakeGranter{}}
	base.Workspace = &fakeResolver{store: st}
	w, err := svc.Create(ctxAs("admin"), "acme", plan)
	if err != nil {
		t.Fatal(err)
	}
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		app(w.ID, "web", appv1alpha1.TypeWebService),
		app(w.ID, "site", appv1alpha1.TypeStaticSite),
		app(w.ID, "private", appv1alpha1.TypePrivateService),
		app(w.ID, "worker", appv1alpha1.TypeBackgroundWorker),
		app(w.ID, "cron", appv1alpha1.TypeCronJob),
		app("tea-other", "foreign", appv1alpha1.TypeWebService),
	).Build()
	base.Client = cl
	databases, keyValues := &layerTarget{}, &layerTarget{}
	svc.IPLayerTargets = []IPLayerTarget{databases, keyValues}
	return ipFixture{svc: svc, st: st, cl: cl, audit: audit, databases: databases, keyValues: keyValues, checker: checker, ws: w.ID}
}

func (f ipFixture) appLayer(t *testing.T, ns, name string) []string {
	t.Helper()
	var a appv1alpha1.App
	if err := f.cl.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, &a); err != nil {
		t.Fatal(err)
	}
	return a.Spec.WorkspaceIPAllowList
}

var office = []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}

// An Admin on Scale saves rules: they are stored, every web service, static
// site, Postgres and Key Value of the workspace gets the layer, private
// services, workers, cron jobs and other workspaces' services do not, and one
// audit row names the actor.
func TestSetIPAllowList_AdminOnScaleStoresProjectsAndAudits(t *testing.T) {
	f := newIPFixture(t, "scale")
	v, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, office)
	if err != nil {
		t.Fatalf("SetIPAllowList: %v", err)
	}
	if f.checker.lastRel != core.RelCanManage || f.checker.lastObject != core.WorkspaceObject(f.ws) {
		t.Fatalf("authorized %s on %s, want %s on %s", f.checker.lastRel, f.checker.lastObject, core.RelCanManage, core.WorkspaceObject(f.ws))
	}
	if !slices.Equal(v.IPAllowList, office) || !slices.Equal(f.st.lists[f.ws], office) {
		t.Fatalf("view %v stored %v, want %v", v.IPAllowList, f.st.lists[f.ws], office)
	}
	want := []string{"203.0.113.0/24"}
	for _, name := range []string{"web", "site"} {
		if got := f.appLayer(t, f.ws, name); !slices.Equal(got, want) {
			t.Errorf("%s layer = %v, want %v", name, got, want)
		}
	}
	for _, name := range []string{"private", "worker", "cron"} {
		if got := f.appLayer(t, f.ws, name); got != nil {
			t.Errorf("ineligible %s got layer %v", name, got)
		}
	}
	if got := f.appLayer(t, "tea-other", "foreign"); got != nil {
		t.Errorf("another workspace's service got layer %v", got)
	}
	if !slices.Equal(f.databases.set[f.ws], want) || !slices.Equal(f.keyValues.set[f.ws], want) {
		t.Errorf("datastores got %v / %v, want %v", f.databases.set, f.keyValues.set, want)
	}
	rows := f.audit.verbs(core.AuditVerbSetWorkspaceIPAllowList)
	if len(rows) != 1 {
		t.Fatalf("audit rows for %s = %d, want 1 (all: %+v)", core.AuditVerbSetWorkspaceIPAllowList, len(rows), f.audit.events)
	}
	ev := rows[0]
	if ev.Caller != "admin" || ev.Outcome != core.AuditAllowed || ev.Resource != core.WorkspaceObject(f.ws) || ev.Relation != core.RelCanManage {
		t.Fatalf("audit row = %+v", ev)
	}
}

// A Developer (no can_manage) is refused before anything is read or written.
func TestSetIPAllowList_NonAdminRefused(t *testing.T) {
	f := newIPFixture(t, "scale")
	f.checker.allow = false
	if _, err := f.svc.SetIPAllowList(ctxAs("developer"), f.ws, office); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-admin: want forbidden, got %v", err)
	}
	if f.st.writes != 0 || f.appLayer(t, f.ws, "web") != nil || f.databases.set != nil {
		t.Fatalf("a refused change wrote: writes=%d web=%v dbs=%v", f.st.writes, f.appLayer(t, f.ws, "web"), f.databases.set)
	}
	if rows := f.audit.verbs(core.AuditVerbSetWorkspaceIPAllowList); len(rows) != 1 || rows[0].Outcome != core.AuditDenied {
		t.Fatalf("denial audit = %+v, want one denied row", rows)
	}
}

// Below Scale a restriction is refused with the one plan code; resetting to
// allow-all and re-sending a stored list (a downgraded workspace) stay open.
func TestSetIPAllowList_BelowScaleRefusedWithPlanCode(t *testing.T) {
	for _, plan := range []string{"hobby", "pro"} {
		t.Run(plan, func(t *testing.T) {
			f := newIPFixture(t, plan)
			_, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, office)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode {
				t.Fatalf("want %s, got %v", core.PlanFeatureRequiredCode, err)
			}
			if coded.Params["feature"] != "inboundIPRules" || coded.Params["requiredPlan"] != "scale" || coded.Params["plan"] != plan {
				t.Fatalf("params = %v", coded.Params)
			}
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("plan refusal is not a 400: %v", err)
			}
			if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, []core.IPAllowListEntry{}); err == nil {
				t.Fatal("an empty (deny-all) list below Scale was accepted")
			}
			if f.st.writes != 0 {
				t.Fatalf("refused changes wrote %d times", f.st.writes)
			}
			// The seeded allow-all, unchanged: accepted, and nothing to store.
			if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, core.DefaultEnvironmentAllowList()); err != nil || f.st.writes != 0 {
				t.Fatalf("allow-all on an unset workspace: err=%v writes=%d", err, f.st.writes)
			}
			// A workspace downgraded with rules saved keeps them, may re-send
			// them, and may lift them; it may not tighten them.
			f.st.lists[f.ws] = office
			if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, office); err != nil {
				t.Fatalf("re-sending the stored rules: %v", err)
			}
			if got := f.appLayer(t, f.ws, "web"); !slices.Equal(got, []string{"203.0.113.0/24"}) {
				t.Fatalf("re-save did not re-project the stored rules: %v", got)
			}
			narrower := []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/28"}}
			if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, narrower); !errors.As(err, &coded) {
				t.Fatalf("tightening below Scale: want the plan refusal, got %v", err)
			}
			if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, core.DefaultEnvironmentAllowList()); err != nil {
				t.Fatalf("resetting to allow-all below Scale: %v", err)
			}
			if got := f.appLayer(t, f.ws, "web"); !slices.Equal(got, []string{"0.0.0.0/0", "::/0"}) {
				t.Fatalf("reset layer = %v", got)
			}
		})
	}
}

// A malformed CIDR is refused before anything is stored.
func TestSetIPAllowList_ValidationRefused(t *testing.T) {
	f := newIPFixture(t, "scale")
	_, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/024"}})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("want bad request, got %v", err)
	}
	if f.st.writes != 0 || len(f.audit.verbs(core.AuditVerbSetWorkspaceIPAllowList)) != 0 {
		t.Fatalf("an invalid list wrote %d times / audited %+v", f.st.writes, f.audit.verbs(core.AuditVerbSetWorkspaceIPAllowList))
	}
}

// An empty list is deny-all: stored empty, projected as the unmatchable
// placeholder on every eligible resource.
func TestSetIPAllowList_EmptyListProjectsDenyAll(t *testing.T) {
	f := newIPFixture(t, "scale")
	if _, err := f.st.UpdateTenantPlan(context.Background(), f.ws, "enterprise"); err != nil {
		t.Fatal(err)
	}
	v, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stored, ok := f.st.lists[f.ws]; !ok || stored == nil || len(stored) != 0 || v.IPAllowList == nil || len(v.IPAllowList) != 0 {
		t.Fatalf("stored %#v (present %v), view %#v; want a saved empty list", stored, ok, v.IPAllowList)
	}
	deny := []string{core.DenyAllCIDR}
	if got := f.appLayer(t, f.ws, "web"); !slices.Equal(got, deny) {
		t.Errorf("web layer = %v, want %v", got, deny)
	}
	if !slices.Equal(f.databases.set[f.ws], deny) || !slices.Equal(f.keyValues.set[f.ws], deny) {
		t.Errorf("datastores got %v / %v, want %v", f.databases.set, f.keyValues.set, deny)
	}
}

// Two overlapping saves converge on the one stored last.
func TestSetIPAllowList_ConvergesOnTheLatestSave(t *testing.T) {
	f := newIPFixture(t, "scale")
	later := []core.IPAllowListEntry{{CIDRBlock: "198.51.100.0/24", Description: "vpn"}}
	f.st.afterWrite = later
	v, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, office)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(v.IPAllowList, later) {
		t.Fatalf("view = %v, want the later save %v", v.IPAllowList, later)
	}
	if got := f.appLayer(t, f.ws, "web"); !slices.Equal(got, []string{"198.51.100.0/24"}) {
		t.Fatalf("web layer = %v, want the later save", got)
	}
	if got := f.databases.set[f.ws]; !slices.Equal(got, []string{"198.51.100.0/24"}) {
		t.Fatalf("postgres layer = %v, want the later save", got)
	}
}

// Without the rules table the setter is unavailable and reads stay the
// seeded allow-all.
func TestSetIPAllowList_StoreWithoutRulesTable(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, nil, nil)
	w, err := svc.Create(ctxAs("admin"), "acme", "scale")
	if err != nil {
		t.Fatal(err)
	}
	owners, err := svc.ListOwners(ctxAs("admin"), OwnerFilter{})
	if err != nil || len(owners) != 1 || !slices.Equal(owners[0].IPAllowList, core.DefaultEnvironmentAllowList()) {
		t.Fatalf("owners = %+v, %v; want the seeded allow-all", owners, err)
	}
	if _, err := svc.SetIPAllowList(ctxAs("admin"), w.ID, office); !errors.Is(err, core.ErrWorkspacesUnavailable) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

// GET /v1/owners and GET /v1/owners/{ownerId} carry Render's ipAllowList:
// the seeded allow-all for a workspace that never saved rules, the saved
// {cidrBlock, description} entries otherwise, and [] for a saved deny-all.
func TestRESTOwnersCarryIPAllowList(t *testing.T) {
	f := newIPFixture(t, "scale")
	mux := http.NewServeMux()
	f.svc.RegisterREST(mux)
	get := func(path string) []byte {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctxAs("admin"))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}
	type entry struct {
		CIDRBlock   *string `json:"cidrBlock"`
		Description *string `json:"description"`
	}
	check := func(label string, got []entry, want []core.IPAllowListEntry) {
		t.Helper()
		if got == nil || len(got) != len(want) {
			t.Fatalf("%s ipAllowList = %v, want %v", label, got, want)
		}
		for i, e := range got {
			// Render's cidrBlockAndDescription requires both keys.
			if e.CIDRBlock == nil || e.Description == nil || *e.CIDRBlock != want[i].CIDRBlock || *e.Description != want[i].Description {
				t.Fatalf("%s entry %d = %+v, want %+v", label, i, e, want[i])
			}
		}
	}
	read := func(want []core.IPAllowListEntry) {
		t.Helper()
		var one struct {
			ID          string  `json:"id"`
			IPAllowList []entry `json:"ipAllowList"`
		}
		if err := json.Unmarshal(get("/v1/owners/"+f.ws), &one); err != nil {
			t.Fatal(err)
		}
		check("retrieve", one.IPAllowList, want)
		var list []struct {
			Owner struct {
				ID          string  `json:"id"`
				IPAllowList []entry `json:"ipAllowList"`
			} `json:"owner"`
		}
		if err := json.Unmarshal(get("/v1/owners"), &list); err != nil {
			t.Fatal(err)
		}
		if len(list) != 1 || list[0].Owner.ID != f.ws {
			t.Fatalf("list = %+v", list)
		}
		check("list", list[0].Owner.IPAllowList, want)
	}
	read(core.DefaultEnvironmentAllowList())
	if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, office); err != nil {
		t.Fatal(err)
	}
	read(office)
	if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, []core.IPAllowListEntry{}); err != nil {
		t.Fatal(err)
	}
	var denied struct {
		IPAllowList []entry `json:"ipAllowList"`
	}
	if err := json.Unmarshal(get("/v1/owners/"+f.ws), &denied); err != nil {
		t.Fatal(err)
	}
	if denied.IPAllowList == nil || len(denied.IPAllowList) != 0 {
		t.Fatalf("saved deny-all reads %v, want []", denied.IPAllowList)
	}
	// Render has no owner write: the REST surface stays read-only.
	req := httptest.NewRequest(http.MethodPatch, "/v1/owners/"+f.ws, nil).WithContext(ctxAs("admin"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH /v1/owners/{id} = %d, want 405", rec.Code)
	}
}

// Internal reads that never show the rules — the workspace list the
// dashboard polls and the CLI resolves names through, one workspace's own
// read, the resource limits — do not read them.
func TestInternalWorkspaceReadsSkipTheRules(t *testing.T) {
	f := newIPFixture(t, "scale")
	reads := f.st.reads
	if _, err := f.svc.List(ctxAs("admin")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GetWorkspace(ctxAs("admin"), f.ws); err != nil {
		t.Fatal(err)
	}
	if f.st.reads != reads {
		t.Fatalf("internal reads read the rules %d times", f.st.reads-reads)
	}
}

// GraphQL reads the rules on Workspace and writes them with
// setWorkspaceIPAllowList; the plan refusal carries its machine code.
func TestGraphQLWorkspaceIPAllowList(t *testing.T) {
	f := newIPFixture(t, "scale")
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: f.svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func(q string, vars map[string]any) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, RequestString: q, VariableValues: vars, Context: ctxAs("admin")})
	}
	// The polled list reads no rules unless the field is selected.
	reads := f.st.reads
	if res := run(`{ workspaces { id name plan } }`, nil); len(res.Errors) > 0 || f.st.reads != reads {
		t.Fatalf("list without ipAllowList: errors %v, rule reads %d", res.Errors, f.st.reads-reads)
	}
	// workspace(id) is the one-workspace read Workspace Settings uses.
	res := run(`query($id: String!) { workspace(id: $id) { id ipAllowList { cidrBlock description } } }`, map[string]any{"id": f.ws})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	b, _ := json.Marshal(res.Data)
	if want := `{"workspace":{"id":"` + f.ws + `","ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"Allow all (default)"},{"cidrBlock":"::/0","description":"Allow all (default)"}]}}`; string(b) != want {
		t.Fatalf("workspace(id) = %s\nwant %s", b, want)
	}
	res = run(`{ workspaces { id ipAllowList { cidrBlock description } } }`, nil)
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	b, _ = json.Marshal(res.Data)
	if want := `{"workspaces":[{"id":"` + f.ws + `","ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"Allow all (default)"},{"cidrBlock":"::/0","description":"Allow all (default)"}]}]}`; string(b) != want {
		t.Fatalf("unset read = %s\nwant %s", b, want)
	}
	res = run(`mutation($id: String!, $l: [IPAllowListEntryInput!]!) { setWorkspaceIPAllowList(id: $id, ipAllowList: $l) { id ipAllowList { cidrBlock description } } }`,
		map[string]any{"id": f.ws, "l": []any{map[string]any{"cidrBlock": "203.0.113.0/24", "description": "office"}}})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	b, _ = json.Marshal(res.Data)
	if want := `{"setWorkspaceIPAllowList":{"id":"` + f.ws + `","ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}}`; string(b) != want {
		t.Fatalf("mutation = %s\nwant %s", b, want)
	}
	if got := f.appLayer(t, f.ws, "web"); !slices.Equal(got, []string{"203.0.113.0/24"}) {
		t.Fatalf("mutation did not project: %v", got)
	}
	res = run(`{ workspaces { ipAllowList { cidrBlock } } }`, nil)
	b, _ = json.Marshal(res.Data)
	if want := `{"workspaces":[{"ipAllowList":[{"cidrBlock":"203.0.113.0/24"}]}]}`; string(b) != want {
		t.Fatalf("read after write = %s", b)
	}
	// Below Scale the resolver returns the coded refusal.
	pro := newIPFixture(t, "pro")
	schema, _ = graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: pro.svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: pro.svc.GraphQLMutation()}),
	})
	res = run(`mutation($id: String!) { setWorkspaceIPAllowList(id: $id, ipAllowList: [{cidrBlock: "203.0.113.0/24"}]) { id } }`, map[string]any{"id": pro.ws})
	if len(res.Errors) != 1 {
		t.Fatalf("pro mutation errors = %v", res.Errors)
	}
	var coded *core.CodedError
	orig := res.Errors[0].OriginalError()
	if wrapped, ok := orig.(*gqlerrors.Error); ok {
		orig = wrapped.OriginalError
	}
	if !errors.As(orig, &coded) || coded.Code != core.PlanFeatureRequiredCode {
		t.Fatalf("pro refusal = %v, want %s", res.Errors[0], core.PlanFeatureRequiredCode)
	}
}

// MCP's list_workspaces carries the rules read-only; there is no MCP setter,
// as Render's MCP has no workspace mutations.
func TestMCPListWorkspacesCarriesIPAllowList(t *testing.T) {
	f := newIPFixture(t, "scale")
	if _, err := f.svc.SetIPAllowList(ctxAs("admin"), f.ws, office); err != nil {
		t.Fatal(err)
	}
	ctx := ctxAs("admin")
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	f.svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "list_workspaces" && tool.Name != "get_workspace_limits" {
			t.Errorf("unexpected workspace MCP tool %q (Render's MCP has no workspace mutations)", tool.Name)
		}
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_workspaces", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("list_workspaces: %+v %v", res, err)
	}
	b, _ := json.Marshal(res.StructuredContent)
	var out struct {
		Workspaces []struct {
			ID          string                  `json:"id"`
			IPAllowList []core.IPAllowListEntry `json:"ipAllowList"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Workspaces) != 1 || !slices.Equal(out.Workspaces[0].IPAllowList, office) {
		t.Fatalf("list_workspaces = %s", b)
	}
}
