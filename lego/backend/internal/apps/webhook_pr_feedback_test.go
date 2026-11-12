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
	body := `{"action":"created","installation":{"id":7},"issue":{"number":3}}`
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

	sink.err = errors.New("db down")
	if rec := deliverFeedback(h, "issue_comment", "gh-key", body); rec.Code != http.StatusBadGateway {
		t.Fatalf("a sink failure answered %d", rec.Code)
	}
	if rec := deliverFeedback(&GitWebhook{GitHubSecret: "gh-key", Installations: fakeInstallations{7: {"tea-a"}}}, "issue_comment", "gh-key", body); rec.Code != http.StatusOK {
		t.Fatalf("no sink: %d", rec.Code)
	}
}
