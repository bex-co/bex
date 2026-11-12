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
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

// w9/095 on real Postgres, through Render's REST verbs: in a Blueprint whose
// syncs wait for approval, a render.yaml change whose only effect is to
// protect an environment plans that environment (201; the rawPlan's update
// lists `permissions.protection` by the environment's public id) and leaves
// it unprotected; approving it protects the environment, with one audit row
// by the approver; and planning the same file again finds no changes (200, no
// sync).
func TestApprovalModeBlueprintEnvironmentProtectionPlansAndAppliesPG(t *testing.T) {
	f := newRegroupPG(t, "plan-protection")
	file := regroupManifest("shop", "staging")
	f.srv.Apps.GitFetcher = movingBlueprintFile{contents: &file}
	protection := func(environmentID string) string {
		t.Helper()
		rec := do(t, f.handler, http.MethodGet, "/v1/environments/"+environmentID, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET environment = %d %s", rec.Code, rec.Body.String())
		}
		var view struct {
			ProtectedStatus string `json:"protectedStatus"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		return view.ProtectedStatus
	}
	const controlsUpdated = "apps.BlueprintGrouping.environment_controls_updated"

	created := f.post(t, "/v1/blueprints", fmt.Sprintf(`{"name":"shop","ownerId":%q,"source":{"repo":"https://github.com/acme/shop","branch":"main","path":"render.yaml"}}`, f.owner), http.StatusCreated)
	bpID, _ := created["blueprint"].(map[string]any)["id"].(string)
	f.post(t, "/v1/blueprints/"+bpID+"/approve", `{}`, http.StatusOK)
	production := f.at(t, "shop", "production")[1]
	if got := protection(production); got != "unprotected" {
		t.Fatalf("the create left shop/production %q, want unprotected", got)
	}

	const shopProduction = "      - name: production\n  - name: blog\n"
	if !strings.Contains(file, shopProduction) {
		t.Fatalf("fixture changed: %q", file)
	}
	file = strings.Replace(file, shopProduction, "      - name: production\n        permissions:\n          protection: enabled\n  - name: blog\n", 1)
	plan := f.post(t, "/v1/blueprints/"+bpID+"/syncs", "", http.StatusCreated)
	if plan["planningOutcome"] != apps.PlanningPlanned {
		t.Fatalf("plan = %v, want planned", plan)
	}
	if updates, want := plannedRawUpdates(plan), []string{"update environment production " + production + " permissions.protection"}; !slices.Equal(updates, want) {
		t.Fatalf("planned %v, want %v", updates, want)
	}
	if got := protection(production); got != "unprotected" {
		t.Fatalf("planning left shop/production %q, want it unprotected until the approval", got)
	}

	f.post(t, "/v1/blueprints/"+bpID+"/approve", `{}`, http.StatusOK)
	if got := protection(production); got != "protected" {
		t.Fatalf("the approval left shop/production %q, want protected", got)
	}
	if callers := f.auditRows(t, controlsUpdated); !slices.Equal(callers, []string{"client-1"}) {
		t.Fatalf("environment setting rows name %v, want one by the approver", callers)
	}

	if again := f.post(t, "/v1/blueprints/"+bpID+"/syncs", "", http.StatusOK); again["planningOutcome"] != apps.PlanningNoChanges || again["sync"] != nil {
		t.Fatalf("re-plan of the applied file = %v, want no_changes", again)
	}
}
