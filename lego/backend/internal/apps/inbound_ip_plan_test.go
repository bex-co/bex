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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m137: a service's non-empty inbound IP rules need the Scale plan on every
// write path; a rule that already exists stays enforced and can be cleared.

func tenantWebApp(name string, rules ...string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelServiceName: name}
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Port = 8080
	a.Spec.SetIPAllowListEntries(core.AllowListToSpec(core.AllowListFromCIDRs(rules)))
	return a
}

func withWorkspacePlan(svc *Service, plan string) {
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })
}

func TestServiceInboundIPRulesNeedScalePlan(t *testing.T) {
	office := core.AllowListFromCIDRs([]string{"203.0.113.0/24"})
	for plan, ok := range map[string]bool{"hobby": false, "pro": false, "scale": true, "enterprise": true} {
		t.Run(plan, func(t *testing.T) {
			svc, cl := newService(nil, tenantWebApp("web"))
			withWorkspacePlan(svc, plan)
			_, err := svc.SetIPAllowList(context.Background(), "web", office)
			if ok {
				if err != nil {
					t.Fatalf("%s: %v", plan, err)
				}
				return
			}
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), core.InboundIPRulesMessage) {
				t.Fatalf("%s: want the Scale-plan refusal, got %v", plan, err)
			}
			if len(getApp(t, cl, "web").Spec.EffectiveIPAllowListEntries()) != 0 {
				t.Fatal("a refused write changed the App")
			}
			// The PATCH row checks before applying anything.
			if _, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{IPAllowList: &office}); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("PATCH: want the refusal, got %v", err)
			}
		})
	}
}

func TestServiceInboundIPRulesLegacyRuleStaysEnforcedAndClearable(t *testing.T) {
	svc, cl := newService(nil, tenantWebApp("web", "203.0.113.0/24"))
	withWorkspacePlan(svc, "hobby")
	ctx := context.Background()
	legacy := core.AllowListFromCIDRs([]string{"203.0.113.0/24"})

	if _, err := svc.SetIPAllowList(ctx, "web", legacy); err != nil {
		t.Fatalf("re-sending the unchanged legacy list must pass: %v", err)
	}
	edit := core.AllowListFromCIDRs([]string{"198.51.100.0/24"})
	if _, err := svc.SetIPAllowList(ctx, "web", edit); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("editing a legacy rule must be refused, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.EffectiveIPAllowListEntries(); len(got) != 1 || got[0].CIDR != "203.0.113.0/24" {
		t.Fatalf("the legacy rule stopped being enforced: %+v", got)
	}
	if _, err := svc.SetIPAllowList(ctx, "web", []core.IPAllowListEntry{}); err != nil {
		t.Fatalf("clearing must be allowed: %v", err)
	}
	if len(getApp(t, cl, "web").Spec.EffectiveIPAllowListEntries()) != 0 {
		t.Fatal("clear did not apply")
	}
}

func TestServiceCreateAndBlueprintApplyWithInboundIPRulesNeedScalePlan(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	req := CreateRequest{
		Name: "rules", Type: "web_service", Image: "nginx:1.27", Port: 80,
		IPAllowList: core.AllowListFromCIDRs([]string{"203.0.113.0/24"}),
	}
	for _, tc := range []struct {
		plan  string
		apply func(*Service) error
	}{
		{"hobby", func(s *Service) error { _, err := s.Create(ctx, req); return err }},
		{"scale", func(s *Service) error { _, err := s.Create(ctx, req); return err }},
		{"hobby", func(s *Service) error { _, err := applyServiceForTest(ctx, t, s, req, nil); return err }},
		{"scale", func(s *Service) error { _, err := applyServiceForTest(ctx, t, s, req, nil); return err }},
	} {
		cl := fakeClient()
		svc := &Service{
			Base:  &core.Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
			Store: &recordingStore{},
		}
		withWorkspacePlan(svc, tc.plan)
		err := tc.apply(svc)
		var list appv1alpha1.AppList
		if lerr := cl.List(ctx, &list); lerr != nil {
			t.Fatal(lerr)
		}
		if tc.plan == "scale" {
			if err != nil || len(list.Items) != 1 {
				t.Fatalf("scale: err=%v apps=%d", err, len(list.Items))
			}
			continue
		}
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), core.InboundIPRulesMessage) {
			t.Fatalf("hobby: want the Scale-plan refusal, got %v", err)
		}
		if len(list.Items) != 0 {
			t.Fatalf("a refused create left %d Apps", len(list.Items))
		}
	}
}
