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

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The workspace agent environment (w10/m17): no row until first written, a
// partial upsert keeps every field it does not name, the CHECKs bound the
// tier and host count, and the row goes with its workspace.
func TestAgentEnvironmentUpsertIsPartialAndBounded(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-environment", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	if _, err := st.GetAgentEnvironment(ctx, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unwritten environment error = %v, want ErrNotFound", err)
	}
	model := "claude-sonnet-4-5"
	got, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DefaultModel: &model})
	if err != nil {
		t.Fatal(err)
	}
	if got.NetworkTier != "registries" || got.CustomHosts == nil || len(got.CustomHosts) != 0 || got.DefaultModel != model || got.UpdatedAt.IsZero() {
		t.Fatalf("first partial write = %+v, want the column defaults plus the model", got)
	}
	tier, hosts := "custom", []string{"a.example.com", "b.example.com"}
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &tier, CustomHosts: &hosts}); err != nil {
		t.Fatal(err)
	}
	if got.NetworkTier != tier || !slices.Equal(got.CustomHosts, hosts) || got.DefaultModel != model {
		t.Fatalf("second partial write = %+v, lost the model or the hosts", got)
	}
	read, err := st.GetAgentEnvironment(ctx, tenant.ID)
	if err != nil || read.NetworkTier != tier || !slices.Equal(read.CustomHosts, hosts) || read.DefaultModel != model {
		t.Fatalf("read back = %+v err=%v", read, err)
	}
	empty := []string{}
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{CustomHosts: &empty}); err != nil || len(got.CustomHosts) != 0 || got.NetworkTier != tier {
		t.Fatalf("clearing the hosts = %+v err=%v", got, err)
	}

	// The spend cap override (ADR091 D5): set, kept by other partial writes,
	// cleared back to the plan default; zero is refused by the CHECK.
	capMicro := int64(50_000_000)
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{SpendCapOverrideMicroUSD: &capMicro}); err != nil ||
		got.SpendCapOverrideMicroUSD == nil || *got.SpendCapOverrideMicroUSD != capMicro || got.NetworkTier != tier {
		t.Fatalf("setting the cap = %+v err=%v", got, err)
	}
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DefaultModel: &model}); err != nil || got.SpendCapOverrideMicroUSD == nil {
		t.Fatalf("a model write lost the cap: %+v err=%v", got, err)
	}
	if got, err = st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{ClearSpendCapOverride: true}); err != nil || got.SpendCapOverrideMicroUSD != nil {
		t.Fatalf("clearing the cap = %+v err=%v", got, err)
	}
	zero := int64(0)
	if _, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{SpendCapOverrideMicroUSD: &zero}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero cap error = %v, want ErrInvalid", err)
	}

	bad := "internet"
	if _, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &bad}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown tier error = %v, want ErrInvalid", err)
	}
	tooMany := make([]string, 33)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("h%d.example.com", i)
	}
	if _, err := st.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{CustomHosts: &tooMany}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("33 hosts error = %v, want ErrInvalid", err)
	}
	if _, err := st.UpsertAgentEnvironment(ctx, "tea-00000000000000000000", AgentEnvironmentPatch{NetworkTier: &tier}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown workspace error = %v, want ErrNotFound", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetAgentEnvironment(ctx, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("environment survived its workspace: %v", err)
	}
}

// w7/m392: the turned-off agents round-trip and a later partial update keeps
// them; an acknowledgement is recorded once and the first one is kept.
func TestAgentEnvironmentAgentAvailabilityRoundTrip(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-availability")
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM agent_environments WHERE workspace_id = $1`, tenant.ID) })

	disabled := []string{"codex", "muse"}
	got, err := s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DisabledAgents: &disabled})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.DisabledAgents, disabled) || len(got.DataUseAcks) != 0 {
		t.Fatalf("first write = %v / %v", got.DisabledAgents, got.DataUseAcks)
	}
	first, err := s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{AcknowledgeDataUse: []string{"muse"}, AcknowledgedBy: "usr-first"})
	if err != nil {
		t.Fatal(err)
	}
	ack, ok := first.DataUseAcks["muse"]
	if !ok || ack.By != "usr-first" || ack.At.IsZero() || !slices.Equal(first.DisabledAgents, disabled) {
		t.Fatalf("after acknowledgement = %+v, disabled %v", first.DataUseAcks, first.DisabledAgents)
	}
	none := []string{}
	again, err := s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DisabledAgents: &none, AcknowledgeDataUse: []string{"muse"}, AcknowledgedBy: "usr-second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.DisabledAgents) != 0 || again.DataUseAcks["muse"].By != "usr-first" || !again.DataUseAcks["muse"].At.Equal(ack.At) {
		t.Fatalf("second write = %v / %+v, want the first acknowledgement kept", again.DisabledAgents, again.DataUseAcks)
	}
	read, err := s.GetAgentEnvironment(ctx, tenant.ID)
	if err != nil || read.DataUseAcks["muse"].By != "usr-first" {
		t.Fatalf("read = %+v, %v", read.DataUseAcks, err)
	}
}

// w10/m524: a first write that does not name the GitHub CLI setting leaves
// `gh` on, an explicit off reads back and survives a partial write of another
// field, and an explicit on restores it.
func TestAgentEnvironmentGitHubCLISettingRoundTrip(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-github-cli")
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM agent_environments WHERE workspace_id = $1`, tenant.ID) })

	model := "claude-sonnet-4-5"
	got, err := s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DefaultModel: &model})
	if err != nil || got.GitHubCLIDisabled {
		t.Fatalf("first write without the setting = %+v err=%v, want gh on", got, err)
	}
	off, on := true, false
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{GitHubCLIDisabled: &off}); err != nil || !got.GitHubCLIDisabled || got.DefaultModel != model {
		t.Fatalf("turning gh off = %+v err=%v", got, err)
	}
	tier := "full"
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &tier}); err != nil || !got.GitHubCLIDisabled {
		t.Fatalf("a tier write lost the setting: %+v err=%v", got, err)
	}
	if read, err := s.GetAgentEnvironment(ctx, tenant.ID); err != nil || !read.GitHubCLIDisabled {
		t.Fatalf("read back = %+v err=%v, want gh off", read, err)
	}
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{GitHubCLIDisabled: &on}); err != nil || got.GitHubCLIDisabled || got.NetworkTier != tier {
		t.Fatalf("turning gh back on = %+v err=%v", got, err)
	}

	// A workspace whose first write turns gh off gets the other defaults.
	other, err := s.CreateTenant(ctx, "agent-github-cli-first", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, other.ID) })
	if got, err = s.UpsertAgentEnvironment(ctx, other.ID, AgentEnvironmentPatch{GitHubCLIDisabled: &off}); err != nil || !got.GitHubCLIDisabled || got.NetworkTier != "registries" {
		t.Fatalf("first write turning gh off = %+v err=%v", got, err)
	}
}

// w10/m561: a row written without the gh bindings inherits and is not
// mandatory; the automations default round-trips through on, off and back to
// inherit (an explicit nil), the mandatory flag through on and off, and a
// partial write of another field keeps both.
func TestAgentEnvironmentGitHubCLIBindingsRoundTrip(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-github-cli-bindings")
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM agent_environments WHERE workspace_id = $1`, tenant.ID) })

	model := "claude-sonnet-4-5"
	got, err := s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DefaultModel: &model})
	if err != nil || got.AutomationsGitHubCLI != nil || got.GitHubCLIMandatory {
		t.Fatalf("first write without the bindings = %+v err=%v, want inherit and not mandatory", got, err)
	}
	off, on, yes := false, true, true
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{SetAutomationsGitHubCLI: true, AutomationsGitHubCLI: &off, GitHubCLIMandatory: &yes}); err != nil ||
		got.AutomationsGitHubCLI == nil || *got.AutomationsGitHubCLI || !got.GitHubCLIMandatory || got.DefaultModel != model {
		t.Fatalf("automations off and mandatory = %+v err=%v", got, err)
	}
	tier := "full"
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &tier, AutomationsGitHubCLI: &on}); err != nil ||
		got.AutomationsGitHubCLI == nil || *got.AutomationsGitHubCLI || !got.GitHubCLIMandatory {
		t.Fatalf("a write that does not set the automations default changed it: %+v err=%v", got, err)
	}
	if read, err := s.GetAgentEnvironment(ctx, tenant.ID); err != nil || read.AutomationsGitHubCLI == nil || *read.AutomationsGitHubCLI || !read.GitHubCLIMandatory {
		t.Fatalf("read back = %+v err=%v", read, err)
	}
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{SetAutomationsGitHubCLI: true, AutomationsGitHubCLI: &on}); err != nil || got.AutomationsGitHubCLI == nil || !*got.AutomationsGitHubCLI {
		t.Fatalf("automations on = %+v err=%v", got, err)
	}
	no := false
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{SetAutomationsGitHubCLI: true, GitHubCLIMandatory: &no}); err != nil || got.AutomationsGitHubCLI != nil || got.GitHubCLIMandatory || got.NetworkTier != tier {
		t.Fatalf("back to inherit and not mandatory = %+v err=%v", got, err)
	}

	// A workspace whose first write binds automations gets the other defaults.
	other, err := s.CreateTenant(ctx, "agent-github-cli-bindings-first", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, other.ID) })
	if got, err = s.UpsertAgentEnvironment(ctx, other.ID, AgentEnvironmentPatch{SetAutomationsGitHubCLI: true, AutomationsGitHubCLI: &off}); err != nil ||
		got.AutomationsGitHubCLI == nil || *got.AutomationsGitHubCLI || got.GitHubCLIMandatory || got.GitHubCLIDisabled || got.NetworkTier != "registries" {
		t.Fatalf("first write binding automations = %+v err=%v", got, err)
	}
}

// w10/m527: a first write that does not name the turned-off models offers the
// whole catalog, a list reads back and survives a partial write of another
// field, and an explicit empty list turns every model back on.
func TestAgentEnvironmentDisabledModelsRoundTrip(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-models")
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM agent_environments WHERE workspace_id = $1`, tenant.ID) })

	tier := "full"
	got, err := s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{NetworkTier: &tier})
	if err != nil || got.DisabledModels == nil || len(got.DisabledModels) != 0 {
		t.Fatalf("first write without the list = %#v err=%v, want none turned off", got.DisabledModels, err)
	}
	off := []string{"anthropic/claude-opus-5.5", "anthropic/claude-sonnet-5.5"}
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DisabledModels: &off}); err != nil || !slices.Equal(got.DisabledModels, off) || got.NetworkTier != tier {
		t.Fatalf("turning models off = %+v err=%v", got, err)
	}
	model := "anthropic/claude-haiku-5.5"
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DefaultModel: &model}); err != nil || !slices.Equal(got.DisabledModels, off) {
		t.Fatalf("a default-model write lost the list: %+v err=%v", got, err)
	}
	if read, err := s.GetAgentEnvironment(ctx, tenant.ID); err != nil || !slices.Equal(read.DisabledModels, off) {
		t.Fatalf("read back = %+v err=%v", read, err)
	}
	none := []string{}
	if got, err = s.UpsertAgentEnvironment(ctx, tenant.ID, AgentEnvironmentPatch{DisabledModels: &none}); err != nil || len(got.DisabledModels) != 0 || got.DefaultModel != model {
		t.Fatalf("turning every model back on = %+v err=%v", got, err)
	}

	// A workspace whose first write turns a model off gets the other defaults.
	other, err := s.CreateTenant(ctx, "agent-models-first", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, other.ID) })
	if got, err = s.UpsertAgentEnvironment(ctx, other.ID, AgentEnvironmentPatch{DisabledModels: &off}); err != nil || !slices.Equal(got.DisabledModels, off) || got.NetworkTier != "registries" || got.GitHubCLIDisabled {
		t.Fatalf("first write turning models off = %+v err=%v", got, err)
	}
}
