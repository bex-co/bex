package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"k8s.io/apimachinery/pkg/util/validation"
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
		case r.Method == http.MethodDelete && r.URL.Path == "/sandboxes/sandbox-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	// DefaultPlan is the Render sandbox default (1c-2g): an agent session's
	// sandbox must not take it (w8/247, asserted below).
	svc := &Service{Base: &core.Base{}, Client: NewClient(upstream.URL), DefaultPlan: DefaultPlan,
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}, CPU: "2", Memory: "4Gi", SessionOnly: true}}}
	eg := &fakeSessionEgress{}
	svc.SessionEgress = eg
	lifecycle := NewAgentSessionLifecycle(svc)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	placeholder := agentsession.ModelKeyPlaceholder("ags-session")
	created, err := lifecycle.CreateAgentSessionSandbox(ctx, AgentSessionSandbox{WorkspaceID: "tea-a", Template: "agent", SessionID: "ags-session", Repository: "bex-co/example", Branch: "bex-agent/session-test", ModelEndpoint: "https://api.openai.com/v1", ModelAPIKey: placeholder, Egress: sessionegress.Egress{Hosts: []string{"docs.example.com"}}, DriverEnv: map[string]string{"BEX_AGENT_PROMPT": "do it", "BEX_AGENT_TURN": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := agentsession.BindingLabels("ags-session", "bex-co/example", "bex-agent/session-test")
	if err != nil {
		t.Fatal(err)
	}
	if create.Metadata[agentsession.LabelDispatchTurn] != "2" || created.ID != "sandbox-1" || create.Metadata[metadataOwner] != "alice" || create.Metadata[metadataWorkspace] != "tea-a" ||
		create.Metadata[metadataRegime] != metadataSandboxRegime || create.Metadata[metadataNetworkPolicy] != string(NetworkPolicyDenyAll) ||
		create.Metadata[metadataAgentSession] != "ags-session" || create.Metadata[agentsession.LabelRepository] != bindings[agentsession.LabelRepository] ||
		create.Metadata[agentsession.LabelBranch] != bindings[agentsession.LabelBranch] {
		t.Fatalf("reserved metadata = %#v", create.Metadata)
	}
	// An agent session's sandbox is labeled with its template's size whatever
	// the Render sandbox default is (w8/247, w6/m565).
	if create.Metadata[metadataPlan] != string(Plan2C4G) || created.Plan != Plan2C4G {
		t.Fatalf("agent-session sandbox plan = metadata %q / %q, want 2c-4g under a %s default", create.Metadata[metadataPlan], created.Plan, DefaultPlan)
	}
	// Regression guard (w3/m43, caught live on prod): OpenSandbox persists
	// metadata as Kubernetes LABELS, so no value may contain a label-invalid
	// character. The egress allowlist (JSON) and model endpoint (URL) used to be
	// stamped here as `[]`/`https://…` and were rejected by real OpenSandbox with
	// SANDBOX::INVALID_METADATA_LABEL; they must not reappear in metadata.
	for k, v := range create.Metadata {
		if len(validation.IsValidLabelValue(v)) != 0 {
			t.Fatalf("sandbox metadata %q=%q is not a valid k8s label value", k, v)
		}
	}
	// Only the session-bound proxy placeholder enters pod env; the reusable BYO
	// key remains behind the gateway and neither value enters metadata.
	if create.Env[ModelAPIKeyEnvVar] != placeholder {
		t.Fatalf("create env[%s] = %q, want the proxy placeholder", ModelAPIKeyEnvVar, create.Env[ModelAPIKeyEnvVar])
	}
	for _, v := range create.Metadata {
		if v == placeholder {
			t.Fatal("model proxy placeholder leaked into sandbox metadata")
		}
	}
	if err := lifecycle.EnterAgentSessionPhase(ctx, "tea-a", "ags-session", "sandbox-1", "https://api.openai.com/v1", sessionegress.Egress{Hosts: []string{"docs.example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.CancelAgentSessionSandbox(ctx, "tea-a", "ags-session", "sandbox-1"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 { // create, get+transition, get+delete
		t.Fatalf("lifecycle calls = %v", calls)
	}
	if len(eg.calls) != 3 || eg.calls[0].op != "setup" || eg.calls[1].op != "agent" || eg.calls[2].op != "delete" {
		t.Fatalf("egress lifecycle = %#v", eg.calls)
	}
}

// An agent sandbox's plan, charge label and usage tier name its template's
// size (w6/m565): every one used to read starter, Render's half-CPU plan, so a
// 4 CPU / 8 GiB sandbox's charge said the smallest size there is. Its weighted
// charge is the template's, as before.
func TestAgentSandboxIsLabeledWithItsTemplateSize(t *testing.T) {
	const image = "ghcr.io/bex-co/bex-agent-sandbox:v2"
	sized := func(plan Plan) Template {
		cpu, memory, _ := PlanLimits(plan)
		return Template{Image: image, Entrypoint: []string{"driver"}, CPU: cpu, Memory: memory, SessionOnly: true}
	}
	for _, tc := range []struct {
		template string
		tmpl     Template
		plan     Plan
		weight   string
	}{
		{"agent", sized(Plan2C4G), Plan2C4G, "2422"},
		{"agent-4c-8g", sized(Plan4C8G), Plan4C8G, "4845"},
		// A template that names no size runs at half a CPU and 512Mi, which is
		// starter's size.
		{"agent-default", Template{Image: image, Entrypoint: []string{"driver"}, SessionOnly: true}, PlanStarter, "553"},
		// A size no plan has names no plan rather than a wrong one.
		{"agent-odd", Template{Image: image, Entrypoint: []string{"driver"}, CPU: "3", Memory: "6Gi", SessionOnly: true}, "", "3634"},
	} {
		t.Run(tc.template, func(t *testing.T) {
			var create createRequest
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
					t.Fatal(err)
				}
				_, _ = fmt.Fprint(w, `{"id":"sandbox-1","status":{"state":"Running"}}`)
			}))
			t.Cleanup(upstream.Close)
			meter := &fakeMeterStore{}
			svc := &Service{Base: &core.Base{}, Client: NewClient(upstream.URL), DefaultPlan: DefaultPlan,
				Templates: map[string]Template{tc.template: tc.tmpl}, Meter: &Meter{Store: meter}}
			svc.SessionEgress = &fakeSessionEgress{}
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
			created, err := NewAgentSessionLifecycle(svc).CreateAgentSessionSandbox(ctx, AgentSessionSandbox{WorkspaceID: "tea-a", Template: tc.template, SessionID: "ags-session",
				Repository: "bex-co/example", Branch: "bex-agent/session-test", ModelEndpoint: "https://api.openai.com/v1", ModelAPIKey: agentsession.ModelKeyPlaceholder("ags-session")})
			if err != nil {
				t.Fatal(err)
			}
			if create.Metadata[metadataPlan] != string(tc.plan) || created.Plan != tc.plan {
				t.Errorf("plan = metadata %q / %q, want %q", create.Metadata[metadataPlan], created.Plan, tc.plan)
			}
			if create.Metadata[metadataComputeWeight] != tc.weight {
				t.Errorf("weight = %q, want %s", create.Metadata[metadataComputeWeight], tc.weight)
			}
			wantLabel := image
			if tc.plan != "" {
				wantLabel = string(tc.plan) + " · " + image
			}
			if got := meter.labels["tea-a/sandbox-1"]; got != wantLabel {
				t.Errorf("charge label = %q, want %q", got, wantLabel)
			}
			if len(meter.observations) != 1 || meter.observations[0].Tier != string(tc.plan) || fmt.Sprint(meter.observations[0].WeightMilli) != tc.weight {
				t.Errorf("meter observations = %+v, want one at tier %q and weight %s", meter.observations, tc.plan, tc.weight)
			}
		})
	}
}

func TestAgentSessionLifecycleRejectsReusableOrMissingModelCredential(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))
	t.Cleanup(upstream.Close)

	svc := &Service{Base: &core.Base{}, Client: NewClient(upstream.URL), DefaultPlan: PlanStarter,
		Templates: map[string]Template{"agent": {Image: "bex/agent:1", Entrypoint: []string{"driver"}}}}
	svc.SessionEgress = &fakeSessionEgress{}
	lifecycle := NewAgentSessionLifecycle(svc)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	for _, credential := range []string{"", "sk-tenant-secret", agentsession.ModelKeyPlaceholder("ags-other")} {
		if _, err := lifecycle.CreateAgentSessionSandbox(ctx, AgentSessionSandbox{WorkspaceID: "tea-a", Template: "agent", SessionID: "ags-session", Repository: "bex-co/example", Branch: "bex-agent/session-test", ModelEndpoint: "https://api.openai.com/v1", ModelAPIKey: credential}); !errors.Is(err, core.ErrAgentSessionsUnavailable) {
			t.Fatalf("credential %q error = %v, want agent sessions unavailable", credential, err)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid credential reached OpenSandbox %d time(s)", calls)
	}
}

// The templates a session can name are the session-only ones, smallest
// memory first, each sized in a person's words (w10/017).
func TestAgentTemplatesListTheSessionOnlyOnesSmallestFirst(t *testing.T) {
	svc := &Service{Templates: map[string]Template{
		"base":      {CPU: "1", Memory: "2Gi"},
		"big":       {CPU: "4", Memory: "8Gi", SessionOnly: true},
		"agent":     {CPU: "2", Memory: "4Gi", SessionOnly: true},
		"half":      {CPU: "500m", Memory: "512Mi", SessionOnly: true},
		"also-half": {CPU: "500m", Memory: "512Mi", SessionOnly: true},
	}}
	got := NewAgentSessionLifecycle(svc).AgentTemplates()
	var words []string
	for _, tmpl := range got {
		words = append(words, tmpl.Name+": "+tmpl.Size())
	}
	want := []string{"also-half: 0.5 CPU, 0.5 GB", "half: 0.5 CPU, 0.5 GB", "agent: 2 CPU, 4 GB", "big: 4 CPU, 8 GB"}
	if fmt.Sprint(words) != fmt.Sprint(want) {
		t.Fatalf("AgentTemplates() = %q, want %q", words, want)
	}
	if !got[3].HasMoreMemory(got[2]) || got[2].HasMoreMemory(got[3]) || got[1].HasMoreMemory(got[0]) {
		t.Fatalf("HasMoreMemory does not order %v by memory", got)
	}
}
