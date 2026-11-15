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

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtemp, mkdir, writeFile, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { ensureRepo, type RepoSetupConfig } from "../src/git-setup.js";
import { git, seedBareRemote } from "./helpers.js";

async function bareRemote() {
  const root = await mkdtemp(path.join(tmpdir(), "bex-agent-git-setup-"));
  return { root, remote: await seedBareRemote(root) };
}

function baseConfig(
  cwd: string,
  overrides: Partial<RepoSetupConfig> = {},
): RepoSetupConfig {
  return {
    cwd,
    branch: "bex-agent/session-1",
    baseBranch: "main",
    gitName: "bex agent",
    gitEmail: "agent@bex.co",
    repoUrl: "",
    ...overrides,
  };
}

test("ensureRepo clones an empty workspace onto the session branch", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, { repoUrl: remote });
  await ensureRepo(config);
  assert.equal(
    git(cwd, ["rev-parse", "--abbrev-ref", "HEAD"]),
    "bex-agent/session-1",
  );
});

// Isolated from the developer's own Git identity: only the repo-local config
// ensureRepo writes may supply the author.
function agentGit(cwd: string, args: string[]): string {
  const env: NodeJS.ProcessEnv = {
    ...process.env,
    GIT_CONFIG_GLOBAL: "/dev/null",
    GIT_CONFIG_NOSYSTEM: "1",
  };
  for (const name of [
    "GIT_AUTHOR_NAME",
    "GIT_AUTHOR_EMAIL",
    "GIT_COMMITTER_NAME",
    "GIT_COMMITTER_EMAIL",
    "EMAIL",
  ])
    delete env[name];
  return execFileSync("git", args, { cwd, env }).toString().trim();
}

test("after ensureRepo a plain agent commit + push lands on the bound branch", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  await ensureRepo(
    baseConfig(cwd, {
      repoUrl: remote,
      gitName: "bex agent",
      gitEmail: "agent@bex.co",
    }),
  );
  await writeFile(path.join(cwd, "fix.txt"), "the agent's work\n");
  agentGit(cwd, ["add", "fix.txt"]);
  agentGit(cwd, ["commit", "-m", "fix: the agent's own commit"]);
  agentGit(cwd, ["push"]);
  const head = agentGit(cwd, ["rev-parse", "HEAD"]);
  assert.equal(
    git(remote, ["rev-parse", "refs/heads/bex-agent/session-1"]),
    head,
  );
  assert.equal(
    git(remote, ["log", "-1", "--format=%an <%ae>|%s", "bex-agent/session-1"]),
    "bex agent <agent@bex.co>|fix: the agent's own commit",
  );
  // The pull --rebase recovery path the overlay names works without arguments.
  agentGit(cwd, ["pull", "--rebase"]);
  // The remote is the credential-free proxy URL; no credential lands in config.
  const config = await readFile(path.join(cwd, ".git", "config"), "utf8");
  assert.equal(agentGit(cwd, ["remote", "get-url", "origin"]), remote);
  assert.doesNotMatch(config, /credential|token|password|extraheader/i);
  assert.equal(agentGit(cwd, ["config", "push.default"]), "current");
});

// ADR047 D12 (w22/m413): a turn a person stopped keeps its disk, and the
// session's next turn runs in a fresh sandbox booted from it. That sandbox's
// driver starts over the stopped turn's checkout: ensureRepo must leave the
// commits the agent had not pushed and the edits it had not committed where
// they are, on the session branch, for the agent to deliver. The driver itself
// never commits or pushes (ADR090), so nothing here is written to the remote.
async function stoppedTurnDisk() {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  // The stopped turn: the driver set up the checkout, and the agent worked.
  await ensureRepo(baseConfig(cwd, { repoUrl: remote }));
  await writeFile(path.join(cwd, "pushed.txt"), "already delivered\n");
  agentGit(cwd, ["add", "pushed.txt"]);
  agentGit(cwd, ["commit", "-m", "feat: delivered part"]);
  agentGit(cwd, ["push"]);
  await writeFile(
    path.join(cwd, "feature.txt"),
    "the first half of the feature\n",
  );
  agentGit(cwd, ["add", "feature.txt"]);
  agentGit(cwd, ["commit", "-m", "feat: unpushed commit"]);
  await writeFile(path.join(cwd, "README.md"), "seed\nedited, not committed\n");
  await writeFile(path.join(cwd, "scratch.txt"), "untracked work\n");
  return { root, remote, cwd };
}

async function assertWorkSurvived(
  cwd: string,
  remote: string,
  unpushed: string,
) {
  assert.equal(
    git(cwd, ["rev-parse", "--abbrev-ref", "HEAD"]),
    "bex-agent/session-1",
  );
  assert.equal(git(cwd, ["rev-parse", "HEAD"]), unpushed);
  assert.equal(git(cwd, ["log", "-1", "--format=%s"]), "feat: unpushed commit");
  assert.equal(
    await readFile(path.join(cwd, "feature.txt"), "utf8"),
    "the first half of the feature\n",
  );
  assert.equal(
    await readFile(path.join(cwd, "README.md"), "utf8"),
    "seed\nedited, not committed\n",
  );
  assert.equal(
    await readFile(path.join(cwd, "scratch.txt"), "utf8"),
    "untracked work\n",
  );
  // (the helper trims the first line's leading space)
  const status = git(cwd, ["status", "--porcelain"])
    .split("\n")
    .map((line) => line.trim())
    .sort();
  assert.deepEqual(status, ["?? scratch.txt", "M README.md"]);
  // Nothing was pushed on the agent's behalf: the remote branch is where the
  // agent left it, one commit behind the checkout.
  assert.notEqual(
    git(remote, ["rev-parse", "refs/heads/bex-agent/session-1"]),
    unpushed,
  );
}

test("a sandbox booted from a stopped turn's disk keeps its dirty tree and unpushed commit", async () => {
  const { root, remote, cwd } = await stoppedTurnDisk();
  const unpushed = git(cwd, ["rev-parse", "HEAD"]);
  // The next turn's sandbox gets its own proxy URL for the same repository,
  // and the repository moved on meanwhile.
  const moved = path.join(root, "moved");
  git(root, ["clone", remote, moved]);
  git(moved, ["config", "user.email", "other@bex.co"]);
  git(moved, ["config", "user.name", "other"]);
  await writeFile(path.join(moved, "other.txt"), "someone else's change\n");
  git(moved, ["add", "-A"]);
  git(moved, ["commit", "-m", "other: a change on main"]);
  git(moved, ["push", "origin", "main"]);

  await ensureRepo(baseConfig(cwd, { repoUrl: remote }));
  await assertWorkSurvived(cwd, remote, unpushed);
  // A third turn over the same disk changes nothing either.
  await ensureRepo(baseConfig(cwd, { repoUrl: remote }));
  await assertWorkSurvived(cwd, remote, unpushed);
});

test("a stopped turn that left the checkout on another branch keeps its work on the session branch", async () => {
  const { remote, cwd } = await stoppedTurnDisk();
  // The agent had switched to a scratch branch before it was stopped: its
  // commits and edits come with it, onto the session branch.
  agentGit(cwd, ["checkout", "-b", "scratch"]);
  const unpushed = git(cwd, ["rev-parse", "HEAD"]);
  await ensureRepo(baseConfig(cwd, { repoUrl: remote }));
  await assertWorkSurvived(cwd, remote, unpushed);
});

test("the disk's checkout is pointed at the next turn's own remote and holds no credential", async () => {
  const { root, remote, cwd } = await stoppedTurnDisk();
  const other = path.join(root, "next-remote.git");
  git(root, ["clone", "--bare", remote, other]);
  await ensureRepo(baseConfig(cwd, { repoUrl: other }));
  assert.equal(agentGit(cwd, ["remote", "get-url", "origin"]), other);
  const config = await readFile(path.join(cwd, ".git", "config"), "utf8");
  assert.doesNotMatch(config, /credential|token|password|extraheader/i);
});
