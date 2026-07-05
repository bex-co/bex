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

package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/operator/api/v1alpha1"
)

// postgres.go extends Core with managed-Postgres methods over the Database CR,
// mirroring Render's /v1/postgres API. Same one-Core design: REST and GraphQL
// call these; the connection-info method is the one place the DB password is
// surfaced (to an authenticated caller only), read from CNPG's generated Secret
// at request time — never from the CR status.

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
}

// PostgresConnectionInfo mirrors Render's postgresConnectionInfo schema.
type PostgresConnectionInfo struct {
	Password                 string `json:"password"`
	InternalConnectionString string `json:"internalConnectionString"`
	ExternalConnectionString string `json:"externalConnectionString,omitempty"`
	// Pooler variants are empty until a PgBouncer Pooler is wired (deferred).
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
}

// pgIdent mirrors the operator's normalizeIdent: a valid unquoted PostgreSQL
// identifier (lowercase, hyphens -> underscores). Kept in sync with
// internal/controller so the API reports the same db/role names CNPG created.
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
		HighAvailabilityEnabled: false, // single-instance MVP
		Suspended:               renderNotSuspended,
		CreatedAt:               created,
		ExternalHost:            d.Status.ExternalHost,
		Public:                  d.Spec.Public,
	}
}

func (c *Core) fetchDatabase(ctx context.Context, name string) (*appv1alpha1.Database, error) {
	var d appv1alpha1.Database
	err := c.Client.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: name}, &d)
	if apierrors.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListPostgres returns every managed Postgres in the namespace.
func (c *Core) ListPostgres(ctx context.Context) ([]PostgresView, error) {
	var list appv1alpha1.DatabaseList
	if err := c.Client.List(ctx, &list, client.InNamespace(c.Namespace)); err != nil {
		return nil, err
	}
	out := make([]PostgresView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, pgView(&list.Items[i]))
	}
	return out, nil
}

// GetPostgres returns one managed Postgres, or ErrNotFound.
func (c *Core) GetPostgres(ctx context.Context, name string) (PostgresView, error) {
	d, err := c.fetchDatabase(ctx, name)
	if err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
}

// CreatePostgres provisions a managed Postgres (a Database CR the operator
// projects to a CNPG Cluster).
func (c *Core) CreatePostgres(ctx context.Context, req CreatePostgresRequest) (PostgresView, error) {
	if req.Name == "" {
		return PostgresView{}, fmt.Errorf("name is required")
	}
	d := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: c.Namespace},
		Spec: appv1alpha1.DatabaseSpec{
			Plan:      req.Plan,
			Version:   req.Version,
			StorageGB: req.DiskSizeGB,
			Public:    req.Public,
		},
	}
	if err := c.Client.Create(ctx, d); err != nil {
		return PostgresView{}, err
	}
	return pgView(d), nil
}

// DeletePostgres removes a managed Postgres (cascades the CNPG Cluster, PVC,
// Secret and any external route via owner refs).
func (c *Core) DeletePostgres(ctx context.Context, name string) error {
	d, err := c.fetchDatabase(ctx, name)
	if err != nil {
		return err
	}
	return c.Client.Delete(ctx, d)
}

// PostgresConnectionInfo assembles the internal + external connection strings
// from CNPG's generated "<name>-app" Secret (the only place the password is
// surfaced, to an authenticated caller).
func (c *Core) PostgresConnectionInfo(ctx context.Context, name string) (PostgresConnectionInfo, error) {
	d, err := c.fetchDatabase(ctx, name)
	if err != nil {
		return PostgresConnectionInfo{}, err
	}
	secretName := d.Status.SecretName
	if secretName == "" {
		secretName = name + "-app"
	}
	var sec corev1.Secret
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: secretName}, &sec); err != nil {
		if apierrors.IsNotFound(err) {
			return PostgresConnectionInfo{}, ErrNotFound // not provisioned yet
		}
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
			pass, name, c.Namespace, user, dbn),
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
	return info, nil
}
