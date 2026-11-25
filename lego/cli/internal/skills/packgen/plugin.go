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
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// bex's Claude Code plugin (w4/m631) is what Render's
// render-oss/render-plugin-claude-code is for Render: the skills, an agent,
// two slash commands, a render.yaml validation hook and the MCP server, set up
// by `/plugin marketplace add bex-co/bex` and `/plugin install bex`. Its
// skills are the pack, copied file for file; the rest is bex's adaptation of
// Render's MIT files, kept under plugin/.
const (
	// PluginDir is the plugin, relative to the repository root. A
	// subdirectory, never the root: the root's .mcp.json and .claude/skills
	// are the maintainers' own.
	PluginDir = "plugins/claude-code/bex"
	// MarketplaceFile is the repository's plugin marketplace.
	MarketplaceFile = ".claude-plugin/marketplace.json"
	// RenderPluginCommit is the render-oss/render-plugin-claude-code commit
	// (v0.4.1) the agent, commands, hook and MCP configuration follow.
	RenderPluginCommit = "2df0bc0"
)

//go:embed all:plugin
var pluginFiles embed.FS

// pluginManifest is the plugin's .claude-plugin/plugin.json. Its version is a
// digest of every other file in the plugin, so a change to any of them
// reaches users on their next update with no bump to remember, and an
// unchanged plugin never re-downloads.
type pluginManifest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Version          string   `json:"version"`
	Author           author   `json:"author"`
	Homepage         string   `json:"homepage"`
	Repository       string   `json:"repository"`
	License          string   `json:"license"`
	PrivacyPolicyURL string   `json:"privacyPolicyUrl"`
	Keywords         []string `json:"keywords"`
}

type author struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type marketplace struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Owner       author            `json:"owner"`
	Plugins     []marketplaceItem `json:"plugins"`
}

type marketplaceItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      author `json:"author"`
	Source      string `json:"source"`
	Category    string `json:"category"`
}

var bexAuthor = author{Name: "bex", URL: "https://bex.co"}

const pluginDescription = "Deploy, debug, and monitor applications on bex"

// GeneratePlugin writes the Claude Code plugin and the marketplace that lists
// it under the repository root, from the pack at pack (the directory Generate
// writes). It replaces PluginDir whole.
func GeneratePlugin(pack, root string) error {
	files := map[string][]byte{}
	own, err := fs.Sub(pluginFiles, "plugin")
	if err != nil {
		return err
	}
	if err := addFiles(files, own, "."); err != nil {
		return err
	}
	if err := addFiles(files, os.DirFS(pack), "skills"); err != nil {
		return err
	}
	license, err := os.ReadFile(filepath.Join(pack, "LICENSE"))
	if err != nil {
		return err
	}
	files["LICENSE"] = append([]byte("bex's Claude Code plugin. Its agent, commands, hook and MCP configuration are adapted from "+
		"https://github.com/render-oss/render-plugin-claude-code at commit "+RenderPluginCommit+
		" (MIT License, Copyright (c) 2026 Render, the license reproduced below). Its skills are bex's skills pack:\n\n"), license...)

	manifest, err := marshal(pluginManifest{
		Name: "bex",
		Description: pluginDescription + ". Includes skills, an agent, slash commands, a render.yaml validation hook, " +
			"and the bex MCP server preconfigured over OAuth.",
		Version:          "1.0.0+" + digest(files),
		Author:           bexAuthor,
		Homepage:         "https://github.com/bex-co/bex/tree/main/" + PluginDir,
		Repository:       "https://github.com/bex-co/bex",
		License:          "MIT",
		PrivacyPolicyURL: "https://bex.co/privacy",
		Keywords:         []string{"bex", "render", "deploy", "cloud", "blueprint", "monitoring", "debugging"},
	})
	if err != nil {
		return err
	}
	files[".claude-plugin/plugin.json"] = manifest

	dst := filepath.Join(root, filepath.FromSlash(PluginDir))
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	for rel, data := range files {
		if err := writeFile(filepath.Join(dst, filepath.FromSlash(rel)), data); err != nil {
			return err
		}
	}
	index, err := marshal(marketplace{
		Name:        "bex-plugins",
		Description: "bex plugins for Claude Code",
		Owner:       bexAuthor,
		Plugins: []marketplaceItem{{
			Name:        "bex",
			Description: pluginDescription,
			Author:      bexAuthor,
			Source:      "./" + PluginDir,
			Category:    "developer-tools",
		}},
	})
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(root, filepath.FromSlash(MarketplaceFile)), index)
}

// addFiles reads every file under dir of fsys into files, keyed by its path.
func addFiles(files map[string][]byte, fsys fs.FS, dir string) error {
	return fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, p)
		files[p] = data
		return err
	})
}

// digest is a short hash of the plugin's files, names and contents.
func digest(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name + "\x00"))
		h.Write(files[name])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	return append(data, '\n'), err
}

// writeFile writes one plugin file; the hook's scripts are executable, as
// Claude Code runs them by path.
func writeFile(target string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if strings.HasSuffix(target, ".sh") {
		mode = 0o755
	}
	return os.WriteFile(target, data, mode)
}
