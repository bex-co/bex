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

// w9/093 on real Postgres: Blueprint deploys that move an existing service
// into another environment, then into another project, land one
// environments.MoveService row each through the store's audit sink, by the
// member who deployed, and the service's REST events feed lists each as
// service_moved with the details a REST move lists: the public prj-/evm- ids
// before and after. The create and a re-deploy that keeps the grouping list
// none, and the REST move back lists the same shape.
func TestBlueprintRegroupListsServiceMovedInTheFeedPG(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "regroup-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
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
		Identities: metadataIdentities{},
	})
	caller := core.WithIdentity(ctx, core.Identity{Subject: "client-1", Method: "session"})
	deploy := func(project, environment string) {
		t.Helper()
		if _, err := srv.Apps.DeployStack(caller, apps.DeployRequest{OwnerID: owner.ID, Manifest: regroupManifest(project, environment)}); err != nil {
			t.Fatalf("deploy into %s/%s: %v", project, environment, err)
		}
	}
	// at is a grouping's public project and environment ids.
	at := func(project, environment string) [2]string {
		t.Helper()
		projects, err := st.ListProjects(ctx, owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range projects {
			if p.Name != project {
				continue
			}
			environments, err := st.ListEnvironments(ctx, p.ID)
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
	var serviceID string
	start := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	moves := func() []string {
		t.Helper()
		rec := do(t, handler, http.MethodGet, "/v1/services/"+serviceID+"/events?startTime="+start, testToken, "")
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
	move := func(from, to [2]string) string { return from[0] + "/" + from[1] + "->" + to[0] + "/" + to[1] }

	deploy("shop", "staging")
	if err := pool.QueryRow(ctx, `SELECT id FROM apps WHERE tenant_id = $1`, owner.ID).Scan(&serviceID); err != nil {
		t.Fatal(err)
	}
	if got := moves(); len(got) != 0 {
		t.Fatalf("the Blueprint's create of the service listed %v, want no move", got)
	}
	staging, production, blog := at("shop", "staging"), at("shop", "production"), at("blog", "production")
	var want []string
	for _, step := range []struct {
		project, environment string
		adds                 []string
	}{
		{"shop", "production", []string{move(staging, production)}},
		{"shop", "production", nil},
		{"blog", "production", []string{move(production, blog)}},
	} {
		deploy(step.project, step.environment)
		want = append(want, step.adds...)
		if got := moves(); !slices.Equal(got, want) {
			t.Fatalf("feed after deploying into %s/%s = %v, want %v", step.project, step.environment, got, want)
		}
	}
	// The REST move back lists the same shape.
	rec := do(t, handler, http.MethodPost, "/v1/environments/"+staging[1]+"/resources", testToken, fmt.Sprintf(`{"resourceIds":[%q]}`, serviceID))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST move = %d %s", rec.Code, rec.Body.String())
	}
	want = append(want, move(blog, staging))
	if got := moves(); !slices.Equal(got, want) {
		t.Fatalf("feed after the REST move = %v, want %v", got, want)
	}

	var callers []string
	rows, err := pool.Query(ctx, `SELECT caller FROM audit_events WHERE workspace_id = $1 AND verb = $2 ORDER BY at`, owner.ID, core.AuditVerbEnvironmentServiceMoved)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
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
	if !slices.Equal(callers, []string{"client-1", "client-1", "client-1"}) {
		t.Fatalf("move rows name %v, want one per listed move by the member", callers)
	}
}
