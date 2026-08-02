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
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// crossworkspace_gqlmcp_test.go is t003: the GraphQL and MCP halves of the
// negative-authorization matrix. Same fixture as the REST matrix (a caller who
// is a member of tea-a only; every named resource owned by tea-b), enumerated
// the same way — from the schema's own field set and the MCP registry's own tool
// list, so a new resolver/tool joins the matrix automatically or fails a
// completeness guard.

// wireAllFeatures builds a Server with EVERY feature service present and sharing
// one fixture Base — the reflection twin of TestSweepCoversEveryWiredService's
// discovery, extended to also inject the Base so the wired services actually
// authorize. It gives newSchema()/MCPServer() the full field and tool set without
// threading every feature's Deps, matching sweepableServices' coverage (both are
// held to features() by TestSweepCoversEveryWiredService).
func wireAllFeatures(base *core.Base) *Server {
	srv := &Server{}
	baseType := reflect.TypeOf(&core.Base{})
	v := reflect.ValueOf(srv).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Ptr || f.Type().Elem().Kind() != reflect.Struct || !f.CanSet() {
			continue
		}
		st := f.Type().Elem()
		baseField := -1
		for j := 0; j < st.NumField(); j++ {
			if sf := st.Field(j); sf.Anonymous && sf.Type == baseType {
				baseField = j
			}
		}
		if baseField < 0 {
			continue
		}
		svc := reflect.New(st)
		svc.Elem().Field(baseField).Set(reflect.ValueOf(base))
		f.Set(svc)
	}
	return srv
}

// crossWorkspaceTarget is the name every workspace-B resource (App/Database/
// KeyValue) is seeded under and the value the matrices fill into resource-id
// arguments — one constant so the fixture and the callers can't drift.
const crossWorkspaceTarget = "target"

// crossWorkspaceBase is the shared fixture used by every m55 matrix (t001–t003):
// allow-own/deny-other checker, a resolver that makes the caller a member of
// tea-a only, and App/Database/KeyValue crossWorkspaceTarget all owned by tea-b.
// A fresh one per call gives each destructive sweep its own apiserver.
func crossWorkspaceBase() *core.Base {
	return &core.Base{
		Client: fakeClient(
			ownedApp(crossWorkspaceTarget, "tea-b"),
			ownedDB(crossWorkspaceTarget, "tea-b"),
			ownedKV(crossWorkspaceTarget, "tea-b"),
		),
		Namespace: "default",
		Authz:     allowOwnChecker{},
		Workspace: nonMemberResolver{},
	}
}

// gqlArgValue supplies a workspace-B-targeting value for a resolver argument: any
// string/id becomes the tea-b resource name "target"; numeric/bool args take a
// zero so a resolver reading them by type assertion doesn't panic. Input-object
// args are left unset (create-shaped verbs are caller-scoped exclusions).
func gqlArgValue(arg *graphql.Argument) (any, bool) {
	switch graphql.GetNamed(arg.Type) {
	case graphql.String, graphql.ID:
		return crossWorkspaceTarget, true
	case graphql.Int:
		return 0, true
	case graphql.Float:
		return 0.0, true
	case graphql.Boolean:
		return false, true
	}
	return nil, false
}

// callGQLField invokes one top-level resolver with tea-b-targeting args as the
// non-member caller, folding a panic (a resolver reading a missing/typed arg
// against nil deps) into an error — a panic is not a data leak, so the matrix's
// one question, "did this resolver return DATA cross-workspace?", stays the only
// failure.
func callGQLField(ctx context.Context, def *graphql.FieldDefinition) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result, err = nil, context.Canceled
		}
	}()
	args := map[string]any{}
	for _, a := range def.Args {
		if v, ok := gqlArgValue(a); ok {
			args[a.Name()] = v
		}
	}
	if def.Resolve == nil {
		return nil, context.Canceled
	}
	return def.Resolve(graphql.ResolveParams{Context: ctx, Args: args})
}

// callerScopedGQLFields are the resolvers the matrix enumerates but does not
// assert-deny — the GraphQL twin of callerScopedVerbs: each acts on the caller's
// own resolved workspace/identity (a collection, a create, a whoami, a global
// catalog), taking no tea-b-scoped target. A resolver that returns data here and
// is NOT justified fails the sweep.
var callerScopedGQLFields = map[string]bool{
	// Global compute/plan catalogs — identical for every workspace, no resource.
	"Query.instanceTypes":         true,
	"Query.databaseInstanceTypes": true,
	"Query.keyValueInstanceTypes": true,
	// Name-availability probe — checks the CALLER's own workspace (apps.NameAvailable).
	"Query.serviceNameAvailable": true,
	// Static vocabularies / stubs — return a fixed list, read no resource.
	"Query.webhookEventTypes":            true, // the webhook event-type vocabulary
	"Query.metricsPathFilterSuggestions": true, // bex stub: always an empty path list
	"Query.pushNotificationsAvailable":   true, // caller-scoped transport capability, no resource argument
	"Query.notificationInbox":            true, // caller's own tenant + subject are derived from auth context
	"Query.unreadPushNotificationCount":  true, // caller's own tenant + subject are derived from auth context
	"Mutation.markPushNotificationRead":  true, // exact id is constrained inside the caller-derived tenant + subject scope
}

func TestCrossWorkspaceGraphQLMatrix(t *testing.T) {
	srv := wireAllFeatures(crossWorkspaceBase())
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "attacker", Method: "session"})

	fieldSets := map[string]graphql.FieldDefinitionMap{
		"Query":    schema.QueryType().Fields(),
		"Mutation": schema.MutationType().Fields(),
	}
	swept, denied := 0, 0
	seen := map[string]bool{}
	for root, fields := range fieldSets {
		names := make([]string, 0, len(fields))
		for n := range fields {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			swept++
			key := root + "." + n
			seen[key] = true
			_, ferr := callGQLField(ctx, fields[n])
			if ferr == nil {
				if callerScopedGQLFields[key] {
					continue
				}
				t.Errorf("%s: resolved to DATA for a workspace-A caller targeting a workspace-B resource — "+
					"either it leaks cross-workspace or it is caller-scoped and must be justified in callerScopedGQLFields", key)
				continue
			}
			denied++
		}
	}
	if swept < 60 {
		t.Fatalf("only %d GraphQL fields swept — schema assembly broke?", swept)
	}
	assertNoStaleExclusions(t, "callerScopedGQLFields", callerScopedGQLFields, seen)
	t.Logf("%d/%d GraphQL fields deny a non-member cross-workspace caller; %d caller-scoped exclusions", denied, swept, len(callerScopedGQLFields))
}

// toolStringArgs reads a listed tool's JSON-Schema (decoded to map[string]any by
// the client) and returns its string-typed property names — the ones filled with
// the tea-b resource id "target". workspaceId/ownerId are OMITTED on purpose: the
// Render per-call contract binds workspaceId as the caller's ACTING workspace
// (mcp_workspace.go), so filling it with "target" would make every tool a
// non-member of the named workspace and deny at that gate — masking a confused
// deputy that authorizes the caller's OWN workspace. Leaving them unset keeps the
// caller in tea-a, so only the resource id points at tea-b and a leak surfaces.
func toolStringArgs(schema any) []string {
	m, _ := schema.(map[string]any)
	props, _ := m["properties"].(map[string]any)
	var out []string
	for name, p := range props {
		switch name {
		case "workspaceId", "ownerId", "ownerID":
			continue
		}
		if pm, ok := p.(map[string]any); ok && pm["type"] == "string" {
			out = append(out, name)
		}
	}
	return out
}

// callerScopedMCPTools are the tools the matrix enumerates but does not
// assert-deny — the MCP twin of callerScopedVerbs. A tool returning a non-error
// result here that is NOT justified fails the sweep.
var callerScopedMCPTools = map[string]bool{
	// Collection lists — scoped to the caller's own (tea-a) workspace; the tea-b
	// resources are filtered out by the owner label, so the list is the caller's
	// own (here empty), never a cross-workspace read.
	"list_services":           true,
	"list_key_value":          true,
	"list_postgres_instances": true,
	// Validates a posted bex.yml manifest string — a pure function on the input,
	// no resource and no workspace touched.
	"validate_bex_yml": true,
}

func TestCrossWorkspaceMCPMatrix(t *testing.T) {
	srv := wireAllFeatures(crossWorkspaceBase())
	cs := mcpSessionAs(t, srv, "attacker")
	ctx := context.Background()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) < 60 {
		t.Fatalf("only %d MCP tools listed — registry assembly broke?", len(tools.Tools))
	}
	names := make([]string, 0, len(tools.Tools))
	byName := map[string]*mcp.Tool{}
	registered := map[string]bool{}
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
		byName[tl.Name] = tl
		registered[tl.Name] = true
	}
	sort.Strings(names)

	denied := 0
	for _, name := range names {
		args := map[string]any{}
		for _, a := range toolStringArgs(byName[name].InputSchema) {
			args[a] = crossWorkspaceTarget
		}
		res, cerr := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		leaked := cerr == nil && res != nil && !res.IsError
		if leaked {
			if callerScopedMCPTools[name] {
				continue
			}
			t.Errorf("MCP tool %q: succeeded for a workspace-A caller targeting a workspace-B resource — "+
				"either it leaks cross-workspace or it is caller-scoped and must be justified in callerScopedMCPTools", name)
			continue
		}
		denied++
	}
	assertNoStaleExclusions(t, "callerScopedMCPTools", callerScopedMCPTools, registered)
	t.Logf("%d/%d MCP tools deny a non-member cross-workspace caller; %d caller-scoped exclusions", denied, len(names), len(callerScopedMCPTools))
}
