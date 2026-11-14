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

package agentsessions

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakePRStateStore struct {
	mu          sync.Mutex
	due         []store.AgentSessionPRCheck
	retry       time.Duration
	states      map[string]string
	next        map[string]*time.Time
	connections map[string]int64 // workspace/owner -> installation
	lookups     int
	byPR        []string // RecordPullRequestState calls
}

func (f *fakePRStateStore) ClaimAgentSessionPRChecks(_ context.Context, _ time.Time, retry time.Duration, limit int) ([]store.AgentSessionPRCheck, error) {
	f.retry = retry
	out := f.due
	if len(out) > limit {
		out = out[:limit]
	}
	f.due = nil
	return out, nil
}

func (f *fakePRStateStore) RecordAgentSessionPRState(_ context.Context, id, state string, _ time.Time, next *time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	changed := f.states[id] != state
	f.states[id] = state
	if f.next == nil {
		f.next = map[string]*time.Time{}
	}
	f.next[id] = next
	return changed, nil
}

func (f *fakePRStateStore) RecordPullRequestState(_ context.Context, ws []string, repo string, number int, state string, _ time.Time, _ *time.Time) (int, error) {
	f.byPR = append(f.byPR, fmt.Sprintf("%v %s#%d=%s", ws, repo, number, state))
	return 1, nil
}

func (f *fakePRStateStore) GetGitConnectionByOwner(_ context.Context, ws, owner string) (store.GitConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lookups++
	id, ok := f.connections[ws+"/"+owner]
	if !ok {
		return store.GitConnection{}, store.ErrNotFound
	}
	return store.GitConnection{WorkspaceID: ws, InstallationID: id, AccountLogin: owner}, nil
}

type fakePRReader struct {
	mu    sync.Mutex
	prs   map[string]github.PullRequestDetail // installation/owner/repo#n
	reads []string
}

func (f *fakePRReader) GetPullRequest(_ context.Context, installationID int64, owner, repo string, number int) (github.PullRequestDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprintf("%d/%s/%s#%d", installationID, owner, repo, number)
	f.reads = append(f.reads, key)
	pr, ok := f.prs[key]
	if !ok {
		return github.PullRequestDetail{}, errors.New("github: 404")
	}
	return pr, nil
}

func prDetail(state string, merged bool) github.PullRequestDetail {
	d := github.PullRequestDetail{Merged: merged}
	d.State = state
	return d
}

func TestPRStatesReadEachDuePullRequest(t *testing.T) {
	st := &fakePRStateStore{
		states:      map[string]string{"ags-gone": "open"},
		connections: map[string]int64{"tea-a/bex-co": 7},
		due: []store.AgentSessionPRCheck{
			{SessionID: "ags-merged", WorkspaceID: "tea-a", Repo: "bex-co/app", PRNumber: 1},
			{SessionID: "ags-closed", WorkspaceID: "tea-a", Repo: "bex-co/app", PRNumber: 2, PRState: "open"},
			{SessionID: "ags-draft", WorkspaceID: "tea-a", Repo: "bex-co/web", PRNumber: 3},
			{SessionID: "ags-gone", WorkspaceID: "tea-a", Repo: "bex-co/app", PRNumber: 4, PRState: "open"},
			{SessionID: "ags-unconnected", WorkspaceID: "tea-b", Repo: "acme/x", PRNumber: 5},
		},
	}
	gh := &fakePRReader{prs: map[string]github.PullRequestDetail{
		"7/bex-co/app#1": prDetail("closed", true),
		"7/bex-co/app#2": prDetail("closed", false),
		"7/bex-co/web#3": prDetail("open", false), // a draft reads open, as on Devin
	}}
	p := &PRStates{Store: st, GitHub: gh, Now: func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }}
	if err := p.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"ags-merged": "merged", "ags-closed": "closed", "ags-draft": "open", "ags-gone": "open"}
	for id, state := range want {
		if st.states[id] != state {
			t.Errorf("%s = %q, want %q", id, st.states[id], state)
		}
	}
	if _, ok := st.states["ags-unconnected"]; ok {
		t.Error("a session whose workspace has no installation for the owner got a state")
	}
	// One installation lookup per workspace and owner within a tick.
	if st.lookups != 2 {
		t.Errorf("installation lookups = %d, want 2 (tea-a/bex-co once, tea-b/acme once)", st.lookups)
	}
	if st.retry != prStateRetry {
		t.Errorf("claim retry = %v", st.retry)
	}
	// A merged pull request is never read again; an open one soon.
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	if st.next["ags-merged"] != nil {
		t.Errorf("a merged pull request is read again at %v", st.next["ags-merged"])
	}
	if n := st.next["ags-draft"]; n == nil || !n.Equal(now.Add(5*time.Minute)) {
		t.Errorf("a newly open pull request's next read = %v", n)
	}
}

func TestPullRequestEventsUpdateStatesInTheirWorkspaces(t *testing.T) {
	st := &fakePRStateStore{states: map[string]string{}}
	p := &PRStates{Store: st}
	for _, tc := range []struct {
		body string
		want string
	}{
		{`{"action":"closed","pull_request":{"number":8,"state":"closed","merged":true},"repository":{"full_name":"Bex-Co/App"}}`, "[tea-a tea-b] bex-co/app#8=merged"},
		{`{"action":"closed","pull_request":{"number":9,"state":"closed","merged":false},"repository":{"full_name":"bex-co/app"}}`, "[tea-a tea-b] bex-co/app#9=closed"},
		{`{"action":"reopened","pull_request":{"number":9,"state":"open"},"repository":{"full_name":"bex-co/app"}}`, "[tea-a tea-b] bex-co/app#9=open"},
		{`{"action":"closed","pull_request":{"number":0,"state":"closed"},"repository":{"full_name":"bex-co/app"}}`, ""},
		{`{"action":"closed","pull_request":{"number":3,"state":"closed"},"repository":{"full_name":"not-a-repo"}}`, ""},
	} {
		st.byPR = nil
		if err := p.HandlePullRequestEvent(context.Background(), []byte(tc.body), []string{"tea-a", "tea-b"}); err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		got := ""
		if len(st.byPR) == 1 {
			got = st.byPR[0]
		}
		if got != tc.want || len(st.byPR) > 1 {
			t.Errorf("%s: recorded %v, want %q", tc.body, st.byPR, tc.want)
		}
	}
	// No workspace bound to the installation: nothing is touched.
	st.byPR = nil
	if err := p.HandlePullRequestEvent(context.Background(), []byte(`{"action":"closed","pull_request":{"number":8,"state":"closed"},"repository":{"full_name":"bex-co/app"}}`), nil); err != nil || len(st.byPR) != 0 {
		t.Fatalf("unscoped delivery = %v %v", st.byPR, err)
	}
	if err := p.HandlePullRequestEvent(context.Background(), []byte(`{`), []string{"tea-a"}); err == nil {
		t.Fatal("a malformed delivery was accepted")
	}
}

func TestNextPRCheckBacksOffAndStops(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	for _, tc := range []struct {
		state string
		age   time.Duration
		want  time.Duration // 0: never
	}{
		{store.PRStateMerged, 0, 0},
		{store.PRStateClosed, time.Hour, time.Hour},
		{store.PRStateClosed, 7 * day, 0},
		{store.PRStateOpen, 0, 5 * time.Minute},
		{store.PRStateOpen, 6 * day, 5 * time.Minute},
		{store.PRStateOpen, 8 * day, time.Hour},
		{store.PRStateOpen, 31 * day, day},
	} {
		got := nextPRCheck(tc.state, now.Add(-tc.age), now)
		switch {
		case tc.want == 0 && got != nil:
			t.Errorf("%s aged %v: read again at %v, want never", tc.state, tc.age, got)
		case tc.want != 0 && (got == nil || !got.Equal(now.Add(tc.want))):
			t.Errorf("%s aged %v: next = %v, want +%v", tc.state, tc.age, got, tc.want)
		}
	}
}

// An unchanged open pull request backs off from when it last changed, not
// from this read.
func TestPRStatesBackOffFromTheLastChange(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	openedAt := now.Add(-10 * 24 * time.Hour)
	st := &fakePRStateStore{states: map[string]string{"ags-old": "open"}, connections: map[string]int64{"tea-a/bex-co": 7},
		due: []store.AgentSessionPRCheck{{SessionID: "ags-old", WorkspaceID: "tea-a", Repo: "bex-co/app", PRNumber: 1, PRState: "open", PRStateAt: &openedAt}}}
	gh := &fakePRReader{prs: map[string]github.PullRequestDetail{"7/bex-co/app#1": prDetail("open", false)}}
	if err := (&PRStates{Store: st, GitHub: gh, Now: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := st.next["ags-old"]; n == nil || !n.Equal(now.Add(time.Hour)) {
		t.Fatalf("next read = %v, want an hour (open for ten days)", n)
	}
}

func TestPRStateListFilterAndView(t *testing.T) {
	req := ListRequest{PRStates: []string{"merged", "merged", "closed"}}
	q, err := req.storeQuery()
	if err != nil || !slices.Equal(q.PRStates, []string{"merged", "closed"}) {
		t.Fatalf("store query = %v / %v", q.PRStates, err)
	}
	if q, err := (&ListRequest{PRStates: []string{" open "}}).storeQuery(); err != nil || !slices.Equal(q.PRStates, []string{"open"}) {
		t.Fatalf("a padded value = %v / %v", q.PRStates, err)
	}
	for _, bad := range []string{"draft", "MERGED", ""} {
		req := ListRequest{PRStates: []string{bad}}
		if _, err := req.storeQuery(); codeOf(err) != "AGENT_SESSION_FILTER_INVALID" {
			t.Errorf("prState %q = %v", bad, err)
		}
	}
	at := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	view, err := viewOf(store.AgentSession{ID: "ags-x", AgentConfig: []byte(`{}`), PRNumber: 4, PRURL: "https://github.com/a/b/pull/4", PRState: "merged", PRStateAt: &at})
	if err != nil || view.PRState != "merged" || view.PRStateAt == nil || !view.PRStateAt.Equal(at) {
		t.Fatalf("view = %+v / %v", view, err)
	}
}
