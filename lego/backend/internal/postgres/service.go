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

// Package postgres is the managed-Postgres feature over the Database CR,
// mirroring Render's /v1/postgres API. One Service the REST + GraphQL adapters
// share; the connection-info verb is the one place the DB password is surfaced
// (to an authenticated caller), read from CNPG's generated Secret at request time.
package postgres

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var supportedPostgresVersions = []string{"13", "14", "15", "16", "17", "18"}

func postgresVersionKnown(version string) bool {
	for _, supported := range supportedPostgresVersions {
		if version == supported {
			return true
		}
	}
	return false
}

func supportedPostgresVersionText() string {
	return strings.Join(supportedPostgresVersions, "|")
}

// Service exposes managed Postgres as Render's "postgres" shape.
type Service struct {
	*core.Base
	Protection core.EnvironmentProtectionStore
	Owners     resourcemeta.OwnerResolver
	Metadata   resourcemeta.Config
	// Environments is the shared create-time assignment resolver used by all
	// three resource kinds.
	Environments core.EnvironmentResolver
	// ExportSigner mints short-lived object-store download URLs after the
	// ListExports verb has passed can_view_sensitive. Production wires the
	// Kubernetes Secret-backed S3 signer; tests can replace it.
	ExportSigner ExportURLSigner
	// DatabaseLogs is the production query seam for canonical dpg- resources. The
	// API composition root wires it to the generic durable logs service so the
	// dedicated compatibility endpoints and Render's /logs surface share one
	// authorization/filtering engine. nil enables the direct-pod test seam.
	DatabaseLogs DatabaseLogQuerySource
	// PodLogs backs the direct-pod path used by isolated tests. nil with no
	// DatabaseLogs source => ErrLogsUnavailable.
	PodLogs core.PodLogSource
	// queryExecutor is the SQL transport seam used by Query and ExecuteQuery.
	// Production leaves it nil and uses pgx; tests replace it so the REST and
	// GraphQL adapters can exercise authz + secret resolution without a live DB.
	queryExecutor queryExecutor
}

// PostgresView is the Render-shaped "postgres" object.
type PostgresView struct {
	ID           string `json:"id"` // immutable dpg-... id
	Name         string `json:"name"`
	Plan         string `json:"plan"`
	Version      string `json:"version,omitempty"`
	Status       string `json:"status"`       // Render databaseStatus enum
	DatabaseName string `json:"databaseName"` // the actual (normalized) db
	DatabaseUser string `json:"databaseUser"`
	DiskSizeGB   int32  `json:"diskSizeGB,omitempty"`
	// DiskAutoscalingEnabled is Render's read-side field. Writes use the
	// intentionally asymmetric enableDiskAutoscaling input name.
	DiskAutoscalingEnabled bool `json:"diskAutoscalingEnabled"`

	// HighAvailabilityEnabled reflects the operator's observed state (≥2 ready
	// instances). Render's highAvailabilityEnabled read field.
	HighAvailabilityEnabled bool `json:"highAvailabilityEnabled"`
	// ReadReplicas is the named replica array — each with its host info.
	// Password is not included here; use PostgresConnectionInfo for credentials.
	// Render's readReplicas: [{name, connectionInfo}].
	ReadReplicas []ReadReplicaView `json:"readReplicas,omitempty"`

	Suspended string `json:"suspended"` // string enum, like services
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`

	// Region / DashboardURL mirror the Render fields on services; populated by
	// the Service.view wrapper (not pgView) so they flow through GraphQL/MCP.
	Region       string `json:"region,omitempty"`
	DashboardURL string `json:"dashboardUrl,omitempty"`

	// bex-native extras (Render clients ignore unknown keys).
	ExternalHost string `json:"externalHost,omitempty"`
	Public       bool   `json:"public"`

	// IPAllowList is the CIDR allowlist gating the EXTERNAL endpoint (Render's
	// ipAllowList). Empty => the external route is open to all source IPs.
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
	// PoolerEnabled reports whether a PgBouncer pooler is provisioned (its pooled
	// connection strings appear in connection-info).
	PoolerEnabled bool `json:"poolerEnabled"`
	// BackupsEnabled reports whether continuous backups (and so recovery/PITR)
	// are active for this instance — surfaced by the controller once projected.
	BackupsEnabled bool `json:"backupsEnabled"`

	// OwnerID is Render's workspace-scoping field (w6/m2/t004), read from the
	// Database CR's core.LabelTenant label (the same one apps.AppView.OwnerID
	// and the App CR projector use). Populated for any Database created via
	// CreatePostgres with the store on (w6/m4/t001); a hand-applied CR without
	// the label still reads as unowned.
	OwnerID string `json:"ownerId,omitempty"`

	// ProjectID is the owning Project's id (w1/m31 extension), read from the
	// Database CR's core.LabelProject label. Empty means unassigned. Set via
	// SetProjectID; the projects feature is the only writer.
	ProjectID string `json:"projectId,omitempty"`

	// EnvironmentID is the owning Environment's id (w6/m20 extension), read
	// from the Database CR's core.LabelEnvironment label. Empty means
	// unassigned. Set via SetEnvironmentID; the environments feature is the
	// only writer.
	EnvironmentID string `json:"environmentId,omitempty"`
}

// ReadReplicaView is one named read replica as returned in the Render-shaped
// postgres object. Maps to Render's readReplicas[{name, connectionInfo}].
// Passwords are not included; use PostgresConnectionInfo for credentials.
type ReadReplicaView struct {
	Name           string                     `json:"name"`
	ConnectionInfo *ReadReplicaConnectionInfo `json:"connectionInfo,omitempty"`
}

// ReadReplicaConnectionInfo holds the internal (and optionally external)
// read-only connection string hosts for a named read replica. The full strings
// (with password) are only available through PostgresConnectionInfo.
type ReadReplicaConnectionInfo struct {
	InternalHost string `json:"internalHost,omitempty"`
	ExternalHost string `json:"externalHost,omitempty"`
}

// ReadReplicaInput is one item in CreatePostgresRequest.ReadReplicas.
type ReadReplicaInput struct {
	Name string `json:"name"`
}

// PostgresConnectionInfo mirrors Render's postgresConnectionInfo schema.
type PostgresConnectionInfo struct {
	Password                 string `json:"password"`
	InternalConnectionString string `json:"internalConnectionString"`
	ExternalConnectionString string `json:"externalConnectionString,omitempty"`
	// Pooler variants are populated when a PgBouncer Pooler is provisioned
	// (spec.pooler); empty otherwise. The external variant additionally needs Public.
	InternalConnectionPoolString string `json:"internalConnectionPoolString,omitempty"`
	ExternalConnectionPoolString string `json:"externalConnectionPoolString,omitempty"`
	PSQLCommand                  string `json:"psqlCommand"`
	// ReadReplicaConnectionStrings has one entry per spec.readReplicas, keyed by
	// replica name. Each is the full internal (and optionally external) connection
	// string including the password, for callers that need to query standbys.
	ReadReplicaConnectionStrings []ReplicaConnectionStrings `json:"readReplicaConnectionStrings,omitempty"`
}

// ReplicaConnectionStrings is the full internal + optional external read-only
// connection string for one named replica (including password).
type ReplicaConnectionStrings struct {
	Name                     string `json:"name"`
	InternalConnectionString string `json:"internalConnectionString,omitempty"`
	ExternalConnectionString string `json:"externalConnectionString,omitempty"`
}

// CreatePostgresRequest is the POST /v1/postgres body (bex subset of Render's).
type CreatePostgresRequest struct {
	// OwnerID is the workspace to create IN — Render's `ownerId` (w6/m14). Empty
	// means the caller's default workspace; a workspace the caller is not a
	// member of is core.ErrForbidden, never a create in the wrong one. Bound to
	// the context by the verb, before its authorization check.
	OwnerID       string `json:"ownerId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
	Name          string `json:"name"`
	// DatabaseName and DatabaseUser are optional create-time physical PostgreSQL
	// identifiers. Empty preserves the stable resource-id-derived defaults.
	DatabaseName string `json:"databaseName,omitempty"`
	DatabaseUser string `json:"databaseUser,omitempty"`
	Plan         string `json:"plan,omitempty"`
	Version      string `json:"version,omitempty"`
	// Region is accepted from Render clients as a placement hint. bex currently
	// exposes one configured region per control plane, so the server-owned
	// BEX_REGION remains authoritative instead of persisting this request field.
	Region     string `json:"region,omitempty"`
	DiskSizeGB int32  `json:"diskSizeGB,omitempty"`
	// Datadog fields are decoded so the Render CLI receives an explicit
	// unsupported error instead of a successful no-op. Pointers distinguish an
	// omitted field from an explicitly supplied empty string without ever
	// echoing the credential.
	DatadogAPIKey *string `json:"datadogAPIKey,omitempty"`
	DatadogSite   *string `json:"datadogSite,omitempty"`
	// EnableDiskAutoscaling is Render's create/update input field.
	EnableDiskAutoscaling bool `json:"enableDiskAutoscaling,omitempty"`
	Public                bool `json:"public,omitempty"`
	// IPAllowList optionally seeds the external-endpoint allowlist at create.
	// Render's wire shape ({cidrBlock, description} entries) — see
	// core.IPAllowListEntry; both fields persist on the CR (w4/m24).
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
	// Pooler optionally provisions a PgBouncer pooler at create.
	Pooler bool `json:"pooler,omitempty"`
	// EnableHighAvailability provisions a replicated CNPG cluster (primary +
	// standby with pod anti-affinity). Render's enableHighAvailability create field.
	// Independent of ReadReplicas.
	EnableHighAvailability bool `json:"enableHighAvailability,omitempty"`
	// ReadReplicas is the named replica array — each entry gets its own
	// addressable read-only connection URL. Render's readReplicas: [{name}].
	// Independent of EnableHighAvailability.
	ReadReplicas []ReadReplicaInput `json:"readReplicas,omitempty"`
	// DryRun, when true, resolves and returns the spec preview without any k8s
	// write — zero side effects (w2/m29). Validation still runs.
	DryRun bool `json:"dryRun,omitempty"`
}

func validateDatabaseName(name string) error {
	if !appv1alpha1.ValidDatabaseName(name) {
		return fmt.Errorf("%w: name must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen", core.ErrBadRequest)
	}
	return nil
}

func validatePhysicalIdentifier(field, name string) error {
	if name == "" {
		return nil
	}
	if !appv1alpha1.ValidPostgresIdentifier(name) {
		return core.NewBadRequestError(
			"POSTGRES_IDENTIFIER_INVALID",
			fmt.Sprintf("%s must start with a lowercase letter or underscore, contain only lowercase letters, digits, and underscores, and be at most 63 bytes", field),
			map[string]any{"field": field},
		)
	}
	return nil
}

func unsupportedDatadogError() error {
	return core.NewBadRequestError(
		"POSTGRES_DATADOG_UNSUPPORTED",
		"Postgres Datadog monitoring is not supported; remove datadogAPIKey and datadogSite",
		nil,
	)
}

func (req CreatePostgresRequest) validatePhysicalIdentifiers() error {
	if req.DatadogAPIKey != nil || req.DatadogSite != nil {
		return unsupportedDatadogError()
	}
	if err := validatePhysicalIdentifier("databaseName", req.DatabaseName); err != nil {
		return err
	}
	return validatePhysicalIdentifier("databaseUser", req.DatabaseUser)
}

// dbStatus maps bex's Database phase onto Render's databaseStatus enum.
func dbStatus(p appv1alpha1.DatabasePhase) string {
	switch p {
	case appv1alpha1.DBPhaseReady:
		return "available"
	case appv1alpha1.DBPhaseUpgrading:
		return "upgrading"
	case appv1alpha1.DBPhaseFailed:
		return "unavailable"
	default:
		return "creating"
	}
}

func pgView(d *appv1alpha1.Database) PostgresView {
	created := ""
	if !d.CreationTimestamp.IsZero() {
		created = d.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	dbn := d.Spec.EffectiveDatabaseName(d.Name)
	dbUser := d.Spec.EffectiveDatabaseUser(d.Name)
	replicas := make([]ReadReplicaView, 0, len(d.Status.ReadReplicaStatuses))
	for _, rs := range d.Status.ReadReplicaStatuses {
		rv := ReadReplicaView{Name: rs.Name}
		if rs.InternalHost != "" || rs.ExternalHost != "" {
			rv.ConnectionInfo = &ReadReplicaConnectionInfo{
				InternalHost: rs.InternalHost,
				ExternalHost: rs.ExternalHost,
			}
		}
		replicas = append(replicas, rv)
	}
	version := d.Status.CurrentVersion
	if version == "" {
		version = d.Spec.Version
	}
	status := dbStatus(d.Status.Phase)
	if !d.DeletionTimestamp.IsZero() {
		status = "deleting"
	}
	return PostgresView{
		ID:                      d.Name,
		Name:                    d.Spec.Name,
		Plan:                    d.Spec.Plan,
		Version:                 version,
		Status:                  status,
		DatabaseName:            dbn,
		DatabaseUser:            dbUser,
		DiskSizeGB:              databaseStorageHighWater(d),
		DiskAutoscalingEnabled:  d.Spec.DiskAutoscaling,
		HighAvailabilityEnabled: d.Status.HighAvailabilityEnabled,
		ReadReplicas:            replicas,
		Suspended:               core.SuspendedEnum(d.Spec.Suspended),
		CreatedAt:               created,
		UpdatedAt:               resourcemeta.UpdatedAt(d),
		ExternalHost:            d.Status.ExternalHost,
		Public:                  d.Spec.Public,
		IPAllowList:             core.AllowListFromSpec(d.Spec.IPAllowList),
		PoolerEnabled:           d.Spec.Pooler,
		BackupsEnabled:          d.Status.BackupsEnabled,
		OwnerID:                 d.Labels[core.LabelTenant],
		ProjectID:               d.Labels[core.LabelProject],
		EnvironmentID:           d.Labels[core.LabelEnvironment],
	}
}

// view wraps pgView and stamps the platform region and dashboard URL onto the
// result, matching the apps.Service.view pattern so all external-facing verbs
// return the enriched shape through GraphQL and MCP as well as REST.
func (s *Service) view(d *appv1alpha1.Database) PostgresView {
	v := pgView(d)
	v.Region = s.Metadata.PlatformRegion()
	v.DashboardURL = s.Metadata.DashboardURL(resourcemeta.PostgresDashboardRoute, v.ID)
	return v
}

// fetchDatabase resolves a Database by name through the shared core.Base seam
// (w6/m17's core.Base.AuthorizeDatabase: authorize + fetch in one call, against
// the Database's OWN workspace — the same rule apps.AuthorizeApp applies; also
// shared by internal/metrics' w3/m10 datastore-scoped metrics). Kept as a thin
// wrapper so this package's many call sites don't all need to spell
// core.Base.AuthorizeDatabase.
func (s *Service) fetchDatabase(ctx context.Context, relation, name string) (*appv1alpha1.Database, error) {
	return s.AuthorizeDatabase(ctx, relation, name)
}

// loadAppSecret resolves a Database and its CNPG-generated "<stable-id>-app" Secret
// (username/password/dbname/uri) — the credential path both connection-info and
// the read-only query verb share. Returns core.ErrNotFound when the Database or
// its Secret isn't provisioned yet.
func (s *Service) loadAppSecret(ctx context.Context, relation, name string) (*appv1alpha1.Database, *corev1.Secret, error) {
	d, err := s.fetchDatabase(ctx, relation, name)
	if err != nil {
		return nil, nil, err
	}
	sec, err := s.databaseSecret(ctx, d)
	if err != nil {
		return nil, nil, err
	}
	return d, sec, nil
}

// ListPostgres returns every managed Postgres in the namespace, optionally
// narrowed to a single owning workspace — Render's `ownerId` list-filter
// contract (w6/m2/t004, labeling fixed by w6/m4/t001), mirroring
// apps.Service.List. ownerID == "" resolves to the caller's default workspace
// when the control-plane workspace resolver is enabled. A non-empty ownerID names
// the workspace to list (core.WithWorkspace), authorized+membership-checked by
// the same resolveWorkspace mechanism every other verb uses (w6/m17 —
// previously an OpenFGA-only check with no IsMember) and then filters by
// core.LabelTenant; never silently returns unscoped data for a scoped request.
func (s *Service) ListPostgres(ctx context.Context, ownerID string) ([]PostgresView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	tenantID := ownerID
	if tenantID == "" && s.Workspace != nil {
		var ok bool
		tenantID, ok = s.Tenant(ctx)
		if !ok {
			return []PostgresView{}, nil
		}
	}
	// Scoping is pushed into the list call itself (server-side label selector)
	// rather than fetching a namespace and filtering in memory. Under ADR043 D8
	// the selector also has to be what carries the tenant boundary: a workspace's
	// datastores live in its own namespace, so no single namespace holds them all.
	opts := s.DatastoreListOptions(tenantID)
	var list appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]PostgresView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, s.view(&list.Items[i]))
	}
	return out, nil
}

// GetPostgres returns one managed Postgres, or core.ErrNotFound.
func (s *Service) GetPostgres(ctx context.Context, name string) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return PostgresView{}, err
	}
	return s.view(d), nil
}

// ensureDatabaseNameAvailable enforces Render's workspace-scoped display-name
// uniqueness without coupling identity to that name. excludeID is the stable
// metadata.name of an object being renamed. Unlabelled dev objects form
// their own scope when the control-plane tenant resolver is disabled.
func (s *Service) ensureDatabaseNameAvailable(ctx context.Context, tenantID, name, excludeID string) error {
	var list appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &list, s.DatastoreListOptions(tenantID)...); err != nil {
		return fmt.Errorf("checking database name: %w", err)
	}
	for i := range list.Items {
		d := &list.Items[i]
		if d.Name == excludeID || d.Labels[core.LabelTenant] != tenantID {
			continue
		}
		if d.Spec.Name == name {
			return fmt.Errorf("%w: a Postgres database named %q already exists in this workspace", core.ErrConflict, name)
		}
	}
	return nil
}

// CreatePostgres provisions a managed Postgres (a Database CR the operator
// projects to a CNPG Cluster).
func (s *Service) CreatePostgres(ctx context.Context, req CreatePostgresRequest) (PostgresView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return PostgresView{}, err
	}
	if err := validateDatabaseName(req.Name); err != nil {
		return PostgresView{}, err
	}
	if err := req.validatePhysicalIdentifiers(); err != nil {
		return PostgresView{}, err
	}
	tenantID, tenantOK := s.Tenant(ctx)
	if err := s.ensureDatabaseNameAvailable(ctx, tenantID, req.Name, ""); err != nil {
		return PostgresView{}, err
	}
	if req.Version != "" && !postgresVersionKnown(req.Version) {
		return PostgresView{}, unknownPostgresVersionError(req.Version)
	}
	if err := core.ValidateAllowList(req.IPAllowList); err != nil {
		return PostgresView{}, err
	}
	var environment core.EnvironmentAssignment
	if req.EnvironmentID != "" {
		if s.Environments == nil || !tenantOK {
			return PostgresView{}, core.ErrWorkspacesUnavailable
		}
		var err error
		environment, err = s.Environments.ResolveForCreate(ctx, req.EnvironmentID, tenantID)
		if err != nil {
			return PostgresView{}, fmt.Errorf("resolving environment: %w", err)
		}
	}
	crReplicas := make([]appv1alpha1.DatabaseReadReplica, 0, len(req.ReadReplicas))
	for _, r := range req.ReadReplicas {
		crReplicas = append(crReplicas, appv1alpha1.DatabaseReadReplica{Name: r.Name})
	}
	d := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: id.New(id.Postgres), Namespace: s.TenantNamespace(tenantID)},
		Spec: appv1alpha1.DatabaseSpec{
			Name:             req.Name,
			DatabaseName:     req.DatabaseName,
			DatabaseUser:     req.DatabaseUser,
			Plan:             req.Plan,
			Version:          req.Version,
			StorageGB:        req.DiskSizeGB,
			DiskAutoscaling:  req.EnableDiskAutoscaling,
			Public:           req.Public,
			IPAllowList:      core.AllowListToSpec(req.IPAllowList),
			Pooler:           req.Pooler,
			HighAvailability: req.EnableHighAvailability,
			ReadReplicas:     crReplicas,
		},
	}
	// Stamp both the tenant label (ownerId scoping — pgView/ListPostgres read
	// this) and the workspace label (so the database controller can propagate
	// it to CNPG pod metadata for same-workspace NetworkPolicy selectors,
	// docs/ADR022-tenant-isolation.md), mirroring the App CR dual-stamp
	// (store/reconciler.go's stampLabels). Skip when the store is off (no resolver).
	if tenantOK {
		d.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
	}
	if environment.ID != "" {
		d.Labels[core.LabelProject] = environment.ProjectID
		d.Labels[core.LabelEnvironment] = environment.ID
		// Newborn members inherit the environment's inbound-IP layer (w4/m28).
		d.Spec.EnvironmentIPAllowList = core.EnvironmentLayerCIDRs(environment.IPAllowList)
	}
	// Dry-run: return the resolved spec preview without any k8s write (w2/m29).
	if req.DryRun {
		return s.view(d), nil
	}
	if core.PaidPlan(req.Plan) {
		if err := s.RequirePaymentMethod(ctx, tenantID); err != nil {
			return PostgresView{}, err
		}
	}
	if err := s.RequireBillingMutation(ctx, tenantID); err != nil {
		return PostgresView{}, err
	}
	resourcemeta.Touch(d, s.Now())
	if err := s.Client.Create(ctx, d); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return PostgresView{}, fmt.Errorf("%w: generated Postgres id collision; retry the request", core.ErrConflict)
		}
		// The per-namespace ResourceQuota is what enforces the plan's Postgres
		// cap now that the CR lands in `<ws>` (ADR043 D8, closing w3/010).
		// Translate the API server's Forbidden into the same Render-shaped
		// message the service cap returns, or the caller sees a raw 403 about a
		// Kubernetes object they have no concept of.
		if mapped, ok := core.QuotaCapError(err, store.DatabasesQuotaCountKey, "Postgres database"); ok {
			return PostgresView{}, mapped
		}
		return PostgresView{}, err
	}
	s.RecordDatabaseEffect(ctx, d, core.DatabaseCreated)
	return s.view(d), nil
}

// DeletePostgres removes a managed Postgres (cascades the CNPG Cluster, PVC,
// Secret and any external route via owner refs).
func (s *Service) DeletePostgres(ctx context.Context, name string) error {
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	if err := s.requireUnprotected(ctx, d, "delete"); err != nil {
		return err
	}
	return s.Client.Delete(ctx, d)
}

// PostgresConnectionInfo assembles the internal + external connection strings
// from CNPG's generated "<id>-app" Secret (the only place the password is
// surfaced, to an authenticated caller).
func (s *Service) PostgresConnectionInfo(ctx context.Context, name string) (PostgresConnectionInfo, error) {
	d, sec, err := s.loadAppSecret(ctx, core.RelCanViewSensitive, name)
	if err != nil {
		return PostgresConnectionInfo{}, err
	}
	user := string(sec.Data["username"])
	pass := string(sec.Data["password"])
	dbn := string(sec.Data["dbname"])
	internal := string(sec.Data["uri"]) // CNPG's ready-made internal URI

	info := PostgresConnectionInfo{
		Password:                 pass,
		InternalConnectionString: internal,
		// Qualify with the Database's OWN namespace (ADR043 D8) — this command is
		// copy-pasted by humans from anywhere, so it must never depend on the
		// reader happening to sit in the same namespace.
		PSQLCommand: fmt.Sprintf("PGPASSWORD=%s psql -h %s-rw.%s.svc -U %s %s",
			pass, d.Name, d.Namespace, user, dbn),
	}
	if d.Status.ExternalHost != "" {
		// Standard sslmode=require works for all clients: the pg-sni-proxy
		// (w1/m29) handles the SSLRequest preamble before TLS, so
		// sslnegotiation=direct is no longer needed.
		info.ExternalConnectionString = fmt.Sprintf(
			"postgresql://%s:%s@%s:5432/%s?sslmode=require",
			user, pass, d.Status.ExternalHost, dbn)
		info.PSQLCommand = fmt.Sprintf("PGPASSWORD=%s psql 'host=%s port=5432 dbname=%s user=%s sslmode=require'",
			pass, d.Status.ExternalHost, dbn, user)
	}
	// Pooled strings: same credentials, routed through the PgBouncer pooler.
	// The hosts come straight from status (the operator's contract) — the backend
	// doesn't recompute CNPG's Service naming. Each is omitted until reconciled,
	// exactly like the external string is gated on status.ExternalHost.
	if d.Status.PoolerHost != "" {
		info.InternalConnectionPoolString = fmt.Sprintf(
			"postgresql://%s:%s@%s:5432/%s", user, pass, d.Status.PoolerHost, dbn)
	}
	if d.Status.PoolerExternalHost != "" {
		info.ExternalConnectionPoolString = fmt.Sprintf(
			"postgresql://%s:%s@%s:5432/%s?sslmode=require",
			user, pass, d.Status.PoolerExternalHost, dbn)
	}
	// Per-replica read-only connection strings (CNPG -ro service or external SNI).
	if len(d.Status.ReadReplicaStatuses) > 0 {
		rcs := make([]ReplicaConnectionStrings, 0, len(d.Status.ReadReplicaStatuses))
		for _, rs := range d.Status.ReadReplicaStatuses {
			rc := ReplicaConnectionStrings{Name: rs.Name}
			if rs.InternalHost != "" {
				rc.InternalConnectionString = fmt.Sprintf(
					"postgresql://%s:%s@%s:5432/%s", user, pass, rs.InternalHost, dbn)
			}
			if rs.ExternalHost != "" {
				rc.ExternalConnectionString = fmt.Sprintf(
					"postgresql://%s:%s@%s:5432/%s?sslmode=require",
					user, pass, rs.ExternalHost, dbn)
			}
			rcs = append(rcs, rc)
		}
		info.ReadReplicaConnectionStrings = rcs
	}
	return info, nil
}

// SetPlan changes the managed Postgres database's instance type (spec.plan).
// Unknown plans are rejected before any write (the caller maps core.ErrBadRequest
// to 400/a GraphQL error, listing the valid plans). A plan change resizes the
// CNPG Cluster's pod resources on the next operator reconcile — same cost as any
// rolling update.
func (s *Service) SetPlan(ctx context.Context, name, plan string) (PostgresView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if _, ok := tiers.Postgres.ByID(plan); !ok {
		return PostgresView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
	}
	if core.PaidPlan(plan) {
		if err := s.RequirePaymentMethod(ctx, d.Labels[core.LabelTenant]); err != nil {
			return PostgresView{}, err
		}
	}
	if err := s.RequireBillingMutation(ctx, d.Labels[core.LabelTenant]); err != nil {
		return PostgresView{}, err
	}
	from := d.Spec.Plan
	view, err := s.patchDatabaseObj(ctx, d, func(d *appv1alpha1.Database) {
		d.Spec.Plan = plan
	})
	if err != nil {
		return PostgresView{}, err
	}
	// Recorded even when from == plan, matching apps' SetPlan precedent: the
	// verb names the call the caller made and the equal pair shows nothing
	// changed — never the Update* verb of a call they didn't make (w10/m5).
	s.RecordDatabasePlanChanged(ctx, d, from, plan)
	return view, nil
}

// PreviewSetPlan returns what SetPlan would produce without writing — the same
// validation and in-memory spec update — zero side effects (w2/m29 dry-run).
// Requires can_view on the named database (no audit event, no write).
func (s *Service) PreviewSetPlan(ctx context.Context, name, plan string) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return PostgresView{}, err
	}
	if _, ok := tiers.Postgres.ByID(plan); !ok {
		return PostgresView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
	}
	preview := d.DeepCopy()
	preview.Spec.Plan = plan
	return s.view(preview), nil
}

// PostgresPatch is the mutable-field set for PATCH /v1/postgres/{id} — Render's
// "only the fields you pass are changed" semantics (nil = leave unchanged,
// mirroring the pointer fields on the CLI's generated PostgresPATCHInput).
type PostgresPatch struct {
	Name                   *string
	DatadogAPIKey          *string
	DatadogSite            *string
	Plan                   *string
	Version                *string
	DiskSizeGB             *int32
	EnableDiskAutoscaling  *bool
	EnableHighAvailability *bool
	IPAllowList            *[]core.IPAllowListEntry // nil = unchanged; non-nil empty slice clears it
	ParameterOverrides     *map[string]string       // nil = unchanged; non-nil empty map clears it
}

// validate checks every field present in the patch (plan enum, CIDR syntax)
// before any write; shared by UpdatePostgres and PreviewUpdatePostgres so the
// two paths can never accept different inputs.
func (patch PostgresPatch) validate() error {
	if patch.DatadogAPIKey != nil || patch.DatadogSite != nil {
		return unsupportedDatadogError()
	}
	if patch.Name != nil {
		if err := validateDatabaseName(*patch.Name); err != nil {
			return err
		}
	}
	if patch.Plan != nil {
		if _, ok := tiers.Postgres.ByID(*patch.Plan); !ok {
			return fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
		}
	}
	if patch.Version != nil && !postgresVersionKnown(*patch.Version) {
		return unknownPostgresVersionError(*patch.Version)
	}
	if patch.DiskSizeGB != nil && *patch.DiskSizeGB <= 0 {
		return fmt.Errorf("%w: diskSizeGB must be greater than zero", core.ErrBadRequest)
	}
	if patch.IPAllowList != nil {
		if err := core.ValidateAllowList(*patch.IPAllowList); err != nil {
			return err
		}
	}
	return nil
}

func databaseStorageHighWater(d *appv1alpha1.Database) int32 {
	plan, ok := tiers.Postgres.ByID(d.Spec.Plan)
	if !ok {
		plan = tiers.Postgres.Default()
	}
	return max(plan.StorageGB, d.Spec.StorageGB, d.Status.AllocatedStorageGB)
}

func validateDatabaseStorageResize(d *appv1alpha1.Database, requested *int32) error {
	if requested == nil {
		return nil
	}
	current := databaseStorageHighWater(d)
	if *requested < current {
		return fmt.Errorf("%w: Postgres storage is grow-only: requested %d GB is below the allocated %d GB", core.ErrBadRequest, *requested, current)
	}
	return nil
}

func (patch PostgresPatch) apply(d *appv1alpha1.Database) {
	if patch.Name != nil {
		d.Spec.Name = *patch.Name
	}
	if patch.Plan != nil {
		d.Spec.Plan = *patch.Plan
	}
	if patch.Version != nil {
		d.Spec.Version = *patch.Version
	}
	if patch.DiskSizeGB != nil {
		d.Spec.StorageGB = *patch.DiskSizeGB
	}
	if patch.EnableDiskAutoscaling != nil {
		d.Spec.DiskAutoscaling = *patch.EnableDiskAutoscaling
	}
	if patch.EnableHighAvailability != nil {
		d.Spec.HighAvailability = *patch.EnableHighAvailability
	}
	if patch.IPAllowList != nil {
		d.Spec.IPAllowList = core.AllowListToSpec(*patch.IPAllowList)
	}
	if patch.ParameterOverrides != nil {
		d.Spec.Parameters = normalizeParameterOverrides(*patch.ParameterOverrides)
	}
}

func normalizeParameterOverrides(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for name, value := range params {
		if name != "shared_preload_libraries" {
			out[name] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// UpdatePostgres applies a partial update (Render's PATCH /postgres/{id}
// semantics — only fields set in patch change; everything else is left alone).
// SetPlan/PreviewSetPlan above remain the plan-only entry points GraphQL's
// updatePostgresPlan mutation and the update_postgres_plan MCP tool use; this
// is the general handler REST's PATCH route needs (rename, disk, HA,
// ip-allow-list — not just plan).
func (s *Service) UpdatePostgres(ctx context.Context, name string, patch PostgresPatch) (PostgresView, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if err := patch.validate(); err != nil {
		return PostgresView{}, err
	}
	if patch.Plan != nil && core.PaidPlan(*patch.Plan) {
		if err := s.RequirePaymentMethod(ctx, d.Labels[core.LabelTenant]); err != nil {
			return PostgresView{}, err
		}
	}
	if patch.Plan != nil || patch.Version != nil || patch.DiskSizeGB != nil || patch.EnableDiskAutoscaling != nil || patch.EnableHighAvailability != nil {
		if err := s.RequireBillingMutation(ctx, d.Labels[core.LabelTenant]); err != nil {
			return PostgresView{}, err
		}
	}
	if err := validateDatabaseStorageResize(d, patch.DiskSizeGB); err != nil {
		return PostgresView{}, err
	}
	if patch.Name != nil {
		if err := s.ensureDatabaseNameAvailable(ctx, d.Labels[core.LabelTenant], *patch.Name, d.Name); err != nil {
			return PostgresView{}, err
		}
	}
	if patch.Version != nil {
		if err := s.validateVersionUpgrade(ctx, d, *patch.Version, patch.Plan); err != nil {
			return PostgresView{}, err
		}
	}
	fromPlan := d.Spec.Plan
	view, err := s.patchDatabaseObj(ctx, d, patch.apply)
	if err != nil {
		return PostgresView{}, err
	}
	if patch.Plan != nil && fromPlan != d.Spec.Plan {
		s.RecordDatabaseEffect(ctx, d, core.DatabasePlanChanged)
	} else {
		s.RecordDatabaseEffect(ctx, d, core.DatabaseUpdated)
	}
	return view, nil
}

// PreviewUpdatePostgres is UpdatePostgres's dry-run twin (w2/m29 pattern): same
// validation, zero side effects. Requires can_view (no audit event, no write).
func (s *Service) PreviewUpdatePostgres(ctx context.Context, name string, patch PostgresPatch) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return PostgresView{}, err
	}
	if err := patch.validate(); err != nil {
		return PostgresView{}, err
	}
	if err := validateDatabaseStorageResize(d, patch.DiskSizeGB); err != nil {
		return PostgresView{}, err
	}
	if patch.Name != nil {
		if err := s.ensureDatabaseNameAvailable(ctx, d.Labels[core.LabelTenant], *patch.Name, d.Name); err != nil {
			return PostgresView{}, err
		}
	}
	if patch.Version != nil {
		if err := s.validateVersionUpgrade(ctx, d, *patch.Version, patch.Plan); err != nil {
			return PostgresView{}, err
		}
	}
	preview := d.DeepCopy()
	patch.apply(preview)
	return s.view(preview), nil
}

func unknownPostgresVersionError(version string) error {
	return core.NewBadRequestError(
		"POSTGRES_VERSION_UNKNOWN",
		fmt.Sprintf("PostgreSQL version %q is not supported; choose one of %s", version, supportedPostgresVersionText()),
		map[string]any{"version": version, "supportedVersions": supportedPostgresVersions},
	)
}

func currentPostgresVersion(d *appv1alpha1.Database) string {
	if d.Status.CurrentVersion != "" {
		return d.Status.CurrentVersion
	}
	return d.Spec.Version
}

func planRequiresUpgradeBackup(plan string) bool {
	tier, ok := tiers.Postgres.ByID(plan)
	return ok && tier.Backup
}

func (s *Service) hasCompletedBackup(ctx context.Context, d *appv1alpha1.Database) bool {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cnpgBackupGVK)
	if err := s.Client.List(ctx, list, client.InNamespace(d.Namespace), client.MatchingLabels{labelCNPGCluster: d.Name}); err != nil {
		return false
	}
	serverName := d.Status.BackupServerName
	if serverName == "" {
		serverName = d.Name
	}
	for i := range list.Items {
		backup := &list.Items[i]
		phase, _, _ := unstructured.NestedString(backup.Object, "status", "phase")
		backupServerName, _, _ := unstructured.NestedString(backup.Object, "status", "serverName")
		// Older CNPG objects may omit serverName when it defaulted to the
		// cluster name. Only that legacy generation may use the empty value.
		if phase == "completed" && (backupServerName == serverName || (backupServerName == "" && serverName == d.Name)) {
			return true
		}
	}
	return false
}

// validateVersionUpgrade is the shared safety gate for the dedicated GraphQL/
// MCP verb and REST's partial PATCH. Major versions are upward-only. A plan
// whose durability contract includes backups must have a completed physical
// backup before the offline pg_upgrade begins; checking BackupsEnabled alone is
// insufficient because the first ScheduledBackup may still be pending.
func (s *Service) validateVersionUpgrade(ctx context.Context, d *appv1alpha1.Database, target string, patchedPlan *string) error {
	if !postgresVersionKnown(target) {
		return unknownPostgresVersionError(target)
	}
	if d.Status.Phase == appv1alpha1.DBPhaseUpgrading {
		return core.NewConflictError(
			"POSTGRES_UPGRADE_IN_PROGRESS",
			"a PostgreSQL major-version upgrade is already in progress",
			map[string]any{"targetVersion": d.Spec.Version},
		)
	}
	current := d.Status.CurrentVersion
	currentMajor, currentErr := strconv.Atoi(current)
	targetMajor, targetErr := strconv.Atoi(target)
	if currentErr != nil || current == "" {
		return core.NewConflictError(
			"POSTGRES_VERSION_NOT_OBSERVED",
			"the running PostgreSQL version has not been observed yet; wait for the database to become available",
			map[string]any{"targetVersion": target},
		)
	}
	if targetErr != nil || targetMajor <= currentMajor {
		return core.NewBadRequestError(
			"POSTGRES_VERSION_NOT_NEWER",
			fmt.Sprintf("PostgreSQL version upgrades are upward-only: target %s must be newer than current version %s", target, current),
			map[string]any{"currentVersion": current, "targetVersion": target},
		)
	}
	requiresBackup := planRequiresUpgradeBackup(d.Spec.Plan)
	if patchedPlan != nil {
		requiresBackup = requiresBackup || planRequiresUpgradeBackup(*patchedPlan)
	}
	if requiresBackup && (!d.Status.BackupsEnabled || !s.hasCompletedBackup(ctx, d)) {
		return core.NewConflictError(
			"POSTGRES_UPGRADE_BACKUP_REQUIRED",
			"a completed physical backup is required before upgrading this durable Postgres instance",
			map[string]any{"currentVersion": current, "targetVersion": target, "plan": d.Spec.Plan},
		)
	}
	return nil
}

// SetVersion requests an offline CNPG major-version upgrade. The operator
// observes the spec change, updates Cluster.spec.imageName, and reports the
// pg_upgrade lifecycle through Database.status.
func (s *Service) SetVersion(ctx context.Context, name, target string) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if err := s.validateVersionUpgrade(ctx, d, target, nil); err != nil {
		return PostgresView{}, err
	}
	return s.patchDatabaseObj(ctx, d, func(d *appv1alpha1.Database) {
		d.Spec.Version = target
	})
}

// patchDatabaseObj applies mutate to an already-fetched Database and writes it
// back as a merge patch — conflict-free against the operator's concurrent status
// writes (no full-object optimistic lock), the same discipline apps.patchFetched
// follows. Callers that already hold the object use this to avoid a re-fetch.
func (s *Service) patchDatabaseObj(ctx context.Context, d *appv1alpha1.Database, mutate func(d *appv1alpha1.Database)) (PostgresView, error) {
	patch := client.MergeFrom(d.DeepCopy())
	mutate(d)
	resourcemeta.Touch(d, s.Now())
	if err := s.Client.Patch(ctx, d, patch); err != nil {
		return PostgresView{}, err
	}
	return s.view(d), nil
}

// patchDatabase fetches the Database and merge-patches it — the spec-intent
// writer the operator converges (the App/KeyValue lifecycle discipline).
func (s *Service) patchDatabase(ctx context.Context, relation, name string, mutate func(d *appv1alpha1.Database)) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, relation, name)
	if err != nil {
		return PostgresView{}, err
	}
	return s.patchDatabaseObj(ctx, d, mutate)
}

// SetProjectID assigns (or, with an empty projectID, clears) this Database's
// project (w1/m31 extension) — the internal/projects feature's write path,
// mirroring keyvalue.Service.SetProjectID. Authorized the same as the other
// tenant-mutating verbs on a named Database (RelCanCreate, matching DeletePostgres).
func (s *Service) SetProjectID(ctx context.Context, name, projectID string) error {
	_, err := s.patchDatabase(ctx, core.RelCanCreate, name, func(d *appv1alpha1.Database) {
		if projectID == "" {
			delete(d.Labels, core.LabelProject)
			return
		}
		if d.Labels == nil {
			d.Labels = map[string]string{}
		}
		d.Labels[core.LabelProject] = projectID
	})
	return err
}

// SetEnvironmentIPAllowList projects (or, with nil, clears) the environment
// inbound-IP layer onto this Database (w4/m28) — the internal/environments
// fan-out's write path. The Database's OWN IPAllowList is never touched: the
// operator chains one middleware per layer, so a source must pass both.
func (s *Service) SetEnvironmentIPAllowList(ctx context.Context, name string, cidrs []string) error {
	d, err := s.AuthorizeDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	if slices.Equal(d.Spec.EnvironmentIPAllowList, cidrs) {
		return nil // unchanged layer: no Update, no resourceVersion churn
	}
	_, err = s.patchDatabaseObj(ctx, d, func(d *appv1alpha1.Database) {
		d.Spec.EnvironmentIPAllowList = cidrs
	})
	return err
}

// SetEnvironmentID assigns (or, with an empty environmentID, clears) this
// Database's environment (w6/m20 extension) — the internal/environments
// feature's write path, mirroring keyvalue.Service.SetEnvironmentID and
// SetProjectID above. Authorized the same as the other tenant-mutating verbs
// on a named Database (RelCanCreate, matching DeletePostgres).
func (s *Service) SetEnvironmentID(ctx context.Context, name, environmentID string) error {
	_, err := s.patchDatabase(ctx, core.RelCanCreate, name, func(d *appv1alpha1.Database) {
		if environmentID == "" {
			delete(d.Labels, core.LabelEnvironment)
			return
		}
		if d.Labels == nil {
			d.Labels = map[string]string{}
		}
		d.Labels[core.LabelEnvironment] = environmentID
	})
	return err
}
