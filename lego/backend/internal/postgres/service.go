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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service exposes managed Postgres as Render's "postgres" shape.
type Service struct {
	*core.Base
	// ExportSigner mints short-lived object-store download URLs after the
	// ListExports verb has passed can_view_sensitive. Production wires the
	// Kubernetes Secret-backed S3 signer; tests can replace it.
	ExportSigner ExportURLSigner
	// Selections is the shared MCP per-session workspace selection
	// (w6/m2/t005): list_postgres_instances falls back to the caller's
	// selected workspace when its ownerId argument is omitted. Read-only
	// (postgres never selects a workspace). Nil => no fallback.
	Selections core.WorkspaceSelectionReader
	// MaxPostgres, when positive, caps how many Postgres instances a workspace
	// may own. 0 = unlimited (the default; byte-identical to before). Only
	// enforced when the caller's tenant is resolvable (w7/m9).
	MaxPostgres int
	// queryExecutor is the SQL transport seam used by Query and ExecuteQuery.
	// Production leaves it nil and uses pgx; tests replace it so the REST and
	// GraphQL adapters can exercise authz + secret resolution without a live DB.
	queryExecutor queryExecutor
}

// PostgresView is the Render-shaped "postgres" object.
type PostgresView struct {
	ID           string `json:"id"` // Render ids are opaque; bex uses the Database name
	Name         string `json:"name"`
	Plan         string `json:"plan"`
	Version      string `json:"version,omitempty"`
	Status       string `json:"status"`       // Render databaseStatus enum
	DatabaseName string `json:"databaseName"` // the actual (normalized) db
	DatabaseUser string `json:"databaseUser"`
	DiskSizeGB   int32  `json:"diskSizeGB,omitempty"`

	// HighAvailabilityEnabled reflects the operator's observed state (≥2 ready
	// instances). Render's highAvailabilityEnabled read field.
	HighAvailabilityEnabled bool `json:"highAvailabilityEnabled"`
	// ReadReplicas is the named replica array — each with its host info.
	// Password is not included here; use PostgresConnectionInfo for credentials.
	// Render's readReplicas: [{name, connectionInfo}].
	ReadReplicas []ReadReplicaView `json:"readReplicas,omitempty"`

	Suspended string `json:"suspended"` // string enum, like services
	CreatedAt string `json:"createdAt,omitempty"`

	// bex-native extras (Render clients ignore unknown keys).
	ExternalHost string `json:"externalHost,omitempty"`
	Public       bool   `json:"public"`

	// IPAllowList is the CIDR allowlist gating the EXTERNAL endpoint (Render's
	// ipAllowList). Empty => the external route is open to all source IPs.
	IPAllowList []IPAllowListEntry `json:"ipAllowList,omitempty"`
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

// IPAllowListEntry is Render's ipAllowList wire shape (components.schemas
// cidrBlockAndDescription: {cidrBlock, description}), verified against the
// render-oss/cli generated client (`client.CidrBlockAndDescription`) — a bare
// CIDR-string array breaks the CLI's decode of both `postgres create
// --ip-allow-list` and `postgres get`/`list`. bex doesn't persist a
// per-entry description (the Database CR's spec.ipAllowList is just
// []string); an incoming Description is accepted but dropped, and read back
// entries always carry an empty one — a deliberate, documented subset.
type IPAllowListEntry struct {
	CIDRBlock   string `json:"cidrBlock"`
	Description string `json:"description,omitempty"`
}

// ipAllowListToWire/ipAllowListFromWire convert between bex's internal CIDR-
// string storage and Render's {cidrBlock, description} wire entries.
func ipAllowListToWire(cidrs []string) []IPAllowListEntry {
	if len(cidrs) == 0 {
		return nil
	}
	out := make([]IPAllowListEntry, len(cidrs))
	for i, c := range cidrs {
		out[i] = IPAllowListEntry{CIDRBlock: c}
	}
	return out
}

func ipAllowListFromWire(entries []IPAllowListEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.CIDRBlock
	}
	return out
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
	OwnerID    string `json:"ownerId,omitempty"`
	Name       string `json:"name"`
	Plan       string `json:"plan,omitempty"`
	Version    string `json:"version,omitempty"`
	DiskSizeGB int32  `json:"diskSizeGB,omitempty"`
	Public     bool   `json:"public,omitempty"`
	// IPAllowList optionally seeds the external-endpoint CIDR allowlist at create.
	// Render's wire shape ({cidrBlock, description} entries) — see IPAllowListEntry.
	IPAllowList []IPAllowListEntry `json:"ipAllowList,omitempty"`
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

// pgIdent mirrors the operator's normalizeIdent: a valid unquoted PostgreSQL
// identifier (lowercase, hyphens -> underscores).
func pgIdent(name string) string { return strings.ToLower(strings.ReplaceAll(name, "-", "_")) }

// dbStatus maps bex's Database phase onto Render's databaseStatus enum.
func dbStatus(p appv1alpha1.DatabasePhase) string {
	switch p {
	case appv1alpha1.DBPhaseReady:
		return "available"
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
	dbn := pgIdent(d.Name)
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
	return PostgresView{
		ID:                      d.Name,
		Name:                    d.Name,
		Plan:                    d.Spec.Plan,
		Version:                 d.Spec.Version,
		Status:                  dbStatus(d.Status.Phase),
		DatabaseName:            dbn,
		DatabaseUser:            dbn + "_user",
		DiskSizeGB:              d.Spec.StorageGB,
		HighAvailabilityEnabled: d.Status.HighAvailabilityEnabled,
		ReadReplicas:            replicas,
		Suspended:               core.SuspendedEnum(d.Spec.Suspended),
		CreatedAt:               created,
		ExternalHost:            d.Status.ExternalHost,
		Public:                  d.Spec.Public,
		IPAllowList:             ipAllowListToWire(d.Spec.IPAllowList),
		PoolerEnabled:           d.Spec.Pooler,
		BackupsEnabled:          d.Status.BackupsEnabled,
		OwnerID:                 d.Labels[core.LabelTenant],
		ProjectID:               d.Labels[core.LabelProject],
		EnvironmentID:           d.Labels[core.LabelEnvironment],
	}
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

// loadAppSecret resolves a Database and its CNPG-generated "<name>-app" Secret
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
// apps.Service.List. ownerID == "" lists unscoped. A non-empty ownerID names
// the workspace to list (core.WithWorkspace), authorized+membership-checked by
// the same resolveWorkspace mechanism every other verb uses (w6/m17 —
// previously an OpenFGA-only check with no IsMember) and then filters by
// core.LabelTenant; never silently returns unscoped data for a scoped request.
func (s *Service) ListPostgres(ctx context.Context, ownerID string) ([]PostgresView, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	opts := []client.ListOption{client.InNamespace(s.Namespace)}
	if ownerID != "" {
		// Push the scoping into the list call itself (server-side label selector)
		// rather than fetching the whole namespace and filtering in memory.
		opts = append(opts, client.MatchingLabels{core.LabelTenant: ownerID})
	}
	var list appv1alpha1.DatabaseList
	if err := s.Client.List(ctx, &list, opts...); err != nil {
		return nil, err
	}
	out := make([]PostgresView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, pgView(&list.Items[i]))
	}
	return out, nil
}

// GetPostgres returns one managed Postgres, or core.ErrNotFound.
func (s *Service) GetPostgres(ctx context.Context, name string) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, name)
	if err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
}

// CreatePostgres provisions a managed Postgres (a Database CR the operator
// projects to a CNPG Cluster).
func (s *Service) CreatePostgres(ctx context.Context, req CreatePostgresRequest) (PostgresView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return PostgresView{}, err
	}
	if req.Name == "" {
		return PostgresView{}, fmt.Errorf("name is required")
	}
	cidrs := ipAllowListFromWire(req.IPAllowList)
	if err := core.ValidateCIDRs(cidrs); err != nil {
		return PostgresView{}, err
	}
	// Per-workspace Postgres cap (w7/m9).
	if s.MaxPostgres > 0 {
		if tenantID, ok := s.Tenant(ctx); ok {
			var existing appv1alpha1.DatabaseList
			if listErr := s.ListByTenant(ctx, &existing, tenantID); listErr != nil {
				return PostgresView{}, fmt.Errorf("checking postgres cap: %w", listErr)
			}
			if len(existing.Items) >= s.MaxPostgres {
				return PostgresView{}, fmt.Errorf("%w: workspace is limited to %d Postgres instances; delete an existing one to create another", core.ErrBadRequest, s.MaxPostgres)
			}
		}
	}
	crReplicas := make([]appv1alpha1.DatabaseReadReplica, 0, len(req.ReadReplicas))
	for _, r := range req.ReadReplicas {
		crReplicas = append(crReplicas, appv1alpha1.DatabaseReadReplica{Name: r.Name})
	}
	d := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: s.Namespace},
		Spec: appv1alpha1.DatabaseSpec{
			Plan:             req.Plan,
			Version:          req.Version,
			StorageGB:        req.DiskSizeGB,
			Public:           req.Public,
			IPAllowList:      cidrs,
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
	if tenantID, ok := s.Tenant(ctx); ok {
		d.Labels = map[string]string{core.LabelTenant: tenantID, core.LabelWorkspace: tenantID}
	}
	// Dry-run: return the resolved spec preview without any k8s write (w2/m29).
	if req.DryRun {
		return pgView(d), nil
	}
	if err := s.Client.Create(ctx, d); err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
}

// DeletePostgres removes a managed Postgres (cascades the CNPG Cluster, PVC,
// Secret and any external route via owner refs).
func (s *Service) DeletePostgres(ctx context.Context, name string) error {
	d, err := s.fetchDatabase(ctx, core.RelCanCreate, name)
	if err != nil {
		return err
	}
	return s.Client.Delete(ctx, d)
}

// PostgresConnectionInfo assembles the internal + external connection strings
// from CNPG's generated "<name>-app" Secret (the only place the password is
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
		PSQLCommand: fmt.Sprintf("PGPASSWORD=%s psql -h %s-rw.%s.svc -U %s %s",
			pass, name, s.Namespace, user, dbn),
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
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if _, ok := tiers.Postgres.ByID(plan); !ok {
		return PostgresView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
	}
	return s.patchDatabaseObj(ctx, d, func(d *appv1alpha1.Database) {
		d.Spec.Plan = plan
	})
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
	return pgView(preview), nil
}

// PostgresPatch is the mutable-field set for PATCH /v1/postgres/{id} — Render's
// "only the fields you pass are changed" semantics (nil = leave unchanged,
// mirroring the pointer fields on the CLI's generated PostgresPATCHInput).
type PostgresPatch struct {
	Plan                   *string
	DiskSizeGB             *int32
	EnableHighAvailability *bool
	IPAllowList            *[]string // nil = unchanged; non-nil empty slice clears it
}

// validate checks every field present in the patch (plan enum, CIDR syntax)
// before any write; shared by UpdatePostgres and PreviewUpdatePostgres so the
// two paths can never accept different inputs.
func (patch PostgresPatch) validate() error {
	if patch.Plan != nil {
		if _, ok := tiers.Postgres.ByID(*patch.Plan); !ok {
			return fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Postgres.IDs(), "|"))
		}
	}
	if patch.IPAllowList != nil {
		if err := core.ValidateCIDRs(*patch.IPAllowList); err != nil {
			return err
		}
	}
	return nil
}

func (patch PostgresPatch) apply(d *appv1alpha1.Database) {
	if patch.Plan != nil {
		d.Spec.Plan = *patch.Plan
	}
	if patch.DiskSizeGB != nil {
		d.Spec.StorageGB = *patch.DiskSizeGB
	}
	if patch.EnableHighAvailability != nil {
		d.Spec.HighAvailability = *patch.EnableHighAvailability
	}
	if patch.IPAllowList != nil {
		if len(*patch.IPAllowList) == 0 {
			d.Spec.IPAllowList = nil
		} else {
			d.Spec.IPAllowList = *patch.IPAllowList
		}
	}
}

// UpdatePostgres applies a partial update (Render's PATCH /postgres/{id}
// semantics — only fields set in patch change; everything else is left alone).
// SetPlan/PreviewSetPlan above remain the plan-only entry points GraphQL's
// updatePostgresPlan mutation and the update_postgres_plan MCP tool use; this
// is the general handler REST's PATCH route needs (rename, disk, HA,
// ip-allow-list — not just plan).
func (s *Service) UpdatePostgres(ctx context.Context, name string, patch PostgresPatch) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanOperate, name)
	if err != nil {
		return PostgresView{}, err
	}
	if err := patch.validate(); err != nil {
		return PostgresView{}, err
	}
	return s.patchDatabaseObj(ctx, d, patch.apply)
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
	preview := d.DeepCopy()
	patch.apply(preview)
	return pgView(preview), nil
}

// patchDatabaseObj applies mutate to an already-fetched Database and writes it
// back as a merge patch — conflict-free against the operator's concurrent status
// writes (no full-object optimistic lock), the same discipline apps.patchFetched
// follows. Callers that already hold the object use this to avoid a re-fetch.
func (s *Service) patchDatabaseObj(ctx context.Context, d *appv1alpha1.Database, mutate func(d *appv1alpha1.Database)) (PostgresView, error) {
	patch := client.MergeFrom(d.DeepCopy())
	mutate(d)
	if err := s.Client.Patch(ctx, d, patch); err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
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
