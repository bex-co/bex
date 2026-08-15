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
import { readFile, readdir, rm } from "node:fs/promises";

const run = promisify(execFile);

const MAX_COMMANDS = 40;
const MAX_TEST_LINES = 40;
const MAX_OUTPUT_TAIL = 8000;
const MAX_LINE_LEN = 2000;
const MAX_WALK_DEPTH = 8;

export interface DeliveryConfig {
  cwd: string;
  branch: string;
  baseBranch: string;
  prompt: string;
  gitName: string;
  gitEmail: string;
  // containsSecret, when set, fails the push closed if any commit on the branch
  // still carries the model credential (round-5 finding 6). Optional so direct
  // callers/tests that never handle a credential are unaffected.
  containsSecret?: (text: string) => boolean;
}

export interface DeliveryResult {
  branch: string;
  baseBranch: string;
  headSha: string;
  pushed: boolean;
  commits: number;
  changedFiles: string[];
}

export interface EvidenceResult {
  commandLog: string[];
  testOutput: string[];
  outputTail: string;
  truncated: boolean;
}

interface EvidenceAccumulator {
  commands: string[];
  tests: string[];
  text: string;
}

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await run("git", args, { cwd, maxBuffer: 32 * 1024 * 1024 });
  return stdout.trim();
}

async function isGitRepo(cwd: string): Promise<boolean> {
  try {
    await git(cwd, ["rev-parse", "--is-inside-work-tree"]);
    return true;
  } catch {
    return false;
  }
}

async function detectBaseBranch(cwd: string, fallback: string): Promise<string> {
  if (fallback) return fallback;
  try {
    const ref = await git(cwd, ["symbolic-ref", "--short", "refs/remotes/origin/HEAD"]);
    return ref.replace(/^origin\//, "");
  } catch {
    return "main";
  }
}

// restoreWorkspace hydrates the workspace from a hibernation snapshot (ADR059
// D4) before the setup clone: it streams the presigned GET URL through tar,
// restoring `/workspace` (+ `~/.zed_server`) with the working tree — uncommitted
// edits and installed dependencies — intact, so ensureRepo then leaves the
// pre-populated repo alone. The URL is passed via env (never argv/ps) so it is
// not visible in process listings; it carries no durable credential. A restore
// failure is fatal to setup on purpose: silently falling back to a clean clone
// would lose the user's uncommitted work without a signal.
export async function restoreWorkspace(
  config: { restoreUrl: string },
  destRoot = "/",
): Promise<void> {
  if (!config.restoreUrl) return;
  await run(
    "/bin/sh",
    ["-c", 'curl -sf "$BEX_RESTORE_URL" | tar xzf - -C "$BEX_RESTORE_DEST"'],
    {
      env: {
        ...process.env,
        BEX_RESTORE_URL: config.restoreUrl,
        BEX_RESTORE_DEST: destRoot,
      },
    },
  );
}

// cloneWithRetry clones the session repo, retrying a transient authorization
// failure from the gateway credential broker. bex-api's dispatch records the
// session's sandbox_id AFTER the sandbox is already running (agentsessions
// Service.dispatch order: CreateAgentSessionSandbox -> EnterAgentSessionPhase ->
// RecordAgentSessionDispatch), so the driver's first clone can race ahead of that
// write and the broker's live-sandbox mint check answers 403 ("remote: forbidden")
// until the id is persisted a few seconds later. Retrying with backoff turns that
// startup race into a brief delay instead of a failed session; each retry cleans
// any partial clone so `git clone .` sees an empty directory.
async function cloneWithRetry(cwd: string, repoUrl: string): Promise<void> {
  const attempts = 10;
  let lastError: unknown;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      await git(cwd, ["clone", repoUrl, "."]);
      return;
    } catch (error) {
      lastError = error;
      if (attempt === attempts) break;
      await cleanDirectory(cwd);
      await new Promise((resolve) => setTimeout(resolve, 3000));
    }
  }
  throw lastError;
}

// cleanDirectory empties a directory (removing a failed clone's partial state)
// without removing the directory itself, so the next `git clone .` sees it empty.
async function cleanDirectory(cwd: string): Promise<void> {
  const entries = await readdir(cwd);
  await Promise.all(
    entries.map((entry) => rm(`${cwd}/${entry}`, { recursive: true, force: true })),
  );
}

// ensureRepo makes the workspace a checkout of the session branch. A pre-cloned
// workspace is left alone; an empty one is cloned and switched to the branch
// (created from the default when it does not exist remotely). The remote is the
// Pod-bound gateway smart-HTTP proxy; no GitHub credential enters this process.
export async function ensureRepo(
  config: DeliveryConfig & { repoUrl: string },
): Promise<void> {
  if (await isGitRepo(config.cwd)) return;
  if (!config.repoUrl) {
    throw new Error("workspace is not a git repository and BEX_AGENT_REPO_URL is unset");
  }
  await cloneWithRetry(config.cwd, config.repoUrl);
  const base = await detectBaseBranch(config.cwd, config.baseBranch);
  try {
    await git(config.cwd, ["checkout", "-B", config.branch, `origin/${config.branch}`]);
  } catch {
    await git(config.cwd, ["checkout", "-B", config.branch, base]);
  }
}

function commitMessage(config: DeliveryConfig): string {
  const first = String(config.prompt || "").split("\n")[0].trim();
  const subject = first ? first.slice(0, 72) : "agent session update";
  return `bex agent: ${subject}\n\nDelivered by bex cloud coding-agent session.`;
}

// deliverBranch stages, commits, and pushes the session branch, returning the
// delivery record the Completer consumes. A turn that produced no change pushes
// nothing (pushed:false) — an honest no-op, not an error.
export async function deliverBranch(config: DeliveryConfig): Promise<DeliveryResult> {
  const cwd = config.cwd;
  const base = await detectBaseBranch(cwd, config.baseBranch);
  // Stay on the session branch without disturbing the agent's working-tree edits.
  await git(cwd, ["checkout", "-B", config.branch]);
  await git(cwd, ["add", "-A"]);
  const status = await git(cwd, ["status", "--porcelain"]);
  let changedFiles: string[] = [];
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
    // round-5 finding 6: fail closed if any commit on the branch still contains
    // the model credential. The working tree is byte-scrubbed before delivery,
    // but that cannot reach a credential the agent itself already committed (it
    // lives in a compressed git object), and `git push` would publish it to the
    // connected repository, its collaborators, CI, and every clone. Scan the
    // branch's full patch and refuse to push on a match (never on argv — the
    // needle stays inside containsSecret).
    if (config.containsSecret) {
      const patch = await git(cwd, ["log", "-p", "--no-color", `${base}..${config.branch}`]);
      if (config.containsSecret(patch)) {
        throw new Error("refusing to push: model credential found in commit history");
      }
    }
    await git(cwd, ["push", "origin", `${config.branch}:${config.branch}`]);
    pushed = true;
    headSha = await git(cwd, ["rev-parse", "HEAD"]);
  }
  return { branch: config.branch, baseBranch: base, headSha, pushed, commits, changedFiles };
}

function pushLine(list: string[], value: unknown, marker: { truncated: boolean }): void {
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
function collectEvidence(node: unknown, acc: EvidenceAccumulator, depth = 0): void {
  if (node == null || depth > MAX_WALK_DEPTH || typeof node !== "object") return;
  if (Array.isArray(node)) {
    for (const item of node) collectEvidence(item, acc, depth + 1);
    return;
  }
  const record = node as Record<string, unknown>;
  for (const key of ["command", "commandLine"]) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) acc.commands.push(value);
  }
  if (
    record.sessionUpdate === "tool_call" &&
    typeof record.title === "string" &&
    !record.command
  ) {
    acc.commands.push(record.title);
  }
  for (const key of ["output", "stdout", "stderr"]) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) acc.tests.push(value);
  }
  if (record.type === "text" && typeof record.text === "string") acc.text += record.text;
  for (const value of Object.values(record)) collectEvidence(value, acc, depth + 1);
}

// extractEvidence reads the redacted session log and returns the bounded,
// Codex-style evidence extract (command log + test-output tails + a trailing
// output window). Every cap that drops content sets truncated explicitly.
export async function extractEvidence(config: {
  sessionLogPath: string;
}): Promise<EvidenceResult> {
  let raw = "";
  try {
    raw = await readFile(config.sessionLogPath, "utf8");
  } catch {
    raw = "";
  }
  const acc: EvidenceAccumulator = { commands: [], tests: [], text: "" };
  for (const line of raw.split("\n")) {
    if (!line.trim()) continue;
    let record: { part?: unknown } | unknown;
    try {
      record = JSON.parse(line);
    } catch {
      continue;
    }
    collectEvidence((record as { part?: unknown })?.part ?? record, acc);
  }

  const marker = { truncated: false };
  const commandLog: string[] = [];
  for (const command of acc.commands.slice(0, MAX_COMMANDS)) pushLine(commandLog, command, marker);
  if (acc.commands.length > MAX_COMMANDS) marker.truncated = true;

  const testTail = acc.tests.slice(-MAX_TEST_LINES);
  const testOutput: string[] = [];
  for (const line of testTail) pushLine(testOutput, line, marker);
  if (acc.tests.length > MAX_TEST_LINES) marker.truncated = true;

  let outputTail = acc.text;
  if (outputTail.length > MAX_OUTPUT_TAIL) {
    outputTail = outputTail.slice(-MAX_OUTPUT_TAIL);
    marker.truncated = true;
  }

  return { commandLog, testOutput, outputTail, truncated: marker.truncated };
}
