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
	"slices"
	"strings"
	"testing"
)

// attributionService is the QA a903 shape: one Free web service whose name,
// plan, type, auto-deploy, health path and literal env stay equal while its
// source moves between an image and Git.
func attributionService(source string) string {
	return "services:\n  - name: own-source\n    type: web\n    plan: free\n    autoDeploy: false\n    healthCheckPath: /\n" +
		"    envVars:\n      - key: GREETING\n        value: \"line one\\nline two ☃\"\n      - key: MODE\n        value: production\n" + source
}

var (
	attributionImage = attributionService("    runtime: image\n    image:\n      url: " + switchImage + "\n")
	attributionGit   = attributionService("    runtime: node\n    repo: " + declaredRepo + "\n    branch: master\n    rootDir: site\n" +
		"    buildCommand: 'true'\n    startCommand: node server.js\n")
	// attributionInherited declares no repo: it builds the Blueprint's own.
	attributionInherited = attributionService("    runtime: node\n    rootDir: site\n    buildCommand: 'true'\n    startCommand: node server.js\n")
)

// previewChangedFields creates the service from initial through the real
// stack create, then previews next as a Blueprint fetched from
// containingRepo@main, and returns the service's one action.
func previewChangedFields(t *testing.T, initial, next string) BlueprintPlanAction {
	t.Helper()
	svc, ctx := ownSourceService(newFakeBlueprintStore(), &recordingStore{})
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: initial}); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: next}
	preview, err := svc.PreviewBlueprint(ctx, "tea-a", containingRepo, containingBranch, "", "")
	if err != nil || preview.Validation == nil || preview.Validation.Plan == nil {
		t.Fatalf("PreviewBlueprint: %+v %v", preview, err)
	}
	if actions := preview.Validation.Plan.Actions; len(actions) == 1 {
		return actions[0]
	}
	t.Fatalf("plan = %+v, want one service action", preview.Validation.Plan.Actions)
	return BlueprintPlanAction{}
}

// A source switch lists the fields it changes, once each. Every field a
// narrowed probe reapplied the switch for — the match key, the Free plan,
// equal health path, auto-deploy and env — stays off the list (w4/m229).
func TestBlueprintSourceSwitchListsOnlyTheFieldsItChanges(t *testing.T) {
	for _, tc := range []struct {
		name, initial, next string
		want                []string
	}{
		{"image to declared Git", attributionImage, attributionGit,
			[]string{"branch", "buildCommand", "repo", "rootDir", "runtime", "startCommand"}},
		// The Blueprint's own repo arrives on the branch the image already
		// named, so the switch is listed under the repo it fills in.
		{"image to the Blueprint's Git", attributionImage, attributionInherited,
			[]string{"buildCommand", "repo", "rootDir", "runtime", "startCommand"}},
		{"declared Git back to the image", attributionGit, attributionImage, []string{"image", "runtime"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action := previewChangedFields(t, tc.initial, tc.next)
			if got := fieldPaths(action.ChangedFields); action.Operation != BlueprintPlanUpdate || !slices.Equal(got, tc.want) {
				t.Errorf("%s changedFields = %v, want an update of exactly %v", action.Operation, got, tc.want)
			}
		})
	}
}

// The settled source plans nothing.
func TestBlueprintSettledSourcePlansNoChange(t *testing.T) {
	for _, manifest := range []string{attributionImage, attributionGit} {
		if action := previewChangedFields(t, manifest, manifest); action.Operation != BlueprintPlanNoop || len(action.ChangedFields) != 0 {
			t.Errorf("unchanged manifest planned %s %v, want noop", action.Operation, fieldPaths(action.ChangedFields))
		}
	}
}

// An omission that changes a service is listed under the path it omits, and
// beside a declared change: an omitted buildFilter clears the filter, which
// no declared field does.
func TestBlueprintPlanListsAnOmissionUnderItsOwnPath(t *testing.T) {
	filtered := strings.Replace(attributionGit, "    rootDir: site\n", "    rootDir: site\n    buildFilter:\n      paths: [site/**]\n", 1)
	for _, tc := range []struct {
		name, next string
		want       []string
	}{
		{"omitted buildFilter", attributionGit, []string{"buildFilter"}},
		{"omitted buildFilter and a new health path", strings.Replace(attributionGit, "healthCheckPath: /\n", "healthCheckPath: /live\n", 1),
			[]string{"buildFilter", "healthCheckPath"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action := previewChangedFields(t, filtered, tc.next)
			if got := fieldPaths(action.ChangedFields); action.Operation != BlueprintPlanUpdate || !slices.Equal(got, tc.want) {
				t.Errorf("%s changedFields = %v, want an update of exactly %v", action.Operation, got, tc.want)
			}
		})
	}
}

// REST, GraphQL and MCP validation list the one planner's paths for the same
// switch, as the dashboard's two plan summaries render them.
func TestBlueprintSourceSwitchAttributionAcrossSurfaces(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: attributionImage}); err != nil {
		t.Fatal(err)
	}
	svc.Client = fakeClient(serializedBlueprintApp(t, getApp(t, cl, "own-source")))
	want := []string{"branch", "buildCommand", "repo", "rootDir", "runtime", "startCommand"}
	for _, surface := range blueprintValidationSurfaces(t, svc) {
		t.Run(surface.name, func(t *testing.T) {
			out := surface.validate(t, attributionGit)
			actions, _ := out["plan"].(map[string]any)["actions"].([]any)
			if out["valid"] != true || len(actions) != 1 {
				t.Fatalf("validation = %+v", out)
			}
			action := actions[0].(map[string]any)
			if paths := actionPaths(t, surface.name, action); action["operation"] != "update" || !slices.Equal(paths, want) {
				t.Errorf("%v changedFields = %v, want an update of exactly %v", action["operation"], paths, want)
			}
		})
	}
}

// A declared scaling block owns the instance count, so a numInstances beside
// it changes nothing and is not listed: each path is judged beside the others
// the file declares.
func TestBlueprintPlanJudgesNumInstancesBesideScaling(t *testing.T) {
	paid := strings.Replace(attributionGit, "plan: free", "plan: starter", 1)
	scaled := paid + "    numInstances: 2\n    scaling:\n      minInstances: 1\n      maxInstances: 3\n      targetCPUPercent: 60\n"
	action := previewChangedFields(t, paid, scaled)
	if got := fieldPaths(action.ChangedFields); action.Operation != BlueprintPlanUpdate || !slices.Equal(got, []string{"scaling"}) {
		t.Errorf("%s changedFields = %v, want an update of exactly [scaling]", action.Operation, got)
	}
}
