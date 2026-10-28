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

package skills

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	upstreamcmd "github.com/render-oss/cli/cmd"
	upstream "github.com/render-oss/cli/pkg/skills"
	"github.com/spf13/pflag"
)

// userDirecting matches a reference that would send an agent to operate
// Render instead of bex (w8/m143 definition of done). Render's docs stay
// linked as the behavior reference, so render.com/docs itself is allowed.
var userDirecting = regexp.MustCompile(`mcp\.render\.com|dashboard\.render\.com|api\.render\.com|onrender\.com|render\.com/(pricing|contact)|status\.render\.com|renderdns\.com|render\.com/docs/(cli|cli-reference|mcp-server)\b|RENDER_API_KEY|~/\.render/|brew install render\b|(^|[\s\x60$(|;&"'/])render (blueprints|deploys|docs|ea|environments|help|jobs|keyvalues|kv-cli|login|logout|logs|pgcli|postgres|projects|psql|restart|resume|services|skills|ssh|suspend|whoami|workspaces?|workflows|sandboxes|--version)\b|\brender-[a-z]+|bex-oss`)

func TestPackOperatesBexOnly(t *testing.T) {
	skills := 0
	err := fs.WalkDir(pack, "pack/skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(path, "pack/skills/")
		dir, _, _ := strings.Cut(rel, "/")
		if !strings.HasPrefix(dir, prefix) {
			t.Errorf("%s: skill directory is not %s*", rel, prefix)
		}
		data, err := pack.ReadFile(path)
		if err != nil {
			return err
		}
		for n, line := range strings.Split(string(data), "\n") {
			if m := userDirecting.FindString(strings.ReplaceAll(line, "render-oss/skills", "")); m != "" {
				t.Errorf("%s:%d: user-directing Render reference %q", rel, n+1, m)
			}
		}
		if strings.HasSuffix(rel, "/SKILL.md") {
			skills++
			if !strings.Contains(string(data), "\nname: "+dir+"\n") {
				t.Errorf("%s: frontmatter name is not %s", rel, dir)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if skills != 20 {
		t.Errorf("pack has %d skills, want 20 (render-oss/skills minus workflows and sandboxes)", skills)
	}
	for _, dropped := range []string{"bex-workflows", "bex-sandboxes"} {
		if _, err := fs.Stat(pack, "pack/skills/"+dropped); err == nil {
			t.Errorf("%s ships, but it is a bex non-goal", dropped)
		}
	}
	mcp, err := pack.ReadFile("pack/skills/bex-mcp/SKILL.md")
	if err != nil || !strings.Contains(string(mcp), "https://api.bex.co/mcp") {
		t.Errorf("bex-mcp does not point at https://api.bex.co/mcp: %v", err)
	}
	if license, err := pack.ReadFile("pack/LICENSE"); err != nil || !strings.Contains(string(license), "Copyright (c) 2026 Render") {
		t.Errorf("pack LICENSE lacks Render's MIT attribution: %v", err)
	}
}

// sandbox points HOME, the config dir and the working directory at a fresh
// tree holding one detected tool with a genuine Render skill, plus Render's
// own state file.
func sandbox(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("RENDER_CLI_CONFIG_DIR", filepath.Join(home, ".bex"))
	t.Chdir(home)
	write(t, filepath.Join(home, ".claude", "skills", "render-deploy", "SKILL.md"), "---\nname: render-deploy\n---\nRender's own skill\n")
	write(t, filepath.Join(home, ".render", "skills.yaml"), "tools: [Claude Code]\n")
	return home
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func installedSkills(t *testing.T, home string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, ".claude", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// Render's skills and state survive every subcommand (w8/m143 DoD).
func TestLifecycleLeavesRenderSkillsAlone(t *testing.T) {
	home := sandbox(t)
	user := target{scope: upstream.ScopeUser}
	tools, err := detectTools("claude")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := install(user, tools, []string{"bex-cli"}, true); err != nil {
		t.Fatal(err)
	}
	if got := installedSkills(t, home); !slices.Equal(got, []string{"render-deploy"}) {
		t.Fatalf("dry run wrote %v", got)
	}

	result, err := install(user, tools, nil, false)
	if err != nil || len(result.Skills) != 20 {
		t.Fatalf("install = %d skills, %v", len(result.Skills), err)
	}
	if got := installedSkills(t, home); len(got) != 21 || !slices.Contains(got, "render-deploy") {
		t.Fatalf("after install: %v", got)
	}

	updated, err := update(user, false)
	if err != nil || len(updated.Skills) != 0 {
		t.Fatalf("update of a current install = %v, %v; want nothing", updated.Skills, err)
	}
	write(t, filepath.Join(home, ".claude", "skills", "bex-cli", "SKILL.md"), "edited")
	state, _ := loadState()
	for i := range state.Skills {
		if state.Skills[i].DirName == "bex-cli" {
			state.Skills[i].Hash = "stale"
		}
	}
	if err := saveState(state); err != nil {
		t.Fatal(err)
	}
	if updated, err = update(user, false); err != nil || len(updated.Skills) != 1 || updated.Skills[0].DirName != "bex-cli" {
		t.Fatalf("update after a pack change = %v, %v; want bex-cli", updated.Skills, err)
	}

	// A file dropped from the pack does not outlive an update.
	write(t, filepath.Join(home, ".claude", "skills", "bex-cli", "stale.md"), "old")
	if _, err := update(user, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "bex-cli", "stale.md")); err == nil {
		t.Fatal("update kept a file the pack no longer has")
	}

	// Removing from one tool keeps the skill in state while another
	// recorded tool still has it.
	write(t, filepath.Join(home, ".agents", "skills", ".keep"), "")
	codex, err := detectTools("codex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := install(user, codex, []string{"bex-mcp"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := remove(user, codex, []string{"bex-mcp"}, false); err != nil {
		t.Fatal(err)
	}
	if state, _ := loadState(); !slices.ContainsFunc(state.Skills, func(s upstream.InstalledSkill) bool { return s.DirName == "bex-mcp" }) {
		t.Fatal("a --tool removal dropped bex-mcp from state while Claude Code still has it")
	}
	all, err := detectTools("")
	if err != nil {
		t.Fatal(err)
	}
	tools = all
	if _, err := remove(user, tools, []string{"bex-mcp"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := remove(user, tools, []string{"render-deploy"}, false); err == nil {
		t.Fatal("remove accepted a Render skill")
	}
	if _, err := remove(user, tools, nil, true); err != nil {
		t.Fatal(err)
	}
	if got := installedSkills(t, home); !slices.Equal(got, []string{"render-deploy"}) {
		t.Fatalf("after remove --all: %v", got)
	}
	if data, _ := os.ReadFile(filepath.Join(home, ".render", "skills.yaml")); string(data) != "tools: [Claude Code]\n" {
		t.Fatalf("Render's skills.yaml changed: %q", data)
	}
	if _, err := os.Stat(filepath.Join(home, ".bex", stateFile)); err != nil {
		t.Fatalf("bex state not in the bex config dir: %v", err)
	}
}

// Upstream's installer deletes every render-* skill for an empty selection;
// no path may hand it one.
func TestGuardSelection(t *testing.T) {
	for _, selected := range [][]string{nil, {"render-deploy"}, {"bex-cli", "render"}} {
		if guardSelection(selected) == nil {
			t.Errorf("guardSelection(%v) passed", selected)
		}
	}
	if err := guardSelection([]string{"bex-cli"}); err != nil {
		t.Error(err)
	}
}

// The subcommands keep the pinned Render CLI's surface (w8/m143 t004).
func TestCommandSurfaceMatchesUpstream(t *testing.T) {
	var theirs = map[string][]string{}
	for _, c := range upstreamcmd.RootCmd.Commands() {
		if c.Name() == "skills" {
			for _, sub := range c.Commands() {
				theirs[sub.Name()] = flagNames(sub.Flags())
			}
		}
	}
	if len(theirs) == 0 {
		t.Fatal("upstream skills command not found")
	}
	ours := map[string][]string{}
	for _, sub := range Command().Commands() {
		ours[sub.Name()] = flagNames(sub.Flags())
	}
	for name, flags := range theirs {
		if !slices.Equal(ours[name], flags) {
			t.Errorf("skills %s flags = %v, upstream %v", name, ours[name], flags)
		}
	}
	if len(ours) != len(theirs) {
		t.Errorf("subcommands = %v, upstream %v", ours, theirs)
	}
}

func flagNames(flags *pflag.FlagSet) []string {
	var names []string
	flags.VisitAll(func(f *pflag.Flag) { names = append(names, f.Name) })
	slices.Sort(names)
	return names
}
