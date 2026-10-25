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

// Setup-phase Git for a repo session (ADR090). Before the agent runs, the
// driver clones the session repository through the Pod-bound gateway proxy
// when the workspace is empty,
// checks out the bound bex-agent/* branch and writes repo-local identity and
// push defaults. After that the agent owns Git: it commits, pushes and opens
// its draft PR itself. The driver only reads Git state afterwards
// (delivery-check.ts); it never commits or pushes.

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { readdir, rm } from "node:fs/promises";
import { withGitAuthRetry } from "./git-auth-retry.js";

const run = promisify(execFile);

export interface RepoSetupConfig {
  cwd: string;
  branch: string;
  baseBranch: string;
  repoUrl: string;
  gitName: string;
  gitEmail: string;
  deliveryBaseline?: DeliveryBaseline;
}

export interface DeliveryBaseline {
  baseBranch: string;
  baseOid: string;
  remoteBranchOid: string;
}

// git runs the driver's own setup-phase and read-only Git calls. The workspace
// and its repository-local configuration are tenant-controlled, so
// these calls never run hooks, fsmonitor helpers, credential prompts or
// external diff commands. The agent's own Git is deliberately not hardened:
// repo hooks run for it as they would for a developer.
export async function git(cwd: string, args: string[]): Promise<string> {
  const hardenedArgs = [
    "-c",
    "core.hooksPath=/dev/null",
    "-c",
    "core.fsmonitor=false",
    "-c",
    "diff.external=",
    "-c",
    "core.attributesFile=/dev/null",
    ...args,
  ];
  const { stdout } = await run("git", hardenedArgs, {
    cwd,
    maxBuffer: 32 * 1024 * 1024,
    env: {
      ...process.env,
      GIT_CONFIG_NOSYSTEM: "1",
      GIT_CONFIG_GLOBAL: "/dev/null",
      GIT_TERMINAL_PROMPT: "0",
    },
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
  const advertised = await gitRemote(cwd, [
    "ls-remote",
    "--symref",
    repoUrl,
    "HEAD",
  ]);
  const match = advertised.match(/^ref: refs\/heads\/(.+)\tHEAD$/m);
  const branch = match?.[1] || "main";
  await validBranch(cwd, branch);
  return branch;
}

// remoteOid reads a branch head from the session Git proxy. Setup retries the
// startup authorization race; the post-turn delivery check probes once.
export async function remoteOid(
  cwd: string,
  repoUrl: string,
  branch: string,
  { retry = true }: { retry?: boolean } = {},
): Promise<string> {
  const args = ["ls-remote", "--refs", repoUrl, `refs/heads/${branch}`];
  const output = retry ? await gitRemote(cwd, args) : await git(cwd, args);
  if (!output) return "";
  const oid = output.split(/\s+/, 1)[0];
  if (!/^[0-9a-f]{40,64}$/.test(oid))
    throw new Error("Git remote returned an invalid object ID");
  return oid;
}

// hasCommit reports whether oid names a commit present in the local object
// database.
export async function hasCommit(cwd: string, oid: string): Promise<boolean> {
  if (!oid) return false;
  try {
    await git(cwd, ["cat-file", "-e", `${oid}^{commit}`]);
    return true;
  } catch {
    return false;
  }
}

async function fetchOid(
  cwd: string,
  repoUrl: string,
  oid: string,
): Promise<void> {
  if (await hasCommit(cwd, oid)) return;
  await gitRemote(cwd, ["fetch", "--no-tags", repoUrl, oid]);
  if (!(await hasCommit(cwd, oid))) {
    throw new Error(`Git remote did not provide commit ${oid}`);
  }
}

// gitRemote retries ls-remote/fetch against the session git proxy. A pre-cloned
// workspace skips cloneWithRetry (the tree is already a repo) and would otherwise
// fail closed on the first 403 of the sandbox-id mint race.
async function gitRemote(cwd: string, args: string[]): Promise<string> {
  return withGitAuthRetry(() => git(cwd, args));
}

async function captureDeliveryBaseline(
  config: RepoSetupConfig,
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
  await withGitAuthRetry(() => git(cwd, ["clone", repoUrl, "."]), {
    onRetry: () => cleanDirectory(cwd),
  });
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
// Pod-bound gateway smart-HTTP proxy; no GitHub credential enters this process
// or .git/config. Repo-local identity, push.default=current and the branch
// upstream make a plain `git commit` + `git push` by the agent land on the
// bound branch.
export async function ensureRepo(
  config: RepoSetupConfig,
): Promise<DeliveryBaseline> {
  if (!config.repoUrl) {
    throw new Error("BEX_AGENT_REPO_URL is required for delivery");
  }
  const existing = await isGitRepo(config.cwd);
  if (!existing) {
    await cloneWithRetry(config.cwd, config.repoUrl);
  }
  // A pre-cloned workspace is tenant-controlled state. Reset origin to
  // the control-plane supplied proxy before reading any remote identity.
  await git(config.cwd, ["remote", "set-url", "origin", config.repoUrl]);
  const baseline = await captureDeliveryBaseline(config);
  if (existing) {
    // Preserve existing commits and working-tree edits, but put them on the
    // session ref; the agent delivers them on its next turn.
    await git(config.cwd, ["checkout", "-B", config.branch]);
  } else {
    await git(config.cwd, [
      "checkout",
      "-B",
      config.branch,
      baseline.remoteBranchOid || baseline.baseOid,
    ]);
  }
  await configureAgentGit(config);
  config.deliveryBaseline = baseline;
  return baseline;
}

async function configureAgentGit(config: RepoSetupConfig): Promise<void> {
  const settings: [string, string][] = [
    ["user.name", config.gitName],
    ["user.email", config.gitEmail],
    ["push.default", "current"],
    [`branch.${config.branch}.remote`, "origin"],
    [`branch.${config.branch}.merge`, `refs/heads/${config.branch}`],
  ];
  for (const [key, value] of settings) {
    await git(config.cwd, ["config", "--local", key, value]);
  }
}
