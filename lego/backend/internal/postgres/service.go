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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service exposes managed Postgres as Render's "postgres" shape.
type Service struct {
	*core.Base
	// Selections is the shared MCP per-session workspace selection
	// (w6/m2/t005): list_postgres_instances falls back to the caller's
	// selected workspace when its ownerId argument is omitted. Read-only
	// (postgres never selects a workspace). Nil => no fallback.
	Selections core.WorkspaceSelectionReader
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

	HighAvailabilityEnabled bool   `json:"highAvailabilityEnabled"`
	Suspended               string `json:"suspended"` // string enum, like services
	CreatedAt               string `json:"createdAt,omitempty"`

	// bex-native extras (Render clients ignore unknown keys).
	ExternalHost string `json:"externalHost,omitempty"`
	Public       bool   `json:"public"`

	// IPAllowList is the CIDR allowlist gating the EXTERNAL endpoint (Render's
	// ipAllowList). Empty => the external route is open to all source IPs.
	IPAllowList []string `json:"ipAllowList,omitempty"`
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
}

// CreatePostgresRequest is the POST /v1/postgres body (bex subset of Render's).
type CreatePostgresRequest struct {
	Name       string `json:"name"`
	Plan       string `json:"plan,omitempty"`
	Version    string `json:"version,omitempty"`
	DiskSizeGB int32  `json:"diskSizeGB,omitempty"`
	Public     bool   `json:"public,omitempty"`
	// IPAllowList optionally seeds the external-endpoint CIDR allowlist at create.
	IPAllowList []string `json:"ipAllowList,omitempty"`
	// Pooler optionally provisions a PgBouncer pooler at create.
	Pooler bool `json:"pooler,omitempty"`
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
	return PostgresView{
		ID:                      d.Name,
		Name:                    d.Name,
		Plan:                    d.Spec.Plan,
		Version:                 d.Spec.Version,
		Status:                  dbStatus(d.Status.Phase),
		DatabaseName:            dbn,
		DatabaseUser:            dbn + "_user",
		DiskSizeGB:              d.Spec.StorageGB,
		HighAvailabilityEnabled: false, // single-instance MVP (HA => w1/013)
		Suspended:               core.SuspendedEnum(d.Spec.Suspended),
		CreatedAt:               created,
		ExternalHost:            d.Status.ExternalHost,
		Public:                  d.Spec.Public,
		IPAllowList:             d.Spec.IPAllowList,
		PoolerEnabled:           d.Spec.Pooler,
		BackupsEnabled:          d.Status.BackupsEnabled,
		OwnerID:                 d.Labels[core.LabelTenant],
	}
}

func (s *Service) fetchDatabase(ctx context.Context, name string) (*appv1alpha1.Database, error) {
	var d appv1alpha1.Database
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, &d)
	if apierrors.IsNotFound(err) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// loadAppSecret resolves a Database and its CNPG-generated "<name>-app" Secret
// (username/password/dbname/uri) — the credential path both connection-info and
// the read-only query verb share. Returns core.ErrNotFound when the Database or
// its Secret isn't provisioned yet.
func (s *Service) loadAppSecret(ctx context.Context, name string) (*appv1alpha1.Database, *corev1.Secret, error) {
	d, err := s.fetchDatabase(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	secretName := d.Status.SecretName
	if secretName == "" {
		secretName = name + "-app"
	}
	var sec corev1.Secret
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: secretName}, &sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil, core.ErrNotFound // not provisioned yet
		}
		return nil, nil, err
	}
	return d, &sec, nil
}

// ListPostgres returns every managed Postgres in the namespace, optionally
// narrowed to a single owning workspace — Render's `ownerId` list-filter
// contract (w6/m2/t004, labeling fixed by w6/m4/t001), mirroring
// apps.Service.List. ownerID == "" lists unscoped. A non-empty ownerID
// authorizes can_view on that exact workspace (an inaccessible ownerId is
// ErrForbidden) and then filters by core.LabelTenant; never silently returns
// unscoped data for a scoped request.
func (s *Service) ListPostgres(ctx context.Context, ownerID string) ([]PostgresView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	opts := []client.ListOption{client.InNamespace(s.Namespace)}
	if ownerID != "" {
		if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(ownerID)); err != nil {
			return nil, err
		}
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
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return PostgresView{}, err
	}
	d, err := s.fetchDatabase(ctx, name)
	if err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
}

// CreatePostgres provisions a managed Postgres (a Database CR the operator
// projects to a CNPG Cluster).
func (s *Service) CreatePostgres(ctx context.Context, req CreatePostgresRequest) (PostgresView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return PostgresView{}, err
	}
	if req.Name == "" {
		return PostgresView{}, fmt.Errorf("name is required")
	}
	d := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: s.Namespace},
		Spec: appv1alpha1.DatabaseSpec{
			Plan:        req.Plan,
			Version:     req.Version,
			StorageGB:   req.DiskSizeGB,
			Public:      req.Public,
			IPAllowList: req.IPAllowList,
			Pooler:      req.Pooler,
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
	if err := s.Client.Create(ctx, d); err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
}

// DeletePostgres removes a managed Postgres (cascades the CNPG Cluster, PVC,
// Secret and any external route via owner refs).
func (s *Service) DeletePostgres(ctx context.Context, name string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	d, err := s.fetchDatabase(ctx, name)
	if err != nil {
		return err
	}
	return s.Client.Delete(ctx, d)
}

// PostgresConnectionInfo assembles the internal + external connection strings
// from CNPG's generated "<name>-app" Secret (the only place the password is
// surfaced, to an authenticated caller).
func (s *Service) PostgresConnectionInfo(ctx context.Context, name string) (PostgresConnectionInfo, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return PostgresConnectionInfo{}, err
	}
	d, sec, err := s.loadAppSecret(ctx, name)
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
		// Render-shaped external string. sslnegotiation=direct is required today
		// (Postgres SSLRequest preamble blocks Traefik SNI for older clients).
		info.ExternalConnectionString = fmt.Sprintf(
			"postgresql://%s:%s@%s:5432/%s?sslmode=require&sslnegotiation=direct",
			user, pass, d.Status.ExternalHost, dbn)
		info.PSQLCommand = fmt.Sprintf("PGPASSWORD=%s psql 'host=%s port=5432 dbname=%s user=%s sslmode=require sslnegotiation=direct'",
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
			"postgresql://%s:%s@%s:5432/%s?sslmode=require&sslnegotiation=direct",
			user, pass, d.Status.PoolerExternalHost, dbn)
	}
	return info, nil
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
func (s *Service) patchDatabase(ctx context.Context, name string, mutate func(d *appv1alpha1.Database)) (PostgresView, error) {
	d, err := s.fetchDatabase(ctx, name)
	if err != nil {
		return PostgresView{}, err
	}
	return s.patchDatabaseObj(ctx, d, mutate)
}
