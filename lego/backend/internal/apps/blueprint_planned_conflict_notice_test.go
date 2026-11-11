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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprint_planned_conflict_notice_test.go is w8/251. A plan that fails
// because another Blueprint manages a resource used to end "retry with
// confirm=…". Render's create, plan and approve have no `confirm`, and the
// pinned CLI has no flag for one, so the surface that printed the advice could
// not follow it. The surfaces that do take the phrase keep it.

func TestAFailedPlanNamesWhatCanBeDoneWithoutAConfirmPhrase(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, perSourceManifest)
	first, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("first", store.BlueprintExistingAdopt))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, "tea-a", nil); err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("second", store.BlueprintExistingAdopt))
	if err != nil {
		t.Fatal(err)
	}
	replanned, err := svc.PlanBlueprint(ctx, second.Blueprint.ID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	want := `service "api" is managed by blueprint ` + first.Blueprint.ID + ` ("first"), and a resource has one managing Blueprint. ` +
		`For a separate copy, create a new Blueprint with existingResources "create_new". ` +
		`To manage this service here, remove it from that Blueprint's file and sync that Blueprint first, which releases it.`
	for name, result := range map[string]BlueprintPlanResult{"create": second, "plan": replanned} {
		if result.PlanningOutcome != PlanningFailed || len(result.Notices) != 1 {
			t.Fatalf("%s = outcome %q, notices %+v, want one failed-plan notice", name, result.PlanningOutcome, result.Notices)
		}
		if got := result.Notices[0].Message; got != want {
			t.Errorf("%s notice =\n  %s\nwant\n  %s", name, got, want)
		}
		if strings.Contains(result.Notices[0].Message, "confirm") {
			t.Errorf("%s notice still offers a confirm phrase: %s", name, result.Notices[0].Message)
		}
	}
}

// Two Blueprints that both planned the same new resource: the second approval
// meets the conflict while it applies. Approve carries no `confirm` either.
func TestAnApprovalThatMeetsAConflictOffersNoConfirmPhrase(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, perSourceManifest)
	first, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("first", store.BlueprintExistingAdopt))
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreatePlannedBlueprint(ctx, perSourceCreate("second", store.BlueprintExistingAdopt))
	if err != nil || second.PlanningOutcome != PlanningPlanned {
		t.Fatalf("second plan = %q, err %v, want a clean plan while nothing manages api yet", second.PlanningOutcome, err)
	}
	if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, "tea-a", nil); err != nil {
		t.Fatal(err)
	}
	_, err = svc.ApproveBlueprint(ctx, second.Blueprint.ID, "tea-a", nil)
	var conflict *core.CodedError
	if !errors.As(err, &conflict) || conflict.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("second approval = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
	}
	if strings.Contains(conflict.Error(), "confirm") || conflict.Params["confirm"] != nil {
		t.Errorf("approval conflict = %q params %v, want no confirm phrase", conflict.Error(), conflict.Params)
	}
	if !strings.Contains(conflict.Error(), `service "api" is managed by blueprint `+first.Blueprint.ID+` ("first")`) {
		t.Errorf("approval conflict = %q, want the resource and its managing Blueprint", conflict.Error())
	}
}

// The one-step create and the review made for it take `confirm`, so they keep
// the phrase, in the message and in params.confirm.
func TestSurfacesThatTakeConfirmKeepThePhrase(t *testing.T) {
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	connectA(t, svc)

	_, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: "https://github.com/bex-co/other", Branch: "main", Path: pathSite, Name: "other"})
	var conflict *core.CodedError
	if !errors.As(err, &conflict) || conflict.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("a second adopt create of a managed resource = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
	}
	phrase, _ := conflict.Params["confirm"].(string)
	if !strings.HasPrefix(phrase, "takeover blueprint ") || !strings.Contains(conflict.Error(), `retry with confirm="`+phrase+`"`) {
		t.Errorf("create conflict = %q params %v, want the takeover phrase in both", conflict.Error(), conflict.Params)
	}

	review, err := svc.previewBlueprintWith(ctx, connOwner, "https://github.com/bex-co/other", "main", pathSite, blueprintPreviewOptions{})
	if err != nil || review.Validation == nil || len(review.Validation.Errors) == 0 {
		t.Fatalf("review = %+v, err %v, want the conflict as a validation entry", review.Validation, err)
	}
	if entry := review.Validation.Errors[0]; entry.Code != "BLUEPRINT_RESOURCE_CONFLICT" || !strings.Contains(entry.Error, `retry with confirm="`+phrase+`"`) {
		t.Errorf("review entry = %+v, want the phrase for the create that takes it", entry)
	}
}
