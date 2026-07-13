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
			Type:        gqlStr(m, "type"),
			Source:      gqlStr(m, "source"),
			Destination: gqlStr(m, "destination"),
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
			Path:  gqlStr(m, "path"),
			Name:  gqlStr(m, "name"),
			Value: gqlStr(m, "value"),
		})
	}
	return out
}

func gqlEnvVarInputs(args map[string]any, key string) []appv1alpha1.EnvVar {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]appv1alpha1.EnvVar, 0, len(raw))
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, appv1alpha1.EnvVar{Name: gqlStr(m, "key"), Value: gqlStr(m, "value")})
	}
	return out
}

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// Render-shaped fields (id is the App name; type is the serviceType enum).
		"id":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		"type":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Type })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return core.SuspendedEnum(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.CreatedAt })},
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
		// ownerId mirrors Render's REST/MCP workspace-scoping field (w6/m2/t004).
		"ownerId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.OwnerID })},
		// rootDir is the subdirectory of the repo this App builds from (Render's
		// Root Directory setting, monorepo support); empty is the repo root.
		"rootDir": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.RootDir })},
		// repo/branch are the build-from-git source, empty for an image-backed App.
		"repo":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Repo })},
		"branch":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Branch })},
		"autoDeploy":      &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(a AppView) any { return a.AutoDeploy })},
		"healthCheckPath": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.HealthCheckPath })},
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
	},
})

// cronRunGQLType renders one CronRunView — a cron_job's execution history entry.
var cronRunGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CronRun",
	Fields: graphql.Fields{
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

// gqlStr / gqlInt read an optional argument, tolerating absence (graphql-go
// omits unset optional args from the map) — the create mutation's scalar args
// are all optional except name.
func gqlStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func gqlStrPtr(args map[string]any, key string) *string {
	v, ok := args[key].(string)
	if !ok {
		return nil
	}
	return &v
}

func gqlInt(args map[string]any, key string) int {
	if v, ok := args[key].(int); ok {
		return v
	}
	return 0
}

// gqlBoolPtr reads an optional Boolean arg as a tri-state *bool (absent => nil,
// so the platform default applies).
func gqlBoolPtr(args map[string]any, key string) *bool {
	if v, ok := args[key].(bool); ok {
		return &v
	}
	return nil
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
			},
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context, gqlStr(p.Args, "ownerId")) },
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
		"instanceTypes": &graphql.Field{ // bex extension backing the plan picker (see InstanceType)
			Type:    graphql.NewList(instanceTypeGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.InstanceTypes(p.Context) },
		},
		// Custom domains — Render-dashboard-shaped queries.
		"customDomains": &graphql.Field{
			Type: graphql.NewList(customDomainGQLType),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListDomains(p.Context, p.Args["id"].(string))
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
	}
}

// GraphQLMutation returns the lifecycle mutations (Render dashboard names
// suspendService / resumeService / restartServer).
func (s *Service) GraphQLMutation() graphql.Fields {
	verb := func(fn func(context.Context, string) (AppView, error)) graphql.FieldResolveFn {
		return func(p graphql.ResolveParams) (any, error) {
			return fn(p.Context, p.Args["id"].(string))
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
				"ownerId":    &graphql.ArgumentConfig{Type: graphql.String},
				"type":       &graphql.ArgumentConfig{Type: graphql.String}, // web_service (default) | private_service | background_worker | cron_job
				"schedule":   &graphql.ArgumentConfig{Type: graphql.String}, // cron expression, required when type is cron_job
				"command":    &graphql.ArgumentConfig{Type: graphql.String}, // overrides the image's entrypoint for a cron_job
				"repo":       &graphql.ArgumentConfig{Type: graphql.String},
				"image":      &graphql.ArgumentConfig{Type: graphql.String},
				"branch":     &graphql.ArgumentConfig{Type: graphql.String},
				"rootDir":    &graphql.ArgumentConfig{Type: graphql.String}, // subdirectory of repo to build from (monorepo support)
				"plan":       &graphql.ArgumentConfig{Type: graphql.String},
				"autoDeploy": &graphql.ArgumentConfig{Type: graphql.Boolean},
				"port":       &graphql.ArgumentConfig{Type: graphql.Int},
				"replicas":   &graphql.ArgumentConfig{Type: graphql.Int},
				// envVars sets literal (non-secret) environment variables at create time
				// (Render parity, w5/m19): REST/MCP parity — those surfaces accepted
				// envVars at create since w2/m2; GraphQL now reaches the same shape.
				// envVars: reuses gqlutil.EnvVarInputType (shared with secrets.setEnvVars
				// to avoid duplicate type names in the composed schema).
				"envVars": &graphql.ArgumentConfig{Type: graphql.NewList(gqlutil.EnvVarInputType)},
				// static_site create fields.
				"publishPath": &graphql.ArgumentConfig{Type: graphql.String},
				"routes":      &graphql.ArgumentConfig{Type: graphql.NewList(staticRouteInputType)},
				"headers":     &graphql.ArgumentConfig{Type: graphql.NewList(staticHeaderInputType)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, CreateRequest{
					OwnerID:     gqlStr(p.Args, "ownerId"),
					Name:        p.Args["name"].(string),
					Type:        gqlStr(p.Args, "type"),
					Schedule:    gqlStr(p.Args, "schedule"),
					Command:     gqlStr(p.Args, "command"),
					Repo:        gqlStr(p.Args, "repo"),
					Image:       gqlStr(p.Args, "image"),
					Branch:      gqlStr(p.Args, "branch"),
					RootDir:     gqlStr(p.Args, "rootDir"),
					Plan:        gqlStr(p.Args, "plan"),
					AutoDeploy:  gqlBoolPtr(p.Args, "autoDeploy"),
					Port:        int32(gqlInt(p.Args, "port")),
					Replicas:    int32(gqlInt(p.Args, "replicas")),
					Env:         gqlEnvVarInputs(p.Args, "envVars"),
					PublishPath: gqlStr(p.Args, "publishPath"),
					Routes:      gqlRouteInputs(p.Args, "routes"),
					Headers:     gqlHeaderInputs(p.Args, "headers"),
				})
			},
		},
		// deleteService: delete a service (the delete half of the lifecycle).
		// Returns a success boolean like deleteCustomDomain — there is no service
		// object left to return. A bex extension (Render's dashboard delete
		// mutation name wasn't captured), following the deleteCustomDomain shape.
		"deleteService": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.Delete(p.Context, p.Args["id"].(string))
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
				return s.SetCronJob(p.Context, p.Args["id"].(string), &sched, gqlStrPtr(p.Args, "command"))
			},
		},
		// runCronJob: trigger a one-off run of a cron_job (Render's cron run verb);
		// the run shows in status.runs once the operator reconciles.
		"runCronJob":     &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.TriggerCronRun)},
		"suspendService": &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Suspend)},
		"resumeService":  &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Resume)},
		"restartServer":  &graphql.Field{Type: serviceGQLType, Args: gqlutil.IDArg(), Resolve: verb(s.Restart)},
		// updateServicePlan: a bex extension (naming unconfirmed against a live
		// Render dashboard capture — see the "plan" field comment above).
		"updateServicePlan": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetPlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
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
	}
}
