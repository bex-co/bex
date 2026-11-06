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

  // 076 live check: a list read carries no outcome.result, so a succeeded
  // session's final result must come from its transcript turn.
  it("shows the turn's final result when the cached outcome lacks it", () => {
    turnResult.mockReturnValue({ turn: 1, result: '{"n":1}', final: true });
    render(
      <StructuredResultCallout
        session={agentSessionView({
          phase: "completed",
          outcome: withResult(null),
        })}
      />,
    );
    const block = screen.getByTestId("agent-session-result");
    expect(block).toHaveAttribute("data-result-state", "final");
    expect(screen.getByText("Result")).toBeInTheDocument();
    expect(screen.queryByText(/Not final yet/)).not.toBeInTheDocument();
    expect(block.querySelector("pre")?.textContent).toBe('{\n  "n": 1\n}');
  });

  it("keeps a non-final value a draft after the turn ends", () => {
    turnResult.mockReturnValue({ turn: 1, result: '{"n":1}', final: false });
    render(
      <StructuredResultCallout
        session={agentSessionView({
          phase: "completed",
          outcome: withResult(null),
        })}
      />,
    );
    expect(screen.getByTestId("agent-session-result")).toHaveAttribute(
      "data-result-state",
      "draft",
    );
  });
});
