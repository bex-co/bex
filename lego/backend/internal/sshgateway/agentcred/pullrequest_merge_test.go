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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const mergeHead = "0123456789abcdef0123456789abcdef01234567"

// mergeSessions is credentialSessions after ags-one opened draft PR #7 at
// mergeHead (prNumber 0: not yet), recording each pull request state.
type mergeSessions struct {
	prNumber int
	mu       *sync.Mutex
	states   *[]string
}

func (s mergeSessions) GetAgentSession(ctx context.Context, id string) (store.AgentSession, error) {
	session, err := credentialSessions{}.GetAgentSession(ctx, id)
	if s.prNumber > 0 {
		session.PRNumber, session.PRURL, session.HeadSHA = s.prNumber, "https://github.com/octo/repo/pull/7", mergeHead
	}
	return session, err
}

func (mergeSessions) RecordAgentSessionPullRequest(context.Context, string, string, int, string) error {
	return nil
}

// RecordPullRequestState records "<repo>#<n> <state>" for a merged pull
// request that is not the session's own (w10/041).
func (s mergeSessions) RecordPullRequestState(_ context.Context, _ []string, repo string, number int, state string, _ time.Time, _ *time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*s.states = append(*s.states, fmt.Sprintf("%s#%d %s", repo, number, state))
	return 1, nil
}

func (s mergeSessions) RecordAgentSessionPRState(_ context.Context, _, state string, _ time.Time, _ *time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*s.states = append(*s.states, state)
	return true, nil
}

// mergeHarness is prHarness with the mark-ready and merge verbs wired over
// the same signed hop and the same Pod table; GitHub's PR #7 is a draft at
// mergeHead.
func mergeHarness(t *testing.T, prNumber int) (*Broker, *prForge, *credentialAudit, *[]string) {
	t.Helper()
	secret := []byte("merge-secret")
	forge := &prForge{pr: github.PullRequestDetail{
		PullRequest: github.PullRequest{Number: 7, HTMLURL: "https://github.com/octo/repo/pull/7", State: "open", Draft: true},
		NodeID:      "PR_node7", HeadRef: "bex-agent/task-1", HeadSHA: mergeHead, HeadRepo: "octo/repo", BaseRepo: "octo/repo", BaseRef: "main",
	}}
	states := &[]string{}
	pullRequests := &agentsession.PullRequester{GitHub: forge, Connections: credentialConnections{}, Sessions: mergeSessions{prNumber: prNumber, mu: &sync.Mutex{}, states: states}}
	nonces := &onceNonces{seen: map[string]bool{}}
	mux := http.NewServeMux()
	mux.Handle(agentsession.InternalPullRequestReadyPath, &agentsession.PullRequestReadyHandler{Secret: secret, PullRequests: pullRequests, Nonce: nonces})
	mux.Handle(agentsession.InternalPullRequestMergePath, &agentsession.PullRequestMergeHandler{Secret: secret, PullRequests: pullRequests, Nonce: nonces})
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	broker, _, audit := prHarness(t)
	broker.PullRequestReady = &agentsession.PullRequestReadyClient{URL: api.URL + agentsession.InternalPullRequestReadyPath, Secret: secret, HTTP: api.Client()}
	broker.PullRequestMerges = &agentsession.PullRequestMergeClient{URL: api.URL + agentsession.InternalPullRequestMergePath, Secret: secret, HTTP: api.Client()}
	broker.Metrics = sshgateway.NewMetrics(prometheus.NewRegistry())
	return broker, forge, audit, states
}

func stateURL(t *testing.T, namespace, session, branch, segment string) string {
	t.Helper()
	repoURL, err := agentsession.ProxyRepositoryURL("http://gateway.internal:8082", namespace, session, "octo/repo", branch)
	if err != nil {
		t.Fatal(err)
	}
	return repoURL + "/" + segment
}

func ownReadyURL(t *testing.T) string {
	return stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", agentsession.PullRequestReadyProxySegment)
}

func ownMergeURL(t *testing.T) string {
	return stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", agentsession.PullRequestMergeProxySegment)
}

func TestPullRequestReadyAndMergeRoutesActOnTheSessionsPR(t *testing.T) {
	broker, forge, audit, states := mergeHarness(t, 7)
	// A draft is not merged: the agent is told to mark it ready first.
	reply := postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{}`)
	if reply.code != http.StatusConflict || reply.error.Code != agentsession.PullRequestDraft || !strings.Contains(reply.error.Hint, "mark_pull_request_ready first") {
		t.Fatalf("draft merge: status=%d body=%s", reply.code, reply.raw)
	}
	reply = postPR(t, broker, ownReadyURL(t), "10.0.0.1", "application/json", `{}`)
	var ready agentsession.PullRequestReadyResponse
	if reply.code != http.StatusOK || json.Unmarshal([]byte(reply.raw), &ready) != nil || ready.Number != 7 || ready.AlreadyReady {
		t.Fatalf("ready: status=%d body=%s", reply.code, reply.raw)
	}
	reply = postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json; charset=utf-8", `{"method":"merge","commitTitle":"Add retries (#7)"}`)
	var merged agentsession.PullRequestMergeResponse
	if reply.code != http.StatusOK || json.Unmarshal([]byte(reply.raw), &merged) != nil || !merged.Merged || merged.Number != 7 || merged.SHA == "" || merged.Method != "merge" {
		t.Fatalf("merge: status=%d body=%s", reply.code, reply.raw)
	}
	if len(forge.readied) != 1 || forge.readied[0] != "PR_node7" || len(forge.merged) != 1 || forge.merged[0].SHA != mergeHead || forge.merged[0].CommitTitle != "Add retries (#7)" {
		t.Fatalf("GitHub readied=%v merged=%+v", forge.readied, forge.merged)
	}
	if len(*states) != 1 || (*states)[0] != store.PRStateMerged {
		t.Fatalf("recorded states = %v, want merged at once", *states)
	}
	verbs := []string{}
	for _, event := range audit.events {
		verbs = append(verbs, event.Verb+"/"+string(event.Outcome))
		// w10/041: an answer names the pull request, as bex-api's audit does;
		// a refusal that names none names the repository.
		targetName := "octo/repo#7"
		if event.Outcome == core.AuditDenied {
			targetName = "octo/repo"
		}
		if event.Caller != "sbx-a-0" || event.Target != "agent-session:ags-one" || event.TargetName != targetName {
			t.Fatalf("gateway audit = %+v, want it to name %s", event, targetName)
		}
	}
	want := agentsession.AuditVerbPullRequestMerge + "/" + string(core.AuditDenied) + " " + agentsession.AuditVerbPullRequestReady + "/" + string(core.AuditAllowed) + " " +
		agentsession.AuditVerbPullRequestMerge + "/" + string(core.AuditAllowed)
	if strings.Join(verbs, " ") != want {
		t.Fatalf("gateway audit = %v, want %s", verbs, want)
	}
}

func TestPullRequestReadyAndMergeRoutesRefuseForeignPodsAndBranches(t *testing.T) {
	for _, segment := range []string{agentsession.PullRequestReadyProxySegment, agentsession.PullRequestMergeProxySegment} {
		cases := map[string]struct{ url, ip string }{
			"unresolved source pod":         {stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", segment), "10.0.0.9"},
			"sibling session's pod":         {stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", segment), "10.0.0.2"},
			"pod bound to another branch":   {stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", segment), "10.0.0.3"},
			"other workspace's pod":         {stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", segment), "10.0.0.4"},
			"URL claims another session":    {stateURL(t, "tea-a-sandbox", "ags-two", "bex-agent/task-1", segment), "10.0.0.1"},
			"labels match, record does not": {stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/other", segment), "10.0.0.3"},
		}
		for name, tc := range cases {
			t.Run(segment+"/"+name, func(t *testing.T) {
				broker, forge, _, _ := mergeHarness(t, 7)
				forge.pr.Draft = false
				reply := postPR(t, broker, tc.url, tc.ip, "application/json", `{}`)
				if reply.code != http.StatusForbidden || reply.error.Code != agentsession.PullRequestForbidden {
					t.Fatalf("status=%d body=%s, want 403 forbidden", reply.code, reply.raw)
				}
				if forge.calls != 0 {
					t.Fatal("a refused request reached GitHub")
				}
				if !strings.Contains(reply.error.Hint, "its own session's repository") || !strings.Contains(reply.error.Hint, "gh pr merge") {
					t.Fatalf("hint = %q", reply.error.Hint)
				}
			})
		}
	}
}

// w10/041: a session marks ready and merges a pull request of its repository
// it did not open when it names its number, here a person's from a fork while
// the session has none of its own: pinned to the head GitHub reports, its
// merged state recorded on the workspace's sessions that recorded it, and
// both hops' audit naming which pull request.
func TestPullRequestReadyAndMergeRoutesActOnANamedPullRequest(t *testing.T) {
	const personHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	broker, forge, audit, states := mergeHarness(t, 0)
	forge.pr.HeadRef, forge.pr.HeadRepo, forge.pr.HeadSHA = "fix-typo", "alice/repo", personHead
	reply := postPR(t, broker, ownReadyURL(t), "10.0.0.1", "application/json", `{"number":7}`)
	var ready agentsession.PullRequestReadyResponse
	if reply.code != http.StatusOK || json.Unmarshal([]byte(reply.raw), &ready) != nil || ready.Number != 7 || !ready.OtherPullRequest {
		t.Fatalf("ready: status=%d body=%s", reply.code, reply.raw)
	}
	reply = postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{"number":7,"method":"squash"}`)
	var merged agentsession.PullRequestMergeResponse
	if reply.code != http.StatusOK || json.Unmarshal([]byte(reply.raw), &merged) != nil || !merged.Merged || merged.Number != 7 || merged.HeadSHA != personHead || !merged.OtherPullRequest {
		t.Fatalf("merge: status=%d body=%s", reply.code, reply.raw)
	}
	if !strings.Contains(reply.raw, `"otherPullRequest":true`) {
		t.Fatalf("merge answer %s does not say it is another pull request", reply.raw)
	}
	if len(forge.readied) != 1 || len(forge.merged) != 1 || forge.merged[0].SHA != personHead {
		t.Fatalf("GitHub readied=%v merged=%+v, want the named pull request at GitHub's head", forge.readied, forge.merged)
	}
	if len(*states) != 1 || (*states)[0] != "octo/repo#7 merged" {
		t.Fatalf("recorded states = %v, want #7 merged on the workspace's sessions", *states)
	}
	if len(audit.events) != 2 {
		t.Fatalf("gateway audit = %+v", audit.events)
	}
	for _, event := range audit.events {
		if event.Outcome != core.AuditAllowed || event.TargetName != "octo/repo#7" {
			t.Fatalf("gateway audit = %+v, want allowed naming octo/repo#7", event)
		}
	}
}

// w10/m577: auto arms GitHub auto-merge on the session's pull request at its
// recorded head through both hops, the answer says armed and not merged, and
// nothing records the pull request merged; disableAuto disarms it.
func TestPullRequestMergeRouteArmsAndDisarmsAutoMerge(t *testing.T) {
	broker, forge, audit, states := mergeHarness(t, 7)
	forge.pr.Draft = false
	forge.state = github.MergeState{Status: "BLOCKED"}
	reply := postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{"auto":true,"method":"squash","commitTitle":"Add retries (#7)"}`)
	var armed agentsession.PullRequestMergeResponse
	if reply.code != http.StatusOK || json.Unmarshal([]byte(reply.raw), &armed) != nil {
		t.Fatalf("auto: status=%d body=%s", reply.code, reply.raw)
	}
	if armed.Merged || !armed.AutoMerge || armed.MergeQueue || armed.SHA != "" || armed.Method != "squash" || armed.HeadSHA != mergeHead || armed.Number != 7 {
		t.Fatalf("auto answer = %+v (%s), want armed, not merged, at the recorded head", armed, reply.raw)
	}
	if !strings.Contains(reply.raw, `"merged":false`) {
		t.Fatalf("auto answer %s does not say merged false", reply.raw)
	}
	if len(forge.merged) != 0 || len(forge.armed) != 1 || forge.armed[0].SHA != mergeHead || forge.armed[0].CommitTitle != "Add retries (#7)" {
		t.Fatalf("GitHub merged=%+v armed=%+v, want one arm pinned to the recorded head", forge.merged, forge.armed)
	}
	if len(*states) != 0 {
		t.Fatalf("recorded states = %v, want none for an armed pull request", *states)
	}
	// Armed already: the same answer, and GitHub is not asked again.
	reply = postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{"auto":true}`)
	if reply.code != http.StatusOK || !strings.Contains(reply.raw, `"autoMerge":true`) || len(forge.armed) != 1 {
		t.Fatalf("auto again: status=%d body=%s armed=%d", reply.code, reply.raw, len(forge.armed))
	}
	reply = postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{"disableAuto":true}`)
	var disarmed agentsession.PullRequestMergeResponse
	if reply.code != http.StatusOK || json.Unmarshal([]byte(reply.raw), &disarmed) != nil || !disarmed.AutoMergeDisabled || disarmed.Merged || disarmed.AutoMerge || forge.disarmed != 1 {
		t.Fatalf("disableAuto: status=%d body=%s disarmed=%d", reply.code, reply.raw, forge.disarmed)
	}
	if len(*states) != 0 || len(audit.events) != 3 {
		t.Fatalf("states=%v audit=%+v", *states, audit.events)
	}
	for _, event := range audit.events {
		if event.Verb != agentsession.AuditVerbPullRequestMerge || event.Outcome != core.AuditAllowed {
			t.Fatalf("gateway audit = %+v, want the merge verb allowed", event)
		}
	}
}

// The sandbox names nothing but a pull request number (w10/041), the merge
// method and commit text: a request naming the repository, the head or
// anything else is refused before bex-api, as is a malformed body.
func TestPullRequestReadyAndMergeRoutesValidateTheBody(t *testing.T) {
	cases := []struct {
		name, segment, contentType, body string
		status                           int
		code                             string
	}{
		{"ready names a negative number", agentsession.PullRequestReadyProxySegment, "application/json", `{"number":-8}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"ready names a number as text", agentsession.PullRequestReadyProxySegment, "application/json", `{"number":"8"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"ready names a repository", agentsession.PullRequestReadyProxySegment, "application/json", `{"number":8,"repository":"octo/other"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"ready not json", agentsession.PullRequestReadyProxySegment, "application/json", ``, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"ready wrong content type", agentsession.PullRequestReadyProxySegment, "text/plain", `{}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge names a fractional number", agentsession.PullRequestMergeProxySegment, "application/json", `{"number":8.5}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge names a negative number", agentsession.PullRequestMergeProxySegment, "application/json", `{"number":-8}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge names an owner", agentsession.PullRequestMergeProxySegment, "application/json", `{"number":8,"owner":"octo"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge names the head", agentsession.PullRequestMergeProxySegment, "application/json", `{"sha":"abc"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge names a repository", agentsession.PullRequestMergeProxySegment, "application/json", `{"repository":"octo/other"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge snake_case method", agentsession.PullRequestMergeProxySegment, "application/json", `{"merge_method":"squash"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge unknown method", agentsession.PullRequestMergeProxySegment, "application/json", `{"method":"octopus"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge two-line title", agentsession.PullRequestMergeProxySegment, "application/json", `{"commitTitle":"a\nb"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge message too long", agentsession.PullRequestMergeProxySegment, "application/json", `{"commitMessage":"` + strings.Repeat("m", agentsession.MaxMergeCommitMessageBytes+1) + `"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
		{"merge request over 64 KiB", agentsession.PullRequestMergeProxySegment, "application/json", `{"commitMessage":"` + strings.Repeat("m", agentsession.MaxPullRequestRequestBytes) + `"}`, http.StatusRequestEntityTooLarge, agentsession.PullRequestTooLarge},
		{"merge query string", agentsession.PullRequestMergeProxySegment + "?number=8", "application/json", `{}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge auto not a boolean", agentsession.PullRequestMergeProxySegment, "application/json", `{"auto":"yes"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge auto and disableAuto", agentsession.PullRequestMergeProxySegment, "application/json", `{"auto":true,"disableAuto":true}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge disableAuto with a method", agentsession.PullRequestMergeProxySegment, "application/json", `{"disableAuto":true,"method":"squash"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
		{"merge names the head to arm", agentsession.PullRequestMergeProxySegment, "application/json", `{"auto":true,"expectedHeadOid":"abc"}`, http.StatusUnprocessableEntity, agentsession.PullRequestInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			broker, forge, _, _ := mergeHarness(t, 7)
			forge.pr.Draft = false
			reply := postPR(t, broker, stateURL(t, "tea-a-sandbox", "ags-one", "bex-agent/task-1", tc.segment), "10.0.0.1", tc.contentType, tc.body)
			if reply.code != tc.status || reply.error.Code != tc.code {
				t.Fatalf("status=%d code=%q, want %d %q (body %.200s)", reply.code, reply.error.Code, tc.status, tc.code, reply.raw)
			}
			if forge.calls != 0 {
				t.Fatal("an invalid request reached GitHub")
			}
		})
	}
}

func TestPullRequestReadyAndMergeRoutesRelayBexAPIRefusals(t *testing.T) {
	broker, forge, audit, _ := mergeHarness(t, 0)
	reply := postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{}`)
	if reply.code != http.StatusConflict || reply.error.Code != agentsession.PullRequestNoPullRequest || forge.calls != 0 {
		t.Fatalf("no PR yet: status=%d body=%s", reply.code, reply.raw)
	}
	if !strings.Contains(reply.error.Hint, "open_or_update_draft_pr first") {
		t.Fatalf("hint = %q", reply.error.Hint)
	}
	if len(audit.events) != 1 || audit.events[0].Verb != agentsession.AuditVerbPullRequestMerge || audit.events[0].Outcome != core.AuditDenied {
		t.Fatalf("gateway audit = %+v", audit.events)
	}

	// A head bex has not recorded is never merged.
	broker, forge, _, _ = mergeHarness(t, 7)
	forge.pr.Draft, forge.pr.HeadSHA = false, "89abcdef0123456789abcdef0123456789abcdef"
	reply = postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{}`)
	if reply.code != http.StatusConflict || reply.error.Code != agentsession.PullRequestHeadChanged || len(forge.merged) != 0 {
		t.Fatalf("moved head: status=%d body=%s", reply.code, reply.raw)
	}

	for _, unwire := range []func(*Broker){
		func(b *Broker) { b.PullRequestReady = nil },
		func(b *Broker) { b.PullRequestMerges = nil },
	} {
		broker, _, _, _ = mergeHarness(t, 7)
		unwire(broker)
		url := ownReadyURL(t)
		if broker.PullRequestMerges == nil {
			url = ownMergeURL(t)
		}
		if reply := postPR(t, broker, url, "10.0.0.1", "application/json", `{}`); reply.code != http.StatusServiceUnavailable || reply.error.Code != agentsession.PullRequestUnavailable {
			t.Fatalf("unwired: status=%d body=%s", reply.code, reply.raw)
		}
	}
}

// The routes charge the same per-session budget as Git and the PR verb.
func TestPullRequestReadyAndMergeRoutesShareTheBudget(t *testing.T) {
	broker, forge, _, _ := mergeHarness(t, 7)
	broker.MaxRequestsPerSession = 1
	if reply := postPR(t, broker, ownReadyURL(t), "10.0.0.1", "application/json", `{}`); reply.code != http.StatusOK {
		t.Fatalf("ready status=%d body=%s", reply.code, reply.raw)
	}
	reply := postPR(t, broker, ownMergeURL(t), "10.0.0.1", "application/json", `{}`)
	if reply.code != http.StatusTooManyRequests || reply.error.Code != agentsession.PullRequestRateLimited || len(forge.merged) != 0 {
		t.Fatalf("budget: status=%d body=%s", reply.code, reply.raw)
	}
}
