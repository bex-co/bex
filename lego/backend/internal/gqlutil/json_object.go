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

package gqlutil

import (
	"encoding/json"
	"strconv"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// JSONObject is a JSON object value, the type Render's private GraphQL gives a
// saved search's `filters` and a log query's `filterDocument` (w3/m505). As
// output it serializes a map or raw JSON object as is; as input it takes an
// object, from a variable or an inline literal. The receiving verb validates
// the object's shape.
var JSONObject = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSONObject",
	Description: "A JSON object.",
	Serialize: func(value any) any {
		switch v := value.(type) {
		case json.RawMessage:
			var out map[string]any
			if json.Unmarshal(v, &out) != nil {
				return nil
			}
			return out
		case map[string]any:
			return v
		}
		return nil
	},
	ParseValue: func(value any) any {
		if v, ok := value.(map[string]any); ok {
			return v
		}
		return nil
	},
	ParseLiteral: func(valueAST ast.Value) any {
		if obj, ok := valueAST.(*ast.ObjectValue); ok {
			return literalValue(obj)
		}
		return nil
	},
})

// literalValue is an inline GraphQL literal as the JSON value it spells.
// Variables inside a literal are not resolved: graphql-go hands the scalar no
// variable values, so one reads as null.
func literalValue(v ast.Value) any {
	switch v := v.(type) {
	case *ast.ObjectValue:
		out := make(map[string]any, len(v.Fields))
		for _, f := range v.Fields {
			out[f.Name.Value] = literalValue(f.Value)
		}
		return out
	case *ast.ListValue:
		out := make([]any, 0, len(v.Values))
		for _, item := range v.Values {
			out = append(out, literalValue(item))
		}
		return out
	case *ast.StringValue:
		return v.Value
	case *ast.EnumValue:
		return v.Value
	case *ast.BooleanValue:
		return v.Value
	case *ast.IntValue:
		if n, err := strconv.ParseInt(v.Value, 10, 64); err == nil {
			return n
		}
	case *ast.FloatValue:
		if n, err := strconv.ParseFloat(v.Value, 64); err == nil {
			return n
		}
	}
	return nil
}
