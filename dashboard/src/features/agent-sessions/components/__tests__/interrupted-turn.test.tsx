import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import type { UIMessageChunk } from "ai";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import { SessionConversationImpl } from "@/features/agent-sessions/components/session-conversation-impl";
import { TurnInterruptedNote } from "@/features/agent-sessions/components/turn-interrupted-note";
import { createAgentSessionTransport } from "@/features/agent-sessions/lib/transport";
import { makeFixtureFetch } from "@/features/agent-sessions/lib/mock-stream";
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

function renderConversation(
  transcript: UIMessageChunk[],
  interruptedTurns?: ReadonlySet<number>,
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
    renderConversation(THREE_TURNS, new Set([2]));

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
    renderConversation(THREE_TURNS, new Set([3]));
    const note = await screen.findByText(NOTE(3));
    expect(before(screen.getByText("linter is clean"), note)).toBe(true);
    expect(screen.getAllByTestId("turn-interrupted")).toHaveLength(1);
  });

  it("marks every interrupted turn, each once", async () => {
    renderConversation(THREE_TURNS, new Set([1, 3]));
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
    renderConversation(THREE_TURNS, new Set([2]));
    const note = await screen.findByTestId("turn-interrupted");
    expect(note).not.toHaveClass("text-destructive");
    expect(note.querySelector(".text-destructive")).toBeNull();
    expect(screen.queryByText(/failed/i)).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("is marked on a single-turn session that was stopped", async () => {
    renderConversation(
      THREE_TURNS.slice(0, 5).concat({ type: "finish" }),
      new Set([1]),
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
