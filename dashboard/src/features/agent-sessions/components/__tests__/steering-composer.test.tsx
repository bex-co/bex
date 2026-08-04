import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SteeringComposer } from "@/features/agent-sessions/components/steering-composer";
import type { ConversationChatHandle } from "@/features/agent-sessions/components/session-conversation";
import type {
  AgentSessionPhase,
  AgentSessionView,
} from "@/features/agent-sessions/types";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";

const steer = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ steer }),
}));

const toastSuccess = vi.fn();
vi.mock("sonner", () => ({
  toast: { success: (m: string) => toastSuccess(m) },
}));

beforeEach(() => {
  steer.mockReset();
  steer.mockResolvedValue({ session: { id: "as-1" } });
  toastSuccess.mockReset();
});

/** A view whose phase drives `isTerminal`/`isSteerable` like the mapper does. */
function view(phase: AgentSessionPhase): AgentSessionView {
  const isTerminal = ["completed", "failed", "canceled"].includes(phase);
  const isSteerable = ["completed", "failed"].includes(phase);
  return {
    id: "as-1",
    ownerId: "tea-1",
    repo: "acme/widgets",
    branch: "bex-agent/fix",
    agentConfig: {
      agent: "claude",
      model: null,
      modelEndpoint: null,
      task: "t",
      template: null,
    },
    sandboxId: null,
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
    isTerminal,
    isSteerable,
  };
}

function chatHandle(status: string): ConversationChatHandle {
  return { status, sendMessage: vi.fn().mockResolvedValue(undefined) };
}

describe("SteeringComposer state routing", () => {
  it("idle (completed) session → redispatches via the steer mutation, not the live chat", async () => {
    const chat = chatHandle("ready");
    const onSteered = vi.fn();
    const user = userEvent.setup();
    render(
      <SteeringComposer
        session={view("completed")}
        chat={chat}
        onSteered={onSteered}
      />,
    );

    await user.type(screen.getByRole("textbox"), "try again with tests");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "try again with tests"),
    );
    expect(chat.sendMessage).not.toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalled();
    expect(onSteered).toHaveBeenCalled();
  });

  it("live (running) session → sends through the conversation useChat, not the mutation", async () => {
    const chat = chatHandle("ready");
    const user = userEvent.setup();
    render(<SteeringComposer session={view("running")} chat={chat} />);

    await user.type(screen.getByRole("textbox"), "focus on the parser");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(chat.sendMessage).toHaveBeenCalledWith("focus on the parser"),
    );
    expect(steer).not.toHaveBeenCalled();
  });

  it("disables live steering with a reason when the stream handle is missing", () => {
    render(<SteeringComposer session={view("running")} chat={null} />);
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(
      screen.getByText(
        "The conversation stream is unavailable, so live steering is paused.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
  });

  it("disables the composer with a reason while canceling", () => {
    render(
      <SteeringComposer
        session={view("canceling")}
        chat={chatHandle("ready")}
      />,
    );
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(
      screen.getByText(/This session is being canceled/),
    ).toBeInTheDocument();
  });

  it("keeps the live path enabled but blocks submit while a turn is in flight", () => {
    render(
      <SteeringComposer
        session={view("running")}
        chat={chatHandle("streaming")}
      />,
    );
    // Input stays enabled (you can compose the next message), but sending is
    // blocked until the in-flight turn finishes.
    expect(screen.getByRole("textbox")).not.toBeDisabled();
    expect(screen.getByText(/A turn is in progress/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sending…" })).toBeDisabled();
  });

  it("surfaces a typed redispatch error inline instead of throwing", async () => {
    steer.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_NOT_STEERABLE", "server copy", {
        phase: "completed",
      }),
    );
    const user = userEvent.setup();
    render(<SteeringComposer session={view("completed")} chat={null} />);

    await user.type(screen.getByRole("textbox"), "go");
    await user.click(screen.getByRole("button", { name: "Send" }));

    expect(await screen.findByText("Couldn't send")).toBeInTheDocument();
    expect(
      screen.getByText(/can't be steered in its current phase \(completed\)/),
    ).toBeInTheDocument();
  });
});
