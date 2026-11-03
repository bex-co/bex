package agentsessions

import (
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestCreateWithoutBranchGetsTheSessionsOwnBranch(t *testing.T) {
	svc, _, _, _ := fixture()
	req := createInput()
	req.Branch = ""
	view, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if view.Branch != "bex-agent/"+view.ID {
		t.Fatalf("branch = %q, want bex-agent/%s", view.Branch, view.ID)
	}
	// An explicit branch is still validated against the prefix rule.
	req.Branch = "main"
	if _, err := svc.Create(caller("alice"), req); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("explicit non-bex-agent branch = %v, want AGENT_SESSION_INPUT_INVALID", err)
	}
}

func TestRepoLessSessionStillNeedsNoBranch(t *testing.T) {
	svc, _, _, _ := fixture()
	req := createInput()
	req.Repo, req.Branch = "", ""
	view, err := svc.Create(caller("alice"), req)
	if err != nil || view.Branch != "" {
		t.Fatalf("repo-less create = %+v err=%v, want empty branch", view, err)
	}
}

func TestTitleDefaultsToTheTasksFirstLineAndTagsAreValidated(t *testing.T) {
	svc, _, _, _ := fixture()
	req := createInput()
	req.AgentConfig.Task = "Fix the flaky webhook test\nthen update the docs"
	req.Tags = []string{"ci", "team:billing", "ci"}
	view, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if view.Title != "Fix the flaky webhook test" || strings.Join(view.Tags, ",") != "ci,team:billing" {
		t.Fatalf("title=%q tags=%v, want first-line title and de-duplicated tags", view.Title, view.Tags)
	}

	req = createInput()
	req.Title = "Explicit title"
	if view, err = svc.Create(caller("alice"), req); err != nil || view.Title != "Explicit title" {
		t.Fatalf("explicit title = %q err=%v", view.Title, err)
	}

	for name, tags := range map[string][]string{
		"uppercase": {"CI"},
		"space":     {"two words"},
		"empty":     {""},
		"too long":  {strings.Repeat("a", 65)},
		"too many":  make([]string, 21),
	} {
		req = createInput()
		req.Tags = tags
		if name == "too many" {
			for i := range req.Tags {
				req.Tags[i] = "t" + strings.Repeat("x", i)
			}
		}
		if _, err := svc.Create(caller("alice"), req); !isCode(err, "AGENT_SESSION_TAGS_INVALID") {
			t.Errorf("%s tags: err = %v, want AGENT_SESSION_TAGS_INVALID", name, err)
		}
	}
	req = createInput()
	req.Title = strings.Repeat("t", 201)
	if _, err := svc.Create(caller("alice"), req); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Errorf("201-char explicit title = %v, want AGENT_SESSION_INPUT_INVALID", err)
	}
}

func TestListFiltersByStateAndTag(t *testing.T) {
	svc, st, fga, _ := fixture()
	mk := func(phase string, tags ...string) string {
		return seedSession(st, fga, func(r *store.AgentSession) { r.Phase, r.Tags = phase, tags })
	}
	working := mk(PhaseRunning, "ci")
	done := mk(PhaseCompleted, "ci", "nightly")
	failed := mk(PhaseFailed)
	canceled := mk(PhaseCanceled, "nightly")

	list := func(req ListRequest) map[string]bool {
		t.Helper()
		req.OwnerID = "tea-a"
		views, err := svc.List(caller("alice"), req)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, v := range views {
			got[v.ID] = true
		}
		return got
	}
	if got := list(ListRequest{States: []string{StateWorking}}); len(got) != 1 || !got[working] {
		t.Errorf("state=working = %v", got)
	}
	if got := list(ListRequest{States: []string{StateSucceeded, StateFailed}}); len(got) != 2 || !got[done] || !got[failed] {
		t.Errorf("state=succeeded,failed = %v", got)
	}
	if got := list(ListRequest{Tag: "ci"}); len(got) != 2 || !got[working] || !got[done] {
		t.Errorf("tag=ci = %v", got)
	}
	if got := list(ListRequest{Tag: "nightly", States: []string{StateCanceled}}); len(got) != 1 || !got[canceled] {
		t.Errorf("tag=nightly state=canceled = %v", got)
	}
	// state ∩ phase: a phase outside the state matches nothing.
	if got := list(ListRequest{States: []string{StateSucceeded}, Phases: []string{PhaseFailed}}); len(got) != 0 {
		t.Errorf("state=succeeded ∩ phase=failed = %v, want none", got)
	}
	// needs_input covers no phase yet: it lists nothing, never everything.
	if got := list(ListRequest{States: []string{StateNeedsInput}}); len(got) != 0 {
		t.Errorf("state=needs_input = %v, want none", got)
	}
	if _, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", States: []string{"bogus"}}); !isCode(err, "AGENT_SESSION_FILTER_INVALID") {
		t.Errorf("unknown state = %v, want AGENT_SESSION_FILTER_INVALID", err)
	}
}
