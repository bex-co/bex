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
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdtemp, mkdir, writeFile, readFile } from "node:fs/promises";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  ensureRepo,
  restoreWorkspace,
  type RepoSetupConfig,
} from "../src/git-setup.js";
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

test("restoreWorkspace is a no-op when no restore URL is set (normal clone path)", async () => {
  // ADR059 D4: without a snapshot the driver clones as before; restore must not
  // run curl/tar at all.
  await restoreWorkspace({ restoreUrl: "" });
});

test("restoreWorkspace fetches + untars a snapshot, leaving ensureRepo a no-op", async () => {
  // Build a real .tgz of a pre-populated git workspace, serve it over a local
  // HTTP server (the presigned-GET stand-in), restore it, and assert the working
  // tree — including an uncommitted edit — survives and ensureRepo skips cloning.
  const { root, remote } = await bareRemote();
  const src = path.join(root, "src-workspace");
  await mkdir(src);
  git(src, ["clone", remote, "."]);
  git(src, ["checkout", "-B", "bex-agent/session-1"]);
  await writeFile(path.join(src, "uncommitted.txt"), "unpushed work\n");
  const archive = path.join(root, "snap.tgz");
  execFileSync("tar", ["czf", archive, "-C", src, "."]);
  const body = await readFile(archive);

  const server = createServer((_req, res) => {
    res.writeHead(200);
    res.end(body);
  });
  await new Promise<void>((r) => server.listen(0, r));
  const port = (server.address() as AddressInfo).port;
  const cwd = path.join(root, "restored");
  await mkdir(cwd);
  try {
    await restoreWorkspace(
      { restoreUrl: `http://127.0.0.1:${port}/snap.tgz` },
      cwd,
    );
    // The uncommitted working-tree edit survived the snapshot round-trip.
    assert.equal(
      (await readFile(path.join(cwd, "uncommitted.txt"))).toString(),
      "unpushed work\n",
    );
    // A restored workspace is already a git repo, so ensureRepo does not re-clone.
    await ensureRepo(baseConfig(cwd, { repoUrl: remote }));
    assert.ok(
      (await readFile(path.join(cwd, "uncommitted.txt")))
        .toString()
        .includes("unpushed"),
    );
  } finally {
    server.close();
  }
});

test("restoreWorkspace refuses a mismatched snapshot digest before extract", async () => {
  const { root, remote } = await bareRemote();
  const src = path.join(root, "src-workspace");
  await mkdir(src);
  git(src, ["clone", remote, "."]);
  const archive = path.join(root, "snap.tgz");
  execFileSync("tar", ["czf", archive, "-C", src, "."]);
  const body = await readFile(archive);
  const realSha = createHash("sha256").update(body).digest("hex");

  const server = createServer((_req, res) => {
    res.writeHead(200);
    res.end(body);
  });
  await new Promise<void>((r) => server.listen(0, r));
  const port = (server.address() as AddressInfo).port;
  const cwd = path.join(root, "restored");
  await mkdir(cwd);
  try {
    await assert.rejects(
      () =>
        restoreWorkspace(
          {
            restoreUrl: `http://127.0.0.1:${port}/snap.tgz`,
            restoreSha: "0".repeat(64),
            restoreBytes: body.length,
          },
          cwd,
        ),
      /snapshot digest mismatch/,
    );
    await assert.rejects(
      () =>
        restoreWorkspace(
          {
            restoreUrl: `http://127.0.0.1:${port}/snap.tgz`,
            restoreSha: realSha,
            restoreBytes: body.length + 1,
          },
          cwd,
        ),
      /snapshot size mismatch/,
    );
  } finally {
    server.close();
  }
});

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
