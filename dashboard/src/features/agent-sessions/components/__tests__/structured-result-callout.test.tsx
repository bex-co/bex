import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { StructuredResultCallout } from "@/features/agent-sessions/components/structured-result-callout";
import { agentSessionView } from "@/test/mocks/agent-session";
import type { AgentSessionView } from "@/features/agent-sessions/types";

const turnResult = vi.fn();
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-turn-result",
  () => ({
    useAgentSessionTurnResult: (...args: unknown[]) => turnResult(...args),
  }),
);

function withResult(result: string | null): AgentSessionView["outcome"] {
  return {
    state: "idle",
    steerable: true,
    turn: 1,
    summary: null,
    deliveryNote: null,
    failureReasonCode: null,
    result,
    cost: null,
  };
}

beforeEach(() => {
  turnResult.mockReset();
  turnResult.mockReturnValue(null);
});

describe("StructuredResultCallout", () => {
  it("renders nothing for a free-form turn", () => {
    const { container } = render(
      <StructuredResultCallout session={agentSessionView()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("shows outcome.result as pretty-printed JSON", () => {
    render(
      <StructuredResultCallout
        session={agentSessionView({
          phase: "completed",
          outcome: withResult('{"files":2,"ok":true}'),
        })}
      />,
    );
    const block = screen.getByTestId("agent-session-result");
    expect(block).toHaveAttribute("data-result-state", "final");
    expect(screen.getByText("Result")).toBeInTheDocument();
    expect(block.querySelector("pre")?.textContent).toBe(
      '{\n  "files": 2,\n  "ok": true\n}',
    );
  });

  it("labels the in-flight value a draft while the turn runs", () => {
    turnResult.mockReturnValue({ turn: 1, result: '{"n":1}', final: false });
    render(<StructuredResultCallout session={agentSessionView()} />);
    expect(screen.getByText("Draft result")).toBeInTheDocument();
    expect(screen.getByText(/Not final yet/)).toBeInTheDocument();
    expect(turnResult).toHaveBeenCalledWith("as-1", true);
  });

  it("prefers the final outcome.result over a draft", () => {
    turnResult.mockReturnValue({ turn: 1, result: '{"n":1}', final: false });
    render(
      <StructuredResultCallout
        session={agentSessionView({
          phase: "completed",
          outcome: withResult('{"n":2}'),
        })}
      />,
    );
    expect(screen.getByText("Result")).toBeInTheDocument();
    expect(screen.queryByText("Draft result")).not.toBeInTheDocument();
  });

  it("does not show a final turn value as a draft before outcome catches up", () => {
    turnResult.mockReturnValue({ turn: 1, result: '{"n":1}', final: true });
    const { container } = render(
      <StructuredResultCallout session={agentSessionView()} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
