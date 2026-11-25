#!/usr/bin/env bash
# Claude Code PostToolUse hook of the bex plugin: after an Edit, Write or
# MultiEdit of a Blueprint (render.yaml, render.yml, or bex's legacy bex.yml),
# ask Claude to validate it with `bex blueprints validate`.
#
# Adapted from Render's scripts/validate-render-yaml-hook.sh
# (render-oss/render-plugin-claude-code, MIT). Like Render's since 7b6aba9, it
# never runs the CLI: validation uses the user's bex login, so it goes through
# the user's normal Bash approval. It runs nothing but jq, never reads the
# edited file, and quotes the path in the command it suggests. With no jq, an
# unreadable payload or any other file it prints nothing and exits 0, so it
# never blocks an edit.

jq -c --arg install 'https://bex.co/docs/cli#install-bex' '
  (try (.tool_input | .file_path // .filePath) catch null)
  | strings
  | select((split("/") | last | split("\\") | last) as $name | ["render.yaml", "render.yml", "bex.yml"] | index($name))
  | {hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext:
      "[bex plugin] \(.) changed. Validate it by running `bex blueprints validate \(@sh)` and fix any errors it reports. If the bex CLI is not installed, see \($install)."}}
' 2>/dev/null
exit 0
