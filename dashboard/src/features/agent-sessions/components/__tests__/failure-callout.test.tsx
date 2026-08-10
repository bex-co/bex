import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";

const steer = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ steer }),
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (m: string) => toastSuccess(m),
    error: (m: string) => toastError(m),
  },
}));

beforeEach(() => {
  steer.mockReset();
  steer.mockResolvedValue({ session: { id: "as-1" } });
  toastSuccess.mockReset();
  toastError.mockReset();
});

function view(
  phase: AgentSessionPhase,
  over: Partial<AgentSessionView> = {},
): AgentSessionView {
  return {
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "fix the failing tests",
      template: null,
    },
    sandboxId: null,
    sshAddress: null,
    phase,
    status: "s",
    headSha: null,
    prUrl: null,
    prNumber: null,
    evidence: null,
    turns: 0,
    deliveryMode: null,
    failureReason: null,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    canceledAt: null,
    isTerminal: ["completed", "failed", "canceled"].includes(phase),
    isSteerable: ["completed", "failed"].includes(phase),
    ...over,
  };
}

describe("FailureCallout", () => {
  it("renders nothing for a non-failed session", () => {
    const { container } = render(<FailureCallout session={view("running")} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the recorded failureReason when the Completer named one", () => {
    render(
      <FailureCallout
        session={view("failed", { failureReason: "agent turn failed" })}
      />,
    );
    expect(screen.getByText("Session failed")).toBeInTheDocument();
    expect(screen.getByText("agent turn failed")).toBeInTheDocument();
  });

  it("falls back to the lifecycle status for a background-provisioning failure", () => {
    // A dispatch failure stamps the reason on `status`, not `failureReason`.
    render(
      <FailureCallout
        session={view("failed", {
          failureReason: null,
          status: "sandbox create failed",
        })}
      />,
    );
    expect(screen.getByText("sandbox create failed")).toBeInTheDocument();
  });

  it("falls back to generic copy when the session carries no reason at all", () => {
    render(
      <FailureCallout
        session={view("failed", { failureReason: null, status: "" })}
      />,
    );
    expect(screen.getByText("The session failed to start.")).toBeInTheDocument();
  });

  it("retries by re-running the original task through the steer mutation", async () => {
    const onRetried = vi.fn();
    const user = userEvent.setup();
    render(
      <FailureCallout
        session={view("failed", { status: "sandbox create failed" })}
        onRetried={onRetried}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "fix the failing tests"),
    );
    expect(toastSuccess).toHaveBeenCalled();
    expect(onRetried).toHaveBeenCalled();
  });

  it("toasts an error and does not converge when the retry is rejected", async () => {
    steer.mockRejectedValue(new Error("boom"));
    const onRetried = vi.fn();
    const user = userEvent.setup();
    render(
      <FailureCallout
        session={view("failed", { status: "sandbox create failed" })}
        onRetried={onRetried}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(onRetried).not.toHaveBeenCalled();
  });
});
