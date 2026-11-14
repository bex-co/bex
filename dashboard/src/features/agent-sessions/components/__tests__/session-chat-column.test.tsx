import { afterEach, describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionChatColumn } from "@/features/agent-sessions/components/session-chat-column";
import type {
  AgentSessionPhase,
  AgentSessionView,
  QueuedMessageView,
} from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

// Isolate the column's own logic (provisioning gate + optimistic echo). The
// heavy/collaborating children are stubbed: the conversation renders its footer
// prop so we can inspect what the column injects, the composer exposes a button
// that drives the optimistic-steer callback, and the header/PR/failure children
// are inert.
vi.mock("@/features/agent-sessions/components/session-detail-header", () => ({
  SessionDetailHeader: () => <div data-testid="header" />,
}));
vi.mock("@/features/agent-sessions/components/session-conversation", () => ({
  SessionConversation: ({ footer }: { footer: React.ReactNode }) => (
    <div data-testid="conversation">{footer}</div>
  ),
}));
vi.mock("@/features/agent-sessions/components/steering-composer", () => ({
  SteeringComposer: ({
    onOptimisticSteer,
    queuedCount,
  }: {
    onOptimisticSteer?: (p: string | null) => void;
    queuedCount?: number;
  }) => (
    <>
      <button
        type="button"
        data-queued-count={queuedCount}
        onClick={() => onOptimisticSteer?.("echoed prompt")}
      >
        echo
      </button>
      <button type="button" onClick={() => onOptimisticSteer?.(null)}>
        withdraw
      </button>
    </>
  ),
}));

// The queue read (ADR047 D12) is its own GraphQL query; the column's logic is
// what it does with the messages. The list renders its messages' prompts.
const queue = vi.hoisted(() => ({ messages: [] as QueuedMessageView[] }));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-queue", () => ({
  useAgentSessionQueue: () => queue.messages,
}));
vi.mock("@/features/agent-sessions/components/queued-messages", () => ({
  QueuedMessageList: ({ messages }: { messages: QueuedMessageView[] }) =>
    messages.length > 0 ? (
      <ul data-testid="queue">
        {messages.map((m) => (
          <li key={m.id}>{m.prompt}</li>
        ))}
      </ul>
    ) : null,
}));
vi.mock(
  "@/features/agent-sessions/components/structured-result-callout",
  () => ({ StructuredResultCallout: () => null }),
);
vi.mock("@/features/agent-sessions/components/session-spend-note", () => ({
  SessionSpendNote: () => null,
}));
vi.mock("@/features/agent-sessions/components/failure-callout", () => ({
  FailureCallout: () => null,
}));
vi.mock("@/features/agent-sessions/components/delivery-note-callout", () => ({
  DeliveryNoteCallout: () => null,
}));

vi.mock("@/features/agent-sessions/components/child-session-cards", () => ({
  ChildSessionCards: ({ childSessions }: { childSessions: unknown[] }) =>
    childSessions.length > 0 ? (
      <div data-testid="cards">{childSessions.length}</div>
    ) : null,
}));

function view(
  phase: AgentSessionPhase,
  over: Partial<AgentSessionView> = {},
): AgentSessionView {
  return agentSessionView({ phase, task: "t", ...over });
}

const noop = () => {};

function column(session: AgentSessionView) {
  return (
    <SessionChatColumn
      session={session}
      chat={null}
      onChatStateChange={noop}
      onChanged={noop}
    />
  );
}

describe("SessionChatColumn provisioning gate (w2/m64)", () => {
  it("shows the provisioning placeholder, not the stream, while a new session has no sandbox", () => {
    render(column(view("creating", { sandboxId: null })));
    expect(screen.getByText("Starting the sandbox…")).toBeInTheDocument();
    // The conversation stream must NOT mount — attaching with no sandbox 409s.
    expect(screen.queryByTestId("conversation")).not.toBeInTheDocument();
  });

  it("mounts the conversation stream once a sandbox id exists", () => {
    render(column(view("running", { sandboxId: "sandbox-1" })));
    expect(screen.getByTestId("conversation")).toBeInTheDocument();
    expect(screen.queryByText("Starting the sandbox…")).not.toBeInTheDocument();
  });

  it("replays the conversation for a terminal session whose sandbox is gone (ADR065 D2)", () => {
    // Before w2/m70 this rendered no conversation at all — the durable
    // transcript existed but the gate keyed on sandboxId. A reaped terminal
    // session now mounts the stream, which attaches via a replay-only ticket.
    render(column(view("failed", { sandboxId: null })));
    expect(screen.queryByText("Starting the sandbox…")).not.toBeInTheDocument();
    expect(screen.getByTestId("conversation")).toBeInTheDocument();
  });

  it("replays the conversation for a session closed after its idle TTL (ADR065 D2)", () => {
    render(column(view("completed", { sandboxId: null, turns: 2 })));
    expect(screen.getByTestId("conversation")).toBeInTheDocument();
  });

  it("shows provisioning, not the stream, while redispatching without a sandbox", () => {
    render(column(view("redispatching", { sandboxId: "", turns: 2 })));
    expect(screen.getByText("Starting the sandbox…")).toBeInTheDocument();
    expect(screen.queryByTestId("conversation")).not.toBeInTheDocument();
  });
});

describe("SessionChatColumn archived gate (ADR065 D1)", () => {
  it("hides the steering composer on an archived session", () => {
    const archived = view("completed", {
      sandboxId: null,
      archivedAt: new Date().toISOString(),
      isArchived: true,
    });
    render(column(archived));
    // The composer stub renders as the "echo" button; archived sessions refuse
    // steer/resume (AGENT_SESSION_ARCHIVED), so no dead input is offered.
    expect(
      screen.queryByRole("button", { name: "echo" }),
    ).not.toBeInTheDocument();
    // The conversation still replays — viewable is the point of the archive.
    expect(screen.getByTestId("conversation")).toBeInTheDocument();
  });
});

describe("SessionChatColumn optimistic redispatch echo (w2/m64)", () => {
  it("echoes the steered prompt immediately, then hides it once the turn settles", async () => {
    const user = userEvent.setup();
    const idle = view("completed", { sandboxId: "sandbox-1", turns: 1 });
    const { rerender } = render(column(idle));

    // Before submit: no echo.
    expect(screen.queryByText("echoed prompt")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "echo" }));
    // The echo appears immediately, inside the (mocked) conversation footer.
    expect(screen.getByText("echoed prompt")).toBeInTheDocument();

    // Re-dispatched turn accepted: the keyed conversation remount replays the
    // durable prompt, so the optimistic echo is withdrawn.
    rerender(column(view("completed", { sandboxId: "sandbox-2", turns: 2 })));
    expect(screen.queryByText("echoed prompt")).not.toBeInTheDocument();
  });

  it("keeps the echo visible until the durable turn count advances", async () => {
    const user = userEvent.setup();
    const idle = view("completed", { sandboxId: "sandbox-1", turns: 1 });
    const { rerender } = render(column(idle));

    await user.click(screen.getByRole("button", { name: "echo" }));
    expect(screen.getByText("echoed prompt")).toBeInTheDocument();

    // A phase-only transition with no accepted-turn increment must not hide it.
    rerender(column(view("redispatching", { sandboxId: "", turns: 2 })));
    expect(screen.getByText("echoed prompt")).toBeInTheDocument();
  });

  it("does not bring the echo back when the idle sandbox later closes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      column(view("completed", { sandboxId: "", turns: 3 })),
    );

    await user.click(screen.getByRole("button", { name: "echo" }));
    rerender(column(view("running", { sandboxId: "sandbox-2", turns: 4 })));
    rerender(column(view("completed", { sandboxId: "sandbox-2", turns: 4 })));
    expect(screen.queryByText("echoed prompt")).not.toBeInTheDocument();

    // After the 2h idle close the sandbox id is cleared; the replayed durable
    // prompt is the only copy, so the optimistic echo must stay hidden.
    rerender(column(view("completed", { sandboxId: "", turns: 4 })));
    expect(screen.queryByText("echoed prompt")).not.toBeInTheDocument();
  });
});

describe("SessionChatColumn lineage (ADR102)", () => {
  it("adds the panel region and child cards only for a lineage session", () => {
    const base = view("running", { sandboxId: "sandbox-1" });
    const { rerender } = render(column(base));
    expect(screen.queryByTestId("panel")).not.toBeInTheDocument();
    expect(screen.queryByTestId("cards")).not.toBeInTheDocument();

    rerender(
      <SessionChatColumn
        session={base}
        chat={null}
        onChatStateChange={noop}
        onChanged={noop}
        childSessions={[view("running", { id: "c1" })]}
        agentsPanel={<aside data-testid="panel" />}
      />,
    );
    expect(screen.getByTestId("cards")).toHaveTextContent("1");
    // The panel precedes the conversation in the DOM (stacked above it on
    // narrow screens) and shares the row wrapper with it.
    const panel = screen.getByTestId("panel");
    const conversation = screen.getByTestId("conversation");
    expect(
      panel.compareDocumentPosition(conversation) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(panel.parentElement).toHaveClass("lg:flex-row");
  });
});

describe("SessionChatColumn message queue (ADR047 D12)", () => {
  const waiting = (id: string, prompt: string, position: number) => ({
    id,
    position,
    prompt,
    author: "alice",
    attachmentCount: 0,
    blockedReason: null,
    createdAt: null,
  });

  afterEach(() => {
    queue.messages = [];
  });

  it("shows no queue region for a session with nothing queued", () => {
    render(column(view("running", { sandboxId: "sandbox-1" })));
    expect(screen.queryByTestId("queue")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "echo" })).toHaveAttribute(
      "data-queued-count",
      "0",
    );
  });

  it("lists the queued messages above the composer and tells it how many wait", () => {
    queue.messages = [
      waiting("aqm-1", "run the linter", 1),
      waiting("aqm-2", "then the tests", 2),
    ];
    render(column(view("running", { sandboxId: "sandbox-1" })));

    const list = screen.getByTestId("queue");
    expect(list).toHaveTextContent("run the linter");
    expect(list).toHaveTextContent("then the tests");
    const composer = screen.getByRole("button", { name: "echo" });
    expect(composer).toHaveAttribute("data-queued-count", "2");
    // Above the composer in document order.
    expect(
      list.compareDocumentPosition(composer) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("keeps the queue and composer off an archived session", () => {
    queue.messages = [waiting("aqm-1", "run the linter", 1)];
    render(
      column(
        view("completed", {
          sandboxId: null,
          archivedAt: new Date().toISOString(),
          isArchived: true,
        }),
      ),
    );
    expect(screen.queryByTestId("queue")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "echo" }),
    ).not.toBeInTheDocument();
  });

  it("takes the optimistic echo back when the server queued the message instead", async () => {
    const user = userEvent.setup();
    render(column(view("completed", { sandboxId: "sandbox-1", turns: 1 })));
    await user.click(screen.getByRole("button", { name: "echo" }));
    expect(screen.getByText("echoed prompt")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "withdraw" }));
    expect(screen.queryByText("echoed prompt")).not.toBeInTheDocument();
  });
});
