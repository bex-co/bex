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
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import type { CredentialManager } from "../src/credentials.js";
import {
  encodeUIMessageFrame,
  UIMessageStreamHub,
} from "../src/stream-hub.js";
import { TurnLogSink } from "../src/turn-log.js";
import { validateAgentProfiles } from "../src/profiles.js";

const identityCredentialManager = {
  redact: (value: string) => value,
} as CredentialManager;

test("stream hot-path baseline counts bounded encodes and one log open", async () => {
  const hub = new UIMessageStreamHub();
  const parts = representativeStreamParts();
  for (const part of parts) hub.publish(part);
  assert.equal(hub.encodeCount, parts.length);

  const root = await mkdtemp(path.join(tmpdir(), "bex-hotpath-"));
  const logPath = path.join(root, "session.jsonl");
  const sink = new TurnLogSink(
    logPath,
    0,
    16 << 20,
    1,
    identityCredentialManager,
  );
  await sink.open();
  for (const part of parts) {
    await sink.appendPart(part);
  }
  await sink.close();
  assert.equal(sink.openCount, 1);
  assert.equal(sink.writeCount, parts.length);
  const replay = (await readFile(logPath, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line).part);
  assert.deepEqual(replay, parts);
  await rm(root, { recursive: true, force: true });
});

test("stream hub encodes each published part once", () => {
  const hub = new UIMessageStreamHub({
    maxHistoryParts: 2,
    maxHistoryBytes: 1 << 20,
  });
  const parts = representativeStreamParts().slice(0, 3);
  for (const part of parts) hub.publish(part);
  assert.equal(hub.encodeCount, 3);
  assert.equal(encodeUIMessageFrame(parts[1]), encodeUIMessageFrame(parts[1]));
});

test("agent profile manifest validates release-locked adapters", () => {
  assert.doesNotThrow(() =>
    validateAgentProfiles({
      version: 1,
      profiles: [
        {
          id: "claude",
          executable: "/usr/local/bin/claude-code-acp",
          args: [],
          env: {},
          modelEndpoint: "https://api.anthropic.com/v1",
          modelProxy: {
            baseUrlEnv: "ANTHROPIC_BASE_URL",
            baseUrlSuffix: "",
            credentialEnv: "ANTHROPIC_API_KEY",
          },
        },
      ],
    }),
  );
});

function representativeStreamParts(): Record<string, unknown>[] {
  return [
    { type: "start" },
    { type: "text-delta", delta: "hello" },
    { type: "reasoning", text: "thinking" },
    { type: "tool-input-start", toolCallId: "t1", toolName: "bash" },
    { type: "tool-input-delta", toolCallId: "t1", inputTextDelta: "{}" },
    { type: "data-acp-plan", data: { steps: ["inspect"] } },
    { type: "data-acp-diff", data: { path: "a.ts" } },
    { type: "data-acp-terminal", data: { output: "ok" } },
    { type: "finish" },
  ];
}
