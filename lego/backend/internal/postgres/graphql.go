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
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// Render's dashboard GraphQL calls a managed Postgres a "database" (query
// database(id), databaseStatusQuery, ...) — captured live — even though its REST
// noun is "postgres". bex mirrors that split: REST /v1/postgres, GraphQL
// database* (which also matches bex's own Database CRD).
var postgresGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Database",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.ID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Name })},
		"plan":                    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Plan })},
		"version":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Version })},
		"status":                  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Status })},
		"databaseName":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.DatabaseName })},
		"databaseUser":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.DatabaseUser })},
		"diskSizeGB":              &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v PostgresView) any { return v.DiskSizeGB })},
		"highAvailabilityEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PostgresView) any { return v.HighAvailabilityEnabled })},
		"suspended":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Suspended })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.CreatedAt })},
		"externalHost":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.ExternalHost })},
		"public":                  &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Public })},
		"ipAllowList":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v PostgresView) any { return v.IPAllowList })},
		"poolerEnabled":           &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PostgresView) any { return v.PoolerEnabled })},
		"backupsEnabled":          &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PostgresView) any { return v.BackupsEnabled })},
		"ownerId":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.OwnerID })},
	},
})

// databaseBackupGQLType is one base backup / on-demand export in object storage.
var databaseBackupGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseBackup",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v BackupView) any { return v.ID })},
		"status":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v BackupView) any { return v.Status })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v BackupView) any { return v.CreatedAt })},
	},
})

// databaseRecoveryInfoGQLType mirrors Render's postgres recovery info.
var databaseRecoveryInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseRecoveryInfo",
	Fields: graphql.Fields{
		"enabled":              &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v RecoveryInfoView) any { return v.Enabled })},
		"earliestRecoveryTime": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v RecoveryInfoView) any { return v.EarliestRecoveryTime })},
		"latestRecoveryTime":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v RecoveryInfoView) any { return v.LatestRecoveryTime })},
		"backups":              &graphql.Field{Type: graphql.NewList(databaseBackupGQLType), Resolve: gqlutil.Field(func(v RecoveryInfoView) any { return v.Backups })},
	},
})

// databaseUserGQLType is one additional managed login role (no password).
var databaseUserGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseUser",
	Fields: graphql.Fields{
		"name": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresUserView) any { return v.Name })},
	},
})

// databaseUserWithPasswordGQLType is createDatabaseUser's one-time result, the
// only place a role's generated password is surfaced.
var databaseUserWithPasswordGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DatabaseUserWithPassword",
	Fields: graphql.Fields{
		"name":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CreateUserResult) any { return v.Name })},
		"password": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v CreateUserResult) any { return v.Password })},
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
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t DatabaseInstanceType) any { return t.ID })},
		"name":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t DatabaseInstanceType) any { return t.Name })},
		"cpu":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t DatabaseInstanceType) any { return t.CPU })},
		"memory":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(t DatabaseInstanceType) any { return t.Memory })},
		"storageGB": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(t DatabaseInstanceType) any { return t.StorageGB })},
	},
})

var connectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresConnectionInfo",
	Fields: graphql.Fields{
		"password":                     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.Password })},
		"internalConnectionString":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.InternalConnectionString })},
		"externalConnectionString":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.ExternalConnectionString })},
		"internalConnectionPoolString": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.InternalConnectionPoolString })},
		"externalConnectionPoolString": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.ExternalConnectionPoolString })},
		"psqlCommand":                  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.PSQLCommand })},
	},
})

// GraphQLQuery returns the database read fields (Render dashboard nouns).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"databases": &graphql.Field{ // list; Render lists via env, bex offers a top-level list
			Type: graphql.NewList(postgresGQLType),
			Args: graphql.FieldConfigArgument{
				// ownerId mirrors Render's REST/MCP databases list filter (w6/m2/t004).
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				ownerID, _ := p.Args["ownerId"].(string)
				return s.ListPostgres(p.Context, ownerID)
			},
		},
		"database": &graphql.Field{ // Render's dashboard query name
			Type: postgresGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetPostgres(p.Context, p.Args["id"].(string))
			},
		},
		"databaseConnectionInfo": &graphql.Field{
			Type: connectionInfoGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PostgresConnectionInfo(p.Context, p.Args["id"].(string))
			},
		},
		"databaseInstanceTypes": &graphql.Field{ // bex extension backing the create dialog's plan picker
			Type:    graphql.NewList(databaseInstanceTypeGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.InstanceTypes(p.Context) },
		},
		"databaseRecoveryInfo": &graphql.Field{ // PITR window + backup list
			Type: databaseRecoveryInfoGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RecoveryInfo(p.Context, p.Args["id"].(string))
			},
		},
		"databaseExports": &graphql.Field{ // on-demand export snapshots
			Type: graphql.NewList(databaseBackupGQLType),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListExports(p.Context, p.Args["id"].(string))
			},
		},
		"databaseUsers": &graphql.Field{ // additional managed login roles
			Type: graphql.NewList(databaseUserGQLType),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListUsers(p.Context, p.Args["id"].(string))
			},
		},
		"databaseIpAllowList": &graphql.Field{ // external-endpoint CIDR allowlist
			Type: graphql.NewList(graphql.String),
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetIPAllowList(p.Context, p.Args["id"].(string))
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
				"name":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan":       &graphql.ArgumentConfig{Type: graphql.String},
				"version":    &graphql.ArgumentConfig{Type: graphql.String},
				"diskSizeGB": &graphql.ArgumentConfig{Type: graphql.Int},
				"public":     &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := CreatePostgresRequest{Name: p.Args["name"].(string)}
				if v, ok := p.Args["plan"].(string); ok {
					req.Plan = v
				}
				if v, ok := p.Args["version"].(string); ok {
					req.Version = v
				}
				if v, ok := p.Args["diskSizeGB"].(int); ok {
					req.DiskSizeGB = int32(v)
				}
				if v, ok := p.Args["public"].(bool); ok {
					req.Public = v
				}
				return s.CreatePostgres(p.Context, req)
			},
		},
		"deleteDatabase": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeletePostgres(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},

		// --- lifecycle ---
		"suspendDatabase": &graphql.Field{
			Type: postgresGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Suspend(p.Context, p.Args["id"].(string))
			},
		},
		"resumeDatabase": &graphql.Field{
			Type: postgresGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Resume(p.Context, p.Args["id"].(string))
			},
		},
		"restartDatabase": &graphql.Field{
			Type: postgresGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Restart(p.Context, p.Args["id"].(string))
			},
		},

		// --- recovery / exports ---
		"recoverDatabase": &graphql.Field{ // restore to a NEW instance (PITR)
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // source
				"name":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}, // new instance
				"targetTime": &graphql.ArgumentConfig{Type: graphql.String},
				"plan":       &graphql.ArgumentConfig{Type: graphql.String},
				"version":    &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := RecoverRequest{Name: p.Args["name"].(string)}
				if v, ok := p.Args["targetTime"].(string); ok {
					req.TargetTime = v
				}
				if v, ok := p.Args["plan"].(string); ok {
					req.Plan = v
				}
				if v, ok := p.Args["version"].(string); ok {
					req.Version = v
				}
				return s.Recover(p.Context, p.Args["id"].(string), req)
			},
		},
		"createDatabaseExport": &graphql.Field{
			Type: databaseBackupGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateExport(p.Context, p.Args["id"].(string))
			},
		},

		// --- access: IP allowlist + users ---
		"setDatabaseIpAllowList": &graphql.Field{
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cidrs": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var cidrs []string
				if raw, ok := p.Args["cidrs"].([]any); ok {
					for _, c := range raw {
						if str, ok := c.(string); ok {
							cidrs = append(cidrs, str)
						}
					}
				}
				return s.SetIPAllowList(p.Context, p.Args["id"].(string), cidrs)
			},
		},
		"createDatabaseUser": &graphql.Field{
			Type: databaseUserWithPasswordGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateUser(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		"deleteDatabaseUser": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteUser(p.Context, p.Args["id"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},
	}
}
