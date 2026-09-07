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

package postgres

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// Render's dashboard GraphQL calls a managed Postgres a "database" (query
// database(id), databaseStatusQuery, ...) — captured live — even though its REST
// noun is "postgres". bex mirrors that split: REST /v1/postgres, GraphQL
// database* (which also matches bex's own Database CRD).
var postgresGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Database",
	Fields: graphql.Fields{
		"id":                      gqlutil.StrField(func(v PostgresView) any { return v.ID }),
		"name":                    gqlutil.StrField(func(v PostgresView) any { return v.Name }),
		"plan":                    gqlutil.StrField(func(v PostgresView) any { return v.Plan }),
		"version":                 gqlutil.StrField(func(v PostgresView) any { return v.Version }),
		"status":                  gqlutil.StrField(func(v PostgresView) any { return v.Status }),
		"databaseName":            gqlutil.StrField(func(v PostgresView) any { return v.DatabaseName }),
		"databaseUser":            gqlutil.StrField(func(v PostgresView) any { return v.DatabaseUser }),
		"diskSizeGB":              gqlutil.IntField(func(v PostgresView) any { return v.DiskSizeGB }),
		"diskAutoscalingEnabled":  gqlutil.BoolField(func(v PostgresView) any { return v.DiskAutoscalingEnabled }),
		"highAvailabilityEnabled": gqlutil.BoolField(func(v PostgresView) any { return v.HighAvailabilityEnabled }),
		"readReplicas":            gqlutil.Typed(graphql.NewList(readReplicaViewGQLType), func(v PostgresView) any { return v.ReadReplicas }),
		"suspended":               gqlutil.StrField(func(v PostgresView) any { return v.Suspended }),
		"createdAt":               gqlutil.StrField(func(v PostgresView) any { return v.CreatedAt }),
		"updatedAt":               gqlutil.StrField(func(v PostgresView) any { return v.UpdatedAt }),
		"region":                  gqlutil.StrField(func(v PostgresView) any { return v.Region }),
		"dashboardUrl":            gqlutil.StrField(func(v PostgresView) any { return v.DashboardURL }),
		"externalHost":            gqlutil.StrField(func(v PostgresView) any { return v.ExternalHost }),
		"public":                  gqlutil.BoolField(func(v PostgresView) any { return v.Public }),
		"ipAllowList":             gqlutil.StrsField(func(v PostgresView) any { return core.AllowListCIDRs(v.IPAllowList) }),
		"ipAllowListEntries":      gqlutil.Typed(graphql.NewList(gqlutil.IPAllowEntryType), func(v PostgresView) any { return v.IPAllowList }),
		"poolerEnabled":           gqlutil.BoolField(func(v PostgresView) any { return v.PoolerEnabled }),
		"connectionPool":          gqlutil.StrField(func(v PostgresView) any { return v.ConnectionPool }),
		"backupsEnabled":          gqlutil.BoolField(func(v PostgresView) any { return v.BackupsEnabled }),
		"ownerId":                 gqlutil.StrField(func(v PostgresView) any { return v.OwnerID }),
		"projectId":               gqlutil.OptionalStrField(func(v PostgresView) any { return v.ProjectID }),
		"environmentId":           gqlutil.OptionalStrField(func(v PostgresView) any { return v.EnvironmentID }),
	},
})

// readReplicaConnectionInfoGQLType is the host-only connection info for one
// named read replica as surfaced in the postgres view. Passwords are omitted;
// use databaseConnectionInfo for credentials.
var readReplicaConnectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ReadReplicaConnectionInfo",
	Fields: graphql.Fields{
		"internalHost": gqlutil.StrField(func(v ReadReplicaConnectionInfo) any { return v.InternalHost }),
		"externalHost": gqlutil.StrField(func(v ReadReplicaConnectionInfo) any { return v.ExternalHost }),
	},
})

// readReplicaViewGQLType is one named read replica entry in a postgres object.
// Render's readReplicas: [{name, connectionInfo}].
var readReplicaViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ReadReplicaView",
	Fields: graphql.Fields{
		"name":           gqlutil.StrField(func(v ReadReplicaView) any { return v.Name }),
		"connectionInfo": gqlutil.Typed(readReplicaConnectionInfoGQLType, func(v ReadReplicaView) any { return v.ConnectionInfo }),
	},
})

// replicaConnectionStringsGQLType is one entry in databaseConnectionInfo's
// readReplicaConnectionStrings — the full connection strings (with password)
// for a named read replica.
var replicaConnectionStringsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ReplicaConnectionStrings",
	Fields: graphql.Fields{
		"name":                     gqlutil.StrField(func(v ReplicaConnectionStrings) any { return v.Name }),
		"internalConnectionString": gqlutil.StrField(func(v ReplicaConnectionStrings) any { return v.InternalConnectionString }),
		"externalConnectionString": gqlutil.StrField(func(v ReplicaConnectionStrings) any { return v.ExternalConnectionString }),
	},
})

// databaseBackupGQLType is one physical base backup in object storage.
var databaseBackupGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseBackup",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(v BackupView) any { return v.ID }),
		"status":    gqlutil.StrField(func(v BackupView) any { return v.Status }),
		"createdAt": gqlutil.StrField(func(v BackupView) any { return v.CreatedAt }),
	},
})

// databaseExportGQLType is one Render-shaped logical export. url is freshly
// presigned on every list read after can_view_sensitive authorization.
var databaseExportGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseExport",
	Fields: graphql.Fields{
		"id":            gqlutil.StrField(func(v ExportView) any { return v.ID }),
		"createdAt":     gqlutil.StrField(func(v ExportView) any { return v.CreatedAt }),
		"status":        gqlutil.StrField(func(v ExportView) any { return v.Status }),
		"url":           gqlutil.StrField(func(v ExportView) any { return v.URL }),
		"urlExpiresAt":  gqlutil.StrField(func(v ExportView) any { return v.URLExpiresAt }),
		"expiresAt":     gqlutil.StrField(func(v ExportView) any { return v.ExpiresAt }),
		"filename":      gqlutil.StrField(func(v ExportView) any { return v.Filename }),
		"failureReason": gqlutil.StrField(func(v ExportView) any { return v.FailureReason }),
	},
})

// databaseRecoveryInfoGQLType mirrors Render's postgres recovery info.
var databaseRecoveryInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseRecoveryInfo",
	Fields: graphql.Fields{
		"enabled":              gqlutil.BoolField(func(v RecoveryInfoView) any { return v.Enabled }),
		"earliestRecoveryTime": gqlutil.StrField(func(v RecoveryInfoView) any { return v.EarliestRecoveryTime }),
		"latestRecoveryTime":   gqlutil.StrField(func(v RecoveryInfoView) any { return v.LatestRecoveryTime }),
		"backups":              gqlutil.Typed(graphql.NewList(databaseBackupGQLType), func(v RecoveryInfoView) any { return v.Backups }),
	},
})

// databaseUserGQLType is one additional managed login role (no password).
var databaseUserGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseUser",
	Fields: graphql.Fields{
		"name": gqlutil.StrField(func(v PostgresUserView) any { return v.Name }),
	},
})

// databaseUserWithPasswordGQLType is createDatabaseUser's one-time result, the
// only place a role's generated password is surfaced.
var databaseUserWithPasswordGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseUserWithPassword",
	Fields: graphql.Fields{
		"name":     gqlutil.StrField(func(v CreateUserResult) any { return v.Name }),
		"password": gqlutil.StrField(func(v CreateUserResult) any { return v.Password }),
	},
})

// databaseInstanceTypeGQLType renders DatabaseInstanceType — the create
// dialog's plan-picker source, the managed-Postgres sibling of apps'
// instanceTypes. A bex extension (see DatabaseInstanceType's doc comment):
// Render's dashboard has no public query to mirror, so this is REST/MCP-free
// by design, recorded in w5/m8's README rather than left silently asymmetric.
var databaseInstanceTypeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseInstanceType",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(t DatabaseInstanceType) any { return t.ID }),
		"name":      gqlutil.StrField(func(t DatabaseInstanceType) any { return t.Name }),
		"cpu":       gqlutil.StrField(func(t DatabaseInstanceType) any { return t.CPU }),
		"memory":    gqlutil.StrField(func(t DatabaseInstanceType) any { return t.Memory }),
		"storageGB": gqlutil.IntField(func(t DatabaseInstanceType) any { return t.StorageGB }),
	},
})

var connectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresConnectionInfo",
	Fields: graphql.Fields{
		"password":                     gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.Password }),
		"internalConnectionString":     gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.InternalConnectionString }),
		"externalConnectionString":     gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.ExternalConnectionString }),
		"internalConnectionPoolString": gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.InternalConnectionPoolString }),
		"externalConnectionPoolString": gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.ExternalConnectionPoolString }),
		"psqlCommand":                  gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.PSQLCommand }),
		"serverCaCertificate":          gqlutil.StrField(func(v PostgresConnectionInfo) any { return v.ServerCACertificate }),
		"readReplicaConnectionStrings": gqlutil.Typed(graphql.NewList(replicaConnectionStringsGQLType), func(v PostgresConnectionInfo) any { return v.ReadReplicaConnectionStrings }),
	},
})

// parameterInputGQLType is the {name, value} input pair for
// setDatabaseParameterOverrides (GraphQL has no built-in map type).
var parameterInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "ParameterInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// databaseLogGQLType is one CNPG log line — timestamp, message, and labels
// (service/instance/type) — the same Render shape as app log entries.
var databaseLogGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseLogEntry",
	Fields: graphql.Fields{
		"timestamp": gqlutil.StrField(func(e DatabaseLogEntry) any { return e.Timestamp }),
		"message":   gqlutil.StrField(func(e DatabaseLogEntry) any { return e.Message }),
		"instance":  gqlutil.StrField(func(e DatabaseLogEntry) any { return e.Labels["instance"] }),
		"type":      gqlutil.StrField(func(e DatabaseLogEntry) any { return e.Labels["type"] }),
	},
})

// --- insights GQL types ---

var processViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseProcess",
	Fields: graphql.Fields{
		"pid":             gqlutil.IntField(func(v ProcessView) any { return v.PID }),
		"userName":        gqlutil.StrField(func(v ProcessView) any { return v.UserName }),
		"applicationName": gqlutil.StrField(func(v ProcessView) any { return v.ApplicationName }),
		"state":           gqlutil.StrField(func(v ProcessView) any { return v.State }),
		"query":           gqlutil.StrField(func(v ProcessView) any { return v.Query }),
		"waitEventType":   gqlutil.StrField(func(v ProcessView) any { return v.WaitEventType }),
		"waitEvent":       gqlutil.StrField(func(v ProcessView) any { return v.WaitEvent }),
		"durationSeconds": gqlutil.IntField(func(v ProcessView) any { return v.DurationSeconds }),
	},
})

var topQueryViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseTopQuery",
	Fields: graphql.Fields{
		"query":          gqlutil.StrField(func(v TopQueryView) any { return v.Query }),
		"calls":          gqlutil.IntField(func(v TopQueryView) any { return v.Calls }),
		"totalTimeMs":    gqlutil.FloatField(func(v TopQueryView) any { return v.TotalTimeMs }),
		"meanTimeMs":     gqlutil.FloatField(func(v TopQueryView) any { return v.MeanTimeMs }),
		"rows":           gqlutil.IntField(func(v TopQueryView) any { return v.Rows }),
		"sharedHitBlks":  gqlutil.IntField(func(v TopQueryView) any { return v.SharedHitBlks }),
		"sharedReadBlks": gqlutil.IntField(func(v TopQueryView) any { return v.SharedReadBlks }),
	},
})

var databaseSizeViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseSizeInfo",
	Fields: graphql.Fields{
		"name":       gqlutil.StrField(func(v DatabaseSizeView) any { return v.Name }),
		"sizeBytes":  gqlutil.IntField(func(v DatabaseSizeView) any { return v.SizeBytes }),
		"sizePretty": gqlutil.StrField(func(v DatabaseSizeView) any { return v.SizePretty }),
	},
})

var tableSizeViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "TableSizeInfo",
	Fields: graphql.Fields{
		"schema":     gqlutil.StrField(func(v TableSizeView) any { return v.Schema }),
		"name":       gqlutil.StrField(func(v TableSizeView) any { return v.Name }),
		"sizeBytes":  gqlutil.IntField(func(v TableSizeView) any { return v.SizeBytes }),
		"sizePretty": gqlutil.StrField(func(v TableSizeView) any { return v.SizePretty }),
	},
})

var sizesViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseSizes",
	Fields: graphql.Fields{
		"database": gqlutil.Typed(databaseSizeViewGQLType, func(v SizesView) any { return v.Database }),
		"tables":   gqlutil.Typed(graphql.NewList(tableSizeViewGQLType), func(v SizesView) any { return v.Tables }),
	},
})

var tableScanViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseTableScan",
	Fields: graphql.Fields{
		"schema":        gqlutil.StrField(func(v TableScanView) any { return v.Schema }),
		"name":          gqlutil.StrField(func(v TableScanView) any { return v.Name }),
		"seqScans":      gqlutil.IntField(func(v TableScanView) any { return v.SeqScans }),
		"seqScanRows":   gqlutil.IntField(func(v TableScanView) any { return v.SeqScanRows }),
		"indexScans":    gqlutil.IntField(func(v TableScanView) any { return v.IndexScans }),
		"indexScanRows": gqlutil.IntField(func(v TableScanView) any { return v.IndexScanRows }),
		"liveRows":      gqlutil.IntField(func(v TableScanView) any { return v.LiveRows }),
		"deadRows":      gqlutil.IntField(func(v TableScanView) any { return v.DeadRows }),
	},
})

var parameterOverrideViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseParameterOverride",
	Fields: graphql.Fields{
		"name":        gqlutil.StrField(func(v ParameterOverrideView) any { return v.Name }),
		"setting":     gqlutil.StrField(func(v ParameterOverrideView) any { return v.Setting }),
		"unit":        gqlutil.StrField(func(v ParameterOverrideView) any { return v.Unit }),
		"source":      gqlutil.StrField(func(v ParameterOverrideView) any { return v.Source }),
		"description": gqlutil.StrField(func(v ParameterOverrideView) any { return v.Description }),
	},
})

// parameterSpecViewGQLType is the tenant's DECLARED overrides — what a write
// replaces, and what the editor binds to. Distinct from
// DatabaseParameterOverride above, which is the observed pg_settings config and
// is mostly the operator's (w6/m133).
var parameterSpecViewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseParameterSpec",
	Fields: graphql.Fields{
		"name":  gqlutil.StrField(func(v ParameterSpecView) any { return v.Name }),
		"value": gqlutil.StrField(func(v ParameterSpecView) any { return v.Value }),
	},
})

type databaseQueryRow struct {
	Values []any
}

type databaseQueryResult struct {
	Columns   []string
	Rows      []databaseQueryRow
	RowCount  int
	Truncated bool
}

var databaseQueryRowGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseQueryRow",
	Fields: graphql.Fields{
		"values": gqlutil.StrsField(func(v databaseQueryRow) any { return v.Values }),
	},
})

var databaseQueryResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseQueryResult",
	Fields: graphql.Fields{
		"columns":   gqlutil.StrsField(func(v databaseQueryResult) any { return v.Columns }),
		"rows":      gqlutil.Typed(graphql.NewList(databaseQueryRowGQLType), func(v databaseQueryResult) any { return v.Rows }),
		"rowCount":  gqlutil.IntField(func(v databaseQueryResult) any { return v.RowCount }),
		"truncated": gqlutil.BoolField(func(v databaseQueryResult) any { return v.Truncated }),
	},
})

func queryResultForGraphQL(result QueryResult) databaseQueryResult {
	rows := make([]databaseQueryRow, len(result.Rows))
	for i, row := range result.Rows {
		values := make([]any, len(row))
		for j, value := range row {
			values[j] = queryCellString(value)
		}
		rows[i] = databaseQueryRow{Values: values}
	}
	return databaseQueryResult{
		Columns:   result.Columns,
		Rows:      rows,
		RowCount:  result.RowCount,
		Truncated: result.Truncated,
	}
}

// queryCellString preserves NULL while rendering pgx values deterministically
// for GraphQL's tabular string cells. REST and MCP retain the native JSON values.
func queryCellString(value any) any {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return base64.StdEncoding.EncodeToString(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	}
	if encoded, err := json.Marshal(value); err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
}

// GraphQLQuery returns the database read fields (Render dashboard nouns).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"databases": &graphql.Field{ // list; Render lists via env, bex offers a top-level list
			Type: graphql.NewList(postgresGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				// ownerId mirrors Render's REST/MCP databases list filter (w6/m2/t004).
				"ownerId": gqlutil.Arg(graphql.String),
				// cursor/limit are bex's paged twin of this top-level dashboard list.
				// The result remains [Database], so existing dashboard selections stay valid.
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ownerID := gqlutil.Str(p.Args, "ownerId")
				out, err := s.ListPostgres(p.Context, ownerID)
				if err != nil {
					return nil, err
				}
				return gqlutil.Page(p, out, func(pg PostgresView) string { return pg.ID }), nil
			},
		},
		"database":               gqlutil.IDVerb(postgresGQLType, s.GetPostgres), // Render's dashboard query name
		"databaseConnectionInfo": gqlutil.IDVerb(connectionInfoGQLType, s.PostgresConnectionInfo),
		"databaseInstanceTypes": &graphql.Field{ // bex extension backing the create dialog's plan picker
			Type:    graphql.NewList(databaseInstanceTypeGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.InstanceTypes(p.Context) },
		},
		"databaseRecoveryInfo": gqlutil.IDVerb(databaseRecoveryInfoGQLType, s.RecoveryInfo),           // PITR window + backup list
		"databaseExports":      gqlutil.IDVerb(graphql.NewList(databaseExportGQLType), s.ListExports), // logical pg_dump exports
		"databaseUsers":        gqlutil.IDVerb(graphql.NewList(databaseUserGQLType), s.ListUsers),     // additional managed login roles
		"databaseIpAllowList": &graphql.Field{ // external-endpoint CIDR allowlist (strings; the Database type's ipAllowListEntries carries descriptions)
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
		// --- insights (m25) ---
		"databaseProcesses":          gqlutil.IDVerb(graphql.NewList(processViewGQLType), s.Processes),
		"databaseTopQueries":         gqlutil.IDVerb(graphql.NewList(topQueryViewGQLType), s.TopQueries),
		"databaseSizes":              gqlutil.IDVerb(sizesViewGQLType, s.Sizes),
		"databaseTableScans":         gqlutil.IDVerb(graphql.NewList(tableScanViewGQLType), s.TableScans),
		"databaseParameterOverrides": gqlutil.IDVerb(graphql.NewList(parameterOverrideViewGQLType), s.ParameterOverrides),
		"databaseParameterSpec":      gqlutil.IDVerb(graphql.NewList(parameterSpecViewGQLType), s.ParameterSpec),
		// --- logs (w3/m28) ---
		"databaseLogs": &graphql.Field{
			Type: graphql.NewList(databaseLogGQLType),
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
				return s.QueryDatabaseLogs(p.Context, p.Args["id"].(string), DatabaseLogQuery{
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

// GraphQLMutation returns the createDatabase / deleteDatabase mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createDatabase": &graphql.Field{
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"name":         gqlutil.ReqArg(graphql.String),
				"databaseName": gqlutil.Arg(graphql.String),
				"databaseUser": gqlutil.Arg(graphql.String),
				// ownerId is the workspace to create IN (w6/m14) — the write-side
				// twin of the databases list filter; optional, defaulting to the
				// caller's default workspace, forbidden for a non-member.
				"ownerId":               gqlutil.Arg(graphql.String),
				"environmentId":         gqlutil.Arg(graphql.String),
				"plan":                  gqlutil.Arg(graphql.String),
				"version":               gqlutil.Arg(graphql.String),
				"diskSizeGB":            gqlutil.Arg(graphql.Int),
				"enableDiskAutoscaling": gqlutil.Arg(graphql.Boolean),
				"public":                gqlutil.Arg(graphql.Boolean),
				"ipAllowList":           gqlutil.Arg(graphql.NewList(graphql.String)),
				// ipAllowListEntries is the description-carrying form (w4/m24);
				// when present it wins over ipAllowList.
				"ipAllowListEntries":     gqlutil.Arg(graphql.NewList(graphql.NewNonNull(gqlutil.IPAllowEntryInputType))),
				"enableHighAvailability": gqlutil.Arg(graphql.Boolean),
				// dryRun, when true, returns the resolved spec without any writes (w2/m29).
				"dryRun": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreatePostgres(p.Context, CreatePostgresRequest{
					Name:                  p.Args["name"].(string),
					OwnerID:               gqlutil.Str(p.Args, "ownerId"),
					EnvironmentID:         gqlutil.Str(p.Args, "environmentId"),
					DatabaseName:          gqlutil.Str(p.Args, "databaseName"),
					DatabaseUser:          gqlutil.Str(p.Args, "databaseUser"),
					Plan:                  gqlutil.Str(p.Args, "plan"),
					Version:               gqlutil.Str(p.Args, "version"),
					DiskSizeGB:            int32(gqlutil.Int(p.Args, "diskSizeGB")),
					EnableDiskAutoscaling: gqlutil.Bool(p.Args, "enableDiskAutoscaling"),
					Public:                gqlutil.Bool(p.Args, "public"),
					IPAllowList: core.AllowListOrCIDRs(
						gqlutil.AllowList(p.Args["ipAllowListEntries"]), gqlutil.StringList(p.Args["ipAllowList"])),
					EnableHighAvailability: gqlutil.Bool(p.Args, "enableHighAvailability"),
					DryRun:                 gqlutil.Bool(p.Args, "dryRun"),
				})
			},
		},
		"deleteDatabase": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"confirm": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeletePostgres(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["id"].(string))
				return err == nil, err
			},
		},
		"updateDatabasePlan":    gqlutil.PlanMutation(postgresGQLType, s.SetPlan, s.PreviewSetPlan),
		"updateDatabaseVersion": gqlutil.ArgMutation(postgresGQLType, "version", s.SetVersion),
		"updateDatabaseDiskAutoscaling": &graphql.Field{
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"enabled": gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				enabled := p.Args["enabled"].(bool)
				return s.UpdatePostgres(p.Context, p.Args["id"].(string), PostgresPatch{EnableDiskAutoscaling: &enabled})
			},
		},
		"renameDatabase": gqlutil.PatchMutation(postgresGQLType, "name",
			func(name string) PostgresPatch { return PostgresPatch{Name: &name} },
			s.UpdatePostgres, s.PreviewUpdatePostgres),

		// --- failover (HA only) — Render's POST /postgres/{id}/failover → 202 ---
		"failoverDatabase": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.Failover(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},

		// --- lifecycle ---
		"suspendDatabase": &graphql.Field{
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"confirm": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Suspend(core.WithConfirm(p.Context, gqlutil.Str(p.Args, "confirm")), p.Args["id"].(string))
			},
		},
		"resumeDatabase":  gqlutil.IDVerb(postgresGQLType, s.Resume),
		"restartDatabase": gqlutil.IDVerb(postgresGQLType, s.Restart),

		// --- recovery / exports ---
		"recoverDatabase": &graphql.Field{ // restore to a NEW instance (PITR)
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         gqlutil.ReqArg(graphql.String), // source
				"name":       gqlutil.ReqArg(graphql.String), // new instance
				"targetTime": gqlutil.Arg(graphql.String),
				"plan":       gqlutil.Arg(graphql.String),
				"version":    gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Recover(p.Context, p.Args["id"].(string), RecoverRequest{
					Name:       p.Args["name"].(string),
					TargetTime: gqlutil.Str(p.Args, "targetTime"),
					Plan:       gqlutil.Str(p.Args, "plan"),
					Version:    gqlutil.Str(p.Args, "version"),
				})
			},
		},
		"createDatabaseExport": gqlutil.IDVerb(databaseExportGQLType, s.CreateExport),

		// --- access: IP allowlist + users ---
		"setDatabaseIpAllowList": &graphql.Field{
			Type: postgresGQLType,
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
		"createDatabaseUser": gqlutil.ArgMutation(databaseUserWithPasswordGQLType, "name", s.CreateUser),
		"deleteDatabaseUser": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":   gqlutil.ReqArg(graphql.String),
				"name": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteUser(p.Context, p.Args["id"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},

		// --- insights (m25): parameter overrides write ---
		"setDatabaseParameterOverrides": &graphql.Field{
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"id": gqlutil.ReqArg(graphql.String),
				// parameters is a list of {name, value} pairs (GraphQL has no map type).
				"parameters": gqlutil.Arg(graphql.NewList(parameterInputGQLType)),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				items, _ := p.Args["parameters"].([]any)
				params := make(map[string]string, len(items))
				for _, item := range items {
					m, _ := item.(map[string]any)
					k, _ := m["name"].(string)
					v, _ := m["value"].(string)
					if k != "" {
						params[k] = v
					}
				}
				return s.SetParameterOverrides(p.Context, p.Args["id"].(string), params)
			},
		},
		"executeDatabaseQuery": &graphql.Field{
			Type: databaseQueryResultGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"sql":         gqlutil.ReqArg(graphql.String),
				"allowWrites": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				allowWrites := gqlutil.Bool(p.Args, "allowWrites")
				result, err := s.ExecuteQuery(p.Context, p.Args["id"].(string), p.Args["sql"].(string), allowWrites)
				if err != nil {
					return nil, err
				}
				return queryResultForGraphQL(result), nil
			},
		},
	}
}
