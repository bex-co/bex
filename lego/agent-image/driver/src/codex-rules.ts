/*
 * Copyright 2026.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Approval mode for codex (ADR100, w9/081). codex-acp 1.4.0 sends every turn
// with its own approval policy ("on-request" in the default `agent` mode),
// under which codex asks the client only for commands the model escalates
// out of its sandbox: an in-sandbox `sudo true` never reached the driver's
// gate. codex's exec policy closes that gap: a `prefix_rule` with decision
// "prompt" makes codex ask before running a matching command, whatever the
// turn's policy, and the ask arrives as an ordinary ACP permission request.
//
// The rules are deliberately broad, by program: the driver's gate
// (approval.ts) still decides each request, so a prompted `git status` is
// answered allow_once at once and only what the gate calls risky reaches a
// person. `risky` prompts the programs the gate's table inspects; `all`
// prompts every executable on PATH except a short read-only list, matching
// "ask before every action" (Claude Code likewise never asks for read-only
// commands). Autonomous sessions get no rules file at all.

import { readdir, rm, stat, mkdir, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import type { ApprovalMode } from "./approval.js";

export const codexRulesFileName = "bex-approval.rules";

// Every program the approval table's rules look at (approval.ts
// riskyCommands), plus the shells and wrappers a command can hide behind.
export const codexRiskyPrograms = [
  "sudo",
  "doas",
  "su",
  "env",
  "git",
  "rm",
  "npm",
  "pnpm",
  "yarn",
  "cargo",
  "twine",
  "gem",
  "poetry",
  "gh",
  "docker",
  "podman",
  "helm",
  "kubectl",
  "oc",
  "terraform",
  "tofu",
  "aws",
  "gcloud",
  "az",
  "curl",
  "wget",
  "base64",
  "xxd",
  "bex",
  "render",
  "bash",
  "sh",
  "zsh",
  "dash",
  "eval",
  "source",
] as const;

// Read-only programs an `all` session runs without asking, the way Claude
// Code's own read-only allowlist does for claude sessions.
export const codexReadOnlyPrograms = new Set([
  "cat",
  "date",
  "echo",
  "false",
  "file",
  "grep",
  "head",
  "id",
  "ls",
  "printf",
  "pwd",
  "rg",
  "stat",
  "tail",
  "true",
  "uname",
  "wc",
  "which",
  "whoami",
]);

// codexHome is where codex reads its config and `rules/` directory.
export function codexHome(env: Record<string, string | undefined>): string {
  if (env.CODEX_HOME) return env.CODEX_HOME;
  return path.join(env.HOME || homedir(), ".codex");
}

async function pathExecutables(searchPath: string): Promise<string[]> {
  const names = new Set<string>();
  for (const dir of searchPath.split(path.delimiter)) {
    if (!dir) continue;
    let entries: string[];
    try {
      entries = await readdir(dir);
    } catch {
      continue;
    }
    for (const name of entries) {
      try {
        const info = await stat(path.join(dir, name));
        if (info.isFile() && (info.mode & 0o111) !== 0) names.add(name);
      } catch {
        // A dangling link or a racing removal: not a program to gate.
      }
    }
  }
  return [...names];
}

// A program name codex's Starlark rules can carry without quoting trouble.
const safeName = /^[A-Za-z0-9][A-Za-z0-9._+-]*$/;

export async function codexPromptPrograms(
  mode: ApprovalMode,
  searchPath: string,
): Promise<string[]> {
  if (mode === "autonomous") return [];
  const programs = new Set<string>(codexRiskyPrograms);
  if (mode === "all")
    for (const name of await pathExecutables(searchPath))
      if (!codexReadOnlyPrograms.has(name)) programs.add(name);
  return [...programs].filter((name) => safeName.test(name)).sort();
}

// codexRulesText renders the prompt rules, a few hundred programs per rule.
export function codexRulesText(programs: string[]): string {
  const lines = [
    "# Written by the bex agent driver for approval mode (ADR100). codex asks",
    "# before every matching command; the driver's policy answers each ask.",
  ];
  for (let i = 0; i < programs.length; i += 200) {
    const batch = programs.slice(i, i + 200).map((p) => JSON.stringify(p));
    lines.push(
      `prefix_rule(pattern = [[${batch.join(", ")}]], decision = "prompt", justification = "bex approval mode")`,
    );
  }
  return `${lines.join("\n")}\n`;
}

// installCodexApprovalRules writes (or, for an autonomous session, removes)
// the driver's rules file before codex starts. The session's `.codex`
// directory persists across turns, so a stale file must not outlive a mode.
export async function installCodexApprovalRules(
  mode: ApprovalMode,
  env: Record<string, string | undefined>,
): Promise<string> {
  const file = path.join(codexHome(env), "rules", codexRulesFileName);
  const programs = await codexPromptPrograms(mode, env.PATH ?? "");
  if (programs.length === 0) {
    await rm(file, { force: true });
    return file;
  }
  await mkdir(path.dirname(file), { recursive: true });
  await writeFile(file, codexRulesText(programs), { mode: 0o644 });
  return file;
}
