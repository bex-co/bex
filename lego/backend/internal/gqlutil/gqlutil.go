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

// Package gqlutil holds the presentation helpers the feature GraphQL fragments
// share: a typed resolver adapter and the common id argument. It imports only
// graphql-go and core's wire types — a shared presentation utility, not domain
// logic.
package gqlutil

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Field adapts a typed projection into a GraphQL resolver: it type-asserts the
// source and applies f, resolving nil for a foreign source. One helper for every
// object type (AppView, PostgresView, connection info, logs, API keys).
func Field[T any](f func(T) any) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		v, ok := p.Source.(T)
		if !ok {
			return nil, nil
		}
		return f(v), nil
	}
}

// IDArg is the `(id: String!)` argument shared by the single-resource queries and
// mutations (server(id), database(id), suspendService(id), ...). A fresh map per
// call so graphql-go never shares argument state across fields.
func IDArg() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
	}
}

// EnvVarInputType is the shared `{key, value|generateValue}` input object used by both
// the secrets feature's setEnvVars mutation and the apps feature's createService
// mutation (w5/m19). Defined once here so the composed schema never has duplicate
// type names — graphql-go rejects duplicates at schema-build time.
var EnvVarInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvVarInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"key":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"generateValue": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
	},
})

// IPAllowEntryType / IPAllowEntryInputType are the shared {cidrBlock,
// description} allow-list entry shape (Render's cidrBlockAndDescription,
// core.IPAllowListEntry) used by the ipAllowListEntries fields/arguments the
// postgres, keyvalue, and environments fragments expose (w4/m24). Defined once
// here so the composed schema never has duplicate type names. The legacy
// string-list ipAllowList fields/arguments stay — these extend them with the
// description-carrying form.
var IPAllowEntryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "IPAllowListEntry",
	Fields: graphql.Fields{
		"cidrBlock":   &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: Field(func(e core.IPAllowListEntry) any { return e.CIDRBlock })},
		"description": &graphql.Field{Type: graphql.String, Resolve: Field(func(e core.IPAllowListEntry) any { return e.Description })},
	},
})

var IPAllowEntryInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "IPAllowListEntryInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"cidrBlock":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"description": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// AllowList coerces an `[IPAllowListEntryInput!]` argument value ([]any of
// maps from graphql-go) into core entries. Nil or absent => nil, so callers
// can distinguish "argument omitted" from an explicit empty list.
func AllowList(arg any) []core.IPAllowListEntry {
	raw, ok := arg.([]any)
	if !ok {
		return nil
	}
	out := make([]core.IPAllowListEntry, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		e := core.IPAllowListEntry{}
		e.CIDRBlock, _ = m["cidrBlock"].(string)
		e.Description, _ = m["description"].(string)
		out = append(out, e)
	}
	return out
}

// Str reads an optional string argument, "" when absent — graphql-go omits an
// unset optional arg from the args map entirely, so a plain map index can't
// distinguish "absent" from an explicit empty string; this can. The one
// resolver-arg helper every feature's GraphQL fragment shared as a verbatim
// copy before graduating here (w10/m2).
func Str(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// StrPtr reads an optional string argument as a nil-able pointer — nil when
// absent, distinct from an explicit empty string. The pointer-returning sibling
// of Str, for verbs (like a rename or a PATCH-shaped update) that must tell
// "omitted" apart from "clear it". Graduated alongside Str (w10/m2): both
// apps/graphql.go and registrycreds/graphql.go carried an identical copy.
func StrPtr(args map[string]any, key string) *string {
	v, ok := args[key].(string)
	if !ok {
		return nil
	}
	return &v
}

// BoolPtr reads an optional Boolean argument as a tri-state *bool — nil when
// absent, so the caller can leave a field unchanged rather than forcing it
// false. StrPtr's Boolean-typed sibling; graduated here (w4/m30) after
// apps/graphql.go and environments/graphql.go each carried an identical
// `gqlBoolPtr` copy, the same duplication threshold StrPtr itself graduated
// on (w10/m2).
func BoolPtr(args map[string]any, key string) *bool {
	if v, ok := args[key].(bool); ok {
		return &v
	}
	return nil
}

// Page applies Render's cursor-pagination contract to a list resolver's result:
// it reads the shared `cursor`/`limit` arguments, defaults an absent limit to
// core.DefaultPageLimit, clamps a supplied one through core.PageLimit, and hands
// the window to core.StablePage. cursorOf yields each item's opaque cursor.
//
// The `requested` flag StablePage takes is deliberately `cursorSet || limitSet`
// rather than a constant: a caller that named NEITHER argument gets the whole
// list back unpaged (the pre-pagination behavior every list query shipped
// with), while naming either one opts into the windowed contract.
//
// Graduated here (w10/m2's threshold, the same one Str and StrPtr graduated on)
// after six verbatim copies of this eight-line block accumulated across the
// apps (services, customDomains), environments, projects, postgres, and
// keyvalue fragments — enough that the defaulting rule could drift per feature
// without anything failing.
func Page[T any](p graphql.ResolveParams, items []T, cursorOf func(T) string) []T {
	cursor, cursorSet := p.Args["cursor"].(string)
	limit, limitSet := p.Args["limit"].(int)
	if !limitSet {
		limit = core.DefaultPageLimit
	} else {
		limit = core.PageLimit(limit)
	}
	return core.StablePage(items, cursor, limit, cursorSet || limitSet, cursorOf)
}

// StringList coerces a `[String]` argument value ([]any from graphql-go) into
// []string, skipping non-string entries. Nil or absent => nil. Shared by the
// CIDR-allowlist arguments (setDatabaseIpAllowList, setKeyValueIpAllowList,
// create seeds).
func StringList(arg any) []string {
	raw, ok := arg.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
