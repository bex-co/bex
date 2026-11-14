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
// read-only query — none echo caller-supplied SQL.  ParameterSpec reads the
// declared Database.spec.parameters, which is read-only since parameterOverrides
// was retired (w2/066); the operator keeps projecting any existing set to the
// CNPG Cluster's spec.postgresql.parameters.
package postgres

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pgInsufficientPrivilege is PostgreSQL's own placeholder for a query text the
// caller may not read. pg_stat_activity.query and pg_stat_statements.query
// substitute it verbatim for backends owned by another role when the caller
// holds neither pg_monitor nor superuser (PostgreSQL 9.6+, unchanged through
// 18 — src/backend/utils/adt/pgstatfuncs.c). The operator now grants the
// database owner pg_monitor (w4/m115), so this should be unreachable for an
// owner-scoped read — but a role we do not control, or a future least-privilege
// tightening, can still produce it, and it must never reach a surface as if it
// were query text.
const pgInsufficientPrivilege = "<insufficient privilege>"

// redactedLiteral replaces every string literal in published query text. It
// stays a quoted SQL literal so the statement still reads as SQL, and is
// visibly distinct from the privilege-masked path (which publishes nothing).
const redactedLiteral = "'<redacted>'"

// publishedQuery returns the query text Insights may publish, and whether
// PostgreSQL masked it. A privilege placeholder publishes nothing (Masked); any
// other text publishes with its literals redacted. pg_stat_statements does not
// normalize utility statements, so the provisioning ALTER ROLE … PASSWORD
// 'SCRAM-SHA-256$…' sits there verbatim from a database's first minute
// (w4/m167) — a verifier no reader, however privileged, should see.
func publishedQuery(q string) (string, bool) {
	if q == pgInsufficientPrivilege {
		return "", true
	}
	return redactLiterals(q), false
}

// redactLiterals replaces every string literal in q with redactedLiteral, so a
// secret never survives whatever syntax carries it: a PASSWORD clause, a
// conninfo string, a connection URI (w5/075). pg_stat_statements already turns
// DML constants into $n, so top queries lose nothing; live query text loses
// its literal values, which a monitoring view need not show.
//
// Literals are '…' with an optional E, U&, B, X or N prefix, and dollar-quoted
// $tag$…$tag$ bodies. Comments (-- and nested /* */) and "quoted identifiers"
// are copied as they are, so a quote inside one starts nothing, and a
// positional parameter ($1) is not a literal. A literal the reader truncated
// runs to the end and is redacted to it.
func redactLiterals(q string) string {
	var b strings.Builder
	copied := 0 // q[:copied] is already in b
	for i := 0; i < len(q); {
		switch {
		case strings.HasPrefix(q[i:], "--"):
			i += lineCommentLen(q[i:])
		case strings.HasPrefix(q[i:], "/*"):
			i += blockCommentLen(q[i:])
		case q[i] == '"':
			i += quotedLen(q[i:], 1, '"', false)
		default:
			n := literalLen(q, i)
			if n == 0 {
				i++
				continue
			}
			if b.Len() == 0 {
				b.Grow(len(q))
			}
			b.WriteString(q[copied:i])
			b.WriteString(redactedLiteral)
			i += n
			copied = i
		}
	}
	if b.Len() == 0 {
		return q // no literal: the common case, nothing copied
	}
	b.WriteString(q[copied:])
	return b.String()
}

// literalLen returns the byte length of the string literal starting at q[i],
// prefix included, or 0 when none starts there. A prefix letter or a $ that
// continues an identifier (date'…', name$x) opens nothing; a bare quote always
// opens a literal, even right after a keyword (PASSWORD'x').
func literalLen(q string, i int) int {
	s := q[i:]
	if s[0] == '\'' {
		return plainLiteralLen(s, 1)
	}
	if i > 0 && identifierByte(q[i-1]) {
		return 0
	}
	switch {
	case len(s) > 1 && (s[0] == 'E' || s[0] == 'e') && s[1] == '\'':
		return quotedLen(s, 2, '\'', true)
	case len(s) > 2 && (s[0] == 'U' || s[0] == 'u') && s[1] == '&' && s[2] == '\'':
		return plainLiteralLen(s, 3)
	case len(s) > 1 && strings.IndexByte("BbXxNn", s[0]) >= 0 && s[1] == '\'':
		return plainLiteralLen(s, 2)
	}
	if tag := dollarTagLen(s); tag > 0 {
		if end := strings.Index(s[tag:], s[:tag]); end >= 0 {
			return tag + end + tag
		}
		return len(s)
	}
	return 0
}

// plainLiteralLen returns the length of the non-escape '…' literal whose body
// starts at offset start of s. A backslash before a quote is text under the
// default standard_conforming_strings and escapes the quote with it off. The
// text does not say which the session used, so when the two readings disagree
// the rest of s is taken as the literal: over-redacting, never leaking.
func plainLiteralLen(s string, start int) int {
	if n := quotedLen(s, start, '\'', false); n == quotedLen(s, start, '\'', true) {
		return n
	}
	return len(s)
}

// dollarTagLen returns the length of the dollar-quote opener at s ($$ or
// $tag$), or 0 when s does not start one. A tag starts with a letter, _ or
// non-ASCII byte and continues with those or digits, so a positional parameter
// ($1) is not one.
func dollarTagLen(s string) int {
	if s[0] != '$' {
		return 0
	}
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == '$':
			return i + 1
		case !identifierByte(c), i == 1 && '0' <= c && c <= '9':
			return 0
		}
	}
	return 0
}

// identifierByte reports whether c can continue an unquoted identifier: a
// letter, digit, _ or $, or any byte of a multibyte (non-ASCII) character.
func identifierByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		('0' <= c && c <= '9') || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

// quotedLen returns the length of the quoted token at s, whose body starts at
// offset start and ends at the closing quote. A doubled quote always escapes a
// quote; with backslash set (an E'…' literal), a backslash escapes the next
// byte. An unterminated token runs to the end of s.
func quotedLen(s string, start int, quote byte, backslash bool) int {
	for i := start; i < len(s); i++ {
		switch {
		case backslash && s[i] == '\\':
			i++
		case s[i] == quote:
			if i+1 < len(s) && s[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(s)
}

// lineCommentLen returns the length of the -- comment at s, which a newline
// or carriage return ends.
func lineCommentLen(s string) int {
	if end := strings.IndexAny(s, "\r\n"); end >= 0 {
		return end + 1
	}
	return len(s)
}

// blockCommentLen returns the length of the /* */ comment at s. PostgreSQL
// nests them; an unterminated one runs to the end of s.
func blockCommentLen(s string) int {
	depth := 0
	for i := 0; i+1 < len(s); i++ {
		switch {
		case s[i] == '/' && s[i+1] == '*':
			depth++
			i++
		case s[i] == '*' && s[i+1] == '/':
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}

// ProcessView is one row from pg_stat_activity (live backend process).
type ProcessView struct {
	PID             int32  `json:"pid"`
	UserName        string `json:"userName"`
	ApplicationName string `json:"applicationName"`
	State           string `json:"state"`
	Query           string `json:"query,omitempty"`
	// Masked is true when PostgreSQL hid this row's query text from the reading
	// role. Query is empty in that case; consumers explain rather than render.
	Masked          bool   `json:"masked,omitempty"`
	WaitEventType   string `json:"waitEventType,omitempty"`
	WaitEvent       string `json:"waitEvent,omitempty"`
	DurationSeconds int32  `json:"durationSeconds"`
	DatabaseName    string `json:"databaseName,omitempty"`
	// ClientAddr is empty for a connection over a local socket.
	ClientAddr string `json:"clientAddr,omitempty"`
	// BackendStart and QueryStart are RFC 3339 UTC; QueryStart is empty for a
	// connection that has not run a statement yet.
	BackendStart string `json:"backendStart,omitempty"`
	QueryStart   string `json:"queryStart,omitempty"`
	// IsLeader is Render's: the process runs on the primary of a highly
	// available database. False on a single instance.
	IsLeader bool `json:"isLeader,omitempty"`
}

// TopQueryView is one row from pg_stat_statements (aggregated by query text).
type TopQueryView struct {
	Query string `json:"query"`
	// Masked is true when PostgreSQL hid this row's query text from the reading
	// role. Query is empty in that case; the statistics beside it stay real.
	Masked         bool    `json:"masked,omitempty"`
	Calls          int64   `json:"calls"`
	TotalTimeMs    float64 `json:"totalTimeMs"`
	MeanTimeMs     float64 `json:"meanTimeMs"`
	Rows           int64   `json:"rows"`
	SharedHitBlks  int64   `json:"sharedHitBlks"`
	SharedReadBlks int64   `json:"sharedReadBlks"`
	// QueryID is pg_stat_statements.queryid as a decimal string, as Render
	// types it.
	QueryID           string  `json:"queryId,omitempty"`
	MinTimeMs         float64 `json:"minTimeMs"`
	MaxTimeMs         float64 `json:"maxTimeMs"`
	StddevTimeMs      float64 `json:"stddevTimeMs"`
	SharedDirtiedBlks int64   `json:"sharedDirtiedBlks"`
	SharedWrittenBlks int64   `json:"sharedWrittenBlks"`
	LocalHitBlks      int64   `json:"localHitBlks"`
	LocalReadBlks     int64   `json:"localReadBlks"`
	LocalDirtiedBlks  int64   `json:"localDirtiedBlks"`
	LocalWrittenBlks  int64   `json:"localWrittenBlks"`
	TempReadBlks      int64   `json:"tempReadBlks"`
	TempWrittenBlks   int64   `json:"tempWrittenBlks"`
}

// DatabaseSizeView is the overall database size.
type DatabaseSizeView struct {
	Name       string `json:"name"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizePretty string `json:"sizePretty"`
}

// TableSizeView is one table's size without its indexes (pg_table_size: heap,
// TOAST, free space and visibility maps), so a table's row and its indexes'
// rows add up rather than count the indexes twice (w4/634). Render documents
// no table measure; its sizes endpoint lists each index as a row of its own.
type TableSizeView struct {
	Schema     string `json:"schema"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizePretty string `json:"sizePretty"`
}

// IndexSizeView is one user index's size (pg_relation_size).
type IndexSizeView struct {
	Schema     string `json:"schema"`
	Table      string `json:"table"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"sizeBytes"`
	SizePretty string `json:"sizePretty"`
}

// SizesView bundles the database-level size and the 50 largest tables and the
// 50 largest indexes.
type SizesView struct {
	Database DatabaseSizeView `json:"database"`
	Tables   []TableSizeView  `json:"tables"`
	Indexes  []IndexSizeView  `json:"indexes"`
}

// TableScanView is one row from pg_stat_user_tables.
type TableScanView struct {
	Database      string `json:"database"`
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
	// Observed means pg_settings reported this name, not that Value was applied.
	ObservationStatus string  `json:"observationStatus"`
	ObservedSetting   *string `json:"observedSetting"`
	ObservedUnit      *string `json:"observedUnit"`
}

// insightsTag marks every statement Insights runs as the owner
// (runAuthorizedInsight), so Top queries and Active processes can leave bex's
// own introspection out (w4/557). It goes after the leading SELECT, not before
// it: PostgreSQL 18's pg_stat_statements drops a statement's leading comment
// from the text it stores, and keeps one inside it. The stored text is the
// first one seen for a query id, which ignores comments, so a database that
// ran these statements untagged keeps those rows until its statistics reset.
const insightsTag = "/* bex:insights */"

// insightsEnvelopeStatements are the two statements every read-only round trip
// sends around its SELECT (runSQLQuery's BeginTx and Rollback; the SQL console
// runs the same envelope). pg_stat_statements records them under the connecting
// role, which is the customer's by design, and a comment cannot tag a
// statement that has no SELECT, so insightsTag never matches them: on a quiet
// database Top queries listed the panel's own `begin read only` and `rollback`
// (one row per role it connected as) instead of its empty state (w21/058). They
// are left out by their exact text, the spelling pgx sends; the statements carry
// no SQL, so no customer information is hidden, and a transaction-control
// statement an application sends in another spelling (`BEGIN`, `COMMIT`,
// `ROLLBACK`) still shows.
const insightsEnvelopeStatements = `'begin read only', 'rollback'`

// tagInsight places insightsTag after sql's leading SELECT; every Insights
// statement is one.
func tagInsight(sql string) string {
	body := strings.TrimLeft(sql, " \t\n")
	return "SELECT " + insightsTag + strings.TrimPrefix(body, "SELECT")
}

// reservedRoleSQL is v1alpha1.ReservedPostgresRole as a SQL predicate on col:
// the roles the platform itself connects as — CloudNativePG's instance manager
// (`postgres`), its metrics exporter (`cnpg_metrics_exporter`), streaming
// replication and the PgBouncer pooler's login. w4/m115 projected the owner
// into pg_monitor so their processes and statements stop reading "Hidden",
// which also exposes these — constant exporter scrapes that otherwise outrank
// the customer's own work (w21/026, w4/557).
func reservedRoleSQL(col string) string {
	quoted := make([]string, len(appv1alpha1.ReservedPostgresRoleNames))
	for i, name := range appv1alpha1.ReservedPostgresRoleNames {
		quoted[i] = "'" + name + "'"
	}
	clauses := []string{col + " IN (" + strings.Join(quoted, ", ") + ")"}
	for _, prefix := range appv1alpha1.ReservedPostgresRolePrefixes {
		clauses = append(clauses, "starts_with("+col+", '"+prefix+"')")
	}
	return "(" + strings.Join(clauses, " OR ") + ")"
}

// rfc3339SQL formats the timestamptz column col as RFC 3339 UTC text, or as
// an empty string when it is NULL.
func rfc3339SQL(col string) string {
	return "COALESCE(to_char(" + col + ` AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), '')`
}

var (
	sqlProcesses = `
SELECT pid::int,
       COALESCE(usename, '')             AS username,
       COALESCE(application_name, '')    AS appname,
       COALESCE(state, '')               AS state,
       COALESCE(left(query, 200), '')    AS query,
       COALESCE(wait_event_type, '')     AS wait_event_type,
       COALESCE(wait_event, '')          AS wait_event,
       COALESCE(extract(epoch FROM clock_timestamp() - query_start)::int, 0) AS duration_sec,
       COALESCE(datname, '')             AS datname,
       COALESCE(host(client_addr), '')   AS client_addr,
       ` + rfc3339SQL("backend_start") + ` AS backend_start,
       ` + rfc3339SQL("query_start") + ` AS query_start,
       NOT pg_is_in_recovery()           AS on_primary
FROM pg_stat_activity
WHERE pid <> pg_backend_pid()
  AND backend_type = 'client backend'
  AND NOT ` + reservedRoleSQL("COALESCE(usename, '')") + `
  AND COALESCE(query, '') NOT LIKE '%` + insightsTag + `%'
ORDER BY state, duration_sec DESC NULLS LAST`

	sqlTopQueries = `
SELECT COALESCE(left(query, 500), '')   AS query,
       calls,
       total_exec_time                  AS total_ms,
       mean_exec_time                   AS mean_ms,
       rows,
       shared_blks_hit                  AS hit_blks,
       shared_blks_read                 AS read_blks,
       COALESCE(queryid::text, '')      AS queryid,
       min_exec_time, max_exec_time, stddev_exec_time,
       shared_blks_dirtied, shared_blks_written,
       local_blks_hit, local_blks_read, local_blks_dirtied, local_blks_written,
       temp_blks_read, temp_blks_written
FROM pg_stat_statements
WHERE userid NOT IN (SELECT oid FROM pg_roles WHERE ` + reservedRoleSQL("rolname") + `)
  AND COALESCE(query, '') NOT LIKE '%` + insightsTag + `%'
  AND COALESCE(query, '') NOT IN (` + insightsEnvelopeStatements + `)
ORDER BY total_exec_time DESC
LIMIT 25`

	sqlDatabaseSize = `
SELECT current_database()                                    AS dbname,
       pg_database_size(current_database())                  AS size_bytes,
       pg_size_pretty(pg_database_size(current_database()))  AS size_pretty`

	sqlTableSizes = `
SELECT schemaname                                                                    AS schema,
       tablename                                                                     AS tbl,
       pg_table_size(quote_ident(schemaname)||'.'||quote_ident(tablename))           AS size_bytes,
       pg_size_pretty(pg_table_size(quote_ident(schemaname)||'.'||quote_ident(tablename))) AS size_pretty
FROM pg_tables
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
ORDER BY size_bytes DESC
LIMIT 50`

	sqlIndexSizes = `
SELECT schemaname                                    AS schema,
       relname                                       AS tbl,
       indexrelname                                  AS idx,
       pg_relation_size(indexrelid)                  AS size_bytes,
       pg_size_pretty(pg_relation_size(indexrelid))  AS size_pretty
FROM pg_stat_user_indexes
ORDER BY size_bytes DESC
LIMIT 50`

	sqlTableScans = `
SELECT current_database()                AS dbname,
       schemaname                        AS schema,
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

// insightStatements are every statement Insights runs as the owner. One run
// untagged, before w4/557, left its untagged text in pg_stat_statements under a
// query id the tag does not change, so the tagged runs since add to that row and
// Top queries leaves those query ids out too (w5/184).
var insightStatements = []string{sqlProcesses, sqlTopQueries, sqlDatabaseSize, sqlTableSizes, sqlIndexSizes, sqlTableScans, sqlParameterOverrides}

// insightQueryIDTTL bounds how long a database's Insights query ids are reused:
// they depend on its catalog, which a restore or a major upgrade replaces.
const insightQueryIDTTL = time.Hour

type insightQueryIDs struct {
	ids []int64
	at  time.Time
}

// insightQueryIDCache holds each database's Insights query ids, so Top queries
// asks for them once an hour rather than on every read.
var insightQueryIDCache sync.Map

var explainQueryID = regexp.MustCompile(`Query Identifier: (-?[0-9]+)`)

// insightQueryIDSQL is a NOT IN clause on queryid for Insights' own statements,
// read from EXPLAIN, which reports the query id pg_stat_statements files a
// statement under (PostgreSQL computes it when pg_stat_statements is loaded).
// Empty when the database reports none, as without pg_stat_statements.
func insightQueryIDSQL(ctx context.Context, db *appv1alpha1.Database, uri string) string {
	key := string(db.UID) + "/" + db.Namespace + "/" + db.Name
	if v, ok := insightQueryIDCache.Load(key); ok && time.Since(v.(insightQueryIDs).at) < insightQueryIDTTL {
		return queryIDClause(v.(insightQueryIDs).ids)
	}
	var ids []int64
	for _, sql := range insightStatements {
		res, err := runReadOnlyQuery(ctx, uri, "EXPLAIN (VERBOSE) "+tagInsight(sql), queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap})
		if err != nil {
			continue
		}
		for _, row := range res.Rows {
			if len(row) == 0 {
				continue
			}
			if m := explainQueryID.FindStringSubmatch(strVal(row[0])); m != nil {
				if id, err := strconv.ParseInt(m[1], 10, 64); err == nil {
					ids = append(ids, id)
				}
				break
			}
		}
	}
	insightQueryIDCache.Store(key, insightQueryIDs{ids: ids, at: time.Now()})
	return queryIDClause(ids)
}

func queryIDClause(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	list := make([]string, len(ids))
	for i, id := range ids {
		list[i] = strconv.FormatInt(id, 10)
	}
	return "\n  AND queryid NOT IN (" + strings.Join(list, ", ") + ")"
}

// runInsight dials the database and executes sql inside the standard read-only
// envelope (same safety rails as Query). The caller maps rows to its typed view.
func (s *Service) runInsight(ctx context.Context, relation, dbID, sql string) (QueryResult, error) {
	db, err := s.insightDatabase(ctx, relation, dbID)
	if err != nil {
		return QueryResult{}, err
	}
	return s.runAuthorizedInsight(ctx, db, sql, false)
}

// insightDatabase loads a database for an insight and authorizes relation on
// it uncached. Processes/TopQueries (and every other insight that dials the
// tenant database) surface live query text. RelCanViewSensitive and RelCanView
// are both read relations, so Authorize uses the decision cache; re-check
// uncached before loading the connection Secret (codex round-15 #4).
func (s *Service) insightDatabase(ctx context.Context, relation, dbID string) (*appv1alpha1.Database, error) {
	db, err := s.fetchDatabaseForRead(ctx, relation, dbID)
	if err != nil {
		return nil, err
	}
	if err := s.AuthorizeDatabaseFresh(ctx, relation, db); err != nil {
		return nil, err
	}
	return db, nil
}

// requireQueryable refuses SQL on a database that cannot run it right now,
// naming why, as the dashboard console does (w4/177): a suspended database has
// no compute, and one being created, upgraded or restarted accepts no
// connections yet. Without it the dial fails and the caller reads a redacted
// 500 "internal error" (w3/060).
func requireQueryable(db *appv1alpha1.Database) error {
	if db.Spec.Suspended {
		return fmt.Errorf("%w: database is suspended; resume it to run SQL", core.ErrUnavailable)
	}
	state := map[string]string{
		core.DatastoreCreating:      "still being created",
		"upgrading":                 "upgrading",
		core.DatastoreConfigRestart: "restarting",
	}[dbStatus(db)]
	if state == "" {
		return nil
	}
	return fmt.Errorf("%w: database is %s; run SQL once it is available", core.ErrUnavailable, state)
}

// runAuthorizedInsight runs sql on a database the caller already authorized,
// tagged as Insights' own (insightsTag).
// serverSettings dials without the session rails, which only the pg_settings
// read wants (queryLimits.serverSettings).
func (s *Service) runAuthorizedInsight(ctx context.Context, db *appv1alpha1.Database, sql string, serverSettings bool) (QueryResult, error) {
	if err := requireQueryable(db); err != nil {
		return QueryResult{}, err
	}
	uri, err := s.databaseURI(ctx, db)
	if err != nil {
		return QueryResult{}, err
	}
	return runReadOnlyQuery(ctx, uri, tagInsight(sql), queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap,
		serverSettings: serverSettings})
}

// databaseURI is the connection string an insight dials as the owner.
func (s *Service) databaseURI(ctx context.Context, db *appv1alpha1.Database) (string, error) {
	sec, err := s.databaseSecret(ctx, db)
	if err != nil {
		return "", err
	}
	uri := string(sec.Data["uri"])
	if uri == "" {
		return "", core.ErrNotFound
	}
	return uri, nil
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
	db, err := s.insightDatabase(ctx, core.RelCanViewSensitive, dbID)
	if err != nil {
		return nil, err
	}
	res, err := s.runAuthorizedInsight(ctx, db, sqlProcesses, false)
	if err != nil {
		return nil, err
	}
	return processViews(res.Rows, db.Status.HighAvailabilityEnabled), nil
}

// processViews maps sqlProcesses rows onto ProcessView, folding PostgreSQL's
// privilege placeholder into the Masked flag. Split out from Processes so the
// mapping — masking included — is testable without a live database. A row is
// the leader's when the database is highly available and the backend runs on
// the primary, the instance an insight dials.
func processViews(rows [][]any, highlyAvailable bool) []ProcessView {
	out := make([]ProcessView, 0, len(rows))
	for _, row := range rows {
		if len(row) < 13 {
			continue
		}
		onPrimary, _ := row[12].(bool)
		query, masked := publishedQuery(strVal(row[4]))
		out = append(out, ProcessView{
			PID:             int32(intVal(row[0])),
			UserName:        strVal(row[1]),
			ApplicationName: strVal(row[2]),
			State:           strVal(row[3]),
			Query:           query,
			Masked:          masked,
			WaitEventType:   strVal(row[5]),
			WaitEvent:       strVal(row[6]),
			DurationSeconds: int32(intVal(row[7])),
			DatabaseName:    strVal(row[8]),
			ClientAddr:      strVal(row[9]),
			BackendStart:    strVal(row[10]),
			QueryStart:      strVal(row[11]),
			IsLeader:        highlyAvailable && onPrimary,
		})
	}
	return out
}

// TopQueries returns the top 25 queries by total execution time from
// pg_stat_statements. Unavailable statistics must not look like an empty result.
// Requires RelCanViewSensitive because query texts may contain literal values.
func (s *Service) TopQueries(ctx context.Context, dbID string) ([]TopQueryView, error) {
	db, err := s.insightDatabase(ctx, core.RelCanViewSensitive, dbID)
	if err != nil {
		return nil, err
	}
	// Ahead of the read: its own failure text below would hide the reason.
	if err := requireQueryable(db); err != nil {
		return nil, err
	}
	sql := sqlTopQueries
	if uri, err := s.databaseURI(ctx, db); err == nil {
		sql = strings.Replace(sql, "\nORDER BY total_exec_time", insightQueryIDSQL(ctx, db, uri)+"\nORDER BY total_exec_time", 1)
	}
	res, err := s.runAuthorizedInsight(ctx, db, sql, false)
	if err != nil {
		return nil, fmt.Errorf("%w: top query statistics could not be read; the database must be reachable with pg_stat_statements enabled", core.ErrUnavailable)
	}
	return topQueryViews(res.Rows), nil
}

// topQueryViews maps sqlTopQueries rows onto TopQueryView, folding PostgreSQL's
// privilege placeholder into the Masked flag. The statistics beside a masked
// query text are real and stay.
func topQueryViews(rows [][]any) []TopQueryView {
	out := make([]TopQueryView, 0, len(rows))
	for _, row := range rows {
		if len(row) < 19 {
			continue
		}
		query, masked := publishedQuery(strVal(row[0]))
		out = append(out, TopQueryView{
			Query:             query,
			Masked:            masked,
			Calls:             intVal(row[1]),
			TotalTimeMs:       floatVal(row[2]),
			MeanTimeMs:        floatVal(row[3]),
			Rows:              intVal(row[4]),
			SharedHitBlks:     intVal(row[5]),
			SharedReadBlks:    intVal(row[6]),
			QueryID:           strVal(row[7]),
			MinTimeMs:         floatVal(row[8]),
			MaxTimeMs:         floatVal(row[9]),
			StddevTimeMs:      floatVal(row[10]),
			SharedDirtiedBlks: intVal(row[11]),
			SharedWrittenBlks: intVal(row[12]),
			LocalHitBlks:      intVal(row[13]),
			LocalReadBlks:     intVal(row[14]),
			LocalDirtiedBlks:  intVal(row[15]),
			LocalWrittenBlks:  intVal(row[16]),
			TempReadBlks:      intVal(row[17]),
			TempWrittenBlks:   intVal(row[18]),
		})
	}
	return out
}

// Sizes returns the database size and per-table and per-index sizes via
// pg_database_size / pg_table_size / pg_relation_size.
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
	idxRes, err := s.runInsight(ctx, core.RelCanView, dbID, sqlIndexSizes)
	if err != nil {
		return SizesView{}, err
	}
	indexes := make([]IndexSizeView, 0, len(idxRes.Rows))
	for _, row := range idxRes.Rows {
		if len(row) < 5 {
			continue
		}
		indexes = append(indexes, IndexSizeView{
			Schema:     strVal(row[0]),
			Table:      strVal(row[1]),
			Name:       strVal(row[2]),
			SizeBytes:  intVal(row[3]),
			SizePretty: strVal(row[4]),
		})
	}
	return SizesView{Database: dbSize, Tables: tables, Indexes: indexes}, nil
}

// TableScans returns sequential vs index scan stats from pg_stat_user_tables.
func (s *Service) TableScans(ctx context.Context, dbID string) ([]TableScanView, error) {
	res, err := s.runInsight(ctx, core.RelCanView, dbID, sqlTableScans)
	if err != nil {
		return nil, err
	}
	out := make([]TableScanView, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 9 {
			continue
		}
		out = append(out, TableScanView{
			Database:      strVal(row[0]),
			Schema:        strVal(row[1]),
			Name:          strVal(row[2]),
			SeqScans:      intVal(row[3]),
			SeqScanRows:   intVal(row[4]),
			IndexScans:    intVal(row[5]),
			IndexScanRows: intVal(row[6]),
			LiveRows:      intVal(row[7]),
			DeadRows:      intVal(row[8]),
		})
	}
	return out, nil
}

// ParameterOverrides returns non-default postgresql.conf parameters from pg_settings.
func (s *Service) ParameterOverrides(ctx context.Context, dbID string) ([]ParameterOverrideView, error) {
	db, err := s.insightDatabase(ctx, core.RelCanView, dbID)
	if err != nil {
		return nil, err
	}
	res, err := s.runAuthorizedInsight(ctx, db, sqlParameterOverrides, true)
	if err != nil {
		return nil, err
	}
	return parameterOverrideViews(res), nil
}

func parameterOverrideViews(res QueryResult) []ParameterOverrideView {
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
	return out
}

// ParameterSpec returns the tenant's DECLARED parameter overrides
// (Database.spec.parameters) as a stable name-sorted list — the read every
// surface exposes. GetParameterSpec below returns the same data as a map for
// callers that want one.
//
// Read-only: no API writes this set since parameterOverrides was retired
// (w2/066), but a database that already declares parameters keeps applying
// them. Distinct from ParameterOverrides, the observed pg_settings config,
// which is mostly the operator's (w6/m133).
func (s *Service) ParameterSpec(ctx context.Context, dbID string) ([]ParameterSpecView, error) {
	db, err := s.fetchDatabaseForRead(ctx, core.RelCanView, dbID)
	if err != nil {
		return nil, err
	}
	if len(db.Spec.Parameters) == 0 {
		return []ParameterSpecView{}, nil
	}
	// Authorization failures are not runtime diagnostics. Re-check before
	// loading credentials, exactly as the observed-parameter endpoint does.
	if err := s.AuthorizeDatabaseFresh(ctx, core.RelCanView, db); err != nil {
		return nil, err
	}
	// A restarting or unreachable server must not hold the declaration
	// hostage to the normal insight query's longer timeout.
	observationCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	observed, observationErr := s.runAuthorizedInsight(observationCtx, db, sqlParameterOverrides, true)
	return parameterSpecViews(db.Spec.Parameters, parameterOverrideViews(observed), observationErr), nil
}

// parameterSpecViews matches declarations to observations case-insensitively:
// PostgreSQL parameter names are, so a declared `timezone` is the runtime's
// `TimeZone` (w4/317). Only the lookup folds; each declaration keeps its
// spelling and value.
func parameterSpecViews(params map[string]string, observed []ParameterOverrideView, observationErr error) []ParameterSpecView {
	byName := make(map[string]ParameterOverrideView, len(observed))
	for _, row := range observed {
		byName[strings.ToLower(row.Name)] = row
	}
	out := make([]ParameterSpecView, 0, len(params))
	for name, value := range params {
		row := ParameterSpecView{Name: name, Value: value, ObservationStatus: "not_observed"}
		if observationErr != nil {
			row.ObservationStatus = "unavailable"
		} else if current, ok := byName[strings.ToLower(name)]; ok {
			row.ObservationStatus = "observed"
			row.ObservedSetting = &current.Setting
			if current.Unit != "" {
				row.ObservedUnit = &current.Unit
			}
		}
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b ParameterSpecView) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// GetParameterSpec returns the currently stored parameter overrides from the
// Database CR (spec.parameters), not from pg_settings. Use ParameterOverrides for
// the live database view.
func (s *Service) GetParameterSpec(ctx context.Context, dbID string) (map[string]string, error) {
	d, err := s.fetchDatabaseForRead(ctx, core.RelCanView, dbID)
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
