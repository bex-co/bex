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

// query.go is the read-only SQL execution path behind the MCP query_render_postgres
// tool. The safety rails are enforced by the database session, not by parsing the
// caller's SQL (agents write creative queries; denylists are not the mechanism):
// the connection starts with default_transaction_read_only=on and a statement
// timeout, and every statement runs inside an explicit BEGIN READ ONLY transaction.
// Errors are mapped to fixed, value-free messages — a query's text and its result
// values never reach a log line or an error string (the env-var-values rule).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Read-only query safety rails. A statement that runs longer than the timeout is
// cancelled server-side; a result set larger than the row cap is truncated and
// flagged rather than streamed unbounded into an agent's context.
const (
	queryStatementTimeout = 10 * time.Second
	queryRowCap           = 500
)

// The value-free error vocabulary the query path surfaces. None of these ever
// embed the caller's SQL or a row value — Render-trained agents key on the class,
// not the Postgres message (which can echo query tokens). The two caller-fault
// classes wrap core.ErrBadRequest so they flow through the shared WriteErr
// mapping as 400 on any surface, like every other feature's verbs; the two
// internal-fault classes stay unclassified (500).
var (
	errQueryReadOnly  = fmt.Errorf("%w: read-only transaction; write and DDL statements are not permitted", core.ErrBadRequest)
	errQueryTimeout   = fmt.Errorf("%w: query exceeded the statement timeout", core.ErrBadRequest)
	errQueryConnClose = core.Err("could not connect to the database")
	errQueryFailed    = core.Err("query failed")
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
// quickly; Query uses defaultQueryLimits.
type queryLimits struct {
	statementTimeout time.Duration
	rowCap           int
}

// Query runs a single read-only SQL statement against a managed database and
// returns its columns and rows. It connects over CNPG's internal URI (bex-api runs
// in-cluster next to the databases) inside a hard read-only envelope; writes, DDL,
// multi-statement escapes and over-long queries are rejected by Postgres itself.
func (s *Service) Query(ctx context.Context, dbID, sql string) (QueryResult, error) {
	if err := s.Authorize(ctx, core.RelCanViewSensitive); err != nil {
		return QueryResult{}, err
	}
	if strings.TrimSpace(sql) == "" {
		return QueryResult{}, fmt.Errorf("%w: sql is required", core.ErrBadRequest)
	}
	_, sec, err := s.loadAppSecret(ctx, core.RelCanViewSensitive, dbID)
	if err != nil {
		return QueryResult{}, err // core.ErrNotFound for an unknown/unprovisioned db
	}
	uri := string(sec.Data["uri"])
	if uri == "" {
		return QueryResult{}, core.ErrNotFound
	}
	return runReadOnlyQuery(ctx, uri, sql, queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap})
}

// runReadOnlyQuery dials connString and executes sql read-only. The read-only
// guarantee is layered: default_transaction_read_only=on at session startup, a
// server-side statement_timeout, and an explicit BEGIN READ ONLY transaction — so
// a SET transaction_read_write escape and multi-statement payloads both fail at
// the database, no SQL parsing required.
func runReadOnlyQuery(ctx context.Context, connString, sql string, lim queryLimits) (QueryResult, error) {
	cfg, err := pgx.ParseConfig(connString)
	if err != nil {
		return QueryResult{}, errQueryConnClose
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.RuntimeParams["statement_timeout"] = strconv.FormatInt(lim.statementTimeout.Milliseconds(), 10)

	// Bound the whole round-trip a little past the server-side timeout so a hung
	// dial or a server that ignores statement_timeout can't wedge the request.
	ctx, cancel := context.WithTimeout(ctx, lim.statementTimeout+5*time.Second)
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return QueryResult{}, errQueryConnClose
	}
	defer func() { _ = conn.Close(ctx) }()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
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
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = f.Name
	}
	out := QueryResult{Columns: cols, Rows: [][]any{}}
	for rows.Next() {
		if len(out.Rows) >= lim.rowCap {
			out.Truncated = true // a row beyond the cap exists; stop reading
			break
		}
		vals, err := rows.Values()
		if err != nil {
			return QueryResult{}, mapPGError(err)
		}
		out.Rows = append(out.Rows, vals)
	}
	// rows.Err surfaces read-only violations / timeouts that fire mid-stream.
	if err := rows.Err(); err != nil {
		return QueryResult{}, mapPGError(err)
	}
	out.RowCount = len(out.Rows)
	return out, nil
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
	return errQueryFailed
}
