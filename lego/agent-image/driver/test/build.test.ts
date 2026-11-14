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

// Snapshot builds (ADR111 D1, D2, D5): the driver's build mode clones the
// default branch, runs the setup steps, scrubs its per-run state and writes
// the build's status, with no agent.

import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
  BUILD_STEP_TIMEOUT_MS,
  runBuild,
  type BuildOptions,
} from "../src/build.js";
import {
  driverMode,
  loadBuildConfig,
  type BuildConfig,
} from "../src/config.js";
import type { RepoStep } from "../src/repo-setup.js";
import { git, seedBareRemote, tempConfig } from "./helpers.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const mainScript = path.join(here, "..", "src", "main.ts");
const secret = "sk_live_BUILD-REDACTION-0123456789";

async function buildFixture(setup: RepoStep[]) {
  const { root, cwd } = await tempConfig();
  const remote = await seedBareRemote(root);
  const config: BuildConfig = {
    cwd,
    repoUrl: remote,
    repository: "acme/widgets",
    repoEnvironment: { setup, checks: [] },
    statusPath: path.join(root, "run", "status.json"),
    sessionLogPath: path.join(root, "log", "session.jsonl"),
    prRecordPath: path.join(root, "run", "pull-request.json"),
    resultRecordPath: path.join(root, "run", "result.json"),
    secretValues: new Map([["BUILD_TOKEN", secret]]),
  };
  return { root, remote, config };
}

function stepEnv(): Record<string, string> {
  return {
    PATH: process.env.PATH ?? "",
    HOME: process.env.HOME ?? "",
    BUILD_TOKEN: secret,
  };
}

async function build(config: BuildConfig, options: BuildOptions = {}) {
  const status = await runBuild(config, {
    env: stepEnv(),
    cloneRetry: { attempts: 1 },
    ...options,
  });
  assert.deepEqual(
    JSON.parse(await readFile(config.statusPath, "utf8")),
    JSON.parse(JSON.stringify(status)),
    "the returned status is not the one written",
  );
  return status;
}

async function waitForFile(file: string, ms = 30_000): Promise<string> {
  const deadline = Date.now() + ms;
  for (;;) {
    const contents = await readFile(file, "utf8").catch(() => "");
    if (contents) return contents;
    if (Date.now() > deadline) throw new Error(`${file} never appeared`);
    await sleep(50);
  }
}

test("a build step's limit is Devin's 3 hours", () => {
  assert.equal(BUILD_STEP_TIMEOUT_MS, 3 * 60 * 60 * 1000);
});

test("build mode is read from BEX_AGENT_MODE, and needs no model proxy or agent", () => {
  assert.equal(driverMode({}), "session");
  assert.equal(driverMode({ BEX_AGENT_MODE: "build" }), "build");
  assert.throws(() => driverMode({ BEX_AGENT_MODE: "nope" }), /BEX_AGENT_MODE/);
  const config = loadBuildConfig({
    BEX_AGENT_MODE: "build",
    BEX_AGENT_REPO_URL: "http://gw/git/build",
    BEX_AGENT_REPOSITORY: "acme/widgets",
    BEX_REPO_ENVIRONMENT: '{"setup":[{"name":"deps","run":"npm ci"}]}',
    BEX_AGENT_SECRET_NAMES: "BUILD_TOKEN",
    BUILD_TOKEN: secret,
  });
  assert.equal(config.cwd, "/workspace");
  assert.equal(config.statusPath, "/var/run/bex-agent/status.json");
  assert.equal(config.sessionLogPath, "/var/log/bex-agent/session.jsonl");
  assert.equal(config.repository, "acme/widgets");
  assert.deepEqual(config.repoEnvironment?.setup, [
    { name: "deps", run: "npm ci" },
  ]);
  assert.equal(config.secretValues.get("BUILD_TOKEN"), secret);
});

test("a build clones the default branch, runs setup, scrubs its run state and idles without an agent", async (t) => {
  const { root, remote } = await buildFixture([]);
  // Only the default branch reaches the image, without tags.
  git(remote, ["branch", "feature", "main"]);
  git(remote, ["tag", "v1", "main"]);
  const cwd = path.join(root, "workspace");
  const statusPath = path.join(root, "run", "status.json");
  const sessionLog = path.join(root, "log", "session.jsonl");
  // A driver log left in the sandbox: the image must not carry it.
  await mkdir(path.dirname(sessionLog), { recursive: true });
  await writeFile(sessionLog, '{"turn":1}\n');
  const environment = {
    setup: [
      {
        name: "deps",
        // A file the step writes stays in the image; credential config a
        // step sets does not.
        run: [
          "mkdir -p node_modules/dep && echo installed > node_modules/dep/index.js",
          'echo "token=$BUILD_TOKEN"',
          "git config credential.helper store",
          "git config http.https://github.com/.extraheader 'AUTHORIZATION: basic eA=='",
          "git config core.editor true",
        ].join(" && "),
      },
      { name: "big", run: "head -c 5000 /dev/zero | tr '\\0' a" },
    ],
    checks: [{ name: "test", run: "npm test" }],
  };
  const child = spawn(process.execPath, ["--import", "tsx", mainScript], {
    cwd: path.join(here, ".."),
    env: {
      PATH: process.env.PATH,
      HOME: process.env.HOME,
      BEX_AGENT_MODE: "build",
      BEX_AGENT_CWD: cwd,
      BEX_AGENT_REPO_URL: remote,
      BEX_AGENT_REPOSITORY: "acme/widgets",
      BEX_REPO_ENVIRONMENT: JSON.stringify(environment),
      BEX_AGENT_STATUS_FILE: statusPath,
      BEX_AGENT_SESSION_LOG: sessionLog,
      BEX_AGENT_SECRET_NAMES: "BUILD_TOKEN",
      BUILD_TOKEN: secret,
    },
    stdio: ["ignore", "ignore", "pipe"],
  });
  let stderr = "";
  child.stderr.on("data", (chunk) => (stderr += chunk));
  const exited = new Promise<number | null>((resolve) =>
    child.once("exit", (code) => resolve(code)),
  );
  t.after(() => child.kill("SIGKILL"));

  const status = JSON.parse(await waitForFile(statusPath));
  assert.equal(status.state, "succeeded", stderr);
  assert.deepEqual(Object.keys(status).sort(), ["build", "state"]);
  assert.deepEqual(Object.keys(status.build).sort(), ["head", "steps"]);
  assert.equal(status.build.head, git(remote, ["rev-parse", "main"]));
  assert.deepEqual(
    status.build.steps.map((s: Record<string, unknown>) => [
      s.name,
      s.exitCode,
    ]),
    [
      ["deps", 0],
      ["big", 0],
    ],
  );
  for (const step of status.build.steps) {
    assert.deepEqual(Object.keys(step).sort(), [
      "durationMs",
      "exitCode",
      "name",
      "output",
    ]);
    assert.ok(Number.isInteger(step.durationMs) && step.durationMs >= 0);
  }
  assert.ok(!JSON.stringify(status).includes(secret), "output leaked");
  assert.match(status.build.steps[0].output, /token=\[redacted:BUILD_TOKEN\]/);
  // A passing step keeps a 2 KiB tail.
  assert.equal(status.build.steps[1].output, "a".repeat(2048));

  // On the default branch, with what setup wrote, without per-run Git state.
  assert.equal(git(cwd, ["rev-parse", "--abbrev-ref", "HEAD"]), "main");
  assert.doesNotMatch(git(cwd, ["branch", "-r"]), /feature/);
  assert.equal(git(cwd, ["tag"]), "");
  assert.equal(
    git(cwd, ["config", "remote.origin.fetch"]),
    "+refs/heads/*:refs/remotes/origin/*",
  );
  assert.equal(
    await readFile(path.join(cwd, "node_modules/dep/index.js"), "utf8"),
    "installed\n",
  );
  assert.equal(
    git(cwd, ["remote", "get-url", "origin"]),
    "https://github.com/acme/widgets.git",
  );
  const gitConfig = await readFile(path.join(cwd, ".git", "config"), "utf8");
  assert.doesNotMatch(gitConfig, /credential|extraheader|AUTHORIZATION/i);
  assert.doesNotMatch(gitConfig, new RegExp(remote));
  assert.match(gitConfig, /editor = true/, "the step's own config is kept");
  await assert.rejects(readFile(sessionLog), "the driver log was kept");

  // No agent: the process has no child (tsx's own esbuild service aside), and
  // it stays up for bex-api's read, pause and terminate.
  await sleep(300);
  assert.equal(child.exitCode, null, "the build exited instead of idling");
  const children = spawnSync("pgrep", ["-lf", "-P", String(child.pid)])
    .stdout.toString()
    .split("\n")
    .filter((line) => line.trim() && !/\besbuild\b/.test(line));
  assert.deepEqual(children, [], "a child is running");
  child.kill("SIGTERM");
  assert.equal(await exited, 0);
});

test("a step that exits non-zero fails the build setup_failed, naming it, with the steps so far", async () => {
  const { config } = await buildFixture([
    { name: "ok", run: "head -c 3000 /dev/zero | tr '\\0' b" },
    {
      name: "broken",
      run: `echo "token=$BUILD_TOKEN"; head -c 10000 /dev/zero | tr '\\0' c; exit 3`,
    },
    { name: "never", run: "touch never-ran" },
  ]);
  const status = await build(config);
  assert.equal(status.state, "failed");
  if (status.state !== "failed") return;
  assert.equal(status.code, "setup_failed");
  assert.match(status.error, /setup step "broken" exited with code 3/);
  assert.deepEqual(
    status.build.steps.map((s) => [s.name, s.exitCode]),
    [
      ["ok", 0],
      ["broken", 3],
    ],
  );
  assert.equal(status.build.steps[0].output, "b".repeat(2048));
  // The failing step keeps an 8 KiB tail.
  assert.equal(status.build.steps[1].output, "c".repeat(8192));
  assert.ok(!("head" in status.build));
  await assert.rejects(readFile(path.join(config.cwd, "never-ran")));
});

test("a failing step's output is redacted", async () => {
  const { config } = await buildFixture([
    { name: "leak", run: 'echo "token=$BUILD_TOKEN"; exit 1' },
  ]);
  const status = await build(config);
  assert.equal(status.state, "failed");
  const written = await readFile(config.statusPath, "utf8");
  assert.ok(!written.includes(secret));
  assert.match(written, /token=\[redacted:BUILD_TOKEN\]/);
});

test("a step past its own limit fails the build step_timeout", async () => {
  const { config } = await buildFixture([
    { name: "slow", run: "sleep 30 & sleep 30; wait" },
  ]);
  const started = Date.now();
  const status = await build(config, { stepTimeoutMs: 300 });
  assert.ok(Date.now() - started < 10_000, "the step was not killed");
  assert.equal(status.state, "failed");
  if (status.state !== "failed") return;
  assert.equal(status.code, "step_timeout");
  assert.match(status.error, /setup step "slow" ran past its .* limit/);
  assert.deepEqual(
    status.build.steps.map((s) => [s.name, s.exitCode]),
    [["slow", null]],
  );
});

// bex-api enforces the whole build's limit; the driver limits each step only.
test("steps longer together than one step's limit pass", async () => {
  const { config } = await buildFixture([
    { name: "a", run: "sleep 0.6" },
    { name: "b", run: "sleep 0.6" },
  ]);
  const status = await build(config, { stepTimeoutMs: 1_000 });
  assert.equal(status.state, "succeeded");
});

test("a clone that fails, or no repository to name origin, fails checkout_failed", async () => {
  const { root, config } = await buildFixture([
    { name: "deps", run: "touch ran" },
  ]);
  // The reason is redacted like step output.
  const missing = path.join(root, `missing-${secret}.git`);
  const status = await build({ ...config, repoUrl: missing });
  assert.equal(status.state, "failed");
  if (status.state !== "failed") return;
  assert.equal(status.code, "checkout_failed");
  assert.match(status.error, /could not clone the repository/);
  assert.ok(!status.error.includes(secret));
  assert.match(status.error, /\[redacted:BUILD_TOKEN\]/);
  assert.deepEqual(status.build.steps, []);
  await assert.rejects(readFile(path.join(config.cwd, "ran")));

  for (const repository of ["", "acme", "acme/widgets/extra"]) {
    const { config: fresh } = await buildFixture([]);
    const refused = await build({ ...fresh, repository });
    assert.equal(refused.state, "failed");
    if (refused.state === "failed") {
      assert.equal(refused.code, "checkout_failed");
      assert.match(refused.error, /BEX_AGENT_REPOSITORY/);
    }
  }
  const { config: noUrl } = await buildFixture([]);
  const unset = await build({ ...noUrl, repoUrl: "" });
  assert.equal(unset.state === "failed" && unset.code, "checkout_failed");
});
