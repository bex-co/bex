import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { SessionSpendNote } from "@/features/agent-sessions/components/session-spend-note";
import type { AgentSessionOutcome } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

const outcome = (cost: AgentSessionOutcome["cost"]): AgentSessionOutcome => ({
  state: "idle",
  steerable: true,
  turn: 1,
  summary: null,
  deliveryNote: null,
  failureReasonCode: null,
  result: null,
  cost,
});

// ADR094 D7 / w9/m178: the total is the headline, with the model and sandbox
// compute parts beside it; the session limit caps the model part only.
describe("SessionSpendNote", () => {
  it("renders nothing when nothing is priced or a part could not be read", () => {
    const { container } = render(
      <SessionSpendNote
        session={agentSessionView({ outcome: outcome(null) })}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the total with its model and compute parts", () => {
    render(
      <SessionSpendNote
        session={agentSessionView({
          outcome: outcome({
            modelUSD: "0.42",
            computeUSD: "0.0712",
            totalUSD: "0.4912",
          }),
        })}
      />,
    );
    expect(
      screen.getByText("Cost $0.4912 · model $0.42, compute $0.0712"),
    ).toBeInTheDocument();
  });

  it("puts the session limit beside the model part it caps", () => {
    render(
      <SessionSpendNote
        session={agentSessionView({
          maxSpendUSD: "5",
          outcome: outcome({
            modelUSD: "0.42",
            computeUSD: "0.00",
            totalUSD: "0.42",
          }),
        })}
      />,
    );
    expect(
      screen.getByText(
        "Cost $0.42 · model $0.42 of the $5.00 session limit, compute $0.00",
      ),
    ).toBeInTheDocument();
  });

  it("falls back to model spend from a server without totalUSD", () => {
    const { rerender } = render(
      <SessionSpendNote
        session={agentSessionView({
          outcome: outcome({
            modelUSD: "0.001234",
            computeUSD: null,
            totalUSD: null,
          }),
        })}
      />,
    );
    expect(screen.getByText("Model spend $0.001234")).toBeInTheDocument();
    rerender(
      <SessionSpendNote
        session={agentSessionView({
          maxSpendUSD: "5",
          outcome: outcome({
            modelUSD: "0.42",
            computeUSD: null,
            totalUSD: null,
          }),
        })}
      />,
    );
    expect(
      screen.getByText("Model spend $0.42 of the $5.00 session limit"),
    ).toBeInTheDocument();
  });
});
