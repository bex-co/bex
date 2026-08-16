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
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// GroupingStore is the project/environment surface the Blueprint apply loop
// writes through — satisfied by *PGStore (pool-backed, non-transactional) and
// by the RunGroupingTx callback's tx-scoped facade (w8/m20 t001: a mid-loop
// failure then rolls back every grouping row from that sync).
type GroupingStore interface {
	ListProjects(ctx context.Context, tenantID string) ([]Project, error)
	CreateProject(ctx context.Context, tenantID, name string) (Project, error)
	ListEnvironments(ctx context.Context, projectID string) ([]Environment, error)
	CreateEnvironment(ctx context.Context, projectID, tenantID, name string) (Environment, error)
	SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error
	// CountWorkspaceGroupings returns the workspace's durable project and
	// environment totals — the quota input (w8/m20 t002). Inside a grouping
	// transaction the counts are consistent with the writes.
	CountWorkspaceGroupings(ctx context.Context, tenantID string) (projects, environments int, err error)
}

// RunGroupingTx runs fn against a transaction-scoped GroupingStore: every
// grouping write inside fn commits together or not at all. fn returning an
// error (or a panic) rolls the whole set back.
func (s *PGStore) RunGroupingTx(ctx context.Context, fn func(GroupingStore) error) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		return fn(txGroupingStore{tx: tx})
	})
}

// CountWorkspaceGroupings implements the pool-backed count (also used outside
// a transaction, e.g. by reads).
func (s *PGStore) CountWorkspaceGroupings(ctx context.Context, tenantID string) (int, int, error) {
	return countWorkspaceGroupings(ctx, s.Pool, tenantID)
}

// groupingQuerier is the query surface shared by *pgxpool.Pool and pgx.Tx —
// the same free-function-over-a-small-interface shape as setEnvironmentACL —
// so every grouping statement is written once and runs on either.
type groupingQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func listProjects(ctx context.Context, q groupingQuerier, tenantID string) ([]Project, error) {
	rows, err := q.Query(ctx,
		`SELECT id, tenant_id, name, created_at FROM projects WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func createProject(ctx context.Context, q groupingQuerier, tenantID, name string) (Project, error) {
	p := Project{ID: ids.New(ids.Project), TenantID: tenantID, Name: name}
	err := q.QueryRow(ctx,
		`INSERT INTO projects (id, tenant_id, name) VALUES ($1, $2, $3) RETURNING created_at`,
		p.ID, tenantID, name,
	).Scan(&p.CreatedAt)
	if err != nil {
		return Project{}, classify("project", err)
	}
	return p, nil
}

func listEnvironments(ctx context.Context, q groupingQuerier, projectID string) ([]Environment, error) {
	rows, err := q.Query(ctx,
		`SELECT `+environmentColumns+` FROM environments WHERE project_id = $1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Environment
	for rows.Next() {
		e, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func createEnvironment(ctx context.Context, q groupingQuerier, projectID, tenantID, name string) (Environment, error) {
	seed, err := json.Marshal(core.DefaultEnvironmentAllowList())
	if err != nil {
		return Environment{}, err
	}
	return scanEnvironment(q.QueryRow(ctx,
		`INSERT INTO environments (id, project_id, tenant_id, name, ip_allow_list) VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+environmentColumns,
		ids.New(ids.Environment), projectID, tenantID, name, seed,
	))
}

type txGroupingStore struct{ tx pgx.Tx }

func (t txGroupingStore) ListProjects(ctx context.Context, tenantID string) ([]Project, error) {
	return listProjects(ctx, t.tx, tenantID)
}

func (t txGroupingStore) CreateProject(ctx context.Context, tenantID, name string) (Project, error) {
	return createProject(ctx, t.tx, tenantID, name)
}

func (t txGroupingStore) ListEnvironments(ctx context.Context, projectID string) ([]Environment, error) {
	return listEnvironments(ctx, t.tx, projectID)
}

func (t txGroupingStore) CreateEnvironment(ctx context.Context, projectID, tenantID, name string) (Environment, error) {
	return createEnvironment(ctx, t.tx, projectID, tenantID, name)
}

func (t txGroupingStore) SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error {
	return setEnvironmentACL(ctx, t.tx, id, protectedStatus, networkIsolationEnabled, ipAllowList)
}

func (t txGroupingStore) CountWorkspaceGroupings(ctx context.Context, tenantID string) (int, int, error) {
	return countWorkspaceGroupings(ctx, t.tx, tenantID)
}

// countingQuerier is the single-row query surface shared by pool and tx.
type countingQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func countWorkspaceGroupings(ctx context.Context, q countingQuerier, tenantID string) (int, int, error) {
	var projects, environments int
	err := q.QueryRow(ctx,
		`SELECT
		   (SELECT count(*) FROM projects WHERE tenant_id = $1),
		   (SELECT count(*) FROM environments WHERE tenant_id = $1)`, tenantID,
	).Scan(&projects, &environments)
	return projects, environments, err
}

// GroupingPair names one blueprint-declared project/environment grouping.
type GroupingPair struct {
	Project     string
	Environment string
}

// ReclaimEmptyBlueprintGroupings deletes, in ONE transaction, the named
// grouping rows a disconnected Blueprint minted that nothing still references
// (w8/m20 t004): an environment goes when no apps row is assigned to it and
// its id is not in referencedEnvironments (CR-side datastore members); a
// candidate project goes when it has no environments left, no apps members,
// and is not in referencedProjects. Deployed resources are never touched —
// a populated grouping survives (Render disconnect semantics). Returns the
// removed names for post-commit auditing.
func (s *PGStore) ReclaimEmptyBlueprintGroupings(ctx context.Context, tenantID string, pairs []GroupingPair, referencedEnvironments, referencedProjects map[string]bool) (removedEnvironments, removedProjects []string, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		candidateProjects := map[string]string{} // id → name
		for _, pair := range pairs {
			var projectID string
			if err := tx.QueryRow(ctx,
				`SELECT id FROM projects WHERE tenant_id = $1 AND name = $2`, tenantID, pair.Project,
			).Scan(&projectID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				return err
			}
			candidateProjects[projectID] = pair.Project
			var envID string
			if err := tx.QueryRow(ctx,
				`SELECT id FROM environments WHERE project_id = $1 AND tenant_id = $2 AND name = $3`,
				projectID, tenantID, pair.Environment,
			).Scan(&envID); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				return err
			}
			if referencedEnvironments[envID] {
				continue
			}
			var members int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM apps WHERE environment_id = $1`, envID,
			).Scan(&members); err != nil {
				return err
			}
			if members > 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `DELETE FROM environments WHERE id = $1`, envID); err != nil {
				return err
			}
			removedEnvironments = append(removedEnvironments, pair.Project+"/"+pair.Environment)
		}
		for projectID, name := range candidateProjects {
			if referencedProjects[projectID] {
				continue
			}
			var environments, members int
			if err := tx.QueryRow(ctx,
				`SELECT
				   (SELECT count(*) FROM environments WHERE project_id = $1),
				   (SELECT count(*) FROM apps WHERE project_id = $1)`, projectID,
			).Scan(&environments, &members); err != nil {
				return err
			}
			if environments > 0 || members > 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `DELETE FROM projects WHERE id = $1`, projectID); err != nil {
				return err
			}
			removedProjects = append(removedProjects, name)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return removedEnvironments, removedProjects, nil
}
