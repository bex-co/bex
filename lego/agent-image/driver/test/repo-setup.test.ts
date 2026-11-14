/*
 * Copyright 2026.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { platformOverlay } from "../src/overlay.js";
import {
  parseRepoEnvironment,
  RepoSetupError,
  runRepoSetup,
  SETUP_FAILED_CODE,
  type RepoEnvironment,
} from "../src/repo-setup.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

type Chunk = Record<string, unknown>;

async function run(
  environment: RepoEnvironment,
  cwd: string,
  options: { stepTimeoutMs?: number; totalTimeoutMs?: number } = {},
) {
  const chunks: Chunk[] = [];
  let beats = 0;
  let error: unknown;
  try {
    await runRepoSetup(environment, {
      cwd,
      env: { PATH: process.env.PATH ?? "", SETUP_VALUE: "from-env" },
      write: (chunk) => chunks.push(chunk),
      heartbeat: () => beats++,
      ...options,
    });
  } catch (caught) {
    error = caught;
  }
  return { chunks, beats, error };
}

test("parseRepoEnvironment reads the dispatch value and ignores what does not parse", () => {
  assert.equal(parseRepoEnvironment(undefined), undefined);
  assert.equal(parseRepoEnvironment("{"), undefined);
  assert.equal(parseRepoEnvironment('{"setup":[],"checks":[]}'), undefined);
  assert.deepEqual(
    parseRepoEnvironment(
      '{"setup":[{"name":"deps","run":"npm ci"},{"name":1}],"checks":[{"name":"test","run":"npm test"}]}',
    ),
    {
      setup: [{ name: "deps", run: "npm ci" }],
      checks: [{ name: "test", run: "npm test" }],
    },
  );
});

test("setup steps run in order from the repository root as setup tool calls", async () => {
  const config = await tempConfig();
  await writeFile(path.join(config.cwd, "marker"), "root");
  const { chunks, error } = await run(
    {
      setup: [
        { name: "first", run: 'cat marker; echo " $SETUP_VALUE" > out' },
        { name: "second", run: "cat out" },
      ],
      checks: [],
    },
    config.cwd,
  );
  assert.equal(error, undefined);
  assert.deepEqual(
    chunks.map((c) => `${c.type}:${c.toolCallId}`),
    [
      "tool-input-start:bex-setup-1",
      "tool-input-available:bex-setup-1",
      "tool-output-available:bex-setup-1",
      "tool-input-start:bex-setup-2",
      "tool-input-available:bex-setup-2",
      "tool-output-available:bex-setup-2",
    ],
  );
  assert.equal(chunks[0].toolName, "setup: first");
  assert.equal(chunks[0].title, "setup: first");
  assert.deepEqual(chunks[1].input, {
    step: "first",
    command: 'cat marker; echo " $SETUP_VALUE" > out',
  });
  const first = String(chunks[2].output);
  assert.match(
    first,
    /^exited 0 after \d+\.\ds\nroot\n?$/,
    "the step did not run in the repository root",
  );
  assert.match(
    String(chunks[5].output),
    /\n ?from-env\n?$/,
    "steps do not share the root or the env",
  );
});

test("a failing step names itself, stops setup and carries the setup failure code", async () => {
  const config = await tempConfig();
  const { chunks, error } = await run(
    {
      setup: [
        { name: "broken", run: "echo partial; exit 3" },
        { name: "never", run: "touch never-ran" },
      ],
      checks: [],
    },
    config.cwd,
  );
  assert.ok(error instanceof RepoSetupError);
  assert.equal(error.code, SETUP_FAILED_CODE);
  assert.match(error.message, /setup step "broken" exited with code 3/);
  const failed = chunks.find((c) => c.type === "tool-output-error");
  assert.match(String(failed?.errorText), /exited with code 3\npartial/);
  assert.ok(!chunks.some((c) => c.toolCallId === "bex-setup-2"));
  await assert.rejects(readFile(path.join(config.cwd, "never-ran")));
});

test("a step past its limit is killed with everything it started", async () => {
  const config = await tempConfig();
  const started = Date.now();
  const { error, beats } = await run(
    {
      setup: [{ name: "slow", run: "sleep 30 & sleep 30; wait" }],
      checks: [],
    },
    config.cwd,
    { stepTimeoutMs: 300 },
  );
  assert.ok(error instanceof RepoSetupError);
  assert.match(error.message, /"slow" ran past its .* limit/);
  assert.ok(Date.now() - started < 10_000, "the step was not killed");
  assert.ok(beats >= 0);
});

test("setup as a whole stops at its total limit", async () => {
  const config = await tempConfig();
  const { error } = await run(
    {
      setup: [
        { name: "a", run: "sleep 1" },
        { name: "b", run: "sleep 30" },
      ],
      checks: [],
    },
    config.cwd,
    { stepTimeoutMs: 60_000, totalTimeoutMs: 1_500 },
  );
  assert.ok(error instanceof RepoSetupError);
  assert.match(error.message, /"b" ran past setup's total limit/);
});

test("the overlay names the repository's checks, fenced, and only when there are some", () => {
  const base = {
    cwd: "/workspace",
    deliver: false,
    repoUrl: "",
    branch: "",
    baseBranch: "",
    repository: "",
  };
  assert.doesNotMatch(platformOverlay(base), /Repository checks/);
  assert.doesNotMatch(
    platformOverlay({ ...base, repoEnvironment: { setup: [], checks: [] } }),
    /Repository checks/,
  );
  const overlay = platformOverlay({
    ...base,
    repoEnvironment: {
      setup: [],
      checks: [
        { name: "lint", run: "npm run lint" },
        { name: "test", run: "go vet ./...\ngo test ./..." },
      ],
    },
  });
  assert.match(overlay, /## Repository checks/);
  assert.match(overlay, /### lint\n\n```sh\nnpm run lint\n```/);
  assert.match(
    overlay,
    /### test\n\n```sh\ngo vet \.\/\.\.\.\ngo test \.\/\.\.\.\n```/,
  );
});

test("a turn runs setup before the agent, redacts its output, and fails without the agent on a broken step", async (t) => {
  const secret = "sk_live_SETUP-REDACTION-0123456789";
  const config = await tempConfig({
    secretValues: new Map([["SETUP_TOKEN", secret]]),
    repoEnvironment: {
      setup: [
        {
          name: "print",
          run: 'echo "token=$SETUP_TOKEN"; test -n "$HOME"; command -v bash >/dev/null',
        },
      ],
      checks: [],
    },
  });
  const okLog = path.join(config.root, "agent.log");
  // In a sandbox the session's secrets are the driver's own env (ADR098 D5).
  process.env.SETUP_TOKEN = secret;
  t.after(() => delete process.env.SETUP_TOKEN);
  config.agentEnv = { ACP_FIXTURE_LOG: okLog };
  const hub = new UIMessageStreamHub();
  const result = await runHeadlessTurn(config, childEnv(config), hub);
  assert.equal(result.state, "succeeded");
  const history = JSON.stringify(hub.history);
  assert.ok(!history.includes(secret), "setup output leaked the value");
  assert.match(history, /token=\[redacted:SETUP_TOKEN\]/);
  const firstSetup = hub.history.findIndex(
    (part) => part.toolCallId === "bex-setup-1",
  );
  const firstText = hub.history.findIndex((part) => part.type === "text-delta");
  assert.ok(
    firstSetup >= 0 && firstSetup < firstText,
    "setup did not run first",
  );
  assert.match(
    await readFile(okLog, "utf8"),
    /prompt/,
    "the agent did not run after setup",
  );
  const log = await readFile(config.sessionLogPath, "utf8");
  assert.ok(log.includes("bex-setup-1") && !log.includes(secret));

  // A second turn in the same sandbox does not run setup again.
  const hub2 = new UIMessageStreamHub();
  await runHeadlessTurn(config, childEnv(config), hub2, { turn: 2 });
  assert.ok(!hub2.history.some((part) => part.toolCallId === "bex-setup-1"));

  // A broken step fails the turn with the code, and the agent never runs.
  const broken = await tempConfig({
    repoEnvironment: { setup: [{ name: "deps", run: "exit 7" }], checks: [] },
  });
  const agentLog = path.join(broken.root, "agent.log");
  broken.agentEnv = { ACP_FIXTURE_LOG: agentLog };
  await assert.rejects(
    runHeadlessTurn(broken, childEnv(broken), new UIMessageStreamHub()),
    /setup step "deps" exited with code 7/,
  );
  const status = JSON.parse(await readFile(broken.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.equal(status.code, SETUP_FAILED_CODE);
  assert.match(status.error, /"deps" exited with code 7/);
  await assert.rejects(
    readFile(agentLog),
    "the agent started after a failed setup",
  );
});
