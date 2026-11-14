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
vi.mock("@/features/agent-environment/hooks/use-agent-skills", () => ({
  useAgentSkills: () => ({
    skills: [
      {
        __typename: "AgentSkill",
        id: "skl-1",
        name: "run-tests",
        description: "Run the integration suite",
        triggers: ["user", "model"],
        enabled: true,
        version: 1,
      },
    ],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    read: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    saving: false,
  }),
}));

vi.mock("sonner", () => ({
  toast: { success: (m: string) => toastSuccess(m) },
}));

// What a steer answers when the session was busy and queued the message.
const queuedResult = {
  session: { id: "as-1" },
  queuedMessage: {
    id: "aqm-1",
    position: 1,
    prompt: "queued",
    author: "alice",
    attachmentCount: 0,
    blockedReason: null,
    createdAt: null,
  },
};

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

  it("sends chosen skills with the follow-up and clears them (ADR099)", async () => {
    const user = userEvent.setup();
    render(<SteeringComposer session={view("completed")} chat={null} />);
    await user.click(screen.getByRole("button", { name: "Skills" }));
    await user.click(await screen.findByLabelText(/^run-tests/));
    await user.type(
      screen.getByLabelText("Arguments for run-tests"),
      "--only api",
    );
    await user.keyboard("{Escape}");
    await user.type(screen.getByRole("textbox"), "and now the API");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "and now the API", {
        attachmentIds: [],
        skills: [{ name: "run-tests", arguments: "--only api" }],
      }),
    );
    expect(screen.queryByTestId("agent-composer-skill-chips")).toBeNull();
  });

  it("offers the skill picker while a turn runs: a queued message keeps its skills", async () => {
    steer.mockResolvedValue(queuedResult);
    const user = userEvent.setup();
    render(<SteeringComposer session={view("running")} chat={null} />);
    await user.click(screen.getByRole("button", { name: "Skills" }));
    await user.click(await screen.findByLabelText(/^run-tests/));
    await user.keyboard("{Escape}");
    await user.type(screen.getByRole("textbox"), "and then the API");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "and then the API", {
        attachmentIds: [],
        skills: [{ name: "run-tests" }],
        idempotencyKey: expect.any(String),
      }),
    );
  });

  it("stays enabled while a turn runs and queues a send through the steer mutation, not the live chat", async () => {
    steer.mockResolvedValue(queuedResult);
    const chat = chatHandle("streaming");
    const onSteered = vi.fn();
    const onOptimisticSteer = vi.fn();
    const user = userEvent.setup();
    render(
      <SteeringComposer
        session={view("running")}
        chat={chat}
        onSteered={onSteered}
        onOptimisticSteer={onOptimisticSteer}
      />,
    );

    const box = screen.getByRole("textbox");
    expect(box).toBeEnabled();
    expect(
      screen.getByText(/Your message is queued and runs as its own turn/),
    ).toBeInTheDocument();
    expect(box).toHaveAttribute(
      "placeholder",
      "Queue a message. It runs when the current turn ends.",
    );

    await user.type(box, "then run the linter");
    await user.click(screen.getByRole("button", { name: "Send" }));

    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "then run the linter", {
        attachmentIds: [],
        idempotencyKey: expect.any(String),
      }),
    );
    expect(chat.sendMessage).not.toHaveBeenCalled();
    // A queued message is a chip, never an echo in the transcript.
    expect(onOptimisticSteer).not.toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith(
      "Queued. It runs when the current turn ends.",
    );
    expect(onSteered).toHaveBeenCalled();
    expect(box).toHaveValue("");
  });

  it.each(["creating", "resuming", "redispatching"] as const)(
    "queues a send while the session is %s",
    async (phase) => {
      steer.mockResolvedValue(queuedResult);
      const user = userEvent.setup();
      render(<SteeringComposer session={view(phase)} chat={null} />);
      expect(screen.getByRole("textbox")).toBeEnabled();
      await user.type(screen.getByRole("textbox"), "later");
      await user.click(screen.getByRole("button", { name: "Send" }));
      await waitFor(() => expect(steer).toHaveBeenCalledTimes(1));
    },
  );

  it("has nothing to send while the box is empty", () => {
    render(<SteeringComposer session={view("running")} chat={null} />);
    expect(screen.getByRole("textbox")).toBeEnabled();
    expect(screen.getByRole("button", { name: "Send" })).toBeDisabled();
  });

  it("queues behind messages that already wait, even on an idle session", async () => {
    steer.mockResolvedValue(queuedResult);
    const onOptimisticSteer = vi.fn();
    const user = userEvent.setup();
    render(
      <SteeringComposer
        session={view("completed")}
        chat={null}
        queuedCount={2}
        onOptimisticSteer={onOptimisticSteer}
      />,
    );
    expect(
      screen.getByText(/Your message is queued and runs as its own turn/),
    ).toBeInTheDocument();
    await user.type(screen.getByRole("textbox"), "after the others");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() =>
      expect(steer).toHaveBeenCalledWith("as-1", "after the others", {
        attachmentIds: [],
        idempotencyKey: expect.any(String),
      }),
    );
    expect(onOptimisticSteer).not.toHaveBeenCalled();
  });

  it("takes the echo back when an idle-looking session queued the message after all", async () => {
    // The page's last read said idle; by the time the steer landed a turn ran.
    steer.mockResolvedValue(queuedResult);
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
    await waitFor(() =>
      expect(onOptimisticSteer).toHaveBeenLastCalledWith(null),
    );
    expect(onOptimisticSteer).toHaveBeenNthCalledWith(1, "go");
    expect(toastSuccess).toHaveBeenCalledWith(
      "Queued. It runs when the current turn ends.",
    );
  });

  it("reuses the idempotency key to retry the same queued message, and mints a new one for the next", async () => {
    steer.mockRejectedValueOnce(new Error("offline"));
    steer.mockResolvedValue(queuedResult);
    const user = userEvent.setup();
    render(<SteeringComposer session={view("running")} chat={null} />);

    await user.type(screen.getByRole("textbox"), "retry me");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("Couldn't send");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(steer).toHaveBeenCalledTimes(2));
    const [first, second] = steer.mock.calls.map(
      (call) => call[2].idempotencyKey,
    );
    expect(first).toBe(second);

    // Sent: the next message is a different request and gets its own key.
    await user.type(screen.getByRole("textbox"), "another");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(steer).toHaveBeenCalledTimes(3));
    expect(steer.mock.calls[2][2].idempotencyKey).not.toBe(first);
  });

  it("sends no idempotency key for a send that starts a turn", async () => {
    const user = userEvent.setup();
    render(<SteeringComposer session={view("completed")} chat={null} />);
    await user.type(screen.getByRole("textbox"), "go");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await waitFor(() => expect(steer).toHaveBeenCalledTimes(1));
    expect(steer.mock.calls[0][2]).toEqual({ attachmentIds: [] });
  });

  it("does not call a normal starting/running turn broken while its handle is missing", () => {
    render(<SteeringComposer session={view("running")} chat={null} />);
    expect(screen.getByRole("textbox")).toBeEnabled();
    expect(
      screen.getByText(/Your message is queued and runs as its own turn/),
    ).toBeInTheDocument();
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

describe("SteeringComposer queue refusals (ADR047 D12)", () => {
  async function sendAndFail(err: unknown) {
    steer.mockRejectedValue(err);
    const user = userEvent.setup();
    render(<SteeringComposer session={view("running")} chat={null} />);
    await user.type(screen.getByRole("textbox"), "one more");
    await user.click(screen.getByRole("button", { name: "Send" }));
    await screen.findByText("Couldn't send");
    // A failed send keeps the text so it can be sent again.
    expect(screen.getByRole("textbox")).toHaveValue("one more");
  }

  it("says a full queue is full, with its limit", async () => {
    await sendAndFail(
      new AgentSessionError("AGENT_SESSION_QUEUE_FULL", "server copy", {
        id: "as-1",
        max: 10,
      }),
    );
    expect(
      screen.getByText(/already has 10 messages queued/),
    ).toBeInTheDocument();
  });

  it("explains why a message with inline secrets is not queued", async () => {
    await sendAndFail(
      new AgentSessionError(
        "AGENT_SESSION_QUEUE_INLINE_SECRETS",
        "server copy",
        { phase: "running" },
      ),
    );
    expect(
      screen.getByText(/inline secrets can't wait in the queue/),
    ).toBeInTheDocument();
  });

  it("shows a canceled session's refusal", async () => {
    await sendAndFail(
      new AgentSessionError("AGENT_SESSION_NOT_STEERABLE", "server copy", {
        phase: "canceled",
      }),
    );
    expect(
      screen.getByText(/can't be steered in its current phase \(canceled\)/),
    ).toBeInTheDocument();
  });

  it("falls back to the server's sentence for a code it has no copy for", async () => {
    await sendAndFail(
      new AgentSessionError(
        "AGENT_SESSION_SOMETHING_NEW",
        "the server copy",
        {},
      ),
    );
    expect(screen.getByText("the server copy")).toBeInTheDocument();
  });

  it("falls back to generic copy for a failure with no code", async () => {
    await sendAndFail(new Error("offline"));
    expect(screen.getByRole("alert")).not.toHaveTextContent("offline");
  });

  it("disables the composer for a canceled session, with its reason", () => {
    render(<SteeringComposer session={view("canceled")} chat={null} />);
    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(screen.getByText("This session was canceled.")).toBeInTheDocument();
  });
});
