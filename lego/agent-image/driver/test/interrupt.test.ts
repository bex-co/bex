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
import { access, readFile, rm, writeFile } from "node:fs/promises";
import test from "node:test";
import { interruptPath, watchInterrupt } from "../src/interrupt.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

// The fixture agent's prompt takes ACP_FIXTURE_DELAY_MS to answer, so a stop
// request lands while the turn is running.
const slowTurn = { ACP_FIXTURE_DELAY_MS: "20000" };

async function exists(file: string): Promise<boolean> {
  return access(file).then(
    () => true,
    () => false,
  );
}

// requestStop writes the sentinel bex-api writes through the gateway exec path.
async function requestStop(file: string, turn: number): Promise<void> {
  await writeFile(file, String(turn));
}

// slowAgent is the fixture agent environment for a turn that is still inside
// its prompt, so a stop reaches the live agent, until waitForPrompt says so.
function slowAgent(root: string): Record<string, string> {
  return { ...slowTurn, ACP_FIXTURE_PROMPTS_LOG: `${root}/prompts.log` };
}

async function waitForPrompt(root: string): Promise<void> {
  for (let i = 0; i < 500; i++) {
    if (await exists(`${root}/prompts.log`)) return;
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  throw new Error("the agent never received its prompt");
}

test("the sentinel ends only the current turn and the status says interrupted", async () => {
  const config = await tempConfig({ turnTimeoutMs: 60_000 });
  config.agentEnv = slowAgent(config.root);
  const sentinel = interruptPath(config);
  const hub = new UIMessageStreamHub();
  const started = Date.now();
  const turn = runHeadlessTurn(config, childEnv(config), hub, {
    interruptPollMs: 25,
  });
  await waitForPrompt(config.root); // the agent is mid-prompt when the stop arrives
  await requestStop(sentinel, config.turn);
  const result = await turn;
  assert.ok(Date.now() - started < 15_000, "the turn waited out the agent");
  assert.equal(result.state, "interrupted");
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "interrupted");
  assert.equal(status.error, undefined, "a stopped turn is not a failure");
  assert.equal(await exists(sentinel), false, "the sentinel was consumed");
});

test("the driver survives an interrupted turn and a later turn runs", async () => {
  const config = await tempConfig({ turnTimeoutMs: 60_000 });
  config.agentEnv = slowAgent(config.root);
  const sentinel = interruptPath(config);
  const hub = new UIMessageStreamHub();
  const first = runHeadlessTurn(config, childEnv(config), hub, {
    turn: 1,
    closeHub: false,
    interruptPollMs: 25,
  });
  await waitForPrompt(config.root);
  await requestStop(sentinel, 1);
  assert.equal((await first).state, "interrupted");

  // The next turn, in the same process, with an agent that answers at once.
  config.agentEnv = {};
  const second = await runHeadlessTurn(config, childEnv(config), hub, {
    turn: 2,
    prompt: "carry on",
    interruptPollMs: 25,
  });
  assert.equal(second.state, "succeeded");
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "succeeded");
});

test("a sentinel naming another turn, or no turn, never ends this one", async () => {
  const config = await tempConfig({ turnTimeoutMs: 20_000 });
  const sentinel = interruptPath(config);
  await requestStop(sentinel, 7); // meant for turn 7, not turn 1
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
    { interruptPollMs: 25 },
  );
  assert.equal(result.state, "succeeded");
  assert.equal(
    await readFile(sentinel, "utf8"),
    "7",
    "another turn's sentinel was consumed",
  );
  // Neither a number with anything after it: it names no turn.
  for (const junk of ["not a number", "1 then more", "1x"]) {
    await writeFile(sentinel, junk);
    const ignored = await runHeadlessTurn(
      config,
      childEnv(config),
      new UIMessageStreamHub(),
      { interruptPollMs: 25 },
    );
    assert.equal(ignored.state, "succeeded", `${junk} ended a turn`);
  }
  const again = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
    { interruptPollMs: 25 },
  );
  assert.equal(again.state, "succeeded");
});

test("a stop written before the turn starts ends it at once", async () => {
  const config = await tempConfig({
    agentEnv: slowTurn,
    turnTimeoutMs: 60_000,
  });
  await requestStop(interruptPath(config), config.turn);
  const result = await runHeadlessTurn(
    config,
    childEnv(config),
    new UIMessageStreamHub(),
    { interruptPollMs: 25 },
  );
  assert.equal(result.state, "interrupted");
});

test("an interrupted repo turn records what it left undelivered, and never commits", async () => {
  const { ensureRepo } = await import("../src/git-setup.js");
  const { git, seedBareRemote } = await import("./helpers.js");
  const config = await tempConfig({
    deliver: true,
    branch: "bex-agent/ags-test",
    baseBranch: "main",
    turnTimeoutMs: 60_000,
  });
  config.agentEnv = slowAgent(config.root);
  config.repoUrl = await seedBareRemote(config.root);
  await ensureRepo(config);
  const hub = new UIMessageStreamHub();
  const turn = runHeadlessTurn(config, childEnv(config), hub, {
    interruptPollMs: 25,
  });
  await waitForPrompt(config.root);
  await writeFile(`${config.cwd}/half-done.txt`, "work in progress\n");
  await requestStop(interruptPath(config), config.turn);
  const result = await turn;
  assert.equal(result.state, "interrupted");
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "interrupted");
  assert.equal(status.git.dirty, true);
  assert.match(status.git.undelivered.join(" "), /uncommitted changes/);
  // The driver stops the agent; it does not commit or push for it.
  assert.match(git(config.cwd, ["status", "--porcelain"]), /half-done\.txt/);
  assert.equal(
    git(config.cwd, ["rev-parse", "HEAD"]),
    git(config.cwd, ["rev-parse", "origin/main"]),
  );
});

test("a stopped watch never fires, and the sentinel is read only for its turn", async () => {
  const config = await tempConfig();
  const sentinel = interruptPath(config);
  let fired = 0;
  const stop = watchInterrupt(sentinel, 3, () => (fired += 1), 10);
  stop();
  await requestStop(sentinel, 3);
  await new Promise((resolve) => setTimeout(resolve, 80));
  assert.equal(fired, 0, "a stopped watch fired");
  assert.equal(await readFile(sentinel, "utf8"), "3");

  const stopLater = watchInterrupt(sentinel, 3, () => (fired += 1), 10);
  await new Promise((resolve) => setTimeout(resolve, 80));
  stopLater();
  assert.equal(fired, 1, "a watch fires once for its turn");
  assert.equal(await exists(sentinel), false);
});

test("a stop that lands while the matched sentinel is being removed never fires", async () => {
  const config = await tempConfig();
  const sentinel = interruptPath(config);
  await requestStop(sentinel, 3);
  let fired = 0;
  let stop: () => void = () => undefined;
  // The turn ends (its watch is stopped) after the poll matched the sentinel
  // and before it finished removing it: the abort must not follow.
  const removed = new Promise<void>((resolve) => {
    stop = watchInterrupt(
      sentinel,
      3,
      () => (fired += 1),
      10,
      async (file) => {
        stop();
        await rm(file, { force: true });
        resolve();
      },
    );
  });
  await removed;
  await new Promise((resolve) => setTimeout(resolve, 80));
  assert.equal(fired, 0, "an abort landed after the turn ended");
  assert.equal(await exists(sentinel), false, "the sentinel was consumed");
});
