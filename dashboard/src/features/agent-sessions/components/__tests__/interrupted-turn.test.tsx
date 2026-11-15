import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import type { UIMessageChunk } from "ai";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { TurnInterruptedNote } from "@/features/agent-sessions/components/turn-interrupted-note";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import { makeFixtureFetch } from "@/features/agent-sessions/lib/mock-stream";
import {
  TURN_RETENTION_STATES,
  type InterruptedTurns,
  type TurnRetentionView,
} from "@/features/agent-sessions/types";
import i18n from "@/i18n/init";
import { agentSessionView } from "@/test/mocks/agent-session";

// A turn a person stopped with Stop turn (ADR047 D12) settles as a completed
// one, flagged `interrupted`: the transcript says so in a neutral line, and no
// failure shows anywhere.

vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-turn-skills",
  () => ({ useAgentSessionTurnSkills: () => [] }),
);
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-turn-origins",
  () => ({ useAgentSessionTurnOrigins: () => [] }),
);
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ steer: vi.fn() }),
}));
vi.mock("@tanstack/react-router", async (importActual) => ({
  ...(await importActual<typeof import("@tanstack/react-router")>()),
  useNavigate: () => vi.fn(),
}));

const mintTicket = async () => ({ ticket: "fixture-ticket" });

const NOTE = (turn: number) =>
  `Turn ${turn} was interrupted before it finished.`;

/** Three turns: 1 ran to the end, 2 was stopped mid-reply, 3 followed it. */
const THREE_TURNS = [
  { type: "start", messageId: "asm-stop" },
  {
    type: "data-user-prompt",
    data: { turn: 1, text: "write the parser", complete: true },
  },
  { type: "text-start", id: "t1" },
  { type: "text-delta", id: "t1", delta: "parser written" },
  { type: "text-end", id: "t1" },
  {
    type: "data-user-prompt",
    data: { turn: 2, text: "now the whole test suite", complete: true },
  },
  { type: "text-start", id: "t2" },
  { type: "text-delta", id: "t2", delta: "half a reply before the stop" },
  { type: "text-end", id: "t2" },
  {
    type: "data-user-prompt",
    data: { turn: 3, text: "just run the linter", complete: true },
  },
  { type: "text-start", id: "t3" },
  { type: "text-delta", id: "t3", delta: "linter is clean" },
  { type: "text-end", id: "t3" },
  { type: "finish" },
] as UIMessageChunk[];

const NO_RETENTION: TurnRetentionView = {
  state: "none",
  until: null,
  lostReason: null,
};

/** Stopped turns whose disk kept nothing: the note alone. */
const stopped = (...turns: number[]): InterruptedTurns =>
  new Map(turns.map((turn) => [turn, NO_RETENTION]));

function renderConversation(
  transcript: UIMessageChunk[],
  interruptedTurns?: InterruptedTurns,
) {
  const transport = createAgentSessionTransport({
    sessionId: "as-stop",
    mintTicket,
    fetch: makeFixtureFetch(transcript, { terminal: true }),
  });
  return render(
    <SessionConversationImpl
      sessionId="as-stop"
      isTerminal
      transport={transport}
      interruptedTurns={interruptedTurns}
    />,
  );
}

/** True when `a` comes before `b` in the page. */
function before(a: HTMLElement, b: HTMLElement) {
  return Boolean(
    a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING,
  );
}

describe("an interrupted turn in the transcript", () => {
  it("is marked where it ended, between its reply and the next prompt", async () => {
    renderConversation(THREE_TURNS, stopped(2));

    const note = await screen.findByText(NOTE(2));
    expect(screen.getAllByTestId("turn-interrupted")).toHaveLength(1);
    expect(before(screen.getByText(/half a reply before the stop/), note)).toBe(
      true,
    );
    expect(before(note, screen.getByText("just run the linter"))).toBe(true);
    // The other turns carry no mark.
    expect(screen.queryByText(NOTE(1))).toBeNull();
    expect(screen.queryByText(NOTE(3))).toBeNull();
  });

  it("is marked after the last reply when it was the final turn", async () => {
    renderConversation(THREE_TURNS, stopped(3));
    const note = await screen.findByText(NOTE(3));
    expect(before(screen.getByText("linter is clean"), note)).toBe(true);
    expect(screen.getAllByTestId("turn-interrupted")).toHaveLength(1);
  });

  it("marks every interrupted turn, each once", async () => {
    renderConversation(THREE_TURNS, stopped(1, 3));
    await screen.findByText(NOTE(1));
    expect(screen.getByText(NOTE(3))).toBeInTheDocument();
    expect(screen.getAllByTestId("turn-interrupted")).toHaveLength(2);
  });

  it("marks nothing when no turn was interrupted", async () => {
    renderConversation(THREE_TURNS);
    await screen.findByText("linter is clean");
    expect(screen.queryByTestId("turn-interrupted")).toBeNull();
  });

  it("does not read as a failure", async () => {
    renderConversation(THREE_TURNS, stopped(2));
    const note = await screen.findByTestId("turn-interrupted");
    expect(note).not.toHaveClass("text-destructive");
    expect(note.querySelector(".text-destructive")).toBeNull();
    expect(screen.queryByText(/failed/i)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("is marked on a single-turn session that was stopped", async () => {
    renderConversation(
      THREE_TURNS.slice(0, 5).concat({ type: "finish" }),
      stopped(1),
    );
    await waitFor(() => expect(screen.getByText(NOTE(1))).toBeInTheDocument());
    expect(screen.getByText("parser written")).toBeInTheDocument();
  });
});

describe("TurnInterruptedNote", () => {
  it("names the turn in a muted system line", () => {
    render(<TurnInterruptedNote turn={4} />);
    expect(screen.getByText(NOTE(4))).toBeInTheDocument();
    expect(screen.getByTestId("turn-interrupted")).toHaveClass(
      "text-muted-foreground",
    );
  });
});

describe("the failure callout after Stop turn", () => {
  it("shows no failure for the completed session an interrupted turn leaves", () => {
    // Stop turn settles the session as completed (phase) with the turn flagged,
    // never as failed, so the callout has nothing to say.
    const { container } = render(
      <FailureCallout
        session={agentSessionView({
          phase: "completed",
          state: "succeeded",
          failureReason: null,
        })}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});

// w22/m413 t004: under the note, what became of the unpushed work of the turn
// a person stopped.
describe("what became of a stopped turn's unpushed work", () => {
  const retention = (over: Partial<TurnRetentionView>): TurnRetentionView => ({
    ...NO_RETENTION,
    ...over,
  });
  const KEPT = retention({ state: "kept", until: "2026-11-16T10:00:00Z" });
  const LINE = () => screen.getByTestId("turn-retention");

  it("says until when a kept disk is kept, at the viewer's own time", () => {
    render(<TurnInterruptedNote turn={2} retention={KEPT} />);
    // The tests run in UTC; the line carries the viewer's clock.
    expect(LINE()).toHaveTextContent(
      "Your changes are kept until November 16, 2026 at 10:00 AM.",
    );
    expect(LINE()).toHaveAttribute("data-retention", "kept");
    // The note itself is unchanged and still comes first.
    expect(screen.getByText(NOTE(2))).toBeInTheDocument();
  });

  it("names no time before hydration: the server's clock is not the viewer's", () => {
    const html = renderToString(
      <TurnInterruptedNote turn={2} retention={KEPT} />,
    );
    expect(html).toContain("Your changes are kept for your next message.");
    expect(html).not.toMatch(/2026|10:00/);
  });

  it("says a lost disk was lost when the retained sandbox expired", () => {
    render(
      <TurnInterruptedNote
        turn={2}
        retention={retention({ state: "lost", lostReason: "expired" })}
      />,
    );
    expect(LINE()).toHaveTextContent(
      "Your unpushed changes were lost when the retained sandbox expired.",
    );
    expect(LINE()).toHaveAttribute("data-retention", "lost");
  });

  it.each([
    [
      "storage_full",
      "Your unpushed changes were not kept because the workspace's sandbox storage was full.",
    ],
    ["save_failed", "Your unpushed changes could not be saved and were lost."],
    [
      "editor_open",
      "Your unpushed changes were not kept because an editor was open on the sandbox.",
    ],
    [
      "not_restored",
      "Your unpushed changes could not be restored, so the next message started from a fresh checkout.",
    ],
    ["canceled", "Your unpushed changes were lost."],
    ["archived", "Your unpushed changes were lost."],
    [null, "Your unpushed changes were lost."],
  ] as const)("says why a disk lost to %s was lost", (lostReason, sentence) => {
    render(
      <TurnInterruptedNote
        turn={2}
        retention={retention({ state: "lost", lostReason })}
      />,
    );
    expect(LINE()).toHaveTextContent(sentence);
  });

  it("says a disk is being saved, and that the next message continued from one", () => {
    const { rerender } = render(
      <TurnInterruptedNote
        turn={2}
        retention={retention({ state: "saving" })}
      />,
    );
    expect(LINE()).toHaveTextContent(
      "Saving your changes so the next message can continue from them.",
    );
    rerender(
      <TurnInterruptedNote
        turn={2}
        retention={retention({ state: "continued" })}
      />,
    );
    expect(LINE()).toHaveTextContent(
      "The next message continued from these changes.",
    );
  });

  it("adds nothing for a turn that kept nothing, or with no retention read", () => {
    const { rerender } = render(
      <TurnInterruptedNote turn={2} retention={NO_RETENTION} />,
    );
    expect(screen.queryByTestId("turn-retention")).toBeNull();
    expect(screen.getByText(NOTE(2))).toBeInTheDocument();
    rerender(<TurnInterruptedNote turn={2} />);
    expect(screen.queryByTestId("turn-retention")).toBeNull();
  });

  it("never reads as a failure, whichever state it is in", () => {
    for (const state of TURN_RETENTION_STATES) {
      const { unmount } = render(
        <TurnInterruptedNote
          turn={2}
          retention={retention({
            state,
            until: state === "kept" ? KEPT.until : null,
            lostReason: state === "lost" ? "save_failed" : null,
          })}
        />,
      );
      const note = screen.getByTestId("turn-interrupted");
      expect(note).toHaveClass("text-muted-foreground");
      expect(note.querySelector(".text-destructive")).toBeNull();
      expect(screen.queryByRole("alert")).toBeNull();
      unmount();
    }
  });

  it("sits under the note it belongs to, between the reply and the next prompt", async () => {
    renderConversation(THREE_TURNS, new Map([[2, KEPT]]));
    const note = await screen.findByText(NOTE(2));
    const line = await screen.findByText(
      "Your changes are kept until November 16, 2026 at 10:00 AM.",
    );
    expect(before(note, line)).toBe(true);
    expect(before(line, screen.getByText("just run the linter"))).toBe(true);
    // Only the stopped turn has one.
    expect(screen.getAllByTestId("turn-retention")).toHaveLength(1);
  });

  it("reads in the viewer's language, with the time they would read it in", async () => {
    await i18n.changeLanguage("zh");
    try {
      render(<TurnInterruptedNote turn={2} retention={KEPT} />);
      expect(LINE()).toHaveTextContent("你的更改将保留到 2026年11月16日");
      expect(LINE()).not.toHaveTextContent("Your changes");
    } finally {
      await i18n.changeLanguage("en");
    }
  });

  it("lays out like the note it extends: one narrow column that wraps instead of overflowing", () => {
    render(
      <TurnInterruptedNote
        turn={2}
        retention={retention({ state: "lost", lostReason: "storage_full" })}
      />,
    );
    const note = screen.getByTestId("turn-interrupted");
    // The text column shrinks (min-w-0) beside the fixed-size icon, so a long
    // sentence wraps at a phone's width and never pushes the page sideways.
    const column = LINE().parentElement!;
    expect(column).toHaveClass("min-w-0");
    expect(column.parentElement).toBe(note);
    expect(note).toHaveClass("flex", "items-start", "gap-2", "text-xs");
  });
});
