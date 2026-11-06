import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { SessionSpendNote } from "@/features/agent-sessions/components/session-spend-note";
import { agentSessionView } from "@/test/mocks/agent-session";

const outcome = (modelUSD: string | null) => ({
  state: "idle",
  steerable: true,
  turn: 1,
  summary: null,
  deliveryNote: null,
  waitingFor: null,
  pendingDecision: null,
  failureReasonCode: null,
  result: null,
  cost: modelUSD ? { modelUSD } : null,
});

describe("SessionSpendNote", () => {
  it("renders nothing when the session made no priced request", () => {
    const { container } = render(
      <SessionSpendNote
        session={agentSessionView({ outcome: outcome(null) })}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the exact micro-dollar spend", () => {
    render(
      <SessionSpendNote
        session={agentSessionView({ outcome: outcome("0.001234") })}
      />,
    );
    expect(screen.getByText("Model spend $0.001234")).toBeInTheDocument();
  });

  it("shows the spend beside the session limit", () => {
    render(
      <SessionSpendNote
        session={agentSessionView({
          maxSpendUSD: "5",
          outcome: outcome("0.42"),
        })}
      />,
    );
    expect(
      screen.getByText("Model spend $0.42 of the $5.00 session limit"),
    ).toBeInTheDocument();
  });
});
