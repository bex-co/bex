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
	"encoding/json"
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// Build sources on GraphQL and MCP (w2/m254 t007, ADR097): bex extensions
// named after Render's REST verbs, over the same service verbs REST calls.
// A build source's build-time environment uses the env-var, secret-file and
// env-group verbs with its bds- id as serviceId.

var buildSourceGitGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildSourceGit",
	Fields: graphql.Fields{
		"repoUrl":      gqlutil.StrField(func(g *BuildSourceGit) any { return g.RepoURL }),
		"branch":       gqlutil.StrField(func(g *BuildSourceGit) any { return g.Branch }),
		"buildCommand": gqlutil.OptionalStrField(func(g *BuildSourceGit) any { return g.BuildCommand }),
		"buildFilter": gqlutil.Typed(buildFilterGQLType, func(g *BuildSourceGit) any {
			if g.BuildFilter == nil {
				return nil
			}
			return *g.BuildFilter
		}),
		"dockerContext":        gqlutil.OptionalStrField(func(g *BuildSourceGit) any { return g.DockerContext }),
		"dockerfilePath":       gqlutil.OptionalStrField(func(g *BuildSourceGit) any { return g.DockerfilePath }),
		"runtime":              gqlutil.OptionalStrField(func(g *BuildSourceGit) any { return g.Runtime }),
		"registryCredentialId": gqlutil.StrField(func(g *BuildSourceGit) any { return g.RegistryCredentialID }),
		"region":               gqlutil.OptionalStrField(func(g *BuildSourceGit) any { return g.Region }),
		"rootDir":              gqlutil.OptionalStrField(func(g *BuildSourceGit) any { return g.RootDir }),
	},
})

var buildSourceImageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildSourceImage",
	Fields: graphql.Fields{
		"ownerId":              gqlutil.StrField(func(i *BuildSourceImage) any { return i.OwnerID }),
		"imageUrl":             gqlutil.StrField(func(i *BuildSourceImage) any { return i.ImageURL }),
		"registryCredentialId": gqlutil.StrField(func(i *BuildSourceImage) any { return i.RegistryCredentialID }),
	},
})

var buildSourceServiceLinkGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildSourceServiceLink",
	Fields: graphql.Fields{
		"id":   gqlutil.StrField(func(l BuildSourceServiceLink) any { return l.ID }),
		"name": gqlutil.StrField(func(l BuildSourceServiceLink) any { return l.Name }),
	},
})

var buildSourceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "BuildSource",
	Fields: graphql.Fields{
		"id":           gqlutil.StrField(func(b BuildSourceView) any { return b.ID }),
		"name":         gqlutil.StrField(func(b BuildSourceView) any { return b.Name }),
		"ownerId":      gqlutil.StrField(func(b BuildSourceView) any { return b.OwnerID }),
		"projectId":    gqlutil.OptionalStrField(func(b BuildSourceView) any { return b.ProjectID }),
		"git":          gqlutil.Typed(buildSourceGitGQLType, func(b BuildSourceView) any { return b.Git }),
		"image":        gqlutil.Typed(buildSourceImageGQLType, func(b BuildSourceView) any { return b.Image }),
		"serviceLinks": gqlutil.Typed(graphql.NewList(buildSourceServiceLinkGQLType), func(b BuildSourceView) any { return b.ServiceLinks }),
		"cursor":       gqlutil.StrField(func(b BuildSourceView) any { return b.ID }),
		"createdAt":    gqlutil.StrField(func(b BuildSourceView) any { return b.CreatedAt }),
		"updatedAt":    gqlutil.StrField(func(b BuildSourceView) any { return b.UpdatedAt }),
	},
})

var buildSourceGitInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BuildSourceGitInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"repoUrl":              &graphql.InputObjectFieldConfig{Type: graphql.String},
		"branch":               &graphql.InputObjectFieldConfig{Type: graphql.String},
		"buildCommand":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"buildFilter":          &graphql.InputObjectFieldConfig{Type: buildFilterInputType},
		"dockerContext":        &graphql.InputObjectFieldConfig{Type: graphql.String},
		"dockerfilePath":       &graphql.InputObjectFieldConfig{Type: graphql.String},
		"runtime":              &graphql.InputObjectFieldConfig{Type: graphql.String},
		"registryCredentialId": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"region":               &graphql.InputObjectFieldConfig{Type: graphql.String},
		"rootDir":              &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var buildSourceImageInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "BuildSourceImageInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"ownerId":              &graphql.InputObjectFieldConfig{Type: graphql.String},
		"imageUrl":             &graphql.InputObjectFieldConfig{Type: graphql.String},
		"registryCredentialId": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// gqlDecodeArg decodes an input argument into its REST shape: the input
// fields carry the REST JSON names. The zero value when absent.
func gqlDecodeArg[T any](args map[string]any, key string) (T, error) {
	var out T
	v, ok := args[key]
	if !ok || v == nil {
		return out, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("%w: %s: %v", core.ErrBadRequest, key, err)
	}
	return out, nil
}

func (s *Service) buildSourceGQLQueryFields() graphql.Fields {
	return graphql.Fields{
		"buildSources": &graphql.Field{
			Type: graphql.NewList(buildSourceGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"ownerId":       gqlutil.Arg(graphql.String),
				"name":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"projectId":     gqlutil.Arg(graphql.String),
				"createdBefore": gqlutil.Arg(graphql.String),
				"createdAfter":  gqlutil.Arg(graphql.String),
				"updatedBefore": gqlutil.Arg(graphql.String),
				"updatedAfter":  gqlutil.Arg(graphql.String),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				f, err := parseBuildSourceFilter(gqlutil.StringList(p.Args["name"]), func(k string) string { return gqlutil.Str(p.Args, k) })
				if err != nil {
					return nil, err
				}
				return s.listBuildSourcesPage(p.Context, gqlutil.Str(p.Args, "ownerId"), f, gqlutil.Str(p.Args, "cursor"), core.PageLimitOrDefault(gqlutil.Int(p.Args, "limit")))
			},
		},
		"buildSource": gqlutil.IDVerb(buildSourceGQLType, s.GetBuildSource),
	}
}

func (s *Service) buildSourceGQLMutationFields() graphql.Fields {
	source := func(p graphql.ResolveParams) (git *BuildSourceGit, image *BuildSourceImage, err error) {
		if git, err = gqlDecodeArg[*BuildSourceGit](p.Args, "git"); err != nil {
			return nil, nil, err
		}
		image, err = gqlDecodeArg[*BuildSourceImage](p.Args, "image")
		return git, image, err
	}
	return graphql.Fields{
		"createBuildSource": &graphql.Field{
			Type: buildSourceGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":     gqlutil.ReqArg(graphql.String),
				"name":        gqlutil.ReqArg(graphql.String),
				"projectId":   gqlutil.Arg(graphql.String),
				"git":         gqlutil.Arg(buildSourceGitInputType),
				"image":       gqlutil.Arg(buildSourceImageInputType),
				"envVars":     gqlutil.Arg(graphql.NewList(gqlutil.EnvVarInputType)),
				"secretFiles": gqlutil.Arg(graphql.NewList(secretFileInputType)),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				git, image, err := source(p)
				if err != nil {
					return nil, err
				}
				envVars, err := gqlDecodeArg[[]envVarInput](p.Args, "envVars")
				if err != nil {
					return nil, err
				}
				secretFiles, err := gqlDecodeArg[[]secretFileInput](p.Args, "secretFiles")
				if err != nil {
					return nil, err
				}
				return s.CreateBuildSource(p.Context, CreateBuildSourceRequest{
					OwnerID: p.Args["ownerId"].(string), Name: p.Args["name"].(string), ProjectID: gqlutil.Str(p.Args, "projectId"),
					Git: git, Image: image, EnvVars: envVars, SecretFiles: secretFiles,
				})
			},
		},
		"updateBuildSource": &graphql.Field{
			Type: buildSourceGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    gqlutil.ReqArg(graphql.String),
				"name":  gqlutil.Arg(graphql.String),
				"git":   gqlutil.Arg(buildSourceGitInputType),
				"image": gqlutil.Arg(buildSourceImageInputType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				git, image, err := source(p)
				if err != nil {
					return nil, err
				}
				return s.UpdateBuildSource(p.Context, p.Args["id"].(string), UpdateBuildSourceRequest{Name: gqlutil.StrPtr(p.Args, "name"), Git: git, Image: image})
			},
		},
		// linkBuildSource is REST's PATCH buildSourceId: the service builds from
		// the source from now on, saved without deploying; startCommand is the
		// only setting that may ride along (Render).
		"linkBuildSource": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"serviceId":     gqlutil.ReqArg(graphql.String),
				"buildSourceId": gqlutil.ReqArg(graphql.String),
				"startCommand":  gqlutil.Arg(graphql.String),
				// dryRun runs every check and writes nothing (REST's ?dryRun).
				"dryRun": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				dryRun, _ := p.Args["dryRun"].(bool)
				return s.LinkBuildSource(p.Context, p.Args["serviceId"].(string), p.Args["buildSourceId"].(string), gqlutil.StrPtr(p.Args, "startCommand"), dryRun)
			},
		},
		"deleteBuildSource": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteBuildSource(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}

type listBuildSourcesArgs struct {
	OwnerID       string   `json:"ownerId,omitempty" jsonschema:"the workspace id; defaults to your workspace"`
	Name          []string `json:"name,omitempty" jsonschema:"only build sources with these names"`
	ProjectID     string   `json:"projectId,omitempty" jsonschema:"only build sources scoped to this project"`
	CreatedBefore string   `json:"createdBefore,omitempty" jsonschema:"RFC3339; only build sources created before this time"`
	CreatedAfter  string   `json:"createdAfter,omitempty" jsonschema:"RFC3339; only build sources created after this time"`
	UpdatedBefore string   `json:"updatedBefore,omitempty" jsonschema:"RFC3339; only build sources updated before this time"`
	UpdatedAfter  string   `json:"updatedAfter,omitempty" jsonschema:"RFC3339; only build sources updated after this time"`
	Cursor        string   `json:"cursor,omitempty" jsonschema:"the cursor of the previous page's last build source"`
	Limit         int      `json:"limit,omitempty" jsonschema:"page size, default 20, at most 100"`
}

type listBuildSourcesResult struct {
	BuildSources []buildSourceWithCursor `json:"buildSources"`
}

type buildSourceIDArgs struct {
	BuildSourceID string `json:"buildSourceId" jsonschema:"the build source id (bds-…)"`
}

type updateBuildSourceArgs struct {
	BuildSourceID string            `json:"buildSourceId" jsonschema:"the build source id (bds-…)"`
	Name          *string           `json:"name,omitempty" jsonschema:"a new name"`
	Git           *BuildSourceGit   `json:"git,omitempty" jsonschema:"git settings to change (a git source only)"`
	Image         *BuildSourceImage `json:"image,omitempty" jsonschema:"image settings to change (an image source only)"`
}

type linkBuildSourceArgs struct {
	ServiceID     string  `json:"serviceId" jsonschema:"the service id, as returned by list_services"`
	BuildSourceID string  `json:"buildSourceId" jsonschema:"the build source id (bds-…) the service builds from from now on"`
	StartCommand  *string `json:"startCommand,omitempty" jsonschema:"the service's start command: required for a native-runtime source, refused for a Docker or image one"`
	DryRun        bool    `json:"dryRun,omitempty" jsonschema:"run every check and change nothing"`
}

type deleteBuildSourceResult struct {
	Deleted bool `json:"deleted"`
}

func (s *Service) registerBuildSourceTools(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_build_sources",
		Description: "List the workspace's build sources: one build feeding the services linked to it, as on Render. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listBuildSourcesArgs) (*mcp.CallToolResult, listBuildSourcesResult, error) {
		f, err := parseBuildSourceFilter(in.Name, func(k string) string {
			return map[string]string{"projectId": in.ProjectID, "createdBefore": in.CreatedBefore, "createdAfter": in.CreatedAfter,
				"updatedBefore": in.UpdatedBefore, "updatedAfter": in.UpdatedAfter}[k]
		})
		if err != nil {
			return nil, listBuildSourcesResult{}, err
		}
		page, err := s.listBuildSourcesPage(ctx, in.OwnerID, f, in.Cursor, core.PageLimitOrDefault(in.Limit))
		return nil, listBuildSourcesResult{BuildSources: toBuildSourceList(page)}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_build_source",
		Description: "Get one build source with its git or image settings and linked services. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in buildSourceIDArgs) (*mcp.CallToolResult, BuildSourceView, error) {
		v, err := s.GetBuildSource(ctx, in.BuildSourceID)
		return nil, v, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_build_source",
		Description: "Create a git- or image-backed build source (exactly one of git or image), optionally seeding its build-time envVars and secretFiles. Change them later with the env-var tools, passing the build source id as serviceId; link services by creating or updating them with buildSourceId. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CreateBuildSourceRequest) (*mcp.CallToolResult, BuildSourceView, error) {
		v, err := s.CreateBuildSource(ctx, in)
		return nil, v, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_build_source",
		Description: "Change a build source's name or settings. Saved without building, as on Render; trigger a build to apply it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateBuildSourceArgs) (*mcp.CallToolResult, BuildSourceView, error) {
		v, err := s.UpdateBuildSource(ctx, in.BuildSourceID, UpdateBuildSourceRequest{Name: in.Name, Git: in.Git, Image: in.Image})
		return nil, v, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "link_service_to_build_source",
		Description: "Make a service build from a build source, as Render's update-service buildSourceId does: its own repo or image is cleared, and the link is saved without deploying (trigger a deploy to use it). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in linkBuildSourceArgs) (*mcp.CallToolResult, renderService, error) {
		return renderServiceResult(s.LinkBuildSource(ctx, in.ServiceID, in.BuildSourceID, in.StartCommand, in.DryRun))
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "delete_build_source",
		Description: "Delete a build source. Refused while services are linked to it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in buildSourceIDArgs) (*mcp.CallToolResult, deleteBuildSourceResult, error) {
		err := s.DeleteBuildSource(ctx, in.BuildSourceID)
		return nil, deleteBuildSourceResult{Deleted: err == nil}, err
	})
}
