import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FailureCallout } from "@/features/agent-sessions/components/failure-callout";
import { agentSessionView } from "@/test/mocks/agent-session";

// Retry on a failed session steers it; with messages already waiting the steer
// is queued behind them (ADR047 D12), and the toast says so.

const steer = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ steer }),
}));
vi.mock("@tanstack/react-router", async (importActual) => ({
  ...(await importActual<typeof import("@tanstack/react-router")>()),
  useNavigate: () => vi.fn(),
}));
const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (m: string) => toastSuccess(m), error: vi.fn() },
}));

beforeEach(() => {
  steer.mockReset();
  toastSuccess.mockReset();
});

const failed = agentSessionView({
  phase: "failed",
  failureReason: "agent turn failed",
});

describe("FailureCallout retry and the queue", () => {
  it("says the retry was queued when messages were waiting", async () => {
    steer.mockResolvedValue({
      session: { id: "as-1" },
      queuedMessage: { id: "aqm-1" },
    });
    const user = userEvent.setup();
    render(<FailureCallout session={failed} />);
    await user.click(screen.getByRole("button", { name: /Retry/ }));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        "Queued. It runs when the current turn ends.",
      ),
    );
  });

  it("says a new turn is starting when the retry was not queued", async () => {
    steer.mockResolvedValue({ session: { id: "as-1" }, queuedMessage: null });
    const user = userEvent.setup();
    render(<FailureCallout session={failed} />);
    await user.click(screen.getByRole("button", { name: /Retry/ }));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        "Sent — redispatching a new turn.",
      ),
    );
  });
});
