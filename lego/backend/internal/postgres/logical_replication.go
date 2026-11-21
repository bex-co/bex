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
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Logical replication as a publisher (w34/m541; docs/ADR009 § Logical
// replication). Render "do[es] not enable logical replication by default":
// a workspace on Pro or higher asks its support team, naming the database,
// the role subscribers use, the schemas they read and whether to publish all
// tables (render.com/docs/postgresql-logical-replication, read 2026-11-20).
// bex's operators play that support team with this verb. It has no REST,
// GraphQL or MCP surface, as Render has no API field.

// LogicalReplicationPath is the verb on bex-api's cluster-internal :8091
// listener, behind the control-plane bearer (BEX_CP_TOKEN), like the fleet
// rebuild. Never mounted on :8090. Operators reach it with
// `/api postgres-logical-replication` inside the bex-api pod
// (docs/runbooks/postgres-logical-replication.md).
const LogicalReplicationPath = "/internal/postgres-logical-replication"

// The verb's audit rows, recorded against the database with no caller
// identity (RecordDatabaseSystemEffect): the platform changed it at the
// workspace's request.
const (
	AuditVerbEnableLogicalReplication  = "postgres.EnableLogicalReplication"
	AuditVerbDisableLogicalReplication = "postgres.DisableLogicalReplication"
	logicalReplicationAuditCaller      = "bex-operations"
)

// The verb's refusal codes. Each refuses before anything is written.
const (
	LogicalReplicationPlanRequiredCode     = "LOGICAL_REPLICATION_PLAN_REQUIRED"
	LogicalReplicationFreePlanCode         = "LOGICAL_REPLICATION_FREE_PLAN"
	LogicalReplicationStorageTooSmallCode  = "LOGICAL_REPLICATION_STORAGE_TOO_SMALL"
	LogicalReplicationRoleUnknownCode      = "LOGICAL_REPLICATION_ROLE_UNKNOWN"
	LogicalReplicationSchemaUnknownCode    = "LOGICAL_REPLICATION_SCHEMA_UNKNOWN"
	LogicalReplicationUnavailableCode      = "LOGICAL_REPLICATION_DATABASE_UNAVAILABLE"
	LogicalReplicationPublicationInUseCode = "LOGICAL_REPLICATION_PUBLICATION_NAME_IN_USE"
	logicalReplicationInvalidRequestCode   = "LOGICAL_REPLICATION_INVALID_REQUEST"
)

// How long the verb waits for the operator and CNPG to converge, and how
// often it reads the database meanwhile. Variables so tests can shorten them.
var (
	logicalReplicationConvergeTimeout = 2 * time.Minute
	logicalReplicationPollInterval    = 2 * time.Second
)

// logicalReplicationWriteTimeout covers the wait plus the SQL around it: the
// :8091 server's own WriteTimeout is 30 s.
const logicalReplicationWriteTimeout = 4 * time.Minute

// LogicalReplicationRequest is one enable or disable: Render's request inputs.
type LogicalReplicationRequest struct {
	// Database is the dpg- id.
	Database string `json:"database"`
	// Enable true makes the database a publisher; false stops it publishing.
	Enable bool `json:"enable"`
	// PublisherRole, Schemas and AllTables are read only when enabling.
	PublisherRole string   `json:"publisherRole,omitempty"`
	Schemas       []string `json:"schemas,omitempty"`
	AllTables     bool     `json:"allTables,omitempty"`
}

// LogicalReplicationResult reports the outcome, read back from the database
// itself.
type LogicalReplicationResult struct {
	Database      string   `json:"database"`
	Enabled       bool     `json:"enabled"`
	PublisherRole string   `json:"publisherRole,omitempty"`
	Schemas       []string `json:"schemas,omitempty"`
	AllTables     bool     `json:"allTables"`
	// Publication is the all-tables publication's name, when requested.
	Publication string `json:"publication,omitempty"`
	// ExternalHost is the host subscribers connect to (never the pooler's).
	ExternalHost string `json:"externalHost,omitempty"`
	// Converged: PostgreSQL reports the role's REPLICATION attribute and the
	// all-tables publication as requested.
	Converged       bool `json:"converged"`
	RoleReplication bool `json:"roleReplication"`
	PublicationLive bool `json:"publicationLive"`
	// DroppedSlots are the logical replication slots a disable dropped;
	// RemainingSlots are any it could not, which still hold back WAL.
	DroppedSlots   []string `json:"droppedSlots,omitempty"`
	RemainingSlots []string `json:"remainingSlots,omitempty"`
	// Message is the operator's LogicalReplication condition when the
	// database did not converge in time.
	Message string `json:"message,omitempty"`
}

// SetLogicalReplication enables or disables d as a logical-replication
// publisher. It is the platform's own action on a workspace's request, so it
// carries no caller authorization: the caller is the cluster-internal bearer.
func (s *Service) SetLogicalReplication(ctx context.Context, req LogicalReplicationRequest) (LogicalReplicationResult, error) {
	d, err := s.findDatabaseByID(ctx, req.Database)
	if err != nil {
		return LogicalReplicationResult{}, err
	}
	if err := requirePublishable(d); err != nil {
		return LogicalReplicationResult{}, err
	}
	if !req.Enable {
		return s.disableLogicalReplication(ctx, d)
	}
	lr, err := s.checkLogicalReplicationRequest(ctx, d, req)
	if err != nil {
		return LogicalReplicationResult{}, err
	}
	uri, err := s.databaseURI(ctx, d)
	if err != nil {
		return LogicalReplicationResult{}, err
	}
	if err := checkPublisherSQL(ctx, uri, lr); err != nil {
		return LogicalReplicationResult{}, err
	}
	// Render's form asks for the schemas, so its support grants them. The
	// grants are idempotent, so re-running a request repairs them.
	if err := grantPublisherSchemas(ctx, uri, lr, ownerPrivilegedRoles(d)); err != nil {
		return LogicalReplicationResult{}, err
	}
	// Re-running the same request (to check on it) writes no spec and records
	// nothing new.
	if !reflect.DeepEqual(d.Spec.LogicalReplication, lr) {
		if d, err = s.writeLogicalReplication(ctx, d, lr); err != nil {
			return LogicalReplicationResult{}, err
		}
		s.RecordDatabaseSystemEffect(ctx, d, AuditVerbEnableLogicalReplication, logicalReplicationAuditCaller)
	}
	return s.awaitLogicalReplication(ctx, d, uri, lr), nil
}

// slotDropBudget bounds the whole slot-drop phase of a disable, so many
// contested slots cannot run it past the verb's write deadline; the slots
// left are reported.
const slotDropBudget = 45 * time.Second

// disableLogicalReplication drops the publisher's slots, clears the spec and
// cuts the walsenders left. A slot nobody follows any more would otherwise
// hold back WAL with no bound, since the bound leaves with the publisher.
func (s *Service) disableLogicalReplication(ctx context.Context, d *appv1alpha1.Database) (LogicalReplicationResult, error) {
	lr := d.Spec.LogicalReplication
	uri, err := s.databaseURI(ctx, d)
	if err != nil {
		return LogicalReplicationResult{}, err
	}
	if lr == nil {
		return s.readLogicalReplication(ctx, d, uri, nil), nil
	}
	// The disable goes ahead whatever the drop managed: the slots left are
	// reported, with why.
	var dropped, problems []string
	publisherURI, err := s.roleURI(ctx, d, uri, lr.PublisherRole)
	if err == nil {
		dropCtx, cancel := context.WithTimeout(ctx, slotDropBudget)
		dropped, err = dropLogicalSlots(dropCtx, publisherURI)
		cancel()
	}
	if err != nil {
		problems = append(problems, "dropping the slots: "+err.Error())
	}
	if d, err = s.writeLogicalReplication(ctx, d, nil); err != nil {
		return LogicalReplicationResult{}, err
	}
	s.RecordDatabaseSystemEffect(ctx, d, AuditVerbDisableLogicalReplication, logicalReplicationAuditCaller)
	res := s.awaitLogicalReplication(ctx, d, uri, nil)
	if publisherURI != "" {
		if err := terminateWalsenders(ctx, publisherURI); err != nil {
			problems = append(problems, "ending the walsenders: "+err.Error())
		}
	}
	res.DroppedSlots = dropped
	if res.RemainingSlots, err = logicalSlots(ctx, uri); err != nil {
		problems = append(problems, "listing the slots left: "+err.Error())
	}
	res.Message = joinMessages(append([]string{res.Message}, problems...)...)
	if len(problems) > 0 {
		log.Printf("postgres logical replication: disable %s: %s", d.Name, strings.Join(problems, "; "))
	}
	return res, nil
}

// joinMessages joins the non-empty parts of a result's Message.
func joinMessages(parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(p string) bool { return p == "" }), "; ")
}

// findDatabaseByID locates a Database CR by id in any namespace, with no
// caller authorization (the verb's caller is the platform).
func (s *Service) findDatabaseByID(ctx context.Context, id string) (*appv1alpha1.Database, error) {
	if id == "" {
		return nil, core.NewBadRequestError(logicalReplicationInvalidRequestCode, "database is required (its dpg- id)", nil)
	}
	var list appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	// A datastore namespace cutover can leave a stale twin of the same name
	// (ADR043); prefer the live copy, as every other lookup does.
	items := core.DedupDatabaseTwins(slices.DeleteFunc(list.Items, func(d appv1alpha1.Database) bool { return d.Name != id }))
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: postgres %q", core.ErrNotFound, id)
	}
	return &items[0], nil
}

// requirePublishable refuses a database whose instance cannot run the SQL
// the verb needs: deleting, suspended, or not available right now.
func requirePublishable(d *appv1alpha1.Database) error {
	state := ""
	switch {
	case d.DeletionTimestamp != nil:
		state = "being deleted"
	case d.Spec.Suspended:
		state = "suspended; resume it first"
	case d.Status.Phase != appv1alpha1.DBPhaseReady || dbStatus(d) != core.DatastoreAvailable:
		state = dbStatus(d) + "; try again once it is available"
	default:
		return nil
	}
	return core.NewUnavailableError(LogicalReplicationUnavailableCode, "the database is "+state,
		map[string]any{"database": d.Name, "status": dbStatus(d)})
}

// checkLogicalReplicationRequest applies Render's prerequisites and the
// request's own validity, reading nothing but the CR and the workspace plan.
func (s *Service) checkLogicalReplicationRequest(ctx context.Context, d *appv1alpha1.Database, req LogicalReplicationRequest) (*appv1alpha1.DatabaseLogicalReplication, error) {
	workspace := d.Labels[core.LabelTenant]
	params := map[string]any{"database": d.Name}
	plan, ok, err := s.WorkspacePlanOf(ctx, workspace)
	if err != nil {
		return nil, err
	}
	// Fail closed: a workspace whose plan cannot be read gets nothing.
	if !ok || !core.PlanIncludes(plan, core.FeatureLogicalReplication) {
		message := "logical replication requires the Pro plan or higher; this database's workspace plan could not be read"
		if ok && plan != "" {
			message = core.PlanFeatureMessage(plan, core.FeatureLogicalReplication)
		}
		params["plan"] = plan
		return nil, core.NewBadRequestError(LogicalReplicationPlanRequiredCode, message, params)
	}
	if isFreePlan(d.Spec.Plan) {
		return nil, core.NewBadRequestError(LogicalReplicationFreePlanCode, "logical replication is not available for a free database; change it to a paid plan first", params)
	}
	storage := tiers.Postgres.EffectiveStorageGB(d.Spec.Plan, d.Spec.StorageGB, d.Status.AllocatedStorageGB)
	if storage < tiers.PostgresLogicalReplicationMinStorageGB {
		params["storageGB"], params["minStorageGB"] = storage, tiers.PostgresLogicalReplicationMinStorageGB
		return nil, core.NewBadRequestError(LogicalReplicationStorageTooSmallCode,
			fmt.Sprintf("logical replication requires at least %d GB of storage; this database has %d GB", tiers.PostgresLogicalReplicationMinStorageGB, storage), params)
	}
	if !publisherCandidate(d, req.PublisherRole) {
		params["publisherRole"] = req.PublisherRole
		return nil, core.NewBadRequestError(LogicalReplicationRoleUnknownCode,
			fmt.Sprintf("publisherRole %q is not one of this database's credentials (its owner or a user added to it)", req.PublisherRole), params)
	}
	schemas := slices.Clone(req.Schemas)
	slices.Sort(schemas)
	schemas = slices.Compact(schemas)
	if len(schemas) == 0 {
		return nil, core.NewBadRequestError(logicalReplicationInvalidRequestCode, "schemas is required: the schemas subscribers read", params)
	}
	for _, schema := range schemas {
		if schema == "" || len(schema) > 63 || strings.ContainsRune(schema, 0) {
			params["schema"] = schema
			return nil, core.NewBadRequestError(logicalReplicationInvalidRequestCode, fmt.Sprintf("schema %q is not a valid PostgreSQL schema name", schema), params)
		}
	}
	return &appv1alpha1.DatabaseLogicalReplication{PublisherRole: req.PublisherRole, Schemas: schemas, AllTables: req.AllTables}, nil
}

// publisherCandidate reports whether role is a live credential of d: its
// owner or a user, never a retired login or a role PostgreSQL reserves.
func publisherCandidate(d *appv1alpha1.Database, role string) bool {
	if !appv1alpha1.ValidPostgresIdentifier(role) || appv1alpha1.ReservedPostgresRole(role) || slices.Contains(d.Spec.DeletedUsers, role) {
		return false
	}
	return role == d.Spec.EffectiveDatabaseUser(d.Name) ||
		slices.ContainsFunc(d.Spec.Users, func(u appv1alpha1.DatabaseUser) bool { return u.Name == role })
}

// ownerPrivilegedRoles are the roles that may own the tenant's tables: the
// owner and every credential with owner privileges. New tables any of them
// creates are granted to the publisher too.
func ownerPrivilegedRoles(d *appv1alpha1.Database) []string {
	roles := []string{d.Spec.EffectiveDatabaseUser(d.Name)}
	for _, u := range d.Spec.Users {
		if u.OwnerPrivileges {
			roles = append(roles, u.Name)
		}
	}
	return roles
}

// writeLogicalReplication sets spec.logicalReplication (nil clears it) under
// an optimistic lock.
func (s *Service) writeLogicalReplication(ctx context.Context, d *appv1alpha1.Database, lr *appv1alpha1.DatabaseLogicalReplication) (*appv1alpha1.Database, error) {
	patch := client.MergeFromWithOptions(d.DeepCopy(), client.MergeFromWithOptimisticLock{})
	d.Spec.LogicalReplication = lr
	if err := s.Client.Patch(ctx, d, patch); err != nil {
		return nil, err
	}
	return d, nil
}

// awaitLogicalReplication waits until PostgreSQL itself reports lr (nil: no
// publisher), or the timeout, and reports what it read last.
func (s *Service) awaitLogicalReplication(ctx context.Context, d *appv1alpha1.Database, uri string, lr *appv1alpha1.DatabaseLogicalReplication) LogicalReplicationResult {
	deadline := time.Now().Add(logicalReplicationConvergeTimeout)
	for {
		res := s.readLogicalReplication(ctx, d, uri, lr)
		if res.Converged || time.Now().After(deadline) || ctx.Err() != nil {
			if !res.Converged {
				res.Message = joinMessages(res.Message, s.logicalReplicationCondition(ctx, d))
			}
			return res
		}
		select {
		case <-ctx.Done():
		case <-time.After(logicalReplicationPollInterval):
		}
	}
}

// readLogicalReplication reads the publisher's state from the database.
func (s *Service) readLogicalReplication(ctx context.Context, d *appv1alpha1.Database, uri string, lr *appv1alpha1.DatabaseLogicalReplication) LogicalReplicationResult {
	res := LogicalReplicationResult{Database: d.Name, Enabled: lr != nil, ExternalHost: d.Status.ExternalHost}
	if lr != nil {
		res.PublisherRole, res.Schemas, res.AllTables = lr.PublisherRole, lr.Schemas, lr.AllTables
		if lr.AllTables {
			res.Publication = appv1alpha1.LogicalReplicationAllTablesPublication
		}
	}
	replicates, publication, err := publisherState(ctx, uri, res.PublisherRole)
	if err != nil {
		res.Message = "could not read the database: " + err.Error()
		return res
	}
	res.RoleReplication, res.PublicationLive = replicates, publication
	if lr == nil {
		res.Converged = !publication
	} else {
		res.Converged = replicates && publication == lr.AllTables
	}
	return res
}

// logicalReplicationCondition is the operator's report on the publisher,
// read fresh: the verb's copy of d predates the operator's work.
func (s *Service) logicalReplicationCondition(ctx context.Context, d *appv1alpha1.Database) string {
	var fresh appv1alpha1.Database
	if err := s.Client.Get(ctx, client.ObjectKeyFromObject(d), &fresh); err != nil {
		return "could not read the operator's report: " + err.Error()
	}
	if c := meta.FindStatusCondition(fresh.Status.Conditions, appv1alpha1.ConditionLogicalReplication); c != nil {
		return c.Reason + ": " + c.Message
	}
	return "the operator has not reported on the publisher yet"
}

// roleURI is uri with role's credentials: the owner's from CNPG's
// application Secret, a user's from its own Secret.
func (s *Service) roleURI(ctx context.Context, d *appv1alpha1.Database, uri, role string) (string, error) {
	secretName := appv1alpha1.ApplicationSecretName(d.Name)
	if role != d.Spec.EffectiveDatabaseUser(d.Name) {
		i := slices.IndexFunc(d.Spec.Users, func(u appv1alpha1.DatabaseUser) bool { return u.Name == role })
		if i < 0 || d.Spec.Users[i].SecretName == "" {
			return "", fmt.Errorf("no Secret holds %s's password", role)
		}
		secretName = d.Spec.Users[i].SecretName
	}
	var sec corev1.Secret
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: secretName}, &sec); err != nil {
		return "", err
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", errors.New("the connection URI does not parse")
	}
	parsed.User = url.UserPassword(role, string(sec.Data["password"]))
	return parsed.String(), nil
}

// --- SQL ---

// logicalReplicationLimits bound each statement the verb runs and each dial.
var logicalReplicationLimits = queryLimits{statementTimeout: 15 * time.Second}

const logicalReplicationConnectTimeout = 10 * time.Second

// connectForLogicalReplication dials uri with the console's session rails,
// read-write. The cause stays in the error: the caller is an operator.
func connectForLogicalReplication(ctx context.Context, uri string) (*pgx.Conn, error) {
	cfg, err := buildQueryConnConfig(uri, logicalReplicationLimits, false)
	if err != nil {
		return nil, err
	}
	cfg.ConnectTimeout = logicalReplicationConnectTimeout
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errQueryConnClose, err)
	}
	return conn, nil
}

// checkPublisherSQL refuses a schema the database does not have, and an
// existing publication of the all-tables name that is not a FOR ALL TABLES
// one (a user's FOR TABLE publication, which CNPG would adopt and, on disable,
// drop).
func checkPublisherSQL(ctx context.Context, uri string, lr *appv1alpha1.DatabaseLogicalReplication) error {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	var missing []string
	if err := conn.QueryRow(ctx,
		`SELECT coalesce(array_agg(n ORDER BY n), '{}') FROM unnest($1::text[]) AS n WHERE NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = n)`,
		lr.Schemas).Scan(&missing); err != nil {
		return mapPGError(err)
	}
	if len(missing) > 0 {
		return core.NewBadRequestError(LogicalReplicationSchemaUnknownCode,
			"the database has no schema named "+strings.Join(missing, ", "), map[string]any{"schemas": missing})
	}
	if !lr.AllTables {
		return nil
	}
	var allTables bool
	err = conn.QueryRow(ctx, `SELECT puballtables FROM pg_catalog.pg_publication WHERE pubname = $1`, appv1alpha1.LogicalReplicationAllTablesPublication).Scan(&allTables)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return mapPGError(err)
	}
	if !allTables {
		return core.NewConflictError(LogicalReplicationPublicationInUseCode,
			fmt.Sprintf("the database already has a publication named %s that is not FOR ALL TABLES; drop or rename it first", appv1alpha1.LogicalReplicationAllTablesPublication),
			map[string]any{"publication": appv1alpha1.LogicalReplicationAllTablesPublication})
	}
	return nil
}

// grantPublisherSchemas grants the publisher USAGE and SELECT on each
// schema, now and for tables created later by any role that may own the
// tenant's tables. It runs as the database's default user, who belongs to
// every such role, in one transaction. A table no such role owns is left to
// its owner (PostgreSQL warns and grants nothing).
func grantPublisherSchemas(ctx context.Context, uri string, lr *appv1alpha1.DatabaseLogicalReplication, owners []string) error {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	var creators []string
	if err := conn.QueryRow(ctx,
		`SELECT coalesce(array_agg(r ORDER BY r), '{}') FROM unnest($1::text[]) AS r WHERE EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = r) AND pg_catalog.pg_has_role(current_user, r, 'MEMBER')`,
		owners).Scan(&creators); err != nil {
		return mapPGError(err)
	}
	role := pgx.Identifier{lr.PublisherRole}.Sanitize()
	forRoles := make([]string, len(creators))
	for i, r := range creators {
		forRoles[i] = pgx.Identifier{r}.Sanitize()
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return mapPGError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, name := range lr.Schemas {
		schema := pgx.Identifier{name}.Sanitize()
		statements := []string{
			"GRANT USAGE ON SCHEMA " + schema + " TO " + role,
			"GRANT SELECT ON ALL TABLES IN SCHEMA " + schema + " TO " + role,
		}
		if len(forRoles) > 0 {
			statements = append(statements, "ALTER DEFAULT PRIVILEGES FOR ROLE "+strings.Join(forRoles, ", ")+" IN SCHEMA "+schema+" GRANT SELECT ON TABLES TO "+role)
		}
		for _, statement := range statements {
			if _, err := tx.Exec(ctx, statement); err != nil {
				return fmt.Errorf("granting %s on schema %s: %w", lr.PublisherRole, name, mapPGError(err))
			}
		}
	}
	return mapPGError(tx.Commit(ctx))
}

// publisherState reads whether role holds REPLICATION (false for an empty
// role) and whether the all-tables publication exists as FOR ALL TABLES.
func publisherState(ctx context.Context, uri, role string) (replicates, allTables bool, err error) {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return false, false, err
	}
	defer func() { _ = conn.Close(ctx) }()
	err = conn.QueryRow(ctx,
		`SELECT coalesce((SELECT rolreplication FROM pg_catalog.pg_roles WHERE rolname = $1), false),
		        coalesce((SELECT puballtables FROM pg_catalog.pg_publication WHERE pubname = $2), false)`,
		role, appv1alpha1.LogicalReplicationAllTablesPublication).Scan(&replicates, &allTables)
	return replicates, allTables, mapPGError(err)
}

// listLogicalSlots lists the current database's logical replication slots,
// which pg_replication_slots shows every user.
func listLogicalSlots(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	rows, err := conn.Query(ctx, `SELECT slot_name FROM pg_catalog.pg_replication_slots WHERE slot_type = 'logical' AND database = current_database() AND NOT temporary ORDER BY slot_name`)
	if err != nil {
		return nil, mapPGError(err)
	}
	slots, err := pgx.CollectRows(rows, pgx.RowTo[string])
	return slots, mapPGError(err)
}

func logicalSlots(ctx context.Context, uri string) ([]string, error) {
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(ctx) }()
	return listLogicalSlots(ctx, conn)
}

// dropLogicalSlots drops the current database's logical slots, connected as
// the publisher while it still holds REPLICATION (which dropping a slot
// needs). A subscriber's apply worker reconnects about a second after its
// walsender ends, so each slot is taken in a tight loop: end the walsender
// holding it, wait for the slot to be released, drop it at once, and start
// over if a reconnect won the race.
func dropLogicalSlots(ctx context.Context, publisherURI string) ([]string, error) {
	conn, err := connectForLogicalReplication(ctx, publisherURI)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	slots, err := listLogicalSlots(ctx, conn)
	if err != nil {
		return nil, err
	}
	var dropped []string
	for _, slot := range slots {
		if err := dropLogicalSlot(ctx, conn, slot); err != nil {
			return dropped, err
		}
		dropped = append(dropped, slot)
	}
	return dropped, nil
}

// slotDropAttempts bounds dropLogicalSlot's races against a reconnecting
// subscriber; each attempt waits at most slotReleaseWait for the walsender
// to exit.
const (
	slotDropAttempts = 20
	slotReleaseWait  = 2 * time.Second
)

func dropLogicalSlot(ctx context.Context, conn *pgx.Conn, slot string) error {
	var err error
	for range slotDropAttempts {
		if _, err = conn.Exec(ctx, `SELECT pg_catalog.pg_terminate_backend(active_pid) FROM pg_catalog.pg_replication_slots WHERE slot_name = $1 AND active_pid IS NOT NULL`, slot); err != nil {
			return mapPGError(err)
		}
		gone, waitErr := awaitSlotRelease(ctx, conn, slot)
		if waitErr != nil || gone {
			return waitErr
		}
		_, err = conn.Exec(ctx, `SELECT pg_catalog.pg_drop_replication_slot($1)`, slot)
		var pgErr *pgconn.PgError
		if err == nil || !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.ObjectInUse {
			break
		}
	}
	if err != nil && !isUndefinedObject(err) {
		return mapPGError(err)
	}
	return nil
}

// awaitSlotRelease polls until slot is inactive (its walsender exited) or
// gone, for at most slotReleaseWait.
func awaitSlotRelease(ctx context.Context, conn *pgx.Conn, slot string) (gone bool, err error) {
	deadline := time.Now().Add(slotReleaseWait)
	for {
		var active bool
		err := conn.QueryRow(ctx, `SELECT active FROM pg_catalog.pg_replication_slots WHERE slot_name = $1`, slot).Scan(&active)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return true, nil
		case err != nil:
			return false, mapPGError(err)
		case !active || time.Now().After(deadline):
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func isUndefinedObject(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UndefinedObject
}

// terminateWalsenders ends every walsender still connected as the publisher
// after it lost REPLICATION, so a subscriber's next connection is refused
// rather than a session from before the disable streaming on.
func terminateWalsenders(ctx context.Context, publisherURI string) error {
	conn, err := connectForLogicalReplication(ctx, publisherURI)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, `SELECT pg_catalog.pg_terminate_backend(pid) FROM pg_catalog.pg_stat_activity
		WHERE backend_type = 'walsender' AND usename = current_user AND pid <> pg_catalog.pg_backend_pid()`)
	return mapPGError(err)
}

// --- transport ---

// LogicalReplicationHandler serves POST LogicalReplicationPath. Token is the
// control-plane bearer; an empty token never authorizes.
func LogicalReplicationHandler(s *Service, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := []byte("Bearer " + token)
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			core.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(logicalReplicationWriteTimeout))
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		req, err := core.DecodeBody[LogicalReplicationRequest](r)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		res, err := s.SetLogicalReplication(r.Context(), req)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, res)
	})
}
