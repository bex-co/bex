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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ADR113 D7 server facts: a sandbox create records one created product
// activity row with the surface that created it.

func createdServer(t *testing.T) *Service {
	t.Helper()
	return stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Creating"}}`))
	})
}

func recordActivity(svc *Service, err error) *[]core.ProductActivity {
	var events []core.ProductActivity
	svc.ProductActivity = func(_ context.Context, e core.ProductActivity) error {
		events = append(events, e)
		return err
	}
	return &events
}

func originCtx(transport, userAgent string) context.Context {
	return core.WithRequestOrigin(callerCtx(), core.RequestOrigin{Transport: transport, UserAgent: userAgent})
}

func TestSandboxCreateOverRESTFromTheCLIRecordsCLI(t *testing.T) {
	svc := createdServer(t)
	events := recordActivity(svc, nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(`{"template":"node"}`)).
		WithContext(originCtx(core.TransportREST, "render-cli/2.36.0 (Linux - 6.8)"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	var box Sandbox
	if err := json.Unmarshal(rec.Body.Bytes(), &box); err != nil {
		t.Fatal(err)
	}
	want := core.ProductActivity{WorkspaceID: "tea-a", ResourceID: box.ID, ResourceType: "sandbox", EventType: "created", Surface: core.SurfaceCLI}
	if len(*events) != 1 || !sameFact((*events)[0], want) || (*events)[0].ActorID != "id-a" {
		t.Fatalf("product activity = %+v, want one %+v by id-a", *events, want)
	}
}

func TestSandboxCreateOverMCPRecordsMCP(t *testing.T) {
	svc := createdServer(t)
	events := recordActivity(svc, nil)
	server := mcp.NewServer(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := originCtx(core.TransportMCP, "")
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "sandbox-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_sandbox", Arguments: map[string]any{"template": "node"}})
	if err != nil || res.IsError {
		t.Fatalf("MCP = %+v err=%v", res, err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var box Sandbox
	if err := json.Unmarshal(raw, &box); err != nil {
		t.Fatal(err)
	}
	want := core.ProductActivity{WorkspaceID: "tea-a", ResourceID: box.ID, ResourceType: "sandbox", EventType: "created", Surface: core.SurfaceMCP}
	if len(*events) != 1 || !sameFact((*events)[0], want) {
		t.Fatalf("product activity = %+v, want one %+v", *events, want)
	}
}

// A sandbox that was never created records nothing, and a recorder that fails
// never fails a create.
func TestSandboxCreateRecordsOnlyACreatedSandboxAndNeverFailsOnAnalytics(t *testing.T) {
	refused := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte(`{"code":"KUBERNETES::POD_READY_TIMEOUT","message":"sandbox pod not ready within 300s"}`))
	})
	events := recordActivity(refused, nil)
	if _, err := refused.Create(originCtx(core.TransportREST, "render-cli/2.36.0"), CreateRequest{Template: "node"}); err == nil {
		t.Fatal("create succeeded against a refusing server")
	}
	if len(*events) != 0 {
		t.Fatalf("a failed create recorded %+v", *events)
	}

	svc := createdServer(t)
	events = recordActivity(svc, errors.New("product analytics unavailable"))
	box, err := svc.Create(originCtx(core.TransportREST, "render-cli/2.36.0"), CreateRequest{Template: "node"})
	if err != nil || !strings.HasPrefix(box.ID, "sbx-") {
		t.Fatalf("create = %+v err=%v, want success despite the recorder", box, err)
	}
	if len(*events) != 1 || (*events)[0].ResourceID != box.ID {
		t.Fatalf("product activity = %+v, want the one attempted record", *events)
	}
}

// sameFact compares the fields a creation fact is defined by.
func sameFact(got, want core.ProductActivity) bool {
	return got.WorkspaceID == want.WorkspaceID && got.ResourceID == want.ResourceID &&
		got.ResourceType == want.ResourceType && got.EventType == want.EventType && got.Surface == want.Surface
}
