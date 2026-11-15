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
	"strings"
	"testing"
)

// redactLiteralsFuzzSeeds are the SQL shapes the package's own redaction tests
// exercise — every literal spelling the lexer must recognize (plain, the E, U&,
// B, X and N prefixed forms, dollar-quoted and tagged, doubled and
// backslash-escaped quotes, truncated bodies) plus the constructs that must
// stay verbatim (comments, quoted identifiers, $n parameters) and a few
// malformed edges. Seeds live as
// source so a reviewer sees exactly what the untrusted surface is probed with;
// the on-disk testdata/fuzz cache is never committed.
func redactLiteralsFuzzSeeds(f *testing.F) {
	f.Helper()
	for _, seed := range []string{
		"",
		"'",
		"''",
		"'''",
		"\"",
		"\"\"",
		"$",
		"$$",
		"$$$$",
		"$a$",
		"$a$$a$",
		"E'",
		"E'\\'",
		"U&'",
		"B'",
		"X'",
		"N'",
		"--",
		"/*",
		"*/",
		"/*/*/",
		"/**/",
		"/* */ -- \n'x'",
		"ALTER ROLE \"dpg_x_user\" WITH PASSWORD 'SCRAM-SHA-256$4096:c2FsdA==$c3RvcmVk:c2VydmVy'",
		`create user u encrypted Password 'hunter2' valid until 'infinity'`,
		`ALTER ROLE u PASSWORD 'it''s' LOGIN`,
		`ALTER ROLE u PASSWORD E'a\'b' LOGIN`,
		`ALTER ROLE u PASSWORD $$s3cr3t$$`,
		`ALTER ROLE u PASSWORD $p$a$$b$p$ LOGIN`,
		`ALTER ROLE u WITH PASSWORD 'SCRAM-SHA-256$4096:c2Fs`,
		`CREATE USER MAPPING FOR u SERVER s OPTIONS (user 'a', password 'pw')`,
		`CREATE SUBSCRIPTION s CONNECTION 'host=h password=pw dbname=d' PUBLICATION p`,
		`ALTER ROLE a PASSWORD 'x'; ALTER ROLE b PASSWORD 'y'`,
		`ALTER ROLE u PASSWORD /*c*/ 'x'`,
		`ALTER ROLE u PASSWORD U&'x' LOGIN`,
		`CREATE SUBSCRIPTION s CONNECTION 'host=h password=''x'' dbname=d' PUBLICATION p`,
		`SELECT dblink_connect('postgresql://u:x@h/db')`,
		`ALTER ROLE u PASSWORD'x'`,
		`SELECT B'1010', X'1F', N'x'`,
		`CREATE FUNCTION f() RETURNS int AS $body$ SELECT 'k' $body$ LANGUAGE sql`,
		`SELECT * FROM users WHERE email = 'a@b.co' AND id = $1`,
		`ALTER ROLE u PASSWORD $é$s3cr3t$é$ LOGIN`,
		`SELECT date'2026-01-01'`,
		`SELECT E'a\\', 'x'`,
		`ALTER ROLE u PASSWORD 'pa\'ss' LOGIN`,
		"SELECT 1 -- don't\nFROM t WHERE p = 'x'",
		"SELECT 1 -- c\rAND pw = 'secret'",
		`SELECT password FROM users WHERE id = $1`,
		`UPDATE users SET password = $1 WHERE id = $2`,
		`ALTER ROLE u PASSWORD NULL`,
		`ALTER ROLE u CONNECTION LIMIT 5`,
		`CREATE DATABASE "app" OWNER "app_user"`,
		`CREATE EXTENSION IF NOT EXISTS pg_stat_statements`,
		`SELECT "it's" FROM t WHERE x = $1`,
		"SELECT 1 -- don't\nFROM t",
		`/* a /* it's */ 'still comment' */ SELECT $1`,
		`SELECT a$b$ FROM t$x$`,
		`SELECT name FROM e WHERE e.x = $1`,
	} {
		f.Add(seed)
	}
}

// FuzzRedactLiterals drives the hand-rolled SQL lexer that stands between a
// tenant's live query text and every Insights surface (REST/GraphQL/MCP): the
// `Processes`/`TopQueries` views publish pg_stat_activity and
// pg_stat_statements text after redactLiterals has replaced every string
// literal. The lexer walks raw, attacker-controlled bytes with manual index
// arithmetic (comments, quoted identifiers, every literal prefix, dollar tags),
// so it must never panic or fail to make progress. Two properties bound what it
// may publish: a body planted inside a plain literal must never survive, and
// publishedQuery must never report a masked row for text it publishes.
//
// Redaction is deliberately NOT asserted to be idempotent: an empty literal
// immediately followed by an unterminated bit-string literal redacts to two
// adjacent redaction markers, which SQL reads back as one doubled-quote
// literal, so a second pass collapses them. Nothing leaks either way — the
// caller applies it once — so that is a property of the marker, not a defect
// worth forcing the lexer to avoid.
func FuzzRedactLiterals(f *testing.F) {
	redactLiteralsFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, query string) {
		redacted := redactLiterals(query)
		// A body with no single quote cannot end the literal early, so the
		// whole of it is literal text and must be gone from the published form.
		if !strings.ContainsRune(query, '\'') {
			// The bexzz…zzbex wrapper makes the body unique to the literal, so a
			// short fuzz input ("A") cannot match the statement's own keywords.
			body := "bexzz" + query + "zzbex"
			planted := "ALTER ROLE u PASSWORD '" + body + "'"
			if out := redactLiterals(planted); strings.Contains(out, body) {
				t.Fatalf("literal body survived redaction: %q -> %q", planted, out)
			}
		}
		published, masked := publishedQuery(query)
		if query == pgInsufficientPrivilege {
			if !masked || published != "" {
				t.Fatalf("privilege placeholder published: masked=%v published=%q", masked, published)
			}
			return
		}
		if masked || published != redacted {
			t.Fatalf("publishedQuery disagrees with redactLiterals for %q: masked=%v published=%q redacted=%q", query, masked, published, redacted)
		}
	})
}
