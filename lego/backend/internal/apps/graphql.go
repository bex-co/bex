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
	"fmt"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// graphql.go is the GraphQL fragment, matching the operation names Render's
// dashboard uses (captured live): query server(id) / services; mutations
// suspendService(id) / resumeService(id) / restartServer(id); type Service with
// the string `suspended` enum. Every resolver delegates to the Service — the
// schema is presentation, the behavior is shared with REST and MCP.

// autoscalingGQLType renders AutoscalingView — the per-service autoscaling
// config backed by spec.autoscaling (Render's Scaling tab shape).
var autoscalingGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Autoscaling",
	Fields: graphql.Fields{
		"enabled":             &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(a AutoscalingView) any { return a.Enabled })},
		"minInstances":        &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AutoscalingView) any { return a.MinInstances })},
		"maxInstances":        &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AutoscalingView) any { return a.MaxInstances })},
		"targetCPUPercent":    &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AutoscalingView) any { return a.TargetCPUPercent })},
		"targetMemoryPercent": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AutoscalingView) any { return a.TargetMemoryPercent })},
	},
})

// staticRouteGQLType / staticHeaderGQLType render a static_site's edge rules
// (Render's route and header shapes); the *Input variants are their mutation
// inputs (setRoutes/setHeaders and createService).
var staticRouteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "StaticRoute",
	Fields: graphql.Fields{
		"type":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r StaticRouteView) any { return r.Type })},
		"source":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r StaticRouteView) any { return r.Source })},
		"destination": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r StaticRouteView) any { return r.Destination })},
	},
})

var staticHeaderGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "StaticHeader",
	Fields: graphql.Fields{
		"path":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(h StaticHeaderView) any { return h.Path })},
		"name":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(h StaticHeaderView) any { return h.Name })},
		"value": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(h StaticHeaderView) any { return h.Value })},
	},
})

var staticRouteInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "StaticRouteInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"source":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"destination": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

var staticHeaderInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "StaticHeaderInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"path":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"name":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// buildFilterGQLType renders Render's Build Filters object (spec.buildFilter);
// buildFilterInputType is its mutation input (createService + setBuildFilter).
var buildFilterGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildFilter",
	Fields: graphql.Fields{
		"paths":        &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(f BuildFilterView) any { return f.Paths })},
		"ignoredPaths": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(f BuildFilterView) any { return f.IgnoredPaths })},
	},
})

var buildFilterInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BuildFilterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"paths":        &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
		"ignoredPaths": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.String)},
	},
})

var secretFileInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SecretFileInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"content": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// maintenanceModeGQLType renders Render's maintenanceMode object
// (spec.maintenanceMode): {enabled, uri}; maintenanceModeInputType is its
// mutation input (createService + setMaintenanceMode). Unlike BuildFilter,
// never null — MaintenanceModeView is a value type that's always present
// (zero value == disabled), matching docs/render-artifacts/maintenance-mode.md.
var maintenanceModeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "MaintenanceMode",
	Fields: graphql.Fields{
		"enabled": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(m MaintenanceModeView) any { return m.Enabled })},
		"uri":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(m MaintenanceModeView) any { return m.URI })},
	},
})

var maintenanceModeInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MaintenanceModeInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"enabled": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		"uri":     &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// gqlMaintenanceModeInput parses a MaintenanceModeInput argument into the
// neutral view. Returns nil when the argument is absent, so create leaves
// maintenanceMode unset (disabled) — same absence convention as
// gqlBuildFilterInput.
func gqlMaintenanceModeInput(args map[string]any, key string) *MaintenanceModeView {
	m, ok := args[key].(map[string]any)
	if !ok {
		return nil
	}
	enabled, _ := m["enabled"].(bool)
	return &MaintenanceModeView{Enabled: enabled, URI: gqlutil.Str(m, "uri")}
}

// gqlBuildFilterInput parses a BuildFilterInput argument into the neutral view
// (graphql-go delivers the input object as map[string]any and each [String] list
// as []any of strings). Returns nil when the argument is absent, so create leaves
// the filter unset.
func gqlBuildFilterInput(args map[string]any, key string) *BuildFilterView {
	m, ok := args[key].(map[string]any)
	if !ok {
		return nil
	}
	return &BuildFilterView{
		Paths:        gqlutil.StringList(m["paths"]),
		IgnoredPaths: gqlutil.StringList(m["ignoredPaths"]),
	}
}

// gqlRouteInputs / gqlHeaderInputs / gqlEnvVarInputs parse list arguments of
// input objects (graphql-go delivers each element as map[string]any).
func gqlRouteInputs(args map[string]any, key string) []StaticRouteView {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]StaticRouteView, 0, len(raw))
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, StaticRouteView{
			Type:        gqlutil.Str(m, "type"),
			Source:      gqlutil.Str(m, "source"),
			Destination: gqlutil.Str(m, "destination"),
		})
	}
	return out
}

func gqlHeaderInputs(args map[string]any, key string) []StaticHeaderView {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]StaticHeaderView, 0, len(raw))
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, StaticHeaderView{
			Path:  gqlutil.Str(m, "path"),
			Name:  gqlutil.Str(m, "name"),
			Value: gqlutil.Str(m, "value"),
		})
	}
	return out
}

func gqlSecretFileInputs(args map[string]any, key string) []core.SecretFile {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]core.SecretFile, 0, len(raw))
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, core.SecretFile{Name: gqlutil.Str(m, "name"), Content: gqlutil.Str(m, "content")})
	}
	return out
}

func gqlEnvVarInputs(args map[string]any, key string) ([]appv1alpha1.EnvVar, error) {
	raw, ok := args[key].([]any)
	if !ok {
		return nil, nil
	}
	out := make([]appv1alpha1.EnvVar, 0, len(raw))
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		name := gqlutil.Str(m, "key")
		value := gqlutil.Str(m, "value")
		generate, _ := m["generateValue"].(bool)
		if generate && value != "" {
			return nil, fmt.Errorf("%w: env var %q sets both value and generateValue — pick one", core.ErrBadRequest, name)
		}
		if generate {
			var err error
			value, err = core.GenerateValue()
			if err != nil {
				return nil, err
			}
		}
		out = append(out, appv1alpha1.EnvVar{Name: name, Value: value})
	}
	return out, nil
}

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// id is the minted Render-shaped srv-… id (w1/m46, closing ADR020's
		// GraphQL name-as-id deviation) — the same value REST/MCP serve; legacy
		// hand-applied CRs without LabelAppID fall back to the name (publicID).
		// Routing accepts BOTH shapes: every verb funnels through
		// core.Base.AuthorizeApp/GetApp, which resolve LabelAppID first and fall
		// back to LabelServiceName, so pre-flip name URLs keep working.
		"id":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.ID })},
		"name": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		// slug is the globally-unique platform-host segment (w4/m19/w4/m20/t002) —
		// distinct from name, which is only workspace-unique.
		"slug":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Slug })},
		"displayName": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.DisplayName })},
		"type":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Type })},
		"suspended":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return core.SuspendedEnum(a.Suspended) })},
		// suspenders lists WHO suspended the service (Render's array; w4/014):
		// ["user"] while suspended — the suspend verb is bex's only suspend
		// path — and [] otherwise.
		"suspenders":   &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(a AppView) any { return suspenders(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.DashboardURL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.CreatedAt })},
		"updatedAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.UpdatedAt })},
		"region":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Region })},
		"sshAddress":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.SSHAddress })},
		// bex-native extras.
		"phase":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Phase })},
		"replicas": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AppView) any { return a.Replicas })},
		"revision": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Revision })},
		// idleTTLSeconds is the free-tier auto-sleep window (bex extension, no
		// Render counterpart); the Settings tab reads it and setIdleTimeout writes it.
		"idleTTLSeconds": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AppView) any { return a.IdleTTLSeconds })},
		// Env vars are nested under the service, Render-dashboard-shaped (captured
		// live: Render's `serviceEnvVarKeys` reads `service{ envVarKeys{ id key } }`):
		// envVarKeys lists keys only; envVar(key) fetches one variable's value on
		// demand ("Show secret"). Resolved through the core.EnvVarReader the root
		// injects — the secrets feature, reached via context so this shared type
		// stays stateless (no import of secrets, no per-server closure).
		"envVarKeys": &graphql.Field{
			Type:    graphql.NewList(envVarGQLType),
			Resolve: envVarKeysResolve,
		},
		"envVar": &graphql.Field{
			Type: envVarGQLType,
			Args: graphql.FieldConfigArgument{
				"key": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: envVarValueResolve,
		},
		// Secret files nest under the service the same way env vars do (through the
		// core.SecretFileReader the root injects): secretFileNames lists names only;
		// secretFile(name) fetches one file's contents on demand.
		"secretFileNames": &graphql.Field{
			Type:    graphql.NewList(secretFileGQLType),
			Resolve: secretFileNamesResolve,
		},
		"secretFile": &graphql.Field{
			Type: secretFileGQLType,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: secretFileContentResolve,
		},
		// plan is a bex extension (not yet captured live from Render's dashboard
		// traffic — the instance-type field/mutation naming there is unconfirmed);
		// it follows the existing suspendService/resumeService/restartServer
		// convention rather than inventing a different shape.
		"plan": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Plan })},
		// schedule + command + runs describe a cron_job (empty/null for other
		// types): the cron's schedule, its entrypoint override, and its recent
		// run history (status.runs, newest first).
		"schedule": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Schedule })},
		"command":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Command })},
		"runs": &graphql.Field{
			Type:    graphql.NewList(cronRunGQLType),
			Resolve: gqlutil.Field(func(a AppView) any { return a.Runs }),
		},
		"lastSuccessfulRunAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.LastSuccessfulRunAt })},
		// ownerId mirrors Render's REST/MCP workspace-scoping field (w6/m2/t004).
		"ownerId":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.OwnerID })},
		"projectId":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.ProjectID })},
		"environmentId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.EnvironmentID })},
		// rootDir is the subdirectory of the repo this App builds from (Render's
		// Root Directory setting, monorepo support); empty is the repo root.
		"rootDir":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.RootDir })},
		"runtime":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Runtime })},
		"buildCommand": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.BuildCommand })},
		"startCommand": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.StartCommand })},
		// dockerfilePath is Render's Dockerfile Path setting, relative to rootDir;
		// applies only when runtime is docker.
		"dockerfilePath": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.DockerfilePath })},
		"builder":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Builder })},
		// repo/branch are the build-from-git source, empty for an image-backed App.
		"repo":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Repo })},
		"branch": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Branch })},
		"registryCredentialId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any {
			if a.RegistryCredentialID == nil {
				return nil
			}
			return *a.RegistryCredentialID
		})},
		// buildFilter is Render's Build Filters (spec.buildFilter): the glob
		// patterns gating git-push auto-deploys. Null when unset.
		"buildFilter": &graphql.Field{
			Type: buildFilterGQLType,
			Resolve: gqlutil.Field(func(a AppView) any {
				if a.BuildFilter == nil {
					return nil
				}
				return *a.BuildFilter
			}),
		},
		"autoDeploy": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(a AppView) any { return a.AutoDeploy })},
		// notifyOnFail is Render's per-service deploy-failure notification
		// override (default | notify | ignore, docs/render-artifacts/
		// notify-on-fail.md); the Settings → Notifications section reads it and
		// writes it via setNotifyOnFail.
		"notifyOnFail":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.NotifyOnFail })},
		"notificationsToSend": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.NotificationsToSend })},
		// renderSubdomainPolicy is Render's field controlling whether the platform
		// subdomain <slug>.onbex.co is active (enabled|disabled, w7/m31). The
		// Settings → Custom Domains section reads it and writes it via
		// setSubdomainPolicy.
		"renderSubdomainPolicy": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.RenderSubdomainPolicy })},
		"healthCheckPath":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.HealthCheckPath })},
		"maxShutdownDelaySeconds": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AppView) any {
			if a.MaxShutdownDelaySeconds == 0 {
				return nil
			}
			return a.MaxShutdownDelaySeconds
		})},
		// preDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand);
		// the Settings → Build & Deploy section reads it and writes via
		// setPreDeployCommand (w1/m33).
		"preDeployCommand": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.PreDeployCommand })},
		// autoscaling is the per-service autoscaling config (Render's Scaling tab).
		// Null when spec.autoscaling is unset (autoscaling never configured).
		"autoscaling": &graphql.Field{
			Type: autoscalingGQLType,
			Resolve: gqlutil.Field(func(a AppView) any {
				if a.Autoscaling == nil {
					return nil
				}
				return *a.Autoscaling
			}),
		},
		// publishPath/routes/headers describe a static_site (empty/null for other
		// types): the served output directory and its edge rules.
		"publishPath": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.PublishPath })},
		"routes": &graphql.Field{
			Type:    graphql.NewList(staticRouteGQLType),
			Resolve: gqlutil.Field(func(a AppView) any { return a.Routes }),
		},
		"headers": &graphql.Field{
			Type:    graphql.NewList(staticHeaderGQLType),
			Resolve: gqlutil.Field(func(a AppView) any { return a.Headers }),
		},
		// ipAllowList is Render's inbound CIDR allowlist for web_service and
		// static_site. Empty/nil means open to all source IPs (Render's default).
		// The dashboard's Networking section reads it and writes it via
		// setServiceIpAllowList.
		"ipAllowList": &graphql.Field{
			Type:    graphql.NewList(graphql.String),
			Resolve: gqlutil.Field(func(a AppView) any { return a.IPAllowList }),
		},
		// maintenanceMode is Render's maintenanceMode object (web_service only;
		// every other type reports the zero value {enabled:false, uri:""}). The
		// Settings → Maintenance Mode section reads it and writes it via
		// setMaintenanceMode.
		"maintenanceMode": &graphql.Field{
			Type:    graphql.NewNonNull(maintenanceModeGQLType),
			Resolve: gqlutil.Field(func(a AppView) any { return a.MaintenanceMode }),
		},
		// latestDeployId is the id of the first deploy row, populated on Create
		// only (w3/m14). The dashboard uses it to navigate straight to the
		// in-flight deploy page after a git-sourced service is created.
		"latestDeployId": &graphql.Field{
			Type:    graphql.String,
			Resolve: gqlutil.Field(func(a AppView) any { return a.LatestDeployID }),
		},
	},
})

// cronRunGQLType renders one CronRunView — a cron_job's execution history entry.
var cronRunGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CronRun",
	Fields: graphql.Fields{
		"id":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r CronRunView) any { return r.ID })},
		"name":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r CronRunView) any { return r.Name })},
		"startedAt":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r CronRunView) any { return r.StartedAt })},
		"finishedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r CronRunView) any { return r.FinishedAt })},
		"status":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r CronRunView) any { return r.Status })},
	},
})

// instanceTypeGQLType renders InstanceType — the plan picker's data source.
// A bex extension (see InstanceType's doc comment): Render's dashboard has no
// public instanceTypes query to mirror, so this is REST/MCP-free by design,
// recorded in w5/m7's README rather than left silently asymmetric.
var instanceTypeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "InstanceType",
	Fields: graphql.Fields{
		"id":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t InstanceType) any { return t.ID })},
		"name":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t InstanceType) any { return t.Name })},
		"cpu":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t InstanceType) any { return t.CPU })},
		"memory": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t InstanceType) any { return t.Memory })},
	},
})

// serviceInstanceGQLType backs serviceInstances — Render's per-service instance
// list ({id, createdAt}), the source for the Web Shell instance picker (w2/m55)
// and Render's own instance-selection UX. Mirrors REST GET /v1/services/{id}/instances.
var serviceInstanceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceInstance",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v ServiceInstanceView) any { return v.ID })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v ServiceInstanceView) any { return v.CreatedAt.UTC().Format(time.RFC3339) })},
	},
})

// shellSessionGQLType backs createShellSession — the Browser Web Shell exec
// ticket the dashboard terminal opens the gateway WebSocket with
// (docs/ADR035-ssh.md § Browser Web Shell). bex extension over Render's GraphQL.
var shellSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ShellSession",
	Fields: graphql.Fields{
		"ticket":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v ShellSessionView) any { return v.Ticket })},
		"url":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v ShellSessionView) any { return v.URL })},
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v ShellSessionView) any { return v.ExpiresAt })},
	},
})

// nameAvailabilityGQLType backs serviceNameAvailable — the create form's
// debounced availability check (w4/m19), a bex extension (Render has no
// public availability API, docs/render-artifacts/duplicate-service-names.md).
var nameAvailabilityGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "NameAvailability",
	Fields: graphql.Fields{
		"available":  &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(a NameAvailability) any { return a.Available })},
		"suggestion": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a NameAvailability) any { return a.Suggestion })},
	},
})

// envVarGQLType renders the kernel's neutral core.EnvVar ({id,key,value}), the
// object Render's dashboard nests under a service. bex has no separate id (the
// key is unique within a service), so id == key; the keys-only list leaves value
// empty (values are fetched per-key via envVar(key)).
var envVarGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvVar",
	Fields: graphql.Fields{
		"id":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v core.EnvVar) any { return v.ID })},
		"key":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v core.EnvVar) any { return v.Key })},
		"value": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v core.EnvVar) any { return v.Value })},
	},
})

// gqlInt reads an optional argument, tolerating absence (graphql-go omits
// unset optional args from the map) — the create mutation's scalar args are
// all optional except name.
func gqlInt(args map[string]any, key string) int {
	if v, ok := args[key].(int); ok {
		return v
	}
	return 0
}

func gqlInt32Ptr(args map[string]any, key string) *int32 {
	v, ok := args[key].(int)
	if !ok {
		return nil
	}
	value := int32(v)
	return &value
}

// customDomainGQLType renders a DomainView as Render's CustomDomain shape.
// Field names match Render's dashboard operations for custom domains.
// dnsRecordGQLType renders a DNSRecordView — the DNS record a tenant creates to
// point a custom domain at the service (Render's post-add DNS instructions).
var dnsRecordGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DNSRecord",
	Fields: graphql.Fields{
		"type":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r DNSRecordView) any { return r.Type })},
		"name":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r DNSRecordView) any { return r.Name })},
		"value": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r DNSRecordView) any { return r.Value })},
	},
})

var customDomainGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CustomDomain",
	Fields: graphql.Fields{
		"id":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any { return d.Name })},
		"name":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any { return d.Name })},
		"domainType":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any { return d.DomainType })},
		"verificationStatus": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any { return d.VerificationStatus })},
		"serverStatus":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any { return d.ServerStatus })},
		"redirectForName": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any {
			if d.RedirectForName == "" {
				return nil
			}
			return d.RedirectForName
		})},
		// dnsRecord is the record the tenant must create (bex extension; the target is
		// the app's platform host <app>.<base-domain>).
		"dnsRecord": &graphql.Field{Type: dnsRecordGQLType, Resolve: gqlutil.Field(func(d DomainView) any { return d.DNSRecord })},
	},
})

func envVarKeysResolve(p graphql.ResolveParams) (any, error) {
	a, ok := p.Source.(AppView)
	if !ok {
		return nil, nil
	}
	r, ok := core.EnvVarsFrom(p.Context)
	if !ok {
		return nil, core.ErrSecretsUnavailable
	}
	return r.EnvVarKeys(p.Context, a.Name)
}

func envVarValueResolve(p graphql.ResolveParams) (any, error) {
	a, ok := p.Source.(AppView)
	if !ok {
		return nil, nil
	}
	r, ok := core.EnvVarsFrom(p.Context)
	if !ok {
		return nil, core.ErrSecretsUnavailable
	}
	return r.EnvVarValue(p.Context, a.Name, p.Args["key"].(string))
}

// secretFileGQLType renders the kernel's neutral core.SecretFile ({id,name,content}),
// nested under a service like env vars. id == name; the names-only list leaves
// content empty (fetched per-file via secretFile(name)).
var secretFileGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SecretFile",
	Fields: graphql.Fields{
		"id":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f core.SecretFile) any { return f.ID })},
		"name":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f core.SecretFile) any { return f.Name })},
		"content": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f core.SecretFile) any { return f.Content })},
	},
})

func secretFileNamesResolve(p graphql.ResolveParams) (any, error) {
	a, ok := p.Source.(AppView)
	if !ok {
		return nil, nil
	}
	r, ok := core.SecretFilesFrom(p.Context)
	if !ok {
		return nil, core.ErrSecretsUnavailable
	}
	return r.SecretFileNames(p.Context, a.Name)
}

func secretFileContentResolve(p graphql.ResolveParams) (any, error) {
	a, ok := p.Source.(AppView)
	if !ok {
		return nil, nil
	}
	r, ok := core.SecretFilesFrom(p.Context)
	if !ok {
		return nil, core.ErrSecretsUnavailable
	}
	return r.SecretFileContent(p.Context, a.Name, p.Args["name"].(string))
}

// blueprintGQLType is the GraphQL shape for a BlueprintView (w2/m15).
var blueprintGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Blueprint",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.Name })},
		"repo":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.Repo })},
		"branch":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.Branch })},
		"manifest":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.Manifest })},
		"status":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.Status })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.CreatedAt })},
		"updatedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(b BlueprintView) any { return b.UpdatedAt })},
	},
})

var blueprintValidationErrorGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintValidationError",
	Fields: graphql.Fields{
		"error":  &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(e BlueprintValidationError) any { return e.Error })},
		"line":   &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(e BlueprintValidationError) any { return e.Line })},
		"column": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(e BlueprintValidationError) any { return e.Column })},
		"path":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e BlueprintValidationError) any { return e.Path })},
	},
})

var blueprintValidationPlanGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintValidationPlan",
	Fields: graphql.Fields{
		"services":     &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(p BlueprintValidationPlan) any { return p.Services })},
		"databases":    &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(p BlueprintValidationPlan) any { return p.Databases })},
		"keyValue":     &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(p BlueprintValidationPlan) any { return p.KeyValue })},
		"envGroups":    &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(p BlueprintValidationPlan) any { return p.EnvGroups })},
		"totalActions": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(p BlueprintValidationPlan) any { return p.TotalActions })},
	},
})

// blueprintValidationGQLType preserves the dashboard's original errors:
// [String] field while also exposing Render's structured error details and plan.
var blueprintValidationGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintValidation",
	Fields: graphql.Fields{
		"valid": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v BlueprintValidation) any { return v.Valid })},
		"errors": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v BlueprintValidation) any {
			messages := make([]string, len(v.Errors))
			for i, validationErr := range v.Errors {
				messages[i] = validationErr.Error
			}
			return messages
		})},
		"errorDetails": &graphql.Field{Type: graphql.NewList(blueprintValidationErrorGQLType), Resolve: gqlutil.Field(func(v BlueprintValidation) any { return v.Errors })},
		"plan": &graphql.Field{Type: blueprintValidationPlanGQLType, Resolve: gqlutil.Field(func(v BlueprintValidation) any {
			if v.Plan == nil {
				return nil
			}
			return *v.Plan
		})},
	},
})

// syncBlueprintResultGQLType is the GraphQL shape for SyncBlueprintResult.
var syncBlueprintResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SyncBlueprintResult",
	Fields: graphql.Fields{
		"blueprint": &graphql.Field{Type: blueprintGQLType, Resolve: gqlutil.Field(func(r SyncBlueprintResult) any { return r.Blueprint })},
		// services and databases from the stack apply — summary only (poll via server/postgres for full state).
		"services": &graphql.Field{Type: graphql.NewList(serviceGQLType), Resolve: gqlutil.Field(func(r SyncBlueprintResult) any { return r.Stack.Services })},
		"databases": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(r SyncBlueprintResult) any {
			names := make([]string, len(r.Stack.Databases))
			for i, d := range r.Stack.Databases {
				names[i] = d.Name
			}
			return names
		})},
	},
})

// GraphQLQuery returns the App read fields (Render dashboard names services /
// server(id)) for the composition root to merge into the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		// autoscalingConfig: read the autoscaling config for a specific service by id.
		// A bex extension (Render exposes autoscaling only via REST PUT/DELETE).
		"autoscalingConfig": &graphql.Field{
			Type: autoscalingGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				av, err := s.GetAutoscaling(p.Context, p.Args["id"].(string))
				if err != nil {
					return nil, err
				}
				return av, nil
			},
		},
		"services": &graphql.Field{
			Type: graphql.NewList(serviceGQLType),
			Args: graphql.FieldConfigArgument{
				// ownerId mirrors Render's REST/MCP services list filter (w6/m2/t004).
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
				"cursor":  &graphql.ArgumentConfig{Type: graphql.String},
				"limit":   &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				out, err := s.List(p.Context, gqlutil.Str(p.Args, "ownerId"))
				if err != nil {
					return nil, err
				}
				cursor, cursorSet := p.Args["cursor"].(string)
				limit, limitSet := p.Args["limit"].(int)
				if !limitSet {
					limit = core.DefaultPageLimit
				} else {
					limit = core.PageLimit(limit)
				}
				return core.StablePage(out, cursor, limit, cursorSet || limitSet, func(a AppView) string { return a.ID }), nil
			},
		},
		"server": &graphql.Field{ // Render's dashboard query name
			Type: serviceGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
		"service": &graphql.Field{ // Render's dashboard also queries service(id) (e.g. serviceEnvVarKeys)
			Type: serviceGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
		// First-class cron run reads (bex extensions over Render's current public
		// API, which only exposes trigger/cancel-current). Both delegate to the
		// same status.runs verbs REST/MCP use.
		"cronJobRuns": &graphql.Field{
			Type: graphql.NewList(cronRunGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListCronRuns(p.Context, p.Args["serviceId"].(string), gqlutil.Str(p.Args, "cursor"), gqlInt(p.Args, "limit"))
			},
		},
		"cronJobRun": &graphql.Field{
			Type: cronRunGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"runId":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetCronRun(p.Context, p.Args["serviceId"].(string), p.Args["runId"].(string))
			},
		},
		"instanceTypes": &graphql.Field{ // bex extension backing the plan picker (see InstanceType)
			Type:    graphql.NewList(instanceTypeGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.InstanceTypes(p.Context) },
		},
		// serviceInstances: Render's per-service instance list ({id, createdAt}) —
		// the source for the Web Shell instance picker (w2/m55). Mirrors REST
		// GET /v1/services/{id}/instances via the same ListInstances verb.
		"serviceInstances": &graphql.Field{
			Type: graphql.NewList(serviceInstanceGQLType),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListInstances(p.Context, p.Args["id"].(string))
			},
		},
		// serviceNameAvailable: the create form's debounced availability check
		// (w4/m19), a bex extension backing the "Name is already in use" +
		// suggestion UX.
		"serviceNameAvailable": &graphql.Field{
			Type: nameAvailabilityGQLType,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.NameAvailable(p.Context, p.Args["name"].(string))
			},
		},
		// Custom domains — Render-dashboard-shaped queries.
		// cursor/limit + verificationStatus/domainType added in w7/m40 to match
		// the REST filters w7/m38 shipped.
		"customDomains": &graphql.Field{
			Type: graphql.NewList(customDomainGQLType),
			Args: graphql.FieldConfigArgument{
				"id":                 &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cursor":             &graphql.ArgumentConfig{Type: graphql.String},
				"limit":              &graphql.ArgumentConfig{Type: graphql.Int},
				"verificationStatus": &graphql.ArgumentConfig{Type: graphql.String},
				"domainType":         &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				out, err := s.ListDomains(p.Context, p.Args["id"].(string))
				if err != nil {
					return nil, err
				}
				if vs := gqlutil.Str(p.Args, "verificationStatus"); vs != "" {
					switch vs {
					case "pending", "verified":
						filtered := make([]DomainView, 0, len(out))
						for _, d := range out {
							if d.VerificationStatus == vs {
								filtered = append(filtered, d)
							}
						}
						out = filtered
					default:
						return nil, fmt.Errorf("%w: unknown verificationStatus %q (want pending|verified)",
							core.ErrBadRequest, vs)
					}
				}
				if dt := gqlutil.Str(p.Args, "domainType"); dt != "" {
					switch dt {
					case "apex", "subdomain":
						filtered := make([]DomainView, 0, len(out))
						for _, d := range out {
							if d.DomainType == dt {
								filtered = append(filtered, d)
							}
						}
						out = filtered
					default:
						return nil, fmt.Errorf("%w: unknown domainType %q (want apex|subdomain)",
							core.ErrBadRequest, dt)
					}
				}
				cursor, cursorSet := p.Args["cursor"].(string)
				limit, limitSet := p.Args["limit"].(int)
				if !limitSet {
					limit = core.DefaultPageLimit
				} else {
					limit = core.PageLimit(limit)
				}
				return core.StablePage(out, cursor, limit, cursorSet || limitSet,
					func(d DomainView) string { return d.Name }), nil
			},
		},
		"customDomain": &graphql.Field{
			Type: customDomainGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetDomain(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		// blueprints: list known bex.yml stack sources for a workspace (w2/m15).
		"blueprints": &graphql.Field{
			Type: graphql.NewList(blueprintGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListBlueprints(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
		// blueprint: fetch a single blueprint by id (w7/m27 — dashboard detail page).
		"blueprint": &graphql.Field{
			Type: blueprintGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetBlueprint(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"))
			},
		},
		// validateBlueprint: dry-run parse a bex.yml — per-entry errors, no apply (w2/m15).
		"validateBlueprint": &graphql.Field{
			Type: blueprintValidationGQLType,
			Args: graphql.FieldConfigArgument{
				"bexYaml": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ValidateBlueprint(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["bexYaml"].(string))
			},
		},
	}
}

// GraphQLMutation returns the lifecycle mutations (Render dashboard names
// suspendService / resumeService / restartServer).
func (s *Service) GraphQLMutation() graphql.Fields {
	// confirmIDArgs is gqlutil.IDArg() plus an optional confirm — used by the
	// verbs w6/m19's protected-environment guard can block (suspendService,
	// deleteService below); confirm is a no-op (never read) for verbs whose
	// Args omit it, since verb reads it with a safe comma-ok assert.
	confirmIDArgs := func() graphql.FieldConfigArgument {
		return graphql.FieldConfigArgument{
			"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"confirm": &graphql.ArgumentConfig{Type: graphql.String},
		}
	}
	verb := func(fn func(context.Context, string) (AppView, error)) graphql.FieldResolveFn {
		return func(p graphql.ResolveParams) (any, error) {
			confirm, _ := p.Args["confirm"].(string)
			ctx := withConfirm(p.Context, confirm)
			return fn(ctx, p.Args["id"].(string))
		}
	}
	return graphql.Fields{
		// createService: create-or-update a service (the create half of the
		// lifecycle). A bex extension — the create mutation's name/shape is not
		// confirmed against a live Render dashboard capture (same caveat as
		// updateServicePlan/scaleService); it follows their scalar-arg convention.
		// One of repo/image is required, the rest fall back to platform defaults.
		"createService": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				// ownerId is the workspace to create IN (w6/m14) — the write-side
				// twin of the services list filter above, and the same optional
				// contract REST's create body has: omitted => the caller's default
				// workspace; a workspace they don't belong to => forbidden.
				"ownerId":              &graphql.ArgumentConfig{Type: graphql.String},
				"environmentId":        &graphql.ArgumentConfig{Type: graphql.String},
				"type":                 &graphql.ArgumentConfig{Type: graphql.String}, // web_service (default) | private_service | background_worker | cron_job
				"schedule":             &graphql.ArgumentConfig{Type: graphql.String}, // cron expression, required when type is cron_job
				"command":              &graphql.ArgumentConfig{Type: graphql.String}, // overrides the image's entrypoint for a cron_job
				"repo":                 &graphql.ArgumentConfig{Type: graphql.String},
				"image":                &graphql.ArgumentConfig{Type: graphql.String},
				"registryCredentialId": &graphql.ArgumentConfig{Type: graphql.String},
				"branch":               &graphql.ArgumentConfig{Type: graphql.String},
				"rootDir":              &graphql.ArgumentConfig{Type: graphql.String},       // subdirectory of repo to build from (monorepo support)
				"buildFilter":          &graphql.ArgumentConfig{Type: buildFilterInputType}, // Render's Build Filters: globs gating push auto-deploys
				"runtime":              &graphql.ArgumentConfig{Type: graphql.String},       // Render runtime: native language | docker | image
				"buildCommand":         &graphql.ArgumentConfig{Type: graphql.String},
				"startCommand":         &graphql.ArgumentConfig{Type: graphql.String},
				// dockerfilePath is Render's Dockerfile Path, relative to rootDir; docker runtime only.
				"dockerfilePath": &graphql.ArgumentConfig{Type: graphql.String},
				"builder":        &graphql.ArgumentConfig{Type: graphql.String}, // auto (default) | buildpack | dockerfile
				"plan":           &graphql.ArgumentConfig{Type: graphql.String},
				"autoDeploy":     &graphql.ArgumentConfig{Type: graphql.Boolean},
				// notifyOnFail is Render's per-service deploy-failure notification
				// override (default | notify | ignore); omitted => "default".
				"notifyOnFail": &graphql.ArgumentConfig{Type: graphql.String},
				"port":         &graphql.ArgumentConfig{Type: graphql.Int},
				"replicas":     &graphql.ArgumentConfig{Type: graphql.Int},
				// envVars sets literal (non-secret) environment variables at create time
				// (Render parity, w5/m19): REST/MCP parity — those surfaces accepted
				// envVars at create since w2/m2; GraphQL now reaches the same shape.
				// envVars: reuses gqlutil.EnvVarInputType (shared with secrets.setEnvVars
				// to avoid duplicate type names in the composed schema).
				"envVars":     &graphql.ArgumentConfig{Type: graphql.NewList(gqlutil.EnvVarInputType)},
				"secretFiles": &graphql.ArgumentConfig{Type: graphql.NewList(secretFileInputType)},
				// static_site create fields.
				"publishPath":             &graphql.ArgumentConfig{Type: graphql.String},
				"routes":                  &graphql.ArgumentConfig{Type: graphql.NewList(staticRouteInputType)},
				"headers":                 &graphql.ArgumentConfig{Type: graphql.NewList(staticHeaderInputType)},
				"healthCheckPath":         &graphql.ArgumentConfig{Type: graphql.String},
				"maxShutdownDelaySeconds": &graphql.ArgumentConfig{Type: graphql.Int},
				// preDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand, w1/m33).
				"preDeployCommand": &graphql.ArgumentConfig{Type: graphql.String},
				// maintenanceMode is Render's maintenanceMode object at create time
				// (w1/m37); web_service only. Omitted => disabled.
				"maintenanceMode": &graphql.ArgumentConfig{Type: maintenanceModeInputType},
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				dryRun, _ := p.Args["dryRun"].(bool)
				env, err := gqlEnvVarInputs(p.Args, "envVars")
				if err != nil {
					return nil, err
				}
				return s.Create(p.Context, CreateRequest{
					OwnerID:                 gqlutil.Str(p.Args, "ownerId"),
					EnvironmentID:           gqlutil.Str(p.Args, "environmentId"),
					Name:                    p.Args["name"].(string),
					Type:                    gqlutil.Str(p.Args, "type"),
					Schedule:                gqlutil.Str(p.Args, "schedule"),
					Command:                 gqlutil.Str(p.Args, "command"),
					Repo:                    gqlutil.Str(p.Args, "repo"),
					Image:                   gqlutil.Str(p.Args, "image"),
					RegistryCredentialID:    gqlutil.StrPtr(p.Args, "registryCredentialId"),
					Branch:                  gqlutil.Str(p.Args, "branch"),
					RootDir:                 gqlutil.Str(p.Args, "rootDir"),
					BuildFilter:             gqlBuildFilterInput(p.Args, "buildFilter"),
					Runtime:                 gqlutil.Str(p.Args, "runtime"),
					BuildCommand:            gqlutil.Str(p.Args, "buildCommand"),
					StartCommand:            gqlutil.Str(p.Args, "startCommand"),
					DockerfilePath:          gqlutil.Str(p.Args, "dockerfilePath"),
					Builder:                 gqlutil.Str(p.Args, "builder"),
					Plan:                    gqlutil.Str(p.Args, "plan"),
					AutoDeploy:              gqlutil.BoolPtr(p.Args, "autoDeploy"),
					NotifyOnFail:            gqlutil.Str(p.Args, "notifyOnFail"),
					Port:                    int32(gqlInt(p.Args, "port")),
					Replicas:                int32(gqlInt(p.Args, "replicas")),
					Env:                     env,
					SecretFiles:             gqlSecretFileInputs(p.Args, "secretFiles"),
					PublishPath:             gqlutil.Str(p.Args, "publishPath"),
					Routes:                  gqlRouteInputs(p.Args, "routes"),
					Headers:                 gqlHeaderInputs(p.Args, "headers"),
					HealthCheckPath:         gqlutil.Str(p.Args, "healthCheckPath"),
					MaxShutdownDelaySeconds: gqlInt32Ptr(p.Args, "maxShutdownDelaySeconds"),
					PreDeployCommand:        gqlutil.Str(p.Args, "preDeployCommand"),
					MaintenanceMode:         gqlMaintenanceModeInput(p.Args, "maintenanceMode"),
					DryRun:                  dryRun,
				})
			},
		},
		// deleteService: delete a service (the delete half of the lifecycle).
		// Returns a success boolean like deleteCustomDomain — there is no service
		// object left to return. A bex extension (Render's dashboard delete
		// mutation name wasn't captured), following the deleteCustomDomain shape.
		"deleteService": &graphql.Field{
			Type: graphql.Boolean,
			// confirm arms a delete blocked by w6/m19's protected-environment
			// guard (apps.ProtectedConfirmation) — a no-op otherwise.
			Args: confirmIDArgs(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				confirm, _ := p.Args["confirm"].(string)
				err := s.Delete(withConfirm(p.Context, confirm), p.Args["id"].(string))
				return err == nil, err
			},
		},
		// updateCronJob: change a cron_job's schedule and/or command (w5/m18).
		// Rejected for a non-cron service (core.ErrBadRequest). schedule is
		// required; command is optional (empty clears the image-entrypoint override).
		"updateCronJob": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"schedule": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"command":  &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				sched := p.Args["schedule"].(string)
				return s.SetCronJob(p.Context, p.Args["id"].(string), &sched, gqlutil.StrPtr(p.Args, "command"))
			},
		},
		// runCronJob returns the deterministic pending run, matching REST's
		// Render-shaped trigger response instead of returning the parent service.
		"runCronJob": &graphql.Field{
			Type: cronRunGQLType, Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.TriggerCronRun(p.Context, p.Args["id"].(string))
			},
		},
		"cancelCronJobRun": &graphql.Field{
			Type: cronRunGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"runId":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CancelCronRun(p.Context, p.Args["serviceId"].(string), p.Args["runId"].(string))
			},
		},
		// confirmIDArgs, not gqlutil.IDArg(): suspend can be blocked by w6/m19's
		// protected-environment guard, armed via the optional confirm arg.
		"suspendService": &graphql.Field{Type: serviceGQLType, Args: confirmIDArgs(), Resolve: verb(s.Suspend)},
		"resumeService":  &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Resume)},
		// restartServer moved to deploys.GraphQLMutation (w2/m30): routing through
		// deploys.Restart ensures every restart opens a deploy-history row.
		// updateServicePlan: a bex extension (naming unconfirmed against a live
		// Render dashboard capture — see the "plan" field comment above).
		"updateServicePlan": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				dryRun, _ := p.Args["dryRun"].(bool)
				if dryRun {
					return s.PreviewSetPlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
				}
				return s.SetPlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
			},
		},
		// setDisplayName relabels a service for humans while leaving its immutable
		// App name/id, platform hostname, and derived Kubernetes resources alone.
		// An empty displayName clears the label and restores the name fallback.
		"setDisplayName": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"displayName": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetDisplayName(p.Context, p.Args["id"].(string), p.Args["displayName"].(string))
			},
		},
		// setRegistryCredential binds an image-backed service or Dockerfile build
		// to one stored workspace credential. Empty clears the binding; the
		// service verb owns the same membership/source checks used by REST/MCP.
		"setRegistryCredential": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"registryCredentialId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRegistryCredential(p.Context, p.Args["id"].(string), p.Args["registryCredentialId"].(string))
			},
		},
		// scaleService: Render's manual-scaling verb. numInstances mirrors the
		// REST scale body field; out-of-range is a GraphQL error (core.ErrBadRequest).
		"scaleService": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"numInstances": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Scale(p.Context, p.Args["id"].(string), int32(p.Args["numInstances"].(int)))
			},
		},
		// createShellSession: mints a Browser Web Shell exec ticket the dashboard
		// terminal opens the gateway WebSocket with (docs/ADR035-ssh.md § Browser
		// Web Shell). bex extension over Render's GraphQL. Optional instanceId pins
		// one Ready replica; omitted selects a random one, matching native SSH.
		// 503 (ErrShellUnavailable) when the browser transport is unconfigured.
		"createShellSession": &graphql.Field{
			Type: shellSessionGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"instanceId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				instanceID, _ := p.Args["instanceId"].(string)
				return s.CreateShellSession(p.Context, p.Args["id"].(string), instanceID)
			},
		},
		// setIdleTimeout: bex extension (no Render counterpart) — sets the free-tier
		// auto-sleep window (spec.idleTTLSeconds; 0 restores the controller default).
		// Out-of-range is a GraphQL error (core.ErrBadRequest).
		"setIdleTimeout": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"idleTTLSeconds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetIdleTTL(p.Context, p.Args["id"].(string), int32(p.Args["idleTTLSeconds"].(int)))
			},
		},
		// setRootDir: the Settings → Build & Deploy save flow (w5/m13) writes
		// Render's Root Directory setting (spec.rootDir) on an existing App
		// (create-time rootDir is handled by createService above). Rejected for
		// an image-backed App (core.ErrBadRequest — nothing to build).
		"setRootDir": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"rootDir": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRootDir(p.Context, p.Args["id"].(string), p.Args["rootDir"].(string))
			},
		},
		// setBuildCommand changes the build command for a repo-backed service.
		// Applies to static_site (the primary user) and native-runtime services.
		// The shared SetCommands verb also backs Render's REST PATCH shape; this
		// scalar setter is the dashboard-friendly GraphQL projection.
		"setBuildCommand": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"command": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				command := p.Args["command"].(string)
				return s.SetCommands(p.Context, p.Args["id"].(string), &command, nil)
			},
		},
		// setStartCommand changes the command used to start an existing service.
		// The shared SetCommands verb also backs Render's REST PATCH shape; this
		// scalar setter is the dashboard-friendly GraphQL projection.
		"setStartCommand": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"command": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				command := p.Args["command"].(string)
				return s.SetCommands(p.Context, p.Args["id"].(string), nil, &command)
			},
		},
		// setDockerfilePath changes Render's Dockerfile Path on an existing
		// repo-backed Docker service. Empty restores the default Dockerfile.
		"setDockerfilePath": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"dockerfilePath": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetDockerfilePath(p.Context, p.Args["id"].(string), p.Args["dockerfilePath"].(string))
			},
		},
		// setBuildFilter: the Settings → Build & Deploy Build Filters rows (w1/m34)
		// write Render's Build Filters (spec.buildFilter) — the glob patterns gating
		// git-push auto-deploys — on an existing App. Passing an all-empty object
		// clears the filter. Rejected for an image-backed App (nothing to build).
		// Follows the scalar/object-arg setter grammar (setRootDir/setStaticRoutes).
		"setBuildFilter": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"buildFilter": &graphql.ArgumentConfig{Type: graphql.NewNonNull(buildFilterInputType)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetBuildFilter(p.Context, p.Args["id"].(string), gqlBuildFilterInput(p.Args, "buildFilter"))
			},
		},
		// setMaintenanceMode: the Settings → Maintenance Mode toggle (w1/m37)
		// writes Render's maintenanceMode object (spec.maintenanceMode) — takes a
		// web service offline behind an interstitial page without touching its
		// pods. Rejected for a non-web_service (core.ErrBadRequest).
		"setMaintenanceMode": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":              &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"maintenanceMode": &graphql.ArgumentConfig{Type: graphql.NewNonNull(maintenanceModeInputType)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				in := gqlMaintenanceModeInput(p.Args, "maintenanceMode")
				return s.SetMaintenanceMode(p.Context, p.Args["id"].(string), *in)
			},
		},
		// setHealthCheckPath: the Settings → Health & Alerts health-check path
		// (w5/009). Changes spec.healthCheckPath — the HTTP path the ReadinessProbe
		// pings (w1/m23/t001). Rejected for cron_job/background_worker (no HTTP port).
		"setHealthCheckPath": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"path": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetHealthCheckPath(p.Context, p.Args["id"].(string), p.Args["path"].(string))
			},
		},
		// setMaxShutdownDelay changes Render's per-service SIGTERM grace window.
		// The shared service verb enforces 1-300 and rejects cron/static services.
		"setMaxShutdownDelay": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"seconds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetMaxShutdownDelay(p.Context, p.Args["id"].(string), int32(p.Args["seconds"].(int)))
			},
		},
		// setPreDeployCommand: the Settings → Build & Deploy Pre-Deploy Command
		// field (w1/m33). Changes spec.preDeployCommand — the command run against
		// the new image before it serves traffic. An empty command clears the
		// step. Rejected for cron_job/static_site (the field doesn't apply).
		"setPreDeployCommand": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"command": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetPreDeployCommand(p.Context, p.Args["id"].(string), p.Args["command"].(string))
			},
		},
		// setAutoDeploy: the Settings → Build & Deploy Auto-Deploy toggle (w2/m9)
		// flips whether a signed git push redeploys the App (spec.autoDeploy). A
		// bex extension name (Render's dashboard mutation is uncaptured), following
		// the scalar-arg convention.
		"setAutoDeploy": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetAutoDeploy(p.Context, p.Args["id"].(string), p.Args["enabled"].(bool))
			},
		},
		// setNotifyOnFail: the Settings → Notifications per-service override
		// (w4/m21, docs/render-artifacts/notify-on-fail.md). Changes
		// spec.notifyOnFail — default | notify | ignore, Render's exact
		// name/enum. Unrecognized value ⇒ core.ErrBadRequest.
		"setNotifyOnFail": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"value": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetNotifyOnFail(p.Context, p.Args["id"].(string), p.Args["value"].(string))
			},
		},
		"setNotificationsToSend": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"value": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetNotificationsToSend(p.Context, p.Args["id"].(string), p.Args["value"].(string))
			},
		},
		// setSubdomainPolicy: the Settings → Custom Domains platform-subdomain
		// toggle (w7/m31). Changes spec.subdomainPolicy — enabled | disabled,
		// Render's exact renderSubdomainPolicy enum. "disabled" without a custom
		// domain ⇒ core.ErrBadRequest (would silently kill the service).
		"setSubdomainPolicy": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"policy": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetSubdomainPolicy(p.Context, p.Args["id"].(string), p.Args["policy"].(string))
			},
		},
		// setServiceIpAllowList replaces the inbound CIDR allowlist for a
		// web_service or static_site. An empty or null cidrs arg clears the
		// allowlist (open to all source IPs, Render's default). Each element
		// must be a valid IPv4 or IPv6 CIDR — an invalid CIDR is 400.
		"setServiceIpAllowList": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cidrs": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				rawCIDRs, _ := p.Args["cidrs"].([]any)
				cidrs := make([]string, 0, len(rawCIDRs))
				for _, c := range rawCIDRs {
					if s, ok := c.(string); ok {
						cidrs = append(cidrs, s)
					}
				}
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), cidrs)
			},
		},
		// Static-site edge-rule mutations: replace the whole routes/headers list
		// (Render's bulk PUT), or change the publish directory. Rejected for a
		// non-static_site (core.ErrBadRequest).
		"setStaticRoutes": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"routes": &graphql.ArgumentConfig{Type: graphql.NewList(staticRouteInputType)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRoutes(p.Context, p.Args["id"].(string), gqlRouteInputs(p.Args, "routes"))
			},
		},
		"setStaticHeaders": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"headers": &graphql.ArgumentConfig{Type: graphql.NewList(staticHeaderInputType)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetHeaders(p.Context, p.Args["id"].(string), gqlHeaderInputs(p.Args, "headers"))
			},
		},
		"setPublishPath": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"publishPath": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetPublishPath(p.Context, p.Args["id"].(string), p.Args["publishPath"].(string))
			},
		},
		// Custom domain mutations — Render-dashboard-shaped operation names.
		"addCustomDomain": &graphql.Field{
			Type: customDomainGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AddDomain(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		"deleteCustomDomain": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteDomain(p.Context, p.Args["id"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},
		// verifyCustomDomain re-checks a domain's DNS/cert state now and returns its
		// fresh status (Render's Verify button / POST …/verify). bex verification is
		// automatic, so this is an idempotent re-read, not a state trigger.
		"verifyCustomDomain": &graphql.Field{
			Type: customDomainGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.VerifyDomain(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		// setAutoscaling: enable/update autoscaling on a service (mirrors Render's
		// PUT /v1/services/{id}/autoscaling). Returns the updated autoscaling config.
		"setAutoscaling": &graphql.Field{
			Type: autoscalingGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"minInstances":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				"maxInstances":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
				"targetCPUPercent":    &graphql.ArgumentConfig{Type: graphql.Int},
				"targetMemoryPercent": &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := SetAutoscalingRequest{
					MinInstances: int32(p.Args["minInstances"].(int)),
					MaxInstances: int32(p.Args["maxInstances"].(int)),
				}
				if v, ok := p.Args["targetCPUPercent"].(int); ok {
					i := int32(v)
					req.TargetCPUPercent = &i
				}
				if v, ok := p.Args["targetMemoryPercent"].(int); ok {
					i := int32(v)
					req.TargetMemoryPercent = &i
				}
				return s.SetAutoscaling(p.Context, p.Args["id"].(string), req)
			},
		},
		// disableAutoscaling: disable autoscaling on a service (mirrors Render's
		// DELETE /v1/services/{id}/autoscaling). Returns true on success.
		"disableAutoscaling": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteAutoscaling(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
		// syncBlueprint: re-apply a stored blueprint idempotently (w2/m15).
		// If bexYaml is provided, the stored manifest is replaced before re-apply.
		"syncBlueprint": &graphql.Field{
			Type: syncBlueprintResultGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"bexYaml": &graphql.ArgumentConfig{Type: graphql.String},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
				"confirm": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SyncBlueprint(p.Context, p.Args["id"].(string),
					gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "bexYaml"), gqlutil.Str(p.Args, "confirm"))
			},
		},
	}
}
