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
import { createHash } from "node:crypto";
import { mkdtemp, open, readdir, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

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
  repoUrl: string;
  prompt: string;
  gitName: string;
  gitEmail: string;
  // containsSecret, when set, fails the push closed if any commit on the branch
  // still carries the model credential (round-5 finding 6). Optional so direct
  // callers/tests that never handle a credential are unaffected.
  containsSecret?: (text: string) => boolean;
  // secretNeedles, when set, supplies every byte form of the credential (raw,
  // JSON-escaped, and the reversible encodings) for the object-level scan
  // (codex round-9 #1): every blob newly reachable from the branch is read and
  // searched byte-for-byte, so an encoded credential — or one inside a binary
  // blob `git log -p` omits — still fails the push. Optional for the same
  // reason as containsSecret; either may be set alone.
  secretNeedles?: () => Buffer[];
  deliveryBaseline?: DeliveryBaseline;
}

export interface DeliveryBaseline {
  baseBranch: string;
  baseOid: string;
  remoteBranchOid: string;
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
  truncated: boolean;
}

async function git(cwd: string, args: string[]): Promise<string> {
  const { stdout } = await run("git", args, {
    cwd,
    maxBuffer: 32 * 1024 * 1024,
  });
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

async function validBranch(cwd: string, branch: string): Promise<void> {
  if (!branch || branch.startsWith("-")) throw new Error("invalid Git branch");
  await git(cwd, ["check-ref-format", "--branch", branch]);
}

async function remoteBaseBranch(
  cwd: string,
  repoUrl: string,
  fallback: string,
): Promise<string> {
  if (fallback) {
    await validBranch(cwd, fallback);
    return fallback;
  }
  const advertised = await git(cwd, ["ls-remote", "--symref", repoUrl, "HEAD"]);
  const match = advertised.match(/^ref: refs\/heads\/(.+)\tHEAD$/m);
  const branch = match?.[1] || "main";
  await validBranch(cwd, branch);
  return branch;
}

async function remoteOid(
  cwd: string,
  repoUrl: string,
  branch: string,
): Promise<string> {
  const output = await git(cwd, [
    "ls-remote",
    "--refs",
    repoUrl,
    `refs/heads/${branch}`,
  ]);
  if (!output) return "";
  const oid = output.split(/\s+/, 1)[0];
  if (!/^[0-9a-f]{40,64}$/.test(oid))
    throw new Error("Git remote returned an invalid object ID");
  return oid;
}

async function fetchOid(
  cwd: string,
  repoUrl: string,
  oid: string,
): Promise<void> {
  try {
    await git(cwd, ["cat-file", "-e", `${oid}^{commit}`]);
  } catch {
    await git(cwd, ["fetch", "--no-tags", repoUrl, oid]);
  }
  await git(cwd, ["cat-file", "-e", `${oid}^{commit}`]);
}

async function captureDeliveryBaseline(
  config: DeliveryConfig,
): Promise<DeliveryBaseline> {
  const baseBranch = await remoteBaseBranch(
    config.cwd,
    config.repoUrl,
    config.baseBranch,
  );
  await validBranch(config.cwd, config.branch);
  const baseOid = await remoteOid(config.cwd, config.repoUrl, baseBranch);
  if (!baseOid)
    throw new Error(`remote base branch ${baseBranch} does not exist`);
  const remoteBranchOid = await remoteOid(
    config.cwd,
    config.repoUrl,
    config.branch,
  );
  await fetchOid(config.cwd, config.repoUrl, baseOid);
  if (remoteBranchOid)
    await fetchOid(config.cwd, config.repoUrl, remoteBranchOid);
  return { baseBranch, baseOid, remoteBranchOid };
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
  config: { restoreUrl: string; restoreSha?: string; restoreBytes?: number },
  destRoot = "/",
): Promise<void> {
  if (!config.restoreUrl) return;
  const dir = await mkdtemp(path.join(tmpdir(), "bex-restore-"));
  const archive = path.join(dir, "snap.tgz");
  try {
    await run("/bin/sh", ["-c", 'curl -sf "$BEX_RESTORE_URL" -o "$BEX_RESTORE_FILE"'], {
      env: {
        ...process.env,
        BEX_RESTORE_URL: config.restoreUrl,
        BEX_RESTORE_FILE: archive,
      },
    });
    const body = await readFile(archive);
    if (
      config.restoreBytes != null &&
      config.restoreBytes > 0 &&
      body.length !== config.restoreBytes
    ) {
      throw new Error(
        `snapshot size mismatch: got ${body.length}, want ${config.restoreBytes}`,
      );
    }
    if (config.restoreSha) {
      const sha = createHash("sha256").update(body).digest("hex");
      if (sha !== config.restoreSha) {
        throw new Error("snapshot digest mismatch");
      }
    }
    await run(
      "/bin/sh",
      ["-c", 'tar xzf "$BEX_RESTORE_FILE" -C "$BEX_RESTORE_DEST"'],
      {
        env: {
          ...process.env,
          BEX_RESTORE_FILE: archive,
          BEX_RESTORE_DEST: destRoot,
        },
      },
    );
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
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
    entries.map((entry) =>
      rm(`${cwd}/${entry}`, { recursive: true, force: true }),
    ),
  );
}

// ensureRepo makes the workspace a checkout of the session branch. A pre-cloned
// workspace is left alone; an empty one is cloned and switched to the branch
// (created from the default when it does not exist remotely). The remote is the
// Pod-bound gateway smart-HTTP proxy; no GitHub credential enters this process.
export async function ensureRepo(
  config: DeliveryConfig & { repoUrl: string },
): Promise<DeliveryBaseline> {
  if (!config.repoUrl) {
    throw new Error("BEX_AGENT_REPO_URL is required for delivery");
  }
  const existing = await isGitRepo(config.cwd);
  if (!existing) {
    await cloneWithRetry(config.cwd, config.repoUrl);
  }
  // A restored workspace is tenant-controlled persisted state. Reset origin to
  // the control-plane supplied proxy before reading any remote identity.
  await git(config.cwd, ["remote", "set-url", "origin", config.repoUrl]);
  const baseline = await captureDeliveryBaseline(config);
  if (existing) {
    // Preserve restored commits and working-tree edits, but put them on the
    // session ref. Their full closure is scanned against baseOid at delivery.
    await git(config.cwd, ["checkout", "-B", config.branch]);
  } else {
    await git(config.cwd, [
      "checkout",
      "-B",
      config.branch,
      baseline.remoteBranchOid || baseline.baseOid,
    ]);
  }
  config.deliveryBaseline = baseline;
  return baseline;
}

function commitMessage(config: DeliveryConfig): string {
  const first = String(config.prompt || "")
    .split("\n")[0]
    .trim();
  const subject = first ? first.slice(0, 72) : "agent session update";
  return `bex agent: ${subject}\n\nDelivered by bex cloud coding-agent session.`;
}

// deliverBranch stages, commits, and pushes the session branch, returning the
// delivery record the Completer consumes. A turn that produced no change pushes
// nothing (pushed:false) — an honest no-op, not an error.
export async function deliverBranch(
  config: DeliveryConfig,
): Promise<DeliveryResult> {
  const cwd = config.cwd;
  const baseline = config.deliveryBaseline;
  if (!baseline)
    throw new Error(
      "refusing to push: trusted remote baseline was not captured",
    );
  const base = baseline.baseBranch;
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
    commits =
      Number(
        await git(cwd, [
          "rev-list",
          "--count",
          `${baseline.baseOid}..${config.branch}`,
        ]),
      ) || 0;
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
      const patch = await git(cwd, [
        "log",
        "-p",
        "--no-color",
        `${baseline.baseOid}..${config.branch}`,
      ]);
      if (config.containsSecret(patch)) {
        throw new Error(
          "refusing to push: model credential found in commit history",
        );
      }
    }
    // codex round-9 #1: the textual patch scan misses reversible ENCODINGS of
    // the credential (base64/hex render as ordinary text) and binary blobs
    // (`git log -p` prints "Binary files differ" without the bytes). Enumerate
    // every object the branch newly reaches, read each blob's actual bytes,
    // and search every needle form. A blob too large to inspect — or a payload
    // whose total exceeds the scan budget — refuses the push: uninspectable
    // new objects are exactly what an exfiltration hides behind.
    if (config.secretNeedles) {
      await scanBranchObjects(
        cwd,
        baseline.baseOid,
        config.branch,
        config.secretNeedles(),
      );
    }
    const candidateOid = await git(cwd, [
      "rev-parse",
      `${config.branch}^{commit}`,
    ]);
    // Re-read the remote immediately before the write. A prior delivery whose
    // response was lost (or an explicit retry of the same turn) may already
    // have published the exact scanned object. Treat that as idempotent success
    // instead of issuing a no-update receive-pack that can obscure the outcome
    // with a transport error. Any different remote update remains a conflict.
    const beforePushOid = await remoteOid(
      cwd,
      config.repoUrl,
      config.branch,
    );
    if (beforePushOid !== candidateOid) {
      if (beforePushOid !== baseline.remoteBranchOid) {
        throw new Error(
          "refusing delivery: remote session branch changed after setup",
        );
      }
      // Push the exact object that was scanned. A force-with-lease binds the
      // write to the remote session ref captured before tenant execution, so
      // neither a local-ref rewrite nor a concurrent remote update can change
      // the result.
      const lease = `--force-with-lease=refs/heads/${config.branch}:${baseline.remoteBranchOid}`;
      try {
        await git(cwd, [
          "push",
          lease,
          config.repoUrl,
          `${candidateOid}:refs/heads/${config.branch}`,
        ]);
      } catch (pushError) {
        // A dropped HTTP response is ambiguous: the forge may have committed
        // the ref update even though Git reported an RPC failure. Only suppress
        // the transport error when a fresh read proves the exact scanned OID is
        // now published; otherwise preserve the original diagnostic.
        let observed = "";
        try {
          observed = await remoteOid(cwd, config.repoUrl, config.branch);
        } catch {
          throw pushError;
        }
        if (observed !== candidateOid) throw pushError;
      }
    }
    const publishedOid = await remoteOid(cwd, config.repoUrl, config.branch);
    if (publishedOid !== candidateOid) {
      throw new Error(
        "refusing delivery: remote branch does not match the scanned object",
      );
    }
    pushed = true;
    headSha = candidateOid;
    baseline.remoteBranchOid = candidateOid;
  }
  return {
    branch: config.branch,
    baseBranch: base,
    headSha,
    pushed,
    commits,
    changedFiles,
  };
}

// One blob larger than this is refused rather than scanned (matching the
// persisted-state scrubber's per-file ceiling); the total budget bounds the
// whole delivery's scan cost. Both are fail-closed refusals, not skips.
const maxScanBlobBytes = 32 * 1024 * 1024;
const maxScanTotalBytes = 256 * 1024 * 1024;

// scanBranchObjects reads every blob reachable from branch but not base and
// fails (throws) when any needle appears — or when the payload cannot be fully
// inspected. Commit and tree objects are structural and carry no free bytes an
// attacker controls beyond the blob contents already scanned (plus the commit
// message, which the textual patch scan above covers), so only blobs are read.
async function scanBranchObjects(
  cwd: string,
  base: string,
  branch: string,
  needles: Buffer[],
): Promise<void> {
  if (needles.length === 0) return;
  const objects = await git(cwd, [
    "rev-list",
    "--objects",
    "--no-object-names",
    `${base}..${branch}`,
  ]);
  let total = 0;
  for (const oid of objects
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)) {
    let type: string;
    try {
      type = await git(cwd, ["cat-file", "-t", oid]);
    } catch {
      continue; // object disappeared between listing and typing (concurrent gc)
    }
    if (type !== "blob") continue;
    const size = Number(await git(cwd, ["cat-file", "-s", oid])) || 0;
    if (size > maxScanBlobBytes) {
      throw new Error(
        `refusing to push: object ${oid} is too large to inspect for the model credential`,
      );
    }
    total += size;
    if (total > maxScanTotalBytes) {
      throw new Error(
        "refusing to push: branch payload is too large to inspect for the model credential",
      );
    }
    // encoding:"buffer" keeps the blob's raw bytes — a utf8 string decode would
    // mangle binary content and the byte-needle match with it.
    const { stdout } = await run("git", ["cat-file", "blob", oid], {
      cwd,
      maxBuffer: maxScanBlobBytes + 1024 * 1024,
      encoding: "buffer",
    });
    if (needles.some((needle) => stdout.includes(needle))) {
      throw new Error(
        "refusing to push: model credential found in commit history",
      );
    }
  }
}

function pushLine(
  list: string[],
  value: unknown,
  marker: { truncated: boolean },
): void {
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
function collectEvidence(
  node: unknown,
  acc: EvidenceAccumulator,
  depth = 0,
): void {
  if (node == null || depth > MAX_WALK_DEPTH || typeof node !== "object")
    return;
  if (Array.isArray(node)) {
    for (const item of node) collectEvidence(item, acc, depth + 1);
    return;
  }
  const record = node as Record<string, unknown>;
  for (const key of ["command", "commandLine"]) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) {
      if (acc.commands.length < MAX_COMMANDS) acc.commands.push(value);
      else acc.truncated = true;
    }
  }
  if (
    record.sessionUpdate === "tool_call" &&
    typeof record.title === "string" &&
    !record.command
  ) {
    if (acc.commands.length < MAX_COMMANDS) acc.commands.push(record.title);
    else acc.truncated = true;
  }
  for (const key of ["output", "stdout", "stderr"]) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) {
      acc.tests.push(value);
      if (acc.tests.length > MAX_TEST_LINES) {
        acc.tests.shift();
        acc.truncated = true;
      }
    }
  }
  if (record.type === "text" && typeof record.text === "string") {
    acc.text += record.text;
    if (acc.text.length > MAX_OUTPUT_TAIL) {
      acc.text = acc.text.slice(-MAX_OUTPUT_TAIL);
      acc.truncated = true;
    }
  }
  for (const value of Object.values(record))
    collectEvidence(value, acc, depth + 1);
}

// extractEvidence reads the redacted session log and returns the bounded,
// Codex-style evidence extract (command log + test-output tails + a trailing
// output window). Every cap that drops content sets truncated explicitly.
export async function extractEvidence(config: {
  sessionLogPath: string;
}): Promise<EvidenceResult> {
  let raw = "";
  let inputTruncated = false;
  try {
    const file = await open(config.sessionLogPath, "r");
    try {
      const info = await file.stat();
      const maxEvidenceBytes = 16 << 20;
      const length = Math.min(info.size, maxEvidenceBytes);
      const offset = info.size - length;
      const buffer = Buffer.alloc(length);
      await file.read(buffer, 0, length, offset);
      raw = buffer.toString("utf8");
      if (offset > 0) {
        inputTruncated = true;
        const newline = raw.indexOf("\n");
        raw = newline === -1 ? "" : raw.slice(newline + 1);
      }
    } finally {
      await file.close();
    }
  } catch {
    raw = "";
  }
  const acc: EvidenceAccumulator = {
    commands: [],
    tests: [],
    text: "",
    truncated: inputTruncated,
  };
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

  const marker = { truncated: acc.truncated };
  const commandLog: string[] = [];
  for (const command of acc.commands) pushLine(commandLog, command, marker);

  const testOutput: string[] = [];
  for (const line of acc.tests) pushLine(testOutput, line, marker);

  const outputTail = acc.text;

  return { commandLog, testOutput, outputTail, truncated: marker.truncated };
}
