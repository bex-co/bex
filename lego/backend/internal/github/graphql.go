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

package github

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// gitConnectionGQLType mirrors the REST Connection object. Both queries and the
// connect/disconnect mutations resolve it (a bex extension — Render's dashboard
// GraphQL for git connections is uncaptured).
var gitConnectionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "GitConnection",
	Fields: graphql.Fields{
		"connected":      &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(c Connection) any { return c.Connected })},
		"accountLogin":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(c Connection) any { return c.AccountLogin })},
		"installationId": &graphql.Field{Type: graphql.Float, Resolve: gqlutil.Field(func(c Connection) any { return c.InstallationID })},
		"createdAt":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(c Connection) any { return c.CreatedAt })},
		"installUrl":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(c Connection) any { return c.InstallURL })},
	},
})

// repoGQLType mirrors the REST Repo object (bex extension — Render has no public
// repo-list GraphQL). Field set is identical to REST/MCP.
var repoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Repo",
	Fields: graphql.Fields{
		"id":            &graphql.Field{Type: graphql.Float, Resolve: gqlutil.Field(func(r Repo) any { return r.ID })},
		"fullName":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r Repo) any { return r.FullName })},
		"private":       &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(r Repo) any { return r.Private })},
		"defaultBranch": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r Repo) any { return r.DefaultBranch })},
		"htmlUrl":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r Repo) any { return r.HTMLURL })},
		"cloneUrl":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(r Repo) any { return r.CloneURL })},
	},
})

// GraphQLQuery returns the gitConnection + repos queries.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"gitConnection": &graphql.Field{
			Type:    gitConnectionGQLType,
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.GetConnection(p.Context) },
		},
		"repos": &graphql.Field{
			Type:    graphql.NewList(repoGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.ListRepos(p.Context) },
		},
	}
}

// GraphQLMutation returns connectGit (returns the connection + install URL) and
// disconnectGit.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"connectGit": &graphql.Field{
			Type:    gitConnectionGQLType,
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.StartConnect(p.Context) },
		},
		"disconnectGit": &graphql.Field{
			Type: graphql.Boolean,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.Disconnect(p.Context)
				return err == nil, err
			},
		},
	}
}
