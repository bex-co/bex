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

package deploys

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the GraphQL fragment: a read query, deploys(serviceId), for
// the dashboard's Deploys/Events tab (w5) to nest deploy history under a
// service, plus w2/m10's cancelDeploy/rollbackService mutations and the
// triggerDeploy mutation (w2/006). Named deploys/serviceId (not id) since it
// isn't a single-resource fetch-by-id like server(id) — it's a service-scoped
// list, matching the milestone's own naming.

var deployGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Deploy",
	Fields: graphql.Fields{
		"id":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return d.ID })},
		"status":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return d.Status })},
		"trigger":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return d.Trigger })},
		"image":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return d.Image })},
		"rollbackOf": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return d.RollbackOf })},
		"createdAt":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return formatTime(d.CreatedAt) })},
		"startedAt":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return formatTimePtr(d.StartedAt) })},
		"finishedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(d DeployView) any { return formatTimePtr(d.FinishedAt) })},
	},
})

// GraphQLQuery returns the deploys(serviceId) field for the composition root
// to merge into the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"deploys": &graphql.Field{
			Type: graphql.NewList(deployGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, p.Args["serviceId"].(string))
			},
		},
	}
}

// deployMutationArgs is the (serviceId, deployId) argument shape cancelDeploy and
// rollbackService share.
var deployMutationArgs = graphql.FieldConfigArgument{
	"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
	"deployId":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
}

// GraphQLMutation returns triggerDeploy (w2/006) plus cancelDeploy/rollbackService
// (w2/m10). triggerDeploy mirrors Render's dashboard Manual Deploy action —
// delegates to the same Trigger verb as REST POST .../deploys so the surfaces
// cannot drift. cancelDeploy/rollbackService follow the existing
// suspendService/resumeService scalar-arg convention.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"triggerDeploy": &graphql.Field{
			Type: deployGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Trigger(p.Context, p.Args["serviceId"].(string))
			},
		},
		"cancelDeploy": &graphql.Field{
			Type: deployGQLType,
			Args: deployMutationArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Cancel(p.Context, p.Args["serviceId"].(string), p.Args["deployId"].(string))
			},
		},
		"rollbackService": &graphql.Field{
			Type: deployGQLType,
			Args: deployMutationArgs,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Rollback(p.Context, p.Args["serviceId"].(string), p.Args["deployId"].(string))
			},
		},
	}
}
