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
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- Hermetic policy checks (w4/m157) -------------------------------------------

func TestNormalizeQueryValue(t *testing.T) {
	uuid := [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00}
	cases := []struct {
		name string
		in   any
		want string // JSON of the normalized value
	}{
		{"float8 NaN", math.NaN(), `"NaN"`},
		{"float8 +Inf", math.Inf(1), `"Infinity"`},
		{"float8 -Inf", math.Inf(-1), `"-Infinity"`},
		{"float4 NaN", float32(math.NaN()), `"NaN"`},
		{"float4 -Inf", float32(math.Inf(-1)), `"-Infinity"`},
		{"finite float4", float32(1.5), `1.5`},
		{"finite float8", 2.25, `2.25`},
		{"numeric NaN", pgtype.Numeric{NaN: true, Valid: true}, `"NaN"`},
		{"numeric +Inf", pgtype.Numeric{InfinityModifier: pgtype.Infinity, Valid: true}, `"Infinity"`},
		{"numeric -Inf", pgtype.Numeric{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, `"-Infinity"`},
		{"temporal +inf", pgtype.Infinity, `"infinity"`},
		{"temporal -inf", pgtype.NegativeInfinity, `"-infinity"`},
		{"int8 1 stays a number", int8(1), `1`},
		{"int64 -1 stays a number", int64(-1), `-1`},
		{"uuid", uuid, `"550e8400-e29b-41d4-a716-446655440000"`},
		{"bytea stays base64", []byte{0x00, 0x01, 0xff}, `"AAH/"`},
		{"nil", nil, `null`},
		{"nested leaves", []any{[]any{math.Inf(1), int32(1)}, []any{uuid, nil}}, `[["Infinity",1],["550e8400-e29b-41d4-a716-446655440000",null]]`},
	}
	for _, c := range cases {
		got, err := normalizeQueryValue(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		encoded, err := json.Marshal(got)
		if err != nil || string(encoded) != c.want {
			t.Errorf("%s => %s (%v), want %s", c.name, encoded, err, c.want)
		}
	}

	var exact pgtype.Numeric
	if err := exact.Scan("12345678901234567890.123456789"); err != nil {
		t.Fatal(err)
	}
	got, _ := normalizeQueryValue(exact)
	if encoded, _ := json.Marshal(got); string(encoded) != "12345678901234567890.123456789" {
		t.Errorf("finite numeric => %s, want its exact digits", encoded)
	}

	// Marker states the codecs never produce are refused, not guessed at.
	for _, bad := range []any{
		pgtype.Finite,
		pgtype.InfinityModifier(7),
		pgtype.Numeric{InfinityModifier: pgtype.InfinityModifier(7), Valid: true},
		[]any{pgtype.InfinityModifier(-7)},
	} {
		if _, err := normalizeQueryValue(bad); !errors.Is(err, errQueryFailed) {
			t.Errorf("normalize(%#v) => %v, want errQueryFailed", bad, err)
		}
	}
}

func TestNestQueryArray(t *testing.T) {
	dims := func(lengths ...int32) []pgtype.ArrayDimension {
		out := make([]pgtype.ArrayDimension, len(lengths))
		for i, l := range lengths {
			out[i] = pgtype.ArrayDimension{Length: l, LowerBound: 1}
		}
		return out
	}
	seq := func(n int) []any {
		out := make([]any, n)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	for _, c := range []struct {
		name  string
		elems []any
		dims  []pgtype.ArrayDimension
		want  string
	}{
		{"empty", nil, nil, `[]`},
		{"flat", seq(3), dims(3), `[1,2,3]`},
		{"matrix", seq(4), dims(2, 2), `[[1,2],[3,4]]`},
		{"cube", seq(8), dims(2, 2, 2), `[[[1,2],[3,4]],[[5,6],[7,8]]]`},
		{"2x3 row-major", seq(6), dims(2, 3), `[[1,2,3],[4,5,6]]`},
		// Custom lower bounds are not part of the JSON contract; ordinal order is.
		{"lower bound 0", seq(4), []pgtype.ArrayDimension{{Length: 2, LowerBound: 0}, {Length: 2, LowerBound: -3}}, `[[1,2],[3,4]]`},
	} {
		got, err := nestQueryArray(c.elems, c.dims)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if encoded, _ := json.Marshal(got); string(encoded) != c.want {
			t.Errorf("%s => %s, want %s", c.name, encoded, c.want)
		}
	}

	// Malformed shapes fail closed before allocating anything per dimension.
	for _, c := range []struct {
		name  string
		elems []any
		dims  []pgtype.ArrayDimension
	}{
		{"elements without dims", seq(1), nil},
		{"zero length", nil, dims(1000000, 0)},
		{"negative length", seq(2), dims(-2, -1)},
		{"short", seq(3), dims(2, 2)},
		{"long", seq(5), dims(2, 2)},
		{"overflowing product", seq(4), dims(math.MaxInt32, math.MaxInt32, math.MaxInt32)},
	} {
		if _, err := nestQueryArray(c.elems, c.dims); !errors.Is(err, errQueryFailed) {
			t.Errorf("%s => %v, want errQueryFailed", c.name, err)
		}
	}
}

// --- Real pgx decoding against BEX_TEST_DB_URI ----------------------------------

// typedValuesCase is one SELECT whose normalized rows must encode to want —
// the JSON REST and MCP write.
type typedValuesCase struct {
	name, sql, want string
}

var typedValuesCases = []typedValuesCase{
	{
		name: "special floats",
		sql: `SELECT 'NaN'::float8, 'Infinity'::float8, '-Infinity'::float8, 'NaN'::real, 'Infinity'::real, '-Infinity'::real,
			1.5::real, 2.25::float8, ARRAY['Infinity'::float8, 1.25, '-Infinity', 'NaN']`,
		want: `[["NaN","Infinity","-Infinity","NaN","Infinity","-Infinity",1.5,2.25,["Infinity",1.25,"-Infinity","NaN"]]]`,
	},
	{
		name: "special and exact numerics",
		sql: `SELECT 'Infinity'::numeric, '-Infinity'::numeric, 'NaN'::numeric, ARRAY['Infinity'::numeric,'-Infinity'::numeric,12.5::numeric],
			12345678901234567890.123456789::numeric, 0.12345678901234567890123456789::numeric, 9007199254740993::bigint,
			ARRAY[9007199254740993::bigint,-9007199254740993::bigint], NULL::numeric, 'Infinity'::numeric::text`,
		want: `[["Infinity","-Infinity","NaN",["Infinity","-Infinity",12.5],12345678901234567890.123456789,0.12345678901234567890123456789,` +
			`9007199254740993,[9007199254740993,-9007199254740993],null,"Infinity"]]`,
	},
	{
		name: "temporal infinities",
		sql: `SELECT 'infinity'::date, '-infinity'::date, 'infinity'::timestamp, '-infinity'::timestamp, 'infinity'::timestamptz, '-infinity'::timestamptz,
			ARRAY['infinity'::date, '-infinity'::date, '2024-01-02'::date], ARRAY[['infinity'::timestamp],['-infinity'::timestamp]],
			'2024-01-02'::date, NULL::date, 1::int8, -1::int4, 1::int2, 'infinity'::date::text`,
		want: `[["infinity","-infinity","infinity","-infinity","infinity","-infinity",["infinity","-infinity","2024-01-02T00:00:00Z"],` +
			`[["infinity"],["-infinity"]],"2024-01-02T00:00:00Z",null,1,-1,1,"infinity"]]`,
	},
	{
		name: "uuids",
		sql: `SELECT '550e8400-e29b-41d4-a716-446655440000'::uuid, ARRAY['550E8400-E29B-41D4-A716-446655440000'::uuid, NULL],
			ARRAY[['550e8400-e29b-41d4-a716-446655440000'::uuid],['00000000-0000-0000-0000-000000000001'::uuid]],
			NULL::uuid, decode('0001ff','hex'), ARRAY[85,14,132]::int[], '550e8400-e29b-41d4-a716-446655440000'::uuid::text`,
		want: `[["550e8400-e29b-41d4-a716-446655440000",["550e8400-e29b-41d4-a716-446655440000",null],` +
			`[["550e8400-e29b-41d4-a716-446655440000"],["00000000-0000-0000-0000-000000000001"]],` +
			`null,"AAH/",[85,14,132],"550e8400-e29b-41d4-a716-446655440000"]]`,
	},
	{
		name: "array dimensions",
		sql: `SELECT ARRAY[[1,2],[3,4]], ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]], ARRAY[['a',NULL],['NULL','b']],
			'{}'::int[], NULL::int[], ARRAY[1,NULL,3], ARRAY[decode('00','hex')], '[0:1][-1:0]={{1,2},{3,4}}'::int[],
			ARRAY[['NaN'::float8, 1],[2, 'Infinity'::float8]], ARRAY[[1,2,3],[4,5,6]]`,
		want: `[[[[1,2],[3,4]],[[[1,2],[3,4]],[[5,6],[7,8]]],[["a",null],["NULL","b"]],` +
			`[],null,[1,null,3],["AA=="],[[1,2],[3,4]],[["NaN",1],[2,"Infinity"]],[[1,2,3],[4,5,6]]]]`,
	},
	{
		// PostgreSQL's own to_json is the shape oracle for native arrays.
		name: "to_json controls",
		sql:  `SELECT to_json(ARRAY[[1,2],[3,4]]), to_json(ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]]), to_json(ARRAY[['a',NULL],['NULL','b']])`,
		want: `[[[[1,2],[3,4]],[[[1,2],[3,4]],[[5,6],[7,8]]],[["a",null],["NULL","b"]]]]`,
	},
}

func TestQueryTypedValuesIntegration(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()
	lim := queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap}
	for _, c := range typedValuesCases {
		res, err := runReadOnlyQuery(ctx, uri, c.sql, lim)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if encoded, _ := json.Marshal(res.Rows); string(encoded) != c.want {
			t.Errorf("%s\n got %s\nwant %s", c.name, encoded, c.want)
		}
	}

	// Finite timestamps keep pgx's time.Time (and so their existing RFC3339
	// rendering); only the infinity markers changed.
	res, err := runReadOnlyQuery(ctx, uri, `SELECT '2024-01-02 03:04:05.5+00'::timestamptz, '2024-01-02 03:04:05'::timestamp`, lim)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []time.Time{
		time.Date(2024, 1, 2, 3, 4, 5, 5e8, time.UTC),
		time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	} {
		if ts, ok := res.Rows[0][i].(time.Time); !ok || !ts.Equal(want) {
			t.Errorf("finite time cell %d = %#v, want %v", i, res.Rows[0][i], want)
		}
	}
}

// TestQueryTypedValuesWritableReturning covers the confirmed write path's
// RETURNING rows through the same conversion, on a throwaway local table.
func TestQueryTypedValuesWritableReturning(t *testing.T) {
	uri := testDBURI(t)
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, `DROP TABLE IF EXISTS q_typed; CREATE TABLE q_typed(f float8, n numeric, d date, u uuid, m int[])`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP TABLE IF EXISTS q_typed`) })

	res, err := runSQLQuery(ctx, uri,
		`INSERT INTO q_typed VALUES ('-Infinity', 'NaN', 'infinity', '550e8400-e29b-41d4-a716-446655440000', '{{1,2},{3,4}}') RETURNING *`,
		queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap}, false)
	if err != nil {
		t.Fatal(err)
	}
	if encoded, _ := json.Marshal(res.Rows); string(encoded) != `[["-Infinity","NaN","infinity","550e8400-e29b-41d4-a716-446655440000",[[1,2],[3,4]]]]` {
		t.Fatalf("RETURNING rows = %s", encoded)
	}
	if res.RowCount != 1 {
		t.Fatalf("RETURNING rowCount = %d", res.RowCount)
	}
}

// TestQueryTypedValuesAcrossSurfaces runs one mixed SELECT through REST,
// GraphQL and MCP on a real database: REST/MCP carry the JSON values, GraphQL
// the bare tokens as string cells and nested arrays as JSON text.
func TestQueryTypedValuesAcrossSurfaces(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "typed-db", uri)
	const sql = `SELECT 'NaN'::float8 AS f, '-Infinity'::numeric AS n, 'infinity'::date AS d, ` +
		`'550e8400-e29b-41d4-a716-446655440000'::uuid AS u, ARRAY[[1,2],[3,4]] AS m, 12345678901234567890.123456789::numeric AS x, NULL::int AS z`
	const wantRows = `[["NaN","-Infinity","infinity","550e8400-e29b-41d4-a716-446655440000",[[1,2],[3,4]],12345678901234567890.123456789,null]]`

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body, _ := json.Marshal(map[string]string{"sql": sql})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/postgres/typed-db/query", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"rows":`+wantRows) {
		t.Fatalf("REST => %d %s", rec.Code, rec.Body)
	}

	args, _ := json.Marshal(map[string]string{"postgresId": "typed-db", "sql": sql})
	res := mcpQueryWire(t, svc, string(args))
	if got := mcpQueryText(t, res); !strings.Contains(got, `"rows":`+wantRows) {
		t.Fatalf("MCP => %s", got)
	}

	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString:  `mutation($sql: String!) { executeDatabaseQuery(id:"typed-db", sql:$sql) { rows { values } } }`,
		VariableValues: map[string]any{"sql": sql}})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL: %v", gql.Errors)
	}
	cells, _ := json.Marshal(gql.Data.(map[string]any)["executeDatabaseQuery"].(map[string]any)["rows"])
	if want := `[{"values":["NaN","-Infinity","infinity","550e8400-e29b-41d4-a716-446655440000","[[1,2],[3,4]]","12345678901234567890.123456789",null]}]`; string(cells) != want {
		t.Fatalf("GraphQL cells\n got %s\nwant %s", cells, want)
	}
}
