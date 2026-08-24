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

	"github.com/bex-co/bex/lego/backend/internal/core"
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
		"id":                 gqlutil.StrField(func(v KeyValueView) any { return v.ID }),
		"name":               gqlutil.StrField(func(v KeyValueView) any { return v.Name }),
		"plan":               gqlutil.StrField(func(v KeyValueView) any { return v.Plan }),
		"version":            gqlutil.StrField(func(v KeyValueView) any { return v.Version }),
		"status":             gqlutil.StrField(func(v KeyValueView) any { return v.Status }),
		"suspended":          gqlutil.StrField(func(v KeyValueView) any { return v.Suspended }),
		"createdAt":          gqlutil.StrField(func(v KeyValueView) any { return v.CreatedAt }),
		"updatedAt":          gqlutil.StrField(func(v KeyValueView) any { return v.UpdatedAt }),
		"region":             gqlutil.StrField(func(v KeyValueView) any { return v.Region }),
		"dashboardUrl":       gqlutil.StrField(func(v KeyValueView) any { return v.DashboardURL }),
		"externalHost":       gqlutil.StrField(func(v KeyValueView) any { return v.ExternalHost }),
		"public":             gqlutil.BoolField(func(v KeyValueView) any { return v.Public }),
		"ipAllowList":        gqlutil.StrsField(func(v KeyValueView) any { return core.AllowListCIDRs(v.IPAllowList) }),
		"ipAllowListEntries": gqlutil.Typed(graphql.NewList(gqlutil.IPAllowEntryType), func(v KeyValueView) any { return v.IPAllowList }),
		"maxmemoryPolicy":    gqlutil.StrField(func(v KeyValueView) any { return v.MaxmemoryPolicy }),
		"persistenceMode":    gqlutil.StrField(func(v KeyValueView) any { return v.PersistenceMode }),
		"ownerId":            gqlutil.StrField(func(v KeyValueView) any { return v.OwnerID }),
		"projectId":          gqlutil.OptionalStrField(func(v KeyValueView) any { return v.ProjectID }),
		"environmentId":      gqlutil.OptionalStrField(func(v KeyValueView) any { return v.EnvironmentID }),
	},
})

// keyValueInstanceTypeGQLType renders KeyValueInstanceType — the create dialog's
// plan-picker source, the managed key-value sibling of databaseInstanceTypes. A
// bex extension (Render's dashboard has no public query to mirror), REST/MCP-free
// by design and recorded in the milestone rather than left silently asymmetric.
var keyValueInstanceTypeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValueInstanceType",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(t KeyValueInstanceType) any { return t.ID }),
		"name":      gqlutil.StrField(func(t KeyValueInstanceType) any { return t.Name }),
		"cpu":       gqlutil.StrField(func(t KeyValueInstanceType) any { return t.CPU }),
		"memory":    gqlutil.StrField(func(t KeyValueInstanceType) any { return t.Memory }),
		"storageGB": gqlutil.IntField(func(t KeyValueInstanceType) any { return t.StorageGB }),
	},
})

var keyValueConnectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValueConnectionInfo",
	Fields: graphql.Fields{
		"internalConnectionString": gqlutil.StrField(func(v KeyValueConnectionInfo) any { return v.InternalConnectionString }),
		"externalConnectionString": gqlutil.StrField(func(v KeyValueConnectionInfo) any { return v.ExternalConnectionString }),
		"cliCommand":               gqlutil.StrField(func(v KeyValueConnectionInfo) any { return v.CLICommand }),
	},
})

// keyValueLogGQLType is one Valkey log line — timestamp, message, and labels
// (instance, type). Mirrors databaseLogGQLType in postgres/graphql.go.
var keyValueLogGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValueLogEntry",
	Fields: graphql.Fields{
		"timestamp": gqlutil.StrField(func(e KeyValueLogEntry) any { return e.Timestamp }),
		"message":   gqlutil.StrField(func(e KeyValueLogEntry) any { return e.Message }),
		"instance":  gqlutil.StrField(func(e KeyValueLogEntry) any { return e.Labels["instance"] }),
		"type":      gqlutil.StrField(func(e KeyValueLogEntry) any { return e.Labels["type"] }),
	},
})

// GraphQLQuery returns the key-value read fields.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"keyValues": &graphql.Field{
			Type: graphql.NewList(keyValueGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				// ownerId mirrors Render's REST/MCP key-value list filter (w6/m4/t002).
				"ownerId": gqlutil.Arg(graphql.String),
				// Keep the dashboard's existing [KeyValue] result shape; these optional
				// args only select a stable page when a caller opts in.
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ownerID := gqlutil.Str(p.Args, "ownerId")
				out, err := s.ListKeyValues(p.Context, ownerID)
				if err != nil {
					return nil, err
				}
				return gqlutil.Page(p, out, func(kv KeyValueView) string { return kv.ID }), nil
			},
		},
		"keyValue":               gqlutil.IDVerb(keyValueGQLType, s.GetKeyValue),
		"keyValueConnectionInfo": gqlutil.IDVerb(keyValueConnectionInfoGQLType, s.KeyValueConnectionInfo),
		"keyValueInstanceTypes": &graphql.Field{ // bex extension backing the create dialog's plan picker
			Type:    graphql.NewList(keyValueInstanceTypeGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.InstanceTypes(p.Context) },
		},
		"keyValueIpAllowList": &graphql.Field{ // strings; the KeyValue type's ipAllowListEntries carries descriptions
			Type: graphql.NewList(graphql.String),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				list, err := s.GetIPAllowList(p.Context, p.Args["id"].(string))
				if err != nil {
					return nil, err
				}
				return core.AllowListCIDRs(list), nil
			},
		},
		// --- logs (w3/m30) ---
		"keyValueLogs": &graphql.Field{
			Type: graphql.NewList(keyValueLogGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"id":        gqlutil.ReqArg(graphql.String),
				"text":      gqlutil.Arg(graphql.String),
				"startTime": gqlutil.Arg(graphql.String),
				"endTime":   gqlutil.Arg(graphql.String),
				"direction": gqlutil.Arg(graphql.String),
				"instance":  gqlutil.Arg(graphql.NewList(graphql.String)),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				since, end, err := core.ParseTimeWindow(gqlutil.Str(p.Args, "startTime"), gqlutil.Str(p.Args, "endTime"))
				if err != nil {
					return nil, err
				}
				var limit int64
				if v, ok := p.Args["limit"].(int); ok {
					limit = int64(v)
				}
				return s.QueryKeyValueLogs(p.Context, p.Args["id"].(string), KeyValueLogQuery{
					Search:    gqlutil.Str(p.Args, "text"),
					Since:     since,
					End:       end,
					Limit:     limit,
					Direction: gqlutil.Str(p.Args, "direction"),
					Instance:  gqlutil.StringList(p.Args["instance"]),
				})
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
				"name": gqlutil.ReqArg(graphql.String),
				// ownerId is the workspace to create IN (w6/m14) — the write-side
				// twin of the key-value list filter; optional, defaulting to the
				// caller's default workspace, forbidden for a non-member.
				"ownerId":       gqlutil.Arg(graphql.String),
				"environmentId": gqlutil.Arg(graphql.String),
				"plan":          gqlutil.Arg(graphql.String),
				"version":       gqlutil.Arg(graphql.String),
				"storageGB":     gqlutil.Arg(graphql.Int),
				"public":        gqlutil.Arg(graphql.Boolean),
				"ipAllowList":   gqlutil.Arg(graphql.NewList(graphql.String)),
				// ipAllowListEntries is the description-carrying form (w4/m24);
				// when present it wins over ipAllowList.
				"ipAllowListEntries": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))),
				"maxmemoryPolicy":    gqlutil.Arg(graphql.String),
				"persistenceMode":    gqlutil.Arg(graphql.String),
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateKeyValue(p.Context, CreateKeyValueRequest{
					Name:            p.Args["name"].(string),
					OwnerID:         gqlutil.Str(p.Args, "ownerId"),
					EnvironmentID:   gqlutil.Str(p.Args, "environmentId"),
					Plan:            gqlutil.Str(p.Args, "plan"),
					Version:         gqlutil.Str(p.Args, "version"),
					StorageGB:       int32(gqlutil.Int(p.Args, "storageGB")),
					Public:          gqlutil.Bool(p.Args, "public"),
					MaxmemoryPolicy: gqlutil.Str(p.Args, "maxmemoryPolicy"),
					PersistenceMode: gqlutil.Str(p.Args, "persistenceMode"),
					IPAllowList: core.AllowListOrCIDRs(
						gqlutil.AllowList(p.Args["ipAllowListEntries"]), gqlutil.StringList(p.Args["ipAllowList"])),
					DryRun: gqlutil.Bool(p.Args, "dryRun"),
				})
			},
		},
		"deleteKeyValue": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"confirm": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteKeyValue(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["id"].(string))
				return err == nil, err
			},
		},
		"updateKeyValuePlan": gqlutil.PlanMutation(keyValueGQLType, s.SetPlan, s.PreviewSetPlan),
		// setKeyValueMaxmemoryPolicy is the GraphQL/MCP mirror of the REST PATCH's
		// maxmemoryPolicy field (w7/m45): the per-field verb pattern updateKeyValuePlan
		// / setKeyValueIpAllowList already follow, routed through the shared
		// UpdateKeyValue so all surfaces normalize + validate the policy identically.
		"setKeyValueMaxmemoryPolicy": gqlutil.PatchMutation(keyValueGQLType, "maxmemoryPolicy",
			func(policy string) KeyValuePatch { return KeyValuePatch{MaxmemoryPolicy: &policy} },
			s.UpdateKeyValue, s.PreviewUpdateKeyValue),
		"renameKeyValue": gqlutil.PatchMutation(keyValueGQLType, "name",
			func(name string) KeyValuePatch { return KeyValuePatch{Name: &name} },
			s.UpdateKeyValue, s.PreviewUpdateKeyValue),
		"suspendKeyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"confirm": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Suspend(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["id"].(string))
			},
		},
		"resumeKeyValue": gqlutil.IDVerb(keyValueGQLType, s.Resume),
		"setKeyValueIpAllowList": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    gqlutil.ReqArg(graphql.String),
				"cidrs": gqlutil.Arg(graphql.NewList(graphql.String)),
				// entries is the description-carrying form; precedence over cidrs
				// lives in core.AllowListOrCIDRs.
				"entries": gqlutil.Arg(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				entries := core.AllowListOrCIDRs(gqlutil.AllowList(p.Args["entries"]), gqlutil.StringList(p.Args["cidrs"]))
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), entries)
			},
		},
	}
}
