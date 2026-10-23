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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

// prForge stands in for GitHub behind bex-api's real PullRequester.
type prForge struct {
	mu    sync.Mutex
	calls int
	heads []string
}

func (f *prForge) SyncDraftPullRequest(_ context.Context, _ int64, owner, repo, head, _, _ string) (github.DraftPullRequestResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.heads = append(f.heads, head)
	return github.DraftPullRequestResult{
		PullRequest: github.PullRequest{Number: 7, HTMLURL: "https://github.com/" + owner + "/" + repo + "/pull/7", State: "open", Draft: true},
		Created:     f.calls == 1, HeadSHA: "0123456789abcdef0123456789abcdef01234567",
	}, nil
}

// prHarness wires the gateway Broker to a real bex-api PullRequestHandler over
// the signed hop. Source IPs: .1 the session's own pod; .2 a sibling session's
// pod; .3 a pod bound to another branch; .4 another workspace's pod.
func prHarness(t *testing.T) (*Broker, *prForge, *credentialAudit) {
	t.Helper()
	secret := []byte("pr-secret")
	forge := &prForge{}
	api := httptest.NewServer(&agentsession.PullRequestHandler{Secret: secret, PullRequests: &agentsession.PullRequester{
		GitHub: forge, Connections: credentialConnections{}, Sessions: credentialSessions{},
	}})
	t.Cleanup(api.Close)
	audit := &credentialAudit{}
	broker := &Broker{
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Pods: credentialPodResolver{pods: map[string]SessionPod{
			"10.0.0.1": {Name: "sbx-a-0", UID: "uid-a", Labels: credentialLabels(t, "tea-a", "ags-one", "bex-agent/task-1")},
			"10.0.0.2": {Name: "sbx-b-0", UID: "uid-b", Labels: credentialLabels(t, "tea-a", "ags-two", "bex-agent/task-1")},
			"10.0.0.3": {Name: "sbx-a-0", UID: "uid-c", Labels: credentialLabels(t, "tea-a", "ags-one", "bex-agent/other")},
			"10.0.0.4": {Name: "sbx-a-0", UID: "uid-d", Labels: credentialLabels(t, "tea-b", "ags-one", "bex-agent/task-1")},
		}},
		API:          &agentsession.Client{URL: api.URL, Secret: secret, HTTP: api.Client()},
		PullRequests: &agentsession.PullRequestClient{URL: api.URL, Secret: secret, HTTP: api.Client()},
		Audit:        audit,
	}
	return broker, forge, audit
}

func prURL(t *testing.T, namespace, session, branch string) string {
	t.Helper()
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway.internal:8082", namespace, session, "octo/repo", branch)
	if err != nil {
		t.Fatal(err)
	}
	return repoURL + "/" + agentsession.PullRequestProxySegment
}

type prReply struct {
	code  int
	raw   string
	error struct{ Code, Message, Hint string }
}

func postPR(t *testing.T, broker *Broker, url, sourceIP, contentType, body string) prReply {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request.RemoteAddr = sourceIP + ":43210"
	recorder := httptest.NewRecorder()
	broker.Handler().ServeHTTP(recorder, request)
	reply := prReply{code: recorder.Code, raw: recorder.Body.String()}
	var envelope struct {
		Error *struct{ Code, Message, Hint string } `json:"error"`
	}
	if json.Unmarshal(recorder.Body.Bytes(), &envelope) == nil && envelope.Error != nil {
		reply.error = *envelope.Error
	}
	if strings.Contains(reply.raw, "ghs_") || strings.Contains(strings.ToLower(reply.raw), `"token"`) {
		t.Fatalf("a credential-shaped value reached the sandbox: %s", reply.raw)
	}
	return reply
}

const okBody = `{"title":"Add retries","body":"## Summary\nadds retries"}`

func TestPullRequestRouteOpensTheBoundBranchOnly(t *testing.T) {
	broker, forge, audit := prHarness(t)
	reply := postPR(t, broker, prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.1", "application/json; charset=utf-8", okBody)
	if reply.code != http.StatusOK {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
	var got agentsession.PullRequestResponse
	if err := json.Unmarshal([]byte(reply.raw), &got); err != nil || got.URL != "https://github.com/octo/repo/pull/7" || got.Number != 7 || !got.Created || got.HeadSHA == "" {
		t.Fatalf("response=%+v err=%v", got, err)
	}
	if len(forge.heads) != 1 || forge.heads[0] != "bex-agent/task-1" {
		t.Fatalf("GitHub head = %v, want the bound branch", forge.heads)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbPullRequest || audit.events[0].Caller != "sbx-a-0" || audit.events[0].Target != "agent-session:ags-one" {
		t.Fatalf("gateway audit = %+v", audit.events)
	}
}

func TestPullRequestRouteRefusesForeignPodsAndBranches(t *testing.T) {
	cases := map[string]struct{ url, ip string }{
		"unresolved source pod":         {prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.9"},
		"sibling session's pod":         {prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.2"},
		"pod bound to another branch":   {prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.3"},
		"other workspace's pod":         {prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.4"},
		"URL claims another branch":     {prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/other"), "10.0.0.1"},
		"URL claims another session":    {prURL(t, "tea-a-sandbox", "ags-two", "bex-agent/task-1"), "10.0.0.1"},
		"labels match, record does not": {prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/other"), "10.0.0.3"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broker, forge, _ := prHarness(t)
			reply := postPR(t, broker, tc.url, tc.ip, "application/json", okBody)
			if reply.code != http.StatusForbidden || reply.error.Code != agentsession.PullRequestForbidden {
				t.Fatalf("status=%d body=%s, want 403 forbidden", reply.code, reply.raw)
			}
			if forge.calls != 0 {
				t.Fatal("a refused request reached GitHub")
			}
			if !strings.Contains(reply.error.Hint, "Do not try another route") {
				t.Fatalf("hint = %q", reply.error.Hint)
			}
		})
	}
}

func TestPullRequestRouteValidatesTheBody(t *testing.T) {
	url := prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1")
	cases := []struct {
		name, contentType, body string
		status                  int
		code                    string
	}{
		{"head override", "application/json", `{"title":"t","body":"b","head":"main"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"base override", "application/json", `{"title":"t","body":"b","base":"release"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"not json", "application/json", `title=t`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"wrong content type", "text/plain", okBody, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"multi-line title", "application/json", `{"title":"a\nb","body":"b"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"title too long", "application/json", `{"title":"` + strings.Repeat("t", agentsession.MaxPullRequestTitleRunes+1) + `","body":"b"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
		{"body too long", "application/json", `{"title":"t","body":"` + strings.Repeat("b", agentsession.MaxPullRequestBodyBytes+1) + `"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
		{"request over 64 KiB", "application/json", `{"title":"t","body":"` + strings.Repeat("b", agentsession.MaxPullRequestRequestBytes) + `"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broker, forge, _ := prHarness(t)
			reply := postPR(t, broker, url, "10.0.0.1", tc.contentType, tc.body)
			if reply.code != tc.status || reply.error.Code != tc.code {
				t.Fatalf("status=%d code=%q, want %d %q (body %.200s)", reply.code, reply.error.Code, tc.status, tc.code, reply.raw)
			}
			if forge.calls != 0 {
				t.Fatal("an invalid request reached GitHub")
			}
		})
	}
}

func TestPullRequestRouteSharesTheGitLimiterAndBudgets(t *testing.T) {
	url := prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1")

	broker, forge, _ := prHarness(t)
	broker.Limits = sshgateway.NewSessionLimiter(4, 1)
	if ok, _ := broker.Limits.Acquire("10.0.0.1"); !ok {
		t.Fatal("could not occupy the per-pod slot")
	}
	reply := postPR(t, broker, url, "10.0.0.1", "application/json", okBody)
	if reply.code != http.StatusTooManyRequests || reply.error.Code != agentsession.PullRequestRateLimited || forge.calls != 0 {
		t.Fatalf("held slot: status=%d body=%s", reply.code, reply.raw)
	}
	broker.Limits.Release("10.0.0.1")

	broker, forge, _ = prHarness(t)
	broker.MaxRequestsPerSession = 1
	if reply := postPR(t, broker, url, "10.0.0.1", "application/json", okBody); reply.code != http.StatusOK {
		t.Fatalf("first request status=%d", reply.code)
	}
	reply = postPR(t, broker, url, "10.0.0.1", "application/json", okBody)
	if reply.code != http.StatusTooManyRequests || reply.error.Code != agentsession.PullRequestRateLimited || forge.calls != 1 {
		t.Fatalf("budget: status=%d calls=%d body=%s", reply.code, forge.calls, reply.raw)
	}
}

func TestPullRequestRouteUnavailableWithoutTheVerb(t *testing.T) {
	broker, _, _ := prHarness(t)
	broker.PullRequests = nil
	reply := postPR(t, broker, prURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1"), "10.0.0.1", "application/json", okBody)
	if reply.code != http.StatusServiceUnavailable || reply.error.Code != agentsession.PullRequestUnavailable {
		t.Fatalf("status=%d body=%s", reply.code, reply.raw)
	}
}
