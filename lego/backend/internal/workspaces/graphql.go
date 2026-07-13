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

package workspaces

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment the dashboard consumes: the `workspaces`
// query (the switcher's list) and create/rename/delete mutations. It is
// deliberately GraphQL-only — Render exposes no REST workspace-mutation API
// (verified in .pm/w6/RESEARCH-workspaces.md: /v1/owners is read-only), so the
// REST owners read endpoints + MCP tools are w6/m2, not a REST mutation surface
// here. Every resolver delegates to the Service; the schema is presentation.

// workspaceGQLType renders WorkspaceView. Fields follow Render's owner/workspace
// vocabulary (id, name, plan) plus the caller's role; the REST owner shape in
// w6/m2 maps the same view.
var workspaceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Workspace",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(w WorkspaceView) any { return w.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(w WorkspaceView) any { return w.Name })},
		"plan":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(w WorkspaceView) any { return w.Plan })},
		"role":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(w WorkspaceView) any { return w.Role })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(w WorkspaceView) any { return w.CreatedAt })},
	},
})

// GraphQLQuery contributes the caller's workspace list to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"workspaces": &graphql.Field{
			Type:    graphql.NewList(workspaceGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.List(p.Context) },
		},
	}
}

// GraphQLMutation contributes the lifecycle mutations to the root Mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createWorkspace": &graphql.Field{
			Type: workspaceGQLType,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				plan, _ := p.Args["plan"].(string)
				return s.Create(p.Context, p.Args["name"].(string), plan)
			},
		},
		"renameWorkspace": &graphql.Field{
			Type: workspaceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Rename(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		"changeWorkspacePlan": &graphql.Field{
			// w6/m12: upgrade/downgrade a workspace's plan
			// (docs/render-artifacts/workspace-plan-change.md). GraphQL-only —
			// Render's REST owners surface has no plan-mutation endpoint and its
			// MCP has no workspace mutations, so REST/MCP deliberately gain
			// nothing here (parity by absence).
			Type: workspaceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ChangePlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
			},
		},
		"deleteWorkspace": &graphql.Field{
			// Returns the deleted workspace's id on success (Render dashboard
			// delete mutations echo the affected id); a boolean would lose it.
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"confirmation": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
					// Render's live delete guard: "sudo delete workspace <name>"
					// (docs/render-artifacts/workspace-lifecycle.md).
					Description: `must equal "sudo delete workspace <name>"`,
				},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				id := p.Args["id"].(string)
				if err := s.Delete(p.Context, id, p.Args["confirmation"].(string)); err != nil {
					return nil, err
				}
				return id, nil
			},
		},
	}
}
