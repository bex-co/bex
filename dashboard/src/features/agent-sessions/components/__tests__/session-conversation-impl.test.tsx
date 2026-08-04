import { describe, expect, it } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
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
  it("renders a terminal session's full grouped transcript and settles on [DONE]", async () => {
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

    // The plan group is open by default → its checklist items render.
    await waitFor(() =>
      expect(screen.getByText("Edit the result file")).toBeInTheDocument(),
    );
    expect(screen.getByText("Run the tests")).toBeInTheDocument();

    // Grouped: a Thought (reasoning) disclosure, a Terminal disclosure, and the
    // dynamic tool with its state badge all render as collapsible groups.
    expect(screen.getByText("Thought")).toBeInTheDocument();
    expect(screen.getByText("Terminal")).toBeInTheDocument();
    expect(screen.getByText("acp_agent")).toBeInTheDocument();
    expect(screen.getByText("Done")).toBeInTheDocument();

    // The final assistant text (markdown) is rendered.
    expect(screen.getByText(/Task committed\./)).toBeInTheDocument();

    // A terminal session settles: the "Session ended." footer appears once the
    // stream reaches [DONE] and the chat is ready.
    await waitFor(() =>
      expect(screen.getByText("Session ended.")).toBeInTheDocument(),
    );

    // The collapsed reasoning content is revealed on expand.
    await userEvent.click(screen.getByText("Thought"));
    expect(
      screen.getByText(/The task asks me to edit the file/),
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

  it("shows the degraded state when the stream errors", async () => {
    const failingFetch = (async () =>
      new Response("nope", { status: 503 })) as typeof globalThis.fetch;
    const transport = createAgentSessionTransport({
      sessionId: "as-err",
      mintTicket,
      fetch: failingFetch,
    });

    render(
      <SessionConversationImpl
        sessionId="as-err"
        isTerminal={false}
        transport={transport}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getByText("The conversation stream is unavailable right now."),
      ).toBeInTheDocument(),
    );
  });
});
