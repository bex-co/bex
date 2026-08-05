import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { collapseDoubledParts } from "@/features/agent-sessions/lib/collapse-doubled-parts";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation-impl";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import {
  makeFixtureFetch,
  makeManualStream,
  RUNNING_TRANSCRIPT_HEAD,
  TERMINAL_TRANSCRIPT,
} from "@/features/agent-sessions/lib/mock-stream";
import type { UIMessageChunk } from "ai";

const mintTicket = async () => ({ ticket: "fixture-ticket" });

describe("SessionConversationImpl", () => {
  it("renders a terminal session's grouped transcript and settles on [DONE]", async () => {
    const transport = createAgentSessionTransport({
      sessionId: "as-term",
      mintTicket,
      fetch: makeFixtureFetch(TERMINAL_TRANSCRIPT, { terminal: true }),
    });

    render(
      <SessionConversationImpl
        sessionId="as-term"
        isTerminal
        transport={transport}
      />,
    );

    // The plan checklist renders its entries inline (not folded, not collapsed).
    await waitFor(() =>
      expect(screen.getByText("Edit the result file")).toBeInTheDocument(),
    );
    expect(screen.getByText("Run the tests")).toBeInTheDocument();

    // Reasoning renders as a collapsible "Thought" block, collapsed by default.
    expect(screen.getByText("Thought")).toBeInTheDocument();
    expect(
      screen.queryByText(/The task asks me to edit the file/),
    ).not.toBeInTheDocument();

    // The tool + diff + terminal parts fold into ONE activity group whose
    // collapsed summary is the Devin "Worked" label (no derived duration on a
    // one-frame replay). The tool name and terminal output stay hidden until the
    // group is expanded.
    expect(screen.getByText("Worked")).toBeInTheDocument();
    expect(screen.queryByText("acp_agent")).not.toBeInTheDocument();

    // The final assistant text (markdown) renders.
    expect(screen.getByText(/Task committed\./)).toBeInTheDocument();

    // A terminal session settles: the footer appears once [DONE] lands and the
    // chat is ready.
    await waitFor(() =>
      expect(screen.getByText("Session ended.")).toBeInTheDocument(),
    );

    // Expanding the reasoning block reveals its content.
    await userEvent.click(screen.getByText("Thought"));
    expect(
      screen.getByText(/The task asks me to edit the file/),
    ).toBeInTheDocument();

    // Expanding the activity group reveals each step (a vertical timeline): the
    // tool call and the folded diff.
    await userEvent.click(screen.getByText("Worked"));
    expect(screen.getByText("acp_agent")).toBeInTheDocument();
    expect(screen.getByText(/committed by the agent/)).toBeInTheDocument();
  });

  it("appends parts incrementally as a running session live-tails", async () => {
    const manual = makeManualStream();
    const transport = createAgentSessionTransport({
      sessionId: "as-run",
      mintTicket,
      fetch: manual.fetch,
    });

    render(
      <SessionConversationImpl
        sessionId="as-run"
        isTerminal={false}
        transport={transport}
      />,
    );

    // Push the opening slice (start + plan + first reasoning chunk).
    await act(async () => {
      for (const chunk of RUNNING_TRANSCRIPT_HEAD) manual.push(chunk);
    });
    await waitFor(() =>
      expect(screen.getByText("Edit the result file")).toBeInTheDocument(),
    );
    // The final answer has not streamed yet.
    expect(screen.queryByText(/Task committed\./)).not.toBeInTheDocument();
    // Not terminal → no "Session ended." note.
    expect(screen.queryByText("Session ended.")).not.toBeInTheDocument();

    // Live-tail the remaining chunks; the text answer appends without a reload.
    const rest = TERMINAL_TRANSCRIPT.slice(RUNNING_TRANSCRIPT_HEAD.length);
    await act(async () => {
      for (const chunk of rest) manual.push(chunk as UIMessageChunk);
      manual.done();
    });
    await waitFor(() =>
      expect(screen.getByText(/Task committed\./)).toBeInTheDocument(),
    );
  });

  it("folds consecutive tool and command parts into one activity group", async () => {
    const GROUPED: UIMessageChunk[] = [
      { type: "start", messageId: "asm-g" },
      {
        type: "tool-input-start",
        toolCallId: "gt1",
        toolName: "search",
        dynamic: true,
      },
      {
        type: "tool-input-available",
        toolCallId: "gt1",
        toolName: "search",
        input: { q: "x" },
        dynamic: true,
      },
      {
        type: "tool-output-available",
        toolCallId: "gt1",
        output: { hits: 2 },
        dynamic: true,
      },
      {
        type: "data-acp",
        data: {
          sessionUpdate: "tool_call",
          title: "List files",
          command: "ls -la",
        },
      } as UIMessageChunk,
      {
        type: "data-acp",
        data: {
          sessionUpdate: "tool_call",
          title: "Show file",
          command: "cat main.go",
        },
      } as UIMessageChunk,
      { type: "finish" },
    ];

    const transport = createAgentSessionTransport({
      sessionId: "as-group",
      mintTicket,
      fetch: makeFixtureFetch(GROUPED, { terminal: true }),
    });

    render(
      <SessionConversationImpl
        sessionId="as-group"
        isTerminal
        transport={transport}
      />,
    );

    // A single activity group folds the two commands (the tool + two ACP
    // command parts merged) under one Devin "Worked" summary. Its steps are
    // collapsed until expanded.
    const summary = await screen.findByText("Worked");
    expect(summary).toBeInTheDocument();
    expect(screen.getAllByText("Worked")).toHaveLength(1);
    expect(screen.queryByText("search")).not.toBeInTheDocument();

    // Expanding reveals every folded step in the one group.
    await userEvent.click(summary);
    expect(screen.getByText("search")).toBeInTheDocument();
    expect(screen.getByText("List files")).toBeInTheDocument();
    expect(screen.getByText("ls -la")).toBeInTheDocument();
    expect(screen.getByText("Show file")).toBeInTheDocument();
  });

  it("lifts the live-steering handle via onChatStateChange", async () => {
    const onChatStateChange = vi.fn();
    const transport = createAgentSessionTransport({
      sessionId: "as-handle",
      mintTicket,
      fetch: makeFixtureFetch(TERMINAL_TRANSCRIPT, { terminal: true }),
    });

    render(
      <SessionConversationImpl
        sessionId="as-handle"
        isTerminal
        transport={transport}
        onChatStateChange={onChatStateChange}
      />,
    );

    await waitFor(() => expect(onChatStateChange).toHaveBeenCalled());
    const handle = onChatStateChange.mock.calls.at(
      -1,
    )?.[0] as ConversationChatHandle | null;
    expect(handle).not.toBeNull();
    expect(typeof handle?.sendMessage).toBe("function");
    expect(typeof handle?.status).toBe("string");
  });

  it("shows the degraded state when the stream errors", async () => {
    const failingFetch = (async () =>
      new Response("nope", { status: 503 })) as typeof globalThis.fetch;
    const transport = createAgentSessionTransport({
      sessionId: "as-err",
      mintTicket,
      fetch: failingFetch,
    });

    const onChatStateChange = vi.fn();
    render(
      <SessionConversationImpl
        sessionId="as-err"
        isTerminal={false}
        transport={transport}
        onChatStateChange={onChatStateChange}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getByText("The conversation stream is unavailable right now."),
      ).toBeInTheDocument(),
    );
    // The steering handle is withheld (null) while the stream is errored.
    await waitFor(() => expect(onChatStateChange).toHaveBeenCalledWith(null));
  });
});

describe("collapseDoubledParts", () => {
  const P = (type: string, extra: Record<string, unknown> = {}) => ({
    type,
    ...extra,
  });
  // The transcript a single replay produces.
  const transcript = () => [
    P("text", { id: "a", text: "Hello" }),
    P("data-acp", { data: { type: "plan", entries: [] } }),
    P("tool-run", { toolName: "acp_agent" }),
    P("text", { id: "b", text: "Done" }),
  ];

  it("collapses an exact doubled transcript to a single copy", () => {
    // A dev double-mount appends the replay twice into one message; the second
    // copy has FRESH per-part ids, so this must dedupe id-agnostically.
    const doubled = [
      ...transcript(),
      P("text", { id: "a2", text: "Hello" }),
      P("data-acp", { data: { type: "plan", entries: [] } }),
      P("tool-run", { toolName: "acp_agent" }),
      P("text", { id: "b2", text: "Done" }),
    ];
    const result = collapseDoubledParts(doubled);
    expect(result).toHaveLength(4);
    expect(result.map((p) => (p as { text?: string }).text)).toEqual([
      "Hello",
      undefined,
      undefined,
      "Done",
    ]);
  });

  it("leaves a genuine (non-mirrored) transcript untouched", () => {
    const real = [
      ...transcript(),
      P("text", { id: "c", text: "A follow-up turn" }),
      P("tool-run", { toolName: "other_tool" }),
    ];
    expect(collapseDoubledParts(real)).toHaveLength(real.length);
  });

  it("leaves an odd-length or short parts list untouched", () => {
    expect(collapseDoubledParts(transcript().slice(0, 3))).toHaveLength(3);
    expect(collapseDoubledParts([P("text", { text: "hi" })])).toHaveLength(1);
  });
})
