import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DecisionCallout } from "@/features/agent-sessions/components/decision-callout";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import type {
  AgentSessionDecisionView,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

const answerDecision = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ answerDecision }),
}));

const toastInfo = vi.fn();
vi.mock("sonner", () => ({
  toast: { info: (m: string) => toastInfo(m), error: vi.fn() },
}));

beforeEach(() => {
  answerDecision.mockReset();
  answerDecision.mockResolvedValue("allowed");
  toastInfo.mockReset();
});

const approval: AgentSessionDecisionView = {
  id: "dcs-1",
  kind: "approval",
  title: "Run (force push): git push -f origin main",
  detail: {
    toolKind: "execute",
    command: "git push -f origin main",
    rule: "force-push",
    paths: ["/etc/hosts"],
  },
  state: "pending",
  createdAt: "2026-11-06T10:00:00Z",
  expiresAt: "2026-11-07T10:00:00Z",
};

const question: AgentSessionDecisionView = {
  id: "dcs-2",
  kind: "question",
  title: "Which database?",
  detail: {
    question: "Which database should the migration target?",
    options: ["staging", "prod"],
  },
  state: "pending",
  createdAt: null,
  expiresAt: null,
};

function paused(
  decision: AgentSessionDecisionView | null,
  over: Partial<AgentSessionView> = {},
): AgentSessionView {
  const base = agentSessionView({ phase: "running" });
  return {
    ...base,
    state: "needs_input",
    waitingFor: decision?.kind === "question" ? "user" : "approval",
    outcome: {
      state: "needs_input",
      steerable: false,
      turn: 1,
      waitingFor: decision?.kind === "question" ? "user" : "approval",
      pendingDecision: decision,
      summary: null,
      deliveryNote: null,
      failureReasonCode: null,
      result: null,
      cost: null,
    },
    ...over,
  };
}

describe("DecisionCallout", () => {
  it("renders nothing unless the session is paused on a pending decision", () => {
    const onAnswered = vi.fn();
    const running = agentSessionView({ phase: "running" });
    const { container, rerender } = render(
      <DecisionCallout session={running} onAnswered={onAnswered} />,
    );
    expect(container).toBeEmptyDOMElement();
    // A decision already answered (the cached copy moved on) shows nothing.
    rerender(
      <DecisionCallout
        session={paused({ ...approval, state: "allowed" })}
        onAnswered={onAnswered}
      />,
    );
    expect(container).toBeEmptyDOMElement();
    // needs_input from a list read carries no decision yet.
    rerender(
      <DecisionCallout session={paused(null)} onAnswered={onAnswered} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows an approval's command, paths and rule, and sends each answer", async () => {
    const user = userEvent.setup();
    const onAnswered = vi.fn();
    render(
      <DecisionCallout session={paused(approval)} onAnswered={onAnswered} />,
    );

    expect(
      screen.getByText("The agent is asking before it acts"),
    ).toBeInTheDocument();
    expect(screen.getByText("git push -f origin main")).toBeInTheDocument();
    expect(screen.getByText("/etc/hosts")).toBeInTheDocument();
    expect(screen.getByText("Matched rule: force-push")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Allow once" }));
    expect(answerDecision).toHaveBeenLastCalledWith(session(), "dcs-1", {
      allow: true,
      scope: "once",
    });
    await waitFor(() => expect(onAnswered).toHaveBeenCalledTimes(1));

    await user.click(
      screen.getByRole("button", { name: "Allow for this session" }),
    );
    expect(answerDecision).toHaveBeenLastCalledWith(session(), "dcs-1", {
      allow: true,
      scope: "session",
    });

    await user.click(screen.getByRole("button", { name: "Deny" }));
    expect(answerDecision).toHaveBeenLastCalledWith(session(), "dcs-1", {
      allow: false,
    });
  });

  it("disables the answers while one is in flight", async () => {
    const user = userEvent.setup();
    let resolve: (state: string) => void = () => {};
    answerDecision.mockReturnValue(
      new Promise<string>((r) => {
        resolve = r;
      }),
    );
    render(<DecisionCallout session={paused(approval)} onAnswered={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Allow once" }));
    expect(screen.getByRole("button", { name: "Deny" })).toBeDisabled();
    resolve("allowed");
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Deny" })).toBeEnabled(),
    );
  });

  it("answers a question with a suggested option or typed text", async () => {
    const user = userEvent.setup();
    render(<DecisionCallout session={paused(question)} onAnswered={vi.fn()} />);
    expect(screen.getByText("The agent has a question")).toBeInTheDocument();
    expect(
      screen.getByText("Which database should the migration target?"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "staging" }));
    expect(answerDecision).toHaveBeenLastCalledWith(session(), "dcs-2", {
      text: "staging",
    });

    const send = screen.getByRole("button", { name: "Send answer" });
    expect(send).toBeDisabled();
    await user.type(
      screen.getByRole("textbox", { name: "Your answer" }),
      "  the replica  ",
    );
    await user.click(send);
    expect(answerDecision).toHaveBeenLastCalledWith(session(), "dcs-2", {
      text: "the replica",
    });
  });

  it("re-reads the session with a notice when someone else already answered", async () => {
    const user = userEvent.setup();
    const onAnswered = vi.fn();
    answerDecision.mockRejectedValue(
      new AgentSessionError(
        "AGENT_SESSION_DECISION_ANSWERED",
        "decision already answered",
        {},
      ),
    );
    render(
      <DecisionCallout session={paused(approval)} onAnswered={onAnswered} />,
    );
    await user.click(screen.getByRole("button", { name: "Deny" }));
    await waitFor(() =>
      expect(toastInfo).toHaveBeenCalledWith("Someone already answered this."),
    );
    expect(onAnswered).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("agent-decision-error")).toBeNull();
  });

  it("treats an expired decision the same way", async () => {
    const user = userEvent.setup();
    const onAnswered = vi.fn();
    answerDecision.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_DECISION_EXPIRED", "expired", {}),
    );
    render(
      <DecisionCallout session={paused(approval)} onAnswered={onAnswered} />,
    );
    await user.click(screen.getByRole("button", { name: "Allow once" }));
    await waitFor(() =>
      expect(toastInfo).toHaveBeenCalledWith(
        "This decision is no longer open.",
      ),
    );
    expect(onAnswered).toHaveBeenCalledTimes(1);
  });

  it("keeps the decision open with an inline error for an invalid answer", async () => {
    const user = userEvent.setup();
    const onAnswered = vi.fn();
    answerDecision.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_DECISION_INVALID", "bad answer", {}),
    );
    render(
      <DecisionCallout session={paused(question)} onAnswered={onAnswered} />,
    );
    await user.click(screen.getByRole("button", { name: "prod" }));
    expect(await screen.findByTestId("agent-decision-error")).toHaveTextContent(
      "That answer doesn't fit this decision. Try again.",
    );
    expect(onAnswered).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "prod" })).toBeEnabled();
  });
});

function session(): string {
  return agentSessionView({ phase: "running" }).id;
}
