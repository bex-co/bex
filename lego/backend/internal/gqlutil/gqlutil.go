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
	"context"
	"strings"

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

// Typed is a whole view field: the GraphQL output type paired with the Field
// resolver that projects one member off the typed source. Every object field in
// every feature fragment is this one shape — 663 of them carried the
// `&graphql.Field{Type: X, Resolve: gqlutil.Field(...)}` literal by hand, and
// not one carried any other key (no Description, Args, or DeprecationReason),
// so the literal was pure ceremony around the Type↔resolver pairing.
//
// StrField and its siblings below are the spellings for the scalar types; Typed
// takes the output type directly for object, list, and enum fields.
func Typed[T any](out graphql.Output, f func(T) any) *graphql.Field {
	return &graphql.Field{Type: out, Resolve: Field(f)}
}

// StrField, IntField, BoolField, and FloatField are Typed for the four scalar
// output types, which together cover the overwhelming majority of view fields.
func StrField[T any](f func(T) any) *graphql.Field   { return Typed(graphql.String, f) }
func IntField[T any](f func(T) any) *graphql.Field   { return Typed(graphql.Int, f) }
func BoolField[T any](f func(T) any) *graphql.Field  { return Typed(graphql.Boolean, f) }
func FloatField[T any](f func(T) any) *graphql.Field { return Typed(graphql.Float, f) }

// OptionalStrField is StrField for a string whose Go zero value ("") means
// "no relationship" (an optional foreign key: ProjectID, EnvironmentID, …) —
// it resolves "" to nil instead of the empty string, so GraphQL agrees with
// REST's `omitempty` omission of the same field instead of leaking the raw Go
// zero value as if it were real data (w6/m48).
func OptionalStrField[T any](f func(T) any) *graphql.Field {
	return Typed(graphql.String, func(v T) any {
		s, _ := f(v).(string)
		if s == "" {
			return nil
		}
		return s
	})
}

// ReqStrField and ReqBoolField are the non-null spellings, StrsField the
// `[String]` one — the three composed types common enough to name.
func ReqStrField[T any](f func(T) any) *graphql.Field {
	return Typed(graphql.NewNonNull(graphql.String), f)
}
func ReqBoolField[T any](f func(T) any) *graphql.Field {
	return Typed(graphql.NewNonNull(graphql.Boolean), f)
}
func StrsField[T any](f func(T) any) *graphql.Field {
	return Typed(graphql.NewList(graphql.String), f)
}

// IDArg is the `(id: String!)` argument shared by the single-resource queries and
// mutations (server(id), database(id), suspendService(id), ...). A fresh map per
// call so graphql-go never shares argument state across fields.
func IDArg() graphql.FieldConfigArgument { return KeyArg("id") }

// Arg and ReqArg are the optional and non-null spellings of a single argument
// declaration — the `&graphql.ArgumentConfig{Type: ...}` literal that every
// argument map repeats. A fresh pointer per call, for the same reason IDArg
// returns a fresh map.
func Arg(t graphql.Input) *graphql.ArgumentConfig { return &graphql.ArgumentConfig{Type: t} }
func ReqArg(t graphql.Input) *graphql.ArgumentConfig {
	return &graphql.ArgumentConfig{Type: graphql.NewNonNull(t)}
}

// KeyArg is IDArg for a single required string key under another name
// (serviceId, workspaceId, name, ...).
func KeyArg(name string) graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{name: ReqArg(graphql.String)}
}

// PageArgs adds the shared `cursor`/`limit` pagination pair to a list query's
// arguments, whether the window is then applied in-process by Page or pushed
// down to the store. It is the declaring half of the contract Page consumes:
// before this, every list declared the two arguments by hand and separately
// read them, so a list that declared neither (or only one) silently never
// paginated and nothing failed.
func PageArgs(args graphql.FieldConfigArgument) graphql.FieldConfigArgument {
	if args == nil {
		args = graphql.FieldConfigArgument{}
	}
	args["cursor"] = Arg(graphql.String)
	args["limit"] = Arg(graphql.Int)
	return args
}

// NilOnError makes every field of a built schema resolve to null when its
// resolver returns an error — the guarantee graphql-go's executor drops.
//
// graphql-go assigns the resolver's value to resolveField's NAMED result before
// it panics on the resolver's error; the deferred recover then returns that same
// named result, so the raw, un-completed Go value lands straight in the response
// map (graphql-go v0.8.1 executor.go:649-666). A resolver returning a pointer or
// a slice leaks a nil there and the field reads null by luck. A resolver
// returning a VALUE struct — every `func(ctx, id) (SomeView, error)` read verb
// in this codebase — leaks the ZERO struct, JSON-encoded whole (every Go field,
// not just the selected ones). So `server(id: "<dead>")` answered
// {"id":"","name":"","phase":"", ...} instead of null: no client could tell a
// deleted resource from an empty one, and the dashboard rendered a fully-chromed
// detail page — enabled Manual Deploy and all — for a service that does not
// exist, walking straight past the `!resource` predicate w9/m55's dead-id
// redirect is built on (w6/m44).
//
// Applied once to the composed schema instead of verb by verb: the defect is
// graphql-go's, not any one feature's, so every current AND future resolver
// inherits the fix rather than the bug — including the hand-written
// `&graphql.Field{Resolve: ...}` literals that never go through KeyVerb. The
// error itself is passed through untouched, so the response still carries why
// (`app not found`) and REST/MCP, which never see the executor, are unaffected.
func NilOnError(schema *graphql.Schema) {
	for name, t := range schema.TypeMap() {
		obj, ok := t.(*graphql.Object)
		if !ok || strings.HasPrefix(name, "__") {
			continue // introspection types resolve out of graphql-go's own machinery
		}
		for _, f := range obj.Fields() {
			if f.Resolve == nil {
				continue // DefaultResolveFn: a struct/map read that returns no error
			}
			resolve := f.Resolve
			f.Resolve = func(p graphql.ResolveParams) (any, error) {
				out, err := resolve(p)
				if err != nil {
					return nil, err
				}
				return out, nil
			}
		}
	}
}

// KeyVerb is a whole `(<key>: String!) -> fn(ctx, key)` field: the shape every
// single-resource query and every argument-less lifecycle mutation takes.
// IDVerb is its "id" spelling, which is nearly all of them.
func KeyVerb[T any](out graphql.Output, key string, fn func(context.Context, string) (T, error)) *graphql.Field {
	return &graphql.Field{
		Type: out,
		Args: KeyArg(key),
		Resolve: func(p graphql.ResolveParams) (any, error) {
			return fn(p.Context, p.Args[key].(string))
		},
	}
}

func IDVerb[T any](out graphql.Output, fn func(context.Context, string) (T, error)) *graphql.Field {
	return KeyVerb(out, "id", fn)
}

// ArgMutation is PatchMutation without the preview branch: the `(id, <arg>)`
// setter shape taken by every verb that writes one string field and has no
// dryRun counterpart (setRootDir, setPublishPath, renameProject, ...).
func ArgMutation[T any](out graphql.Output, argName string,
	set func(ctx context.Context, id, value string) (T, error)) *graphql.Field {
	args := IDArg()
	args[argName] = ReqArg(graphql.String)
	return &graphql.Field{
		Type: out,
		Args: args,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			return set(p.Context, p.Args["id"].(string), p.Args[argName].(string))
		},
	}
}

// PatchMutation is the one shape a "change this one field of a resource"
// mutation takes: `(id, <field>, dryRun)`, where dryRun routes to the PREVIEW
// verb — which resolves the identical spec and writes nothing (w2/m29).
//
// Shared because the dryRun branch is a rule, not boilerplate. Six mutations
// across three features carry it (updateServicePlan, updateDatabasePlan,
// updateKeyValuePlan, renameDatabase, renameKeyValue,
// setKeyValueMaxmemoryPolicy), and a seventh that reimplemented the field and
// dropped the branch would silently APPLY a change the caller asked only to
// preview — the response looks the same either way, so nothing would surface it.
//
// `patch` lifts the single string argument into the feature's patch struct;
// apply and preview are that feature's write and preview verbs, which share a
// signature and are therefore swappable at the call site. Each feature's
// TestGraphQLDryRun* pins its own wiring in both directions.
func PatchMutation[P, T any](out graphql.Output, argName string, patch func(string) P,
	apply, preview func(ctx context.Context, id string, patch P) (T, error)) *graphql.Field {
	args := IDArg()
	args[argName] = ReqArg(graphql.String)
	args["dryRun"] = Arg(graphql.Boolean)
	return &graphql.Field{
		Type: out,
		Args: args,
		Resolve: func(p graphql.ResolveParams) (any, error) {
			id, value := p.Args["id"].(string), p.Args[argName].(string)
			if dryRun, _ := p.Args["dryRun"].(bool); dryRun {
				return preview(p.Context, id, patch(value))
			}
			return apply(p.Context, id, patch(value))
		},
	}
}

// PlanMutation is PatchMutation for the plan-bearing resources, whose set and
// preview verbs take the plan string directly rather than a patch struct.
func PlanMutation[T any](out graphql.Output, set, preview func(ctx context.Context, id, plan string) (T, error)) *graphql.Field {
	return PatchMutation(out, "plan", func(plan string) string { return plan }, set, preview)
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
		"cidrBlock":   ReqStrField(func(e core.IPAllowListEntry) any { return e.CIDRBlock }),
		"description": StrField(func(e core.IPAllowListEntry) any { return e.Description }),
	},
})

var IPAllowEntryInputType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "IPAllowListEntryInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"cidrBlock":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"description": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

// ActionDecisionType is the shared wire shape of one projected resource-verb
// decision (core.ActionDecision — ADR087, w6/m136), exposed by the
// serverActions / deployActions / databaseActions / keyValueActions queries.
// Defined once here so the composed schema never has duplicate type names
// (the IPAllowEntryType precedent). outcome/reason are core's bounded
// tri-state vocabulary; precondition is the bounded blocking condition for an
// otherwise-permitted action; "" projects as null. Clients fail closed on
// anything unrecognized.
var ActionDecisionType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ActionDecision",
	Fields: graphql.Fields{
		"action":       ReqStrField(func(d core.ActionDecision) any { return d.Action }),
		"outcome":      ReqStrField(func(d core.ActionDecision) any { return d.Outcome }),
		"reason":       OptionalStrField(func(d core.ActionDecision) any { return d.Reason }),
		"precondition": OptionalStrField(func(d core.ActionDecision) any { return d.Precondition }),
	},
})

// ActionDecisionsOut is the `[ActionDecision!]!` output the four action
// projections share.
var ActionDecisionsOut = graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(ActionDecisionType)))

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

// Int reads an optional Int argument, 0 when absent — Str's integer-typed
// sibling, graduated on the same duplication threshold after the apps, sandbox,
// and webhooks fragments each carried a verbatim `gqlInt` copy.
func Int(args map[string]any, key string) int {
	if v, ok := args[key].(int); ok {
		return v
	}
	return 0
}

// Bool reads an optional Boolean argument, false when absent — Str's
// boolean-typed sibling, for the create verbs that want a plain flag rather
// than BoolPtr's tri-state.
func Bool(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

// PositiveLimit reads a `limit` GraphQL argument for a REJECT-policy list — one
// whose store reads limit <= 0 as "absent" and substitutes its own cap, so a
// bad value must 400 rather than come back as a full page (see
// core.ErrLimitNotPositive for the full rule).
//
// The presence check is the whole point, and Int cannot express it: an omitted
// argument and an explicit 0 are the same int once read, but only one of them
// is legal. graphql-go omits an unset optional arg from the map entirely, which
// is what makes the distinction recoverable here and nowhere downstream.
//
// Absent ⇒ (0, nil), meaning "no caller-supplied cap".
func PositiveLimit(args map[string]any) (int, error) {
	v, ok := args["limit"].(int)
	if !ok {
		return 0, nil
	}
	if v < 1 {
		return 0, core.ErrLimitNotPositive
	}
	return v, nil
}
