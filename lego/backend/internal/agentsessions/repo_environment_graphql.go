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

package agentsessions

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// GraphQL fragment of the ADR111 repository environment builds.

var repoEnvironmentBuildStepGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentRepoEnvironmentBuildStep",
	Description: "One setup step of a build: its exit, duration and redacted output tail.",
	Fields: graphql.Fields{
		"name":       gqlutil.ReqStrField(func(v RepoEnvironmentBuildStep) any { return v.Name }),
		"exitCode":   &graphql.Field{Type: graphql.Int, Description: "Null for a step that was killed or never started.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuildStep) any { return v.ExitCode })},
		"durationMs": &graphql.Field{Type: graphql.NewNonNull(graphql.Float), Resolve: gqlutil.Field(func(v RepoEnvironmentBuildStep) any { return float64(v.DurationMs) })},
		"output":     gqlutil.ReqStrField(func(v RepoEnvironmentBuildStep) any { return v.Output }),
	},
})

var repoEnvironmentBuildGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "AgentRepoEnvironmentBuild",
	Description: "A build of a repository environment's setup steps into the image its sessions boot from (ADR111).",
	Fields: graphql.Fields{
		"id":            gqlutil.ReqStrField(func(v RepoEnvironmentBuild) any { return v.ID }),
		"ownerId":       gqlutil.ReqStrField(func(v RepoEnvironmentBuild) any { return v.OwnerID }),
		"repo":          gqlutil.ReqStrField(func(v RepoEnvironmentBuild) any { return v.Repo }),
		"version":       &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "The environment version the build baked; sessions use it while the environment's version is the same.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuild) any { return v.Version })},
		"trigger":       &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "environment_saved, secret_changed, manual or refresh.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuild) any { return v.Trigger })},
		"status":        &graphql.Field{Type: graphql.NewNonNull(graphql.String), Description: "queued, building, success, failed or cancelled.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuild) any { return v.Status })},
		"failureCode":   &graphql.Field{Type: graphql.String, Description: "setup_failed, step_timeout, build_timeout, checkout_failed, storage_limit_reached or internal_error, on a failed build.", Resolve: gqlutil.OptionalStrField(func(v RepoEnvironmentBuild) any { return v.FailureCode }).Resolve},
		"failureReason": gqlutil.OptionalStrField(func(v RepoEnvironmentBuild) any { return v.FailureReason }),
		"headSha":       &graphql.Field{Type: graphql.String, Description: "The default branch commit the build checked out.", Resolve: gqlutil.OptionalStrField(func(v RepoEnvironmentBuild) any { return v.HeadSHA }).Resolve},
		"sizeBytes": &graphql.Field{Type: graphql.Float, Description: "The image's compressed size.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuild) any {
			if v.SizeBytes == nil {
				return nil
			}
			return float64(*v.SizeBytes)
		})},
		"imageAvailable": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Description: "Whether sessions can boot from the build's image.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuild) any { return v.ImageAvailable })},
		"steps":          &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(repoEnvironmentBuildStepGQLType)), Description: "The build's redacted log; null on lists, read agentRepoEnvironmentBuild(id) for it.", Resolve: gqlutil.Field(func(v RepoEnvironmentBuild) any { return v.Steps })},
		"createdAt":      gqlutil.TimeField(func(v RepoEnvironmentBuild) any { return v.CreatedAt }),
		"startedAt":      gqlutil.TimeField(func(v RepoEnvironmentBuild) any { return v.StartedAt }),
		"finishedAt":     gqlutil.TimeField(func(v RepoEnvironmentBuild) any { return v.FinishedAt }),
	},
})

var repoEnvironmentBuildListGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AgentRepoEnvironmentBuildList",
	Fields: graphql.Fields{
		"ownerId": gqlutil.ReqStrField(func(v RepoEnvironmentBuildList) any { return v.OwnerID }),
		"repo":    gqlutil.ReqStrField(func(v RepoEnvironmentBuildList) any { return v.Repo }),
		"builds":  gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(repoEnvironmentBuildGQLType))), func(v RepoEnvironmentBuildList) any { return v.Builds }),
	},
})

func (s *Service) repoEnvironmentBuildQueries() graphql.Fields {
	return graphql.Fields{
		"agentRepoEnvironmentBuilds": &graphql.Field{
			Type:        graphql.NewNonNull(repoEnvironmentBuildListGQLType),
			Description: "A repository's environment builds, newest first (the last 20).",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"repo":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RepoEnvironmentBuilds(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "repo"))
			},
		},
		"agentRepoEnvironmentBuild": &graphql.Field{
			Type:        graphql.NewNonNull(repoEnvironmentBuildGQLType),
			Description: "One repository environment build with its redacted log.",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"id":      gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RepoEnvironmentBuild(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "id"))
			},
		},
	}
}

func (s *Service) repoEnvironmentBuildMutations() graphql.Fields {
	return graphql.Fields{
		"buildAgentRepoEnvironment": &graphql.Field{
			Type:        graphql.NewNonNull(repoEnvironmentBuildGQLType),
			Description: "Build a repository environment now (admins only). A queued build is replaced; a running one finishes first. At most 12 builds start per repository in 24 hours.",
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"repo":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.BuildRepoEnvironment(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Str(p.Args, "repo"))
			},
		},
	}
}
