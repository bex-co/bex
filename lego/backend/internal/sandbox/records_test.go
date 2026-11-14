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
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeRecords is an in-memory RecordStore holding the ended records the paths
// under test write; the store's own semantics are pinned on Postgres
// (TestSandboxRecordsPG).
type fakeRecords struct {
	mu    sync.Mutex
	ended map[string]store.SandboxRecord
}

func (f *fakeRecords) RecordSandbox(_ context.Context, r store.SandboxRecord) error {
	if r.TerminatedAt == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ended == nil {
		f.ended = map[string]store.SandboxRecord{}
	}
	f.ended[r.SandboxID] = r
	return nil
}
func (f *fakeRecords) SetSandboxExpiry(context.Context, string, string, *time.Time) error { return nil }
func (f *fakeRecords) EndAbsentSandboxes(context.Context, string, []string, time.Time) error {
	return nil
}
func (f *fakeRecords) ExpireSandboxRecords(context.Context, time.Time) error { return nil }
func (f *fakeRecords) EndedSandbox(context.Context, string, string) (store.SandboxRecord, error) {
	return store.SandboxRecord{}, store.ErrNotFound
}
func (f *fakeRecords) EndedSandboxes(context.Context, string) ([]store.SandboxRecord, error) {
	return nil, nil
}
func (f *fakeRecords) EndedSandboxWorkspaces(context.Context, string) ([]string, error) {
	return nil, nil
}

// recordsFixture is the groups fixture with sandbox records, names and
// executions on the real store, and a second, non-admin member id-b.
func recordsFixture(t *testing.T) *groupFixture {
	t.Helper()
	f := newGroupFixture(t)
	f.svc.Records, f.svc.Names, f.svc.Executions = f.st, f.st, f.st
	f.svc.Base.Workspace = fakeWorkspace{"id-a": f.ws, "id-b": f.ws}
	return f
}

func (f *groupFixture) getSandbox(t *testing.T, subject, id string) (int, Sandbox, string) {
	t.Helper()
	code, body := f.doAs(t, subject, http.MethodGet, "/v1/sandboxes/"+id+"?ownerId="+f.ws, "")
	var sb Sandbox
	_ = json.Unmarshal([]byte(body), &sb)
	return code, sb, body
}

func (f *groupFixture) listIDs(t *testing.T, query string) []string {
	t.Helper()
	code, body := f.do(t, http.MethodGet, "/v1/sandboxes?ownerId="+f.ws+query, "")
	var items []SandboxEnvelope
	if err := json.Unmarshal([]byte(body), &items); code != http.StatusOK || err != nil {
		t.Fatalf("list%s = %d %s", query, code, body)
	}
	var out []string
	for _, item := range items {
		out = append(out, item.Sandbox.ID)
	}
	return out
}

// TestTerminatedSandboxStaysReadable pins w3/m318 against Render's contract:
// after terminate, GET answers the sandbox with status terminated and
// terminatedAt, list-sandboxes hides it unless status=terminated is named, a
// repeat terminate answers 204, its execution history stays readable, and both
// connect-token mints answer 409 with the end the CLI prints. Another member,
// who could not see it live, still cannot.
func TestTerminatedSandboxStaysReadable(t *testing.T) {
	f := recordsFixture(t)
	sb := f.mustCreate(t, "")
	terminate := "/v1/sandboxes/" + sb.ID + "/terminate?ownerId=" + f.ws
	if code, body := f.do(t, http.MethodPost, terminate, ""); code != http.StatusNoContent {
		t.Fatalf("terminate = %d %s", code, body)
	}

	code, got, body := f.getSandbox(t, "id-a", sb.ID)
	if code != http.StatusOK || got.Status != StatusTerminated || got.TerminatedAt == nil || !got.TerminatedAt.Equal(f.now) ||
		got.SandboxGroupID != sb.SandboxGroupID || got.Region != "oregon" || got.CreatedAt == nil {
		t.Fatalf("GET terminated = %d %s, want 200 terminated at %s in group %s", code, body, f.now.Format(time.RFC3339), sb.SandboxGroupID)
	}
	if code, body := f.do(t, http.MethodPost, terminate, ""); code != http.StatusNoContent {
		t.Fatalf("repeat terminate = %d %s, want Render's idempotent 204", code, body)
	}
	if ids := f.listIDs(t, ""); slices.Contains(ids, sb.ID) {
		t.Fatalf("default list = %v, want the terminated sandbox hidden", ids)
	}
	for _, query := range []string{"&status=terminated", "&status=running,terminated"} {
		if ids := f.listIDs(t, query); !slices.Equal(ids, []string{sb.ID}) {
			t.Fatalf("list%s = %v, want [%s]", query, ids, sb.ID)
		}
	}
	if code, body := f.do(t, http.MethodGet, "/v1/sandboxes/"+sb.ID+"/execs?ownerId="+f.ws, ""); code != http.StatusOK {
		t.Fatalf("execs of a terminated sandbox = %d %s, want 200", code, body)
	}
	at := f.now.Format(time.RFC3339)
	for _, mint := range []struct{ path, body string }{
		{"/v1/sandboxes/" + sb.ID + "/runs/stream/token?ownerId=" + f.ws, `{"command":"ls"}`},
		{"/v1/sandboxes/" + sb.ID + "/files/download/token?ownerId=" + f.ws + "&path=/tmp/a", ""},
	} {
		code, body := f.do(t, http.MethodPost, mint.path, mint.body)
		if code != http.StatusConflict || errCode(body) != CodeSandboxNotRunning || !strings.Contains(body, "was terminated at "+at) {
			t.Fatalf("mint %s = %d %s, want 409 %s naming the end", mint.path, code, body, CodeSandboxNotRunning)
		}
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/resume?ownerId="+f.ws, ""); code != http.StatusConflict {
		t.Fatalf("resume of a terminated sandbox = %d %s, want 409", code, body)
	}
	if code, _, body := f.getSandbox(t, "id-b", sb.ID); code != http.StatusNotFound {
		t.Fatalf("another member's GET = %d %s, want 404 as while it lived", code, body)
	}

	// GraphQL and MCP read the same ended sandbox through the same service.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: f.svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:         schema,
		RequestString:  `query($id: String!) { sandbox(id: $id) { status terminatedAt } sandboxes(status: ["terminated"]) { id } }`,
		VariableValues: map[string]any{"id": sb.ID},
		Context:        callerCtx(),
	})
	data, _ := res.Data.(map[string]any)
	gqlBox, _ := data["sandbox"].(map[string]any)
	gqlList, _ := data["sandboxes"].([]any)
	if len(res.Errors) != 0 || gqlBox["status"] != "terminated" || gqlBox["terminatedAt"] != at || len(gqlList) != 1 {
		t.Fatalf("GraphQL ended sandbox = %#v (errors %v), want terminated at %s, listed once", res.Data, res.Errors, at)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
	f.svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := callerCtx()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	listed, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "list_sandboxes", Arguments: map[string]any{"status": []string{"terminated"}}})
	if err != nil || listed.IsError || !strings.Contains(listed.Content[0].(*mcp.TextContent).Text, sb.ID) {
		t.Fatalf("MCP list_sandboxes(status terminated) = %#v (err %v), want %s", listed, err, sb.ID)
	}
}

// An errored sandbox is refused a connect token as on Render, so the CLI says
// it errored instead of opening a stream into it.
func TestErroredSandboxRefusesConnectTokens(t *testing.T) {
	f := recordsFixture(t)
	f.addBox("os-failed", "Failed")
	code, body := f.do(t, http.MethodPost, "/v1/sandboxes/os-failed/runs/stream/token?ownerId="+f.ws, `{"command":"ls"}`)
	if code != http.StatusConflict || errCode(body) != CodeSandboxNotRunning || !strings.Contains(body, "errored") {
		t.Fatalf("mint on an errored sandbox = %d %s, want 409 %s", code, body, CodeSandboxNotRunning)
	}
}

// w32/017: OpenSandbox deletes a sandbox at its lifetime deadline, before
// bex's own reaper, and the sweeper is what records the end. A sandbox past
// its deadline ends at that deadline as soon as it is absent, however young
// its record; a young one not past its deadline is still a create the list
// may not show yet, and stays live.
func TestSweeperEndsAnExpiredSandboxAtItsDeadline(t *testing.T) {
	f := recordsFixture(t)
	code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"timeoutSeconds":60}`, f.ws))
	var short Sandbox
	if err := json.Unmarshal([]byte(body), &short); code != http.StatusCreated || err != nil || short.CreatedAt == nil {
		t.Fatalf("create 60 s sandbox = %d %s", code, body)
	}
	fresh := f.mustCreate(t, "")
	for _, sb := range []Sandbox{short, fresh} {
		substrate := f.os.idOf(sb.ID)
		f.os.mu.Lock()
		delete(f.os.boxes, substrate)
		f.os.mu.Unlock()
	}
	ctx := context.Background()
	f.now = f.now.Add(90 * time.Second)
	f.sweeper.Sweep(ctx)

	deadline := short.CreatedAt.Add(60 * time.Second)
	code, got, body := f.getSandbox(t, "id-a", short.ID)
	if code != http.StatusOK || got.Status != StatusTerminated || got.TerminatedAt == nil || !got.TerminatedAt.Equal(deadline) {
		t.Fatalf("GET 90 s after a 60 s create = %d %s, want 200 terminated at %s", code, body, deadline.Format(time.RFC3339Nano))
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+short.ID+"/runs/stream/token?ownerId="+f.ws, `{"command":"true"}`); code != http.StatusConflict ||
		!strings.Contains(body, "was terminated at "+deadline.UTC().Format(time.RFC3339)) {
		t.Fatalf("run mint = %d %s, want 409 naming the deadline", code, body)
	}
	if code, _, body := f.getSandbox(t, "id-a", fresh.ID); code != http.StatusNotFound {
		t.Fatalf("GET of a young absent sandbox before its deadline = %d %s, want 404 (its create may not be listed yet)", code, body)
	}
}

// The recorded deadline follows the sandbox's own: the end of the 30-day
// suspended window once a pause is accepted, the resume plus timeoutSeconds
// after a resume (w32/017, lifetimeDeadline).
func TestSuspendAndResumeMoveTheRecordedDeadline(t *testing.T) {
	f := recordsFixture(t)
	code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"timeoutSeconds":600}`, f.ws))
	var sb Sandbox
	if err := json.Unmarshal([]byte(body), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("create = %d %s", code, body)
	}
	expires := func() time.Time {
		t.Helper()
		var at *time.Time
		if err := f.st.Pool.QueryRow(context.Background(), `SELECT expires_at FROM sandbox_records WHERE workspace_id = $1 AND sandbox_id = $2`, f.ws, sb.ID).Scan(&at); err != nil || at == nil {
			t.Fatalf("expires_at = %v, %v", at, err)
		}
		return *at
	}
	if got, want := expires(), sb.CreatedAt.Add(600*time.Second); !got.Equal(want) {
		t.Fatalf("deadline at create = %s, want %s", got, want)
	}
	f.now = f.now.Add(time.Minute)
	f.suspend(t, sb)
	if got, want := expires(), f.now.Add(suspendedRetention); !got.Equal(want) {
		t.Fatalf("deadline once suspended = %s, want %s", got, want)
	}
	f.now = f.now.Add(time.Hour)
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/resume?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("resume = %d %s", code, body)
	}
	if got, want := expires(), f.now.Add(600*time.Second); !got.Equal(want) {
		t.Fatalf("deadline after resume = %s, want %s", got, want)
	}
}

// A sandbox OpenSandbox deleted on its own (its expireTime) is ended by the
// sweeper's next complete list, and a record ended more than 30 days ago is
// gone: the sandbox then answers 404.
func TestSweeperEndsAbsentSandboxesAndExpiresOldRecords(t *testing.T) {
	f := recordsFixture(t)
	sb := f.mustCreate(t, "")
	substrate := f.os.idOf(sb.ID)
	f.os.mu.Lock()
	delete(f.os.boxes, substrate)
	f.os.mu.Unlock()
	ctx := context.Background()
	if _, err := f.st.Pool.Exec(ctx, `UPDATE sandbox_records SET created_at = now() - interval '1 hour' WHERE workspace_id = $1`, f.ws); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Hour)
	f.sweeper.Sweep(ctx)
	code, got, body := f.getSandbox(t, "id-a", sb.ID)
	if code != http.StatusOK || got.Status != StatusTerminated || got.TerminatedAt == nil || !got.TerminatedAt.Equal(f.now) {
		t.Fatalf("GET after OpenSandbox deleted it = %d %s, want 200 terminated at the sweep", code, body)
	}
	f.now = f.now.Add(31 * 24 * time.Hour)
	f.sweeper.Sweep(ctx)
	if code, _, body := f.getSandbox(t, "id-a", sb.ID); code != http.StatusNotFound {
		t.Fatalf("GET 31 days after its end = %d %s, want 404", code, body)
	}
}
