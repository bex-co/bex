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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// mcp.go is the MCP fragment for services. Tool names track Render's official
// MCP server (render-oss/render-mcp-server): list_services / get_service are 1:1;
// the lifecycle verbs (restart/suspend/resume_service) are bex extensions named
// after Render's REST verbs. Every tool delegates to the same Service method
// REST/GraphQL call, so the three surfaces cannot drift.

// serviceArgs is the shared single-service argument. Render's tools key on
// `serviceId` (see get_service); for bex that id is the App name (opaque,
// round-tripped from list_services).
type serviceArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
}

// listServicesResult wraps the array — MCP tool outputs must be JSON objects.
type listServicesResult struct {
	Services []renderService `json:"services"`
}

// updatePlanArgs is update_service_plan's input — Render's plan spelling
// (e.g. "pro_plus"), same as the REST/GraphQL surfaces.
type updatePlanArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Plan      string `json:"plan" jsonschema:"the new instance plan, e.g. starter, standard, pro, pro_plus, pro_max, pro_ultra"`
}

// scaleArgs is scale_service's input — the desired running instance count,
// keyed on numInstances like Render's REST/GraphQL surfaces.
type scaleArgs struct {
	ServiceID    string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	NumInstances int32  `json:"numInstances" jsonschema:"the desired number of running instances (1-100)"`
}

// idleTimeoutArgs is update_idle_timeout's input — the free-tier auto-sleep
// window in seconds (0 = controller default). A bex extension, no Render tool.
type idleTimeoutArgs struct {
	ServiceID      string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	IdleTTLSeconds int32  `json:"idleTTLSeconds" jsonschema:"seconds a free-tier service may idle before it auto-sleeps; 0 restores the controller default"`
}

// createWebServiceArgs is create_web_service's input — Render's MCP tool name.
// name/repo/branch/plan/envVars track Render's tool; image/port/replicas are bex
// extensions (Render's tool is git-only and has no port/replicas). One of
// repo/image is required. Render's runtime/buildCommand/startCommand/region are
// omitted — bex builds via Dockerfile/CNB auto-detection, one region.
type createWebServiceArgs struct {
	Name       string      `json:"name" jsonschema:"the service name (a DNS label, 1-30 chars)"`
	Type       string      `json:"type,omitempty" jsonschema:"service type: web_service (default), private_service, or background_worker. Use create_cron_job for a cron_job"`
	Repo       string      `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image      string      `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	Branch     string      `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	Plan       string      `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro, pro_plus, pro_max, pro_ultra (default free)"`
	EnvVars    []envVarArg `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the service"`
	AutoDeploy string      `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
	Port       int32       `json:"port,omitempty" jsonschema:"the port the app listens on (default 3000; ignored for a background_worker)"`
	Replicas   int32       `json:"replicas,omitempty" jsonschema:"desired running instances (default 1)"`
}

// envVarArg is Render's {key, value} env-var shape, shared by the create tool.
type envVarArg struct {
	Key   string `json:"key" jsonschema:"the environment variable name"`
	Value string `json:"value" jsonschema:"the literal value"`
}

func (a createWebServiceArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		Name:       a.Name,
		Type:       a.Type,
		Repo:       a.Repo,
		Image:      a.Image,
		Branch:     a.Branch,
		Plan:       a.Plan,
		Env:        toEnvVars(a.EnvVars),
		AutoDeploy: parseYesNo(a.AutoDeploy),
		Port:       a.Port,
		Replicas:   a.Replicas,
	}
}

// createCronJobArgs is create_cron_job's input — Render's MCP tool name. It
// tracks create_web_service but requires a schedule and has no port/replicas
// (a cron runs its command to completion on the schedule, not as a server).
type createCronJobArgs struct {
	Name       string      `json:"name" jsonschema:"the cron job name (a DNS label, 1-30 chars)"`
	Schedule   string      `json:"schedule" jsonschema:"the cron schedule (standard 5-field crontab, e.g. '0 * * * *')"`
	Repo       string      `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image      string      `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	Branch     string      `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	Plan       string      `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro (default free)"`
	EnvVars    []envVarArg `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the job"`
	AutoDeploy string      `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
}

func (a createCronJobArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		Name:       a.Name,
		Type:       appv1alpha1.TypeCronJob,
		Schedule:   a.Schedule,
		Repo:       a.Repo,
		Image:      a.Image,
		Branch:     a.Branch,
		Plan:       a.Plan,
		Env:        toEnvVars(a.EnvVars),
		AutoDeploy: parseYesNo(a.AutoDeploy),
	}
}

// toEnvVars maps the Render {key,value} env-var shape onto the CR EnvVar type,
// shared by the create tools.
func toEnvVars(in []envVarArg) []appv1alpha1.EnvVar {
	var env []appv1alpha1.EnvVar
	for _, e := range in {
		env = append(env, appv1alpha1.EnvVar{Name: e.Key, Value: e.Value})
	}
	return env
}

// deployArgs is the deploy tool's input: a repo + its bex.yml. Deploy-from-chat
// is create_web_service with a manifest — one agent call takes code to a URL.
type deployArgs struct {
	Repo    string `json:"repo,omitempty" jsonschema:"git repository URL to deploy (overrides the repo in bexYaml, if any)"`
	Branch  string `json:"branch,omitempty" jsonschema:"branch to deploy (overrides the branch in bexYaml, if any)"`
	BexYAML string `json:"bexYaml" jsonschema:"the project's bex.yml (render.yaml-shaped manifest) describing the service"`
}

// domainArgs is the shared custom-domain argument (serviceId + domain name).
type domainArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Name      string `json:"name" jsonschema:"the custom domain FQDN, e.g. www.example.com"`
}

// domainListResult wraps the array — MCP tool outputs must be JSON objects.
type domainListResult struct {
	CustomDomains []renderCustomDomain `json:"customDomains"`
}

// deletedResult is delete_custom_domain's return object.
type deletedResult struct {
	Deleted bool `json:"deleted"`
}

// RegisterMCP adds the service and custom-domain tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_services",
		Description: "List all services (bex Apps) in the workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listServicesResult, error) {
		apps, err := s.List(ctx)
		if err != nil {
			return nil, listServicesResult{}, err
		}
		return nil, listServicesResult{Services: toRenderServices(apps)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_service",
		Description: "Get details about a specific service by id.",
	}, s.serviceTool(s.Get))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_web_service",
		Description: "Create (or update) a web service from a repo or a prebuilt image and get back the service to poll until its url is live. Calling it again for the same name redeploys it. Tracks Render's MCP tool.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createWebServiceArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_cron_job",
		Description: "Create (or update) a cron job that runs a repo/image's command on a schedule, and get back the service. Calling it again for the same name redeploys it. Tracks Render's MCP tool.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createCronJobArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "run_cron_job",
		Description: "Trigger a one-off run of a cron job now (Render's cron run trigger). The run appears in the service's run history once it starts. bex extension over Render's MCP.",
	}, s.serviceTool(s.TriggerCronRun))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "deploy",
		Description: "Deploy a project from a git repo and its bex.yml in one call — takes code to a live https URL. Calling it again for the same service redeploys it. bex extension (pillar 4, deploy-from-chat).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deployArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Deploy(ctx, DeployRequest{Repo: in.Repo, Branch: in.Branch, Manifest: in.BexYAML})
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_service",
		Description: "Restart a service (rolling restart, no downtime). bex extension over Render's MCP.",
	}, s.serviceTool(s.Restart))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_service",
		Description: "Suspend a service: scale to zero, keeping host and certificates. bex extension over Render's MCP.",
	}, s.serviceTool(s.Suspend))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resume_service",
		Description: "Resume a suspended service, restoring its replicas. bex extension over Render's MCP.",
	}, s.serviceTool(s.Resume))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_service_plan",
		Description: "Change a service's instance plan/size (e.g. to starter, standard, pro, pro_plus, pro_max, pro_ultra). Resizes the pod's resources and rolls it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlanArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetPlan(ctx, in.ServiceID, in.Plan)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "scale_service",
		Description: "Scale a service to a specific number of running instances (numInstances, 1-100). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in scaleArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Scale(ctx, in.ServiceID, in.NumInstances)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_idle_timeout",
		Description: "Set a service's idle timeout: seconds a free-tier service may idle before it auto-sleeps (0 = controller default). bex extension over Render's MCP (Render's spin-down window is fixed).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idleTimeoutArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetIdleTTL(ctx, in.ServiceID, in.IdleTTLSeconds)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	// Custom domain tools — tracking render-oss/render-mcp-server tool names.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_custom_domains",
		Description: "List all custom domains configured for a service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, domainListResult, error) {
		domains, err := s.ListDomains(ctx, in.ServiceID)
		if err != nil {
			return nil, domainListResult{}, err
		}
		out := make([]renderCustomDomain, 0, len(domains))
		for _, d := range domains {
			out = append(out, toRenderCustomDomain(d))
		}
		return nil, domainListResult{CustomDomains: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_custom_domain",
		Description: "Get details about a specific custom domain on a service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		d, err := s.GetDomain(ctx, in.ServiceID, in.Name)
		if err != nil {
			return nil, renderCustomDomain{}, err
		}
		return nil, toRenderCustomDomain(d), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_custom_domain",
		Description: "Add a custom domain to a service. The domain must be CNAME'd to the service's platform hostname before TLS can be issued.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		d, err := s.AddDomain(ctx, in.ServiceID, in.Name)
		if err != nil {
			return nil, renderCustomDomain{}, err
		}
		return nil, toRenderCustomDomain(d), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_custom_domain",
		Description: "Remove a custom domain from a service. The operator will remove the Ingress rule and let the TLS certificate expire.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.DeleteDomain(ctx, in.ServiceID, in.Name)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "verify_custom_domain",
		Description: "Re-check a custom domain's DNS/certificate state now and return its fresh verification/serving status plus the DNS record to create (Render's verify). Verification is automatic on bex, so this is a re-read, not a trigger.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in domainArgs) (*mcp.CallToolResult, renderCustomDomain, error) {
		d, err := s.VerifyDomain(ctx, in.ServiceID, in.Name)
		if err != nil {
			return nil, renderCustomDomain{}, err
		}
		return nil, toRenderCustomDomain(d), nil
	})
}

// serviceTool adapts a single-service verb (Get/Restart/Suspend/Resume) into an
// MCP tool handler returning the Render service object — the same mapping REST's
// verb handlers use, so the surfaces stay identical.
func (s *Service) serviceTool(fn func(context.Context, string) (AppView, error)) mcp.ToolHandlerFor[serviceArgs, renderService] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := fn(ctx, in.ServiceID)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	}
}
