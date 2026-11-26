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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A sandbox group's dashboard pages (w1/m658): the Sandboxes list, its run
// summary and running count, a sandbox's drawer and events, and the
// Snapshots and Custom images reads, through GraphQL as the dashboard reads
// them, against the real control-plane store.

type groupPagesFixture struct {
	*groupFixture
	schema graphql.Schema
	other  string // a second workspace, whose member is id-b
}

func newGroupPagesFixture(t *testing.T) *groupPagesFixture {
	t.Helper()
	f := newGroupFixture(t)
	f.svc.Records, f.svc.Lifecycle, f.svc.Executions, f.svc.Names = f.st, f.st, f.st, f.st
	f.svc.Images = &Images{Store: f.st}
	f.svc.Snapshots = &Snapshots{Store: f.st}
	f.svc.DefaultPlan = DefaultPlan
	other, err := f.st.CreateWorkspace(context.Background(), fmt.Sprintf("sbpages-other-%d", time.Now().UnixNano()), store.PlanHobby, "sbpages-other-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.st.DeleteTenant(context.Background(), other.ID) })
	f.svc.Base.Workspace = fakeWorkspace{"id-a": f.ws, "id-b": other.ID}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: f.svc.GraphQLMutation()}),
		Types:    f.svc.GraphQLTypes(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &groupPagesFixture{groupFixture: f, schema: schema, other: other.ID}
}

// dashboardCtx is a dashboard request: a signed-in person on GraphQL.
func dashboardCtx(subject, email string) context.Context {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session", Email: email})
	return core.WithRequestOrigin(ctx, core.RequestOrigin{Transport: core.TransportGraphQL})
}

// query runs a GraphQL document as ctx and returns its data, or the code of
// its first error.
func (f *groupPagesFixture) query(t *testing.T, ctx context.Context, doc string, vars map[string]any) (map[string]any, string) {
	t.Helper()
	res := graphql.Do(graphql.Params{Schema: f.schema, RequestString: doc, VariableValues: vars, Context: ctx})
	if len(res.Errors) > 0 {
		code, _ := res.Errors[0].Extensions["code"].(string)
		if code == "" {
			t.Fatalf("GraphQL error without a code: %v", res.Errors[0])
		}
		return nil, code
	}
	data, _ := res.Data.(map[string]any)
	return data, ""
}

func (f *groupPagesFixture) mustQuery(t *testing.T, doc string, vars map[string]any) map[string]any {
	t.Helper()
	data, code := f.query(t, dashboardCtx("id-a", "ada@example.com"), doc, vars)
	if code != "" {
		t.Fatalf("query refused with %s:\n%s", code, doc)
	}
	return data
}

// createNamed starts a sandbox in group from the dashboard.
func (f *groupPagesFixture) createNamed(t *testing.T, groupID, name string) string {
	t.Helper()
	data := f.mustQuery(t, `mutation($g: String, $n: String) { createSandbox(sandboxGroupId: $g, name: $n) { id } }`,
		map[string]any{"g": groupID, "n": name})
	return data["createSandbox"].(map[string]any)["id"].(string)
}

func (f *groupPagesFixture) terminate(t *testing.T, id string) {
	t.Helper()
	f.mustQuery(t, `mutation($id: String!) { terminateSandbox(id: $id) }`, map[string]any{"id": id})
}

const sandboxPage = `query($in: SandboxesForGroupFilterInput!) {
  sandboxesForGroup(input: $in) { cursor hasMore sortField sortDirection
    sandboxes { id name status endedAt startedAt trigger source { ... on SandboxEmailSource { email } } planSpec { name cpus memoryGb } } } }`

func (f *groupPagesFixture) page(t *testing.T, in map[string]any) (ids []string, cursor string, hasMore bool, rows []map[string]any) {
	t.Helper()
	data := f.mustQuery(t, sandboxPage, map[string]any{"in": in})
	page := data["sandboxesForGroup"].(map[string]any)
	for _, raw := range page["sandboxes"].([]any) {
		row := raw.(map[string]any)
		rows = append(rows, row)
		ids = append(ids, row["id"].(string))
	}
	cursor, _ = page["cursor"].(string)
	return ids, cursor, page["hasMore"].(bool), rows
}

func sortedIDs(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestSandboxesForGroupListsFiltersPagesAndSummarizes(t *testing.T) {
	f := newGroupPagesFixture(t)
	g := f.createGroup(t, "pages")
	other := f.createGroup(t, "other")
	alpha := f.createNamed(t, g.ID, "alpha-one")
	beta := f.createNamed(t, g.ID, "beta")
	ended := f.createNamed(t, g.ID, "")
	elsewhere := f.createNamed(t, other.ID, "alpha-two")
	f.terminate(t, ended)

	all, _, more, rows := f.page(t, map[string]any{"sandboxGroupId": g.ID})
	if !slices.Equal(sortedIDs(all...), sortedIDs(alpha, beta, ended)) || more {
		t.Fatalf("the group lists %v (more %v), want its live and ended sandboxes %v and not %s", all, more, []string{alpha, beta, ended}, elsewhere)
	}
	for _, row := range rows {
		switch row["id"] {
		case ended:
			if row["status"] != "terminated" || row["endedAt"] == nil {
				t.Errorf("the terminated sandbox reads %v", row)
			}
		case alpha:
			src, _ := row["source"].(map[string]any)
			if row["trigger"] != TriggerDashboard || src["email"] != "ada@example.com" || row["startedAt"] == nil {
				t.Errorf("a sandbox the dashboard started reads trigger %v, source %v, startedAt %v", row["trigger"], row["source"], row["startedAt"])
			}
			if plan, _ := row["planSpec"].(map[string]any); plan["cpus"] != 1.0 || plan["memoryGb"] != 2.0 {
				t.Errorf("the default plan reads %v, want 1 CPU and 2 GB", plan)
			}
		}
	}

	// Each row is the sandbox REST answers for the same id.
	for _, row := range rows {
		code, body := f.do(t, http.MethodGet, "/v1/sandboxes/"+row["id"].(string)+"?ownerId="+f.ws, "")
		var rest Sandbox
		if err := json.Unmarshal([]byte(body), &rest); code != http.StatusOK || err != nil {
			t.Fatalf("REST get %s = %d %s", row["id"], code, body)
		}
		if string(rest.Status) != row["status"] || (rest.Name == nil) != (row["name"] == nil) {
			t.Errorf("GraphQL lists %v, REST reads status %s name %v", row, rest.Status, rest.Name)
		}
	}

	if running, _, _, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "statuses": []any{"running"}}); !slices.Equal(sortedIDs(running...), sortedIDs(alpha, beta)) {
		t.Errorf("statuses [running] lists %v", running)
	}
	if found, _, _, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "search": "ALPHA"}); !slices.Equal(found, []string{alpha}) {
		t.Errorf("a search for part of a name, in another case, finds %v, want only %s", found, alpha)
	}
	if found, _, _, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "search": strings.ToUpper(ended)}); !slices.Equal(found, []string{ended}) {
		t.Errorf("a search for an id finds %v, want %s", found, ended)
	}
	later := f.now.Add(time.Hour).Format(time.RFC3339)
	if found, _, _, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "createdAfter": later}); len(found) != 0 {
		t.Errorf("created after the sandboxes lists %v", found)
	}
	if found, _, _, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "createdBefore": later}); len(found) != 3 {
		t.Errorf("created before an hour from now lists %v", found)
	}

	first, cursor, more, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "limit": 2})
	rest, _, moreAfter, _ := f.page(t, map[string]any{"sandboxGroupId": g.ID, "limit": 2, "cursor": cursor})
	if len(first) != 2 || !more || len(rest) != 1 || moreAfter || !slices.Equal(slices.Concat(first, rest), all) {
		t.Errorf("two pages of 2 read %v (more %v) then %v (more %v), want %v", first, more, rest, moreAfter, all)
	}

	data := f.mustQuery(t, `query($id: String!) { sandboxGroup(id: $id) { runningSandboxCount } }`, map[string]any{"id": g.ID})
	if n := data["sandboxGroup"].(map[string]any)["runningSandboxCount"]; n != 2 {
		t.Errorf("runningSandboxCount = %v, want the 2 running sandboxes", n)
	}
	data = f.mustQuery(t, `query($id: String!) { sandboxGroupRuns(sandboxGroupId: $id) { total errored succeeded buckets { succeeded errored } } }`, map[string]any{"id": g.ID})
	runs := data["sandboxGroupRuns"].(map[string]any)
	charted := 0
	for _, b := range runs["buckets"].([]any) {
		charted += b.(map[string]any)["succeeded"].(int) + b.(map[string]any)["errored"].(int)
	}
	if runs["total"] != 1 || runs["succeeded"] != 1 || runs["errored"] != 0 || charted != 1 {
		t.Errorf("the group's runs read %v (charted %d), want the 1 terminated run", runs, charted)
	}

	// Another workspace's member finds neither the group nor its sandboxes.
	ctxB := dashboardCtx("id-b", "")
	for _, doc := range []string{
		`query($id: String!) { sandboxesForGroup(input: {sandboxGroupId: $id}) { hasMore } }`,
		`query($id: String!) { sandboxGroupRuns(sandboxGroupId: $id) { total } }`,
	} {
		if _, code := f.query(t, ctxB, doc, map[string]any{"id": g.ID}); code != "SANDBOX_GROUP_NOT_FOUND" && code != "NOT_FOUND" {
			t.Errorf("another workspace's member reads the group's sandboxes: code %q", code)
		}
	}
}

func TestSandboxEventsMergeKindsNewestFirstAndPageBackwards(t *testing.T) {
	f := newGroupPagesFixture(t)
	g := f.createGroup(t, "events")
	id := f.createNamed(t, g.ID, "timeline")
	ctx := context.Background()
	at := func(minutes int) time.Time { return f.now.Add(time.Duration(minutes) * time.Minute) }
	zero, one := 0, 1
	for _, e := range []store.SandboxExecution{
		{ID: "exe-0000000000000000000a", Type: "runs", Operation: "sync", Command: "echo ok", StartedAt: at(1), StoppedAt: ptrTime(at(1)), ExitCode: &zero},
		{ID: "exe-0000000000000000000b", Type: "runs", Operation: "sync", Command: "false", StartedAt: at(2), StoppedAt: ptrTime(at(2)), ExitCode: &one},
		// Still running when the sandbox is terminated, which cuts it off.
		{ID: "exe-0000000000000000000c", Type: "runs", Operation: "stream", Command: "sleep 60", StartedAt: at(2)},
	} {
		e.WorkspaceID, e.SandboxID, e.Subject = f.ws, id, "id-a"
		if err := f.st.RecordSandboxExecution(ctx, e); err != nil {
			t.Fatal(err)
		}
		if e.StoppedAt != nil {
			if err := f.st.StopSandboxExecution(ctx, f.ws, e.ID, *e.StoppedAt, e.ExitCode); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.now = at(3)
	f.terminate(t, id)

	const doc = `query($id: String!, $before: String, $beforeId: String, $limit: Int) {
  sandboxEvents(sandboxId: $id, before: $before, beforeId: $beforeId, limit: $limit) { hasMore events { __typename id timestamp outcome
    ... on SandboxLifecycleEvent { lifecycleType trigger userEmail }
    ... on SandboxExecutionEvent { executionId exitCode } } } }`
	read := func(vars map[string]any) ([]map[string]any, bool) {
		data := f.mustQuery(t, doc, vars)
		page := data["sandboxEvents"].(map[string]any)
		var events []map[string]any
		for _, e := range page["events"].([]any) {
			events = append(events, e.(map[string]any))
		}
		return events, page["hasMore"].(bool)
	}
	all, more := read(map[string]any{"id": id})
	var kinds []string
	for _, e := range all {
		kind := e["__typename"].(string)
		if lt, ok := e["lifecycleType"].(string); ok {
			kind = lt
		} else if ex, ok := e["executionId"].(string); ok {
			kind = ex[len(ex)-1:] + ":" + e["outcome"].(string)
		}
		kinds = append(kinds, kind)
	}
	// Newest first; the two executions of the same minute by id, descending;
	// created and ready share the create's instant.
	want := []string{"TERMINATED", "c:ERROR", "b:ERROR", "a:SUCCESS"}
	if more || len(kinds) != 6 || !slices.Equal(kinds[:4], want) || !slices.Equal(sortedIDs(kinds[4:]...), []string{"CREATED", "READY"}) {
		t.Fatalf("the timeline reads %v (more %v), want %v then created and ready", kinds, more, want)
	}
	for _, e := range all {
		if e["lifecycleType"] == "CREATED" && (e["trigger"] != TriggerDashboard || e["userEmail"] != "ada@example.com") {
			t.Errorf("the created event reads %v, want the dashboard and the person who started it", e)
		}
	}

	var walked []string
	vars := map[string]any{"id": id, "limit": 2}
	for range 5 {
		page, more := read(vars)
		for _, e := range page {
			walked = append(walked, e["id"].(string))
		}
		if !more {
			break
		}
		last := page[len(page)-1]
		vars = map[string]any{"id": id, "limit": 2, "before": last["timestamp"], "beforeId": last["id"]}
	}
	var allIDs []string
	for _, e := range all {
		allIDs = append(allIDs, e["id"].(string))
	}
	if !slices.Equal(walked, allIDs) {
		t.Errorf("pages of 2 walk %v, want the timeline %v once each, in order", walked, allIDs)
	}

	if _, code := f.query(t, dashboardCtx("id-b", ""), doc, map[string]any{"id": id}); code != "SANDBOX_NOT_FOUND" {
		t.Errorf("another workspace's member reads the timeline: code %q", code)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestGroupSnapshotsAndImagePagesReadTheGroup(t *testing.T) {
	f := newGroupPagesFixture(t)
	g := f.createGroup(t, "media")
	source := f.createNamed(t, g.ID, "source")
	ctx := context.Background()
	name := func(s string) *string { return &s }
	var snaps []string
	for i, n := range []string{"base", "nightly", "nightly-2"} {
		snap, err := f.st.CreateSandboxSnapshot(ctx, store.NewSandboxSnapshot{
			WorkspaceID: f.ws, SandboxGroupID: g.ID, SourceSandboxID: source, Name: name(n),
			Kind: "filesystem", Plan: string(Plan2C4G), TTLSeconds: 0, RequestedBy: "id-a", NetworkPolicy: []byte(`{"type":"allow-all"}`),
		}, store.SandboxSnapshotLimits{}, f.now.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, snap.ID)
		// One capture of a sandbox at a time: settle each before the next.
		if i < 2 {
			if _, err := f.st.MarkSandboxSnapshotAvailable(ctx, snap.ID, "sha256:"+strings.Repeat("c", 64), 1024, f.now); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.st.FailSandboxSnapshot(ctx, snap.ID, "the capture did not finish in time"); err != nil {
			t.Fatal(err)
		}
	}

	data := f.mustQuery(t, `query($in: SandboxSnapshotsFilterInput!) { sandboxSnapshotsForGroup(input: $in) {
  hasMore snapshots { id status failureCode planSpec { name cpus } } } }`,
		map[string]any{"in": map[string]any{"sandboxGroupId": g.ID, "search": "NIGHT"}})
	var found []string
	for _, raw := range data["sandboxSnapshotsForGroup"].(map[string]any)["snapshots"].([]any) {
		row := raw.(map[string]any)
		found = append(found, row["id"].(string))
		if row["id"] == snaps[2] && (row["status"] != "failed" || row["failureCode"] != snapshotFailureDeadline) {
			t.Errorf("the capture that ran out of time reads %v", row)
		}
		if plan, _ := row["planSpec"].(map[string]any); plan["cpus"] != 2.0 {
			t.Errorf("a 2c-4g snapshot's plan reads %v", plan)
		}
	}
	if !slices.Equal(found, []string{snaps[2], snaps[1]}) {
		t.Errorf("a search for NIGHT finds %v, want the two nightly snapshots newest first", found)
	}

	// A snapshot by its id alone, as a link to it resolves; not from another workspace.
	by := `query($id: String!) { sandboxSnapshot(id: $id) { id sandboxGroupId } }`
	if data := f.mustQuery(t, by, map[string]any{"id": snaps[0]}); data["sandboxSnapshot"].(map[string]any)["sandboxGroupId"] != g.ID {
		t.Errorf("the snapshot by id reads %v", data)
	}
	if _, code := f.query(t, dashboardCtx("id-b", ""), by, map[string]any{"id": snaps[0]}); code != "snapshot_not_found" {
		t.Errorf("another workspace's member reads the snapshot by id: code %q", code)
	}

	// Images by name: each name's newest version, and its newest available one.
	var builds []string
	for i, b := range []struct{ name, ref, outcome string }{
		{"runtime", "v1", "succeeded"}, {"runtime", "v2", "failed"}, {"tools", "v1", "pending"},
	} {
		row, _, err := f.st.CreateSandboxBuild(ctx, store.NewSandboxBuild{
			WorkspaceID: f.ws, SandboxGroupID: g.ID, Name: b.name, SourceRepository: "ghcr.io", SourceImage: "acme/" + b.name,
			SourceReference: b.ref, SourceDigest: fmt.Sprintf("sha256:%064d", i),
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		switch b.outcome {
		case "succeeded":
			if _, err := f.st.StartSandboxBuild(ctx, row.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.st.SucceedSandboxBuild(ctx, row.ID, "sha256:"+strings.Repeat("b", 64), 1); err != nil {
				t.Fatal(err)
			}
		case "failed":
			if err := f.st.FailSandboxBuild(ctx, row.ID, "image_not_found", "fetch source image: manifest unknown"); err != nil {
				t.Fatal(err)
			}
		}
		builds = append(builds, row.ID)
		time.Sleep(5 * time.Millisecond) // created_at orders the versions
	}
	data = f.mustQuery(t, `query($in: SandboxBuildSourcesFilterInput!) { sandboxBuildSources(input: $in) {
  hasMore sources { name source { reference } latestBuild { id status } latestSucceededBuild { id } } } }`,
		map[string]any{"in": map[string]any{"sandboxGroupId": g.ID}})
	sources := data["sandboxBuildSources"].(map[string]any)["sources"].([]any)
	if len(sources) != 2 {
		t.Fatalf("the group's images read %v, want runtime and tools", sources)
	}
	byName := map[string]map[string]any{}
	for _, raw := range sources {
		byName[raw.(map[string]any)["name"].(string)] = raw.(map[string]any)
	}
	runtime := byName["runtime"]
	if runtime["latestBuild"].(map[string]any)["id"] != builds[1] || runtime["latestSucceededBuild"].(map[string]any)["id"] != builds[0] ||
		runtime["source"].(map[string]any)["reference"] != "v2" {
		t.Errorf("runtime reads %v, want v2 (failed) latest and v1 the latest available", runtime)
	}
	if tools := byName["tools"]; tools["latestSucceededBuild"] != nil {
		t.Errorf("tools, never imported, reads a latest available version: %v", tools)
	}
	if _, code := f.query(t, dashboardCtx("id-b", ""), `query($id: String!) { sandboxBuildSources(input: {sandboxGroupId: $id}) { hasMore } }`,
		map[string]any{"id": g.ID}); code != "SANDBOX_GROUP_NOT_FOUND" && code != "NOT_FOUND" {
		t.Errorf("another workspace's member reads the group's images: code %q", code)
	}
}
