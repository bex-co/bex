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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// regroupManifest declares web in project/environment beside the shop
// (staging, production) and blog (production) groupings.
func regroupManifest(project, environment string) string {
	const web = `          - type: web
            name: web
            runtime: image
            image: {url: docker.io/library/nginx:alpine}
`
	manifest := "projects:\n"
	for _, p := range []struct {
		name         string
		environments []string
	}{{"shop", []string{"staging", "production"}}, {"blog", []string{"production"}}} {
		manifest += fmt.Sprintf("  - name: %s\n    environments:\n", p.name)
		for _, e := range p.environments {
			manifest += fmt.Sprintf("      - name: %s\n", e)
			if p.name == project && e == environment {
				manifest += "        services:\n" + web
			}
		}
	}
	return manifest
}

// regroupPG is a workspace on real Postgres behind the composed server,
// whose member client-1 is an Admin, for the Blueprint regroup tests.
type regroupPG struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	st      *store.PGStore
	owner   string
	handler http.Handler
	srv     *Server
}

func newRegroupPG(t *testing.T, name string) regroupPG {
	t.Helper()
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, name+"-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, owner.ID)
		_ = st.DeleteTenant(context.Background(), owner.ID)
	})
	base := serverBase(t, st)
	base.Client = fakeClient()
	base.Authz = &placementRoleChecker{admin: true}
	base.Audit = st
	handler, srv := serverWith(t, base, Deps{
		WorkspaceStore: st, Store: st, ProjectsStore: st, EnvironmentsStore: st, AppPlacements: st, EventStore: st,
		BlueprintsStore: st, Identities: metadataIdentities{},
	})
	return regroupPG{ctx: ctx, pool: pool, st: st, owner: owner.ID, handler: handler, srv: srv}
}

// at is a grouping's public project and environment ids.
func (f regroupPG) at(t *testing.T, project, environment string) [2]string {
	t.Helper()
	projects, err := f.st.ListProjects(f.ctx, f.owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range projects {
		if p.Name != project {
			continue
		}
		environments, err := f.st.ListEnvironments(f.ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range environments {
			if e.Name == environment {
				return [2]string{p.ID, ids.EnvironmentPublicID(e.ID)}
			}
		}
	}
	t.Fatalf("no grouping %s/%s", project, environment)
	return [2]string{}
}

// regroupMove spells a move as moves lists it.
func regroupMove(from, to [2]string) string {
	return from[0] + "/" + from[1] + "->" + to[0] + "/" + to[1]
}

// serviceID is the workspace's one service.
func (f regroupPG) serviceID(t *testing.T) string {
	t.Helper()
	var id string
	if err := f.pool.QueryRow(f.ctx, `SELECT id FROM apps WHERE tenant_id = $1`, f.owner).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// moves are the service_moved events of the service's REST feed, oldest
// first, each as its details' project/environment before and after.
func (f regroupPG) moves(t *testing.T, serviceID string) []string {
	t.Helper()
	start := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	rec := do(t, f.handler, http.MethodGet, "/v1/services/"+serviceID+"/events?startTime="+start, testToken, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("events = %d %s", rec.Code, rec.Body.String())
	}
	var listed []restEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	var out []string
	for i := len(listed) - 1; i >= 0; i-- { // the feed is newest first
		if ev := listed[i].Event; ev.Type == events.TypeServiceMoved {
			d := ev.Details
			out = append(out, fmt.Sprintf("%v/%v->%v/%v", d["projectFrom"], d["environmentFrom"], d["projectTo"], d["environmentTo"]))
		}
	}
	return out
}

// auditRows are the callers of the workspace's audit rows of verb, oldest
// first.
func (f regroupPG) auditRows(t *testing.T, verb string) []string {
	t.Helper()
	rows, err := f.pool.Query(f.ctx, `SELECT caller FROM audit_events WHERE workspace_id = $1 AND verb = $2 ORDER BY at`, f.owner, verb)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var callers []string
	for rows.Next() {
		var caller string
		if err := rows.Scan(&caller); err != nil {
			t.Fatal(err)
		}
		callers = append(callers, caller)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return callers
}

// post sends body to the composed server's REST path and decodes the answer,
// which must have wantCode.
func (f regroupPG) post(t *testing.T, path, body string, wantCode int) map[string]any {
	t.Helper()
	rec := do(t, f.handler, http.MethodPost, path, testToken, body)
	if rec.Code != wantCode {
		t.Fatalf("POST %s = %d %s, want %d", path, rec.Code, rec.Body.String(), wantCode)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// plannedRawUpdates are the actions a planned sync's rawPlan lists that
// change something, each as "operation kind name resourceId path…".
func plannedRawUpdates(plan map[string]any) []string {
	actions, _ := plan["sync"].(map[string]any)["rawPlan"].(map[string]any)["actions"].([]any)
	var updates []string
	for _, raw := range actions {
		action := raw.(map[string]any)
		if action["operation"] == "noop" {
			continue
		}
		update := fmt.Sprintf("%v %v %v %v", action["operation"], action["kind"], action["name"], action["resourceId"])
		fields, _ := action["changedFields"].([]any)
		for _, field := range fields {
			update += " " + field.(map[string]any)["path"].(string)
		}
		updates = append(updates, update)
	}
	return updates
}

// w9/093 on real Postgres: Blueprint deploys that move an existing service
// into another environment, then into another project, land one
// environments.MoveService row each through the store's audit sink, by the
// member who deployed, and the service's REST events feed lists each as
// service_moved with the details a REST move lists: the public prj-/evm- ids
// before and after. The create and a re-deploy that keeps the grouping list
// none, and the REST move back lists the same shape.
func TestBlueprintRegroupListsServiceMovedInTheFeedPG(t *testing.T) {
	f := newRegroupPG(t, "regroup")
	caller := core.WithIdentity(f.ctx, core.Identity{Subject: "client-1", Method: "session"})
	deploy := func(project, environment string) {
		t.Helper()
		if _, err := f.srv.Apps.DeployStack(caller, apps.DeployRequest{OwnerID: f.owner, Manifest: regroupManifest(project, environment)}); err != nil {
			t.Fatalf("deploy into %s/%s: %v", project, environment, err)
		}
	}

	deploy("shop", "staging")
	serviceID := f.serviceID(t)
	if got := f.moves(t, serviceID); len(got) != 0 {
		t.Fatalf("the Blueprint's create of the service listed %v, want no move", got)
	}
	staging, production, blog := f.at(t, "shop", "staging"), f.at(t, "shop", "production"), f.at(t, "blog", "production")
	var want []string
	for _, step := range []struct {
		project, environment string
		adds                 []string
	}{
		{"shop", "production", []string{regroupMove(staging, production)}},
		{"shop", "production", nil},
		{"blog", "production", []string{regroupMove(production, blog)}},
	} {
		deploy(step.project, step.environment)
		want = append(want, step.adds...)
		if got := f.moves(t, serviceID); !slices.Equal(got, want) {
			t.Fatalf("feed after deploying into %s/%s = %v, want %v", step.project, step.environment, got, want)
		}
	}
	// The REST move back lists the same shape.
	rec := do(t, f.handler, http.MethodPost, "/v1/environments/"+staging[1]+"/resources", testToken, fmt.Sprintf(`{"resourceIds":[%q]}`, serviceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST move = %d %s", rec.Code, rec.Body.String())
	}
	want = append(want, regroupMove(blog, staging))
	if got := f.moves(t, serviceID); !slices.Equal(got, want) {
		t.Fatalf("feed after the REST move = %v, want %v", got, want)
	}
	if callers := f.auditRows(t, core.AuditVerbEnvironmentServiceMoved); !slices.Equal(callers, []string{"client-1", "client-1", "client-1"}) {
		t.Fatalf("move rows name %v, want one per listed move by the member", callers)
	}
}

// movingBlueprintFile serves *contents as the Blueprint file at any commit.
type movingBlueprintFile struct{ contents *string }

func (f movingBlueprintFile) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	return "0123456789abcdef0123456789abcdef01234567", nil
}

func (f movingBlueprintFile) FetchBlueprintFileAtCommit(context.Context, string, string, string, string) (string, error) {
	return *f.contents, nil
}

// w9/094 on real Postgres, through Render's REST verbs: in a Blueprint whose
// syncs wait for approval, a render.yaml change that only moves the service
// into another environment plans that move (201, the rawPlan's update lists
// `environment`), approving it applies the move and lists one service_moved in
// the service's feed, by the approver, and planning the same file again finds
// no changes (200, no sync).
func TestApprovalModeBlueprintMovePlansAndAppliesPG(t *testing.T) {
	f := newRegroupPG(t, "plan-move")
	file := regroupManifest("shop", "staging")
	f.srv.Apps.GitFetcher = movingBlueprintFile{contents: &file}

	created := f.post(t, "/v1/blueprints", fmt.Sprintf(`{"name":"shop","ownerId":%q,"source":{"repo":"https://github.com/acme/shop","branch":"main","path":"render.yaml"}}`, f.owner), http.StatusCreated)
	bpID, _ := created["blueprint"].(map[string]any)["id"].(string)
	f.post(t, "/v1/blueprints/"+bpID+"/approve", `{}`, http.StatusOK)
	serviceID := f.serviceID(t)

	file = regroupManifest("shop", "production")
	plan := f.post(t, "/v1/blueprints/"+bpID+"/syncs", "", http.StatusCreated)
	if plan["planningOutcome"] != apps.PlanningPlanned {
		t.Fatalf("plan = %v, want planned", plan)
	}
	if updates, want := plannedRawUpdates(plan), []string{"update service web " + serviceID + " environment"}; !slices.Equal(updates, want) {
		t.Fatalf("planned %v, want %v", updates, want)
	}
	if got := f.moves(t, serviceID); len(got) != 0 {
		t.Fatalf("planning listed %v, want no move", got)
	}

	f.post(t, "/v1/blueprints/"+bpID+"/approve", `{}`, http.StatusOK)
	if got, want := f.moves(t, serviceID), []string{regroupMove(f.at(t, "shop", "staging"), f.at(t, "shop", "production"))}; !slices.Equal(got, want) {
		t.Fatalf("feed after the approval = %v, want %v", got, want)
	}
	if callers := f.auditRows(t, core.AuditVerbEnvironmentServiceMoved); !slices.Equal(callers, []string{"client-1"}) {
		t.Fatalf("move rows name %v, want one by the approver", callers)
	}

	if again := f.post(t, "/v1/blueprints/"+bpID+"/syncs", "", http.StatusOK); again["planningOutcome"] != apps.PlanningNoChanges || again["sync"] != nil {
		t.Fatalf("re-plan of the applied file = %v, want no_changes", again)
	}
}
