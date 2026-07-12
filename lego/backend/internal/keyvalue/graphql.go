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

package keyvalue

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// The GraphQL noun is "keyValue" — matching bex's own KeyValue CRD and Render's
// current "Key Value" product branding (the same way the postgres feature's
// GraphQL noun "database" matches its Database CRD). If a live Render dashboard
// capture ever shows a legacy "redis" noun, that is a rename follow-up recorded
// in docs/ADR018-render-parity.md, not a silent divergence.
var keyValueGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValue",
	Fields: graphql.Fields{
		"id":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.ID })},
		"name":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Name })},
		"plan":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Plan })},
		"version":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Version })},
		"status":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Status })},
		"suspended":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Suspended })},
		"createdAt":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.CreatedAt })},
		"externalHost": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.ExternalHost })},
		"public":       &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Public })},
		"ipAllowList":  &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v KeyValueView) any { return v.IPAllowList })},
		"ownerId":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.OwnerID })},
	},
})

// keyValueInstanceTypeGQLType renders KeyValueInstanceType — the create dialog's
// plan-picker source, the managed key-value sibling of databaseInstanceTypes. A
// bex extension (Render's dashboard has no public query to mirror), REST/MCP-free
// by design and recorded in the milestone rather than left silently asymmetric.
var keyValueInstanceTypeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValueInstanceType",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t KeyValueInstanceType) any { return t.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t KeyValueInstanceType) any { return t.Name })},
		"cpu":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t KeyValueInstanceType) any { return t.CPU })},
		"memory":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t KeyValueInstanceType) any { return t.Memory })},
		"storageGB": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(t KeyValueInstanceType) any { return t.StorageGB })},
	},
})

var keyValueConnectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValueConnectionInfo",
	Fields: graphql.Fields{
		"internalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueConnectionInfo) any { return v.InternalConnectionString })},
		"externalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueConnectionInfo) any { return v.ExternalConnectionString })},
		"cliCommand":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueConnectionInfo) any { return v.CLICommand })},
	},
})

// GraphQLQuery returns the key-value read fields.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"keyValues": &graphql.Field{
			Type: graphql.NewList(keyValueGQLType),
			Args: graphql.FieldConfigArgument{
				// ownerId mirrors Render's REST/MCP key-value list filter (w6/m4/t002).
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ownerID, _ := p.Args["ownerId"].(string)
				return s.ListKeyValues(p.Context, ownerID)
			},
		},
		"keyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetKeyValue(p.Context, p.Args["id"].(string))
			},
		},
		"keyValueConnectionInfo": &graphql.Field{
			Type: keyValueConnectionInfoGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.KeyValueConnectionInfo(p.Context, p.Args["id"].(string))
			},
		},
		"keyValueInstanceTypes": &graphql.Field{ // bex extension backing the create dialog's plan picker
			Type:    graphql.NewList(keyValueInstanceTypeGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.InstanceTypes(p.Context) },
		},
		"keyValueIpAllowList": &graphql.Field{
			Type: graphql.NewList(graphql.String),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetIPAllowList(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// GraphQLMutation returns the create / delete / suspend / resume mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createKeyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"name":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan":        &graphql.ArgumentConfig{Type: graphql.String},
				"version":     &graphql.ArgumentConfig{Type: graphql.String},
				"storageGB":   &graphql.ArgumentConfig{Type: graphql.Int},
				"public":      &graphql.ArgumentConfig{Type: graphql.Boolean},
				"ipAllowList": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := CreateKeyValueRequest{Name: p.Args["name"].(string)}
				if v, ok := p.Args["plan"].(string); ok {
					req.Plan = v
				}
				if v, ok := p.Args["version"].(string); ok {
					req.Version = v
				}
				if v, ok := p.Args["storageGB"].(int); ok {
					req.StorageGB = int32(v)
				}
				if v, ok := p.Args["public"].(bool); ok {
					req.Public = v
				}
				req.IPAllowList = gqlutil.StringList(p.Args["ipAllowList"])
				return s.CreateKeyValue(p.Context, req)
			},
		},
		"deleteKeyValue": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteKeyValue(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
		"suspendKeyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Suspend(p.Context, p.Args["id"].(string))
			},
		},
		"resumeKeyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Resume(p.Context, p.Args["id"].(string))
			},
		},
		"setKeyValueIpAllowList": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cidrs": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["cidrs"]))
			},
		},
	}
}
