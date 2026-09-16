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
package router

import (
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/graphql-go/graphql"
)

var windowType = graphql.NewObject(graphql.ObjectConfig{Name: "RouterWindow", Fields: graphql.Fields{
	"kind":           gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v Window) any { return v.Kind }),
	"utilizationBps": gqlutil.Typed(graphql.NewNonNull(graphql.Float), func(v Window) any { return v.UtilizationBps }),
	"resetsAt":       gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v Window) any { return v.ResetsAt }),
}})
var optionsType = graphql.NewObject(graphql.ObjectConfig{Name: "RouterOptions", Fields: graphql.Fields{
	"allowOrigin":      gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v *Options) any { return v.AllowOrigin }),
	"allowMethods":     gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v *Options) any { return v.AllowMethods }),
	"allowHeaders":     gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v *Options) any { return v.AllowHeaders }),
	"allowCredentials": gqlutil.Typed(graphql.NewNonNull(graphql.Boolean), func(v *Options) any { return v.AllowCredentials }),
}})
var keyType = graphql.NewObject(graphql.ObjectConfig{Name: "RouterKey", Fields: graphql.Fields{
	"id":        gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v Key) any { return v.ID }),
	"name":      gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v Key) any { return v.Name }),
	"accessKey": gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v Key) any { return v.AccessKey }),
	"createdAt": gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v Key) any { return v.CreatedAt }),
	"options":   gqlutil.Typed(optionsType, func(v Key) any { return v.Options }),
}})
var quotaType = graphql.NewObject(graphql.ObjectConfig{Name: "RouterQuota", Fields: graphql.Fields{
	"observedAt": gqlutil.Typed(graphql.NewNonNull(graphql.String), func(v *Quota) any { return v.ObservedAt }),
	"windows":    gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(windowType))), func(v *Quota) any { return v.Windows }),
}})
var overviewType = graphql.NewObject(graphql.ObjectConfig{Name: "RouterOverview", Fields: graphql.Fields{
	"quota": gqlutil.Typed(quotaType, func(v *Overview) any { return v.Quota }),
	"keys":  gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(keyType))), func(v *Overview) any { return v.Keys }),
}})

// Router is a session-only dashboard beta, with no Render REST/MCP counterpart.
// Keep its verbs on GraphQL until a public machine-credential contract exists.
func (s *Service) GraphQLQuery() graphql.Fields {
	return workspaceFields(graphql.Fields{
		"routerAvailable": &graphql.Field{Type: graphql.Boolean, Resolve: func(p graphql.ResolveParams) (any, error) { return s.Available(p.Context) }},
		"routerOverview":  &graphql.Field{Type: overviewType, Resolve: func(p graphql.ResolveParams) (any, error) { return s.Overview(p.Context) }},
	})
}
func (s *Service) GraphQLMutation() graphql.Fields {
	return workspaceFields(graphql.Fields{
		"createRouterKey": &graphql.Field{Type: graphql.Boolean, Args: gqlutil.KeyArg("name"), Resolve: func(p graphql.ResolveParams) (any, error) { return s.Create(p.Context, p.Args["name"].(string)) }},
		"deleteRouterKey": &graphql.Field{Type: graphql.Boolean, Args: gqlutil.IDArg(), Resolve: func(p graphql.ResolveParams) (any, error) { return s.Delete(p.Context, p.Args["id"].(string)) }},
		"updateRouterKey": &graphql.Field{Type: graphql.Boolean, Args: graphql.FieldConfigArgument{
			"id": gqlutil.ReqArg(graphql.String), "name": gqlutil.ReqArg(graphql.String),
			"allowOrigin": gqlutil.ReqArg(graphql.String), "allowMethods": gqlutil.ReqArg(graphql.String), "allowHeaders": gqlutil.ReqArg(graphql.String), "allowCredentials": gqlutil.ReqArg(graphql.Boolean),
		}, Resolve: func(p graphql.ResolveParams) (any, error) {
			return s.Update(p.Context, p.Args["id"].(string), p.Args["name"].(string), Options{AllowOrigin: p.Args["allowOrigin"].(string), AllowMethods: p.Args["allowMethods"].(string), AllowHeaders: p.Args["allowHeaders"].(string), AllowCredentials: p.Args["allowCredentials"].(bool)})
		}},
	})
}

func workspaceFields(fields graphql.Fields) graphql.Fields {
	for _, field := range fields {
		if field.Args == nil {
			field.Args = graphql.FieldConfigArgument{}
		}
		field.Args["ownerId"] = gqlutil.ReqArg(graphql.String)
		resolve := field.Resolve
		field.Resolve = func(p graphql.ResolveParams) (any, error) {
			p.Context = core.WithWorkspace(p.Context, gqlutil.Str(p.Args, "ownerId"))
			return resolve(p)
		}
	}
	return fields
}
