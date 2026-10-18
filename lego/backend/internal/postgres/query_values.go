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

// query_values.go is the one conversion policy between pgx's decoded cells and
// the JSON every query surface emits (w4/m157). collectQueryRows applies it to
// each row before the encoded-size budget, so REST and MCP encode, and GraphQL
// stringifies, the same values. pgx's default decoding is truthful for most
// types but not for these, each traced in the pinned v5.10.0 codecs:
//
//   - float4/float8 NaN and ±Infinity decode to Go floats encoding/json refuses,
//     failing the whole query → the tokens "NaN", "Infinity", "-Infinity".
//   - numeric ±Infinity decode to a pgtype.Numeric whose MarshalJSON writes 0,
//     and numeric NaN to a quoted "NaN" → the same three tokens as strings.
//   - date/timestamp/timestamptz ±infinity decode to pgtype.InfinityModifier, an
//     int8 that encodes as 1/-1 → PostgreSQL's own "infinity"/"-infinity".
//   - uuid decodes to a [16]byte that encodes as a number array → the canonical
//     lowercase hyphenated string.
//   - an array decodes to one flat []any, dropping its dimensions → re-decoded
//     through pgtype.Array[any], which keeps them, and nested to match to_json.
//   - json/jsonb unmarshal into any, rounding every number through float64 and
//     turning a JSON null or string into a Go nil or bare string → the
//     server's JSON text, kept by query-local codecs (registerQueryJSONCodecs).
//   - a finite date/timestamp/timestamptz outside years 0–9999 decodes to a
//     time.Time encoding/json refuses, failing the whole query → an ISO 8601
//     expanded-year string (expandedYearTime).
//
// Everything else — finite numbers (including exact Numeric digits), integers,
// text, bytea, year 0–9999 times, NULL — keeps pgx's value and its existing
// encoding.
package postgres

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// registerQueryJSONCodecs swaps the query connection's json and jsonb codecs
// for ones that decode a value bound for any as its own JSON text. The array
// types are registered again too: an ArrayCodec scans elements through the
// ElementType it was built with, not through the map, so json[] and jsonb[]
// would otherwise keep rounding. The connection serves one statement, so no
// other reader sees the change.
func registerQueryJSONCodecs(m *pgtype.Map) {
	jsonType := &pgtype.Type{Name: "json", OID: pgtype.JSONOID,
		Codec: &pgtype.JSONCodec{Marshal: json.Marshal, Unmarshal: unmarshalQueryJSON}}
	jsonbType := &pgtype.Type{Name: "jsonb", OID: pgtype.JSONBOID,
		Codec: &pgtype.JSONBCodec{Marshal: json.Marshal, Unmarshal: unmarshalQueryJSON}}
	m.RegisterType(jsonType)
	m.RegisterType(jsonbType)
	m.RegisterType(&pgtype.Type{Name: "_json", OID: pgtype.JSONArrayOID, Codec: &pgtype.ArrayCodec{ElementType: jsonType}})
	m.RegisterType(&pgtype.Type{Name: "_jsonb", OID: pgtype.JSONBArrayOID, Codec: &pgtype.ArrayCodec{ElementType: jsonbType}})
}

// unmarshalQueryJSON keeps JSON text bound for an any (a cell or an array
// element) as a json.RawMessage, so its numbers keep every digit and a JSON
// null or string stays JSON rather than becoming SQL NULL or bare text. The
// bytes are copied because pgx reuses the row buffer they point into.
func unmarshalQueryJSON(data []byte, v any) error {
	if p, ok := v.(*any); ok {
		*p = json.RawMessage(bytes.Clone(data))
		return nil
	}
	return json.Unmarshal(data, v)
}

// queryArrayColumns marks the result columns whose registered codec is pgx's
// ArrayCodec, decided by the field's type OID through the connection's own type
// map rather than by Go slice kind (bytea is a []byte, not an SQL array). An
// unregistered OID keeps pgx's default string/bytes decoding.
func queryArrayColumns(m *pgtype.Map, fields []pgconn.FieldDescription) []bool {
	arrays := make([]bool, len(fields))
	for i, f := range fields {
		if typ, ok := m.TypeForOID(f.DataTypeOID); ok {
			_, arrays[i] = typ.Codec.(*pgtype.ArrayCodec)
		}
	}
	return arrays
}

// decodeQueryArray decodes one non-NULL array cell keeping its dimensions:
// pgtype.Array[any] records them where the default []any target discards them.
// The raw cell was already bounded by collectQueryRows' byte budgets.
func decodeQueryArray(m *pgtype.Map, f pgconn.FieldDescription, raw []byte) (any, error) {
	var arr pgtype.Array[any]
	if err := m.Scan(f.DataTypeOID, f.Format, raw, &arr); err != nil {
		return nil, errQueryFailed
	}
	if !arr.Valid {
		return nil, nil
	}
	return nestQueryArray(arr.Elements, arr.Dims)
}

// nestQueryArray reshapes an array's row-major elements into nested slices,
// one level per dimension: a 2×2 int[] becomes [[1,2],[3,4]], one dimension
// stays a flat list and a zero-dimension (empty) array becomes []. JSON indexes
// are ordinal, so a custom lower bound is not represented — only the lengths
// and element order are. Every length must be positive and their product must
// equal the element count; checking that before nesting bounds the allocation
// by the elements already decoded.
func nestQueryArray(elems []any, dims []pgtype.ArrayDimension) (any, error) {
	if len(dims) == 0 {
		if len(elems) != 0 {
			return nil, errQueryFailed
		}
		return []any{}, nil
	}
	cardinality := 1
	for _, d := range dims {
		if d.Length <= 0 || int(d.Length) > len(elems) {
			return nil, errQueryFailed
		}
		cardinality *= int(d.Length)
		if cardinality > len(elems) {
			return nil, errQueryFailed
		}
	}
	if cardinality != len(elems) {
		return nil, errQueryFailed
	}
	return nestDims(elems, dims), nil
}

func nestDims(elems []any, dims []pgtype.ArrayDimension) []any {
	if len(dims) == 1 {
		return elems
	}
	n := int(dims[0].Length)
	stride := len(elems) / n
	out := make([]any, n)
	for i := range out {
		out[i] = nestDims(elems[i*stride:(i+1)*stride:(i+1)*stride], dims[1:])
	}
	return out
}

// normalizeQueryValue applies the policy above to one decoded value, recursing
// through (possibly nested) arrays in place. A temporal or numeric infinity
// marker outside the states the codecs define is refused rather than guessed.
func normalizeQueryValue(v any) (any, error) {
	switch v := v.(type) {
	case float64:
		if token, special := floatToken(v); special {
			return token, nil
		}
	case float32:
		if token, special := floatToken(float64(v)); special {
			return token, nil
		}
	case pgtype.Numeric:
		switch {
		case !v.Valid || v.InfinityModifier == pgtype.Finite && !v.NaN:
			return v, nil // exact digits via Numeric.MarshalJSON, or null
		case v.NaN:
			return "NaN", nil
		case v.InfinityModifier == pgtype.Infinity:
			return "Infinity", nil
		case v.InfinityModifier == pgtype.NegativeInfinity:
			return "-Infinity", nil
		}
		return nil, errQueryFailed
	case pgtype.InfinityModifier:
		switch v {
		case pgtype.Infinity:
			return "infinity", nil
		case pgtype.NegativeInfinity:
			return "-infinity", nil
		}
		return nil, errQueryFailed
	case [16]byte:
		return pgtype.UUID{Bytes: v, Valid: true}.String(), nil
	case time.Time:
		if y := v.Year(); y < 0 || y > 9999 {
			return expandedYearTime(v), nil
		}
	case []any:
		for i, elem := range v {
			n, err := normalizeQueryValue(elem)
			if err != nil {
				return nil, err
			}
			v[i] = n
		}
		return v, nil
	}
	return v, nil
}

// expandedYearTime spells a finite time whose year RFC 3339 cannot carry
// with ISO 8601's expanded year: a sign and at least six astronomical-year
// digits, so PostgreSQL's 0002 BC is -000001. The rest is the RFC3339Nano form
// year 0–9999 times already encode as — same fraction, same offset.
func expandedYearTime(t time.Time) string {
	sign, year := '+', t.Year()
	if year < 0 {
		sign, year = '-', -year
	}
	return fmt.Sprintf("%c%06d%s", sign, year, t.Format("-01-02T15:04:05.999999999Z07:00"))
}

// floatToken spells a non-finite float the way PostgreSQL does; a finite one
// reports false and stays the native number it already was.
func floatToken(v float64) (string, bool) {
	switch {
	case math.IsNaN(v):
		return "NaN", true
	case math.IsInf(v, 1):
		return "Infinity", true
	case math.IsInf(v, -1):
		return "-Infinity", true
	}
	return "", false
}
