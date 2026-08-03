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
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { deliverBranch, ensureRepo, extractEvidence } from "../src/delivery.mjs";

function git(cwd, args) {
  return execFileSync("git", args, { cwd }).toString().trim();
}

async function bareRemote() {
  const root = await mkdtemp(path.join(tmpdir(), "bex-agent-delivery-"));
  const remote = path.join(root, "remote.git");
  await mkdir(remote);
  git(remote, ["init", "--bare", "--initial-branch=main"]);
  // Seed the remote with an initial commit on main so a base branch exists.
  const seed = path.join(root, "seed");
  await mkdir(seed);
  git(seed, ["init", "--initial-branch=main"]);
  git(seed, ["config", "user.email", "seed@bex.co"]);
  git(seed, ["config", "user.name", "seed"]);
  await writeFile(path.join(seed, "README.md"), "seed\n");
  git(seed, ["add", "-A"]);
  git(seed, ["commit", "-m", "seed"]);
  git(seed, ["remote", "add", "origin", remote]);
  git(seed, ["push", "origin", "main"]);
  return { root, remote };
}

function baseConfig(cwd, overrides = {}) {
  return {
    cwd,
    branch: "bex-agent/session-1",
    baseBranch: "main",
    prompt: "Fix the failing test\nand tidy up",
    gitName: "bex agent",
    gitEmail: "agent@bex.co",
    sessionLogPath: path.join(cwd, "..", "session.jsonl"),
    ...overrides,
  };
}

test("ensureRepo clones an empty workspace onto the session branch", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, { repoUrl: remote });
  await ensureRepo(config);
  assert.equal(git(cwd, ["rev-parse", "--abbrev-ref", "HEAD"]), "bex-agent/session-1");
});

test("deliverBranch commits changes and pushes the branch with a head SHA", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, { repoUrl: remote });
  await ensureRepo(config);
  await writeFile(path.join(cwd, "fix.txt"), "the agent's work\n");

  const delivery = await deliverBranch(config);
  assert.equal(delivery.pushed, true);
  assert.equal(delivery.branch, "bex-agent/session-1");
  assert.equal(delivery.baseBranch, "main");
  assert.equal(delivery.commits, 1);
  assert.deepEqual(delivery.changedFiles, ["fix.txt"]);
  assert.match(delivery.headSha, /^[0-9a-f]{40}$/);
  // The branch really reached the remote.
  assert.ok(git(remote, ["rev-parse", "bex-agent/session-1"]));
});

test("deliverBranch is an honest no-op when the turn changed nothing", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, { repoUrl: remote });
  await ensureRepo(config);

  const delivery = await deliverBranch(config);
  assert.equal(delivery.pushed, false);
  assert.equal(delivery.commits, 0);
  assert.equal(delivery.headSha, "");
  assert.deepEqual(delivery.changedFiles, []);
});

test("extractEvidence pulls a bounded command log, test output, and tail", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "bex-agent-evidence-"));
  const log = path.join(root, "session.jsonl");
  const lines = [
    { part: { type: "data-acp", data: { sessionUpdate: "tool_call", title: "Run tests", kind: "execute", command: "go test ./..." } } },
    { part: { type: "data-acp", data: { sessionUpdate: "tool_call_update", content: [{ type: "terminal", output: "ok  \tpkg\t0.10s" }] } } },
    { part: { type: "text", text: "All tests pass." } },
  ];
  await writeFile(log, lines.map((l) => JSON.stringify(l)).join("\n") + "\n");

  const evidence = await extractEvidence({ sessionLogPath: log });
  assert.ok(evidence.commandLog.includes("go test ./..."));
  assert.ok(evidence.testOutput.some((t) => t.includes("ok")));
  assert.ok(evidence.outputTail.includes("All tests pass."));
  assert.equal(evidence.truncated, false);
});

test("extractEvidence marks truncation when caps drop content", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "bex-agent-evidence-"));
  const log = path.join(root, "session.jsonl");
  const many = [];
  for (let i = 0; i < 100; i++) {
    many.push(JSON.stringify({ part: { type: "data-acp", data: { command: `cmd-${i}` } } }));
  }
  await writeFile(log, many.join("\n") + "\n");

  const evidence = await extractEvidence({ sessionLogPath: log });
  assert.equal(evidence.truncated, true);
  assert.ok(evidence.commandLog.length <= 40);
});
