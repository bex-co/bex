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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeAutomations is the agent_automations table: unique names per
// workspace, version-checked updates, newest-first paging.
type fakeAutomations struct {
	// The scheduler-side methods are not faked here: calling one panics.
	AutomationStore
	mu   sync.Mutex
	seq  int
	rows map[string]store.AgentAutomation
}

func newFakeAutomations() *fakeAutomations {
	return &fakeAutomations{rows: map[string]store.AgentAutomation{}}
}

func (f *fakeAutomations) CreateAgentAutomation(_ context.Context, in store.AgentAutomation) (store.AgentAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.WorkspaceID == in.WorkspaceID && row.Name == in.Name {
			return store.AgentAutomation{}, store.ErrConflict
		}
	}
	f.seq++
	in.Version, in.CreatedAt, in.UpdatedAt = 1, time.Unix(int64(1000+f.seq), 0), time.Now()
	f.rows[in.ID] = in
	return in, nil
}

func (f *fakeAutomations) GetAgentAutomation(_ context.Context, id string) (store.AgentAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row, ok := f.rows[id]
	if !ok {
		return store.AgentAutomation{}, store.ErrNotFound
	}
	return row, nil
}

func (f *fakeAutomations) GetAgentAutomationByName(_ context.Context, ws, name string) (store.AgentAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.WorkspaceID == ws && row.Name == name {
			return row, nil
		}
	}
	return store.AgentAutomation{}, store.ErrNotFound
}

func (f *fakeAutomations) ListAgentAutomations(_ context.Context, ws, createdBy, cursor string, limit int) ([]store.AgentAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.AgentAutomation
	for _, row := range f.rows {
		if row.WorkspaceID == ws && (createdBy == "" || row.CreatedBy == createdBy) {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if cursor != "" {
		i := slices.IndexFunc(out, func(r store.AgentAutomation) bool { return r.ID == cursor })
		out = out[i+1:]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeAutomations) UpdateAgentAutomation(_ context.Context, in store.AgentAutomation, ifVersion int) (store.AgentAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.rows[in.ID]
	if !ok || cur.WorkspaceID != in.WorkspaceID {
		return store.AgentAutomation{}, store.ErrNotFound
	}
	if cur.Version != ifVersion {
		return store.AgentAutomation{}, store.ErrAgentAutomationVersion
	}
	for _, row := range f.rows {
		if row.ID != in.ID && row.WorkspaceID == in.WorkspaceID && row.Name == in.Name {
			return store.AgentAutomation{}, store.ErrConflict
		}
	}
	cur.Name, cur.Enabled, cur.Triggers, cur.Template, cur.NextFireAt = in.Name, in.Enabled, in.Triggers, in.Template, in.NextFireAt
	if in.CreatedBy != "" {
		cur.CreatedBy = in.CreatedBy
	}
	cur.Version++
	f.rows[in.ID] = cur
	return cur, nil
}

func (f *fakeAutomations) DeleteAgentAutomation(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeAutomations) SetAgentAutomationPaused(_ context.Context, id string, paused bool, reason string) (store.AgentAutomation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cur, ok := f.rows[id]
	if !ok {
		return store.AgentAutomation{}, store.ErrNotFound
	}
	cur.Enabled, cur.PausedReason = !paused, ""
	if paused {
		cur.PausedReason = reason
	} else {
		cur.ConsecutiveFailures = 0
	}
	cur.Version++
	f.rows[id] = cur
	return cur, nil
}

func automationTestService(ws string, automations AutomationStore) (*Service, *auditCapture) {
	svc := environmentService(ws, newFakeEnvironments())
	svc.Authz = roleChecker{workspace: ws, roles: map[string]string{
		"admin": "admin", "dev": "developer", "dev2": "developer", "viewer": "viewer",
	}}
	audit := &auditCapture{}
	svc.Audit = audit
	svc.Automations = automations
	// A fixed instant, not the wall clock: schedules, "past" checks and
	// nextFireAt all read it, so no test depends on when it runs.
	svc.Clock = func() time.Time { return automationTestNow }
	return svc, audit
}

// automationTestNow is the instant every automationTestService reads.
var automationTestNow = time.Date(2026, time.November, 7, 10, 0, 0, 0, time.UTC)

func validAutomationTemplate() *AutomationTemplate {
	return &AutomationTemplate{
		Task: "update the dependencies", Repo: "bex-co/example",
		AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"},
	}
}

func dailyTrigger(at string) []AutomationTrigger {
	return []AutomationTrigger{{Type: "schedule", Schedule: &AutomationSchedule{Frequency: "day", At: at, Timezone: "Europe/Berlin"}}}
}

func automationInput(name string) AgentAutomationInput {
	return AgentAutomationInput{Name: str(name), Triggers: dailyTrigger("09:00"), Template: validAutomationTemplate()}
}

func TestAutomationAuthorizationAndLifecycle(t *testing.T) {
	exerciseAutomationAuthorization(t, func() (*Service, *auditCapture, string) {
		svc, audit := automationTestService("tea-a", newFakeAutomations())
		return svc, audit, "tea-a"
	})
}

func exerciseAutomationAuthorization(t *testing.T, build func() (*Service, *auditCapture, string)) {
	t.Helper()
	svc, audit, ws := build()
	devCtx, otherCtx, adminCtx, viewerCtx := caller("dev"), caller("dev2"), caller("admin"), caller("viewer")

	if _, err := svc.CreateAgentAutomation(viewerCtx, ws, automationInput("nightly")); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a viewer created an automation: %v", err)
	}
	if _, err := svc.AgentAutomations(viewerCtx, ws, false, "", 0); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a viewer listed automations: %v", err)
	}

	created, err := svc.CreateAgentAutomation(devCtx, ws, automationInput("nightly"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !strings.HasPrefix(created.ID, "aut-") || created.CreatedBy != "dev" || !created.Enabled || created.Version != 1 ||
		created.OverlapPolicy != "skip" || created.NextFireAt == nil || !created.NextFireAt.After(automationTestNow) ||
		len(created.Triggers) != 1 || created.Template.AgentConfig.Agent != "codex" {
		t.Fatalf("created = %+v", created)
	}

	// Another developer lists, reads, but cannot change it.
	if got, err := svc.AgentAutomation(otherCtx, ws, "nightly"); err != nil || got.ID != created.ID {
		t.Fatalf("get by name = %+v, %v", got, err)
	}
	_, err = svc.UpdateAgentAutomation(otherCtx, ws, created.ID, AgentAutomationInput{Name: str("renamed")})
	if !isCode(err, CodeAutomationNotCreator) || !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-creator update = %v, want %s (403)", err, CodeAutomationNotCreator)
	}
	// Not even an admin edits another member's template or triggers.
	if _, err = svc.UpdateAgentAutomation(adminCtx, ws, created.ID, AgentAutomationInput{Template: validAutomationTemplate()}); !isCode(err, CodeAutomationNotCreator) {
		t.Fatalf("admin update = %v, want %s", err, CodeAutomationNotCreator)
	}
	// A non-creator developer cannot pause or delete either.
	if _, err = svc.PauseAgentAutomation(otherCtx, ws, created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-creator pause = %v", err)
	}
	if err = svc.DeleteAgentAutomation(otherCtx, ws, created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-creator delete = %v", err)
	}
	if _, err = svc.TakeOverAgentAutomation(devCtx, ws, created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("developer take-over = %v", err)
	}

	// The creator may pause and resume their own; resume moves nextFireAt.
	paused, err := svc.PauseAgentAutomation(devCtx, ws, created.ID)
	if err != nil || paused.Enabled {
		t.Fatalf("creator pause = %+v, %v", paused, err)
	}
	// An admin may pause, resume and delete anyone's.
	resumed, err := svc.ResumeAgentAutomation(adminCtx, ws, "nightly")
	if err != nil || !resumed.Enabled || resumed.PausedReason != "" || resumed.NextFireAt == nil {
		t.Fatalf("admin resume = %+v, %v", resumed, err)
	}
	if resumed.CreatedBy != "dev" {
		t.Fatalf("resume changed the creator to %q", resumed.CreatedBy)
	}
	if paused, err = svc.PauseAgentAutomation(adminCtx, ws, created.ID); err != nil || paused.Enabled {
		t.Fatalf("admin pause = %+v, %v", paused, err)
	}

	// Take-over: admin only, sets createdBy and keeps the template.
	taken, err := svc.TakeOverAgentAutomation(adminCtx, ws, created.ID)
	if err != nil || taken.CreatedBy != "admin" || taken.Template.Task != created.Template.Task {
		t.Fatalf("take-over = %+v, %v", taken, err)
	}
	if taken.Enabled {
		t.Fatal("a take-over resumed a manually paused automation")
	}
	// The previous creator is now a stranger to it.
	if _, err = svc.UpdateAgentAutomation(devCtx, ws, created.ID, AgentAutomationInput{Name: str("x")}); !isCode(err, CodeAutomationNotCreator) {
		t.Fatalf("old creator update = %v", err)
	}
	if updated, err := svc.UpdateAgentAutomation(adminCtx, ws, created.ID, AgentAutomationInput{Name: str("weekly")}); err != nil || updated.Name != "weekly" {
		t.Fatalf("new creator update = %+v, %v", updated, err)
	}
	if err = svc.DeleteAgentAutomation(adminCtx, ws, created.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
	if _, err = svc.AgentAutomation(adminCtx, ws, created.ID); !isCode(err, CodeAutomationNotFound) {
		t.Fatalf("get after delete = %v", err)
	}

	// Every write left an audit event against the automation target.
	writes := 0
	for _, ev := range audit.events {
		if ev.Target == AgentAutomationAuditTarget && ev.Outcome == core.AuditAllowed {
			writes++
		}
	}
	denied := 0
	for _, ev := range audit.events {
		if ev.Target == AgentAutomationAuditTarget && ev.Outcome == core.AuditDenied {
			denied++
		}
	}
	if writes < 8 || denied < 2 {
		t.Fatalf("audit: %d allowed, %d denied automation events in %s", writes, denied, audit.dump())
	}
	verbs := map[string]bool{}
	for _, ev := range audit.events {
		verbs[ev.Verb] = true
	}
	for _, verb := range []string{"CreateAgentAutomation", "UpdateAgentAutomation", "PauseAgentAutomation", "ResumeAgentAutomation", "TakeOverAgentAutomation", "DeleteAgentAutomation"} {
		found := false
		for v := range verbs {
			found = found || strings.HasSuffix(v, verb)
		}
		if !found {
			t.Errorf("no audit event for %s: %v", verb, verbs)
		}
	}
}

func TestAutomationTriggerAndTemplateValidation(t *testing.T) {
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	ctx := caller("dev")
	create := func(mutate func(*AgentAutomationInput)) error {
		in := automationInput("auto-" + ids.New(ids.AgentAutomation))
		mutate(&in)
		_, err := svc.CreateAgentAutomation(ctx, "tea-a", in)
		return err
	}
	for name, mutate := range map[string]func(*AgentAutomationInput){
		"no triggers":  func(in *AgentAutomationInput) { in.Triggers = nil },
		"two triggers": func(in *AgentAutomationInput) { in.Triggers = append(dailyTrigger("09:00"), dailyTrigger("10:00")...) },
		"wrong type":   func(in *AgentAutomationInput) { in.Triggers[0].Type = "webhook" },
		"no schedule":  func(in *AgentAutomationInput) { in.Triggers[0].Schedule = nil },
		"no template":  func(in *AgentAutomationInput) { in.Template = nil },
		"bad name":     func(in *AgentAutomationInput) { in.Name = str("Not A Slug") },
		"ifVersion":    func(in *AgentAutomationInput) { in.IfVersion = ptr(1) },
	} {
		if err := create(mutate); !isCode(err, CodeAutomationInvalid) {
			t.Errorf("%s = %v, want %s", name, err, CodeAutomationInvalid)
		}
	}
	if err := create(func(in *AgentAutomationInput) { in.Triggers[0].Schedule.Timezone = "" }); !isCode(err, CodeAutomationScheduleInvalid) {
		t.Errorf("no timezone = %v", err)
	}
	if err := create(func(in *AgentAutomationInput) {
		in.Triggers = []AutomationTrigger{{Type: "schedule", Schedule: &AutomationSchedule{Frequency: "custom", Cron: "* * * * *", Timezone: "UTC"}}}
	}); !isCode(err, CodeAutomationScheduleTooFrequent) {
		t.Errorf("every minute = %v", err)
	}
	past := automationTestNow.Add(-time.Hour)
	if err := create(func(in *AgentAutomationInput) {
		in.Triggers = []AutomationTrigger{{Type: "schedule", Schedule: &AutomationSchedule{Frequency: "once", RunAt: &past, Timezone: "UTC"}}}
	}); !isCode(err, CodeAutomationSchedulePast) {
		t.Errorf("past once = %v", err)
	}

	// Unknown template fields never decode, so a template cannot carry
	// resumeSessionId, attachments or an idempotency key.
	for _, field := range []string{"resumeSessionId", "attachmentIds", "idempotencyKey", "approvalMode"} {
		var in AgentAutomationInput
		body := fmt.Sprintf(`{"name":"x","template":{"task":"t","%s":"v","agentConfig":{"agent":"codex"}}}`, field)
		if err := json.Unmarshal([]byte(body), &in); !isCode(err, CodeAutomationInvalid) {
			t.Errorf("template field %s decoded: %v", field, err)
		}
	}
}

// A template that a manual create refuses is refused on save with the same
// error code, because both run the same validation.
func TestAutomationTemplateFailsLikeManualCreate(t *testing.T) {
	svc, _, _, _ := fixture()
	svc.Automations = newFakeAutomations()
	ctx := caller("alice")
	cases := map[string]func(*AutomationTemplate){
		"bad repo":         func(tpl *AutomationTemplate) { tpl.Repo = "not a repo" },
		"bad model":        func(tpl *AutomationTemplate) { tpl.AgentConfig.Model = "bad model id" },
		"bad branch":       func(tpl *AutomationTemplate) { tpl.Branch = "main" },
		"bad secret name":  func(tpl *AutomationTemplate) { tpl.SecretNames = []string{"lower case!"} },
		"unknown agent":    func(tpl *AutomationTemplate) { tpl.AgentConfig.Agent = "nope" },
		"empty task":       func(tpl *AutomationTemplate) { tpl.Task = " " },
		"bad skill name":   func(tpl *AutomationTemplate) { tpl.Skills = []SkillInvocation{{Name: "Bad Name"}} },
		"bad egress":       func(tpl *AutomationTemplate) { tpl.EgressAllowlist = []string{"not a host!"} },
		"bad spend":        func(tpl *AutomationTemplate) { tpl.MaxSpendUSD = ptr(-1.0) },
		"spend unmetered":  func(tpl *AutomationTemplate) { tpl.MaxSpendUSD = ptr(5.0) },
		"bad output shape": func(tpl *AutomationTemplate) { tpl.OutputSchema = map[string]any{"type": 5} },
	}
	for name, mutate := range cases {
		tpl := validAutomationTemplate()
		tpl.Repo = "bex-co/example"
		mutate(tpl)
		in := createInput()
		in.Repo, in.Branch = tpl.Repo, tpl.Branch
		in.AgentConfig = AgentConfig{Agent: tpl.AgentConfig.Agent, Model: tpl.AgentConfig.Model, Task: tpl.Task, ModelEndpoint: "https://api.openai.com/v1"}
		in.EgressAllowlist, in.Skills, in.MaxSpendUSD = tpl.EgressAllowlist, tpl.Skills, tpl.MaxSpendUSD
		in.SecretNames, in.OutputSchema = tpl.SecretNames, tpl.OutputSchema
		_, manual := svc.Create(ctx, in)
		_, auto := svc.CreateAgentAutomation(ctx, "tea-a", AgentAutomationInput{Name: str("t-" + strings.ReplaceAll(name, " ", "-")), Triggers: dailyTrigger("09:00"), Template: tpl})
		var want, got *core.CodedError
		if !errors.As(manual, &want) {
			t.Errorf("%s: manual create succeeded or is uncoded: %v", name, manual)
			continue
		}
		if !errors.As(auto, &got) || got.Code != want.Code {
			t.Errorf("%s: automation = %v, manual create = %v", name, auto, manual)
		}
	}
	// The valid template saves.
	if _, err := svc.CreateAgentAutomation(ctx, "tea-a", automationInput("valid")); err != nil {
		t.Fatalf("valid template: %v", err)
	}
}

func TestAutomationPlatformMeteredTemplateNeedsSpendCeiling(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	ctx := caller("dev")
	in := automationInput("metered")
	in.Template = &AutomationTemplate{Task: "triage", Repo: "bex-co/example", AgentConfig: AutomationAgentConfig{Agent: "claude"}}
	if _, err := svc.CreateAgentAutomation(ctx, "tea-a", in); !isCode(err, CodeAutomationSpendLimitNeeded) {
		t.Fatalf("metered without maxSpendUSD = %v, want %s", err, CodeAutomationSpendLimitNeeded)
	}
	in.Template.MaxSpendUSD = ptr(2.5)
	created, err := svc.CreateAgentAutomation(ctx, "tea-a", in)
	if err != nil {
		t.Fatalf("metered with maxSpendUSD: %v", err)
	}
	// Dropping the ceiling on update is refused too.
	noCeiling := *in.Template
	noCeiling.MaxSpendUSD = nil
	if _, err := svc.UpdateAgentAutomation(ctx, "tea-a", created.ID, AgentAutomationInput{Template: &noCeiling}); !isCode(err, CodeAutomationSpendLimitNeeded) {
		t.Fatalf("update dropping the ceiling = %v", err)
	}
}

func TestAutomationVersionNameAndLimit(t *testing.T) {
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	ctx := caller("dev")
	a, err := svc.CreateAgentAutomation(ctx, "tea-a", automationInput("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CreateAgentAutomation(ctx, "tea-a", automationInput("alpha")); !isCode(err, CodeAutomationConflict) || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate name = %v, want %s (409)", err, CodeAutomationConflict)
	}
	b, err := svc.CreateAgentAutomation(ctx, "tea-a", automationInput("beta"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateAgentAutomation(ctx, "tea-a", b.ID, AgentAutomationInput{Name: str("alpha")}); !isCode(err, CodeAutomationConflict) {
		t.Fatalf("rename onto a taken name = %v", err)
	}
	updated, err := svc.UpdateAgentAutomation(ctx, "tea-a", a.ID, AgentAutomationInput{Name: str("gamma"), IfVersion: ptr(1)})
	if err != nil || updated.Version != 2 || updated.Name != "gamma" {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	if _, err = svc.UpdateAgentAutomation(ctx, "tea-a", a.ID, AgentAutomationInput{Name: str("delta"), IfVersion: ptr(1)}); !isCode(err, CodeAutomationConflict) {
		t.Fatalf("stale ifVersion = %v, want %s", err, CodeAutomationConflict)
	}

	for i := 2; i < maxWorkspaceAutomations; i++ {
		if _, err = svc.CreateAgentAutomation(ctx, "tea-a", automationInput(fmt.Sprintf("auto-%d", i))); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if _, err = svc.CreateAgentAutomation(ctx, "tea-a", automationInput("one-too-many")); !isCode(err, CodeAutomationLimit) {
		t.Fatalf("over the limit = %v, want %s", err, CodeAutomationLimit)
	}
}

func TestAutomationNextFireAtMovesOnTriggerChangeAndResume(t *testing.T) {
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	svc.Clock = func() time.Time { return now }
	ctx := caller("dev")
	created, err := svc.CreateAgentAutomation(ctx, "tea-a", automationInput("daily"))
	if err != nil {
		t.Fatal(err)
	}
	// 09:00 Berlin is 08:00 UTC; at 10:00 UTC on Nov 7 the next one is Nov 8.
	if want := time.Date(2026, 11, 8, 8, 0, 0, 0, time.UTC); created.NextFireAt == nil || !created.NextFireAt.Equal(want) {
		t.Fatalf("nextFireAt = %v, want %v", created.NextFireAt, want)
	}
	changed, err := svc.UpdateAgentAutomation(ctx, "tea-a", created.ID, AgentAutomationInput{Triggers: dailyTrigger("23:30")})
	if err != nil || !changed.NextFireAt.Equal(time.Date(2026, 11, 7, 22, 30, 0, 0, time.UTC)) {
		t.Fatalf("after trigger change = %+v, %v", changed, err)
	}
	if renamed, err := svc.UpdateAgentAutomation(ctx, "tea-a", created.ID, AgentAutomationInput{Name: str("renamed")}); err != nil || !renamed.NextFireAt.Equal(*changed.NextFireAt) {
		t.Fatalf("rename moved nextFireAt: %+v, %v", renamed, err)
	}
	if disabled, err := svc.UpdateAgentAutomation(ctx, "tea-a", created.ID, AgentAutomationInput{Enabled: ptr(false)}); err != nil || disabled.NextFireAt != nil || disabled.Enabled {
		t.Fatalf("disable = %+v, %v", disabled, err)
	}

	// A day passes while it is paused; resume computes from now.
	paused, err := svc.PauseAgentAutomation(ctx, "tea-a", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(48 * time.Hour)
	resumed, err := svc.ResumeAgentAutomation(ctx, "tea-a", paused.ID)
	if err != nil || resumed.NextFireAt == nil || !resumed.NextFireAt.Equal(time.Date(2026, 11, 9, 22, 30, 0, 0, time.UTC)) {
		t.Fatalf("resume = %+v, %v", resumed, err)
	}

	// A paused automation cannot be re-enabled through update.
	store := svc.Automations.(*fakeAutomations)
	if _, err = store.SetAgentAutomationPaused(context.Background(), created.ID, true, AutomationPausedRepeatedFailures); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateAgentAutomation(ctx, "tea-a", created.ID, AgentAutomationInput{Enabled: ptr(true)}); !isCode(err, CodeAutomationPaused) {
		t.Fatalf("enable a paused automation = %v", err)
	}
}

func TestAutomationTakeOverResumesCreatorLostAccess(t *testing.T) {
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	created, err := svc.CreateAgentAutomation(caller("dev"), "tea-a", automationInput("orphan"))
	if err != nil {
		t.Fatal(err)
	}
	st := svc.Automations.(*fakeAutomations)
	if _, err = st.SetAgentAutomationPaused(context.Background(), created.ID, true, AutomationPausedCreatorLostAccess); err != nil {
		t.Fatal(err)
	}
	taken, err := svc.TakeOverAgentAutomation(caller("admin"), "tea-a", created.ID)
	if err != nil || taken.CreatedBy != "admin" || !taken.Enabled || taken.PausedReason != "" || taken.NextFireAt == nil {
		t.Fatalf("take-over of creator_lost_access = %+v, %v", taken, err)
	}

	// A template that no longer validates refuses the take-over and changes nothing.
	bad, err := svc.CreateAgentAutomation(caller("dev"), "tea-a", automationInput("stale"))
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[bad.ID]
	row.Template = json.RawMessage(`{"task":"t","repo":"not a repo","agentConfig":{"agent":"codex"}}`)
	st.rows[bad.ID] = row
	if _, err = svc.TakeOverAgentAutomation(caller("admin"), "tea-a", bad.ID); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("take-over of an invalid template = %v", err)
	}
	if st.rows[bad.ID].CreatedBy != "dev" {
		t.Fatal("a refused take-over changed the creator")
	}
}

func TestAutomationListMineAndPaging(t *testing.T) {
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	for i, who := range []string{"dev", "dev2", "dev", "dev2", "dev"} {
		if _, err := svc.CreateAgentAutomation(caller(who), "tea-a", automationInput(fmt.Sprintf("auto-%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	all, err := svc.AgentAutomations(caller("dev"), "tea-a", false, "", 0)
	if err != nil || len(all.Automations) != 5 || all.NextCursor != "" {
		t.Fatalf("all = %+v, %v", all, err)
	}
	mine, err := svc.AgentAutomations(caller("dev"), "tea-a", true, "", 0)
	if err != nil || len(mine.Automations) != 3 {
		t.Fatalf("mine = %d, %v", len(mine.Automations), err)
	}
	var seen []string
	cursor := ""
	for {
		page, err := svc.AgentAutomations(caller("dev"), "tea-a", false, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range page.Automations {
			seen = append(seen, a.Name)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if want := []string{"auto-4", "auto-3", "auto-2", "auto-1", "auto-0"}; !slices.Equal(seen, want) {
		t.Fatalf("paged = %v, want %v", seen, want)
	}
}

func TestAutomationsUnavailableWithoutStore(t *testing.T) {
	svc, _ := automationTestService("tea-a", nil)
	svc.Automations = nil
	if _, err := svc.AgentAutomations(caller("dev"), "tea-a", false, "", 0); !isCode(err, CodeAutomationsUnavailableError) {
		t.Fatalf("list = %v", err)
	}
	if _, err := svc.CreateAgentAutomation(caller("dev"), "tea-a", automationInput("x")); !isCode(err, CodeAutomationsUnavailableError) {
		t.Fatalf("create = %v", err)
	}
	if _, err := svc.TakeOverAgentAutomation(caller("admin"), "tea-a", "aut-abcdefghijklmnopqrst"); !isCode(err, CodeAutomationsUnavailableError) {
		t.Fatalf("take-over = %v", err)
	}
}
