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

package agentsession

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeModelKeys struct {
	data map[string]map[string]string
	err  error
}

func (f fakeModelKeys) Get(_ context.Context, path string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.data[path], nil
}
func (fakeModelKeys) Put(context.Context, string, map[string]string) error { return nil }
func (fakeModelKeys) Delete(context.Context, string) error                 { return nil }
func (fakeModelKeys) List(context.Context, string) ([]string, error)       { return nil, nil }

func modelSession(endpoint string) store.AgentSession {
	return store.AgentSession{
		ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running",
		AgentConfig: json.RawMessage(`{"modelEndpoint":"` + endpoint + `"}`),
	}
}

func modelKeysFor(secret string) fakeModelKeys {
	return fakeModelKeys{data: map[string]map[string]string{
		ModelKeySecretPath("tea-a"): {ModelKeyField: secret},
	}}
}

func validModelRequest() ModelMintRequest {
	return ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "sandbox-one", PodUID: "uid-one"}
}

func TestModelMinterInjectsSchemeByProviderAndAudits(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		secret   string
		wantHost string
		wantSch  string
	}{
		{"anthropic api key", "https://api.anthropic.com/v1", "sk-ant-api03-xyz", "api.anthropic.com", AuthSchemeAnthropicKey},
		{"anthropic oauth", "https://api.anthropic.com/v1", "sk-ant-oat01-abc", "api.anthropic.com", AuthSchemeAnthropicOAuth},
		{"openai bearer", "https://api.openai.com/v1", "sk-proj-openai", "api.openai.com", AuthSchemeBearer},
		{"gemini key", "https://generativelanguage.googleapis.com/v1beta", "AIzaGemini", "generativelanguage.googleapis.com", AuthSchemeGoogleKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			audit := &auditRecorder{}
			m := &ModelMinter{Keys: modelKeysFor(tc.secret), Sessions: fakeSessions{session: modelSession(tc.endpoint)}, Audit: audit}
			got, err := m.Mint(context.Background(), validModelRequest())
			if err != nil {
				t.Fatal(err)
			}
			if got.Credential != tc.secret || got.EndpointHost != tc.wantHost || got.Scheme != tc.wantSch {
				t.Fatalf("response = %+v, want host=%q scheme=%q", got, tc.wantHost, tc.wantSch)
			}
			if len(audit.events) != 1 {
				t.Fatalf("audit events = %d, want 1", len(audit.events))
			}
			ev := audit.events[0]
			// The audit event carries the non-secret vendor host, never the credential.
			if ev.Verb != AuditVerbMintModelCredential || ev.Resource != "workspace:tea-a" || ev.Target != "agent-session:ags-one" || ev.TargetName != tc.wantHost || ev.Outcome != core.AuditAllowed {
				t.Fatalf("audit event = %+v", ev)
			}
			if ev.TargetName == tc.secret {
				t.Fatal("audit leaked the credential")
			}
		})
	}
}

func TestModelMinterRefusesUnprovisionedKey(t *testing.T) {
	audit := &auditRecorder{}
	// The workspace has no key row at all: Get returns an empty map (SecretKV's
	// documented not-found behavior), which must refuse, not forward keyless.
	m := &ModelMinter{Keys: fakeModelKeys{}, Sessions: fakeSessions{session: modelSession("https://api.anthropic.com/v1")}, Audit: audit}
	_, err := m.Mint(context.Background(), validModelRequest())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Mint error = %v, want forbidden", err)
	}
	if len(audit.events) != 1 || audit.events[0].Outcome != core.AuditDenied {
		t.Fatalf("denied audit = %+v", audit.events)
	}
}

func TestModelMinterFailsClosedOnStoreError(t *testing.T) {
	// A genuine OpenBao failure must fail closed, distinctly from an unprovisioned
	// key, so the proxy returns 502 (broker unavailable) not 403.
	m := &ModelMinter{Keys: fakeModelKeys{err: errors.New("openbao unreachable")}, Sessions: fakeSessions{session: modelSession("https://api.anthropic.com/v1")}, Audit: &auditRecorder{}}
	_, err := m.Mint(context.Background(), validModelRequest())
	if !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Fatalf("Mint error = %v, want ErrSecretsUnavailable", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Fatal("a store outage must not read as forbidden (would mask the outage)")
	}
}

func TestModelMinterRefusesTerminalOrForeignSession(t *testing.T) {
	// Same live-lifecycle gate as the Git minter: a retained terminal/canceling
	// sandbox, a foreign workspace, a cleared sandbox, or an absent session gets
	// no model credential even though the pod identity is intact.
	base := func(mut func(*store.AgentSession)) fakeSessions {
		s := modelSession("https://api.anthropic.com/v1")
		mut(&s)
		return fakeSessions{session: s}
	}
	cases := map[string]fakeSessions{
		"completed":         base(func(s *store.AgentSession) { s.Phase = "completed" }),
		"failed":            base(func(s *store.AgentSession) { s.Phase = "failed" }),
		"canceled":          base(func(s *store.AgentSession) { s.Phase = "canceled" }),
		"canceling":         base(func(s *store.AgentSession) { s.Phase = "canceling" }),
		"sandbox cleared":   base(func(s *store.AgentSession) { s.SandboxID = "" }),
		"foreign workspace": base(func(s *store.AgentSession) { s.WorkspaceID = "tea-b" }),
		"absent session":    {err: store.ErrNotFound},
	}
	for name, sessions := range cases {
		t.Run(name, func(t *testing.T) {
			m := &ModelMinter{Keys: modelKeysFor("sk-ant-api03-xyz"), Sessions: sessions, Audit: &auditRecorder{}}
			_, err := m.Mint(context.Background(), validModelRequest())
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("Mint error = %v, want forbidden", err)
			}
		})
	}
}

func TestModelMinterRejectsSSRFEndpoint(t *testing.T) {
	// A stored endpoint that somehow points at a private/cluster-local host must be
	// refused at credential-release time (defense in depth over the create gate).
	for _, endpoint := range []string{"https://10.0.0.5/v1", "https://bex-api.bex-system.svc.cluster.local/v1", "http://api.anthropic.com/v1"} {
		m := &ModelMinter{Keys: modelKeysFor("sk-ant-api03-xyz"), Sessions: fakeSessions{session: modelSession(endpoint)}, Audit: &auditRecorder{}}
		if _, err := m.Mint(context.Background(), validModelRequest()); err == nil {
			t.Fatalf("endpoint %q was accepted, want rejected", endpoint)
		}
	}
}

func TestAuthorizeSessionPodBindsExactSession(t *testing.T) {
	labels := map[string]string{
		LabelWorkspace: "tea-a", LabelRegime: RegimeSandbox, LabelSession: "ags-one",
	}
	got, err := AuthorizeSessionPod("tea-a-sandbox", "pod-1", "uid-1", labels, "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	if got != (ModelMintRequest{SessionID: "ags-one", Workspace: "tea-a", PodName: "pod-1", PodUID: "uid-1"}) {
		t.Fatalf("request = %+v", got)
	}
	// Every mismatch a sibling/forged pod could present must be refused.
	deny := []struct {
		name              string
		namespace, sessID string
		labels            map[string]string
	}{
		{"sibling session", "tea-a-sandbox", "ags-one", map[string]string{LabelWorkspace: "tea-a", LabelRegime: RegimeSandbox, LabelSession: "ags-other"}},
		{"wrong namespace", "tea-b-sandbox", "ags-one", labels},
		{"not sandbox regime", "tea-a-sandbox", "ags-one", map[string]string{LabelWorkspace: "tea-a", LabelRegime: "hosting", LabelSession: "ags-one"}},
		{"no workspace label", "tea-a-sandbox", "ags-one", map[string]string{LabelRegime: RegimeSandbox, LabelSession: "ags-one"}},
		{"empty session claim", "tea-a-sandbox", "", labels},
	}
	for _, tc := range deny {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := AuthorizeSessionPod(tc.namespace, "pod-1", "uid-1", tc.labels, tc.sessID); !errors.Is(err, ErrForbidden) {
				t.Fatalf("AuthorizeSessionPod = %v, want forbidden", err)
			}
		})
	}
}

func TestModelProxyURLRoundTrips(t *testing.T) {
	url, err := ModelProxyURL("http://gw:8084", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	// The agent appends its own subpath to this base; simulate `/v1/messages`.
	ns, sess, sub, err := ParseModelProxyPath(strings.TrimPrefix(url, "http://gw:8084") + "/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	if ns != "tea-a-sandbox" || sess != "ags-one" || sub != "/v1/messages" {
		t.Fatalf("parsed ns=%q sess=%q sub=%q", ns, sess, sub)
	}
}

func TestParseModelProxyPath(t *testing.T) {
	url, err := ModelProxyURL("http://gw", "tea-a-sandbox", "ags-one")
	if err != nil {
		t.Fatal(err)
	}
	// Path with no vendor subpath ⇒ "/".
	ns, sess, sub, err := ParseModelProxyPath(strings.TrimPrefix(url, "http://gw"))
	if err != nil || ns != "tea-a-sandbox" || sess != "ags-one" || sub != "/" {
		t.Fatalf("bare base parse: ns=%q sess=%q sub=%q err=%v", ns, sess, sub, err)
	}
	for _, bad := range []string{"/wrong/a/b", "/model/", "/model/@@@/ags", "/model/dGVh"} {
		if _, _, _, err := ParseModelProxyPath(bad); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("ParseModelProxyPath(%q) err = %v, want invalid", bad, err)
		}
	}
}
