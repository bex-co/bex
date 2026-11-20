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
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w3/083: migration 0316 moves every stored view's filter document to
// Render's shape, TEXT_SEARCH with CONTAINS and the time range as the
// top-level timeRange.relative, and its rollback restores the shape the
// earlier bex-api reads.
func TestSavedSearchDocumentMigrationMovesViewsToRenderShape(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	migration := func(path string) {
		t.Helper()
		sql, err := migrationsFS.ReadFile("migrations/" + path)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(sql))
	}
	exec(`CREATE SCHEMA migration_0316_saved_search_document;
 SET LOCAL search_path TO migration_0316_saved_search_document;
 CREATE TABLE projects (id text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());`)
	migration("0308_saved_searches.up.sql")
	exec(`INSERT INTO projects (id) VALUES ('prj-observed')`)

	view := func(id, kind, doc string) {
		t.Helper()
		exec(`INSERT INTO saved_searches (id, project_id, name, type, filters) VALUES ($1, 'prj-observed', $1, $2, $3::jsonb)`,
			id, kind, doc)
	}
	const (
		textAndRange = "svs-0000000000000000000a"
		rangeOnly    = "svs-0000000000000000000b"
		untouched    = "svs-0000000000000000000c"
		renderShaped = "svs-0000000000000000000d"
	)
	view(textAndRange, "APPLICATION_LOGS", `{"version": 1, "filters": [
		{"field": "LEVEL", "operator": "IS_NOT", "values": ["info"]},
		{"field": "TIME_RANGE", "operator": "IS", "values": ["4h"]},
		{"field": "TEXT", "operator": "IS", "values": ["timeout", "refused"]}]}`)
	view(rangeOnly, "METRICS", `{"version": 1, "filters": [{"field": "TIME_RANGE", "operator": "IS", "values": ["7d"]}]}`)
	view(untouched, "REQUEST_LOGS", `{"version": 1, "filters": [{"field": "PATH", "operator": "IS", "values": ["/api/*"]}]}`)
	view(renderShaped, "METRICS", `{"version": 1, "filters": [], "timeRange": {"relative": "24h"}, "display": {"requestGroupBy": "STATUS_CODE"}}`)

	docs := func() map[string]any {
		t.Helper()
		rows, err := tx.Query(ctx, `SELECT id, filters FROM saved_searches WHERE id = ANY($1)`,
			[]string{textAndRange, rangeOnly, untouched, renderShaped})
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]any{}
		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				t.Fatal(err)
			}
			var doc any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			out[id] = doc
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	want := func(stage string, got map[string]any, id, doc string) {
		t.Helper()
		var w any
		if err := json.Unmarshal([]byte(doc), &w); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got[id], w) {
			t.Errorf("%s: %s = %v, want %v", stage, id, got[id], w)
		}
	}
	defaultDoc := `{"filters": [], "version": 1}`

	migration("0316_saved_search_render_document.up.sql")
	got := docs()
	want("up", got, textAndRange, `{"version": 1, "filters": [
		{"field": "LEVEL", "operator": "IS_NOT", "values": ["info"]},
		{"field": "TEXT_SEARCH", "operator": "CONTAINS", "values": ["timeout", "refused"]}],
		"timeRange": {"relative": "4h"}}`)
	want("up", got, rangeOnly, `{"version": 1, "filters": [], "timeRange": {"relative": "7d"}}`)
	want("up", got, untouched, `{"version": 1, "filters": [{"field": "PATH", "operator": "IS", "values": ["/api/*"]}]}`)
	want("up", got, renderShaped, `{"version": 1, "filters": [], "timeRange": {"relative": "24h"}, "display": {"requestGroupBy": "STATUS_CODE"}}`)
	var defaults int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM saved_searches WHERE filters = $1::jsonb`, defaultDoc).Scan(&defaults); err != nil || defaults != 3 {
		t.Fatalf("the project's three default views read %d empty documents (%v), want 3 untouched", defaults, err)
	}

	migration("0316_saved_search_render_document.down.sql")
	got = docs()
	want("down", got, textAndRange, `{"version": 1, "filters": [
		{"field": "LEVEL", "operator": "IS_NOT", "values": ["info"]},
		{"field": "TEXT", "operator": "IS", "values": ["timeout", "refused"]},
		{"field": "TIME_RANGE", "operator": "IS", "values": ["4h"]}]}`)
	want("down", got, rangeOnly, `{"version": 1, "filters": [{"field": "TIME_RANGE", "operator": "IS", "values": ["7d"]}]}`)
	want("down", got, untouched, `{"version": 1, "filters": [{"field": "PATH", "operator": "IS", "values": ["/api/*"]}]}`)
	// The earlier bex-api refuses display, so the rollback drops it.
	want("down", got, renderShaped, `{"version": 1, "filters": [{"field": "TIME_RANGE", "operator": "IS", "values": ["24h"]}]}`)

	// Re-upgrading after the rollback is safe.
	migration("0316_saved_search_render_document.up.sql")
	got = docs()
	want("re-up", got, rangeOnly, `{"version": 1, "filters": [], "timeRange": {"relative": "7d"}}`)
	want("re-up", got, renderShaped, `{"version": 1, "filters": [], "timeRange": {"relative": "24h"}}`)
}
