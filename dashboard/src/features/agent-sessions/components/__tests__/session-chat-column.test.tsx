import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionChatColumn } from "@/features/agent-sessions/components/session-chat-column";
import type {
  AgentSessionPhase,
  AgentSessionView,
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
  }: {
    onOptimisticSteer?: (p: string | null) => void;
  }) => (
    <button type="button" onClick={() => onOptimisticSteer?.("echoed prompt")}>
      echo
    </button>
  ),
}));
vi.mock("@/features/agent-sessions/components/failure-callout", () => ({
  FailureCallout: () => null,
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

  it("replays the conversation for a hibernated session (ADR065 D2)", () => {
    render(column(view("hibernated", { sandboxId: null })));
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
    rerender(
      column(view("redispatching", { sandboxId: "", turns: 2 })),
    );
    expect(screen.getByText("echoed prompt")).toBeInTheDocument();
  });
});
