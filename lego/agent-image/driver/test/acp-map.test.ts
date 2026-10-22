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
import type { SessionUpdate } from "@agentclientprotocol/sdk/unstable";
import { readUIMessageStream, type UIMessage, type UIMessageChunk } from "ai";
import { createUpdateMapper } from "../src/acp-map.js";
import {
  existingSourceTime,
  isUtcTimestamp,
  stampSourceTimestamp,
} from "../src/timestamp.js";

function clock(
  start = "2026-08-19T00:00:00.000Z",
  stepMs = 1000,
): () => string {
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

test("unrendered ACP content and future variants stay observable", () => {
  const mapper = createUpdateMapper();
  for (const sessionUpdate of [
    "agent_message_chunk",
    "agent_thought_chunk",
  ] as const) {
    const update: SessionUpdate = {
      sessionUpdate,
      content: { type: "image", data: "aGVsbG8=", mimeType: "image/png" },
    };
    const chunks = mapper.map(update);
    assert.ok(
      chunks.some(
        (chunk) =>
          chunk.type === "data-acp-info" &&
          chunk.transient &&
          chunk.data === update,
      ),
    );
  }
  const future = {
    sessionUpdate: "future_update",
    value: "visible",
  } as unknown as SessionUpdate;
  assert.ok(
    mapper
      .map(future)
      .some(
        (chunk) =>
          chunk.type === "data-acp-future-update" &&
          chunk.transient &&
          chunk.data === future,
      ),
  );
  const content = {
    type: "content" as const,
    content: { type: "text" as const, text: "tool details" },
  };
  assert.ok(
    mapper
      .map({
        sessionUpdate: "tool_call",
        toolCallId: "details",
        title: "Inspect",
        content: [content],
      })
      .some(
        (chunk) =>
          chunk.type === "data-acp-info" &&
          chunk.transient &&
          JSON.stringify(chunk.data).includes("tool details"),
      ),
  );
  assert.deepEqual(
    mapper.map({
      sessionUpdate: "user_message_chunk",
      content: { type: "text", text: "echo" },
    }),
    [],
  );
});

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
    [
      "reasoning-start",
      "data-acp-plan",
      "tool-input-start",
      "data-acp-diff",
      "data-acp-terminal",
      "text-start",
    ].map((type) => [type, chunks.find((c) => c.type === type)]),
  );
  for (const [type, chunk] of Object.entries(byType)) {
    assert.ok(chunk, `missing ${type}`);
    assert.ok(
      isUtcTimestamp((chunk as { at?: unknown }).at),
      `${type} has no at`,
    );
  }

  const reasoningEnd = chunks.find((c) => c.type === "reasoning-end") as {
    at?: string;
    providerMetadata?: { bex?: { at?: string; endAt?: string } };
  };
  assert.ok(reasoningEnd);
  assert.equal(
    reasoningEnd.providerMetadata?.bex?.at,
    atOf(byType["reasoning-start"] as Record<string, unknown>),
  );
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

// claude-agent-acp 0.94 (captured against a mock model): a Bash
// call opens with a placeholder, its command arrives on a tool_call_update, and
// the result is text content only — no rawOutput.
const claudeAgentBash: SessionUpdate[] = [
  {
    sessionUpdate: "tool_call",
    toolCallId: "toolu_15",
    status: "pending",
    title: "Terminal",
    kind: "execute",
    content: [],
  },
  {
    sessionUpdate: "tool_call_update",
    toolCallId: "toolu_15",
    rawInput: {
      command: "echo mock > mock.txt",
      description: "Write mock file",
    },
    title: "echo mock > mock.txt",
    content: [
      { type: "content", content: { type: "text", text: "Write mock file" } },
    ],
  },
  {
    sessionUpdate: "tool_call_update",
    toolCallId: "toolu_15",
    status: "completed",
    content: [
      {
        type: "content",
        content: { type: "text", text: "```console\nmock\n```" },
      },
    ],
  },
];

async function replay(chunks: UIMessageChunk[]): Promise<UIMessage> {
  let last: UIMessage | undefined;
  const stream = new ReadableStream<UIMessageChunk>({
    start(controller) {
      controller.enqueue({ type: "start" });
      for (const chunk of chunks) controller.enqueue(chunk);
      controller.enqueue({ type: "finish" });
      controller.close();
    },
  });
  for await (const message of readUIMessageStream({ stream })) last = message;
  assert.ok(last);
  return last;
}

test("a refined tool call keeps one durable part with the real input and text output", async () => {
  const mapper = createUpdateMapper({ now: clock() });
  const chunks = claudeAgentBash.flatMap((update) => mapper.map(update));
  const tools = (await replay(chunks)).parts.filter(
    (part) => part.type === "dynamic-tool",
  );
  assert.equal(tools.length, 1);
  const [tool] = tools as Array<Record<string, unknown>>;
  assert.equal(tool.toolCallId, "toolu_15");
  assert.equal(tool.toolName, "Terminal");
  assert.equal(tool.title, "echo mock > mock.txt");
  assert.deepEqual(tool.input, {
    command: "echo mock > mock.txt",
    description: "Write mock file",
  });
  assert.equal(tool.state, "output-available");
  assert.equal(tool.output, "```console\nmock\n```");
});

test("a failed text-only tool result becomes the error text", () => {
  const mapper = createUpdateMapper();
  mapper.map(claudeAgentBash[0]!);
  const chunks = mapper.map({
    sessionUpdate: "tool_call_update",
    toolCallId: "toolu_15",
    status: "failed",
    content: [
      { type: "content", content: { type: "text", text: "exit code 1" } },
    ],
  });
  const error = chunks.find((chunk) => chunk.type === "tool-output-error");
  assert.ok(error && "errorText" in error);
  assert.equal(error.errorText, "exit code 1");
});

test("updates after a tool finishes never rewrite its input", () => {
  const mapper = createUpdateMapper();
  for (const update of claudeAgentBash) mapper.map(update);
  const late = mapper.map({
    sessionUpdate: "tool_call_update",
    toolCallId: "toolu_15",
    title: "late title",
    rawInput: { command: "rm -rf /" },
  });
  assert.ok(!late.some((chunk) => chunk.type === "tool-input-available"));
});

test("a title-only refinement keeps the input already published", () => {
  const mapper = createUpdateMapper();
  mapper.map(claudeAgentBash[0]!);
  mapper.map(claudeAgentBash[1]!);
  const refined = mapper
    .map({
      sessionUpdate: "tool_call_update",
      toolCallId: "toolu_15",
      title: "Write mock",
    })
    .find((chunk) => chunk.type === "tool-input-available");
  assert.ok(refined && "input" in refined);
  assert.deepEqual(refined.input, {
    command: "echo mock > mock.txt",
    description: "Write mock file",
  });
  assert.equal(refined.title, "Write mock");
});

test("a repeated identical refinement publishes nothing new", () => {
  const mapper = createUpdateMapper();
  mapper.map(claudeAgentBash[0]!);
  mapper.map(claudeAgentBash[1]!);
  const repeat = mapper.map(claudeAgentBash[1]!);
  assert.ok(!repeat.some((chunk) => chunk.type === "tool-input-available"));
});
