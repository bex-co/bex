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
import { mkdtemp, mkdir, writeFile, readFile, rm } from "node:fs/promises";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  deliverBranch,
  ensureRepo,
  extractEvidence,
  restoreWorkspace,
  type DeliveryConfig,
} from "../src/delivery.js";
import { createCredentialManager } from "../src/credentials.js";
import type { AgentDriverConfig } from "../src/config.js";

function git(cwd: string, args: string[]): string {
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

function baseConfig(
  cwd: string,
  overrides: Partial<DeliveryConfig & { repoUrl: string }> = {},
): DeliveryConfig & { repoUrl: string } {
  return {
    cwd,
    branch: "bex-agent/session-1",
    baseBranch: "main",
    prompt: "Fix the failing test\nand tidy up",
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
  assert.equal(
    git(remote, ["rev-parse", "bex-agent/session-1"]),
    delivery.headSha,
  );
});

test("deliverBranch scans against the pre-agent remote OID after local base ref rewrite", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const secret = "sk-ant-secret-hidden-in-an-ancestor";
  const config = baseConfig(cwd, {
    repoUrl: remote,
    containsSecret: (text) => text.includes(secret),
    secretNeedles: () => [Buffer.from(secret)],
  });
  await ensureRepo(config);

  await writeFile(path.join(cwd, "leak.txt"), `${secret}\n`);
  git(cwd, ["add", "-A"]);
  git(cwd, [
    "-c",
    "user.name=attacker",
    "-c",
    "user.email=a@example.com",
    "commit",
    "-m",
    "secret ancestor",
  ]);
  await rm(path.join(cwd, "leak.txt"));
  git(cwd, ["add", "-A"]);
  git(cwd, [
    "-c",
    "user.name=attacker",
    "-c",
    "user.email=a@example.com",
    "commit",
    "-m",
    "hide secret",
  ]);
  // The old implementation trusted this mutable local ref and excluded both
  // attacker commits from its scan. The captured remote base OID is immutable.
  git(cwd, ["branch", "-f", "main", "HEAD"]);
  await writeFile(path.join(cwd, "fix.txt"), "otherwise clean work\n");

  await assert.rejects(
    deliverBranch(config),
    /model credential found in commit history/,
  );
  assert.throws(() => git(remote, ["rev-parse", "bex-agent/session-1"]));
});

test("deliverBranch refuses a concurrent remote update after baseline capture", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, { repoUrl: remote });
  await ensureRepo(config);

  const peer = path.join(root, "peer");
  await mkdir(peer);
  git(peer, ["clone", remote, "."]);
  git(peer, ["checkout", "-B", config.branch]);
  await writeFile(path.join(peer, "peer.txt"), "concurrent work\n");
  git(peer, ["add", "-A"]);
  git(peer, [
    "-c",
    "user.name=peer",
    "-c",
    "user.email=peer@example.com",
    "commit",
    "-m",
    "peer update",
  ]);
  git(peer, ["push", "origin", `${config.branch}:${config.branch}`]);

  await writeFile(path.join(cwd, "mine.txt"), "my work\n");
  await assert.rejects(
    deliverBranch(config),
    /remote session branch changed after setup/,
  );
  assert.equal(
    git(remote, ["show", `${config.branch}:peer.txt`]),
    "concurrent work",
  );
});

test("deliverBranch accepts an already-published exact candidate idempotently", async () => {
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, { repoUrl: remote });
  await ensureRepo(config);

  await writeFile(path.join(cwd, "fix.txt"), "the completed work\n");
  git(cwd, ["add", "-A"]);
  git(cwd, [
    "-c",
    "user.name=bex agent",
    "-c",
    "user.email=agent@bex.co",
    "commit",
    "-m",
    "completed work",
  ]);
  const candidate = git(cwd, ["rev-parse", "HEAD"]);
  // Stand in for an earlier push whose success response was lost. Delivery
  // still scans the candidate, then recognizes the exact remote OID as success
  // without sending a second, empty receive-pack exchange.
  git(cwd, ["push", "origin", `${candidate}:refs/heads/${config.branch}`]);

  const delivery = await deliverBranch(config);
  assert.equal(delivery.pushed, true);
  assert.equal(delivery.headSha, candidate);
  assert.equal(git(remote, ["rev-parse", config.branch]), candidate);
});

test("deliverBranch refuses to push when a commit carries the model credential", async () => {
  // round-5 finding 6: the agent committed the credential itself (so it lives in
  // a compressed git object the byte-scrubber cannot reach). The pre-push history
  // scan must fail closed and never publish it.
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const secret = "sk-ant-super-secret-key-value";
  const config = baseConfig(cwd, {
    repoUrl: remote,
    containsSecret: (text) => text.includes(secret),
  });
  await ensureRepo(config);
  await writeFile(path.join(cwd, "leak.txt"), `API_KEY=${secret}\n`);

  await assert.rejects(
    deliverBranch(config),
    /model credential found in commit history/,
  );
  // The branch must NOT have reached the remote.
  assert.throws(() => git(remote, ["rev-parse", "bex-agent/session-1"]));
});

// The credential manager's full needle set, built the way session.ts wires it
// into delivery (codex round-9 #1): literal renderings plus reversible
// encodings, as byte needles for the object-level scan.
function needleConfig(secret: string) {
  const manager = createCredentialManager(
    {
      command: "",
      args: [],
      cwd: "",
      prompt: "",
      branch: "",
      repoUrl: "",
      baseBranch: "",
      deliver: false,
      gitName: "",
      gitEmail: "",
      existingSessionId: "",
      persistSession: false,
      listenHost: "",
      listenPort: 0,
      sessionLogPath: "",
      statusPath: "",
      exitAfterTurn: false,
      turnTimeoutMs: 0,
      credentialEnvName: "ANTHROPIC_API_KEY",
      modelCredential: secret,
      sessionID: "",
      grantPublicKey: "",
      agentEnv: {},
      scrubRoots: [],
    } as unknown as AgentDriverConfig,
    {},
  );
  return {
    containsSecret: (text: string) => manager.containsSecret(text),
    secretNeedles: () => manager.secretNeedles(),
  };
}

const longSecret = "sk-ant-api03-" + "x".repeat(48);

test("deliverBranch refuses to push a base64-encoded credential (round-9 #1)", async () => {
  // The raw literal never appears — only its base64 rendering, which passes the
  // textual `git log -p` containsSecret scan of round-5 finding 6 but must be
  // caught by the encoded needles in both the textual and object scans.
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const needles = needleConfig(longSecret);
  const config = baseConfig(cwd, { repoUrl: remote, ...needles });
  await ensureRepo(config);
  await writeFile(
    path.join(cwd, "encoded.txt"),
    `KEY_B64=${Buffer.from(longSecret).toString("base64")}\n`,
  );
  // The textual scan already knows this encoding (containsSecret carries the
  // encoded forms); the object scan covers the binary case below.
  assert.ok(
    config.containsSecret!(
      await readFile(path.join(cwd, "encoded.txt"), "utf8"),
    ),
  );

  await assert.rejects(
    deliverBranch(config),
    /model credential found in commit history/,
  );
  assert.throws(() => git(remote, ["rev-parse", "bex-agent/session-1"]));
});

test("deliverBranch refuses to push raw credential bytes inside a binary blob (round-9 #1)", async () => {
  // `git log -p` omits binary blob bytes ("Binary files differ"), and the file
  // content never renders the key as text — only the object-level byte scan of
  // every newly reachable blob can catch it.
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const needles = needleConfig(longSecret);
  const config = baseConfig(cwd, { repoUrl: remote, ...needles });
  await ensureRepo(config);
  // NUL bytes make git classify the blob binary; the secret sits between them.
  await writeFile(
    path.join(cwd, "blob.bin"),
    Buffer.concat([
      Buffer.from([0, 0, 1, 2]),
      Buffer.from(longSecret, "utf8"),
      Buffer.from([3, 0, 0]),
    ]),
  );
  // The textual scan genuinely cannot see it: the patch output has no secret.
  const patch = git(cwd, ["log", "-p", "--no-color", "--text", "HEAD..HEAD"]);
  assert.equal(patch, "");

  await assert.rejects(
    deliverBranch(config),
    /model credential found in commit history/,
  );
  assert.throws(() => git(remote, ["rev-parse", "bex-agent/session-1"]));
});

test("deliverBranch pushes a clean branch while secret needles are active (round-9 #1)", async () => {
  // Fail-closed scanning must not fail OPEN branches: an ordinary delivery with
  // the full needle set wired pushes exactly as before.
  const { root, remote } = await bareRemote();
  const cwd = path.join(root, "workspace");
  await mkdir(cwd);
  const config = baseConfig(cwd, {
    repoUrl: remote,
    ...needleConfig(longSecret),
  });
  await ensureRepo(config);
  await writeFile(path.join(cwd, "fix.txt"), "clean work, no key material\n");

  const delivery = await deliverBranch(config);
  assert.equal(delivery.pushed, true);
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
    {
      part: {
        type: "data-acp",
        data: {
          sessionUpdate: "tool_call",
          title: "Run tests",
          kind: "execute",
          command: "go test ./...",
        },
      },
    },
    {
      part: {
        type: "data-acp",
        data: {
          sessionUpdate: "tool_call_update",
          content: [{ type: "terminal", output: "ok  \tpkg\t0.10s" }],
        },
      },
    },
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
  const many: string[] = [];
  for (let i = 0; i < 100; i++) {
    many.push(
      JSON.stringify({
        part: { type: "data-acp", data: { command: `cmd-${i}` } },
      }),
    );
  }
  await writeFile(log, many.join("\n") + "\n");

  const evidence = await extractEvidence({ sessionLogPath: log });
  assert.equal(evidence.truncated, true);
  assert.ok(evidence.commandLog.length <= 40);
});
