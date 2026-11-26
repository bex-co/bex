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
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// namedVerb is the one shape an audit verb has: "<feature>.<Action>", the
// feature package's name and an exported action. A verb derived from a closure
// ("api.route.func1"), a method value ("http.HandlerFunc.ServeHTTP") or an
// unexported helper ("api.serve") has another shape, and so does the stack
// walk's "unknown" fallback.
var namedVerb = regexp.MustCompile(`^[a-z][a-z0-9]*\.[A-Z][A-Za-z0-9]*$`)

// closureMarkers are what the Go runtime puts in the name of a function that is
// not a declared one: a closure ("func1"), a method value ("-fm"), the wrapper of
// a go or defer statement ("gowrap1", "deferwrap1") and a package-level
// initializer ("glob.."). A verb carrying one was derived from such a frame.
var closureMarkers = []string{"func", "-fm", "gowrap", "deferwrap", "glob."}

// verbProblem names why verb is not a verb a reader of the audit log can search
// for, or "" when it is one.
func verbProblem(verb string) string {
	lower := strings.ToLower(verb)
	for _, marker := range closureMarkers {
		if strings.Contains(lower, marker) {
			return "derived from a closure or a function value"
		}
	}
	switch {
	case strings.HasPrefix(verb, "core."):
		// The kernel holds no verb: a "core." verb is the name of a helper
		// (core.AuthorizeLiveApp) or of the worker loop (core.runStep).
		return "named after a core helper, not a feature's action"
	case !namedVerb.MatchString(verb):
		return "not of the form <feature>.<Action>"
	}
	return ""
}

// lockedAuditSink is fakeAuditSink for handlers that may record from another
// goroutine.
type lockedAuditSink struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (s *lockedAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	return nil
}

func (s *lockedAuditSink) take() []core.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.events
	s.events = nil
	return out
}

// objectRouteVerbs is the verb of every Render object route (/v1/objects). A
// route added without a line here fails TestEveryRegisteredRouteRecordsANamedVerb
// until its verb is chosen.
var objectRouteVerbs = map[string]string{
	"GET /v1/objects/{ownerId}/{region}":             objectstorage.AuditVerbListObjects,
	"PUT /v1/objects/{ownerId}/{region}/{key...}":    objectstorage.AuditVerbPutObject,
	"GET /v1/objects/{ownerId}/{region}/{key...}":    objectstorage.AuditVerbGetObject,
	"DELETE /v1/objects/{ownerId}/{region}/{key...}": objectstorage.AuditVerbDeleteObject,
}

// auditVerbFixture is every route the API registers, over a caller that every
// authorization refuses: a refusal is recorded whatever the relation, so one
// request per route shows the verb the route authorizes under.
type auditVerbFixture struct {
	sink *lockedAuditSink
	mux  *http.ServeMux
	ws   string
}

func newAuditVerbFixture(t *testing.T, allow bool) *auditVerbFixture {
	t.Helper()
	svc, ws := newObjectService(t) // the object store over a fake S3, enabled
	sink := &lockedAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: allow}, Audit: sink}
	srv := NewServer(base, Deps{ObjectStorage: svc, BaseDomain: objectsTestBaseDomain})
	srv.HydraAdminURL = fakeHydraURL(t)
	return &auditVerbFixture{sink: sink, ws: ws, mux: srv.restHandler()}
}

// drive serves one request as a signed-in human and returns the audit events it
// recorded. A handler panic on a nil feature dependency is not what this test
// asks about, so it is recovered.
func (f *auditVerbFixture) drive(mux http.Handler, method, path, body string) []core.AuditEvent {
	func() {
		defer func() { _ = recover() }()
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-verbs", Method: core.MethodSession})
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(httptest.NewRecorder(), req)
	}()
	return f.sink.take()
}

// TestEveryRegisteredRouteRecordsANamedVerb is w21/087. The audit log named every
// object route "api.route.func1": core.callerVerb names a verb after the frame
// that called Authorize, and an adapter route authorizes inside a closure. The
// sweeps over the services (TestAuditCoversEveryWriteVerbExactlyOnce) walk the
// types that embed core.Base, which the object adapter does not, so nothing
// looked at it.
//
// This drives every route registered on the REST mux and requires each verb it
// records to be "<feature>.<Action>", so a verb derived from a closure, a method
// value or an unexported helper fails it. The Render object routes must also
// record the verb listed for them.
func TestEveryRegisteredRouteRecordsANamedVerb(t *testing.T) {
	// Refused, every route that authorizes records; allowed, a write route reaches
	// further into its handler before it records, and the verbs it records there
	// must be named too.
	refused := newAuditVerbFixture(t, false)
	allowed := newAuditVerbFixture(t, true)

	patterns := serveMuxPatterns(refused.mux)
	total := len(patterns)
	if total < 150 {
		t.Fatalf("enumerated only %d patterns — the ServeMux tree walk broke (Go release?)", total)
	}

	seen := map[string]bool{}
	recorded := map[string]bool{}
	for _, pattern := range patterns {
		method, path := splitPattern(pattern)
		target := fillWildcards(path, refused.ws)
		if strings.Contains(path, "{region}") {
			target = strings.Replace(path, "{region}", string(objectstorage.Oregon), 1)
			target = fillWildcards(strings.Replace(target, "{ownerId}", refused.ws, 1), "k")
		}
		body := ""
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
			body = "{}"
		}
		events := refused.drive(refused.mux, method, target, body)
		allowedEvents := allowed.drive(allowed.mux, method, target, body)
		if method == http.MethodGet {
			checkReadRecordsNoSuccess(t, "rest", pattern, allowedEvents)
		}
		for _, ev := range append(slices.Clone(events), allowedEvents...) {
			seen[ev.Verb] = true
			if p := verbProblem(ev.Verb); p != "" {
				t.Errorf("rest %s recorded verb %q: %s", pattern, ev.Verb, p)
			}
		}
		want, listed := objectRouteVerbs[pattern]
		switch {
		case !strings.HasPrefix(path, "/v1/objects/"):
			continue
		case len(events) != 1:
			t.Errorf("rest %s recorded %d audit events, want exactly one", pattern, len(events))
		case !listed:
			t.Errorf("rest %s recorded %q but has no line in objectRouteVerbs; give the route its own verb there", pattern, events[0].Verb)
		case events[0].Verb != want:
			t.Errorf("rest %s recorded %q, want %q", pattern, events[0].Verb, want)
		case events[0].Target != wantSweptTarget(pattern):
			t.Errorf("rest %s recorded target %q, want %q: a row names its bucket, and an object's its key (w21/094)",
				pattern, events[0].Target, wantSweptTarget(pattern))
		default:
			recorded[pattern] = true
		}
	}

	// A table line for a route that no longer exists is stale cover.
	registered := map[string]bool{}
	for _, p := range patterns {
		registered[p] = true
	}
	for p := range objectRouteVerbs {
		if !registered[p] {
			t.Errorf("objectRouteVerbs lists %q, which no mux registers", p)
		}
		if !recorded[p] {
			t.Errorf("%q recorded no audit event of its own: the route no longer authorizes under its verb", p)
		}
	}

	// Guard the guard: the sweep has to have seen the feature services' verbs
	// as well as the object ones, or a broken fixture would pass it.
	if len(seen) < 60 {
		names := make([]string, 0, len(seen))
		for v := range seen {
			names = append(names, v)
		}
		sort.Strings(names)
		t.Fatalf("only %d distinct verbs recorded across %d routes — the sweep went vacuous: %v", len(seen), total, names)
	}
	t.Logf("%d routes drove %d distinct verbs", total, len(seen))
}

func checkSweptVerbs(t *testing.T, surface, route string, events []core.AuditEvent, seen map[string]bool) {
	t.Helper()
	for _, ev := range events {
		seen[ev.Verb] = true
		if p := verbProblem(ev.Verb); p != "" {
			t.Errorf("%s %s recorded verb %q: %s", surface, route, ev.Verb, p)
		}
	}
}

// renderAuditedViews are the reads whose success an audit row records
// (ViewEnvVarValuesEvent, ViewConnectionInfoEvent and DownloadDatabaseBackupEvent
// in Render's log, and bex's download of an object, w21/094): the only verbs an
// allowed read may record a success row under. TestTheAuditedReadAllowlistIsPinned
// holds it to core's list.
var renderAuditedViews = map[string]bool{
	core.AuditVerbViewEnvVarValues:       true,
	core.AuditVerbViewConnectionInfo:     true,
	core.AuditVerbDownloadDatabaseBackup: true,
	objectstorage.AuditVerbGetObject:     true,
}

// TestTheAuditedReadAllowlistIsPinned is w21/094's guard that the rule "a successful
// read records nothing" is overridden for exactly these four verbs: the sweep's
// own list and core's closed set (core.AuditedViewVerbs, which the one writer of a
// view row, core.recordView, refuses to widen) must name the same verbs. The object
// verb is the one w21/094 added and the backup download the one w21/105 added. A
// fifth verb is a decision with its own ADR line, not a line here.
func TestTheAuditedReadAllowlistIsPinned(t *testing.T) {
	var swept []string
	for verb := range renderAuditedViews {
		swept = append(swept, verb)
	}
	sort.Strings(swept)
	if got := core.AuditedViewVerbs(); !slices.Equal(got, swept) {
		t.Errorf("core records a success under %v, the sweep allows %v", got, swept)
	}
	want := []string{"datastores.ViewConnectionInfo", "objectstorage.GetObject", "postgres.DownloadDatabaseBackup", "secrets.ViewEnvVarValues"}
	if !slices.Equal(swept, want) {
		t.Errorf("the verbs an allowed read may record a success under are %v, want exactly %v", swept, want)
	}
}

// readsThatWrite are the reads that may record a success row because they also
// write, each with the verb it records.
var readsThatWrite = map[string]string{
	// Resuming a workspace creation mints its Stripe Customer and SetupIntent
	// when the attempt has none yet.
	"graphql workspaceCreationAttempt": "workspaces.ResumeWorkspaceCreation",
	// Render's avatarPresignedUpload is a query that mints an upload slot; a
	// workspace's avatar is decided on can_manage, so its success is recorded
	// (w1/m648). The sweep draws the avatarType enum's first value, which
	// graphql-go orders at random, so it sees the row about half the time.
	"graphql avatarPresignedUpload": "avatars.PresignedUpload",
}

// checkReadRecordsNoSuccess fails a read (a GET route, a GraphQL query field, a
// read MCP tool) that an allowed caller sees record a success row (w4/683).
// Render's audit log records actions and two views; a read a write relation
// gates belongs in core's readVerbs, which keeps its denials recorded and its
// successes out.
func checkReadRecordsNoSuccess(t *testing.T, surface, route string, events []core.AuditEvent) {
	t.Helper()
	for _, ev := range events {
		if ev.Outcome == core.AuditAllowed && !renderAuditedViews[ev.Verb] && readsThatWrite[surface+" "+route] != ev.Verb {
			t.Errorf("%s %s is a read and recorded an allowed %q row: list the verb in core's readVerbs", surface, route, ev.Verb)
		}
	}
}

// isReadTool reports whether an MCP tool reads: annotated read-only, or named
// as a list or a get.
func isReadTool(tool *mcp.Tool) bool {
	if tool.Annotations != nil && tool.Annotations.ReadOnlyHint {
		return true
	}
	return strings.HasPrefix(tool.Name, "list_") || strings.HasPrefix(tool.Name, "get_")
}

// TestEveryMCPToolRecordsANamedVerb is the sweep above for the MCP surface: each
// tool is called with its required arguments and then with each optional one
// added, as a refused and as an allowed caller.
func TestEveryMCPToolRecordsANamedVerb(t *testing.T) {
	seen := map[string]bool{}
	tools := 0
	for _, allow := range []bool{false, true} {
		sink := &lockedAuditSink{}
		base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: allow}, Audit: sink}
		srv := NewServer(base, Deps{})
		raw := mcp.NewServer(&mcp.Implementation{Name: "audit-verbs", Version: "0"}, nil)
		// The client's context does not cross the in-memory transport: sign the
		// server side in, or every allowed call is refused for want of a caller.
		raw.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				return next(core.WithIdentity(ctx, core.Identity{Subject: "user-verbs", Method: core.MethodSession}), method, req)
			}
		})
		for _, f := range srv.features() {
			if r, ok := f.(mcpRegistrar); ok {
				r.RegisterMCP(raw)
			}
		}
		a, b := mcp.NewInMemoryTransports()
		if _, err := raw.Connect(context.Background(), a, nil); err != nil {
			t.Fatal(err)
		}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "audit-verbs", Version: "0"}, nil).Connect(context.Background(), b, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		listed, err := session.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		tools = len(listed.Tools)
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-verbs", Method: core.MethodSession})
		for _, tool := range listed.Tools {
			encoded, _ := json.Marshal(tool.InputSchema)
			var schema map[string]any
			_ = json.Unmarshal(encoded, &schema)
			for _, args := range toolArgVariants(schema) {
				func() {
					defer func() { _ = recover() }()
					_, _ = session.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: args})
				}()
				events := sink.take()
				if allow && isReadTool(tool) {
					checkReadRecordsNoSuccess(t, "mcp", tool.Name, events)
				}
				checkSweptVerbs(t, "mcp", tool.Name, events, seen)
			}
		}
	}
	if tools < 100 || len(seen) < 100 {
		t.Fatalf("%d tools drove %d distinct verbs — the sweep went vacuous", tools, len(seen))
	}
	t.Logf("%d tools drove %d distinct verbs", tools, len(seen))
}

// toolArgVariants are the argument sets one MCP tool is called with: its
// required properties, then those plus each optional one, so a handler that
// refuses an empty call before it authorizes is also reached.
func toolArgVariants(schema map[string]any) []map[string]any {
	props, _ := schema["properties"].(map[string]any)
	required := map[string]bool{}
	if list, ok := schema["required"].([]any); ok {
		for _, name := range list {
			required[name.(string)] = true
		}
	}
	base := map[string]any{}
	for name := range required {
		base[name] = sampleSchemaValue(props[name])
	}
	out := []map[string]any{base}
	for name := range props {
		if required[name] {
			continue
		}
		args := maps.Clone(base)
		args[name] = sampleSchemaValue(props[name])
		out = append(out, args)
	}
	return out
}

func sampleSchemaValue(property any) any {
	p, _ := property.(map[string]any)
	switch p["type"] {
	case "string":
		return "x"
	case "integer", "number":
		return 1
	case "boolean":
		return false
	case "array":
		return []any{}
	}
	return map[string]any{}
}

// TestEveryGraphQLFieldRecordsANamedVerb is the sweep above for the GraphQL
// surface: every query and mutation field is called with its required
// arguments set to a value of their type, as a refused and as an allowed caller.
func TestEveryGraphQLFieldRecordsANamedVerb(t *testing.T) {
	seen := map[string]bool{}
	fields := 0
	for _, allow := range []bool{false, true} {
		sink := &lockedAuditSink{}
		base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: allow}, Audit: sink}
		srv := NewServer(base, Deps{})
		schema, err := srv.newSchema()
		if err != nil {
			t.Fatal(err)
		}
		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-verbs", Method: core.MethodSession})
		fields = 0
		for _, root := range []struct {
			op     string
			fields graphql.FieldDefinitionMap
		}{{"query", schema.QueryType().Fields()}, {"mutation", schema.MutationType().Fields()}} {
			for name, field := range root.fields {
				fields++
				func() {
					defer func() { _ = recover() }()
					graphql.Do(graphql.Params{Schema: schema, RequestString: gqlOperation(root.op, name, field), Context: ctx})
				}()
				events := sink.take()
				if allow && root.op == "query" {
					checkReadRecordsNoSuccess(t, "graphql", name, events)
				}
				checkSweptVerbs(t, "graphql", name, events, seen)
			}
		}
	}
	if fields < 200 || len(seen) < 100 {
		t.Fatalf("%d fields drove %d distinct verbs — the sweep went vacuous", fields, len(seen))
	}
	t.Logf("%d fields drove %d distinct verbs", fields, len(seen))
}

// gqlOperation is a query of field with its required arguments filled in.
func gqlOperation(op, name string, field *graphql.FieldDefinition) string {
	var args []string
	for _, a := range field.Args {
		if _, required := a.Type.(*graphql.NonNull); required {
			args = append(args, a.PrivateName+": "+gqlSampleLiteral(a.Type))
		}
	}
	q := op + " { " + name
	if len(args) > 0 {
		q += "(" + strings.Join(args, ", ") + ")"
	}
	return q + gqlSelection(field.Type) + " }"
}

func gqlSampleLiteral(t graphql.Type) string {
	switch x := t.(type) {
	case *graphql.NonNull:
		return gqlSampleLiteral(x.OfType)
	case *graphql.List:
		return "[" + gqlSampleLiteral(x.OfType) + "]"
	case *graphql.Enum:
		if values := x.Values(); len(values) > 0 {
			return fmt.Sprint(values[0].Name)
		}
	case *graphql.InputObject:
		var parts []string
		for name, f := range x.Fields() {
			if _, required := f.Type.(*graphql.NonNull); required {
				parts = append(parts, name+": "+gqlSampleLiteral(f.Type))
			}
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *graphql.Scalar:
		switch x.Name() {
		case "Int":
			return "1"
		case "Float":
			return "1.5"
		case "Boolean":
			return "false"
		}
	}
	return `"x"`
}

// gqlSelection is the selection set a field of type t needs: its __typename, or
// none for a scalar.
func gqlSelection(t graphql.Type) string {
	switch x := t.(type) {
	case *graphql.NonNull:
		return gqlSelection(x.OfType)
	case *graphql.List:
		return gqlSelection(x.OfType)
	case *graphql.Object, *graphql.Interface, *graphql.Union:
		return " { __typename }"
	}
	return ""
}

// TestVerbProblemFlagsClosureVerbs is the anti-tautology self-check: the
// predicate the sweep applies must reject the shapes that were recorded.
func TestVerbProblemFlagsClosureVerbs(t *testing.T) {
	for _, bad := range []string{
		"api.route.func1",            // an object route's closure
		"api.RegisterREST.func2",     // Render's object routes' closure
		"http.HandlerFunc.ServeHTTP", // a method value mounted straight on the mux
		"api.serve",                  // an unexported helper
		"unknown",                    // callerVerb's fallback
		"",
		"secrets.graphQLQuery.func1",    // a GraphQL resolver's closure (w21/093)
		"agentsessions.RunOnce.func1",   // a goroutine's closure (w21/093)
		"keyvalue.ClearProjectID.func1", // a retry callback's closure (w21/093)
		"apps.glob..func5",              // a package-level table's closure
		"apps.labelNewService-fm",       // a method value
		"core.runStep",                  // core.Poll's step, where a worker's authorization stops
		"core.AuthorizeLiveApp",         // a kernel entry point that delegated to a sibling
		"logs.followAll.gowrap1",        // the wrapper of a go statement
	} {
		if verbProblem(bad) == "" {
			t.Errorf("verbProblem(%q) accepted it", bad)
		}
	}
	for _, good := range []string{"apps.Suspend", "objectstorage.PutObject", "api.ScopeClass", "billing.UpdatePaymentMethod"} {
		if p := verbProblem(good); p != "" {
			t.Errorf("verbProblem(%q) = %q, want accepted", good, p)
		}
	}
}

// TestObjectRoutesRecordAPut drives a real object put and delete through
// Render's contract as an allowed caller, and checks the audit log's rows: one
// allowed row each, under the verb a reader can search for.
func TestObjectRoutesRecordAPut(t *testing.T) {
	svc, ws := newObjectService(t)
	sink := &lockedAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: true}, Audit: sink}
	srv := NewServer(base, Deps{ObjectStorage: svc, BaseDomain: objectsTestBaseDomain})
	srv.HydraAdminURL = fakeHydraURL(t)
	restMux := srv.restHandler()
	identity := core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: core.MethodSession})

	serve := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(identity)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		restMux.ServeHTTP(rec, req)
		return rec
	}
	only := func(t *testing.T, what, verb string) {
		t.Helper()
		events := sink.take()
		if len(events) != 1 {
			t.Fatalf("%s recorded %d audit events, want 1: %+v", what, len(events), events)
		}
		ev := events[0]
		if ev.Verb != verb || ev.Outcome != core.AuditAllowed || ev.Resource != core.WorkspaceObject(ws) || ev.Caller != "user-1" {
			t.Fatalf("%s recorded %+v, want an allowed %s row on the workspace by user-1", what, ev, verb)
		}
	}

	put := serve(http.MethodPut, "/v1/objects/"+ws+"/oregon/b.txt", `{"sizeBytes":5}`)
	if put.Code != http.StatusOK {
		t.Fatalf("render put = %d %s", put.Code, put.Body)
	}
	only(t, "Render's object put", objectstorage.AuditVerbPutObject)
	// Render's delete answers 404 for an object that is not there, so upload one
	// through the minted URL first.
	var minted struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(put.Body.Bytes(), &minted); err != nil {
		t.Fatal(err)
	}
	up, _ := http.NewRequest(http.MethodPut, minted.URL, strings.NewReader("hello"))
	if resp, err := http.DefaultClient.Do(up); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("upload to the presigned URL: %v %v", resp, err)
	}

	// A read that is allowed records nothing (core.readRelations): the log
	// would hold a row for every download.
	if rec := serve(http.MethodGet, "/v1/objects/"+ws+"/oregon", ""); rec.Code != http.StatusOK {
		t.Fatalf("render list = %d %s", rec.Code, rec.Body)
	}
	if events := sink.take(); len(events) != 0 {
		t.Fatalf("an allowed list recorded %+v, want nothing", events)
	}

	if rec := serve(http.MethodDelete, "/v1/objects/"+ws+"/oregon/b.txt", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("render delete = %d %s", rec.Code, rec.Body)
	}
	only(t, "Render's object delete", objectstorage.AuditVerbDeleteObject)
}
