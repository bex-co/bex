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
//
// Everything else — finite numbers (including exact Numeric digits), integers,
// text, bytea, finite times, NULL — keeps pgx's value and its existing encoding.
package postgres

import (
	"math"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

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
