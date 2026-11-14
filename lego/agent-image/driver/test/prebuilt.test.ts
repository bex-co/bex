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

// Sessions booted from a snapshot build's image (ADR111 D4): the checkout
// moves to the latest code, the setup steps are not rerun, and the overlay
// names them as already run. Without a usable image, phase 1 runs as before.

import assert from "node:assert/strict";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { runBuild } from "../src/build.js";
import { ensureRepo } from "../src/git-setup.js";
import { platformOverlay } from "../src/overlay.js";
import type { RepoEnvironment } from "../src/repo-setup.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import {
  childEnv,
  git,
  seedBareRemote,
  tempConfig,
  type TestConfig,
} from "./helpers.js";

const environment: RepoEnvironment = {
  setup: [
    {
      name: "deps",
      // Counts its runs, installs a dependency, and changes a tracked file
      // the way a lockfile rewrite would.
      run: "echo run >> .setup-runs && mkdir -p node_modules && echo dep > node_modules/dep.js && echo setup >> README.md",
    },
  ],
  checks: [{ name: "test", run: "npm test" }],
};

// pushCommit commits files onto the remote's main branch.
async function pushCommit(
  root: string,
  remote: string,
  files: Record<string, string>,
  message: string,
): Promise<string> {
  const clone = await mkdtemp(path.join(root, "push-"));
  git(clone, ["clone", "-q", remote, "."]);
  git(clone, ["config", "user.email", "dev@bex.co"]);
  git(clone, ["config", "user.name", "dev"]);
  for (const [name, contents] of Object.entries(files)) {
    await writeFile(path.join(clone, name), contents);
  }
  git(clone, ["add", "-A"]);
  git(clone, ["commit", "-q", "-m", message]);
  git(clone, ["push", "-q", "origin", "main"]);
  return git(clone, ["rev-parse", "HEAD"]);
}

// snapshotImage builds the environment into a workspace the way a build
// sandbox does, then moves the remote's main past it: the session must start
// from the newer commit.
async function snapshotImage() {
  const image = await tempConfig();
  const remote = await seedBareRemote(image.root);
  await pushCommit(
    image.root,
    remote,
    { ".gitignore": "node_modules/\n.setup-runs\n" },
    "ignore deps",
  );
  const built = await runBuild(
    {
      cwd: image.cwd,
      repoUrl: remote,
      repository: "acme/widgets",
      repoEnvironment: environment,
      statusPath: path.join(image.root, "build-status.json"),
      sessionLogPath: path.join(image.root, "build-session.jsonl"),
      prRecordPath: path.join(image.root, "build-pr.json"),
      resultRecordPath: path.join(image.root, "build-result.json"),
      secretValues: new Map(),
    },
    { env: { PATH: process.env.PATH ?? "" }, cloneRetry: { attempts: 1 } },
  );
  assert.equal(built.state, "succeeded");
  const latest = await pushCommit(
    image.root,
    remote,
    { "README.md": "latest\n" },
    "the latest code",
  );
  return { workspace: image.cwd, remote, latest, built };
}

async function sessionConfig(
  overrides: Partial<TestConfig>,
): Promise<TestConfig & { systemPromptLog: string }> {
  const config = await tempConfig({
    deliver: true,
    branch: "bex-agent/ags-1",
    repository: "acme/widgets",
    repoEnvironment: environment,
    ...overrides,
  });
  const systemPromptLog = path.join(config.root, "sessions.log");
  config.agentEnv = {
    ACP_FIXTURE_SYSTEM_PROMPT: "1",
    ACP_FIXTURE_SESSION_LOG: systemPromptLog,
  };
  return Object.assign(config, { systemPromptLog });
}

// checkout runs ensureRepo as main.ts does, recording its verdict.
async function checkout(config: TestConfig): Promise<boolean> {
  config.setupBaked = (await ensureRepo(config)).setupBaked;
  return config.setupBaked;
}

function setupCalls(hub: UIMessageStreamHub): unknown[] {
  return hub.history.filter((part) =>
    String(part.toolCallId ?? "").startsWith("bex-setup-"),
  );
}

async function overlaySent(file: string): Promise<string> {
  const [call] = (await readFile(file, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  return call.params.systemPrompt.content[0].text;
}

test("a prebuilt session pulls the latest code, keeps what setup installed and does not rerun it", async () => {
  const { workspace, remote, latest } = await snapshotImage();
  const config = await sessionConfig({
    cwd: workspace,
    repoUrl: remote,
    prebuilt: "agb-1",
  });
  assert.equal(await checkout(config), true);
  assert.equal(
    git(workspace, ["rev-parse", "--abbrev-ref", "HEAD"]),
    "bex-agent/ags-1",
  );
  assert.equal(git(workspace, ["rev-parse", "HEAD"]), latest);
  assert.equal(git(workspace, ["rev-parse", "origin/main"]), latest);
  assert.equal(
    await readFile(path.join(workspace, "README.md"), "utf8"),
    "latest\n",
  );
  assert.equal(git(workspace, ["status", "--porcelain"]), "");
  assert.equal(
    await readFile(path.join(workspace, "node_modules/dep.js"), "utf8"),
    "dep\n",
  );
  assert.equal(git(workspace, ["remote", "get-url", "origin"]), remote);

  const hub = new UIMessageStreamHub();
  const result = await runHeadlessTurn(config, childEnv(config), hub);
  assert.equal(result.state, "succeeded");
  assert.deepEqual(setupCalls(hub), [], "setup ran again");
  assert.equal(
    await readFile(path.join(workspace, ".setup-runs"), "utf8"),
    "run\n",
    "setup ran again",
  );
  const overlay = await overlaySent(config.systemPromptLog);
  assert.equal(overlay, platformOverlay(config));
  assert.match(overlay, /## Repository setup\n/);
  assert.match(overlay, /already ran from the repository root/);
  assert.match(overlay, /rerun the step/);
  assert.ok(
    overlay.includes(
      `### deps\n\n\`\`\`sh\n${environment.setup[0].run}\n\`\`\``,
    ),
  );
  assert.match(overlay, /## Repository checks[\s\S]*### test/);

  // A driver restarted in the same sandbox keeps the session's own work.
  await writeFile(path.join(workspace, "work.txt"), "in progress\n");
  git(workspace, ["add", "work.txt"]);
  await ensureRepo(config);
  assert.equal(git(workspace, ["rev-parse", "HEAD"]), latest);
  assert.equal(git(workspace, ["status", "--porcelain"]), "A  work.txt");
});

test("a later turn of a prebuilt session starts from the session branch it pushed", async () => {
  const { workspace, remote, latest } = await snapshotImage();
  const pushed = await (async () => {
    const root = path.dirname(workspace);
    const clone = await mkdtemp(path.join(root, "turn1-"));
    git(clone, ["clone", "-q", remote, "."]);
    git(clone, ["checkout", "-q", "-b", "bex-agent/ags-1", latest]);
    await writeFile(path.join(clone, "turn1.txt"), "turn 1\n");
    git(clone, ["add", "turn1.txt"]);
    git(clone, [
      "-c",
      "user.email=a@bex.co",
      "-c",
      "user.name=a",
      "commit",
      "-q",
      "-m",
      "turn 1",
    ]);
    git(clone, ["push", "-q", "origin", "bex-agent/ags-1"]);
    return git(clone, ["rev-parse", "HEAD"]);
  })();
  const config = await sessionConfig({
    cwd: workspace,
    repoUrl: remote,
    prebuilt: "agb-1",
    turn: 2,
  });
  await ensureRepo(config);
  assert.equal(git(workspace, ["rev-parse", "HEAD"]), pushed);
  assert.equal(config.deliveryBaseline?.baseOid, latest);
});

test("a prebuilt session whose workspace is empty falls back to running setup", async () => {
  const root = await mkdtemp(path.join((await tempConfig()).root, "remote-"));
  const remote = await seedBareRemote(root);
  const config = await sessionConfig({ repoUrl: remote, prebuilt: "agb-1" });
  assert.equal(await checkout(config), false);
  const hub = new UIMessageStreamHub();
  const result = await runHeadlessTurn(config, childEnv(config), hub);
  assert.equal(result.state, "succeeded");
  assert.ok(setupCalls(hub).length > 0, "setup did not run");
  assert.equal(
    await readFile(path.join(config.cwd, ".setup-runs"), "utf8"),
    "run\n",
  );
  const overlay = await overlaySent(config.systemPromptLog);
  assert.doesNotMatch(overlay, /Repository setup/);
  assert.match(overlay, /## Repository checks/);
});

test("without BEX_REPO_PREBUILT a pre-cloned checkout is kept and setup runs, as in phase 1", async () => {
  const { workspace, remote, built } = await snapshotImage();
  assert.equal(built.state, "succeeded");
  const head = built.state === "succeeded" ? built.build.head : "";
  const config = await sessionConfig({ cwd: workspace, repoUrl: remote });
  assert.equal(await checkout(config), false);
  // Phase 1 keeps an existing checkout's commits and edits.
  assert.equal(git(workspace, ["rev-parse", "HEAD"]), head);
  assert.match(git(workspace, ["status", "--porcelain"]), /M README\.md/);
  const hub = new UIMessageStreamHub();
  await runHeadlessTurn(config, childEnv(config), hub);
  assert.ok(setupCalls(hub).length > 0, "setup did not run");
  assert.equal(
    await readFile(path.join(workspace, ".setup-runs"), "utf8"),
    "run\nrun\n",
  );
  assert.doesNotMatch(
    await overlaySent(config.systemPromptLog),
    /Repository setup/,
  );
});

test("the overlay lists setup steps as already run only for a snapshot that has some", () => {
  const base = {
    cwd: "/workspace",
    deliver: false,
    repoUrl: "",
    branch: "",
    baseBranch: "",
    repository: "",
  };
  assert.doesNotMatch(
    platformOverlay({ ...base, repoEnvironment: environment }),
    /Repository setup/,
  );
  assert.doesNotMatch(
    platformOverlay({
      ...base,
      setupBaked: true,
      repoEnvironment: { setup: [], checks: environment.checks },
    }),
    /Repository setup/,
  );
  const overlay = platformOverlay({
    ...base,
    setupBaked: true,
    repoEnvironment: {
      setup: [
        { name: "tools", run: "mise install" },
        { name: "deps", run: "npm ci\nnpm run build" },
      ],
      checks: environment.checks,
    },
  });
  assert.match(
    overlay,
    /## Repository setup\n[\s\S]*### tools\n\n```sh\nmise install\n```\n\n### deps\n\n```sh\nnpm ci\nnpm run build\n```\n\n## Repository checks/,
  );
});
