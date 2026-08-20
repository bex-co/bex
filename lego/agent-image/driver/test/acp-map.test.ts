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
import type { SessionUpdate } from "@agentclientprotocol/sdk";
import { createUpdateMapper } from "../src/acp-map.js";
import {
  existingSourceTime,
  isUtcTimestamp,
  stampSourceTimestamp,
} from "../src/timestamp.js";

function clock(start = "2026-08-19T00:00:00.000Z", stepMs = 1000): () => string {
  let ms = Date.parse(start);
  return () => {
    const iso = new Date(ms).toISOString();
    ms += stepMs;
    return iso;
  };
}

function atOf(part: Record<string, unknown>): string {
  assert.ok(isUtcTimestamp(part.at), `missing at on ${String(part.type)}`);
  return part.at;
}

test("stampSourceTimestamp assigns ISO-8601 UTC once and preserves replay", () => {
  const first = stampSourceTimestamp(
    { type: "data-acp-plan", data: {} },
    "2026-08-19T00:00:00.000Z",
  );
  assert.equal(first.at, "2026-08-19T00:00:00.000Z");
  assert.equal(
    (first.providerMetadata as { bex: { at: string } }).bex.at,
    "2026-08-19T00:00:00.000Z",
  );
  const replayed = stampSourceTimestamp(first, "2026-08-19T00:00:40.000Z");
  assert.equal(replayed.at, first.at);
  assert.equal(existingSourceTime(replayed), first.at);
});

test("stampSourceTimestamp rejects invalid optional timing without dropping the part", () => {
  const stamped = stampSourceTimestamp(
    { type: "text-delta", id: "t", delta: "hi", at: "not-a-time" },
    "2026-08-19T00:00:00.000Z",
  );
  assert.equal(stamped.at, "2026-08-19T00:00:00.000Z");
  assert.equal(stamped.delta, "hi");
  assert.equal(stamped.providerMetadata, undefined);
});

test("mapper stamps text, reasoning, tool, plan, diff, and terminal parts", () => {
  const now = clock();
  const mapper = createUpdateMapper({ now });
  const updates: SessionUpdate[] = [
    {
      sessionUpdate: "agent_thought_chunk",
      content: { type: "text", text: "thinking" },
    },
    {
      sessionUpdate: "plan",
      entries: [{ content: "step", status: "pending", priority: "medium" }],
    },
    {
      sessionUpdate: "tool_call",
      toolCallId: "t1",
      title: "Edit fixture",
      kind: "edit",
      status: "in_progress",
      rawInput: { path: "a.txt" },
    },
    {
      sessionUpdate: "tool_call_update",
      toolCallId: "t1",
      status: "in_progress",
      content: [
        { type: "diff", path: "a.txt", oldText: "", newText: "x\n" },
        { type: "terminal", terminalId: "term-1", output: "ok" },
      ],
    },
    {
      sessionUpdate: "agent_message_chunk",
      content: { type: "text", text: "done" },
    },
    {
      sessionUpdate: "tool_call_update",
      toolCallId: "t1",
      status: "completed",
      rawOutput: { ok: true },
    },
  ];
  const chunks = updates.flatMap((u) => mapper.map(u));
  chunks.push(...mapper.flush());

  const byType = Object.fromEntries(
    ["reasoning-start", "data-acp-plan", "tool-input-start", "data-acp-diff", "data-acp-terminal", "text-start"].map(
      (type) => [type, chunks.find((c) => c.type === type)],
    ),
  );
  for (const [type, chunk] of Object.entries(byType)) {
    assert.ok(chunk, `missing ${type}`);
    assert.ok(isUtcTimestamp((chunk as { at?: unknown }).at), `${type} has no at`);
  }

  const reasoningEnd = chunks.find((c) => c.type === "reasoning-end") as {
    at?: string;
    providerMetadata?: { bex?: { at?: string; endAt?: string } };
  };
  assert.ok(reasoningEnd);
  assert.equal(reasoningEnd.providerMetadata?.bex?.at, atOf(byType["reasoning-start"] as Record<string, unknown>));
  assert.ok(isUtcTimestamp(reasoningEnd.providerMetadata?.bex?.endAt));
  assert.notEqual(
    reasoningEnd.providerMetadata?.bex?.at,
    reasoningEnd.providerMetadata?.bex?.endAt,
  );

  const restamped = chunks.map((c) =>
    stampSourceTimestamp(
      { ...(c as Record<string, unknown>) },
      "2099-01-01T00:00:00.000Z",
    ),
  );
  for (let i = 0; i < chunks.length; i++) {
    assert.equal(
      (restamped[i] as { at: string }).at,
      (chunks[i] as { at: string }).at,
      `replay changed ${chunks[i].type}`,
    );
  }
});
