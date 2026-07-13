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

	"github.com/bex-co/bex/lego/backend/internal/core"

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

// listServicesArgs is list_services' input — the ownerId scoping filter
// (w6/m2/t004), mirroring the REST/GraphQL surfaces. Empty => unscoped.
type listServicesArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict the list to this workspace id (tea-…); omit to use the session's selected workspace, if any"`
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

// rootDirArgs is set_root_directory's input — Render's Root Directory setting:
// the subdirectory of a build-from-git service's repo to build from.
type rootDirArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	RootDir   string `json:"rootDir" jsonschema:"subdirectory of the repo to build from; empty builds from the repo root"`
}

// updateCronJobArgs is update_cron_job's input — bex's functional implementation
// of the verb Render ships as a non-functional stub. schedule is the 5-field
// crontab expression (required); command is a pointer so nil means "keep the
// existing override" and an empty string means "clear it."
type updateCronJobArgs struct {
	ServiceID string  `json:"serviceId" jsonschema:"the cron job id (bex App name), as returned by list_services"`
	Schedule  string  `json:"schedule" jsonschema:"the new cron schedule (5-field crontab, e.g. '0 0 * * *'); required"`
	Command   *string `json:"command,omitempty" jsonschema:"overrides the image's default entrypoint for each run, e.g. 'npm run report'; omit to keep the existing override, empty string to clear it"`
}

// autoDeployArgs is set_auto_deploy's input — Render's Auto-Deploy toggle.
type autoDeployArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Enabled   bool   `json:"enabled" jsonschema:"true = a git push to the tracked branch redeploys; false = only explicit deploys"`
}

// createWebServiceArgs is create_web_service's input — Render's MCP tool name.
// name/repo/branch/plan/envVars track Render's tool; image/port/replicas are bex
// extensions (Render's tool is git-only and has no port/replicas). One of
// repo/image is required. Render's runtime/buildCommand/startCommand/region are
// omitted — bex builds via Dockerfile/CNB auto-detection, one region.
type createWebServiceArgs struct {
	OwnerID    string      `json:"ownerId,omitempty" jsonschema:"the workspace to create in (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	Name       string      `json:"name" jsonschema:"the service name (a DNS label, 1-30 chars)"`
	Type       string      `json:"type,omitempty" jsonschema:"service type: web_service (default), private_service, or background_worker. Use create_cron_job for a cron_job"`
	Repo       string      `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image      string      `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	Branch     string      `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir    string      `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
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
		OwnerID:    a.OwnerID,
		Name:       a.Name,
		Type:       a.Type,
		Repo:       a.Repo,
		Image:      a.Image,
		Branch:     a.Branch,
		RootDir:    a.RootDir,
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
	OwnerID    string      `json:"ownerId,omitempty" jsonschema:"the workspace to create in (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	Name       string      `json:"name" jsonschema:"the cron job name (a DNS label, 1-30 chars)"`
	Schedule   string      `json:"schedule" jsonschema:"the cron schedule (standard 5-field crontab, e.g. '0 * * * *')"`
	Command    string      `json:"command,omitempty" jsonschema:"overrides the image's default entrypoint for each run, e.g. 'npm run report'; omit to run the image's own command"`
	Repo       string      `json:"repo,omitempty" jsonschema:"git repository URL to build from (build-from-git); omit if using image"`
	Image      string      `json:"image,omitempty" jsonschema:"a prebuilt OCI image to run directly; omit if using repo"`
	Branch     string      `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir    string      `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	Plan       string      `json:"plan,omitempty" jsonschema:"instance plan, e.g. free, starter, standard, pro (default free)"`
	EnvVars    []envVarArg `json:"envVars,omitempty" jsonschema:"literal (non-secret) environment variables to set on the job"`
	AutoDeploy string      `json:"autoDeploy,omitempty" jsonschema:"redeploy on a git push to the branch: yes or no (default yes for a repo)"`
}

func (a createCronJobArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		OwnerID:    a.OwnerID,
		Name:       a.Name,
		Type:       appv1alpha1.TypeCronJob,
		Schedule:   a.Schedule,
		Command:    a.Command,
		Repo:       a.Repo,
		Image:      a.Image,
		Branch:     a.Branch,
		RootDir:    a.RootDir,
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
	OwnerID string `json:"ownerId,omitempty" jsonschema:"the workspace to deploy into (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	Repo    string `json:"repo,omitempty" jsonschema:"git repository URL to deploy (overrides the repo in bexYaml, if any)"`
	Branch  string `json:"branch,omitempty" jsonschema:"branch to deploy (overrides the branch in bexYaml, if any)"`
	BexYAML string `json:"bexYaml" jsonschema:"the project's bex.yml (render.yaml-shaped manifest) describing the service"`
}

// autoscalingArgs is set_autoscaling's input — mirrors Render's PUT
// /v1/services/{id}/autoscaling request body (minInstances / maxInstances /
// targetCPUPercent / targetMemoryPercent).
type autoscalingArgs struct {
	ServiceID           string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	MinInstances        int32  `json:"minInstances" jsonschema:"minimum running instances (≥ 0; default 1)"`
	MaxInstances        int32  `json:"maxInstances" jsonschema:"maximum running instances (≥ 1; must be ≥ minInstances)"`
	TargetCPUPercent    *int32 `json:"targetCPUPercent,omitempty" jsonschema:"target average CPU utilization % of tier limit (1-100); required if targetMemoryPercent is absent"`
	TargetMemoryPercent *int32 `json:"targetMemoryPercent,omitempty" jsonschema:"target average memory utilization % of tier limit (1-100); required if targetCPUPercent is absent"`
}

// domainArgs is the shared custom-domain argument (serviceId + domain name).
type domainArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name), as returned by list_services"`
	Name      string `json:"name" jsonschema:"the custom domain FQDN, e.g. www.example.com"`
}

// staticRouteArg / staticHeaderArg are the MCP shapes of a static_site's edge
// rules — Render's route (type/source/destination) and header (path/name/value).
type staticRouteArg struct {
	Type        string `json:"type" jsonschema:"redirect (301 to destination) or rewrite (serve destination's content with 200)"`
	Source      string `json:"source" jsonschema:"request path pattern to match, e.g. /old or /app/* (trailing /* is a wildcard)"`
	Destination string `json:"destination" jsonschema:"target path, e.g. /new or /index.html; :splat or a trailing /* substitutes the wildcard capture"`
}

type staticHeaderArg struct {
	Path  string `json:"path" jsonschema:"request path pattern the header applies to, e.g. /* or /assets/*"`
	Name  string `json:"name" jsonschema:"response header name, e.g. X-Frame-Options"`
	Value string `json:"value" jsonschema:"response header value, e.g. DENY"`
}

// createStaticSiteArgs is create_static_site's input — Render's MCP tool name.
// A static site builds a repo and serves its publishPath output from the
// object-store origin (no running container). publishPath is required; routes and
// headers are the optional edge rules.
type createStaticSiteArgs struct {
	OwnerID     string            `json:"ownerId,omitempty" jsonschema:"the workspace to create in (an owner id, tea-...); omit to use the workspace selected with select_workspace, else your default workspace"`
	Name        string            `json:"name" jsonschema:"the static site name (a DNS label, 1-30 chars)"`
	Repo        string            `json:"repo,omitempty" jsonschema:"git repository URL to build from; omit if using image"`
	Image       string            `json:"image,omitempty" jsonschema:"a prebuilt OCI image whose publishPath holds the built site; omit if using repo"`
	Branch      string            `json:"branch,omitempty" jsonschema:"branch to track when building from a repo (default main)"`
	RootDir     string            `json:"rootDir,omitempty" jsonschema:"subdirectory of the repo to build from, for monorepos (default the repo root)"`
	PublishPath string            `json:"publishPath" jsonschema:"the built output directory to serve as the site root, e.g. dist, build, or public"`
	EnvVars     []envVarArg       `json:"envVars,omitempty" jsonschema:"literal (non-secret) build-time environment variables"`
	Domains     []string          `json:"domains,omitempty" jsonschema:"custom domains to serve the site at, in addition to the platform hostname"`
	Routes      []staticRouteArg  `json:"routes,omitempty" jsonschema:"ordered redirect/rewrite rules (first match wins), e.g. an SPA fallback rewrite of /* to /index.html"`
	Headers     []staticHeaderArg `json:"headers,omitempty" jsonschema:"custom response-header rules scoped by request path"`
}

func (a createStaticSiteArgs) toCreateRequest() CreateRequest {
	return CreateRequest{
		OwnerID:     a.OwnerID,
		Name:        a.Name,
		Type:        appv1alpha1.TypeStaticSite,
		Repo:        a.Repo,
		Image:       a.Image,
		Branch:      a.Branch,
		RootDir:     a.RootDir,
		PublishPath: a.PublishPath,
		Env:         toEnvVars(a.EnvVars),
		Hosts:       a.Domains,
		Routes:      routeArgViews(a.Routes),
		Headers:     headerArgViews(a.Headers),
	}
}

// routesArgs / headersArgs / publishPathArgs are the static-site edge-rule tool
// inputs; the set tools replace the whole list (Render's bulk update).
type routesArgs struct {
	ServiceID string           `json:"serviceId" jsonschema:"the static site id (bex App name), as returned by list_services"`
	Routes    []staticRouteArg `json:"routes" jsonschema:"the full ordered list of redirect/rewrite rules to set (replaces the existing routes)"`
}

type headersArgs struct {
	ServiceID string            `json:"serviceId" jsonschema:"the static site id (bex App name), as returned by list_services"`
	Headers   []staticHeaderArg `json:"headers" jsonschema:"the full list of custom response-header rules to set (replaces the existing headers)"`
}

type publishPathArgs struct {
	ServiceID   string `json:"serviceId" jsonschema:"the static site id (bex App name), as returned by list_services"`
	PublishPath string `json:"publishPath" jsonschema:"the built output directory to serve as the site root, e.g. dist"`
}

// routesResult / headersResult wrap the arrays — MCP tool outputs must be objects.
type routesResult struct {
	Routes []renderRoute `json:"routes"`
}

type headersResult struct {
	Headers []renderHeader `json:"headers"`
}

func routeArgViews(in []staticRouteArg) []StaticRouteView {
	if len(in) == 0 {
		return nil
	}
	out := make([]StaticRouteView, len(in))
	for i, r := range in {
		out[i] = StaticRouteView{Type: r.Type, Source: r.Source, Destination: r.Destination}
	}
	return out
}

func headerArgViews(in []staticHeaderArg) []StaticHeaderView {
	if len(in) == 0 {
		return nil
	}
	out := make([]StaticHeaderView, len(in))
	for i, h := range in {
		out[i] = StaticHeaderView{Path: h.Path, Name: h.Name, Value: h.Value}
	}
	return out
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
		Description: "List all services (bex Apps) in the workspace with their status. Scoped to ownerId if given, else to the session's selected workspace (select_workspace), else unscoped.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listServicesArgs) (*mcp.CallToolResult, listServicesResult, error) {
		apps, err := s.List(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID))
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
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createWebServiceArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_cron_job",
		Description: "Create (or update) a cron job that runs a repo/image's command on a schedule, and get back the service. Calling it again for the same name redeploys it. Tracks Render's MCP tool.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createCronJobArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		app, err := s.Create(ctx, in.toCreateRequest())
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_static_site",
		Description: "Create (or update) a static site: build a repo and serve its publishPath output from the object-store origin (no running container). Redirects/rewrites (routes) and custom response headers apply at the edge. Calling it again for the same name republishes it. Tracks Render's MCP tool.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createStaticSiteArgs) (*mcp.CallToolResult, renderService, error) {
		in.OwnerID = core.SelectedWorkspace(s.Selections, req, in.OwnerID)
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
		Name:        "update_cron_job",
		Description: "Change a cron job's schedule and/or command. Render ships a non-functional stub for this tool; bex makes it real. schedule is the 5-field crontab expression (required); command overrides the image's entrypoint (optional — omit to keep the existing override, empty to clear it).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateCronJobArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetCronJob(ctx, in.ServiceID, &in.Schedule, in.Command)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "deploy",
		Description: "Deploy a project from a git repo and its bex.yml in one call — takes code to a live https URL. Calling it again for the same service redeploys it. bex extension (pillar 4, deploy-from-chat).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deployArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.Deploy(ctx, DeployRequest{
			OwnerID:  core.SelectedWorkspace(s.Selections, req, in.OwnerID),
			Repo:     in.Repo,
			Branch:   in.Branch,
			Manifest: in.BexYAML,
		})
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_service",
		Description: "Delete a service permanently, cascading everything the operator derived from it (Deployment, Service, Ingress). This is irreversible. bex extension over Render's MCP (Render's official server ships no delete tool), named after the REST delete verb.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.Delete(ctx, in.ServiceID)
		return nil, deletedResult{Deleted: err == nil}, err
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_root_directory",
		Description: "Set a build-from-git service's Root Directory: the subdirectory of its repo to build from (monorepo support). Triggers a fresh build scoped to that subdirectory. Tracks Render's Root Directory setting.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in rootDirArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetRootDir(ctx, in.ServiceID, in.RootDir)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
	})

	// Autoscaling tools — tracking Render's PUT/DELETE .../autoscaling contract.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_autoscaling",
		Description: "Get the autoscaling configuration for a service (minInstances, maxInstances, targetCPUPercent, targetMemoryPercent). Returns enabled:false when autoscaling is not configured. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, AutoscalingView, error) {
		av, err := s.GetAutoscaling(ctx, in.ServiceID)
		if err != nil {
			return nil, AutoscalingView{}, err
		}
		return nil, av, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_autoscaling",
		Description: "Enable or update autoscaling for a service. The operator adjusts replicas within [minInstances, maxInstances] to hold the target CPU and/or memory utilization. Tracks Render's PUT /v1/services/{id}/autoscaling.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in autoscalingArgs) (*mcp.CallToolResult, AutoscalingView, error) {
		av, err := s.SetAutoscaling(ctx, in.ServiceID, SetAutoscalingRequest{
			MinInstances:        in.MinInstances,
			MaxInstances:        in.MaxInstances,
			TargetCPUPercent:    in.TargetCPUPercent,
			TargetMemoryPercent: in.TargetMemoryPercent,
		})
		if err != nil {
			return nil, AutoscalingView{}, err
		}
		return nil, av, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "disable_autoscaling",
		Description: "Disable autoscaling for a service, reverting it to its fixed spec.replicas count. Tracks Render's DELETE /v1/services/{id}/autoscaling.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, deletedResult, error) {
		err := s.DeleteAutoscaling(ctx, in.ServiceID)
		return nil, deletedResult{Deleted: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_auto_deploy",
		Description: "Turn a service's Auto-Deploy on or off: whether a signed git push to its tracked branch redeploys it (Render's Auto-Deploy toggle). Off leaves only explicit deploys. Does not itself redeploy.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in autoDeployArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetAutoDeploy(ctx, in.ServiceID, in.Enabled)
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

	// Static-site edge-rule tools. Render's official MCP ships only a
	// non-functional update_static_site stub; bex makes routes/headers/publishPath
	// real, delegating to the same Service verbs REST/GraphQL use.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_static_routes",
		Description: "List a static site's redirect/rewrite rules (in order, first match wins).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, routesResult, error) {
		routes, err := s.ListRoutes(ctx, in.ServiceID)
		if err != nil {
			return nil, routesResult{}, err
		}
		return nil, routesResult{Routes: toRenderRoutes(routes)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_static_routes",
		Description: "Replace a static site's redirect/rewrite rules with the given ordered list (Render's routes). The change takes effect without a rebuild. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in routesArgs) (*mcp.CallToolResult, routesResult, error) {
		app, err := s.SetRoutes(ctx, in.ServiceID, routeArgViews(in.Routes))
		if err != nil {
			return nil, routesResult{}, err
		}
		return nil, routesResult{Routes: toRenderRoutes(app.Routes)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_static_headers",
		Description: "List a static site's custom response-header rules.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceArgs) (*mcp.CallToolResult, headersResult, error) {
		headers, err := s.ListHeaders(ctx, in.ServiceID)
		if err != nil {
			return nil, headersResult{}, err
		}
		return nil, headersResult{Headers: toRenderHeaders(headers)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_static_headers",
		Description: "Replace a static site's custom response-header rules with the given list (Render's headers). The change takes effect without a rebuild. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in headersArgs) (*mcp.CallToolResult, headersResult, error) {
		app, err := s.SetHeaders(ctx, in.ServiceID, headerArgViews(in.Headers))
		if err != nil {
			return nil, headersResult{}, err
		}
		return nil, headersResult{Headers: toRenderHeaders(app.Headers)}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_publish_path",
		Description: "Change the built output directory a static site serves (its publishPath) and republish. Rejected for a non-static-site service.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in publishPathArgs) (*mcp.CallToolResult, renderService, error) {
		app, err := s.SetPublishPath(ctx, in.ServiceID, in.PublishPath)
		if err != nil {
			return nil, renderService{}, err
		}
		return nil, toRenderService(app), nil
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
