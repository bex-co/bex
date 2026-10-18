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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		// Years RFC 3339 cannot carry use ISO 8601's expanded form; 0–9999 keep
		// time.Time's own encoding, boundaries included.
		{"year 10000", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), `"+010000-01-01T00:00:00Z"`},
		{"2 BC keeps its fraction", time.Date(-1, 1, 1, 12, 34, 56, 123456000, time.UTC), `"-000001-01-01T12:34:56.123456Z"`},
		{"expanded keeps the offset", time.Date(10000, 1, 1, 12, 0, 0, 0, time.FixedZone("", 5*3600+1800)), `"+010000-01-01T12:00:00+05:30"`},
		{"seven-digit year", time.Date(5874897, 12, 31, 0, 0, 0, 0, time.UTC), `"+5874897-12-31T00:00:00Z"`},
		{"4713 BC", time.Date(-4712, 1, 1, 0, 0, 0, 0, time.UTC), `"-004712-01-01T00:00:00Z"`},
		{"year 0 stays RFC 3339", time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), `"0000-01-01T00:00:00Z"`},
		{"year 9999 stays RFC 3339", time.Date(9999, 12, 31, 23, 59, 59, 999999000, time.UTC), `"9999-12-31T23:59:59.999999Z"`},
		{"expanded inside arrays", []any{[]any{time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), nil}}, `[["+010000-01-01T00:00:00Z",null]]`},
		{"json text passes through", json.RawMessage(`{"n":9007199254740993}`), `{"n":9007199254740993}`},
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

// TestQueryJSONCodecs decodes json/jsonb through the query-local codecs in
// both wire formats, scalar and array element, without a server.
func TestQueryJSONCodecs(t *testing.T) {
	m := pgtype.NewMap()
	registerQueryJSONCodecs(m)
	const object = `{"n": 9007199254740993, "amount": 12345678901234567890.123456789}`
	decode := func(oid uint32, format int16, src []byte) any {
		t.Helper()
		typ, ok := m.TypeForOID(oid)
		if !ok {
			t.Fatalf("oid %d not registered", oid)
		}
		v, err := typ.Codec.DecodeValue(m, oid, format, src)
		if err != nil {
			t.Fatalf("decode oid %d format %d: %v", oid, format, err)
		}
		return v
	}
	for _, c := range []struct {
		name   string
		oid    uint32
		format int16
		src    []byte
		want   string
	}{
		{"jsonb text", pgtype.JSONBOID, pgtype.TextFormatCode, []byte(object), object},
		{"jsonb binary drops its version byte", pgtype.JSONBOID, pgtype.BinaryFormatCode, append([]byte{1}, object...), object},
		{"json text", pgtype.JSONOID, pgtype.TextFormatCode, []byte(`1.10`), `1.10`},
		{"json binary", pgtype.JSONOID, pgtype.BinaryFormatCode, []byte(`"qa 雪"`), `"qa 雪"`},
		{"jsonb null is JSON", pgtype.JSONBOID, pgtype.TextFormatCode, []byte(`null`), `null`},
	} {
		src := bytes.Clone(c.src)
		got, ok := decode(c.oid, c.format, src).(json.RawMessage)
		if !ok || string(got) != c.want {
			t.Errorf("%s => %#v, want RawMessage %s", c.name, got, c.want)
		}
		// pgx reuses the row buffer: the decoded value must not alias it.
		for i := range src {
			src[i] = 'x'
		}
		if string(got) != c.want {
			t.Errorf("%s aliases the row buffer: %s", c.name, got)
		}
	}
	if v := decode(pgtype.JSONBOID, pgtype.TextFormatCode, nil); v != nil {
		t.Errorf("SQL NULL jsonb => %#v, want nil", v)
	}
	typ, _ := m.TypeForOID(pgtype.JSONBOID)
	if _, err := typ.Codec.DecodeValue(m, pgtype.JSONBOID, pgtype.BinaryFormatCode, []byte{2, '1'}); err == nil {
		t.Error("unknown jsonb version byte decoded; want pgx's refusal")
	}

	// Array elements scan through the array codec's own ElementType, so the
	// re-registered _json/_jsonb types are what keep their digits.
	for _, oid := range []uint32{pgtype.JSONArrayOID, pgtype.JSONBArrayOID} {
		var arr pgtype.Array[any]
		// PostgreSQL spells an SQL NULL element as bare NULL; "null" is JSON.
		src := []byte(`{"{\"n\": 9007199254740993}",NULL,"null","\"s\""}`)
		if err := m.Scan(oid, pgtype.TextFormatCode, src, &arr); err != nil {
			t.Fatalf("scan array oid %d: %v", oid, err)
		}
		if encoded, _ := json.Marshal(arr.Elements); string(encoded) != `[{"n":9007199254740993},null,null,"s"]` {
			t.Errorf("array oid %d elements => %s", oid, encoded)
		}
		if _, ok := arr.Elements[2].(json.RawMessage); !ok || arr.Elements[1] != nil {
			t.Errorf("array oid %d: JSON null %#v must stay JSON, SQL NULL %#v nil", oid, arr.Elements[2], arr.Elements[1])
		}
	}
}

// TestQueryCellStringJSON pins GraphQL's string cells for JSON values: compact
// JSON text without HTML escapes, scalar or in an array, never SQL NULL.
func TestQueryCellStringJSON(t *testing.T) {
	for _, c := range []struct {
		in   any
		want any
	}{
		{json.RawMessage(`{"n": 9007199254740993, "a": [1.10, "<&>"]}`), `{"n":9007199254740993,"a":[1.10,"<&>"]}`},
		{json.RawMessage(`null`), "null"},
		{json.RawMessage(`"qa 雪"`), `"qa 雪"`},
		{nil, nil},
		{[]any{json.RawMessage(`"<&>"`), nil}, `["<&>",null]`},
	} {
		if got := queryCellString(c.in); got != c.want {
			t.Errorf("queryCellString(%#v) = %#v, want %#v", c.in, got, c.want)
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
		// jsonb text output reorders keys (shorter first) and json keeps the
		// literal text; both keep every digit. JSON null and SQL NULL both
		// encode as null on REST/MCP — GraphQL is where they differ.
		name: "json values",
		sql: `SELECT '{"n":9007199254740993,"amount":12345678901234567890.123456789}'::jsonb, 'null'::jsonb, '"qa 雪"'::jsonb,
			NULL::jsonb, '{"n":9007199254740993}'::json, '1.10'::json,
			ARRAY['{"n":9007199254740993}'::jsonb,'null'::jsonb,'"qa 雪"'::jsonb,NULL::jsonb], ARRAY[['1.10'::json],['[2]'::json]],
			9007199254740993::bigint, 12345678901234567890.123456789::numeric, 1.5::float8, false, '{"n":9007199254740993}'::jsonb::text`,
		want: `[[{"n":9007199254740993,"amount":12345678901234567890.123456789},null,"qa 雪",null,{"n":9007199254740993},1.10,` +
			`[{"n":9007199254740993},null,"qa 雪",null],[[1.10],[[2]]],` +
			`9007199254740993,12345678901234567890.123456789,1.5,false,"{\"n\": 9007199254740993}"]]`,
	},
	{
		// Finite years outside 0–9999 as expanded strings; the boundaries keep
		// their RFC 3339 form.
		name: "expanded years",
		sql: `SELECT DATE '10000-01-01', DATE '0002-01-01 BC', TIMESTAMP '10000-01-01 12:34:56.123456',
			ARRAY[DATE '9999-12-31', DATE '10000-01-01', NULL], ARRAY[[TIMESTAMP '0002-01-01 00:00:00.5 BC'],[TIMESTAMP '9999-12-31 23:59:59.999999']],
			DATE '0001-01-01 BC', DATE '0001-01-01', DATE '5874897-12-31', DATE '4713-01-01 BC', 'infinity'::date, NULL::date`,
		want: `[["+010000-01-01T00:00:00Z","-000001-01-01T00:00:00Z","+010000-01-01T12:34:56.123456Z",` +
			`["9999-12-31T00:00:00Z","+010000-01-01T00:00:00Z",null],[["-000001-01-01T00:00:00.5Z"],["9999-12-31T23:59:59.999999Z"]],` +
			`"0000-01-01T00:00:00Z","0001-01-01T00:00:00Z","+5874897-12-31T00:00:00Z","-004712-01-01T00:00:00Z","infinity",null]]`,
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

	// timestamptz decodes in the process's zone, as year 0–9999 values always
	// have. TestNormalizeQueryValue pins the formatter's strings; this checks
	// the decoded instant and zone reach it.
	res, err = runReadOnlyQuery(ctx, uri, `SELECT TIMESTAMPTZ '10000-01-01 12:34:56.123456+00'`, lim)
	if err != nil {
		t.Fatal(err)
	}
	want := expandedYearTime(time.Date(10000, 1, 1, 12, 34, 56, 123456000, time.UTC).Local())
	if res.Rows[0][0] != want {
		t.Errorf("far timestamptz = %#v, want %q", res.Rows[0][0], want)
	}

	// JSON cells are budgeted as encoded, not as received: twenty ~200 KB
	// jsonb strings fit every raw cap, but each '<' encodes as the six bytes
	// \u003c, so that result passes the encoded cap; the same bytes of 'x' do not.
	for ch, wantErr := range map[string]bool{"<": true, "x": false} {
		_, err := runReadOnlyQuery(ctx, uri, fmt.Sprintf(`SELECT to_jsonb(repeat('%s', 200000)) FROM generate_series(1, 20)`, ch), lim)
		if gotErr := errors.Is(err, errQueryResultTooLarge); gotErr != wantErr || (!wantErr && err != nil) {
			t.Errorf("jsonb of %q => %v, want too-large %v", ch, err, wantErr)
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
	if _, err := admin.Exec(ctx, `DROP TABLE IF EXISTS q_typed; CREATE TABLE q_typed(f float8, n numeric, d date, u uuid, m int[], j jsonb, y date)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP TABLE IF EXISTS q_typed`) })

	res, err := runSQLQuery(ctx, uri,
		`INSERT INTO q_typed VALUES ('-Infinity', 'NaN', 'infinity', '550e8400-e29b-41d4-a716-446655440000', '{{1,2},{3,4}}',
			'{"n":9007199254740993}', '10000-01-01') RETURNING *`,
		queryLimits{statementTimeout: queryStatementTimeout, rowCap: queryRowCap}, false)
	if err != nil {
		t.Fatal(err)
	}
	if encoded, _ := json.Marshal(res.Rows); string(encoded) != `[["-Infinity","NaN","infinity","550e8400-e29b-41d4-a716-446655440000",[[1,2],[3,4]],`+
		`{"n":9007199254740993},"+010000-01-01T00:00:00Z"]]` {
		t.Fatalf("RETURNING rows = %s", encoded)
	}
	if res.RowCount != 1 {
		t.Fatalf("RETURNING rowCount = %d", res.RowCount)
	}
}

// TestQueryTypedValuesAcrossSurfaces runs mixed SELECTs through REST, GraphQL
// and MCP on a real database: REST/MCP carry the JSON values, GraphQL the bare
// tokens as string cells and nested arrays and JSON as JSON text.
func TestQueryTypedValuesAcrossSurfaces(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "typed-db", uri)
	assertQueryAcrossSurfaces(t, svc, "typed-db",
		`SELECT 'NaN'::float8 AS f, '-Infinity'::numeric AS n, 'infinity'::date AS d, `+
			`'550e8400-e29b-41d4-a716-446655440000'::uuid AS u, ARRAY[[1,2],[3,4]] AS m, 12345678901234567890.123456789::numeric AS x, NULL::int AS z`,
		`[["NaN","-Infinity","infinity","550e8400-e29b-41d4-a716-446655440000",[[1,2],[3,4]],12345678901234567890.123456789,null]]`,
		`[{"values":["NaN","-Infinity","infinity","550e8400-e29b-41d4-a716-446655440000","[[1,2],[3,4]]","12345678901234567890.123456789",null]}]`)

	// JSON keeps every digit; on GraphQL a JSON null and string stay JSON text,
	// distinct from SQL NULL. Far years are plain strings everywhere.
	assertQueryAcrossSurfaces(t, svc, "typed-db",
		`SELECT '{"n":9007199254740993,"amount":12345678901234567890.123456789}'::jsonb AS o, 'null'::jsonb AS jn, `+
			`'"qa 雪"'::jsonb AS js, NULL::jsonb AS sn, ARRAY['{"n":9007199254740993}'::jsonb, NULL] AS ja, `+
			`DATE '10000-01-01' AS fd, ARRAY[DATE '0002-01-01 BC'] AS bc`,
		`[[{"n":9007199254740993,"amount":12345678901234567890.123456789},null,"qa 雪",null,`+
			`[{"n":9007199254740993},null],"+010000-01-01T00:00:00Z",["-000001-01-01T00:00:00Z"]]]`,
		`[{"values":["{\"n\":9007199254740993,\"amount\":12345678901234567890.123456789}","null","\"qa 雪\"",null,`+
			`"[{\"n\":9007199254740993},null]","+010000-01-01T00:00:00Z","[\"-000001-01-01T00:00:00Z\"]"]}]`)
}

// assertQueryAcrossSurfaces runs sql against the seeded database id through
// REST and MCP (whose rows must contain wantRows) and GraphQL (whose rows must
// encode as wantCells).
func assertQueryAcrossSurfaces(t *testing.T, svc *Service, id, sql, wantRows, wantCells string) {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body, _ := json.Marshal(map[string]string{"sql": sql})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/postgres/"+id+"/query", strings.NewReader(string(body))))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"rows":`+wantRows) {
		t.Fatalf("REST => %d %s", rec.Code, rec.Body)
	}

	args, _ := json.Marshal(map[string]string{"postgresId": id, "sql": sql})
	res := mcpQueryWire(t, svc, string(args))
	if got := mcpQueryText(t, res); !strings.Contains(got, `"rows":`+wantRows) {
		t.Fatalf("MCP => %s", got)
	}

	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString:  `mutation($id: String!, $sql: String!) { executeDatabaseQuery(id:$id, sql:$sql) { rows { values } } }`,
		VariableValues: map[string]any{"id": id, "sql": sql}})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL: %v", gql.Errors)
	}
	cells, _ := json.Marshal(gql.Data.(map[string]any)["executeDatabaseQuery"].(map[string]any)["rows"])
	if string(cells) != wantCells {
		t.Fatalf("GraphQL cells\n got %s\nwant %s", cells, wantCells)
	}
}
