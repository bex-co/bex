import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { SessionConversation } from "@/features/agent-sessions/components/session-conversation";

// The wrapper reads which turns Stop turn ended (ADR047 D12) and hands them to
// the stream-only impl, which has no Apollo of its own.

const attach = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ attach }),
}));

const interrupted = vi.hoisted(() => ({
  turns: new Set<number>([2]),
  calls: [] as Array<[string, string | undefined]>,
}));
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-interrupted-turns",
  () => ({
    useAgentSessionInterruptedTurns: (id: string, key: string | undefined) => {
      interrupted.calls.push([id, key]);
      return interrupted.turns;
    },
  }),
);

vi.mock(
  "@/features/agent-sessions/components/session-conversation-impl",
  () => ({
    SessionConversationImpl: ({
      interruptedTurns,
    }: {
      interruptedTurns: ReadonlySet<number>;
    }) => <div data-testid="impl">{[...interruptedTurns].join(",")}</div>,
  }),
);

describe("SessionConversation", () => {
  it("passes the interrupted turns to the conversation, keyed by the attach signal", async () => {
    render(
      <SessionConversation
        sessionId="as-1"
        isTerminal
        phase="completed"
        attachSignal="3:sbx-1:true"
      />,
    );
    expect(await screen.findByTestId("impl")).toHaveTextContent("2");
    expect(interrupted.calls.at(-1)).toEqual(["as-1", "3:sbx-1:true"]);
  });
});
