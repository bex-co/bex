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

// insights.go is the live introspection surface for managed Postgres databases:
// processes / top-queries / sizes / table-scans / parameter-overrides.  All
// read verbs connect via the CNPG-generated app Secret and run a predefined,
// read-only query — none echo caller-supplied SQL.  The write verb
// (SetParameterOverrides) patches Database.spec.parameters; the operator
// projects it to the CNPG Cluster's spec.postgresql.parameters.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ProcessView is one row from pg_stat_activity (live backend process).
type ProcessView struct {
	PID             int32  `json:"pid"`
	UserName        string `json:"userName"`
	ApplicationName string `json:"applicationName"`
	State           string `json:"state"`
	Query           string `json:"query,omitempty"`
	WaitEventType   string `json:"waitEventType,omitempty"`
	WaitEvent       string `json:"waitEvent,omitempty"`
	DurationSeconds int32  `json:"durationSeconds"`
}

// TopQueryView is one row from pg_stat_statements (aggregated by query text).
type TopQueryView struct {
	Query          string  `json:"query"`
	Calls          int64   `json:"calls"`
	TotalTimeMs    float64 `json:"totalTimeMs"`
	MeanTimeMs     float64 `json:"meanTimeMs"`
	Rows           int64   `json:"rows"`
	SharedHitBlks  int64   `json:"sharedHitBlks"`
	SharedReadBlks int64   `json:"sharedReadBlks"`
}

// DatabaseSizeView is the overall database size.
type DatabaseSizeView struct {
	Name       string `json:"name"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizePretty string `json:"sizePretty"`
}

// TableSizeView is one table's total size (data + indexes).
type TableSizeView struct {
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizePretty string `json:"sizePretty"`
}

// SizesView bundles the database-level size and per-table sizes.
type SizesView struct {
	Database DatabaseSizeView `json:"database"`
	Tables   []TableSizeView  `json:"tables"`
}

// TableScanView is one row from pg_stat_user_tables.
type TableScanView struct {
	Schema        string `json:"schema"`
	Name          string `json:"name"`
	SeqScans      int64  `json:"seqScans"`
	SeqScanRows   int64  `json:"seqScanRows"`
	IndexScans    int64  `json:"indexScans"`
	IndexScanRows int64  `json:"indexScanRows"`
	LiveRows      int64  `json:"liveRows"`
	DeadRows      int64  `json:"deadRows"`
}

// ParameterOverrideView is one non-default pg_settings row.
//
// This is the OBSERVED effective configuration, and most of it belongs to the
// operator, not the tenant: a database nobody has ever configured reports ~48
// rows here, CloudNativePG's archive/restore commands, TLS paths and
// replication settings among them. It is a read-only diagnostic. The tenant's
// own declared overrides are ParameterSpecView, and only those may be edited
// (w6/m133 — the editor used to be seeded from THIS view, so a single edit
// replaced spec.parameters with the operator's own configuration).
type ParameterOverrideView struct {
	Name        string `json:"name"`
	Setting     string `json:"setting"`
	Unit        string `json:"unit,omitempty"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
}

// ParameterSpecView is one parameter the TENANT declared, from
// Database.spec.parameters — the editable set, and the only one a write
// replaces. Empty for a database nobody has configured.
type ParameterSpecView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

const (
	sqlProcesses = `
SELECT pid::int,
       COALESCE(usename, '')             AS username,
       COALESCE(application_name, '')    AS appname,
       COALESCE(state, '')               AS state,
       COALESCE(left(query, 200), '')    AS query,
       COALESCE(wait_event_type, '')     AS wait_event_type,
       COALESCE(wait_event, '')          AS wait_event,
       COALESCE(extract(epoch FROM clock_timestamp() - query_start)::int, 0) AS duration_sec
FROM pg_stat_activity
WHERE pid <> pg_backend_pid()
ORDER BY state, duration_sec DESC NULLS LAST`

	sqlTopQueries = `
SELECT COALESCE(left(query, 500), '')   AS query,
       calls,
       total_exec_time                  AS total_ms,
       mean_exec_time                   AS mean_ms,
       rows,
       shared_blks_hit                  AS hit_blks,
       shared_blks_read                 AS read_blks
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 25`

	sqlDatabaseSize = `
SELECT current_database()                                    AS dbname,
       pg_database_size(current_database())                  AS size_bytes,
       pg_size_pretty(pg_database_size(current_database()))  AS size_pretty`

	sqlTableSizes = `
SELECT schemaname                                                                              AS schema,
       tablename                                                                               AS tbl,
       pg_total_relation_size(quote_ident(schemaname)||'.'||quote_ident(tablename))           AS size_bytes,
       pg_size_pretty(pg_total_relation_size(quote_ident(schemaname)||'.'||quote_ident(tablename))) AS size_pretty
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY size_bytes DESC
LIMIT 50`

	sqlTableScans = `
SELECT schemaname                        AS schema,
       relname                           AS tbl,
       seq_scan,
       seq_tup_read,
       COALESCE(idx_scan, 0)             AS idx_scan,
       COALESCE(idx_tup_fetch, 0)        AS idx_tup_fetch,
       n_live_tup,
       n_dead_tup
FROM pg_stat_user_tables
ORDER BY seq_scan DESC
LIMIT 50`

	sqlParameterOverrides = `
SELECT name,
       setting,
       COALESCE(unit, '')       AS unit,
       source,
       COALESCE(short_desc, '') AS descr
FROM pg_settings
WHERE source NOT IN ('default', 'override')
ORDER BY name`
)

// runInsight dials the database and executes sql inside the standard read-only
// envelope (same safety rails as Query). The caller maps rows to its typed view.
func (s *Service) runInsight(ctx context.Context, relation, dbID, sql string) (QueryResult, error) {
	db, err := s.AuthorizeDatabase(ctx, relation, dbID)
	if err != nil {
		return QueryResult{}, err
	}
	// Processes/TopQueries (and every other insight that dials the tenant
	// database) surface live query text. RelCanViewSensitive and RelCanView
	// are both read relations, so Authorize uses the decision cache; re-check
	// uncached before loading the connection Secret (codex round-15 #4).
	if err := s.AuthorizeDatabaseFresh(ctx, relation, db); err != nil {
		return QueryResult{}, err
	}
	sec, err := s.databaseSecret(ctx, db)
	if err != nil {
		return QueryResult{}, err
	}
	uri := string(sec.Data["uri"])
	if uri == "" {
		return QueryResult{}, core.ErrNotFound
	}
	return runReadOnlyQuery(ctx, uri, sql, queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap})
}

// strVal extracts a string from a pgx any value (nil → "").
func strVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// intVal extracts an int64 from a pgx any value (numeric types, nil → 0).
func intVal(v any) int64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int16:
		return int64(t)
	case int8:
		return int64(t)
	case float64:
		return int64(t)
	case float32:
		return int64(t)
	}
	return 0
}

// floatVal extracts a float64 from a pgx any value (nil → 0).
func floatVal(v any) float64 {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	}
	return 0
}

// Processes returns a snapshot of active backend processes (pg_stat_activity).
// Requires RelCanViewSensitive because process query texts may contain literal values.
func (s *Service) Processes(ctx context.Context, dbID string) ([]ProcessView, error) {
	res, err := s.runInsight(ctx, core.RelCanViewSensitive, dbID, sqlProcesses)
	if err != nil {
		return nil, err
	}
	out := make([]ProcessView, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 8 {
			continue
		}
		out = append(out, ProcessView{
			PID:             int32(intVal(row[0])),
			UserName:        strVal(row[1]),
			ApplicationName: strVal(row[2]),
			State:           strVal(row[3]),
			Query:           strVal(row[4]),
			WaitEventType:   strVal(row[5]),
			WaitEvent:       strVal(row[6]),
			DurationSeconds: int32(intVal(row[7])),
		})
	}
	return out, nil
}

// TopQueries returns the top 25 queries by total execution time from
// pg_stat_statements. Returns an empty list (not an error) when pg_stat_statements
// is not yet available on the cluster — a graceful path for databases provisioned
// before pg_stat_statements was enabled.
// Requires RelCanViewSensitive because query texts may contain literal values.
func (s *Service) TopQueries(ctx context.Context, dbID string) ([]TopQueryView, error) {
	res, err := s.runInsight(ctx, core.RelCanViewSensitive, dbID, sqlTopQueries)
	if err != nil {
		// An auth refusal must still refuse — everything else (unknown db,
		// pg_stat_statements not installed, connection failure) falls through to
		// an empty list, same as before this verb's fetch started gating on
		// workspace membership too (w6/m17).
		if errors.Is(err, core.ErrForbidden) || errors.Is(err, core.ErrAuthzUnavailable) {
			return nil, err
		}
		return []TopQueryView{}, nil
	}
	out := make([]TopQueryView, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 7 {
			continue
		}
		out = append(out, TopQueryView{
			Query:          strVal(row[0]),
			Calls:          intVal(row[1]),
			TotalTimeMs:    floatVal(row[2]),
			MeanTimeMs:     floatVal(row[3]),
			Rows:           intVal(row[4]),
			SharedHitBlks:  intVal(row[5]),
			SharedReadBlks: intVal(row[6]),
		})
	}
	return out, nil
}

// Sizes returns the database size and per-table sizes via pg_database_size /
// pg_total_relation_size.
func (s *Service) Sizes(ctx context.Context, dbID string) (SizesView, error) {
	dbRes, err := s.runInsight(ctx, core.RelCanView, dbID, sqlDatabaseSize)
	if err != nil {
		return SizesView{}, err
	}
	var dbSize DatabaseSizeView
	if len(dbRes.Rows) > 0 && len(dbRes.Rows[0]) >= 3 {
		r := dbRes.Rows[0]
		dbSize = DatabaseSizeView{
			Name:       strVal(r[0]),
			SizeBytes:  intVal(r[1]),
			SizePretty: strVal(r[2]),
		}
	}
	tblRes, err := s.runInsight(ctx, core.RelCanView, dbID, sqlTableSizes)
	if err != nil {
		return SizesView{}, err
	}
	tables := make([]TableSizeView, 0, len(tblRes.Rows))
	for _, row := range tblRes.Rows {
		if len(row) < 4 {
			continue
		}
		tables = append(tables, TableSizeView{
			Schema:     strVal(row[0]),
			Name:       strVal(row[1]),
			SizeBytes:  intVal(row[2]),
			SizePretty: strVal(row[3]),
		})
	}
	return SizesView{Database: dbSize, Tables: tables}, nil
}

// TableScans returns sequential vs index scan stats from pg_stat_user_tables.
func (s *Service) TableScans(ctx context.Context, dbID string) ([]TableScanView, error) {
	res, err := s.runInsight(ctx, core.RelCanView, dbID, sqlTableScans)
	if err != nil {
		return nil, err
	}
	out := make([]TableScanView, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 8 {
			continue
		}
		out = append(out, TableScanView{
			Schema:        strVal(row[0]),
			Name:          strVal(row[1]),
			SeqScans:      intVal(row[2]),
			SeqScanRows:   intVal(row[3]),
			IndexScans:    intVal(row[4]),
			IndexScanRows: intVal(row[5]),
			LiveRows:      intVal(row[6]),
			DeadRows:      intVal(row[7]),
		})
	}
	return out, nil
}

// ParameterOverrides returns non-default postgresql.conf parameters from pg_settings.
func (s *Service) ParameterOverrides(ctx context.Context, dbID string) ([]ParameterOverrideView, error) {
	res, err := s.runInsight(ctx, core.RelCanView, dbID, sqlParameterOverrides)
	if err != nil {
		return nil, err
	}
	out := make([]ParameterOverrideView, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 5 {
			continue
		}
		out = append(out, ParameterOverrideView{
			Name:        strVal(row[0]),
			Setting:     strVal(row[1]),
			Unit:        strVal(row[2]),
			Source:      strVal(row[3]),
			Description: strVal(row[4]),
		})
	}
	return out, nil
}

// SetParameterOverrides replaces the Database's parameter overrides.
// shared_preload_libraries cannot be overridden (the operator always sets it
// to include pg_stat_statements); any entry with that key is silently dropped.
func (s *Service) SetParameterOverrides(ctx context.Context, dbID string, params map[string]string) (PostgresView, error) {
	return s.UpdatePostgres(ctx, dbID, PostgresPatch{ParameterOverrides: &params})
}

// ParameterSpec returns the tenant's DECLARED parameter overrides
// (Database.spec.parameters) as a stable name-sorted list — the read every
// surface exposes and the editor binds to. GetParameterSpec below returns the
// same data as a map for callers that want one.
//
// A write replaces exactly this set, so this is the only honest thing to seed
// an editor from: ParameterOverrides is the observed pg_settings config and is
// mostly the operator's (w6/m133).
func (s *Service) ParameterSpec(ctx context.Context, dbID string) ([]ParameterSpecView, error) {
	params, err := s.GetParameterSpec(ctx, dbID)
	if err != nil {
		return nil, err
	}
	out := make([]ParameterSpecView, 0, len(params))
	for name, value := range params {
		out = append(out, ParameterSpecView{Name: name, Value: value})
	}
	slices.SortFunc(out, func(a, b ParameterSpecView) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// GetParameterSpec returns the currently stored parameter overrides from the
// Database CR (spec.parameters), not from pg_settings. Use ParameterOverrides for
// the live database view.
func (s *Service) GetParameterSpec(ctx context.Context, dbID string) (map[string]string, error) {
	d, err := s.fetchDatabase(ctx, core.RelCanView, dbID)
	if err != nil {
		return nil, err
	}
	if d.Spec.Parameters == nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(d.Spec.Parameters))
	for k, v := range d.Spec.Parameters {
		out[k] = v
	}
	return out, nil
}
