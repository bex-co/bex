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
import test from "node:test";

import { SecretRedactor, secretValuesFromEnv } from "../src/redact.js";
import { markTurnFailed } from "../src/session.js";
import { tempConfig } from "./helpers.js";

const secret = "sk-live-abc123XYZ";
const redactor = () => new SecretRedactor(new Map([["STRIPE_KEY", secret]]));

const joinDeltas = (parts: Record<string, unknown>[]): string =>
  parts
    .filter((p) => p.type === "text-delta")
    .map((p) => String(p.delta))
    .join("");

test("secretValuesFromEnv resolves only the named, set env vars", () => {
  const got = secretValuesFromEnv({
    BEX_AGENT_SECRET_NAMES: "A, B,,MISSING",
    A: "alpha-value",
    B: "",
    OTHER: "never-named",
  });
  assert.deepEqual([...got], [["A", "alpha-value"]]);
});

test("tool outputs are scrubbed of the value and its encodings, deeply", () => {
  const r = redactor();
  const b64 = Buffer.from(secret).toString("base64");
  const [out] = r.apply({
    type: "tool-output-available",
    toolCallId: "t1",
    output: {
      stdout: `key=${secret}\nauth=${b64}`,
      url: [`https://x.test/?k=${encodeURIComponent(secret)}`],
    },
  });
  const text = JSON.stringify(out);
  assert.ok(!text.includes(secret), text);
  assert.ok(!text.includes(b64), text);
  assert.match(text, /\[redacted:STRIPE_KEY\]/);
});

test("a value split across deltas never reaches a published part", () => {
  const r = redactor();
  const parts = [
    ...r.apply({ type: "text-start", id: "txt-1" }),
    ...r.apply({ type: "text-delta", id: "txt-1", delta: "the key is sk-li" }),
    ...r.apply({ type: "text-delta", id: "txt-1", delta: "ve-abc1" }),
    ...r.apply({ type: "text-delta", id: "txt-1", delta: "23XYZ, done" }),
    ...r.apply({ type: "text-end", id: "txt-1" }),
  ];
  for (const p of parts)
    assert.ok(!JSON.stringify(p).includes("sk-li"), JSON.stringify(p));
  assert.equal(joinDeltas(parts), "the key is [redacted:STRIPE_KEY], done");
  assert.equal(parts.at(-1)?.type, "text-end");
});

test("a held tail that never completes a value is released unchanged", () => {
  const r = redactor();
  const parts = [
    ...r.apply({ type: "text-delta", id: "txt-1", delta: "prefix sk-" }),
    ...r.apply({ type: "text-delta", id: "txt-1", delta: "other" }),
    ...r.flush(),
  ];
  assert.equal(joinDeltas(parts), "prefix sk-other");
});

test("no secrets is a pass-through and short values are not scrubbed", () => {
  const part = { type: "text-delta", id: "x", delta: "abc" };
  assert.deepEqual(new SecretRedactor(new Map()).apply(part), [part]);
  const short = new SecretRedactor(new Map([["PIN", "abc"]]));
  assert.equal(short.active, false);
  assert.equal(short.redactString("abc"), "abc");
});

test("a part without a secret is published as is, not copied", () => {
  const r = redactor();
  const part = {
    type: "tool-output-available",
    toolCallId: "t1",
    output: { stdout: "all tests passed", files: ["a.go", "b.go"] },
  };
  const [out] = r.apply(part);
  assert.equal(out, part);
  const touched = { ...part, output: { ...part.output, stdout: secret } };
  const [scrubbed] = r.apply(touched);
  assert.notEqual(scrubbed, touched);
  assert.equal(
    (scrubbed!.output as { files: string[] }).files,
    touched.output.files,
    "an unchanged branch is shared, not copied",
  );
});

test("a failure before the turn starts is recorded without the value", async () => {
  const config = await tempConfig({
    secretValues: new Map([["STRIPE_KEY", secret]]),
  });
  await markTurnFailed(config, new Error(`clone failed: token ${secret}`));
  const status = await readFile(config.statusPath, "utf8");
  assert.ok(!status.includes(secret), status);
  assert.match(status, /\[redacted:STRIPE_KEY\]/);
});
