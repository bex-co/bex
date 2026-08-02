package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
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
	eg := &fakeSessionEgress{}
	svc.SessionEgress = eg
	lifecycle := NewAgentSessionLifecycle(svc)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	created, err := lifecycle.CreateAgentSessionSandbox(ctx, "tea-a", "agent", "ags-session", "bex-co/example", "bex-agent/session-test", "https://api.openai.com/v1", []string{"docs.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := agentsession.BindingLabels("ags-session", "bex-co/example", "bex-agent/session-test")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "sandbox-1" || create.Metadata[metadataOwner] != "alice" || create.Metadata[metadataWorkspace] != "tea-a" ||
		create.Metadata[metadataRegime] != metadataSandboxRegime || create.Metadata[metadataNetworkPolicy] != string(NetworkPolicyDenyAll) ||
		create.Metadata[metadataAgentSession] != "ags-session" || create.Metadata[metadataModelEndpoint] != "https://api.openai.com/v1" ||
		create.Metadata[metadataEgressAllow] != `["docs.example.com"]` || create.Metadata[agentsession.LabelRepository] != bindings[agentsession.LabelRepository] ||
		create.Metadata[agentsession.LabelBranch] != bindings[agentsession.LabelBranch] {
		t.Fatalf("reserved metadata = %#v", create.Metadata)
	}
	if err := lifecycle.EnterAgentSessionPhase(ctx, "tea-a", "ags-session", "sandbox-1"); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.ResumeAgentSessionSandbox(ctx, "tea-a", "ags-session", "sandbox-1"); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.CancelAgentSessionSandbox(ctx, "tea-a", "ags-session", "sandbox-1"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 6 { // create, get+transition, get+resume, get+delete
		t.Fatalf("lifecycle calls = %v", calls)
	}
	if len(eg.calls) != 3 || eg.calls[0].op != "setup" || eg.calls[1].op != "agent" || eg.calls[2].op != "delete" {
		t.Fatalf("egress lifecycle = %#v", eg.calls)
	}
}
