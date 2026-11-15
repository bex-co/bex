import { describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UIMessageChunk } from "ai";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import {
  makeFixtureFetch,
  makeManualStream,
} from "@/features/agent-sessions/lib/mock-stream";

// The turn-skill pills (ADR099) and steer origins (w9/m344) read GraphQL;
// these suites replay the stream without an Apollo client and cover neither.
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-turn-skills",
  () => ({
    useAgentSessionTurnSkills: () => [],
  }),
);
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-turn-origins",
  () => ({
    useAgentSessionTurnOrigins: () => [],
  }),
);

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
    // The thought before the calls opens their group instead of standing alone.
    expect(log.querySelectorAll("[data-block='reasoning']")).toHaveLength(0);
    const text = log.querySelectorAll("[data-block='text']");
    expect(text).toHaveLength(1);
    expect(text[0]).toHaveTextContent("Tests pass now.");
    const groups = log.querySelectorAll("[data-block='activity']");
    expect(groups).toHaveLength(1);
    expect(groups[0].getAttribute("data-steps")).toBe("4");
    expect(groups[0].getAttribute("data-thoughts")).toBe("1");
    expect(groups[0]).toHaveTextContent("Ran 4 commands · Thought 1 time");
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

  it("folds the thoughts between calls into the calls' one group", async () => {
    // The think-act rhythm an ACP agent streams (thought, call, thought, call)
    // reads as one line between two pieces of narration, as Devin folds its
    // activity rows; a thought with no call beside it stays its own block.
    const reasoning = (id: string, delta: string): UIMessageChunk[] => [
      { type: "reasoning-start", id },
      { type: "reasoning-delta", id, delta },
      { type: "reasoning-end", id },
    ];
    const done = (id: string): UIMessageChunk =>
      ({
        type: "tool-output-available",
        toolCallId: id,
        output: "ok",
        dynamic: true,
      }) as UIMessageChunk;
    render(
      <SessionConversationImpl
        sessionId="ags-think-act"
        isTerminal
        phase="completed"
        transport={createAgentSessionTransport({
          sessionId: "ags-think-act",
          mintTicket,
          fetch: makeFixtureFetch(
            [
              { type: "start", messageId: "durable-ags-think-act" },
              { type: "text-start", id: "x0" },
              { type: "text-delta", id: "x0", delta: "Scanning." },
              { type: "text-end", id: "x0" },
              ...reasoning("r1", "Start with the callers."),
              tool("a", "grep -rn foo"),
              done("a"),
              ...reasoning("r2", "Now the definition."),
              ...reasoning("r3", "And its tests."),
              tool("b", "sed -n 1,20p foo.go"),
              done("b"),
              ...reasoning("r4", "That settles it."),
              { type: "text-start", id: "x1" },
              { type: "text-delta", id: "x1", delta: "Found it." },
              { type: "text-end", id: "x1" },
              ...reasoning("r5", "Nothing else to check."),
              { type: "finish" },
            ],
            { terminal: true },
          ),
        })}
      />,
    );
    await waitFor(() =>
      expect(screen.getByText("Session ended.")).toBeInTheDocument(),
    );
    const log = screen.getByRole("log");
    const groups = log.querySelectorAll("[data-block='activity']");
    expect(groups).toHaveLength(1);
    expect(groups[0].getAttribute("data-steps")).toBe("2");
    expect(groups[0].getAttribute("data-thoughts")).toBe("4");
    expect(groups[0]).toHaveTextContent("Ran 2 commands · Thought 4 times");
    // Only the thought after the narration, with no call beside it, stands
    // alone.
    const standalone = log.querySelectorAll("[data-block='reasoning']");
    expect(standalone).toHaveLength(1);
    await userEvent.click(
      within(standalone[0] as HTMLElement).getByRole("button"),
    );
    expect(screen.getByText("Nothing else to check.")).toBeInTheDocument();

    // Expanded, the timeline keeps the order the agent worked in.
    const group = groups[0] as HTMLElement;
    await userEvent.click(within(group).getAllByRole("button")[0]);
    const steps = [...group.querySelectorAll("ol > li")].map((li) =>
      li.querySelector("[data-step='thought']") ? "thought" : "call",
    );
    expect(steps).toEqual([
      "thought",
      "call",
      "thought",
      "thought",
      "call",
      "thought",
    ]);
    expect(screen.queryByText("Now the definition.")).not.toBeInTheDocument();
    await userEvent.click(within(group).getAllByText("Thought")[1]);
    expect(screen.getByText("Now the definition.")).toBeInTheDocument();
  });

  it("says Thinking while a thought streams after a call", async () => {
    const manual = makeManualStream();
    render(
      <SessionConversationImpl
        sessionId="ags-thinking"
        isTerminal={false}
        phase="running"
        transport={createAgentSessionTransport({
          sessionId: "ags-thinking",
          mintTicket,
          fetch: manual.fetch,
        })}
      />,
    );
    await act(async () => {
      manual.push({ type: "start", messageId: "durable-ags-thinking" });
      manual.push(tool("t1", "go test ./..."));
      manual.push({
        type: "tool-output-available",
        toolCallId: "t1",
        output: "ok",
        dynamic: true,
      } as UIMessageChunk);
    });
    await waitFor(() =>
      expect(screen.getByLabelText("Agent status")).toHaveTextContent(
        "go test ./...",
      ),
    );
    const group = () =>
      screen
        .getByRole("log")
        .querySelector("[data-block='activity']") as HTMLElement;
    expect(group().querySelector(".animate-spin")).toBeNull();

    // The thought streams into the call's group: the status line says so and
    // the group spins until the next call lands.
    await act(async () => {
      manual.push({ type: "reasoning-start", id: "r1" });
      manual.push({ type: "reasoning-delta", id: "r1", delta: "Hmm." });
    });
    await waitFor(() =>
      expect(screen.getByLabelText("Agent status")).toHaveTextContent(
        "Thinking…",
      ),
    );
    expect(group().getAttribute("data-thoughts")).toBe("1");
    expect(group().querySelector(".animate-spin")).not.toBeNull();

    await act(async () => {
      manual.push({ type: "reasoning-end", id: "r1" });
      manual.push(tool("t2", "make lint"));
    });
    await waitFor(() =>
      expect(screen.getByLabelText("Agent status")).toHaveTextContent(
        "make lint",
      ),
    );
    expect(
      screen.getByRole("log").querySelectorAll("[data-block='activity']"),
    ).toHaveLength(1);
  });
});
