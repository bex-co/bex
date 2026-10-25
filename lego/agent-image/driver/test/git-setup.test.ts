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
