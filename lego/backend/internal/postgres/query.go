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

// query.go is the shared SQL execution path behind Render-compatible MCP reads
// and the dashboard SQL console. The safety rails are enforced by the database
// session, not by parsing the caller's SQL: every statement has a server-side
// timeout and bounded result set. MCP and ordinary dashboard reads additionally
// run inside an explicit read-only transaction; the dashboard can explicitly
// opt into a write transaction after its confirmation step. Errors are mapped to
// fixed, value-free messages — SQL text and result values never reach logs or
// error strings (the env-var-values rule).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Shared query safety rails. A statement that runs longer than the timeout is
// cancelled server-side; a result set larger than the row cap is truncated and
// flagged rather than streamed unbounded into an agent or browser.
const (
	queryStatementTimeout = 10 * time.Second
	queryRowCap           = 500
	queryColumnCap        = 256
	queryColumnNameCap    = 64 << 10
	queryCellByteCap      = 256 << 10
	queryRowByteCap       = 1 << 20
	queryRawResultByteCap = 4 << 20
	queryJSONResultCap    = 8 << 20

	// The frontend checks this limit before allocating a PostgreSQL protocol
	// message body. It is the first line of defense against one giant DataRow;
	// the tighter decoded cell/row/result budgets below then bound retained data.
	queryWireMessageBodyCap = queryRowByteCap + 64<<10
)

// The value-free error vocabulary the query path surfaces. None of these ever
// embed the caller's SQL or a row value — Render-trained agents key on the class,
// not the Postgres message (which can echo query tokens). The two caller-fault
// classes wrap core.ErrBadRequest so they flow through the shared WriteErr
// mapping as 400 on any surface, like every other feature's verbs; the two
// internal-fault classes stay unclassified (500).
var (
	errQueryReadOnly       = fmt.Errorf("%w: read-only transaction; write and DDL statements are not permitted", core.ErrBadRequest)
	errQueryTimeout        = fmt.Errorf("%w: query exceeded the statement timeout", core.ErrBadRequest)
	errQueryResultTooLarge = fmt.Errorf("%w: query result exceeds the size limit", core.ErrBadRequest)
	errQueryConnClose      = core.Err("could not connect to the database")
	errQueryFailed         = core.Err("query failed")
)

// QueryResult is the columns/rows shape the query_render_postgres tool serializes.
// Rows carry pgx's decoded Go values (JSON-marshalled by the MCP layer); Truncated
// signals the row cap was hit and more rows exist.
type QueryResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int      `json:"rowCount"`
	Truncated bool     `json:"truncated"`
}

// queryLimits parameterizes the rails so tests can exercise the timeout and cap
// quickly; Query and ExecuteQuery use the package defaults above.
type queryLimits struct {
	statementTimeout time.Duration
	rowCap           int
}

type queryExecutor func(context.Context, string, string, queryLimits, bool) (QueryResult, error)

// Query runs a single read-only SQL statement against a managed database and
// returns its columns and rows. It connects over CNPG's internal URI (bex-api runs
// in-cluster next to the databases) inside a hard read-only envelope; writes, DDL,
// multi-statement escapes and over-long queries are rejected by Postgres itself.
func (s *Service) Query(ctx context.Context, dbID, sql string) (QueryResult, error) {
	db, err := s.fetchDatabaseForRead(ctx, core.RelCanViewSensitive, dbID)
	if err != nil {
		return QueryResult{}, err // core.ErrNotFound for an unknown/unprovisioned db
	}
	return s.executeAuthorizedQuery(ctx, db, sql, true)
}

// ExecuteQuery powers the dashboard/REST SQL console. Reads use the exact
// can_view_sensitive + read-only contract as Query (and therefore the MCP tool).
// allowWrites is an explicit opt-in used only after the dashboard confirmation;
// it runs one read-write transaction and requires BOTH developer-level
// can_create AND can_view_sensitive: a writable statement can still return
// rows (SELECT in a read-write transaction, DML ... RETURNING), so the write
// capability alone must never unlock the sensitive-read half (codex round 18).
// On a scoped third-party OAuth token this composes to bex.write AND
// bex.sensitive; a token holding only one is refused. Both modes share the
// same timeout, row cap, secret resolution, and error map.
func (s *Service) ExecuteQuery(ctx context.Context, dbID, sql string, allowWrites bool) (QueryResult, error) {
	relation := core.RelCanViewSensitive
	if allowWrites {
		relation = core.RelCanCreate
	}
	var (
		db  *appv1alpha1.Database
		err error
	)
	if allowWrites {
		// Writable console may still authorize a terminating CR (write carveout).
		db, err = s.fetchDatabase(ctx, relation, dbID)
	} else {
		db, err = s.fetchDatabaseForRead(ctx, relation, dbID)
	}
	if err != nil {
		return QueryResult{}, err
	}
	if allowWrites {
		// The sensitive half rides the Fresh seam: it must not emit an ALLOWED
		// read-relation audit row for what is, to the caller, a write verb.
		if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanViewSensitive, db); err != nil {
			return QueryResult{}, err
		}
	}
	return s.executeAuthorizedQuery(ctx, db, sql, !allowWrites)
}

// executeAuthorizedQuery resolves the connection credential and runs one
// statement. The caller's AuthorizeDatabase already ran (and audited); codex
// round-9 #7 adds an uncached reassertion here — arbitrary SQL is a
// sensitive/destructive sink, so a membership revoked inside PositiveTTL must
// not ride a cached positive to one last query against the Database. The
// writable mode re-asserts BOTH can_create and can_view_sensitive uncached
// (codex round 18): a read-write transaction can return rows, so both halves
// of the capability union are re-checked at the sink.
func (s *Service) executeAuthorizedQuery(ctx context.Context, db *appv1alpha1.Database, sql string, readOnly bool) (QueryResult, error) {
	relation := core.RelCanViewSensitive
	if !readOnly {
		relation = core.RelCanCreate
	}
	if err := s.AuthorizeDatabaseFresh(ctx, relation, db); err != nil {
		return QueryResult{}, err
	}
	if !readOnly {
		if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanViewSensitive, db); err != nil {
			return QueryResult{}, err
		}
	}
	if strings.TrimSpace(sql) == "" {
		return QueryResult{}, fmt.Errorf("%w: sql is required", core.ErrBadRequest)
	}
	sec, err := s.databaseSecret(ctx, db)
	if err != nil {
		return QueryResult{}, err
	}
	uri := string(sec.Data["uri"])
	if uri == "" {
		return QueryResult{}, core.ErrNotFound
	}
	executor := s.queryExecutor
	if executor == nil {
		executor = runSQLQuery
	}
	return executor(ctx, uri, sql, queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap}, readOnly)
}

// databaseSecret resolves a previously-authorized Database's CNPG application
// Secret without performing a second, potentially drifting authorization check.
func (s *Service) databaseSecret(ctx context.Context, db *appv1alpha1.Database) (*corev1.Secret, error) {
	secretName := db.Status.SecretName
	if secretName == "" {
		secretName = db.Name + "-app"
	}
	var sec corev1.Secret
	// The connection Secret is created by CNPG beside its Cluster, so it lives
	// in the Database CR's own namespace (ADR043 D8), not the shared one.
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: secretName}, &sec); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return &sec, nil
}

// budget accumulates one of the result-size dimensions runSQLQuery meters
// (column names, row bytes, raw total, encoded JSON total) and refuses the
// whole query once its cap is passed — every dimension answers the same
// errQueryResultTooLarge, so the check belongs in one place.
type budget struct{ used, cap int }

func (b *budget) add(n int) error {
	b.used += n
	if b.used > b.cap {
		return errQueryResultTooLarge
	}
	return nil
}

// runReadOnlyQuery is retained as the focused MCP/test helper. It delegates to
// the shared executor with the strongest transaction envelope enabled.
func runReadOnlyQuery(ctx context.Context, connString, sql string, lim queryLimits) (QueryResult, error) {
	return runSQLQuery(ctx, connString, sql, lim, true)
}

// runSQLQuery dials connString and executes one statement. In read-only mode the
// guarantee is layered: default_transaction_read_only=on at session startup and
// BEGIN READ ONLY block SET transaction_read_write escapes. Writable mode still
// gets the same statement_timeout, request deadline, single-statement extended
// protocol, and row cap, but commits the transaction on success.
// collectQueryRows streams the result set into out, enforcing the per-cell,
// per-row, whole-result and encoded-size budgets as it goes. Reading stops at
// the row cap (marking the result truncated) so an unbounded result set is never
// materialized.
func collectQueryRows(rows pgx.Rows, lim queryLimits, out *QueryResult) error {
	rawResult := budget{cap: queryRawResultByteCap}
	jsonResult := budget{cap: queryJSONResultCap}
	for rows.Next() {
		if len(out.Rows) >= lim.rowCap {
			out.Truncated = true // a row beyond the cap exists; stop reading
			break
		}
		row := budget{cap: queryRowByteCap}
		for _, cell := range rows.RawValues() {
			if len(cell) > queryCellByteCap {
				return errQueryResultTooLarge
			}
			if err := row.add(len(cell)); err != nil {
				return err
			}
		}
		if err := rawResult.add(row.used); err != nil {
			return err
		}

		vals, err := rows.Values()
		if err != nil {
			return mapPGError(err)
		}
		out.Rows = append(out.Rows, vals)
	}
	// rows.Err surfaces read-only violations / timeouts that fire mid-stream.
	if err := rows.Err(); err != nil {
		return mapPGError(err)
	}
	// One encoding for the whole result, not one per row thrown away for its
	// length: rawResult already bounded the loop, so this is bounded work.
	encoded, err := json.Marshal(out.Rows)
	if err != nil {
		return errQueryFailed
	}
	return jsonResult.add(len(encoded))
}

// buildQueryConnConfig pins the session's safety rails onto the connection
// itself — the read-only default and the server-side statement timeout — so they
// apply from the first statement rather than depending on a later SET.
func buildQueryConnConfig(connString string, lim queryLimits, readOnly bool) (*pgx.ConnConfig, error) {
	cfg, err := pgx.ParseConfig(connString)
	if err != nil {
		return nil, errQueryConnClose
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	if readOnly {
		cfg.RuntimeParams["default_transaction_read_only"] = "on"
	}
	cfg.RuntimeParams["statement_timeout"] = strconv.FormatInt(lim.statementTimeout.Milliseconds(), 10)
	cfg.BuildFrontend = newQueryFrontend
	return cfg, nil
}

func runSQLQuery(ctx context.Context, connString, sql string, lim queryLimits, readOnly bool) (QueryResult, error) {
	cfg, err := buildQueryConnConfig(connString, lim, readOnly)
	if err != nil {
		return QueryResult{}, err
	}

	// Bound the whole round-trip a little past the server-side timeout so a hung
	// dial or a server that ignores statement_timeout can't wedge the request.
	ctx, cancel := context.WithTimeout(ctx, lim.statementTimeout+5*time.Second)
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return QueryResult{}, errQueryConnClose
	}
	defer func() { _ = conn.Close(ctx) }()

	txOptions := pgx.TxOptions{}
	if readOnly {
		txOptions.AccessMode = pgx.ReadOnly
	}
	tx, err := conn.BeginTx(ctx, txOptions)
	if err != nil {
		return QueryResult{}, mapPGError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return QueryResult{}, mapPGError(err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	if len(fields) > queryColumnCap {
		return QueryResult{}, errQueryResultTooLarge
	}
	cols := make([]string, len(fields))
	columnNames := budget{cap: queryColumnNameCap}
	for i, f := range fields {
		if err := columnNames.add(len(f.Name)); err != nil {
			return QueryResult{}, err
		}
		cols[i] = f.Name
	}
	out := QueryResult{Columns: cols, Rows: [][]any{}}
	if err := collectQueryRows(rows, lim, &out); err != nil {
		return QueryResult{}, err
	}
	rows.Close()
	out.RowCount = len(out.Rows)
	if !readOnly && len(cols) == 0 {
		out.RowCount = int(rows.CommandTag().RowsAffected())
	}
	if !readOnly {
		if err := tx.Commit(ctx); err != nil {
			return QueryResult{}, mapPGError(err)
		}
	}
	return out, nil
}

func newQueryFrontend(r io.Reader, w io.Writer) *pgproto3.Frontend {
	frontend := pgproto3.NewFrontend(r, w)
	frontend.SetMaxBodyLen(queryWireMessageBodyCap)
	return frontend
}

// mapPGError collapses a Postgres/driver error to one of the fixed, value-free
// sentinels. Non-classified DB errors surface only their SQLSTATE code — never the
// Postgres message, which can echo a query token or a literal value.
func mapPGError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.ReadOnlySQLTransaction:
			return errQueryReadOnly
		case pgerrcode.QueryCanceled: // statement_timeout fired
			return errQueryTimeout
		}
		return fmt.Errorf("%w (SQLSTATE %s)", core.ErrBadRequest, pgErr.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errQueryTimeout
	}
	var bodyLenErr *pgproto3.ExceededMaxBodyLenErr
	if errors.As(err, &bodyLenErr) {
		return errQueryResultTooLarge
	}
	return errQueryFailed
}
