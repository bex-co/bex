import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionDetailHeader } from "@/features/agent-sessions/components/session-detail-header";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import { agentSessionView } from "@/test/mocks/agent-session";

// Stop turn beside Cancel (ADR047 D12): Stop ends only the running turn, asks
// no confirmation and calls interruptAgentSession; Cancel keeps its confirm
// dialog and its terminal verb.

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: React.ReactNode }) => (
    <a {...rest}>{children}</a>
  ),
  useNavigate: () => vi.fn(),
}));
const mutations = vi.hoisted(() => ({
  cancel: vi.fn(),
  interrupt: vi.fn(),
  archive: vi.fn(),
  unarchive: vi.fn(),
  deleteSession: vi.fn(),
  setTags: vi.fn(),
}));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => mutations,
}));
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));
vi.mock("@/features/agent-automations/hooks/use-agent-automation", () => ({
  useAgentAutomation: () => ({
    automation: null,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-model-cost", () => ({
  useAgentSessionModelCost: () => ({ cost: null, loading: false }),
}));
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-applied-secrets",
  () => ({ useAgentSessionAppliedSecrets: () => [] }),
);

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: {
    success: (m: string) => toastSuccess(m),
    error: (m: string) => toastError(m),
  },
}));

function view(over: Partial<AgentSessionView> = {}): AgentSessionView {
  return agentSessionView({
    phase: "running",
    sandboxId: "sbx-1",
    turns: 2,
    ...over,
  });
}

const stopButton = () => screen.getByRole("button", { name: "Stop turn" });

beforeEach(() => {
  for (const fn of Object.values(mutations)) fn.mockReset();
  mutations.interrupt.mockResolvedValue(view());
  mutations.cancel.mockResolvedValue(view({ phase: "canceling" }));
  toastSuccess.mockReset();
  toastError.mockReset();
});

describe("Stop turn button", () => {
  it("sits beside Cancel with a label of its own while a turn runs", () => {
    render(<SessionDetailHeader session={view()} />);
    expect(stopButton()).toBeEnabled();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();
    // Two verbs, two labels: Stop turn is never worded as Cancel.
    expect(stopButton()).not.toBe(
      screen.getByRole("button", { name: "Cancel" }),
    );
    expect(stopButton()).toHaveAttribute(
      "aria-keyshortcuts",
      "Control+Shift+Backspace Meta+Shift+Backspace",
    );
  });

  it("calls interruptAgentSession at once, with no confirmation, and re-reads the session", async () => {
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<SessionDetailHeader session={view()} onChanged={onChanged} />);
    await user.click(stopButton());

    await waitFor(() =>
      expect(mutations.interrupt).toHaveBeenCalledWith("as-1"),
    );
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(mutations.cancel).not.toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith(
      "Stopping the turn. It ends within about 15 seconds.",
    );
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("rests while the call is in flight and after it, until the turn has ended", async () => {
    let finish: (v: AgentSessionView) => void = () => undefined;
    mutations.interrupt.mockReturnValue(
      new Promise<AgentSessionView>((resolve) => {
        finish = resolve;
      }),
    );
    const user = userEvent.setup();
    const { rerender } = render(<SessionDetailHeader session={view()} />);
    await user.click(stopButton());

    const resting = screen.getByRole("button", { name: "Stopping…" });
    expect(resting).toBeDisabled();
    // A second click during the call does nothing.
    await user.click(resting);
    expect(mutations.interrupt).toHaveBeenCalledTimes(1);

    // The 202 is not the end of the turn: the session reads running for up to
    // a Completer tick, and Stop turn does not offer itself again meanwhile.
    finish(view());
    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
    rerender(<SessionDetailHeader session={view()} />);
    expect(screen.getByRole("button", { name: "Stopping…" })).toBeDisabled();
    fireEvent.keyDown(window, {
      key: "Backspace",
      ctrlKey: true,
      shiftKey: true,
    });
    expect(mutations.interrupt).toHaveBeenCalledTimes(1);

    // The turn settles: the session is no longer running, so there is no turn
    // left to stop and no Stop turn button.
    rerender(<SessionDetailHeader session={view({ phase: "completed" })} />);
    expect(screen.queryByRole("button", { name: "Stop turn" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Stopping…" })).toBeNull();
  });

  it("comes back for the next turn when the stopped one was replaced between two reads", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<SessionDetailHeader session={view()} />);
    await user.click(stopButton());
    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
    expect(screen.getByRole("button", { name: "Stopping…" })).toBeDisabled();

    // The read never saw the idle phase: the next queued message's turn is
    // already running (turn 3), and that turn can be stopped.
    rerender(<SessionDetailHeader session={view({ turns: 3 })} />);
    expect(stopButton()).toBeEnabled();
    await user.click(stopButton());
    await waitFor(() => expect(mutations.interrupt).toHaveBeenCalledTimes(2));
  });

  it.each([
    ["Ctrl", { ctrlKey: true }],
    ["Cmd", { metaKey: true }],
  ])("stops the turn with %s+Shift+Backspace", async (_name, modifier) => {
    render(<SessionDetailHeader session={view()} />);
    fireEvent.keyDown(window, {
      key: "Backspace",
      shiftKey: true,
      ...modifier,
    });
    await waitFor(() => expect(mutations.interrupt).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(toastSuccess).toHaveBeenCalled());
  });

  it("ignores Backspace without the full chord", () => {
    render(<SessionDetailHeader session={view()} />);
    fireEvent.keyDown(window, { key: "Backspace" });
    fireEvent.keyDown(window, { key: "Backspace", ctrlKey: true });
    fireEvent.keyDown(window, { key: "Backspace", shiftKey: true });
    fireEvent.keyDown(window, {
      key: "Enter",
      ctrlKey: true,
      shiftKey: true,
    });
    expect(mutations.interrupt).not.toHaveBeenCalled();
  });

  it("does not answer the shortcut while the sandbox is still starting", () => {
    render(
      <SessionDetailHeader
        session={view({ phase: "creating", sandboxId: "" })}
      />,
    );
    expect(stopButton()).toBeDisabled();
    fireEvent.keyDown(window, {
      key: "Backspace",
      ctrlKey: true,
      shiftKey: true,
    });
    expect(mutations.interrupt).not.toHaveBeenCalled();
  });

  it.each(["creating", "resuming", "redispatching"] as const)(
    "is offered but rests while the session is %s",
    (phase) => {
      render(<SessionDetailHeader session={view({ phase })} />);
      expect(stopButton()).toBeDisabled();
    },
  );

  it.each(["completed", "failed", "canceling", "canceled"] as const)(
    "is not offered once the session is %s",
    (phase) => {
      render(<SessionDetailHeader session={view({ phase })} />);
      expect(screen.queryByRole("button", { name: "Stop turn" })).toBeNull();
      fireEvent.keyDown(window, {
        key: "Backspace",
        ctrlKey: true,
        shiftKey: true,
      });
      expect(mutations.interrupt).not.toHaveBeenCalled();
    },
  );

  it("stops listening for the shortcut once the turn is over", () => {
    const { rerender } = render(<SessionDetailHeader session={view()} />);
    rerender(<SessionDetailHeader session={view({ phase: "completed" })} />);
    fireEvent.keyDown(window, {
      key: "Backspace",
      metaKey: true,
      shiftKey: true,
    });
    expect(mutations.interrupt).not.toHaveBeenCalled();
  });

  it("shows a refusal's message and re-reads the session", async () => {
    mutations.interrupt.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_NO_TURN_RUNNING", "server copy", {}),
    );
    const onChanged = vi.fn();
    const user = userEvent.setup();
    render(<SessionDetailHeader session={view()} onChanged={onChanged} />);
    await user.click(stopButton());

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "No turn is running, so there is nothing to stop.",
      ),
    );
    expect(toastSuccess).not.toHaveBeenCalled();
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    await waitFor(() => expect(stopButton()).toBeEnabled());
  });

  it("asks to retry when the turn's sandbox was not up yet", async () => {
    mutations.interrupt.mockRejectedValue(
      new AgentSessionError("AGENT_SESSION_TURN_STARTING", "server copy", {}),
    );
    const user = userEvent.setup();
    render(<SessionDetailHeader session={view()} />);
    await user.click(stopButton());
    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "The turn is still starting, so there is nothing to stop yet. Try again in a few seconds.",
      ),
    );
  });
});

describe("Cancel is unchanged beside Stop turn", () => {
  it("still asks for confirmation and only then cancels the session", async () => {
    const user = userEvent.setup();
    render(<SessionDetailHeader session={view()} />);
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Cancel this session?");
    expect(dialog).toHaveTextContent(
      "This stops the agent. Any commits already pushed to the branch and the draft PR are preserved.",
    );
    expect(mutations.cancel).not.toHaveBeenCalled();
    expect(mutations.interrupt).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Cancel session" }));
    await waitFor(() => expect(mutations.cancel).toHaveBeenCalledWith("as-1"));
    expect(mutations.interrupt).not.toHaveBeenCalled();
    expect(toastSuccess).toHaveBeenCalledWith("Canceling the session…");
  });

  it("is still offered, disabled, while canceling, with no Stop turn", () => {
    render(<SessionDetailHeader session={view({ phase: "canceling" })} />);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Stop turn" })).toBeNull();
  });

  it("is still absent from a finished session", () => {
    render(<SessionDetailHeader session={view({ phase: "completed" })} />);
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
  });
});
