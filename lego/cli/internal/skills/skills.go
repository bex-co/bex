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

// Package skills is the launcher's `bex skills` command (w8/m143). Upstream's
// command clones render-oss/skills, whose skills operate Render, and keeps
// its state in ~/.render/skills.yaml. This one installs the embedded bex pack
// (internal/skills/pack, derived by packgen) under `bex-*` names, with state
// in the bex config directory. It never selects a `render-*` directory or
// touches Render's state, so a user's genuine Render skills keep working.
package skills

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/render-oss/cli/pkg/command"
	upstream "github.com/render-oss/cli/pkg/skills"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

//go:embed all:pack
var pack embed.FS

// prefix names every skill in the pack; it keeps bex's skills apart from
// Render's `render-*` ones in a shared tool directory.
const prefix = "bex-"

// stateFile lives in the bex config directory. Its name differs from
// Render's skills.yaml so that even a config directory pointed at ~/.render
// never shares state with the Render CLI.
const stateFile = "bex-skills.yaml"

// Command replaces upstream's `skills` command tree, with its subcommands
// and flags (the Render CLI's surface) intact.
func Command() *cobra.Command {
	root := &cobra.Command{
		Use:   "skills",
		Short: "Manage bex agent skills for AI coding tools",
		Long: "Install and manage bex agent skills for AI coding tools such as Claude Code, Codex, OpenCode, and Cursor. " +
			"Skills teach your AI coding assistant to deploy, debug, and monitor apps on bex.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.AddCommand(installCommand(), listCommand(), updateCommand(), removeCommand())
	return root
}

func installCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install bex skills to AI coding tools",
		Long: `Install bex agent skills, bundled with this CLI, to detected AI coding tools.

Skills can be installed at two scopes:
  - user:    Install to ~/.{tool}/skills/ (default, current user only)
  - project: Install to ./.{tool}/skills/ (committed to Git, all collaborators)

In a terminal, a prompt lets you pick scope, tools, and skills. Use --scope, --tool, and --skill flags to skip the prompts (useful for CI); without a terminal, every bex skill is installed to every detected tool at user scope.`,
		Example: `  # Install every bex skill for every detected tool
  bex skills install

  # Install for a specific tool and scope
  bex skills install --tool cursor --scope project

  # Preview install changes
  bex skills install --dry-run`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}
	tool := cmd.Flags().String("tool", "", "Install skills to a specific tool only (claude, codex, opencode, or cursor)")
	names := cmd.Flags().StringSlice("skill", nil, "Install specific skills only (use --skill multiple times)")
	dryRun := cmd.Flags().Bool("dry-run", false, "Show what would be installed without making changes")
	scopeFlag := cmd.Flags().String("scope", "", "Set installation scope to user or project (defaults to user)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if interactive(cmd) && *scopeFlag == "" {
			if err := pickScope(scopeFlag); err != nil {
				return err
			}
		}
		target, err := resolveTarget(*scopeFlag)
		if err != nil {
			return err
		}
		tools, err := detectTools(*tool)
		if err != nil {
			return err
		}
		if interactive(cmd) && *tool == "" {
			if tools, err = pickTools(tools); err != nil {
				return err
			}
		}
		if interactive(cmd) && len(*names) == 0 {
			if *names, err = pickPackSkills(); err != nil {
				return err
			}
		}
		result, err := install(target, tools, *names, *dryRun)
		if err != nil {
			return err
		}
		return emit(cmd, result, func(r *upstream.InstallResult) string {
			action := "Installed"
			if r.DryRun {
				action = "Would install"
			}
			return fmt.Sprintf("%s %d skill(s) to %d tool(s)", action, len(r.Skills), len(r.Tools))
		})
	}
	return cmd
}

func listCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed bex skills and detected tools",
		Long: `List installed bex skills and the AI tools they've been installed in. This reads from local state only, so the command doesn't require network access.

Use --scope to filter by installation scope (user or project).`,
		Example: `  # List all installed skills
  bex skills list

  # List project-scoped skills only
  bex skills list --scope project`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}
	scopeFlag := cmd.Flags().String("scope", "", "Filter skills by installation scope (user or project)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var scope upstream.Scope
		if *scopeFlag != "" {
			var err error
			if scope, err = upstream.ParseScope(*scopeFlag); err != nil {
				return err
			}
		}
		state, err := loadState()
		if err != nil {
			return err
		}
		listed := &upstream.SkillsState{Tools: state.Tools, InstalledAt: state.InstalledAt}
		for _, s := range state.Skills {
			if scope == "" || s.EffectiveScope() == scope {
				listed.Skills = append(listed.Skills, s)
			}
		}
		return emit(cmd, listed, func(s *upstream.SkillsState) string {
			if len(s.Skills) == 0 {
				return "No bex skills installed. Run `bex skills install`."
			}
			var b strings.Builder
			for _, sk := range s.Skills {
				fmt.Fprintf(&b, "%s %s (%s)\n", sk.Name, sk.Version, sk.EffectiveScope())
			}
			fmt.Fprintf(&b, "Tools: %s", strings.Join(s.Tools, ", "))
			return b.String()
		})
	}
	return cmd
}

func updateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update previously installed bex skills",
		Long: `Reinstall bex skills using the tool and skill selections saved by a previous "bex skills install" run.

This compares each installed skill with the version bundled with this CLI and updates any that have changed.

Use --scope to update skills at a specific scope (user or project).`,
		Example: `  # Update installed skills
  bex skills update

  # Force reinstall all skills
  bex skills update --force

  # Update project-scoped skills
  bex skills update --scope project`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}
	force := cmd.Flags().Bool("force", false, "Reinstall all skills even if already up to date")
	scopeFlag := cmd.Flags().String("scope", "", "Update skills at the specified scope (user or project)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		target, err := resolveTarget(*scopeFlag)
		if err != nil {
			return err
		}
		result, err := update(target, *force)
		if err != nil {
			return err
		}
		return emit(cmd, result, func(r *upstream.InstallResult) string {
			return fmt.Sprintf("Updated %d skill(s) in %d tool(s)", len(r.Skills), len(r.Tools))
		})
	}
	return cmd
}

func removeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove installed bex skills from AI coding tools",
		Long: `Remove installed bex skills from AI coding tools. Skills from other sources, including Render's render-* skills, are never removed.

Use --skill to remove specific skills, or --all to remove every bex skill at the selected scope.`,
		Example: `  # Remove one skill
  bex skills remove --skill bex-deploy

  # Remove all bex skills from project scope
  bex skills remove --all --scope project`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}
	names := cmd.Flags().StringSlice("skill", nil, "Remove specific skills only (use --skill multiple times)")
	tool := cmd.Flags().String("tool", "", "Remove skills from a specific tool only (claude, codex, opencode, or cursor)")
	all := cmd.Flags().Bool("all", false, "Remove all installed bex skills")
	scopeFlag := cmd.Flags().String("scope", "", "Remove skills from the specified scope (user or project)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		target, err := resolveTarget(*scopeFlag)
		if err != nil {
			return err
		}
		if !*all && len(*names) == 0 {
			if !interactive(cmd) {
				return errors.New("specify --skill or --all")
			}
			if *names, err = pickInstalledSkills(target); err != nil {
				return err
			}
		}
		tools, err := detectTools(*tool)
		if err != nil {
			return err
		}
		result, err := remove(target, tools, *names, *all)
		if err != nil {
			return err
		}
		return emit(cmd, result, func(r *upstream.RemoveResult) string {
			return fmt.Sprintf("Removed %d skill(s) from %d tool(s)", len(r.RemovedSkills), len(r.Tools))
		})
	}
	return cmd
}

// interactive is a terminal with no --output: Render's skills commands
// prompt for any choice their flags leave open there.
func interactive(cmd *cobra.Command) bool {
	format := command.GetFormatFromContext(cmd.Context())
	return format != nil && *format == command.Interactive
}

// errNothingPicked ends a picker the user left empty.
var errNothingPicked = errors.New("nothing selected")

func pickScope(scope *string) error {
	options := []huh.Option[string]{huh.NewOption("User (~/.{tool}/skills, only you)", string(upstream.ScopeUser))}
	if _, err := upstream.GetRepoRoot(); err == nil {
		options = append(options, huh.NewOption("Project (./.{tool}/skills, committed for collaborators)", string(upstream.ScopeProject)))
	}
	*scope = string(upstream.ScopeUser)
	if len(options) == 1 {
		return nil
	}
	return huh.NewSelect[string]().Title("Install scope").Options(options...).Value(scope).Run()
}

func pickTools(tools []upstream.Tool) ([]upstream.Tool, error) {
	if len(tools) < 2 {
		return tools, nil
	}
	names := upstream.ToolNames(tools)
	chosen := slices.Clone(names)
	options := huh.NewOptions(names...)
	if err := huh.NewMultiSelect[string]().Title("Install to which tools?").Options(options...).Value(&chosen).Run(); err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		return nil, errNothingPicked
	}
	return upstream.FilterToolsByNames(tools, chosen), nil
}

func pickPackSkills() ([]string, error) {
	src, cleanup, err := extractPack()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	var options []huh.Option[string]
	var chosen []string
	for _, s := range upstream.ReadSkillsFromRepo(src) {
		options = append(options, huh.NewOption(s.DirName, s.DirName))
		chosen = append(chosen, s.DirName)
	}
	return pickMany("Install which skills?", options, chosen)
}

func pickInstalledSkills(t target) ([]string, error) {
	state, err := loadState()
	if err != nil {
		return nil, err
	}
	var options []huh.Option[string]
	for _, s := range state.Skills {
		if s.EffectiveScope() == t.scope {
			options = append(options, huh.NewOption(s.EffectiveDirName(), s.EffectiveDirName()))
		}
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("no bex skills installed at %s scope", t.scope)
	}
	return pickMany("Remove which skills?", options, nil)
}

func pickMany(title string, options []huh.Option[string], chosen []string) ([]string, error) {
	if err := huh.NewMultiSelect[string]().Title(title).Options(options...).Value(&chosen).Run(); err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		return nil, errNothingPicked
	}
	return chosen, nil
}

// emit writes data in the selected output format. Interactive (a terminal
// with no --output) prints the text form: these commands have no TUI.
func emit[T any](cmd *cobra.Command, data T, text func(T) string) error {
	if format := command.GetFormatFromContext(cmd.Context()); format == nil || *format == command.Interactive {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), text(data))
		return err
	}
	_, err := command.PrintData(cmd, data, text)
	return err
}

// target is an installation scope and, for project scope, its repository.
type target struct {
	scope    upstream.Scope
	repoRoot string
}

func resolveTarget(flag string) (target, error) {
	t := target{scope: upstream.ScopeUser}
	if flag != "" {
		var err error
		if t.scope, err = upstream.ParseScope(flag); err != nil {
			return target{}, err
		}
	}
	if t.scope == upstream.ScopeProject {
		root, err := upstream.GetRepoRoot()
		if err != nil {
			return target{}, fmt.Errorf("project scope requires a git repository: %w", err)
		}
		t.repoRoot = root
	}
	return t, nil
}

func (t target) dir(tool upstream.Tool) string {
	return upstream.GetScopedSkillsDir(tool, t.scope, t.repoRoot)
}

func detectTools(filter string) ([]upstream.Tool, error) {
	tools, err := upstream.DetectTools()
	if err != nil {
		return nil, fmt.Errorf("failed to detect tools: %w", err)
	}
	if len(tools) == 0 {
		return nil, errors.New("no supported AI coding tools detected")
	}
	if filter == "" {
		return tools, nil
	}
	if tools = upstream.FilterTools(tools, filter); len(tools) == 0 {
		return nil, fmt.Errorf("no installed tool matching %q found", filter)
	}
	return tools, nil
}

// extractPack writes the embedded pack to a temporary directory laid out
// like a render-oss/skills checkout, which upstream's installer reads.
func extractPack() (string, func(), error) {
	dir, err := os.MkdirTemp("", "bex-skills-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	sub, err := fs.Sub(pack, "pack")
	if err == nil {
		err = os.CopyFS(dir, sub)
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

// resolveNames maps --skill values (a name or a directory name) to pack
// directory names. With no filter it selects the whole pack: upstream's
// installer treats an empty selection as "replace every render-* skill", so
// a selection is always explicit.
func resolveNames(available []upstream.SkillInfo, filter []string) ([]string, error) {
	if len(filter) == 0 {
		names := make([]string, 0, len(available))
		for _, s := range available {
			names = append(names, s.DirName)
		}
		return names, nil
	}
	resolved, unmatched := upstream.ResolveSkillNames(available, filter)
	if len(unmatched) > 0 {
		return nil, fmt.Errorf("no bex skill named %s", strings.Join(unmatched, ", "))
	}
	return resolved, nil
}

func install(t target, tools []upstream.Tool, filter []string, dryRun bool) (*upstream.InstallResult, error) {
	src, cleanup, err := extractPack()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	available := upstream.ReadSkillsFromRepo(src)
	selected, err := resolveNames(available, filter)
	if err != nil {
		return nil, err
	}
	result := &upstream.InstallResult{Tools: tools, DryRun: dryRun}
	for _, s := range available {
		if slices.Contains(selected, s.DirName) {
			result.Skills = append(result.Skills, s)
		}
	}
	if dryRun {
		return result, nil
	}
	state, err := loadState()
	if err != nil {
		return nil, err
	}
	if result.Tools, err = copyTo(t, tools, src, selected); err != nil {
		return nil, err
	}
	return result, record(state, t, result.Tools, result.Skills, src)
}

// copyTo replaces the selected skills in every tool and returns the tools it
// reached. Upstream's installer clears only render-* directories, so each
// bex-* directory is cleared here first: a file dropped from the pack must
// not outlive its update.
func copyTo(t target, tools []upstream.Tool, src string, selected []string) ([]upstream.Tool, error) {
	if err := guardSelection(selected); err != nil {
		return nil, err
	}
	var reached []upstream.Tool
	var failures []string
	for _, tool := range tools {
		dir := t.dir(tool)
		err := os.MkdirAll(dir, 0o755)
		for _, name := range selected {
			if err == nil {
				err = os.RemoveAll(filepath.Join(dir, name))
			}
		}
		if err == nil {
			_, err = upstream.InstallSelectedSkills(dir, src, selected)
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", tool.Name, err))
			continue
		}
		reached = append(reached, tool)
	}
	if len(reached) == 0 {
		return nil, fmt.Errorf("failed to install skills to any tool: %s", strings.Join(failures, "; "))
	}
	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "warning: some tools were skipped: %s\n", strings.Join(failures, "; "))
	}
	return reached, nil
}

// guardSelection holds the invariant that keeps Render's skills safe:
// upstream deletes every render-* directory for an empty selection, and the
// named ones otherwise.
func guardSelection(selected []string) error {
	if len(selected) == 0 {
		return errors.New("no bex skills selected")
	}
	for _, name := range selected {
		if !strings.HasPrefix(name, prefix) {
			return fmt.Errorf("refusing to replace non-bex skill %q", name)
		}
	}
	return nil
}

// record saves installed skills (with their pack hashes) and the tools they
// reached into state.
func record(state *upstream.SkillsState, t target, tools []upstream.Tool, installed []upstream.SkillInfo, src string) error {
	for _, tool := range tools {
		if !slices.Contains(state.Tools, tool.Name) {
			state.Tools = append(state.Tools, tool.Name)
		}
	}
	for _, s := range installed {
		hash, err := upstream.HashSkillDir(filepath.Join(src, "skills", s.DirName))
		if err != nil {
			return err
		}
		state.Skills = slices.DeleteFunc(state.Skills, func(old upstream.InstalledSkill) bool {
			return old.EffectiveDirName() == s.DirName && old.EffectiveScope() == t.scope
		})
		state.Skills = append(state.Skills, s.ToInstalledWithScope(hash, t.scope))
	}
	state.Touch()
	return saveState(state)
}

func update(t target, force bool) (*upstream.InstallResult, error) {
	state, err := loadState()
	if err != nil {
		return nil, err
	}
	detected, err := upstream.DetectTools()
	if err != nil {
		return nil, fmt.Errorf("failed to detect tools: %w", err)
	}
	result := &upstream.InstallResult{Tools: upstream.IntersectToolsByState(detected, state)}
	installed := map[string]string{}
	for _, s := range state.Skills {
		if s.EffectiveScope() == t.scope {
			installed[s.EffectiveDirName()] = s.Hash
		}
	}
	if len(installed) == 0 || len(result.Tools) == 0 {
		return result, nil
	}
	src, cleanup, err := extractPack()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	var outdated []upstream.SkillInfo
	var names []string
	for _, s := range upstream.ReadSkillsFromRepo(src) {
		old, ok := installed[s.DirName]
		if !ok {
			continue
		}
		hash, err := upstream.HashSkillDir(filepath.Join(src, "skills", s.DirName))
		if err != nil {
			return nil, err
		}
		if force || old != hash {
			outdated = append(outdated, s)
			names = append(names, s.DirName)
		}
	}
	if len(outdated) == 0 {
		return result, nil
	}
	if result.Tools, err = copyTo(t, result.Tools, src, names); err != nil {
		return nil, err
	}
	result.Skills = outdated
	return result, record(state, t, result.Tools, outdated, src)
}

// remove deletes bex skills from the given tools. A skill leaves state only
// when it left every tool state records: a --tool removal, or one a tool
// failed, leaves it installed somewhere that update must still reach.
func remove(t target, tools []upstream.Tool, filter []string, all bool) (*upstream.RemoveResult, error) {
	state, err := loadState()
	if err != nil {
		return nil, err
	}
	var installed []upstream.InstalledSkill
	for _, s := range state.Skills {
		if s.EffectiveScope() == t.scope {
			installed = append(installed, s)
		}
	}
	var names []string
	if all {
		for _, s := range installed {
			names = append(names, s.EffectiveDirName())
		}
	} else {
		var unmatched []string
		names, unmatched = upstream.ResolveInstalledSkillNames(installed, filter)
		if len(unmatched) > 0 {
			return nil, fmt.Errorf("bex skill %s is not installed at %s scope", strings.Join(unmatched, ", "), t.scope)
		}
	}
	result := &upstream.RemoveResult{Tools: tools, RemovedSkills: names}
	if len(names) == 0 {
		return result, nil
	}
	if err := guardSelection(names); err != nil {
		return nil, err
	}
	for _, tool := range tools {
		if err := upstream.RemoveSkills(t.dir(tool), names); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", tool.Name, err))
		}
	}
	everywhere := len(result.Errors) == 0
	for _, name := range state.Tools {
		everywhere = everywhere && slices.ContainsFunc(tools, func(tool upstream.Tool) bool { return tool.Name == name })
	}
	if everywhere {
		state.Skills = slices.DeleteFunc(state.Skills, func(s upstream.InstalledSkill) bool {
			return s.EffectiveScope() == t.scope && slices.Contains(names, s.EffectiveDirName())
		})
	}
	return result, saveState(state)
}

// statePath is in the bex config directory the launcher's bridge exports as
// RENDER_CLI_CONFIG_DIR (default ~/.bex).
func statePath() (string, error) {
	if dir := os.Getenv("RENDER_CLI_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, stateFile), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bex", stateFile), nil
}

func loadState() (*upstream.SkillsState, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	state := &upstream.SkillsState{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return state, nil
}

func saveState(state *upstream.SkillsState) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
