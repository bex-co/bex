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
import test from "node:test";
import {
  renderPreamble,
  resolveContinuity,
} from "../src/continuity.js";

const context = JSON.stringify([
  { turn: 1, prompt: "fix the translation", reply: "I fixed pricing.tier" },
  { turn: 2, prompt: "also check zh", reply: "zh checked" },
]);

test("renderPreamble renders oldest-first with both roles", () => {
  const preamble = renderPreamble(context);
  assert.ok(preamble.startsWith("<session-context>"));
  const first = preamble.indexOf("fix the translation");
  const firstReply = preamble.indexOf("I fixed pricing.tier");
  const second = preamble.indexOf("also check zh");
  assert.ok(first > 0 && firstReply > first && second > firstReply);
  assert.ok(preamble.includes("</session-context>"));
});

test("renderPreamble trims oldest exchanges first under the byte budget", () => {
  const big = JSON.stringify([
    { turn: 1, prompt: "OLD ".repeat(200), reply: "old reply" },
    { turn: 2, prompt: "NEW question", reply: "NEW reply" },
  ]);
  const preamble = renderPreamble(big, 512);
  assert.ok(preamble.includes("NEW question"));
  assert.ok(!preamble.includes("OLD OLD"));
});

test("renderPreamble renders nothing for invalid or empty input", () => {
  assert.equal(renderPreamble(""), "");
  assert.equal(renderPreamble("not json"), "");
  assert.equal(renderPreamble('{"turn":1}'), "");
  // A budget too small for even the newest exchange renders nothing rather
  // than a truncated fragment.
  assert.equal(renderPreamble(context, 8), "");
});

test("session-load wins over any injected material", () => {
  const result = resolveContinuity(
    { contextJson: context, originalTask: "task" },
    "loaded",
    "continue",
  );
  assert.equal(result.rung, "session-load");
  assert.equal(result.prompt, "continue");
});

test("context material re-primes a fresh session and precedes the prompt", () => {
  const result = resolveContinuity(
    { contextJson: context, originalTask: "task" },
    "new",
    "try again",
  );
  assert.equal(result.rung, "transcript-reprime");
  assert.ok(result.prompt.endsWith("try again"));
  assert.ok(result.prompt.indexOf("also check zh") < result.prompt.indexOf("try again"));
});

test("original task is re-delivered when no conversation ever happened", () => {
  const result = resolveContinuity(
    { contextJson: "", originalTask: "fix translation for pricing" },
    "unsupported",
    "try again",
  );
  assert.equal(result.rung, "task-redelivery");
  assert.ok(result.prompt.includes("fix translation for pricing"));
  assert.ok(result.prompt.endsWith("try again"));
});

test("a prompt that IS the task gets no redundant redelivery", () => {
  const result = resolveContinuity(
    { contextJson: "", originalTask: "fix it" },
    "new",
    "fix it",
  );
  assert.equal(result.rung, "none");
  assert.equal(result.prompt, "fix it");
});

test("no material and no load stays a plain cold prompt", () => {
  const result = resolveContinuity({}, "new", "hello");
  assert.equal(result.rung, "none");
  assert.equal(result.prompt, "hello");
});
