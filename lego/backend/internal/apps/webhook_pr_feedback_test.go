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

package apps

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

type recordingFeedbackSink struct {
	events []string
	scopes [][]string
	err    error
}

func (r *recordingFeedbackSink) HandlePRFeedback(_ context.Context, event string, _ []byte, workspaceIDs []string) (int, error) {
	r.events = append(r.events, event)
	r.scopes = append(r.scopes, workspaceIDs)
	if r.err != nil {
		return 0, r.err
	}
	return 1, nil
}

func deliverFeedback(h *GitWebhook, event, secret, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/git", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", sign(secret, []byte(body)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Comments and reviews reach the PR feedback sink confined to the
// installation's workspaces (ADR047 D11); a manual-key delivery, an unbound
// installation or a missing sink queues nothing, and a sink failure is a 502
// so GitHub's delivery log shows it.
func TestGitWebhookForwardsPRFeedbackConfinedToTheInstallation(t *testing.T) {
	sink := &recordingFeedbackSink{}
	h := &GitWebhook{Secret: "manual", GitHubSecret: "gh-key", Installations: fakeInstallations{7: {"tea-a", "tea-b"}}, PRFeedback: sink}
	body := `{"action":"created","installation":{"id":7},"issue":{"number":3,"state":"open","pull_request":{"url":"x"}},` +
		`"pull_request":{"number":3,"state":"open"},"comment":{"body":"/bex fix it","user":{"type":"User"}}}`
	if rec := deliverFeedback(h, "issue_comment", "gh-key", body); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"queued":1`) {
		t.Fatalf("app-signed comment = %d %s", rec.Code, rec.Body)
	}
	if len(sink.scopes) != 1 || !slices.Equal(sink.scopes[0], []string{"tea-a", "tea-b"}) || sink.events[0] != "issue_comment" {
		t.Fatalf("sink saw %v %v", sink.events, sink.scopes)
	}
	deliverFeedback(h, "pull_request_review", "gh-key", body)
	if len(sink.events) != 2 || sink.events[1] != "pull_request_review" {
		t.Fatalf("review not forwarded: %v", sink.events)
	}

	for name, tc := range map[string]struct{ secret, body string }{
		"manual key":            {"manual", body},
		"unbound installation":  {"gh-key", `{"action":"created","installation":{"id":8}}`},
		"no installation in it": {"gh-key", `{"action":"created"}`},
	} {
		before := len(sink.events)
		if rec := deliverFeedback(h, "issue_comment", tc.secret, tc.body); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"queued":0`) {
			t.Fatalf("%s = %d %s", name, rec.Code, rec.Body)
		}
		if len(sink.events) != before {
			t.Fatalf("%s reached the sink", name)
		}
	}

	// An inline `/bex` comment reaches the same sink. What can neither steer
	// nor start a session drops before any workspace lookup: an edit or a
	// deletion, a bot's comment, a closed pull request, a plain issue, and an
	// inline comment that is not the command (its review is delivered too).
	if rec := deliverFeedback(h, "pull_request_review_comment", "gh-key", body); rec.Code != http.StatusOK || sink.events[len(sink.events)-1] != "pull_request_review_comment" {
		t.Fatalf("inline comment = %d %s, sink saw %v", rec.Code, rec.Body, sink.events)
	}
	dropped := map[string][]string{
		"edited":            {`"created"`, `"edited"`},
		"deleted":           {`"created"`, `"deleted"`},
		"a bot's":           {`"User"`, `"Bot"`},
		"on a closed PR":    {`"state":"open"`, `"state":"closed"`},
		"not the command":   {"/bex fix it", "please fix it"},
		"on a plain issue":  {`"pull_request":{"url":"x"}`, `"pull_request":null`},
		"with no commenter": {`"user":{"type":"User"}`, `"user":{}`},
	}
	for _, event := range []string{"pull_request_review_comment", "issue_comment"} {
		for name, swap := range dropped {
			if event == "issue_comment" && name == "not the command" {
				continue // a conversation comment is feedback whatever it says
			}
			if event == "pull_request_review_comment" && name == "on a plain issue" {
				continue
			}
			before := len(sink.events)
			variant := strings.Replace(body, swap[0], swap[1], -1)
			if rec := deliverFeedback(h, event, "gh-key", variant); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"queued":0`) || len(sink.events) != before {
				t.Fatalf("%s %s = %d %s, reached the sink: %v", event, name, rec.Code, rec.Body, len(sink.events) != before)
			}
		}
	}

	sink.err = errors.New("db down")
	if rec := deliverFeedback(h, "issue_comment", "gh-key", body); rec.Code != http.StatusBadGateway {
		t.Fatalf("a sink failure answered %d", rec.Code)
	}
	if rec := deliverFeedback(&GitWebhook{GitHubSecret: "gh-key", Installations: fakeInstallations{7: {"tea-a"}}}, "issue_comment", "gh-key", body); rec.Code != http.StatusOK {
		t.Fatalf("no sink: %d", rec.Code)
	}
}

type recordingPRStateSink struct {
	scopes [][]string
	err    error
}

func (r *recordingPRStateSink) HandlePullRequestEvent(_ context.Context, _ []byte, workspaceIDs []string) error {
	r.scopes = append(r.scopes, workspaceIDs)
	return r.err
}

// A pull_request delivery reaches the state sink confined to the
// installation's workspaces (w9/m386); a manual-key delivery or an unbound
// installation reaches nothing, and a sink failure still answers 200 because
// the reconciler reads every state from GitHub anyway.
func TestGitWebhookForwardsPullRequestStateConfinedToTheInstallation(t *testing.T) {
	sink := &recordingPRStateSink{}
	h := &GitWebhook{Secret: "manual", GitHubSecret: "gh-key", Installations: fakeInstallations{7: {"tea-a", "tea-b"}}, PRStates: sink}
	body := `{"action":"closed","installation":{"id":7},"pull_request":{"number":3,"state":"closed","merged":true},"repository":{"full_name":"bex-co/app"}}`
	if rec := deliverFeedback(h, "pull_request", "gh-key", body); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"accepted":"pull_request"`) {
		t.Fatalf("app-signed pull_request = %d %s", rec.Code, rec.Body)
	}
	if len(sink.scopes) != 1 || !slices.Equal(sink.scopes[0], []string{"tea-a", "tea-b"}) {
		t.Fatalf("sink saw %v", sink.scopes)
	}
	for name, tc := range map[string]struct{ secret, body string }{
		"manual key":           {"manual", body},
		"unbound installation": {"gh-key", strings.Replace(body, `"id":7`, `"id":8`, 1)},
		"no installation":      {"gh-key", `{"action":"closed"}`},
		"a synchronize":        {"gh-key", strings.Replace(body, `"action":"closed"`, `"action":"synchronize"`, 1)},
		"a label change":       {"gh-key", strings.Replace(body, `"action":"closed"`, `"action":"labeled"`, 1)},
	} {
		before := len(sink.scopes)
		if rec := deliverFeedback(h, "pull_request", tc.secret, tc.body); rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", name, rec.Code, rec.Body)
		}
		if len(sink.scopes) != before {
			t.Fatalf("%s reached the sink", name)
		}
	}
	sink.err = errors.New("postgres down")
	if rec := deliverFeedback(h, "pull_request", "gh-key", body); rec.Code != http.StatusOK {
		t.Fatalf("a sink failure = %d %s", rec.Code, rec.Body)
	}
	// No sink: still acknowledged, not ignored as an unknown event.
	h.PRStates = nil
	if rec := deliverFeedback(h, "pull_request", "gh-key", body); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "accepted") {
		t.Fatalf("no sink = %d %s", rec.Code, rec.Body)
	}
}
