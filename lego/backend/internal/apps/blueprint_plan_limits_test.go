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

package apps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const threeEnvironmentManifest = `version: "1"
projects:
  - name: platform
    environments:
      - name: production
        services:
          - type: web
            name: web
            runtime: image
            image:
              url: nginx:alpine
      - name: staging
        networking:
          isolation: enabled
      - name: dev
`

// w8/m138 t013: Blueprint validate reports the Hobby environment cap and the
// Pro-only network isolation at their authored entries, with the machine
// code, where it used to answer valid while the apply refused. Pro validates.
func TestBlueprintValidateReportsHobbyPlanLimits(t *testing.T) {
	for plan, hobby := range map[string]bool{"hobby": true, "pro": false} {
		t.Run(plan, func(t *testing.T) {
			st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
			svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
			svc.BlueprintGroups = st
			svc.Environments = st
			withWorkspacePlan(svc, plan)
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

			v, err := svc.ValidateBlueprint(ctx, "tea-a", threeEnvironmentManifest, "")
			if err != nil {
				t.Fatal(err)
			}
			if !hobby {
				if !v.Valid {
					t.Fatalf("pro must validate: %+v", v.Errors)
				}
				return
			}
			if v.Valid || len(v.Errors) != 2 {
				t.Fatalf("hobby: want two plan entries, got %+v", v)
			}
			want := []struct{ code, path, text string }{
				{core.PlanFeatureRequiredCode, "projects[0].environments[1].networking.isolation", "networkIsolationEnabled: network-isolated environments require the Pro plan or higher; this workspace is on Hobby"},
				{core.PlanLimitCode, "projects[0].environments[2]", "environments: a Hobby workspace is limited to 2 environments per project; upgrade the plan to add more"},
			}
			for i, w := range want {
				got := v.Errors[i]
				if got.Code != w.code || got.Path == nil || *got.Path != w.path || got.Error != w.text || got.Line == nil {
					t.Errorf("entry %d = %+v (path %v), want %+v", i, got, got.Path, w)
				}
			}

			// The apply refuses with the same code, and writes no environment.
			_, err = svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: threeEnvironmentManifest})
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Params["plan"] != "hobby" ||
				(coded.Code != core.PlanFeatureRequiredCode && coded.Code != core.PlanLimitCode) {
				t.Fatalf("apply: want a coded plan refusal, got %v", err)
			}
		})
	}
}

// An existing project already at the cap refuses every new environment, while
// an existing isolated environment re-declared as enabled stays valid (legacy
// state keeps working).
func TestBlueprintValidatePlanLimitsCountExistingEnvironments(t *testing.T) {
	st := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	st.projects = []store.Project{{ID: "prj-1", TenantID: "tea-a", Name: "platform"}}
	st.environments = []store.Environment{
		{ID: "evm-1", ProjectID: "prj-1", TenantID: "tea-a", Name: "production"},
		{ID: "evm-2", ProjectID: "prj-1", TenantID: "tea-a", Name: "staging", NetworkIsolationEnabled: true},
	}
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, st)
	svc.BlueprintGroups = st
	svc.Environments = st
	withWorkspacePlan(svc, "hobby")
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

	v, err := svc.ValidateBlueprint(ctx, "tea-a", threeEnvironmentManifest, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Valid || len(v.Errors) != 1 || v.Errors[0].Code != core.PlanLimitCode || !strings.HasSuffix(*v.Errors[0].Path, "environments[2]") {
		t.Fatalf("want only the third environment refused, got %+v", v.Errors)
	}
}
