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
import { readFile } from "node:fs/promises";
import test, { type TestContext } from "node:test";
import type { SessionUpdate } from "@agentclientprotocol/sdk/unstable";
import { loadConfig, maxTurnTimeoutMs } from "../src/config.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { TurnBound } from "../src/turn-bound.js";
import { childEnv, tempConfig } from "./helpers.js";

const minute = 60 * 1000;
const idleMs = 30 * minute;
const toolCallMs = 3 * 60 * minute;
const ceilingMs = 4 * 60 * minute;

const chunk = {
  sessionUpdate: "agent_message_chunk",
  content: { type: "text", text: "still working" },
} as SessionUpdate;

function toolCall(
  status: string,
  update = false,
  id = "build-1",
): SessionUpdate {
  return {
    sessionUpdate: update ? "tool_call_update" : "tool_call",
    toolCallId: id,
    title: "npm test",
    status,
  } as SessionUpdate;
}

// boundUnderFakeTimers starts a TurnBound on mocked setTimeout and Date and
// returns it with the errors it expired with.
function boundUnderFakeTimers(
  t: TestContext,
  options: Partial<{
    idleMs: number;
    toolCallMs: number;
    ceilingMs: number;
  }> = {},
) {
  t.mock.timers.enable({ apis: ["setTimeout", "Date"], now: 0 });
  const expired: Error[] = [];
  const bound = new TurnBound({
    idleMs,
    toolCallMs,
    ceilingMs,
    ...options,
    onExpire: (error) => expired.push(error),
  });
  t.after(() => bound.stop());
  return { bound, expired };
}

test("updates every 10 minutes survive well past the idle bound", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t);
  for (let elapsed = 0; elapsed < 3 * 60 * minute; elapsed += 10 * minute) {
    t.mock.timers.tick(10 * minute);
    bound.update(chunk);
  }
  assert.deepEqual(expired, [], "a turn reporting progress was cut");
});

test("a silent agent fails at the idle bound", (t) => {
  const { expired } = boundUnderFakeTimers(t);
  t.mock.timers.tick(idleMs - 1);
  assert.deepEqual(expired, []);
  t.mock.timers.tick(1);
  assert.equal(expired.length, 1);
  assert.match(expired[0].message, /ACP turn made no progress for 1800000ms/);
});

test("the idle bound counts from the last progress, not from the start", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t);
  t.mock.timers.tick(20 * minute);
  bound.progress();
  t.mock.timers.tick(idleMs - 1);
  assert.deepEqual(expired, []);
  t.mock.timers.tick(1);
  assert.equal(expired.length, 1);
});

test("a long tool call is not cut early, and the idle clock resumes after it", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t);
  t.mock.timers.tick(5 * minute);
  bound.update(toolCall("pending"));
  bound.update(toolCall("in_progress", true));
  t.mock.timers.tick(2 * 60 * minute); // a silent two-hour build
  assert.deepEqual(
    expired,
    [],
    "a running tool call was cut by the idle bound",
  );
  bound.update(toolCall("completed", true));
  t.mock.timers.tick(idleMs - 1);
  assert.deepEqual(expired, []);
  t.mock.timers.tick(1);
  assert.equal(expired.length, 1);
  assert.match(expired[0].message, /made no progress/);
});

test("a hung tool call fails at the tool-call cap, not the ceiling", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t);
  t.mock.timers.tick(10 * minute);
  bound.update(toolCall("in_progress"));
  t.mock.timers.tick(30 * minute);
  bound.update(toolCall("in_progress", true)); // output streaming in
  t.mock.timers.tick(toolCallMs - 30 * minute - 1);
  assert.deepEqual(expired, [], "the cap counted from the last update");
  t.mock.timers.tick(1);
  assert.equal(expired.length, 1);
  assert.match(expired[0].message, /ACP tool call ran longer than 10800000ms/);
});

test("the cap applies to each tool call from its own start", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t, {
    ceilingMs: 8 * 60 * minute,
  });
  bound.update(toolCall("in_progress", false, "a"));
  t.mock.timers.tick(60 * minute);
  bound.update(toolCall("in_progress", false, "b"));
  t.mock.timers.tick(60 * minute);
  bound.update(toolCall("completed", true, "a")); // b has run 1h of its 3h
  t.mock.timers.tick(2 * 60 * minute - 1);
  assert.deepEqual(expired, []);
  t.mock.timers.tick(1);
  assert.equal(expired.length, 1);
  assert.match(expired[0].message, /tool call ran longer/);
});

test("a prompt that ends settles any tool call the agent left running", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t);
  bound.update(toolCall("in_progress"));
  bound.promptSettled();
  t.mock.timers.tick(idleMs);
  assert.equal(expired.length, 1);
});

test("the ceiling still fires on a turn that keeps making progress", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t, {
    idleMs,
    ceilingMs: 60 * minute,
  });
  for (let elapsed = minute; elapsed < 60 * minute; elapsed += minute) {
    t.mock.timers.tick(minute);
    bound.update(chunk);
  }
  assert.deepEqual(expired, []);
  t.mock.timers.tick(minute);
  assert.equal(expired.length, 1);
  assert.match(expired[0].message, /ACP turn exceeded 3600000ms/);
  t.mock.timers.tick(2 * idleMs);
  assert.equal(expired.length, 1, "a bound expired twice");
});

test("the ceiling still fires during a tool call", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t, {
    idleMs,
    ceilingMs: 60 * minute,
  });
  bound.update(toolCall("in_progress"));
  t.mock.timers.tick(60 * minute);
  assert.equal(expired.length, 1);
  assert.match(expired[0].message, /exceeded/);
});

test("a stopped bound never expires", (t) => {
  const { bound, expired } = boundUnderFakeTimers(t);
  bound.stop();
  t.mock.timers.tick(ceilingMs);
  assert.deepEqual(expired, []);
});

test("the bounds default sanely, stay under the ceiling, and the ceiling under the sandbox's life", () => {
  const hosted = {
    BEX_AGENT_MODEL_PROXY_URL:
      "http://bex-ssh-gateway.bex-system.svc.cluster.local:8084/model/ns/ags-1",
    BEX_AGENT_SESSION_ID: "ags-1",
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-1",
  };
  const unset = loadConfig(hosted);
  assert.deepEqual(
    [unset.turnTimeoutMs, unset.turnIdleTimeoutMs, unset.toolCallTimeoutMs],
    [30 * minute, 30 * minute, 30 * minute],
  );
  const set = loadConfig({
    ...hosted,
    BEX_AGENT_TURN_IDLE_TIMEOUT_MS: String(20 * minute),
    BEX_AGENT_TOOL_CALL_TIMEOUT_MS: String(90 * minute),
    BEX_AGENT_TURN_TIMEOUT_MS: String(48 * 60 * minute),
  });
  assert.deepEqual(
    [set.turnTimeoutMs, set.turnIdleTimeoutMs, set.toolCallTimeoutMs],
    [maxTurnTimeoutMs, 20 * minute, 90 * minute],
  );
  // A bound above the ceiling (or past setTimeout's 2^31-1 ms) is the ceiling.
  const over = loadConfig({
    ...hosted,
    BEX_AGENT_TURN_TIMEOUT_MS: String(60 * minute),
    BEX_AGENT_TURN_IDLE_TIMEOUT_MS: String(30 * 24 * 60 * minute),
    BEX_AGENT_TOOL_CALL_TIMEOUT_MS: String(5 * 60 * minute),
  });
  assert.deepEqual(
    [over.turnTimeoutMs, over.turnIdleTimeoutMs, over.toolCallTimeoutMs],
    [60 * minute, 60 * minute, 60 * minute],
  );
  assert.throws(
    () => loadConfig({ ...hosted, BEX_AGENT_TURN_IDLE_TIMEOUT_MS: "soon" }),
    /BEX_AGENT_TURN_IDLE_TIMEOUT_MS must be a positive integer/,
  );
});

test("a turn whose agent goes silent fails at the idle bound, not the ceiling", async () => {
  const config = await tempConfig({
    agentEnv: { ACP_FIXTURE_DELAY_MS: "10000" },
    turnTimeoutMs: 20_000,
    turnIdleTimeoutMs: 200,
  });
  const started = Date.now();
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
    /ACP turn made no progress for 200ms/,
  );
  assert.ok(Date.now() - started < 10_000, "the turn waited for the ceiling");
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.match(status.error, /made no progress/);
});
