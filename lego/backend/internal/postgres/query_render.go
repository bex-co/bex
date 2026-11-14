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

package postgres

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
)

// query_render.go is query_render_postgres' answer in Render's text
// (render-oss/render-mcp-server pkg/postgres/tools.go, collectRows and
// truncatedResultMessage; w4/m378): a JSON array of row objects keyed by
// column name, bounded by 5 MB of that JSON rather than by a row count, and
// past the bound the rows that fit plus a notice — never a refusal. Values
// still go through bex's exact value policy (query_values.go), so a bigint or
// numeric keeps every digit where Render's raw pgx values would not.

// renderQueryResultBytes is Render's maxQueryResultBytes: the most JSON one
// query_render_postgres answer holds.
const renderQueryResultBytes = 5 << 20

// renderRowJSON encodes one row as Render does: a map keyed by column name, so
// encoding/json writes the keys sorted and a repeated column name keeps its
// last column's value.
func renderRowJSON(columns []string, vals []any) ([]byte, error) {
	row := make(map[string]any, len(columns))
	for i, col := range columns {
		row[col] = vals[i]
	}
	body, err := json.Marshal(row)
	if err != nil {
		return nil, errQueryFailed
	}
	return body, nil
}

// renderRowArray is Render's size check on the growing JSON array: a row fits
// only if the array so far, the row, a separating comma and the closing
// bracket stay within the limit (the comma is counted even for the first row,
// as Render counts it).
type renderRowArray struct{ used, rows, limit int }

func newRenderRowArray(limit int) renderRowArray { return renderRowArray{used: 1, limit: limit} }

// add reports whether a row of n encoded bytes fits, and counts it if so.
func (a *renderRowArray) add(n int) bool {
	if a.used+n+2 > a.limit {
		return false
	}
	if a.rows > 0 {
		a.used++
	}
	a.used += n
	a.rows++
	return true
}

// collectRenderRows is collectQueryRows under Render's limits: it keeps rows
// until the next one's JSON would pass lim.renderJSONBytes, then stops and
// marks the result truncated. A DataRow over the wire cap (sized to the same
// budget) is a row that alone could never fit, so it ends the result the same
// way instead of failing it.
func collectRenderRows(rows pgx.Rows, lim queryLimits, out *QueryResult) error {
	typeMap := rows.Conn().TypeMap()
	fields := rows.FieldDescriptions()
	arrays := queryArrayColumns(typeMap, fields)
	array := newRenderRowArray(lim.renderJSONBytes)
	for rows.Next() {
		vals, err := queryRowValues(rows, typeMap, fields, arrays)
		if err != nil {
			return err
		}
		row, err := renderRowJSON(out.Columns, vals)
		if err != nil {
			return err
		}
		if !array.add(len(row)) {
			out.Truncated = true
			return nil
		}
		out.Rows = append(out.Rows, vals)
	}
	if err := rows.Err(); err != nil {
		var bodyLenErr *pgproto3.ExceededMaxBodyLenErr
		if errors.As(err, &bodyLenErr) {
			out.Truncated = true
			return nil
		}
		return mapPGError(err)
	}
	return nil
}

// renderQueryRows is query_render_postgres' first text item: the rows as
// Render's JSON array of objects, cut before it would pass 5 MB. It returns
// how many rows it kept and whether it cut any.
func renderQueryRows(columns []string, rows [][]any) (body []byte, kept int, cut bool, err error) {
	array := newRenderRowArray(renderQueryResultBytes)
	body = []byte{'['}
	for _, vals := range rows {
		row, err := renderRowJSON(columns, vals)
		if err != nil {
			return nil, 0, false, err
		}
		if !array.add(len(row)) {
			return append(body, ']'), kept, true, nil
		}
		if kept > 0 {
			body = append(body, ',')
		}
		body = append(body, row...)
		kept++
	}
	return append(body, ']'), kept, false, nil
}

// renderTruncatedMessage is Render's truncatedResultMessage, the second text
// item of a cut answer, worded for how many rows were kept.
func renderTruncatedMessage(rowCount int) string {
	if rowCount == 0 {
		return fmt.Sprintf("Results truncated: the first row alone is over %d MB of JSON, so no rows are shown. "+
			"Select fewer columns, or trim large values in SQL, for example with left(column, 1000).",
			renderQueryResultBytes>>20)
	}
	return fmt.Sprintf("Results truncated: showing the first %d rows. "+
		"This tool returns at most %d MB of JSON per query. "+
		"To see the rest, rerun the query with an ORDER BY plus LIMIT and OFFSET, select fewer columns, or aggregate in SQL.",
		rowCount, renderQueryResultBytes>>20)
}
