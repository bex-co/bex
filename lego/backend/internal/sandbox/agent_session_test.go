package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestAgentSessionLifecyclePreservesReservedMetadata(t *testing.T) {
	var create createRequest
	var calls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes":
			if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
				t.Fatal(err)
			}
			_, _ = fmt.Fprint(w, `{"id":"sandbox-1","status":{"state":"Running"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/sandbox-1":
			_ = json.NewEncoder(w).Encode(osSandbox{ID: "sandbox-1", Metadata: create.Metadata})
		case r.Method == http.MethodPost && r.URL.Path == "/sandboxes/sandbox-1/resume":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/sandboxes/sandbox-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	svc := &Service{Base: &core.Base{}, Client: NewClient(upstream.URL), DefaultPlan: PlanStarter,
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}}}}
	lifecycle := NewAgentSessionLifecycle(svc)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	created, err := lifecycle.CreateAgentSessionSandbox(ctx, "tea-a", "agent", "ags-session")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "sandbox-1" || create.Metadata[metadataOwner] != "alice" || create.Metadata[metadataWorkspace] != "tea-a" ||
		create.Metadata[metadataRegime] != metadataSandboxRegime || create.Metadata[metadataNetworkPolicy] != string(NetworkPolicyDenyAll) ||
		create.Metadata[metadataAgentSession] != "ags-session" {
		t.Fatalf("reserved metadata = %#v", create.Metadata)
	}
	if err := lifecycle.ResumeAgentSessionSandbox(ctx, "tea-a", "ags-session", "sandbox-1"); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.CancelAgentSessionSandbox(ctx, "tea-a", "ags-session", "sandbox-1"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 5 { // create, get+resume, get+delete
		t.Fatalf("lifecycle calls = %v", calls)
	}
}
