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
)

// graphql.go is the GraphQL fragment, matching the operation names Render's
// dashboard uses (captured live): query server(id) / services; mutations
// suspendService(id) / resumeService(id) / restartServer(id); type Service with
// the string `suspended` enum. Every resolver delegates to the Service — the
// schema is presentation, the behavior is shared with REST and MCP.

var serviceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Service",
	Fields: graphql.Fields{
		// Render-shaped fields (id is the App name; type is always web_service).
		"id":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Name })},
		"type":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return renderWebService })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return core.SuspendedEnum(a.Suspended) })},
		"dashboardUrl": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"url":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.URL })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.CreatedAt })},
		// bex-native extras.
		"phase":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Phase })},
		"replicas": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(a AppView) any { return a.Replicas })},
		"revision": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Revision })},
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
		// plan is a bex extension (not yet captured live from Render's dashboard
		// traffic — the instance-type field/mutation naming there is unconfirmed);
		// it follows the existing suspendService/resumeService/restartServer
		// convention rather than inventing a different shape.
		"plan": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AppView) any { return a.Plan })},
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

// GraphQLQuery returns the App read fields (Render dashboard names services /
// server(id)) for the composition root to merge into the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"services": &graphql.Field{
			Type:    graphql.NewList(serviceGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context) },
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
	}
}
