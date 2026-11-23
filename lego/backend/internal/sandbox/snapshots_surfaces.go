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
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// The REST, GraphQL and MCP surfaces of sandbox snapshots (w32/m285). All
// three call the same Service verbs, so a refusal carries one code.

// registerSnapshotREST mounts Render's four sandbox-snapshot routes.
func (s *Service) registerSnapshotREST(mux *http.ServeMux) {
	s.handle(mux, "POST /v1/sandboxes/{id}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		// Render's body is optional. An empty one asks for its default kind,
		// which the Service refuses by name.
		var req CreateSnapshotRequest
		r = r.WithContext(core.WithStrictJSONDecoding(r.Context()))
		if err := core.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		ctx := core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId"))
		snap, err := s.CreateSnapshot(ctx, r.PathValue("id"), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusAccepted, snap)
	})
	s.handle(mux, "GET /v1/sandbox-groups/{sandboxGroupId}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		after, limit := core.PageParams(query)
		snaps, err := s.ListSnapshots(core.WithWorkspace(r.Context(), query.Get("ownerId")), r.PathValue("sandboxGroupId"),
			query.Get("name"), core.QueryList(query, "status"), after, limit)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		envelopes := make([]SnapshotEnvelope, 0, len(snaps))
		for _, snap := range snaps {
			envelopes = append(envelopes, SnapshotEnvelope{Snapshot: snap, Cursor: snap.ID})
		}
		core.WriteJSON(w, http.StatusOK, envelopes)
	})
	s.handle(mux, "GET /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}", func(w http.ResponseWriter, r *http.Request) {
		snap, err := s.GetSnapshot(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")), r.PathValue("sandboxGroupId"), r.PathValue("snapshotId"))
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, snap)
	})
	s.handle(mux, "DELETE /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}", core.HandleNoBody(http.StatusNoContent, func(r *http.Request) error {
		return s.DeleteSnapshot(core.WithWorkspace(r.Context(), r.URL.Query().Get("ownerId")), r.PathValue("sandboxGroupId"), r.PathValue("snapshotId"))
	}))
}

// sandboxSnapshotGQLType is the GraphQL projection of a Snapshot. Byte and
// second counts are Float, GraphQL's Int being 32 bits.
var sandboxSnapshotGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxSnapshot",
	Fields: graphql.Fields{
		"id":              gqlutil.ReqStrField(func(sn Snapshot) any { return sn.ID }),
		"sandboxGroupId":  gqlutil.ReqStrField(func(sn Snapshot) any { return sn.SandboxGroupID }),
		"sourceSandboxId": gqlutil.ReqStrField(func(sn Snapshot) any { return sn.SourceSandboxID }),
		"name":            gqlutil.StrField(func(sn Snapshot) any { return sn.Name }),
		"kind":            gqlutil.ReqStrField(func(sn Snapshot) any { return sn.Kind }),
		"status":          gqlutil.ReqStrField(func(sn Snapshot) any { return sn.Status }),
		"plan":            gqlutil.ReqStrField(func(sn Snapshot) any { return sn.Plan }),
		"requestedAt":     gqlutil.TimeField(func(sn Snapshot) any { return sn.RequestedAt }),
		"capturedAt":      gqlutil.TimeField(func(sn Snapshot) any { return sn.CapturedAt }),
		"expiresAt":       gqlutil.TimeField(func(sn Snapshot) any { return sn.ExpiresAt }),
		"ttlSeconds":      gqlutil.FloatField(func(sn Snapshot) any { return float64(sn.TTLSeconds) }),
		"sizeBytes": gqlutil.FloatField(func(sn Snapshot) any {
			if sn.SizeBytes == nil {
				return nil
			}
			return float64(*sn.SizeBytes)
		}),
		"error": gqlutil.StrField(func(sn Snapshot) any { return sn.Error }),
	},
})

func (s *Service) snapshotQueryFields() graphql.Fields {
	return graphql.Fields{
		"sandboxSnapshots": &graphql.Field{
			Type: graphql.NewList(sandboxSnapshotGQLType),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"name":           gqlutil.Arg(graphql.String),
				"status":         gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
				"ownerId":        gqlutil.Arg(graphql.String),
				"cursor":         gqlutil.Arg(graphql.String),
				"limit":          gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListSnapshots(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), gqlutil.Str(p.Args, "name"),
					gqlutil.StringList(p.Args["status"]), gqlutil.Str(p.Args, "cursor"), gqlutil.Int(p.Args, "limit"))
			},
		},
		"sandboxSnapshot": &graphql.Field{
			Type: sandboxSnapshotGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetSnapshot(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) snapshotMutationFields() graphql.Fields {
	return graphql.Fields{
		"createSandboxSnapshot": &graphql.Field{
			Type: sandboxSnapshotGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxId": gqlutil.ReqArg(graphql.String),
				// Render's kind: `filesystem`. Omitted is Render's default,
				// `runtime`, which is refused by name.
				"kind": gqlutil.Arg(graphql.String),
				"name": gqlutil.Arg(graphql.String),
				// Render's ttlSeconds (0 keeps the snapshot until it is
				// deleted) or its deprecated expiresAt, an RFC 3339 time.
				"ttlSeconds": gqlutil.Arg(graphql.Int),
				"expiresAt":  gqlutil.Arg(graphql.String),
				"ownerId":    gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := CreateSnapshotRequest{Kind: gqlutil.Str(p.Args, "kind")}
				if name, ok := p.Args["name"].(string); ok {
					req.Name = &name
				}
				if ttl := gqlutil.IntPtr(p.Args, "ttlSeconds"); ttl != nil {
					seconds := int64(*ttl)
					req.TTLSeconds = &seconds
				}
				if raw, ok := p.Args["expiresAt"].(string); ok {
					at, err := time.Parse(time.RFC3339, raw)
					if err != nil {
						return nil, core.NewBadRequestError(CodeSandboxSnapshotInvalid, "expiresAt must be an RFC 3339 time",
							map[string]any{"field": "expiresAt"})
					}
					req.ExpiresAt = &at
				}
				return s.CreateSnapshot(gqlutil.OwnerScoped(p), p.Args["sandboxId"].(string), req)
			},
		},
		"deleteSandboxSnapshot": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteSnapshot(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string)); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
	}
}

type createSnapshotArgs struct {
	OwnerID   string `json:"ownerId,omitempty"`
	SandboxID string `json:"sandboxId"`
	CreateSnapshotRequest
}

type listSnapshotsArgs struct {
	SandboxGroupID string   `json:"sandboxGroupId"`
	Name           string   `json:"name,omitempty"`
	Status         []string `json:"status,omitempty"`
	Cursor         string   `json:"cursor,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type listSnapshotsResult struct {
	SandboxSnapshots []Snapshot `json:"sandboxSnapshots"`
}

type getSnapshotArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	ID             string `json:"id"`
}

func (s *Service) registerSnapshotMCP(server *mcp.Server) {
	addTool(s, server, &mcp.Tool{Name: "create_sandbox_snapshot", Description: "Capture a snapshot of a running sandbox's filesystem (sandboxId), so new sandboxes can start from it (spawn_sandbox snapshotId or snapshotName). kind must be filesystem: Render's default, runtime, also saves memory and running processes, which bex cannot capture yet. The sandbox keeps running. Returns at once with status creating; poll get_sandbox_snapshot until it is available or failed (error says why). Optional name (at most 64 characters, never snp-…) lets a create name it; names can repeat and the newest available one wins. ttlSeconds is its lifetime after its last use (default 30 days; 0 keeps it until it is deleted), restarted each time a sandbox is created from it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createSnapshotArgs) (*mcp.CallToolResult, Snapshot, error) {
			snap, err := s.CreateSnapshot(core.WithWorkspace(ctx, in.OwnerID), in.SandboxID, in.CreateSnapshotRequest)
			return nil, snap, err
		})
	addTool(s, server, &mcp.Tool{Name: "get_sandbox_snapshot", Description: "Get one snapshot of a sandbox group, with its status, size and expiry. A deleted or expired snapshot is not found."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getSnapshotArgs) (*mcp.CallToolResult, Snapshot, error) {
			snap, err := s.GetSnapshot(ctx, in.SandboxGroupID, in.ID)
			return nil, snap, err
		})
	addTool(s, server, &mcp.Tool{Name: "delete_sandbox_snapshot", Description: "Delete a snapshot of a sandbox group, as Render's delete-sandbox-snapshot. It no longer lists, reads or starts sandboxes; sandboxes already created from it are not affected, and deleting it again succeeds. Refused with snapshot_creating while its capture is still in flight.", Annotations: mcputil.DestructiveAnnotations(true)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getSnapshotArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.DeleteSnapshot(ctx, in.SandboxGroupID, in.ID); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{ID: in.ID, Deleted: true}, nil
		})
	addTool(s, server, &mcp.Tool{Name: "list_sandbox_snapshots", Description: "List the snapshots of a sandbox group, newest first, without deleted and expired ones. Optional name keeps one name's versions; optional status keeps creating, available or failed ones."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listSnapshotsArgs) (*mcp.CallToolResult, listSnapshotsResult, error) {
			out, err := s.ListSnapshots(ctx, in.SandboxGroupID, in.Name, in.Status, in.Cursor, in.Limit)
			return nil, listSnapshotsResult{SandboxSnapshots: out}, err
		})
}
