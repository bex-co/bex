import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DeliveryNoteCallout } from "@/features/agent-sessions/components/delivery-note-callout";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

function view(
  phase: AgentSessionPhase,
  over: Partial<AgentSessionView> = {},
): AgentSessionView {
  return agentSessionView({ phase, task: "add a helper", ...over });
}

describe("DeliveryNoteCallout", () => {
  it("renders nothing when the agent delivered its work", () => {
    const { container } = render(
      <DeliveryNoteCallout session={view("completed")} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the recorded note and the follow-up next step", () => {
    render(
      <DeliveryNoteCallout
        session={view("completed", {
          deliveryNote: "2 local commits were not pushed",
        })}
      />,
    );
    expect(screen.getByText("Changes not fully delivered")).toBeInTheDocument();
    expect(
      screen.getByText("2 local commits were not pushed"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Send a follow-up message/)).toBeInTheDocument();
  });

  it("defers to the failure callout on a failed session", () => {
    const { container } = render(
      <DeliveryNoteCallout
        session={view("failed", { deliveryNote: "tree is dirty" })}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
