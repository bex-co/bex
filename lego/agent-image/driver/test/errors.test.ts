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

import { describeError } from "../src/errors.js";

// The regression this file exists for: the ACP SDK rejects a failed request with
// the raw JSON-RPC error OBJECT, so `String(error)` wrote the literal
// "[object Object]" into the status file, into agent_sessions.failure_reason,
// and onto the dashboard's failure callout.
test("describeError renders a rejected JSON-RPC error, never [object Object]", () => {
  const described = describeError({
    code: -32603,
    message: "Internal error",
    data: { details: "403 model operation is not allowed" },
  });
  assert.ok(!described.includes("[object Object]"), described);
  assert.ok(described.includes("Internal error"), described);
  assert.ok(described.includes("-32603"), described);
  assert.ok(described.includes("403 model operation is not allowed"), described);
});

test("describeError keeps an Error's message", () => {
  assert.equal(describeError(new Error("ACP turn exceeded 1800000ms")), "ACP turn exceeded 1800000ms");
  assert.equal(describeError(new RangeError("")), "RangeError");
});

test("describeError falls back to the object's own fields, then to text", () => {
  assert.equal(describeError({ reason: "sandbox gone" }), '{"reason":"sandbox gone"}');
  assert.equal(describeError("plain failure"), "plain failure");
  assert.equal(describeError(undefined), "unknown error");
  assert.equal(describeError(null), "unknown error");
  assert.equal(describeError(7), "7");
});

test("describeError survives a circular or non-serializable value", () => {
  const circular: Record<string, unknown> = {};
  circular.self = circular;
  assert.equal(
    describeError(circular),
    "unserializable error object (keys: self)",
  );
});

// A hostile or verbose agent must not be able to write an unbounded reason into
// the durable session row through the status file.
test("describeError bounds the description length", () => {
  const described = describeError(new Error("x".repeat(10_000)));
  assert.ok(described.length < 2100, `length ${described.length}`);
  assert.ok(described.endsWith("… (truncated)"), described);
});
