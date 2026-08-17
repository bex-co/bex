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

  it("renders durable user prompts in turn order after refresh", async () => {
    const transcript = [
      { type: "start", messageId: "asm-durable" },
      {
        type: "data-user-prompt",
        data: { turn: 1, text: "initial durable prompt", complete: true },
      },
      { type: "text-start", id: "t1" },
      { type: "text-delta", id: "t1", delta: "first answer" },
      { type: "text-end", id: "t1" },
      {
        type: "data-user-prompt",
        data: {
          turn: 2,
          text: "follow-up survives refresh",
          complete: false,
          truncated: true,
          reason: "quota",
        },
      },
      { type: "text-start", id: "t2" },
      { type: "text-delta", id: "t2", delta: "second answer" },
      { type: "text-end", id: "t2" },
      { type: "finish" },
    ] as UIMessageChunk[];
    const transport = createAgentSessionTransport({
      sessionId: "as-durable-prompts",
      mintTicket,
      fetch: makeFixtureFetch(transcript, { terminal: true }),
    });

    render(
      <SessionConversationImpl
        sessionId="as-durable-prompts"
        isTerminal
        transport={transport}
      />,
    );

    expect(
      await screen.findByText("initial durable prompt"),
    ).toBeInTheDocument();
    expect(screen.getByText("first answer")).toBeInTheDocument();
    expect(screen.getByText("follow-up survives refresh")).toBeInTheDocument();
    expect(screen.getByText("second answer")).toBeInTheDocument();
    expect(
      screen.getByText(/Some assistant output could not be preserved: quota/),
    ).toBeInTheDocument();
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

  it("folds consecutive tool and diff parts into one activity group", async () => {
    // The driver now emits real dynamic tool parts (true names) and typed
    // `data-acp-diff` parts — no synthetic command collapse. Consecutive tool +
    // diff parts still fold into ONE Devin "Worked" group.
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
        type: "data-acp-diff",
        data: {
          path: "main.go",
          oldText: "",
          newText: "package main\n",
          toolCallId: "gt1",
        },
      } as UIMessageChunk,
      {
        type: "tool-input-start",
        toolCallId: "gt2",
        toolName: "read_file",
        dynamic: true,
      },
      {
        type: "tool-input-available",
        toolCallId: "gt2",
        toolName: "read_file",
        input: { path: "main.go" },
        dynamic: true,
      },
      {
        type: "tool-output-available",
        toolCallId: "gt2",
        output: { bytes: 12 },
        dynamic: true,
      },
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

    // A single activity group folds the two tools + the diff under one Devin
    // "Worked" summary. Its steps are collapsed until expanded.
    const summary = await screen.findByText("Worked");
    expect(summary).toBeInTheDocument();
    expect(screen.getAllByText("Worked")).toHaveLength(1);
    expect(screen.queryByText("search")).not.toBeInTheDocument();

    // Expanding reveals every folded step in the one group.
    await userEvent.click(summary);
    expect(screen.getByText("search")).toBeInTheDocument();
    expect(screen.getByText("read_file")).toBeInTheDocument();
    expect(screen.getByText("main.go")).toBeInTheDocument();
  });

  it("collapses ACP plan snapshots to the latest and never spins after settle", async () => {
    // ACP re-sends the whole plan on every update. Two snapshots must render ONE
    // plan checklist showing the LATEST statuses — not a stack of stale snapshots
    // each frozen (and, pre-fix, spinning) at its own in_progress state (ADR051).
    const PLAN_SNAPSHOTS: UIMessageChunk[] = [
      { type: "start", messageId: "asm-p" },
      {
        type: "data-acp-plan",
        data: {
          entries: [
            { content: "Wire the handler", status: "in_progress" },
            { content: "Open a draft PR", status: "pending" },
          ],
        },
      } as UIMessageChunk,
      {
        type: "data-acp-plan",
        data: {
          entries: [
            { content: "Wire the handler", status: "completed" },
            { content: "Open a draft PR", status: "in_progress" },
          ],
        },
      } as UIMessageChunk,
      { type: "finish" },
    ];
    const transport = createAgentSessionTransport({
      sessionId: "as-plan",
      mintTicket,
      fetch: makeFixtureFetch(PLAN_SNAPSHOTS, { terminal: true }),
    });

    const { container } = render(
      <SessionConversationImpl
        sessionId="as-plan"
        isTerminal
        transport={transport}
      />,
    );

    await waitFor(() =>
      expect(screen.getByText("Wire the handler")).toBeInTheDocument(),
    );
    // ONE plan block (one "Plan" trigger, each entry once) — snapshots collapsed.
    expect(screen.getAllByText("Plan")).toHaveLength(1);
    expect(screen.getAllByText("Wire the handler")).toHaveLength(1);
    expect(screen.getAllByText("Open a draft PR")).toHaveLength(1);

    // Settled terminal session: no spinner anywhere (the forever-spin regression).
    await waitFor(() =>
      expect(screen.getByText("Session ended.")).toBeInTheDocument(),
    );
    expect(container.querySelector(".animate-spin")).toBeNull();
  });

  it("unwraps the opaque ACP dynamic tool instead of dumping raw JSON", async () => {
    // The shipped provider collapses ACP tools into one dynamic tool whose input
    // is {toolCallId, toolName, args}; a naive render dumped `{"command":"ls"}` /
    // `{"ok":true}`. The unwrap recovers the real name + command and drops the
    // trivial ack (ADR051 glue #2).
    const ENVELOPE_TOOL: UIMessageChunk[] = [
      { type: "start", messageId: "asm-e" },
      {
        type: "tool-input-start",
        toolCallId: "e1",
        toolName: "acp.acp_provider_agent_dynamic_tool",
        dynamic: true,
      },
      {
        type: "tool-input-available",
        toolCallId: "e1",
        toolName: "acp.acp_provider_agent_dynamic_tool",
        input: {
          toolCallId: "e1",
          toolName: "bash",
          args: { command: "ls -la" },
        },
        dynamic: true,
      },
      {
        type: "tool-output-available",
        toolCallId: "e1",
        output: { ok: true },
        dynamic: true,
      },
      { type: "finish" },
    ];
    const transport = createAgentSessionTransport({
      sessionId: "as-tool",
      mintTicket,
      fetch: makeFixtureFetch(ENVELOPE_TOOL, { terminal: true }),
    });

    const { container } = render(
      <SessionConversationImpl
        sessionId="as-tool"
        isTerminal
        transport={transport}
      />,
    );

    const summary = await screen.findByText("Worked");
    await userEvent.click(summary);

    // Real tool name recovered; the opaque wrapper name never shown.
    expect(screen.getByText("bash")).toBeInTheDocument();
    expect(
      screen.queryByText("acp.acp_provider_agent_dynamic_tool"),
    ).not.toBeInTheDocument();
    // Command lifted inline as a compact code line; trivial {ok:true} ack dropped
    // (no raw JSON output blob).
    expect(screen.getByText("ls -la")).toBeInTheDocument();
    expect(container.textContent).not.toContain('"ok"');
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
});
