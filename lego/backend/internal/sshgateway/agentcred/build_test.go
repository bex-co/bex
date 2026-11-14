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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const proxyBuildID = "arb-0123456789abcdefghij"

type proxyBuilds struct{}

func (proxyBuilds) GetAgentRepoEnvironmentBuildBinding(_ context.Context, id string) (store.AgentRepoEnvironmentBuildBinding, error) {
	if id != proxyBuildID {
		return store.AgentRepoEnvironmentBuildBinding{}, store.ErrNotFound
	}
	return store.AgentRepoEnvironmentBuildBinding{ID: id, WorkspaceID: "tea-a", Repo: "octo/repo", Status: store.AgentRepoEnvironmentBuildBuilding, SandboxID: "builder-1"}, nil
}

// ADR111: a repository environment build's builder clones and fetches its
// repository through the proxy, and the gateway refuses its every push and
// pull-request call before admitting it.
func TestGitProxyBuildPrincipalOnlyClones(t *testing.T) {
	secret := []byte("shared-domain-separated-secret")
	gh := &credentialGitHub{}
	apiServer := httptest.NewServer(&agentsession.Handler{Secret: secret, Minter: &agentsession.Minter{
		GitHub: gh, Connections: credentialConnections{}, Sessions: credentialSessions{}, Builds: proxyBuilds{},
	}})
	defer apiServer.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		_, _ = io.WriteString(w, "0000")
	}))
	defer upstream.Close()
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.9": {Name: "builder-1-0", UID: "uid-builder", Labels: credentialLabels(t, "tea-a", proxyBuildID, agentsession.BuildBranch(proxyBuildID))},
		}},
		API:   &agentsession.Client{URL: apiServer.URL, Secret: secret, HTTP: apiServer.Client()},
		Audit: &credentialAudit{}, UpstreamOrigin: upstream.URL, HTTP: upstream.Client(),
	}
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway.internal:8082", "tea-a-sandbox", proxyBuildID, "octo/repo", agentsession.BuildBranch(proxyBuildID))
	if err != nil {
		t.Fatal(err)
	}
	serve := func(method, path, contentType, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, repoURL+path, strings.NewReader(body))
		request.RemoteAddr = "10.0.0.9:43210"
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		recorder := httptest.NewRecorder()
		broker.Handler().ServeHTTP(recorder, request)
		return recorder
	}
	if rec := serve(http.MethodGet, "/info/refs?service=git-upload-pack", "", ""); rec.Code != http.StatusOK || gh.calls != 1 {
		t.Fatalf("a build's clone = %d (github calls %d)", rec.Code, gh.calls)
	}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"receive-pack advertisement": serve(http.MethodGet, "/info/refs?service=git-receive-pack", "", ""),
		"push":                       serve(http.MethodPost, "/git-receive-pack", "application/x-git-receive-pack-request", "0000"),
		"pull request":               serve(http.MethodPost, "/"+agentsession.PullRequestProxySegment, "application/json", `{"title":"x"}`),
	} {
		if rec.Code != http.StatusForbidden {
			t.Errorf("a build's %s = %d, want 403", name, rec.Code)
		}
	}
	if gh.calls != 1 {
		t.Fatalf("a refused build request minted a credential (github calls %d)", gh.calls)
	}
}
