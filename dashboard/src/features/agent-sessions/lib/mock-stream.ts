// A mock harness replaying a recorded v1 UI-message stream, so the conversation
// column is fully developable and unit-testable before m43 is live. The wire
// format is fixed by the shipped driver (`x-vercel-ai-ui-message-stream: v1`,
// SSE `data:` frames terminated by `[DONE]`); these fixtures reproduce the
// exact chunk shapes the AI SDK provider + driver emit (plan/diff/terminal
// `data-acp` parts interleaved with text/reasoning/tool UI parts). A fixture
// `fetch` swapped into the transport drives `useChat` with no network.

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
    type: "data-acp",
    data: {
      type: "plan",
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
    type: "data-acp",
    data: {
      type: "diff",
      path: "agent-result.txt",
      oldText: "",
      newText: "committed by the agent\n",
      toolCallId: "t1",
    },
  } as UIMessageChunk,
  {
    type: "data-acp",
    data: {
      type: "terminal",
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

/**
 * A fixture `fetch` that streams a fixed chunk list, then (for a terminal
 * session) the `[DONE]` sentinel and close. Injected into the transport in
 * place of `globalThis.fetch`.
 */
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
