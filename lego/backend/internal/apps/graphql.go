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
	"maps"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
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
		"enabled":             gqlutil.BoolField(func(a AutoscalingView) any { return a.Enabled }),
		"minInstances":        gqlutil.IntField(func(a AutoscalingView) any { return a.MinInstances }),
		"maxInstances":        gqlutil.IntField(func(a AutoscalingView) any { return a.MaxInstances }),
		"targetCPUPercent":    gqlutil.IntField(func(a AutoscalingView) any { return a.TargetCPUPercent }),
		"targetMemoryPercent": gqlutil.IntField(func(a AutoscalingView) any { return a.TargetMemoryPercent }),
	},
})

// staticRouteGQLType / staticHeaderGQLType render a static_site's edge rules
// (Render's route and header shapes); the *Input variants are their mutation
// inputs (setRoutes/setHeaders and createService).
var staticRouteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "StaticRoute",
	Fields: graphql.Fields{
		"type":        gqlutil.StrField(func(r StaticRouteView) any { return r.Type }),
		"source":      gqlutil.StrField(func(r StaticRouteView) any { return r.Source }),
		"destination": gqlutil.StrField(func(r StaticRouteView) any { return r.Destination }),
	},
})

var staticHeaderGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "StaticHeader",
	Fields: graphql.Fields{
		"path":  gqlutil.StrField(func(h StaticHeaderView) any { return h.Path }),
		"name":  gqlutil.StrField(func(h StaticHeaderView) any { return h.Name }),
		"value": gqlutil.StrField(func(h StaticHeaderView) any { return h.Value }),
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
		"paths":        gqlutil.StrsField(func(f BuildFilterView) any { return f.Paths }),
		"ignoredPaths": gqlutil.StrsField(func(f BuildFilterView) any { return f.IgnoredPaths }),
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
		"enabled": gqlutil.ReqBoolField(func(m MaintenanceModeView) any { return m.Enabled }),
		"uri":     gqlutil.ReqStrField(func(m MaintenanceModeView) any { return m.URI }),
	},
})

var maintenanceModeInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "MaintenanceModeInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"enabled": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		"uri":     &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var blueprintEnvVarValueInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BlueprintEnvVarValueInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"key":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
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

// blueprintEnvVarValues decodes GraphQL's list representation into the neutral
// request map used by REST and MCP. A list is necessary because GraphQL has no
// input-map type. Duplicate prompt keys are rejected rather than making a
// secret's winner depend on input ordering.
func blueprintEnvVarValues(raw any) (map[string]string, error) {
	entries, ok := raw.([]any)
	if !ok || len(entries) == 0 {
		return nil, nil
	}
	values := make(map[string]string, len(entries))
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%w: envVarValues entries must contain key and value", core.ErrBadRequest)
		}
		key, _ := entry["key"].(string)
		value, _ := entry["value"].(string)
		if key == "" {
			return nil, fmt.Errorf("%w: envVarValues key is required", core.ErrBadRequest)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("%w: envVarValues contains duplicate key %q", core.ErrBadRequest, key)
		}
		values[key] = value
	}
	return values, nil
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
		"id": gqlutil.StrField(func(a AppView) any { return a.ID }),
		// name is Render's MUTABLE service.name — the human-facing label
		// (displayName when set, else the immutable name), through the ONE shared
		// helper REST/MCP/webhooks already use (renderServiceName). Before w6/m115
		// this read the raw immutable name, so GraphQL disagreed with the other
		// three read surfaces for any renamed service. The immutable, addressable
		// name now lives in `immutableName` (below); `id` addresses too.
		"name":          gqlutil.StrField(func(a AppView) any { return renderServiceName(a) }),
		"immutableName": gqlutil.StrField(func(a AppView) any { return a.Name }),
		// slug is the globally-unique platform-host segment (w4/m19/w4/m20/t002) —
		// distinct from name, which is only workspace-unique.
		"slug":        gqlutil.StrField(func(a AppView) any { return a.Slug }),
		"displayName": gqlutil.StrField(func(a AppView) any { return a.DisplayName }),
		"type":        gqlutil.StrField(func(a AppView) any { return a.Type }),
		"suspended":   gqlutil.StrField(func(a AppView) any { return core.SuspendedEnum(a.Suspended) }),
		// suspenders lists WHO suspended the service (Render's array; w4/014):
		// ["user"] while suspended — the suspend verb is bex's only suspend
		// path — and [] otherwise.
		"suspenders":   gqlutil.StrsField(func(a AppView) any { return suspenders(a.Suspended) }),
		"dashboardUrl": gqlutil.StrField(func(a AppView) any { return a.DashboardURL }),
		"url":          gqlutil.StrField(func(a AppView) any { return a.URL }),
		// internalAddress is the private-network "<slug>:<port>" for web/private
		// services (empty otherwise) — the dashboard's Service Address / Connect
		// data source; a bex extension (docs/ADR041-service-addresses.md D4).
		"internalAddress": gqlutil.StrField(func(a AppView) any { return a.InternalAddress }),
		"createdAt":       gqlutil.StrField(func(a AppView) any { return a.CreatedAt }),
		"updatedAt":       gqlutil.StrField(func(a AppView) any { return a.UpdatedAt }),
		"region":          gqlutil.StrField(func(a AppView) any { return a.Region }),
		"sshAddress":      gqlutil.StrField(func(a AppView) any { return a.SSHAddress }),
		// bex-native extras.
		"phase": gqlutil.StrField(func(a AppView) any { return a.Phase }),
		// Why an exposed service has no public address (w7/m79). Empty when it
		// is routed or is not the kind that carries a public URL.
		"publicRoutingNotice": gqlutil.StrField(func(a AppView) any { return a.PublicRoutingNotice }),
		"replicas":            gqlutil.IntField(func(a AppView) any { return a.Replicas }),
		"revision":            gqlutil.StrField(func(a AppView) any { return a.Revision }),
		// idleTTLSeconds is the free-tier auto-sleep window (bex extension, no
		// Render counterpart); the Settings tab reads it and setIdleTimeout writes it.
		"idleTTLSeconds": gqlutil.IntField(func(a AppView) any { return a.IdleTTLSeconds }),
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
				"key": gqlutil.ReqArg(graphql.String),
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
				"name": gqlutil.ReqArg(graphql.String),
			},
			Resolve: secretFileContentResolve,
		},
		// plan is a bex extension (not yet captured live from Render's dashboard
		// traffic — the instance-type field/mutation naming there is unconfirmed);
		// it follows the existing suspendService/resumeService/restartServer
		// convention rather than inventing a different shape.
		"plan": gqlutil.StrField(func(a AppView) any { return a.Plan }),
		// schedule + command + runs describe a cron_job (empty/null for other
		// types): the cron's schedule, its entrypoint override, and its recent
		// run history (status.runs, newest first).
		"schedule": gqlutil.StrField(func(a AppView) any { return a.Schedule }),
		"command":  gqlutil.StrField(func(a AppView) any { return a.Command }),
		"runs": &graphql.Field{
			Type:    graphql.NewList(cronRunGQLType),
			Resolve: gqlutil.Field(func(a AppView) any { return a.Runs }),
		},
		"lastSuccessfulRunAt": gqlutil.StrField(func(a AppView) any { return a.LastSuccessfulRunAt }),
		// nextRunAt is a bex extension (Render has no next-run field): the cron's
		// next scheduled fire time (computed), empty for a suspended/non-cron.
		"nextRunAt": gqlutil.StrField(func(a AppView) any { return a.NextRunAt }),
		// ownerId mirrors Render's REST/MCP workspace-scoping field (w6/m2/t004).
		"ownerId":       gqlutil.StrField(func(a AppView) any { return a.OwnerID }),
		"projectId":     gqlutil.OptionalStrField(func(a AppView) any { return a.ProjectID }),
		"environmentId": gqlutil.OptionalStrField(func(a AppView) any { return a.EnvironmentID }),
		// rootDir is the subdirectory of the repo this App builds from (Render's
		// Root Directory setting, monorepo support); empty is the repo root.
		"rootDir":      gqlutil.StrField(func(a AppView) any { return a.RootDir }),
		"runtime":      gqlutil.StrField(func(a AppView) any { return a.Runtime }),
		"buildCommand": gqlutil.StrField(func(a AppView) any { return a.BuildCommand }),
		"startCommand": gqlutil.StrField(func(a AppView) any { return a.StartCommand }),
		// dockerfilePath is Render's Dockerfile Path setting, relative to rootDir;
		// applies only when runtime is docker.
		"dockerfilePath": gqlutil.StrField(func(a AppView) any { return a.DockerfilePath }),
		"builder":        gqlutil.StrField(func(a AppView) any { return a.Builder }),
		// repo/branch are the build-from-git source, empty for an image-backed App.
		"repo":   gqlutil.StrField(func(a AppView) any { return a.Repo }),
		"branch": gqlutil.StrField(func(a AppView) any { return a.Branch }),
		// imagePath is the CONFIGURED prebuilt image (REST's imagePath, from
		// SourceImage), empty for a repo-backed App — the dashboard Source card
		// reads it to show/edit an image-backed service's source (w5/m76). Distinct
		// from the observed running image digest.
		"imagePath": gqlutil.StrField(func(a AppView) any { return a.SourceImage }),
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
		"autoDeploy": gqlutil.BoolField(func(a AppView) any { return a.AutoDeploy }),
		// autoDeployTrigger is Render's newer enum for the same toggle
		// ("commit"|"off", w5/m53) mapped from bex's boolean spec.autoDeploy —
		// the dashboard's Auto-Deploy select reads it. bex never emits
		// "checksPass" (documented divergence).
		"autoDeployTrigger": gqlutil.StrField(func(a AppView) any { return triggerEnum(a.AutoDeploy) }),
		// pushDeliveryMethod is whether a push can REACH bex for this repo, the
		// signal the autoDeploy setting alone cannot express (w6/m99). Populated
		// on server(id)/service(id), empty on the list — see pushdelivery.go.
		"pushDeliveryMethod": gqlutil.StrField(func(a AppView) any { return a.PushDeliveryMethod }),
		// notifyOnFail is Render's per-service deploy-failure notification
		// override (default | notify | ignore, docs/render-artifacts/
		// notify-on-fail.md); the Settings → Notifications section reads it and
		// writes it via setNotifyOnFail.
		"notifyOnFail":        gqlutil.StrField(func(a AppView) any { return a.NotifyOnFail }),
		"notificationsToSend": gqlutil.StrField(func(a AppView) any { return a.NotificationsToSend }),
		// renderSubdomainPolicy is Render's field controlling whether the platform
		// subdomain <slug>.onbex.co is active (enabled|disabled, w7/m31). The
		// Settings → Custom Domains section reads it and writes it via
		// setSubdomainPolicy. OptionalStrField resolves the "" a non-ingress type
		// carries (view() empties it for private/worker/cron) to null, so GraphQL
		// agrees with REST's omission instead of reporting a phantom "enabled" for
		// a service that has no platform subdomain (w6/m130).
		"renderSubdomainPolicy": gqlutil.OptionalStrField(func(a AppView) any { return a.RenderSubdomainPolicy }),
		"healthCheckPath":       gqlutil.StrField(func(a AppView) any { return a.HealthCheckPath }),
		"maxShutdownDelaySeconds": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AppView) any {
			if a.MaxShutdownDelaySeconds == 0 {
				return nil
			}
			return a.MaxShutdownDelaySeconds
		})},
		// preDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand);
		// the Settings → Build & Deploy section reads it and writes via
		// setPreDeployCommand (w1/m33).
		"preDeployCommand": gqlutil.StrField(func(a AppView) any { return a.PreDeployCommand }),
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
		// disk: the attached persistent disk's shape, null when the service has
		// none (docs/ADR082-persistent-disks.md).
		"disk": &graphql.Field{
			Type: serviceDiskGQLType,
			Resolve: gqlutil.Field(func(a AppView) any {
				if a.Disk == nil {
					return nil
				}
				return *a.Disk
			}),
		},
		// publishPath/routes/headers describe a static_site (empty/null for other
		// types): the served output directory and its edge rules.
		"publishPath": gqlutil.StrField(func(a AppView) any { return a.PublishPath }),
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
			Resolve: gqlutil.Field(func(a AppView) any { return core.AllowListCIDRs(a.IPAllowList) }),
		},
		"ipAllowListEntries": &graphql.Field{
			Type:    graphql.NewList(gqlutil.IPAllowEntryType),
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
		// outboundIps is Render's retrieve-service-outbound-ips read nested under
		// the Service (w2/023; a bex extension — Render publishes no GraphQL
		// contract): {type, ips} of the shared tenant node pool's current
		// ExternalIPs. Resolved through the core.OutboundIPReader the root
		// injects — the same context seam env vars use, so this shared type
		// stays stateless.
		"outboundIps": &graphql.Field{
			Type:    outboundIPsGQLType,
			Resolve: outboundIPsResolve,
		},
	},
})

// cronRunGQLType renders one CronRunView — a cron_job's execution history entry.
var cronRunGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CronRun",
	Fields: graphql.Fields{
		"id":         gqlutil.StrField(func(r CronRunView) any { return r.ID }),
		"name":       gqlutil.StrField(func(r CronRunView) any { return r.Name }),
		"startedAt":  gqlutil.StrField(func(r CronRunView) any { return r.StartedAt }),
		"finishedAt": gqlutil.StrField(func(r CronRunView) any { return r.FinishedAt }),
		"status":     gqlutil.StrField(func(r CronRunView) any { return r.Status }),
	},
})

// instanceTypeGQLType renders InstanceType — the plan picker's data source.
// A bex extension (see InstanceType's doc comment): Render's dashboard has no
// public instanceTypes query to mirror, so this is REST/MCP-free by design,
// recorded in w5/m7's README rather than left silently asymmetric.
var instanceTypeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "InstanceType",
	Fields: graphql.Fields{
		"id":     gqlutil.StrField(func(t InstanceType) any { return t.ID }),
		"name":   gqlutil.StrField(func(t InstanceType) any { return t.Name }),
		"cpu":    gqlutil.StrField(func(t InstanceType) any { return t.CPU }),
		"memory": gqlutil.StrField(func(t InstanceType) any { return t.Memory }),
		"monthlyUsd": gqlutil.StrField(func(t InstanceType) any {
			return t.MonthlyUSD
		}),
	},
})

// serviceInstanceGQLType backs serviceInstances — Render's per-service instance
// list ({id, createdAt}), the source for the Web Shell instance picker (w2/m55)
// and Render's own instance-selection UX. Mirrors REST GET /v1/services/{id}/instances.
var serviceInstanceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ServiceInstance",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(v ServiceInstanceView) any { return v.ID }),
		"createdAt": gqlutil.StrField(func(v ServiceInstanceView) any { return v.CreatedAt.UTC().Format(time.RFC3339) }),
	},
})

// shellSessionGQLType backs createShellSession — the Browser Web Shell exec
// ticket the dashboard terminal opens the gateway WebSocket with
// (docs/ADR035-ssh.md § Browser Web Shell). bex extension over Render's GraphQL.
var shellSessionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ShellSession",
	Fields: graphql.Fields{
		"ticket":    gqlutil.StrField(func(v ShellSessionView) any { return v.Ticket }),
		"url":       gqlutil.StrField(func(v ShellSessionView) any { return v.URL }),
		"expiresAt": gqlutil.StrField(func(v ShellSessionView) any { return v.ExpiresAt }),
	},
})

// nameAvailabilityGQLType backs serviceNameAvailable — the create form's
// debounced availability check (w4/m19), a bex extension (Render has no
// public availability API, docs/render-artifacts/duplicate-service-names.md).
var nameAvailabilityGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "NameAvailability",
	Fields: graphql.Fields{
		"available":  gqlutil.BoolField(func(a NameAvailability) any { return a.Available }),
		"suggestion": gqlutil.StrField(func(a NameAvailability) any { return a.Suggestion }),
	},
})

// envVarGQLType renders the kernel's neutral core.EnvVar
// ({id,key,value,revision}), the
// object Render's dashboard nests under a service. bex has no separate id (the
// key is unique within a service), so id == key; the keys-only list leaves value
// empty (values are fetched per-key via envVar(key)). Revision is an opaque
// whole-environment CAS token: every item in one masked list and the explicit
// reveal carry the revision of the coherent map they observed.
var envVarGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvVar",
	Fields: graphql.Fields{
		"id":       gqlutil.StrField(func(v core.EnvVar) any { return v.ID }),
		"key":      gqlutil.StrField(func(v core.EnvVar) any { return v.Key }),
		"value":    gqlutil.StrField(func(v core.EnvVar) any { return v.Value }),
		"revision": gqlutil.StrField(func(v core.EnvVar) any { return v.Revision }),
	},
})

// gqlInt reads an optional argument, tolerating absence (graphql-go omits
// unset optional args from the map) — the create mutation's scalar args are
// all optional except name.

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
		"type":  gqlutil.StrField(func(r DNSRecordView) any { return r.Type }),
		"name":  gqlutil.StrField(func(r DNSRecordView) any { return r.Name }),
		"value": gqlutil.StrField(func(r DNSRecordView) any { return r.Value }),
	},
})

var customDomainGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CustomDomain",
	Fields: graphql.Fields{
		"id":                 gqlutil.StrField(func(d DomainView) any { return d.Name }),
		"name":               gqlutil.StrField(func(d DomainView) any { return d.Name }),
		"domainType":         gqlutil.StrField(func(d DomainView) any { return d.DomainType }),
		"ownershipStatus":    gqlutil.StrField(func(d DomainView) any { return d.OwnershipStatus }),
		"verificationStatus": gqlutil.StrField(func(d DomainView) any { return d.VerificationStatus }),
		"serverStatus":       gqlutil.StrField(func(d DomainView) any { return d.ServerStatus }),
		"redirectForName": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DomainView) any {
			if d.RedirectForName == "" {
				return nil
			}
			return d.RedirectForName
		})},
		// dnsRecord is the record the tenant must create (bex extension; the target is
		// the app's platform host <app>.<base-domain>).
		"dnsRecord": gqlutil.Typed(dnsRecordGQLType, func(d DomainView) any { return d.DNSRecord }),
		"ownershipDnsRecord": &graphql.Field{Type: dnsRecordGQLType, Resolve: gqlutil.Field(func(d DomainView) any {
			if d.OwnershipDNSRecord == nil {
				return nil
			}
			return *d.OwnershipDNSRecord
		})},
	},
})

// withParentWorkspace rebinds the request context to the parent service's own
// workspace before a nested resolver re-resolves that service by its
// workspace-scoped public name (round-21 finding 6). The GraphQL execution
// context carries no workspace selection, so without this a name lookup falls
// back to appCandidateNames(caller's-default-workspace, name) and can bind a
// same-named service in the caller's default workspace instead of the one the
// parent field selected — serving one workspace's secrets under another
// workspace's service object. The parent AppView was already resolved by its
// typed id, so its OwnerID is authoritative; an empty OwnerID (hand-applied App,
// never store-stamped) leaves the context unchanged, matching the bare-name
// resolution those Apps already use. The authorization boundary is unaffected —
// the reader still authorizes (and freshly re-asserts) against whichever
// workspace it resolves — so a mismatch fails closed rather than leaking.
func withParentWorkspace(ctx context.Context, a AppView) context.Context {
	if a.OwnerID == "" {
		return ctx
	}
	return core.WithWorkspace(ctx, a.OwnerID)
}

func envVarKeysResolve(p graphql.ResolveParams) (any, error) {
	a, ok := p.Source.(AppView)
	if !ok {
		return nil, nil
	}
	r, ok := core.EnvVarsFrom(p.Context)
	if !ok {
		return nil, core.ErrSecretsUnavailable
	}
	return r.EnvVarKeys(withParentWorkspace(p.Context, a), a.Name)
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
	return r.EnvVarValue(withParentWorkspace(p.Context, a), a.Name, p.Args["key"].(string))
}

// secretFileGQLType renders the kernel's neutral core.SecretFile ({id,name,content}),
// nested under a service like env vars. id == name; the names-only list leaves
// content empty (fetched per-file via secretFile(name)).
var secretFileGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SecretFile",
	Fields: graphql.Fields{
		"id":      gqlutil.StrField(func(f core.SecretFile) any { return f.ID }),
		"name":    gqlutil.StrField(func(f core.SecretFile) any { return f.Name }),
		"content": gqlutil.StrField(func(f core.SecretFile) any { return f.Content }),
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
	return r.SecretFileNames(withParentWorkspace(p.Context, a), a.Name)
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
	return r.SecretFileContent(withParentWorkspace(p.Context, a), a.Name, p.Args["name"].(string))
}

// outboundIPsGQLType renders the kernel's neutral core.OutboundIPs — Render's
// outboundIps schema — nested under a Service like env vars.
var outboundIPsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "OutboundIPs",
	Fields: graphql.Fields{
		"type": gqlutil.StrField(func(o core.OutboundIPs) any { return o.Type }),
		// dedicatedIpId is null in bex — always "shared" (dedicated egress IP
		// sets are a recorded non-goal), and Render emits the id only for
		// type=dedicated.
		"dedicatedIpId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(o core.OutboundIPs) any {
			if o.DedicatedIPID == "" {
				return nil
			}
			return o.DedicatedIPID
		})},
		"ips": gqlutil.StrsField(func(o core.OutboundIPs) any { return o.IPs }),
	},
})

func outboundIPsResolve(p graphql.ResolveParams) (any, error) {
	a, ok := p.Source.(AppView)
	if !ok {
		return nil, nil
	}
	r, ok := core.OutboundIPsFrom(p.Context)
	if !ok {
		return nil, core.ErrUnavailable
	}
	return r.OutboundIPs(p.Context, a.ID)
}

// blueprintResourceGQLType is the GraphQL shape for a BlueprintResource (w2/m62).
var blueprintResourceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintResource",
	Fields: graphql.Fields{
		"id":   gqlutil.StrField(func(r BlueprintResource) any { return r.ID }),
		"name": gqlutil.StrField(func(r BlueprintResource) any { return r.Name }),
		"type": gqlutil.StrField(func(r BlueprintResource) any { return r.Type }),
	},
})

// blueprintSyncGQLType is the GraphQL shape for a BlueprintSyncView (w2/m62).
var blueprintSyncGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintSync",
	Fields: graphql.Fields{
		"id":           gqlutil.StrField(func(r BlueprintSyncView) any { return r.ID }),
		"commitId":     gqlutil.StrField(func(r BlueprintSyncView) any { return r.CommitID }),
		"state":        gqlutil.StrField(func(r BlueprintSyncView) any { return r.State }),
		"startedAt":    gqlutil.StrField(func(r BlueprintSyncView) any { return r.StartedAt }),
		"completedAt":  gqlutil.StrField(func(r BlueprintSyncView) any { return r.CompletedAt }),
		"errorMessage": gqlutil.StrField(func(r BlueprintSyncView) any { return r.ErrorMessage }),
	},
})

// blueprintGQLType is the GraphQL shape for a BlueprintView (w2/m15 + w2/m62).
var blueprintGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Blueprint",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(b BlueprintView) any { return b.ID }),
		"name":      gqlutil.StrField(func(b BlueprintView) any { return b.Name }),
		"repo":      gqlutil.StrField(func(b BlueprintView) any { return b.Repo }),
		"branch":    gqlutil.StrField(func(b BlueprintView) any { return b.Branch }),
		"path":      gqlutil.StrField(func(b BlueprintView) any { return b.Path }),
		"autoSync":  gqlutil.BoolField(func(b BlueprintView) any { return b.AutoSync }),
		"manifest":  gqlutil.StrField(func(b BlueprintView) any { return b.Manifest }),
		"status":    gqlutil.StrField(func(b BlueprintView) any { return b.Status }),
		"lastSync":  gqlutil.StrField(func(b BlueprintView) any { return b.LastSync }),
		"resources": gqlutil.Typed(graphql.NewList(blueprintResourceGQLType), func(b BlueprintView) any { return b.Resources }),
		"createdAt": gqlutil.StrField(func(b BlueprintView) any { return b.CreatedAt }),
		"updatedAt": gqlutil.StrField(func(b BlueprintView) any { return b.UpdatedAt }),
	},
})

var blueprintValidationErrorGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintValidationError",
	Fields: graphql.Fields{
		"code":   gqlutil.StrField(func(e BlueprintValidationError) any { return e.Code }),
		"error":  gqlutil.ReqStrField(func(e BlueprintValidationError) any { return e.Error }),
		"line":   gqlutil.IntField(func(e BlueprintValidationError) any { return e.Line }),
		"column": gqlutil.IntField(func(e BlueprintValidationError) any { return e.Column }),
		"path":   gqlutil.StrField(func(e BlueprintValidationError) any { return e.Path }),
	},
})

var blueprintValidationPlanGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintValidationPlan",
	Fields: graphql.Fields{
		"mode":          gqlutil.StrField(func(p BlueprintValidationPlan) any { return p.Mode }),
		"services":      gqlutil.StrsField(func(p BlueprintValidationPlan) any { return p.Services }),
		"databases":     gqlutil.StrsField(func(p BlueprintValidationPlan) any { return p.Databases }),
		"keyValue":      gqlutil.StrsField(func(p BlueprintValidationPlan) any { return p.KeyValue }),
		"envGroups":     gqlutil.StrsField(func(p BlueprintValidationPlan) any { return p.EnvGroups }),
		"syncFalseVars": gqlutil.StrsField(func(p BlueprintValidationPlan) any { return p.SyncFalseVars }),
		"totalActions":  gqlutil.IntField(func(p BlueprintValidationPlan) any { return p.TotalActions }),
		"actions":       gqlutil.Typed(graphql.NewList(blueprintPlanActionGQLType), func(p BlueprintValidationPlan) any { return p.Actions }),
	},
})

var blueprintPlanFieldChangeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintPlanFieldChange",
	Fields: graphql.Fields{
		"path": gqlutil.StrField(func(c BlueprintFieldChange) any { return c.Path }),
	},
})

var blueprintPlanActionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintPlanAction",
	Fields: graphql.Fields{
		"operation":     gqlutil.StrField(func(a BlueprintPlanAction) any { return a.Operation }),
		"kind":          gqlutil.StrField(func(a BlueprintPlanAction) any { return a.Kind }),
		"name":          gqlutil.StrField(func(a BlueprintPlanAction) any { return a.Name }),
		"sourcePath":    gqlutil.StrField(func(a BlueprintPlanAction) any { return a.SourcePath }),
		"resourceId":    gqlutil.StrField(func(a BlueprintPlanAction) any { return a.ResourceID }),
		"changedFields": gqlutil.Typed(graphql.NewList(blueprintPlanFieldChangeGQLType), func(a BlueprintPlanAction) any { return a.ChangedFields }),
		"message":       gqlutil.StrField(func(a BlueprintPlanAction) any { return a.Message }),
	},
})

// blueprintPricingLineGQLType is one priced row of the estimated-pricing
// projection (w8/m18) — plan label + monthly cost, with the instance/storage
// breakdown the panel's tooltip renders for datastores.
var blueprintPricingLineGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintPricingLine",
	Fields: graphql.Fields{
		"name":         gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.Name }),
		"resourceKind": gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.ResourceKind }),
		"tier":         gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.Tier }),
		"tierLabel":    gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.TierLabel }),
		"monthlyUsd":   gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.MonthlyUSD }),
		"instanceUsd":  gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.InstanceUSD }),
		"storageUsd":   gqlutil.StrField(func(l pricing.MonthlyLine) any { return l.StorageUSD }),
		"storageGb":    gqlutil.IntField(func(l pricing.MonthlyLine) any { return l.StorageGB }),
	},
})

// blueprintVariableCostGQLType is a resource listed but excluded from the
// estimated total because its cost depends on runtime behavior.
var blueprintVariableCostGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintVariableCost",
	Fields: graphql.Fields{
		"name":   gqlutil.StrField(func(v pricing.VariableCost) any { return v.Name }),
		"reason": gqlutil.StrField(func(v pricing.VariableCost) any { return v.Reason }),
	},
})

// blueprintEstimatedPricingGQLType is the always-on monthly projection attached
// to a valid dry-run (the dashboard's Estimated pricing panel).
var blueprintEstimatedPricingGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintEstimatedPricing",
	Fields: graphql.Fields{
		"totalUsd": gqlutil.StrField(func(e pricing.MonthlyEstimate) any { return e.TotalUSD }),
		"lines":    gqlutil.Typed(graphql.NewList(blueprintPricingLineGQLType), func(e pricing.MonthlyEstimate) any { return e.Lines }),
		"variable": gqlutil.Typed(graphql.NewList(blueprintVariableCostGQLType), func(e pricing.MonthlyEstimate) any { return e.Variable }),
	},
})

// blueprintValidationGQLType preserves the dashboard's original errors:
// [String] field while also exposing Render's structured error details and plan.
var blueprintValidationGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintValidation",
	Fields: graphql.Fields{
		"valid": gqlutil.BoolField(func(v BlueprintValidation) any { return v.Valid }),
		"errors": &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v BlueprintValidation) any {
			messages := make([]string, len(v.Errors))
			for i, validationErr := range v.Errors {
				messages[i] = validationErr.Error
			}
			return messages
		})},
		"errorDetails": gqlutil.Typed(graphql.NewList(blueprintValidationErrorGQLType), func(v BlueprintValidation) any { return v.Errors }),
		"plan": &graphql.Field{Type: blueprintValidationPlanGQLType, Resolve: gqlutil.Field(func(v BlueprintValidation) any {
			if v.Plan == nil {
				return nil
			}
			return *v.Plan
		})},
		"estimatedPricing": &graphql.Field{Type: blueprintEstimatedPricingGQLType, Resolve: gqlutil.Field(func(v BlueprintValidation) any {
			if v.EstimatedPricing == nil {
				return nil
			}
			return *v.EstimatedPricing
		})},
	},
})

// generatedBlueprintGQLType is generateBlueprint's result (w8/m22).
var generatedBlueprintGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "GeneratedBlueprint",
	Fields: graphql.Fields{
		"manifest": gqlutil.StrField(func(r GenerateBlueprintResult) any { return r.Manifest }),
		"filename": gqlutil.StrField(func(r GenerateBlueprintResult) any { return r.Filename }),
	},
})

// blueprintPreviewGQLType is the GraphQL shape for a BlueprintPreview — the
// pre-create fetch + dry-run behind the dashboard's Review step.
var blueprintPreviewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BlueprintPreview",
	Fields: graphql.Fields{
		"found":    gqlutil.BoolField(func(p BlueprintPreview) any { return p.Found }),
		"manifest": gqlutil.StrField(func(p BlueprintPreview) any { return p.Manifest }),
		"commitId": gqlutil.StrField(func(p BlueprintPreview) any { return p.CommitID }),
		"warning":  gqlutil.StrField(func(p BlueprintPreview) any { return p.Warning }),
		"error":    gqlutil.StrField(func(p BlueprintPreview) any { return p.Error }),
		"validation": &graphql.Field{Type: blueprintValidationGQLType, Resolve: gqlutil.Field(func(p BlueprintPreview) any {
			if p.Validation == nil {
				return nil
			}
			return *p.Validation
		})},
	},
})

// syncBlueprintResultGQLType is the GraphQL shape for SyncBlueprintResult.
var syncBlueprintResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SyncBlueprintResult",
	Fields: graphql.Fields{
		"blueprint": gqlutil.Typed(blueprintGQLType, func(r SyncBlueprintResult) any { return r.Blueprint }),
		// services and databases from the stack apply — summary only (poll via server/postgres for full state).
		"services": gqlutil.Typed(graphql.NewList(serviceGQLType), func(r SyncBlueprintResult) any { return r.Stack.Services }),
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
	fields := graphql.Fields{
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
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				// ownerId mirrors Render's REST/MCP services list filter (w6/m2/t004).
				"ownerId": gqlutil.Arg(graphql.String),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				out, err := s.List(p.Context, gqlutil.Str(p.Args, "ownerId"))
				if err != nil {
					return nil, err
				}
				return gqlutil.Page(p, out, func(a AppView) string { return a.ID }), nil
			},
		},
		"server":  gqlutil.IDVerb(serviceGQLType, s.Get), // Render's dashboard query name
		"service": gqlutil.IDVerb(serviceGQLType, s.Get), // Render's dashboard also queries service(id) (e.g. serviceEnvVarKeys)
		// First-class cron run reads (bex extensions over Render's current public
		// API, which only exposes trigger/cancel-current). Both delegate to the
		// same status.runs verbs REST/MCP use.
		"cronJobRuns": &graphql.Field{
			Type: graphql.NewList(cronRunGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListCronRuns(p.Context, p.Args["serviceId"].(string), gqlutil.Str(p.Args, "cursor"), gqlutil.Int(p.Args, "limit"))
			},
		},
		"cronJobRun": &graphql.Field{
			Type: cronRunGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"runId":     gqlutil.ReqArg(graphql.String),
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
		"serviceInstances": gqlutil.IDVerb(graphql.NewList(serviceInstanceGQLType), s.ListInstances),
		// serviceNameAvailable: the create form's debounced availability check
		// (w4/m19), a bex extension backing the "Name is already in use" +
		// suggestion UX.
		"serviceNameAvailable": &graphql.Field{
			Type: nameAvailabilityGQLType,
			Args: graphql.FieldConfigArgument{
				"name": gqlutil.ReqArg(graphql.String),
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
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"id":                 gqlutil.ReqArg(graphql.String),
				"verificationStatus": gqlutil.Arg(graphql.String),
				"domainType":         gqlutil.Arg(graphql.String),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				out, err := s.ListDomains(p.Context, p.Args["id"].(string))
				if err != nil {
					return nil, err
				}
				out, err = filterDomains(out, gqlutil.Str(p.Args, "verificationStatus"), gqlutil.Str(p.Args, "domainType"))
				if err != nil {
					return nil, err
				}
				return gqlutil.Page(p, out, func(d DomainView) string { return d.Name }), nil
			},
		},
		"customDomain": gqlutil.ArgMutation(customDomainGQLType, "name", s.GetDomain),
		// blueprints: list known render.yaml stack sources for a workspace (w2/m15).
		"blueprints": &graphql.Field{
			Type: graphql.NewList(blueprintGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListBlueprints(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
		// blueprint: fetch a single blueprint by id (w7/m27 — dashboard detail page).
		"blueprint": &graphql.Field{
			Type: blueprintGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetBlueprintByID(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"))
			},
		},
		// validateBlueprint: dry-run parse render.yaml content — per-entry errors, no apply (w2/m15).
		"validateBlueprint": &graphql.Field{
			Type: blueprintValidationGQLType,
			Args: graphql.FieldConfigArgument{
				"bexYaml": gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ValidateBlueprint(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["bexYaml"].(string))
			},
		},
		// generateBlueprint: export selected resources as render.yaml (w8/m22).
		"generateBlueprint": &graphql.Field{
			Type: generatedBlueprintGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":     gqlutil.Arg(graphql.String),
				"serviceIds":  gqlutil.Arg(graphql.NewList(graphql.String)),
				"postgresIds": gqlutil.Arg(graphql.NewList(graphql.String)),
				"keyValueIds": gqlutil.Arg(graphql.NewList(graphql.String)),
				"envGroupIds": gqlutil.Arg(graphql.NewList(graphql.String)),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GenerateBlueprint(p.Context, GenerateBlueprintRequest{
					OwnerID:     gqlutil.Str(p.Args, "ownerId"),
					ServiceIDs:  gqlutil.StringList(p.Args["serviceIds"]),
					PostgresIDs: gqlutil.StringList(p.Args["postgresIds"]),
					KeyValueIDs: gqlutil.StringList(p.Args["keyValueIds"]),
					EnvGroupIDs: gqlutil.StringList(p.Args["envGroupIds"]),
				})
			},
		},
		// blueprintPreview: fetch a repo's render.yaml and dry-run validate it before
		// any create — the dashboard's Render-parity Review step.
		"blueprintPreview": &graphql.Field{
			Type: blueprintPreviewGQLType,
			Args: graphql.FieldConfigArgument{
				"repo":    gqlutil.ReqArg(graphql.String),
				"branch":  gqlutil.ReqArg(graphql.String),
				"path":    gqlutil.Arg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PreviewBlueprint(p.Context, gqlutil.Str(p.Args, "ownerId"),
					p.Args["repo"].(string), p.Args["branch"].(string), gqlutil.Str(p.Args, "path"))
			},
		},
		// blueprintSyncs: sync run history for a blueprint (w2/m62).
		"blueprintSyncs": &graphql.Field{
			Type: graphql.NewList(blueprintSyncGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				limit := gqlutil.Int(p.Args, "limit")
				return s.ListBlueprintSyncs(p.Context, p.Args["id"].(string),
					gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "cursor"), limit)
			},
		},
	}
	// Disk reads live in their own file beside the disk verbs (disks.go).
	maps.Copy(fields, s.diskGQLQueryFields())
	maps.Copy(fields, s.diskSnapshotGQLQueryFields())
	return fields
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
			"id":      gqlutil.ReqArg(graphql.String),
			"confirm": gqlutil.Arg(graphql.String),
		}
	}
	verb := func(fn func(context.Context, string) (AppView, error)) graphql.FieldResolveFn {
		return func(p graphql.ResolveParams) (any, error) {
			confirm := gqlutil.Str(p.Args, "confirm")
			ctx := core.WithConfirm(p.Context, confirm)
			return fn(ctx, p.Args["id"].(string))
		}
	}
	fields := graphql.Fields{
		// createService: create-or-update a service (the create half of the
		// lifecycle). A bex extension — the create mutation's name/shape is not
		// confirmed against a live Render dashboard capture (same caveat as
		// updateServicePlan/scaleService); it follows their scalar-arg convention.
		// One of repo/image is required, the rest fall back to platform defaults.
		"createService": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"name": gqlutil.ReqArg(graphql.String),
				// ownerId is the workspace to create IN (w6/m14) — the write-side
				// twin of the services list filter above, and the same optional
				// contract REST's create body has: omitted => the caller's default
				// workspace; a workspace they don't belong to => forbidden.
				"ownerId":              gqlutil.Arg(graphql.String),
				"environmentId":        gqlutil.Arg(graphql.String),
				"type":                 gqlutil.Arg(graphql.String), // web_service (default) | private_service | background_worker | cron_job
				"schedule":             gqlutil.Arg(graphql.String), // cron expression, required when type is cron_job
				"command":              gqlutil.Arg(graphql.String), // overrides the image's entrypoint for a cron_job
				"repo":                 gqlutil.Arg(graphql.String),
				"image":                gqlutil.Arg(graphql.String),
				"registryCredentialId": gqlutil.Arg(graphql.String),
				"branch":               gqlutil.Arg(graphql.String),
				"rootDir":              gqlutil.Arg(graphql.String),       // subdirectory of repo to build from (monorepo support)
				"buildFilter":          gqlutil.Arg(buildFilterInputType), // Render's Build Filters: globs gating push auto-deploys
				"runtime":              gqlutil.Arg(graphql.String),       // Render runtime: native language | docker | image
				"buildCommand":         gqlutil.Arg(graphql.String),
				"startCommand":         gqlutil.Arg(graphql.String),
				// dockerfilePath is Render's Dockerfile Path, relative to rootDir; docker runtime only.
				"dockerfilePath": gqlutil.Arg(graphql.String),
				"builder":        gqlutil.Arg(graphql.String), // auto (default) | buildpack | dockerfile
				"plan":           gqlutil.Arg(graphql.String),
				"autoDeploy":     gqlutil.Arg(graphql.Boolean),
				// notifyOnFail is Render's per-service deploy-failure notification
				// override (default | notify | ignore); omitted => "default".
				"notifyOnFail": gqlutil.Arg(graphql.String),
				"port":         gqlutil.Arg(graphql.Int),
				"replicas":     gqlutil.Arg(graphql.Int),
				// envVars sets literal (non-secret) environment variables at create time
				// (Render parity, w5/m19): REST/MCP parity — those surfaces accepted
				// envVars at create since w2/m2; GraphQL now reaches the same shape.
				// envVars: reuses gqlutil.EnvVarInputType (shared with secrets.setEnvVars
				// to avoid duplicate type names in the composed schema).
				"envVars":     gqlutil.Arg(graphql.NewList(gqlutil.EnvVarInputType)),
				"secretFiles": gqlutil.Arg(graphql.NewList(secretFileInputType)),
				// static_site create fields.
				"publishPath":             gqlutil.Arg(graphql.String),
				"routes":                  gqlutil.Arg(graphql.NewList(staticRouteInputType)),
				"headers":                 gqlutil.Arg(graphql.NewList(staticHeaderInputType)),
				"healthCheckPath":         gqlutil.Arg(graphql.String),
				"maxShutdownDelaySeconds": gqlutil.Arg(graphql.Int),
				// preDeployCommand is Render's Pre-Deploy Command (spec.preDeployCommand, w1/m33).
				"preDeployCommand": gqlutil.Arg(graphql.String),
				// maintenanceMode is Render's maintenanceMode object at create time
				// (w1/m37); web_service only. Omitted => disabled.
				"maintenanceMode": gqlutil.Arg(maintenanceModeInputType),
				// Description-aware service allowlist plus the legacy CIDR list.
				// Conflicting simultaneous values are rejected by Core.
				"ipAllowList":        gqlutil.Arg(graphql.NewList(graphql.String)),
				"ipAllowListEntries": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))),
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				dryRun := gqlutil.Bool(p.Args, "dryRun")
				env, err := gqlEnvVarInputs(p.Args, "envVars")
				if err != nil {
					return nil, err
				}
				_, entriesSet := p.Args["ipAllowListEntries"]
				_, cidrsSet := p.Args["ipAllowList"]
				allowList, err := core.ResolveAllowListInputs(
					gqlutil.AllowList(p.Args["ipAllowListEntries"]), entriesSet,
					gqlutil.StringList(p.Args["ipAllowList"]), cidrsSet,
				)
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
					Port:                    int32(gqlutil.Int(p.Args, "port")),
					Replicas:                int32(gqlutil.Int(p.Args, "replicas")),
					Env:                     env,
					SecretFiles:             gqlSecretFileInputs(p.Args, "secretFiles"),
					PublishPath:             gqlutil.Str(p.Args, "publishPath"),
					Routes:                  gqlRouteInputs(p.Args, "routes"),
					Headers:                 gqlHeaderInputs(p.Args, "headers"),
					HealthCheckPath:         gqlutil.Str(p.Args, "healthCheckPath"),
					MaxShutdownDelaySeconds: gqlInt32Ptr(p.Args, "maxShutdownDelaySeconds"),
					PreDeployCommand:        gqlutil.Str(p.Args, "preDeployCommand"),
					MaintenanceMode:         gqlMaintenanceModeInput(p.Args, "maintenanceMode"),
					IPAllowList:             allowList,
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
				confirm := gqlutil.Str(p.Args, "confirm")
				err := s.Delete(core.WithConfirm(p.Context, confirm), p.Args["id"].(string))
				return err == nil, err
			},
		},
		// updateCronJob: change a cron_job's schedule and/or command (w5/m18).
		// Rejected for a non-cron service (core.ErrBadRequest). schedule is
		// required; command is optional (empty clears the image-entrypoint override).
		"updateCronJob": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":       gqlutil.ReqArg(graphql.String),
				"schedule": gqlutil.ReqArg(graphql.String),
				"command":  gqlutil.Arg(graphql.String),
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
				"serviceId": gqlutil.ReqArg(graphql.String),
				"runId":     gqlutil.ReqArg(graphql.String),
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
		"updateServicePlan": gqlutil.PlanMutation(serviceGQLType, s.SetPlan, s.PreviewSetPlan),
		// setDisplayName relabels a service for humans while leaving its immutable
		// App name/id, platform hostname, and derived Kubernetes resources alone.
		// An empty displayName clears the label and restores the name fallback.
		"setDisplayName": gqlutil.ArgMutation(serviceGQLType, "displayName", s.SetDisplayName),
		// setRegistryCredential binds an image-backed service or Dockerfile build
		// to one stored workspace credential. Empty clears the binding; the
		// service verb owns the same membership/source checks used by REST/MCP.
		"setRegistryCredential": gqlutil.ArgMutation(serviceGQLType, "registryCredentialId", s.SetRegistryCredential),
		// scaleService: Render's manual-scaling verb. numInstances mirrors the
		// REST scale body field; out-of-range is a GraphQL error (core.ErrBadRequest).
		"scaleService": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":           gqlutil.ReqArg(graphql.String),
				"numInstances": gqlutil.ReqArg(graphql.Int),
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
				"id":         gqlutil.ReqArg(graphql.String),
				"instanceId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				instanceID := gqlutil.Str(p.Args, "instanceId")
				return s.CreateShellSession(p.Context, p.Args["id"].(string), instanceID)
			},
		},
		// setIdleTimeout: bex extension (no Render counterpart) — sets the free-tier
		// auto-sleep window (spec.idleTTLSeconds; 0 selects the platform default idle
		// window, 15 min, stored unrewritten). Out-of-range is a GraphQL error
		// (core.ErrBadRequest).
		"setIdleTimeout": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":             gqlutil.ReqArg(graphql.String),
				"idleTTLSeconds": gqlutil.ReqArg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetIdleTTL(p.Context, p.Args["id"].(string), int32(p.Args["idleTTLSeconds"].(int)))
			},
		},
		// setRootDir: the Settings → Build & Deploy save flow (w5/m13) writes
		// Render's Root Directory setting (spec.rootDir) on an existing App
		// (create-time rootDir is handled by createService above). Rejected for
		// an image-backed App (core.ErrBadRequest — nothing to build).
		"setRootDir": gqlutil.ArgMutation(serviceGQLType, "rootDir", s.SetRootDir),
		// setBranch changes the Git branch a repo-backed service builds and
		// deploys from (Render's editable Branch field, w5/m48/t005). Delegates
		// to the shared source verb — the same path Render's REST PATCH `branch`
		// field takes — so validation and push-to-deploy semantics cannot drift
		// between surfaces. Empty restores the default "main".
		"setBranch": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":     gqlutil.ReqArg(graphql.String),
				"branch": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				branch := p.Args["branch"].(string)
				return s.SetSourceAndRegistryCredential(p.Context, p.Args["id"].(string), sourcePatch{Branch: &branch})
			},
		},
		// setRepo switches the Git repository a service builds from (Render's
		// editable Source field, w5/m54). Delegates to the same shared source
		// verb as setBranch/REST PATCH `repo`, so validation and deferred-until-
		// next-deploy semantics stay identical across surfaces.
		"setRepo": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":     gqlutil.ReqArg(graphql.String),
				"repo":   gqlutil.ReqArg(graphql.String),
				"branch": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				repo := p.Args["repo"].(string)
				patch := sourcePatch{Repo: &repo, Branch: gqlutil.StrPtr(p.Args, "branch")}
				return s.SetSourceAndRegistryCredential(p.Context, p.Args["id"].(string), patch)
			},
		},
		// setImage switches a service to a prebuilt container image (Render's
		// Update-Source repo→image half, w5/m76). The same shared source verb as
		// setRepo/setBranch and REST PATCH `image`, so mutual exclusion (setting
		// the image clears repo/branch source) and validation (ValidImage) cannot
		// drift; optional registryCredentialId is validated against the image host
		// before either reaches the App. Neither this nor setRepo triggers a
		// deploy — the next deploy uses the new source (Render's semantics).
		"setImage": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                   gqlutil.ReqArg(graphql.String),
				"image":                gqlutil.ReqArg(graphql.String),
				"registryCredentialId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				image := p.Args["image"].(string)
				patch := sourcePatch{Image: &image}
				if cred, ok := p.Args["registryCredentialId"].(string); ok {
					patch.RegistryCredentialID = &cred
				}
				return s.SetSourceAndRegistryCredential(p.Context, p.Args["id"].(string), patch)
			},
		},
		// setBuildCommand changes the build command for a repo-backed service.
		// Applies to static_site (the primary user) and native-runtime services.
		// The shared SetCommands verb also backs Render's REST PATCH shape; this
		// scalar setter is the dashboard-friendly GraphQL projection.
		"setBuildCommand": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"command": gqlutil.ReqArg(graphql.String),
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
				"id":      gqlutil.ReqArg(graphql.String),
				"command": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				command := p.Args["command"].(string)
				return s.SetCommands(p.Context, p.Args["id"].(string), nil, &command)
			},
		},
		// setDockerfilePath changes Render's Dockerfile Path on an existing
		// repo-backed Docker service. Empty restores the default Dockerfile.
		"setDockerfilePath": gqlutil.ArgMutation(serviceGQLType, "dockerfilePath", s.SetDockerfilePath),
		// setBuildFilter: the Settings → Build & Deploy Build Filters rows (w1/m34)
		// write Render's Build Filters (spec.buildFilter) — the glob patterns gating
		// git-push auto-deploys — on an existing App. Passing an all-empty object
		// clears the filter. Rejected for an image-backed App (nothing to build).
		// Follows the scalar/object-arg setter grammar (setRootDir/setStaticRoutes).
		"setBuildFilter": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"buildFilter": gqlutil.ReqArg(buildFilterInputType),
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
				"id":              gqlutil.ReqArg(graphql.String),
				"maintenanceMode": gqlutil.ReqArg(maintenanceModeInputType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				in := gqlMaintenanceModeInput(p.Args, "maintenanceMode")
				return s.SetMaintenanceMode(p.Context, p.Args["id"].(string), *in)
			},
		},
		// setHealthCheckPath: the Settings → Health & Alerts health-check path
		// (w5/009). Changes spec.healthCheckPath — the path the startup,
		// readiness, and liveness probes GET (w1/m23/t001, w7/m81). An empty path clears the field and
		// selects the TCP probe instead (w7/m80), so this is the surface a caller
		// uses to move a service off the strict HTTP check. Rejected for
		// cron_job/background_worker (no HTTP port).
		"setHealthCheckPath": gqlutil.ArgMutation(serviceGQLType, "path", s.SetHealthCheckPath),
		// setMaxShutdownDelay changes Render's per-service SIGTERM grace window.
		// The shared service verb enforces 1-300 and rejects cron/static services.
		"setMaxShutdownDelay": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"seconds": gqlutil.ReqArg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetMaxShutdownDelay(p.Context, p.Args["id"].(string), int32(p.Args["seconds"].(int)))
			},
		},
		// setPreDeployCommand: the Settings → Build & Deploy Pre-Deploy Command
		// field (w1/m33). Changes spec.preDeployCommand — the command run against
		// the new image before it serves traffic. An empty command clears the
		// step. Rejected for cron_job/static_site (the field doesn't apply).
		"setPreDeployCommand": gqlutil.ArgMutation(serviceGQLType, "command", s.SetPreDeployCommand),
		// setAutoDeploy: the Settings → Build & Deploy Auto-Deploy toggle (w2/m9)
		// flips whether a signed git push redeploys the App (spec.autoDeploy). A
		// bex extension name (Render's dashboard mutation is uncaptured), following
		// the scalar-arg convention.
		"setAutoDeploy": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"enabled": gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetAutoDeploy(p.Context, p.Args["id"].(string), p.Args["enabled"].(bool))
			},
		},
		// setNotifyOnFail: the Settings → Notifications per-service override
		// (w4/m21, docs/render-artifacts/notify-on-fail.md). Changes
		// spec.notifyOnFail — default | notify | ignore, Render's exact
		// name/enum. Unrecognized value ⇒ core.ErrBadRequest.
		"setNotifyOnFail":        gqlutil.ArgMutation(serviceGQLType, "value", s.SetNotifyOnFail),
		"setNotificationsToSend": gqlutil.ArgMutation(serviceGQLType, "value", s.SetNotificationsToSend),
		// setSubdomainPolicy: the Settings → Custom Domains platform-subdomain
		// toggle (w7/m31). Changes spec.subdomainPolicy — enabled | disabled,
		// Render's exact renderSubdomainPolicy enum. "disabled" without a custom
		// domain ⇒ core.ErrBadRequest (would silently kill the service).
		"setSubdomainPolicy": gqlutil.ArgMutation(serviceGQLType, "policy", s.SetSubdomainPolicy),
		// setServiceIpAllowList replaces the inbound allowlist for a web_service
		// or static_site. entries preserves descriptions; cidrs is the legacy
		// compatibility input. Empty clears the list.
		"setServiceIpAllowList": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"cidrs":   gqlutil.Arg(graphql.NewList(graphql.String)),
				"entries": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				_, entriesSet := p.Args["entries"]
				_, cidrsSet := p.Args["cidrs"]
				entries, err := core.ResolveAllowListInputs(
					gqlutil.AllowList(p.Args["entries"]), entriesSet,
					gqlutil.StringList(p.Args["cidrs"]), cidrsSet,
				)
				if err != nil {
					return nil, err
				}
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), entries)
			},
		},
		// Static-site edge-rule mutations: replace the whole routes/headers list
		// (Render's bulk PUT), or change the publish directory. Rejected for a
		// non-static_site (core.ErrBadRequest).
		"setStaticRoutes": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":     gqlutil.ReqArg(graphql.String),
				"routes": gqlutil.Arg(graphql.NewList(staticRouteInputType)),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetRoutes(p.Context, p.Args["id"].(string), gqlRouteInputs(p.Args, "routes"))
			},
		},
		"setStaticHeaders": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"headers": gqlutil.Arg(graphql.NewList(staticHeaderInputType)),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetHeaders(p.Context, p.Args["id"].(string), gqlHeaderInputs(p.Args, "headers"))
			},
		},
		"setPublishPath": gqlutil.ArgMutation(serviceGQLType, "publishPath", s.SetPublishPath),
		// Custom domain mutations — Render-dashboard-shaped operation names.
		"addCustomDomain": gqlutil.ArgMutation(customDomainGQLType, "name", s.AddDomain),
		"deleteCustomDomain": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":   gqlutil.ReqArg(graphql.String),
				"name": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteDomain(p.Context, p.Args["id"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},
		// verifyCustomDomain checks the durable ownership TXT proof and atomically
		// promotes a pending managed claim before it can enter serving intent.
		"verifyCustomDomain": gqlutil.ArgMutation(customDomainGQLType, "name", s.VerifyDomain),
		// setAutoscaling: enable/update autoscaling on a service (mirrors Render's
		// PUT /v1/services/{id}/autoscaling). Returns the updated autoscaling config.
		"setAutoscaling": &graphql.Field{
			Type: autoscalingGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                  gqlutil.ReqArg(graphql.String),
				"minInstances":        gqlutil.ReqArg(graphql.Int),
				"maxInstances":        gqlutil.ReqArg(graphql.Int),
				"targetCPUPercent":    gqlutil.Arg(graphql.Int),
				"targetMemoryPercent": gqlutil.Arg(graphql.Int),
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
		// createBlueprint: create a Git-connected Blueprint from a repo (w2/m62).
		"createBlueprint": &graphql.Field{
			Type: blueprintGQLType,
			Args: graphql.FieldConfigArgument{
				"repo":         gqlutil.ReqArg(graphql.String),
				"branch":       gqlutil.ReqArg(graphql.String),
				"path":         gqlutil.Arg(graphql.String),
				"name":         gqlutil.Arg(graphql.String),
				"ownerId":      gqlutil.Arg(graphql.String),
				"envVarValues": gqlutil.Arg(graphql.NewList(blueprintEnvVarValueInputType)),
				"confirm":      gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				values, err := blueprintEnvVarValues(p.Args["envVarValues"])
				if err != nil {
					return nil, err
				}
				return s.CreateBlueprint(p.Context, gqlutil.Str(p.Args, "ownerId"), CreateBlueprintRequest{
					Repo:         p.Args["repo"].(string),
					Branch:       p.Args["branch"].(string),
					Path:         gqlutil.Str(p.Args, "path"),
					Name:         gqlutil.Str(p.Args, "name"),
					EnvVarValues: values,
					Confirm:      gqlutil.Str(p.Args, "confirm"),
				})
			},
		},
		// syncBlueprint: re-apply a stored blueprint idempotently (w2/m15 + w2/m62).
		// If bexYaml is provided, the stored manifest is replaced before re-apply.
		"syncBlueprint": &graphql.Field{
			Type: syncBlueprintResultGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"bexYaml": gqlutil.Arg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
				"confirm": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SyncBlueprint(p.Context, p.Args["id"].(string),
					gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "bexYaml"), gqlutil.Str(p.Args, "confirm"))
			},
		},
		// updateBlueprint: PATCH name/autoSync/path (w2/m62).
		"updateBlueprint": &graphql.Field{
			Type: blueprintGQLType,
			Args: graphql.FieldConfigArgument{
				"id":       gqlutil.ReqArg(graphql.String),
				"name":     gqlutil.Arg(graphql.String),
				"autoSync": gqlutil.Arg(graphql.Boolean),
				"path":     gqlutil.Arg(graphql.String),
				"ownerId":  gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var name, bpPath *string
				var autoSync *bool
				if v, ok := p.Args["name"].(string); ok {
					name = &v
				}
				if v, ok := p.Args["path"].(string); ok {
					bpPath = &v
				}
				if v, ok := p.Args["autoSync"].(bool); ok {
					autoSync = &v
				}
				return s.UpdateBlueprint(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"), UpdateBlueprintRequest{
					Name:     name,
					AutoSync: autoSync,
					Path:     bpPath,
				})
			},
		},
		// disconnectBlueprint: stop syncing + hide; resources remain (w2/m62).
		"disconnectBlueprint": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DisconnectBlueprint(p.Context, p.Args["id"].(string), gqlutil.Str(p.Args, "ownerId"))
				return err == nil, err
			},
		},
	}
	maps.Copy(fields, s.diskGQLMutationFields())
	maps.Copy(fields, s.diskSnapshotGQLMutationFields())
	return fields
}
