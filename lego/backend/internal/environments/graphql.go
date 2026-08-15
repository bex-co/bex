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

package environments

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the environments GraphQL fragment, mirroring
// internal/projects/graphql.go's shape (bex extension; Type name
// "Environment" is distinct from the pre-existing, unrelated "EnvGroup" type
// internal/envgroups registers, so the merged schema has no collision).

var environmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Environment",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ID })},
		"projectId":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ProjectID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.Name })},
		"ownerId":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.OwnerID })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.CreatedAt })},
		"serviceIds":              &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ServiceIDs })},
		"databaseIds":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.DatabaseIDs })},
		"keyValueIds":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.KeyValueIDs })},
		"envGroupIds":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.EnvGroupIDs })},
		"protectedStatus":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ProtectedStatus })},
		"networkIsolationEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.NetworkIsolationEnabled })},
		"ipAllowList":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return core.AllowListCIDRs(e.IPAllowList) })},
		"ipAllowListEntries":      &graphql.Field{Type: graphql.NewList(gqlutil.IPAllowEntryType), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.IPAllowList })},
	},
})

// GraphQLQuery contributes the environments reads to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"environments": &graphql.Field{
			Type: graphql.NewList(environmentGQLType),
			Args: graphql.FieldConfigArgument{
				"projectId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				out, err := s.List(p.Context, p.Args["projectId"].(string))
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
				return core.StablePage(out, cursor, limit, cursorSet || limitSet, func(e EnvironmentView) string { return e.ID }), nil
			},
		},
		"environment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// allowListPtr reads updateEnvironment's optional ipAllowList/ipAllowListEntries
// pair as a *[]core.IPAllowListEntry — nil when neither arg key is present
// (unchanged), non-nil (possibly empty, an explicit deny-all) otherwise.
// graphql-go omits an unset optional arg from p.Args entirely, so the map-key
// presence check (not the decoded value) is what distinguishes "absent" from
// "explicit empty list".
func allowListPtr(p graphql.ResolveParams) *[]core.IPAllowListEntry {
	_, entriesOK := p.Args["ipAllowListEntries"]
	_, cidrsOK := p.Args["ipAllowList"]
	if !entriesOK && !cidrsOK {
		return nil
	}
	entries := core.AllowListOrCIDRs(gqlutil.AllowList(p.Args["ipAllowListEntries"]), gqlutil.StringList(p.Args["ipAllowList"]))
	return &entries
}

// GraphQLMutation contributes the environment write verbs to the root Mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createEnvironment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"name":                    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"projectId":               &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"protectedStatus":         &graphql.ArgumentConfig{Type: graphql.String},
				"networkIsolationEnabled": &graphql.ArgumentConfig{Type: graphql.Boolean},
				"ipAllowList":             &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				isolated, _ := p.Args["networkIsolationEnabled"].(bool)
				return s.CreateWithACL(p.Context, CreateEnvironmentRequest{
					Name: p.Args["name"].(string), ProjectID: p.Args["projectId"].(string),
					ProtectedStatus: gqlutil.Str(p.Args, "protectedStatus"), NetworkIsolationEnabled: isolated,
					IPAllowList: gqlutil.AllowList(p.Args["ipAllowList"]),
				})
			},
		},
		"renameEnvironment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Rename(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		// updateEnvironment is the partial-update verb (w4/m30): every field
		// optional, absent fields untouched, riding the core Update verb — the
		// GraphQL/MCP counterpart to REST PATCH /v1/environments/{id}. The
		// existing renameEnvironment/setEnvironmentACL verbs (bex-native,
		// single-field/full-replace) keep working unchanged.
		"updateEnvironment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name":                    &graphql.ArgumentConfig{Type: graphql.String},
				"protectedStatus":         &graphql.ArgumentConfig{Type: graphql.String},
				"networkIsolationEnabled": &graphql.ArgumentConfig{Type: graphql.Boolean},
				"ipAllowList":             &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
				"ipAllowListEntries":      &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Update(p.Context, p.Args["id"].(string), EnvironmentPatch{
					Name:                    gqlutil.StrPtr(p.Args, "name"),
					ProtectedStatus:         gqlutil.StrPtr(p.Args, "protectedStatus"),
					NetworkIsolationEnabled: gqlutil.BoolPtr(p.Args, "networkIsolationEnabled"),
					IPAllowList:             allowListPtr(p),
				})
			},
		},
		"deleteEnvironment": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				id := p.Args["id"].(string)
				if err := s.Delete(p.Context, id); err != nil {
					return nil, err
				}
				return id, nil
			},
		},
		"setEnvironmentServices": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"serviceIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetServices(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["serviceIds"]))
			},
		},
		"setEnvironmentDatabases": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"databaseIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetDatabases(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["databaseIds"]))
			},
		},
		"setEnvironmentKeyValues": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"keyValueIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetKeyValues(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["keyValueIds"]))
			},
		},
		"setEnvironmentEnvGroups": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"envGroupIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetEnvGroups(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["envGroupIds"]))
			},
		},
		// setEnvironmentACL replaces the full protected-environment ACL triple
		// (w6/m19) — full-replace, matching setEnvironmentServices above.
		"setEnvironmentACL": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"protectedStatus":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"networkIsolationEnabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				"ipAllowList":             &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
				// ipAllowListEntries is the description-carrying form; precedence
				// over ipAllowList lives in core.AllowListOrCIDRs.
				"ipAllowListEntries": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				entries := core.AllowListOrCIDRs(gqlutil.AllowList(p.Args["ipAllowListEntries"]), gqlutil.StringList(p.Args["ipAllowList"]))
				return s.SetACL(p.Context, p.Args["id"].(string), p.Args["protectedStatus"].(string), p.Args["networkIsolationEnabled"].(bool), entries)
			},
		},
	}
}
