import { describe, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UIMessageChunk } from "ai";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import {
  makeFixtureFetch,
  makeManualStream,
} from "@/features/agent-sessions/lib/mock-stream";

// The ADR051 item-model replay end to end (w10/005): the gateway expands every
// stored item into its minimal standard chunks (agenttranscript.Expand) —
// text/reasoning as start + ONE delta + end, a tool as tool-input-available
// plus its outcome chunk, a partial tool (the stream ended inside it, the
// harvest never completed it) as tool-input-available alone. This drives that
// exact shape through the real useChat + transport.

const mintTicket = async () => ({ ticket: "fixture-ticket" });

const tool = (id: string, command: string): UIMessageChunk =>
  ({
    type: "tool-input-available",
    toolCallId: id,
    toolName: "bash",
    input: { command },
    dynamic: true,
    title: command,
  }) as UIMessageChunk;

const ITEM_REPLAY: UIMessageChunk[] = [
  { type: "start", messageId: "durable-ags-items" },
  {
    type: "data-user-prompt",
    data: { turn: 1, text: "fix it" },
  } as UIMessageChunk,
  { type: "reasoning-start", id: "r1" },
  { type: "reasoning-delta", id: "r1", delta: "Look at the tests first." },
  { type: "reasoning-end", id: "r1" },
  tool("t-ok", "go test ./..."),
  {
    type: "tool-output-available",
    toolCallId: "t-ok",
    output: "ok  \tpkg\t0.1s",
    dynamic: true,
  } as UIMessageChunk,
  tool("t-err", "make lint"),
  {
    type: "tool-output-error",
    toolCallId: "t-err",
    errorText: "lint failed",
    dynamic: true,
  } as UIMessageChunk,
  tool("t-deny", "rm -rf build"),
  { type: "tool-output-denied", toolCallId: "t-deny" } as UIMessageChunk,
  tool("t-partial", "git push"),
  { type: "text-start", id: "x1" },
  { type: "text-delta", id: "x1", delta: "Tests pass now." },
  { type: "text-end", id: "x1" },
];

describe("item-expanded replay", () => {
  it("renders every item block and settles a terminal replay's partial tool as Interrupted", async () => {
    render(
      <SessionConversationImpl
        sessionId="ags-items"
        isTerminal
        phase="completed"
        transport={createAgentSessionTransport({
          sessionId: "ags-items",
          mintTicket,
          fetch: makeFixtureFetch([...ITEM_REPLAY, { type: "finish" }], {
            terminal: true,
          }),
        })}
      />,
    );
    await waitFor(() =>
      expect(screen.getByText("Session ended.")).toBeInTheDocument(),
    );
    const log = screen.getByRole("log");
    expect(screen.getByText("fix it")).toBeInTheDocument();
    expect(log.querySelectorAll("[data-block='reasoning']")).toHaveLength(1);
    const text = log.querySelectorAll("[data-block='text']");
    expect(text).toHaveLength(1);
    expect(text[0]).toHaveTextContent("Tests pass now.");
    const groups = log.querySelectorAll("[data-block='activity']");
    expect(groups).toHaveLength(1);
    expect(groups[0].getAttribute("data-steps")).toBe("4");
    // The error and the denial both count as failures in the group line.
    expect(
      within(groups[0] as HTMLElement).getByText(/2 failed/),
    ).toBeVisible();
    // Settled for good: nothing spins.
    expect(log.querySelector(".animate-spin")).toBeNull();

    await userEvent.click(within(groups[0] as HTMLElement).getByRole("button"));
    const group = within(groups[0] as HTMLElement);
    expect(group.getByText("Done")).toBeInTheDocument();
    expect(group.getByText("Failed")).toBeInTheDocument();
    expect(group.getByText("Denied")).toBeInTheDocument();
    expect(group.getByText("Interrupted")).toBeInTheDocument();
    expect(group.queryByText("Running")).not.toBeInTheDocument();
    expect(log.querySelector(".animate-spin")).toBeNull();
  });

  it("keeps a live session's pending tool spinning", async () => {
    const manual = makeManualStream();
    render(
      <SessionConversationImpl
        sessionId="ags-items-live"
        isTerminal={false}
        phase="running"
        transport={createAgentSessionTransport({
          sessionId: "ags-items-live",
          mintTicket,
          fetch: manual.fetch,
        })}
      />,
    );
    await act(async () => {
      for (const chunk of ITEM_REPLAY) manual.push(chunk);
    });
    await waitFor(() =>
      expect(screen.getByText("Tests pass now.")).toBeInTheDocument(),
    );
    const group = screen
      .getByRole("log")
      .querySelector("[data-block='activity']") as HTMLElement;
    expect(group.querySelector(".animate-spin")).not.toBeNull();
    await userEvent.click(within(group).getByRole("button"));
    expect(within(group).getByText("Running")).toBeInTheDocument();
    expect(within(group).queryByText("Interrupted")).not.toBeInTheDocument();
  });
});
