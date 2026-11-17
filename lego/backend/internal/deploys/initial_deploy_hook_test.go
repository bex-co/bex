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

package deploys

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w33/m435: Render runs a Blueprint's initialDeployHook after the first
// successful deploy. bex ran it as the pre-deploy step, so a failing hook read
// as the deploy's own failure. The hook's outcome is now a fact beside the
// deploy, on every surface, and never its status.
func TestAFailedInitialDeployHookChangesNoDeployStatus(t *testing.T) {
	finished := time.Date(2026, 11, 16, 9, 0, 8, 0, time.UTC)
	for name, tc := range map[string]struct {
		run          *appv1alpha1.PreDeployStatus
		wantStatus   string
		wantFinished string
	}{
		"failed": {&appv1alpha1.PreDeployStatus{
			Job: "predeploy-web-gen-2-hook", Generation: 2, Status: appv1alpha1.PreDeployFailed,
			Message: "the initial deploy hook exited with code 3", FinishedAt: finished.Format(time.RFC3339),
		}, store.PreDeployFailed, "2026-11-16T09:00:08Z"},
		"running": {&appv1alpha1.PreDeployStatus{
			Job: "predeploy-web-gen-2-hook", Generation: 2, Status: appv1alpha1.PreDeployRunning,
		}, store.PreDeployRunning, ""},
		"succeeded": {&appv1alpha1.PreDeployStatus{
			Job: "predeploy-web-gen-2-hook", Generation: 2, Status: appv1alpha1.PreDeploySucceeded, FinishedAt: finished.Format(time.RFC3339),
		}, store.PreDeploySucceeded, "2026-11-16T09:00:08Z"},
		"no run": {nil, "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			ds := newFakeStore()
			// Newest first: a later deploy that failed on its own, the deploy
			// that went live under the hook's release, and an older one.
			ds.byApp["srv-1"] = []store.Deploy{
				{ID: "dep-3", AppID: "srv-1", Status: store.DeployUpdateFailed, Generation: 3},
				{ID: "dep-2", AppID: "srv-1", Status: store.DeployLive, Generation: 2, PreDeployStatus: store.PreDeploySucceeded},
				{ID: "dep-1", AppID: "srv-1", Status: store.DeployDeactivated, Generation: 1},
			}
			app := sampleApp("web", "srv-1")
			app.Status.InitialDeployHook = tc.run
			svc, _ := newService(ds, app)

			got, err := svc.List(context.Background(), "web", ListFilter{})
			if err != nil || len(got) != 3 {
				t.Fatalf("List = %+v (err %v), want three deploys", got, err)
			}
			live := got[1]
			if live.Status != store.DeployLive || live.PreDeployStatus != store.PreDeploySucceeded || live.FailureReason != "" {
				t.Fatalf("the hook changed its deploy: %+v, want live with its pre-deploy step succeeded and no failure", live)
			}
			if live.InitialDeployHookStatus != tc.wantStatus || formatTimePtr(live.InitialDeployHookFinishedAt) != tc.wantFinished {
				t.Fatalf("hook on its deploy = %q finished %q, want %q finished %q",
					live.InitialDeployHookStatus, formatTimePtr(live.InitialDeployHookFinishedAt), tc.wantStatus, tc.wantFinished)
			}
			// Only the deploy that went live under the run's release carries it.
			for _, other := range []DeployView{got[0], got[2]} {
				if other.InitialDeployHookStatus != "" || other.InitialDeployHookFinishedAt != nil {
					t.Fatalf("deploy %s carries another deploy's hook run: %+v", other.ID, other)
				}
			}

			// REST and GraphQL read the same view (MCP renders REST's shape).
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/deploys/dep-2", nil))
			var rest map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("GET deploy => %d %s (%v)", rec.Code, rec.Body, err)
			}
			if rest["status"] != store.DeployLive {
				t.Fatalf("REST status = %v, want live whatever the hook did", rest["status"])
			}
			restStatus, _ := rest["initialDeployHookStatus"].(string)
			restFinished, _ := rest["initialDeployHookFinishedAt"].(string)
			if restStatus != tc.wantStatus || restFinished != tc.wantFinished {
				t.Fatalf("REST hook = %q finished %q, want %q finished %q", restStatus, restFinished, tc.wantStatus, tc.wantFinished)
			}
			if _, present := rest["initialDeployHookStatus"]; present != (tc.wantStatus != "") {
				t.Fatalf("REST initialDeployHookStatus present = %v, want it only when the hook ran: %s", present, rec.Body)
			}

			res := graphql.Do(graphql.Params{Schema: testSchema(t, svc), Context: context.Background(),
				RequestString: `{ deploy(serviceId: "web", deployId: "dep-2") { status initialDeployHookStatus initialDeployHookFinishedAt } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("deploy query: %v", res.Errors)
			}
			gql := res.Data.(map[string]any)["deploy"].(map[string]any)
			gqlStatus, _ := gql["initialDeployHookStatus"].(string)
			gqlFinished, _ := gql["initialDeployHookFinishedAt"].(string)
			if gql["status"] != store.DeployLive || gqlStatus != tc.wantStatus || gqlFinished != tc.wantFinished {
				t.Fatalf("GraphQL deploy = %+v, want live with hook %q finished %q", gql, tc.wantStatus, tc.wantFinished)
			}
		})
	}
}

// A deploy that never went live cannot have run the hook, even under the
// run's own generation.
func TestInitialDeployHookRunIsReadOnlyOnADeployThatWentLive(t *testing.T) {
	app := sampleApp("web", "srv-1")
	app.Status.InitialDeployHook = &appv1alpha1.PreDeployStatus{Generation: 2, Status: appv1alpha1.PreDeploySucceeded}
	for _, status := range []string{store.DeployUpdateFailed, store.DeployCanceled, store.DeployUpdateInProgress, store.DeployPreDeployFailed} {
		if got, _ := initialDeployHookRun(app, store.Deploy{Generation: 2, Status: status}); got != "" {
			t.Errorf("a %s deploy reads hook status %q, want none", status, got)
		}
	}
	if got, _ := initialDeployHookRun(app, store.Deploy{Status: store.DeployLive}); got != "" {
		t.Errorf("a deploy with no generation reads hook status %q, want none", got)
	}
	if got, _ := initialDeployHookRun(nil, store.Deploy{Generation: 2, Status: store.DeployLive}); got != "" {
		t.Errorf("no App reads hook status %q, want none", got)
	}
}
