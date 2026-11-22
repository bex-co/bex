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
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A database's admin apps (ADR009 § Admin apps, w4/m551): Render's Postgres
// "Apps" page deploys pgAdmin or PgHero as an ordinary web service in the
// database's workspace, region and environment, connected over the private
// network (docs/render-artifacts/postgres-admin-apps.md). Render assembles
// the service in the browser from a password the page loaded; bex builds it
// here, so the connection string never travels to the client.

// The two labels that link an admin app's service to its database: which app
// it is, and the dpg- id it administers. Render links them by the service's
// slug (`<app>-<database id>`), which bex's 30-character resource names cannot
// hold, so the link is explicit; like Render's, it survives a rename.
const (
	LabelPostgresAdminApp         = "bex.co/postgres-admin-app"
	LabelPostgresAdminAppDatabase = "bex.co/postgres-admin-app-database"
)

// The admin apps' ids, Render's catalog keys.
const (
	PostgresAdminAppPgAdmin = "pgadmin"
	PostgresAdminAppPgHero  = "pghero"
)

// postgresAdminAppConflictCode refuses a second app of one kind for one
// database: Render's page never offers it, since a deployed app's card has
// Open app instead of Deploy app.
const postgresAdminAppConflictCode = "POSTGRES_ADMIN_APP_EXISTS"

// postgresAdminApp is one entry of Render's catalog: what the Apps page shows
// and the service its Deploy app creates.
type postgresAdminApp struct {
	id   string
	name string
	plan string
	// diskSizeGB is the disk the service is created with, 0 for none.
	diskSizeGB int32
	// usernameFromEmail marks an app whose login is the deploying user's
	// email (pgAdmin), not a generated name (PgHero).
	usernameFromEmail bool
	// service builds the create for a database's connection and the chosen
	// login.
	service func(db adminAppDatabase, username, password string) CreateRequest
}

// adminAppDatabase is what an admin app's service is built from: the
// database's display name and its internal connection.
type adminAppDatabase struct {
	displayName string
	conn        postgres.InternalConnection
}

// pgHeroImage and pgAdminRepo are the sources Render deploys. PgHero listens
// on 8080; bex sets that port so no anonymous Docker Hub read is needed to
// discover it (w4/429).
const (
	pgHeroImage   = "docker.io/ankane/pghero:latest"
	pgHeroPort    = 8080
	pgAdminRepo   = "https://github.com/render-examples/pgadmin"
	pgAdminPort   = "10000"
	pgAdminMount  = "/var/lib/pgadmin"
	pgAdminDiskGB = 1
	pgAdminServer = "/var/lib/pgadmin/storage/servers.json"
)

var postgresAdminAppCatalog = []postgresAdminApp{
	{
		id: PostgresAdminAppPgAdmin, name: "pgAdmin", plan: "starter", diskSizeGB: pgAdminDiskGB, usernameFromEmail: true,
		service: func(db adminAppDatabase, username, password string) CreateRequest {
			servers, _ := json.Marshal(map[string]any{"Servers": map[string]any{"1": map[string]any{
				"Name": db.conn.Database, "Group": "Servers", "Port": postgres.Port, "Username": db.conn.User,
				"PassFile": "pgpassfile", "Host": db.conn.Host, "SSLMode": "allow", "MaintenanceDB": "postgres",
			}}})
			return CreateRequest{
				Repo: pgAdminRepo, Branch: "main", Runtime: "docker", DockerfilePath: "./Dockerfile",
				Disk: &ServiceDiskView{Name: "pgadmin", MountPath: pgAdminMount, SizeGB: pgAdminDiskGB},
				Env: literalEnv(
					"GUNICORN_THREADS", "2",
					"PGADMIN_DATABASE_HOST", db.conn.Host,
					"PGADMIN_DATABASE_NAME", db.displayName,
					"PGADMIN_DATABASE_USER", db.conn.User,
					"PGADMIN_DEFAULT_EMAIL", username,
					"PGADMIN_DEFAULT_PASSWORD", password,
					"PGADMIN_LISTEN_PORT", pgAdminPort,
					"PGADMIN_SERVER_JSON_FILE", pgAdminServer,
					"PORT", pgAdminPort,
				),
				// The repository's entrypoint copies both into pgAdmin's storage,
				// owned by its user, so the tree opens with the server and its
				// saved password.
				SecretFiles: []core.SecretFile{
					{Name: "pgpassfile", Content: pgPassLine(db.conn)},
					{Name: "servers.json", Content: string(servers)},
				},
			}
		},
	},
	{
		id: PostgresAdminAppPgHero, name: "PgHero", plan: "free",
		service: func(db adminAppDatabase, username, password string) CreateRequest {
			return CreateRequest{
				Image: pgHeroImage, Runtime: "image", Port: pgHeroPort,
				Env: literalEnv(
					"DATABASE_URL", db.conn.URL(),
					"PGHERO_USERNAME", username,
					"PGHERO_PASSWORD", password,
					"WEB_CONCURRENCY", "2",
				),
			}
		},
	},
}

// literalEnv pairs keys and values into literal env vars, in order.
func literalEnv(pairs ...string) []appv1alpha1.EnvVar {
	out := make([]appv1alpha1.EnvVar, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, appv1alpha1.EnvVar{Name: pairs[i], Value: pairs[i+1]})
	}
	return out
}

// pgPassLine is the connection's .pgpass entry (Render's
// `<host>:5432:*:<user>:<password>`), with libpq's escapes for ':' and '\'.
func pgPassLine(c postgres.InternalConnection) string {
	escape := strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace
	return strings.Join([]string{escape(c.Host), strconv.Itoa(postgres.Port), "*", escape(c.User), escape(c.Password)}, ":")
}

func postgresAdminAppByID(id string) (postgresAdminApp, bool) {
	for _, app := range postgresAdminAppCatalog {
		if app.id == id {
			return app, true
		}
	}
	return postgresAdminApp{}, false
}

// PostgresAdminAppView is one catalog app on a database's Apps page and, once
// deployed, its service.
type PostgresAdminAppView struct {
	ID         string
	Name       string
	Plan       string
	DiskSizeGB int32
	// MonthlyUSD and DiskMonthlyUSD are the dialog's billing lines, from the
	// price sheet ("" for no disk).
	MonthlyUSD     string
	DiskMonthlyUSD string
	Service        *AppView
}

// PostgresAdminApps answers a database's Apps page: Render's catalog, each app
// with the service deployed for this database, or none. Reading it is the
// database's can_view, as its page is.
func (s *Service) PostgresAdminApps(ctx context.Context, databaseID string) ([]PostgresAdminAppView, error) {
	d, err := s.AuthorizeDatabase(ctx, core.RelCanView, databaseID)
	if err != nil {
		return nil, err
	}
	if err := core.NotFoundIfDeleting(d); err != nil {
		return nil, err
	}
	deployed, err := s.postgresAdminAppServices(ctx, d)
	if err != nil {
		return nil, err
	}
	out := make([]PostgresAdminAppView, 0, len(postgresAdminAppCatalog))
	for _, app := range postgresAdminAppCatalog {
		monthly, _ := pricing.Default.InstanceMonthlyUSD(app.plan, store.ResourceKindService)
		v := PostgresAdminAppView{
			ID: app.id, Name: app.name, Plan: app.plan, DiskSizeGB: app.diskSizeGB,
			MonthlyUSD: monthly, DiskMonthlyUSD: pricing.Default.DiskMonthlyUSD(app.diskSizeGB),
		}
		if svc, ok := deployed[app.id]; ok {
			v.Service = &svc
		}
		out = append(out, v)
	}
	return out, nil
}

// postgresAdminAppServices is each app's live service for d, by the link
// labels, read in d's own workspace. Two of one kind (a race the refusal
// below narrows but cannot close) answer the first one listed.
func (s *Service) postgresAdminAppServices(ctx context.Context, d *appv1alpha1.Database) (map[string]AppView, error) {
	tenantID := d.Labels[core.LabelTenant]
	labels := client.MatchingLabels{LabelPostgresAdminAppDatabase: d.Name}
	if tenantID != "" {
		labels[core.LabelTenant] = tenantID
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.InNamespace(s.AppNamespace(tenantID)), labels); err != nil {
		return nil, err
	}
	out := map[string]AppView{}
	for _, a := range core.ServiceApps(list.Items) {
		kind := a.Labels[LabelPostgresAdminApp]
		if _, seen := out[kind]; kind == "" || seen {
			continue
		}
		// One at a time, so each view keeps its App's kind: readViews drops a
		// deleting App and one whose row is gone.
		views, err := s.readViews(ctx, []appv1alpha1.App{a})
		if err != nil {
			return nil, err
		}
		if len(views) == 1 {
			out[kind] = views[0]
		}
	}
	return out, nil
}

// DeployPostgresAdminAppRequest is the Apps page's Deploy app: which app, for
// which database, with the login its dialog showed. An empty username or
// password is generated as Render's dialog generates it.
type DeployPostgresAdminAppRequest struct {
	DatabaseID string
	App        string
	Username   string
	Password   string
}

// DeployPostgresAdminApp creates an admin app's web service for a database
// (ADR009 § Admin apps). The service receives the database's credential, so
// the caller needs the database's can_view_sensitive, re-asserted uncached as
// databaseConnectionInfo does; the create is then the shared service create in
// the database's workspace, region and environment, with its can_create, its
// protected-environment Admin gate and its payment gate, audited as a service
// create.
func (s *Service) DeployPostgresAdminApp(ctx context.Context, req DeployPostgresAdminAppRequest) (AppView, error) {
	d, err := s.AuthorizeDatabase(ctx, core.RelCanViewSensitive, req.DatabaseID)
	if err != nil {
		return AppView{}, err
	}
	if err := core.NotFoundIfDeleting(d); err != nil {
		return AppView{}, err
	}
	if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanViewSensitive, d); err != nil {
		return AppView{}, err
	}
	app, ok := postgresAdminAppByID(req.App)
	if !ok {
		return AppView{}, core.NewBadRequestError("INVALID_POSTGRES_ADMIN_APP",
			fmt.Sprintf("app must be one of: %s, %s", PostgresAdminAppPgAdmin, PostgresAdminAppPgHero), map[string]any{"app": req.App})
	}
	deployed, err := s.postgresAdminAppServices(ctx, d)
	if err != nil {
		return AppView{}, err
	}
	if svc, ok := deployed[app.id]; ok {
		return AppView{}, core.NewConflictError(postgresAdminAppConflictCode,
			fmt.Sprintf("%s is already deployed for this database as %s", app.name, renderServiceName(svc)),
			map[string]any{"app": app.id, "serviceId": svc.ID})
	}
	conn, err := s.postgresAdminAppConnection(ctx, d)
	if err != nil {
		return AppView{}, err
	}
	username, password, err := app.login(ctx, req.Username, req.Password)
	if err != nil {
		return AppView{}, err
	}
	create := app.service(adminAppDatabase{displayName: cmp.Or(d.Spec.Name, d.Name), conn: conn}, username, password)
	// Render's name for the service, `<app>-<database id>`.
	create.Name = app.name + "-" + d.Name
	create.Type = appv1alpha1.TypeWebService
	// Render sends autoDeploy: true for both; bex's default is the same for
	// pgAdmin's repository, and an image has no push to deploy on.
	create.Plan = app.plan
	create.OwnerID = d.Labels[core.LabelTenant]
	create.EnvironmentID = d.Labels[core.LabelEnvironment]
	create.Region = d.Annotations[resourcemeta.AnnotRegion]
	create.adminApp = &postgresAdminAppLink{app: app.id, database: d.Name}
	return s.Create(ctx, create)
}

// postgresAdminAppConnection is d's internal connection, refused while the
// database has no private address or credential yet.
func (s *Service) postgresAdminAppConnection(ctx context.Context, d *appv1alpha1.Database) (postgres.InternalConnection, error) {
	sec, err := postgres.ConnectionSecret(ctx, s.Client, d)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return postgres.InternalConnection{}, err
	}
	var conn postgres.InternalConnection
	if sec != nil {
		conn = postgres.InternalConnectionOf(d, sec)
	}
	if conn.Host == "" || conn.User == "" || conn.Password == "" {
		return postgres.InternalConnection{}, fmt.Errorf("%w: database %s is not available yet; deploy an admin app once it is", core.ErrUnavailable, d.Name)
	}
	return conn, nil
}

// login is the app's username and password: the caller's, else Render's
// generated ones (docs/render-artifacts/postgres-admin-apps.md § The
// credentials): `<app>-` and 8 hex characters, or the caller's email for
// pgAdmin, which logs in by email; a password of 32 hex characters.
func (app postgresAdminApp) login(ctx context.Context, username, password string) (string, string, error) {
	username, password = strings.TrimSpace(username), strings.TrimSpace(password)
	if username == "" {
		if app.usernameFromEmail {
			id, _ := core.IdentityFrom(ctx)
			username = id.Email
		} else {
			username = app.id + "-" + randomHex(8)
		}
	}
	if username == "" {
		return "", "", core.NewBadRequestError("POSTGRES_ADMIN_APP_USERNAME_REQUIRED",
			app.name+" logs in by email: send username", map[string]any{"app": app.id})
	}
	if password == "" {
		password = randomHex(32)
	}
	return username, password, nil
}

// randomHex is n random hex characters, Render's generator.
func randomHex(n int) string {
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

// postgresAdminAppLink is the database an admin app's new service is linked
// to; Create stamps it as the link labels.
type postgresAdminAppLink struct{ app, database string }

func (l *postgresAdminAppLink) stamp(a *appv1alpha1.App) {
	if l == nil {
		return
	}
	if a.Labels == nil {
		a.Labels = map[string]string{}
	}
	a.Labels[LabelPostgresAdminApp] = l.app
	a.Labels[LabelPostgresAdminAppDatabase] = l.database
}

// --- GraphQL ---

// postgresAdminAppGQLType is one card of a database's Apps page: Render's
// catalog entry and its deployed service (null until one is deployed).
var postgresAdminAppGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresAdminApp",
	Fields: graphql.Fields{
		"id":   gqlutil.StrField(func(v PostgresAdminAppView) any { return v.ID }),
		"name": gqlutil.StrField(func(v PostgresAdminAppView) any { return v.Name }),
		"plan": gqlutil.StrField(func(v PostgresAdminAppView) any { return v.Plan }),
		// diskSizeGB is the disk the service is created with, null for none.
		"diskSizeGB": gqlutil.IntField(func(v PostgresAdminAppView) any {
			if v.DiskSizeGB == 0 {
				return nil
			}
			return int(v.DiskSizeGB)
		}),
		// monthlyUsd and diskMonthlyUsd are the deploy dialog's billing lines.
		"monthlyUsd":     gqlutil.StrField(func(v PostgresAdminAppView) any { return v.MonthlyUSD }),
		"diskMonthlyUsd": gqlutil.OptionalStrField(func(v PostgresAdminAppView) any { return v.DiskMonthlyUSD }),
		"service": gqlutil.Typed(serviceGQLType, func(v PostgresAdminAppView) any {
			if v.Service == nil {
				return nil
			}
			return *v.Service
		}),
	},
})

// postgresAdminAppGQLQueryFields is the Apps page's read; a bex extension of
// Render's page, which reads the workspace's services and matches slugs.
func (s *Service) postgresAdminAppGQLQueryFields() graphql.Fields {
	return graphql.Fields{
		"postgresAdminApps": &graphql.Field{
			Type: graphql.NewList(postgresAdminAppGQLType),
			Args: graphql.FieldConfigArgument{"databaseId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PostgresAdminApps(p.Context, p.Args["databaseId"].(string))
			},
		},
	}
}

// postgresAdminAppGQLMutationFields is the Apps page's Deploy app. Render's
// dashboard sends createServer with the service it assembled in the browser;
// bex assembles it server side (ADR009 § Admin apps). Render has no REST or
// MCP surface for admin apps, so bex adds none.
func (s *Service) postgresAdminAppGQLMutationFields() graphql.Fields {
	return graphql.Fields{
		"deployPostgresAdminApp": &graphql.Field{
			Type: serviceGQLType,
			Args: graphql.FieldConfigArgument{
				"databaseId": gqlutil.ReqArg(graphql.String),
				"app":        gqlutil.ReqArg(graphql.String), // pgadmin | pghero
				"username":   gqlutil.Arg(graphql.String),    // omitted: generated, or the caller's email for pgAdmin
				"password":   gqlutil.Arg(graphql.String),    // omitted: generated
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.DeployPostgresAdminApp(p.Context, DeployPostgresAdminAppRequest{
					DatabaseID: p.Args["databaseId"].(string),
					App:        p.Args["app"].(string),
					Username:   gqlutil.Str(p.Args, "username"),
					Password:   gqlutil.Str(p.Args, "password"),
				})
			},
		},
	}
}
