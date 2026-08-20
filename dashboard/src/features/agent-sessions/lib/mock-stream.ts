// A mock harness replaying a recorded v1 UI-message stream, so the conversation
// column is fully developable and unit-testable before m43 is live. The wire
// format is fixed by the shipped driver (`x-vercel-ai-ui-message-stream: v1`,
// SSE `data:` frames terminated by `[DONE]`); these fixtures reproduce the
// exact chunk shapes the driver emits (typed `data-acp-plan|diff|terminal` parts
// interleaved with text/reasoning/tool UI parts). A fixture `fetch` swapped into
// the transport drives `useChat` with no network.

import type { UIMessageChunk } from "ai";

/** UI-message-stream v1 SSE headers (must match the driver's `server.ts`). */
const V1_HEADERS: Record<string, string> = {
  "content-type": "text/event-stream",
  "cache-control": "no-cache, no-transform",
  "x-vercel-ai-ui-message-stream": "v1",
};

/** Serializes one chunk as an SSE `data:` frame. */
export function encodeChunk(chunk: UIMessageChunk): string {
  return `data: ${JSON.stringify(chunk)}\n\n`;
}

function sseResponse(body: ReadableStream<Uint8Array>): Response {
  return new Response(body, { status: 200, headers: V1_HEADERS });
}

/**
 * A recorded terminal transcript: plan → reasoning → tool (with a diff and a
 * terminal `data-acp` part) → final text answer. Terminates with `finish`; the
 * fixture fetch appends `[DONE]`, matching a terminal session's replay.
 */
export const TERMINAL_TRANSCRIPT: UIMessageChunk[] = [
  { type: "start", messageId: "asm-1" },
  { type: "start-step" },
  {
    type: "data-acp-plan",
    data: {
      entries: [
        {
          content: "Edit the result file",
          status: "completed",
          priority: "high",
        },
        { content: "Run the tests", status: "completed", priority: "medium" },
        {
          content: "Open a draft PR",
          status: "in_progress",
          priority: "medium",
        },
      ],
    },
  } as UIMessageChunk,
  { type: "reasoning-start", id: "r1" },
  {
    type: "reasoning-delta",
    id: "r1",
    delta: "The task asks me to edit the file ",
  },
  { type: "reasoning-delta", id: "r1", delta: "and verify the build passes." },
  { type: "reasoning-end", id: "r1" },
  {
    type: "tool-input-start",
    toolCallId: "t1",
    toolName: "acp_agent",
    dynamic: true,
  },
  {
    type: "tool-input-available",
    toolCallId: "t1",
    toolName: "acp_agent",
    input: { path: "agent-result.txt" },
    dynamic: true,
  },
  {
    type: "data-acp-diff",
    data: {
      path: "agent-result.txt",
      oldText: "",
      newText: "committed by the agent\n",
      toolCallId: "t1",
    },
  } as UIMessageChunk,
  {
    type: "data-acp-terminal",
    data: {
      terminalId: "term-1",
      output: "$ go test ./...\nok  \tpkg\t0.10s",
      toolCallId: "t1",
    },
  } as UIMessageChunk,
  {
    type: "tool-output-available",
    toolCallId: "t1",
    output: { ok: true },
    dynamic: true,
  },
  { type: "text-start", id: "txt1" },
  { type: "text-delta", id: "txt1", delta: "Task committed. " },
  {
    type: "text-delta",
    id: "txt1",
    delta: "Opened a draft PR on `bex-agent/fix`.",
  },
  { type: "text-end", id: "txt1" },
  { type: "finish-step" },
  { type: "finish" },
];

/**
 * The opening slice of a running session's live tail: the message start, the
 * plan, and the first reasoning chunk. The manual harness pushes the remainder
 * of `TERMINAL_TRANSCRIPT` incrementally to exercise the append path.
 */
export const RUNNING_TRANSCRIPT_HEAD: UIMessageChunk[] =
  TERMINAL_TRANSCRIPT.slice(0, 4);

/** Stamp a fixture chunk with the driver source-timestamp fields. */
export function withSourceTime(
  chunk: UIMessageChunk,
  at: string,
  extra?: { endAt?: string },
): UIMessageChunk {
  const current = chunk as UIMessageChunk & {
    providerMetadata?: { bex?: { at?: string; endAt?: string } };
  };
  const skipMeta =
    chunk.type === "text-delta" || chunk.type === "reasoning-delta";
  if (skipMeta) {
    return { ...current, at } as unknown as UIMessageChunk;
  }
  return {
    ...current,
    at,
    providerMetadata: {
      ...current.providerMetadata,
      bex: {
        ...current.providerMetadata?.bex,
        at: current.providerMetadata?.bex?.at ?? at,
        ...(extra?.endAt ? { endAt: extra.endAt } : {}),
      },
    },
  } as unknown as UIMessageChunk;
}

const T0 = "2026-08-19T00:00:00.000Z";
const T40 = "2026-08-19T00:00:40.000Z";

/**
 * A 40-second activity group: tool + diff with persisted source timestamps so
 * live and one-frame replay both render "Worked for 40s".
 */
export const TIMESTAMPED_ACTIVITY: UIMessageChunk[] = [
  { type: "start", messageId: "asm-timed" },
  withSourceTime(
    {
      type: "tool-input-start",
      toolCallId: "tt1",
      toolName: "search",
      dynamic: true,
    },
    T0,
  ),
  withSourceTime(
    {
      type: "tool-input-available",
      toolCallId: "tt1",
      toolName: "search",
      input: { q: "x" },
      dynamic: true,
    },
    T0,
  ),
  withSourceTime(
    {
      type: "tool-output-available",
      toolCallId: "tt1",
      output: { hits: 2 },
      dynamic: true,
    },
    T40,
  ),
  withSourceTime(
    {
      type: "data-acp-diff",
      data: {
        path: "main.go",
        oldText: "",
        newText: "package main\n",
        toolCallId: "tt1",
      },
    } as UIMessageChunk,
    T40,
  ),
  { type: "finish" },
];

/**
 * Reasoning that started at T0 and ended 12s later, via `endAt` on the end chunk
 * so a one-frame replay still shows "Thought for 12s".
 */
export const TIMESTAMPED_REASONING: UIMessageChunk[] = [
  { type: "start", messageId: "asm-rsn" },
  withSourceTime({ type: "reasoning-start", id: "r-timed" }, T0),
  withSourceTime(
    {
      type: "reasoning-delta",
      id: "r-timed",
      delta: "Considering the edit.",
    },
    T0,
  ),
  withSourceTime(
    {
      type: "reasoning-end",
      id: "r-timed",
      providerMetadata: { bex: { at: T0 } },
    } as UIMessageChunk,
    "2026-08-19T00:00:12.000Z",
    { endAt: "2026-08-19T00:00:12.000Z" },
  ),
  { type: "finish" },
];

export function makeFixtureFetch(
  chunks: UIMessageChunk[],
  { terminal = true }: { terminal?: boolean } = {},
): typeof globalThis.fetch {
  return (async () => {
    const encoder = new TextEncoder();
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of chunks) {
          controller.enqueue(encoder.encode(encodeChunk(chunk)));
        }
        if (terminal) controller.enqueue(encoder.encode("data: [DONE]\n\n"));
        controller.close();
      },
    });
    return sseResponse(stream);
  }) as typeof globalThis.fetch;
}

/** A hand-driven live stream: the test pushes chunks then closes with `[DONE]`. */
export interface ManualStream {
  fetch: typeof globalThis.fetch;
  push: (chunk: UIMessageChunk) => void;
  done: () => void;
}

/**
 * A fixture `fetch` whose body the caller feeds one chunk at a time, so a test
 * can assert the column appends parts as they arrive (the live-tail path).
 */
export function makeManualStream(): ManualStream {
  const encoder = new TextEncoder();
  let controller!: ReadableStreamDefaultController<Uint8Array>;
  const stream = new ReadableStream<Uint8Array>({
    start(c) {
      controller = c;
    },
  });
  return {
    fetch: (async () => sseResponse(stream)) as typeof globalThis.fetch,
    push(chunk) {
      controller.enqueue(encoder.encode(encodeChunk(chunk)));
    },
    done() {
      controller.enqueue(encoder.encode("data: [DONE]\n\n"));
      controller.close();
    },
  };
}
