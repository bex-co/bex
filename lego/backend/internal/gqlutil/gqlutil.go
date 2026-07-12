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
// share: a typed resolver adapter and the common id argument. It is a leaf that
// imports only graphql-go — a shared presentation utility, not domain logic.
package gqlutil

import "github.com/graphql-go/graphql"

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
