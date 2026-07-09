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

package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/xid"
)

// ID prefixes, Render-style: a typed opaque id is "<prefix>-<xid>", e.g.
// "srv-c185th5c2rvvnhbfiltg" — the type is greppable, the xid suffix is
// k-sortable and non-guessable, and a rename never breaks a reference
// because ids, not names, are the keys (docs/postgresql-management.md §4).
// Prefixes follow Render's public API: tea- teams (bex tenants), srv-
// services (bex apps), cdm- custom domains.
const (
	TenantIDPrefix = "tea"
	AppIDPrefix    = "srv"
	DomainIDPrefix = "cdm"
)

// MaxReplicas is the shared upper bound on an App's replica count, enforced by
// both the create path (store/api.go) and the apps scale verb so the two can't
// disagree about what a valid App is. The lower bounds legitimately differ
// (create treats 0 as "default 1"; scale rejects 0 — see apps.Service.Scale).
const MaxReplicas = 100

// newID mints a typed opaque id: "<prefix>-<20-char xid>".
func newID(prefix string) string { return prefix + "-" + xid.New().String() }

// Error taxonomy shared by the store and the API: the store classifies
// Postgres failures into these, the API maps them to status codes
// (ErrInvalid→400, ErrNotFound→404, ErrConflict→409).
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
	ErrInvalid  = errors.New("invalid")
)

// Tenant is a row of `tenants` — who owns apps; plan names the tier ladder row.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"createdAt"`
}

// App is a row of `apps` — the source-of-truth service definition the
// reconciler projects into an App CR. Observed state (phase, url) is NOT stored
// here: it lives on the App CR's status, which bex-api reads at query time.
type App struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenantId"`
	Name           string    `json:"name"`
	Repo           string    `json:"repo,omitempty"`
	Image          string    `json:"image,omitempty"`
	Branch         string    `json:"branch"`
	Port           int32     `json:"port"`
	Replicas       int32     `json:"replicas"`
	Tier           string    `json:"tier"`
	IdleTTLSeconds int32     `json:"idleTTLSeconds"`
	Suspended      bool      `json:"suspended"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Domain is a row of `domains` — a BYOD custom domain attached to an app.
// The primary domain becomes the App CR's spec.host, the rest spec.hosts.
// Verification + cert status are the operator's/cert-manager's concern (read
// from the cluster), not stored here.
type Domain struct {
	ID        string    `json:"id"`
	AppID     string    `json:"appId"`
	Host      string    `json:"host"`
	Primary   bool      `json:"primary"`
	CreatedAt time.Time `json:"createdAt"`
}

// DesiredApp is an apps row joined with everything projection needs: the
// owning tenant's name (part of the CR name) and the app's domains,
// primary first.
type DesiredApp struct {
	App
	TenantName string
	Hosts      []string
}

// Store is the persistence boundary. The API writes through it, the reconciler
// reads desired state and projects it into App CRs. Observed state (phase/url)
// is not persisted — it stays on the CR. The production implementation is
// PGStore; tests use an in-memory fake.
type Store interface {
	CreateTenant(ctx context.Context, name, plan string) (Tenant, error)
	ListTenants(ctx context.Context) ([]Tenant, error)
	CreateApp(ctx context.Context, a App) (App, error)
	GetApp(ctx context.Context, id string) (App, error)
	ListApps(ctx context.Context) ([]App, error)
	DeleteApp(ctx context.Context, id string) error
	CreateDomain(ctx context.Context, appID, host string, primary bool) (Domain, error)
	// DeleteDomain removes a custom domain row. Not-found is ErrNotFound.
	DeleteDomain(ctx context.Context, appID, host string) error
	ListDesiredApps(ctx context.Context) ([]DesiredApp, error)
	// SetAppSuspended flips the row's suspended flag — the single write path
	// for suspend/resume on store-managed Apps. bex-api's lifecycle verbs call
	// this (row first, then the CR fast-path) so the projection loop never
	// reverts a suspend it didn't know about.
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
	// SetAppTier updates the row's tier — the single write path for plan
	// changes on store-managed Apps, same row-first rationale as
	// SetAppSuspended.
	SetAppTier(ctx context.Context, id string, tier string) error
	// SetAppReplicas updates the row's replica count — the single write path
	// for the manual-scale verb on store-managed Apps, same row-first
	// rationale as SetAppSuspended (the projector owns spec.replicas).
	SetAppReplicas(ctx context.Context, id string, replicas int32) error
}

// PGStore is the Postgres-backed Store over a pgx pool. It holds no business
// logic — validation happens in the API layer, classification of Postgres
// errors happens here.
type PGStore struct {
	Pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{Pool: pool} }

// Ping reports whether the database is reachable — the /healthz check.
func (s *PGStore) Ping(ctx context.Context) error { return s.Pool.Ping(ctx) }

func (s *PGStore) CreateTenant(ctx context.Context, name, plan string) (Tenant, error) {
	t := Tenant{ID: newID(TenantIDPrefix), Name: name, Plan: plan}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO tenants (id, name, plan) VALUES ($1, $2, $3) RETURNING created_at`,
		t.ID, name, plan,
	).Scan(&t.CreatedAt)
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	return t, nil
}

func (s *PGStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, plan, created_at FROM tenants ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PGStore) CreateApp(ctx context.Context, a App) (App, error) {
	a.ID = newID(AppIDPrefix)
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO apps (id, tenant_id, name, repo, image, branch, port, replicas, tier, idle_ttl_seconds, suspended)
		 VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, $9, $10, $11)
		 RETURNING created_at`,
		a.ID, a.TenantID, a.Name, a.Repo, a.Image, a.Branch, a.Port, a.Replicas, a.Tier, a.IdleTTLSeconds, a.Suspended,
	).Scan(&a.CreatedAt)
	if err != nil {
		return App{}, classify("app", err)
	}
	return a, nil
}

const appColumns = `a.id, a.tenant_id, a.name, COALESCE(a.repo,''), COALESCE(a.image,''),
	a.branch, a.port, a.replicas, a.tier, a.idle_ttl_seconds, a.suspended, a.created_at`

func scanApp(row pgx.Row) (App, error) {
	var a App
	err := row.Scan(&a.ID, &a.TenantID, &a.Name, &a.Repo, &a.Image,
		&a.Branch, &a.Port, &a.Replicas, &a.Tier, &a.IdleTTLSeconds, &a.Suspended, &a.CreatedAt)
	return a, err
}

func (s *PGStore) GetApp(ctx context.Context, id string) (App, error) {
	a, err := scanApp(s.Pool.QueryRow(ctx,
		`SELECT `+appColumns+` FROM apps a WHERE a.id = $1`, id))
	if err != nil {
		return App{}, classify("app", err)
	}
	return a, nil
}

func (s *PGStore) ListApps(ctx context.Context) ([]App, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+appColumns+` FROM apps a ORDER BY a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PGStore) DeleteApp(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM apps WHERE id = $1`, id)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app %s: %w", id, ErrNotFound)
	}
	return nil
}

func (s *PGStore) CreateDomain(ctx context.Context, appID, host string, primary bool) (Domain, error) {
	d := Domain{ID: newID(DomainIDPrefix), AppID: appID, Host: host, Primary: primary}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO domains (id, app_id, host, is_primary) VALUES ($1, $2, $3, $4)
		 RETURNING created_at`,
		d.ID, appID, host, primary,
	).Scan(&d.CreatedAt)
	if err != nil {
		return Domain{}, classify("domain", err)
	}
	return d, nil
}

func (s *PGStore) DeleteDomain(ctx context.Context, appID, host string) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM domains WHERE app_id = $1 AND host = $2`, appID, host)
	if err != nil {
		return classify("domain", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("domain: %w", ErrNotFound)
	}
	return nil
}

// AddDomain appends a non-primary domain row for apps.IntentStore — idempotent
// (conflict means it's already registered; silently ignored).
func (s *PGStore) AddDomain(ctx context.Context, appID, host string) error {
	_, err := s.CreateDomain(ctx, appID, host, false)
	if err != nil && errors.Is(err, ErrConflict) {
		return nil
	}
	return err
}

// RemoveDomain deletes a domain row for apps.IntentStore — idempotent
// (not-found silently ignored).
func (s *PGStore) RemoveDomain(ctx context.Context, appID, host string) error {
	err := s.DeleteDomain(ctx, appID, host)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (s *PGStore) ListDesiredApps(ctx context.Context) ([]DesiredApp, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+appColumns+`, t.name FROM apps a JOIN tenants t ON t.id = a.tenant_id ORDER BY a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DesiredApp
	index := map[string]int{}
	for rows.Next() {
		var d DesiredApp
		err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Repo, &d.Image,
			&d.Branch, &d.Port, &d.Replicas, &d.Tier, &d.IdleTTLSeconds, &d.Suspended,
			&d.CreatedAt, &d.TenantName)
		if err != nil {
			return nil, err
		}
		index[d.ID] = len(out)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Attach domains, primary first (the ORDER BY makes the primary land at
	// Hosts[0] — projection's spec.host).
	drows, err := s.Pool.Query(ctx,
		`SELECT app_id, host FROM domains ORDER BY is_primary DESC, created_at`)
	if err != nil {
		return nil, err
	}
	defer drows.Close()
	for drows.Next() {
		var appID, host string
		if err := drows.Scan(&appID, &host); err != nil {
			return nil, err
		}
		if i, ok := index[appID]; ok {
			out[i].Hosts = append(out[i].Hosts, host)
		}
	}
	return out, drows.Err()
}

func (s *PGStore) SetAppSuspended(ctx context.Context, id string, suspended bool) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET suspended = $2, updated_at = now() WHERE id = $1`,
		id, suspended)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppTier updates the row's tier (the apps feature's plan-change verb
// validates it against lego/types/tiers before calling this). The projector
// carries it onto spec.tier the same way it carries suspended.
func (s *PGStore) SetAppTier(ctx context.Context, id string, tier string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET tier = $2, updated_at = now() WHERE id = $1`,
		id, tier)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// SetAppReplicas updates the row's replica count (the apps feature's scale
// verb validates the bound before calling this). The projector carries it
// onto spec.replicas the same way it carries suspended/tier.
func (s *PGStore) SetAppReplicas(ctx context.Context, id string, replicas int32) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET replicas = $2, updated_at = now() WHERE id = $1`,
		id, replicas)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

// classify maps Postgres errors to the shared taxonomy: unique violations are
// conflicts, FK violations mean the referenced parent doesn't exist, and
// check violations are caller errors. (Ids are plain text, so a malformed id
// simply matches no rows — ErrNoRows covers it.)
func classify(entity string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", entity, ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s: %w", entity, ErrConflict)
		case "23503": // foreign_key_violation — referenced tenant/app is gone
			return fmt.Errorf("%s reference: %w", entity, ErrNotFound)
		case "23514": // check_violation
			return fmt.Errorf("%s: %w: %s", entity, ErrInvalid, pgErr.ConstraintName)
		}
	}
	return err
}
