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
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

// w9/m94: the pinned CLI parses a sandbox id client-side (`ea sandboxes copy`
// requires ^sbx-[A-Za-z0-9]+), so every id bex hands a tenant must carry the
// prefix. Production used to return the substrate's bare UUID, which the client
// refused before sending anything.
var publicIDShape = regexp.MustCompile(`^sbx-[0-9a-v]{20}$`)

// ownedMeta is a fully hardened sandbox's metadata, optionally carrying a
// public id, rendered as the OpenSandbox server's JSON.
func ownedMeta(publicID string) string {
	meta := `"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"`
	if publicID != "" {
		meta += `,"bex.co/sandbox-id":"` + publicID + `"`
	}
	return meta
}

func osSandboxJSON(substrateID, publicID string) string {
	return `{"id":"` + substrateID + `","metadata":{` + ownedMeta(publicID) + `},"status":{"state":"Running"}}`
}

// TestCreateMintsPublicIDAndStampsIt proves the EA create path mints the id
// itself (the substrate's id never reaches the caller) and persists it as the
// metadata every read path resolves through.
func TestCreateMintsPublicIDAndStampsIt(t *testing.T) {
	var sent createRequest
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("decode create: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"c372c97e-980b-4573-9aae-4c82b8ad1c4b","status":{"state":"Creating"}}`))
	})
	box, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: PlanStarter})
	if err != nil {
		t.Fatal(err)
	}
	if !publicIDShape.MatchString(box.ID) {
		t.Errorf("created id = %q, want a bex sbx- id", box.ID)
	}
	if got := sent.Metadata[metadataPublicID]; got != box.ID {
		t.Errorf("stamped metadata id = %q, want the returned id %q", got, box.ID)
	}
}

// TestPublicIDResolvesOnEveryReadPath is the dual-accept contract: list and get
// report the canonical id, a stamped sandbox answers to BOTH its public and its
// substrate id, and an unstamped (legacy or agent-session) one keeps answering
// to the substrate id it was minted with.
func TestPublicIDResolvesOnEveryReadPath(t *testing.T) {
	const publicID = "sbx-daoco4pjg4r481aljit0"
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			_, _ = w.Write([]byte(`[` +
				osSandboxJSON("os-new", publicID) + `,` +
				osSandboxJSON("os-legacy", "") + `]`))
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-new":
			_, _ = w.Write([]byte(osSandboxJSON("os-new", publicID)))
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-legacy":
			_, _ = w.Write([]byte(osSandboxJSON("os-legacy", "")))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/sandboxes/"):
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/sandboxes/"))
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}, Client: NewClient(srv.URL)}

	list, err := svc.List(callerCtx())
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if list[0].ID != publicID {
		t.Errorf("stamped sandbox lists as %q, want %q", list[0].ID, publicID)
	}
	if list[1].ID != "os-legacy" {
		t.Errorf("unstamped sandbox lists as %q, want its substrate id", list[1].ID)
	}

	for _, addressed := range []string{publicID, "os-new"} {
		got, err := svc.Get(callerCtx(), addressed)
		if err != nil {
			t.Fatalf("get %q: %v", addressed, err)
		}
		if got.ID != publicID {
			t.Errorf("get %q reported id %q, want the canonical %q", addressed, got.ID, publicID)
		}
	}

	// Terminate through the public id must delete the SUBSTRATE object.
	if err := svc.Terminate(callerCtx(), publicID); err != nil {
		t.Fatalf("terminate by public id: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "os-new" {
		t.Errorf("deleted %v, want the substrate id os-new", deleted)
	}
}

// TestUnknownPublicIDIsNonEnumeratingNotFound covers the failure neighbors: an
// id that matches no sandbox, and one belonging to another workspace's object,
// both answer with the same named 404 — never a 500 and never a distinguishable
// existence signal.
func TestUnknownPublicIDIsNonEnumeratingNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/sandboxes" {
			// One foreign-workspace object, stamped: visible to the substrate
			// list but never addressable by this caller.
			_, _ = w.Write([]byte(`[{"id":"os-foreign","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-other","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox","bex.co/sandbox-id":"sbx-daoco4pjg4r481aljit1"},"status":{"state":"Running"}}]`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}, Client: NewClient(srv.URL)}

	for _, id := range []string{
		"sbx-daoco4pjg4r481aljitz",             // never minted
		"sbx-daoco4pjg4r481aljit1",             // another workspace's sandbox
		"c372c97e-980b-4573-9aae-4c82b8ad1c4b", // a legacy-shaped id that is gone
	} {
		if _, err := svc.Get(callerCtx(), id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("get %q = %v, want a non-enumerating not found", id, err)
		}
		if err := svc.Terminate(callerCtx(), id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("terminate %q = %v, want a non-enumerating not found", id, err)
		}
	}
}

// TestExecTicketBindsSubstrateID pins the one place the two id forms must NOT
// be interchangeable: the gateway derives the pod name as `<id>-0`, so a ticket
// minted for a caller-supplied public id would address a pod that cannot exist.
func TestExecTicketBindsSubstrateID(t *testing.T) {
	const publicID = "sbx-daoco4pjg4r481aljit0"
	secret := []byte("exec-secret-exec-secret-exec-sec")
	var ticket string
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticket = r.Header.Get(sandboxexec.TicketHeader)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(gateway.Close)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/sandboxes" {
			_, _ = w.Write([]byte(`[` + osSandboxJSON("os-new", publicID) + `]`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client: NewClient(srv.URL),
		Exec:   &ExecConfig{Secret: secret, GatewayURL: gateway.URL, Client: gateway.Client()},
	}
	resp, err := svc.dialGateway(callerCtx(), ExecRequest{SandboxID: publicID, Command: "true"})
	if err != nil {
		t.Fatalf("exec by public id: %v", err)
	}
	_ = resp.Body.Close()

	claims, err := sandboxexec.Verify(secret, ticket, time.Now())
	if err != nil {
		t.Fatalf("verify minted ticket: %v", err)
	}
	if claims.SandboxID != "os-new" {
		t.Errorf("ticket sandbox id = %q, want the substrate id os-new", claims.SandboxID)
	}
	if claims.PodName() != "os-new-0" {
		t.Errorf("ticket pod = %q, want os-new-0", claims.PodName())
	}
}

// TestAgentSessionSandboxKeepsSubstrateIdentity pins the deliberate carve-out
// documented on canonicalID: the trusted agent-session create must NOT stamp a
// public id, because the platform names its pod and its usage rows after the
// substrate id it returns.
func TestAgentSessionSandboxKeepsSubstrateIdentity(t *testing.T) {
	var sent createRequest
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("decode create: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-session","status":{"state":"Running"}}`))
	})
	svc.SessionEgress = &fakeSessionEgress{}
	box, err := NewAgentSessionLifecycle(svc).CreateAgentSessionSandbox(
		context.Background(), "tea-a", "node", "ags-one", "bex-co/example", "bex-agent/session-test",
		"https://api.openai.com/v1", agentsession.ModelKeyPlaceholder("ags-one"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if box.ID != "os-session" {
		t.Errorf("agent-session sandbox id = %q, want the substrate id", box.ID)
	}
	if got, ok := sent.Metadata[metadataPublicID]; ok {
		t.Errorf("agent-session create stamped a public id %q; the pod name depends on the substrate id", got)
	}
}

// TestCanonicalIDAgreesAcrossRESTGraphQLMCP is the m94 Render-parity evidence:
// one stamped sandbox reads with the identical `sbx-` id on all three surfaces
// (REST get, GraphQL query, MCP list), so a client never has to learn which
// surface speaks which id.
func TestCanonicalIDAgreesAcrossRESTGraphQLMCP(t *testing.T) {
	const publicID = "sbx-daoco4pjg4r481aljit0"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sandboxes":
			_, _ = w.Write([]byte(`[` + osSandboxJSON("os-new", publicID) + `]`))
		case r.URL.Path == "/sandboxes/os-new":
			_, _ = w.Write([]byte(osSandboxJSON("os-new", publicID)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}, Client: NewClient(upstream.URL)}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sandboxes/"+publicID, nil).WithContext(callerCtx()))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST get = %d %s", rec.Code, rec.Body.String())
	}
	var rest Sandbox
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatalf("REST decode: %v", err)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ sandboxes { id } }`, Context: callerCtx()})
	if len(res.Errors) != 0 {
		t.Fatalf("GraphQL errors = %#v", res.Errors)
	}
	boxes := res.Data.(map[string]any)["sandboxes"].([]any)
	if len(boxes) != 1 {
		t.Fatalf("GraphQL sandboxes = %#v", boxes)
	}
	gqlID, _ := boxes[0].(map[string]any)["id"].(string)

	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
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
	toolRes, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "list_sandboxes"})
	if err != nil || toolRes.IsError {
		t.Fatalf("MCP list = %#v, %v", toolRes, err)
	}
	rawMCP, err := json.Marshal(toolRes.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	mcpID := firstSandboxID(t, rawMCP)

	if rest.ID != publicID || gqlID != publicID || mcpID != publicID {
		t.Errorf("ids disagree: REST %q, GraphQL %q, MCP %q; want %q", rest.ID, gqlID, mcpID, publicID)
	}
}

// firstSandboxID pulls the one listed sandbox's id out of the MCP tool's
// structured content, whatever envelope shape the tool wraps it in.
func firstSandboxID(t *testing.T, raw []byte) string {
	t.Helper()
	var envelope struct {
		Sandboxes []Sandbox `json:"sandboxes"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Sandboxes) == 1 {
		return envelope.Sandboxes[0].ID
	}
	var list []Sandbox
	if err := json.Unmarshal(raw, &list); err == nil && len(list) == 1 {
		return list[0].ID
	}
	t.Fatalf("MCP list payload not understood: %s", raw)
	return ""
}
