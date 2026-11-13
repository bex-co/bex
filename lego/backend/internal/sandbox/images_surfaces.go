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

package sandbox

import (
	"context"
	"net/http"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// The REST, GraphQL and MCP surfaces of imported sandbox images (w2/m358).
// All three call the same Service verbs, so a refusal carries one code.

// registerImageREST mounts Render's three sandbox-build routes.
func (s *Service) registerImageREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sandbox-groups/{sandboxGroupId}/builds", func(w http.ResponseWriter, r *http.Request) {
		var req ImportImageRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		b, err := s.ImportImage(r.Context(), r.PathValue("sandboxGroupId"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusCreated, b)
	})
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}/builds", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		after, limit := core.PageParams(query)
		builds, err := s.ListImages(r.Context(), r.PathValue("sandboxGroupId"), query.Get("name"), core.QueryList(query, "status"), after, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		envelopes := make([]BuildEnvelope, 0, len(builds))
		for _, b := range builds {
			envelopes = append(envelopes, BuildEnvelope{Build: b, Cursor: b.ID})
		}
		core.WriteJSON(w, http.StatusOK, envelopes)
	})
	mux.HandleFunc("GET /v1/sandbox-groups/{sandboxGroupId}/builds/{buildId}", func(w http.ResponseWriter, r *http.Request) {
		b, err := s.GetImage(r.Context(), r.PathValue("sandboxGroupId"), r.PathValue("buildId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, b)
	})
}

var sandboxImageSourceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxImageSource",
	Fields: graphql.Fields{
		"repository": gqlutil.ReqStrField(func(s BuildSource) any { return s.Repository }),
		"image":      gqlutil.ReqStrField(func(s BuildSource) any { return s.Image }),
		"reference":  gqlutil.ReqStrField(func(s BuildSource) any { return s.Reference }),
	},
})

// sandboxImageGQLType is the GraphQL projection of a Build.
var sandboxImageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxImage",
	Fields: graphql.Fields{
		"id":             gqlutil.ReqStrField(func(b Build) any { return b.ID }),
		"sandboxGroupId": gqlutil.ReqStrField(func(b Build) any { return b.SandboxGroupID }),
		"name":           gqlutil.ReqStrField(func(b Build) any { return b.Name }),
		"source":         gqlutil.Typed(graphql.NewNonNull(sandboxImageSourceGQLType), func(b Build) any { return b.Source }),
		"sourceDigest":   gqlutil.ReqStrField(func(b Build) any { return b.SourceDigest }),
		"status":         gqlutil.ReqStrField(func(b Build) any { return b.Status }),
		"failureReason":  gqlutil.StrField(func(b Build) any { return b.FailureReason }),
		"createdAt":      gqlutil.TimeField(func(b Build) any { return b.CreatedAt }),
		"finishedAt":     gqlutil.TimeField(func(b Build) any { return b.FinishedAt }),
	},
})

func (s *Service) imageQueryFields() graphql.Fields {
	return graphql.Fields{
		"sandboxImages": &graphql.Field{
			Type: graphql.NewList(sandboxImageGQLType),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"name":           gqlutil.Arg(graphql.String),
				"status":         gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"ownerId":        gqlutil.Arg(graphql.String),
				"cursor":         gqlutil.Arg(graphql.String),
				"limit":          gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListImages(ownerScoped(p), p.Args["sandboxGroupId"].(string), gqlutil.Str(p.Args, "name"),
					gqlutil.StringList(p.Args["status"]), gqlutil.Str(p.Args, "cursor"), gqlutil.Int(p.Args, "limit"))
			},
		},
		"sandboxImage": &graphql.Field{
			Type: sandboxImageGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetImage(ownerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) imageMutationFields() graphql.Fields {
	return graphql.Fields{
		"importSandboxImage": &graphql.Field{
			Type: sandboxImageGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"name":           gqlutil.ReqArg(graphql.String),
				"sourceImage":    gqlutil.ReqArg(graphql.String),
				// A private registry's credential, used for this import only.
				"registryUsername": gqlutil.Arg(graphql.String),
				"registryPassword": gqlutil.Arg(graphql.String),
				"ownerId":          gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := ImportImageRequest{Name: p.Args["name"].(string), SourceImage: p.Args["sourceImage"].(string)}
				if user, pass := gqlutil.Str(p.Args, "registryUsername"), gqlutil.Str(p.Args, "registryPassword"); user != "" || pass != "" {
					req.RegistryCredential = &RegistryCredential{Username: user, Password: pass}
				}
				return s.ImportImage(ownerScoped(p), p.Args["sandboxGroupId"].(string), req)
			},
		},
	}
}

type listImagesArgs struct {
	SandboxGroupID string   `json:"sandboxGroupId"`
	Name           string   `json:"name,omitempty"`
	Status         []string `json:"status,omitempty"`
	Cursor         string   `json:"cursor,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type listImagesResult struct {
	SandboxImages []Build `json:"sandboxImages"`
}

type getImageArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	ID             string `json:"id"`
}

type importImageArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	ImportImageRequest
}

func (s *Service) registerImageMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{Name: "import_sandbox_image", Description: "Import an OCI image (sourceImage, by tag or digest, e.g. python:3.13) into a sandbox group under a name, so sandboxes can start from it (spawn_sandbox buildId or buildName). Returns at once with status pending or running; poll get_sandbox_image until it is succeeded or failed (failureReason says why). Only linux/amd64 images run. registryCredential {username, password} pulls from a private registry and is used for this import only, never stored. Importing a name and digest the group already has returns that image."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in importImageArgs) (*mcp.CallToolResult, Build, error) {
			b, err := s.ImportImage(ctx, in.SandboxGroupID, in.ImportImageRequest)
			return nil, b, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_sandbox_image", Description: "Get one image imported into a sandbox group, with its status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getImageArgs) (*mcp.CallToolResult, Build, error) {
			b, err := s.GetImage(ctx, in.SandboxGroupID, in.ID)
			return nil, b, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "list_sandbox_images", Description: "List the images imported into a sandbox group, newest first. Optional name keeps one image's versions; optional status keeps pending, running, succeeded or failed ones."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listImagesArgs) (*mcp.CallToolResult, listImagesResult, error) {
			out, err := s.ListImages(ctx, in.SandboxGroupID, in.Name, in.Status, in.Cursor, in.Limit)
			return nil, listImagesResult{SandboxImages: out}, err
		})
}
