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

// w8/056: create refuses, by type, every setting update refuses — the matrix
// QA found accepted and stored (or rejected for an unrelated reason).
func TestCreateRefusesSettingsTheTypeCannotUse(t *testing.T) {
	base := func(svcType string) CreateRequest {
		req := CreateRequest{Name: "qa", Type: svcType, Image: "busybox:1.37", DryRun: true}
		switch svcType {
		case appv1alpha1.TypeCronJob:
			req.Schedule, req.StartCommand = "0 0 1 1 *", "echo hi"
		case appv1alpha1.TypeStaticSite:
			req.Image, req.Repo, req.BuildCommand, req.PublishPath = "", "https://github.com/acme/site", "npm run build", "dist"
		}
		return req
	}
	settings := map[string]func(*CreateRequest){
		"health check path":     func(r *CreateRequest) { r.HealthCheckPath = "/qa-hc" },
		"pre-deploy command":    func(r *CreateRequest) { r.PreDeployCommand = "echo pre" },
		"renderSubdomainPolicy": func(r *CreateRequest) { r.SubdomainPolicy = "disabled" },
		"ipAllowList":           func(r *CreateRequest) { r.IPAllowList = []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}} },
	}
	refused := map[string][]string{
		appv1alpha1.TypeCronJob:          {"health check path", "pre-deploy command", "renderSubdomainPolicy", "ipAllowList"},
		appv1alpha1.TypeBackgroundWorker: {"health check path", "renderSubdomainPolicy", "ipAllowList"},
		appv1alpha1.TypePrivateService:   {"renderSubdomainPolicy", "ipAllowList"},
		appv1alpha1.TypeStaticSite:       {"pre-deploy command"},
	}
	for svcType, names := range refused {
		for _, name := range names {
			req := base(svcType)
			settings[name](&req)
			err := validateTypeSpecificCreate(svcType, req)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), svcType) {
				t.Errorf("%s with %s: %v, want a 400 naming the type", svcType, name, err)
			}
		}
	}
	// Controls: each setting is accepted where it applies.
	accepted := map[string][]string{
		appv1alpha1.TypeWebService:       {"health check path", "pre-deploy command", "ipAllowList"},
		appv1alpha1.TypePrivateService:   {"health check path", "pre-deploy command"},
		appv1alpha1.TypeBackgroundWorker: {"pre-deploy command"},
		appv1alpha1.TypeStaticSite:       {"ipAllowList"},
	}
	for svcType, names := range accepted {
		for _, name := range names {
			req := base(svcType)
			settings[name](&req)
			if err := validateTypeSpecificCreate(svcType, req); err != nil {
				t.Errorf("%s with %s refused: %v", svcType, name, err)
			}
		}
	}
}

// The renderSubdomainPolicy refusal on a cron job is the type reason, not the
// custom-domain guard a later step used to answer with; and update refuses a
// non-empty ipAllowList on a type with no ingress while still allowing a clear.
func TestCreateAndUpdateShareTheTypeReasons(t *testing.T) {
	svc, _ := newService(nil, cronApp("nightly"))
	_, err := svc.Create(context.Background(), CreateRequest{Name: "c", Type: appv1alpha1.TypeCronJob, Image: "busybox:1.37",
		Schedule: "0 0 1 1 *", StartCommand: "echo hi", SubdomainPolicy: "disabled", DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "no platform subdomain to toggle") {
		t.Fatalf("create cron with renderSubdomainPolicy = %v", err)
	}
	if _, err := svc.SetIPAllowList(context.Background(), "nightly", []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}); err == nil ||
		!strings.Contains(err.Error(), "ipAllowList applies only to web services and static sites") {
		t.Fatalf("update cron ipAllowList = %v", err)
	}
	if _, err := svc.SetIPAllowList(context.Background(), "nightly", nil); err != nil {
		t.Fatalf("clearing a cron's ipAllowList = %v", err)
	}
}

// A Blueprint worker declaring healthCheckPath is refused at that field.
func TestBlueprintWorkerHealthCheckPathIsLocated(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	const manifest = "services:\n  - type: worker\n    name: w1\n    runtime: image\n    image:\n      url: busybox:1.37\n    plan: starter\n    healthCheckPath: /h\n"
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Valid || len(v.Errors) != 1 || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].healthCheckPath" ||
		!strings.Contains(v.Errors[0].Error, "health check path is not applicable to a background_worker") {
		t.Fatalf("validate = %+v", v)
	}
}
