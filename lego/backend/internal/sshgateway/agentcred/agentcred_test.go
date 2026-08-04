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

package agentcred

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type credentialPodResolver struct{ pods map[string]SessionPod }

func (r credentialPodResolver) ResolveSessionPod(_ context.Context, _, sourceIP string) (SessionPod, error) {
	return r.pods[sourceIP], nil
}

type credentialConnections struct{}

func (credentialConnections) GetGitConnection(_ context.Context, workspace string) (store.GitConnection, error) {
	return store.GitConnection{WorkspaceID: workspace, InstallationID: 42, AccountLogin: "octo"}, nil
}

type credentialGitHub struct{ calls int }

func (g *credentialGitHub) MintSessionInstallationToken(context.Context, int64, string) (github.InstallationToken, error) {
	g.calls++
	return github.InstallationToken{Token: "ghs_gateway_secret", ExpiresAt: time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)}, nil
}

type credentialAudit struct{ events []core.AuditEvent }

func (a *credentialAudit) Record(_ context.Context, event core.AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

func credentialLabels(t *testing.T, workspace string) map[string]string {
	t.Helper()
	labels, err := agentsession.BindingLabels("ags-one", "octo/repo", "bex-agent/task-1")
	if err != nil {
		t.Fatal(err)
	}
	labels[agentsession.LabelWorkspace] = workspace
	labels[agentsession.LabelRegime] = agentsession.RegimeSandbox
	return labels
}

func TestAgentCredentialGatewayBindsSourcePodAndProxiesSignedMint(t *testing.T) {
	fixed := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	secret := []byte("shared-domain-separated-secret")
	gh := &credentialGitHub{}
	apiAudit := &credentialAudit{}
	apiHandler := &agentsession.Handler{
		Secret: secret,
		Minter: &agentsession.Minter{GitHub: gh, Connections: credentialConnections{}, Audit: apiAudit, Now: func() time.Time { return fixed }},
		Now:    func() time.Time { return fixed },
	}
	apiServer := httptest.NewServer(apiHandler)
	defer apiServer.Close()

	gatewayAudit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sandbox-a", UID: "uid-a", Labels: credentialLabels(t, "tea-a")},
			"10.0.0.2": {Name: "sandbox-b", UID: "uid-b", Labels: credentialLabels(t, "tea-b")},
		}},
		API:   &agentsession.Client{URL: apiServer.URL, Secret: secret, HTTP: apiServer.Client(), Now: func() time.Time { return fixed }},
		Audit: gatewayAudit,
		Now:   func() time.Time { return fixed },
	}
	body, _ := json.Marshal(agentsession.MintRequest{SessionID: "ags-one", Repository: "octo/repo", Branch: "bex-agent/task-1"})
	request := httptest.NewRequest(http.MethodPost, agentsession.GatewayPath, bytes.NewReader(body))
	request.RemoteAddr = "10.0.0.1:43210"
	request.Header.Set(agentsession.NamespaceHeader, "tea-a-sandbox")
	recorder := httptest.NewRecorder()
	broker.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "ghs_gateway_secret") || recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("gateway response status=%d headers=%v body=%q", recorder.Code, recorder.Header(), recorder.Body.String())
	}
	if gh.calls != 1 || len(apiAudit.events) != 1 || len(gatewayAudit.events) != 1 {
		t.Fatalf("github calls=%d api audits=%d gateway audits=%d", gh.calls, len(apiAudit.events), len(gatewayAudit.events))
	}
	if apiAudit.events[0].Verb != agentsession.AuditVerbMintCredential || gatewayAudit.events[0].Verb != agentsession.AuditVerbProxyCredential {
		t.Fatalf("audit verbs api=%q gateway=%q", apiAudit.events[0].Verb, gatewayAudit.events[0].Verb)
	}

	// Sandbox B cannot claim A's namespace/workspace even when it knows every
	// public session binding value. The request never reaches bex-api/GitHub.
	request = httptest.NewRequest(http.MethodPost, agentsession.GatewayPath, bytes.NewReader(body))
	request.RemoteAddr = "10.0.0.2:43210"
	request.Header.Set(agentsession.NamespaceHeader, "tea-a-sandbox")
	recorder = httptest.NewRecorder()
	broker.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || gh.calls != 1 || strings.Contains(recorder.Body.String(), "ghs_gateway_secret") {
		t.Fatalf("cross-workspace status=%d calls=%d body=%q", recorder.Code, gh.calls, recorder.Body.String())
	}
}

func TestAgentCredentialInternalMintRejectsUnsignedRequest(t *testing.T) {
	handler := &agentsession.Handler{Secret: []byte("secret"), Minter: &agentsession.Minter{}}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, agentsession.InternalMintPath, strings.NewReader(`{}`)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned status = %d, want 401", recorder.Code)
	}
}
