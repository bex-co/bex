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
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// parseKVGQLTimeWindow parses optional startTime/endTime RFC3339 string args
// from a GraphQL resolver's args map — same contract as the REST log handlers.
func parseKVGQLTimeWindow(args map[string]any) (since, end time.Time, err error) {
	s, _ := args["startTime"].(string)
	e, _ := args["endTime"].(string)
	return core.ParseTimeWindow(s, e)
}

// The GraphQL noun is "keyValue" — matching bex's own KeyValue CRD and Render's
// current "Key Value" product branding (the same way the postgres feature's
// GraphQL noun "database" matches its Database CRD). If a live Render dashboard
// capture ever shows a legacy "redis" noun, that is a rename follow-up recorded
// in docs/ADR018-render-parity.md, not a silent divergence.
var keyValueGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValue",
	Fields: graphql.Fields{
		"id":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.ID })},
		"name":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Name })},
		"plan":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Plan })},
		"version":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Version })},
		"status":             &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Status })},
		"suspended":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Suspended })},
		"createdAt":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.CreatedAt })},
		"updatedAt":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.UpdatedAt })},
		"region":             &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Region })},
		"dashboardUrl":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.DashboardURL })},
		"externalHost":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.ExternalHost })},
		"public":             &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.Public })},
		"ipAllowList":        &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v KeyValueView) any { return core.AllowListCIDRs(v.IPAllowList) })},
		"ipAllowListEntries": &graphql.Field{Type: graphql.NewList(gqlutil.IPAllowEntryType), Resolve: gqlutil.Field(func(v KeyValueView) any { return v.IPAllowList })},
		"maxmemoryPolicy":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.MaxmemoryPolicy })},
		"persistenceMode":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.PersistenceMode })},
		"ownerId":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.OwnerID })},
		"projectId":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.ProjectID })},
		"environmentId":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v KeyValueView) any { return v.EnvironmentID })},
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

// keyValueLogGQLType is one Valkey log line — timestamp, message, and labels
// (instance, type). Mirrors databaseLogGQLType in postgres/graphql.go.
var keyValueLogGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "KeyValueLogEntry",
	Fields: graphql.Fields{
		"timestamp": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e KeyValueLogEntry) any { return e.Timestamp })},
		"message":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e KeyValueLogEntry) any { return e.Message })},
		"instance":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e KeyValueLogEntry) any { return e.Labels["instance"] })},
		"type":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e KeyValueLogEntry) any { return e.Labels["type"] })},
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
				// Keep the dashboard's existing [KeyValue] result shape; these optional
				// args only select a stable page when a caller opts in.
				"cursor": &graphql.ArgumentConfig{Type: graphql.String},
				"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ownerID, _ := p.Args["ownerId"].(string)
				out, err := s.ListKeyValues(p.Context, ownerID)
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
				return core.StablePage(out, cursor, limit, cursorSet || limitSet, func(kv KeyValueView) string { return kv.ID }), nil
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
			Args: graphql.FieldConfigArgument{
				"id":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"text":      &graphql.ArgumentConfig{Type: graphql.String},
				"startTime": &graphql.ArgumentConfig{Type: graphql.String},
				"endTime":   &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
				"direction": &graphql.ArgumentConfig{Type: graphql.String},
				"instance":  &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				since, end, err := parseKVGQLTimeWindow(p.Args)
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
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				// ownerId is the workspace to create IN (w6/m14) — the write-side
				// twin of the key-value list filter; optional, defaulting to the
				// caller's default workspace, forbidden for a non-member.
				"ownerId":       &graphql.ArgumentConfig{Type: graphql.String},
				"environmentId": &graphql.ArgumentConfig{Type: graphql.String},
				"plan":          &graphql.ArgumentConfig{Type: graphql.String},
				"version":       &graphql.ArgumentConfig{Type: graphql.String},
				"storageGB":     &graphql.ArgumentConfig{Type: graphql.Int},
				"public":        &graphql.ArgumentConfig{Type: graphql.Boolean},
				"ipAllowList":   &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
				// ipAllowListEntries is the description-carrying form (w4/m24);
				// when present it wins over ipAllowList.
				"ipAllowListEntries": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))},
				"maxmemoryPolicy":    &graphql.ArgumentConfig{Type: graphql.String},
				"persistenceMode":    &graphql.ArgumentConfig{Type: graphql.String},
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ownerID, _ := p.Args["ownerId"].(string)
				req := CreateKeyValueRequest{Name: p.Args["name"].(string), OwnerID: ownerID}
				if v, ok := p.Args["environmentId"].(string); ok {
					req.EnvironmentID = v
				}
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
				if v, ok := p.Args["maxmemoryPolicy"].(string); ok {
					req.MaxmemoryPolicy = v
				}
				if v, ok := p.Args["persistenceMode"].(string); ok {
					req.PersistenceMode = v
				}
				req.IPAllowList = core.AllowListOrCIDRs(gqlutil.AllowList(p.Args["ipAllowListEntries"]), gqlutil.StringList(p.Args["ipAllowList"]))
				req.DryRun, _ = p.Args["dryRun"].(bool)
				return s.CreateKeyValue(p.Context, req)
			},
		},
		"deleteKeyValue": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"confirm": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteKeyValue(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["id"].(string))
				return err == nil, err
			},
		},
		"updateKeyValuePlan": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				dryRun, _ := p.Args["dryRun"].(bool)
				if dryRun {
					return s.PreviewSetPlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
				}
				return s.SetPlan(p.Context, p.Args["id"].(string), p.Args["plan"].(string))
			},
		},
		// setKeyValueMaxmemoryPolicy is the GraphQL/MCP mirror of the REST PATCH's
		// maxmemoryPolicy field (w7/m45): the per-field verb pattern updateKeyValuePlan
		// / setKeyValueIpAllowList already follow, routed through the shared
		// UpdateKeyValue so all surfaces normalize + validate the policy identically.
		"setKeyValueMaxmemoryPolicy": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":              &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"maxmemoryPolicy": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"dryRun":          &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				policy := p.Args["maxmemoryPolicy"].(string)
				patch := KeyValuePatch{MaxmemoryPolicy: &policy}
				if dryRun, _ := p.Args["dryRun"].(bool); dryRun {
					return s.PreviewUpdateKeyValue(p.Context, p.Args["id"].(string), patch)
				}
				return s.UpdateKeyValue(p.Context, p.Args["id"].(string), patch)
			},
		},
		"renameKeyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"dryRun": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				name := p.Args["name"].(string)
				patch := KeyValuePatch{Name: &name}
				dryRun, _ := p.Args["dryRun"].(bool)
				if dryRun {
					return s.PreviewUpdateKeyValue(p.Context, p.Args["id"].(string), patch)
				}
				return s.UpdateKeyValue(p.Context, p.Args["id"].(string), patch)
			},
		},
		"suspendKeyValue": &graphql.Field{
			Type: keyValueGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"confirm": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Suspend(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["id"].(string))
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
				// entries is the description-carrying form; precedence over cidrs
				// lives in core.AllowListOrCIDRs.
				"entries": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				entries := core.AllowListOrCIDRs(gqlutil.AllowList(p.Args["entries"]), gqlutil.StringList(p.Args["cidrs"]))
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), entries)
			},
		},
	}
}
