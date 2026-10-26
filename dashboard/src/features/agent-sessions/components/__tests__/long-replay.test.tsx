import { readFileSync } from "node:fs";
import path from "node:path";
import { gunzipSync } from "node:zlib";
import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import type { UIMessageChunk } from "ai";
import {
  SessionConversationImpl,
  type ConversationChatHandle,
} from "@/features/agent-sessions/components/session-conversation-impl";
import type { ConversationState } from "@/features/agent-sessions/lib/conversation-state";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import {
  encodeChunk,
  makeFixtureFetch,
  makeManualStream,
} from "@/features/agent-sessions/lib/mock-stream";

// The recorded production replay of ags-dbf8nggkbcjs73a5r3u0 (w10/m16 t001):
// 5,602 raw AI-SDK chunks (3,835 reasoning deltas, 299 tool calls, 13 text
// blocks). Tool outputs and ACP info payloads are clipped to keep the fixture
// small; the chunk sequence and count are verbatim. Before the fix this replay
// threw React #185 ("Maximum update depth exceeded") after the first reasoning
// block and aborted the stream.
const RAW: UIMessageChunk[] = JSON.parse(
  gunzipSync(
    readFileSync(
      path.join(__dirname, "fixtures/ags-dbf8nggkbcjs73a5r3u0.json.gz"),
    ),
  ).toString("utf8"),
);

// Exactly what the gateway replays: a durable start + the user prompt, the
// stored chunks, then finish (makeFixtureFetch appends [DONE]).
const REPLAY: UIMessageChunk[] = [
  { type: "start", messageId: "durable-ags-dbf8nggkbcjs73a5r3u0" },
  {
    type: "data-user-prompt",
    data: { turn: 1, text: "/rt-simplify-logic" },
  } as UIMessageChunk,
  ...RAW,
  { type: "finish" },
];

const mintTicket = async () => ({ ticket: "fixture" });

// The detail page wires both callbacks to parent state setters.
function Harness({
  transport,
  isTerminal = true,
}: {
  transport: ReturnType<typeof createAgentSessionTransport>;
  isTerminal?: boolean;
}) {
  const [, setChat] = useState<ConversationChatHandle | null>(null);
  const [, setState] = useState<ConversationState>("not-started");
  return (
    <SessionConversationImpl
      sessionId="ags-dbf8nggkbcjs73a5r3u0"
      isTerminal={isTerminal}
      phase={isTerminal ? "completed" : "running"}
      transport={transport}
      onChatStateChange={setChat}
      onConversationStateChange={setState}
    />
  );
}

describe("long replay (ags-dbf8nggkbcjs73a5r3u0)", () => {
  it("fixture is the full chunk burst", () => {
    expect(RAW).toHaveLength(5602);
    const count = (type: string) => RAW.filter((c) => c.type === type).length;
    expect(count("reasoning-start")).toBe(96);
    expect(count("reasoning-delta")).toBe(3835);
    expect(count("text-start")).toBe(13);
    expect(
      new Set(
        RAW.flatMap((c) =>
          "toolCallId" in c ? [(c as { toolCallId: string }).toolCallId] : [],
        ),
      ).size,
    ).toBe(299);
  });

  it("replays every block through the real useChat without React #185", async () => {
    const errors: string[] = [];
    const spy = vi.spyOn(console, "error").mockImplementation((...args) => {
      errors.push(args.map(String).join(" "));
    });
    try {
      render(
        <Harness
          transport={createAgentSessionTransport({
            sessionId: "ags-dbf8nggkbcjs73a5r3u0",
            mintTicket,
            fetch: makeFixtureFetch(REPLAY, { terminal: true }),
          })}
        />,
      );

      await waitFor(
        () => expect(screen.getByText("Session ended.")).toBeInTheDocument(),
        { timeout: 60_000 },
      );
      const log = screen.getByRole("log");
      // All 96 reasoning blocks, collapsed (a terminal replay never opens one).
      const reasoning = log.querySelectorAll("[data-block='reasoning']");
      expect(reasoning).toHaveLength(96);
      expect(
        [...reasoning].some((el) => el.querySelector("[aria-expanded='true']")),
      ).toBe(false);
      expect(log.querySelectorAll("[data-block='text']")).toHaveLength(13);
      // ~300 tool calls, grouped into one summary line per run of steps.
      const groups = [...log.querySelectorAll("[data-block='activity']")];
      expect(
        groups.reduce((n, el) => n + Number(el.getAttribute("data-steps")), 0),
      ).toBe(299);
      expect(groups.length).toBeLessThan(299);
      expect(screen.getAllByText(/Ran \d+ commands?/).length).toBeGreaterThan(
        0,
      );
      expect(screen.getAllByText(/Read \d+ files?/).length).toBeGreaterThan(0);
      expect(screen.getAllByText(/Ran \d+ subagents?/).length).toBeGreaterThan(
        0,
      );
      // The one failed call surfaces without expanding its group.
      expect(screen.getAllByText(/1 failed/)).toHaveLength(1);
      // Terminal replay: no live status line, no partial-load row.
      expect(screen.queryByLabelText("Agent status")).not.toBeInTheDocument();
      expect(screen.queryByText(/partially loaded/)).not.toBeInTheDocument();
      expect(errors.filter((e) => e.includes("Maximum update depth"))).toEqual(
        [],
      );
    } finally {
      spy.mockRestore();
    }
  }, 120_000);
});

describe("terminal replay loading state (t001)", () => {
  it("shows the skeleton, never the ended-empty copy, while the replay is in flight", async () => {
    const manual = makeManualStream();
    render(
      <Harness
        transport={createAgentSessionTransport({
          sessionId: "as-slow",
          mintTicket,
          fetch: manual.fetch,
        })}
      />,
    );
    expect(
      await screen.findByLabelText("Connecting to the session stream…"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "This session ended before a conversation was recorded.",
      ),
    ).not.toBeInTheDocument();

    // Settled with nothing to show: only now is it "ended".
    await act(async () => manual.done());
    expect(
      await screen.findByText(
        "This session ended before a conversation was recorded.",
      ),
    ).toBeInTheDocument();
  });
});

describe("partial load (t001)", () => {
  it("shows an inline partial-load row with Retry when the stream fails after content rendered", async () => {
    const encoder = new TextEncoder();
    const head: UIMessageChunk[] = [
      { type: "start", messageId: "asm-partial" },
      { type: "text-start", id: "t1" },
      { type: "text-delta", id: "t1", delta: "first half" },
      { type: "text-end", id: "t1" },
    ];
    let calls = 0;
    const fetch = (async () => {
      calls += 1;
      if (calls > 1) {
        return makeFixtureFetch(
          [...head, { type: "finish" }] as UIMessageChunk[],
          { terminal: true },
        )("");
      }
      const body = new ReadableStream<Uint8Array>({
        start(controller) {
          for (const chunk of head) {
            controller.enqueue(encoder.encode(encodeChunk(chunk)));
          }
          // Error after the head was consumed (erroring discards queued chunks).
          setTimeout(
            () => controller.error(new Error("connection reset")),
            100,
          );
        },
      });
      return new Response(body, {
        status: 200,
        headers: {
          "content-type": "text/event-stream",
          "x-vercel-ai-ui-message-stream": "v1",
        },
      });
    }) as typeof globalThis.fetch;

    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      render(
        <Harness
          transport={createAgentSessionTransport({
            sessionId: "as-partial",
            mintTicket,
            fetch,
          })}
        />,
      );
      expect(
        await screen.findByText("Conversation only partially loaded."),
      ).toBeInTheDocument();
      // The rendered half stays on screen.
      expect(screen.getByText("first half")).toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: "Retry" }));
      await waitFor(() =>
        expect(screen.getByText("Session ended.")).toBeInTheDocument(),
      );
      expect(
        screen.queryByText("Conversation only partially loaded."),
      ).not.toBeInTheDocument();
      // The re-replayed copy is folded: the text renders once.
      expect(screen.getAllByText("first half")).toHaveLength(1);
      expect(calls).toBe(2);
    } finally {
      spy.mockRestore();
    }
  });
});

describe("live status line (t005)", () => {
  it("shows one line with the latest tool or ACP status, replaced in place, only while live", async () => {
    const manual = makeManualStream();
    render(
      <Harness
        isTerminal={false}
        transport={createAgentSessionTransport({
          sessionId: "as-live",
          mintTicket,
          fetch: manual.fetch,
        })}
      />,
    );
    const push = async (chunks: UIMessageChunk[]) => {
      await act(async () => {
        for (const chunk of chunks) manual.push(chunk);
      });
    };

    // The backend's completed-item replay shape: no tool-input-start.
    await push([
      { type: "start", messageId: "asm-live" },
      {
        type: "tool-input-available",
        toolCallId: "c1",
        toolName: "Terminal",
        title: "git status",
        input: { command: "git status", description: "Check git state" },
        dynamic: true,
      } as UIMessageChunk,
    ]);
    await waitFor(() =>
      expect(screen.getByLabelText("Agent status")).toHaveTextContent(
        "Check git state",
      ),
    );

    await push([
      {
        type: "tool-output-available",
        toolCallId: "c1",
        output: "clean",
        dynamic: true,
      },
      {
        type: "tool-input-available",
        toolCallId: "c2",
        toolName: "Read lego/types/tiers/tiers.go",
        input: { file_path: "lego/types/tiers/tiers.go" },
        dynamic: true,
      } as UIMessageChunk,
    ]);
    await waitFor(() =>
      expect(screen.getByLabelText("Agent status")).toHaveTextContent(
        "Read lego/types/tiers/tiers.go",
      ),
    );
    expect(screen.getAllByLabelText("Agent status")).toHaveLength(1);

    // A transient ACP status replaces it in place.
    await push([
      {
        type: "data-acp-session-info-update",
        transient: true,
        data: {
          _meta: {
            jetbrains: {
              air: {
                sessionFailure: { title: "Retrying Claude, attempt 1 of 10." },
              },
            },
          },
        },
      } as UIMessageChunk,
    ]);
    await waitFor(() =>
      expect(screen.getByLabelText("Agent status")).toHaveTextContent(
        "Retrying Claude, attempt 1 of 10.",
      ),
    );

    await act(async () => {
      manual.push({ type: "finish" });
      manual.done();
    });
    await waitFor(() =>
      expect(screen.queryByLabelText("Agent status")).not.toBeInTheDocument(),
    );
  });
});
