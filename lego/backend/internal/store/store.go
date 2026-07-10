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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Typed resource ids ("tea-…"/"srv-…"/"cdm-…") are minted through the one id
// package (docs/identifiers.md) — never hand-concatenated here, so the format
// and its DNS-safety stay guarded in one place. A rename never breaks a
// reference because ids, not names, are the keys (docs/postgresql-management.md §4).

// MaxReplicas is the shared upper bound on an App's replica count, enforced by
// both the create path (store/api.go) and the apps scale verb so the two can't
// disagree about what a valid App is. The lower bounds legitimately differ
// (create treats 0 as "default 1"; scale rejects 0 — see apps.Service.Scale).
const MaxReplicas = 100

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

// Deploy status vocabulary — Render's deploy status enum, the subset bex can
// honor today (docs/deployment.md health gating). build_in_progress/
// build_failed are reserved for w1/m5 (build-from-git); deactivated/canceled
// are deferred (w2/m5's README).
const (
	DeployUpdateInProgress = "update_in_progress"
	DeployLive             = "live"
	DeployUpdateFailed     = "update_failed"
)

// Deploy is a row of `deploys` — one rollout attempt of an app, Render's
// deploy history (list_deploys/get_deploy). Trigger is "create" (the app's
// first deploy, opened by CreateApp) or "api" (an explicit POST .../deploys).
// Commit is omitted here — it stays empty until w1/m5 tracks build-from-git
// commits; callers project it out rather than surface an always-empty field.
type Deploy struct {
	ID         string     `json:"id"`
	AppID      string     `json:"appId"`
	Trigger    string     `json:"trigger"`
	Image      string     `json:"image,omitempty"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
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
	// GetTenant and CountAppsForTenant back the per-plan service cap the create
	// path enforces (w6/m1): a Hobby workspace is capped at 25 apps.
	GetTenant(ctx context.Context, id string) (Tenant, error)
	CountAppsForTenant(ctx context.Context, tenantID string) (int, error)
	// TenantForIdentity returns the tenant a subject (Kratos identity id or
	// Hydra client id) is a member of via tenant_members, or ErrNotFound — the
	// workspace a caller resolves to (w1/m9). One lookup serves both human and
	// machine callers since tenant_members.subject covers both kinds of id.
	TenantForIdentity(ctx context.Context, subject string) (Tenant, error)
	// CreateTenantWithMember mints a personal tenant for an identity on first
	// login: a tenant row owned by the identity plus an admin membership. It is
	// idempotent and race-safe — concurrent first logins for the same identity
	// yield exactly one tenant (the partial unique index on owner_identity_id
	// is the gate, not a check-then-insert). The tenant name is a placeholder
	// (its id) pending a future rename API.
	CreateTenantWithMember(ctx context.Context, identityID, plan string) (Tenant, error)
	// AddMember records a subject's membership in a tenant (idempotent). The
	// platform tenant-create path uses it to make the Admin identity a member —
	// without the row the resolver can't map that identity to its workspace.
	AddMember(ctx context.Context, subject, tenantID, role string) error
	// BindClient records that an API key belongs to a tenant — a tenant_members
	// row keyed by the key's client_id, the same table TenantForIdentity reads
	// (idempotent — a re-bind to the same or another tenant upserts). The
	// api-keys mint calls this after creating the Hydra client.
	BindClient(ctx context.Context, clientID, tenantID string) error
	// UnbindClient removes an API key's tenant binding (idempotent — a key that
	// was never bound is not an error). The api-keys revoke calls this.
	UnbindClient(ctx context.Context, clientID string) error
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
	// SetAppIdleTTL updates the row's idle-TTL — the single write path for the
	// idle-timeout verb on store-managed Apps (the projector owns
	// spec.idleTTLSeconds), same row-first rationale as SetAppReplicas.
	SetAppIdleTTL(ctx context.Context, id string, seconds int32) error

	// UpsertUsageHourly writes one window row idempotently (ON CONFLICT DO
	// UPDATE) — the write path for the metering loop (w8/m1). Re-processing
	// the same window is safe.
	UpsertUsageHourly(ctx context.Context, row HourlyRow) error
	// LatestUsageWindow returns the most-recent window_start for a service so
	// the metering loop can catch up from where it left off after a restart.
	// Returns zero time when no rows exist yet.
	LatestUsageWindow(ctx context.Context, serviceID string) (time.Time, error)
	// UsageMonthToDate returns month-to-date aggregates (grouped by service /
	// kind / tier) for a workspace, bounded by the caller-supplied now so tests
	// don't depend on wall time. Sums usage_hourly and usage_monthly together,
	// so the result is exact whether or not the month has been compacted.
	UsageMonthToDate(ctx context.Context, workspaceID string, now time.Time) ([]UsageSummaryRow, error)
	// CompactUsage folds hourly rows older than before into usage_monthly and
	// purges them — atomic and idempotent; the retention loop (w8/m4) calls it
	// daily with the hot-window boundary.
	CompactUsage(ctx context.Context, before time.Time) (UsageCompaction, error)

	// CreateDeploy opens a new deploy row for appID (status
	// DeployUpdateInProgress) — CreateApp calls this for an app's first deploy
	// (trigger "create"); the deploys feature's Trigger verb calls it for an
	// explicit redeploy (trigger "api"). The reconciler's write-back closes it.
	CreateDeploy(ctx context.Context, appID, trigger, image string) (Deploy, error)
	// ListDeploys returns an app's deploy history, newest first.
	ListDeploys(ctx context.Context, appID string) ([]Deploy, error)
	// GetDeploy fetches one deploy scoped to appID — a deployID belonging to a
	// different app is ErrNotFound, not a cross-app leak.
	GetDeploy(ctx context.Context, appID, deployID string) (Deploy, error)
	// ListOpenDeploys returns every non-terminal (DeployUpdateInProgress)
	// deploy across all apps in one query — the reconciler's write-back hook
	// calls this once per ReconcileOnce pass and looks apps up in the result,
	// rather than one query per app in its per-app loop.
	ListOpenDeploys(ctx context.Context) ([]Deploy, error)
	// CloseDeploy marks a deploy row terminal (status DeployLive or
	// DeployUpdateFailed) with finished_at = now. A no-op if already terminal.
	CloseDeploy(ctx context.Context, id, status string) error
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
	t := Tenant{ID: ids.New(ids.Workspace), Name: name, Plan: plan}
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
		t, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// scanTenant reads the four public tenant columns off a row (id/name/plan/
// created_at — owner_identity_id is internal and never surfaced in the Tenant
// shape the API returns).
func scanTenant(row pgx.Row) (Tenant, error) {
	var t Tenant
	err := row.Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt)
	return t, err
}

func (s *PGStore) TenantForIdentity(ctx context.Context, subject string) (Tenant, error) {
	t, err := scanTenant(s.Pool.QueryRow(ctx, `
		SELECT t.id, t.name, t.plan, t.created_at FROM tenants t
		JOIN tenant_members m ON m.tenant_id = t.id
		WHERE m.subject = $1`, subject))
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	return t, nil
}

// CreateTenantWithMember mints a personal tenant for an identity in one
// transaction. The INSERT ... ON CONFLICT (owner_identity_id) DO UPDATE is the
// race-safe idempotent gate: a concurrent first login that already inserted a
// tenant for this identity makes this a no-op that returns the winner's row
// (DO UPDATE exists only to surface RETURNING for the existing row). The
// membership upsert is idempotent for the same reason. The tenant name is its
// id — a unique DNS-safe placeholder.
func (s *PGStore) CreateTenantWithMember(ctx context.Context, identityID, plan string) (Tenant, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Tenant{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit is the source of truth
	id := ids.New(ids.Workspace)
	t, err := scanTenant(tx.QueryRow(ctx, `
		INSERT INTO tenants (id, name, plan, owner_identity_id)
		VALUES ($1, $1, $2, $3)
		ON CONFLICT (owner_identity_id) WHERE owner_identity_id IS NOT NULL
		DO UPDATE SET owner_identity_id = EXCLUDED.owner_identity_id
		RETURNING id, name, plan, created_at`,
		id, plan, identityID))
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_members (tenant_id, subject, role)
		VALUES ($1, $2, 'admin')
		ON CONFLICT DO NOTHING`, t.ID, identityID); err != nil {
		return Tenant{}, classify("tenant_member", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Tenant{}, err
	}
	return t, nil
}

// AddMember records a subject's membership in a tenant. Used both by the
// platform tenant-create path (store/api.go, an explicit Admin) and by
// BindClient (a minted API key is "membership" too — same table, same shape).
func (s *PGStore) AddMember(ctx context.Context, subject, tenantID, role string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO tenant_members (tenant_id, subject, role)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`, tenantID, subject, role)
	if err != nil {
		return classify("tenant_member", err)
	}
	return nil
}

// BindClient records that an API key belongs to a tenant: a tenant_members row
// with the key's client_id as subject and role "developer" — least privilege
// that still covers every resource verb, matching the FGA grant apikeys mints
// (w1/m9). The PK is (tenant_id, subject), so a rebind to a DIFFERENT tenant
// is not a PK conflict — delete any existing binding for this client first, in
// the same transaction, so a client is bound to at most one tenant.
func (s *PGStore) BindClient(ctx context.Context, clientID, tenantID string) error {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM tenant_members WHERE subject = $1`, clientID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO tenant_members (tenant_id, subject, role)
			VALUES ($1, $2, 'developer')`, tenantID, clientID)
		return err
	})
	if err != nil {
		return classify("tenant_member", err)
	}
	return nil
}

// UnbindClient removes an API key's tenant_members row across every tenant it
// might be bound to (idempotent — a key that was never bound is not an error).
func (s *PGStore) UnbindClient(ctx context.Context, clientID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM tenant_members WHERE subject = $1`, clientID)
	if err != nil {
		return classify("tenant_member", err)
	}
	return nil // idempotent — a key that was never bound is not an error
}

// CreateApp inserts the app row and opens its first deploy row (trigger
// "create") in one transaction — every store-managed app has exactly one
// deploy the instant it exists, so ListDeploys is never truthfully empty for
// an app the reconciler is about to project (t001: "creating a service
// records deploy #1"). Bundling a second, related insert into the row's own
// creation is the same precedent CreateTenantWithMember already sets (a
// tenant plus its owner membership, one transaction) — an invariant the type
// itself guarantees is safer here than trusting every future caller to
// remember a second call.
func (s *PGStore) CreateApp(ctx context.Context, a App) (App, error) {
	a.ID = ids.New(ids.Service)
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO apps (id, tenant_id, name, repo, image, branch, port, replicas, tier, idle_ttl_seconds, suspended)
			 VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6, $7, $8, $9, $10, $11)
			 RETURNING created_at`,
			a.ID, a.TenantID, a.Name, a.Repo, a.Image, a.Branch, a.Port, a.Replicas, a.Tier, a.IdleTTLSeconds, a.Suspended,
		).Scan(&a.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO deploys (id, app_id, trigger, image, status) VALUES ($1, $2, 'create', $3, $4)`,
			ids.New(ids.Deploy), a.ID, a.Image, DeployUpdateInProgress)
		return err
	})
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
	d := Domain{ID: ids.New(ids.Domain), AppID: appID, Host: host, Primary: primary}
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

// SetAppIdleTTL updates the row's idle-TTL seconds (the apps feature's
// idle-timeout verb validates the bound before calling this). The projector
// carries it onto spec.idleTTLSeconds the same way it carries replicas.
func (s *PGStore) SetAppIdleTTL(ctx context.Context, id string, seconds int32) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE apps SET idle_ttl_seconds = $2, updated_at = now() WHERE id = $1`,
		id, seconds)
	if err != nil {
		return classify("app", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	return nil
}

func (s *PGStore) CreateDeploy(ctx context.Context, appID, trigger, image string) (Deploy, error) {
	d := Deploy{ID: ids.New(ids.Deploy), AppID: appID, Trigger: trigger, Image: image, Status: DeployUpdateInProgress}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO deploys (id, app_id, trigger, image, status) VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		d.ID, d.AppID, d.Trigger, d.Image, d.Status,
	).Scan(&d.CreatedAt)
	if err != nil {
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

const deployColumns = `id, app_id, trigger, image, status, created_at, started_at, finished_at`

func scanDeploy(row pgx.Row) (Deploy, error) {
	var d Deploy
	err := row.Scan(&d.ID, &d.AppID, &d.Trigger, &d.Image, &d.Status, &d.CreatedAt, &d.StartedAt, &d.FinishedAt)
	return d, err
}

func (s *PGStore) ListDeploys(ctx context.Context, appID string) ([]Deploy, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+deployColumns+` FROM deploys WHERE app_id = $1 ORDER BY created_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deploy
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGStore) GetDeploy(ctx context.Context, appID, deployID string) (Deploy, error) {
	d, err := scanDeploy(s.Pool.QueryRow(ctx,
		`SELECT `+deployColumns+` FROM deploys WHERE id = $1 AND app_id = $2`, deployID, appID))
	if err != nil {
		return Deploy{}, classify("deploy", err)
	}
	return d, nil
}

func (s *PGStore) ListOpenDeploys(ctx context.Context) ([]Deploy, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+deployColumns+` FROM deploys WHERE status = $1`, DeployUpdateInProgress)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deploy
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PGStore) CloseDeploy(ctx context.Context, id, status string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE deploys SET status = $2, finished_at = now() WHERE id = $1 AND finished_at IS NULL`, id, status)
	return err
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
