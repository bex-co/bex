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

// Delivery is the deterministic completion step of a fire-and-forget agent turn
// (ADR047 D4): the driver — not the agent — stages the working tree, commits,
// and pushes the session's bex-agent/* branch, then extracts bounded evidence
// from the session log. bex-api's Completer reads the resulting status file and
// opens the draft PR. Keeping delivery in the driver makes it deterministic and
// enforced outside the agent (the Copilot model).

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { readFile } from "node:fs/promises";

const run = promisify(execFile);

const MAX_COMMANDS = 40;
const MAX_TEST_LINES = 40;
const MAX_OUTPUT_TAIL = 8000;
const MAX_LINE_LEN = 2000;
const MAX_WALK_DEPTH = 8;

async function git(cwd, args) {
  const { stdout } = await run("git", args, { cwd, maxBuffer: 32 * 1024 * 1024 });
  return stdout.trim();
}

async function isGitRepo(cwd) {
  try {
    await git(cwd, ["rev-parse", "--is-inside-work-tree"]);
    return true;
  } catch {
    return false;
  }
}

async function detectBaseBranch(cwd, fallback) {
  if (fallback) return fallback;
  try {
    const ref = await git(cwd, ["symbolic-ref", "--short", "refs/remotes/origin/HEAD"]);
    return ref.replace(/^origin\//, "");
  } catch {
    return "main";
  }
}

// ensureRepo makes the workspace a checkout of the session branch. A pre-cloned
// workspace is left alone; an empty one is cloned and switched to the branch
// (created from the default when it does not exist remotely). Clone/fetch use
// the system-wide bex Git credential helper.
export async function ensureRepo(config) {
  if (await isGitRepo(config.cwd)) return;
  if (!config.repoUrl) {
    throw new Error("workspace is not a git repository and BEX_AGENT_REPO_URL is unset");
  }
  await git(config.cwd, ["clone", config.repoUrl, "."]);
  const base = await detectBaseBranch(config.cwd, config.baseBranch);
  try {
    await git(config.cwd, ["checkout", "-B", config.branch, `origin/${config.branch}`]);
  } catch {
    await git(config.cwd, ["checkout", "-B", config.branch, base]);
  }
}

function commitMessage(config) {
  const first = String(config.prompt || "").split("\n")[0].trim();
  const subject = first ? first.slice(0, 72) : "agent session update";
  return `bex agent: ${subject}\n\nDelivered by bex cloud coding-agent session.`;
}

// deliverBranch stages, commits, and pushes the session branch, returning the
// delivery record the Completer consumes. A turn that produced no change pushes
// nothing (pushed:false) — an honest no-op, not an error.
export async function deliverBranch(config) {
  const cwd = config.cwd;
  const base = await detectBaseBranch(cwd, config.baseBranch);
  // Stay on the session branch without disturbing the agent's working-tree edits.
  await git(cwd, ["checkout", "-B", config.branch]);
  await git(cwd, ["add", "-A"]);
  const status = await git(cwd, ["status", "--porcelain"]);
  let changedFiles = [];
  if (status) {
    changedFiles = (await git(cwd, ["diff", "--cached", "--name-only"]))
      .split("\n")
      .filter(Boolean);
    await git(cwd, [
      "-c",
      `user.name=${config.gitName}`,
      "-c",
      `user.email=${config.gitEmail}`,
      "commit",
      "-m",
      commitMessage(config),
    ]);
  }
  let commits = 0;
  try {
    commits = Number(await git(cwd, ["rev-list", "--count", `${base}..${config.branch}`])) || 0;
  } catch {
    commits = 0;
  }
  let pushed = false;
  let headSha = "";
  if (commits > 0) {
    await git(cwd, ["push", "origin", `${config.branch}:${config.branch}`]);
    pushed = true;
    headSha = await git(cwd, ["rev-parse", "HEAD"]);
  }
  return { branch: config.branch, baseBranch: base, headSha, pushed, commits, changedFiles };
}

function pushLine(list, value, marker) {
  const text = String(value);
  if (text.length > MAX_LINE_LEN) {
    list.push(text.slice(0, MAX_LINE_LEN));
    marker.truncated = true;
  } else {
    list.push(text);
  }
}

// collectEvidence walks a logged UI-message/ACP part tolerantly: it does not
// assume a specific provider schema, only that commands live under
// command/commandLine (or a tool-call title) and terminal output under
// output/stdout/stderr — a forward-compatible, honest extraction.
function collectEvidence(node, acc, depth = 0) {
  if (node == null || depth > MAX_WALK_DEPTH || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const item of node) collectEvidence(item, acc, depth + 1);
    return;
  }
  for (const key of ["command", "commandLine"]) {
    if (typeof node[key] === "string" && node[key].trim()) acc.commands.push(node[key]);
  }
  if (node.sessionUpdate === "tool_call" && typeof node.title === "string" && !node.command) {
    acc.commands.push(node.title);
  }
  for (const key of ["output", "stdout", "stderr"]) {
    if (typeof node[key] === "string" && node[key].trim()) acc.tests.push(node[key]);
  }
  if (node.type === "text" && typeof node.text === "string") acc.text += node.text;
  for (const value of Object.values(node)) collectEvidence(value, acc, depth + 1);
}

// extractEvidence reads the redacted session log and returns the bounded,
// Codex-style evidence extract (command log + test-output tails + a trailing
// output window). Every cap that drops content sets truncated explicitly.
export async function extractEvidence(config) {
  let raw = "";
  try {
    raw = await readFile(config.sessionLogPath, "utf8");
  } catch {
    raw = "";
  }
  const acc = { commands: [], tests: [], text: "" };
  for (const line of raw.split("\n")) {
    if (!line.trim()) continue;
    let record;
    try {
      record = JSON.parse(line);
    } catch {
      continue;
    }
    collectEvidence(record.part ?? record, acc);
  }

  const marker = { truncated: false };
  const commandLog = [];
  for (const command of acc.commands.slice(0, MAX_COMMANDS)) pushLine(commandLog, command, marker);
  if (acc.commands.length > MAX_COMMANDS) marker.truncated = true;

  const testTail = acc.tests.slice(-MAX_TEST_LINES);
  const testOutput = [];
  for (const line of testTail) pushLine(testOutput, line, marker);
  if (acc.tests.length > MAX_TEST_LINES) marker.truncated = true;

  let outputTail = acc.text;
  if (outputTail.length > MAX_OUTPUT_TAIL) {
    outputTail = outputTail.slice(-MAX_OUTPUT_TAIL);
    marker.truncated = true;
  }

  return { commandLog, testOutput, outputTail, truncated: marker.truncated };
}
