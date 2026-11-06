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

package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/graphql-go/graphql/language/ast"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The API boundary refuses a request string no store can hold — invalid UTF-8
// or a NUL — before any resolver, handler, authorization check or store sees it
// (w4/519). Route by route those strings reached a Postgres bind or an OpenFGA
// check and came back as a 500, INTERNAL or "authorization service
// unavailable"; one check where arguments are read closes the class, including
// for fields not yet written. Stores keep their own textBindable guards for
// callers that do not come through here.

// withValidQueryText answers a REST request whose query string carries such a
// parameter name or value with 400 INVALID_TEXT naming the parameter. A query
// that does not parse is left to the route, as before.
func withValidQueryText(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, bad := invalidQueryText(r.URL.RawQuery); bad {
			core.WriteErr(w, core.NewInvalidTextError("query parameter", name))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func invalidQueryText(rawQuery string) (string, bool) {
	if rawQuery == "" {
		return "", false
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", false
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !core.ValidText(name) {
			return strings.ToValidUTF8(strings.ReplaceAll(name, "\x00", ""), ""), true
		}
		for _, v := range values[name] {
			if !core.ValidText(v) {
				return name, true
			}
		}
	}
	return "", false
}

// invalidGraphQLText names the first argument literal or variable of a parsed
// document whose string value is not core.ValidText, walking lists and input
// objects, directives, variable defaults and fragments.
func invalidGraphQLText(fragments map[string]*ast.FragmentDefinition, ops []*ast.OperationDefinition, variables map[string]any) (string, bool) {
	names := make([]string, 0, len(variables))
	for name := range variables {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !validTextValue(variables[name]) {
			return "$" + name, true
		}
	}
	for _, op := range ops {
		for _, def := range op.VariableDefinitions {
			if def.DefaultValue != nil && !validTextLiteral(def.DefaultValue) && def.Variable != nil && def.Variable.Name != nil {
				return "$" + def.Variable.Name.Value, true
			}
		}
		if name, bad := invalidDirectiveText(op.Directives); bad {
			return name, true
		}
		if name, bad := invalidSelectionText(op.SelectionSet); bad {
			return name, true
		}
	}
	for _, frag := range fragments {
		if name, bad := invalidDirectiveText(frag.Directives); bad {
			return name, true
		}
		if name, bad := invalidSelectionText(frag.SelectionSet); bad {
			return name, true
		}
	}
	return "", false
}

func invalidSelectionText(set *ast.SelectionSet) (string, bool) {
	if set == nil {
		return "", false
	}
	for _, sel := range set.Selections {
		switch s := sel.(type) {
		case *ast.Field:
			if name, bad := invalidArgumentText(s.Arguments); bad {
				return name, true
			}
			if name, bad := invalidDirectiveText(s.Directives); bad {
				return name, true
			}
			if name, bad := invalidSelectionText(s.SelectionSet); bad {
				return name, true
			}
		case *ast.InlineFragment:
			if name, bad := invalidDirectiveText(s.Directives); bad {
				return name, true
			}
			if name, bad := invalidSelectionText(s.SelectionSet); bad {
				return name, true
			}
		case *ast.FragmentSpread:
			if name, bad := invalidDirectiveText(s.Directives); bad {
				return name, true
			}
		}
	}
	return "", false
}

func invalidDirectiveText(directives []*ast.Directive) (string, bool) {
	for _, d := range directives {
		if name, bad := invalidArgumentText(d.Arguments); bad {
			return name, true
		}
	}
	return "", false
}

func invalidArgumentText(args []*ast.Argument) (string, bool) {
	for _, arg := range args {
		if !validTextLiteral(arg.Value) {
			name := ""
			if arg.Name != nil {
				name = arg.Name.Value
			}
			return name, true
		}
	}
	return "", false
}

func validTextLiteral(v ast.Value) bool {
	switch v := v.(type) {
	case *ast.StringValue:
		return core.ValidText(v.Value)
	case *ast.ListValue:
		return !slices.ContainsFunc(v.Values, func(item ast.Value) bool { return !validTextLiteral(item) })
	case *ast.ObjectValue:
		for _, f := range v.Fields {
			if !validTextLiteral(f.Value) {
				return false
			}
		}
	}
	return true
}

func validTextValue(v any) bool {
	switch v := v.(type) {
	case string:
		return core.ValidText(v)
	case []any:
		return !slices.ContainsFunc(v, func(item any) bool { return !validTextValue(item) })
	case map[string]any:
		for k, item := range v {
			if !core.ValidText(k) || !validTextValue(item) {
				return false
			}
		}
	}
	return true
}

// invalidMCPText names the first top-level tool argument whose string content
// (nested lists and objects included) is not core.ValidText. Arguments that do
// not decode are left to the tool's own decoding error.
func invalidMCPText(raw json.RawMessage) (string, bool) {
	var args map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &args) != nil {
		return "", false
	}
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if !core.ValidText(name) || !validTextValue(args[name]) {
			return strings.ToValidUTF8(strings.ReplaceAll(name, "\x00", ""), ""), true
		}
	}
	return "", false
}
