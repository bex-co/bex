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
	"fmt"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestValidateTypeSpecificCreate pins the type-conditional shape checks that
// specFromCreate delegates to validateTypeSpecificCreate: each service type ×
// offending field must produce the exact pre-refactor error string.
func TestValidateTypeSpecificCreate(t *testing.T) {
	cases := []struct {
		name    string
		svcType string
		req     CreateRequest
		wantErr string // "" => valid
	}{
		{
			name:    "cron_job without schedule",
			svcType: appv1alpha1.TypeCronJob,
			req:     CreateRequest{},
			wantErr: "bad request: schedule is required for a cron_job",
		},
		{
			name:    "cron_job with whitespace-only schedule",
			svcType: appv1alpha1.TypeCronJob,
			req:     CreateRequest{Schedule: "   "},
			wantErr: "bad request: schedule is required for a cron_job",
		},
		{
			name:    "background_worker with hosts",
			svcType: appv1alpha1.TypeBackgroundWorker,
			req:     CreateRequest{Hosts: []string{"api.example.com"}},
			wantErr: "bad request: a background_worker has no ingress and cannot have custom domains",
		},
		{
			name:    "cron_job with hosts",
			svcType: appv1alpha1.TypeCronJob,
			req:     CreateRequest{Schedule: "* * * * *", Hosts: []string{"api.example.com"}},
			wantErr: "bad request: a cron_job has no ingress and cannot have custom domains",
		},
		{
			name:    "static_site without publishPath",
			svcType: appv1alpha1.TypeStaticSite,
			req:     CreateRequest{},
			wantErr: "bad request: publishPath is required for a static_site",
		},
		{
			name:    "static_site with a prebuilt image",
			svcType: appv1alpha1.TypeStaticSite,
			req:     CreateRequest{Image: "docker.io/library/nginx:latest", PublishPath: "dist"},
			wantErr: "bad request: a static_site builds from a Git repo; a prebuilt image is not supported",
		},
		{
			name:    "static_site with invalid route type",
			svcType: appv1alpha1.TypeStaticSite,
			req: CreateRequest{
				PublishPath: "dist",
				Routes:      []StaticRouteView{{Type: "proxy", Source: "/a", Destination: "/b"}},
			},
			wantErr: "bad request: routes[0].type must be redirect or rewrite",
		},
		{
			name:    "static_site with invalid header path",
			svcType: appv1alpha1.TypeStaticSite,
			req: CreateRequest{
				PublishPath: "dist",
				Headers:     []StaticHeaderView{{Path: "no-slash", Name: "X-A", Value: "1"}},
			},
			wantErr: "bad request: headers[0].path must be a path starting with /",
		},
		{
			name:    "web_service with publishPath",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{PublishPath: "dist"},
			wantErr: "bad request: publishPath/routes/headers only apply to a static_site",
		},
		{
			name:    "web_service with routes",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Routes: []StaticRouteView{{Type: "redirect", Source: "/a", Destination: "/b"}}},
			wantErr: "bad request: publishPath/routes/headers only apply to a static_site",
		},
		{
			name:    "private_service with headers",
			svcType: appv1alpha1.TypePrivateService,
			req:     CreateRequest{Headers: []StaticHeaderView{{Path: "/", Name: "X-A", Value: "1"}}},
			wantErr: "bad request: publishPath/routes/headers only apply to a static_site",
		},
		{
			name:    "schedule precedence over hosts on a cron_job",
			svcType: appv1alpha1.TypeCronJob,
			req:     CreateRequest{Hosts: []string{"api.example.com"}},
			wantErr: "bad request: schedule is required for a cron_job",
		},
		{
			name:    "valid web_service",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Hosts: []string{"api.example.com"}},
		},
		{
			name:    "valid cron_job",
			svcType: appv1alpha1.TypeCronJob,
			req:     CreateRequest{Schedule: "*/5 * * * *"},
		},
		// The four image-valid types keep their prebuilt-image source — the
		// static_site guard above must not over-reach (w8/m32).
		{
			name:    "web_service with a prebuilt image",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Image: "nginx:1"},
		},
		{
			name:    "private_service with a prebuilt image",
			svcType: appv1alpha1.TypePrivateService,
			req:     CreateRequest{Image: "nginx:1"},
		},
		{
			name:    "background_worker with a prebuilt image",
			svcType: appv1alpha1.TypeBackgroundWorker,
			req:     CreateRequest{Image: "nginx:1"},
		},
		{
			name:    "cron_job with a prebuilt image",
			svcType: appv1alpha1.TypeCronJob,
			req:     CreateRequest{Schedule: "*/5 * * * *", Image: "nginx:1"},
		},
		{
			name:    "valid static_site",
			svcType: appv1alpha1.TypeStaticSite,
			req: CreateRequest{
				PublishPath: "dist",
				Routes:      []StaticRouteView{{Type: "rewrite", Source: "/a", Destination: "/b"}},
				Headers:     []StaticHeaderView{{Path: "/", Name: "X-A", Value: "1"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTypeSpecificCreate(tc.svcType, tc.req)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateTypeSpecificCreate(%s) = %v, want nil", tc.svcType, err)
				}
				return
			}
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("validateTypeSpecificCreate(%s) = %v, want %q", tc.svcType, err, tc.wantErr)
			}
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("validateTypeSpecificCreate(%s) error is not core.ErrBadRequest: %v", tc.svcType, err)
			}
		})
	}
}

// TestSpecFromCreateStaticSiteImageParity pins the full create path (w8/m32):
// a static_site + image request is refused before any spec is assembled —
// Render's static sites are Git-repo-only (ADR029) — while the four
// image-valid types still create from the same image.
func TestSpecFromCreateStaticSiteImageParity(t *testing.T) {
	_, err := specFromCreate(CreateRequest{
		Name: "site", Type: appv1alpha1.TypeStaticSite,
		Image: "docker.io/library/nginx:latest", PublishPath: "dist",
	})
	if err == nil || !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "a static_site builds from a Git repo") {
		t.Fatalf("specFromCreate(static_site+image) = %v, want the static-image bad request", err)
	}
	for _, tc := range []struct {
		svcType  string
		schedule string
	}{
		{svcType: appv1alpha1.TypeWebService},
		{svcType: appv1alpha1.TypePrivateService},
		{svcType: appv1alpha1.TypeBackgroundWorker},
		{svcType: appv1alpha1.TypeCronJob, schedule: "*/5 * * * *"},
	} {
		spec, err := specFromCreate(CreateRequest{Name: "svc", Type: tc.svcType, Image: "nginx:1", Schedule: tc.schedule})
		if err != nil {
			t.Fatalf("specFromCreate(%s+image): %v", tc.svcType, err)
		}
		if spec.Image != "nginx:1" {
			t.Fatalf("specFromCreate(%s+image) image = %q, want nginx:1", tc.svcType, spec.Image)
		}
	}
}

// TestSpecFromCreateDefaults pins the defaulting the create normalization
// applies: empty inputs resolve to the platform defaults, asserted on the
// assembled spec so the pins survive any reshuffling of the private
// normalization helpers.
func TestSpecFromCreateDefaults(t *testing.T) {
	t.Run("image-backed request defaults", func(t *testing.T) {
		spec, err := specFromCreate(CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1"})
		if err != nil {
			t.Fatalf("specFromCreate: %v", err)
		}
		if spec.Tier != tiers.Compute.Default().ID {
			t.Errorf("tier = %q, want default %q", spec.Tier, tiers.Compute.Default().ID)
		}
		if spec.Port != appv1alpha1.DefaultPort {
			t.Errorf("port = %d, want %d", spec.Port, appv1alpha1.DefaultPort)
		}
		if spec.Replicas != 1 {
			t.Errorf("replicas = %d, want 1", spec.Replicas)
		}
		if spec.Branch != "" {
			t.Errorf("branch = %q, want empty for an image-backed create", spec.Branch)
		}
		if spec.Runtime != "" || spec.Builder != "" {
			t.Errorf("runtime/builder = %q/%q, want empty/empty", spec.Runtime, spec.Builder)
		}
		if spec.AutoDeploy {
			t.Error("autoDeploy = true, want false for an image-backed create")
		}
		if spec.BuildFilter != nil {
			t.Errorf("buildFilter = %+v, want nil", spec.BuildFilter)
		}
		if spec.MaintenanceMode != nil {
			t.Errorf("maintenanceMode = %+v, want nil", spec.MaintenanceMode)
		}
		if spec.NotifyOnFail != notifyOnFailDefault {
			t.Errorf("notifyOnFail = %q, want %q", spec.NotifyOnFail, notifyOnFailDefault)
		}
		if spec.SubdomainPolicy != appv1alpha1.SubdomainPolicyEnabled {
			t.Errorf("subdomainPolicy = %q, want %q", spec.SubdomainPolicy, appv1alpha1.SubdomainPolicyEnabled)
		}
		if spec.RegistryCredentialID != nil {
			t.Errorf("registryCredentialID = %v, want nil", spec.RegistryCredentialID)
		}
	})

	t.Run("repo-backed defaults", func(t *testing.T) {
		spec, err := specFromCreate(CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Repo: "https://github.com/acme/web"})
		if err != nil {
			t.Fatalf("specFromCreate: %v", err)
		}
		if spec.Branch != "main" {
			t.Errorf("branch = %q, want main", spec.Branch)
		}
		if !spec.AutoDeploy {
			t.Error("autoDeploy = false, want true for a repo-backed create")
		}
	})

	t.Run("explicit values win", func(t *testing.T) {
		autoDeploy := false
		spec, err := specFromCreate(CreateRequest{
			Name:                 "web",
			Type:                 appv1alpha1.TypeWebService,
			Repo:                 "https://github.com/acme/web",
			Branch:               "release",
			Runtime:              "docker",
			Port:                 8080,
			Replicas:             3,
			Plan:                 "starter",
			AutoDeploy:           &autoDeploy,
			NotifyOnFail:         "notify",
			SubdomainPolicy:      appv1alpha1.SubdomainPolicyDisabled,
			Hosts:                []string{"www.example.com"},
			RegistryCredentialID: strp(" rc-1 "),
		})
		if err != nil {
			t.Fatalf("specFromCreate: %v", err)
		}
		wantTier, ok := tiers.Compute.ByRenderPlan("starter")
		if !ok {
			t.Fatal("tiers catalog has no starter plan")
		}
		if spec.Tier != wantTier.ID {
			t.Errorf("tier = %q, want %q", spec.Tier, wantTier.ID)
		}
		if spec.Port != 8080 || spec.Replicas != 3 || spec.Branch != "release" {
			t.Errorf("port/replicas/branch = %d/%d/%q, want 8080/3/release", spec.Port, spec.Replicas, spec.Branch)
		}
		if spec.Runtime != "docker" || spec.Builder != "dockerfile" {
			t.Errorf("runtime/builder = %q/%q, want docker/dockerfile", spec.Runtime, spec.Builder)
		}
		if spec.AutoDeploy {
			t.Error("autoDeploy = true, want explicit false to win over the repo default")
		}
		if spec.NotifyOnFail != "notify" {
			t.Errorf("notifyOnFail = %q, want notify", spec.NotifyOnFail)
		}
		if spec.SubdomainPolicy != appv1alpha1.SubdomainPolicyDisabled {
			t.Errorf("subdomainPolicy = %q, want disabled", spec.SubdomainPolicy)
		}
		if spec.RegistryCredentialID == nil || *spec.RegistryCredentialID != "rc-1" {
			t.Errorf("registryCredentialID = %v, want trimmed rc-1", spec.RegistryCredentialID)
		}
	})
}

// TestSpecFromCreateNormalizationErrors pins the exact error string of every
// rule the create normalization enforces, one case per rule.
func TestSpecFromCreateNormalizationErrors(t *testing.T) {
	// Each request carries a valid name and source so the rule under test is
	// the one that answers, not an earlier guard.
	cases := []struct {
		name    string
		svcType string
		req     CreateRequest
		wantErr string
	}{
		{
			name:    "unknown plan",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Plan: "mega"},
			wantErr: fmt.Sprintf("bad request: plan must be one of %s", joinRenderPlans()),
		},
		{
			name:    "maintenanceMode on a private service",
			svcType: appv1alpha1.TypePrivateService,
			req:     CreateRequest{Plan: "starter", MaintenanceMode: &MaintenanceModeView{Enabled: true}},
			wantErr: "bad request: maintenanceMode is available only for web services",
		},
		{
			name:    "maintenanceMode on the free plan",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{MaintenanceMode: &MaintenanceModeView{Enabled: true}},
			wantErr: "bad request: maintenanceMode requires a paid web service plan",
		},
		{
			name:    "negative port",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Port: -1},
			wantErr: "bad request: port must be 1-65535",
		},
		{
			name:    "port beyond 65535",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Port: 65536},
			wantErr: "bad request: port must be 1-65535",
		},
		{
			name:    "negative replicas",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Replicas: -1},
			wantErr: fmt.Sprintf("bad request: replicas must be 0-%d", store.MaxReplicas),
		},
		{
			name:    "replicas beyond the cap",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Replicas: store.MaxReplicas + 1},
			wantErr: fmt.Sprintf("bad request: replicas must be 0-%d", store.MaxReplicas),
		},
		{
			name:    "runtime and builder both set",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Runtime: "docker", Builder: "buildpack"},
			wantErr: "bad request: runtime and builder cannot both select a build strategy",
		},
		{
			name:    "invalid buildFilter glob",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{BuildFilter: &BuildFilterView{Paths: []string{"["}}},
			wantErr: `bad request: buildFilter.paths has an invalid glob pattern "["`,
		},
		{
			name:    "maintenanceMode uri not absolute",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{Plan: "starter", MaintenanceMode: &MaintenanceModeView{Enabled: true, URI: "not-a-url"}},
			wantErr: "bad request: maintenanceMode.uri must be an absolute http(s) URL",
		},
		{
			name:    "unknown notifyOnFail",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{NotifyOnFail: "sometimes"},
			wantErr: "bad request: notifyOnFail must be one of default|notify|ignore",
		},
		{
			name:    "unknown subdomain policy",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{SubdomainPolicy: "off"},
			wantErr: "bad request: renderSubdomainPolicy must be enabled or disabled",
		},
		{
			name:    "subdomain disabled without custom hosts",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{SubdomainPolicy: appv1alpha1.SubdomainPolicyDisabled},
			wantErr: "bad request: renderSubdomainPolicy cannot be disabled without at least one custom domain",
		},
		{
			name:    "invalid ip allowlist entry",
			svcType: appv1alpha1.TypeWebService,
			req:     CreateRequest{IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "not-a-cidr"}}},
			wantErr: `bad request: "not-a-cidr" is not a valid CIDR`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			req.Name = "web"
			req.Type = tc.svcType
			req.Repo = "https://github.com/acme/web"
			_, err := specFromCreate(req)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("specFromCreate = %v, want %q", err, tc.wantErr)
			}
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("specFromCreate error is not core.ErrBadRequest: %v", err)
			}
		})
	}
}

func joinRenderPlans() string {
	return strings.Join(tiers.Compute.RenderPlans(), "|")
}

func strp(s string) *string { return &s }
