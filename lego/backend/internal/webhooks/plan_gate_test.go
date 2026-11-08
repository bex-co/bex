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

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/m138: Render offers webhooks on Pro and higher — one destination on Pro,
// up to 100 on Scale and Enterprise. Existing endpoints keep delivering. w3/056:
// a Scale workspace reaches its 100th destination and is refused at the 101st.
func TestWebhookCreateFollowsThePlan(t *testing.T) {
	create := func(s *Service, name string) error {
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
		_, err := s.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: name, URL: "https://example.com/" + name, EventTypes: []string{TypeDeployStarted}, Enabled: true})
		return err
	}
	for _, tc := range []struct {
		plan string
		ok   int // endpoints that may be created before the refusal
	}{{"hobby", 0}, {"pro", 1}, {"scale", 100}, {"enterprise", 100}} {
		t.Run(tc.plan, func(t *testing.T) {
			s, _ := newTestService()
			s.Workspace = coretest.Workspaces{"tea-a"}
			s.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return tc.plan, nil })
			for i := range tc.ok {
				if err := create(s, fmt.Sprintf("hook-%d", i+1)); err != nil {
					t.Fatalf("%s create #%d: %v", tc.plan, i+1, err)
				}
			}
			err := create(s, "one-too-many")
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != PlanLimitCode {
				t.Fatalf("%s create #%d: want %s, got %v", tc.plan, tc.ok+1, PlanLimitCode, err)
			}
			if tc.plan == "hobby" && !strings.Contains(err.Error(), "require the Pro plan") {
				t.Fatalf("hobby refusal must name the Pro plan: %v", err)
			}
		})
	}
}

// w3/056: the store's abuse cap backstops the plan gate and never undercuts
// it, so the top plan's destination limit is reachable.
func TestEndpointCapAdmitsTheTopPlanLimit(t *testing.T) {
	for _, plan := range []string{"scale", "enterprise"} {
		if limit := core.WebhookDestinationLimit(plan); store.MaxWebhookEndpointsPerWorkspace < limit {
			t.Errorf("store cap %d is below the %s plan's %d destinations", store.MaxWebhookEndpointsPerWorkspace, plan, limit)
		}
	}
}
