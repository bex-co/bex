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
import { agentSessionView } from "@/test/mocks/agent-session";

const steer = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ steer }),
}));

const capabilitiesState = {
  capabilities: {
    enabled: true,
    modelKeyReady: true,
    github: { connected: true },
    attachments: { enabled: true, maxBytes: 25 * 1024 * 1024, maxPerTurn: 10 },
  },
  loading: false,
  error: undefined,
};
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({
    useAgentSessionCapabilities: () => capabilitiesState,
  }),
);

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));

const uploadOwners = vi.hoisted(() => [] as (string | null)[]);
vi.mock("@/features/agent-sessions/lib/attachments", async (orig) => ({
  ...(await orig<typeof import("@/features/agent-sessions/lib/attachments")>()),
  uploadAttachment: (file: File, opts: { ownerId: string | null }) => {
    uploadOwners.push(opts.ownerId);
    return Promise.resolve({
      id: "att-7",
      name: file.name,
      mediaType: "image/png",
      size: file.size,
    });
  },
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
  return agentSessionView({ phase, task: "t" });
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
      expect(steer).toHaveBeenCalledWith("as-1", "try again with tests", {
        attachmentIds: [],
      }),
    );
    expect(chat.sendMessage).not.toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalled();
    expect(onSteered).toHaveBeenCalled();
  });

  it("live (running) session refuses non-durable POST turns until it settles", async () => {
    const chat = chatHandle("ready");
    render(<SteeringComposer session={view("running")} chat={chat} />);

    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(
      screen.getByText(
        "Wait for the current turn to finish before sending a follow-up.",
      ),
    ).toBeInTheDocument();
    expect(chat.sendMessage).not.toHaveBeenCalled();
    expect(steer).not.toHaveBeenCalled();
  });

  it("does not call a normal starting/running turn broken while its handle is missing", () => {
    render(<SteeringComposer session={view("running")} chat={null} />);
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(
      screen.getByText(
        "Wait for the current turn to finish before sending a follow-up.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
  });

  it("uses unavailable copy only for an explicitly broken stream", () => {
    render(
      <SteeringComposer
        session={view("running")}
        chat={null}
        conversationState="broken"
      />,
    );
    expect(
      screen.getByText(
        "The conversation stream is unavailable, so live steering is paused.",
      ),
    ).toBeInTheDocument();
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

  it("keeps the live path disabled while a turn is in flight", () => {
    render(
      <SteeringComposer
        session={view("running")}
        chat={chatHandle("streaming")}
      />,
    );
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.getByText(/Wait for the current turn/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
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

  // w2/m64 optimistic redispatch echo.
  it("echoes the redispatch prompt optimistically on submit", async () => {
    const onOptimisticSteer = vi.fn();
    const user = userEvent.setup();
    render(
      <SteeringComposer
        session={view("completed")}
        chat={null}
        onOptimisticSteer={onOptimisticSteer}
      />,
    );

    await user.type(screen.getByRole("textbox"), "also update the docs");
    await user.click(screen.getByRole("button", { name: "Send" }));

    // The echo fires with the prompt, and it does so BEFORE the steer resolves.
    expect(onOptimisticSteer).toHaveBeenCalledWith("also update the docs");
    await waitFor(() => expect(steer).toHaveBeenCalled());
    // A successful redispatch never rolls the echo back.
    expect(onOptimisticSteer).not.toHaveBeenCalledWith(null);
  });

  it("rolls the optimistic echo back when the redispatch is rejected", async () => {
    steer.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_TURN_IN_FLIGHT", "server copy", {
        phase: "redispatching",
      }),
    );
    const onOptimisticSteer = vi.fn();
    const user = userEvent.setup();
    render(
      <SteeringComposer
        session={view("completed")}
        chat={null}
        onOptimisticSteer={onOptimisticSteer}
      />,
    );

    await user.type(screen.getByRole("textbox"), "go");
    await user.click(screen.getByRole("button", { name: "Send" }));

    // Echoed on submit, then rolled back (null) once the mutation rejects.
    expect(onOptimisticSteer).toHaveBeenCalledWith("go");
    await waitFor(() => expect(onOptimisticSteer).toHaveBeenCalledWith(null));
  });

  it("does not submit or echo on the disabled live path", () => {
    const onOptimisticSteer = vi.fn();
    const chat = chatHandle("ready");
    render(
      <SteeringComposer
        session={view("running")}
        chat={chat}
        onOptimisticSteer={onOptimisticSteer}
      />,
    );

    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(chat.sendMessage).not.toHaveBeenCalled();
    expect(onOptimisticSteer).not.toHaveBeenCalled();
  });
});

describe("SteeringComposer attachments (ADR095)", () => {
  it("sends uploaded files with the follow-up and clears the chips", async () => {
    const user = userEvent.setup();
    uploadOwners.length = 0;
    // The session lives in another workspace than the one selected.
    render(
      <SteeringComposer
        session={{ ...view("completed"), ownerId: "tea-other" }}
        chat={null}
      />,
    );
    await user.upload(
      screen.getByTestId("attachment-input"),
      new File(["png"], "after.png", { type: "image/png" }),
    );
    await waitFor(() =>
      expect(screen.getByTestId("attachment-chip")).toHaveAttribute(
        "data-status",
        "ready",
      ),
    );
    await user.type(screen.getByRole("textbox"), "does this look right?");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "does this look right?", {
        attachmentIds: ["att-7"],
      }),
    );
    await waitFor(() =>
      expect(screen.queryByTestId("attachment-chip")).toBeNull(),
    );
    expect(uploadOwners).toEqual(["tea-other"]);
  });

  it("offers no attachments while the session cannot take a message", () => {
    render(<SteeringComposer session={view("canceled")} chat={null} />);
    expect(
      screen.queryByRole("button", { name: "Attach files" }),
    ).not.toBeInTheDocument();
  });
});
