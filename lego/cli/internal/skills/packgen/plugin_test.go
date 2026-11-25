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

package packgen

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is the repository root, seen from this package.
const repoRoot = "../../../../.."

// The plugin in the repository is exactly what the pack and packgen/plugin
// generate (w4/m631): a skill changed in the pack but not in the plugin, a
// hand edit to the plugin, or a stale version fails here.
func TestPluginMatchesPack(t *testing.T) {
	root := t.TempDir()
	if err := GeneratePlugin("../pack", root); err != nil {
		t.Fatal(err)
	}
	for _, problem := range diffTrees(t, root, repoRoot, PluginDir, MarketplaceFile) {
		t.Error(problem)
	}
	if t.Failed() {
		t.Log("regenerate with `go run ./hack/claudeplugin` from lego/cli")
	}
}

func TestPluginDiffNamesEveryDrift(t *testing.T) {
	want, got := t.TempDir(), t.TempDir()
	for _, dir := range []string{want, got} {
		if err := GeneratePlugin("../pack", dir); err != nil {
			t.Fatal(err)
		}
	}
	skill := filepath.Join(got, PluginDir, "skills", "bex-deploy", "SKILL.md")
	if err := os.WriteFile(skill, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(got, PluginDir, "agents", "bex-assistant.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(got, PluginDir, "extra.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(got, PluginDir, "scripts", "validate-render-yaml-hook.sh"), 0o644); err != nil {
		t.Fatal(err)
	}
	problems := strings.Join(diffTrees(t, want, got, PluginDir, MarketplaceFile), "\n")
	for _, drift := range []string{
		"skills/bex-deploy/SKILL.md differs",
		"agents/bex-assistant.md is missing",
		"extra.md is not generated",
		"validate-render-yaml-hook.sh is not executable",
	} {
		if !strings.Contains(problems, drift) {
			t.Errorf("drift %q not reported in:\n%s", drift, problems)
		}
	}
}

// diffTrees compares the generated paths under want with those under got.
func diffTrees(t *testing.T, want, got string, paths ...string) []string {
	t.Helper()
	read := func(root string) map[string]os.FileMode {
		files := map[string]os.FileMode{}
		for _, p := range paths {
			err := filepath.WalkDir(filepath.Join(root, p), func(file string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(root, file)
				files[filepath.ToSlash(rel)] = info.Mode()
				return err
			})
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
		return files
	}
	wantFiles, gotFiles := read(want), read(got)
	var problems []string
	for name, mode := range wantFiles {
		gotMode, ok := gotFiles[name]
		if !ok {
			problems = append(problems, name+" is missing")
			continue
		}
		a, errA := os.ReadFile(filepath.Join(want, name))
		b, errB := os.ReadFile(filepath.Join(got, name))
		if errA != nil || errB != nil {
			t.Fatal(errA, errB)
		}
		if !bytes.Equal(a, b) {
			problems = append(problems, name+" differs from what packgen generates")
		}
		if mode&0o111 != 0 && gotMode&0o111 == 0 {
			problems = append(problems, name+" is not executable")
		}
	}
	for name := range gotFiles {
		if _, ok := wantFiles[name]; !ok {
			problems = append(problems, name+" is not generated")
		}
	}
	return problems
}

// The plugin carries the pack's 20 skills, bex's agent and commands, the
// MCP server with no fixed OAuth client (bex registers clients dynamically),
// and a version that follows its contents.
func TestPluginContents(t *testing.T) {
	root := t.TempDir()
	if err := GeneratePlugin("../pack", root); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(root, PluginDir)
	skills, err := filepath.Glob(filepath.Join(plugin, "skills", "*", "SKILL.md"))
	if err != nil || len(skills) != 20 {
		t.Errorf("plugin has %d skills, want the pack's 20: %v", len(skills), err)
	}
	for _, file := range []string{"agents/bex-assistant.md", "commands/deploy-to-bex.md", "commands/check-bex-status.md", "hooks/hooks.json", "README.md"} {
		if _, err := os.Stat(filepath.Join(plugin, file)); err != nil {
			t.Error(err)
		}
	}
	var mcp struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	readJSON(t, filepath.Join(plugin, ".mcp.json"), &mcp)
	server := mcp.MCPServers["bex"]
	if len(mcp.MCPServers) != 1 || server["type"] != "http" || server["url"] != "https://api.bex.co/mcp" {
		t.Errorf(".mcp.json = %v, want one server bex at https://api.bex.co/mcp", mcp.MCPServers)
	}
	if _, fixed := server["oauth"]; fixed {
		t.Errorf("server bex fixes an OAuth client (%v); bex clients register dynamically", server["oauth"])
	}
	var index marketplace
	readJSON(t, filepath.Join(root, MarketplaceFile), &index)
	if index.Name != "bex-plugins" || len(index.Plugins) != 1 || index.Plugins[0].Name != "bex" || index.Plugins[0].Source != "./"+PluginDir {
		t.Errorf("marketplace = %+v, want bex-plugins listing bex at ./%s", index, PluginDir)
	}

	var before pluginManifest
	readJSON(t, filepath.Join(plugin, ".claude-plugin", "plugin.json"), &before)
	pack := t.TempDir()
	if err := os.CopyFS(pack, os.DirFS("../pack")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pack, "skills", "bex-deploy", "SKILL.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := GeneratePlugin(pack, root); err != nil {
		t.Fatal(err)
	}
	var after pluginManifest
	readJSON(t, filepath.Join(plugin, ".claude-plugin", "plugin.json"), &after)
	if before.Name != "bex" || !strings.HasPrefix(before.Version, "1.0.0+") || after.Version == before.Version {
		t.Errorf("version %q → %q after a skill changed; want bex's 1.0.0+<digest>, changing with the contents", before.Version, after.Version)
	}
}

func readJSON(t *testing.T, file string, v any) {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
}

const hook = "plugin/scripts/validate-render-yaml-hook.sh"

// runHook runs the render.yaml hook with only the commands in bin on PATH.
func runHook(t *testing.T, bin, payload string) (stdout, stderr string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	script, err := filepath.Abs(hook)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, script)
	cmd.Dir = t.TempDir()
	cmd.Env = []string{"PATH=" + bin}
	cmd.Stdin = strings.NewReader(payload)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("hook exited %v (stderr %q); it must always exit 0", err, errOut.String())
	}
	if entries, _ := os.ReadDir(cmd.Dir); len(entries) != 0 {
		t.Errorf("hook created %v in its working directory", entries)
	}
	return out.String(), errOut.String()
}

// jqOnly is a PATH holding nothing but a jq that logs each run to log.
func jqOnly(t *testing.T) (bin, log string) {
	t.Helper()
	jq, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("no jq")
	}
	bin = t.TempDir()
	log = filepath.Join(t.TempDir(), "runs")
	shim := "#!/bin/sh\necho jq >> '" + log + "'\nexec '" + jq + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "jq"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, log
}

// The hook only suggests `bex blueprints validate` for a Blueprint, as
// Render's suggests `render blueprints validate` (w4/m631 t002). It runs
// nothing but jq: with nothing else on PATH, any other command would fail.
func TestRenderYAMLHookSuggestsValidation(t *testing.T) {
	bin, log := jqOnly(t)
	for _, tc := range []struct{ payload, path, quoted string }{
		{`{"tool_name":"Edit","tool_input":{"file_path":"/repo/render.yaml"}}`, "/repo/render.yaml", "'/repo/render.yaml'"},
		{`{"tool_name":"Write","tool_input":{"file_path":"/repo/svc/render.yml"}}`, "/repo/svc/render.yml", "'/repo/svc/render.yml'"},
		{`{"tool_name":"MultiEdit","tool_input":{"file_path":"bex.yml"}}`, "bex.yml", "'bex.yml'"},
		{`{"tool_input":{"filePath":"C:\\repo\\render.yaml"}}`, `C:\repo\render.yaml`, `'C:\repo\render.yaml'`},
		// Odd characters stay data: the suggestion quotes the path for the
		// shell, and the hook itself never evaluates it.
		{`{"tool_input":{"file_path":"/a b/$(touch pwned)/it's \"x\"\n/render.yaml"}}`,
			"/a b/$(touch pwned)/it's \"x\"\n/render.yaml", `'/a b/$(touch pwned)/it'\''s "x"` + "\n" + `/render.yaml'`},
	} {
		stdout, stderr := runHook(t, bin, tc.payload)
		var out struct {
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal([]byte(stdout), &out); err != nil {
			t.Errorf("%s: output %q is not JSON: %v (stderr %q)", tc.payload, stdout, err, stderr)
			continue
		}
		ctx := out.HookSpecificOutput.AdditionalContext
		if out.HookSpecificOutput.HookEventName != "PostToolUse" ||
			!strings.HasPrefix(ctx, "[bex plugin] "+tc.path+" changed.") ||
			!strings.Contains(ctx, "`bex blueprints validate "+tc.quoted+"`") ||
			!strings.Contains(ctx, "https://bex.co/docs/cli#install-bex") {
			t.Errorf("%s: context %q", tc.payload, ctx)
		}
		if stderr != "" {
			t.Errorf("%s: stderr %q", tc.payload, stderr)
		}
	}
	runs, err := os.ReadFile(log)
	if err != nil || strings.Count(string(runs), "jq\n") != 5 {
		t.Errorf("jq ran %q, want once per edit: %v", runs, err)
	}
}

func TestRenderYAMLHookIgnoresOtherInput(t *testing.T) {
	bin, _ := jqOnly(t)
	for _, payload := range []string{
		`{"tool_input":{"file_path":"/repo/main.go"}}`,
		`{"tool_input":{"file_path":"/repo/render.yaml.bak"}}`,
		`{"tool_input":{"file_path":"/repo/myrender.yaml"}}`,
		`{"tool_input":{"file_path":"/repo/render.yaml/notes.md"}}`,
		`{"tool_input":{"file_path":"/repo/Render.yaml"}}`,
		// bex.yaml is no Blueprint name; deploy/gitops/base/bex.yaml is a
		// platform manifest (ADR049 D1).
		`{"tool_input":{"file_path":"/repo/deploy/gitops/base/bex.yaml"}}`,
		`{"tool_input":{"file_path":42}}`,
		`{"tool_input":"render.yaml"}`,
		`{"tool_name":"Edit"}`,
		`[{"tool_input":{"file_path":"render.yaml"}}]`,
		`not json`,
		``,
	} {
		if stdout, stderr := runHook(t, bin, payload); stdout != "" || stderr != "" {
			t.Errorf("%q: stdout %q, stderr %q; want nothing", payload, stdout, stderr)
		}
	}
}

// Without jq the hook stays silent and never blocks the edit.
func TestRenderYAMLHookWithoutJQ(t *testing.T) {
	stdout, stderr := runHook(t, t.TempDir(), `{"tool_input":{"file_path":"/repo/render.yaml"}}`)
	if stdout != "" || stderr != "" {
		t.Errorf("stdout %q, stderr %q; want nothing", stdout, stderr)
	}
}
