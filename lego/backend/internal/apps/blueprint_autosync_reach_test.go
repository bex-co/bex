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
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A Blueprint on a repo whose pushes can't reach bex is reviewed and created
// without auto-sync, and turning auto-sync on is refused; a covered repo keeps
// it (w4/622 § Blueprints too). Only an install that rejects the shared manual
// webhook key has such repos.
func TestBlueprintOnAnUnreachableRepoIsCreatedWithoutAutoSync(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		granted, multitenant bool
		want                 bool
	}{
		{"uncovered", false, true, false},
		{"covered", true, true, true},
		{"uncovered, manual webhook accepted", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, fs := connectionService(t)
			svc.GitHub, svc.ManualWebhookDisabled = &fakeCloneTokens{token: "tok", ok: tc.granted}, tc.multitenant
			ctx := ownershipCtx()
			preview, err := svc.previewBlueprintWith(ctx, connOwner, repoBex, "main", pathCron, blueprintPreviewOptions{})
			if err != nil || preview.AutoSync == nil || *preview.AutoSync != tc.want {
				t.Fatalf("preview autoSync = %v (%v), want %v", preview.AutoSync, err, tc.want)
			}
			b, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "bp"})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if stored := fs.blueprints[b.ID]; stored.AutoSync != tc.want || b.AutoSync != tc.want {
				t.Errorf("autoSync stored %v, read %v; want %v", stored.AutoSync, b.AutoSync, tc.want)
			}
			on := true
			_, err = svc.UpdateBlueprint(ctx, b.ID, connOwner, UpdateBlueprintRequest{AutoSync: &on})
			if tc.want && err != nil {
				t.Errorf("autoSync on: %v", err)
			}
			if !tc.want && (!errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "autoSync needs a connected Git provider")) {
				t.Errorf("autoSync on for an unreachable repo = %v, want the refusal", err)
			}
		})
	}
}

// Render's create (REST POST /v1/blueprints with a source, GraphQL
// createPlannedBlueprint, MCP create_blueprint) follows the same rule: a repo
// whose pushes can't reach bex is created without auto-sync and its approval
// settles paused. An explicit true, which the pinned CLI sends on every
// create, is answered with a warning notice rather than refused (w32/069).
func TestPlannedBlueprintOnAnUnreachableRepoIsCreatedWithoutAutoSync(t *testing.T) {
	on, off := true, false
	for _, tc := range []struct {
		name                 string
		granted, multitenant bool
		autoSync             *bool
		want, notice         bool
	}{
		{"uncovered, omitted", false, true, nil, false, false},
		{"uncovered, true", false, true, &on, false, true},
		{"uncovered, false", false, true, &off, false, false},
		{"covered, omitted", true, true, nil, true, false},
		{"covered, true", true, true, &on, true, false},
		{"uncovered, manual webhook accepted, true", false, false, &on, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, st, ctx := newOnboardingService(t, onboardingManifest)
			svc.GitHub = &fakeCloneTokens{token: "tok", ok: tc.granted, defaultBranch: "master"}
			svc.ManualWebhookDisabled = tc.multitenant
			res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a",
				Source: RenderBlueprintSource{Repo: "https://github.com/render-examples/go-web-server"}, AutoSync: tc.autoSync})
			if err != nil || res.Blueprint == nil || res.PlanningOutcome != PlanningPlanned {
				t.Fatalf("create = %+v err=%v, want a planned Blueprint", res, err)
			}
			stored := st.blueprints[ids.BlueprintStorageID(res.Blueprint.ID)]
			if stored.AutoSync != tc.want || res.Blueprint.AutoSync != tc.want {
				t.Errorf("autoSync stored %v, answered %v; want %v", stored.AutoSync, res.Blueprint.AutoSync, tc.want)
			}
			var warnings []string
			for _, n := range res.Notices {
				if n.Severity == "warning" {
					warnings = append(warnings, n.Message)
				}
			}
			var want []string
			if tc.notice {
				want = []string{"autoSync is off: bex doesn't receive this repo's pushes. Sync the Blueprint manually, or connect the repo through the GitHub app"}
			}
			if !slices.Equal(warnings, want) {
				t.Errorf("warnings = %q, want %q", warnings, want)
			}
			token := "s3cret"
			if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", []ManualEnvVar{{ServiceType: "web", ServiceName: "api", Key: "API_TOKEN", Value: &token}}); err != nil {
				t.Fatalf("approve: %v", err)
			}
			wantStatus := store.BlueprintStatusInSync
			if !tc.want {
				wantStatus = store.BlueprintStatusPaused
			}
			if got := st.blueprints[stored.ID].Status; got != wantStatus {
				t.Errorf("status after approval = %q, want %q", got, wantStatus)
			}
		})
	}
}

// A created Blueprint whose first plan finds nothing to change leaves created
// for paused when its auto-sync is off, as an approved sync does, not in_sync
// (w32/069).
func TestNoChangesFirstPlanOfAManualBlueprintSettlesPaused(t *testing.T) {
	for _, tc := range []struct {
		name     string
		autoSync bool
		want     string
	}{
		{"auto-sync on", true, store.BlueprintStatusInSync},
		{"auto-sync off", false, store.BlueprintStatusPaused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, st, ctx := newOnboardingService(t, recreateManifest)
			req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
			first, err := svc.CreatePlannedBlueprint(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, "tea-a", "", nil); err != nil {
				t.Fatal(err)
			}
			if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, "tea-a"); err != nil {
				t.Fatal(err)
			}
			req.AutoSync = &tc.autoSync
			again, err := svc.CreatePlannedBlueprint(ctx, req)
			if err != nil || again.PlanningOutcome != PlanningNoChanges {
				t.Fatalf("re-create = %+v err=%v, want no_changes", again, err)
			}
			if got := st.blueprints[ids.BlueprintStorageID(again.Blueprint.ID)].Status; got != tc.want || again.Blueprint.Status != tc.want {
				t.Errorf("status stored %q, answered %q; want %q", got, again.Blueprint.Status, tc.want)
			}
		})
	}
}
